package providers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	kiroSkillObjectKind = "kiro_global_skill_directory"
	kiroMCPObjectKind   = "kiro_global_mcp_server"
)

func buildKiroNativeObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	configRoot := strings.TrimSpace(plan.NativeRegistryRoot)
	if configRoot == "" {
		return nil, fmt.Errorf("Kiro config root is unavailable")
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(envelope.Skills)+len(envelope.MCP.Servers))
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		switch component.Kind {
		case domain.ComponentSkill:
			if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
				return nil, fmt.Errorf("invalid Kiro skill name %q: %w", component.Name, err)
			}
			skill, ok := envelope.Skills[component.Name]
			if !ok {
				return nil, fmt.Errorf("planned Kiro skill %q is missing", component.Name)
			}
			relative := filepath.FromSlash(strings.TrimSpace(skill.RelativePath))
			if relative == "" {
				relative = filepath.Join("skills", component.Name, "SKILL.md")
			}
			sourceRoot := filepath.Join(stagingRoot, filepath.Dir(relative))
			if err := pathpolicy.RequireContainedChild(stagingRoot, sourceRoot); err != nil {
				return nil, fmt.Errorf("unsafe Kiro skill source %q: %w", component.Name, err)
			}
			digest, err := digestKiroSkillDirectory(sourceRoot)
			if err != nil {
				return nil, fmt.Errorf("digest Kiro skill %q: %w", component.Name, err)
			}
			target := filepath.Join(configRoot, "skills", component.Name)
			if err := validateKiroNativePath(configRoot, target); err != nil {
				return nil, err
			}
			objects = append(objects, domain.NativeObjectOwnership{
				ObjectID: "kiro-skill:" + component.Name, Kind: kiroSkillObjectKind,
				LogicalName: component.Name, Path: target,
				SourceRelative: filepath.ToSlash(filepath.Dir(relative)), ManagedDigest: digest,
				ProtectionClass: "managed",
			})
		case domain.ComponentMCPServer:
			if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
				return nil, fmt.Errorf("invalid Kiro MCP server name %q: %w", component.Name, err)
			}
			server, err := projectedKiroMCPServer(stagingRoot, component.Name)
			if err != nil {
				return nil, err
			}
			objects = append(objects, domain.NativeObjectOwnership{
				ObjectID: "kiro-mcp:" + component.Name, Kind: kiroMCPObjectKind,
				LogicalName: component.Name, Path: filepath.Join(configRoot, "settings", "mcp.json"),
				ManagedDigest: digestJSONObject(server), ProtectionClass: "managed",
			})
		}
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}

func activateKiroNative(ctx context.Context, request domain.ActivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyKiroNativeMutation(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects)
}

func deactivateKiroNative(ctx context.Context, request domain.DeactivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyKiroNativeMutation(request.Client.ConfigRoot, "", request.NativeObjects, nil)
}

func verifyKiroNativeObjects(configRoot string, objects []domain.NativeObjectOwnership, allowMissing bool) error {
	filtered := kiroObjects(objects)
	for _, object := range filtered {
		if err := validateKiroObject(configRoot, object); err != nil {
			return err
		}
	}
	mcp := map[string]any{}
	if len(previousMCPObjects(filtered)) > 0 {
		var err error
		mcp, _, _, _, err = readKiroMCPConfig(filepath.Join(configRoot, "settings", "mcp.json"))
		if err != nil {
			return err
		}
	}
	for _, object := range filtered {
		switch object.Kind {
		case kiroSkillObjectKind:
			digest, err := digestKiroSkillDirectory(object.Path)
			if os.IsNotExist(err) && allowMissing {
				continue
			}
			if err != nil {
				return fmt.Errorf("inspect managed Kiro skill %q: %w", object.LogicalName, err)
			}
			if digest != object.ManagedDigest {
				return fmt.Errorf("managed Kiro skill %q changed outside agentplugins", object.LogicalName)
			}
		case kiroMCPObjectKind:
			rawServer, exists := mcp[object.LogicalName]
			if !exists && allowMissing {
				continue
			}
			if !exists {
				return fmt.Errorf("managed Kiro MCP server %q is missing", object.LogicalName)
			}
			server, ok := rawServer.(map[string]any)
			if !ok {
				return fmt.Errorf("managed Kiro MCP server %q is malformed", object.LogicalName)
			}
			if digestJSONObject(server) != object.ManagedDigest {
				return fmt.Errorf("managed Kiro MCP server %q changed outside agentplugins", object.LogicalName)
			}
		}
	}
	return nil
}

func applyKiroNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) (resultErr error) {
	configRoot = strings.TrimSpace(configRoot)
	if configRoot == "" {
		return fmt.Errorf("Kiro config root is unavailable")
	}
	previous, desired = kiroObjects(previous), kiroObjects(desired)
	if err := verifyKiroNativeObjects(configRoot, previous, true); err != nil {
		return err
	}
	previousByID, desiredByID := objectMap(previous), objectMap(desired)
	for id, object := range desiredByID {
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !shared.SameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("Kiro native object identity changed unexpectedly for %s", id)
			}
			continue
		}
		if err := requireKiroObjectAbsent(configRoot, object); err != nil {
			return err
		}
	}

	transactionRoot := ""
	if hasKiroSkillObjects(previous) || hasKiroSkillObjects(desired) {
		skillsRoot := filepath.Join(configRoot, "skills")
		if err := validateKiroNativePath(configRoot, skillsRoot); err != nil {
			return err
		}
		if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
			return fmt.Errorf("create Kiro skills root: %w", err)
		}
		var err error
		transactionRoot, err = os.MkdirTemp(skillsRoot, ".agentplugins-native-")
		if err != nil {
			return fmt.Errorf("create Kiro native transaction: %w", err)
		}
		defer os.RemoveAll(transactionRoot)
	}

	staged := map[string]string{}
	for id, object := range desiredByID {
		if object.Kind != kiroSkillObjectKind {
			continue
		}
		if strings.TrimSpace(activePath) == "" {
			return fmt.Errorf("active package path is required for Kiro skill installation")
		}
		source := filepath.Join(activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(activePath, source); err != nil {
			return fmt.Errorf("unsafe Kiro skill source for %q: %w", object.LogicalName, err)
		}
		target := filepath.Join(transactionRoot, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return fmt.Errorf("stage Kiro skill %q: %w", object.LogicalName, err)
		}
		digest, err := digestKiroSkillDirectory(target)
		if err != nil || digest != object.ManagedDigest {
			return fmt.Errorf("staged Kiro skill %q does not match its ownership digest", object.LogicalName)
		}
		staged[id] = target
	}

	mcpPath := filepath.Join(configRoot, "settings", "mcp.json")
	mcp, originalMCP, originalMode, originalExists := map[string]any{}, []byte(nil), os.FileMode(0o600), false
	hasMCPMutation := len(previousMCPObjects(previous))+len(previousMCPObjects(desired)) > 0
	if hasMCPMutation {
		var err error
		mcp, originalMCP, originalMode, originalExists, err = readKiroMCPConfig(mcpPath)
		if err != nil {
			return err
		}
		for _, object := range previousByID {
			if object.Kind == kiroMCPObjectKind {
				delete(mcp, object.LogicalName)
			}
		}
		for _, object := range desiredByID {
			if object.Kind != kiroMCPObjectKind {
				continue
			}
			server, err := projectedKiroMCPServer(activePath, object.LogicalName)
			if err != nil {
				return err
			}
			if digestJSONObject(server) != object.ManagedDigest {
				return fmt.Errorf("projected Kiro MCP server %q does not match its ownership digest", object.LogicalName)
			}
			mcp[object.LogicalName] = server
		}
	}
	newMCP := []byte(nil)
	if hasMCPMutation {
		var err error
		newMCP, err = encodeKiroMCPConfig(originalMCP, mcp)
		if err != nil {
			return err
		}
	}

	backups, installed := map[string]string{}, map[string]domain.NativeObjectOwnership{}
	mcpWritten := false
	rollback := func() error {
		var rollbackErr error
		if mcpWritten {
			if originalExists {
				rollbackErr = atomicfile.Write(mcpPath, originalMCP, originalMode)
			} else if body, readErr := os.ReadFile(mcpPath); readErr == nil && bytes.Equal(body, newMCP) {
				rollbackErr = os.Remove(mcpPath)
			}
		}
		for id, object := range installed {
			if digest, digestErr := digestKiroSkillDirectory(object.Path); digestErr == nil && digest == object.ManagedDigest {
				if err := os.RemoveAll(object.Path); err != nil && rollbackErr == nil {
					rollbackErr = err
				}
			}
			if backup := backups[id]; backup != "" {
				if err := os.Rename(backup, object.Path); err != nil && rollbackErr == nil {
					rollbackErr = err
				}
				delete(backups, id)
			}
		}
		for id, backup := range backups {
			if err := os.Rename(backup, previousByID[id].Path); err != nil && rollbackErr == nil {
				rollbackErr = err
			}
		}
		return rollbackErr
	}
	defer func() {
		if resultErr != nil {
			if rollbackErr := rollback(); rollbackErr != nil {
				resultErr = fmt.Errorf("%v; Kiro native rollback failed: %w", resultErr, rollbackErr)
			}
		}
	}()

	for id, object := range previousByID {
		if object.Kind != kiroSkillObjectKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		backup := filepath.Join(transactionRoot, "old-"+object.LogicalName)
		if err := os.Rename(object.Path, backup); err != nil {
			return fmt.Errorf("backup Kiro skill %q: %w", object.LogicalName, err)
		}
		backups[id] = backup
	}
	for id, source := range staged {
		object := desiredByID[id]
		if err := os.Rename(source, object.Path); err != nil {
			return fmt.Errorf("activate Kiro skill %q: %w", object.LogicalName, err)
		}
		installed[id] = object
	}
	if hasMCPMutation {
		if err := atomicfile.Write(mcpPath, newMCP, originalMode); err != nil {
			return fmt.Errorf("write Kiro MCP configuration: %w", err)
		}
		mcpWritten = true
	}
	if err := verifyKiroNativeObjects(configRoot, desired, false); err != nil {
		return err
	}
	return nil
}

