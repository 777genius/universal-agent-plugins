package app

import "testing"

func TestPublicationContextDestPackageRootUsesSlashAwareJoin(t *testing.T) {
	t.Parallel()

	got := publicationContextDestPackageRoot(publicationContext{
		dest:        "/tmp/out",
		packageRoot: "plugins/demo",
	})
	if got != "/tmp/out/plugins/demo" {
		t.Fatalf("destPackageRoot = %q", got)
	}
}
