package grok

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

type projectSkillMetadataABAIO struct {
	nativeconfig.FileIO
	live, captured        string
	original, substituted []byte
	reads                 int
}

func (files *projectSkillMetadataABAIO) ReadNoFollowBounded(path string, limit int64) ([]byte, os.FileMode, bool, error) {
	files.reads++
	files.captured = path
	if err := os.WriteFile(files.live, files.substituted, 0600); err != nil {
		return nil, 0, false, err
	}
	file, err := nativeconfig.New().ReadExactFileBounded(path, limit)
	restore := os.WriteFile(files.live, files.original, 0600)
	return file.Body, file.Mode, file.Exists, errors.Join(err, restore)
}

func TestProjectGrokObservedSkillsBindMetadataDuringLiveABA(t *testing.T) {
	for _, nativeMetadata := range []string{"owned", "substituted"} {
		t.Run(nativeMetadata, func(t *testing.T) {
			f := newProjectSkillFixture(t)
			live := filepath.Join(f.request.Plan.ActivePath, "skills", "docs", "SKILL.md")
			original, err := os.ReadFile(live)
			if err != nil {
				t.Fatal(err)
			}
			files := &projectSkillMetadataABAIO{live: live, original: original,
				substituted: bytes.ReplaceAll(original, []byte("Exact owned text"), []byte("Substituted metadata"))}
			f.env.NativeConfig = nativeconfig.NewWithFileIO(files)
			if nativeMetadata == "substituted" {
				f.skill()["description"] = "Substituted metadata"
			}
			outcome, err := New().Activate(t.Context(), f.env, f.request)
			if err != nil || f.runner.calls != 1 || files.reads != 1 || (outcome.Activation == domain.ActivationActive) != (nativeMetadata == "owned") {
				t.Fatalf("live metadata ABA changed native proof: outcome=%+v calls=%d reads=%d err=%v", outcome, f.runner.calls, files.reads, err)
			}
			body, err := os.ReadFile(live)
			if err != nil || !bytes.Equal(body, original) {
				t.Fatalf("live package was not restored: %v", err)
			}
			if files.captured == live {
				t.Fatal("metadata was reread from the live package")
			}
			sealedRoot := filepath.Dir(filepath.Dir(filepath.Dir(files.captured)))
			if _, err := os.Lstat(sealedRoot); !os.IsNotExist(err) {
				t.Fatalf("sealed package was not cleaned: %v", err)
			}
		})
	}
}

func TestProjectGrokObservedSkillsCompleteWithoutNativeWrites(t *testing.T) {
	for _, verifyOnly := range []bool{false, true} {
		t.Run(fmt.Sprint(verifyOnly), func(t *testing.T) {
			f := newProjectSkillFixture(t)
			f.request.VerifyOnly = verifyOnly
			f.runner.document["skills"] = append(f.runner.document["skills"].([]any), map[string]any{
				"name": "foreign", "description": "Retained original User Skill", "userInvocable": true,
				"source": map[string]any{"type": "plugin", "plugin_name": "other", "path": f.request.Client.ConfigRoot + "/other/./skills/foreign/SKILL.md"}})
			unchanged := projectSkillFilesUnchanged(t, filepath.Join(f.request.Client.ConfigRoot, "config.toml"),
				filepath.Join(f.request.Client.ConfigRoot, "trusted_folders.toml"), filepath.Join(f.request.Client.ConfigRoot, "keep.txt"),
				filepath.Join(f.request.Plan.ActivePath, "skills", "docs", "SKILL.md"))
			out, err := New().Activate(t.Context(), f.env, f.request)
			if err != nil || out.Activation != domain.ActivationActive || out.Verification != domain.VerificationInstalled ||
				out.Authentication != domain.AuthenticationNotRequired || out.Policy != domain.PolicyAllowed ||
				out.NativeEffect != domain.NativeEffectUnchanged || out.ActivationAttested || out.AuthenticationAttested || len(out.UserActions) != 0 || f.runner.calls != 1 {
				t.Fatalf("exact enabled native Skill not qualified: %+v %v calls=%d", out, err, f.runner.calls)
			}
			if !New().VerifierAvailable(f.request.Client, f.request.Plan, f.request.BackendExecutable) ||
				New().CanCompletePreparedSetup(f.request, out) || New().AutomaticallyActivates(f.env, f.request) {
				t.Fatal("read-only capability acquired acting authority or is unavailable")
			}
			unchanged()
			digest, err := (packagesnapshot.Builder{}).Digest(t.Context(), f.request.Plan.ActivePath)
			if err != nil || digest != f.request.Delivery.ArtifactDigest {
				t.Fatal("managed bytes changed", err)
			}
		})
	}
}

