package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("skill name is empty")
	}
	for _, r := range name {
		if !(r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return fmt.Errorf("invalid skill name %q", name)
		}
	}
	return nil
}

func validateSkillMetadata(skillPath, name string, doc domain.SkillDocument) []ValidationFailure {
	var failures []ValidationFailure
	if strings.TrimSpace(doc.Spec.Name) == "" {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "missing frontmatter field: name"})
	} else if strings.TrimSpace(doc.Spec.Name) != name {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: fmt.Sprintf("frontmatter name %q must match skill directory %q", doc.Spec.Name, name)})
	} else if err := validateName(strings.TrimSpace(doc.Spec.Name)); err != nil {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: err.Error()})
	}
	if strings.TrimSpace(doc.Spec.Description) == "" {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "missing frontmatter field: description"})
	}
	switch doc.Spec.ExecutionMode {
	case domain.ExecutionDocsOnly, domain.ExecutionCommand:
	default:
		failures = append(failures, ValidationFailure{Path: skillPath, Message: fmt.Sprintf("invalid execution_mode %q (expected %q or %q)", doc.Spec.ExecutionMode, domain.ExecutionDocsOnly, domain.ExecutionCommand)})
	}
	if len(doc.Spec.SupportedAgents) == 0 {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "missing frontmatter field: supported_agents"})
	}
	return failures
}

func validateSkillCollections(skillPath string, doc domain.SkillDocument) []ValidationFailure {
	var failures []ValidationFailure
	failures = append(failures, validateTrimmedUniqueValues(skillPath, "allowed_tools", doc.Spec.AllowedTools)...)
	failures = append(failures, validateNonEmptyValues(skillPath, "inputs", doc.Spec.Inputs)...)
	failures = append(failures, validateNonEmptyValues(skillPath, "outputs", doc.Spec.Outputs)...)
	failures = append(failures, validateNonEmptyValues(skillPath, "compatibility.requires", doc.Spec.Compatibility.Requires)...)
	failures = append(failures, validateNonEmptyValues(skillPath, "compatibility.supported_os", doc.Spec.Compatibility.SupportedOS)...)
	failures = append(failures, validateNonEmptyValues(skillPath, "compatibility.notes", doc.Spec.Compatibility.Notes)...)
	return failures
}

func validateSkillAgentHints(skillPath string, doc domain.SkillDocument) []ValidationFailure {
	var failures []ValidationFailure
	agentHintKeys := make([]string, 0, len(doc.Spec.AgentHints))
	for key := range doc.Spec.AgentHints {
		agentHintKeys = append(agentHintKeys, key)
	}
	sort.Strings(agentHintKeys)
	for _, key := range agentHintKeys {
		hint := doc.Spec.AgentHints[key]
		switch domain.Agent(key) {
		case domain.AgentClaude, domain.AgentCodex:
		default:
			failures = append(failures, ValidationFailure{Path: skillPath, Message: fmt.Sprintf("unsupported agent_hints key %q", key)})
			continue
		}
		if !containsAgent(doc.Spec.SupportedAgents, domain.Agent(key)) {
			failures = append(failures, ValidationFailure{Path: skillPath, Message: fmt.Sprintf("agent_hints.%s requires %q in supported_agents", key, key)})
		}
		for _, note := range hint.Notes {
			if strings.TrimSpace(note) == "" {
				failures = append(failures, ValidationFailure{Path: skillPath, Message: fmt.Sprintf("agent_hints.%s.notes cannot contain empty values", key)})
			}
		}
	}
	return failures
}

func validateSkillAgents(skillPath string, doc domain.SkillDocument) []ValidationFailure {
	var failures []ValidationFailure
	seenAgents := make(map[domain.Agent]struct{}, len(doc.Spec.SupportedAgents))
	for _, agent := range doc.Spec.SupportedAgents {
		if _, ok := seenAgents[agent]; ok {
			failures = append(failures, ValidationFailure{Path: skillPath, Message: fmt.Sprintf("supported_agents contains duplicate value %q", agent)})
			continue
		}
		seenAgents[agent] = struct{}{}
		switch agent {
		case domain.AgentClaude, domain.AgentCodex:
		default:
			failures = append(failures, ValidationFailure{Path: skillPath, Message: fmt.Sprintf("unsupported agent %q (supported: %q, %q)", agent, domain.AgentClaude, domain.AgentCodex)})
		}
	}
	return failures
}

func validateTrimmedUniqueValues(skillPath, field string, values []string) []ValidationFailure {
	var failures []ValidationFailure
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			failures = append(failures, ValidationFailure{Path: skillPath, Message: field + " cannot contain empty values"})
			continue
		}
		if _, ok := seen[trimmed]; ok {
			failures = append(failures, ValidationFailure{Path: skillPath, Message: fmt.Sprintf("%s contains duplicate value %q", field, trimmed)})
			continue
		}
		seen[trimmed] = struct{}{}
	}
	return failures
}

func validateNonEmptyValues(skillPath, field string, values []string) []ValidationFailure {
	var failures []ValidationFailure
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			failures = append(failures, ValidationFailure{Path: skillPath, Message: field + " cannot contain empty values"})
		}
	}
	return failures
}

func containsAgent(agents []domain.Agent, want domain.Agent) bool {
	for _, agent := range agents {
		if agent == want {
			return true
		}
	}
	return false
}
