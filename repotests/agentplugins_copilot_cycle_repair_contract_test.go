//go:build linux

package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/copilot"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestCopilotCycleExactFrozenRevisionRepairRestoresOnlyMissingOwnedSkill(t *testing.T) {
	for _, kind := range []string{"absent", "missing-leaf", "extra-leaf"} {
		t.Run(kind, func(t *testing.T) {
			c := copilotCycleInert(t)
			r := c.recipes["a"]
			delete(c.trees, r.Source)
			r.Revision, r.Version = "a-two", "1.1.0"
			c.author(t, r, true)
			request := copilotCycleStageInert(t, c, "a")
			beforeBinding, beforeInstallation := r.Binding, r.Installation
			request.Replacing = true
			request.PreviousNativeObjects = append([]domain.NativeObjectOwnership{}, r.Binding.NativeObjects...)
			if kind == "absent" {
				c.missingA(t)
			} else {
				delete(c.f.protected, r.SkillPath)
				delete(c.trees, filepath.Dir(filepath.Dir(r.SkillPath)))
				if kind == "missing-leaf" {
					if err := os.Remove(r.SkillPath); err != nil {
						t.Fatal(err)
					}
				} else {
					projectNativeRuntimeWrite(t, filepath.Join(filepath.Dir(r.SkillPath), "foreign.txt"), []byte("foreign must not be adopted"), 0600)
				}
			}
			before, err := clineAttemptTree(filepath.Dir(filepath.Dir(r.SkillPath)))
			if err != nil {
				t.Fatal(err)
			}
			out, err := copilot.New().Activate(c.f.ctx, clients.Env{NativeConfig: nativeconfig.New()}, request)
			if kind == "absent" {
				if err != nil || out.Verification != domain.VerificationInstalled || out.Activation != domain.ActivationPrepared {
					t.Fatal("exact owned absence was not restored", err)
				}
				c.proof(t, r, "inert-restored-a-two")
				if !reflect.DeepEqual(r.Binding, beforeBinding) || !reflect.DeepEqual(r.Installation, beforeInstallation) {
					t.Fatal("file repair replaced logical/source/data authority")
				}
				return
			}
			after, treeErr := clineAttemptTree(filepath.Dir(filepath.Dir(r.SkillPath)))
			if err == nil || treeErr != nil || out.Verification != domain.VerificationFailed || out.NativeEffect != domain.NativeEffectUnchanged || !reflect.DeepEqual(before, after) {
				t.Fatal("partial native tampering was adopted or changed", err, treeErr)
			}
		})
	}
}

func TestCopilotCycleLocalSourceDriftCannotBecomeRecordedRepairAuthority(t *testing.T) {
	c := copilotCycleInert(t)
	copilotCycleStageInert(t, c, "a")
	r := c.recipes["a"]
	recorded := r.Installation
	delete(c.trees, r.Source)
	r.Revision, r.Version = "a-two", "1.1.0"
	c.author(t, r, true)
	if copilotCycleSource(recorded, r) == nil {
		t.Fatal("changed live direct-local source was accepted as recorded revision")
	}
}
