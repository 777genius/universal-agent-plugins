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
	clineSkillObjectKind = "cline_global_skill_directory"
	clineMCPObjectKind   = "cline_global_mcp_server"
	clineProjectionFile  = ".agentplugins-cline-native.json"
)

type clineProjection struct {
	Servers map[string]nativeconfig.Server `json:"servers"`
}

func projectClineNative(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	servers := make(map[string]nativeconfig.Server)
	for _, name := range supportedMCPNames(plan) {
		portable := envelope.MCP.Servers[name]
		if portable.Type == "stdio" {
			portable.Decoded = cloneObject(portable.Decoded)
			if err := applyStdioDataContract(portable.Decoded, plan.ActivePath, dataPath, root); err != nil {
				return fmt.Errorf("project Cline MCP server %s: %w", name, err)
			}
		}
		server, err := clineNeutralServer(portable)
		if err != nil {
			return fmt.Errorf("project Cline MCP server %s: %w", name, err)
		}
		server.StdioValuesResolved = portable.Type == "stdio"
		// This first pass validates the codec without touching client state. The
		// exact configured path is bound later when ownership objects are built.
		settingsPath := filepath.Join(root, ".cline-settings-validation.json")
		if _, err := nativeconfig.DesiredReceipt(settingsPath, nativeconfig.CodecCline, name, server, nativeconfig.Placeholders{PackageRoot: plan.ActivePath, DataRoot: dataPath}); err != nil {
			return fmt.Errorf("project Cline MCP server %s: %w", name, err)
		}
		servers[name] = server
	}
	if len(servers) == 0 {
		return nil
	}
	return writeJSON(filepath.Join(root, clineProjectionFile), clineProjection{Servers: servers})
}

func buildClineNativeObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("Cline config root is unavailable")
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(envelope.Skills)+len(envelope.MCP.Servers))
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
			return nil, fmt.Errorf("invalid Cline component name %q: %w", component.Name, err)
		}
		switch component.Kind {
		case domain.ComponentSkill:
			skill, ok := envelope.Skills[component.Name]
			if !ok {
				return nil, fmt.Errorf("planned Cline skill %q is missing", component.Name)
			}
			relative := filepath.FromSlash(strings.TrimSpace(skill.RelativePath))
			if relative == "" {
				relative = filepath.Join("skills", component.Name, "SKILL.md")
			}
			sourceRoot := filepath.Join(stagingRoot, filepath.Dir(relative))
			if err := pathpolicy.RequireContainedChild(stagingRoot, sourceRoot); err != nil {
				return nil, err
			}
			digest, err := digestKiroSkillDirectory(sourceRoot)
			if err != nil {
				return nil, err
			}
			objects = append(objects, domain.NativeObjectOwnership{
				ObjectID: "cline-skill:" + component.Name, Kind: clineSkillObjectKind,
				LogicalName: component.Name, Path: filepath.Join(root, "skills", component.Name),
				SourceRelative: filepath.ToSlash(filepath.Dir(relative)), ManagedDigest: digest,
				ProtectionClass: "managed",
			})
		case domain.ComponentMCPServer:
			projection, err := readClineProjection(stagingRoot)
			if err != nil {
				return nil, err
			}
			server, ok := projection.Servers[component.Name]
			if !ok {
				return nil, fmt.Errorf("projected Cline MCP server %q is missing", component.Name)
			}
			settingsPath := clineMCPSettingsPath(root)
			if !filepath.IsAbs(settingsPath) {
				return nil, fmt.Errorf("Cline MCP settings path must be absolute")
			}
			receipt, err := nativeconfig.DesiredReceipt(settingsPath, nativeconfig.CodecCline, component.Name, server, nativeconfig.Placeholders{})
			if err != nil {
				return nil, err
			}
			objects = append(objects, domain.NativeObjectOwnership{
				ObjectID: "cline-mcp:" + component.Name, Kind: clineMCPObjectKind,
				LogicalName: component.Name, Path: settingsPath, ManagedDigest: receipt.Digest,
				SourceRelative: clineProjectionFile, ProtectionClass: "managed",
			})
		}
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}

