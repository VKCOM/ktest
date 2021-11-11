package bench

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/z7zmey/php-parser/pkg/conf"
	"github.com/z7zmey/php-parser/pkg/errors"
	"github.com/z7zmey/php-parser/pkg/parser"
	"github.com/z7zmey/php-parser/pkg/version"
	"github.com/z7zmey/php-parser/pkg/visitor/traverser"

	"github.com/VKCOM/ktest/internal/fileutil"
	"github.com/VKCOM/ktest/internal/teamcity"
)

type runner struct {
	conf *RunConfig

	benchFiles []*benchFile

	logger       *teamcity.Logger
	composerMode bool

	buildDir string
}

type benchFile struct {
	id int

	fullName  string
	shortName string

	info *benchParsedInfo

	generatedMain []byte
}

type benchMethod struct {
	Name string
	Key  string
}

type benchParsedInfo struct {
	ClassName    string
	BenchMethods []benchMethod
}

func newRunner(conf *RunConfig) *runner {
	var output io.Writer

	if conf.TeamcityOutput {
		output = conf.Output
	} else {
		output = io.Discard
	}

	return &runner{conf: conf, logger: teamcity.NewLogger(output)}
}

func (r *runner) debugf(format string, args ...interface{}) {
	if r.conf.DebugPrint != nil {
		r.conf.DebugPrint(fmt.Sprintf(format, args...))
	}
}

func (r *runner) Run() error {
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
		{"check for issues", r.stepCheckIssues},
		{"find bench files", r.stepFindBenchFiles},
		{"prepare temp build dir", r.stepPrepareTempBuildDir},
		{"parse bench files", r.stepParseBenchFiles},
		{"filter only parsed files", r.stepFilterOnlyParsedFiles},
		{"sort bench files", r.stepSortBenchFiles},
		{"generate bench main", r.stepGenerateBenchMain},
		{"run bench", r.stepRunBench},
	}

	for _, step := range steps {
		if err := step.fn(); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}

	return nil
}

func (r *runner) stepCheckIssues() error {
	for _, issue := range checkIssues() {
		log.Printf("WARNING: %s", issue)
	}

	return nil
}

func (r *runner) stepFindBenchFiles() error {
	var testDir string
	var benchFiles []string
	if strings.HasSuffix(r.conf.BenchTarget, ".php") {
		benchFiles = []string{r.conf.BenchTarget}
	} else {
		var err error
		benchFiles, err = findBenchFiles(r.conf.BenchTarget)
		if err != nil {
			return err
		}
	}
	if !strings.HasSuffix(testDir, "/") {
		testDir += "/"
	}

	r.benchFiles = make([]*benchFile, len(benchFiles))
	for i, f := range benchFiles {
		r.benchFiles[i] = &benchFile{
			fullName:  f,
			shortName: strings.TrimPrefix(f, testDir),
		}
	}

	if r.conf.DebugPrint != nil {
		for _, f := range r.benchFiles {
			r.debugf("test file: %q", f.fullName)
		}
	}

	return nil
}

func (r *runner) stepPrepareTempBuildDir() error {
	tempDir, err := ioutil.TempDir("", "kphpbench-build")
	if err != nil {
		return err
	}
	r.buildDir = tempDir
	r.debugf("temp build dir: %q", tempDir)

	return nil
}

func (r *runner) stepParseBenchFiles() error {
	for _, f := range r.benchFiles {
		src, err := ioutil.ReadFile(f.fullName)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		var parserErrors []*errors.Error
		errorHandler := func(e *errors.Error) {
			parserErrors = append(parserErrors, e)
		}
		rootNode, err := parser.Parse(src, conf.Config{
			Version:          &version.Version{Major: 7, Minor: 4},
			ErrorHandlerFunc: errorHandler,
		})
		if err != nil || len(parserErrors) != 0 {
			for _, parseErr := range parserErrors {
				log.Printf("%s: parse error: %v", f.fullName, parseErr)
			}
			return err
		}
		f.info = &benchParsedInfo{}
		visitor := &astVisitor{out: f.info}
		traverser.NewTraverser(visitor).Traverse(rootNode)

		if f.info.ClassName == "" {
			return fmt.Errorf("%s: can't find a benchmark class inside a file", f.shortName)
		}
	}

	return nil
}

