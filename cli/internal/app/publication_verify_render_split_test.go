package app

import "testing"

func TestBuildPublicationVerifyRootStatusReadyHasNoNextSteps(t *testing.T) {
	t.Parallel()
	status := buildPublicationVerifyRootStatus(publicationContext{}, publicationVerifyPlan{})
	if !status.ready || status.label != "ready" {
		t.Fatalf("status = %#v", status)
	}
	if len(status.nextSteps) != 0 {
		t.Fatalf("nextSteps = %#v", status.nextSteps)
	}
}
