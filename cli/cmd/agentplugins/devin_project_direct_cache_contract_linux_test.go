//go:build linux

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type devinDirectOwned struct {
	installation domain.Installation
	binding      domain.ClientBinding
}

// Existing Devin journeys cover transport/recovery. This closes the distinct
// direct-local public DTO and receipt-history contract without a native client.
func TestProjectDevinDirectDefaultCacheLifecycleStages(t *testing.T) {
	f, cache := newWindDirectCacheJourney(t)
	sourceA, sourceB := projectJourneyTemp(t), projectJourneyTemp(t)
	write := func(root, version, skill, endpoint string) {
		t.Helper()
		projectJourneyWrite(t, filepath.Join(root, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"docs","version":"`+version+`"}`)
		projectJourneyWrite(t, filepath.Join(root, "skills", "docs", "SKILL.md"), skill)
		projectJourneyWrite(t, filepath.Join(root, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"`+endpoint+`"}}}`)
	}
	write(sourceA, "1.0.0", projectJourneySkill, "http://127.0.0.1:9/a-one")
	write(sourceB, "1.0.0", projectJourneySkill, "http://127.0.0.1:9/b-one")
	user := securityFixtureTree(t, f.devinRoot(""))
	marker := filepath.Join(f.namespace(f.a), "plugin-data", "foreign", "keep.txt")
	projectJourneyWrite(t, marker, "unrelated caller data\n")
	latest := map[string]devinDirectOwned{}
	var bTree map[string]string
	run := func(stage, verb, project, source, version string, mutated bool) {
		t.Helper()
		previous := latest[project]
		statePath := filepath.Join(f.namespace(project), "state-v2.json")
		var oldState string
		var oldNative, oldPackage map[string]string
		if !mutated {
			oldState = projectJourneyRead(t, statePath)
			oldNative = securityFixtureTree(t, f.devinRoot(project))
			oldPackage = securityFixtureTree(t, previous.binding.TargetLocator)
		}
		selector := previous.installation.InstallationID
		if verb == "add" {
			selector = source
		}
		cache.start = time.Now()
		time.Sleep(20 * time.Millisecond)
		raw := devinDirectExecute(t, f, project, verb, selector)
		cache.end = time.Now()
		state := f.state(t, project)
		if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
			t.Fatal("direct no-save state membership differs", stage)
		}
		current := devinDirectOwned{installation: state.Installations[0]}
		if verb == "remove" {
			devinDirectRemoved(t, f, project, previous, state)
			current.binding = previous.binding
		} else {
			if len(current.installation.Clients) != 1 || len(current.installation.DataReceipts) != 0 || len(state.TransactionReceipts) != 0 {
				t.Fatal("remote recipe unexpected binding/data/transaction state", stage)
			}
			for _, binding := range current.installation.Clients {
				current.binding = binding
			}
			devinDirectBinding(t, f, project, source, version, previous, current, mutated)
		}
		devinDirectDTO(t, raw, stage, verb, version, current, mutated)
		cache.observe(t, stage, raw, current.installation)
		if !mutated && (oldState != projectJourneyRead(t, statePath) || !reflect.DeepEqual(oldNative, securityFixtureTree(t, f.devinRoot(project))) || !reflect.DeepEqual(oldPackage, securityFixtureTree(t, previous.binding.TargetLocator))) {
			t.Fatal("direct no-op rewrote state/native/payload", stage)
		}
		if !reflect.DeepEqual(user, securityFixtureTree(t, f.devinRoot(""))) || projectJourneyRead(t, marker) != "unrelated caller data\n" {
			t.Fatal("direct project lifecycle changed User or unrelated data", stage)
		}
		if project == f.a && bTree != nil && !reflect.DeepEqual(bTree, securityFixtureTree(t, f.b)) {
			t.Fatal("direct A operation changed complete original B", stage)
		}
		latest[project] = current
		t.Logf("%s: binding_receipts=%d transaction_receipts=%d data_retained=%t", stage, len(current.binding.Receipts), len(state.TransactionReceipts), current.installation.DataRetained)
	}
	run("add-a", "add", f.a, sourceA, "1.0.0", true)
	run("add-b", "add", f.b, sourceB, "1.0.0", true)
	if latest[f.a].installation.InstallationID == latest[f.b].installation.InstallationID || latest[f.a].binding.PhysicalArtifact == latest[f.b].binding.PhysicalArtifact {
		t.Fatal("same-name direct scopes share installation/physical authority")
	}
	bTree = securityFixtureTree(t, f.b)
	run("repeat-add-a", "add", f.a, sourceA, "1.0.0", false)
	run("unchanged-update-a", "update", f.a, sourceA, "1.0.0", false)
	run("intact-repair-a", "repair", f.a, sourceA, "1.0.0", true)
	write(sourceA, "1.1.0", projectJourneyAdvancedSkill, "http://127.0.0.1:9/a-two")
	run("update-a", "update", f.a, sourceA, "1.1.0", true)
	active := latest[f.a].binding.TargetLocator
	managed := securityFixtureTree(t, active)
	config := projectJourneyRead(t, filepath.Join(f.devinRoot(f.a), "mcp_config.json"))
	if projectJourneyRead(t, filepath.Join(f.devinRoot(f.a), "skills", "docs", "SKILL.md")) != projectJourneyAdvancedSkill {
		t.Fatal("owned native skill does not match the exact fault recipe")
	}
	if err := os.RemoveAll(filepath.Join(f.devinRoot(f.a), "skills", "docs")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(managed, securityFixtureTree(t, active)) || projectJourneyRead(t, filepath.Join(f.devinRoot(f.a), "mcp_config.json")) != config {
		t.Fatal("native skill absence fault changed managed payload/MCP")
	}
	run("missing-repair-a", "repair", f.a, sourceA, "1.1.0", true)
	run("remove-a", "remove", f.a, sourceA, "1.1.0", true)
	before := projectJourneyRead(t, filepath.Join(f.namespace(f.a), "state-v2.json"))
	cache.start = time.Now()
	time.Sleep(20 * time.Millisecond)
	if _, err := f.execute(f.a, "remove", latest[f.a].installation.InstallationID, "--target", "windsurf"); err == nil || err.Error() != `plugin is not installed for target "windsurf" in project scope; no target was changed` {
		t.Fatal("direct repeated removal refusal differs", err)
	}
	cache.end = time.Now()
	if before != projectJourneyRead(t, filepath.Join(f.namespace(f.a), "state-v2.json")) || !reflect.DeepEqual(bTree, securityFixtureTree(t, f.b)) {
		t.Fatal("direct repeated removal changed state or B")
	}
	cache.observe(t, "repeat-remove-a", nil, latest[f.a].installation)
	afterA := securityFixtureTree(t, f.a)
	run("remove-b", "remove", f.b, sourceB, "1.0.0", true)
	if cache.stage != 10 || cache.scanner.calls != 3 {
		t.Fatal("exact cache lifecycle stage/scan ledger differs")
	}
	if !reflect.DeepEqual(afterA, securityFixtureTree(t, f.a)) {
		t.Fatal("B removal changed retained A")
	}
}

