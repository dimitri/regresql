package regresql

import (
	"github.com/pmezard/go-difflib/difflib"
	"io/ioutil"
)

func readLines(filename string) []string {
	data, err := ioutil.ReadFile(filename)

	if err != nil {
		panic(err)
	}

	lines := difflib.SplitLines(string(data))
	return lines
}

func DiffFiles(a string, b string, c int) string {
	a_lines := readLines(a)
	b_lines := readLines(b)

	return DiffLines(a, b, a_lines, b_lines, c)
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
