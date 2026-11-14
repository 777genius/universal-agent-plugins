package app

import (
	"fmt"
)

func processGoldenAssertions(goldenDir, event, stdout, stderr string, exitCode int, update bool) (string, []string, []string, []PluginTestMismatch, string) {
	stdoutPath, stderrPath, exitCodePath := runtimeTestGoldenPaths(goldenDir, event)
	files := []string{stdoutPath, stderrPath, exitCodePath}
	if update {
		if err := writeGoldenAssertions(stdoutPath, stderrPath, exitCodePath, goldenAssertionValues{
			stdout:   stdout,
			stderr:   stderr,
			exitCode: exitCode,
		}); err != nil {
			return "mismatch", files, nil, nil, formatGoldenWriteError(err)
		}
		return "updated", files, nil, nil, ""
	}

	existing := existingGoldenFileCount(files)
	switch existing {
	case 0:
		return "not_configured", files, nil, nil, ""
	case len(files):
	default:
		return "mismatch", files, []string{"golden_files"}, []PluginTestMismatch{{
			Field:           "golden_files",
			ExpectedPreview: "stdout/stderr/exitcode goldens must all exist",
			ActualPreview:   fmt.Sprintf("%d of %d files present", existing, len(files)),
		}}, "golden files are partially configured"
	}

	values, err := readGoldenAssertions(stdoutPath, stderrPath, exitCodePath)
	if err != nil {
		return "mismatch", files, nil, nil, formatGoldenReadError(err)
	}
	mismatches, mismatchInfo := compareGoldenAssertions(files, values, goldenAssertionValues{
		stdout:   stdout,
		stderr:   stderr,
		exitCode: exitCode,
	})
	if len(mismatches) == 0 {
		return "matched", files, nil, nil, ""
	}
	return "mismatch", files, mismatches, mismatchInfo, ""
}
