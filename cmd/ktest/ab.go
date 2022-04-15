package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/perf/benchstat"
)

type abResult struct {
	oldSamples io.Reader
	newSamples io.Reader
	addGeomean bool
}

func cmdBenchAB(args []string) error {
	const usageHelp = `
Usage [1]: ktest bench-ab old new [...bench command args]
* old is a regexp for first benchmark name
* new is a regexp for second benchmark name
Runs old and new benchmark and compares results with benchstat
Note: bench-ab implicitely passes -count=10 along command line
      args forwarded to bench, but you can override that by
	  specifying it explicitely

Example: ktest bench-ab Implode ConcatLoop BenchmarkStrinOps.php
Example: ktest bench-ab FuncCall Inlined --count 20 --benchmem BenchmarkArrays.php

Usage [2]: ktest bench-ab oldfile newfile
* oldfile is a first benchmark filename
* newfile is a second benchmark filename
All benchmarks from oldfile are compared with results from newfile

Example: ktest bench-ab BenchmarkA.php BenchmarkB.php
`

	fs := flag.NewFlagSet("ktest bench-ab", flag.ExitOnError)
	fs.Usage = func() {
		log.Print(strings.TrimSpace(usageHelp))
	}
	fs.Parse(args)

	if len(fs.Args()) < 2 {
		fs.Usage()
		return errors.New("not enough arguments")
	}

	oldPattern := fs.Arg(0)
	newPattern := fs.Arg(1)

	// In case error occurs, we want to clear all progress-related text.
	defer func() {
		flushProgress()
	}()

	runFunc := runFuncsAB
	if strings.HasSuffix(oldPattern, ".php") && strings.HasSuffix(newPattern, ".php") {
		runFunc = runFilesAB
	}
	result, err := runFunc(oldPattern, newPattern, fs.Args()[2:])
	if err != nil {
		return err
	}
	return abCompare(result)
}

func abCompare(results *abResult) error {
	// Run a benchstat without running subcommand and creating extra tmp files.
	// We'll use the default options and colored output.
	benchstatCollection := &benchstat.Collection{
		Alpha:      0.05,
		DeltaTest:  benchstat.UTest,
		AddGeoMean: results.addGeomean,
	}
	if err := benchstatCollection.AddFile("old", results.oldSamples); err != nil {
		return err
	}
	if err := benchstatCollection.AddFile("new", results.newSamples); err != nil {
		return err
	}
	tables := benchstatCollection.Tables()

	flushProgress()
	fixBenchstatTables(tables)
	colorizeBenchstatTables(tables)
	var buf bytes.Buffer
	benchstat.FormatText(&buf, tables)
	os.Stdout.Write(buf.Bytes())

	return nil
}

func runFuncsAB(oldPattern, newPattern string, args []string) (*abResult, error) {
	oldRegexp, err := regexp.Compile(oldPattern)
	if err != nil {
		return nil, fmt.Errorf("compile Old (first) benchmark pattern %q: %v", oldPattern, err)
	}
	newRegexp, err := regexp.Compile(newPattern)
	if err != nil {
		return nil, fmt.Errorf("compile New (second) benchmark pattern %q: %v", newPattern, err)
	}

	printProgress("compiling KPHP benchmarks...")
	benchArgs := []string{
		"bench",
		"--count", "10",
		"--run", fmt.Sprintf("(?:%s)|(?:%s)", oldPattern, newPattern),
	}
	benchArgs = append(benchArgs, args...)
	out, err := runBenchWithProgress(oldPattern+" and "+newPattern, os.Args[0], benchArgs)
	if err != nil {
		return nil, err
	}

	// Divide collected samples in three groups:
	// 1. old result set
	// 2. new result set
	// 3. ignored result set (not actually saved)
	oldBenchmarkName := ""
	newBenchmarkName := ""
	var oldResults bytes.Buffer
	var newResults bytes.Buffer
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}
		tabPos := strings.IndexByte(line, '\t')
		if tabPos == -1 {
			continue
		}
		benchName := line[:tabPos]
		// We'll get "Benchmark" title in the output since one Benchmark
		// prefix will be removed by the benchstat.
		newLine := "BenchmarkBenchmark" + line[tabPos:]
		if oldRegexp.MatchString(benchName) {
			oldResults.WriteString(newLine + "\n")
			if oldBenchmarkName == "" {
				oldBenchmarkName = benchName
			} else if oldBenchmarkName != benchName {
				return nil, fmt.Errorf("%s regexp matched more than one benchmark: %s and %s", oldPattern, oldBenchmarkName, benchName)
			}
		}
		if newRegexp.MatchString(benchName) {
			newResults.WriteString(newLine + "\n")
			if newBenchmarkName == "" {
				newBenchmarkName = benchName
			} else if newBenchmarkName != benchName {
				return nil, fmt.Errorf("%s regexp matched more than one benchmark: %s and %s", newPattern, newBenchmarkName, benchName)
			}
		}
	}

	if oldBenchmarkName == newBenchmarkName {
		return nil, fmt.Errorf("old/new regexp both matched %s", oldBenchmarkName)
	}
	if oldBenchmarkName == "" {
		return nil, fmt.Errorf("%s regexp matched no benchmarks", oldPattern)
	}
	if newBenchmarkName == "" {
		return nil, fmt.Errorf("%s regexp matched no benchmarks", newPattern)
	}
	result := &abResult{
		oldSamples: &oldResults,
		newSamples: &newResults,
	}
	return result, nil
}

func runFilesAB(oldFilename, newFilename string, args []string) (*abResult, error) {
	benchArgs := []string{"bench", "--count", "10", "--benchmem"}

	oldKey := strings.TrimSuffix(filepath.Base(oldFilename), ".php")
	newKey := strings.TrimSuffix(filepath.Base(newFilename), ".php")

	printProgress(fmt.Sprintf("compiling %s...", oldKey))
	oldOutput, err := runBenchWithProgress(oldKey, os.Args[0], append(benchArgs, oldFilename))
	if err != nil {
		return nil, err
	}

	printProgress(fmt.Sprintf("compiling %s...", newKey))
	newOutput, err := runBenchWithProgress(newKey, os.Args[0], append(benchArgs, newFilename))
	if err != nil {
		return nil, err
	}

	newOutput = bytes.ReplaceAll(newOutput, []byte(newKey+"::"), []byte(oldKey+"::"))
	result := &abResult{
		oldSamples: bytes.NewReader(oldOutput),
		newSamples: bytes.NewReader(newOutput),
		addGeomean: true,
	}
	return result, nil
}
