package domain

import (
	"bytes"
	"testing"
)

func TestInstallationAndBindingIdentitiesAreIndependentFromDeclaredName(t *testing.T) {
	t.Parallel()
	first, err := newInstallationID(bytes.NewReader(make([]byte, 16)))
	if err != nil {
		t.Fatal(err)
	}
	secondBytes := make([]byte, 16)
	secondBytes[15] = 1
	second, err := newInstallationID(bytes.NewReader(secondBytes))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("installation IDs collided")
	}
	firstSource := ComputeSourceBindingID(SourceIdentity{CanonicalSource: "https://example.com/one", PackageSubpath: "plugins/demo"})
	secondSource := ComputeSourceBindingID(SourceIdentity{CanonicalSource: "https://example.com/two", PackageSubpath: "plugins/demo"})
	if firstSource == secondSource {
		t.Fatal("different sources received the same binding ID")
	}
	if ComputePhysicalArtifactID("demo", first) == ComputePhysicalArtifactID("demo", second) {
		t.Fatal("duplicate declared names received the same physical artifact ID")
	}
}
