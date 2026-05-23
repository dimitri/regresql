package regresql

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mndrix/tap-go"
)

/*
CompareResultsSets load the expected result set and compares it with the
given Plan's ResultSet, and fills in a tap.T test output.

The test is considered passed when the diff is empty.

When pgMajor > 0, a version-specific expected file (e.g. query.pg16.out) is
checked first; the generic file (query.out) is used as fallback.

Rather than returning an error in case something wrong happens, we register
a diagnostic against the tap output and fail the test case.
*/
func (p *Plan) CompareResultSets(regressDir string, expectedDir string, t *tap.T, pgMajor int) {
	for i, rs := range p.ResultSets {
		testName := strings.TrimPrefix(rs.Filename, regressDir+"/out/")
		base := filepath.Base(rs.Filename)
		expectedFilename := filepath.Join(expectedDir, base)

		if pgMajor > 0 {
			ext := filepath.Ext(base)
			stem := strings.TrimSuffix(base, ext)
			versioned := filepath.Join(expectedDir,
				fmt.Sprintf("%s.pg%d%s", stem, pgMajor, ext))
			if _, err := os.Stat(versioned); err == nil {
				expectedFilename = versioned
			}
		}

		diff, err := DiffFiles(expectedFilename, rs.Filename, 3)

		// p.Names and p.Bindings are empty for parameterless queries; guard
		// against an out-of-range panic before entering the diagnostic branches.
		bindingName := ""
		if i < len(p.Names) {
			bindingName = p.Names[i]
		}
		var bindingParams interface{} = map[string]string{}
		if i < len(p.Bindings) {
			bindingParams = p.Bindings[i]
		}

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
					bindingName,
					bindingParams,
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
					bindingName,
					bindingParams,
					expectedFilename,
					rs.Filename,
					diff))
		}
		t.Ok(diff == "", testName)
	}
}
