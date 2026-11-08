package readiness

import (
	"encoding/json"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func doctorEvidenceFile(path string) packageview.Observation {
	return packageview.Observation{Path: path, Kind: "file", State: packageview.Present, Captured: true}
}

func doctorEvidenceByID(checks []Check) map[string]Check {
	out := make(map[string]Check, len(checks))
	for _, check := range checks {
		out[check.ID] = check
	}
	return out
}

func TestDoctorLanguageLockEvidenceDoesNotGuessConsistency(t *testing.T) {
	p := project.Result{
		Facts: conformance.Facts{Package: &domain.PackageEnvelope{}},
		Input: packageview.Input{Inventory: []packageview.Observation{
			doctorEvidenceFile("pyproject.toml"),
			doctorEvidenceFile("uv.lock"),
			doctorEvidenceFile("poetry.lock"),
			doctorEvidenceFile("go.mod"),
			doctorEvidenceFile("go.sum"),
		}},
	}
	got := Doctor(p)
	byID := doctorEvidenceByID(got)
	if len(got) != 6 || byID["python_project_file"].Status != "pass" ||
		byID["python_lockfile_presence"].Status != "not_evaluated" ||
		byID["go_project_file"].Status != "pass" ||
		byID["go_lockfile_presence"].Status != "pass" ||
		byID["go_dependency_consistency"].Status != "not_evaluated" {
		t.Fatalf("language evidence must not infer a working dependency graph: %+v", got)
	}
	if !strings.Contains(byID["python_lockfile_presence"].Action, "Multiple manager lockfiles") {
		t.Fatal("ambiguous Python lock manager lost its public guidance")
	}
	p.Input.Inventory = p.Input.Inventory[:1:1]
	p.Input.Inventory = append(p.Input.Inventory, doctorEvidenceFile("uv.lock"), doctorEvidenceFile("go.sum"))
	got = Doctor(p)
	byID = doctorEvidenceByID(got)
	if byID["python_lockfile_presence"].Status != "pass" ||
		byID["python_dependency_consistency"].Status != "not_evaluated" ||
		byID["go_project_file"].ID != "" || len(got) != 3 {
		t.Fatalf("one captured Python lock must not imply Go readiness: %+v", got)
	}
}

func TestDoctorHostEvidenceIsSortedHashedAndInert(t *testing.T) {
	const privateName = "DO_NOT_LEAK_HOST_TOKEN"
	pkg := &domain.PackageEnvelope{
		Skills: map[string]domain.Skill{
			"z-" + privateName: {Compatibility: "requires-host"},
			"a":                {AllowedTools: "host-tool"},
			"unconstrained":    {},
		},
		MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
			"z-remote-" + privateName: {Type: "streamable-http", Decoded: map[string]any{"url": "https://user:password@example.invalid/mcp"}},
			"a-bundled":               {Type: "stdio", Decoded: map[string]any{"command": "./bin/runner"}},
			"b-path":                  {Type: "stdio", Decoded: map[string]any{"command": "node"}},
		}},
	}
	p := project.Result{
		Facts: conformance.Facts{Package: pkg},
		Input: packageview.Input{Inventory: []packageview.Observation{
			{Path: "bin", Kind: "directory", State: packageview.Present, Captured: true},
			{Path: "bin/runner", Kind: "file", State: packageview.Present, Captured: true, Executable: true},
		}},
	}
	p.Input.Coverage.InventoryComplete = true
	got := Doctor(p)
	if len(got) != 11 {
		t.Fatalf("unexpected static Skill/MCP evidence count: %+v", got)
	}
	byKey := make(map[string]Check, len(got))
	for i, check := range got {
		byKey[check.ID+"|"+check.ItemID] = check
		if i > 0 && (got[i-1].ID > check.ID ||
			(got[i-1].ID == check.ID && got[i-1].ItemID > check.ItemID)) {
			t.Fatalf("Doctor output is not canonically ordered: %+v", got)
		}
	}
	skillID := itemID("skill:z-" + privateName)
	if byKey["skill_host_requirements|"+skillID].Status != "not_evaluated" ||
		byKey["skill_host_requirements|"+itemID("skill:a")].Status != "not_evaluated" {
		t.Fatal("Skill requirements must not assume host support")
	}
	remoteID := itemID("mcp:z-remote-" + privateName)
	if byKey["remote_connection|"+remoteID].Status != "not_evaluated" ||
		byKey["oauth|"+remoteID].Status != "not_evaluated" {
		t.Fatal("remote MCP auth cannot be validated statically")
	}
	bundledID := itemID("mcp:a-bundled")
	if byKey["bundled_executable_presence|"+bundledID].Status != "pass" {
		t.Fatal("captured bundled file was not recognized")
	}
	wantMode := "pass"
	if runtime.GOOS == "windows" {
		wantMode = "not_evaluated"
	}
	if byKey["bundled_executable_mode|"+bundledID].Status != wantMode ||
		byKey["runtime|"+bundledID].Status != "not_evaluated" ||
		byKey["path_executable_availability|"+itemID("mcp:b-path")].Status != "not_evaluated" {
		t.Fatal("MCP executable status must not invoke host processes")
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{privateName, "password", "./bin/runner"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("raw source or secret leaked through public Doctor checks: %q", secret)
		}
	}
	p.Input.Inventory[0], p.Input.Inventory[1] = p.Input.Inventory[1], p.Input.Inventory[0]
	if !reflect.DeepEqual(got, Doctor(p)) {
		t.Fatal("inventory order or map iteration changed public Doctor output")
	}
}
