package regresql

import (
	"path/filepath"
	"os"
	"fmt"
)

type Folder struct {
	Dir     string
	Files []string
}

func newFolder(path string) *Folder {
	return &Folder{path, []string{}}
}

func appendPath(folders []Folder, path string) []Folder {
	var dir  string = filepath.Dir(path)
	var name string = filepath.Base(path)

	// search dir in folders
	for i := range folders {
		if folders[i].Dir == dir {
			// dir is already known, append file here
			folders[i].Files = append(folders[i].Files, name)
			return folders
		}
	}
	
	// we didn't find the path folder, append a new entry and return it
	f := newFolder(dir)
	f.Files = append(f.Files, name)
	n := append(folders, *f)
	return n
}

func Walk(dir string) []Folder {
	var folders []Folder;
	
	visit := func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == ".sql" {
			folders = appendPath(folders, path)
		}
		return nil
	}
	filepath.Walk(dir, visit)

	if false {
		fmt.Printf("Accumulated files:\n %#v\n", folders)
	}
	return folders
}
