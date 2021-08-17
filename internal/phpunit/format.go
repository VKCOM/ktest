package phpunit

import (
	"fmt"
	"io"
	"path/filepath"
)

func formatResult(w io.Writer, conf *FormatConfig, result *RunResult) {
	if conf.PrintTime {
		fmt.Fprintf(w, "\nTime: %s\n\n", result.Time)
	} else {
		fmt.Fprint(w, "\n")
	}

	if len(result.Failures) != 0 {
		if len(result.Failures) == 1 {
			fmt.Fprintf(w, "There was 1 failure:\n\n")
		} else {
			fmt.Fprintf(w, "There were %d failures:\n\n", len(result.Failures))
		}

		for i, failure := range result.Failures {
			fmt.Fprintf(w, "%d) %s\n", i+1, failure.Name)
			if failure.Message != "" {
				fmt.Fprintf(w, "%s\n", failure.Message)
			}
			fmt.Fprintf(w, "%s.\n\n", failure.Reason)
			if conf.ShortLocation {
				fmt.Fprintf(w, "%s:%d\n\n", filepath.Base(failure.File), failure.Line)
			} else {
				fmt.Fprintf(w, "%s:%d\n\n", failure.File, failure.Line)
			}
		}
		fmt.Fprintln(w, "FAILURES!")
		fmt.Fprintf(w, "Tests: %d, Assertions: %d, Failures: %d.\n",
			result.Tests, result.Assertions, len(result.Failures))
	} else {
		fmt.Fprintf(w, "OK (%d tests, %d assertions)\n",
			result.Tests, result.Assertions)
	}
}
