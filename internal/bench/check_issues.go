package bench

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/VKCOM/ktest/internal/fileutil"
)

func checkIssues() []string {
	var issues []string

	cpuBoostVariants := []struct {
		path    string
		enabled string
	}{
		{"/sys/devices/system/cpu/intel_pstate/no_turbo", "0"},
		{"/sys/devices/system/cpu/cpufreq/boost", "1"},
	}

	for _, boost := range cpuBoostVariants {
		if fileutil.FileExists(boost.path) {
			data, err := ioutil.ReadFile(boost.path)
			if err == nil && strings.TrimSpace(string(data)) == boost.enabled {
				issues = append(issues, fmt.Sprintf("cpu boost is not disabled (%s)", boost.path))
				break
			}
		}
	}

	return issues
}
