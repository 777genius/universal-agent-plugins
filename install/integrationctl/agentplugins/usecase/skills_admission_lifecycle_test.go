package usecase

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func skillAdmissionInput(t *testing.T, client domain.DetectedClient, extra, revision string) AddInput {
	t.Helper()
	input := addInput(t, client, "https://example.test/skill-admission")
	if revision == "invalid" {
		manifest := filepath.Join(input.Envelope.SnapshotRoot, "plugin.json")
		body, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifest, []byte(strings.Replace(string(body), `"version": "1.0.0"`, `"version": "2.0.0"`, 1)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	root := input.Envelope.SnapshotRoot
	for name, fields := range map[string]string{"good": "", "candidate": extra} {
		dir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: Test skill\n"+fields+"---\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(`{"$schema":"`+domain.MCPSchemaV1+`","mcpServers":{"remote":{"type":"streamable-http","url":"https://example.test/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	source := input.Envelope.Source
	source.ResolvedRevision = revision
	input.Envelope, err = (loader.Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: root, Source: source, TreeDigest: "sha256:" + revision})
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func assertSkillAdmissionArtifact(t *testing.T, active string, candidate bool) {
	t.Helper()
	for name, present := range map[string]bool{"good": true, "candidate": candidate} {
		_, err := os.Stat(filepath.Join(active, "skills", name, "SKILL.md"))
		if (present && err != nil) || (!present && !os.IsNotExist(err)) {
			t.Fatalf("skill %s present=%v: %v", name, present, err)
		}
	}
	mcp := readUsecaseObject(t, filepath.Join(active, "mcp.json"))
	if servers, ok := mcp["mcpServers"].(map[string]any); !ok || len(servers) != 1 || servers["remote"] == nil {
		t.Fatalf("healthy MCP lost: %+v", mcp)
	}
}

func TestSkillAdmissionInstallAndUpdate(t *testing.T) {
	for _, extra := range []string{"metadata: {count: 5}\n", "metadata: {nested: {value: x}}\n", "compatibility: ''\n"} {
		t.Run(extra, func(t *testing.T) {
			service, store, client := serviceFixture(t)
			invalid := skillAdmissionInput(t, client, extra, "invalid")
			invalid.Confirmed = true
			installed, err := service.Add(context.Background(), invalid)
			if err != nil {
				t.Fatal(err)
			}
			assertSkillAdmissionArtifact(t, installed.Plan.ActivePath, false)

			// A separately installed valid revision can be replaced only after confirmation.
			service, store, client = serviceFixture(t)
			valid := skillAdmissionInput(t, client, "", "valid")
			valid.Confirmed = true
			installed, err = service.Add(context.Background(), valid)
			if err != nil {
				t.Fatal(err)
			}
			candidate := skillAdmissionInput(t, client, extra, "invalid")
			candidate.InstallationID = installed.InstallationID
			candidate.OperationID = "skill-update"
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			preview, err := service.Update(context.Background(), candidate)
			if err != nil || preview.Mutated || !preview.RequiresConfirmation {
				t.Fatalf("preview=%+v err=%v", preview, err)
			}
			assertSkillAdmissionArtifact(t, installed.Plan.ActivePath, true)
			after, err := store.Load()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("preview changed state: %v", err)
			}
			candidate.Confirmed = true
			updated, err := service.Update(context.Background(), candidate)
			if err != nil || !updated.Mutated {
				t.Fatalf("update=%+v err=%v", updated, err)
			}
			assertSkillAdmissionArtifact(t, updated.Plan.ActivePath, false)
		})
	}
}

func TestInvalidSkillAdmissionLifecycleHistoricalRepair(t *testing.T) {
	service, store, client := serviceFixture(t)
	input := skillAdmissionInput(t, client, "metadata: {count: 5}\n", "historical")
	// Model the old loader's admission of these exact invalid bytes, then repair
	// the same source revision using the corrected loader result.
	current := input.Envelope
	raw, err := os.ReadFile(filepath.Join(current.SnapshotRoot, "skills", "candidate", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	input.Envelope.Skills = map[string]domain.Skill{"good": current.Skills["good"], "candidate": {Name: "candidate", Description: "Test skill", Metadata: map[string]any{"count": 5}, RelativePath: "skills/candidate/SKILL.md", Raw: raw}}
	input.Envelope.Inventory.InvalidSkills = nil
	input.Envelope.Diagnostics = nil
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	assertSkillAdmissionArtifact(t, installed.Plan.ActivePath, true)
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	// Damage an owned artifact so exact-revision repair must rebuild it.
	if err := os.WriteFile(filepath.Join(installed.Plan.ActivePath, "damage"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	input.Envelope = current
	input.InstallationID = installed.InstallationID
	input.OperationID = "historical-skill-repair"
	repaired, err := service.Repair(context.Background(), input)
	if err == nil || !strings.Contains(err.Error(), "resolved repair projection digest differs") || repaired.Mutated {
		t.Fatalf("historical repair silently changed selection: %+v %v", repaired, err)
	}
	assertSkillAdmissionArtifact(t, installed.Plan.ActivePath, true)
	after, loadErr := store.Load()
	if loadErr != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("refused repair changed state: %v", loadErr)
	}
}
