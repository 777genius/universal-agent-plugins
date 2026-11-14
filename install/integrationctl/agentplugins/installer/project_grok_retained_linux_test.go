//go:build linux

package installer

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This explicitly authored historical prototype is not product enablement
// evidence. Its reader and all subsequent operations use the real facade.
func newGrokPrototypeSDKFixture(t *testing.T) *grokEnableSDKFixture {
	t.Helper()
	f := newGrokEnableSDKFixture(t, true)
	f.install(t)
	state, err := f.engine.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := f.binding(t)
	pathHash := fmt.Sprintf("%x", sha256.Sum256([]byte(binding.TargetLocator)))
	profileHash := fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Dir(f.config))))
	id := "project/" + pathHash[:8] + "/grok-sdk-owned"
	before := deathRead(t, f.config)
	after := strings.Replace(before, "enabled=[", "enabled=['"+id+"',", 1)
	deathWrite(t, f.config, after)
	binding.NativeObjects = append(binding.NativeObjects, domain.NativeObjectOwnership{
		ObjectID: "grok-enabled:" + profileHash + ":" + id, Kind: "grok_enabled_plugin", LogicalName: "grok-sdk-owned", Path: binding.TargetLocator,
		BeforeDigest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(before))), ManagedDigest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(after))), ProtectionClass: "native_config_member",
	})
	state.Installations[0].Clients[binding.ClientBindingID] = binding
	if err := f.engine.store.Save(state); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestProjectGrokSDKUpdateRepairRetainsOwnedMemberAtDurablePackageBoundary(t *testing.T) {
	for _, op := range []Operation{OpUpdate, OpRepair} {
		t.Run(string(op), func(t *testing.T) {
			f := newGrokPrototypeSDKFixture(t)
			binding := f.binding(t)
			owned, config := binding.NativeObjects[len(binding.NativeObjects)-1], deathRead(t, f.config)
			skill := filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")
			original := deathRead(t, skill)
			if op == OpUpdate {
				deathWrite(t, filepath.Join(f.pkg, "skills", "docs", "SKILL.md"), strings.Replace(original, "Original", "Updated", 1))
			} else {
				deathWrite(t, skill, "immutable drift")
			}
			req := f.selected(t, op, false)
			seen := false
			f.engine.cfg.OnCommittedBinding = func(_ context.Context, facts BindingFacts) error {
				current := f.binding(t)
				committed := false
				for _, receipt := range current.Receipts {
					committed = committed || receipt.OperationID == req.OperationID
				}
				if !committed {
					return nil
				} // Existing Prepared handoff precedes this operation.
				seen = true
				if facts.TargetPath != binding.TargetLocator || len(current.NativeObjects) != 2 || current.NativeObjects[1] != owned || current.NativeActivationAttempt == "" {
					t.Fatalf("durable package boundary lost prior native ownership: %+v", current)
				}
				want := original
				if op == OpUpdate {
					want = strings.Replace(original, "Original", "Updated", 1)
				}
				if deathRead(t, skill) != want {
					t.Fatal("callback did not observe the actual new package commit")
				}
				return errors.New("inert interruption after durable package commit")
			}
			if result, err := f.execute(t, req, Decision{Confirmed: true}); err == nil {
				t.Fatalf("commit-boundary interruption accepted: %+v", result)
			}
			if !seen || deathRead(t, f.config) != config || f.native.duplex != 0 {
				t.Fatal("ordinary package replacement changed native enablement")
			}
			f.engine = f.reopen(t, f.home)
			current := f.binding(t)
			if len(current.NativeObjects) != 2 || current.NativeObjects[1] != owned {
				t.Fatal("cold state lost owned member at interruption boundary")
			}
		})
	}
}

func TestProjectGrokSDKRetainedMembershipDriftRefusesBeforePackageOrState(t *testing.T) {
	for _, mode := range []string{"missing", "disabled", "changed-id", "blank-home", "foreign-home"} {
		t.Run(mode, func(t *testing.T) {
			f := newGrokPrototypeSDKFixture(t)
			request := f.selected(t, OpUpdate, false)
			binding := f.binding(t)
			skill := filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")
			beforeSkill := deathRead(t, skill)
			deathWrite(t, filepath.Join(f.pkg, "skills", "docs", "SKILL.md"), strings.Replace(beforeSkill, "Original", "Updated", 1))
			switch mode {
			case "missing":
				deathWrite(t, f.config, "[plugins]\nenabled=['foreign']\ndisabled=['other']\n")
			case "disabled":
				deathWrite(t, f.config, strings.Replace(deathRead(t, f.config), "disabled=['other']", "disabled=['other','grok-sdk-owned']", 1))
			case "changed-id":
				deathWrite(t, f.config, strings.Replace(deathRead(t, f.config), "/grok-sdk-owned", "/foreign-owned", 1))
			case "blank-home":
				f.engine = f.reopen(t, "")
			case "foreign-home":
				f.engine = f.reopen(t, f.home+"-foreign")
			}
			beforeState, err := f.engine.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			beforeConfig, calls, duplex := deathRead(t, f.config), f.native.calls, f.native.duplex
			if prepared, err := f.engine.Prepare(t.Context(), request); err == nil {
				_ = prepared.Close()
				t.Fatal("changed retained native authority accepted")
			}
			afterState, err := f.engine.store.Load()
			if err != nil || !reflect.DeepEqual(beforeState, afterState) || deathRead(t, skill) != beforeSkill || deathRead(t, f.config) != beforeConfig || f.native.calls != calls || f.native.duplex != duplex {
				t.Fatal("retained-membership refusal changed native/package/state authority", err)
			}
		})
	}
}

func TestProjectGrokSDKPrototypeOwnedRemovalRefusesBeforeEffects(t *testing.T) {
	f := newGrokPrototypeSDKFixture(t)
	request := f.selected(t, OpRemove, false)
	binding := f.binding(t)
	statePath := filepath.Join(f.project, ".uap", "state-v2.json")
	state := deathRead(t, statePath)
	unchanged := snapshotGrokSDKFiles(t, f, f.config, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"))
	for _, decision := range []Decision{{}, {Confirmed: true}} {
		if result, err := f.execute(t, request, decision); err == nil {
			t.Fatalf("unqualified prototype removal accepted: %+v", result)
		}
		unchanged()
		// Confirmed Remove may persist and acknowledge its own unchanged
		// attempt. Its exact predecessor bytes survive; state inode/mtime
		// are not promised by that existing generic durability protocol.
		if deathRead(t, statePath) != state {
			t.Fatal("refused removal did not restore exact predecessor state")
		}
		current := f.binding(t)
		if current.NativeActivationAttempt != "" || !reflect.DeepEqual(binding.NativeObjects, current.NativeObjects) {
			t.Fatal("refused removal changed prototype ownership or created an attempt")
		}
	}
	if _, err := os.Stat(binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
}