func devinDirectExecute(t *testing.T, f devinProjectJourney, project, verb, selector string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	app := f.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), &stdout, &stderr, false
	root := agentpluginscli.NewRoot(app)
	args := []string{verb, selector, "--scope", "project", "--project-root", project, "--target", "windsurf", "--format", "json"}
	if verb == "add" {
		args = append(args, "--no-save")
	}
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("direct %s: %v\n%s\n%s", verb, err, stdout.String(), stderr.String())
	}
	return stdout.Bytes()
}

func devinDirectBinding(t *testing.T, f devinProjectJourney, project, source, version string, previous, current devinDirectOwned, mutated bool) {
	t.Helper()
	i, b := current.installation, current.binding
	manifest := sha256.Sum256([]byte(projectJourneyRead(t, filepath.Join(source, "plugin.json"))))
	if i.DeclaredName != "docs" || i.Source.CanonicalSource != source || i.Source.RequestedSource != source || i.Package.Version != version || i.Package.ManifestDigest != "sha256:"+hex.EncodeToString(manifest[:]) || b.ClientID != "windsurf" || b.Scope != "project" || b.NativeProfileRoot != f.devinRoot(project) || b.Activation != domain.ActivationPrepared || b.Verification != domain.VerificationInstalled || b.PackageRevision == nil || b.PackageRevision.Version != version || b.PackageRevision.TreeDigest != i.Source.TreeDigest || b.PackageRevision.ManifestDigest != i.Package.ManifestDigest {
		t.Fatal("direct source/project/Prepared binding differs")
	}
	old := previous.binding
	if previous.installation.InstallationID != "" && (previous.installation.InstallationID != i.InstallationID || old.ClientBindingID != b.ClientBindingID || old.PhysicalArtifact != b.PhysicalArtifact || old.TargetLocator != b.TargetLocator || previous.installation.Source.SourceBindingID != i.Source.SourceBindingID) {
		t.Fatal("direct mutation changed held installation/binding/source identity")
	}
	added := 0
	if mutated {
		added = 1
	}
	if len(b.Receipts) != len(old.Receipts)+added || !reflect.DeepEqual(b.Receipts[:len(old.Receipts)], old.Receipts) && len(old.Receipts) != 0 {
		t.Fatal("direct mutation rewrote receipt prefix/count")
	}
	if mutated {
		r := b.Receipts[len(b.Receipts)-1]
		if r.MutationType != "directory_swap" || r.Phase != "committed" || r.Sequence != len(b.Receipts) || r.ClientBindingID != b.ClientBindingID || r.ActivePath != b.TargetLocator || r.OperationGroupID != i.OperationGroupID || r.OperationID != i.OperationGroupID+"-001" || r.AfterDigest == "" {
			t.Fatal("direct selected committed mutation receipt differs")
		}
	}
	if previous.installation.Package.Version != "" && previous.installation.Package.Version != version && previous.installation.Source.TreeDigest == i.Source.TreeDigest {
		t.Fatal("direct A version update did not advance source digest")
	}
	skill := projectJourneySkill
	if version == "1.1.0" {
		skill = projectJourneyAdvancedSkill
	}
	for _, root := range []string{b.TargetLocator, f.devinRoot(project)} {
		if projectJourneyRead(t, filepath.Join(root, "skills", "docs", "SKILL.md")) != skill {
			t.Fatal("direct managed/native selected skill bytes differ")
		}
	}
	devinDirectForeign(t, f, project)
}

