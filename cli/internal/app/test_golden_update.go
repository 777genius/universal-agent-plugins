package app

import (
	"os"
	"path/filepath"
	"strconv"
)

func writeGoldenAssertions(stdoutPath, stderrPath, exitCodePath string, values goldenAssertionValues) error {
	if err := os.MkdirAll(filepath.Dir(stdoutPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(stdoutPath, []byte(values.stdout), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(stderrPath, []byte(values.stderr), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(exitCodePath, []byte(strconv.Itoa(values.exitCode)+"\n"), 0o644); err != nil {
		return err
	}
	return nil
}
