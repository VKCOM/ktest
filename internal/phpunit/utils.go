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

func findTestFiles(root string) ([]string, error) {
	var out []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), "Test.php") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
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
