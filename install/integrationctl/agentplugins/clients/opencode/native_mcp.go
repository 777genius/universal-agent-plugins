package opencode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func VerifyOpenCodeNativeObjects(configRoot, activePath string, objects []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, allowMissing bool) error {
	if err := kernel.RequireFileIO(); err != nil {
		return err
	}
	projection := OpenCodeProjection{ConfigJSON: filepath.Join(configRoot, "opencode.json"), ConfigJSONC: filepath.Join(configRoot, "opencode.jsonc")}
	if len(OpenCodeObjects(objects)) > 0 && activePath != "" {
		var err error
		projection, err = ReadOpenCodeProjection(activePath)
		if err != nil {
			return err
		}
		if err := validateOpenCodeProjection(configRoot, activePath, projection); err != nil {
			return err
		}
	}
	for _, object := range OpenCodeObjects(objects) {
		if err := verifyOpenCodeObject(configRoot, projection, object, kernel, allowMissing); err != nil {
			return err
		}
	}
	return nil
}

func verifyOpenCodeObject(configRoot string, projection OpenCodeProjection, object domain.NativeObjectOwnership, kernel nativeconfig.Kernel, allowMissing bool) error {
	if err := validateOpenCodeObject(configRoot, projection, object); err != nil {
		return err
	}
	if object.Kind == openCodeSkillKind {
		return verifyOpenCodeSkill(object, allowMissing)
	}
	return verifyOpenCodeMCP(projection, object, kernel, allowMissing)
}

func verifyOpenCodeSkill(object domain.NativeObjectOwnership, allowMissing bool) error {
	digest, err := shared.DigestSkillDirectory(object.Path)
	if os.IsNotExist(err) && allowMissing {
		return nil
	}
	if err != nil || digest != object.ManagedDigest {
		return fmt.Errorf("managed OpenCode skill %q is missing or changed", object.LogicalName)
	}
	return nil
}

func verifyOpenCodeMCP(projection OpenCodeProjection, object domain.NativeObjectOwnership, kernel nativeconfig.Kernel, allowMissing bool) error {
	receipt, err := receiptFromOpenCodeObject(object)
	if err != nil {
		return err
	}
	present, exactlyOwned, err := kernel.Inspect(nativeconfig.Paths{JSON: projection.ConfigJSON, JSONC: projection.ConfigJSONC}, receipt.Codec, object.LogicalName, &receipt)
	if err != nil {
		return fmt.Errorf("verify managed OpenCode MCP server %q: %w", object.LogicalName, err)
	}
	if !present && allowMissing {
		return nil
	}
	if !present || !exactlyOwned {
		return fmt.Errorf("verify managed OpenCode MCP server %q: %w", object.LogicalName, nativeconfig.ErrNotOwned)
	}
	return nil
}

func validateOpenCodeProjection(configRoot, activePath string, projection OpenCodeProjection) error {
	if !shared.SameCleanPath(projection.ConfigJSON, filepath.Join(configRoot, "opencode.json")) ||
		!shared.SameCleanPath(projection.ConfigJSONC, filepath.Join(configRoot, "opencode.jsonc")) ||
		!shared.SameCleanPath(projection.PackageRoot, activePath) {
		return fmt.Errorf("OpenCode native projection is not bound to the detected client and active package")
	}
	if projection.DataRoot != "" && !filepath.IsAbs(projection.DataRoot) {
		return fmt.Errorf("OpenCode native projection data root must be absolute")
	}
	return nil
}

