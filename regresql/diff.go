package regresql

import (
	"fmt"
	"github.com/pmezard/go-difflib/difflib"
	"io/ioutil"
)

func readLines(filename string) ([]string, error) {
	data, err := ioutil.ReadFile(filename)

	if err != nil {
		var data []string
		return data, fmt.Errorf("Failed to read lines from '%s': %s\n",
			filename, err)
	}

	lines := difflib.SplitLines(string(data))
	return lines, nil
}

func DiffFiles(a string, b string, c int) (string, error) {
	var a_lines, b_lines []string
	var r string
	var err error

	if a_lines, err = readLines(a); err != nil {
		return r, err
	}

	if b_lines, err = readLines(b); err != nil {
		return r, err
	}

	return DiffLines(a, b, a_lines, b_lines, c), nil
}

func DiffLines(from string, to string, a []string, b []string, c int) string {
	diff := difflib.UnifiedDiff{
		A:        a,
		B:        b,
		FromFile: from,
		ToFile:   to,
		Context:  c,
	}
	text, _ := difflib.GetUnifiedDiffString(diff)
	return text
}
