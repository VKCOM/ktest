package bench

import (
	"io"
)

type RunConfig struct {
	ProjectRoot  string
	ComposerRoot string
	BenchTarget  string
	Preload      string
	RunFilter    string

	ProfileDir  string
	CompileOnly bool

	Workdir string

	KphpCommand string
	PhpCommand  string

	AdditionalKphpIncludeDirs string
	DisableAutoloadForKPHP    bool
	TeamcityOutput            bool
	Benchmem                  bool
	NoJIT                     bool

	Count int

	Output     io.Writer
	DebugPrint func(string)

	NoCleanup bool
}

func Run(conf *RunConfig) error {
	r := newRunner(conf)
	return r.Run()
}
