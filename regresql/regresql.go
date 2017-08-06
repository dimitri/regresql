package regresql

import (
	"fmt"
)

func Init(root string, pguri string) {
	err := TestConnectionString(pguri)
	if err != nil {
		panic(err)
	}

	suite := Walk(root)

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

func Update(root string) {
	suite := Walk(root)
	config := suite.readConfig()

	suite.createExpectedResults(config.PgUri)

	fmt.Println("")
	fmt.Println(`Expected files have now been created.
You can run regression tests for your SQL queries with the command

  regresql test [ -C directory ]

When you add new queries to your code repository, run regresql init to
create the missing test plans, edit them to add test parameters, and then
run regresql update to have exepected data files to test against.

If you change the expected result set (because picking a new data set of
because new requirements impacts the result of existing queries, you can run
the regresql update command again to reset the expected output files.
 `)
}

func Test(root string) {
	suite := Walk(root)
	config := suite.readConfig()

	suite.testQueries(config.PgUri)
}

func List(dir string) {
	suite := Walk(dir)
	suite.Println()

	return
}
