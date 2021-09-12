package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/VKCOM/ktest/internal/bench"
	"github.com/VKCOM/ktest/internal/kenv"
	"github.com/VKCOM/ktest/internal/phpunit"
	"github.com/cespare/subcmd"
)

// Build* variables are initialized during the build via -ldflags.
var (
	BuildVersion string
	BuildTime    string
	BuildOSUname string
	BuildCommit  string
)

func main() {
	log.SetFlags(0)

	cmds := []subcmd.Command{
		{
			Name:        "phpunit",
			Description: "run phpunit tests using KPHP",
			Do:          phpunitMain,
		},

		{
			Name:        "benchstat",
			Description: "compute and compare statistics about benchmark results",
			Do:          benchstatMain,
		},

		{
			Name:        "bench",
			Description: "run benchmarks using KPHP",
			Do:          benchMain,
		},

		{
			Name:        "bench-php",
			Description: "run benchmarks using PHP",
			Do:          benchPHPMain,
		},

		{
			Name:        "bench-vs-php",
			Description: "run benchmarks using both KPHP and PHP, compare the results",
			Do:          benchVsPHPMain,
		},

		{
			Name:        "env",
			Description: "print ktest-related env variables",
			Do:          envMain,
		},

		{
			Name:        "version",
			Description: "print ktest version info",
			Do:          versionMain,
		},
	}

	subcmd.Run(cmds)
}

func versionMain(args []string) {
	if BuildCommit == "" {
		fmt.Printf("ktest built without version info\n")
	} else {
		fmt.Printf("ktest version %s\nbuilt on: %s\nos: %s\ncommit: %s\n",
			BuildVersion, BuildTime, BuildOSUname, BuildCommit)
	}
}

func envMain(args []string) {
	kphpVars := []string{
		"KPHP_ROOT",
		"KPHP_TESTS_POLYFILLS_REPO",
	}

	for _, name := range kphpVars {
		v := os.Getenv(name)
		fmt.Printf("%s=%q\n", name, v)
	}
}

func benchstatMain(args []string) {
	if err := cmdBenchstat(args); err != nil {
		log.Fatalf("ktest benchstat: error: %v", err)
	}
}

func benchMain(args []string) {
	if err := cmdBench(args); err != nil {
		log.Fatalf("ktest bench: error: %v", err)
	}
}

func benchPHPMain(args []string) {
	if err := cmdBenchPHP(args); err != nil {
		log.Fatalf("ktest bench-php: error: %v", err)
	}
}

func benchVsPHPMain(args []string) {
	if err := cmdBenchVsPHP(args); err != nil {
		log.Fatalf("ktest bench-vs-php: error: %v", err)
	}
}

func cmdBenchVsPHP(args []string) error {
	return benchmarkVsPHP(args)
}

func cmdBenchPHP(args []string) error {
	conf := &bench.RunConfig{}

	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("ktest bench-php", flag.ExitOnError)
	debug := fs.Bool("debug", false,
		`print debug info`)
	fs.IntVar(&conf.Count, "count", 1,
		`run each benchmark n times`)
	fs.BoolVar(&conf.NoCleanup, "no-cleanup", false,
		`whether to keep temp build directory`)
	fs.StringVar(&conf.ProjectRoot, "project-root", workdir,
		`project root directory`)
	fs.StringVar(&conf.PhpCommand, "php", "php",
		`PHP command to run the benchmarks`)
	fs.Parse(args)

	if len(fs.Args()) == 0 {
		// TODO: print command help here?
		log.Printf("Expected at least 1 positional argument, the benchmarking target")
		return nil
	}

	benchTarget, err := filepath.Abs(fs.Args()[0])
	if err != nil {
		return fmt.Errorf("resolve benchmarking target path: %v", err)
	}

	conf.BenchTarget = benchTarget
	conf.Output = os.Stdout
	if *debug {
		conf.DebugPrint = func(msg string) {
			log.Print(msg)
		}
	}
	return benchCmdImpl(conf)
}

