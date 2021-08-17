package phpunit

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/VKCOM/ktest/internal/fileutil"
	"github.com/z7zmey/php-parser/pkg/conf"
	"github.com/z7zmey/php-parser/pkg/errors"
	"github.com/z7zmey/php-parser/pkg/parser"
	"github.com/z7zmey/php-parser/pkg/version"
	"github.com/z7zmey/php-parser/pkg/visitor/traverser"
)

type runner struct {
	conf *RunConfig

	result RunResult

	testDir   string
	testFiles []*testFile

	buildDir      string
	buildDirTests string
	buildDirMains string
}

type testFile struct {
	id int

	fullName  string
	shortName string

	mainFilename string

	info *testParsedInfo

	contents             []byte
	preprocessedContents []byte
	generatedMain        []byte
}

type testParsedInfo struct {
	ClassName   string
	TestMethods []string

	fixes []textEdit
}

func newRunner(conf *RunConfig) *runner {
	return &runner{conf: conf}
}

func (r *runner) Run() (*RunResult, error) {
	defer func() {
		if r.buildDir == "" || r.conf.NoCleanup {
			return
		}
		if err := os.RemoveAll(r.buildDir); err != nil {
			log.Printf("remove temp build dir: %v", err)
		}
	}()

	steps := []struct {
		name string
		fn   func() error
	}{
		{"find test files", r.stepFindTestFiles},
		{"prepare temp build dir", r.stepPrepareTempBuildDir},
		{"parse test files", r.stepParseTestFiles},
		{"filter only parsed files", r.stepFilterOnlyParsedFiles},
		{"sort test files", r.stepSortTestFiles},
		{"preprocess contents", r.stepPreprocessContents},
		{"generate test main", r.stepGenerateTestMain},
		{"write preprocessed test files", r.stepWritePreprocessedTestFiles},
		{"write test main", r.stepWriteTestMain},
		{"run kphp tests", r.stepRunKphpTests},
	}

	for _, step := range steps {
		if err := step.fn(); err != nil {
			return nil, fmt.Errorf("%s: %w", step.name, err)
		}
	}

	return &r.result, nil
}

func (r *runner) debugf(format string, args ...interface{}) {
	if r.conf.DebugPrint != nil {
		r.conf.DebugPrint(fmt.Sprintf(format, args...))
	}
}

func (r *runner) stepFindTestFiles() error {
	var testDir string
	var testFiles []string
	if strings.HasSuffix(r.conf.TestTarget, ".php") {
		testFiles = []string{r.conf.TestTarget}
		testDir = filepath.Dir(r.conf.TestTarget)
	} else {
		var err error
		testFiles, err = findTestFiles(r.conf.TestTarget)
		if err != nil {
			return err
		}
		testDir = r.conf.TestTarget
	}
	if !strings.HasSuffix(testDir, "/") {
		testDir += "/"
	}

	r.testDir = testDir

	r.testFiles = make([]*testFile, len(testFiles))
	for i, f := range testFiles {
		r.testFiles[i] = &testFile{
			fullName:  f,
			shortName: strings.TrimPrefix(f, testDir),
		}
	}

	if r.conf.DebugPrint != nil {
		r.debugf("test dir: %q", r.testDir)
		for _, f := range r.testFiles {
			r.debugf("test file: %q", f.fullName)
		}
	}

	return nil
}

func (r *runner) stepPrepareTempBuildDir() error {
	tempDir, err := ioutil.TempDir("", "kphpunit-build")
	if err != nil {
		return err
	}
	r.buildDir = tempDir
	r.debugf("temp build dir: %q", tempDir)

	links := []string{
		r.conf.SrcDir,
		"vendor",
		"composer.json",
	}

	for _, l := range links {
		if err := os.Symlink(filepath.Join(r.conf.ProjectRoot, l), filepath.Join(tempDir, l)); err != nil {
			return err
		}
	}

	testDirRel := strings.TrimPrefix(r.testDir, r.conf.ProjectRoot)
	r.buildDirTests = filepath.Join(tempDir, testDirRel)
	if err := fileutil.MkdirAll(r.buildDirTests); err != nil {
		return err
	}

	r.buildDirMains = filepath.Join(tempDir, "mains")
	if err := fileutil.MkdirAll(r.buildDirMains); err != nil {
		return err
	}

	return nil
}

func (r *runner) stepParseTestFiles() error {
	for _, f := range r.testFiles {
		src, err := ioutil.ReadFile(f.fullName)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		f.contents = src
		var parserErrors []*errors.Error
		errorHandler := func(e *errors.Error) {
			parserErrors = append(parserErrors, e)
		}
		rootNode, err := parser.Parse(src, conf.Config{
			Version:          &version.Version{Major: 7, Minor: 4},
			ErrorHandlerFunc: errorHandler,
		})
		if len(parserErrors) != 0 {
			for _, parseErr := range parserErrors {
				log.Printf("%s: parse error: %v", f.fullName, parseErr)
			}
			continue
		}
		f.info = &testParsedInfo{}
		visitor := &astVisitor{out: f.info}
		traverser.NewTraverser(visitor).Traverse(rootNode)
	}

	return nil
}

