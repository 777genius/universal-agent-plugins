package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers/nativeconfig"
)

const (
	geminiSkillObjectKind = "gemini_global_skill_directory"
	geminiMCPObjectKind   = "gemini_global_mcp_server"
	geminiDescriptorName  = ".agentplugins-gemini.json"
)

type geminiDescriptor struct {
	DataRoot string `json:"data_root"`
}

func buildGeminiNativeObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, pluginDataPath string) ([]domain.NativeObjectOwnership, error) {
	configRoot := strings.TrimSpace(plan.NativeRegistryRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return nil, fmt.Errorf("Gemini config root is unavailable")
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(envelope.Skills)+len(envelope.MCP.Servers))
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
			return nil, fmt.Errorf("invalid Gemini component name %q: %w", component.Name, err)
		}
		switch component.Kind {
		case domain.ComponentSkill:
			skill, ok := envelope.Skills[component.Name]
			if !ok {
				return nil, fmt.Errorf("planned Gemini skill %q is missing", component.Name)
			}
			relative := filepath.FromSlash(strings.TrimSpace(skill.RelativePath))
			if relative == "" {
				relative = filepath.Join("skills", component.Name, "SKILL.md")
			}
			sourceRelative := filepath.Dir(relative)
			sourceRoot := filepath.Join(stagingRoot, sourceRelative)
			if err := pathpolicy.RequireContainedChild(stagingRoot, sourceRoot); err != nil {
				return nil, fmt.Errorf("unsafe Gemini skill source %q: %w", component.Name, err)
			}
			digest, err := digestKiroSkillDirectory(sourceRoot)
			if err != nil {
				return nil, fmt.Errorf("digest Gemini skill %q: %w", component.Name, err)
			}
			objects = append(objects, domain.NativeObjectOwnership{
				ObjectID: "gemini-skill:" + component.Name, Kind: geminiSkillObjectKind,
				LogicalName: component.Name, Path: filepath.Join(configRoot, "skills", component.Name),
				SourceRelative: filepath.ToSlash(sourceRelative), ManagedDigest: digest, ProtectionClass: "managed",
			})
		case domain.ComponentMCPServer:
			server, ok := envelope.MCP.Servers[component.Name]
			if !ok {
				return nil, fmt.Errorf("planned Gemini MCP server %q is missing", component.Name)
			}
			native, err := geminiNativeServer(server)
			if err != nil {
				return nil, fmt.Errorf("project Gemini MCP server %q: %w", component.Name, err)
			}
			native, err = materializeGeminiServer(native, plan.ActivePath, pluginDataPath, stagingRoot)
			if err != nil {
				return nil, fmt.Errorf("bind Gemini MCP server %q: %w", component.Name, err)
			}
			receipt, err := nativeconfig.DesiredReceipt(filepath.Join(configRoot, "settings.json"), nativeconfig.CodecGemini, component.Name, native, nativeconfig.Placeholders{PackageRoot: plan.ActivePath, DataRoot: pluginDataPath})
			if err != nil {
				return nil, err
			}
			objects = append(objects, domain.NativeObjectOwnership{
				ObjectID: "gemini-mcp:" + component.Name, Kind: geminiMCPObjectKind,
				LogicalName: component.Name, Path: filepath.Join(configRoot, "settings.json"),
				ManagedDigest: receipt.Digest, ProtectionClass: "managed",
			})
		}
	}
	descriptorBody, err := json.Marshal(geminiDescriptor{DataRoot: pluginDataPath})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(stagingRoot, geminiDescriptorName), append(descriptorBody, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("write Gemini projection descriptor: %w", err)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}

func activateGeminiNative(ctx context.Context, request domain.ActivationRequest) error {
	return activateGeminiNativeWithKernel(ctx, request, nativeconfig.New())
}

func activateGeminiNativeWithKernel(ctx context.Context, request domain.ActivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyGeminiNativeMutationWithKernel(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, kernel)
}

func deactivateGeminiNative(ctx context.Context, request domain.DeactivationRequest) error {
	return deactivateGeminiNativeWithKernel(ctx, request, nativeconfig.New())
}

func deactivateGeminiNativeWithKernel(ctx context.Context, request domain.DeactivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyGeminiNativeMutationWithKernel(request.Client.ConfigRoot, "", request.NativeObjects, nil, kernel)
}

