package phpunit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/z7zmey/php-parser/pkg/ast"
)

func astNameToString(name *ast.Name) string {
	var parts []string
	for _, p := range name.Parts {
		parts = append(parts, string(p.(*ast.NamePart).Value))
	}
	return strings.Join(parts, `\`)
}

type testFiles struct {
	scripts  []string
	testdata []string
}

func findTestFiles(root string) (testFiles, error) {
	var out testFiles

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "testdata" {
				out.testdata = append(out.testdata, path)
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), "Test.php") {
			out.scripts = append(out.scripts, path)
		}
		return nil
	})
	if err != nil {
		return out, err
	}

	return out, nil
}

func jsonString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(b)
}
