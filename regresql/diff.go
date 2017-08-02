package regresql

import (
	"io/ioutil"
	"github.com/pmezard/go-difflib/difflib"
)

func readLines(filename string) []string {
	data, err := ioutil.ReadFile(filename)

	if err != nil {
		panic(err)
	}

	lines := difflib.SplitLines(string(data))
	return lines
}

func Diff(a string, b string, c int) string {
	a_lines := readLines(a)
	b_lines := readLines(b)

	diff := difflib.UnifiedDiff{
		A:        a_lines,
		B:        b_lines,
		FromFile: a,
		ToFile:   b,
		Context:  c,
	}
	text, _ := difflib.GetUnifiedDiffString(diff)
	return text
}