func TestProjectGrokObservedSkillsRefuseIncompleteInventory(t *testing.T) {
	for name, change := range projectSkillInventoryNegatives() {
		t.Run(name, func(t *testing.T) {
			f := newProjectSkillFixture(t)
			change(f)
			out, err := New().Activate(t.Context(), f.env, f.request)
			if out.Activation != domain.ActivationPrepared || out.Authentication != domain.AuthenticationNotChecked ||
				out.Verification != domain.VerificationPackageValid || len(out.UserActions) != 1 || f.runner.calls != 1 {
				t.Fatalf("incomplete native inventory promoted: %+v %v", out, err)
			}
		})
	}
}

func projectSkillInventoryNegatives() map[string]func(*projectSkillFixture) {
	return map[string]func(*projectSkillFixture){
		"untrusted":           func(f *projectSkillFixture) { f.runner.document["projectTrusted"] = false },
		"disabled-plugin":     func(f *projectSkillFixture) { f.plugin()["enabled"] = false },
		"missing-plugin":      func(f *projectSkillFixture) { f.runner.document["plugins"] = []any{} },
		"missing-skill":       func(f *projectSkillFixture) { f.runner.document["skills"] = []any{} },
		"wrong-count":         func(f *projectSkillFixture) { f.plugin()["provides"].(map[string]any)["skills"] = 2 },
		"unrecognized-fields": func(f *projectSkillFixture) { f.runner.document["unknown"] = true },
		"mcp-present": func(f *projectSkillFixture) {
			f.runner.document["mcpServers"] = []any{map[string]any{"name": "foreign"}}
		},
		"disabled-skill": func(f *projectSkillFixture) { f.skill()["disabled"] = true },
		"not-invocable":  func(f *projectSkillFixture) { f.skill()["userInvocable"] = false },
		"description":    func(f *projectSkillFixture) { f.skill()["description"] = "foreign metadata" },
		"skill-alias":    func(f *projectSkillFixture) { f.skill()["name"] = "Docs" },
		"source-type":    func(f *projectSkillFixture) { f.skill()["source"].(map[string]any)["type"] = "user" },
		"source-plugin":  func(f *projectSkillFixture) { f.skill()["source"].(map[string]any)["plugin_name"] = "other" },
		"source-path": func(f *projectSkillFixture) {
			f.skill()["source"].(map[string]any)["path"] = filepath.Join(f.request.Client.ConfigRoot, "SKILL.md")
		},
		"source-alias": func(f *projectSkillFixture) {
			f.skill()["source"].(map[string]any)["Path"] = f.skill()["source"].(map[string]any)["path"]
		},
		"duplicate-skill": func(f *projectSkillFixture) { f.runner.document["skills"] = []any{f.skill(), f.skill()} },
		"folded-duplicate": func(f *projectSkillFixture) {
			f.runner.document["skills"] = []any{f.skill(), map[string]any{"name": "Docs"}}
		},
		"missing-provides":   func(f *projectSkillFixture) { delete(f.plugin(), "provides") },
		"old-native-version": func(f *projectSkillFixture) { f.runner.document["grokVersion"] = "1.0.67" },
	}
}

