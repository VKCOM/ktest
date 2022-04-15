package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cespare/subcmd"
	"github.com/google/go-cmp/cmp"

	"github.com/VKCOM/ktest/internal/bench"
	"github.com/VKCOM/ktest/internal/fileutil"
	"github.com/VKCOM/ktest/internal/kenv"
	"github.com/VKCOM/ktest/internal/kphpscript"
	"github.com/VKCOM/ktest/internal/phpscript"
	"github.com/VKCOM/ktest/internal/phpunit"
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
			Name:        "compare",
			Description: "test that KPHP and PHP scripts output is identical",
			Do:          compareMain,
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
			Name:        "bench-ab",
			Description: "run two selected benchmarks using KPHP and compare their results",
			Do:          benchABMain,
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

		"KTEST_KPHP2CPP_BINARY",
		"KTEST_DISABLE_KPHP_AUTOLOAD",
		"KTEST_INCLUDE_DIRS",
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

func benchABMain(args []string) {
	if err := cmdBenchAB(args); err != nil {
		log.Fatalf("ktest bench-ab: error: %v", err)
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
	fs.StringVar(&conf.Preload, "preload", "",
		`opcache.preload script`)
	fs.StringVar(&conf.PhpCommand, "php", "php",
		`PHP command to run the benchmarks`)
	fs.StringVar(&conf.RunFilter, "run", ".*",
		`regexp that selects the benchmarks to run`)
	fs.BoolVar(&conf.DisableAutoloadForKPHP, "disable-kphp-autoload", envBool("KTEST_DISABLE_KPHP_AUTOLOAD", false),
		`disables autoload for KPHP`)
	fs.BoolVar(&conf.TeamcityOutput, "teamcity", false,
		`report bench execution progress in TeamCity format`)
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

	conf.ComposerRoot = kenv.FindComposerRoot(conf.ProjectRoot)
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
	fs.StringVar(&conf.KphpCommand, "kphp2cpp-binary", envString("KTEST_KPHP2CPP_BINARY", ""),
		`kphp binary path; if empty, $KPHP_ROOT/objs/kphp2cpp is used`)
	fs.StringVar(&conf.AdditionalKphpIncludeDirs, "include-dirs", envString("KTEST_INCLUDE_DIRS", ""),
		`comma separated list of additional kphp include-dirs`)
	fs.StringVar(&conf.RunFilter, "run", ".*",
		`regexp that selects the benchmarks to run`)
	fs.BoolVar(&conf.DisableAutoloadForKPHP, "disable-kphp-autoload", envBool("KTEST_DISABLE_KPHP_AUTOLOAD", false),
		`disables autoload for KPHP`)
	fs.BoolVar(&conf.TeamcityOutput, "teamcity", false,
		`report bench execution progress in TeamCity format`)
	fs.BoolVar(&conf.Benchmem, "benchmem", false,
		`print memory allocation statistics for benchmarks`)
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

	conf.ComposerRoot = kenv.FindComposerRoot(conf.ProjectRoot)
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

func compareMain(args []string) {
	if err := cmdCompare(args); err != nil {
		log.Fatalf("ktest compare: error: %v", err)
	}
}

func cmdCompare(args []string) error {
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	var kphpCommand string
	fs := flag.NewFlagSet("ktest compare", flag.ExitOnError)
	flagPhpCommand := fs.String("php", "php", `PHP command to run the benchmarks`)
	flagPreload := fs.String("preload", "", `opcache.preload script`)
	flagProjectRoot := fs.String("project-root", workdir,
		`project root directory`)
	fs.StringVar(&kphpCommand, "kphp2cpp-binary", envString("KTEST_KPHP2CPP_BINARY", ""), `kphp binary path; if empty, $KPHP_ROOT/objs/kphp2cpp is used`)
	fs.Parse(args)

	if len(fs.Args()) == 0 {
		log.Printf("Expected exactly 1 positional argument, script filename")
		return nil
	}

	if kphpCommand == "" {
		kphpBinary := kenv.FindKphpBinary()
		if kphpBinary == "" {
			return fmt.Errorf("can't locate kphp2cpp binary; please set -kphp2cpp-binary arg")
		}
		kphpCommand = kphpBinary
	}

	scriptName := fs.Args()[0]

	phpResult, err := phpscript.Run(phpscript.RunConfig{
		PHPCommand: *flagPhpCommand,
		Preload:    *flagPreload,
		Script:     scriptName,
		Workdir:    workdir,
	})
	if err != nil {
		return fmt.Errorf("run php: %v", err)
	}

	composerRoot := *flagProjectRoot
	if !fileutil.FileExists(filepath.Join(*flagProjectRoot, "composer.json")) {
		composerRoot = ""
	}
	kphpBuildDir, err := ioutil.TempDir("", "kphpcompare-build")
	if err != nil {
		return err
	}
	defer func() {
		if kphpBuildDir == "" || kphpBuildDir == "/" {
			return
		}
		if err := os.RemoveAll(kphpBuildDir); err != nil {
			log.Printf("remove temp build dir: %v", err)
		}
	}()
	buildResult, err := kphpscript.Build(kphpscript.BuildConfig{
		KPHPCommand:  kphpCommand,
		Script:       scriptName,
		ComposerRoot: composerRoot,
		OutputDir:    kphpBuildDir,
		Workdir:      workdir,
	})
	if err != nil {
		return fmt.Errorf("build kphp: %v", err)
	}
	kphpRunResult, err := kphpscript.Run(kphpscript.RunConfig{
		Executable: buildResult.Executable,
		Workdir:    workdir,
	})
	if err != nil {
		return fmt.Errorf("run kphp: %v", err)
	}
	stdoutDiff := cmp.Diff(string(phpResult.Stdout), string(kphpRunResult.Stdout))
	if stdoutDiff != "" {
		return fmt.Errorf("stdout differs (-PHP +KPHP):\n%s", stdoutDiff)
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
	fs.StringVar(&conf.KphpCommand, "kphp2cpp-binary", envString("KTEST_KPHP2CPP_BINARY", ""),
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

	conf.ComposerRoot = kenv.FindComposerRoot(conf.ProjectRoot)
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
