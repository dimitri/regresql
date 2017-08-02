package regresql

import (
	"fmt"
	"database/sql"

	// "github.com/mndrix/tap-go"
	_ "github.com/lib/pq"
)

func Init(dir string, pguri string) {
	fmt.Println("Init: init -C %s", dir)

	fmt.Printf("Trying to connect to %s\n", pguri)
	_, err := sql.Open("postgres", pguri)

	if err != nil {
		fmt.Println(err)
		return
	} else {
		fmt.Println("Connected")
	}
}

func Update(dir string) {
	fmt.Println("Update: update -C %s", dir)
}

func Test(dir string) {
	fmt.Println("Test: test -C %s", dir)
}

func List(dir string) {
	fmt.Println("List: list -C %s", dir)

	folders := Walk(dir)

	for _, dir := range folders {
		fmt.Printf("%s\n", dir.Dir)
		for _, name := range dir.Files {
			fmt.Printf("  %s\n", name)
		}
	}

	return
}
