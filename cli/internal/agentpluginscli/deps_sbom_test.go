package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func testSBOMLock() environment.Lock {
	pin := environment.LockedPackage{
		Alias: "app_pkg", Version: "1.2.3",
		Source:          environment.LockedSource{Git: &environment.LockedGitSource{Repository: "example/app", Commit: strings.Repeat("a", 40)}},
		DigestAlgorithm: environment.TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("b", 64),
	}
	transitive := environment.LockedPackage{
		Alias: "tool", Version: "2.0.0",
		Source:          environment.LockedSource{Git: &environment.LockedGitSource{Repository: "example/tool", Commit: strings.Repeat("c", 40)}},
		DigestAlgorithm: environment.TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("d", 64),
		RequiredBy: []environment.RequiredBy{{Alias: "app_pkg"}},
	}
	return environment.Lock{LockVersion: 1, ManifestDigest: "sha256:" + strings.Repeat("e", 64),
		Packages: []environment.LockedPackage{transitive, pin}}
}

func TestDependencySBOMDeterministicAndHonest(t *testing.T) {
	t.Parallel()
	lock := testSBOMLock()
	for _, kind := range []string{"cyclonedx", "spdx"} {
		t.Run(kind, func(t *testing.T) {
			first, err := marshalDependencySBOM(lock, kind)
			if err != nil {
				t.Fatal(err)
			}
			reversed := lock
			reversed.Packages = append([]environment.LockedPackage(nil), lock.Packages...)
			reversed.Packages[0], reversed.Packages[1] = reversed.Packages[1], reversed.Packages[0]
			second, err := marshalDependencySBOM(reversed, kind)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, second) {
				t.Fatalf("%s depends on input package order", kind)
			}
			if !json.Valid(first) {
				t.Fatalf("%s is not valid JSON", kind)
			}
			if bytes.Contains(first, []byte("\"alg\": \"SHA-256\"")) {
				t.Fatal("UAP canonical tree digest must not be labeled a raw SHA-256 file checksum")
			}
			if !bytes.Contains(first, []byte("example/tool")) {
				t.Fatalf("%s lost the pinned source identity", kind)
			}
			if !bytes.Contains(first, []byte("app_pkg")) {
				t.Fatalf("%s lost the direct dependency", kind)
			}
			if !bytes.Contains(first, []byte("tool")) {
				t.Fatalf("%s lost the transitive dependency", kind)
			}
		})
	}
}

func TestCycloneDXDependencyEdgeDirection(t *testing.T) {
	t.Parallel()
	encoded, err := marshalDependencySBOM(testSBOMLock(), "cyclonedx")
	if err != nil {
		t.Fatal(err)
	}
	var doc cyclonedxDocument
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.BOMFormat != "CycloneDX" || doc.SpecVersion != "1.5" || len(doc.Dependencies) != 2 {
		t.Fatalf("unexpected document: %+v", doc)
	}
	for _, dep := range doc.Dependencies {
		if dep.Ref == "urn:uap:app_pkg" && (len(dep.DependsOn) != 1 || dep.DependsOn[0] != "urn:uap:tool") {
			t.Fatalf("parent must depend on transitive package: %+v", dep)
		}
		if dep.Ref == "urn:uap:tool" && len(dep.DependsOn) != 0 {
			t.Fatalf("leaf must not depend on its parent: %+v", dep)
		}
	}
}

func TestSPDXIDsAcceptUAPUnderscoreAlias(t *testing.T) {
	t.Parallel()
	encoded, err := marshalDependencySBOM(testSBOMLock(), "spdx")
	if err != nil {
		t.Fatal(err)
	}
	var doc spdxDocument
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SPDXVersion != "SPDX-2.3" || len(doc.Packages) != 2 {
		t.Fatalf("unexpected document: %+v", doc)
	}
	if doc.Packages[0].ID != spdxPackageID("app_pkg") || strings.Contains(doc.Packages[0].ID, "_") {
		t.Fatalf("invalid SPDX package ID: %q", doc.Packages[0].ID)
	}
}

func TestDependencySBOMUnsupportedFormat(t *testing.T) {
	t.Parallel()
	if _, err := marshalDependencySBOM(testSBOMLock(), "unexpected"); err == nil {
		t.Fatal("unsupported SBOM type must fail closed")
	}
}
