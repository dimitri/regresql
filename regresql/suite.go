package regresql

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/lib/pq"
	"github.com/mndrix/tap-go"
)

type Suite struct {
	Root       string
	RegressDir string
	Dirs       []Folder
}

type Folder struct {
	Dir   string
	Files []string
}

func newSuite(root string) *Suite {
	regressdir := filepath.Join(root, "regresql")
	return &Suite{root, regressdir, []Folder{}}
}

func newFolder(path string) *Folder {
	return &Folder{path, []string{}}
}

func (s *Suite) appendPath(path string) *Suite {
	dir, _ := filepath.Rel(s.Root, filepath.Dir(path))
	var name string = filepath.Base(path)

	// search dir in folders
	for i := range s.Dirs {
		if s.Dirs[i].Dir == dir {
			// dir is already known, append file here
			s.Dirs[i].Files = append(s.Dirs[i].Files, name)
			return s
		}
	}

	// we didn't find the path folder, append a new entry and return it
	f := newFolder(dir)
	f.Files = append(f.Files, name)
	s.Dirs = append(s.Dirs, *f)
	return s
}

func Walk(root string) *Suite {
	suite := newSuite(root)

	visit := func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == ".sql" {
			suite = suite.appendPath(path)
		}
		return nil
	}
	filepath.Walk(root, visit)

	return suite
}

func (s *Suite) Println() {
	fmt.Printf("%s\n", s.Root)
	for _, folder := range s.Dirs {
		fmt.Printf("  %s/\n", folder.Dir)
		for _, name := range folder.Files {
			fmt.Printf("    %s\n", name)
		}
	}
}

func (s *Suite) initRegressHierarchy() {
	for _, folder := range s.Dirs {
		rdir := filepath.Join(s.RegressDir, "plans", folder.Dir)
		maybeMkdirAll(rdir)

		for _, name := range folder.Files {
			qfile := filepath.Join(s.Root, folder.Dir, name)

			q := parseQueryFile(qfile)
			q.CreateEmptyPlan(rdir)
		}
	}
}

func (s *Suite) createExpectedResults(pguri string) {
	fmt.Printf("Connecting to '%s'\n", pguri)
	db, err := sql.Open("postgres", pguri)

	if err != nil {
		panic(err)
	}
	defer db.Close()

	fmt.Println("Wrote expected Result Sets:")

	for _, folder := range s.Dirs {
		rdir := filepath.Join(s.RegressDir, "plans", folder.Dir)
		edir := filepath.Join(s.RegressDir, "expected", folder.Dir)
		maybeMkdirAll(edir)

		fmt.Printf("  %s\n", edir)

		for _, name := range folder.Files {
			qfile := filepath.Join(s.Root, folder.Dir, name)

			q := parseQueryFile(qfile)
			p := q.GetPlan(rdir)
			p.Execute(db)
			p.WriteResultSets(edir)

			for _, rs := range p.ResultSets {
				fmt.Printf("    %s\n", filepath.Base(rs.Filename))
			}
		}
	}
}

func (s *Suite) testQueries(pguri string) {
	fmt.Printf("Connecting to '%s'\n", pguri)
	db, err := sql.Open("postgres", pguri)

	if err != nil {
		panic(err)
	}
	defer db.Close()

	t := tap.New()
	t.Header(2)

	for _, folder := range s.Dirs {
		rdir := filepath.Join(s.RegressDir, "plans", folder.Dir)
		edir := filepath.Join(s.RegressDir, "expected", folder.Dir)
		odir := filepath.Join(s.RegressDir, "out", folder.Dir)
		maybeMkdirAll(odir)

		for _, name := range folder.Files {
			qfile := filepath.Join(s.Root, folder.Dir, name)

			q := parseQueryFile(qfile)
			p := q.GetPlan(rdir)
			p.Execute(db)
			p.WriteResultSets(odir)
			p.CompareResultSets(s.RegressDir, edir, t)
		}
	}
}

// Only create dir(s) when it doesn't exists already
func maybeMkdirAll(dir string) {
	stat, err := os.Stat(dir)
	if err != nil || !stat.IsDir() {
		fmt.Printf("Creating directory '%s'\n", dir)

		err := os.MkdirAll(dir, 0755)

		if err != nil {
			panic(err)
		}
	}
}