func verifyGeminiNativeObjects(configRoot string, objects []domain.NativeObjectOwnership, allowMissing bool) error {
	for _, object := range geminiObjects(objects) {
		if err := validateGeminiObject(configRoot, object); err != nil {
			return err
		}
		switch object.Kind {
		case geminiSkillObjectKind:
			digest, err := digestKiroSkillDirectory(object.Path)
			if os.IsNotExist(err) && allowMissing {
				continue
			}
			if err != nil {
				return fmt.Errorf("inspect managed Gemini skill %q: %w", object.LogicalName, err)
			}
			if digest != object.ManagedDigest {
				return fmt.Errorf("managed Gemini skill %q changed outside agentplugins", object.LogicalName)
			}
		case geminiMCPObjectKind:
			present, owned, err := nativeconfig.New().Inspect(geminiConfigPaths(configRoot), nativeconfig.CodecGemini, object.LogicalName, geminiReceipt(object))
			if err != nil {
				return err
			}
			if !present && allowMissing {
				continue
			}
			if !present || !owned {
				return fmt.Errorf("managed Gemini MCP server %q changed outside agentplugins", object.LogicalName)
			}
		}
	}
	return nil
}

type geminiRenameFunc func(string, string) error

func renameGeminiDirectoryNoReplace(oldPath, newPath string, rename geminiRenameFunc) error {
	if _, err := os.Lstat(newPath); err == nil {
		return fmt.Errorf("destination already exists: %s", newPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	return rename(oldPath, newPath)
}

func applyGeminiNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyGeminiNativeMutationWithKernel(configRoot, activePath, previous, desired, nativeconfig.New())
}

func applyGeminiNativeMutationWithRename(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename geminiRenameFunc) (resultErr error) {
	return applyGeminiNativeMutationWithKernelAndRename(configRoot, activePath, previous, desired, nativeconfig.New(), rename)
}

func applyGeminiNativeMutationWithKernel(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	return applyGeminiNativeMutationWithKernelAndRename(configRoot, activePath, previous, desired, kernel, renameDirectoryExclusive)
}

func applyGeminiNativeMutationWithKernelAndRename(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename geminiRenameFunc) (resultErr error) {
	return applyGeminiNativeMutationWithKernelRenameAndCapacity(configRoot, activePath, previous, desired, kernel, rename, checkedCombinedCapacity)
}

func applyGeminiNativeMutationWithKernelRenameAndCapacity(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename geminiRenameFunc, capacity combinedCapacityFunc) (resultErr error) {
	if rename == nil {
		return fmt.Errorf("Gemini rename operation is unavailable")
	}
	if capacity == nil {
		return fmt.Errorf("Gemini capacity checker is unavailable")
	}
	configRoot = strings.TrimSpace(configRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return fmt.Errorf("Gemini config root is unavailable")
	}
	previous, desired = geminiObjects(previous), geminiObjects(desired)
	if err := verifyGeminiNativeObjects(configRoot, previous, true); err != nil {
		return err
	}
	previousByID, desiredByID := objectMap(previous), objectMap(desired)
	idsCapacity, capacityErr := capacity(len(previousByID), len(desiredByID))
	if capacityErr != nil {
		return fmt.Errorf("prepare managed Gemini object set: %w", capacityErr)
	}
	for id, object := range desiredByID {
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !sameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("Gemini native object identity changed unexpectedly for %s", id)
			}
			continue
		}
		if err := requireGeminiObjectAbsent(configRoot, object); err != nil {
			return err
		}
	}

	descriptor := geminiDescriptor{}
	if len(desired) > 0 {
		if strings.TrimSpace(activePath) == "" {
			return fmt.Errorf("active package path is required for Gemini native installation")
		}
		body, err := os.ReadFile(filepath.Join(activePath, geminiDescriptorName))
		if err != nil || json.Unmarshal(body, &descriptor) != nil || strings.TrimSpace(descriptor.DataRoot) == "" || !filepath.IsAbs(descriptor.DataRoot) {
			return fmt.Errorf("read Gemini projection descriptor: invalid or missing descriptor")
		}
	}

	transactionRoot := ""
	cleanupTransaction := true
	if hasGeminiSkillObjects(previous) || hasGeminiSkillObjects(desired) {
		skillsRoot := filepath.Join(configRoot, "skills")
		if err := pathpolicy.RequireContainedChild(configRoot, skillsRoot); err != nil {
			return err
		}
		if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
			return fmt.Errorf("create Gemini skills root: %w", err)
		}
		var err error
		transactionRoot, err = os.MkdirTemp(skillsRoot, ".agentplugins-native-")
		if err != nil {
			return fmt.Errorf("create Gemini native transaction: %w", err)
		}
	}

	staged := map[string]string{}
	for id, object := range desiredByID {
		if object.Kind != geminiSkillObjectKind {
			continue
		}
		source := filepath.Join(activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(activePath, source); err != nil {
			return fmt.Errorf("unsafe Gemini skill source for %q: %w", object.LogicalName, err)
		}
		target := filepath.Join(transactionRoot, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return fmt.Errorf("stage Gemini skill %q: %w", object.LogicalName, err)
		}
		if digest, err := digestKiroSkillDirectory(target); err != nil || digest != object.ManagedDigest {
			return fmt.Errorf("staged Gemini skill %q does not match its ownership digest", object.LogicalName)
		}
		staged[id] = target
	}

	backups, installed := map[string]string{}, map[string]domain.NativeObjectOwnership{}
	rollbackSkills := func() error {
		var rollbackErr error
		attempted := map[string]bool{}
		for id, object := range installed {
			attempted[id] = true
			if digest, err := digestKiroSkillDirectory(object.Path); err == nil && digest == object.ManagedDigest {
				if err := os.RemoveAll(object.Path); err != nil && rollbackErr == nil {
					rollbackErr = err
				}
			}
			if backup := backups[id]; backup != "" {
				if err := renameGeminiDirectoryNoReplace(backup, object.Path, rename); err != nil {
					if rollbackErr == nil {
						rollbackErr = err
					}
				} else {
					delete(backups, id)
				}
			}
		}
		for id, backup := range backups {
			if attempted[id] {
				continue
			}
			if err := renameGeminiDirectoryNoReplace(backup, previousByID[id].Path, rename); err != nil {
				if rollbackErr == nil {
					rollbackErr = err
				}
			} else {
				delete(backups, id)
			}
		}
		return rollbackErr
	}
	defer func() {
		if cleanupTransaction && transactionRoot != "" {
			_ = os.RemoveAll(transactionRoot)
		}
	}()
	// A failed restore leaves the only recoverable copy inside transactionRoot.
	// Keep that directory intact and report its exact location to the caller.
	defer func() {
		if resultErr == nil || nativeconfig.IsCommittedCleanup(resultErr) {
			return
		}
		if rollbackErr := rollbackSkills(); rollbackErr != nil {
			cleanupTransaction = false
			resultErr = fmt.Errorf("%v; Gemini skill rollback failed: %w; recovery retained at %q", resultErr, rollbackErr, transactionRoot)
		}
	}()
	for id, object := range previousByID {
		if object.Kind != geminiSkillObjectKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		backup := filepath.Join(transactionRoot, "old-"+object.LogicalName)
		if err := rename(object.Path, backup); err != nil {
			return fmt.Errorf("backup Gemini skill %q: %w", object.LogicalName, err)
		}
		backups[id] = backup
		digest, digestErr := digestKiroSkillDirectory(backup)
		if digestErr != nil || digest != object.ManagedDigest {
			if digestErr != nil {
				return fmt.Errorf("verify isolated Gemini skill backup %q: %w", object.LogicalName, digestErr)
			}
			return fmt.Errorf("isolated Gemini skill backup %q changed outside agentplugins", object.LogicalName)
		}
	}
	for id, source := range staged {
		object := desiredByID[id]
		if err := renameGeminiDirectoryNoReplace(source, object.Path, rename); err != nil {
			return fmt.Errorf("activate Gemini skill %q: %w", object.LogicalName, err)
		}
		installed[id] = object
	}

	requests := make([]nativeconfig.Request, 0)
	ids := make([]string, 0, idsCapacity)
	seen := map[string]bool{}
	for id := range previousByID {
		ids, seen[id] = append(ids, id), true
	}
	for id := range desiredByID {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		prior, hadPrior := previousByID[id]
		next, hasNext := desiredByID[id]
		if (hadPrior && prior.Kind != geminiMCPObjectKind) || (hasNext && next.Kind != geminiMCPObjectKind) {
			continue
		}
		if hasNext {
			server, err := geminiServerFromPackage(activePath, next.LogicalName)
			if err != nil {
				return err
			}
			server, err = materializeGeminiServer(server, activePath, descriptor.DataRoot)
			if err != nil {
				return err
			}
			present, owned := false, false
			if hadPrior {
				present, owned, err = nativeconfig.New().Inspect(geminiConfigPaths(configRoot), nativeconfig.CodecGemini, prior.LogicalName, geminiReceipt(prior))
				if err != nil {
					return err
				}
			}
			action := nativeconfig.ActionAdd
			var receipt *nativeconfig.Receipt
			if present {
				if !owned {
					return fmt.Errorf("Gemini MCP server %q is no longer owned", prior.LogicalName)
				}
				action, receipt = nativeconfig.ActionUpdate, geminiReceipt(prior)
			}
			requests = append(requests, nativeconfig.Request{Paths: geminiConfigPaths(configRoot), Codec: nativeconfig.CodecGemini, Action: action, Name: next.LogicalName, Server: server, Placeholders: nativeconfig.Placeholders{PackageRoot: activePath, DataRoot: descriptor.DataRoot}, Owned: receipt})
		} else if hadPrior {
			present, owned, err := nativeconfig.New().Inspect(geminiConfigPaths(configRoot), nativeconfig.CodecGemini, prior.LogicalName, geminiReceipt(prior))
			if err != nil {
				return err
			}
			if present {
				if !owned {
					return fmt.Errorf("Gemini MCP server %q is no longer owned", prior.LogicalName)
				}
				requests = append(requests, nativeconfig.Request{Paths: geminiConfigPaths(configRoot), Codec: nativeconfig.CodecGemini, Action: nativeconfig.ActionRemove, Name: prior.LogicalName, Owned: geminiReceipt(prior)})
			}
		}
	}
	for _, request := range requests {
		if request.Action == nativeconfig.ActionRemove {
			continue
		}
		expected := desiredByID["gemini-mcp:"+request.Name]
		preview, err := nativeconfig.DesiredReceipt(filepath.Join(configRoot, "settings.json"), nativeconfig.CodecGemini, request.Name, request.Server, nativeconfig.Placeholders{PackageRoot: activePath, DataRoot: descriptor.DataRoot})
		if err != nil || preview.Digest != expected.ManagedDigest {
			return fmt.Errorf("Gemini MCP server %q does not match staged ownership", request.Name)
		}
	}
	_, err := kernel.ApplyBatch(requests)
	if err != nil {
		return err
	}
	backups = map[string]string{}
	return nil
}

