package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Codex keeps its own plugin data for a root-manifest package, outside the
// installer's PLUGIN_DATA. The user's --purge-data must reach it through
// RemoveGroup, and an ordinary remove must leave it alone.
func TestCodexRemoveGroupPurgeDataDeletesCodexOwnPluginData(t *testing.T) {
	for _, purge := range []bool{true, false} {
		name := "ordinary remove keeps Codex data"
		if purge {
			name = "purge deletes Codex data"
		}
		t.Run(name, func(t *testing.T) {
			service, _, _ := serviceFixture(t)
			profile := canonicalCodexProfile(t, ".codex")
			codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: profile}
			added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{addInput(t, codex, "https://example.com/codex-purge-native")}, OperationGroupID: "codex-purge-add", Confirmed: true})
			if err != nil {
				t.Fatal(err)
			}
			plan := added.Targets[0].Plan
			marketplace := shared.ManagedMarketplaceName(plan.PhysicalArtifactID)
			sum := sha256.Sum256([]byte(marketplace + "\x00" + plan.DeclaredName))
			data := filepath.Join(profile, "plugins", "data", "agent-plugins", hex.EncodeToString(sum[:]))
			if err := os.MkdirAll(data, 0o700); err != nil {
				t.Fatal(err)
			}
			_, err = service.RemoveGroup(context.Background(), RemoveGroupInput{
				Selector: added.InstallationID, PurgeData: purge,
				Targets:          []RemoveInput{{Client: codex, Scope: domain.ScopeUser, ExternalUninstalled: true}},
				OperationGroupID: "codex-purge-remove", Confirmed: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			_, statErr := os.Stat(data)
			if purge == (statErr == nil) {
				t.Fatalf("purge=%v but Codex data present=%v (%v)", purge, statErr == nil, statErr)
			}
		})
	}
}
