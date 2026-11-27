package enterprisepolicy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func writePolicy(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFilesConjoinMachineAndProjectWithoutOverrides(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	machine, project := filepath.Join(root, "machine.yaml"), filepath.Join(root, "project.yaml")
	writePolicy(t, machine, "schema_version: 2\nsources:\n  deny_hosts: [github.com]\n")
	writePolicy(t, project, "schema_version: 2\nsources:\n  allow_hosts: [github.com]\n")
	gate := Files{Paths: []Path{{Layer: "machine", Path: machine}, {Layer: "project", Path: project}}}
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		_, err := gate.Check(context.Background(), domain.InstallPolicyRequest{Scope: scope, Source: domain.SourceIdentity{Repository: "acme/safe"}})
		var refusal *domain.InstallPolicyError
		if !errors.As(err, &refusal) || refusal.Violations[0].Layer != "machine" || refusal.Violations[0].PolicyPath != machine {
			t.Fatalf("machine denial lost for %s: %v", scope, err)
		}
	}
	if err := os.Remove(machine); err != nil {
		t.Fatal(err)
	}
	gate.Paths[0].Required = true
	partial, err := gate.Load(context.Background())
	var config *ConfigError
	if !errors.As(err, &config) || config.Layer != "machine" || len(partial.Layers) != 1 {
		t.Fatalf("missing required source must fail with partial diagnostic snapshot: %+v %v", partial, err)
	}
}

func TestPolicySessionFingerprintTracksRulesAndLayerPresence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	gate := Files{Paths: []Path{{Layer: "machine", Path: path}}}
	ctx := context.Background()
	req := domain.InstallPolicyRequest{Source: domain.SourceIdentity{Repository: "acme/safe"}}
	session := NewSession(gate)
	if _, err := session.Check(ctx, req); err != nil {
		t.Fatal(err)
	}
	writePolicy(t, path, "schema_version: 2\nsources:\n  allow_hosts: [github.com, other.test]\n")
	if _, err := session.Check(ctx, req); !errors.Is(err, domain.ErrInstallPolicyChanged) {
		t.Fatalf("policy appearance escaped retained review: %v", err)
	}
	session = NewSession(gate)
	first, err := session.Check(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	writePolicy(t, path, "# review-only edit\nschema_version: 2\nsources:\n  allow_hosts: [other.test, github.com, github.com]\n")
	next, err := session.Check(ctx, req)
	if err != nil || next != first {
		t.Fatalf("semantic-equivalent edit invalidated review: %q %q %v", first, next, err)
	}
	writePolicy(t, path, "schema_version: 2\nsources:\n  allow_hosts: [github.com]\n")
	if _, err := session.Check(ctx, req); !errors.Is(err, domain.ErrInstallPolicyChanged) {
		t.Fatalf("changed rules escaped review: %v", err)
	}
	session = NewSession(gate)
	if _, err := session.Check(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Check(ctx, req); !errors.Is(err, domain.ErrInstallPolicyChanged) {
		t.Fatalf("policy deletion escaped retained review: %v", err)
	}
	if _, err := NewSession(gate).Check(ctx, req); err != nil {
		t.Fatalf("new operation inherited stale session: %v", err)
	}
}

func TestPolicyFilesRefuseUnsafeOrMalformedConfiguredInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "actual.yaml")
	writePolicy(t, path, "schema_version: 2\n")
	link := filepath.Join(root, "policy-link.yaml")
	if err := os.Symlink(path, link); err == nil {
		if _, err := ReadFile(link); err == nil {
			t.Fatal("policy leaf symlink accepted")
		}
		parentLink := filepath.Join(root, "policy-parent")
		if err := os.Symlink(root, parentLink); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadFile(filepath.Join(parentLink, "actual.yaml")); err == nil {
			t.Fatal("policy parent symlink accepted")
		}
	}
	if _, err := ReadFile(root); err == nil {
		t.Fatal("policy directory accepted")
	}
	writePolicy(t, path, "schema_version: 2\nsources: null\n")
	if _, err := (Files{Paths: []Path{{Layer: "machine", Path: path}}}).Load(context.Background()); err == nil {
		t.Fatal("optional but present invalid policy was ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Files{Paths: []Path{{Layer: "machine", Path: path}}}).Load(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not preserved: %v", err)
	}
}

type sourceSensitiveGate struct{ calls int }

func (g *sourceSensitiveGate) Check(_ context.Context, req domain.InstallPolicyRequest) (string, error) {
	g.calls++
	if req.Source.Repository != "" {
		return "custom", &domain.InstallPolicyError{Violations: []domain.InstallPolicyViolation{{Code: "custom", Subject: "source", Message: "custom source refusal"}}}
	}
	return "custom", nil
}

func TestUnrestrictedInspectionNeverSkipsACustomGate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	custom := &sourceSensitiveGate{}
	for _, gate := range []interface {
		Check(context.Context, domain.InstallPolicyRequest) (string, error)
	}{custom, NewSession(custom)} {
		free, err := IsUnrestricted(ctx, gate)
		if err != nil || free || custom.calls != 0 {
			t.Fatalf("custom policy was probed or skipped: %v %v %d", free, err, custom.calls)
		}
	}
	path := filepath.Join(t.TempDir(), "managed.yaml")
	gate := NewSession(Files{Paths: []Path{{Layer: "machine", Path: path}}})
	free, err := IsUnrestricted(ctx, gate)
	if err != nil || !free {
		t.Fatalf("absent policy did not permit optimization: %v %v", free, err)
	}
	writePolicy(t, path, "schema_version: 2\nsources:\n  deny_hosts: [bad.test]\n")
	if _, err := IsUnrestricted(ctx, gate); !errors.Is(err, domain.ErrInstallPolicyChanged) {
		t.Fatalf("optimization failed to retain reviewed absence: %v", err)
	}
}

func TestNewOperationUnwrapsPriorSessionWithoutChangingItsReview(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	writePolicy(t, path, "schema_version: 2\nsources:\n  deny_hosts: [bad.test]\n")
	ctx := context.Background()
	req := domain.InstallPolicyRequest{Source: domain.SourceIdentity{Repository: "acme/safe"}}
	old := NewSession(Files{Paths: []Path{{Layer: "machine", Path: path}}})
	if _, err := old.Check(ctx, req); err != nil {
		t.Fatal(err)
	}
	writePolicy(t, path, "schema_version: 2\nsources:\n  allow_hosts: [github.com]\n")
	fresh := NewSession(old)
	if _, err := fresh.Check(ctx, req); err != nil {
		t.Fatalf("new operation inherited stale retained review: %v", err)
	}
	if _, err := old.Check(ctx, req); !errors.Is(err, domain.ErrInstallPolicyChanged) {
		t.Fatalf("fresh operation changed the old review authority: %v", err)
	}
}
