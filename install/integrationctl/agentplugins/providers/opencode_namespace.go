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
	if !domain.ClientTraitsFor(client.ClientID).ReportsMCPToolNamespaceCollision || len(domain.SelectedMCPNames(plan)) == 0 {
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
	previous, source, receipts, err := guard.inspectOwnedOpenCodeMCP(paths, managed)
	if err != nil {
		return err
	}
	proposed := domain.SelectedMCPNames(plan)
	if source != "" && source != codec {
		sort.Slice(receipts, func(i, j int) bool { return receipts[i].Name < receipts[j].Name })
		sort.Strings(proposed)
		return guard.Kernel.CheckDialectTransition(paths, source, codec, receipts, proposed)
	}
	if err := guard.checkProposedOpenCodeMCP(paths, codec, proposed, previous); err != nil {
		return err
	}
	err = guard.Kernel.CheckOpenCodeNamespaceForCodec(paths, codec, proposed, previous)
	if errors.Is(err, nativeconfig.ErrOpenCodeV2Namespace) {
		return errors.Join(nativeconfig.ErrNativeMigrationRequired, err)
	}
	return err
}

func (guard OpenCodeNamespacePreflight) inspectOwnedOpenCodeMCP(paths nativeconfig.Paths, managed *domain.ClientBinding) ([]string, nativeconfig.Codec, []nativeconfig.Receipt, error) {
	previous := []string{}
	var source nativeconfig.Codec
	var receipts []nativeconfig.Receipt
	if managed == nil {
		return previous, source, receipts, nil
	}
	for _, object := range managed.NativeObjects {
		stored, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
		if err != nil {
			return nil, "", nil, err
		}
		if !mcp {
			continue
		}
		if source != "" && source != stored {
			return nil, "", nil, nativeconfig.ErrNativeMigrationRequired
		}
		source = stored
		owned := nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: stored, Name: object.LogicalName, Digest: object.ManagedDigest}
		present, exactlyOwned, err := guard.Kernel.Inspect(paths, stored, object.LogicalName, &owned)
		if err != nil {
			return nil, "", nil, fmt.Errorf("inspect prior OpenCode MCP server %q: %w", object.LogicalName, err)
		}
		if present && !exactlyOwned {
			return nil, "", nil, fmt.Errorf("prior OpenCode MCP server %q is not exactly owned: %w", object.LogicalName, nativeconfig.ErrNotOwned)
		}
		previous = append(previous, object.LogicalName)
		receipts = append(receipts, owned)
	}
	return previous, source, receipts, nil
}

func (guard OpenCodeNamespacePreflight) checkProposedOpenCodeMCP(paths nativeconfig.Paths, codec nativeconfig.Codec, proposed, previous []string) error {
	// Disabled foreign entries reserve their names even though they do not
	// participate in the active callable namespace. They cannot be adopted.
	prior := make(map[string]bool, len(previous))
	for _, name := range previous {
		prior[name] = true
	}
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
	return nil
}
