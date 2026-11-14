package cline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

func projectClineServer(root string, source domain.MCPServer, plan domain.DeliveryPlan, data string) (nativeconfig.Server, error) {
	portable := source
	if source.Type == "stdio" {
		// Preserve authored type errors before the shared stdio contract fills
		// absent env/cwd defaults. Explicit null must never look like absence.
		if source.Decoded == nil {
			return nativeconfig.Server{}, fmt.Errorf("cline authored stdio MCP configuration is missing")
		}
		if _, err := ClineNeutralServer(source); err != nil {
			return nativeconfig.Server{}, fmt.Errorf("cline authored stdio fields invalid: %w", err)
		}
		portable.Decoded = shared.CloneObject(source.Decoded)
		if err := shared.ApplyStdioDataContract(portable.Decoded, plan.ActivePath, data, root); err != nil {
			return nativeconfig.Server{}, err
		}
	}
	server, err := ClineNeutralServer(portable)
	if err != nil {
		return server, err
	}
	server.StdioValuesResolved = source.Type == "stdio"
	if source.Type == "stdio" && clineProjectSelected(plan) {
		return bindClineProjectStdio(server, source, plan.ActivePath, data)
	}
	return server, nil
}

func bindClineProjectStdio(server nativeconfig.Server, source domain.MCPServer, active, data string) (nativeconfig.Server, error) {
	if !filepath.IsAbs(active) || filepath.Clean(active) != active || !filepath.IsAbs(data) || filepath.Clean(data) != data {
		return server, fmt.Errorf("cline selected stdio roots must be clean and absolute")
	}
	rawCWD, _ := source.Decoded["cwd"].(string)
	cwd, err := pathcontract.ExpandCWD(rawCWD, active, data)
	if err != nil {
		return server, err
	}
	command, ok := source.Decoded["command"].(string)
	if !ok || command == "" {
		return server, fmt.Errorf("cline authored stdio command missing")
	}
	server.Args = managedstdio.Arguments(active, data, server.CWD, cwd.Anchor, command, server.Args)
	server.Command = filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	return server, nil
}

// The stager checks the immutable package digest before this read-only join.
// Old unwrapped projections require transactional refresh, never in-place edits.
func validateClineProjectStdio(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, data string) error {
	var projection *ClineProjection
	for _, name := range shared.SupportedMCPNames(plan) {
		source := envelope.MCP.Servers[name]
		if source.Type != "stdio" {
			continue
		}
		expected, err := projectClineServer(root, source, plan, data)
		if err != nil {
			return fmt.Errorf("%w: Cline stdio source differs: %w", domain.ErrProjectionRefreshRequired, err)
		}
		if projection == nil {
			file, err := nativeconfig.New().ReadExactFile(filepath.Join(root, ClineProjectionFile))
			if err != nil {
				return fmt.Errorf("%w: Cline stdio projection missing: %w", domain.ErrProjectionRefreshRequired, err)
			}
			if _, err = shared.DecodeStrictJSONObject(file.Body); err != nil {
				return fmt.Errorf("%w: Cline projection JSON differs: %w", domain.ErrProjectionRefreshRequired, err)
			}
			var loaded ClineProjection
			decoder := json.NewDecoder(bytes.NewReader(file.Body))
			decoder.DisallowUnknownFields()
			if err = decoder.Decode(&loaded); err != nil {
				return fmt.Errorf("%w: Cline projection fields differ: %w", domain.ErrProjectionRefreshRequired, err)
			}
			projection = &loaded
		}
		actual, ok := projection.Servers[name]
		actual.StdioValuesResolved = true
		if !ok || !reflect.DeepEqual(actual, expected) {
			return fmt.Errorf("%w: Cline stdio package/data/launcher projection differs", domain.ErrProjectionRefreshRequired)
		}
	}
	return nil
}
