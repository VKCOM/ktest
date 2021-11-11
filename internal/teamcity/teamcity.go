package teamcity

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type Logger struct {
	writer io.Writer
}

func NewLogger(writer io.Writer) *Logger {
	return &Logger{writer: writer}
}

func (l *Logger) TestSuiteStarted(name string) {
	fmt.Fprintf(l.writer, "##teamcity[testSuiteStarted name='%s']\n", strings.TrimPrefix(name, "Benchmark"))
}

func (l *Logger) TestSuiteFinished(name string, duration time.Duration) {
	fmt.Fprintf(l.writer, "##teamcity[testSuiteFinished name='%s' duration='%s']\n", strings.TrimPrefix(name, "Benchmark"), duration.String())
}

func (l *Logger) TestStarted(name string) {
	fmt.Fprintf(l.writer, "##teamcity[testStarted name='%s']\n", strings.TrimPrefix(name, "Benchmark"))
}

func (l *Logger) TestFinished(name string) {
	fmt.Fprintf(l.writer, "##teamcity[testFinished name='%s']\n", strings.TrimPrefix(name, "Benchmark"))
}
