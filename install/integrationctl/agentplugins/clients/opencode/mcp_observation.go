package opencode

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Effective native configuration must describe the exact owned local server,
// not a same-name higher-priority remote server or another command/environment.
func exactOpenCodeLocalMCPConfig(env clients.Env, effective []byte, request domain.ActivationRequest, names []string) bool {
	projection, err := ReadOpenCodeProjection(request.Delivery.ActivePath)
	if err != nil {
		return false
	}
	file, err := env.NativeConfig.ReadExactFile(projection.ConfigPath)
	if err != nil || !file.Exists || len(file.Body) > 4<<20 {
		return false
	}
	standard, err := hujson.Standardize(file.Body)
	if err != nil {
		return false
	}
	owned, ok := openCodeMCPEntries(standard)
	if !ok {
		return false
	}
	observed, ok := openCodeMCPEntries(effective)
	if !ok {
		return false
	}
	for _, name := range names {
		expected, present := owned[name]
		if !present || expected["type"] != "local" {
			return false
		}
		actual, present := observed[name]
		if !present || !reflect.DeepEqual(expected, actual) {
			return false
		}
		if enabled, present := actual["enabled"]; present && enabled != true {
			return false
		}
		if disabled, present := actual["disabled"]; present && disabled != false {
			return false
		}
	}
	return true
}
func openCodeMCPEntries(body []byte) (map[string]map[string]any, bool) {
	var document map[string]json.RawMessage
	if json.Unmarshal(body, &document) != nil {
		return nil, false
	}
	var entries map[string]map[string]any
	if json.Unmarshal(document["mcp"], &entries) != nil || entries == nil {
		return nil, false
	}
	if nested, present := entries["servers"]; present {
		// V2's container is distinct from a V1 server literally named servers.
		if _, server := nested["type"]; !server {
			raw, err := json.Marshal(nested)
			if err != nil || json.Unmarshal(raw, &entries) != nil {
				return nil, false
			}
		}
	}
	return entries, true
}

var openCodeObservationANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// OpenCode 1.x exposes MCP status through this CLI surface, not JSON. Require
// its complete catalog framing and one unambiguous connected row per name.
// A successful process exit alone also occurs for failed/disabled servers.
func connectedOpenCodeMCP(body []byte, names []string) bool {
	text := openCodeObservationANSI.ReplaceAllString(string(body), "")
	if !strings.HasPrefix(text, "┌  MCP Servers\n") || !strings.Contains(text, "server(s)\n") {
		return false
	}
	counts := map[string]int{}
	connected := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "●" {
			counts[fields[2]]++
			connected[fields[2]] = len(fields) == 4 && fields[1] == "✓" && fields[3] == "connected"
		}
	}
	for _, name := range names {
		if counts[name] != 1 || !connected[name] {
			return false
		}
	}
	return true
}
