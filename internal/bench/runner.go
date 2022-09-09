package bench

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/z7zmey/php-parser/pkg/conf"
	phperrors "github.com/z7zmey/php-parser/pkg/errors"
	"github.com/z7zmey/php-parser/pkg/parser"
	"github.com/z7zmey/php-parser/pkg/version"
	"github.com/z7zmey/php-parser/pkg/visitor/traverser"

	"github.com/VKCOM/ktest/internal/fileutil"
	"github.com/VKCOM/ktest/internal/kphpscript"
	"github.com/VKCOM/ktest/internal/phpscript"
	"github.com/VKCOM/ktest/internal/teamcity"
)

type runner struct {
	conf *RunConfig

	benchFiles []*benchFile

	logger *teamcity.Logger

	buildDir       string
	profilerPrefix string
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
	ClassFQN     string
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
		{"move profiles", r.moveProfiles},
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

	if r.conf.ProfileDir != "" {
		r.profilerPrefix = filepath.Join(r.buildDir, "profiles", "ktest")
		if err := fileutil.MkdirAll(filepath.Dir(r.profilerPrefix)); err != nil {
			return err
		}
	}

	links := []string{
		"vendor",
		"composer.json",
	}

	if r.conf.Preload != "" {
		links = append(links, r.conf.Preload)
	}

	// Respecting the convention of some FFI libs.
	// If ./ffilibs exists, we create a symlink for it as well.
	if fileutil.FileExists(filepath.Join(r.conf.ProjectRoot, "ffilibs")) {
		links = append(links, "ffilibs")
	}

	for _, l := range links {
		if err := os.Symlink(filepath.Join(r.conf.ProjectRoot, l), filepath.Join(tempDir, l)); err != nil {
			return err
		}
	}

	return nil
}

