package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"github.com/VKCOM/ktest/internal/fileutil"
	"github.com/VKCOM/ktest/internal/teamcity"
)

func benchmarkVsPHP(args []string) error {
	fs := flag.NewFlagSet("ktest bench-vs-php", flag.ExitOnError)
	flagGeomean := fs.Bool("geomean", false, "print the geometric mean of each file")
	flagCount := fs.Int("count", 10, `run each benchmark n times`)
	flagPhpCommand := fs.String("php", "php", `PHP command to run the benchmarks`)
	flagPreload := fs.String("preload", "", `opcache.preload script`)
	flagRunFilter := fs.String("run", ".*", `regexp that selects the benchmarks to run`)
	flagKphpCommand := fs.String("kphp2cpp-binary", envString("KTEST_KPHP2CPP_BINARY", ""), `kphp binary path; if empty, $KPHP_ROOT/objs/kphp2cpp is used`)
	flagAdditionalKphpIncludeDirs := fs.String("include-dirs", envString("KTEST_INCLUDE_DIRS", ""), `comma separated list of additional kphp include-dirs`)
	flagDisableKphpAutoload := fs.Bool("disable-kphp-autoload", envBool("KTEST_DISABLE_KPHP_AUTOLOAD", false), `disables autoload for KPHP`)
	flagTeamcity := fs.Bool("teamcity", false, `report bench execution progress in TeamCity format`)
	fs.Parse(args)

	if len(fs.Args()) == 0 {
		// TODO: print command help here?
		log.Printf("Expected at least 1 positional argument, the benchmarking target")
		return nil
	}

	logger := teamcity.NewLogger(os.Stdout)

	benchTarget := fs.Args()[0]

	// In case error occurs, we want to clear all progress-related text.
	defer func() {
		flushProgress()
	}()

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

	if *flagTeamcity {
		logger.TestStarted("PHP vs KPHP performance comparison")
	}

	printProgress("compiling KPHP benchmarks...")
	{
		args := []string{
			"bench",
			"--run", *flagRunFilter,
			"--count", fmt.Sprint(*flagCount),
		}
		if *flagDisableKphpAutoload {
			args = append(args, "--disable-kphp-autoload")
		}
		if *flagAdditionalKphpIncludeDirs != "" {
			args = append(args, "--include-dirs", *flagAdditionalKphpIncludeDirs)
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
			"--run", *flagRunFilter,
			"--count", fmt.Sprint(*flagCount),
		}
		if *flagPreload != "" {
			args = append(args, "--preload", *flagPreload)
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

	if *flagTeamcity {
		logger.TestFinished("PHP vs KPHP performance comparison")
	}

	return nil
}
