package regresql

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mndrix/tap-go"
)

func (p *Plan) CompareResultSets(regressDir string, expectedDir string, t *tap.T) {
	for i, rs := range p.ResultSets {
		testName := strings.TrimPrefix(rs.Filename, regressDir+"/")
		expectedFilename := filepath.Join(expectedDir,
			filepath.Base(rs.Filename))
		diff := DiffFiles(expectedFilename, rs.Filename, 3)

		if diff != "" {
			t.Diagnostic(
				fmt.Sprintf(`Query File: '%s'
Bindings File: '%s'
Bindings Name: '%s'
Query Parameters: '%v'
Expected Result File: '%s'
Actual Result File: '%s'

%s`, p.Query.Path, p.Path, p.Names[i], p.Bindings[i], expectedFilename, rs.Filename, diff))
		}
		t.Ok(diff == "", testName)
	}
}
