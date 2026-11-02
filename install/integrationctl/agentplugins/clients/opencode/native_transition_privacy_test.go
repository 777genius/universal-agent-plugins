package opencode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// This exercises the real staged tree, durable journal writer/reader and fresh
// recovery authority. Windows mode bits alone made both cases fail before ACL
// privacy support. No client process or real user project is involved.
func TestNativeTransitionPrivacyReopensDurableJournal(t *testing.T) {
	for _, committed := range []bool{false, true} {
		name := "prepared"
		if committed {
			name = "native_committed"
		}
		t.Run(name, func(t *testing.T) {
			fixture, root := privateTransitionJournalFixture(t, committed)
			if _, err := readTransitionRecord(root); err != nil {
				t.Fatal("reopen durable journal", err)
			}
			reopened := freshTransitions(fixture)
			if err := reopened.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := freshTransitions(fixture).Recover(context.Background()); err != nil {
				t.Fatal("idempotent fresh recovery", err)
			}
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := state.Installations[0].Clients[fixture.request.NativeAttempt.BindingID]
			if binding.NativeActivationAttempt != "" {
				t.Fatal("recovery left an unresolved native attempt")
			}
			wantCodec, wantSkill := nativeconfig.CodecOpenCode, "source skill\n"
			if committed {
				wantCodec, wantSkill = nativeconfig.CodecOpenCodeV2, "target skill\n"
			}
			codec, _, _ := nativeconfig.OpenCodeCodecForKind(binding.NativeObjects[0].Kind)
			if codec != wantCodec {
				t.Fatalf("recovered codec = %s, want %s", codec, wantCodec)
			}
			if err := VerifyOpenCodeNativeObjects(fixture.request.Client.ConfigRoot, "", binding.NativeObjects, nativeconfig.New(), false); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(filepath.Join(fixture.request.Client.ConfigRoot, "skills", "skill", "SKILL.md"))
			if err != nil || string(body) != wantSkill {
				t.Fatalf("recovered skill = %q, want %q: %v", body, wantSkill, err)
			}
			if _, err := os.Lstat(root); !os.IsNotExist(err) {
				t.Fatal("settled journal was retained", err)
			}
		})
	}
}

func privateTransitionJournalFixture(t *testing.T, commit bool) (transitionFixtureData, string) {
	t.Helper()
	f := providerTransitionFixture(t)
	p, err := prepareOpenCodeNativeApply(f.request.Client.ConfigRoot, f.request.Delivery.ActivePath, f.request.PreviousNativeObjects, f.request.Delivery.NativeObjects, nativeconfig.New(), shared.RenameDirectoryExclusive, os.RemoveAll)
	if err != nil {
		t.Fatal(err)
	}
	req, err := openCodeTransitionRequest(p)
	if err != nil {
		t.Fatal(err)
	}
	txn, staged, err := stageOpenCodeTransitionSkills(f.request, p)
	if err != nil {
		t.Fatal(err)
	}
	stop := errors.New("TEST stop with prepared journal")
	req.PersistPrepared = func(prepared nativeconfig.PreparedTransition) error {
		if err := freshTransitions(f).PersistPrepared(context.Background(), f.request.NativeAttempt, txn.root, transitionDTO(prepared), f.request.PreviousNativeObjects, f.request.Delivery.NativeObjects); err != nil {
			return err
		}
		if !commit {
			return stop
		}
		return commitOpenCodeTransitionSkills(f.request.Client.ConfigRoot, txn, p, staged)
	}
	_, err = nativeconfig.New().ApplyDialectTransition(req)
	if commit && err != nil || !commit && !errors.Is(err, stop) {
		t.Fatal("transition did not reach durable boundary", err)
	}
	return f, txn.root
}
