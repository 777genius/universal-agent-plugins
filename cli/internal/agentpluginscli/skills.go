package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

type skillsAddOptions struct {
	names         []string
	folder, ref   string
	ui, noUI, yes bool
}

func newSkillsCommand(app App, opts *options) *cobra.Command {
	root := &cobra.Command{Use: "skills", Short: "Install raw Agent Skills from GitHub or HTTPS publisher indexes"}
	selection := &skillsAddOptions{}
	add := &cobra.Command{Use: "add <owner/repository or HTTPS URL>", Short: "Select and install raw Skills through the saved environment installer", Long: "Select Skills from a public GitHub repository, folder or SKILL.md URL.\nHTTPS publishers expose a complete .well-known Skills index; use --skill with an exact name.\n\nExplicit --skill, --path, --target and exact file/folder URLs use terminal review.\n--ui opens interactive selection and cannot combine with --skill, --target or --dry-run.\nJSON and non-interactive use require explicit selections, targets and --yes (except --dry-run).\nTerminal selection: numbers toggle Skills, /query searches, / clears the filter,\nclear removes all selections, done reviews, q or Enter cancels.", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return selection.run(cmd, app, opts, args[0]) }}
	add.Flags().StringSliceVar(&selection.names, "skill", nil, "exact Skill name (repeatable; ambiguity requires --path)")
	add.Flags().StringVar(&selection.folder, "path", "", "exact repository-relative Skill folder (empty selects repository root)")
	add.Flags().StringVar(&selection.ref, "ref", "", "exact Git ref when a URL boundary is ambiguous")
	add.Flags().BoolVar(&selection.yes, "yes", false, "accept the reviewed terminal plan without prompting")
	add.Flags().BoolVar(&selection.ui, "ui", false, "open the bundled browser picker (interactive terminal only)")
	add.Flags().BoolVar(&selection.noUI, "no-ui", false, "select and review Skills in the terminal")
	root.AddCommand(add)
	return root
}
func (selection *skillsAddOptions) validate(opts *options, interactive bool) error {
	if err := validateCommonOptions(opts); err != nil {
		return err
	}
	if selection.ui && selection.noUI {
		return errors.New("--ui and --no-ui cannot be combined")
	}
	if selection.ui && (len(selection.names) > 0 || opts.target != "" || opts.dryRun) {
		return errors.New("--ui selects Skills and targets interactively; omit --skill, --target and --dry-run, or use --no-ui for explicit flags")
	}
	if selection.ui && !interactive {
		return errors.New("--ui requires an interactive terminal; use --no-ui with explicit selections and --target")
	}
	seen := map[string]bool{}
	for _, name := range selection.names {
		if name == "" || seen[name] {
			return skillsSelectionError{Code: "INVALID_SELECTION", Message: "--skill requires unique non-empty exact names", Candidates: []skillspicker.Candidate{}}
		}
		seen[name] = true
	}
	return nil
}
func (selection *skillsAddOptions) run(cmd *cobra.Command, app App, opts *options, input string) error {
	interactive := app.Terminal && opts.format != "json"
	if err := selection.validate(opts, interactive); err != nil {
		return renderSkillsCommandIssue(cmd, opts, err)
	}
	if err := cmd.Context().Err(); err != nil {
		return err
	}
	if isWebsiteSkillInput(input) {
		return selection.runWebsite(cmd, app, opts, input, interactive)
	}
	request := SkillsPickerRequest{Input: input, Ref: selection.ref, Path: selection.folder, PathSet: cmd.Flags().Changed("path")}
	if request.PathSet {
		if err := sourceacquisition.ValidateSkillPath(request.Path); err != nil {
			return skillsMissingSelection(cmd, opts, "INVALID_PATH", "--path must be an exact repository-relative Skill folder", []skillspicker.Candidate{})
		}
	}
	if interactive {
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Resolving Skills source…"); err != nil {
			return err
		}
	}
	session, err := app.openSkillsCatalog(cmd.Context(), request)
	if err != nil {
		return err
	}
	defer session.cleanup()
	browser := selection.ui || interactive && !selection.noUI && !opts.dryRun && len(selection.names) == 0 && !request.PathSet && !session.source.ExactPath && opts.target == ""
	if browser {
		handled, err := runSkillsBrowser(cmd, app, opts, session, selection.ui)
		if handled || err != nil {
			return err
		}
	}
	return runSkillsTerminal(cmd, app, opts, session, selection.names, interactive, selection.yes)
}
func runSkillsBrowser(cmd *cobra.Command, app App, opts *options, session *skillsCatalogSession, explicit bool) (bool, error) {
	if !skillspicker.Bundled() {
		if explicit {
			return true, errors.New("browser picker assets are not bundled in this development binary; use --no-ui")
		}
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Browser picker is not bundled; continuing with --no-ui terminal selection.")
		return false, nil
	}
	server, err := session.startPicker(cmd.Context())
	if err != nil {
		return true, err
	}
	defer server.Close()
	if err := openSkillsBrowser(cmd.Context(), server.URL()); err == nil {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Skills picker opened. Keep this command running while reviewing and installing.")
		return true, waitForSkillsPicker(cmd.Context(), server)
	}
	// Display a capability URL only for the human opening this local tab, never
	// in structured output or provider logs. Keep it usable until fallback is chosen.
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Browser could not open. Picker URL: %s\n", server.URL())
	prompted, err := withPrompter(cmd, app, opts)
	if err != nil {
		return true, err
	}
	answer, err := prompted.Prompter.Confirm(cmd.Context(), prompt.ConfirmationRequest{Title: "Continue with terminal selection?", Default: true})
	if err != nil {
		return true, err
	}
	if !answer.Accepted {
		return true, waitForSkillsPicker(cmd.Context(), server)
	}
	return false, nil
}
func waitForSkillsPicker(ctx context.Context, server *skillspicker.Server) error {
	select {
	case <-server.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *skillsCatalogSession) startPicker(ctx context.Context) (*skillspicker.Server, error) {
	revision := s.source.Commit
	return skillspicker.Start(ctx, skillspicker.Options{Source: skillspicker.Source{Repository: s.source.Repository, SourceRevision: &revision, Scope: s.source.RequestedPath}, Callbacks: s.callbacks()})
}
func openSkillsBrowser(ctx context.Context, url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
		args = []string{url}
	case "windows":
		name = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		name = "xdg-open"
		args = []string{url}
	}
	return exec.CommandContext(ctx, name, args...).Run()
}
