package app

import "testing"

func TestMaterializePublicationContextInputUsesResolvedPackageRoot(t *testing.T) {
	t.Parallel()
	input := materializePublicationContextInput{packageRoot: "plugins/demo"}
	if input.packageRoot != "plugins/demo" {
		t.Fatalf("packageRoot = %q", input.packageRoot)
	}
}