func inspectGeminiRegistry(plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" {
		return registryIndeterminate, nil
	}
	if managed != nil {
		if err := verifyGeminiNativeObjects(root, managed.NativeObjects, true); err != nil {
			return registryIndeterminate, err
		}
	}
	finding := registryClear
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		exists, owned := false, false
		switch component.Kind {
		case domain.ComponentSkill:
			path := filepath.Join(root, "skills", component.Name)
			_, err := os.Lstat(path)
			exists = err == nil
			if err != nil && !os.IsNotExist(err) {
				return registryIndeterminate, err
			}
			owned = managed != nil && managedGeminiObjectExists(managed.NativeObjects, geminiSkillObjectKind, component.Name)
		case domain.ComponentMCPServer:
			var receipt *nativeconfig.Receipt
			if managed != nil {
				for _, object := range geminiObjects(managed.NativeObjects) {
					if object.Kind == geminiMCPObjectKind && object.LogicalName == component.Name {
						receipt = geminiReceipt(object)
					}
				}
			}
			var err error
			exists, owned, err = nativeconfig.New().Inspect(geminiConfigPaths(root), nativeconfig.CodecGemini, component.Name, receipt)
			if err != nil {
				return registryIndeterminate, err
			}
		default:
			continue
		}
		if exists && !owned {
			return registryCollision, nil
		}
		if exists && owned {
			finding = registryExpected
		}
	}
	return finding, nil
}

