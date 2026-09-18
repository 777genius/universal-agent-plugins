package opencode

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	OpenCodeProjectionFile = ".agentplugins-opencode.json"
	OpenCodeMCPObjectKind  = "opencode_global_mcp_server"
	openCodeSkillKind      = "opencode_global_skill_directory"
)

type OpenCodeProjection struct {
	ResolvedCWD map[string]bool                `json:"resolved_cwd,omitempty"`
	Version     int                            `json:"version"`
	ConfigPath  string                         `json:"config_path"`
	ConfigJSON  string                         `json:"config_json"`
	ConfigJSONC string                         `json:"config_jsonc"`
	PackageRoot string                         `json:"package_root"`
	DataRoot    string                         `json:"data_root,omitempty"`
	MCPServers  map[string]nativeconfig.Server `json:"mcp_servers,omitempty"`
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

func ReadOpenCodeProjection(root string) (OpenCodeProjection, error) {
	body, err := os.ReadFile(filepath.Join(root, OpenCodeProjectionFile))
	if err != nil {
		return OpenCodeProjection{}, fmt.Errorf("read OpenCode native projection: %w", err)
	}
	projection, err := decodeOpenCodeProjection(body)
	if err != nil {
		return OpenCodeProjection{}, err
	}
	if err := validateOpenCodeProjectionPaths(projection); err != nil {
		return OpenCodeProjection{}, err
	}
	return applyResolvedOpenCodeCWDs(projection)
}

func decodeOpenCodeProjection(body []byte) (OpenCodeProjection, error) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var projection OpenCodeProjection
	if err := decoder.Decode(&projection); err != nil || projection.Version != 1 {
		return OpenCodeProjection{}, fmt.Errorf("decode OpenCode native projection")
	}
	if err := requireJSONEOF(decoder); err != nil {
		return OpenCodeProjection{}, err
	}
	return projection, nil
}

func validateOpenCodeProjectionPaths(projection OpenCodeProjection) error {
	if !filepath.IsAbs(projection.ConfigPath) || !filepath.IsAbs(projection.ConfigJSON) || !filepath.IsAbs(projection.ConfigJSONC) || !filepath.IsAbs(projection.PackageRoot) {
		return fmt.Errorf("OpenCode projection contains relative paths")
	}
	if projection.ConfigPath != projection.ConfigJSON && projection.ConfigPath != projection.ConfigJSONC {
		return fmt.Errorf("OpenCode projection selects an unexpected config path")
	}
	return nil
}

func applyResolvedOpenCodeCWDs(projection OpenCodeProjection) (OpenCodeProjection, error) {
	for name, resolved := range projection.ResolvedCWD {
		server, ok := projection.MCPServers[name]
		if !ok || !resolved || server.Type != "stdio" || !filepath.IsAbs(server.CWD) {
			return OpenCodeProjection{}, fmt.Errorf("invalid resolved OpenCode cwd provenance")
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

func openCodeConfigPresence(configRoot string) (jsonExists, jsoncExists bool, err error) {
	jsonExists, err = regularNativeFileExists(filepath.Join(configRoot, "opencode.json"))
	if err != nil {
		return false, false, err
	}
	jsoncExists, err = regularNativeFileExists(filepath.Join(configRoot, "opencode.jsonc"))
	return jsonExists, jsoncExists, err
}

func sameOpenCodeMCPObject(left, right domain.NativeObjectOwnership) bool {
	return left.ObjectID == right.ObjectID && left.Kind == OpenCodeMCPObjectKind && right.Kind == OpenCodeMCPObjectKind &&
		left.LogicalName == right.LogicalName && shared.SameCleanPath(left.Path, right.Path) && left.ManagedDigest == right.ManagedDigest
}

func receiptFromOpenCodeObject(object domain.NativeObjectOwnership) nativeconfig.Receipt {
	return nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecOpenCode, Name: object.LogicalName, Digest: object.ManagedDigest}
}

func OpenCodeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	var result []domain.NativeObjectOwnership
	for _, object := range objects {
		if object.Kind == OpenCodeMCPObjectKind || object.Kind == openCodeSkillKind {
			result = append(result, object)
		}
	}
	return result
}
