package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type listedSkill struct {
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	Path            string   `json:"path"`
	Status          string   `json:"status"`
	Reason          string   `json:"reason,omitempty"`
	DiagnosticCodes []string `json:"diagnostic_codes,omitempty"`
}

type skillsListReport struct {
	Scope                string                    `json:"scope"`
	ScopeSelectionReason string                    `json:"scope_selection_reason"`
	Source               string                    `json:"source"`
	Revision             string                    `json:"revision"`
	SourcePath           string                    `json:"source_path,omitempty"`
	Complete             bool                      `json:"complete"`
	Candidates           []listedSkill             `json:"candidates"`
	Diagnostics          []skillspicker.Diagnostic `json:"diagnostics"`
}

func newSkillsListCommand(app App, opts *options) *cobra.Command {
	var ref, folder string
	command := &cobra.Command{
		Use:   "list <owner/repository or GitHub/HTTPS source>",
		Short: "Read-only, bounded discovery of source Skills for scripts and agents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			if opts.target != "" {
				return CLIUsageError{Code: "TARGET_NOT_ALLOWED", Message: "skills list does not install anything; omit --target"}
			}
			scoped, err := app.forOptions(opts)
			if err != nil {
				return err
			}
			return runSkillsList(cmd, scoped, opts, args[0], ref, folder, cmd.Flags().Changed("path"))
		},
	}
	command.Flags().StringVar(&ref, "ref", "", "exact Git ref if the source URL is ambiguous")
	command.Flags().StringVar(&folder, "path", "", "exact repository-relative folder to inspect")
	return command
}

func runSkillsList(cmd *cobra.Command, app App, opts *options, input, ref, folder string, pathSet bool) error {
	if isWebsiteSkillInput(input) {
		if ref != "" || pathSet {
			return CLIUsageError{Code: "INVALID_SELECTION", Message: "HTTPS publisher listings do not accept --ref or --path"}
		}
		return listHTTPSPublisherSkills(cmd, app, opts, input)
	}
	if pathSet {
		if err := sourceacquisition.ValidateSkillPath(folder); err != nil {
			return fmt.Errorf("invalid Skill folder: %w", err)
		}
	}
	session, err := app.openSkillsCatalog(cmd.Context(), SkillsPickerRequest{
		Input: input, Ref: ref, Path: folder, PathSet: pathSet,
	})
	if err != nil {
		return err
	}
	defer session.cleanup()
	catalog, err := session.scan(cmd.Context(), session.source.RequestedPath)
	if err != nil {
		return err
	}
	report := skillsListReport{
		Scope: string(app.scope()), Source: catalog.Repository, Revision: catalog.SourceRevision,
		SourcePath: catalog.Scope, Complete: catalog.Complete,
		Candidates: []listedSkill{}, Diagnostics: catalog.Diagnostics,
	}
	for _, c := range catalog.Candidates {
		report.Candidates = append(report.Candidates, listedSkill{
			Name: c.Name, Description: c.Description, Path: c.Path,
			Status: string(c.Status), DiagnosticCodes: c.DiagnosticCodes,
		})
	}
	return renderSkillsList(cmd, opts, report)
}

func listHTTPSPublisherSkills(cmd *cobra.Command, app App, opts *options, input string) error {
	if _, err := domain.NormalizeWebSkillURL(input); err != nil {
		return CLIUsageError{Code: "INVALID_SOURCE", Message: "HTTPS Skills source must be a credential-free HTTPS URL without query or fragment"}
	}
	if err := app.checkSkillsContext(nil); err != nil {
		return httpsSkillsContextFailure(err)
	}
	lister, ok := app.SourceAcquirer.(interface {
		ListHTTPSkills(context.Context, string) (sourceacquisition.HTTPSkillCatalog, error)
	})
	if !ok {
		return CLIUsageError{Code: "SOURCE_UNAVAILABLE", Message: "HTTPS publisher catalog is unavailable"}
	}
	index, err := lister.ListHTTPSkills(cmd.Context(), input)
	if err != nil {
		return httpsSkillsListFailure(err)
	}
	report := skillsListReport{
		Scope: string(app.scope()), Source: index.URL, Revision: "sha256:" + index.IndexSHA256,
		Complete: true, Candidates: []listedSkill{}, Diagnostics: []skillspicker.Diagnostic{},
	}
	for _, c := range index.Candidates {
		status := "unavailable"
		if c.Available {
			status = "available"
		}
		report.Candidates = append(report.Candidates, listedSkill{
			Name: c.Name, Description: c.Description, Path: c.Name, Status: status, Reason: c.Reason,
		})
	}
	return renderSkillsList(cmd, opts, report)
}

// httpsSkillsContextFailure does not mislabel local project failures as
// publisher outages. Keep the already typed local errors and hide all others.
func httpsSkillsContextFailure(err error) CLIUsageError {
	var usage CLIUsageError
	if errors.As(err, &usage) {
		return usage
	}
	return CLIUsageError{
		Code:    "COMMAND_FAILED",
		Message: "Local Skills context cannot be validated; inspect --scope and --project-root before trying the publisher",
	}
}

// httpsSkillsListFailure discards untrusted transport diagnostics while keeping
// cancellation and deadline failures machine-readable.
func httpsSkillsListFailure(err error) CLIUsageError {
	var usage CLIUsageError
	if errors.As(err, &usage) {
		return usage
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return CLIUsageError{Code: "TIMEOUT", Message: "HTTPS publisher discovery timed out; check publisher availability before retrying"}
	case errors.Is(err, context.Canceled):
		return CLIUsageError{Code: "CANCELED", Message: "HTTPS publisher discovery was canceled"}
	default:
		return CLIUsageError{Code: "SOURCE_UNAVAILABLE", Message: "HTTPS publisher catalog could not be read or validated"}
	}
}

func renderSkillsList(cmd *cobra.Command, opts *options, report skillsListReport) error {
	report.ScopeSelectionReason = scopeSelectionReason(cmd, opts)
	sort.Slice(report.Candidates, func(i, j int) bool {
		if report.Candidates[i].Path == report.Candidates[j].Path {
			return report.Candidates[i].Name < report.Candidates[j].Name
		}
		return report.Candidates[i].Path < report.Candidates[j].Path
	})
	if opts.format == "json" {
		return writeJSONOutput(cmd.OutOrStdout(), "skills list", report)
	}
	complete := "complete"
	if !report.Complete {
		complete = "incomplete; narrow --path to inspect a folder"
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Skills in %s (%s; %s)\n",
		prompt.SafeText(report.Source), prompt.SafeText(report.Revision), complete); err != nil {
		return err
	}
	for _, c := range report.Candidates {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s (%s)\n",
			prompt.SafeText(c.Path), prompt.SafeText(c.Name), prompt.SafeText(c.Status)); err != nil {
			return err
		}
	}
	for _, d := range report.Diagnostics {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n",
			prompt.SafeText(d.Code), prompt.SafeText(d.Message)); err != nil {
			return err
		}
	}
	if len(report.Candidates) == 0 && len(report.Diagnostics) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "No Skills found in the selected source scope.")
		return err
	}
	return nil
}
