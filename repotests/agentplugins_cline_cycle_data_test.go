//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (c *clineProjectCycle) ownedDataReceipt(r *clineCycleRecipe, i domain.Installation, id, physical string) (domain.DataReceipt, error) {
	wantPhysical := clineCycleName + "-" + clineAttemptSHA([]byte(i.InstallationID))[:12]
	wantID := "data_" + clineAttemptSHA([]byte(strings.Join([]string{i.InstallationID, wantPhysical, scopeName(r)}, "\x00")))[:24]
	locator := filepath.Join(filepath.Dir(c.statePath(r)), "plugin-data", wantPhysical)
	d := i.DataReceipts[id]
	wantDigest := "sha256:" + clineAttemptSHA([]byte("agentplugins-plugin-data-v1\x00"+wantID+"\x00"+locator))
	if i.InstallationID == "" || physical != wantPhysical || len(i.DataReceipts) != 1 || id != wantID || d.DataReceiptID != id || d.PhysicalBackend != wantPhysical || d.Scope != scopeName(r) || d.Locator != locator || d.State != domain.DataReceiptOwned || d.OwnershipDigest != wantDigest {
		return domain.DataReceipt{}, fmt.Errorf("Cline cycle exact independently computed owned data receipt differs")
	}
	return d, nil
}

func (c *clineProjectCycle) userUnallocatedData(r *clineCycleRecipe, i domain.Installation, b domain.ClientBinding) (string, error) {
	physical := clineCycleName + "-" + clineAttemptSHA([]byte(i.InstallationID))[:12]
	if r.Scope != "user" || i.InstallationID == "" || b.PhysicalArtifact != physical || b.DataReceiptID != "" || len(i.DataReceipts) != 0 {
		return "", fmt.Errorf("fresh Cline skill-only user package must have zero data receipts and no binding token")
	}
	// delivery.packageNeedsPluginData is false for this independently authored
	// skill-only source; group_stage therefore cannot allocate active user data.
	path := filepath.Join(filepath.Dir(c.statePath(r)), "plugin-data", physical)
	if err := projectNativeRuntimeParents(path); err != nil {
		return "", fmt.Errorf("unallocated Cline user data parent differs")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return "", fmt.Errorf("unallocated Cline user data path already exists or is unobservable")
	}
	return path, nil
}

func (c *clineProjectCycle) nativeDataBefore(t *testing.T, r *clineCycleRecipe, q *clineProjectProvider) map[string]string {
	t.Helper()
	root := filepath.Join(filepath.Dir(c.statePath(r)), "plugin-data")
	before, err := clineAttemptTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if q.operation == "write" {
		if r.Data == "" {
			t.Fatal("native write has no prior owned data receipt")
		}
		// Only one exact leaf may be added; all immutable per-file guards remain.
		delete(c.f.trees, r.Data)
	}
	return before
}
func (c *clineProjectCycle) nativeDataAfter(t *testing.T, r *clineCycleRecipe, q *clineProjectProvider, before map[string]string) {
	t.Helper()
	root := filepath.Join(filepath.Dir(c.statePath(r)), "plugin-data")
	after, err := clineAttemptTree(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for path, value := range before {
		want[path] = value
	}
	if q.operation == "write" {
		path, err := filepath.Rel(root, filepath.Join(r.Data, "native-marker.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := want[path]; exists {
			t.Fatal("first native data write would overwrite existing marker")
		}
		want[path] = "file:600:" + clineAttemptSHA([]byte(q.marker))
	}
	if !reflect.DeepEqual(after, want) {
		t.Fatal("native read/empty/write changed data outside its single authorized marker leaf")
	}
	if r.Data != "" {
		tree, err := clineAttemptTree(r.Data)
		if err != nil {
			t.Fatal(err)
		}
		c.f.trees[r.Data] = tree
	}
}
