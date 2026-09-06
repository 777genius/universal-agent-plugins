package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers/nativeconfig"
)

const (
	openCodeProjectionFile = ".agentplugins-opencode.json"
	openCodeMCPObjectKind  = "opencode_global_mcp_server"
	openCodeSkillKind      = "opencode_global_skill_directory"
)

type openCodeProjection struct {
	ResolvedCWD map[string]bool                `json:"resolved_cwd,omitempty"`
	Version     int                            `json:"version"`
	ConfigPath  string                         `json:"config_path"`
	ConfigJSON  string                         `json:"config_json"`
	ConfigJSONC string                         `json:"config_jsonc"`
	PackageRoot string                         `json:"package_root"`
	DataRoot    string                         `json:"data_root,omitempty"`
	MCPServers  map[string]nativeconfig.Server `json:"mcp_servers,omitempty"`
}

func projectOpenCodeNative(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataRoot string) error {
	configRoot := strings.TrimSpace(plan.NativeRegistryRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return fmt.Errorf("OpenCode config root is unavailable")
	}
	jsonPath, jsoncPath := filepath.Join(configRoot, "opencode.json"), filepath.Join(configRoot, "opencode.jsonc")
	selected, err := selectOpenCodeConfig(jsonPath, jsoncPath)
	if err != nil {
		return err
	}
	projection := openCodeProjection{Version: 1, ConfigPath: selected, ConfigJSON: jsonPath, ConfigJSONC: jsoncPath,
		PackageRoot: plan.ActivePath, DataRoot: dataRoot, MCPServers: map[string]nativeconfig.Server{}, ResolvedCWD: map[string]bool{}}
	for _, component := range plan.Components {
		if component.Kind != domain.ComponentMCPServer || component.Support == domain.SupportUnsupported {
			continue
		}
		server, ok := envelope.MCP.Servers[component.Name]
		if !ok {
			return fmt.Errorf("planned OpenCode MCP server %q is missing", component.Name)
		}
		neutral, err := neutralOpenCodeServer(server)
		if err != nil {
			return fmt.Errorf("project OpenCode MCP server %q: %w", component.Name, err)
		}
		if neutral.Type == "stdio" {
			command, cwd, pathErr := resolveStdioPaths(neutral.Command, neutral.CWD, plan.ActivePath, dataRoot, root)
			if pathErr != nil {
				return fmt.Errorf("project OpenCode MCP server %q: %w", component.Name, pathErr)
			}
			neutral.Command, neutral.CWD = command, cwd
			projection.ResolvedCWD[component.Name] = true
		}

		projection.MCPServers[component.Name] = neutral
	}
	body, err := json.MarshalIndent(projection, "", "  ")
	if err != nil {
		return err
	}
	projectionPath := filepath.Join(root, openCodeProjectionFile)
	if _, err := os.Lstat(projectionPath); err == nil {
		return fmt.Errorf("package contains reserved OpenCode projection path %q", openCodeProjectionFile)
	} else if !os.IsNotExist(err) {
		return err
	}
	return atomicfile.Write(projectionPath, append(body, '\n'), 0o600)
}

func selectOpenCodeConfig(jsonPath, jsoncPath string) (string, error) {
	jsonExists, err := regularNativeFileExists(jsonPath)
	if err != nil {
		return "", err
	}
	jsoncExists, err := regularNativeFileExists(jsoncPath)
	if err != nil {
		return "", err
	}
	if jsonExists && jsoncExists {
		return "", nativeconfig.ErrAmbiguousConfig
	}
	if jsoncExists {
		return jsoncPath, nil
	}
	return jsonPath, nil
}

func regularNativeFileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("native config must be a regular file")
	}
	return true, nil
}

