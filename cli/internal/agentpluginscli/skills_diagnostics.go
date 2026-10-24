package agentpluginscli

import "strings"

// Codes come from the bounded source/conformance parser. Keep provider errors
// out of user guidance and retain the machine-readable codes on the candidate.
func skillDiagnosticGuidance(codes []string) string {
	messages := []string{}
	seen := map[string]bool{}
	for _, code := range codes {
		message := skillDiagnosticMessage(code)
		if !seen[message] {
			messages = append(messages, message)
			seen[message] = true
		}
	}
	return strings.Join(messages, "; ")
}

func skillDiagnosticMessage(code string) string {
	switch code {
	case "skill_required_field", "skill_required_type":
		return "frontmatter needs non-empty string name and description fields"
	case "skill_name_invalid":
		return "the Skill folder name must follow Agent Skills naming rules"
	case "skill_name_mismatch":
		return "the frontmatter name must match its Skill folder name"
	case "skill_frontmatter_open", "skill_frontmatter_close":
		return "SKILL.md must begin with YAML frontmatter enclosed by --- lines"
	case "skill_document_too_large", "document_bytes_limit":
		return "SKILL.md exceeds the document size limit"
	case "skill_document_unavailable":
		return "SKILL.md could not be read from this source revision"
	case "skill_document_wrong_kind":
		return "SKILL.md must be a regular file"
	case "skill_aggregate_bytes_limit", "aggregate_bytes_limit":
		return "the scan byte limit was reached; narrow the source folder"
	case "skill_description_length":
		return "description must contain between 1 and 1024 characters"
	case "skill_metadata_type":
		return "metadata must be an object with string values"
	}
	if strings.HasPrefix(code, "yaml_") {
		return "frontmatter YAML is invalid or exceeds parser limits"
	}
	return "the Skill document did not pass bounded metadata validation"
}