func activateClineNative(ctx context.Context, request domain.ActivationRequest) error {
	return activateClineNativeWithKernel(ctx, request, nativeconfig.New())
}

func activateClineNativeWithKernel(ctx context.Context, request domain.ActivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyClineNativeMutationWithKernel(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, kernel)
}

func deactivateClineNative(ctx context.Context, request domain.DeactivationRequest) error {
	return deactivateClineNativeWithKernel(ctx, request, nativeconfig.New())
}

func deactivateClineNativeWithKernel(ctx context.Context, request domain.DeactivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyClineNativeMutationWithKernel(request.Client.ConfigRoot, "", request.NativeObjects, nil, kernel)
}

func verifyClineNativeObjects(configRoot string, objects []domain.NativeObjectOwnership, allowMissing bool) error {
	kernel := nativeconfig.New()
	for _, object := range clineObjects(objects) {
		if err := validateClineObject(configRoot, object); err != nil {
			return err
		}
		switch object.Kind {
		case clineSkillObjectKind:
			digest, err := digestKiroSkillDirectory(object.Path)
			if os.IsNotExist(err) && allowMissing {
				continue
			}
			if err != nil || digest != object.ManagedDigest {
				return fmt.Errorf("managed Cline skill %q changed outside agentplugins", object.LogicalName)
			}
		case clineMCPObjectKind:
			receipt := clineReceipt(object)
			present, owned, err := kernel.Inspect(nativeconfig.Paths{JSON: object.Path}, nativeconfig.CodecCline, object.LogicalName, &receipt)
			if err != nil {
				return fmt.Errorf("inspect managed Cline MCP server %q: %w", object.LogicalName, err)
			}
			if !present && allowMissing {
				continue
			}
			if !present || !owned {
				return fmt.Errorf("managed Cline MCP server %q changed outside agentplugins: %w", object.LogicalName, nativeconfig.ErrNotOwned)
			}
		}
	}
	return nil
}

type clineRenameFunc func(string, string) error

func renameClineDirectoryNoReplace(oldPath, newPath string, rename clineRenameFunc) error {
	return rename(oldPath, newPath)
}

func applyClineNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyClineNativeMutationWithKernel(configRoot, activePath, previous, desired, nativeconfig.New())
}

func applyClineNativeMutationWithRename(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename clineRenameFunc) (resultErr error) {
	return applyClineNativeMutationWithKernelAndRename(configRoot, activePath, previous, desired, nativeconfig.New(), rename)
}

func applyClineNativeMutationWithKernel(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	return applyClineNativeMutationWithKernelAndRename(configRoot, activePath, previous, desired, kernel, renameDirectoryExclusive)
}

func applyClineNativeMutationWithKernelAndRename(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename clineRenameFunc) (resultErr error) {
	return applyClineNativeMutationWithKernelRenameAndCapacity(configRoot, activePath, previous, desired, kernel, rename, checkedCombinedCapacity)
}