func requireKiroObjectAbsent(configRoot string, object domain.NativeObjectOwnership) error {
	if err := validateKiroObject(configRoot, object); err != nil {
		return err
	}
	switch object.Kind {
	case kiroSkillObjectKind:
		if _, err := os.Lstat(object.Path); err == nil {
			return fmt.Errorf("Kiro skill %q already exists without agentplugins ownership", object.LogicalName)
		} else if !os.IsNotExist(err) {
			return err
		}
	case kiroMCPObjectKind:
		mcp, _, _, _, err := readKiroMCPConfig(object.Path)
		if err != nil {
			return err
		}
		if _, exists := mcp[object.LogicalName]; exists {
			return fmt.Errorf("Kiro MCP server %q already exists without agentplugins ownership", object.LogicalName)
		}
	}
	return nil
}

func validateKiroObject(configRoot string, object domain.NativeObjectOwnership) error {
	if err := pathpolicy.ValidateLeafID(object.LogicalName); err != nil {
		return fmt.Errorf("invalid Kiro native object name %q: %w", object.LogicalName, err)
	}
	var expected string
	switch object.Kind {
	case kiroSkillObjectKind:
		expected = filepath.Join(configRoot, "skills", object.LogicalName)
	case kiroMCPObjectKind:
		expected = filepath.Join(configRoot, "settings", "mcp.json")
	default:
		return fmt.Errorf("unsupported Kiro native object kind %q", object.Kind)
	}
	if !shared.SameCleanPath(expected, object.Path) {
		return fmt.Errorf("Kiro native object %q has an untrusted path", object.LogicalName)
	}
	return validateKiroNativePath(configRoot, object.Path)
}