func cmdBench(args []string) error {
	conf := &bench.RunConfig{}

	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("ktest bench", flag.ExitOnError)
	debug := fs.Bool("debug", false,
		`print debug info`)
	fs.IntVar(&conf.Count, "count", 1,
		`run each benchmark n times`)
	fs.BoolVar(&conf.NoCleanup, "no-cleanup", false,
		`whether to keep temp build directory`)
	fs.StringVar(&conf.ProjectRoot, "project-root", workdir,
		`project root directory`)
	fs.StringVar(&conf.KphpCommand, "kphp2cpp-binary", "",
		`kphp binary path; if empty, $KPHP_ROOT/objs/kphp2cpp is used`)
	fs.Parse(args)

	if len(fs.Args()) == 0 {
		// TODO: print command help here?
		log.Printf("Expected at least 1 positional argument, the benchmarking target")
		return nil
	}

	benchTarget, err := filepath.Abs(fs.Args()[0])
	if err != nil {
		return fmt.Errorf("resolve benchmarking target path: %v", err)
	}

	conf.BenchTarget = benchTarget
	conf.Output = os.Stdout
	if *debug {
		conf.DebugPrint = func(msg string) {
			log.Print(msg)
		}
	}
	return benchCmdImpl(conf)
}

func benchCmdImpl(conf *bench.RunConfig) error {
	var err error
	conf.ProjectRoot, err = filepath.Abs(conf.ProjectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root path: %v", err)
	}
	if !strings.HasSuffix(conf.ProjectRoot, "/") {
		conf.ProjectRoot += "/"
	}

	if conf.KphpCommand == "" {
		kphpBinary := kenv.FindKphpBinary()
		if kphpBinary == "" {
			return fmt.Errorf("can't locate kphp2cpp binary; please set -kphp2cpp-binary arg")
		}
		conf.KphpCommand = kphpBinary
	}

	if err := bench.Run(conf); err != nil {
		return err
	}

	return nil
}

func phpunitMain(args []string) {
	if err := cmdPhpunit(args); err != nil {
		log.Fatalf("ktest phpunit: error: %v", err)
	}
}

func cmdPhpunit(args []string) error {
	conf := &phpunit.RunConfig{}

	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("ktest phpunit", flag.ExitOnError)
	debug := fs.Bool("debug", false,
		`print debug info`)
	fs.BoolVar(&conf.NoCleanup, "no-cleanup", false,
		`whether to keep temp build directory`)
	fs.StringVar(&conf.ProjectRoot, "project-root", workdir,
		`project root directory`)
	fs.StringVar(&conf.SrcDir, "src-dir", "src",
		`project sources root`)
	fs.StringVar(&conf.KphpCommand, "kphp2cpp-binary", "",
		`kphp binary path; if empty, $KPHP_ROOT/objs/kphp2cpp is used`)
	fs.Parse(args)

	if len(fs.Args()) == 0 {
		// TODO: print command help here?
		log.Printf("Expected at least 1 positional argument, the test target")
		return nil
	}

	testTarget, err := filepath.Abs(fs.Args()[0])
	if err != nil {
		return fmt.Errorf("resolve test target path: %v", err)
	}

	conf.ProjectRoot, err = filepath.Abs(conf.ProjectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root path: %v", err)
	}
	if !strings.HasSuffix(conf.ProjectRoot, "/") {
		conf.ProjectRoot += "/"
	}

	conf.TestTarget = testTarget
	conf.TestArgv = fs.Args()[1:]
	conf.Output = os.Stdout

	if *debug {
		conf.DebugPrint = func(msg string) {
			log.Print(msg)
		}
	}

	if conf.KphpCommand == "" {
		kphpBinary := kenv.FindKphpBinary()
		if kphpBinary == "" {
			return fmt.Errorf("can't locate kphp2cpp binary; please set -kphp2cpp-binary arg")
		}
		conf.KphpCommand = kphpBinary
	}

	result, err := phpunit.Run(conf)
	if err != nil {
		return err
	}

	formatConfig := &phpunit.FormatConfig{
		PrintTime: true,
	}
	phpunit.FormatResult(os.Stdout, formatConfig, result)

	return nil
}
