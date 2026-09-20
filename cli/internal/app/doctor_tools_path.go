package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func doctorPATHHint() string {
	hint := "if the runtime is installed but doctor cannot see it here, check PATH for non-interactive shells"
	if runtime.GOOS == "darwin" {
		hint += " (for example ~/.zshenv on macOS)"
	}
	if missing := doctorMissingCommonPATHDirs(); len(missing) > 0 {
		hint += "; common directories missing from PATH: " + strings.Join(missing, ", ")
	}
	return hint + "."
}

func doctorMissingCommonPATHDirs() []string {
	candidates := []string{"/usr/local/bin"}
	switch runtime.GOOS {
	case "darwin":
		candidates = append([]string{"/opt/homebrew/bin", "/usr/local/go/bin"}, candidates...)
	case "linux":
		candidates = append([]string{"/usr/local/sbin"}, candidates...)
	}

	current := map[string]struct{}{}
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		entry = strings.TrimSpace(entry)
		if entry != "" {
			current[entry] = struct{}{}
		}
	}

	var missing []string
	for _, candidate := range candidates {
		if _, ok := current[candidate]; !ok {
			missing = append(missing, candidate)
		}
	}
	return missing
}