func neutralOpenCodeServer(server domain.MCPServer) (nativeconfig.Server, error) {
	switch server.Type {
	case "stdio":
		command, ok := server.Decoded["command"].(string)
		if !ok || strings.TrimSpace(command) == "" {
			return nativeconfig.Server{}, fmt.Errorf("stdio command is required")
		}
		args, err := openCodeStrings(server.Decoded["args"])
		if err != nil {
			return nativeconfig.Server{}, fmt.Errorf("args: %w", err)
		}
		env, err := openCodeStringMap(server.Decoded["env"])
		if err != nil {
			return nativeconfig.Server{}, fmt.Errorf("env: %w", err)
		}
		if _, exists := env["PLUGIN_ROOT"]; exists {
			return nativeconfig.Server{}, fmt.Errorf("env: PLUGIN_ROOT is reserved and client-managed")
		}
		if _, exists := env["PLUGIN_DATA"]; exists {
			return nativeconfig.Server{}, fmt.Errorf("env: PLUGIN_DATA is reserved and client-managed")
		}
		if env == nil {
			env = map[string]string{}
		}
		env["PLUGIN_ROOT"] = "${PLUGIN_ROOT}"
		env["PLUGIN_DATA"] = "${PLUGIN_DATA}"
		cwd, err := openCodeOptionalString(server.Decoded["cwd"])
		if err != nil {
			return nativeconfig.Server{}, fmt.Errorf("cwd: %w", err)
		}
		cwd, err = normalizeOpenCodeCWD(cwd)
		if err != nil {
			return nativeconfig.Server{}, fmt.Errorf("cwd: %w", err)
		}
		if strings.TrimSpace(cwd) == "" {
			cwd = "${PLUGIN_ROOT}"
		}
		return nativeconfig.Server{Type: "stdio", Command: command, Args: args, Env: env, CWD: cwd}, nil
	case "sse":
		return nativeconfig.Server{}, fmt.Errorf("internal consistency: declared SSE is not supported by OpenCode (remote starts with Streamable HTTP)")
	case "streamable-http":
		url, ok := server.Decoded["url"].(string)
		if !ok || strings.TrimSpace(url) == "" {
			return nativeconfig.Server{}, fmt.Errorf("remote url is required")
		}
		headers, err := openCodeStringMap(server.Decoded["headers"])
		if err != nil {
			return nativeconfig.Server{}, fmt.Errorf("headers: %w", err)
		}
		return nativeconfig.Server{Type: "remote", URL: url, Headers: headers}, nil
	default:
		return nativeconfig.Server{}, fmt.Errorf("unsupported transport %q", server.Type)
	}
}

func openCodeOptionalString(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("must be a string")
	}
	return text, nil
}

func normalizeOpenCodeCWD(value string) (string, error) {
	parsed, err := pathcontract.ParseCWD(value)
	if err != nil {
		return "", err
	}
	root := "${PLUGIN_ROOT}"
	if parsed.Anchor == pathcontract.Data {
		root = "${PLUGIN_DATA}"
	}
	if parsed.Relative != "" {
		root += "/" + parsed.Relative
	}
	return root, nil
}

func openCodeStrings(value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	if strings, ok := value.([]string); ok {
		return append([]string(nil), strings...), nil
	}
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("must be an array")
	}
	result := make([]string, len(values))
	for index, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("item %d must be a string", index)
		}
		result[index] = text
	}
	return result, nil
}

func openCodeStringMap(value any) (map[string]string, error) {
	if value == nil {
		return nil, nil
	}
	if values, ok := value.(map[string]string); ok {
		result := make(map[string]string, len(values))
		for key, value := range values {
			result[key] = value
		}
		return result, nil
	}
	values, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("must be an object")
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("value for %q must be a string", key)
		}
		result[key] = text
	}
	return result, nil
}

