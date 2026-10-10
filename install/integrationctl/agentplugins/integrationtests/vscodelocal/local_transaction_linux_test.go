//go:build linux

package vscodelocal_test

import (
	"bytes"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"golang.org/x/sys/unix"
)

// Regression: the Darwin wiring removes the existing Linux metadata gate or
// opts legacy Linux Local bindings into the new Darwin physical-owner contract.
func TestLocalLinuxMetadataContract(t *testing.T) {
	a, req, settings := localTransactionRequest(t)
	if _, opted := any(a).(clients.PhysicalProfileAuthority); opted {
		t.Fatal("Linux legacy Local contract changed")
	}
	must(t, unix.Setxattr(settings, "user.TEST-local-refusal", []byte("TEST retain"), 0))
	before := readLocal(t, settings)
	files := &localFailureIO{body: bytes.Clone(before)}
	out, err := a.Activate(t.Context(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files)}, req)
	if err == nil || out.NativeEffect != domain.NativeEffectUnchanged || len(out.NativeObjects) != 0 || out.LocalEntryObservation != nil || files.writes != 0 || !bytes.Equal(readLocal(t, settings), before) {
		t.Fatalf("Linux metadata refusal lost: %+v %v writes=%d", out, err, files.writes)
	}
	facts, _ := req.Plan.SelectedDelivery.LocalFacts()
	removed, err := a.Deactivate(t.Context(), clients.Env{NativeConfig: nativeconfig.New()}, domain.DeactivationRequest{Client: req.Client, SelectedDelivery: req.Plan.SelectedDelivery, ManagedArtifactPath: facts.Registration.Selector, NativeObjects: []domain.NativeObjectOwnership{facts.Registration.Ownership(settings)}, Confirmed: true, RemoveOwnedEntry: true})
	must(t, err)
	if !removed.ExternalRemovalComplete || !bytes.Equal(readLocal(t, settings), before) {
		t.Fatal("Linux attributed no-op removal changed bytes")
	}
}
