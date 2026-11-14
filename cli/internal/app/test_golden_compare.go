package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type goldenAssertionValues struct {
	stdout   string
	stderr   string
	exitCode int
}

func existingGoldenFileCount(files []string) int {
	existing := 0
	for _, path := range files {
		if _, err := os.Stat(path); err == nil {
			existing++
		}
	}
	return existing
}

func readGoldenAssertions(stdoutPath, stderrPath, exitCodePath string) (goldenAssertionValues, error) {
	wantStdout, err := os.ReadFile(stdoutPath)
	if err != nil {
		return goldenAssertionValues{}, err
	}
	wantStderr, err := os.ReadFile(stderrPath)
	if err != nil {
		return goldenAssertionValues{}, err
	}
	wantExitCodeRaw, err := os.ReadFile(exitCodePath)
	if err != nil {
		return goldenAssertionValues{}, err
	}
	wantExitCode, err := strconv.Atoi(strings.TrimSpace(string(wantExitCodeRaw)))
	if err != nil {
		return goldenAssertionValues{}, fmt.Errorf("invalid exit code in %s", exitCodePath)
	}
	return goldenAssertionValues{
		stdout:   string(wantStdout),
		stderr:   string(wantStderr),
		exitCode: wantExitCode,
	}, nil
}

func compareGoldenAssertions(files []string, expected goldenAssertionValues, actual goldenAssertionValues) ([]string, []PluginTestMismatch) {
	var mismatches []string
	var mismatchInfo []PluginTestMismatch

	if expected.stdout != actual.stdout {
		mismatches = append(mismatches, "stdout")
		mismatchInfo = append(mismatchInfo, PluginTestMismatch{
			Field:           "stdout",
			GoldenFile:      files[0],
			ExpectedPreview: runtimeTestPreview(expected.stdout),
			ActualPreview:   runtimeTestPreview(actual.stdout),
		})
	}
	if expected.stderr != actual.stderr {
		mismatches = append(mismatches, "stderr")
		mismatchInfo = append(mismatchInfo, PluginTestMismatch{
			Field:           "stderr",
			GoldenFile:      files[1],
			ExpectedPreview: runtimeTestPreview(expected.stderr),
			ActualPreview:   runtimeTestPreview(actual.stderr),
		})
	}
	if expected.exitCode != actual.exitCode {
		mismatches = append(mismatches, "exit_code")
		mismatchInfo = append(mismatchInfo, PluginTestMismatch{
			Field:           "exit_code",
			GoldenFile:      files[2],
			ExpectedPreview: strconv.Itoa(expected.exitCode),
			ActualPreview:   strconv.Itoa(actual.exitCode),
		})
	}
	return mismatches, mismatchInfo
}

func formatGoldenWriteError(err error) string {
	return fmt.Sprintf("golden write failed: %v", err)
}

func formatGoldenReadError(err error) string {
	return fmt.Sprintf("golden read failed: %v", err)
}
