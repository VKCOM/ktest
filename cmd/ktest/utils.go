package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func printProgress(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "\033[2K\r%s", msg)
}

func flushProgress() {
	printProgress("")
}

func runBenchWithProgress(label, command string, args []string) ([]byte, error) {
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
