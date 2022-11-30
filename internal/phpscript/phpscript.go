package phpscript

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/VKCOM/ktest/internal/fileutil"
)

type RunResult struct {
	Stdout []byte
	Stderr []byte
	Time   time.Duration
}

type RunConfig struct {
	PHPCommand string
	Preload    string
	JIT        bool
	Script     string
	Workdir    string
	ScriptArgs []string
	Stdout     io.Writer
	Stderr     io.Writer
}

func Run(config RunConfig) (*RunResult, error) {
	args := []string{
		"-f", config.Script,
		"-d", "ffi.enable=preload",
		"-d", "opcache.enable=1",
		"-d", "opcache.enable_cli=1",
	}
	if config.Preload != "" {
		preloadScript := fileutil.AbsPath(config.Workdir, config.Preload)
		args = append(args,
			"-d", "opcache.preload="+preloadScript)
	}
	if config.JIT {
		args = append(args,
			"-d", "opcache.jit_buffer_size=96M",
			"-d", "opcache.jit=on")
	} else {
		args = append(args,
			"-d", "opcache.jit_buffer_size=0",
			"-d", "opcache.jit=0")
	}
	args = append(args, config.ScriptArgs...)
	runCommand := exec.Command(config.PHPCommand, args...)
	runCommand.Dir = config.Workdir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runCommand.Stdout = &stdout
	runCommand.Stderr = &stderr
	if config.Stdout != nil {
		runCommand.Stdout = io.MultiWriter(&stdout, config.Stdout)
	}
	if config.Stderr != nil {
		runCommand.Stderr = io.MultiWriter(&stderr, config.Stderr)
	}

	start := time.Now()
	runErr := runCommand.Run()
	elapsed := time.Since(start)
	result := &RunResult{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
		Time:   elapsed,
	}
	if runErr != nil {
		var combinedOutput []byte
		combinedOutput = append(combinedOutput, stdout.Bytes()...)
		combinedOutput = append(combinedOutput, stderr.Bytes()...)
		return result, fmt.Errorf("%s: %v: %s", config.PHPCommand, runErr, combinedOutput)
	}

	return result, nil
}
