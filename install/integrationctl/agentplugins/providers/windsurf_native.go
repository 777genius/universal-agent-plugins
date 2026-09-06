package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers/nativeconfig"
)

const windsurfMCPObjectKind = "windsurf_mcp_entry"

func projectWindsurfMCP(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	if !hasSupportedMCP(plan.Components) {
		return nil
	}
	servers := make(map[string]any)
	for _, name := range supportedMCPNames(plan) {
		server, ok := envelope.MCP.Servers[name]
		if !ok {
			return fmt.Errorf("Windsurf MCP server %q is missing from the package envelope", name)
		}
		resolved, err := windsurfServer(server, plan.ActivePath, dataPath)
		if err != nil {
			return fmt.Errorf("project Windsurf MCP server %q: %w", name, err)
		}
		if server.Type == "stdio" {
			command, _ := server.Decoded["command"].(string)
			if strings.HasPrefix(command, "./") {
				relative, e := pathcontract.ParseCommand(command)
				if e != nil {
					return e
				}
				observation := pathcontract.Resolve(root, relative)
				if observation.State != pathcontract.Resolved {
					return fmt.Errorf("stdio command escapes PLUGIN_ROOT or is unavailable: %v", observation.Err)
				}
			}
			rawCWD, _ := server.Decoded["cwd"].(string)
			cwd, e := pathcontract.ExpandCWD(rawCWD, plan.ActivePath, dataPath)
			if e != nil {
				return e
			}
			anchor := root
			if cwd.Anchor == pathcontract.Data {
				anchor = dataPath
			}
			observation := pathcontract.Resolve(anchor, cwd.Relative)
			if observation.State != pathcontract.Resolved {
				return fmt.Errorf("stdio cwd unavailable: %v", observation.Err)
			}
		}
		projected, err := standardWindsurfServer(resolved, server.Type)
		if err != nil {
			return fmt.Errorf("project Windsurf MCP server %q: %w", name, err)
		}
		servers[name] = projected
	}
	return writeJSON(filepath.Join(root, "mcp.json"), map[string]any{
		"$schema":    domain.MCPSchemaV1,
		"mcpServers": servers,
	})
}

func buildWindsurfNativeObjects(stagingRoot string, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	if strings.TrimSpace(plan.NativeRegistryRoot) == "" || !hasSupportedMCP(plan.Components) {
		return nil, nil
	}
	configPath, err := windsurfConfigPath(plan.NativeRegistryRoot)
	if err != nil {
		return nil, err
	}
	servers, err := readProjectedWindsurfServers(stagingRoot)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	objects := make([]domain.NativeObjectOwnership, 0, len(names))
	for _, name := range names {
		receipt, err := desiredWindsurfReceipt(configPath, name, servers[name])
		if err != nil {
			return nil, fmt.Errorf("preview Windsurf MCP ownership %q: %w", name, err)
		}
		objects = append(objects, domain.NativeObjectOwnership{
			ObjectID:        "windsurf:mcp:" + name,
			Kind:            windsurfMCPObjectKind,
			LogicalName:     name,
			Path:            configPath,
			SourceRelative:  "mcp.json",
			ManagedDigest:   receipt.Digest,
			ProtectionClass: "managed_entry",
		})
	}
	return objects, nil
}

func activateWindsurfNative(ctx context.Context, request domain.ActivationRequest) error {
	return activateWindsurfNativeWithKernel(ctx, request, nativeconfig.New())
}

func activateWindsurfNativeWithKernel(ctx context.Context, request domain.ActivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyWindsurfNativeMutationWithKernel(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, kernel)
}

func deactivateWindsurfNative(ctx context.Context, request domain.DeactivationRequest) error {
	return deactivateWindsurfNativeWithKernel(ctx, request, nativeconfig.New())
}

func deactivateWindsurfNativeWithKernel(ctx context.Context, request domain.DeactivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyWindsurfNativeMutationWithKernel(request.Client.ConfigRoot, "", request.NativeObjects, nil, kernel)
}

type windsurfMutation struct {
	action   nativeconfig.Action
	name     string
	server   nativeconfig.Server
	previous *nativeconfig.Receipt
	desired  *nativeconfig.Receipt
}

func applyWindsurfNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyWindsurfNativeMutationWithKernel(configRoot, activePath, previous, desired, nativeconfig.New())
}

func applyWindsurfNativeMutationWithKernel(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	configPath, err := windsurfConfigPath(configRoot)
	if err != nil {
		return err
	}
	previousMap, err := windsurfObjectMap(configRoot, previous)
	if err != nil {
		return err
	}
	desiredMap, err := windsurfObjectMap(configRoot, desired)
	if err != nil {
		return err
	}
	desiredServers := map[string]nativeconfig.Server{}
	if len(desiredMap) > 0 {
		desiredServers, err = readProjectedWindsurfServers(activePath)
		if err != nil {
			return err
		}
	}
	if len(desiredServers) != len(desiredMap) {
		return fmt.Errorf("prepared Windsurf MCP projection does not match desired ownership")
	}

	namesCapacity, capacityErr := checkedCombinedCapacity(len(previousMap), len(desiredMap))
	if capacityErr != nil {
		return fmt.Errorf("prepare managed Windsurf MCP server set: %w", capacityErr)
	}
	names := make([]string, 0, namesCapacity)
	seen := map[string]bool{}
	for name := range previousMap {
		seen[name] = true
		names = append(names, name)
	}
	for name := range desiredMap {
		if !seen[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	mutations := make([]windsurfMutation, 0, len(names))
	for _, name := range names {
		oldObject, hadOld := previousMap[name]
		newObject, hasNew := desiredMap[name]
		mutation := windsurfMutation{name: name}
		switch {
		case hadOld && hasNew:
			oldReceipt := windsurfReceipt(oldObject)
			newReceipt := windsurfReceipt(newObject)
			mutation.action, mutation.server, mutation.previous, mutation.desired = nativeconfig.ActionUpdate, desiredServers[name], &oldReceipt, &newReceipt
		case !hadOld && hasNew:
			newReceipt := windsurfReceipt(newObject)
			mutation.action, mutation.server, mutation.desired = nativeconfig.ActionAdd, desiredServers[name], &newReceipt
		case hadOld && !hasNew:
			oldReceipt := windsurfReceipt(oldObject)
			mutation.action, mutation.previous = nativeconfig.ActionRemove, &oldReceipt
		default:
			continue
		}
		if mutation.previous != nil {
			present, owned, inspectErr := kernel.Inspect(nativeconfig.Paths{JSON: configPath}, nativeconfig.CodecWindsurf, name, mutation.previous)
			if inspectErr != nil {
				return fmt.Errorf("preflight existing Windsurf MCP entry %q: %w", name, inspectErr)
			}
			if !present {
				if mutation.action == nativeconfig.ActionRemove {
					continue
				}
				mutation.action = nativeconfig.ActionAdd
				mutation.previous = nil
			} else if !owned {
				return fmt.Errorf("preflight existing Windsurf MCP entry %q: %w", name, nativeconfig.ErrNotOwned)
			}
		} else {
			present, _, inspectErr := kernel.Inspect(nativeconfig.Paths{JSON: configPath}, nativeconfig.CodecWindsurf, name, nil)
			if inspectErr != nil {
				return fmt.Errorf("preflight Windsurf MCP entry %q: %w", name, inspectErr)
			}
			if present {
				return fmt.Errorf("preflight Windsurf MCP entry %q: %w", name, nativeconfig.ErrCollision)
			}
		}
		if mutation.desired != nil {
			preview, previewErr := desiredWindsurfReceipt(configPath, name, mutation.server)
			if previewErr != nil {
				return fmt.Errorf("preview Windsurf MCP entry %q: %w", name, previewErr)
			}
			if preview != *mutation.desired {
				return fmt.Errorf("Windsurf MCP entry %q desired ownership digest drifted", name)
			}
		}
		mutations = append(mutations, mutation)
	}
	if len(mutations) == 0 {
		return verifyWindsurfNativeObjects(configRoot, activePath, desired, false)
	}
	requests := make([]nativeconfig.Request, len(mutations))
	for index := range mutations {
		requests[index] = windsurfNativeRequest(configPath, mutations[index])
	}
	results, err := kernel.ApplyBatch(requests)
	if err != nil {
		return fmt.Errorf("apply Windsurf MCP configuration: %w", err)
	}
	for index := range mutations {
		if mutations[index].desired != nil && results[index] != *mutations[index].desired {
			return fmt.Errorf("Windsurf MCP entry %q ownership digest changed during apply", mutations[index].name)
		}
	}
	return verifyWindsurfNativeObjects(configRoot, activePath, desired, false)
}

func windsurfNativeRequest(configPath string, mutation windsurfMutation) nativeconfig.Request {
	return nativeconfig.Request{
		Paths:  nativeconfig.Paths{JSON: configPath},
		Codec:  nativeconfig.CodecWindsurf,
		Action: mutation.action,
		Name:   mutation.name,
		Server: mutation.server,
		Owned:  mutation.previous,
	}
}

func verifyWindsurfNativeObjects(configRoot, activePath string, objects []domain.NativeObjectOwnership, allowMissing bool) error {
	objectMap, err := windsurfObjectMap(configRoot, objects)
	if err != nil {
		return err
	}
	if len(objectMap) == 0 {
		return nil
	}
	servers, err := readProjectedWindsurfServers(activePath)
	if err != nil {
		return err
	}
	configPath, err := windsurfConfigPath(configRoot)
	if err != nil {
		return err
	}
	kernel := nativeconfig.New()
	for name, object := range objectMap {
		server, ok := servers[name]
		if !ok {
			return fmt.Errorf("prepared Windsurf MCP server %q is missing", name)
		}
		receipt := windsurfReceipt(object)
		present, owned, inspectErr := kernel.Inspect(nativeconfig.Paths{JSON: configPath}, nativeconfig.CodecWindsurf, name, &receipt)
		if inspectErr != nil {
			return fmt.Errorf("verify Windsurf MCP entry %q: %w", name, inspectErr)
		}
		if !present && allowMissing {
			continue
		}
		if !present || !owned {
			return fmt.Errorf("verify Windsurf MCP entry %q: %w", name, nativeconfig.ErrNotOwned)
		}
		preview, previewErr := desiredWindsurfReceipt(configPath, name, server)
		if previewErr != nil || preview.Digest != object.ManagedDigest {
			return fmt.Errorf("verify Windsurf MCP entry %q: desired digest drifted", name)
		}
	}
	return nil
}

func inspectWindsurfRegistry(plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	if strings.TrimSpace(plan.NativeRegistryRoot) == "" {
		return registryClear, nil
	}
	configPath, err := windsurfConfigPath(plan.NativeRegistryRoot)
	if err != nil {
		return registryIndeterminate, err
	}
	if managed != nil {
		if err := verifyWindsurfNativeObjects(plan.NativeRegistryRoot, plan.ActivePath, managed.NativeObjects, false); err != nil {
			return registryIndeterminate, err
		}
		if len(windsurfObjects(managed.NativeObjects)) > 0 {
			return registryExpected, nil
		}
	}
	kernel := nativeconfig.New()
	for _, component := range plan.Components {
		if component.Kind != domain.ComponentMCPServer || component.Support == domain.SupportUnsupported {
			continue
		}
		present, _, inspectErr := kernel.Inspect(nativeconfig.Paths{JSON: configPath}, nativeconfig.CodecWindsurf, component.Name, nil)
		if present {
			return registryCollision, nil
		}
		if inspectErr != nil {
			return registryIndeterminate, inspectErr
		}
	}
	return registryClear, nil
}

func windsurfConfigPath(configRoot string) (string, error) {
	root := filepath.Clean(strings.TrimSpace(configRoot))
	if root == "." || !filepath.IsAbs(root) {
		return "", fmt.Errorf("Windsurf channel config root must be absolute")
	}
	path := filepath.Join(root, "mcp_config.json")
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return "", fmt.Errorf("unsafe Windsurf MCP config path: %w", err)
	}
	return path, nil
}

func windsurfObjectMap(configRoot string, objects []domain.NativeObjectOwnership) (map[string]domain.NativeObjectOwnership, error) {
	configPath, err := windsurfConfigPath(configRoot)
	if err != nil {
		return nil, err
	}
	result := map[string]domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind == "managed_package_directory" {
			continue
		}
		if object.Kind != windsurfMCPObjectKind || object.LogicalName == "" || object.ManagedDigest == "" || object.ProtectionClass != "managed_entry" || filepath.Clean(object.Path) != configPath {
			return nil, fmt.Errorf("invalid Windsurf native ownership object %q", object.ObjectID)
		}
		if _, exists := result[object.LogicalName]; exists {
			return nil, fmt.Errorf("duplicate Windsurf native ownership for %q", object.LogicalName)
		}
		result[object.LogicalName] = object
	}
	return result, nil
}

func windsurfObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind == windsurfMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func windsurfReceipt(object domain.NativeObjectOwnership) nativeconfig.Receipt {
	return nativeconfig.Receipt{Version: "1", Path: filepath.Clean(object.Path), Codec: nativeconfig.CodecWindsurf, Name: object.LogicalName, Digest: object.ManagedDigest}
}

func desiredWindsurfReceipt(configPath, name string, server nativeconfig.Server) (nativeconfig.Receipt, error) {
	return nativeconfig.DesiredReceipt(configPath, nativeconfig.CodecWindsurf, name, server, nativeconfig.Placeholders{})
}

func readProjectedWindsurfServers(root string) (map[string]nativeconfig.Server, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("prepared Windsurf package path is unavailable")
	}
	body, err := os.ReadFile(filepath.Join(root, "mcp.json"))
	if err != nil {
		return nil, fmt.Errorf("read prepared Windsurf MCP projection: %w", err)
	}
	var document struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &document); err != nil || document.Servers == nil {
		return nil, fmt.Errorf("decode prepared Windsurf MCP projection")
	}
	result := make(map[string]nativeconfig.Server, len(document.Servers))
	for name, raw := range document.Servers {
		server, err := decodeProjectedWindsurfServer(raw)
		if err != nil {
			return nil, fmt.Errorf("decode prepared Windsurf MCP server %q: %w", name, err)
		}
		result[name] = server
	}
	return result, nil
}