func (r *runner) stepFilterOnlyParsedFiles() error {
	parsedFiles := make([]*benchFile, 0, len(r.benchFiles))
	for _, f := range r.benchFiles {
		if f.info != nil {
			parsedFiles = append(parsedFiles, f)
		}
	}
	r.benchFiles = parsedFiles

	return nil
}

func (r *runner) stepSortBenchFiles() error {
	sort.Slice(r.benchFiles, func(i, j int) bool {
		return r.benchFiles[i].fullName < r.benchFiles[j].fullName
	})

	for i, f := range r.benchFiles {
		f.id = i
	}

	return nil
}

func (r *runner) stepGenerateBenchMain() error {
	r.composerMode = fileutil.FileExists(filepath.Join(r.conf.ProjectRoot, "composer.json"))

	for _, f := range r.benchFiles {
		var generated bytes.Buffer
		templateData := map[string]interface{}{
			"BenchFilename":   f.fullName,
			"BenchClassName":  f.info.ClassName,
			"BenchMethods":    f.info.BenchMethods,
			"BenchQN":         fmt.Sprintf("php_qn://%s::%s::", f.fullName, f.info.ClassName),
			"Unroll":          make([]struct{}, 20),
			"MinTries":        20,
			"IterationsRate":  100000000,
			"Count":           r.conf.Count,
			"Teamcity":        r.conf.TeamcityOutput,
			"OnlyPhpAutoload": r.conf.DisableAutoloadForKPHP,
		}
		if r.composerMode {
			templateData["Bootstrap"] = filepath.Join(r.conf.ProjectRoot, "vendor", "autoload.php")
		}
		if err := benchMainTemplate.Execute(&generated, templateData); err != nil {
			return fmt.Errorf("%s: %w", f.fullName, err)
		}
		f.generatedMain = generated.Bytes()
	}

	return nil
}

var benchMainTemplate = template.Must(template.New("bench_main").Parse(`<?php

require_once '{{.BenchFilename}}';

{{if .Bootstrap}}

{{if .OnlyPhpAutoload}}
#ifndef KPHP
{{end}}
require_once '{{.Bootstrap}}';
{{if .OnlyPhpAutoload}}
#endif
{{end}}

{{end}}

function remove_prefix($text, $prefix) {
  if (strpos($text, $prefix) === 0) {
    $text = substr($text, strlen($prefix));
  }
  return $text;
}

function test_started(string $name, string $place) {
{{if .Teamcity}}
  fprintf(STDERR, "##teamcity[testStarted name='%s' locationHint='{{.BenchQN}}%s']\n", remove_prefix($name, "benchmark"), $place);
{{end}}
}

function test_finished(string $name) {
{{if .Teamcity}}
  fprintf(STDERR, "##teamcity[testFinished name='%s']\n", remove_prefix($name, "benchmark"));
{{end}}
}

function __bench_main(int $count) {
  $bench = new {{.BenchClassName}}();
  $min_tries = {{.MinTries}};
  $iterations_rate = {{.IterationsRate}};

  {{range $bench := $.BenchMethods}}

  // try to run the method if it contains an error so that test_started is not executed
  $bench->{{$bench.Name}}();

  test_started("{{$bench.Name}}", "{{$bench.Name}}");

  for ($num_run = 0; $num_run < $count; ++$num_run) {
    fprintf(STDERR, "{{$.BenchClassName}}::{{$bench.Key}}\t");
    // run0 is not counted to allow the warmup
    $bench->{{$bench.Name}}();
    $run1_start = hrtime(true);
    $bench->{{$bench.Name}}();
    $run1_end = hrtime(true);
    $op_time_approx = $run1_end - $run1_start;
    $max_tries = max((int)($iterations_rate / $op_time_approx), $min_tries);
    $time_total = 0;
    $i = 0;
    while ($i < $max_tries) {
      $start = hrtime(true);
      {{ range $.Unroll}}
      $bench->{{$bench.Name}}();
      {{- end}}
      $time_total += hrtime(true) - $start;
      $i += {{len $.Unroll}};
    }
    $avg_time = (int)($time_total / $i);
    fprintf(STDERR, "$i\t$avg_time.0 ns/op\n");
  }

  test_finished("{{$bench.Name}}");

  {{- end}}
}

$count = '{{.Count}}';
__bench_main(intval($count));
`))

