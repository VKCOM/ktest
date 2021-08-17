package fileutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

func IsUnixCharDevice(f *os.File) bool {
	fileInfo, err := f.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func WriteFile(path string, contents []byte) error {
	dir := filepath.Dir(path)
	if err := MkdirAll(dir); err != nil {
		return err
	}
	return ioutil.WriteFile(path, contents, 0666)
}

func MkdirAll(path string) error {
	if FileExists(path) {
		return nil
	}
	return os.MkdirAll(path, 0755)
}
