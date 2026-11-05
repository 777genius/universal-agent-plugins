package installer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// Group updates need observational facts for unselected live bindings, not
// permission to mutate them. This is a focused SDK seam regression; a public
// three-client native update still requires separate end-to-end qualification.
func TestGroupUpdateUsesUnselectedKnownTargetChecks(t *testing.T) {
	s := newBothClientSandbox(t)
	id := "00000000-0000-4000-8000-000000000595"
	installBothClients(s.ctx, t, s.engine, s.pkg, s.probe, id, "before-group-compatibility", bothClientTargets(s.codexConfig, s.claudeConfig, s.probe))
	view, err := s.engine.Inspect(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	claudeBinding := inspectedBinding(t, view, "claude")
	registry, err := clients.NewRegistry(codex.New(), claude.New(), pi.New())
	if err != nil {
		t.Fatal(err)
	}
	s.engine.cfg.Registry = registry
	piRoot := filepath.Join(filepath.Dir(s.codexConfig), "pi-profile")
	if err := os.MkdirAll(piRoot, 0700); err != nil {
		t.Fatal(err)
	}
	selected := []usecase.AddInput{
		{Client: domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: s.codexConfig, ExecutablePath: s.probe}, BackendExecutable: s.probe},
		{Client: domain.DetectedClient{ClientID: domain.ClientPi, ConfigRoot: piRoot, ExecutablePath: s.probe}, BackendExecutable: s.probe},
	}
	req := Request{
		Operation: OpUpdate, InstallationID: id,
		Targets:      []ClientTarget{{ClientID: "codex", ClientConfigRoot: s.codexConfig, ClientExecutable: s.probe}, {ClientID: "pi", ClientConfigRoot: piRoot, ClientExecutable: s.probe}},
		KnownTargets: []TargetFacts{{ClientID: "claude", BindingID: claudeBinding.BindingID, ConfigRoot: s.claudeConfig, Executable: s.probe}},
	}
	original, err := os.ReadFile(s.engine.cfg.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := s.engine.groupCompatibilityChecks(req, selected)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 3 || checks[0].Client.ClientID != domain.ClientCodex || checks[1].Client.ClientID != domain.ClientPi ||
		checks[2].Client.ClientID != domain.ClientClaude || checks[2].Client.ConfigRoot != s.claudeConfig ||
		checks[2].BackendExecutable != s.probe {
		t.Fatalf("selected targets or observational sibling lost: %+v", checks)
	}
	for _, invalid := range []struct {
		name  string
		facts []TargetFacts
	}{
		{"missing", nil},
		{"wrong-binding", []TargetFacts{{ClientID: "claude", BindingID: "foreign-binding", ConfigRoot: s.claudeConfig, Executable: s.probe}}},
		{"duplicate", []TargetFacts{req.KnownTargets[0], req.KnownTargets[0]}},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			attempt := req
			attempt.KnownTargets = invalid.facts
			if _, err := s.engine.groupCompatibilityChecks(attempt, selected); !errors.Is(err, ErrTargetFactsUnavailable) {
				t.Fatalf("unverified live sibling admitted: %v", err)
			}
		})
	}
	// Existing two-client updates remain supported without redundant
	// KnownTargets for clients already selected by this group.
	bothSelected := []usecase.AddInput{
		selected[0],
		{Client: domain.DetectedClient{ClientID: domain.ClientClaude, ConfigRoot: s.claudeConfig, ExecutablePath: s.probe}, BackendExecutable: s.probe},
	}
	both := req
	both.Targets = []ClientTarget{{ClientID: "codex"}, {ClientID: "claude"}}
	both.KnownTargets = nil
	if got, err := s.engine.groupCompatibilityChecks(both, bothSelected); err != nil || len(got) != 2 {
		t.Fatalf("selected siblings wrongly require external KnownTargets: %+v %v", got, err)
	}
	for _, op := range []Operation{OpInstall, OpRepair} {
		unrelated := req
		unrelated.Operation, unrelated.KnownTargets = op, nil
		got, err := s.engine.groupCompatibilityChecks(unrelated, selected)
		if err != nil || len(got) != len(selected) {
			t.Fatalf("%s unexpectedly gained compatibility-only targets: %+v %v", op, got, err)
		}
	}
	after, err := os.ReadFile(s.engine.cfg.StateFile)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatalf("observational compatibility mutated UAP state: %v", err)
	}
}
