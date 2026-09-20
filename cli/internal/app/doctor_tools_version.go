package app

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func doctorVersion(root, path string, args []string) string {
	if len(args) == 0 {
		return ""
	}
	out, err := runtimecheck.RunCommand(root, path, args...)
	if err != nil {
		return ""
	}
	return firstDoctorOutputLine(out)
}

func firstDoctorOutputLine(out string) string {
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