func (r *runner) runPhpBench() error {
	for _, f := range r.benchFiles {
		r.logger.TestSuiteStarted(f.info.ClassName)

		mainFilename := filepath.Join(r.buildDir, "main.php")
		if err := fileutil.WriteFile(mainFilename, f.generatedMain); err != nil {
			return err
		}

		args := []string{
			"-f", mainFilename,
		}
		runCommand := exec.Command(r.conf.PhpCommand, args...)
		runCommand.Dir = r.buildDir
		var runStdout bytes.Buffer
		runCommand.Stderr = r.conf.Output
		runCommand.Stdout = &runStdout
		fmt.Fprintf(r.conf.Output, "class: %s\n", f.info.ClassName)
		start := time.Now()
		runErr := runCommand.Run()
		elapsed := time.Since(start)
		if runErr != nil {
			log.Printf("%s: run error: %v\nPHP Output:\n%s", f.fullName, runErr, runStdout.String())
			r.logger.TestSuiteFinished(f.info.ClassName, elapsed)

			return fmt.Errorf("error running %s", f.fullName)
		}
		fmt.Fprintf(r.conf.Output, "ok %s %v\n", f.info.ClassName, elapsed)

		r.logger.TestSuiteFinished(f.info.ClassName, elapsed)
	}

	return nil
}

func (r *runner) stepRunBench() error {
	if r.conf.PhpCommand != "" {
		return r.runPhpBench()
	}

	for _, f := range r.benchFiles {
		r.logger.TestSuiteStarted(f.info.ClassName)

		mainFilename := filepath.Join(r.buildDir, "main.php")
		if err := fileutil.WriteFile(mainFilename, f.generatedMain); err != nil {
			return err
		}

		// 1. Build.
		args := []string{
			"--mode", "cli",
			"--destination-directory", r.buildDir,
		}
		if r.conf.AdditionalKphpIncludeDirs != "" {
			for _, dir := range strings.Split(r.conf.AdditionalKphpIncludeDirs, ",") {
				args = append(args, "-I", dir)
			}
		}
		if r.composerMode {
			args = append(args, "--composer-root", r.conf.ProjectRoot)
		}
		args = append(args, mainFilename)

		r.debugf("kphp run command: %s", strings.Join(args, " "))

		buildCommand := exec.Command(r.conf.KphpCommand, args...)
		buildCommand.Dir = r.buildDir
		out, err := buildCommand.CombinedOutput()
		if err != nil {
			log.Printf("%s: build error: %v: %s", f.fullName, err, out)
			return fmt.Errorf("can't build %s", f.fullName)
		}

		// 2. Run.
		executableName := filepath.Join(r.buildDir, "cli")
		runCommand := exec.Command(executableName)
		runCommand.Dir = r.buildDir
		var runStdout bytes.Buffer
		runCommand.Stderr = r.conf.Output
		runCommand.Stdout = &runStdout
		fmt.Fprintf(r.conf.Output, "class: %s\n", f.info.ClassName)
		start := time.Now()
		runErr := runCommand.Run()
		elapsed := time.Since(start)
		if runErr != nil {
			log.Printf("%s: rerror: %v", f.fullName, runErr)
			return fmt.Errorf("error running %s", f.fullName)
		}
		fmt.Fprintf(r.conf.Output, "ok %s %v\n", f.info.ClassName, elapsed)

		r.logger.TestSuiteFinished(f.info.ClassName, elapsed)
	}

	return nil
}
