package grok

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

var _ clients.ActiveNativeProjector = (*Adapter)(nil)

// The stager verifies immutable package bytes before this read-only operation.
// Existing User projection behavior does not acquire a new native observation.
func (*Adapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plan.Scope != domain.ScopeProject {
		return nil, nil
	}
	if plan.ClientID != domain.ClientGrok || root != plan.ActivePath {
		return nil, fmt.Errorf("grok active projection client or root differs")
	}
	if _, err := grokPlanRoot(domain.DetectedClient{ClientID: domain.ClientGrok, ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
		return nil, err
	}
	if err := plan.SelectedDelivery.ValidatePlan(plan, envelope.TreeDigest); err != nil {
		return nil, err
	}
	return nil, validateGrokStdio(root, envelope, plan, dataPath)
}

func grokMCPDocument(root string) (map[string]any, map[string]any, error) {
	file, err := nativeconfig.New().ReadExactFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		return nil, nil, err
	}
	document, err := shared.DecodeStrictJSONObject(file.Body)
	if err != nil {
		return nil, nil, err
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("grok projected MCP collection is missing")
	}
	return document, servers, nil
}

// Grok 1.0.70 discards the native cwd field. The existing trusted process
// launcher establishes portable cwd without trusting the caller's directory.
func projectGrokStdio(in clients.ProjectionInput) error {
	if !shared.HasSupportedMCP(in.Plan.Components) {
		return nil
	}
	document, servers, err := grokMCPDocument(in.StagingPath)
	if err != nil {
		return err
	}
	for _, name := range shared.SupportedMCPNames(in.Plan) {
		source := in.Envelope.MCP.Servers[name]
		if source.Type != "stdio" {
			continue
		}
		entry, ok := servers[name].(map[string]any)
		if !ok {
			return fmt.Errorf("grok projected stdio server is missing")
		}
		if err := bindGrokStdio(entry, source, in.Plan.ActivePath, in.PluginDataPath); err != nil {
			return err
		}
	}
	return shared.WriteJSON(filepath.Join(in.StagingPath, ".mcp.json"), document)
}

func bindGrokStdio(entry map[string]any, source domain.MCPServer, plugin, data string) error {
	if !filepath.IsAbs(plugin) || !filepath.IsAbs(data) || filepath.Clean(plugin) != plugin || filepath.Clean(data) != data {
		return fmt.Errorf("grok managed stdio roots must be clean absolute paths")
	}
	rawCWD, _ := source.Decoded["cwd"].(string)
	cwd, err := pathcontract.ExpandCWD(rawCWD, plugin, data)
	if err != nil {
		return err
	}
	absoluteCWD, ok := entry["cwd"].(string)
	if !ok {
		return fmt.Errorf("grok projected stdio cwd is missing")
	}
	args, err := grokStdioArguments(entry)
	if err != nil {
		return err
	}
	command, ok := source.Decoded["command"].(string)
	if !ok || command == "" {
		return fmt.Errorf("grok stdio source command is missing")
	}
	encoded := managedstdio.EncodedArguments(plugin, data, absoluteCWD, cwd.Anchor, command, args)
	values := make([]any, len(encoded))
	for i, value := range encoded {
		values[i] = value
	}
	entry["command"] = filepath.Join(plugin, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	entry["args"] = values
	delete(entry, "cwd")
	return nil
}

func grokStdioArguments(entry map[string]any) ([]string, error) {
	value, present := entry["args"]
	if !present {
		return nil, nil
	}
	if args, ok := value.([]string); ok {
		return append([]string(nil), args...), nil
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("grok stdio arguments must be an array")
	}
	args := make([]string, len(raw))
	for i, value := range raw {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("grok stdio arguments must be strings")
		}
		args[i] = text
	}
	return args, nil
}

func validateGrokStdio(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, data string) error {
	for _, name := range shared.SupportedMCPNames(plan) {
		source := envelope.MCP.Servers[name]
		if source.Type != "stdio" {
			continue
		}
		_, servers, err := grokMCPDocument(root)
		if err != nil {
			return err
		}
		actual, ok := servers[name].(map[string]any)
		if !ok {
			return fmt.Errorf("%w: grok stdio entry is missing", domain.ErrProjectionRefreshRequired)
		}
		expected := shared.CloneObject(source.Decoded)
		delete(expected, "type")
		if err := shared.ApplyStdioDataContract(expected, plan.ActivePath, data, root); err != nil {
			return err
		}
		if err := bindGrokStdio(expected, source, plan.ActivePath, data); err != nil {
			return err
		}
		if !reflect.DeepEqual(actual, expected) {
			return fmt.Errorf("%w: grok launcher package/data/cwd/arguments differ", domain.ErrProjectionRefreshRequired)
		}
		if err := verifyGrokLauncher(root); err != nil {
			return err
		}
	}
	return nil
}

func verifyGrokLauncher(root string) error {
	directory := filepath.Join(root, filepath.FromSlash(managedstdio.RelativeDirectory))
	metadata, err := nativeconfig.New().ReadExactFile(filepath.Join(directory, "metadata.json"))
	if err != nil {
		return err
	}
	value, err := shared.DecodeStrictJSONObject(metadata.Body)
	if err != nil {
		return err
	}
	digest, err := grokHelperDigest(root)
	if err != nil {
		return err
	}
	protocol, ok := value["protocol"].(json.Number)
	version, versionOK := value["cliVersion"].(string)
	if len(value) != 3 || !ok || protocol.String() != "1" || !versionOK || version == "" || value["sha256"] != digest {
		return fmt.Errorf("%w: grok managed helper identity differs", domain.ErrProjectionRefreshRequired)
	}
	return nil
}

func grokHelperDigest(root string) (string, error) {
	relative := filepath.ToSlash(filepath.Join(managedstdio.RelativeDirectory, managedstdio.ExecutableName))
	resolved := pathcontract.Resolve(root, relative)
	path := filepath.Join(root, filepath.FromSlash(relative))
	if resolved.State != pathcontract.Resolved || resolved.Path != path {
		return "", fmt.Errorf("%w: grok managed helper path differs", domain.ErrProjectionRefreshRequired)
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() <= 0 || before.Size() > 128<<20 {
		return "", fmt.Errorf("%w: grok managed helper is unavailable", domain.ErrProjectionRefreshRequired)
	}
	file, err := openProjectHandle(path, false)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	held, err := file.Stat()
	if err != nil || !os.SameFile(before, held) {
		return "", fmt.Errorf("grok managed helper replaced")
	}
	body, err := io.ReadAll(io.LimitReader(file, (128<<20)+1))
	if err != nil {
		return "", err
	}
	after, statErr := file.Stat()
	current, pathErr := os.Lstat(path)
	if statErr != nil || pathErr != nil || int64(len(body)) != before.Size() || !os.SameFile(before, current) ||
		after.Size() != held.Size() || !after.ModTime().Equal(held.ModTime()) || pathcontract.Resolve(root, relative).Path != path {
		return "", fmt.Errorf("grok managed helper changed")
	}
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:]), nil
}
