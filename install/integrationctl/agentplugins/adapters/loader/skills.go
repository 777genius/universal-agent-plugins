package loader

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func loadSkills(root string) (map[string]domain.Skill, []string, bool, []domain.Diagnostic) {
	skills := map[string]domain.Skill{}
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return skills, nil, false, nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		diagnostic := skillDiagnostic("skills_root_invalid", "", "skills must be a real directory", err)
		return skills, nil, true, []domain.Diagnostic{diagnostic}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		diagnostic := skillDiagnostic("skills_read_failed", "", "read skills directory", err)
		return skills, nil, true, []domain.Diagnostic{diagnostic}
	}
	var invalid []string
	var diagnostics []domain.Diagnostic
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			candidate := filepath.Join(root, entry.Name())
			resolved, resolveErr := filepath.EvalSymlinks(candidate)
			if resolveErr != nil || !pathContained(filepath.Dir(root), resolved) {
				invalid = append(invalid, entry.Name())
				diagnostics = append(diagnostics, skillDiagnostic("skill_path_unsafe", entry.Name(), "skill entry resolves outside the plugin root", resolveErr))
			}
			continue
		}
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		skillPath := filepath.Join(root, name, "SKILL.md")
		skillInfo, statErr := os.Lstat(skillPath)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			invalid = append(invalid, name)
			diagnostics = append(diagnostics, skillDiagnostic("skill_read_failed", name, "inspect immediate SKILL.md", statErr))
			continue
		}
		if skillInfo.Mode()&os.ModeSymlink != 0 {
			resolved, resolveErr := filepath.EvalSymlinks(skillPath)
			if resolveErr != nil || !pathContained(filepath.Dir(root), resolved) {
				invalid = append(invalid, name)
				diagnostics = append(diagnostics, skillDiagnostic("skill_path_unsafe", name, "SKILL.md resolves outside the plugin root", resolveErr))
			}
			continue
		}
		if !skillInfo.Mode().IsRegular() {
			continue
		}
		body, readErr := os.ReadFile(skillPath)
		if readErr != nil {
			invalid = append(invalid, name)
			diagnostics = append(diagnostics, skillDiagnostic("skill_read_failed", name, "read immediate SKILL.md", readErr))
			continue
		}
		skill, parseErr := conformance.ParseInstallerSkill(name, body)
		if parseErr != nil {
			invalid = append(invalid, name)
			diagnostics = append(diagnostics, skillDiagnostic("skill_invalid", name, "skill was skipped because SKILL.md is invalid", parseErr))
			continue
		}
		skills[name] = skill
	}
	sort.Strings(invalid)
	return skills, invalid, false, diagnostics
}

func pathContained(root, candidate string) bool {
	if root == "" || candidate == "" {
		return false
	}
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func skillDiagnostic(code, name, message string, cause error) domain.Diagnostic {
	if cause != nil {
		message += ": " + cause.Error()
	}
	path := "skills"
	if name != "" {
		path = filepath.ToSlash(filepath.Join("skills", name, "SKILL.md"))
	}
	return domain.Diagnostic{
		Severity: domain.SeverityError,
		Boundary: domain.BoundarySkill,
		Code:     code,
		Path:     path,
		Item:     name,
		Message:  message,
	}
}
