package platformexec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func appendOpenCodeConfigDocs(root string, state pluginmodel.TargetState, doc map[string]any) error {
	if err := appendOpenCodeDefaultAgent(root, state, doc); err != nil {
		return err
	}
	if err := appendOpenCodeInstructions(root, state, doc); err != nil {
		return err
	}
	if err := appendOpenCodePermission(root, state, doc); err != nil {
		return err
	}
	return nil
}

func appendOpenCodeDefaultAgent(root string, state pluginmodel.TargetState, doc map[string]any) error {
	rel := strings.TrimSpace(state.DocPath("default_agent"))
	if rel == "" {
		return nil
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return fmt.Errorf("%s must contain a non-empty agent name", rel)
	}
	doc["default_agent"] = text
	return nil
}

func appendOpenCodeInstructions(root string, state pluginmodel.TargetState, doc map[string]any) error {
	rel := strings.TrimSpace(state.DocPath("instructions_config"))
	if rel == "" {
		return nil
	}
	instructions, _, err := readYAMLDoc[[]string](root, rel)
	if err != nil {
		return fmt.Errorf("parse %s: %w", rel, err)
	}
	if len(instructions) == 0 {
		return fmt.Errorf("%s must contain at least one instruction path", rel)
	}
	for i, instruction := range instructions {
		if strings.TrimSpace(instruction) == "" {
			return fmt.Errorf("%s instruction entry %d must be a non-empty string", rel, i)
		}
		instructions[i] = strings.TrimSpace(instruction)
	}
	doc["instructions"] = instructions
	return nil
}

func appendOpenCodePermission(root string, state pluginmodel.TargetState, doc map[string]any) error {
	rel := strings.TrimSpace(state.DocPath("permission_config"))
	if rel == "" {
		return nil
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return err
	}
	var permission any
	if err := json.Unmarshal(body, &permission); err != nil {
		return fmt.Errorf("parse %s: %w", rel, err)
	}
	if !isOpenCodePermissionValue(permission) {
		return fmt.Errorf("%s must be a JSON string or object", rel)
	}
	doc["permission"] = permission
	return nil
}
