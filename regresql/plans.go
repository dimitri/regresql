package regresql

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"
	"github.com/theherk/viper" // fork with write support
	"gopkg.in/yaml.v2"
)

/*
A query plan associates a Query parsed from a Path (name of the file on
disk) and a list of set of parameters used to run the query. Each set of
parameters as a name in Names[i] and a list of bindings in Bindings[i]. When
the query is executed we store its output in ResultSets[i].
*/
type Plan struct {
	Query      *Query
	Path       string // the file path where we read the Plan from
	Names      []string
	Bindings   []map[string]string
	ResultSets []ResultSet
}

// CreateEmptyPlan creates a YAML file where to store the set of parameters
// associated with a query.
func (q *Query) CreateEmptyPlan(dir string) (*Plan, error) {
	var names []string
	var bindings []map[string]string
	pfile := getPlanPath(q, dir)

	if _, err := os.Stat(pfile); !os.IsNotExist(err) {
		var p Plan
		return &p, fmt.Errorf("Plan file '%s' already exists", pfile)
	}

	if len(q.Vars) > 0 {
		names = make([]string, 1)
		bindings = make([]map[string]string, 1)

		names[0] = "1"
		bindings[0] = make(map[string]string)

		if q.Positional {
			// For positional queries, pre-fill from \bind defaults.
			for idx, varname := range q.Vars {
				if idx < len(q.BindDefaults) {
					bindings[0][varname] = q.BindDefaults[idx]
				} else {
					bindings[0][varname] = ""
				}
			}
		} else {
			// For named queries, pre-fill from \set defaults.
			for _, varname := range q.Vars {
				if def, ok := q.Defaults[varname]; ok {
					bindings[0][varname] = def
				} else {
					bindings[0][varname] = ""
				}
			}
		}
	} else {
		names = []string{}
		bindings = []map[string]string{}
	}

	plan := &Plan{q, pfile, names, bindings, []ResultSet{}}
	plan.Write()

	return plan, nil
}

// GetPlan instanciates a Plan from a Query, parsing a set of actual
// parameters when it exists.
func (q *Query) GetPlan(planDir string) (*Plan, error) {
	var plan *Plan
	pfile := getPlanPath(q, planDir)

	if _, err := os.Stat(pfile); os.IsNotExist(err) {
		if len(q.Params) == 0 {
			// No params and no plan file — perfectly valid.
			return &Plan{q, pfile,
				[]string{},
				[]map[string]string{},
				[]ResultSet{}}, nil
		}
		// Can we synthesise a plan from inline defaults?
		if q.Positional {
			// Positional mode: every $N must have a \bind default.
			if len(q.BindDefaults) >= len(q.Vars) {
				return &Plan{q, pfile,
					[]string{"1"},
					[]map[string]string{{}},
					[]ResultSet{}}, nil
			}
		} else {
			// Named mode: every :varname must have a \set default.
			allCovered := true
			for _, varname := range q.Vars {
				if _, ok := q.Defaults[varname]; !ok {
					allCovered = false
					break
				}
			}
			if allCovered {
				return &Plan{q, pfile,
					[]string{"1"},
					[]map[string]string{{}},
					[]ResultSet{}}, nil
			}
		}
		e := fmt.Errorf("Failed to get plan '%s': %s\n", pfile, err)
		return plan, e
	}

	data, err := ioutil.ReadFile(pfile)
	if err != nil {
		return plan, fmt.Errorf("Failed to read file '%s': %s\n", pfile, err)
	}

	// Parse the plan YAML into a generic map so we can handle both the
	// named-binding format and the positional-array format:
	//
	//   Named (map):
	//     "test1":
	//       name: "value"
	//
	//   Positional (array):
	//     "test1":
	//       - "val1"
	//       - "val2"
	var rawPlan map[string]interface{}
	if err := yaml.Unmarshal(data, &rawPlan); err != nil {
		return plan, fmt.Errorf("Failed to parse plan '%s': %s\n", pfile, err)
	}

	var bindings []map[string]string
	var names []string

	for tcName, tcVal := range rawPlan {
		names = append(names, tcName)
		bm := make(map[string]string)

		switch v := tcVal.(type) {
		case map[interface{}]interface{}:
			// Named-binding format
			for k, val := range v {
				bm[fmt.Sprintf("%v", k)] = fmt.Sprintf("%v", val)
			}
		case []interface{}:
			// Positional-array format: index 0 -> p1, index 1 -> p2, …
			for idx, val := range v {
				bm[fmt.Sprintf("p%d", idx+1)] = fmt.Sprintf("%v", val)
			}
		}
		bindings = append(bindings, bm)
	}

	return &Plan{q, pfile, names, bindings, []ResultSet{}}, nil
}