func openCodeMCPRequests(kernel nativeconfig.Kernel, projection OpenCodeProjection, previous, desired []domain.NativeObjectOwnership) ([]nativeconfig.Request, error) {
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	paths := openCodeMCPPaths(projection, previous)
	placeholders := nativeconfig.Placeholders{PackageRoot: projection.PackageRoot, DataRoot: projection.DataRoot}
	previousRequests, err := openCodeMCPPreviousRequests(kernel, paths, placeholders, projection, previousByID, desiredByID)
	if err != nil {
		return nil, err
	}
	desiredRequests, err := openCodeMCPDesiredRequests(kernel, paths, placeholders, projection, previousByID, desiredByID)
	if err != nil {
		return nil, err
	}
	result := previousRequests
	result = append(result, desiredRequests...)
	if len(projection.MCPServers) > 0 {
		codec, err := projectionCodec(projection)
		if err != nil {
			return nil, err
		}
		proposed := make([]string, 0, len(projection.MCPServers))
		for name := range projection.MCPServers {
			proposed = append(proposed, name)
		}
		var prior []string
		for _, object := range previous {
			_, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
			if err != nil {
				return nil, err
			}
			if mcp {
				prior = append(prior, object.LogicalName)
			}
		}
		if err := kernel.CheckOpenCodeNamespaceForCodec(paths, codec, proposed, prior); err != nil {
			if errors.Is(err, nativeconfig.ErrOpenCodeV2Namespace) {
				err = errors.Join(nativeconfig.ErrNativeMigrationRequired, err)
			}
			return nil, err
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func openCodeMCPPaths(projection OpenCodeProjection, previous []domain.NativeObjectOwnership) nativeconfig.Paths {
	paths := nativeconfig.Paths{JSON: projection.ConfigJSON, JSONC: projection.ConfigJSONC}
	if paths.JSON != "" {
		return paths
	}
	for _, object := range previous {
		if _, mcp, _ := nativeconfig.OpenCodeCodecForKind(object.Kind); mcp {
			root := filepath.Dir(object.Path)
			return nativeconfig.Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
		}
	}
	return paths
}

func openCodeMCPPreviousRequests(kernel nativeconfig.Kernel, paths nativeconfig.Paths, placeholders nativeconfig.Placeholders, projection OpenCodeProjection, previousByID, desiredByID map[string]domain.NativeObjectOwnership) ([]nativeconfig.Request, error) {
	var result []nativeconfig.Request
	for id, object := range previousByID {
		request, include, err := openCodeMCPPreviousRequest(kernel, paths, placeholders, projection, id, object, desiredByID)
		if err != nil {
			return nil, err
		}
		if include {
			result = append(result, request)
		}
	}
	return result, nil
}

func openCodeMCPPreviousRequest(kernel nativeconfig.Kernel, paths nativeconfig.Paths, placeholders nativeconfig.Placeholders, projection OpenCodeProjection, id string, object domain.NativeObjectOwnership, desiredByID map[string]domain.NativeObjectOwnership) (nativeconfig.Request, bool, error) {
	codec, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
	if err != nil {
		return nativeconfig.Request{}, false, err
	}
	if !mcp {
		return nativeconfig.Request{}, false, nil
	}
	owned, err := receiptFromOpenCodeObject(object)
	if err != nil {
		return nativeconfig.Request{}, false, err
	}
	present, exactlyOwned, err := kernel.Inspect(paths, codec, object.LogicalName, &owned)
	if err != nil {
		return nativeconfig.Request{}, false, fmt.Errorf("inspect managed OpenCode MCP server %q: %w", object.LogicalName, err)
	}
	if present && !exactlyOwned {
		return nativeconfig.Request{}, false, fmt.Errorf("managed OpenCode MCP server %q changed outside agentplugins: %w", object.LogicalName, nativeconfig.ErrNotOwned)
	}
	next, kept := desiredByID[id]
	if !kept {
		// An already absent exact-owned entry is a safe idempotent removal.
		if !present {
			return nativeconfig.Request{}, false, nil
		}
		return nativeconfig.Request{Paths: paths, Codec: codec, Action: nativeconfig.ActionRemove, Name: object.LogicalName, Owned: &owned}, true, nil
	}
	return openCodeMCPUpdateRequest(paths, placeholders, projection, object, next, &owned, present)
}

func openCodeMCPUpdateRequest(paths nativeconfig.Paths, placeholders nativeconfig.Placeholders, projection OpenCodeProjection, object, next domain.NativeObjectOwnership, owned *nativeconfig.Receipt, present bool) (nativeconfig.Request, bool, error) {
	action := nativeconfig.ActionUpdate
	receipt := owned
	if !present {
		// Repair may recreate a missing native entry only when the staged
		// desired receipt is exactly the receipt already owned by state. A
		// real update with different bytes remains fail closed.
		if !sameOpenCodeMCPObject(object, next) {
			return nativeconfig.Request{}, false, fmt.Errorf("managed OpenCode MCP server %q is absent during update: %w", object.LogicalName, nativeconfig.ErrNotOwned)
		}
		action, receipt = nativeconfig.ActionAdd, nil
	}
	desiredReceipt, err := receiptFromOpenCodeObject(next)
	if err != nil {
		return nativeconfig.Request{}, false, err
	}
	if owned.Codec != desiredReceipt.Codec {
		return nativeconfig.Request{}, false, nativeconfig.ErrNativeMigrationRequired
	}
	return nativeconfig.Request{Paths: paths, Codec: desiredReceipt.Codec, Action: action, Name: next.LogicalName,
		Server: projection.MCPServers[next.LogicalName], Placeholders: placeholders, Owned: receipt, Desired: &desiredReceipt}, true, nil
}

func openCodeMCPDesiredRequests(kernel nativeconfig.Kernel, paths nativeconfig.Paths, placeholders nativeconfig.Placeholders, projection OpenCodeProjection, previousByID, desiredByID map[string]domain.NativeObjectOwnership) ([]nativeconfig.Request, error) {
	var result []nativeconfig.Request
	for id, object := range desiredByID {
		codec, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
		if err != nil {
			return nil, err
		}
		if !mcp {
			continue
		}
		if _, replacing := previousByID[id]; replacing {
			continue
		}
		present, _, err := kernel.Inspect(paths, codec, object.LogicalName, nil)
		if err != nil {
			return nil, fmt.Errorf("inspect OpenCode MCP server %q before add: %w", object.LogicalName, err)
		}
		if present {
			return nil, fmt.Errorf("OpenCode MCP server %q already exists: %w", object.LogicalName, nativeconfig.ErrCollision)
		}
		desiredReceipt, err := receiptFromOpenCodeObject(object)
		if err != nil {
			return nil, err
		}
		result = append(result, nativeconfig.Request{Paths: paths, Codec: codec, Action: nativeconfig.ActionAdd, Name: object.LogicalName,
			Server: projection.MCPServers[object.LogicalName], Placeholders: placeholders, Desired: &desiredReceipt})
	}
	return result, nil
}

func validateOpenCodeObject(configRoot string, projection OpenCodeProjection, object domain.NativeObjectOwnership) error {
	if object.LogicalName == "" || object.ManagedDigest == "" {
		return fmt.Errorf("OpenCode native object is incomplete")
	}
	if _, _, err := nativeconfig.OpenCodeCodecForKind(object.Kind); err != nil {
		return err
	}
	expected := expectedOpenCodeObjectPath(configRoot, projection, object)
	if expected == "" {
		return fmt.Errorf("unsupported OpenCode native object kind %q", object.Kind)
	}
	if !shared.SameCleanPath(expected, object.Path) {
		return fmt.Errorf("OpenCode native object %q has an untrusted path", object.LogicalName)
	}
	return pathpolicy.RequireContainedChild(configRoot, object.Path)
}

func expectedOpenCodeObjectPath(configRoot string, projection OpenCodeProjection, object domain.NativeObjectOwnership) string {
	switch object.Kind {
	case OpenCodeMCPObjectKind, OpenCodeV2MCPObjectKind:
		if projection.ConfigPath != "" {
			return projection.ConfigPath
		}
		if shared.SameCleanPath(object.Path, filepath.Join(configRoot, "opencode.json")) || shared.SameCleanPath(object.Path, filepath.Join(configRoot, "opencode.jsonc")) {
			return object.Path
		}
		return ""
	case openCodeSkillKind:
		return filepath.Join(configRoot, "skills", object.LogicalName)
	default:
		return ""
	}
}

func preflightOpenCodeObjects(configRoot, activePath string, projection OpenCodeProjection, previous, desired []domain.NativeObjectOwnership) error {
	if err := preflightOpenCodeCodecs(projection, previous, desired); err != nil {
		return err
	}
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	for _, object := range previous {
		if err := validateOpenCodeObject(configRoot, projection, object); err != nil {
			return err
		}
	}
	if err := preflightDesiredOpenCodeObjects(configRoot, projection, previousByID, desiredByID); err != nil {
		return err
	}
	_ = activePath
	return nil
}

func preflightDesiredOpenCodeObjects(configRoot string, projection OpenCodeProjection, previousByID, desiredByID map[string]domain.NativeObjectOwnership) error {
	for id, object := range desiredByID {
		if err := validateOpenCodeObject(configRoot, projection, object); err != nil {
			return err
		}
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind {
				_, priorMCP, _ := nativeconfig.OpenCodeCodecForKind(prior.Kind)
				_, nextMCP, _ := nativeconfig.OpenCodeCodecForKind(object.Kind)
				if priorMCP && nextMCP {
					return nativeconfig.ErrNativeMigrationRequired
				}
			}
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !shared.SameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("OpenCode native object identity changed for %s", id)
			}
			continue
		}
		if err := requireOpenCodeSkillAbsent(object); err != nil {
			return err
		}
	}
	return nil
}

func requireOpenCodeSkillAbsent(object domain.NativeObjectOwnership) error {
	if object.Kind != openCodeSkillKind {
		return nil
	}
	if _, err := os.Lstat(object.Path); err == nil {
		return fmt.Errorf("OpenCode skill %q already exists and is not owned", object.LogicalName)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func preflightOpenCodeCodecs(projection OpenCodeProjection, previous, desired []domain.NativeObjectOwnership) error {
	var selected nativeconfig.Codec
	for _, objects := range [][]domain.NativeObjectOwnership{previous, desired} {
		for _, object := range objects {
			codec, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
			if err != nil {
				return err
			}
			if !mcp {
				continue
			}
			if selected != "" && selected != codec {
				return nativeconfig.ErrNativeMigrationRequired
			}
			selected = codec
		}
	}
	if projection.Version != 0 && selected != "" {
		codec, err := projectionCodec(projection)
		if err != nil {
			return err
		}
		if codec != selected {
			return nativeconfig.ErrNativeMigrationRequired
		}
	}
	return nil
}
