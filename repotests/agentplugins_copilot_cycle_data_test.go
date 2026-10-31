//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (c *copilotProjectCycle) dataSnapshot(t *testing.T, r *copilotCycleRecipe, i domain.Installation) copilotCycleDataSnapshot {
	t.Helper()
	physical := "native-proof-" + copilotProjectConfigHash([]byte(i.InstallationID))[:12]
	id := "data_" + copilotProjectConfigHash([]byte(strings.Join([]string{i.InstallationID, physical, copilotCycleScope(r)}, "\x00")))[:24]
	path := filepath.Join(filepath.Dir(c.statePath(r)), "plugin-data", physical)
	out := copilotCycleDataSnapshot{Path: path}
	if r.Key == "user" && !r.Removed {
		if r.Binding.DataReceiptID != "" || len(i.DataReceipts) != 0 || copilotCycleAbsent(path) != nil {
			t.Fatal("Copilot skill-only User acquired unproved active data authority")
		}
		c.absent[path] = true
		return out
	}
	d := i.DataReceipts[id]
	wantDigest := "sha256:" + copilotProjectConfigHash([]byte("agentplugins-plugin-data-v1\x00"+id+"\x00"+path))
	if len(i.DataReceipts) != 1 || d.DataReceiptID != id || d.PhysicalBackend != physical || d.Scope != copilotCycleScope(r) || d.Locator != path ||
		d.State != domain.DataReceiptOwned || d.OwnershipDigest != wantDigest || r.Key != "user" && r.Binding.DataReceiptID != id {
		t.Fatal("Copilot independently derived data receipt differs")
	}
	if len(r.Installation.DataReceipts) != 0 && !reflect.DeepEqual(i.DataReceipts, r.Installation.DataReceipts) {
		t.Fatal("Copilot retained data ownership changed")
	}
	ownerPath := filepath.Join(path, ".agentplugins-data-owner.json")
	owner, err := projectNativeRuntimeRead(ownerPath, 1<<20)
	if err != nil || !copilotCycleOwner(owner, d) {
		t.Fatal("Copilot data ownership marker differs from its held receipt", err)
	}
	ownerStream := copilotCycleStream(t, owner)
	out.Allocated, out.Receipt, out.Owner = true, &d, &ownerStream
	want := map[string]string{".": fmt.Sprintf("directory:%o", os.ModeDir|0700), ".agentplugins-data-owner.json": "file:600:" + copilotProjectConfigHash(owner)}
	if r.Written {
		if r.Key == "user" {
			t.Fatal("Copilot User control cannot have a native MCP marker")
		}
		markerPath := filepath.Join(path, "native-marker.txt")
		marker, err := projectNativeRuntimeRead(markerPath, 256)
		if err != nil || !bytes.Equal(marker, []byte("retained-"+r.Nonce)) {
			t.Fatal("Copilot persistent native marker changed", err)
		}
		markerStream := copilotCycleStream(t, marker)
		out.Marker = &markerStream
		want["native-marker.txt"] = "file:600:" + copilotProjectConfigHash(marker)
		c.f.protected[markerPath] = copilotProjectConfigHash(marker)
	}
	tree, err := clineAttemptTree(path)
	if err != nil || !reflect.DeepEqual(tree, want) {
		t.Fatal("Copilot data contains an unrecorded or changed leaf", err)
	}
	out.Files = tree
	r.Data = path
	c.trees[path] = tree
	c.f.protected[ownerPath] = copilotProjectConfigHash(owner)
	for _, held := range []string{path, ownerPath} {
		if _, exists := c.held[held]; !exists {
			c.holdOriginal(t, held)
		}
	}
	return out
}

func (c *copilotProjectCycle) dataBeforeWrite(t *testing.T, r *copilotCycleRecipe) map[string]string {
	t.Helper()
	if r.Key == "user" || r.Written || r.Data == "" {
		t.Fatal("Copilot initial native write has no independent unmodified project data")
	}
	before, err := clineAttemptTree(r.Data)
	if err != nil || before["native-marker.txt"] != "" {
		t.Fatal("Copilot native write would overwrite existing data", err)
	}
	delete(c.trees, r.Data)
	return before
}

func (c *copilotProjectCycle) dataAfterWrite(t *testing.T, r *copilotCycleRecipe, before map[string]string) {
	t.Helper()
	after, err := clineAttemptTree(r.Data)
	want := map[string]string{}
	for path, digest := range before {
		want[path] = digest
	}
	want["native-marker.txt"] = "file:600:" + copilotProjectConfigHash([]byte("retained-"+r.Nonce))
	if err != nil || !reflect.DeepEqual(after, want) {
		t.Fatal("Copilot native write changed data beyond its single owned marker", err)
	}
	r.Written = true
	c.trees[r.Data] = after
	c.f.protected[filepath.Join(r.Data, "native-marker.txt")] = copilotProjectConfigHash([]byte("retained-" + r.Nonce))
}
