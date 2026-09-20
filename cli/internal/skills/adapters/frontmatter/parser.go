package frontmatter

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
	"gopkg.in/yaml.v3"
)

func Parse(body []byte) (domain.SkillDocument, error) {
	src := strings.ReplaceAll(string(body), "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	src = strings.TrimPrefix(src, "\ufeff")
	if !strings.HasPrefix(src, "---\n") {
		return domain.SkillDocument{}, fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	rest := strings.TrimPrefix(src, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		if strings.HasSuffix(rest, "\n---") {
			idx = len(rest) - len("\n---")
		} else {
			return domain.SkillDocument{}, fmt.Errorf("SKILL.md frontmatter terminator not found")
		}
	}
	var spec domain.SkillSpec
	if err := yaml.Unmarshal([]byte(rest[:idx]), &spec); err != nil {
		return domain.SkillDocument{}, fmt.Errorf("parse SKILL.md frontmatter: %w", err)
	}
	bodyOffset := idx + len("\n---\n")
	if bodyOffset > len(rest) {
		bodyOffset = len(rest)
	}
	markdown := strings.TrimSpace(rest[bodyOffset:])
	return domain.SkillDocument{
		Spec: spec,
		Body: markdown,
	}, nil
}
