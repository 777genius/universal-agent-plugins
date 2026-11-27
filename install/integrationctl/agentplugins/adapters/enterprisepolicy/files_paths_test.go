package enterprisepolicy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func policyAncestorLink(t *testing.T) (actual, linked string) {
	t.Helper()
	root := t.TempDir()
	target := filepath.Join(root, "actual")
	actual = filepath.Join(target, "config")
	if err := os.MkdirAll(actual, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	return actual, filepath.Join(link, "config")
}

func requireUnsafePolicyParent(t *testing.T, err error) {
	t.Helper()
	var config *ConfigError
	if !errors.As(err, &config) || config.Code != "unsafe_parent_directory" {
		t.Fatalf("expected unsafe policy ancestor refusal, got %v", err)
	}
}

func TestPolicyFilesRejectLinksAboveImmediateParent(t *testing.T) {
	t.Parallel()
	actual, linked := policyAncestorLink(t)
	writePolicy(t, filepath.Join(actual, "policy.yaml"), "schema_version: 2\n")
	path := filepath.Join(linked, "policy.yaml")
	_, err := ReadFile(path)
	requireUnsafePolicyParent(t, err)
	for _, required := range []bool{false, true} {
		snapshot, err := (Files{Paths: []Path{{Layer: "deployment", Path: path, Required: required}}}).Load(context.Background())
		requireUnsafePolicyParent(t, err)
		if len(snapshot.Layers) != 1 || !snapshot.Layers[0].Present || snapshot.Layers[0].Path != path {
			t.Fatalf("configured policy location lost: %+v", snapshot)
		}
	}
}

func TestPolicyFilesAbsentOptionalLayerDoesNotRequireSafeAncestry(t *testing.T) {
	t.Parallel()
	actual, linked := policyAncestorLink(t)
	path := filepath.Join(linked, "policy.yaml")
	ctx := context.Background()
	files := Files{Paths: []Path{{Layer: "user", Path: path}}}
	snapshot, err := files.Load(ctx)
	if err != nil || len(snapshot.Layers) != 1 || snapshot.Layers[0].Present || snapshot.RestrictionCount != 0 || snapshot.Fingerprint == "" {
		t.Fatalf("absent optional layer changed no-policy behavior: %+v %v", snapshot, err)
	}
	session := NewSession(files)
	if _, err := session.Check(ctx, domain.InstallPolicyRequest{}); err != nil {
		t.Fatal(err)
	}
	_, err = (Files{Paths: []Path{{Layer: "deployment", Path: path, Required: true}}}).Load(ctx)
	requireUnsafePolicyParent(t, err)
	_, err = ReadFile(path)
	requireUnsafePolicyParent(t, err)
	writePolicy(t, filepath.Join(actual, "policy.yaml"), "schema_version: 2\n")
	_, err = session.Check(ctx, domain.InstallPolicyRequest{})
	requireUnsafePolicyParent(t, err)
}

func TestPolicyFilesDanglingLeafIsPresentAndRefused(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "policy.yaml")
	if err := os.Symlink(filepath.Join(root, "missing.yaml"), path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, required := range []bool{false, true} {
		snapshot, err := (Files{Paths: []Path{{Layer: "user", Path: path, Required: required}}}).Load(context.Background())
		var config *ConfigError
		if !errors.As(err, &config) || config.Code != "unreadable_or_unsafe_file" || len(snapshot.Layers) != 1 || !snapshot.Layers[0].Present {
			t.Fatalf("dangling policy link was treated as absent: %+v %v", snapshot, err)
		}
	}
}

func TestPolicySessionRejectsIntroducedAncestorLink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parent := filepath.Join(root, "trusted")
	if err := os.MkdirAll(filepath.Join(parent, "config"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "config", "policy.yaml")
	writePolicy(t, path, "schema_version: 2\n")
	session := NewSession(Files{Paths: []Path{{Layer: "machine", Path: path}}})
	if _, err := session.Check(context.Background(), domain.InstallPolicyRequest{}); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(root, "moved")
	if err := os.Rename(parent, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, parent); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := session.Check(context.Background(), domain.InstallPolicyRequest{})
	requireUnsafePolicyParent(t, err)
}

func TestPolicySessionRejectsPolicyHiddenByAncestorRedirect(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parent, empty := filepath.Join(root, "trusted"), filepath.Join(root, "empty")
	for _, path := range []string{parent, empty} {
		if err := os.MkdirAll(filepath.Join(path, "config"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(parent, "config", "policy.yaml")
	writePolicy(t, path, "schema_version: 2\ninstall:\n  allowed_scopes: [user]\n")
	ctx, req := context.Background(), domain.InstallPolicyRequest{Scope: domain.ScopeUser}
	files := Files{Paths: []Path{{Layer: "machine", Path: path}}}
	session := NewSession(files)
	if _, err := session.Check(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parent, filepath.Join(root, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(empty, parent); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := session.Check(ctx, req); !errors.Is(err, domain.ErrInstallPolicyChanged) {
		t.Fatalf("ancestor redirect hid reviewed restrictions without invalidating review: %v", err)
	}
	_, err := (Files{Paths: []Path{{Layer: "deployment", Path: path, Required: true}}}).Load(ctx)
	requireUnsafePolicyParent(t, err)
	// A new operation discovers optional files afresh; requiring a deployment
	// to remain present belongs to its protected required-file configuration.
	if _, err := NewSession(files).Check(ctx, req); err != nil {
		t.Fatalf("new operation changed optional-absent compatibility: %v", err)
	}
}

func TestDarwinPolicyPathCanonicalizesOnlySystemAliases(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ path, want string }{
		{"/var/folders/policy.yaml", "/private/var/folders/policy.yaml"},
		{"/tmp/uap/policy.yaml", "/private/tmp/uap/policy.yaml"},
		{"/etc/agentplugins/policy.yaml", "/private/etc/agentplugins/policy.yaml"},
		{"/var", "/private/var"},
		{"/variable/policy.yaml", "/variable/policy.yaml"},
		{"/tmp-user/policy.yaml", "/tmp-user/policy.yaml"},
		{"/etcetera/policy.yaml", "/etcetera/policy.yaml"},
		{"/trusted/var/policy.yaml", "/trusted/var/policy.yaml"},
		{"/private/var/folders/policy.yaml", "/private/var/folders/policy.yaml"},
	} {
		if got := darwinPolicyPath(tc.path); got != tc.want {
			t.Errorf("Darwin policy path %q = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestPolicyFilesDarwinSystemAliasesPreserveConfiguredLocation(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" {
		t.Skip("macOS fixed system directory aliases")
	}
	for _, base := range []string{"/tmp", "/var/tmp"} {
		t.Run(base, func(t *testing.T) {
			root, err := os.MkdirTemp(base, "uap-policy-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(root) })
			path := filepath.Join(root, "policy.yaml")
			writePolicy(t, path, "schema_version: 2\n")
			files := Files{Paths: []Path{{Layer: "user", Path: path, Required: true}, {Layer: "project", Path: filepath.Join(root, "absent.yaml")}}}
			snapshot, err := files.Load(context.Background())
			if err != nil || len(snapshot.Layers) != 2 || !snapshot.Layers[0].Present || snapshot.Layers[1].Present || snapshot.Layers[0].Path != path {
				t.Fatalf("macOS system alias changed policy location: %+v %v", snapshot, err)
			}
		})
	}
}