func geminiNativeServer(server domain.MCPServer) (nativeconfig.Server, error) {
	getString := func(key string) (string, error) {
		value, exists := server.Decoded[key]
		if !exists {
			return "", nil
		}
		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%s must be a string", key)
		}
		return text, nil
	}
	result := nativeconfig.Server{}
	switch server.Type {
	case "stdio":
		result.Type = "stdio"
		var err error
		result.Command, err = getString("command")
		if err != nil {
			return result, err
		}
		result.CWD, err = getString("cwd")
		if err != nil {
			return result, err
		}
		if strings.HasPrefix(result.CWD, "./") {
			result.CWD = "${PLUGIN_ROOT}/" + strings.TrimPrefix(result.CWD, "./")
		}
		if result.CWD == "" {
			result.CWD = "${PLUGIN_ROOT}"
		}
		if raw, ok := server.Decoded["args"]; ok {
			values, ok := raw.([]any)
			if !ok {
				return result, fmt.Errorf("args must be an array")
			}
			for _, value := range values {
				text, ok := value.(string)
				if !ok {
					return result, fmt.Errorf("args must contain strings")
				}
				result.Args = append(result.Args, text)
			}
		}
		if raw, ok := server.Decoded["env"]; ok {
			values, ok := raw.(map[string]any)
			if !ok {
				return result, fmt.Errorf("env must be an object")
			}
			result.Env = map[string]string{}
			for key, value := range values {
				text, ok := value.(string)
				if !ok {
					return result, fmt.Errorf("env values must be strings")
				}
				result.Env[key] = text
			}
		}
		if result.Env == nil {
			result.Env = map[string]string{}
		}
		result.Env["PLUGIN_ROOT"] = "${PLUGIN_ROOT}"
		result.Env["PLUGIN_DATA"] = "${PLUGIN_DATA}"
	case "streamable-http", "sse":
		result.Type, result.RemoteTransport = "remote", server.Type
		var err error
		result.URL, err = getString("url")
		if err != nil {
			return result, err
		}
		if raw, ok := server.Decoded["headers"]; ok {
			values, ok := raw.(map[string]any)
			if !ok {
				return result, fmt.Errorf("headers must be an object")
			}
			result.Headers = map[string]string{}
			for key, value := range values {
				text, ok := value.(string)
				if !ok {
					return result, fmt.Errorf("header values must be strings")
				}
				result.Headers[key] = text
			}
		}
	default:
		return result, fmt.Errorf("unsupported transport %q", server.Type)
	}
	return result, nil
}

