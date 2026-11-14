package platformexec

import "testing"

func TestOpenCodeWorkspaceDirectoryStepsIncludesAllWorkspaceDirectories(t *testing.T) {
	t.Parallel()
	steps := openCodeWorkspaceDirectorySteps(opencodeScopeConfig{})
	if len(steps) != 6 {
		t.Fatalf("steps len = %d", len(steps))
	}
}
