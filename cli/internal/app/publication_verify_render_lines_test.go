package app

import "testing"

func TestAppendPublicationVerifyNeedsSyncLinesAppendsNextSteps(t *testing.T) {
	t.Parallel()
	lines := appendPublicationVerifyNeedsSyncLines([]string{"base"}, []string{"run sync"})
	if len(lines) != 4 {
		t.Fatalf("lines = %#v", lines)
	}
	if lines[3] != "  run sync" {
		t.Fatalf("last line = %q", lines[3])
	}
}
