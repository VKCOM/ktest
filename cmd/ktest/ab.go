package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"golang.org/x/perf/benchstat"
)

func cmdBenchAB(args []string) error {
	fs := flag.NewFlagSet("ktest bench-ab", flag.ExitOnError)
	fs.Usage = func() {
		log.Printf("Usage: ktest bench-ab Old New [...bench command args]")
		log.Printf("Old is a regexp for A benchmark name")
		log.Printf("New is a regexp for B benchmark name")
		log.Printf("To see bench args run `ktest bench --help`")
	}
	fs.Parse(args)

	if len(fs.Args()) < 2 {
		fs.Usage()
		return errors.New("not enough arguments")
	}

	oldPattern := fs.Arg(0)
	newPattern := fs.Arg(1)

	oldRegexp, err := regexp.Compile(oldPattern)
	if err != nil {
		return fmt.Errorf("compile Old (A) benchmark pattern %q: %v", oldPattern, err)
	}
	newRegexp, err := regexp.Compile(newPattern)
	if err != nil {
		return fmt.Errorf("compile New (B) benchmark pattern %q: %v", newPattern, err)
	}

	// In case error occurs, we want to clear all progress-related text.
	defer func() {
		flushProgress()
	}()

	printProgress("compiling KPHP benchmarks...")
	benchArgs := []string{"bench"}
	benchArgs = append(benchArgs, fs.Args()[2:]...)
	out, err := runBenchWithProgress(oldPattern+" and "+newPattern, os.Args[0], benchArgs)
	if err != nil {
		return err
	}

	// Divide collected samples in three groups:
	// 1. old result set
	// 2. new result set
	// 3. ignored result set (not actually saved)
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
		} else if newRegexp.MatchString(benchName) {
			newResults.WriteString(newLine + "\n")
		}
	}

	// Run a benchstat without running subcommand and creating extra tmp files.
	// We'll use the default options and colored output.
	benchstatCollection := &benchstat.Collection{
		Alpha:     0.05,
		DeltaTest: benchstat.UTest,
	}
	if err := benchstatCollection.AddFile("old", &oldResults); err != nil {
		return err
	}
	if err := benchstatCollection.AddFile("new", &newResults); err != nil {
		return err
	}
	tables := benchstatCollection.Tables()
	colorizeBenchstatTables(tables)

	flushProgress()
	var buf bytes.Buffer
	benchstat.FormatText(&buf, tables)
	os.Stdout.Write(buf.Bytes())

	return nil
}