func devinDirectForeign(t *testing.T, f devinProjectJourney, project string) {
	t.Helper()
	for _, path := range []string{filepath.Join(f.devinRoot(project), "skills", "foreign", "SKILL.md"), filepath.Join(f.devinRoot(project), "hooks.json")} {
		want := "foreign skill\n"
		if filepath.Base(path) == "hooks.json" {
			want = "foreign hooks\n"
		}
		if projectJourneyRead(t, path) != want {
			t.Fatal("direct mutation changed foreign native leaf")
		}
	}
}

func devinDirectRemoved(t *testing.T, f devinProjectJourney, project string, previous devinDirectOwned, state domain.StateFileV2) {
	t.Helper()
	i, b := state.Installations[0], previous.binding
	if i.InstallationID != previous.installation.InstallationID || !reflect.DeepEqual(i.Source, previous.installation.Source) || !reflect.DeepEqual(i.Package, previous.installation.Package) || len(i.Clients) != 0 || !i.DataRetained || len(i.DataReceipts) != 1 || len(state.TransactionReceipts) != 1 {
		t.Fatal("direct default removal retained-data state differs")
	}
	r := state.TransactionReceipts[0]
	if r.MutationType != "directory_remove" || r.Phase != "committed" || r.Sequence != len(b.Receipts)+1 || r.ClientBindingID != b.ClientBindingID || r.ActivePath != b.TargetLocator || r.OperationGroupID != i.OperationGroupID || r.OperationID != i.OperationGroupID+"-001" || r.BeforeDigest != b.Receipts[len(b.Receipts)-1].AfterDigest {
		t.Fatal("direct state-level removal receipt differs")
	}
	projectJourneyAbsent(t, b.TargetLocator, filepath.Join(f.devinRoot(project), "skills", "docs"))
	devinDirectForeign(t, f, project)
	var native map[string]any
	if json.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(f.devinRoot(project), "mcp_config.json"))), &native) != nil || native["foreignPreference"] != "keep" || !reflect.DeepEqual(native["mcpServers"], map[string]any{"foreign": map[string]any{"url": "https://foreign.test/mcp", "disabled": true}}) {
		t.Fatal("direct removal changed foreign native config or retained selected server")
	}
	for id, data := range i.DataReceipts {
		var owner domain.DataReceipt
		if id != data.DataReceiptID || data.PhysicalBackend != b.PhysicalArtifact || data.Scope != "project" || data.State != domain.DataReceiptOwned || data.Locator != filepath.Join(f.namespace(project), "plugin-data", b.PhysicalArtifact) || json.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(data.Locator, ".agentplugins-data-owner.json"))), &owner) != nil || !reflect.DeepEqual(owner, data) {
			t.Fatal("direct default removal ownership marker differs")
		}
	}
}