func materializeGeminiServer(server nativeconfig.Server, packageRoot, dataRoot string, observationRoot ...string) (nativeconfig.Server, error) {
	if server.Type != "stdio" {
		return server, nil
	}
	command, cwd, err := resolveStdioPaths(server.Command, server.CWD, packageRoot, dataRoot, observationRoot...)
	if err != nil {
		return server, err
	}
	server.Command, server.CWD = command, cwd
	server.CWDResolved = true
	return server, nil
}

func geminiServerFromPackage(root, name string) (nativeconfig.Server, error) {
	body, err := os.ReadFile(filepath.Join(root, "mcp.json"))
	if err != nil {
		return nativeconfig.Server{}, fmt.Errorf("read Gemini package MCP configuration: %w", err)
	}
	document, err := decodeStrictJSONObject(body)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("Gemini package has no mcpServers object")
	}
	decoded, ok := servers[name].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("Gemini MCP server %q is missing", name)
	}
	typeName, _ := decoded["type"].(string)
	return geminiNativeServer(domain.MCPServer{Name: name, Type: typeName, Decoded: decoded})
}

func geminiReceipt(object domain.NativeObjectOwnership) *nativeconfig.Receipt {
	return &nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecGemini, Name: object.LogicalName, Digest: object.ManagedDigest}
}
func geminiConfigPaths(root string) nativeconfig.Paths {
	return nativeconfig.Paths{JSON: filepath.Join(root, "settings.json")}
}
func geminiObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind == geminiSkillObjectKind || object.Kind == geminiMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}
func geminiNativeComponents(components []domain.ComponentDecision) bool {
	has := false
	for _, component := range components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if component.Kind != domain.ComponentSkill && component.Kind != domain.ComponentMCPServer {
			return false
		}
		has = true
	}
	return has
}
func hasGeminiSkillObjects(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == geminiSkillObjectKind {
			return true
		}
	}
	return false
}
func managedGeminiObjectExists(objects []domain.NativeObjectOwnership, kind, name string) bool {
	for _, object := range geminiObjects(objects) {
		if object.Kind == kind && object.LogicalName == name {
			return true
		}
	}
	return false
}
func requireGeminiObjectAbsent(root string, object domain.NativeObjectOwnership) error {
	if err := validateGeminiObject(root, object); err != nil {
		return err
	}
	if object.Kind == geminiSkillObjectKind {
		if _, err := os.Lstat(object.Path); err == nil {
			return fmt.Errorf("Gemini skill %q already exists without agentplugins ownership", object.LogicalName)
		} else if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	present, _, err := nativeconfig.New().Inspect(geminiConfigPaths(root), nativeconfig.CodecGemini, object.LogicalName, nil)
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("Gemini MCP server %q already exists without agentplugins ownership", object.LogicalName)
	}
	return nil
}
func validateGeminiObject(root string, object domain.NativeObjectOwnership) error {
	if err := pathpolicy.ValidateLeafID(object.LogicalName); err != nil {
		return err
	}
	expected := filepath.Join(root, "settings.json")
	if object.Kind == geminiSkillObjectKind {
		expected = filepath.Join(root, "skills", object.LogicalName)
	} else if object.Kind != geminiMCPObjectKind {
		return fmt.Errorf("unsupported Gemini native object kind %q", object.Kind)
	}
	if !sameCleanPath(expected, object.Path) {
		return fmt.Errorf("Gemini native object %q has an untrusted path", object.LogicalName)
	}
	return pathpolicy.RequireContainedChild(root, object.Path)
}