func (r *runner) stepParseBenchFiles() error {
	for _, f := range r.benchFiles {
		src, err := ioutil.ReadFile(f.fullName)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		var parserErrors []*phperrors.Error
		errorHandler := func(e *phperrors.Error) {
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
		visitor := &astVisitor{out: f.info, currentFileName: f.shortName}
		traverser.NewTraverser(visitor).Traverse(rootNode)

		for _, err = range visitor.issues {
			return err
		}

		if f.info.ClassFQN == "" {
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
	numBenchmarksSelected := 0
	re, err := regexp.Compile(r.conf.RunFilter)
	if err != nil {
		return err
	}
	for _, f := range r.benchFiles {
		selectedMethods := f.info.BenchMethods[:0]
		for _, m := range f.info.BenchMethods {
			fullName := f.info.ClassName + "::" + m.Key
			if re.MatchString(fullName) {
				numBenchmarksSelected++
				selectedMethods = append(selectedMethods, m)
			}
		}
		f.info.BenchMethods = selectedMethods
	}
	if numBenchmarksSelected == 0 {
		return errors.New("selected benchmarks set contains no methods to run")
	}

	for _, f := range r.benchFiles {
		var generated bytes.Buffer
		templateData := map[string]interface{}{
			"ProfilingEnabled": r.conf.ProfileDir != "",
			"BenchFilename":    f.fullName,
			"BenchClassName":   f.info.ClassName,
			"BenchClassFQN":    f.info.ClassFQN,
			"BenchMethods":     f.info.BenchMethods,
			"BenchQN":          fmt.Sprintf("php_qn://%s::%s::", f.fullName, f.info.ClassFQN),
			"Unroll":           make([]struct{}, 20),
			"MinTries":         20,
			"IterationsRate":   100000000,
			"Count":            r.conf.Count,
			"Teamcity":         r.conf.TeamcityOutput,
			"OnlyPhpAutoload":  r.conf.DisableAutoloadForKPHP,
			"Benchmem":         r.conf.Benchmem, // always false for PHP at the moment
		}
		if r.conf.ComposerRoot != "" {
			templateData["Bootstrap"] = filepath.Join(r.conf.ComposerRoot, "vendor", "autoload.php")
		}
		if err := benchMainTemplate.Execute(&generated, templateData); err != nil {
			return fmt.Errorf("%s: %w", f.fullName, err)
		}
		f.generatedMain = generated.Bytes()
	}

	return nil
}

var benchMainTemplate = template.Must(template.New("bench_main").Parse(`<?php

{{if .Bootstrap}}
  {{ if .OnlyPhpAutoload}}#ifndef KPHP{{end}}
require_once '{{$.Bootstrap}}';
  {{ if .OnlyPhpAutoload}}#endif{{end}}
{{end}}

require_once '{{$.BenchFilename}}';

function remove_prefix($text, $prefix) {
  if (strpos($text, $prefix) === 0) {
    $text = substr($text, strlen($prefix));
  }
  return $text;
}

function remove_benchmark_prefix($text) {
  return remove_prefix(remove_prefix($text, "benchmark"), "_");
}

function test_started(string $name, string $place) {
{{if .Teamcity}}
  fprintf(STDERR, "##teamcity[testStarted name='%s' locationHint='{{$.BenchQN}}%s']\n", remove_benchmark_prefix($name), $place);
{{end}}
}

function test_finished(string $name) {
{{if .Teamcity}}
  fprintf(STDERR, "##teamcity[testFinished name='%s']\n", remove_benchmark_prefix($name));
{{end}}
}

function __bench_main(int $count) {
  global $argv;
  $bench_name = $argv[1];
  switch ($bench_name) {
  {{range $bench := $.BenchMethods}}
    case '{{$bench.Name}}':
      __bench_{{$bench.Name}}($count);
      break;
  {{- end}}
    default:
      fprintf(STDERR, "unexpected method name: $bench_name\n");
      exit(1);
  }
}

{{range $bench := $.BenchMethods}}
/**
 * @param {{$.BenchClassFQN}} $bench
 *
 * {{if $.ProfilingEnabled}}
 * @kphp-profile
 * @kphp-profile-allow-inline
 * {{end}}
 */
function _{{$bench.Name}}($bench) {
  while (false) {
    break;
    if (false) {}
  }
  return $bench->{{$bench.Name}}();
}
function __bench_{{$bench.Name}}(int $count) {
  $bench = new {{$.BenchClassFQN}}();
  $min_tries = {{$.MinTries}};
  $iterations_rate = {{$.IterationsRate}};
  
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
    {{ if $.Benchmem }}
      [$num_allocs_before, $mem_allocated_before] = memory_get_allocations();
    {{ end }}
    $i = 0;
    while ($i < $max_tries) {
      $start = hrtime(true);
      {{ range $.Unroll}}
      _{{$bench.Name}}($bench);
      {{- end}}
      $time_total += hrtime(true) - $start;
      $i += {{len $.Unroll}};
    }
    $avg_time = (int)($time_total / $i);
    {{ if $.Benchmem }}
      [$num_allocs_after, $mem_allocated_after] = memory_get_allocations();
      $op_allocated = (int)(ceil(($mem_allocated_after - $mem_allocated_before) / $i));
      $op_allocs = (int)(ceil(($num_allocs_after - $num_allocs_before) / $i));
      fprintf(STDERR, "$i\t$avg_time.0 ns/op\t$op_allocated B/op\t$op_allocs allocs/op\n");
    {{ else }}
      fprintf(STDERR, "$i\t$avg_time.0 ns/op\n");
    {{ end }}
  }

  test_finished("{{$bench.Name}}");
}
{{- end}}

$count = '{{$.Count}}';
__bench_main(intval($count));
`))

func (r *runner) runPhpBench() error {
	for _, f := range r.benchFiles {
		r.logger.TestSuiteStarted(f.info.ClassFQN)

		mainFilename := filepath.Join(r.buildDir, "main.php")
		if err := fileutil.WriteFile(mainFilename, f.generatedMain); err != nil {
			return err
		}

		fmt.Fprintf(r.conf.Output, "class: %s\n", f.info.ClassFQN)
		timeTotal := time.Duration(0)
		for _, m := range f.info.BenchMethods {
			result, err := phpscript.Run(phpscript.RunConfig{
				PHPCommand: r.conf.PhpCommand,
				Preload:    r.conf.Preload,
				JIT:        !r.conf.NoJIT,
				Script:     mainFilename,
				Workdir:    r.buildDir,
				ScriptArgs: []string{m.Name},
				Stderr:     r.conf.Output,
			})
			timeTotal += result.Time
			if err != nil {
				log.Printf("%s: %s run error: %v", f.fullName, m.Name, err)
				r.logger.TestSuiteFinished(f.info.ClassFQN, result.Time)
				return fmt.Errorf("error running %s", f.fullName)
			}
		}

		fmt.Fprintf(r.conf.Output, "ok %s %v\n", f.info.ClassFQN, timeTotal)
		r.logger.TestSuiteFinished(f.info.ClassFQN, timeTotal)
	}

	return nil
}

func (r *runner) stepRunBench() error {
	if r.conf.PhpCommand != "" {
		return r.runPhpBench()
	}

	for _, f := range r.benchFiles {
		r.logger.TestSuiteStarted(f.info.ClassFQN)

		mainFilename := filepath.Join(r.buildDir, "main.php")
		if err := fileutil.WriteFile(mainFilename, f.generatedMain); err != nil {
			return err
		}

		buildResult, err := kphpscript.Build(kphpscript.BuildConfig{
			ProfilingEnabled:          r.conf.ProfileDir != "",
			KPHPCommand:               r.conf.KphpCommand,
			Script:                    mainFilename,
			ComposerRoot:              r.conf.ComposerRoot,
			OutputDir:                 r.buildDir,
			Workdir:                   r.buildDir,
			AdditionalKphpIncludeDirs: r.conf.AdditionalKphpIncludeDirs,
		})
		if err != nil {
			log.Printf("%s: build error: %v", f.fullName, err)
			return fmt.Errorf("can't build %s", f.fullName)
		}

		fmt.Fprintf(r.conf.Output, "class: %s\n", f.info.ClassFQN)
		timeTotal := time.Duration(0)
		for _, m := range f.info.BenchMethods {
			runResult, err := kphpscript.Run(kphpscript.RunConfig{
				ProfilerPrefix: r.profilerPrefix,
				Executable:     buildResult.Executable,
				Workdir:        r.buildDir,
				ScriptArgs:     []string{m.Name},
				Stderr:         r.conf.Output,
			})
			timeTotal += runResult.Time
			if err != nil {
				log.Printf("%s: %s run error: %v", f.fullName, m.Name, err)
				r.logger.TestSuiteFinished(f.info.ClassFQN, timeTotal)
				return fmt.Errorf("error running %s", f.fullName)
			}
		}

		fmt.Fprintf(r.conf.Output, "ok %s %v\n", f.info.ClassFQN, timeTotal)
		r.logger.TestSuiteFinished(f.info.ClassFQN, timeTotal)
	}

	return nil
}

func (r *runner) moveProfiles() error {
	if r.profilerPrefix == "" {
		return nil
	}
	if err := fileutil.MkdirAll(r.conf.ProfileDir); err != nil {
		return err
	}
	profilesDir := filepath.Dir(r.profilerPrefix)
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`\.[A-F0-9]+\.\d+$`)
	for _, e := range entries {
		oldName := filepath.Join(profilesDir, e.Name())
		name := strings.TrimPrefix(e.Name(), "ktest._")
		name = re.ReplaceAllString(name, "")
		newName := filepath.Join(r.conf.ProfileDir, name+".callgrind")
		data, err := os.ReadFile(oldName)
		if err != nil {
			return err
		}
		if err := fileutil.WriteFile(newName, data); err != nil {
			return err
		}
	}
	return nil
}
