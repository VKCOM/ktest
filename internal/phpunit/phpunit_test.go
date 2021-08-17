package phpunit

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VKCOM/ktest/internal/fileutil"
	"github.com/VKCOM/ktest/internal/kenv"
)

func TestPhpunit(t *testing.T) {
	testFiles, err := ioutil.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}

	absFilepath := func(t *testing.T, filename string) string {
		abs, err := filepath.Abs(filename)
		if err != nil {
			t.Fatal(err)
		}
		return abs
	}

	initComposer := func(t *testing.T, workdir string) {
		if fileutil.FileExists(filepath.Join(workdir, "vendor")) {
			return
		}

		composerInstallCommand := exec.Command("composer", "install")
		composerInstallCommand.Dir = workdir
		t.Log(composerInstallCommand.String())
		out, err := composerInstallCommand.CombinedOutput()
		if err != nil {
			t.Fatalf("run %s: %s: %v", composerInstallCommand, out, err)
		}
	}

	runTest := func(t *testing.T, filename string) {
		testDir := filepath.Join("testdata", filename)
		goldenData, err := ioutil.ReadFile(filepath.Join(testDir, "golden.txt"))
		if err != nil {
			t.Fatalf("read golden file: %v", err)
		}

		workdir := absFilepath(t, testDir)
		initComposer(t, workdir)

		var output bytes.Buffer
		result, err := Run(&RunConfig{
			ProjectRoot: workdir,
			SrcDir:      "src",
			TestTarget:  filepath.Join(workdir, "tests"),
			KphpCommand: kenv.FindKphpBinary(),
			Output:      &output,
		})
		if err != nil {
			t.Fatal(err)
		}
		formatConfig := &FormatConfig{
			PrintTime:     false,
			ShortLocation: true,
		}
		FormatResult(&output, formatConfig, result)
		have := strings.TrimSpace(output.String())
		want := strings.TrimSpace(string(goldenData))
		if have != want {
			t.Errorf("output mismatches!")
			fmt.Printf("have:\n%s\n", have)
			fmt.Printf("want:\n%s\n", want)
		}
	}

	for i := range testFiles {
		f := testFiles[i]
		t.Run(f.Name(), func(t *testing.T) {
			runTest(t, f.Name())
		})
	}
}
