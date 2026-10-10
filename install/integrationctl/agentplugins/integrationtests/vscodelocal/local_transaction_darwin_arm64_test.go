//go:build darwin && arm64 && nativequalification

package vscodelocal_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"golang.org/x/sys/unix"
)

// Regression: Local bypasses the held backend's metadata refusal by granting
// writes from a pathname check; or rejects planning/no-op on an attributed file.
func TestLocalDarwinHeldMetadataRefusalAndNoop(t *testing.T) {
	a, req, settings := localTransactionRequest(t)
	must(t, unix.Setxattr(settings, "com.example.TEST-local-refusal", []byte("TEST retain"), 0))
	before := readLocal(t, settings)
	out, err := a.Activate(t.Context(), clients.Env{NativeConfig: nativeconfig.New()}, req)
	if err == nil || out.NativeEffect != domain.NativeEffectUncertain || len(out.NativeObjects) != 0 || out.LocalEntryObservation != nil || !bytes.Equal(readLocal(t, settings), before) {
		t.Fatalf("attributed mutation bypassed held refusal: %+v %v", out, err)
	}
	// A recorded absent owned removal is unchanged: metadata refusal must not
	// run Apply or manufacture an effect just to perform locked planning.
	facts, _ := req.Plan.SelectedDelivery.LocalFacts()
	removed, err := a.Deactivate(t.Context(), clients.Env{NativeConfig: nativeconfig.New()}, domain.DeactivationRequest{Client: req.Client, SelectedDelivery: req.Plan.SelectedDelivery, ManagedArtifactPath: facts.Registration.Selector, NativeObjects: []domain.NativeObjectOwnership{facts.Registration.Ownership(settings)}, Confirmed: true, RemoveOwnedEntry: true})
	must(t, err)
	if !removed.ExternalRemovalComplete || !bytes.Equal(readLocal(t, settings), before) {
		t.Fatal("no-op removal changed attributed profile")
	}
}

// Regression: an opted-in existing binding rediscovers/recaptures a profile,
// uses today's constructor, or allows missing/foreign client authority.
func TestLocalDarwinBoundPhysicalAuthority(t *testing.T) {
	a, req, settings := localTransactionRequest(t)
	port, ok := any(a).(clients.PhysicalProfileAuthority)
	if !ok {
		t.Fatal("Local Darwin physical authority capability missing")
	}
	for _, client := range []domain.DetectedClient{{ClientID: domain.ClientCursor, ConfigRoot: req.Client.ConfigRoot}, {ClientID: domain.ClientVSCode, ConfigRoot: filepath.Dir(req.Client.ConfigRoot)}} {
		token, err := port.CaptureProfileAuthority(t.Context(), client)
		if err == nil || !token.IsZero() {
			t.Fatal("foreign new-owner capture admitted")
		}
	}
	if err := port.RevalidateProfileAuthority(t.Context(), domain.ClientVSCode, domain.ProfileAuthority{}); err == nil {
		t.Fatal("missing bound token accepted")
	}
	token, err := port.CaptureProfileAuthority(t.Context(), req.Client)
	must(t, err)
	if token.IsZero() || token.Facts().CanonicalRoot != req.Client.ConfigRoot {
		t.Fatal("selected root not frozen")
	}
	if err := port.RevalidateProfileAuthority(t.Context(), domain.ClientCursor, token); err == nil {
		t.Fatal("foreign client token accepted")
	}
	other, _, otherSettings := localTransactionRequest(t)
	must(t, os.Remove(otherSettings))
	// Revalidation uses the existing token, never the other constructor/file.
	must(t, other.RevalidateProfileAuthority(t.Context(), domain.ClientVSCode, token))
	before := readLocal(t, settings)
	oldRoot := req.Client.ConfigRoot + "-TEST-old"
	must(t, os.Rename(req.Client.ConfigRoot, oldRoot))
	t.Cleanup(func() { _ = os.RemoveAll(oldRoot) })
	writeLocal(t, settings, before, 0600)
	if err := other.RevalidateProfileAuthority(t.Context(), domain.ClientVSCode, token); err == nil {
		t.Fatal("same-byte root replacement recaptured/granted authority")
	}
	if !bytes.Equal(readLocal(t, settings), before) {
		t.Fatal("revalidation mutated profile")
	}
}
