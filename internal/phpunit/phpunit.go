package phpunit

import (
	"io"
	"time"
)

type RunConfig struct {
	ProjectRoot string
	TestTarget  string
	TestArgv    []string
	SrcDir      string

	KphpCommand string

	Output     io.Writer
	DebugPrint func(string)

	NoCleanup bool
}

type RunResult struct {
	Tests      int
	Assertions int
	Failures   []TestFailure
	Time       time.Duration
}

type TestFailure struct {
	Name    string
	Reason  string
	Message string
	File    string
	Line    int
}

func Run(conf *RunConfig) (*RunResult, error) {
	startTime := time.Now()
	r := newRunner(conf)
	result, err := r.Run()
	if err != nil {
		return nil, err
	}
	result.Time = time.Since(startTime)
	return result, nil
}

type FormatConfig struct {
	PrintTime     bool
	ShortLocation bool
}

func FormatResult(w io.Writer, conf *FormatConfig, result *RunResult) {
	formatResult(w, conf, result)
}
