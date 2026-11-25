package installer

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestNativeOnlyUserSiblingFactsRetainObservedAuthority(t *testing.T) {
	root := t.TempDir()
	e := &Engine{cfg: Config{Registry: all.Default()}}
	fact := TargetFacts{ClientID: "windsurf", BindingID: "native-binding",
		NativeConfigRoot: filepath.Join(root, "original", "devin"),
		Executable:       filepath.Join(root, "bin", "devin"), Version: "3000.12.57"}
	req := Request{Operation: OpUpdate, ClientID: "cline", ClientConfigRoot: filepath.Join(root, "cline"), KnownTargets: []TargetFacts{fact}}
	detected, err := e.detectedClients(req)
	if err != nil {
		t.Fatal("native-only unselected User binding was rejected", err)
	}
	sibling := detected[domain.ClientWindsurf]
	if sibling.ConfigRoot != "" || sibling.NativeConfigRoot != fact.NativeConfigRoot || sibling.Version != fact.Version || sibling.ExecutablePath != fact.Executable {
		t.Fatal("native sibling authority was dropped or replaced legacy authority", sibling)
	}
	e.store = projectedPinStore{state: domain.StateFileV2{Installations: []domain.Installation{{
		InstallationID: "same-installation",
		Clients: map[string]domain.ClientBinding{
			"native-binding": {ClientID: "windsurf", ClientBindingID: fact.BindingID, NativeProfileRoot: fact.NativeConfigRoot},
			"cline-binding":  {ClientID: "cline", ClientBindingID: "cline-binding"},
		},
	}}}}
	req.InstallationID = "same-installation"
	target := usecase.AddInput{Client: detected[domain.ClientCline]}
	checks, err := e.compatibilityChecks(req, target)
	if err != nil || len(checks) != 2 {
		t.Fatal("same-installation compatibility lost an owned native-only sibling", checks, err)
	}
	found := false
	for _, check := range checks {
		if check.Client.ClientID == domain.ClientWindsurf {
			found = true
			if check.Client.NativeConfigRoot != fact.NativeConfigRoot || check.Client.ConfigRoot != "" || check.Client.Version != fact.Version ||
				check.BackendExecutable != fact.Executable || check.CompatibilityBindingID != fact.BindingID {
				t.Fatal("compatibility reconstruction lost native authority or ownership", check)
			}
		}
	}
	if !found {
		t.Fatal("native-only sibling was omitted")
	}
	wrong := req
	wrong.KnownTargets = []TargetFacts{fact}
	wrong.KnownTargets[0].BindingID = "different-owner"
	if _, err := e.compatibilityChecks(wrong, target); !errors.Is(err, ErrTargetFactsUnavailable) {
		t.Fatal("native facts bypassed the recorded binding identity", err)
	}
	body, err := json.Marshal(fact)
	if err != nil || strings.Contains(string(body), "NativeConfigRoot") || strings.Contains(string(body), "3000.12.57") {
		t.Fatal("native root/version became diagnostic JSON", string(body), err)
	}
}

func TestNativeOnlyKnownFactsKeepAdmissionAndConflictGuards(t *testing.T) {
	root := t.TempDir()
	e := &Engine{cfg: Config{Registry: all.Default()}}
	fact := TargetFacts{ClientID: "windsurf", BindingID: "native-binding",
		NativeConfigRoot: filepath.Join(root, "original", "devin"),
		Executable:       filepath.Join(root, "bin", "devin"), Version: "3000.12.57"}
	for _, name := range []string{"missing-root", "relative-native-root", "unclean-native-root", "non-opted", "non-primary", "malformed-legacy", "missing-binding", "duplicate", "project-empty-root", "missing-required-executable"} {
		t.Run(name, func(t *testing.T) {
			attempt := fact
			engine := *e
			facts := []TargetFacts{attempt}
			switch name {
			case "missing-root":
				facts[0].NativeConfigRoot = ""
			case "relative-native-root":
				facts[0].NativeConfigRoot = "relative/devin"
			case "unclean-native-root":
				facts[0].NativeConfigRoot += string(filepath.Separator) + "."
			case "non-opted":
				facts[0].ClientID = "cline"
			case "non-primary":
				facts[0].Executable = filepath.Join(root, "windsurf")
			case "malformed-legacy":
				facts[0].ConfigRoot = "relative/cascade"
			case "missing-binding":
				facts[0].BindingID = ""
			case "duplicate":
				facts = append(facts, fact)
			case "project-empty-root":
				engine.cfg.ProjectRoot = filepath.Join(root, "project")
			case "missing-required-executable":
				facts = []TargetFacts{{ClientID: "copilot", BindingID: "copilot-binding", ConfigRoot: filepath.Join(root, "copilot")}}
			}
			if _, err := engine.knownTargetIndex(OpRepair, facts); !errors.Is(err, ErrTargetFactsUnavailable) {
				t.Fatal("invalid unselected authority accepted", err)
			}
		})
	}
	base := domain.DetectedClient{ClientID: domain.ClientWindsurf, NativeConfigRoot: fact.NativeConfigRoot, ExecutablePath: fact.Executable, Version: fact.Version}
	for _, name := range []string{"native-root", "version"} {
		t.Run("conflicting-"+name, func(t *testing.T) {
			conflict := base
			if name == "native-root" {
				conflict.NativeConfigRoot = filepath.Join(root, "different", "devin")
			} else {
				conflict.Version = "3000.11.205"
			}
			detected := map[domain.ClientID]domain.DetectedClient{domain.ClientWindsurf: base}
			if err := addDetectedClient(detected, conflict); !errors.Is(err, ErrTargetFactsUnavailable) {
				t.Fatal("conflicting native evidence accepted", err)
			}
		})
	}
}

func TestNativeOnlyKnownRemovalDoesNotRequireVersion(t *testing.T) {
	root := t.TempDir()
	e := &Engine{cfg: Config{Registry: all.Default()}}
	fact := TargetFacts{ClientID: "windsurf", BindingID: "native-binding",
		NativeConfigRoot: filepath.Join(root, "original", "devin"), Executable: filepath.Join(root, "bin", "devin")}
	if _, err := e.detectedClients(Request{Operation: OpRemove, ClientID: "cline", ClientConfigRoot: filepath.Join(root, "cline"), KnownTargets: []TargetFacts{fact}}); err != nil {
		t.Fatal("observational native sibling made removal require a version probe", err)
	}
}
