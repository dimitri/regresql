package regresql

import (
	"fmt"
	"path/filepath"
	// "github.com/mndrix/tap-go"
)

func Init(root string, pguri string) {
	err := TestConnectionString(pguri)
	if err != nil {
		panic(err)
	}

	suite := Walk(root)
	suite.RegressDir = filepath.Join(suite.Root, "regresql")

	suite.createRegressDir()
	suite.setupConfig(pguri)
	suite.initRegressHierarchy()

	fmt.Println("")
	fmt.Println(`Empty test plans have been created.
Edit the plans to add query binding values, then run 

  regresql update [ -C directory ]

to create the expected regression files for your test plans. Plans are
simple YAML files containing multiple set of query parameter bindings. The
default plan files contain a single entry named "1", you can rename the test
case and add a value for each parameter. `)
}

func Update(dir string) {
	fmt.Println("Update: update -C %s", dir)
}

func Test(dir string) {
	fmt.Println("Test: test -C %s", dir)
}

func List(dir string) {
	fmt.Println("List: list -C %s", dir)

	suite := Walk(dir)
	suite.Println()

	return
}
