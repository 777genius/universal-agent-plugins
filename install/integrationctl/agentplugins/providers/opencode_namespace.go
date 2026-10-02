package providers

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
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
	codec, err := clients.DesiredOpenCodeCodec(plan.OpenCodeHost)
	if err != nil {
		return err
	}
	previous := []string{}
	var source nativeconfig.Codec
	var receipts []nativeconfig.Receipt
	if managed != nil {
		for _, object := range managed.NativeObjects {
			stored, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
			if err != nil {
				return err
			}
			if !mcp {
				continue
			}
			if source != "" && source != stored {
				return nativeconfig.ErrNativeMigrationRequired
			}
			source = stored
			owned := nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: stored, Name: object.LogicalName, Digest: object.ManagedDigest}
			present, exactlyOwned, err := guard.Kernel.Inspect(paths, stored, object.LogicalName, &owned)
			if err != nil {
				return fmt.Errorf("inspect prior OpenCode MCP server %q: %w", object.LogicalName, err)
			}
			if present && !exactlyOwned {
				return fmt.Errorf("prior OpenCode MCP server %q is not exactly owned: %w", object.LogicalName, nativeconfig.ErrNotOwned)
			}
			previous = append(previous, object.LogicalName)
			receipts = append(receipts, owned)
		}
	}
	if source != "" && source != codec {
		sort.Slice(receipts, func(i, j int) bool { return receipts[i].Name < receipts[j].Name })
		names := domain.SelectedMCPNames(plan)
		sort.Strings(names)
		return guard.Kernel.CheckDialectTransition(paths, source, codec, receipts, names)
	}
	// Disabled foreign entries reserve their names even though they do not
	// participate in the active callable namespace. They cannot be adopted.
	prior := make(map[string]bool, len(previous))
	for _, name := range previous {
		prior[name] = true
	}
	proposed := domain.SelectedMCPNames(plan)
	for _, name := range proposed {
		if prior[name] {
			continue
		}
		present, _, err := guard.Kernel.Inspect(paths, codec, name, nil)
		if err != nil {
			return err
		}
		if present {
			return fmt.Errorf("OpenCode MCP server %q already exists: %w", name, nativeconfig.ErrCollision)
		}
	}
	err = guard.Kernel.CheckOpenCodeNamespaceForCodec(paths, codec, proposed, previous)
	if errors.Is(err, nativeconfig.ErrOpenCodeV2Namespace) {
		return errors.Join(nativeconfig.ErrNativeMigrationRequired, err)
	}
	return err
}
