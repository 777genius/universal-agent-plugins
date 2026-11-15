package installer_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type legacyClaudeSDKRenderer struct{ *claude.Adapter }

func (legacyClaudeSDKRenderer) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := shared.DeliverManagedStdio(in.DeliverLauncher(), in.StagingPath, in.Envelope, in.Plan); err != nil {
		return nil, err
	}
	return nil, claude.ProjectClaude(in.StagingPath, in.Envelope, in.Plan, in.PluginDataPath)
}

func (legacyClaudeSDKRenderer) InstalledSkillsDirectory(domain.DeliveryPlan) string {
	return claude.ClaudeRuntimeDirectory + "/skills"
}

func TestProjectClaudeLegacyPreparedRefreshPreservesPinAndSelection(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project, user, source := filepath.Join(base, "project"), filepath.Join(base, "user", ".claude"), filepath.Join(base, "source")
	claudeSDKWrite(t, filepath.Join(project, ".claude", "settings.json"), `{"foreign":true}`)
	claudeSDKWrite(t, filepath.Join(user, "settings.json"), `{"foreign":true}`)
	claudeSDKWrite(t, filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"legacy-project","version":"1.0.0"}`)
	const skill = "---\nname: docs\ndescription: Inert migration fixture\n---\nPinned skill.\n"
	claudeSDKWrite(t, filepath.Join(source, "skills", "docs", "SKILL.md"), skill)
	claudeSDKWrite(t, filepath.Join(source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://fixture.invalid/mcp"}}}`)
	legacy := newClaudeSDKFixture(t, base, project, user, source, legacyClaudeSDKRenderer{claude.New()})
	legacy.apply(t, installer.OpInstall, "legacy-prepared-install")
	before := legacy.binding(t)
	if before.Activation != domain.ActivationPrepared || len(before.NativeObjects) != 1 {
		t.Fatalf("legacy fixture did not record its original prepared format: %+v", before)
	}
	current := newClaudeSDKFixture(t, base, project, user, source)
	current.apply(t, installer.OpRefreshProjection, "reviewed-native-refresh")
	after := current.binding(t)
	if after.Activation != domain.ActivationActive || after.Verification != domain.VerificationInstalled || len(after.NativeObjects) != 2 {
		t.Fatalf("native refresh remained incomplete: %+v", after)
	}
	if !reflect.DeepEqual(before.PackageRevision, after.PackageRevision) || before.ClientBindingID != after.ClientBindingID || before.TargetLocator != after.TargetLocator || before.PhysicalArtifact != after.PhysicalArtifact || before.DataReceiptID != after.DataReceiptID {
		t.Fatal("projection refresh changed source pin, binding, path, or data authority")
	}
	if claudeSDKBytes(t, claudeSDKSkillPath(after.TargetLocator, domain.ScopeProject)) != skill {
		t.Fatal("projection refresh changed pinned skill bytes")
	}
	if _, err := os.Stat(filepath.Join(after.TargetLocator, ".claude-plugin", "plugin.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy implicit projection survived refresh: %v", err)
	}
	if len(current.runner.readObject(filepath.Join(after.TargetLocator, ".agentplugins-plugin", ".mcp.json"))) != 1 {
		t.Fatal("projection refresh dropped the selected MCP server")
	}
	current.apply(t, installer.OpRemove, "remove-refreshed-install")
	if claudeSDKBytes(t, filepath.Join(user, "settings.json")) != `{"foreign":true}` {
		t.Fatal("project refresh changed unrelated user settings")
	}
}
