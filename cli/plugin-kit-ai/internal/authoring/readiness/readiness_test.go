package readiness

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

func TestTargetsRegistryAndBounds(t *testing.T) {
	for _, def := range domain.ClientDefinitions() {
		ids, err := Targets(string(def.ID))
		if err != nil || !reflect.DeepEqual(ids, []domain.ClientID{def.ID}) {
			t.Fatalf("registry %s: %v", def.ID, err)
		}
	}
	for _, bad := range []string{"", "all", "codex,codex", "codex,", ",codex", "Codex", " codex", "codex,secret-value", strings.Repeat("x", 4097)} {
		if _, err := Targets(bad); err != ErrTargets {
			t.Fatalf("expected fixed error for bounded invalid selection")
		}
	}
	ids, err := Targets("cursor,codex")
	if err != nil || ids[0] != domain.ClientCodex {
		t.Fatal(ids, err)
	}
}

func TestEngineSharedMetadataCopies(t *testing.T) {
	names := []string{"inspect", "capabilities"}
	c, err := Engine(names)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Profiles, conformance.ProfileIdentities()) {
		t.Fatal("profile drift")
	}
	if len(c.Clients) != len(domain.ClientDefinitions()) {
		t.Fatal("frozen registry")
	}
	for _, v := range c.Clients {
		want, ok := planner.Capabilities(v.ClientID)
		if !ok || !reflect.DeepEqual(v, want) {
			t.Fatal("client metadata drift")
		}
	}
	c.Commands[0] = "changed"
	c.Clients[0].MCPTransports["stdio"] = "changed"
	again, err := Engine(names)
	if err != nil || again.Commands[0] == "changed" || again.Clients[0].MCPTransports["stdio"] == "changed" || names[0] != "inspect" {
		t.Fatal("mutable metadata shared")
	}
	if len(c.Schemas) != 2 || c.Schemas[0].Digest == "" {
		t.Fatal("missing embedded metadata")
	}
}

func TestCompatibilityUnknownAndPlannerParity(t *testing.T) {
	got, err := Compatibility(project.Result{}, []domain.ClientID{domain.ClientCursor})
	if err != nil || got != nil {
		t.Fatal("guessed capabilities for unavailable schema")
	}
	p := project.Result{Facts: conformance.Facts{Package: &domain.PackageEnvelope{}}}
	ids := domain.SupportedClientIDs()
	got, err = Compatibility(p, ids)
	want, e := planner.Compatibility(*p.Facts.Package, ids)
	if err != nil || e != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("second compatibility semantics")
	}
}

func TestRetainedExecutableTraversal(t *testing.T) {
	entries := map[string]packageview.Observation{}
	for _, o := range []packageview.Observation{
		{Path: "bin", Kind: "directory", State: packageview.Present, Captured: true},
		{Path: "bin/server", Kind: "file", State: packageview.Present, Captured: true, Executable: true},
		{Path: "link", Kind: "symlink", Target: "bin/server", State: packageview.Present, Captured: true},
		{Path: "escape", Kind: "symlink", Target: "../outside", State: packageview.Present, Captured: true},
		{Path: "loop", Kind: "symlink", Target: "loop", State: packageview.Present, Captured: true},
		{Path: "blocked", Kind: "file", State: packageview.Blocked},
	} {
		entries[o.Path] = o
	}
	for _, tt := range []struct{ path, status string }{
		{"${PLUGIN_ROOT}/bin/server", "pass"}, {"./link", "pass"}, {"bin/../bin/server", "pass"},
		{"missing/../bin/server", "fail"}, {"bin/server/../server", "fail"}, {"./escape", "fail"},
		{"./loop", "not_evaluated"}, {"./blocked", "not_evaluated"}, {"${PLUGIN_DATA}/server", "not_evaluated"},
		{"/outside", "not_evaluated"}, {"C:\\outside", "not_evaluated"}, {"bin", "fail"},
	} {
		_, s := resolve(entries, tt.path, true)
		if s != tt.status {
			t.Fatalf("%s: %s != %s", tt.path, s, tt.status)
		}
	}
	if _, s := resolve(entries, "missing", false); s != "not_evaluated" {
		t.Fatal("incomplete absence invented")
	}
}

func TestDoctorMetadataOnlyAndNoLeak(t *testing.T) {
	p := project.Result{Facts: conformance.Facts{Package: &domain.PackageEnvelope{}}, Input: packageview.Input{Inventory: []packageview.Observation{
		{Path: "package.json", Kind: "file", State: packageview.Present, Captured: true},
		{Path: "package-lock.json", Kind: "file", State: packageview.Present, Captured: true},
		{Path: "secret-marker", Kind: "file", State: packageview.Present, Captured: true},
	}}}
	checks := Doctor(p)
	statuses := map[string]string{}
	for _, c := range checks {
		statuses[c.ID] = c.Status
	}
	if statuses["node_project_file"] != "pass" || statuses["node_lockfile_presence"] != "pass" || statuses["node_dependency_consistency"] != "not_evaluated" {
		t.Fatal(checks)
	}
	p.Input.Inventory = append(p.Input.Inventory, packageview.Observation{Path: "yarn.lock", Kind: "file", State: packageview.Present, Captured: true})
	for _, c := range Doctor(p) {
		if c.ID == "node_lockfile_presence" && c.Status != "not_evaluated" {
			t.Fatal("ambiguous manager passed")
		}
	}
	b, _ := json.Marshal(checks)
	if strings.Contains(string(b), "secret-marker") {
		t.Fatal("raw inventory leaked")
	}
	if len(Doctor(project.Result{Input: p.Input})) != 0 {
		t.Fatal("unknown schema native guesses")
	}
}
