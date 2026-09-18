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

func VerifyNativeObjects(configRoot, activePath string, objects []domain.NativeObjectOwnership) error {
	projection := openCodeProjection{}
	if len(NativeObjects(objects)) > 0 && activePath != "" {
		var err error
		projection, err = readOpenCodeProjection(activePath)
		if err != nil {
			return err
		}
		if err := validateOpenCodeProjection(configRoot, activePath, projection); err != nil {
			return err
		}
	}
	for _, object := range NativeObjects(objects) {
		if err := validateOpenCodeObject(configRoot, projection, object); err != nil {
			return err
		}
		if object.Kind == openCodeSkillKind {
			digest, err := shared.DigestSkillDirectory(object.Path)
			if err != nil || digest != object.ManagedDigest {
				return fmt.Errorf("managed OpenCode skill %q is missing or changed", object.LogicalName)
			}
			continue
		}
		receipt := receiptFromOpenCodeObject(object)
		present, exactlyOwned, err := nativeconfig.New().Inspect(nativeconfig.Paths{JSON: projection.ConfigJSON, JSONC: projection.ConfigJSONC}, nativeconfig.CodecOpenCode, object.LogicalName, &receipt)
		if err != nil {
			return fmt.Errorf("verify managed OpenCode MCP server %q: %w", object.LogicalName, err)
		}
		if !present || !exactlyOwned {
			return fmt.Errorf("verify managed OpenCode MCP server %q: %w", object.LogicalName, nativeconfig.ErrNotOwned)
		}
	}
	return nil
}

func validateOpenCodeProjection(configRoot, activePath string, projection openCodeProjection) error {
	if !shared.SameCleanPath(projection.ConfigJSON, filepath.Join(configRoot, "opencode.json")) ||
		!shared.SameCleanPath(projection.ConfigJSONC, filepath.Join(configRoot, "opencode.jsonc")) ||
		!shared.SameCleanPath(projection.PackageRoot, activePath) {
		return fmt.Errorf("the OpenCode native projection is not bound to the detected client and active package")
	}
	if projection.DataRoot != "" && !filepath.IsAbs(projection.DataRoot) {
		return fmt.Errorf("the OpenCode native projection data root must be absolute")
	}
	return nil
}

func openCodeMCPRequests(projection openCodeProjection, previous, desired []domain.NativeObjectOwnership) ([]nativeconfig.Request, error) {
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	paths := nativeconfig.Paths{JSON: projection.ConfigJSON, JSONC: projection.ConfigJSONC}
	if paths.JSON == "" {
		for _, object := range previous {
			if object.Kind == openCodeMCPObjectKind {
				root := filepath.Dir(object.Path)
				paths = nativeconfig.Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
				break
			}
		}
	}
	placeholders := nativeconfig.Placeholders{PackageRoot: projection.PackageRoot, DataRoot: projection.DataRoot}
	var result []nativeconfig.Request
	kernel := nativeconfig.New()
	previousRequests, err := openCodeMCPPreviousRequests(kernel, paths, placeholders, projection, previousByID, desiredByID)
	if err != nil {
		return nil, err
	}
	result = append(result, previousRequests...)
	desiredRequests, err := openCodeMCPDesiredRequests(kernel, paths, placeholders, projection, previousByID, desiredByID)
	if err != nil {
		return nil, err
	}
	result = append(result, desiredRequests...)
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func openCodeMCPPreviousRequests(kernel nativeconfig.Kernel, paths nativeconfig.Paths, placeholders nativeconfig.Placeholders, projection openCodeProjection, previousByID, desiredByID map[string]domain.NativeObjectOwnership) ([]nativeconfig.Request, error) {
	var result []nativeconfig.Request
	for id, object := range previousByID {
		if object.Kind != openCodeMCPObjectKind {
			continue
		}
		owned := receiptFromOpenCodeObject(object)
		present, exactlyOwned, err := kernel.Inspect(paths, nativeconfig.CodecOpenCode, object.LogicalName, &owned)
		if err != nil {
			return nil, fmt.Errorf("inspect managed OpenCode MCP server %q: %w", object.LogicalName, err)
		}
		if present && !exactlyOwned {
			return nil, fmt.Errorf("managed OpenCode MCP server %q changed outside agentplugins: %w", object.LogicalName, nativeconfig.ErrNotOwned)
		}
		next, kept := desiredByID[id]
		if !kept {
			if present {
				result = append(result, nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Action: nativeconfig.ActionRemove, Name: object.LogicalName, Owned: &owned})
			}
			continue
		}
		action := nativeconfig.ActionUpdate
		receipt := &owned
		if !present {
			if !sameOpenCodeMCPObject(object, next) {
				return nil, fmt.Errorf("managed OpenCode MCP server %q is absent during update: %w", object.LogicalName, nativeconfig.ErrNotOwned)
			}
			action, receipt = nativeconfig.ActionAdd, nil
		}
		desiredReceipt := receiptFromOpenCodeObject(next)
		result = append(result, nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Action: action, Name: next.LogicalName,
			Server: projection.MCPServers[next.LogicalName], Placeholders: placeholders, Owned: receipt, Desired: &desiredReceipt})
	}
	return result, nil
}

