//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/grok"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// An inert in-memory registry answer around the actual Engine/Store/Stager.
// No native executable or command runner is invoked by this contract.
type grokCycleInertRegistry struct{ project, executable string }

func (r grokCycleInertRegistry) Run(_ context.Context, command ports.Command) (ports.CommandResult, error) {
	if command.Dir != r.project || !reflect.DeepEqual(command.Argv, []string{r.executable, "inspect", "--json"}) {
		return ports.CommandResult{}, fmt.Errorf("unregistered inert Grok command")
	}
	plugins := []any{}
	root := filepath.Join(r.project, ".grok", "plugins")
	entries, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		return ports.CommandResult{}, err
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		raw, err := projectNativeRuntimeRead(filepath.Join(path, ".grok-plugin", "plugin.json"), 1<<20)
		if err != nil {
			return ports.CommandResult{}, err
		}
		manifest, err := projectNativeRuntimeJSON(raw)
		if err != nil {
			return ports.CommandResult{}, err
		}
		plugins = append(plugins, map[string]any{"name": manifest.(map[string]any)["name"], "scope": "project", "path": path, "enabled": true})
	}
	raw, err := json.Marshal(map[string]any{"grokVersion": "1.0.70", "cwd": r.project, "projectRoot": r.project + "/", "projectTrusted": true, "plugins": plugins, "skills": []any{}})
	return ports.CommandResult{Stdout: raw}, err
}

func TestGrokCycleActualEngineNoopAndMissingLeafRepair(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	root := t.TempDir()
	project, user, source := filepath.Join(root, "project"), filepath.Join(root, "home", ".grok"), filepath.Join(root, "source")
	for _, path := range []string{project, user} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	name := "uap-grok-cycle-a"
	manifest := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","description":"Disposable Grok skills cycle","name":"uap-grok-cycle-a","version":"1.0.0"}`)
	projectNativeRuntimeWrite(t, filepath.Join(source, "plugin.json"), manifest, 0600)
	skill := []byte("---\nname: " + name + "\ndescription: Actual inert repair contract\n---\nHeld source.\n")
	projectNativeRuntimeWrite(t, filepath.Join(source, "skills", name, "SKILL.md"), skill, 0600)
	helper := filepath.Join(root, "inert-executable")
	projectNativeRuntimeWrite(t, helper, []byte("inert bytes; never executed\n"), 0700)
	registry, err := clients.NewRegistry(grok.New())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{ProjectRoot: project, StateRoot: filepath.Join(project, ".uap"), Registry: registry, TrustedLocalPackages: true, HelperExecutable: helper, Runner: grokCycleInertRegistry{project, helper}})
	if err != nil {
		t.Fatal(err)
	}
	req := installer.Request{Scope: domain.ScopeProject, ProjectRoot: project, ClientID: "grok", ClientConfigRoot: user, ClientExecutable: helper, ClientVersion: "1.0.70", PackageRoot: source, InstallationID: "00000000-0000-4000-8000-000000000070", RequiredComponents: []string{"skills"}}
	sequence := 0
	apply := func(operation installer.Operation, mutated bool) {
		t.Helper()
		sequence++
		req.Operation, req.OperationID = operation, fmt.Sprintf("grok-inert-%d", sequence)
		handle, err := engine.Prepare(ctx, req)
		if err != nil {
			t.Fatal(operation, err)
		}
		defer func() {
			if err := handle.Close(); err != nil {
				t.Error(err)
			}
		}()
		result, err := engine.Apply(ctx, handle, installer.Decision{Confirmed: true})
		if err != nil || result.Mutated != mutated || (operation == installer.OpRepair && !mutated && !result.NoChange) {
			t.Fatalf("actual Grok %s mutation=%t: %+v %v", operation, mutated, result, err)
		}
	}
	apply(installer.OpInstall, true)
	statePath := filepath.Join(project, ".uap", "state-v2.json")
	before, err := projectNativeRuntimeRead(statePath, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	state, err := clineCycleState(before)
	if err != nil || len(state.Installations) != 1 {
		t.Fatal(err)
	}
	i := state.Installations[0]
	r := &grokCycleRecipe{Scope: "a", Project: project, Source: source, Name: name, Digest: i.Source.TreeDigest, Authored: map[string][]byte{"plugin.json": manifest, "skills/" + name + "/SKILL.md": skill}}
	if err := grokCycleSource(i, r); err != nil {
		t.Fatal(err)
	}
	var binding domain.ClientBinding
	for _, b := range i.Clients {
		binding = b
	}
	payload, err := grokCycleSnapshot(binding.TargetLocator)
	if err != nil || grokCyclePayload(payload, grokCycleProjected(r)) != nil {
		t.Fatal("cycle source/native payload recipe differs from actual Engine projection", err)
	}
	if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationPackageValid || binding.DataReceiptID != "" || len(i.DataReceipts) != 0 {
		t.Fatal("actual Grok Prepared/zero-data contract differs")
	}
	for _, operation := range []installer.Operation{installer.OpInstall, installer.OpUpdate, installer.OpRepair} {
		apply(operation, false)
		after, err := projectNativeRuntimeRead(statePath, 1<<20)
		if err != nil || string(after) != string(before) {
			t.Fatal("actual Grok no-op changed state", operation, err)
		}
	}
	path := filepath.Join(binding.TargetLocator, "skills", name, "SKILL.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	apply(installer.OpRepair, true)
	restored, err := projectNativeRuntimeRead(path, 1<<20)
	if err != nil || string(restored) != string(skill) {
		t.Fatal("actual missing single owned leaf was not restored", err)
	}
}