func validateKiroNativePath(configRoot, path string) error {
	if err := pathpolicy.RequireContainedChild(configRoot, path); err != nil {
		return fmt.Errorf("unsafe Kiro native path: %w", err)
	}
	return nil
}

func projectedKiroMCPServer(root, name string) (map[string]any, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("prepared Kiro package path is unavailable")
	}
	body, err := os.ReadFile(filepath.Join(root, "mcp.json"))
	if err != nil {
		return nil, fmt.Errorf("read projected Kiro MCP configuration: %w", err)
	}
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return nil, fmt.Errorf("decode projected Kiro MCP configuration: %w", err)
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("projected Kiro MCP configuration has no mcpServers object")
	}
	server, ok := servers[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("projected Kiro MCP server %q is missing", name)
	}
	return server, nil
}

func readKiroMCPConfig(path string) (servers map[string]any, original []byte, mode os.FileMode, exists bool, err error) {
	mode = 0o600
	body, readErr := os.ReadFile(path)
	if os.IsNotExist(readErr) {
		return map[string]any{}, nil, mode, false, nil
	}
	if readErr != nil {
		return nil, nil, mode, false, fmt.Errorf("read Kiro MCP configuration: %w", readErr)
	}
	info, statErr := os.Lstat(path)
	if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, nil, mode, false, fmt.Errorf("Kiro MCP configuration must be a regular file")
	}
	document, decodeErr := shared.DecodeStrictJSONObject(body)
	if decodeErr != nil {
		return nil, nil, mode, false, fmt.Errorf("decode Kiro MCP configuration: %w", decodeErr)
	}
	if raw, present := document["mcpServers"]; present {
		servers, exists = raw.(map[string]any)
		if !exists {
			return nil, nil, mode, false, fmt.Errorf("Kiro mcpServers must be an object")
		}
	} else {
		servers = map[string]any{}
	}
	mode = info.Mode().Perm()
	return servers, body, mode, true, nil
}

func encodeKiroMCPConfig(original []byte, servers map[string]any) ([]byte, error) {
	document := map[string]any{}
	if len(original) > 0 {
		var err error
		document, err = shared.DecodeStrictJSONObject(original)
		if err != nil {
			return nil, fmt.Errorf("decode existing Kiro MCP configuration: %w", err)
		}
	}
	document["mcpServers"] = servers
	body, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode Kiro MCP configuration: %w", err)
	}
	return append(body, '\n'), nil
}

func digestKiroSkillDirectory(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed: %s", relative)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		kind := byte('d')
		if !entry.IsDir() {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("special file is not allowed: %s", relative)
			}
			kind = 'f'
		}
		_, _ = hash.Write([]byte{kind, 0})
		_, _ = io.WriteString(hash, filepath.ToSlash(relative))
		_, _ = hash.Write([]byte{0})
		if kind == 'f' {
			if info.Mode().Perm()&0o111 != 0 {
				_, _ = hash.Write([]byte{'x', 0})
			} else {
				_, _ = hash.Write([]byte{'-', 0})
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(hash, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			_, _ = hash.Write([]byte{0})
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func digestJSONObject(value map[string]any) string {
	body, _ := json.Marshal(value)
	digest := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func kiroObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == kiroSkillObjectKind || object.Kind == kiroMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func previousMCPObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind == kiroMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func hasKiroSkillObjects(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == kiroSkillObjectKind {
			return true
		}
	}
	return false
}

func objectMap(objects []domain.NativeObjectOwnership) map[string]domain.NativeObjectOwnership {
	result := make(map[string]domain.NativeObjectOwnership, len(objects))
	for _, object := range objects {
		result[object.ObjectID] = object
	}
	return result
}
