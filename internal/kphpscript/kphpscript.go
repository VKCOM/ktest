package kphpscript

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"
)

type BuildConfig struct {
	KPHPCommand  string
	Script       string
	ComposerRoot string
	OutputDir    string
	Workdir      string
}

type BuildResult struct {
	Executable string
}

type RunConfig struct {
	Executable string
	Workdir    string
	Stdout     io.Writer
	Stderr     io.Writer
}

type RunResult struct {
	Stdout []byte
	Stderr []byte
	Time   time.Duration
}

func Build(config BuildConfig) (*BuildResult, error) {
	args := []string{
		"--mode", "cli",
		"--destination-directory", config.OutputDir,
		"--enable-ffi",
	}
	if config.ComposerRoot != "" {
		args = append(args, "--composer-root", config.ComposerRoot)
	}
	args = append(args, config.Script)
	buildCommand := exec.Command(config.KPHPCommand, args...)
	buildCommand.Dir = config.Workdir
	out, err := buildCommand.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %v: %s", config.KPHPCommand, err, out)
	}
	result := &BuildResult{
		Executable: filepath.Join(config.OutputDir, "cli"),
	}
	return result, nil
}

func Run(config RunConfig) (*RunResult, error) {
	args := []string{"--Xkphp-options", "--disable-sql"}
	runCommand := exec.Command(config.Executable, args...)
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
		return result, fmt.Errorf("%s: %v: %s", config.Executable, runErr, combinedOutput)
	}

	return result, nil
}
