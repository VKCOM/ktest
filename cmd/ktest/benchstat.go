package main

import (
	"bytes"
	"errors"
	"flag"
	"log"
	"math"
	"os"
	"strings"

	"github.com/VKCOM/ktest/internal/fileutil"
	"golang.org/x/perf/benchstat"
)

func colorizeText(str string, colorCode string) string {
	return colorCode + str + "\033[0m"
}

func redColorize(str string) string {
	return colorizeText(str, "\033[31m")
}

func greenColorize(str string) string {
	return colorizeText(str, "\033[32m")
}

func yellowColorize(str string) string {
	return colorizeText(str, "\033[33m")
}

func calculateMeanDiff(m *benchstat.Metrics) float64 {
	if m.Mean == 0 || m.Max == 0 {
		return 0
	}
	diff := 1 - m.Min/m.Mean
	if d := m.Max/m.Mean - 1; d > diff {
		diff = d
	}
	return diff
}

func calculateCombinedMeanDiff(metrics []*benchstat.Metrics) float64 {
	d := 0.0
	for _, m := range metrics {
		if m.Max == m.Min {
			continue
		}
		d += 100.0 * calculateMeanDiff(m)
	}
	return d
}

func isTinyValue(metrics []*benchstat.Metrics) bool {
	const tinyValueThreshold = 32.0 // in nanosecs
	for _, m := range metrics {
		if m.Mean >= tinyValueThreshold {
			return false
		}
	}
	return true
}

func colorizeBenchstatTables(tables []*benchstat.Table) {
	for _, table := range tables {
		for _, row := range table.Rows {
			d := calculateCombinedMeanDiff(row.Metrics)
			if isTinyValue(row.Metrics) {
				// For tiny values, require x2 precision.
				d *= 2
			}
			d++
			if math.Abs(row.PctDelta) < d {
				row.Delta = yellowColorize("~")
				continue
			}
			if strings.HasPrefix(row.Delta, "+") {
				row.Delta = redColorize(row.Delta)
			} else if strings.HasPrefix(row.Delta, "-") {
				row.Delta = greenColorize(row.Delta)
			} else {
				row.Delta = yellowColorize(row.Delta)
			}
		}
	}
}

func fixBenchstatTables(tables []*benchstat.Table) {
	disabledGeomean := map[string]struct{}{}
	for _, table := range tables {
		selectedRows := table.Rows[:0]
		for _, row := range table.Rows {
			if row.PctDelta == 0 && strings.Contains(row.Delta, "0.00%") {
				// For whatever reason, sometimes we get +0.00% results
				// in delta which will be painted red. This is misleading.
				// Let's replace +0.00% with tilde.
				row.Delta = "~"
			}
			for _, m := range row.Metrics {
				for _, v := range m.RValues {
					if v < 0.01 {
						disabledGeomean[m.Unit] = struct{}{}
					}
				}
			}
			if row.Benchmark == "[Geo mean]" {
				if len(row.Metrics) != 0 {
					_, disabled := disabledGeomean[row.Metrics[0].Unit]
					if disabled {
						continue
					}
				}
			}
			selectedRows = append(selectedRows, row)
			if len(row.Metrics) == 0 {
				continue
			}
			if len(row.Metrics[0].RValues) < 5 && row.Benchmark != "[Geo mean]" {
				log.Printf("WARNING: %s needs more samples, re-run with -count=5 or higher?", row.Benchmark)
			}
		}
		table.Rows = selectedRows
	}
}

func cmdBenchstat(args []string) error {
	fs := flag.NewFlagSet("ktest benchstat", flag.ExitOnError)
	flagDeltaTest := fs.String("delta-test", "utest", "significance `test` to apply to delta: utest, ttest, or none")
	flagAlpha := fs.Float64("alpha", 0.05, "consider change significant if p < `α`")
	flagGeomean := fs.Bool("geomean", false, "print the geometric mean of each file")
	flagSplit := fs.String("split", "pkg,goos,goarch", "split benchmarks by `labels`")
	flagSort := fs.String("sort", "none", "sort by `order`: [-]delta, [-]name, none")
	colorize := fs.String("colorize", "auto", "colorize output: auto, true, false")
	fs.Parse(args)

	enableColorize := strings.ToLower(*colorize) == "true"
	if *colorize == "auto" {
		enableColorize = fileutil.IsUnixCharDevice(os.Stdout)
	}

	var deltaTestNames = map[string]benchstat.DeltaTest{
		"none":   benchstat.NoDeltaTest,
		"u":      benchstat.UTest,
		"u-test": benchstat.UTest,
		"utest":  benchstat.UTest,
		"t":      benchstat.TTest,
		"t-test": benchstat.TTest,
		"ttest":  benchstat.TTest,
	}

	var sortNames = map[string]benchstat.Order{
		"none":  nil,
		"name":  benchstat.ByName,
		"delta": benchstat.ByDelta,
	}

	deltaTest := deltaTestNames[strings.ToLower(*flagDeltaTest)]
	if deltaTest == nil {
		return errors.New("invalid delta-test argument")
	}
	sortName := *flagSort
	reverse := false
	if strings.HasPrefix(sortName, "-") {
		reverse = true
		sortName = sortName[1:]
	}
	order, ok := sortNames[sortName]
	if !ok {
		return errors.New("invalid sort argument")
	}

	if len(fs.Args()) == 0 {
		// TODO: print command help here?
		log.Printf("Expected at least 1 positional argument, the benchmarking target")
		return nil
	}

	c := &benchstat.Collection{
		Alpha:      *flagAlpha,
		AddGeoMean: *flagGeomean,
		DeltaTest:  deltaTest,
	}
	if *flagSplit != "" {
		c.SplitBy = strings.Split(*flagSplit, ",")
	}
	if order != nil {
		if reverse {
			order = benchstat.Reverse(order)
		}
		c.Order = order
	}
	for _, file := range fs.Args() {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		if err := c.AddFile(file, f); err != nil {
			return err
		}
		f.Close()
	}

	tables := c.Tables()
	fixBenchstatTables(tables)
	if enableColorize {
		colorizeBenchstatTables(tables)
	}
	var buf bytes.Buffer
	benchstat.FormatText(&buf, tables)
	os.Stdout.Write(buf.Bytes())

	return nil
}
