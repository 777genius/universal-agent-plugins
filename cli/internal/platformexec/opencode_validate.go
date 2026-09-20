package platformexec

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	skillfs "github.com/777genius/plugin-kit-ai/cli/internal/skills/adapters/filesystem"
	skillsapp "github.com/777genius/plugin-kit-ai/cli/internal/skills/app"
)

func (opencodeAdapter) Validate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	meta, _, err := readYAMLDoc[opencodePackageMeta](root, state.DocPath("package_metadata"))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", state.DocPath("package_metadata"), err)
	}
	if err := validateOpenCodePluginRefs(meta.Plugins); err != nil {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     state.DocPath("package_metadata"),
			Target:   "opencode",
			Message:  "OpenCode package metadata " + err.Error(),
		})
	}
	configPath, warnings, ok, err := resolveOpenCodeConfigPath(root)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     "opencode.json",
			Target:   "opencode",
			Message:  "OpenCode config opencode.json or opencode.jsonc is required",
		}}, nil
	}
	for _, warning := range warnings {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityWarning,
			Code:     CodeManifestInvalid,
			Path:     warning.Path,
			Target:   "opencode",
			Message:  warning.Message,
		})
	}
	doc, configDiagnostics := validateOpenCodeConfigDoc(root, configPath)
	if doc == nil && len(configDiagnostics) > 0 {
		return configDiagnostics, nil
	}
	diagnostics = append(diagnostics, configDiagnostics...)
	if len(graph.Portable.Paths("skills")) > 0 {
		authoredRootRel := pluginmodel.SourceDirName
		authoredRoot := filepath.Join(root, authoredRootRel)
		report, err := (skillsapp.Service{Repo: skillfs.Repository{}}).Validate(skillsapp.ValidateOptions{Root: authoredRoot})
		if err != nil {
			return nil, err
		}
		for _, failure := range report.Failures {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     filepath.ToSlash(filepath.Join(authoredRootRel, failure.Path)),
				Target:   "opencode",
				Message:  "OpenCode mirrored skill is incompatible with the shared SKILL.md contract: " + failure.Message,
			})
		}
	}
	diagnostics = append(diagnostics, validateOpenCodeCommandFiles(root, state.ComponentPaths("commands"))...)
	diagnostics = append(diagnostics, validateOpenCodeAgentFiles(root, state.ComponentPaths("agents"))...)
	diagnostics = append(diagnostics, validateOpenCodeDefaultAgent(root, state.DocPath("default_agent"))...)
	diagnostics = append(diagnostics, validateOpenCodeInstructions(root, state.DocPath("instructions_config"))...)
	diagnostics = append(diagnostics, validateOpenCodePermission(root, state.DocPath("permission_config"))...)
	diagnostics = append(diagnostics, validateOpenCodeThemeFiles(root, state.ComponentPaths("themes"))...)
	packageDoc, packageDiagnostics := validateOpenCodePluginPackageJSON(root, state.DocPath("local_plugin_dependencies"))
	diagnostics = append(diagnostics, packageDiagnostics...)
	diagnostics = append(diagnostics, validateOpenCodeToolFiles(root, state.ComponentPaths("tools"), packageDoc)...)
	diagnostics = append(diagnostics, validateOpenCodePluginFiles(root, state.ComponentPaths("local_plugin_code"), packageDoc)...)
	return diagnostics, nil
}
