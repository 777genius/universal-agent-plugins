package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func doctorFindBinary(commands []string) (string, string, error) {
	for _, command := range normalizeDoctorCommands(commands) {
		path, err := runtimecheck.LookPath(command)
		if err == nil {
			return path, command, nil
		}
	}
	return "", "", os.ErrNotExist
}

func doctorMissingLine(spec doctorToolSpec) string {
	if len(spec.Commands) == 1 {
		return spec.Label + ": missing from PATH"
	}
	return spec.Label + ": missing from PATH (checked: " + strings.Join(spec.Commands, ", ") + ")"
}

func joinDoctorRoot(root, rel string) string {
	return filepath.Join(root, rel)
}
