package app

import "testing"

func TestBuildMaterializePublicationContextInputPreservesPackageRoot(t *testing.T) {
	t.Parallel()
	input := buildMaterializePublicationContextInput(materializePublicationPolicyInput{}, "plugins/demo")
	if input.packageRoot != "plugins/demo" {
		t.Fatalf("packageRoot = %q", input.packageRoot)
	}
}
