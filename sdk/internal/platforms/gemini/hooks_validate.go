package gemini

import (
	"encoding/json"
	"fmt"
	"strings"
)

func validateToolInputObject(body json.RawMessage) error {
	if len(body) == 0 {
		return nil
	}
	return validateJSONObjectBytes(body, "hookSpecificOutput.tool_input")
}

func validateTailToolCallRequest(req *TailToolCallRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("hookSpecificOutput.tailToolCallRequest.name is required")
	}
	if err := validateJSONObjectBytes(req.Args, "hookSpecificOutput.tailToolCallRequest.args"); err != nil {
		return err
	}
	return nil
}

func validateModelRequest(body json.RawMessage) error {
	if len(body) == 0 {
		return nil
	}
	return validateJSONObjectBytes(body, "hookSpecificOutput.llm_request")
}

func validateModelResponse(body json.RawMessage) error {
	if len(body) == 0 {
		return nil
	}
	return validateJSONObjectBytes(body, "hookSpecificOutput.llm_response")
}

func validateToolConfig(cfg *ToolConfig) error {
	if cfg == nil {
		return nil
	}
	mode := normalizeToolConfigMode(cfg.Mode)
	switch mode {
	case "", "AUTO", "ANY", "NONE":
	default:
		return fmt.Errorf("hookSpecificOutput.toolConfig.mode must be one of AUTO, ANY, or NONE")
	}
	for _, name := range cfg.AllowedFunctionNames {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("hookSpecificOutput.toolConfig.allowedFunctionNames must not contain empty names")
		}
	}
	if len(cfg.AllowedFunctionNames) > 0 && mode != "ANY" {
		return fmt.Errorf("hookSpecificOutput.toolConfig.allowedFunctionNames currently requires ANY mode")
	}
	return nil
}

func normalizeToolConfigMode(mode string) string {
	return strings.ToUpper(strings.TrimSpace(mode))
}

func normalizeFunctionNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func validateJSONObjectBytes(body []byte, field string) error {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || !strings.HasPrefix(trimmed, "{") {
		return fmt.Errorf("%s must be a JSON object", field)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return fmt.Errorf("%s must be valid JSON object: %w", field, err)
	}
	return nil
}

func validateDecision(decision string) error {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "", "allow", "deny", "block":
		return nil
	default:
		return fmt.Errorf("unknown decision %q", decision)
	}
}
