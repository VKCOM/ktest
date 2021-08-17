package phpunit

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type testFileResult struct {
	finished bool
	asserts  int
	failures []TestFailure
}

func parseTestOutput(f *testFile, output []byte) (*testFileResult, error) {
	res := &testFileResult{}

	var currentTest string
	for i, line := range bytes.Split(output, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var fields []interface{}
		if err := json.Unmarshal(line, &fields); err != nil {
			return nil, fmt.Errorf("output line %d: %s: %v", i+1, line, err)
		}
		if len(fields) == 0 {
			return nil, fmt.Errorf("output line %d: %s: empty fields", i+1, line)
		}
		op := fields[0].(string)
		switch op {
		case "START":
			currentTest = fields[1].(string)
		case "ASSERT_OK":
			res.asserts++
		case "FINISHED":
			res.finished = true
		case "ASSERT_EQUALS_FAILED":
			res.asserts++
			expected := fields[1]
			actual := fields[2]
			message := fields[3].(string)
			line := fields[4].(float64)
			reason := fmt.Sprintf("Failed asserting that %s matches expected %s",
				jsonString(actual), jsonString(expected))
			res.failures = append(res.failures, TestFailure{
				Name:    f.info.ClassName + "::" + currentTest,
				Reason:  reason,
				Message: message,
				File:    f.fullName,
				Line:    int(line),
			})
		case "ASSERT_NOT_EQUALS_FAILED":
			res.asserts++
			expected := fields[1]
			actual := fields[2]
			message := fields[3].(string)
			line := fields[4].(float64)
			reason := fmt.Sprintf("Failed asserting that %s is not equal to %s",
				jsonString(actual), jsonString(expected))
			res.failures = append(res.failures, TestFailure{
				Name:    f.info.ClassName + "::" + currentTest,
				Reason:  reason,
				Message: message,
				File:    f.fullName,
				Line:    int(line),
			})
		case "ASSERT_BOOL_FAILED":
			res.asserts++
			expected := fields[1]
			actual := fields[2]
			message := fields[3].(string)
			line := fields[4].(float64)
			reason := fmt.Sprintf("Failed asserting that %s is %s", jsonString(actual), expected)
			res.failures = append(res.failures, TestFailure{
				Name:    f.info.ClassName + "::" + currentTest,
				Reason:  reason,
				Message: message,
				File:    f.fullName,
				Line:    int(line),
			})
		case "ASSERT_NOT_SAME_FAILED":
			res.asserts++
			expected := fields[1]
			actual := fields[2]
			message := fields[3].(string)
			line := fields[4].(float64)
			reason := fmt.Sprintf("Failed asserting that %s is not identical to %s",
				jsonString(actual), jsonString(expected))
			res.failures = append(res.failures, TestFailure{
				Name:    f.info.ClassName + "::" + currentTest,
				Reason:  reason,
				Message: message,
				File:    f.fullName,
				Line:    int(line),
			})
		case "ASSERT_SAME_FAILED":
			res.asserts++
			expected := fields[1]
			actual := fields[2]
			message := fields[3].(string)
			line := fields[4].(float64)
			reason := fmt.Sprintf("Failed asserting that %s is identical to %s",
				jsonString(actual), jsonString(expected))
			res.failures = append(res.failures, TestFailure{
				Name:    f.info.ClassName + "::" + currentTest,
				Reason:  reason,
				Message: message,
				File:    f.fullName,
				Line:    int(line),
			})
		default:
			return nil, fmt.Errorf("output line %d: %s: unexpected op %s", i+1, line, op)
		}
	}

	return res, nil
}