func (r *runner) stepFilterOnlyParsedFiles() error {
	parsedFiles := make([]*testFile, 0, len(r.testFiles))
	for _, f := range r.testFiles {
		if f.info != nil {
			parsedFiles = append(parsedFiles, f)
		}
	}
	r.testFiles = parsedFiles

	return nil
}

func (r *runner) stepSortTestFiles() error {
	sort.Slice(r.testFiles, func(i, j int) bool {
		return r.testFiles[i].fullName < r.testFiles[j].fullName
	})

	for i, f := range r.testFiles {
		f.id = i
	}

	return nil
}

func (r *runner) stepPreprocessContents() error {
	for _, f := range r.testFiles {
		f.preprocessedContents = applyTextEdits(f.contents, f.info.fixes)
	}

	return nil
}

func (r *runner) stepGenerateTestMain() error {
	for _, f := range r.testFiles {
		var generated bytes.Buffer
		templateData := map[string]interface{}{
			"TestFilename":  filepath.Join(r.buildDirTests, f.shortName),
			"TestClassName": f.info.ClassName,
			"TestMethods":   f.info.TestMethods,
		}
		if err := testMainTemplate.Execute(&generated, templateData); err != nil {
			return fmt.Errorf("%s: %w", f.fullName, err)
		}
		f.generatedMain = generated.Bytes()
	}

	return nil
}

var testMainTemplate = template.Must(template.New("test_main").Parse(`<?php

require_once '{{.TestFilename}}';

use KPHPUnit\Framework\TestCase;
use KPHPUnit\Framework\AssertionFailedException;

function __kphpunit_main() {
  $test = new {{.TestClassName}}();
  {{range .TestMethods}}
  try {
    echo '["START","{{.}}"]' . "\n";
    $test->{{.}}();
    fprintf(STDERR, '.');
  } catch (AssertionFailedException $e) {
    fprintf(STDERR, 'F');
  }
  {{- end}}
  echo '["FINISHED"]' . "\n";
}

__kphpunit_main();
`))

func (r *runner) stepWritePreprocessedTestFiles() error {
	for _, f := range r.testFiles {
		filename := filepath.Join(r.buildDirTests, f.shortName)
		if err := fileutil.WriteFile(filename, f.preprocessedContents); err != nil {
			return err
		}
	}

	return nil
}

func (r *runner) stepWriteTestMain() error {
	for _, f := range r.testFiles {
		f.mainFilename = filepath.Join(r.buildDirMains, fmt.Sprintf("%d.php", f.id))
		if err := fileutil.WriteFile(f.mainFilename, f.generatedMain); err != nil {
			return err
		}
	}

	return nil
}

func (r *runner) stepRunKphpTests() error {
	composerMode := fileutil.FileExists(filepath.Join(r.conf.ProjectRoot, "composer.json"))

	testsTotal := 0
	for _, f := range r.testFiles {
		testsTotal += len(f.info.TestMethods)
	}

	testsCompleted := 0
	for _, f := range r.testFiles {
		testsCompleted += len(f.info.TestMethods)

		// 1. Build.
		args := []string{
			"--mode", "cli",
			"--destination-directory", r.buildDir,
		}
		if composerMode {
			args = append(args, "--composer-root", r.conf.ProjectRoot)
		}
		args = append(args, f.mainFilename)
		buildCommand := exec.Command(r.conf.KphpCommand, args...)
		buildCommand.Dir = r.buildDir
		out, err := buildCommand.CombinedOutput()
		if err != nil {
			log.Printf("%s: build error: %v: %s", f.fullName, err, out)
			continue
		}

		// 2. Run.
		executableName := filepath.Join(r.buildDir, "cli")
		runCommand := exec.Command(executableName)
		runCommand.Dir = r.buildDir
		var runStdout bytes.Buffer
		runCommand.Stderr = r.conf.Output
		runCommand.Stdout = &runStdout
		if err := runCommand.Run(); err != nil {
			log.Printf("%s: run error: %v", f.fullName, err)
			continue
		}

		// 3. Parse output.
		parsed, err := parseTestOutput(f, runStdout.Bytes())
		if err != nil {
			log.Printf("%s: parse test output: %v", f.fullName, err)
			continue
		}

		status := "OK"
		if len(parsed.failures) != 0 {
			status = "FAIL"
		}
		completed := float64(testsCompleted) / float64(testsTotal) * 100.0
		fmt.Fprintf(r.conf.Output, " %d / %d (%2d%%) %s\n", testsCompleted, testsTotal, int(completed), status)

		r.result.Failures = append(r.result.Failures, parsed.failures...)
		r.result.Assertions += parsed.asserts
	}
	r.result.Tests = testsCompleted

	return nil
}
