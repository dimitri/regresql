package regresql

import (
	"fmt"
	// "github.com/mndrix/tap-go"
	// "github.com/lib/pq"
)

func Init(dir string) {
	fmt.Println("Init: init -C %s", dir)
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
