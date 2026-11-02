package environmentapp_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environmentapp"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestIntentChecksAbsentAndChangedPreimages(t *testing.T) {
	host, _, change := applicationFixture()
	delete(host.files, "manifest")
	change.Intent.ManifestBytes = nil
	if err := change.Intent.Check(host); err != nil {
		t.Fatal("absent reviewed manifest refused", err)
	}
	host.files["manifest"] = []byte("new external manifest")
	if err := change.Intent.Check(host); !errors.Is(err, installer.ErrPlanChanged) {
		t.Fatal("externally created manifest accepted", err)
	}
	delete(host.files, "manifest")
	delete(host.files, "lock")
	if err := change.Intent.Check(host); !errors.Is(err, installer.ErrPlanChanged) {
		t.Fatal("deleted reviewed lock accepted", err)
	}
}

func TestIntentPreservesByteIdenticalFiles(t *testing.T) {
	host, _, change := applicationFixture()
	change.Intent.CandidateManifest = append([]byte{}, change.Intent.ManifestBytes...)
	change.Intent.CandidateLock = append([]byte{}, change.Intent.LockBytes...)
	if change.Intent.Pending() {
		t.Fatal("byte-identical candidate required publication")
	}
	if err := change.Intent.Publish(host); err != nil || host.writes != 0 {
		t.Fatal("byte-identical file was replaced", host.trace, err)
	}
}

func TestSelectedLockKeepsClosureMetadataAndOriginalGraph(t *testing.T) {
	_, _, change := applicationFixture()
	full := environmentapp.SelectedLock(change.Lock, nil)
	if !reflect.DeepEqual(full, change.Lock) {
		t.Fatal("unselected materialization changed graph", full)
	}
	selected := environmentapp.SelectedLock(change.Lock, []string{"b", "b", "unknown"})
	if selected.ManifestDigest != change.Lock.ManifestDigest || selected.LockVersion != change.Lock.LockVersion || !reflect.DeepEqual(selected.Packages, []environment.LockedPackage{{Alias: "b"}}) || len(change.Lock.Packages) != 3 {
		t.Fatal("selection modified original or materialized unrelated packages", selected)
	}
	if missing := environmentapp.SelectedLock(change.Lock, []string{"removed"}); len(missing.Packages) != 0 {
		t.Fatal("removed alias retained unrelated materialization", missing)
	}
}