func readOpenCodeProjection(root string) (openCodeProjection, error) {
	body, err := os.ReadFile(filepath.Join(root, openCodeProjectionFile))
	if err != nil {
		return openCodeProjection{}, fmt.Errorf("read OpenCode native projection: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var projection openCodeProjection
	if err := decoder.Decode(&projection); err != nil || projection.Version != 1 {
		return openCodeProjection{}, fmt.Errorf("decode OpenCode native projection")
	}
	if err := requireJSONEOF(decoder); err != nil {
		return openCodeProjection{}, err
	}
	if !filepath.IsAbs(projection.ConfigPath) || !filepath.IsAbs(projection.ConfigJSON) || !filepath.IsAbs(projection.ConfigJSONC) || !filepath.IsAbs(projection.PackageRoot) {
		return openCodeProjection{}, fmt.Errorf("OpenCode projection contains relative paths")
	}
	if projection.ConfigPath != projection.ConfigJSON && projection.ConfigPath != projection.ConfigJSONC {
		return openCodeProjection{}, fmt.Errorf("OpenCode projection selects an unexpected config path")
	}
	for name, resolved := range projection.ResolvedCWD {
		server, ok := projection.MCPServers[name]
		if !ok || !resolved || server.Type != "stdio" || !filepath.IsAbs(server.CWD) {
			return openCodeProjection{}, fmt.Errorf("invalid resolved OpenCode cwd provenance")
		}
		server.CWDResolved = true
		projection.MCPServers[name] = server
	}
	return projection, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("native projection has trailing JSON")
	}
	return nil
}

func buildOpenCodeNativeObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	projection, err := readOpenCodeProjection(stagingRoot)
	if err != nil {
		return nil, err
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(projection.MCPServers)+len(envelope.Skills))
	placeholders := nativeconfig.Placeholders{PackageRoot: projection.PackageRoot, DataRoot: projection.DataRoot}
	for name, server := range projection.MCPServers {
		if err := pathpolicy.ValidateLeafID(name); err != nil {
			return nil, fmt.Errorf("invalid OpenCode MCP server name %q: %w", name, err)
		}
		receipt, err := nativeconfig.DesiredReceipt(projection.ConfigPath, nativeconfig.CodecOpenCode, name, server, placeholders)
		if err != nil {
			return nil, err
		}
		objects = append(objects, domain.NativeObjectOwnership{ObjectID: "opencode-mcp:" + name, Kind: openCodeMCPObjectKind,
			LogicalName: name, Path: receipt.Path, ManagedDigest: receipt.Digest, ProtectionClass: "managed"})
	}
	for _, component := range plan.Components {
		if component.Kind != domain.ComponentSkill || component.Support == domain.SupportUnsupported {
			continue
		}
		skill, ok := envelope.Skills[component.Name]
		if !ok {
			return nil, fmt.Errorf("planned OpenCode skill %q is missing", component.Name)
		}
		if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
			return nil, fmt.Errorf("invalid OpenCode skill name %q: %w", component.Name, err)
		}
		relative := filepath.FromSlash(strings.TrimSpace(skill.RelativePath))
		if relative == "" {
			relative = filepath.Join("skills", component.Name, "SKILL.md")
		}
		source := filepath.Join(stagingRoot, filepath.Dir(relative))
		if err := pathpolicy.RequireContainedChild(stagingRoot, source); err != nil {
			return nil, err
		}
		digest, err := digestKiroSkillDirectory(source)
		if err != nil {
			return nil, err
		}
		target := filepath.Join(plan.NativeRegistryRoot, "skills", component.Name)
		if err := pathpolicy.RequireContainedChild(plan.NativeRegistryRoot, target); err != nil {
			return nil, err
		}
		objects = append(objects, domain.NativeObjectOwnership{ObjectID: "opencode-skill:" + component.Name, Kind: openCodeSkillKind,
			LogicalName: component.Name, Path: target, SourceRelative: filepath.ToSlash(filepath.Dir(relative)), ManagedDigest: digest, ProtectionClass: "managed"})
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}

func activateOpenCodeNative(ctx context.Context, request domain.ActivationRequest) error {
	return activateOpenCodeNativeWithKernel(ctx, request, nativeconfig.New())
}

func activateOpenCodeNativeWithKernel(ctx context.Context, request domain.ActivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyOpenCodeNativeWithKernel(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, kernel)
}

func deactivateOpenCodeNative(ctx context.Context, request domain.DeactivationRequest) error {
	return deactivateOpenCodeNativeWithKernel(ctx, request, nativeconfig.New())
}

func deactivateOpenCodeNativeWithKernel(ctx context.Context, request domain.DeactivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyOpenCodeNativeWithKernel(request.Client.ConfigRoot, "", request.NativeObjects, nil, kernel)
}