func openCodeMCPDesiredRequests(kernel nativeconfig.Kernel, paths nativeconfig.Paths, placeholders nativeconfig.Placeholders, projection openCodeProjection, previousByID, desiredByID map[string]domain.NativeObjectOwnership) ([]nativeconfig.Request, error) {
	var result []nativeconfig.Request
	for id, object := range desiredByID {
		if object.Kind != openCodeMCPObjectKind {
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
			return nil, fmt.Errorf("the OpenCode MCP server %q already exists: %w", object.LogicalName, nativeconfig.ErrCollision)
		}
		desiredReceipt := receiptFromOpenCodeObject(object)
		result = append(result, nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Action: nativeconfig.ActionAdd, Name: object.LogicalName,
			Server: projection.MCPServers[object.LogicalName], Placeholders: placeholders, Desired: &desiredReceipt})
	}
	return result, nil
}

func openCodeConfigPresence(configRoot string) (jsonExists, jsoncExists bool, err error) {
	jsonExists, err = regularNativeFileExists(filepath.Join(configRoot, "opencode.json"))
	if err != nil {
		return false, false, err
	}
	jsoncExists, err = regularNativeFileExists(filepath.Join(configRoot, "opencode.jsonc"))
	return jsonExists, jsoncExists, err
}

func sameOpenCodeMCPObject(left, right domain.NativeObjectOwnership) bool {
	return left.ObjectID == right.ObjectID && left.Kind == openCodeMCPObjectKind && right.Kind == openCodeMCPObjectKind &&
		left.LogicalName == right.LogicalName && shared.SameCleanPath(left.Path, right.Path) && left.ManagedDigest == right.ManagedDigest
}

func receiptFromOpenCodeObject(object domain.NativeObjectOwnership) nativeconfig.Receipt {
	return nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecOpenCode, Name: object.LogicalName, Digest: object.ManagedDigest}
}

func NativeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	var result []domain.NativeObjectOwnership
	for _, object := range objects {
		if object.Kind == openCodeMCPObjectKind || object.Kind == openCodeSkillKind {
			result = append(result, object)
		}
	}
	return result
}

func validateOpenCodeObject(configRoot string, projection openCodeProjection, object domain.NativeObjectOwnership) error {
	if object.LogicalName == "" || object.ManagedDigest == "" {
		return fmt.Errorf("the OpenCode native object is incomplete")
	}
	expected := object.Path
	switch object.Kind {
	case openCodeMCPObjectKind:
		if projection.ConfigPath != "" {
			expected = projection.ConfigPath
		}
	case openCodeSkillKind:
		expected = filepath.Join(configRoot, "skills", object.LogicalName)
	default:
		return fmt.Errorf("unsupported OpenCode native object kind %q", object.Kind)
	}
	if !shared.SameCleanPath(expected, object.Path) {
		return fmt.Errorf("the OpenCode native object %q has an untrusted path", object.LogicalName)
	}
	return pathpolicy.RequireContainedChild(configRoot, object.Path)
}

func preflightOpenCodeObjects(configRoot, activePath string, projection openCodeProjection, previous, desired []domain.NativeObjectOwnership) error {
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	for _, object := range previous {
		if err := validateOpenCodeObject(configRoot, projection, object); err != nil {
			return err
		}
	}
	for id, object := range desiredByID {
		if err := validateOpenCodeObject(configRoot, projection, object); err != nil {
			return err
		}
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !shared.SameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("the OpenCode native object identity changed for %s", id)
			}
			continue
		}
		if object.Kind == openCodeSkillKind {
			if _, err := os.Lstat(object.Path); err == nil {
				return fmt.Errorf("the OpenCode skill %q already exists and is not owned", object.LogicalName)
			} else if !os.IsNotExist(err) {
				return err
			}
		}
	}
	_ = activePath
	return nil
}
