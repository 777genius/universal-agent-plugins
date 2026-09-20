package platformexec

import (
	"fmt"
	"strings"
)

func validateOpenCodeAgentFrontmatterFields(rel string, frontmatter map[string]any) []Diagnostic {
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, validateOpenCodeAgentDescription(rel, frontmatter)...)
	for _, field := range []string{"mode", "model", "variant", "color"} {
		diagnostics = append(diagnostics, validateOpenCodeAgentStringFrontmatter(rel, frontmatter, field)...)
	}
	for _, field := range []string{"temperature", "top_p"} {
		diagnostics = append(diagnostics, validateOpenCodeAgentNumericField(rel, frontmatter, field)...)
	}
	for _, field := range []string{"disable", "hidden"} {
		diagnostics = append(diagnostics, validateOpenCodeAgentBoolField(rel, frontmatter, field)...)
	}
	diagnostics = append(diagnostics, validateOpenCodeAgentStepsField(rel, frontmatter)...)
	diagnostics = append(diagnostics, validateOpenCodeAgentOptionsField(rel, frontmatter)...)
	diagnostics = append(diagnostics, validateOpenCodeAgentPermissionField(rel, frontmatter)...)
	diagnostics = append(diagnostics, validateOpenCodeAgentDeprecatedFields(rel, frontmatter)...)
	return diagnostics
}

func validateOpenCodeAgentDescription(rel string, frontmatter map[string]any) []Diagnostic {
	description, ok := frontmatter["description"].(string)
	if ok && stringsTrimSpace(description) != "" {
		return nil
	}
	return []Diagnostic{openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode agent file %s must declare a non-empty frontmatter description", rel))}
}

func validateOpenCodeAgentStringFrontmatter(rel string, frontmatter map[string]any, field string) []Diagnostic {
	raw, ok := frontmatter[field]
	if !ok {
		return nil
	}
	text, ok := raw.(string)
	if ok && stringsTrimSpace(text) != "" {
		return nil
	}
	return []Diagnostic{openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode agent file %s frontmatter field %q must be a non-empty string", rel, field))}
}

func validateOpenCodeAgentNumericField(rel string, frontmatter map[string]any, field string) []Diagnostic {
	raw, ok := frontmatter[field]
	if !ok {
		return nil
	}
	if _, ok := raw.(float64); ok {
		return nil
	}
	return []Diagnostic{openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode agent file %s frontmatter field %q must be a number", rel, field))}
}

func validateOpenCodeAgentBoolField(rel string, frontmatter map[string]any, field string) []Diagnostic {
	raw, ok := frontmatter[field]
	if !ok {
		return nil
	}
	if _, ok := raw.(bool); ok {
		return nil
	}
	return []Diagnostic{openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode agent file %s frontmatter field %q must be a boolean", rel, field))}
}

func validateOpenCodeAgentStepsField(rel string, frontmatter map[string]any) []Diagnostic {
	raw, ok := frontmatter["steps"]
	if !ok {
		return nil
	}
	if value, ok := raw.(int); ok && value > 0 {
		return nil
	}
	if floatValue, ok := raw.(float64); ok && floatValue == float64(int(floatValue)) && int(floatValue) > 0 {
		return nil
	}
	return []Diagnostic{openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode agent file %s frontmatter field %q must be a positive integer", rel, "steps"))}
}

func validateOpenCodeAgentOptionsField(rel string, frontmatter map[string]any) []Diagnostic {
	raw, ok := frontmatter["options"]
	if !ok {
		return nil
	}
	if _, ok := raw.(map[string]any); ok {
		return nil
	}
	return []Diagnostic{openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode agent file %s frontmatter field %q must be an object", rel, "options"))}
}

func validateOpenCodeAgentPermissionField(rel string, frontmatter map[string]any) []Diagnostic {
	raw, ok := frontmatter["permission"]
	if !ok || isOpenCodePermissionValue(raw) {
		return nil
	}
	return []Diagnostic{openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode agent file %s frontmatter field %q must be a string or object", rel, "permission"))}
}

func validateOpenCodeAgentDeprecatedFields(rel string, frontmatter map[string]any) []Diagnostic {
	var diagnostics []Diagnostic
	if _, ok := frontmatter["tools"]; ok {
		diagnostics = append(diagnostics, openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode agent file %s frontmatter field %q is deprecated; use %q instead", rel, "tools", "permission")))
	}
	if _, ok := frontmatter["maxSteps"]; ok {
		diagnostics = append(diagnostics, openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode agent file %s frontmatter field %q is deprecated; use %q instead", rel, "maxSteps", "steps")))
	}
	return diagnostics
}

func stringsTrimSpace(value string) string {
	return strings.TrimSpace(value)
}