func verifyOpenCodeNativeObjects(configRoot, activePath string, objects []domain.NativeObjectOwnership) error {
	projection := openCodeProjection{}
	if len(openCodeObjects(objects)) > 0 && activePath != "" {
		var err error
		projection, err = readOpenCodeProjection(activePath)
		if err != nil {
			return err
		}
		if err := validateOpenCodeProjection(configRoot, activePath, projection); err != nil {
			return err
		}
	}
	for _, object := range openCodeObjects(objects) {
		if err := validateOpenCodeObject(configRoot, projection, object); err != nil {
			return err
		}
		if object.Kind == openCodeSkillKind {
			digest, err := digestKiroSkillDirectory(object.Path)
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

func applyOpenCodeNative(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyOpenCodeNativeWithKernel(configRoot, activePath, previous, desired, nativeconfig.New())
}

func applyOpenCodeNativeWithKernel(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	return applyOpenCodeNativeWithKernelAndOps(configRoot, activePath, previous, desired, kernel, renameDirectoryExclusive, os.RemoveAll)
}

func applyOpenCodeNativeWithRename(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename openCodeRenameFunc) (resultErr error) {
	return applyOpenCodeNativeWithKernelAndOps(configRoot, activePath, previous, desired, nativeconfig.New(), rename, os.RemoveAll)
}

func applyOpenCodeNativeWithOps(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename openCodeRenameFunc, removeAll func(string) error) (resultErr error) {
	return applyOpenCodeNativeWithKernelAndOps(configRoot, activePath, previous, desired, nativeconfig.New(), rename, removeAll)
}

func applyOpenCodeNativeWithKernelAndOps(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename openCodeRenameFunc, removeAll func(string) error) (resultErr error) {
	if rename == nil {
		return fmt.Errorf("OpenCode rename operation is unavailable")
	}
	if removeAll == nil {
		return fmt.Errorf("OpenCode cleanup operation is unavailable")
	}
	if strings.TrimSpace(configRoot) == "" || !filepath.IsAbs(configRoot) {
		return fmt.Errorf("OpenCode config root is unavailable")
	}
	previous, desired = openCodeObjects(previous), openCodeObjects(desired)
	projection := openCodeProjection{}
	var err error
	if len(desired) > 0 {
		projection, err = readOpenCodeProjection(activePath)
		if err != nil {
			return err
		}
		if err := validateOpenCodeProjection(configRoot, activePath, projection); err != nil {
			return err
		}
	}
	if err := preflightOpenCodeObjects(configRoot, activePath, projection, previous, desired); err != nil {
		return err
	}
	expectedConfig := projection.ConfigPath
	if expectedConfig == "" {
		for _, object := range previous {
			if object.Kind == openCodeMCPObjectKind {
				expectedConfig = object.Path
				break
			}
		}
	}
	if expectedConfig != "" {
		selected, err := selectOpenCodeConfig(filepath.Join(configRoot, "opencode.json"), filepath.Join(configRoot, "opencode.jsonc"))
		if err != nil {
			return err
		}
		jsonExists, jsoncExists, err := openCodeConfigPresence(configRoot)
		if err != nil {
			return err
		}
		if !sameCleanPath(selected, expectedConfig) {
			if len(desired) > 0 || jsonExists || jsoncExists {
				return fmt.Errorf("OpenCode config selection changed after staging; rerun the operation")
			}
			// Removal is already complete when both exact config variants are
			// absent. Do not recreate either path just to remove an absent entry.
		}
	}
	requests, err := openCodeMCPRequests(projection, previous, desired)
	if err != nil {
		return err
	}
	skills, err := installOpenCodeSkillsWithOps(configRoot, activePath, previous, desired, rename, removeAll)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			if err := skills.rollback(); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("rollback OpenCode skills: %w", err))
			}
		}
	}()
	if len(requests) > 0 {
		receipts, applyErr := kernel.ApplyBatch(requests)
		if applyErr != nil && !nativeconfig.IsCommittedCleanup(applyErr) {
			return applyErr
		}
		if len(receipts) != len(requests) {
			if applyErr != nil {
				return errors.Join(applyErr, fmt.Errorf("OpenCode native config committed without complete receipts"))
			}
			return fmt.Errorf("OpenCode native config returned incomplete receipts")
		}
		if applyErr != nil {
			// ApplyBatch promises that typed committed-cleanup failures include the
			// receipts for bytes already written. Preserve both those bytes and the
			// skills installed in the same provider transaction.
			committed = true
		}
		if applyErr == nil {
			committed = true
		}
		desiredByID := objectMap(desired)
		for index, request := range requests {
			if request.Action == nativeconfig.ActionRemove {
				continue
			}
			expected := desiredByID["opencode-mcp:"+request.Name]
			if receipts[index].Digest != expected.ManagedDigest || receipts[index].Path != expected.Path {
				return fmt.Errorf("OpenCode native receipt differs from staged ownership")
			}
		}
		skills.commit()
		if applyErr != nil {
			return applyErr
		}
		return nil
	}
	committed = true
	// Config and skills are now externally committed. Transaction-root cleanup
	// is best effort and cannot truthfully turn this into a failed activation:
	// the lifecycle must persist the desired native receipts. A failed cleanup
	// leaves a committed marker in the private transaction root so the residue
	// is distinguishable from rollback recovery and safe to remove later.
	skills.commit()
	return nil
}

