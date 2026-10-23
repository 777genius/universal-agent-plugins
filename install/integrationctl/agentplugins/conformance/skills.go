package conformance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// MaxSkillNameBytes retains its historical exported name; the normative Skill
// name limit counts Unicode characters. Generated ASCII names also use it.
const (
	MaxSkillNameBytes        = 64
	MaxSkillDescriptionRunes = 1024
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

// PortableSkillNameForm is the ASCII subset used for generated command names.
// Normative Skill admission uses the Unicode-aware shared ParseSkill grammar.
func PortableSkillNameForm(name string) bool {
	return skillNamePattern.MatchString(name) && !strings.Contains(name, "--")
}

var skillFields = map[string]struct{}{
	"name": {}, "description": {}, "license": {}, "compatibility": {}, "metadata": {}, "allowed-tools": {},
}

func ParseInstallerSkill(directoryName string, body []byte) (domain.Skill, error) {
	return ParseSkill(context.Background(), directoryName, body)
}

// ParseSkill shares the bounded normative grammar with authoring. Unknown fields
// are opaque and Raw retains the complete original document bytes.
func ParseSkill(ctx context.Context, directoryName string, body []byte) (domain.Skill, error) {
	if err := ctx.Err(); err != nil {
		return domain.Skill{}, err
	}
	if len(body) > DefaultLimits().SkillBytes {
		return domain.Skill{}, &parseFailure{Code: "document_bytes_limit"}
	}
	if !utf8.Valid(body) {
		return domain.Skill{}, &parseFailure{Code: "document_utf8_invalid"}
	}
	return parseSkill(directoryName, body, func(b []byte) (map[string]any, error) { return skillFrontmatter(ctx, b) }, true)
}

func parseSkill(directoryName string, body []byte, front func([]byte) (map[string]any, error), author bool) (domain.Skill, error) {
	if (!author && (len(directoryName) > MaxSkillNameBytes || !PortableSkillNameForm(directoryName))) || (author && !normativeSkillName(directoryName)) {
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
	if count := utf8.RuneCountInString(description); count < 1 || count > MaxSkillDescriptionRunes {
		return domain.Skill{}, skillError("skill_description_length", "description length must be between 1 and %d characters", MaxSkillDescriptionRunes)
	}
	license, err := optionalString(frontmatter, "license")
	if err != nil {
		return domain.Skill{}, err
	}
	compatibility, err := optionalString(frontmatter, "compatibility")
	if err != nil {
		return domain.Skill{}, err
	}
	if utf8.RuneCountInString(compatibility) > 500 || (frontmatter["compatibility"] != nil && compatibility == "") {
		return domain.Skill{}, skillError("skill_compatibility_length", "compatibility length must be between 1 and 500 characters when supplied")
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
	for _, value := range metadata {
		if _, ok := value.(string); !ok {
			return domain.Skill{}, skillError("skill_metadata_type", "metadata values must be strings")
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
	header, _, err := splitFrontmatter(body)
	return header, err
}

// splitFrontmatter returns the YAML header and everything after its closing
// delimiter, both with CRLF folded to LF.
func splitFrontmatter(body []byte) ([]byte, []byte, error) {
	normalized := bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(normalized, []byte("---\n")) {
		return nil, nil, skillError("skill_frontmatter_open", "SKILL.md must begin with YAML frontmatter")
	}
	remainder := normalized[len("---\n"):]
	end := bytes.Index(remainder, []byte("\n---\n"))
	if end < 0 {
		if bytes.HasSuffix(remainder, []byte("\n---")) {
			return remainder[:len(remainder)-len("\n---")], nil, nil
		}
		return nil, nil, skillError("skill_frontmatter_close", "SKILL.md frontmatter is not terminated")
	}
	return remainder[:end], remainder[end+len("\n---\n"):], nil
}
func skillFrontmatter(ctx context.Context, body []byte) (map[string]any, error) {
	return boundedFrontmatter(ctx, body, DefaultLimits())
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
	if n < 1 || n > MaxSkillNameBytes || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return false
	}
	for _, r := range name {
		if r != '-' && !unicode.IsLower(r) && !unicode.IsNumber(r) {
			return false
		}
	}
	return true
}

// SkillErrorCode distinguishes normative invalidity from incomplete host-safety
// evidence without exposing raw parser errors in discovery responses.
func SkillErrorCode(err error) (code string, unavailable bool) {
	var safety *parseFailure
	if errors.As(err, &safety) {
		return safety.Code, true
	}
	var invalid *skillFailure
	if errors.As(err, &invalid) {
		return invalid.Code, false
	}
	return "skill_metadata_unavailable", true
}
