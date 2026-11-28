package agentpluginscli

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func (selection *skillsAddOptions) localSelections(cmd *cobra.Command, app App, opts *options, input string, interactive bool, targets []string) ([]SkillsSelection, error) {
	root, err := localSkillsRoot(input, selection.folder, cmd.Flags().Changed("path"))
	if err != nil {
		return nil, err
	}
	// A local source is already identified; do not inspect its catalog before
	// the machine/user/project policy admits the requested source kind.
	if app.InstallationPolicy != nil {
		if err := app.checkEnterpriseRequirement(cmd.Context(), environment.Requirement{
			Local: &environment.LocalSource{Path: filepath.ToSlash(root)},
		}, targets); err != nil {
			return nil, err
		}
	}
	if !selection.all && !cmd.Flags().Changed("path") {
		saved, err := savedLocalSkillRoot(app, root)
		if err != nil {
			return nil, err
		}
		if saved {
			return selection.singleLocalSelection(root, opts)
		}
	}
	catalog, err := skillsource.ScanLocal(cmd.Context(), root, skillsource.ScanPolicy{})
	// Re-adding one saved local Skill keeps its immutable pin even after its
	// original directory disappears. New sources still fail in acquisition.
	if !selection.all && os.IsNotExist(err) && !cmd.Flags().Changed("path") {
		return selection.singleLocalSelection(root, opts)
	}
	if err != nil {
		return nil, err
	}
	// Preserve the original single-directory route, including its validation and
	// saved-pin semantics. An invalid root never falls back to descendants.
	if !selection.all && len(catalog.Candidates) == 1 && catalog.Candidates[0].Path == "" {
		return selection.singleLocalSelection(root, opts)
	}
	wire, candidates := localSkillsSelectionCatalog(catalog)
	selected, err := selectSkillCandidates(cmd, app, opts, wire, candidates, selection.names, selection.all, interactive)
	if err != nil {
		return nil, err
	}
	result := make([]SkillsSelection, 0, len(selected))
	for _, candidate := range selected {
		selectedPath, err := skillsource.LocalSkillPath(root, candidate.Path)
		if err != nil {
			return nil, err
		}
		result = append(result, SkillsSelection{LocalPath: filepath.ToSlash(selectedPath), Name: candidate.Name, CandidateID: candidate.ID})
	}
	return result, nil
}

// Saved intent is a selection hint only. PrepareSkills reloads and validates the
// environment under its existing transaction before reusing an immutable pin.
func savedLocalSkillRoot(app App, root string) (bool, error) {
	manifest, _ := app.dependencyPaths()
	requirements, err := readDependencyManifest(manifest)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, requirement := range requirements.Dependencies {
		if requirement.Local != nil && requirement.Local.Path == filepath.ToSlash(root) && requirement.Local.Skill == filepath.Base(root) {
			return true, nil
		}
	}
	return false, nil
}

func (selection *skillsAddOptions) singleLocalSelection(root string, opts *options) ([]SkillsSelection, error) {
	name := filepath.Base(root)
	if len(selection.names) > 1 || len(selection.names) == 1 && selection.names[0] != name {
		return nil, errors.New("--skill must match the selected local Skill directory name")
	}
	return []SkillsSelection{{LocalPath: filepath.ToSlash(root), Name: name, Alias: opts.alias}}, nil
}

func localSkillsRoot(input, folder string, pathSet bool) (string, error) {
	root, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	if err := sourcespec.ValidateLocalSkillPath(filepath.ToSlash(root)); err != nil {
		return "", err
	}
	if pathSet {
		return skillsource.LocalSkillPath(root, folder)
	}
	return root, nil
}

func localSkillsSelectionCatalog(catalog skillsource.Catalog) (skillspicker.Catalog, map[string]skillsource.Candidate) {
	wire := skillspicker.Catalog{
		CatalogDigest: catalog.Digest, Repository: catalog.Repository, Scope: catalog.ScopePath,
		Complete: catalog.Complete, Candidates: []skillspicker.Candidate{}, Diagnostics: []skillspicker.Diagnostic{},
	}
	candidates := make(map[string]skillsource.Candidate, len(catalog.Candidates))
	for _, candidate := range catalog.Candidates {
		candidates[candidate.ID] = candidate
		wire.Candidates = append(wire.Candidates, wireSkillCandidate(candidate))
	}
	for _, reason := range catalog.IncompleteReasons {
		wire.Diagnostics = append(wire.Diagnostics, skillspicker.Diagnostic{
			Code: reason, Severity: skillspicker.Warning, Message: "Local catalog is incomplete; select a smaller directory or an exact Skill with --path.",
		})
	}
	return wire, candidates
}

func listLocalSkills(cmd *cobra.Command, app App, opts *options, input, ref, folder string, pathSet bool) error {
	if ref != "" {
		return CLIUsageError{Code: "INVALID_SELECTION", Message: "local Skill listings do not accept --ref"}
	}
	if err := app.checkSkillsContext(nil); err != nil {
		return err
	}
	root, err := localSkillsRoot(input, folder, pathSet)
	if err != nil {
		return err
	}
	catalog, err := skillsource.ScanLocal(cmd.Context(), root, skillsource.ScanPolicy{})
	if err != nil {
		return err
	}
	wire, _ := localSkillsSelectionCatalog(catalog)
	report := skillsListReport{
		Scope: string(app.scope()), Source: catalog.Repository, SourcePath: folder,
		Complete: catalog.Complete, Candidates: []listedSkill{}, Diagnostics: wire.Diagnostics,
	}
	for _, candidate := range catalog.Candidates {
		report.Candidates = append(report.Candidates, listedSkill{
			Name: candidate.Name, Description: candidate.Description, Path: candidate.Path,
			Status: candidate.Status, DiagnosticCodes: append([]string{}, candidate.Diagnostics...),
		})
	}
	return renderSkillsList(cmd, opts, report)
}
