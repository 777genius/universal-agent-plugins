package app

import "testing"

func TestBuildPublicationVerifyRootEnvelopePreservesStatusAndNextSteps(t *testing.T) {
	t.Parallel()
	result := buildPublicationVerifyRootResultEnvelope(
		publicationContext{dest: "/tmp/out", packageRoot: "plugins/demo"},
		publicationVerifyPlan{catalogRel: ".claude-plugin/marketplace.json"},
		publicationVerifyStatus{ready: false, label: "needs_sync", nextSteps: []string{"run sync"}},
		[]string{"line"},
	)
	if result.Status != "needs_sync" || result.Ready {
		t.Fatalf("result = %+v", result)
	}
	if len(result.NextSteps) != 1 || result.NextSteps[0] != "run sync" {
		t.Fatalf("nextSteps = %#v", result.NextSteps)
	}
}
