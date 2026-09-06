package conformance

import (
	"bytes"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"gopkg.in/yaml.v3"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

var skillFields = map[string]struct{}{
	"name": {}, "description": {}, "license": {}, "compatibility": {}, "metadata": {}, "allowed-tools": {},
}

func ParseInstallerSkill(directoryName string, body []byte) (domain.Skill, error) {
	return parseSkill(directoryName, body, skillFrontmatter, false)
}
func parseSkill(directoryName string, body []byte, front func([]byte) (map[string]any, error), author bool) (domain.Skill, error) {
	if (!author && (len(directoryName) > 64 || !skillNamePattern.MatchString(directoryName) || strings.Contains(directoryName, "--"))) || (author && !normativeSkillName(directoryName)) {
		return domain.Skill{}, skillError("skill_name_invalid", "directory name %q is not a portable Agent Skill name", directoryName)
	}
	frontmatter, err := front(body)
	if err != nil {
		return domain.Skill{}, err
	}
	for key := range frontmatter {
		if _, ok := skillFields[key]; !ok && !author {
			return domain.Skill{}, skillError("skill_unknown_field", "unsupported frontmatter field %q", key)
		}
	}
	name, err := requiredString(frontmatter, "name")
	if err != nil {
		return domain.Skill{}, err
	}
	if name != directoryName {
		return domain.Skill{}, skillError("skill_name_mismatch", "frontmatter name %q does not match directory %q", name, directoryName)
	}
	description, err := requiredString(frontmatter, "description")
	if err != nil {
		return domain.Skill{}, err
	}
	if count := utf8.RuneCountInString(description); count < 1 || count > 1024 {
		return domain.Skill{}, skillError("skill_description_length", "description length must be between 1 and 1024 characters")
	}
	license, err := optionalString(frontmatter, "license")
	if err != nil {
		return domain.Skill{}, err
	}
	compatibility, err := optionalString(frontmatter, "compatibility")
	if err != nil {
		return domain.Skill{}, err
	}
	if utf8.RuneCountInString(compatibility) > 500 || (author && frontmatter["compatibility"] != nil && compatibility == "") {
		return domain.Skill{}, skillError("skill_compatibility_length", "compatibility exceeds 500 characters")
	}
	allowedTools, err := optionalString(frontmatter, "allowed-tools")
	if err != nil {
		return domain.Skill{}, err
	}
	metadata := map[string]any(nil)
	if value, ok := frontmatter["metadata"]; ok {
		var object bool
		metadata, object = value.(map[string]any)
		if !object {
			return domain.Skill{}, skillError("skill_metadata_type", "metadata must be an object")
		}
	}
	if author {
		for _, value := range metadata {
			if _, ok := value.(string); !ok {
				return domain.Skill{}, skillError("skill_metadata_type", "metadata values must be strings")
			}
		}
	}
	return domain.Skill{
		Name:          name,
		Description:   description,
		License:       license,
		Compatibility: compatibility,
		Metadata:      metadata,
		AllowedTools:  allowedTools,
		RelativePath:  "skills/" + directoryName + "/SKILL.md",
		Raw:           append([]byte(nil), body...),
	}, nil
}

func frontmatterBytes(body []byte) ([]byte, error) {
	normalized := bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(normalized, []byte("---\n")) {
		return nil, skillError("skill_frontmatter_open", "SKILL.md must begin with YAML frontmatter")
	}
	remainder := normalized[len("---\n"):]
	end := bytes.Index(remainder, []byte("\n---\n"))
	if end < 0 {
		if bytes.HasSuffix(remainder, []byte("\n---")) {
			end = len(remainder) - len("\n---")
		} else {
			return nil, skillError("skill_frontmatter_close", "SKILL.md frontmatter is not terminated")
		}
	}
	return remainder[:end], nil
}
func skillFrontmatter(body []byte) (map[string]any, error) {
	data, err := frontmatterBytes(body)
	if err != nil {
		return nil, err
	}
	var frontmatter map[string]any
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&frontmatter); err != nil {
		return nil, skillError("skill_yaml_invalid", "parse skill frontmatter: %v", err)
	}
	if frontmatter == nil {
		return nil, skillError("skill_frontmatter_type", "skill frontmatter must be an object")
	}
	return frontmatter, nil
}

func requiredString(values map[string]any, key string) (string, error) {
	value, ok := values[key]
	if !ok {
		return "", skillError("skill_required_field", "frontmatter requires %s", key)
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", skillError("skill_required_type", "frontmatter %s must be a non-empty string", key)
	}
	return text, nil
}

func optionalString(values map[string]any, key string) (string, error) {
	value, ok := values[key]
	if !ok {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", skillError("skill_optional_type", "frontmatter %s must be a string", key)
	}
	return text, nil
}

type skillFailure struct{ Code, message string }

func (e *skillFailure) Error() string { return e.message }
func skillError(code, format string, args ...any) error {
	return &skillFailure{code, fmt.Sprintf(format, args...)}
}
func normativeSkillName(name string) bool {
	n := utf8.RuneCountInString(name)
	if n < 1 || n > 64 || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return false
	}
	for _, r := range name {
		if r != '-' && !unicode.IsLower(r) && !unicode.IsNumber(r) {
			return false
		}
	}
	return true
}
