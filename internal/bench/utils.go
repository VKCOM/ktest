package bench

import (
	"os"
	"path/filepath"
	"strings"
)

func findBenchFiles(root string) ([]string, error) {
	var out []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), "Benchmark") && strings.HasSuffix(info.Name(), ".php") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}