func validateOpenCodeProjection(configRoot, activePath string, projection openCodeProjection) error {
	if !sameCleanPath(projection.ConfigJSON, filepath.Join(configRoot, "opencode.json")) ||
		!sameCleanPath(projection.ConfigJSONC, filepath.Join(configRoot, "opencode.jsonc")) ||
		!sameCleanPath(projection.PackageRoot, activePath) {
		return fmt.Errorf("OpenCode native projection is not bound to the detected client and active package")
	}
	if projection.DataRoot != "" && !filepath.IsAbs(projection.DataRoot) {
		return fmt.Errorf("OpenCode native projection data root must be absolute")
	}
	return nil
}

func openCodeMCPRequests(projection openCodeProjection, previous, desired []domain.NativeObjectOwnership) ([]nativeconfig.Request, error) {
	previousByID, desiredByID := objectMap(previous), objectMap(desired)
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
			// An already absent exact-owned entry is a safe idempotent removal.
			if present {
				result = append(result, nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Action: nativeconfig.ActionRemove, Name: object.LogicalName, Owned: &owned})
			}
			continue
		}
		action := nativeconfig.ActionUpdate
		var receipt *nativeconfig.Receipt = &owned
		if !present {
			// Repair may recreate a missing native entry only when the staged
			// desired receipt is exactly the receipt already owned by state. A
			// real update with different bytes remains fail closed.
			if !sameOpenCodeMCPObject(object, next) {
				return nil, fmt.Errorf("managed OpenCode MCP server %q is absent during update: %w", object.LogicalName, nativeconfig.ErrNotOwned)
			}
			action, receipt = nativeconfig.ActionAdd, nil
		}
		desiredReceipt := receiptFromOpenCodeObject(next)
		result = append(result, nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Action: action, Name: next.LogicalName,
			Server: projection.MCPServers[next.LogicalName], Placeholders: placeholders, Owned: receipt, Desired: &desiredReceipt})
	}
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
			return nil, fmt.Errorf("OpenCode MCP server %q already exists: %w", object.LogicalName, nativeconfig.ErrCollision)
		}
		desiredReceipt := receiptFromOpenCodeObject(object)
		result = append(result, nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Action: nativeconfig.ActionAdd, Name: object.LogicalName,
			Server: projection.MCPServers[object.LogicalName], Placeholders: placeholders, Desired: &desiredReceipt})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
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
		left.LogicalName == right.LogicalName && sameCleanPath(left.Path, right.Path) && left.ManagedDigest == right.ManagedDigest
}

func receiptFromOpenCodeObject(object domain.NativeObjectOwnership) nativeconfig.Receipt {
	return nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecOpenCode, Name: object.LogicalName, Digest: object.ManagedDigest}
}

func openCodeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
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
		return fmt.Errorf("OpenCode native object is incomplete")
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
	if !sameCleanPath(expected, object.Path) {
		return fmt.Errorf("OpenCode native object %q has an untrusted path", object.LogicalName)
	}
	return pathpolicy.RequireContainedChild(configRoot, object.Path)
}

func preflightOpenCodeObjects(configRoot, activePath string, projection openCodeProjection, previous, desired []domain.NativeObjectOwnership) error {
	previousByID, desiredByID := objectMap(previous), objectMap(desired)
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
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !sameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("OpenCode native object identity changed for %s", id)
			}
			continue
		}
		if object.Kind == openCodeSkillKind {
			if _, err := os.Lstat(object.Path); err == nil {
				return fmt.Errorf("OpenCode skill %q already exists and is not owned", object.LogicalName)
			} else if !os.IsNotExist(err) {
				return err
			}
		}
	}
	_ = activePath
	return nil
}

type openCodeBackup struct{ backup, target string }
type openCodeRenameFunc func(string, string) error
type openCodeSkillTxn struct {
	root      string
	backups   map[string]openCodeBackup
	installed map[string]domain.NativeObjectOwnership
	rename    openCodeRenameFunc
	removeAll func(string) error
}

func installOpenCodeSkills(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) (*openCodeSkillTxn, error) {
	return installOpenCodeSkillsWithOps(configRoot, activePath, previous, desired, renameDirectoryExclusive, os.RemoveAll)
}

