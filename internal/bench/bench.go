package bench

import (
	"io"
)

type RunConfig struct {
	ProjectRoot string
	BenchTarget string

	KphpCommand string
	PhpCommand  string

	AdditionalKphpIncludeDirs string
	DisableAutoloadForKPHP    bool
	TeamcityOutput            bool

	Count int

	Output     io.Writer
	DebugPrint func(string)

	NoCleanup bool
}

func Run(conf *RunConfig) error {
	r := newRunner(conf)
	return r.Run()
}
