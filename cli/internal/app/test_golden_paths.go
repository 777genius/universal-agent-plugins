package app

import "path/filepath"

func runtimeTestGoldenPaths(goldenDir, event string) (string, string, string) {
	base := filepath.Join(goldenDir, event)
	return base + ".stdout", base + ".stderr", base + ".exitcode"
}
