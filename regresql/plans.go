package regresql

import (
	"fmt"
	// "os"
	"io/ioutil"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/theherk/viper" // fork with write support
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
	r, _ := regexp.Compile(`:['"]?([A-Za-z][A-Za-z0-9]*)['"]?`)
	uses := r.FindAllStringSubmatch(sql, -1)

	for _, v := range uses {
		mapv[v[1]] = true
	}

	// now we're only interested into the mapv keys: variable names.
	var vars []string
	for k := range mapv {
		vars = append(vars, k)
	}

	return &Query{queryPath, sql, vars}
}

func (q *Query) CreateEmptyPlan(dir string) *Plan {
	var bindings []map[string]string
	pfile := getPlanPath(q, dir)

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
