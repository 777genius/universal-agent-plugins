package vscodelocal_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Regression: the actual adapter discards Result.Receipt, so false cannot survive
// reload/absence. Existing S1 callback tests never compose NewLocal with Engine.
func TestLocalObservedReceiptRestoresBoolean(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "false", true: "true"}[enabled], func(t *testing.T) {
			f := freshLocal(t, false)
			engine := f.engine(t, true)
			installed := applyLocal(t, engine, f.request(installer.OpInstall, ""))
			b := assertLocalObservation(t, f, true)
			facts, _ := b.SelectedDelivery.LocalFacts()
			if !enabled {
				disableLocal(t, f, facts.Registration.Selector)
				profile := snapshotLocalFiles(t, f.settings)
				observed := applyLocal(t, engine, f.request(installer.OpInstall, installed.InstallationID))
				if !observed.Mutated {
					t.Fatal("first actual false observation was not persisted")
				}
				assertLocalFiles(t, profile)
				b = assertLocalObservation(t, f, false)
			}
			rawObservation, rawErr := json.Marshal(b.LocalEntryObservation)
			must(t, rawErr)
			t.Logf("RAW actual Store-loaded observation=%s", rawObservation)
			assertStableLocal(t, f, engine, installed.InstallationID, enabled)
			// A fresh real Store and fresh constructor/Engine must consume durable facts.
			loaded, err := (statev2.Store{Path: filepath.Join(f.state, "state-v2.json")}).Load()
			must(t, err)
			if !onlyLocalBinding(t, loaded).LocalEntryObservation.Equal(b.LocalEntryObservation) {
				t.Fatal("fresh Store lost observation")
			}
			a, err := vscode.NewLocal(f.config)
			must(t, err)
			f.adapter = a
			f.registry, err = clients.NewRegistry(a)
			must(t, err)
			engine = f.engine(t, true)
			absent := []byte("{\n // foreign comment\n \"foreign\": {\"number\":1e2},\n \"chat.pluginLocations\":{\"/TEST-disabled-sibling\":false,},\n \"TEST-late-foreign\":true,\n}\n")
			writeLocal(t, f.settings, absent, 0600)
			before := snapshotLocalFiles(t, f.state, f.settings)
			_, err = engine.Inspect(t.Context())
			must(t, err)
			assertLocalFiles(t, before)
			h, err := engine.Prepare(t.Context(), f.request(installer.OpRepair, installed.InstallationID))
			must(t, err)
			declined, err := engine.Apply(t.Context(), h, installer.Decision{})
			must(t, err)
			must(t, h.Close())
			if declined.Mutated {
				t.Fatal("unconfirmed repair wrote")
			}
			assertLocalFiles(t, before)
			repaired := applyLocal(t, engine, f.request(installer.OpRepair, installed.InstallationID))
			if !repaired.Mutated {
				t.Fatal("confirmed same-revision repair did not restore entry")
			}
			assertLocalObservation(t, f, enabled)
			expected := map[bool]string{true: "true", false: "false"}[enabled]
			if !strings.Contains(string(readLocal(t, f.settings)), quoteLocal(facts.Registration.Selector)+":"+expected) {
				t.Fatal("repair lost recorded boolean")
			}
			assertForeign(t, readLocal(t, f.settings))
			if !bytes.Contains(readLocal(t, f.settings), []byte(`"TEST-late-foreign":true`)) {
				t.Fatal("repair lost late foreign sibling")
			}
			assertStableLocal(t, f, engine, installed.InstallationID, enabled)
		})
	}
}

func assertLocalObservation(t *testing.T, f *localFixture, enabled bool) domain.ClientBinding {
	t.Helper()
	b := onlyLocalBinding(t, loadLocalState(t, f.state))
	if b.LocalEntryObservation == nil {
		t.Fatal("actual adapter returned no durable observation")
	}
	must(t, b.ValidateLocalEntryObservation())
	o := b.LocalEntryObservation.Facts()
	facts, _ := b.SelectedDelivery.LocalFacts()
	if o.Enabled != enabled || !reflect.DeepEqual(o.RevisionBasis, b.SelectedDelivery) || !*facts.Registration.DesiredValue || !b.SelectedDelivery.OwnsProfileEntry(b.NativeObjects) {
		t.Fatal("observed value changed desired bool, ownership or sealed basis")
	}
	if b.PendingNativeIntent != nil || b.NativeActivationAttempt != "" {
		t.Fatal("certain acknowledgement left pending intent")
	}
	raw, err := json.Marshal(o)
	must(t, err)
	t.Logf("actual durable observation enabled=%v receipt=%s basis_wire=%s", o.Enabled, o.ReceiptDigest, testDigest(raw))
	return b
}

func disableLocal(t *testing.T, f *localFixture, selector string) {
	t.Helper()
	before := readLocal(t, f.settings)
	after := strings.Replace(string(before), quoteLocal(selector)+":true", quoteLocal(selector)+":false", 1)
	after = strings.Replace(after, `"foreign":`, `"TEST-late-foreign":true,"foreign":`, 1)
	if after == string(before) || !strings.Contains(after, quoteLocal(selector)+":false") {
		t.Fatal("external edit did not disable selector")
	}
	writeLocal(t, f.settings, []byte(after), 0600)
}

func assertStableLocal(t *testing.T, f *localFixture, engine *installer.Engine, id string, enabled bool) {
	t.Helper()
	before := snapshotLocalFiles(t, filepath.Join(f.state, "state-v2.json"), f.settings)
	prior := assertLocalObservation(t, f, enabled)
	for _, op := range []installer.Operation{installer.OpInstall, installer.OpRepair} {
		t.Run(string(op), func(t *testing.T) {
			result := applyLocal(t, engine, f.request(op, id))
			if result.Mutated {
				t.Fatalf("%s repeated Save/effect", op)
			}
			after := assertLocalObservation(t, f, enabled)
			if after.UpdatedAt != prior.UpdatedAt {
				t.Fatal("repeat changed UpdatedAt")
			}
			assertLocalFiles(t, before)
			t.Logf("stable public %s: no Save, unchanged state/profile bytes/inode/mtime/UpdatedAt", op)
		})
	}
}

type localFileSnapshot struct {
	info fs.FileInfo
	body []byte
}

func snapshotLocalFiles(t *testing.T, paths ...string) map[string]localFileSnapshot {
	t.Helper()
	snapshots := map[string]localFileSnapshot{}
	for _, root := range paths {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if os.IsNotExist(walkErr) {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			var body []byte
			if info.Mode().IsRegular() {
				body, err = os.ReadFile(path)
			}
			snapshots[path] = localFileSnapshot{info, body}
			return err
		})
		must(t, err)
	}
	return snapshots
}
func assertLocalFiles(t *testing.T, before map[string]localFileSnapshot) {
	t.Helper()
	roots := make([]string, 0, len(before))
	for path := range before {
		roots = append(roots, path)
	}
	after := snapshotLocalFiles(t, roots...)
	if len(after) != len(before) {
		t.Fatal("snapshot path count changed")
	}
	for path, old := range before {
		now, ok := after[path]
		if !ok || !os.SameFile(old.info, now.info) || old.info.Mode() != now.info.Mode() || !old.info.ModTime().Equal(now.info.ModTime()) || !bytes.Equal(old.body, now.body) {
			t.Fatalf("read-only/stable path changed: %s", path)
		}
	}
}
