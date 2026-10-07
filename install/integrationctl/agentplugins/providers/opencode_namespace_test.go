package providers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestOpenCodeNamespacePreflightOnlyExcludesExactlyOwnedPriorServer(t *testing.T) {
	root := t.TempDir()
	paths := nativeconfig.Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
	kernel := nativeconfig.New()
	receipt, err := kernel.Apply(nativeconfig.Request{
		Paths: paths, Codec: nativeconfig.CodecOpenCode, Action: nativeconfig.ActionAdd,
		Name: "docs_extra", Server: nativeconfig.Server{Type: "remote", URL: "https://owned.test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: root}
	plan := domain.DeliveryPlan{Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportPrepared}}}
	managed := &domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{{
		Kind: opencode.OpenCodeMCPObjectKind, LogicalName: "docs_extra", Path: receipt.Path, ManagedDigest: receipt.Digest,
	}}}
	guard := OpenCodeNamespacePreflight{Kernel: kernel}
	if err := guard.CheckMCPNamespace(context.Background(), client, plan, managed); err != nil {
		t.Fatalf("owned prior server should be removable: %v", err)
	}
	body, err := os.ReadFile(paths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(body), "https://owned.test", "https://foreign.test", 1)
	if changed == string(body) {
		t.Fatalf("unexpected native config fixture: %s", body)
	}
	if err := os.WriteFile(paths.JSON, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := guard.CheckMCPNamespace(context.Background(), client, plan, managed); !errors.Is(err, nativeconfig.ErrNotOwned) {
		t.Fatalf("modified prior server excluded from collision check: %v", err)
	}
}
