package pathpolicy

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestNormalizeScope(t *testing.T) {
	t.Parallel()
	if got := NormalizeScope("project"); got != "project" {
		t.Fatalf("NormalizeScope(project) = %q", got)
	}
	if got := NormalizeScope(" user "); got != "user" {
		t.Fatalf("NormalizeScope(user) = %q", got)
	}
}

func TestProjectRootPrefersWorkspaceThenProjectRoot(t *testing.T) {
	t.Parallel()
	if got := ProjectRoot("/tmp/workspace", "/tmp/project"); got != filepath.Clean("/tmp/workspace") {
		t.Fatalf("ProjectRoot workspace = %q", got)
	}
	if got := ProjectRoot("", "/tmp/project"); got != filepath.Clean("/tmp/project") {
		t.Fatalf("ProjectRoot project = %q", got)
	}
}

func TestEffectiveGitRootWalksToRepositoryBoundary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, "nested", "repo")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := EffectiveGitRoot(workspace, ""); got != root {
		t.Fatalf("EffectiveGitRoot = %q, want %q", got, root)
	}
}

func TestPreferredExistingPathPrefersExistingCandidate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	first := filepath.Join(root, "one.json")
	second := filepath.Join(root, "two.json")
	if err := os.WriteFile(second, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := PreferredExistingPath(first, second); got != second {
		t.Fatalf("PreferredExistingPath = %q, want %q", got, second)
	}
}

func TestWorkspaceRootFromRecordProjectOnly(t *testing.T) {
	t.Parallel()
	project := domain.InstallationRecord{
		Policy:        domain.InstallPolicy{Scope: "project"},
		WorkspaceRoot: "/tmp/workspace",
	}
	if got := WorkspaceRootFromRecord(project); got != "/tmp/workspace" {
		t.Fatalf("WorkspaceRootFromRecord(project) = %q", got)
	}
	user := domain.InstallationRecord{
		Policy:        domain.InstallPolicy{Scope: "user"},
		WorkspaceRoot: "/tmp/workspace",
	}
	if got := WorkspaceRootFromRecord(user); got != "" {
		t.Fatalf("WorkspaceRootFromRecord(user) = %q", got)
	}
}

func TestWorkspaceRootFromInputsUsesRecordOnly(t *testing.T) {
	t.Parallel()
	record := domain.InstallationRecord{
		Policy:        domain.InstallPolicy{Scope: "project"},
		WorkspaceRoot: "/tmp/workspace",
	}
	if got := WorkspaceRootFromInspect(ports.InspectInput{Record: &record}); got != "/tmp/workspace" {
		t.Fatalf("WorkspaceRootFromInspect = %q", got)
	}
	if got := WorkspaceRootFromApply(ports.ApplyInput{Record: &record}); got != "/tmp/workspace" {
		t.Fatalf("WorkspaceRootFromApply = %q", got)
	}
}

func TestProtectionForScope(t *testing.T) {
	t.Parallel()
	if got := ProtectionForScope("project"); got != domain.ProtectionWorkspace {
		t.Fatalf("ProtectionForScope(project) = %q", got)
	}
	if got := ProtectionForScope("user"); got != domain.ProtectionUserMutable {
		t.Fatalf("ProtectionForScope(user) = %q", got)
	}
}

func TestValidateLeafID(t *testing.T) {
	t.Parallel()
	for _, valid := range []string{"context7", "agent-code-navigator", "legacy_name", "v1.2"} {
		if err := ValidateLeafID(valid); err != nil {
			t.Errorf("ValidateLeafID(%q): %v", valid, err)
		}
	}
	for _, invalid := range []string{"", ".", "..", "../escape", `..\\escape`, "/tmp/escape", `C:\\escape`, "name..part", "name.", "CON", "nul.txt", "has space", " id ", "\\tid", "id\\n", " CON", "valid\\r\\n"} {
		if err := ValidateLeafID(invalid); err == nil {
			t.Errorf("ValidateLeafID(%q) succeeded, want rejection", invalid)
		}
	}
}

func TestValidatePortablePathSegmentRejectsOverlongUTF16Name(t *testing.T) {
	t.Parallel()
	if err := ValidatePortablePathSegment(strings.Repeat("a", 256)); err == nil {
		t.Fatal("256-code-unit path segment was accepted")
	}
}

func TestRequireContainedChild(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := filepath.Join(root, "managed")
	child := filepath.Join(base, "plugins", "context7")
	if err := RequireContainedChild(base, child); err != nil {
		t.Fatalf("contained child rejected: %v", err)
	}
	for _, path := range []string{base, root, filepath.Join(root, "outside")} {
		if err := RequireContainedChild(base, path); err == nil {
			t.Errorf("RequireContainedChild(%q, %q) succeeded, want rejection", base, path)
		}
	}
}

func TestRequireContainedChildRejectsSymlinkAncestor(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available without elevated privileges")
	}
	root := t.TempDir()
	base := filepath.Join(root, "managed")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	if err := RequireContainedChild(base, filepath.Join(base, "link", "victim")); err == nil {
		t.Fatal("symlink ancestor accepted")
	}
}

func TestRequireExactPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	expected := filepath.Join(root, "managed", "context7")
	if err := RequireExactPath(expected, expected); err != nil {
		t.Fatalf("exact path rejected: %v", err)
	}
	if err := RequireExactPath(expected, filepath.Join(root, "outside")); err == nil {
		t.Fatal("different path accepted")
	}
}