// Executes a plan and returns the filepath where the output has been
// written, for later comparing
func (p *Plan) Execute(db *sql.DB) error {
	if len(p.Query.Params) == 0 {
		// this Query has no plans, so don't loop over the bindings
		args := make([]interface{}, 0)
		res, err := QueryDB(db, p.Query.Query, args...)

		if err != nil {
			e := fmt.Errorf("Error executing query: %s\n%s\n",
				err,
				p.Query.Query)
			return e
		}
		result := make([]ResultSet, 1)
		result[0] = *res
		p.ResultSets = result
		return nil
	}

	// general case, with a plan and a set of Bindings to go through
	result := make([]ResultSet, len(p.Bindings))

	for i, bindings := range p.Bindings {
		sql, args, err := p.Query.Prepare(bindings)
		if err != nil {
			return fmt.Errorf("Error preparing query '%s': %s", p.Query.Path, err)
		}
		res, err := QueryDB(db, sql, args...)

		if err != nil {
			e := fmt.Errorf(
				"Error executing query with params: %v\n%s\n%s",
				args,
				err,
				sql)
			return e
		}
		result[i] = *res
	}
	p.ResultSets = result
	return nil
}

// WriteResultSets serialize the result of running a query, as a Pretty
// Printed output (comparable to a simplified `psql` output).
//
// When pgMajor > 0 the output files use a version-specific suffix
// (e.g. query.pg16.out) instead of the generic query.out name.
func (p *Plan) WriteResultSets(dir string, pgMajor int) error {
	for i, rs := range p.ResultSets {
		rsFileName := getResultSetPath(p, dir, i, pgMajor)
		err := rs.Write(rsFileName, true)

		if err != nil {
			e := fmt.Errorf(
				"Failed to write result set '%s': %s\n",
				rsFileName,
				err)
			return e
		}
		p.ResultSets[i].Filename = rsFileName
	}
	return nil
}

// Write a plan to disk in YAML format.
//
// For named-mode queries the existing Viper-based writer is used, producing:
//
//	"1":
//	  varname: value
//
// For positional-mode queries the plan is written as a YAML array directly
// via gopkg.in/yaml.v2, producing:
//
//	"1":
//	  - value1
//	  - value2
func (p *Plan) Write() {
	if len(p.Bindings) == 0 {
		fmt.Printf("Skipping Plan '%s': query uses no variable\n", p.Path)
		return
	}

	fmt.Printf("Creating Empty Plan '%s'\n", p.Path)

	if p.Query.Positional {
		// Build an ordered map of test-case-name -> []string for yaml.v2.
		// yaml.v2 marshals map[string][]string with keys in insertion order
		// only when using yaml.MapSlice; use a slice of structs instead.
		type yamlEntry struct {
			Name   string
			Values []string
		}
		entries := make([]yamlEntry, len(p.Names))
		for i, name := range p.Names {
			vals := make([]string, len(p.Query.Vars))
			for j, varname := range p.Query.Vars {
				vals[j] = p.Bindings[i][varname]
			}
			entries[i] = yamlEntry{name, vals}
		}

		// Marshal as a YAML mapping with array values.
		out := yaml.MapSlice{}
		for _, e := range entries {
			out = append(out, yaml.MapItem{Key: e.Name, Value: e.Values})
		}
		data, err := yaml.Marshal(out)
		if err != nil {
			fmt.Printf("Error marshalling positional plan '%s': %s\n", p.Path, err)
			return
		}
		if err := ioutil.WriteFile(p.Path, data, 0644); err != nil {
			fmt.Printf("Error writing positional plan '%s': %s\n", p.Path, err)
		}
		return
	}

	// Named mode: use viper (fork with write support).
	v := viper.New()
	v.SetConfigType("yaml")

	for i, bindings := range p.Bindings {
		for key, value := range bindings {
			vpath := fmt.Sprintf("%s.%s", p.Names[i], key)
			v.Set(vpath, value)
		}
	}
	v.WriteConfigAs(p.Path)
}

func getPlanPath(q *Query, targetdir string) string {
	planPath := filepath.Join(targetdir, filepath.Base(q.Path))
	planPath = strings.TrimSuffix(planPath, path.Ext(planPath))
	planPath = planPath + ".yaml"

	return planPath
}

func getResultSetPath(p *Plan, targetdir string, index int, pgMajor int) string {
	var rsFileName string
	basename := strings.TrimSuffix(filepath.Base(p.Path), path.Ext(p.Path))

	versionSuffix := ""
	if pgMajor > 0 {
		versionSuffix = fmt.Sprintf(".pg%d", pgMajor)
	}

	if len(p.Query.Params) == 0 {
		rsFileName = fmt.Sprintf("%s%s.out", basename, versionSuffix)
	} else {
		rsFileName = fmt.Sprintf("%s.%s%s.out", basename, p.Names[index], versionSuffix)
	}
	return filepath.Join(targetdir, rsFileName)
}
