package kenv

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/VKCOM/ktest/internal/fileutil"
)

var kphpRoot string

func initKphpRoot() {
	if envRoot := os.Getenv("KPHP_ROOT"); envRoot != "" {
		if fileutil.FileExists(envRoot) {
			kphpRoot = envRoot
			return
		}
	}

	home, err := os.UserHomeDir()
	homeKphp := filepath.Join(home, "kphp")
	if err == nil && fileutil.FileExists(homeKphp) {
		kphpRoot = homeKphp
		return
	}
}

func init() {
	initKphpRoot()
}

func FindKphpBinary() string {
	kphp, err := exec.LookPath("kphp2cpp")
	if err == nil && kphp != "" {
		return kphp
	}
	if kphpRoot != "" {
		rootBinary := filepath.Join(kphpRoot, "objs", "bin", "kphp2cpp")
		if fileutil.FileExists(rootBinary) {
			return rootBinary
		}
	}
	return ""
}
