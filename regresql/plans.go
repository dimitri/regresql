package regresql

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	_ "github.com/lib/pq"
	"github.com/theherk/viper" // fork with write support
)

const (
	psqlVarRE = `:['"]?([A-Za-z][A-Za-z0-9]*)['"]?`
)

type Query struct {
	Path  string
	Query string
	Vars  []string
}

type Plan struct {
	Query    *Query
	Path     string // the file path where we read the Plan from
	Bindings []map[string]string
}

// Parse a SQL file and returns a Query instance, with variables used in the
// query separated in the Query.Vars map.
func parseQueryFile(queryPath string) *Query {
	sqlbytes, err := ioutil.ReadFile(queryPath)
	if err != nil {
		panic(err)
	}
	sql := string(sqlbytes)

	// find a uses of variables in the SQL query text, and put then in a
	// map so that we get each of them only once, even when used several
	// times in the same query
	mapv := make(map[string]bool)
	r, _ := regexp.Compile(psqlVarRE)
	uses := r.FindAllStringSubmatch(sql, -1)

	for _, match := range uses {
		mapv[match[1]] = true
	}

	// now we're only interested into the mapv keys: variable names.
	var vars []string
	for k := range mapv {
		vars = append(vars, k)
	}

	return &Query{queryPath, sql, vars}
}

// Prepare a Query from its raw text by:
//   - transforming :'vars' into ? in the query text
//   - preparing an args... interface{} from given bindings
func (q *Query) Prepare(bindings map[string]string) (string, []interface{}) {
	sql := string(q.Query)

	params := make([]interface{}, 0)
	r, _ := regexp.Compile(psqlVarRE)
	uses := r.FindAllStringSubmatch(q.Query, -1)

	// deduplicate param names in the query, and assign each of them a
	// different ordinal marker ($1, $2, etc) for lib/pq
	mapv := make(map[string]int)

	for i, match := range uses {
		mapv[match[1]] = i + 1
	}

	// FIXME
	//
	// doesn't work in the general case, using :a :b :b which is then
	// translated to $1 $2 $2, and params must be [a b b]
	for name, ord := range mapv {
		r, _ := regexp.Compile(fmt.Sprintf(`:["']?%s["']?`, name))
		sql = r.ReplaceAllLiteralString(sql, fmt.Sprintf("$%d", ord))
		params = append(params, bindings[name])
	}
	return sql, params
}

func (q *Query) CreateEmptyPlan(dir string) *Plan {
	var bindings []map[string]string
	pfile := getPlanPath(q, dir)

	if _, err := os.Stat(pfile); !os.IsNotExist(err) {
		panic(fmt.Sprintf("Fatal: plan file '%s' already exists", pfile))
	}

	if len(q.Vars) > 0 {
		bindings = make([]map[string]string, 1)
		bindings[0] = make(map[string]string)
		for _, varname := range q.Vars {
			bindings[0][varname] = ""
		}
	} else {
		bindings = []map[string]string{}
	}

	plan := &Plan{q, pfile, bindings}
	plan.Write()

	return plan
}

func (q *Query) GetPlan(planDir string) *Plan {
	pfile := getPlanPath(q, planDir)

	if _, err := os.Stat(pfile); os.IsNotExist(err) {
		return &Plan{q, pfile, []map[string]string{}}
	}

	fmt.Printf("Reading bindings from '%s'\n", pfile)

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
	current_map := make(map[string]string)
	i := ""

	for _, key := range v.AllKeys() {
		dots := strings.Split(key, ".") // we expect a single level
		value := v.GetString(key)

		if i != "" && i != dots[0] {
			bindings = append(bindings, current_map)
			i = dots[0]
			current_map = make(map[string]string)
		}
		current_map[dots[1]] = value
	}
	bindings = append(bindings, current_map)

	return &Plan{q, pfile, bindings}
}

// Executes a plan and returns the filepath where the output has been
// written, for later comparing
func (p *Plan) Execute(db *sql.DB, dir string) [][]map[string]interface{} {
	result := [][]map[string]interface{}{}
	for _, bindings := range p.Bindings {
		sql, args := p.Query.Prepare(bindings)
		res, err := QueryDB(db, sql, args...)

		if err != nil {
			fmt.Printf("Error executing\n%s\nwith params: %v",
				sql, args)
			panic(err)
		}

		fmt.Printf("r: %v\n", result)
		result = append(result, res)
	}
	return result
}

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
			// be friendly to the user and count plans from 1
			vpath := fmt.Sprintf("%d.%s", i+1, key)
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
