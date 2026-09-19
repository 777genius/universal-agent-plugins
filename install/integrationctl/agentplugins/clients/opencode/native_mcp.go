package opencode

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func VerifyOpenCodeNativeObjects(configRoot, activePath string, objects []domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	projection := OpenCodeProjection{}
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
		if err := verifyOpenCodeObject(configRoot, projection, object, kernel); err != nil {
			return err
		}
	}
	return nil
}

func verifyOpenCodeObject(configRoot string, projection OpenCodeProjection, object domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	if err := validateOpenCodeObject(configRoot, projection, object); err != nil {
		return err
	}
	if object.Kind == openCodeSkillKind {
		return verifyOpenCodeSkill(object)
	}
	return verifyOpenCodeMCP(projection, object, kernel)
}

func verifyOpenCodeSkill(object domain.NativeObjectOwnership) error {
	digest, err := shared.DigestSkillDirectory(object.Path)
	if err != nil || digest != object.ManagedDigest {
		return fmt.Errorf("managed OpenCode skill %q is missing or changed", object.LogicalName)
	}
	return nil
}

func verifyOpenCodeMCP(projection OpenCodeProjection, object domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	receipt := receiptFromOpenCodeObject(object)
	present, exactlyOwned, err := kernel.Inspect(nativeconfig.Paths{JSON: projection.ConfigJSON, JSONC: projection.ConfigJSONC}, nativeconfig.CodecOpenCode, object.LogicalName, &receipt)
	if err != nil {
		return fmt.Errorf("verify managed OpenCode MCP server %q: %w", object.LogicalName, err)
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
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func openCodeMCPPaths(projection OpenCodeProjection, previous []domain.NativeObjectOwnership) nativeconfig.Paths {
	paths := nativeconfig.Paths{JSON: projection.ConfigJSON, JSONC: projection.ConfigJSONC}
	if paths.JSON != "" {
		return paths
	}
	for _, object := range previous {
		if object.Kind == OpenCodeMCPObjectKind {
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
	if object.Kind != OpenCodeMCPObjectKind {
		return nativeconfig.Request{}, false, nil
	}
	owned := receiptFromOpenCodeObject(object)
	present, exactlyOwned, err := kernel.Inspect(paths, nativeconfig.CodecOpenCode, object.LogicalName, &owned)
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
		return nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Action: nativeconfig.ActionRemove, Name: object.LogicalName, Owned: &owned}, true, nil
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
	desiredReceipt := receiptFromOpenCodeObject(next)
	return nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Action: action, Name: next.LogicalName,
		Server: projection.MCPServers[next.LogicalName], Placeholders: placeholders, Owned: receipt, Desired: &desiredReceipt}, true, nil
}

func openCodeMCPDesiredRequests(kernel nativeconfig.Kernel, paths nativeconfig.Paths, placeholders nativeconfig.Placeholders, projection OpenCodeProjection, previousByID, desiredByID map[string]domain.NativeObjectOwnership) ([]nativeconfig.Request, error) {
	var result []nativeconfig.Request
	for id, object := range desiredByID {
		if object.Kind != OpenCodeMCPObjectKind {
			continue
		}
		if _, replacing := previousByID[id]; replacing {
			continue
		}
		present, _, err := kernel.Inspect(paths, nativeconfig.CodecOpenCode, object.LogicalName, nil)
		if err != nil {
			return nil, fmt.Errorf("inspect OpenCode MCP server %q before add: %w", object.LogicalName, err)
		}
		if present {
			return nil, fmt.Errorf("OpenCode MCP server %q already exists: %w", object.LogicalName, nativeconfig.ErrCollision)
		}
		desiredReceipt := receiptFromOpenCodeObject(object)
		result = append(result, nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Action: nativeconfig.ActionAdd, Name: object.LogicalName,
			Server: projection.MCPServers[object.LogicalName], Placeholders: placeholders, Desired: &desiredReceipt})
	}
	return result, nil
}

func validateOpenCodeObject(configRoot string, projection OpenCodeProjection, object domain.NativeObjectOwnership) error {
	if object.LogicalName == "" || object.ManagedDigest == "" {
		return fmt.Errorf("OpenCode native object is incomplete")
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
	case OpenCodeMCPObjectKind:
		if projection.ConfigPath != "" {
			return projection.ConfigPath
		}
		return object.Path
	case openCodeSkillKind:
		return filepath.Join(configRoot, "skills", object.LogicalName)
	default:
		return ""
	}
}

func preflightOpenCodeObjects(configRoot, activePath string, projection OpenCodeProjection, previous, desired []domain.NativeObjectOwnership) error {
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
