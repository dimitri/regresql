package regresql

import (
	"fmt"
	"os"
	"path/filepath"
)

type Suite struct {
	Root string
	Dirs []Folder
}

type Folder struct {
	Dir   string
	Files []string
}

func newSuite(root string) *Suite {
	return &Suite{root, []Folder{}}
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

func (s *Suite) Println() {
	fmt.Printf("%s\n", s.Root)
	for _, dir := range s.Dirs {
		fmt.Printf("  %s/\n", dir.Dir)
		for _, name := range dir.Files {
			fmt.Printf("    %s\n", name)
		}
	}	
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
