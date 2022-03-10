package testdir

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/VKCOM/ktest/internal/fileutil"
)

type Builder struct {
	ProjectRoot string
	LinkFiles   []string
	MakeDirs    []string
}

func (b *Builder) Build() (string, error) {
	tempDir, err := ioutil.TempDir("", "ktest-build")
	if err != nil {
		return "", err
	}

	links := []string{
		"vendor",
		"composer.json",
		"ffilibs",
	}
	links = append(links, b.LinkFiles...)

	for _, l := range links {
		filename := filepath.Join(b.ProjectRoot, l)
		if !fileutil.FileExists(filename) {
			continue
		}
		if err := os.Symlink(filepath.Join(b.ProjectRoot, l), filepath.Join(tempDir, l)); err != nil {
			return tempDir, err
		}
	}

	for _, d := range b.MakeDirs {
		if err := fileutil.MkdirAll(filepath.Join(tempDir, d)); err != nil {
			return tempDir, err
		}
	}

	return tempDir, nil
}
