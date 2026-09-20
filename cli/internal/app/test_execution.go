package app

import (
	"context"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func runRuntimeTestCase(ctx context.Context, root string, project runtimecheck.Project, opts PluginTestOptions, support runtimeTestSupport) PluginTestCase {
	fixturePath := resolveFixturePath(root, opts.Fixture, support.Platform, support.Event)
	tc := PluginTestCase{
		Platform:    support.Platform,
		Event:       support.Event,
		FixturePath: fixturePath,
		Carrier:     support.Carrier,
		GoldenDir:   resolveGoldenDir(root, opts.GoldenDir, support.Platform),
	}

	payload, err := os.ReadFile(fixturePath)
	if err != nil {
		tc.Failure = fmt.Sprintf("fixture read failed: %v", err)
		return tc
	}

	args, stdin, err := runtimeTestInvocation(project.LauncherPath, support.Event, support.Carrier, payload, support.Platform)
	if err != nil {
		tc.Failure = err.Error()
		return tc
	}
	tc.Command = append([]string(nil), args...)

	stdout, stderr, exitCode, execErr := executeRuntimeTestCommand(ctx, root, args, stdin)
	if execErr != nil {
		tc.Failure = execErr.Error()
		return tc
	}
	tc.Stdout = stdout
	tc.Stderr = stderr
	tc.ExitCode = exitCode

	status, files, mismatches, mismatchInfo, failure := processGoldenAssertions(tc.GoldenDir, support.Event, stdout, stderr, exitCode, opts.UpdateGolden)
	tc.GoldenStatus = status
	tc.GoldenFiles = files
	tc.Mismatches = mismatches
	tc.MismatchInfo = mismatchInfo
	tc.Failure = failure
	switch status {
	case "updated":
		tc.Passed = failure == "" && len(mismatches) == 0
	case "not_configured":
		tc.Passed = failure == "" && exitCode == 0
	default:
		tc.Passed = failure == "" && len(mismatches) == 0
	}
	return tc
}
