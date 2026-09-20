package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
)

func (Repository) InitSkill(root string, data TemplateData, templateName InitTemplate, force bool) error {
	skillRoot := filepath.Join(root, "skills", data.SkillName)
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		return err
	}
	var files []struct {
		path string
		tpl  string
	}
	switch templateName {
	case TemplateGoCommand:
		files = []struct {
			path string
			tpl  string
		}{
			{path: filepath.Join("skills", data.SkillName, "SKILL.md"), tpl: "skill.go-command.md.tmpl"},
			{path: filepath.Join("cmd", data.SkillName, "main.go"), tpl: "skill.go-command.main.go.tmpl"},
			{path: filepath.Join("cmd", data.SkillName, "main_test.go"), tpl: "skill.go-command.test.go.tmpl"},
		}
	case TemplateCLIWrapper:
		files = []struct {
			path string
			tpl  string
		}{
			{path: filepath.Join("skills", data.SkillName, "SKILL.md"), tpl: "skill.cli-wrapper.md.tmpl"},
			{path: filepath.Join("skills", data.SkillName, "scripts", ".keep"), tpl: ""},
		}
	case TemplateDocsOnly:
		files = []struct {
			path string
			tpl  string
		}{
			{path: filepath.Join("skills", data.SkillName, "SKILL.md"), tpl: "skill.docs-only.md.tmpl"},
			{path: filepath.Join("skills", data.SkillName, "references", ".keep"), tpl: ""},
		}
	default:
		return fmt.Errorf("unknown skill template %q", templateName)
	}
	for _, file := range files {
		if err := writeOne(root, file.path, file.tpl, data, force); err != nil {
			return err
		}
	}
	return nil
}

func writeOne(root, relPath, templateName string, data TemplateData, force bool) error {
	full := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(full); err == nil && !force {
		return fmt.Errorf("refusing to overwrite existing file %s (use --force)", relPath)
	}
	if templateName == "" {
		return os.WriteFile(full, nil, 0o644)
	}
	body, err := RenderTemplate(templateName, data)
	if err != nil {
		return err
	}
	return os.WriteFile(full, body, 0o644)
}
