//go:build linux

package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
)

func TestGeminiCycleExactOwnedMissingDirectoryRepairsThroughActualAdapter(t *testing.T) {
	c := geminiCycleInert(t)
	r := c.recipes["a"]
	r.Revision = "A-two"
	c.author(t, r, true)
	geminiCycleStageActual(t, c, "a")
	beforeState, err := os.ReadFile(filepath.Join(r.StateRoot, "state-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	beforePayload, err := geminiCycleTree(c.f.ctx, r.Active)
	if err != nil {
		t.Fatal(err)
	}
	c.missingA(t)
	if gemini.VerifyGeminiNativeObjects(r.Profile, r.Binding.NativeObjects, true, nativeconfig.New()) != nil {
		t.Fatal("actual verifier rejected exact owned directory absence")
	}
	if err := gemini.ApplyGeminiNativeMutation(r.Profile, r.Active, r.Binding.NativeObjects, r.Binding.NativeObjects); err != nil {
		t.Fatal("actual Gemini owned skill restore", err)
	}
	if err := c.skillProof(r, true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.configProof(r, true); err != nil {
		t.Fatal(err)
	}
	afterState, err := os.ReadFile(filepath.Join(r.StateRoot, "state-v2.json"))
	if err != nil || !reflect.DeepEqual(beforeState, afterState) {
		t.Fatal("file adapter changed state")
	}
	afterPayload, err := geminiCycleTree(c.f.ctx, r.Active)
	if err != nil || !reflect.DeepEqual(beforePayload, afterPayload) {
		t.Fatal("repair changed independent payload")
	}
	c.f.check(t)
}
func TestGeminiCyclePartialOrExtraOwnedSkillCannotBecomeAnAbsenceFault(t *testing.T) {
	for _, kind := range []string{"missing-leaf", "extra-file", "leaf-alias", "leaf-fifo", "leaf-directory", "wrong-object", "wrong-scope", "mode", "expired"} {
		t.Run(kind, func(t *testing.T) {
			c := geminiCycleInert(t)
			r := c.recipes["a"]
			r.Revision = "A-two"
			c.author(t, r, true)
			geminiCycleStageActual(t, c, "a")
			switch kind {
			case "missing-leaf":
				if err := os.Remove(r.SkillPath); err != nil {
					t.Fatal(err)
				}
			case "extra-file":
				projectNativeRuntimeWrite(t, filepath.Join(filepath.Dir(r.SkillPath), "extra"), []byte("foreign"), 0600)
			case "leaf-alias":
				if err := os.Remove(r.SkillPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(c.f.base.Probe, r.SkillPath); err != nil {
					t.Fatal(err)
				}
			case "leaf-fifo", "leaf-directory":
				if err := os.Remove(r.SkillPath); err != nil {
					t.Fatal(err)
				}
				if kind == "leaf-fifo" {
					if err := unix.Mkfifo(r.SkillPath, 0644); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Mkdir(r.SkillPath, 0644); err != nil {
					t.Fatal(err)
				}
			case "wrong-object":
				r.Binding.NativeObjects = append(r.Binding.NativeObjects, geminiCycleOwnedSkill(r))
			case "wrong-scope":
				r.Scope = "b"
			case "mode":
				if err := os.Chmod(filepath.Dir(r.SkillPath), 0700); err != nil {
					t.Fatal(err)
				}
			case "expired":
				c.f.cancel()
			}
			var before map[string]geminiCycleRecord
			if kind != "leaf-alias" && kind != "leaf-fifo" && kind != "leaf-directory" && kind != "expired" {
				var err error
				before, err = geminiCycleTree(c.f.ctx, r.Profile)
				if err != nil {
					t.Fatal(err)
				}
			}
			probeSHA := nativeSHA(t, c.f.base.Probe)
			probeBefore, err := os.Lstat(c.f.base.Probe)
			if err != nil {
				t.Fatal(err)
			}
			leafBefore, leafErr := os.Lstat(r.SkillPath)
			done := make(chan error, 1)
			go func() { done <- geminiCycleRemoveOwnedSkill(c.f.ctx, r) }()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("tampering accepted as controlled absence")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("owned absence refusal blocked on a special file")
			}
			if leafErr == nil {
				leafAfter, err := os.Lstat(r.SkillPath)
				if err != nil || !os.SameFile(leafBefore, leafAfter) || !reflect.DeepEqual(leafBefore.Sys(), leafAfter.Sys()) {
					t.Fatal("refusal changed actual held leaf metadata")
				}
			}
			probeAfter, err := os.Lstat(c.f.base.Probe)
			if err != nil || !reflect.DeepEqual(probeBefore.Sys(), probeAfter.Sys()) || nativeSHA(t, c.f.base.Probe) != probeSHA {
				t.Fatal("refusal read or changed aliased probe target")
			}
			if before != nil {
				after, err := geminiCycleTree(c.f.ctx, r.Profile)
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("refused absence fault mutated files")
				}
			}
			if kind == "missing-leaf" || kind == "extra-file" || kind == "leaf-alias" {
				if gemini.VerifyGeminiNativeObjects(r.Profile, r.Binding.NativeObjects, true, nativeconfig.New()) == nil {
					t.Fatal("actual verifier accepted partial/foreign skill tampering")
				}
			}
		})
	}
}
