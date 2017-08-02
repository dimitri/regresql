package cmd

import (
	"os"
	"fmt"
)

// Check that given path string is a directory that exists
func checkDirectory(cwd string) {
	stat, err := os.Stat(cwd)
	if err != nil {
		panic(err)
	}
	if !stat.IsDir() {
		panic(fmt.Sprintf("%s is not a directory!", cwd))
	}
	return
}
