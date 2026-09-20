package scaffold

import (
	"fmt"
	"strings"
)

func buildJobTemplatePlan(d Data) (ProjectPlan, error) {
	templateName := NormalizeTemplate(d.JobTemplate)
	targets, err := normalizeJobTemplateTargets(templateName, d.EffectiveTargets())
	if err != nil {
		return ProjectPlan{}, err
	}
	d.Targets = targets
	if len(targets) == 1 {
		d.Platform = targets[0]
	} else {
		d.Platform = ""
	}
	if d.WithExtras {
		d.HasSkills = true
	}
	return ProjectPlan{
		Platform: strings.Join(targets, ","),
		Data:     d,
		Files:    expandTemplateFiles(jobTemplateFilesFor(templateName, d.WithExtras), d),
	}, nil
}

func normalizeJobTemplateTargets(templateName string, targets []string) ([]string, error) {
	if len(targets) == 0 {
		targets = DefaultJobTemplateTargets(templateName)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("template %q requires at least one target", templateName)
	}
	normalized := append([]string(nil), targets...)
	for i, target := range normalized {
		p, ok := LookupPlatform(target)
		if !ok {
			return nil, fmt.Errorf("unknown platform %q", target)
		}
		if p.Name == "codex-runtime" {
			return nil, fmt.Errorf("template %q does not support --platform %s", templateName, p.Name)
		}
		normalized[i] = p.Name
	}
	return normalized, nil
}

func jobTemplateFilesFor(templateName string, extras bool) []TemplateFile {
	files := []TemplateFile{
		{Path: authoredPath("plugin.yaml"), Template: "plugin.yaml.tmpl", Extra: false},
		{Path: authoredPath("mcp/servers.yaml"), Template: "job.online-service.mcp.servers.yaml.tmpl", Extra: false},
		{Path: authoredPath("README.md"), Template: "job.online-service.README.md.tmpl", Extra: false},
		{Path: "CLAUDE.md", Template: "ROOT.CLAUDE.md.tmpl", Extra: false},
		{Path: "AGENTS.md", Template: "ROOT.AGENTS.md.tmpl", Extra: false},
	}
	switch templateName {
	case InitTemplateLocalTool:
		files[1].Template = "job.local-tool.mcp.servers.yaml.tmpl"
		files[2].Template = "job.local-tool.README.md.tmpl"
	}
	if extras {
		files = append(files, TemplateFile{Path: authoredPath("skills/{{.ProjectName}}/SKILL.md"), Template: "SKILL.md.tmpl", Extra: true})
	}
	return files
}