func renameOpenCodeDirectoryNoReplace(oldPath, newPath string, rename openCodeRenameFunc) error {
	if _, err := os.Lstat(newPath); err == nil {
		return fmt.Errorf("destination already exists: %s", newPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	return rename(oldPath, newPath)
}

func installOpenCodeSkillsWithRename(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename openCodeRenameFunc) (*openCodeSkillTxn, error) {
	return installOpenCodeSkillsWithOps(configRoot, activePath, previous, desired, rename, os.RemoveAll)
}

func installOpenCodeSkillsWithOps(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename openCodeRenameFunc, removeAll func(string) error) (*openCodeSkillTxn, error) {
	if rename == nil {
		return nil, fmt.Errorf("OpenCode rename operation is unavailable")
	}
	if removeAll == nil {
		return nil, fmt.Errorf("OpenCode cleanup operation is unavailable")
	}
	txn := &openCodeSkillTxn{backups: map[string]openCodeBackup{}, installed: map[string]domain.NativeObjectOwnership{}, rename: rename, removeAll: removeAll}
	previousByID, desiredByID := objectMap(previous), objectMap(desired)
	if !containsOpenCodeSkill(previous) && !containsOpenCodeSkill(desired) {
		return txn, nil
	}
	skillsRoot := filepath.Join(configRoot, "skills")
	if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(skillsRoot, ".agentplugins-native-")
	if err != nil {
		return nil, err
	}
	txn.root = root
	fail := func(err error) (*openCodeSkillTxn, error) {
		if rollbackErr := txn.rollback(); rollbackErr != nil {
			return nil, fmt.Errorf("%v; OpenCode skill rollback failed: %w; recovery retained at %q", err, rollbackErr, txn.root)
		}
		return nil, err
	}
	staged := map[string]string{}
	for id, object := range desiredByID {
		if object.Kind != openCodeSkillKind {
			continue
		}
		source := filepath.Join(activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(activePath, source); err != nil {
			return fail(err)
		}
		target := filepath.Join(root, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return fail(err)
		}
		digest, err := digestKiroSkillDirectory(target)
		if err != nil || digest != object.ManagedDigest {
			return fail(fmt.Errorf("staged OpenCode skill %q differs from ownership digest", object.LogicalName))
		}
		staged[id] = target
	}
	for id, object := range previousByID {
		if object.Kind != openCodeSkillKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fail(err)
		}
		digest, err := digestKiroSkillDirectory(object.Path)
		if err != nil || digest != object.ManagedDigest {
			return fail(fmt.Errorf("managed OpenCode skill %q changed outside agentplugins", object.LogicalName))
		}
		backup := filepath.Join(root, "old-"+object.LogicalName)
		if err := os.Rename(object.Path, backup); err != nil {
			return fail(err)
		}
		txn.backups[id] = openCodeBackup{backup: backup, target: object.Path}
	}
	for id, object := range desiredByID {
		if object.Kind != openCodeSkillKind {
			continue
		}
		if err := renameOpenCodeDirectoryNoReplace(staged[id], object.Path, rename); err != nil {
			return fail(err)
		}
		txn.installed[id] = object
	}
	return txn, nil
}

func containsOpenCodeSkill(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == openCodeSkillKind {
			return true
		}
	}
	return false
}

func (txn *openCodeSkillTxn) rollback() error {
	var result error
	for id, object := range txn.installed {
		digest, err := digestKiroSkillDirectory(object.Path)
		if err == nil && digest == object.ManagedDigest {
			err = os.RemoveAll(object.Path)
		}
		if err != nil && !os.IsNotExist(err) && result == nil {
			result = err
		}
		if backup, ok := txn.backups[id]; ok {
			if err := renameOpenCodeDirectoryNoReplace(backup.backup, backup.target, txn.rename); err != nil {
				if result == nil {
					result = err
				}
			} else {
				delete(txn.backups, id)
			}
		}
	}
	for id, backup := range txn.backups {
		if _, err := os.Lstat(backup.target); os.IsNotExist(err) {
			if err := renameOpenCodeDirectoryNoReplace(backup.backup, backup.target, txn.rename); err != nil {
				if result == nil {
					result = err
				}
			} else {
				delete(txn.backups, id)
			}
		} else if err != nil {
			if result == nil {
				result = err
			}
		} else if result == nil {
			result = fmt.Errorf("preserve OpenCode skill backup %s: target is occupied", backup.backup)
		}
	}
	if txn.root != "" && len(txn.backups) == 0 {
		if err := os.RemoveAll(txn.root); err != nil && result == nil {
			result = err
		}
	}
	return result
}

func (txn *openCodeSkillTxn) commit() {
	if txn.root == "" {
		return
	}
	marker := filepath.Join(txn.root, ".agentplugins-committed")
	_ = atomicfile.Write(marker, []byte("OpenCode native config and skills committed; this transaction residue is safe to remove.\n"), 0o600)
	_ = txn.removeAll(txn.root)
}