func applyClineNativeMutationWithKernelRenameAndCapacity(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename clineRenameFunc, capacity combinedCapacityFunc) (resultErr error) {
	if rename == nil {
		return fmt.Errorf("Cline rename operation is unavailable")
	}
	if capacity == nil {
		return fmt.Errorf("Cline capacity checker is unavailable")
	}
	configRoot = strings.TrimSpace(configRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return fmt.Errorf("Cline config root is unavailable")
	}
	previous, desired = clineObjects(previous), clineObjects(desired)
	if err := verifyClineNativeObjects(configRoot, previous, true); err != nil {
		return err
	}
	previousByID, desiredByID := objectMap(previous), objectMap(desired)
	idsCapacity, capacityErr := capacity(len(previousByID), len(desiredByID))
	if capacityErr != nil {
		return fmt.Errorf("prepare managed Cline MCP server set: %w", capacityErr)
	}
	for id, object := range desiredByID {
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !sameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("Cline native object identity changed unexpectedly for %s", id)
			}
			continue
		}
		if object.Kind == clineSkillObjectKind {
			if _, err := os.Lstat(object.Path); err == nil {
				return fmt.Errorf("Cline skill %q already exists without agentplugins ownership", object.LogicalName)
			} else if !os.IsNotExist(err) {
				return err
			}
		}
	}

	skillsRoot := filepath.Join(configRoot, "skills")
	if err := pathpolicy.RequireContainedChild(configRoot, skillsRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
		return fmt.Errorf("create Cline skills root: %w", err)
	}
	transactionRoot, err := os.MkdirTemp(skillsRoot, ".agentplugins-native-")
	if err != nil {
		return err
	}
	cleanupTransaction := true

	staged := map[string]string{}
	for id, object := range desiredByID {
		if object.Kind != clineSkillObjectKind {
			continue
		}
		if activePath == "" {
			return fmt.Errorf("active package path is required for Cline skill installation")
		}
		source := filepath.Join(activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(activePath, source); err != nil {
			return err
		}
		target := filepath.Join(transactionRoot, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return err
		}
		digest, err := digestKiroSkillDirectory(target)
		if err != nil || digest != object.ManagedDigest {
			return fmt.Errorf("staged Cline skill %q does not match its ownership digest", object.LogicalName)
		}
		staged[id] = target
	}

	backups := map[string]string{}
	installed := map[string]domain.NativeObjectOwnership{}
	rollbackSkills := func() error {
		var first error
		attempted := map[string]bool{}
		for id, object := range installed {
			attempted[id] = true
			if digest, err := digestKiroSkillDirectory(object.Path); err == nil && digest == object.ManagedDigest {
				if err := os.RemoveAll(object.Path); err != nil && first == nil {
					first = err
				}
			}
			if backup := backups[id]; backup != "" {
				if err := renameClineDirectoryNoReplace(backup, object.Path, rename); err != nil {
					if first == nil {
						first = err
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
			if err := renameClineDirectoryNoReplace(backup, previousByID[id].Path, rename); err != nil {
				if first == nil {
					first = err
				}
			} else {
				delete(backups, id)
			}
		}
		return first
	}
	defer func() {
		if cleanupTransaction {
			_ = os.RemoveAll(transactionRoot)
		}
	}()
	defer func() {
		if resultErr != nil && !nativeconfig.IsCommittedCleanup(resultErr) {
			if rollbackErr := rollbackSkills(); rollbackErr != nil {
				cleanupTransaction = false
				resultErr = fmt.Errorf("%v; Cline skill rollback failed: %w; recovery retained at %q", resultErr, rollbackErr, transactionRoot)
			}
		}
	}()

	for id, object := range previousByID {
		if object.Kind != clineSkillObjectKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		backup := filepath.Join(transactionRoot, "old-"+object.LogicalName)
		if err := rename(object.Path, backup); err != nil {
			return err
		}
		backups[id] = backup
		digest, digestErr := digestKiroSkillDirectory(backup)
		if digestErr != nil || digest != object.ManagedDigest {
			if digestErr != nil {
				return fmt.Errorf("verify isolated Cline skill backup %q: %w", object.LogicalName, digestErr)
			}
			return fmt.Errorf("isolated Cline skill backup %q changed outside agentplugins", object.LogicalName)
		}
	}
	for id, source := range staged {
		object := desiredByID[id]
		if err := renameClineDirectoryNoReplace(source, object.Path, rename); err != nil {
			return err
		}
		installed[id] = object
	}

	// Prove the filesystem half before committing the single ownership-aware MCP
	// batch. ApplyBatch is the final manager operation, so a reported config
	// failure can still roll the skills back safely.
	if err := verifyClineNativeObjects(configRoot, clineSkillObjects(desired), false); err != nil {
		return err
	}
	if err := mutateClineMCPWithKernelAndCapacity(configRoot, activePath, previousByID, desiredByID, kernel, idsCapacity); err != nil {
		return err
	}
	return nil
}

func mutateClineMCP(configRoot, activePath string, previous, desired map[string]domain.NativeObjectOwnership) error {
	return mutateClineMCPWithKernel(configRoot, activePath, previous, desired, nativeconfig.New())
}

func mutateClineMCPWithKernel(configRoot, activePath string, previous, desired map[string]domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	idsCapacity, capacityErr := checkedCombinedCapacity(len(previous), len(desired))
	if capacityErr != nil {
		return fmt.Errorf("prepare managed Cline MCP server set: %w", capacityErr)
	}
	return mutateClineMCPWithKernelAndCapacity(configRoot, activePath, previous, desired, kernel, idsCapacity)
}

func mutateClineMCPWithKernelAndCapacity(configRoot, activePath string, previous, desired map[string]domain.NativeObjectOwnership, kernel nativeconfig.Kernel, idsCapacity int) error {
	if !hasClineMCP(previous) && !hasClineMCP(desired) {
		return nil
	}
	path := clineMCPSettingsPath(configRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	projection := clineProjection{Servers: map[string]nativeconfig.Server{}}
	if activePath != "" {
		var err error
		projection, err = readClineProjection(activePath)
		if err != nil {
			return err
		}
	}
	ids := make([]string, 0, idsCapacity)
	seen := map[string]bool{}
	for id, object := range previous {
		if object.Kind == clineMCPObjectKind {
			ids, seen[id] = append(ids, id), true
		}
	}
	for id, object := range desired {
		if object.Kind == clineMCPObjectKind && !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	requests := make([]nativeconfig.Request, 0, len(ids))
	for _, id := range ids {
		prior, hadPrior := previous[id]
		next, hasNext := desired[id]
		req := nativeconfig.Request{Paths: nativeconfig.Paths{JSON: path}, Codec: nativeconfig.CodecCline}
		var present, exactlyOwned bool
		if hadPrior {
			receipt := clineReceipt(prior)
			var err error
			present, exactlyOwned, err = kernel.Inspect(req.Paths, req.Codec, prior.LogicalName, &receipt)
			if err != nil {
				return fmt.Errorf("inspect managed Cline MCP server %q: %w", prior.LogicalName, err)
			}
			if present && !exactlyOwned {
				return fmt.Errorf("managed Cline MCP server %q changed outside agentplugins: %w", prior.LogicalName, nativeconfig.ErrNotOwned)
			}
		} else if hasNext {
			var err error
			present, _, err = kernel.Inspect(req.Paths, req.Codec, next.LogicalName, nil)
			if err != nil {
				return fmt.Errorf("inspect Cline MCP server %q before add: %w", next.LogicalName, err)
			}
			if present {
				return fmt.Errorf("Cline MCP server %q already exists: %w", next.LogicalName, nativeconfig.ErrCollision)
			}
		}
		switch {
		case hadPrior && hasNext:
			if !present {
				if !sameClineMCPObject(prior, next) {
					return fmt.Errorf("managed Cline MCP server %q is absent during update: %w", prior.LogicalName, nativeconfig.ErrNotOwned)
				}
				req.Action, req.Name = nativeconfig.ActionAdd, next.LogicalName
			} else {
				receipt := clineReceipt(prior)
				req.Action, req.Name, req.Owned = nativeconfig.ActionUpdate, next.LogicalName, &receipt
			}
			req.Server = projection.Servers[next.LogicalName]
		case hadPrior:
			if !present {
				continue
			}
			receipt := clineReceipt(prior)
			req.Action, req.Name, req.Owned = nativeconfig.ActionRemove, prior.LogicalName, &receipt
		case hasNext:
			req.Action, req.Name, req.Server = nativeconfig.ActionAdd, next.LogicalName, projection.Servers[next.LogicalName]
		}
		if hasNext {
			receipt, err := nativeconfig.DesiredReceipt(path, nativeconfig.CodecCline, next.LogicalName, req.Server, req.Placeholders)
			if err != nil {
				return err
			}
			if receipt.Digest != next.ManagedDigest {
				return fmt.Errorf("Cline MCP desired receipt mismatch for %q", next.LogicalName)
			}
			req.Desired = &receipt
		}
		requests = append(requests, req)
	}
	_, err := kernel.ApplyBatch(requests)
	if err != nil {
		return err
	}
	return nil
}

func sameClineMCPObject(left, right domain.NativeObjectOwnership) bool {
	return left.ObjectID == right.ObjectID && left.Kind == clineMCPObjectKind && right.Kind == clineMCPObjectKind &&
		left.LogicalName == right.LogicalName && sameCleanPath(left.Path, right.Path) && left.ManagedDigest == right.ManagedDigest
}

func readClineProjection(root string) (clineProjection, error) {
	body, err := os.ReadFile(filepath.Join(root, clineProjectionFile))
	if err != nil {
		return clineProjection{}, fmt.Errorf("read projected Cline MCP configuration: %w", err)
	}
	var projection clineProjection
	if err := json.Unmarshal(body, &projection); err != nil || projection.Servers == nil {
		return clineProjection{}, fmt.Errorf("decode projected Cline MCP configuration: %w", err)
	}
	for name, server := range projection.Servers {
		server.StdioValuesResolved = server.Type == "stdio"
		projection.Servers[name] = server
	}
	return projection, nil
}

func clineMCPSettingsPath(configRoot string) string {
	if path := strings.TrimSpace(os.Getenv("CLINE_MCP_SETTINGS_PATH")); path != "" {
		return filepath.Clean(path)
	}
	if data := strings.TrimSpace(os.Getenv("CLINE_DATA_DIR")); data != "" {
		return filepath.Join(filepath.Clean(data), "settings", "cline_mcp_settings.json")
	}
	return filepath.Join(configRoot, "data", "settings", "cline_mcp_settings.json")
}

func clineReceipt(object domain.NativeObjectOwnership) nativeconfig.Receipt {
	return nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecCline, Name: object.LogicalName, Digest: object.ManagedDigest}
}

func clineObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == clineSkillObjectKind || object.Kind == clineMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func clineSkillObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == clineSkillObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func validateClineObject(configRoot string, object domain.NativeObjectOwnership) error {
	if object.ProtectionClass != "managed" || object.ObjectID == "" || object.LogicalName == "" || object.ManagedDigest == "" {
		return fmt.Errorf("invalid Cline native ownership object")
	}
	if object.Kind == clineSkillObjectKind {
		if err := pathpolicy.RequireContainedChild(filepath.Join(configRoot, "skills"), object.Path); err != nil {
			return fmt.Errorf("unsafe Cline skill path: %w", err)
		}
	} else if object.Kind == clineMCPObjectKind {
		if !filepath.IsAbs(object.Path) || !sameCleanPath(object.Path, clineMCPSettingsPath(configRoot)) {
			return fmt.Errorf("Cline MCP ownership path changed")
		}
	} else {
		return fmt.Errorf("unsupported Cline native object kind %q", object.Kind)
	}
	return nil
}

func hasClineMCP(objects map[string]domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == clineMCPObjectKind {
			return true
		}
	}
	return false
}

func clineNeutralServer(server domain.MCPServer) (nativeconfig.Server, error) {
	getString := func(key string) string {
		value, _ := server.Decoded[key].(string)
		return value
	}
	result := nativeconfig.Server{Type: server.Type, Command: getString("command"), URL: getString("url")}
	if server.Type == "stdio" {
		if rawCWD, exists := server.Decoded["cwd"]; exists {
			cwd, ok := rawCWD.(string)
			if !ok {
				return result, fmt.Errorf("Cline stdio MCP server cwd must be a string")
			}
			result.CWD = cwd
		}
	}
	if server.Type == "streamable-http" || server.Type == "sse" {
		result.Type = "remote"
		result.RemoteTransport = server.Type
	}
	if values, ok := server.Decoded["args"].([]any); ok {
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				return result, fmt.Errorf("args must be strings")
			}
			result.Args = append(result.Args, text)
		}
	} else if values, ok := server.Decoded["args"].([]string); ok {
		result.Args = append([]string(nil), values...)
	}
	copyMap := func(key string) (map[string]string, error) {
		result := map[string]string{}
		switch values := server.Decoded[key].(type) {
		case nil:
			return nil, nil
		case map[string]any:
			for name, value := range values {
				text, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("%s values must be strings", key)
				}
				result[name] = text
			}
		case map[string]string:
			for name, value := range values {
				result[name] = value
			}
		default:
			return nil, fmt.Errorf("%s must be an object", key)
		}
		return result, nil
	}
	var err error
	result.Env, err = copyMap("env")
	if err != nil {
		return result, err
	}
	result.Headers, err = copyMap("headers")
	return result, err
}
