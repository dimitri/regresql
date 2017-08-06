package regresql

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"
	"github.com/theherk/viper" // fork with write support
)

type Plan struct {
	Query      *Query
	Path       string // the file path where we read the Plan from
	Names      []string
	Bindings   []map[string]string
	ResultSets []ResultSet
}

func (q *Query) CreateEmptyPlan(dir string) *Plan {
	var names []string
	var bindings []map[string]string
	pfile := getPlanPath(q, dir)

	if _, err := os.Stat(pfile); !os.IsNotExist(err) {
		panic(fmt.Sprintf("Fatal: plan file '%s' already exists", pfile))
	}

	if len(q.Vars) > 0 {
		names = make([]string, 1)
		bindings = make([]map[string]string, 1)

		names[0] = "1"
		bindings[0] = make(map[string]string)
		for _, varname := range q.Vars {
			bindings[0][varname] = ""
		}
	} else {
		names = []string{}
		bindings = []map[string]string{}
	}

	plan := &Plan{q, pfile, names, bindings, []ResultSet{}}
	plan.Write()

	return plan
}

func (q *Query) GetPlan(planDir string) *Plan {
	pfile := getPlanPath(q, planDir)

	if _, err := os.Stat(pfile); os.IsNotExist(err) {
		return &Plan{q, pfile,
			[]string{}, []map[string]string{}, []ResultSet{}}
	}

	v := viper.New()
	v.SetConfigType("yaml")

	data, err := ioutil.ReadFile(pfile)

	if err != nil {
		panic(err)
	}

	v.ReadConfig(bytes.NewBuffer(data))

	// turns out Viper doesn't offer an easy way to build our Plan
	// Bindings from the YAML file we produced, so do it the rather
	// manual way.
	//
	// The viper.GetString() API returns a flat list of keys which
	// encode the nesting levels of the keys thanks to a dot notation.
	// We reverse engineer that into a map, simplifying the operation
	// thanks to knowing we are dealing with a single level of nesting
	// here: that's dot[0] for a Bindings entry then dot[1] for the key
	// names within that Plan Bindings entry.
	var bindings []map[string]string
	var names []string
	var current_map map[string]string
	var current_name string

	for _, key := range v.AllKeys() {
		dots := strings.Split(key, ".") // we expect a single level
		value := v.GetString(key)

		if current_name == "" || current_name != dots[0] {
			if current_name != "" {
				bindings = append(bindings, current_map)
			}
			current_name = dots[0]
			names = append(names, current_name)
			current_map = make(map[string]string)
		}
		current_map[dots[1]] = value
	}
	// don't forget to finish the current map when out of the loop
	bindings = append(bindings, current_map)

	return &Plan{q, pfile, names, bindings, []ResultSet{}}
}

// Executes a plan and returns the filepath where the output has been
// written, for later comparing
func (p *Plan) Execute(db *sql.DB) {
	if len(p.Query.Params) == 0 {
		// this Query has no plans, so don't loop over the bindings
		args := make([]interface{}, 0)
		res, err := QueryDB(db, p.Query.Query, args...)

		if err != nil {
			fmt.Printf("Error executing\n%s", p.Query.Query)
			panic(err)
		}
		result := make([]ResultSet, 1)
		result[0] = *res
		p.ResultSets = result
		return
	}

	// general case, with a plan and a set of Bindings to go through
	result := make([]ResultSet, len(p.Bindings))

	for i, bindings := range p.Bindings {
		sql, args := p.Query.Prepare(bindings)
		res, err := QueryDB(db, sql, args...)

		if err != nil {
			fmt.Printf("Error executing\n%s\nwith params: %v\n",
				sql, args)
			panic(err)
		}
		result[i] = *res
	}
	p.ResultSets = result
}

func (p *Plan) WriteResultSets(dir string) {
	for i, rs := range p.ResultSets {
		rsFileName := getResultSetPath(p, dir, i)
		err := rs.Write(rsFileName, true)

		if err != nil {
			panic(err)
		}
		p.ResultSets[i].Filename = rsFileName
	}
}

// Write a plan to disk in YAML format, thanks to Viper.
func (p *Plan) Write() {
	if len(p.Bindings) == 0 {
		fmt.Printf("Skipping Plan '%s': query uses no variable\n", p.Path)
		return
	}

	fmt.Printf("Creating Empty Plan '%s'\n", p.Path)
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

func getResultSetPath(p *Plan, targetdir string, index int) string {
	var rsFileName string
	basename := strings.TrimSuffix(filepath.Base(p.Path), path.Ext(p.Path))

	if len(p.Query.Params) == 0 {
		rsFileName = fmt.Sprintf("%s.out", basename)
	} else {
		rsFileName = fmt.Sprintf("%s.%s.out", basename, p.Names[index])
	}
	return filepath.Join(targetdir, rsFileName)
}
