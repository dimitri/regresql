package regresql

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mndrix/tap-go"
)

/*
CompareResultsSets load the expected result set and compares it with the
given Plan's ResultSet, and fills in a tap.T test output.

The test is considered passed when the diff is empty.

Rather than returning an error in case something wrong happens, we register
a diagnostic against the tap output and fail the test case.
*/
func (p *Plan) CompareResultSets(regressDir string, expectedDir string, t *tap.T) {
	for i, rs := range p.ResultSets {
		testName := strings.TrimPrefix(rs.Filename, regressDir+"/out/")
		expectedFilename := filepath.Join(expectedDir,
			filepath.Base(rs.Filename))
		diff, err := DiffFiles(expectedFilename, rs.Filename, 3)

		if err != nil {
			t.Diagnostic(
				fmt.Sprintf(`Query File: '%s'
Bindings File: '%s'
Bindings Name: '%s'
Query Parameters: '%v'
Expected Result File: '%s'
Actual Result File: '%s'

Failed to compare results: %s`,
					p.Query.Path,
					p.Path,
					p.Names[i],
					p.Bindings[i],
					expectedFilename,
					rs.Filename,
					err.Error()))
		}

		if diff != "" {
			t.Diagnostic(
				fmt.Sprintf(`Query File: '%s'
Bindings File: '%s'
Bindings Name: '%s'
Query Parameters: '%v'
Expected Result File: '%s'
Actual Result File: '%s'

%s`,
					p.Query.Path,
					p.Path,
					p.Names[i],
					p.Bindings[i],
					expectedFilename,
					rs.Filename,
					diff))
		}
		t.Ok(diff == "", testName)
	}
}
