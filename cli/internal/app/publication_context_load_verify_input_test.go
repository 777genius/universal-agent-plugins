package app

import "testing"

func TestBuildVerifyPublicationContextInputPreservesPackageRoot(t *testing.T) {
	t.Parallel()
	input := buildVerifyPublicationContextInput("plugins/demo", publicationStateStub())
	if input.packageRoot != "plugins/demo" {
		t.Fatalf("packageRoot = %q", input.packageRoot)
	}
}