func devinDirectDTO(t *testing.T, raw []byte, stage, verb, version string, selected devinDirectOwned, mutated bool) {
	t.Helper()
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil || len(row) != 4 || row["schema_version"] != float64(1) || row["command"] != verb || row["result"] != "success" {
		t.Fatal("direct public envelope differs", stage)
	}
	data := row["data"].(map[string]any)
	output := data
	if stage == "repeat-add-a" {
		if data["targets"] != nil || data["operation_id"] != nil {
			t.Fatal("direct repeated add is not a singleton")
		}
	} else {
		targets := data["targets"].([]any)
		if data["batch"] != true || data["succeeded"] != float64(1) || data["failed"] != float64(0) || len(targets) != 1 || targets[0].(map[string]any)["target"] != "windsurf" || targets[0].(map[string]any)["status"] != "external_completed" {
			t.Fatal("direct selected batch differs", stage)
		}
		output = targets[0].(map[string]any)["output"].(map[string]any)
		if output["operation_id"] != data["operation_id"] || mutated && data["operation_id"] != selected.installation.OperationGroupID {
			t.Fatal("direct selected operation group differs", stage)
		}
		if verb == "remove" && (data["status"] != "data_retained" || data["data_retained"] != true || data["plugin_data_preserved"] != true) {
			t.Fatal("direct public default retention flags differ")
		}
	}
	source := "direct local source"
	if verb == "remove" || verb == "repair" {
		source = "local"
	}
	if output["plugin"] != "docs" || output["version"] != version || output["source"] != source || output["tree_digest"] != selected.installation.Source.TreeDigest || output["manifest_digest"] != selected.installation.Package.ManifestDigest || output["dry_run"] != false {
		t.Fatal("direct public source/revision differs", stage)
	}
	outcome := output["result"].(map[string]any)
	keys := strings.Fields("installation_id requires_confirmation mutated")
	if verb == "remove" {
		keys = append(keys, strings.Fields("plugin client_id affected_surfaces deactivation group_phase")...)
		if outcome["plugin"] != "docs" || outcome["client_id"] != "windsurf" || !reflect.DeepEqual(outcome["affected_surfaces"], []any{"windsurf"}) || !reflect.DeepEqual(outcome["deactivation"], map[string]any{"activation": "not_required", "artifact_removal_allowed": true, "external_removal_complete": true}) {
			t.Fatal("direct removal deactivation differs")
		}
	} else {
		keys = append(keys, "plan", "activation")
		if stage != "repeat-add-a" {
			keys = append(keys, "group_phase")
		}
		if stage == "repeat-add-a" || stage == "unchanged-update-a" {
			keys = append(keys, "no_change")
			if outcome["no_change"] != true {
				t.Fatal("direct no-op lost no_change")
			}
		}
		plan := outcome["plan"].(map[string]any)
		activation := outcome["activation"].(map[string]any)
		if plan["client_id"] != "windsurf" || plan["scope"] != "project" || plan["activation"] != "prepared" || plan["verification"] != "package_validated" || plan["physical_artifact_id"] != selected.binding.PhysicalArtifact || activation["activation"] != "prepared" || activation["authentication"] != "not_checked" || activation["policy"] != "allowed" || activation["verification"] != "installation_verified" {
			t.Fatal("direct Prepared result differs")
		}
		components := plan["components"].([]any)
		if len(components) != 2 {
			t.Fatal("direct supported component inventory differs")
		}
		kinds := map[string]bool{"skill": true, "mcp_server": true}
		for _, component := range components {
			row := component.(map[string]any)
			kind := row["kind"].(string)
			if len(row) != 3 || row["name"] != "docs" || row["support"] != "prepared" || !kinds[kind] {
				t.Fatal("direct project component differs", row)
			}
			delete(kinds, kind)
		}
	}
	if len(outcome) != len(keys) || outcome["installation_id"] != selected.installation.InstallationID || outcome["requires_confirmation"] != false || outcome["mutated"] != mutated || stage != "repeat-add-a" && outcome["group_phase"] != "external_completed" {
		t.Fatal("direct exact stage result shape differs", stage, outcome)
	}
	for _, key := range keys {
		if _, present := outcome[key]; !present {
			t.Fatal("direct stage result missing field", stage, key)
		}
	}
}
