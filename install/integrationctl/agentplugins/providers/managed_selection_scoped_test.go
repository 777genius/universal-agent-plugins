package providers

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kimi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func scopedSelectionArtifact(t *testing.T, files map[string]string) (Stager, string, string) {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	stager := testStager(Stager{})
	var mismatch *ports.VerificationError
	if err := stager.Verify(t.Context(), root, "observe"); !errors.As(err, &mismatch) {
		t.Fatal("actual staged-artifact digest was not observed", err)
	}
	return stager, root, mismatch.ActualDigest
}

func TestScopedManagedMCPSelectionDoesNotInferLayoutFromFiles(t *testing.T) {
	stager, root, digest := scopedSelectionArtifact(t, map[string]string{
		"mcp.json":                 `{"mcpServers":{"project-only":{}}}`,
		".kimi-plugin/plugin.json": `{"name":"demo","mcpServers":{"user-only":{}}}`,
	})
	for scope, name := range map[domain.InstallScope]string{domain.ScopeUser: "user-only", domain.ScopeProject: "project-only"} {
		names, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, scope, root, digest)
		if err != nil || !reflect.DeepEqual(names, []string{name}) {
			t.Fatalf("explicit scope %q names=%v err=%v", scope, names, err)
		}
	}
	names, err := stager.ManagedMCPNames(t.Context(), domain.ClientKimi, root, digest)
	if err != nil || !reflect.DeepEqual(names, []string{"user-only"}) {
		t.Fatal("legacy API no longer reads the original user manifest", names, err)
	}
	for _, scope := range []domain.InstallScope{"", "unknown"} {
		if _, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, scope, root, digest); err == nil {
			t.Fatal("unknown scope selected an artifact layout")
		}
	}
}

func TestScopedManagedMCPSelectionRequiresSelectedDocumentAndArtifact(t *testing.T) {
	for _, body := range []string{`{"servers":{}}`, `null`, `[]`} {
		stager, root, digest := scopedSelectionArtifact(t, map[string]string{
			"mcp.json": body, ".kimi-plugin/plugin.json": `{"mcpServers":{"alternate":{}}}`,
		})
		if _, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeProject, root, digest); err == nil {
			t.Fatal("malformed selected document fell back to the alternate")
		}
	}
	stager, root, digest := scopedSelectionArtifact(t, map[string]string{"mcp.json": `{"mcpServers":{"held":{}}}`})
	if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(`{"mcpServers":{"changed":{}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeProject, root, digest); err == nil {
		t.Fatal("selection trusted damaged artifact bytes")
	}
}

func TestScopedManagedMCPSelectionKeepsEmptyAndOtherAdapterPolicies(t *testing.T) {
	stager, root, digest := scopedSelectionArtifact(t, map[string]string{".kimi-plugin/plugin.json": `{"name":"skills-only"}`})
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		names, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, scope, root, digest)
		if err != nil || names == nil || len(names) != 0 {
			t.Fatal("verified skills-only selection differs", scope, names, err)
		}
	}
	stager, root, digest = scopedSelectionArtifact(t, map[string]string{"mcp.json": `{"mcpServers":{"z":{},"a":{}}}`})
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		names, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientCursor, scope, root, digest)
		if err != nil || !reflect.DeepEqual(names, []string{"a", "z"}) {
			t.Fatal("adapter without scoped policy changed", scope, names, err)
		}
	}
}

type selectionPolicyAdapter struct {
	*kimi.Adapter
	choose func(domain.InstallScope) (clients.SelectionLayout, error)
}

func (a selectionPolicyAdapter) ManagedMCPSelectionForScope(scope domain.InstallScope) (clients.SelectionLayout, error) {
	return a.choose(scope)
}

func TestScopedManagedMCPSelectionVerifiesBeforePolicyAndAfterRead(t *testing.T) {
	stager, root, digest := scopedSelectionArtifact(t, map[string]string{"mcp.json": `{"mcpServers":{"held":{}}}`})
	calls := 0
	adapter := selectionPolicyAdapter{Adapter: kimi.New(), choose: func(scope domain.InstallScope) (clients.SelectionLayout, error) {
		calls++
		if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(`{"mcpServers":{"changed":{}}}`), 0600); err != nil {
			return clients.SelectionLayout{}, err
		}
		return kimi.New().ManagedMCPSelectionForScope(scope)
	}}
	var err error
	stager.Registry, err = clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeProject, root, "wrong"); err == nil || calls != 0 {
		t.Fatal("policy ran before complete original artifact verification")
	}
	if _, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeProject, root, digest); err == nil || calls != 1 {
		t.Fatal("second complete verification missed mutation during selection")
	}
}

func TestScopedManagedMCPSelectionPolicyRefusalNeverRetriesUserLayout(t *testing.T) {
	stager, root, digest := scopedSelectionArtifact(t, map[string]string{".kimi-plugin/plugin.json": `{"mcpServers":{"user-only":{}}}`})
	sentinel := errors.New("scoped policy refused")
	adapter := selectionPolicyAdapter{Adapter: kimi.New(), choose: func(domain.InstallScope) (clients.SelectionLayout, error) {
		return clients.SelectionLayout{}, sentinel
	}}
	var err error
	stager.Registry, err = clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeProject, root, digest); !errors.Is(err, sentinel) {
		t.Fatal("scoped policy refusal was hidden by a fallback", err)
	}
}

func TestScopedManagedMCPSelectionRejectsLinkedSelectedFiles(t *testing.T) {
	for _, symbolic := range []bool{false, true} {
		if symbolic && runtime.GOOS == "windows" {
			continue // Windows symlink creation may require elevation.
		}
		stager, root, digest := scopedSelectionArtifact(t, map[string]string{"mcp.json": `{"mcpServers":{"held":{}}}`})
		selected, outside := filepath.Join(root, "mcp.json"), filepath.Join(t.TempDir(), "held-original")
		if err := os.Rename(selected, outside); err != nil {
			t.Fatal(err)
		}
		link := os.Link
		if symbolic {
			link = os.Symlink
		}
		if err := link(outside, selected); err != nil {
			t.Fatal(err)
		}
		if _, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeProject, root, digest); err == nil {
			t.Fatal("selected file alias bypassed full artifact verification")
		}
	}
}
