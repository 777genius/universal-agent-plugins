package app

import "testing"

func TestExecutePublicationMaterializeDryRunReturnsResult(t *testing.T) {
	t.Parallel()

	result, err := executePublicationMaterialize(publicationContext{
		target:      "codex-package",
		dest:        "/tmp/out",
		packageRoot: "plugins/demo",
		channel:     publicationChannelStub("codex-marketplace"),
	}, publicationMaterializePlan{
		packageRootAction:  "create",
		catalogArtifactAct: "create",
	}, true)
	if err != nil {
		t.Fatalf("executePublicationMaterialize: %v", err)
	}
	if result.Mode != "dry-run" {
		t.Fatalf("result = %+v", result)
	}
}
