package app

import "testing"

func TestWithPublicationContextVerifyUsesInspectionPublication(t *testing.T) {
	t.Parallel()
	ctx := publicationContext{
		inspection: publicationInspectionStub("demo"),
	}
	got := withPublicationContextVerify(ctx, "plugins/demo", publicationStateStub())
	if got.packageRoot != "plugins/demo" {
		t.Fatalf("packageRoot = %q", got.packageRoot)
	}
	if got.publication.Core.Name != "demo" {
		t.Fatalf("publication = %#v", got.publication)
	}
}
