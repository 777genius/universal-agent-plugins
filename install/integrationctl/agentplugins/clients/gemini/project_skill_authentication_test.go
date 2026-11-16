package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type geminiSkillAuthProbe struct {
	output   []byte
	commands []legacyports.Command
	after    func()
	err      error
	unclosed bool
}

func (probe *geminiSkillAuthProbe) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	probe.commands = append(probe.commands, command)
	if probe.after != nil {
		probe.after()
	}
	return legacyports.CommandResult{Stdout: probe.output, NoChildStarted: !probe.unclosed}, probe.err
}

type geminiSkillAuthFixture struct {
	request domain.ActivationRequest
	kernel  nativeconfig.Kernel
	outcome domain.ActivationOutcome
	probe   *geminiSkillAuthProbe
	trust   string
	foreign string
	skill   string
}

func newGeminiSkillAuthFixture(t *testing.T) geminiSkillAuthFixture {
	t.Helper()
	t.Setenv("GEMINI_RESTRICTED_MODE", "")
	t.Setenv("GEMINI_CLI_TRUST_WORKSPACE", "")
	request, kernel := geminiProbeFixture(t)
	request.Plan.Authentication = domain.AuthenticationNotChecked
	request.Plan.Policy = domain.PolicyAllowed
	request.Plan.Status = "ready"
	request.Plan.Warnings = []string{"authentication_not_catalog_verified"}
	if err := os.MkdirAll(request.Client.ConfigRoot, 0700); err != nil {
		t.Fatal(err)
	}
	f := geminiSkillAuthFixture{request: request, kernel: kernel,
		outcome: domain.ActivationOutcome{Activation: domain.ActivationPrepared, Authentication: domain.AuthenticationNotChecked,
			Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged},
		probe: &geminiSkillAuthProbe{}, trust: filepath.Join(request.Client.ConfigRoot, "trustedFolders.json"),
		foreign: filepath.Join(filepath.Dir(request.Client.ConfigRoot), "foreign-project"),
		skill:   filepath.Join(request.Plan.NativeRegistryRoot, "skills", "audit", "SKILL.md")}
	f.writeTrust(t, map[string]string{f.foreign: "DO_NOT_TRUST"})
	f.probe.output = []byte("Discovered Agent Skills:\n\naudit [Enabled]\n  Description: audit marker\n  Location:    " + f.skill + "\n\n")
	return f
}

func (f *geminiSkillAuthFixture) writeTrust(t *testing.T, value map[string]string) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.trust, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func (f *geminiSkillAuthFixture) complete(t *testing.T) (domain.ActivationOutcome, error) {
	t.Helper()
	outcome, err := completeGeminiProject(t.Context(), clients.Env{NativeConfig: f.kernel, Runner: f.probe}, f.request, f.outcome)
	for _, command := range f.probe.commands {
		if !reflect.DeepEqual(command.Argv, []string{f.request.Client.ExecutablePath, "skills", "list", "--all"}) ||
			command.Dir != f.request.Plan.TargetContext.ProjectRoot {
			t.Fatalf("unexpected native command: %+v", command)
		}
	}
	return outcome, err
}

func TestGeminiObservedSkillInstallationAuthentication(t *testing.T) {
	for _, acting := range []bool{false, true} {
		name := "verify-only"
		if acting {
			name = "confirmed"
		}
		t.Run(name, func(t *testing.T) {
			f := newGeminiSkillAuthFixture(t)
			f.request.VerifyOnly = !acting
			before, err := os.ReadFile(f.trust)
			if err != nil {
				t.Fatal(err)
			}
			outcome, err := f.complete(t)
			if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled ||
				outcome.Authentication != domain.AuthenticationNotRequired || outcome.Policy != domain.PolicyAllowed || len(f.probe.commands) != 1 {
				t.Fatalf("exact observed Skill did not qualify installation authentication: %+v %v", outcome, err)
			}
			after, err := os.ReadFile(f.trust)
			if err != nil {
				t.Fatal(err)
			}
			if !acting {
				if string(before) != string(after) || outcome.NativeEffect != domain.NativeEffectUnchanged {
					t.Fatal("read-only discovery changed trust or native effect")
				}
				return
			}
			_, mapping, err := geminiTrustDocument(after)
			if err != nil || len(mapping) != 2 || mapping[f.foreign] != "DO_NOT_TRUST" ||
				mapping[f.request.Plan.TargetContext.ProjectRoot] != "TRUST_FOLDER" || outcome.NativeEffect != domain.NativeEffectCommitted {
				t.Fatalf("confirmed exact trust did not preserve the foreign rule: %s %v", after, err)
			}
		})
	}
}

