package app

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/adapters/filesystem"
	"github.com/777genius/plugin-kit-ai/cli/internal/skills/adapters/generate"
	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

type renderer interface {
	Generate(name string, doc domain.SkillDocument) ([]domain.Artifact, error)
	Target() string
}

func (s Service) Generate(opts RenderOptions) (RenderResult, error) {
	names, err := s.Repo.Discover(opts.Root)
	if err != nil {
		return RenderResult{}, err
	}
	renderers, selectedTargets, err := renderTargets(opts.Target)
	if err != nil {
		return RenderResult{}, err
	}
	docs := make(map[string]domain.SkillDocument, len(names))
	var failures []ValidationFailure
	for _, name := range names {
		doc, err := s.Repo.LoadSkill(opts.Root, name)
		if err != nil {
			failures = append(failures, ValidationFailure{Path: filepath.Join("skills", name, "SKILL.md"), Message: err.Error()})
			continue
		}
		docs[name] = doc
		failures = append(failures, validateDoc(opts.Root, name, doc)...)
	}
	if len(failures) > 0 {
		return RenderResult{}, formatValidationError("cannot generate invalid skills", failures)
	}
	var out []domain.Artifact
	managed := make(map[string]struct{})
	existingManaged, err := s.Repo.ListManagedArtifacts(opts.Root, selectedTargets)
	if err != nil {
		return RenderResult{}, err
	}
	for _, path := range existingManaged {
		managed[path] = struct{}{}
	}
	for _, name := range names {
		doc := docs[name]
		supportedRenderers := renderersForSkill(doc.Spec, renderers)
		if len(supportedRenderers) == 0 {
			for path := range managedPathsForSkill(name, selectedTargets) {
				managed[path] = struct{}{}
			}
			continue
		}
		for _, r := range supportedRenderers {
			artifacts, err := r.Generate(name, doc)
			if err != nil {
				return RenderResult{}, err
			}
			out = append(out, artifacts...)
		}
		if doc.Spec.ExecutionMode == domain.ExecutionCommand {
			cmdBody, err := filesystem.RenderTemplate("command.md.tmpl", filesystem.TemplateData{
				SkillName:            name,
				Description:          doc.Spec.Description,
				CommandLine:          filesystem.CommandLine(doc.Spec),
				Runtime:              string(doc.Spec.Runtime),
				AllowedTools:         doc.Spec.AllowedTools,
				CompatibilitySummary: compatibilitySummary(doc.Spec.Compatibility),
				ExecutionNotes:       executionNotes(doc.Spec),
			})
			if err != nil {
				return RenderResult{}, err
			}
			out = append(out, domain.Artifact{
				RelPath: filepath.Join("commands", name+".md"),
				Content: cmdBody,
			})
		}
		for path := range managedPathsForSkill(name, selectedTargets) {
			managed[path] = struct{}{}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	keep := make(map[string]struct{}, len(out))
	for _, artifact := range out {
		keep[artifact.RelPath] = struct{}{}
	}
	var stale []string
	for path := range managed {
		if _, ok := keep[path]; !ok {
			stale = append(stale, path)
		}
	}
	sort.Strings(stale)
	return RenderResult{Artifacts: out, StalePaths: stale}, nil
}

func renderTargets(target string) ([]renderer, map[string]struct{}, error) {
	selectedTargets := make(map[string]struct{})
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "", "all":
		selectedTargets["claude"] = struct{}{}
		selectedTargets["codex"] = struct{}{}
		return []renderer{generate.ClaudeRenderer{}, generate.CodexRenderer{}}, selectedTargets, nil
	case "claude":
		selectedTargets["claude"] = struct{}{}
		return []renderer{generate.ClaudeRenderer{}}, selectedTargets, nil
	case "codex":
		selectedTargets["codex"] = struct{}{}
		return []renderer{generate.CodexRenderer{}}, selectedTargets, nil
	default:
		return nil, nil, fmt.Errorf("unknown generate target %q", target)
	}
}

func renderersForSkill(spec domain.SkillSpec, candidates []renderer) []renderer {
	allowed := make(map[string]struct{}, len(spec.SupportedAgents))
	for _, agent := range spec.SupportedAgents {
		allowed[string(agent)] = struct{}{}
	}
	out := make([]renderer, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := allowed[candidate.Target()]; ok {
			out = append(out, candidate)
		}
	}
	return out
}

func managedPathsForSkill(name string, selectedTargets map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{})
	if _, ok := selectedTargets["claude"]; ok {
		out[filepath.Join("generated", "skills", "claude", name, "SKILL.md")] = struct{}{}
	}
	if _, ok := selectedTargets["codex"]; ok {
		out[filepath.Join("generated", "skills", "codex", name, "SKILL.md")] = struct{}{}
	}
	out[filepath.Join("commands", name+".md")] = struct{}{}
	return out
}
