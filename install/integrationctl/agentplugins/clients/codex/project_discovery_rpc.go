package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func exchangeCodexProjectSkills(stdin io.Writer, stdout io.Reader, profile, project string, plan domain.DeliveryPlan) error {
	reader := bufio.NewScanner(io.LimitReader(stdout, (4<<20)+1))
	reader.Buffer(make([]byte, 4096), 2<<20)
	writer := json.NewEncoder(stdin)
	initialize := map[string]any{"id": 1, "method": "initialize", "params": map[string]any{
		"clientInfo":   map[string]string{"name": "agentplugins", "version": "1"},
		"capabilities": map[string]bool{"experimentalApi": true},
	}}
	if err := writer.Encode(initialize); err != nil {
		return err
	}
	initialized, err := codexDiscoveryResponse(reader, "1")
	if err != nil {
		return err
	}
	if agent, ok := initialized["userAgent"].(string); !ok || agent == "" {
		return fmt.Errorf("codex initialize returned no client identity")
	}
	if root, present := initialized["codexHome"]; present && !codexDiscoveryPathMatches(root, profile) {
		return fmt.Errorf("codex initialized a different profile")
	}
	if err := writer.Encode(map[string]any{"method": "initialized"}); err != nil {
		return err
	}
	if err := writer.Encode(map[string]any{"id": 2, "method": "skills/list", "params": map[string]any{
		"cwds": []string{project}, "forceReload": true,
	}}); err != nil {
		return err
	}
	listing, err := codexDiscoveryResponse(reader, "2")
	if err != nil {
		return err
	}
	return verifyCodexProjectSkillListing(listing, project, plan)
}

func codexDiscoveryResponse(reader *bufio.Scanner, expectedID string) (map[string]any, error) {
	for records := 0; records < 64 && reader.Scan(); records++ {
		message, err := shared.DecodeStrictJSONObject(reader.Bytes())
		if err != nil {
			return nil, fmt.Errorf("codex discovery returned invalid JSON: %w", err)
		}
		id, hasID := message["id"]
		if !hasID {
			if method, ok := message["method"].(string); ok && method != "" {
				continue
			}
			return nil, fmt.Errorf("codex discovery returned an unidentified response")
		}
		if number, ok := id.(json.Number); !ok || number.String() != expectedID {
			return nil, fmt.Errorf("codex discovery returned an unexpected request or response")
		}
		result, ok := message["result"].(map[string]any)
		_, isRequest := message["method"]
		if _, failed := message["error"]; !ok || failed || isRequest {
			return nil, fmt.Errorf("codex discovery request did not succeed")
		}
		return result, nil
	}
	if err := reader.Err(); err != nil {
		return nil, fmt.Errorf("read Codex discovery: %w", err)
	}
	return nil, fmt.Errorf("codex discovery ended or exceeded its response bound")
}

func verifyCodexProjectSkillListing(listing map[string]any, project string, plan domain.DeliveryPlan) error {
	groups, ok := listing["data"].([]any)
	if !ok || len(groups) != 1 {
		return fmt.Errorf("codex discovery did not return the selected project")
	}
	group, ok := groups[0].(map[string]any)
	if !ok || !codexDiscoveryPathMatches(group["cwd"], project) {
		return fmt.Errorf("codex discovery returned a different project")
	}
	errors, ok := group["errors"].([]any)
	if !ok || len(errors) != 0 {
		return fmt.Errorf("codex reported skill discovery errors")
	}
	skills, ok := group["skills"].([]any)
	if !ok {
		return fmt.Errorf("codex did not return a skill inventory")
	}
	wanted := make(map[string]string)
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentSkill && component.Support != domain.SupportUnsupported {
			wanted[component.Name] = filepath.Join(codexProjectSkillPath(project, component.Name), "SKILL.md")
		}
	}
	return verifyCodexProjectSkillEntries(skills, wanted)
}

func verifyCodexProjectSkillEntries(skills []any, wanted map[string]string) error {
	found := make(map[string]bool)
	for _, raw := range skills {
		skill, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("codex returned an invalid skill entry")
		}
		name, _ := skill["name"].(string)
		expected, selected := wanted[name]
		if !selected {
			continue
		}
		plugin := skill["pluginId"]
		if found[name] || !codexDiscoveryPathMatches(skill["path"], expected) || skill["scope"] != "repo" || skill["enabled"] != true || plugin != nil && plugin != "" {
			return fmt.Errorf("codex did not discover the exact enabled project skill %q", name)
		}
		found[name] = true
	}
	if len(wanted) == 0 || len(found) != len(wanted) {
		return fmt.Errorf("codex did not discover every installed project skill")
	}
	return nil
}

// Windows can return different drive/directory casing and separators. Accept
// only those lexical differences, backed by the same existing native object.
// A different path, symlink or junction cannot grant discovery authority.
func codexDiscoveryPathMatches(value any, expected string) bool {
	actual, ok := value.(string)
	if !ok {
		return false
	}
	if actual == expected {
		return true
	}
	normalized := filepath.FromSlash(actual)
	if runtime.GOOS != "windows" || !cleanAbsolute(normalized) || !strings.EqualFold(normalized, expected) {
		return false
	}
	actualInfo, err := os.Lstat(actual)
	if err != nil || actualInfo.Mode()&os.ModeSymlink != 0 {
		return false
	}
	expectedInfo, err := os.Lstat(expected)
	return err == nil && expectedInfo.Mode()&os.ModeSymlink == 0 && os.SameFile(actualInfo, expectedInfo)
}