func (f *geminiSkillAuthFixture) refuse(t *testing.T, name string) {
	t.Helper()
	drift := func() {
		if err := os.WriteFile(f.skill, []byte("---\nname: audit\ndescription: audit marker\n---\nChanged owned body.\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	switch name {
	case "closed-probe-failure":
		f.probe.err = context.DeadlineExceeded
	case "unclosed-probe":
		f.probe.err, f.probe.unclosed = context.DeadlineExceeded, true
	case "disabled":
		f.probe.output = []byte(strings.Replace(string(f.probe.output), "[Enabled]", "[Disabled]", 1))
	case "foreign-location":
		f.probe.output = []byte(strings.Replace(string(f.probe.output), f.skill, filepath.Join(filepath.Dir(f.request.Client.ConfigRoot), "foreign", "SKILL.md"), 1))
	case "metadata-mismatch":
		f.probe.output = []byte(strings.Replace(string(f.probe.output), "Description: audit marker", "Description: unrelated marker", 1))
	case "drift-before":
		drift()
	case "drift-after":
		f.probe.after = drift
	case "missing-receipt":
		f.request.Delivery.NativeObjects = nil
	case "duplicate-receipt":
		f.request.Delivery.NativeObjects = append(f.request.Delivery.NativeObjects, f.request.Delivery.NativeObjects[0])
	case "wrong-receipt-name":
		f.request.Delivery.NativeObjects[0].LogicalName = "foreign"
	case "unknown-source":
		f.request.Plan.Components[0].Source = "foreign"
	case "catalog-unknown":
		f.request.Plan.Warnings = append(f.request.Plan.Warnings, "authentication_requirement_unknown")
	case "catalog-unmarked":
		f.request.Plan.Warnings = nil
	case "outcome-policy":
		f.outcome.Policy = "blocked"
	case "plan-policy":
		f.request.Plan.Policy = "blocked"
	case "activation-attested":
		f.outcome.ActivationAttested = true
	case "authentication-attested":
		f.outcome.AuthenticationAttested = true
	case "request-attested":
		f.request.ActivationComplete = true
	case "uncertain":
		f.outcome.NativeEffect = domain.NativeEffectUncertain
	case "trust-denied":
		f.request.VerifyOnly = false
		f.writeTrust(t, map[string]string{f.foreign: "DO_NOT_TRUST", f.request.Plan.TargetContext.ProjectRoot: "DO_NOT_TRUST"})
	case "environment-trust-denied":
		f.request.VerifyOnly = false
		t.Setenv("GEMINI_CLI_TRUST_WORKSPACE", "false")
	case "configured-disabled":
		body := []byte("{\"skills\":{\"disabled\":[\"audit\"]}}")
		if err := os.WriteFile(filepath.Join(f.request.Plan.NativeRegistryRoot, "settings.json"), body, 0600); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("unknown negative fixture")
	}
}

func TestGeminiObservedSkillAuthenticationRequiresFreshOwnedEvidence(t *testing.T) {
	for _, name := range []string{"closed-probe-failure", "unclosed-probe", "disabled", "foreign-location", "metadata-mismatch",
		"drift-before", "drift-after", "missing-receipt", "duplicate-receipt", "wrong-receipt-name", "unknown-source",
		"catalog-unknown", "catalog-unmarked", "outcome-policy", "plan-policy", "activation-attested", "authentication-attested",
		"request-attested", "uncertain", "trust-denied", "environment-trust-denied", "configured-disabled"} {
		t.Run(name, func(t *testing.T) {
			f := newGeminiSkillAuthFixture(t)
			f.refuse(t, name)
			before, err := os.ReadFile(f.trust)
			if err != nil {
				t.Fatal(err)
			}
			outcome, err := f.complete(t)
			if outcome.Authentication != domain.AuthenticationNotChecked {
				t.Fatalf("unqualified Skill changed authentication: %+v %v", outcome, err)
			}
			if name == "unclosed-probe" {
				if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, context.DeadlineExceeded) || outcome.NativeEffect != domain.NativeEffectUncertain {
					t.Fatalf("unclosed native observation lost typed cause/effect: %+v %v", outcome, err)
				}
			} else if err != nil {
				t.Fatalf("closed negative unexpectedly failed: %v", err)
			}
			after, readErr := os.ReadFile(f.trust)
			if readErr != nil || string(after) != string(before) {
				t.Fatalf("unqualified observation changed existing trust: %v", readErr)
			}
		})
	}
}

func TestGeminiObservedSkillsPreserveAuthenticationDecisions(t *testing.T) {
	for _, field := range []string{"plan", "outcome"} {
		for _, state := range []domain.AuthenticationState{"", "pending", "complete", "failed", domain.AuthenticationNotRequired} {
			t.Run(field+"/"+string(state), func(t *testing.T) {
				f := newGeminiSkillAuthFixture(t)
				if field == "plan" {
					f.request.Plan.Authentication = state
				} else {
					f.outcome.Authentication = state
				}
				before := f.outcome.Authentication
				outcome, err := f.complete(t)
				if err != nil || outcome.Authentication != before || len(f.probe.commands) != 1 {
					t.Fatalf("preexisting authentication decision changed: %+v %v", outcome, err)
				}
			})
		}
	}
}
