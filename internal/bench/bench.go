package bench

import (
	"io"
)

type RunConfig struct {
	ProjectRoot string
	BenchTarget string

	KphpCommand string
	PhpCommand  string

	Count int

	Output     io.Writer
	DebugPrint func(string)

	NoCleanup bool
}

func Run(conf *RunConfig) error {
	r := newRunner(conf)
	return r.Run()
}