func TestProjectGrokObservedSkillsRequireOwnedPackage(t *testing.T) {
	for name, change := range projectSkillOwnershipNegatives() {
		t.Run(name, func(t *testing.T) {
			f := newProjectSkillFixture(t)
			change(f)
			out, err := New().Activate(t.Context(), f.env, f.request)
			calls := 1
			if name == "empty-selection" {
				calls = 0
				if err == nil || err.Error() != "grok project preparation requires supported Skills or MCP" {
					t.Fatalf("empty selection did not retain its exact preflight refusal: %+v %v", out, err)
				}
			}
			if out.Activation != domain.ActivationPrepared || out.Authentication != domain.AuthenticationNotChecked ||
				out.Verification != domain.VerificationPackageValid || f.runner.calls != calls {
				t.Fatalf("unowned or ineligible Skill promoted: %+v %v", out, err)
			}
		})
	}
}

func projectSkillOwnershipNegatives() map[string]func(*projectSkillFixture) {
	return map[string]func(*projectSkillFixture){
		"missing-receipt": func(f *projectSkillFixture) { f.request.Delivery.NativeObjects = nil },
		"duplicate-receipt": func(f *projectSkillFixture) {
			f.request.Delivery.NativeObjects = append(f.request.Delivery.NativeObjects, f.request.Delivery.NativeObjects[0])
		},
		"foreign-receipt":    func(f *projectSkillFixture) { f.request.Delivery.NativeObjects[0].ObjectID = "package:grok:foreign" },
		"wrong-receipt-path": func(f *projectSkillFixture) { f.request.Delivery.NativeObjects[0].Path = f.request.Client.ConfigRoot },
		"wrong-protection":   func(f *projectSkillFixture) { f.request.Delivery.NativeObjects[0].ProtectionClass = "foreign" },
		"modified-receipt":   func(f *projectSkillFixture) { f.request.Delivery.NativeObjects[0].UserModified = true },
		"unowned-fields":     func(f *projectSkillFixture) { f.request.Delivery.NativeObjects[0].SourceRelative = "foreign" },
		"missing-digest":     func(f *projectSkillFixture) { f.request.Delivery.ArtifactDigest = "" },
		"changed-resource": func(f *projectSkillFixture) {
			projectTestWrite(f.runner.t, filepath.Join(f.request.Plan.ActivePath, "skills", "docs", "references", "note.txt"), "changed")
		},
		"old-detected-version":     func(f *projectSkillFixture) { f.request.Client.Version = "1.0.67" },
		"manual-intent":            func(f *projectSkillFixture) { f.request.Plan.InstallIntent = domain.InstallIntentPrepare },
		"policy-unknown":           func(f *projectSkillFixture) { f.request.Plan.Policy = domain.PolicyState("unknown") },
		"non-native-plan":          func(f *projectSkillFixture) { f.request.Plan.PackageMode = domain.PackageMode("managed") },
		"empty-selection":          func(f *projectSkillFixture) { f.request.Plan.Components = nil },
		"foreign-component-source": func(f *projectSkillFixture) { f.request.Plan.Components[0].SourceName = "foreign" },
		"duplicate-selection": func(f *projectSkillFixture) {
			f.request.Plan.Components = append(f.request.Plan.Components, f.request.Plan.Components[0])
		},
		"mixed-mcp": func(f *projectSkillFixture) {
			f.request.Plan.Components = append(f.request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportNative})
			f.plugin()["provides"].(map[string]any)["mcpServers"] = 1
		},
	}
}

func TestProjectGrokObservedSkillsPreserveAuthenticationDecisions(t *testing.T) {
	for _, state := range []domain.AuthenticationState{domain.AuthenticationNotChecked, domain.AuthenticationPending, domain.AuthenticationComplete} {
		t.Run(string(state), func(t *testing.T) {
			f := newProjectSkillFixture(t)
			f.request.Plan.Authentication = state
			f.request.Plan.Warnings = append(f.request.Plan.Warnings, "authentication_requirement_unknown")
			out, err := New().Activate(t.Context(), f.env, f.request)
			if err != nil || out.Activation != domain.ActivationActive || out.Authentication != state {
				t.Fatalf("native installation overwrote catalog auth: %+v %v", out, err)
			}
		})
	}
}

