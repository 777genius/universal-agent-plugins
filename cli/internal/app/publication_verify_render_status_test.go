package app

import "testing"

func TestBuildNeedsSyncPublicationVerifyStatusAddsMaterializeStep(t *testing.T) {
	t.Parallel()

	status := buildNeedsSyncPublicationVerifyStatus(publicationContext{
		root:   ".",
		target: "claude",
		dest:   "/tmp/out",
	})
	if status.ready || status.label != "needs_sync" {
		t.Fatalf("status = %+v", status)
	}
	if len(status.nextSteps) != 1 {
		t.Fatalf("nextSteps = %#v", status.nextSteps)
	}
}