func windsurfServer(server domain.MCPServer, packageRoot, dataRoot string) (nativeconfig.Server, error) {
	decoded := server.Decoded
	result := nativeconfig.Server{}
	switch server.Type {
	case "stdio":
		result.Type = "stdio"
		command, ok := decoded["command"].(string)
		if !ok || strings.TrimSpace(command) == "" {
			return result, fmt.Errorf("stdio command is required")
		}
		result.Command = command
		if rawArgs, ok := decoded["args"].([]any); ok {
			for _, raw := range rawArgs {
				value, ok := raw.(string)
				if !ok {
					return result, fmt.Errorf("stdio args must contain strings")
				}
				result.Args = append(result.Args, value)
			}
		}
		if rawEnv, exists := decoded["env"]; exists {
			result.Env = map[string]string{}
			switch values := rawEnv.(type) {
			case map[string]any:
				for key, raw := range values {
					value, ok := raw.(string)
					if !ok {
						return result, fmt.Errorf("stdio env values must be strings")
					}
					result.Env[key] = value
				}
			case map[string]string:
				for key, value := range values {
					result.Env[key] = value
				}
			default:
				return result, fmt.Errorf("stdio env must be an object")
			}
		}
		if _, exists := result.Env["PLUGIN_ROOT"]; exists {
			return result, fmt.Errorf("stdio env PLUGIN_ROOT is reserved and client-managed")
		}
		if _, exists := result.Env["PLUGIN_DATA"]; exists {
			return result, fmt.Errorf("stdio env PLUGIN_DATA is reserved and client-managed")
		}
		if result.Env == nil {
			result.Env = map[string]string{}
		}
		result.Env["PLUGIN_ROOT"] = packageRoot
		result.Env["PLUGIN_DATA"] = dataRoot
	case "streamable-http", "sse":
		result.Type = "remote"
		result.RemoteTransport = server.Type
		url, ok := decoded["url"].(string)
		if !ok || strings.TrimSpace(url) == "" {
			return result, fmt.Errorf("remote url is required")
		}
		result.URL = url
		if rawHeaders, ok := decoded["headers"].(map[string]any); ok {
			result.Headers = map[string]string{}
			for key, raw := range rawHeaders {
				value, ok := raw.(string)
				if !ok {
					return result, fmt.Errorf("remote header values must be strings")
				}
				result.Headers[key] = value
			}
		}
	default:
		return result, fmt.Errorf("unsupported Windsurf MCP transport %q", server.Type)
	}
	resolved, err := resolveWindsurfPlaceholders(result, packageRoot, dataRoot)
	if err != nil {
		return result, err
	}
	if resolved.Type == "stdio" {
		if !filepath.IsAbs(packageRoot) || !filepath.IsAbs(dataRoot) {
			return result, fmt.Errorf("managed stdio roots must be absolute")
		}
		rawCWD := ""
		if value, exists := decoded["cwd"]; exists {
			var ok bool
			rawCWD, ok = value.(string)
			if !ok {
				return result, fmt.Errorf("stdio cwd must be a string")
			}
		}
		cwd, err := pathcontract.ExpandCWD(rawCWD, packageRoot, dataRoot)
		if err != nil {
			return result, err
		}
		root := packageRoot
		if cwd.Anchor == pathcontract.Data {
			root = dataRoot
		}
		absoluteCWD := root
		if cwd.Relative != "" {
			absoluteCWD = strings.TrimRight(root, string(filepath.Separator)) + string(filepath.Separator) + filepath.FromSlash(cwd.Relative)
		}
		resolved.StdioValuesResolved = true
		resolved.Args = managedstdio.Arguments(packageRoot, dataRoot, absoluteCWD, cwd.Anchor, commandForLauncher(decoded), resolved.Args)
		resolved.Command = filepath.Join(packageRoot, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	}

	return resolved, nil
}

func commandForLauncher(decoded map[string]any) string {
	value, _ := decoded["command"].(string)
	return value
}

func resolveWindsurfPlaceholders(server nativeconfig.Server, packageRoot, dataRoot string) (nativeconfig.Server, error) {
	if server.Type != "stdio" {
		return server, nil
	}
	resolve := func(value string) (string, error) {
		for _, replacement := range []struct{ token, value string }{{"${PLUGIN_ROOT}", packageRoot}, {"${PLUGIN_DATA}", dataRoot}} {
			if strings.Contains(value, replacement.token) {
				if strings.TrimSpace(replacement.value) == "" {
					return "", fmt.Errorf("explicit value for %s is required", replacement.token)
				}
			}
		}
		return strings.NewReplacer("${PLUGIN_ROOT}", packageRoot, "${PLUGIN_DATA}", dataRoot).Replace(value), nil
	}
	var err error
	for index := range server.Args {
		if server.Args[index], err = resolve(server.Args[index]); err != nil {
			return server, err
		}
	}
	for key, value := range server.Env {
		if key == "PLUGIN_ROOT" || key == "PLUGIN_DATA" {
			continue
		}
		if server.Env[key], err = resolve(value); err != nil {
			return server, err
		}
	}
	return server, nil
}

func standardWindsurfServer(server nativeconfig.Server, originalType string) (map[string]any, error) {
	if server.Type == "stdio" {
		if strings.TrimSpace(server.CWD) != "" {
			return nil, fmt.Errorf("Windsurf stdio MCP server does not support cwd")
		}
		result := map[string]any{"type": "stdio", "command": server.Command}
		if len(server.Args) > 0 {
			result["args"] = server.Args
		}
		if len(server.Env) > 0 {
			result["env"] = server.Env
		}
		return result, nil
	}
	result := map[string]any{"type": originalType, "url": server.URL}
	if len(server.Headers) > 0 {
		result["headers"] = server.Headers
	}
	return result, nil
}

func decodeProjectedWindsurfServer(raw json.RawMessage) (nativeconfig.Server, error) {
	var entry struct {
		Type    string            `json:"type"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nativeconfig.Server{}, err
	}
	if entry.Type == "stdio" {
		return nativeconfig.Server{Type: "stdio", Command: entry.Command, Args: entry.Args, Env: entry.Env, StdioValuesResolved: true}, nil
	}
	if entry.Type == "streamable-http" || entry.Type == "sse" {
		return nativeconfig.Server{Type: "remote", URL: entry.URL, Headers: entry.Headers, RemoteTransport: entry.Type}, nil
	}
	return nativeconfig.Server{}, fmt.Errorf("unsupported projected transport %q", entry.Type)
}