func TestProjectGrokObservedSkillsRejectProbeDriftAndUnclosedCause(t *testing.T) {
	t.Run("resource-changes-during-inspect", func(t *testing.T) {
		f := newProjectSkillFixture(t)
		f.runner.during = func() {
			projectTestWrite(t, filepath.Join(f.request.Plan.ActivePath, "skills", "docs", "references", "note.txt"), "changed during probe")
		}
		out, err := New().Activate(t.Context(), f.env, f.request)
		if err == nil || out.Activation != domain.ActivationPrepared || out.Authentication != domain.AuthenticationNotChecked || f.runner.calls != 1 {
			t.Fatalf("drifting package promoted: %+v %v", out, err)
		}
	})
	for _, wrapping := range []string{"wrapped", "joined"} {
		t.Run(wrapping, func(t *testing.T) {
			f := newProjectSkillFixture(t)
			cause := errors.New("native process not reaped")
			probe := &domain.UnclosedClientProbe{Operation: "inspect", Err: cause}
			f.runner.err = fmt.Errorf("native: %w", probe)
			if wrapping == "joined" {
				f.runner.err = errors.Join(errors.New("other failure"), probe)
			}
			out, err := New().Activate(t.Context(), f.env, f.request)
			if !errors.Is(err, cause) || !domain.IsUnclosedClientProbe(err) || out.NativeEffect != domain.NativeEffectUncertain ||
				out.Activation != domain.ActivationPrepared || out.Authentication != domain.AuthenticationNotChecked || f.runner.calls != 1 {
				t.Fatalf("cleanup failure promoted or lost: %+v %v", out, err)
			}
		})
	}
}

// A real Grok 1.0.70 inspect carries diagnostic families not used by the
// adapter's activation proof, and plugin Skill paths contain a harmless /./.
func TestProjectGrokObservedSkillsRealNativeEnvelopeWithoutVersionProbe(t *testing.T) {
	f := newProjectSkillFixture(t)
	f.request.Client.Version = "" // ordinary CLI detection does not run --version
	f.runner.document["channel"] = "unknown"
	f.runner.document["updateChannel"] = nil
	f.runner.document["projectInstructions"] = []any{}
	f.runner.document["permissions"] = map[string]any{"loaded": 0}
	f.runner.document["loginPolicy"] = map[string]any{"apiKeyAuthDisabled": false}
	f.runner.document["hooks"] = []any{}
	f.runner.document["agents"] = []any{}
	f.runner.document["marketplaces"] = []any{}
	f.runner.document["lspServers"] = []any{}
	f.runner.document["configSources"] = map[string]any{"layers": []any{}}
	f.runner.document["experiments"] = []any{}
	f.runner.document["externalCompat"] = map[string]any{"remoteSettingsLoaded": false}
	source := f.skill()["source"].(map[string]any)
	source["path"] = filepath.Join(f.request.Plan.ActivePath, "skills") + "/./docs/SKILL.md"
	out, err := New().Activate(t.Context(), f.env, f.request)
	if err != nil || out.Activation != domain.ActivationActive ||
		out.Verification != domain.VerificationInstalled || f.runner.calls != 1 ||
		!New().VerifierAvailable(f.request.Client, f.request.Plan, f.request.BackendExecutable) {
		t.Fatalf("pinned Grok native report with unprobed version stayed Prepared: %+v %v", out, err)
	}

	for _, mutation := range []string{"wrong-version", "extra-field", "wrong-type", "untrusted"} {
		t.Run(mutation, func(t *testing.T) {
			switch mutation {
			case "wrong-version":
				f.runner.document["grokVersion"] = "1.0.67"
			case "extra-field":
				f.runner.document["unobservedAuthority"] = true
			case "wrong-type":
				f.runner.document["permissions"] = "unknown"
			case "untrusted":
				f.runner.document["projectTrusted"] = false
			}
			defer func() {
				f.runner.document["grokVersion"] = "1.0.70"
				delete(f.runner.document, "unobservedAuthority")
				f.runner.document["permissions"] = map[string]any{"loaded": 0}
				f.runner.document["projectTrusted"] = true
			}()
			f.runner.calls = 0
			out, err := New().Activate(t.Context(), f.env, f.request)
			if err != nil || out.Activation != domain.ActivationPrepared ||
				out.Verification != domain.VerificationPackageValid || f.runner.calls != 1 {
				t.Fatalf("unqualified native envelope promoted: %+v %v", out, err)
			}
		})
	}
}
