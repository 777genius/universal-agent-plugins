package providers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type OpenCodeNamespacePreflight struct {
	Kernel nativeconfig.Kernel
}

func (guard OpenCodeNamespacePreflight) CheckMCPNamespace(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !domain.ClientTraitsFor(client.ClientID).ReportsMCPToolNamespaceCollision {
		return nil
	}
	root := strings.TrimSpace(client.ConfigRoot)
	if !filepath.IsAbs(root) {
		return fmt.Errorf("OpenCode user config root is unavailable")
	}
	paths := nativeconfig.Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
	previous := []string{}
	if managed != nil {
		for _, object := range managed.NativeObjects {
			if object.Kind == nativeconfig.OpenCodeMCPObjectKind {
				owned := nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecOpenCode, Name: object.LogicalName, Digest: object.ManagedDigest}
				present, exactlyOwned, err := guard.Kernel.Inspect(paths, nativeconfig.CodecOpenCode, object.LogicalName, &owned)
				if err != nil {
					return fmt.Errorf("inspect prior OpenCode MCP server %q: %w", object.LogicalName, err)
				}
				if present && !exactlyOwned {
					return fmt.Errorf("prior OpenCode MCP server %q is not exactly owned: %w", object.LogicalName, nativeconfig.ErrNotOwned)
				}
				previous = append(previous, object.LogicalName)
			}
		}
	}
	return guard.Kernel.CheckOpenCodeNamespace(paths, domain.SelectedMCPNames(plan), previous)
}
