package filesystem

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/adapters/frontmatter"
	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

func (Repository) LoadSkill(root, name string) (domain.SkillDocument, error) {
	full := filepath.Join(root, "skills", name, "SKILL.md")
	body, err := os.ReadFile(full)
	if err != nil {
		return domain.SkillDocument{}, err
	}
	return frontmatter.Parse(body)
}

func (Repository) Discover(root string) ([]string, error) {
	skillsRoot := filepath.Join(root, "skills")
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(skillsRoot, entry.Name(), "SKILL.md")); err == nil {
			out = append(out, entry.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}
