package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/VKCOM/ktest/internal/fileutil"
)

func benchmarkVsPHP(args []string) error {
	fs := flag.NewFlagSet("ktest bench-vs-php", flag.ExitOnError)
	flagGeomean := fs.Bool("geomean", false, "print the geometric mean of each file")
	flagCount := fs.Int("count", 10, `run each benchmark n times`)
	flagPhpCommand := fs.String("php", "php", `PHP command to run the benchmarks`)
	flagKphpCommand := fs.String("kphp2cpp-binary", "", `kphp binary path; if empty, $KPHP_ROOT/objs/kphp2cpp is used`)
	fs.Parse(args)

	if len(fs.Args()) == 0 {
		// TODO: print command help here?
		log.Printf("Expected at least 1 positional argument, the benchmarking target")
		return nil
	}

	benchTarget := fs.Args()[0]

	printProgress := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintf(os.Stderr, "\033[2K\r%s", msg)
	}
	flushProgress := func() {
		printProgress("")
	}

	// In case error occurs, we want to clear all progress-related text.
	defer func() {
		flushProgress()
	}()

	runBenchWithProgress := func(label, command string, args []string) ([]byte, error) {
		cmd := exec.Command(command, args...)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		completed := 0
		ch := make(chan error)
		ticker := time.NewTicker(1 * time.Second)
		go func() {
			ch <- cmd.Run()
		}()
		for {
			select {
			case err := <-ch:
				if err != nil {
					combined := append(stderr.Bytes(), stdout.Bytes()...)
					return nil, fmt.Errorf("run %s benchmarks: %v: %s", label, err, combined)
				}
				return stdout.Bytes(), nil
			case <-ticker.C:
				lines := bytes.Count(stdout.Bytes(), []byte("\n"))
				if completed != lines {
					completed = lines
					printProgress("running %s benchmarks: got %d samples...", label, completed)
				}
			}
		}
	}

	var createdFiles []string
	defer func() {
		for _, f := range createdFiles {
			os.Remove(f)
		}
	}()
	createTempFile := func(data []byte) (string, error) {
		f, err := ioutil.TempFile("", "ktest-bench")
		if err != nil {
			return "", err
		}
		createdFiles = append(createdFiles, f.Name())
		if _, err := f.Write(data); err != nil {
			return "", err
		}
		return f.Name(), nil
	}

	// 1. Run `ktest bench ... > kphpResultsFile`
	// 2. Run `ktest bench-php ... > phpResultsFile`
	// 3. Run `ktest benchstat phpResultsFile kphpResultsFile`

	var kphpResultsFile string
	var phpResultsFile string

	printProgress("compiling KPHP benchmarks...")
	{
		args := []string{
			"bench",
			"--count", fmt.Sprint(*flagCount),
		}
		if *flagKphpCommand != "" {
			args = append(args, "--kphp2cpp-binary", *flagKphpCommand)
		}
		args = append(args, benchTarget)
		out, err := runBenchWithProgress("KPHP", os.Args[0], args)
		if err != nil {
			return err
		}
		filename, err := createTempFile(out)
		if err != nil {
			return err
		}
		kphpResultsFile = filename
	}

	printProgress("running PHP benchmarks...")
	{
		args := []string{
			"bench-php",
			"--count", fmt.Sprint(*flagCount),
		}
		if *flagPhpCommand != "" {
			args = append(args, "--php", *flagPhpCommand)
		}
		args = append(args, benchTarget)
		out, err := runBenchWithProgress("PHP", os.Args[0], args)
		if err != nil {
			return err
		}
		filename, err := createTempFile(out)
		if err != nil {
			return err
		}
		phpResultsFile = filename
	}

	printProgress("running benchstat...")
	{
		colorize := "false"
		if fileutil.IsUnixCharDevice(os.Stdout) {
			colorize = "true"
		}
		args := []string{
			"benchstat",
			"-colorize", colorize,
		}
		if *flagGeomean {
			args = append(args, "-geomean")
		}
		args = append(args, phpResultsFile, kphpResultsFile)
		out, err := exec.Command(os.Args[0], args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("run benchstat: %v: %s", err, out)
		}
		out = bytes.Replace(out, []byte("old time"), []byte("PHP time"), 1)
		out = bytes.Replace(out, []byte("new time"), []byte("KPHP time"), 1)
		flushProgress()
		fmt.Print(string(out))
	}

	return nil
}
