package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Exercise the public CLI against exactly the same verified immutable store as
// deps check. The fixture panics on all source acquisition and client probes.
func TestDependencySBOMOfflineCLI(t *testing.T) {
	f := newDependencyCheckFixture(t)
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	before := observeDependencyCheckFiles(t, f.root)
	for _, kind := range []string{"cyclonedx", "spdx"} {
		first, stderr, err := f.execute(false, "deps", "sbom", "--type", kind)
		if err != nil {
			t.Fatalf("offline %s SBOM: %v\n%s", kind, err, stderr)
		}
		second, _, err := f.execute(false, "deps", "sbom", "--type", kind)
		if err != nil || first != second {
			t.Fatalf("%s SBOM changed across identical locked replay: %v", kind, err)
		}
		if !json.Valid([]byte(first)) {
			t.Fatalf("%s SBOM is not JSON: %s", kind, first)
		}
		if !strings.Contains(first, "demo") || !strings.Contains(first, pin.TreeDigest) {
			t.Fatalf("%s SBOM lost verified package identity", kind)
		}
	}
	assertDependencyCheckFilesUnchanged(t, f.root, before)
}

func TestDependencySBOMRefusesMissingCacheAndUnknownFormat(t *testing.T) {
	f := newDependencyCheckFixture(t)
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	if _, _, err := f.execute(false, "deps", "sbom", "--type", "invalid"); err == nil {
		t.Fatal("unsupported SBOM format must return nonzero")
	}
	if err := os.RemoveAll(f.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	before := observeDependencyCheckFiles(t, f.root)
	if stdout, _, err := f.execute(false, "deps", "sbom", "--type", "spdx"); err == nil || strings.Contains(stdout, "SPDXRef-DOCUMENT") {
		t.Fatalf("missing immutable package must not produce a successful SBOM: %v %s", err, stdout)
	}
	assertDependencyCheckFilesUnchanged(t, f.root, before)
}

func TestDependencyPolicyDenialOccursBeforeSyncMutation(t *testing.T) {
	f := newDependencyCheckFixture(t)
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	policy := filepath.Join(f.root, "deny-sources.yml")
	if err := os.WriteFile(policy, []byte("schema_version: 1\nsources:\n  deny_hosts: [github.com]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := observeDependencyCheckFiles(t, f.root)
	stdout, stderr, err := f.execute(false, "deps", "sync", "--locked", "--yes", "--policy", policy)
	if err == nil || !strings.Contains(err.Error(), "local dependency policy") {
		t.Fatalf("denied dependency reached sync: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	assertDependencyCheckFilesUnchanged(t, f.root, before)
}
