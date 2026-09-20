package platformexec

import "testing"

func TestResolveOpenCodeUserScopeConfigDisabledReturnsNoConfig(t *testing.T) {
	t.Parallel()
	_, ok, err := resolveOpenCodeUserScopeConfig(ImportSeed{})
	if err != nil {
		t.Fatalf("resolveOpenCodeUserScopeConfig: %v", err)
	}
	if ok {
		t.Fatal("expected user scope to be disabled")
	}
}

func TestResolveOpenCodeProjectScopeConfigUsesWorkspaceRoot(t *testing.T) {
	t.Parallel()
	cfg, err := resolveOpenCodeProjectScopeConfig("/tmp/demo")
	if err != nil {
		t.Fatalf("resolveOpenCodeProjectScopeConfig: %v", err)
	}
	if cfg.workspaceRoot != "/tmp/demo/.opencode" {
		t.Fatalf("workspaceRoot = %q", cfg.workspaceRoot)
	}
}
