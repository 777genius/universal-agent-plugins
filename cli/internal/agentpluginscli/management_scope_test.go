package agentpluginscli

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestManagementNativeProjectionProfileUsesOwnedObjectsOutsidePrivateArtifact(t *testing.T) {
	profile, err := managementAbsolute(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(t.TempDir(), "private-package")
	binding := domain.ClientBinding{ClientID: "opencode", Scope: "user", TargetLocator: artifact,
		NativeObjects: []domain.NativeObjectOwnership{
			{Kind: "managed_package_directory", Path: artifact, ProtectionClass: "managed"},
			{Kind: "opencode_global_skill_directory", Path: filepath.Join(profile, "skills", "fixture-guide"), ProtectionClass: "managed"},
		}}
	if !managementBindingInProfile(binding, profile) {
		t.Fatal("owned native projections were confused with the private package artifact")
	}
	if managementBindingInProfile(binding, t.TempDir()) {
		t.Fatal("a different native profile inherited the old binding")
	}
	binding.NativeObjects = append(binding.NativeObjects, domain.NativeObjectOwnership{Kind: "opencode_global_skill_directory", Path: filepath.Join(t.TempDir(), "foreign"), ProtectionClass: "managed"})
	if managementBindingInProfile(binding, profile) {
		t.Fatal("mixed native roots were treated as a single captured profile")
	}
	binding.NativeObjects = binding.NativeObjects[:1]
	if managementBindingInProfile(binding, profile) {
		t.Fatal("a private artifact alone claimed native profile ownership")
	}
}

func TestManagementRequiresDeclaredRootSelection(t *testing.T) {
	requirements := environment.Requirements{Dependencies: map[string]environment.Requirement{}}
	locked := environment.Lock{Packages: []environment.LockedPackage{{Alias: "transitive", SourceKey: "github:example/transitive:"}}}
	for _, alias := range []string{"unknown", "transitive"} {
		_, err := managementAliases(requirements, locked, []string{alias}, false)
		var refusal *management.Refusal
		if !errors.As(err, &refusal) || refusal.Code != "DEPENDENCY_NOT_DECLARED" {
			t.Fatalf("unowned selection %q was not refused: %v", alias, err)
		}
	}
}

func TestManagementNativeProjectionAcceptsOwnedWindsurfMCPEntry(t *testing.T) {
	profile, err := managementAbsolute(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(t.TempDir(), "private-package")
	binding := domain.ClientBinding{ClientID: "windsurf", Scope: "user", TargetLocator: artifact, NativeObjects: []domain.NativeObjectOwnership{
		{Kind: "managed_package_directory", Path: artifact, ProtectionClass: "managed"},
		{Kind: "windsurf_global_mcp_server", Path: filepath.Join(profile, "mcp_config.json"), ProtectionClass: "managed_entry"},
	}}
	if !managementBindingInProfile(binding, profile) {
		t.Fatal("owned MCP entry did not prove profile")
	}
	if managementBindingInProfile(binding, t.TempDir()) {
		t.Fatal("alternate profile inherited binding")
	}
	binding.NativeObjects = append(binding.NativeObjects, domain.NativeObjectOwnership{Kind: "windsurf_global_mcp_server", Path: filepath.Join(t.TempDir(), "mcp_config.json"), ProtectionClass: "managed_entry"})
	if managementBindingInProfile(binding, profile) {
		t.Fatal("mixed profile accepted")
	}
	binding.NativeObjects = binding.NativeObjects[2:]
	if managementBindingInProfile(binding, profile) {
		t.Fatal("foreign profile accepted")
	}
	binding.NativeObjects = []domain.NativeObjectOwnership{{Kind: "managed_package_directory", Path: artifact, ProtectionClass: "managed"}}
	if managementBindingInProfile(binding, profile) {
		t.Fatal("private artifact alone proved profile")
	}
}

func TestManagementCopilotUserBindingRequiresRecordedProfile(t *testing.T) {
	profile, err := managementAbsolute(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	binding := domain.ClientBinding{ClientID: "copilot", ClientBindingID: "copilot-owned", Scope: "user", NativeProfileRoot: profile, TargetLocator: filepath.Join(t.TempDir(), "private-marketplace")}
	if !managementBindingInProfile(binding, profile) {
		t.Fatal("new Copilot binding did not retain captured profile authority")
	}
	if managementBindingInProfile(binding, t.TempDir()) {
		t.Fatal("foreign profile inherited Copilot registration")
	}
	binding.NativeProfileRoot = ""
	binding.NativeObjects = []domain.NativeObjectOwnership{{Kind: "managed_package_directory", Path: binding.TargetLocator, ProtectionClass: "managed"}}
	if managementBindingInProfile(binding, profile) {
		t.Fatal("legacy private artifact was adopted as Copilot profile authority")
	}
}
