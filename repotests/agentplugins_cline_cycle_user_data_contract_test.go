//go:build linux

package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func TestClineCycleFreshUserBlankDataTokenRequiresExactZeroReceiptAndAbsence(t *testing.T) {
	c := clineCycleInert(t)
	clineCycleStageActual(t, c, "user")
	r := c.recipes["user"]
	i, b := r.Installation, r.Binding
	path, err := c.userUnallocatedData(r, i, b)
	if err != nil || b.DataReceiptID != "" || len(i.DataReceipts) != 0 || !c.f.absent[path] {
		t.Fatal("actual skill-only user allocation/absence differs", err)
	}
	for _, kind := range []string{"token", "foreign-receipt", "extra-receipts"} {
		wrong, token := i, b
		switch kind {
		case "token":
			token.DataReceiptID = "foreign"
		case "foreign-receipt":
			wrong.DataReceipts = map[string]domain.DataReceipt{"foreign": {DataReceiptID: "foreign", Locator: "/outside", Scope: "user", State: domain.DataReceiptOwned}}
		case "extra-receipts":
			wrong.DataReceipts = map[string]domain.DataReceipt{"one": {DataReceiptID: "one"}, "two": {DataReceiptID: "two"}}
		}
		if actual, err := c.userUnallocatedData(r, wrong, token); err == nil || actual != "" {
			t.Fatal("blank user token authorized foreign/extra data", kind)
		}
	}
	for _, kind := range []string{"directory", "file", "symlink"} {
		switch kind {
		case "directory":
			err = os.Mkdir(path, 0700)
		case "file":
			err = os.WriteFile(path, []byte("unowned"), 0600)
		case "symlink":
			err = os.Symlink(t.TempDir(), path)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.userUnallocatedData(r, i, b); err == nil {
			t.Fatal("unallocated user data adopted an existing path", kind)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	c.f.check(t)
}
func TestClineCycleRetainedUserDataBindsActualManagerWithoutLegacyBindingToken(t *testing.T) {
	c := clineCycleInert(t)
	clineCycleStageActual(t, c, "user")
	r := c.recipes["user"]
	i := r.Installation
	path, err := c.userUnallocatedData(r, i, r.Binding)
	if err != nil {
		t.Fatal(err)
	}
	delete(c.f.absent, path)
	d, _, err := (providers.PluginDataManager{Base: filepath.Dir(path)}).EnsureData(c.f.ctx, i.InstallationID, r.Binding.PhysicalArtifact, "user")
	if err != nil {
		t.Fatal(err)
	}
	i.DataReceipts = map[string]domain.DataReceipt{d.DataReceiptID: d}
	if r.Binding.DataReceiptID != "" {
		t.Fatal("inert legacy-user binding unexpectedly got a data token")
	}
	c.dataProof(t, r, i, d.DataReceiptID, r.Binding.PhysicalArtifact)
	c.f.check(t)
	for _, kind := range []string{"extra", "foreign-path", "foreign-scope", "wrong-id", "wrong-backend", "wrong-ownership"} {
		wrong := i
		receipt := d
		id := d.DataReceiptID
		switch kind {
		case "extra":
			wrong.DataReceipts = map[string]domain.DataReceipt{id: d, "foreign": {DataReceiptID: "foreign"}}
		case "foreign-path":
			receipt.Locator = "/outside"
		case "foreign-scope":
			receipt.Scope = "project"
		case "wrong-id":
			id = "foreign"
		case "wrong-backend":
			receipt.PhysicalBackend = "foreign"
		case "wrong-ownership":
			receipt.OwnershipDigest = "sha256:foreign"
		}
		if kind != "extra" {
			wrong.DataReceipts = map[string]domain.DataReceipt{id: receipt}
		}
		if _, err := c.ownedDataReceipt(r, wrong, id, r.Binding.PhysicalArtifact); err == nil {
			t.Fatal("blank retained binding token authorized foreign/extra receipt", kind)
		}
	}
}
