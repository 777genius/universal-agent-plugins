package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func newRebindCommand(app App, opts *options) *cobra.Command {
	return newBindingChangeCommand(app, opts, usecase.BindingChangeRebind)
}

func newMigrateFormatCommand(app App, opts *options) *cobra.Command {
	return newBindingChangeCommand(app, opts, usecase.BindingChangeMigrateFormat)
}

func newBindingChangeCommand(app App, opts *options, mode usecase.BindingChangeMode) *cobra.Command {
	commandName := "rebind"
	short := "Review and change the source bound to a removed Agent Plugin"
	if mode == usecase.BindingChangeMigrateFormat {
		commandName = "migrate-format"
		short = "Review and migrate a removed legacy binding to Agent Plugins 1.0"
	}
	return &cobra.Command{
		Use:   commandName + " <name-or-installation-id> <new-source>",
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			return runBindingChange(cmd.Context(), cmd, app, opts, mode, args[0], args[1])
		},
	}
}

type bindingChangeSession struct {
	ctx      context.Context
	cmd      *cobra.Command
	app      App
	opts     *options
	mode     usecase.BindingChangeMode
	selector string
	source   string
	loaded   loadedPackage
	service  usecase.Service
	planned  usecase.BindingChangeResult
}

func runBindingChange(
	ctx context.Context,
	cmd *cobra.Command,
	app App,
	opts *options,
	mode usecase.BindingChangeMode,
	selector, source string,
) error {
	session := &bindingChangeSession{
		ctx: ctx, cmd: cmd, app: app, opts: opts, mode: mode, selector: selector, source: source,
	}
	if err := session.load(); err != nil {
		return err
	}
	if session.loaded.cleanup != nil {
		defer func() { _ = session.loaded.cleanup() }()
	}
	if err := session.plan(); err != nil {
		return err
	}
	if opts.dryRun || session.planned.NoChange {
		return renderBindingChange(cmd.OutOrStdout(), opts.format, session.commandName(), session.planned, opts.dryRun)
	}
	if !session.planned.Plan.CanApply {
		return session.blocked()
	}
	confirmed, err := session.confirm()
	if err != nil {
		return err
	}
	if !confirmed {
		return session.canceled()
	}
	return session.apply()
}

func (session *bindingChangeSession) commandName() string {
	return bindingCommandName(session.mode)
}

func (session *bindingChangeSession) load() error {
	writeProgress(session.app, session.opts.format, "Resolving and validating the proposed Agent Plugin binding...")
	loaded, err := session.app.loadPackage(session.ctx, session.source)
	if err != nil {
		return err
	}
	session.loaded = loaded
	return nil
}

func (session *bindingChangeSession) plan() error {
	session.service = session.app.Lifecycle
	session.service.StateStore = session.app.StateStore
	input := usecase.BindingChangeInput{Selector: session.selector, Envelope: session.loaded.envelope}
	planned, err := executeBindingChange(session.ctx, session.service, session.mode, input)
	if err != nil {
		return err
	}
	session.planned = planned
	return nil
}

func (session *bindingChangeSession) blocked() error {
	if renderErr := renderBindingChange(session.cmd.OutOrStdout(), session.opts.format, session.commandName(), session.planned, false); renderErr != nil {
		return renderErr
	}
	return fmt.Errorf("binding change is blocked; remove all listed targets first")
}

func (session *bindingChangeSession) confirm() (bool, error) {
	confirmed := mutationConfirmed(session.app, session.opts)
	writer := session.cmd.OutOrStdout()
	if session.opts.format == "human" {
		if !confirmed && session.app.Terminal {
			var err error
			writer, err = promptio.VisibleOutput(writer, session.cmd.ErrOrStderr())
			if err != nil {
				return false, err
			}
		}
		if err := renderHumanBindingPlan(writer, session.planned.Plan); err != nil {
			return false, err
		}
	}
	if !confirmed && session.opts.format == "human" && session.app.Terminal {
		var err error
		confirmed, err = promptYesNo(session.cmd.Context(), session.cmd.InOrStdin(), writer, writer, "Apply this binding change? [y/N]")
		if err != nil {
			return false, err
		}
	}
	return confirmed, nil
}

func (session *bindingChangeSession) canceled() error {
	if session.opts.format == "json" {
		return renderBindingChange(session.cmd.OutOrStdout(), session.opts.format, session.commandName(), session.planned, false)
	}
	_, _ = fmt.Fprintln(session.cmd.OutOrStdout(), "No changes made.")
	return nil
}

func (session *bindingChangeSession) apply() error {
	writeProgress(session.app, session.opts.format, "Committing the reviewed binding change...")
	input := usecase.BindingChangeInput{Selector: session.selector, Envelope: session.loaded.envelope, Confirmed: true}
	result, changeErr := executeBindingChange(session.ctx, session.service, session.mode, input)
	if renderErr := renderBindingChange(session.cmd.OutOrStdout(), session.opts.format, session.commandName(), result, false); renderErr != nil && changeErr == nil {
		changeErr = renderErr
	}
	return changeErr
}

func executeBindingChange(ctx context.Context, service usecase.Service, mode usecase.BindingChangeMode, input usecase.BindingChangeInput) (usecase.BindingChangeResult, error) {
	if mode == usecase.BindingChangeMigrateFormat {
		return service.MigrateFormat(ctx, input)
	}
	return service.Rebind(ctx, input)
}

func bindingCommandName(mode usecase.BindingChangeMode) string {
	if mode == usecase.BindingChangeMigrateFormat {
		return "migrate-format"
	}
	return "rebind"
}

func renderBindingChange(writer io.Writer, format, commandName string, result usecase.BindingChangeResult, dryRun bool) error {
	data := struct {
		DryRun bool                        `json:"dry_run"`
		Result usecase.BindingChangeResult `json:"result"`
	}{DryRun: dryRun, Result: result}
	if format == "json" {
		return writeJSONOutput(writer, commandName, data)
	}
	return renderHumanBindingChange(writer, result, dryRun)
}

func renderHumanBindingChange(writer io.Writer, result usecase.BindingChangeResult, dryRun bool) error {
	if dryRun || !result.Plan.CanApply {
		if err := renderHumanBindingPlan(writer, result.Plan); err != nil {
			return err
		}
	}
	if !result.Plan.CanApply {
		_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Error, "Blocked. Remove every listed target and native object first."))
		return nil
	}
	if result.NoChange {
		_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Muted, "Binding already matches. No changes made."))
		return nil
	}
	if result.Mutated {
		_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Success, "Binding updated. Reinstall targets explicitly with agentplugins add."))
	}
	return nil
}

func renderHumanBindingPlan(writer io.Writer, plan usecase.BindingChangePlan) error {
	checked := &planWriter{writer: writer}
	writer = checked
	_, _ = fmt.Fprintf(writer, "%s: %s -> %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Plugin"), prompt.SafeText(string(plan.OldName)), prompt.SafeText(string(plan.NewName)))
	_, _ = fmt.Fprintf(writer, "%s: %s -> %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Format"), prompt.SafeText(string(plan.OldFormat.FormatID)), prompt.SafeText(string(plan.NewFormat.FormatID)))
	_, _ = fmt.Fprintf(writer, "%s: %s -> %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Schema"), prompt.SafeText(string(plan.OldFormat.SchemaURI)), prompt.SafeText(string(plan.NewFormat.SchemaURI)))
	_, _ = fmt.Fprintf(writer, "%s: %s -> %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Source"), prompt.SafeText(string(provenanceLabel(plan.OldSource))), prompt.SafeText(string(provenanceLabel(plan.NewSource))))
	_, _ = fmt.Fprintf(writer, "%s: %s -> %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Components"), prompt.SafeText(string(componentInventoryLabel(plan.OldComponents))), prompt.SafeText(string(componentInventoryLabel(plan.NewComponents))))
	_, _ = fmt.Fprintf(writer, "%s: %d\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Native objects"), plan.NativeObjectCount)
	_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Label, "PLUGIN_DATA")+": not transferred")
	for _, target := range plan.Targets {
		_, _ = fmt.Fprintf(writer, "  %s/%s: %s\n", prompt.SafeText(string(target.ClientID)), prompt.SafeText(string(target.Scope)), prompt.SafeText(string(target.Decision)))
	}
	for _, blocker := range plan.Blockers {
		_, _ = fmt.Fprintf(writer, "  %s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Error, "Blocker"), prompt.SafeText(blocker))
	}
	return checked.err
}

func provenanceLabel(source usecase.ProvenanceSummary) string {
	value := source.Kind
	if source.Repository != "" {
		value = source.Repository
		if source.PackageSubpath != "" {
			value += "//" + source.PackageSubpath
		}
	}
	if source.ResolvedRevision != "" {
		value += "@" + source.ResolvedRevision
	}
	if source.TreeDigest != "" {
		value += "#" + source.TreeDigest
	}
	return value
}

func componentInventoryLabel(inventory domain.ComponentInventory) string {
	parts := make([]string, 0, 6)
	parts = append(parts, mcpInventoryLabel(inventory))
	if inventory.AppPresent {
		parts = append(parts, "apps=["+sortedList(inventory.AppBindings)+"]")
	}
	parts = append(parts, "skills=["+sortedList(inventory.Skills)+"]")
	parts = append(parts, "extensions=["+sortedList(inventory.Extensions)+"]")
	if len(inventory.InvalidMCPServer) > 0 {
		parts = append(parts, "invalid_mcp=["+sortedList(inventory.InvalidMCPServer)+"]")
	}
	if len(inventory.InvalidSkills) > 0 {
		parts = append(parts, "invalid_skills=["+sortedList(inventory.InvalidSkills)+"]")
	}
	if inventory.InvalidSkillsRoot {
		parts = append(parts, "invalid_skills_root=true")
	}
	return strings.Join(parts, " ")
}

func mcpInventoryLabel(inventory domain.ComponentInventory) string {
	if !inventory.MCPPresent {
		return "mcp=absent"
	}
	state := "disabled"
	if inventory.MCPEnabled {
		state = "enabled"
	}
	return "mcp=" + state + "[" + sortedList(inventory.MCPServers) + "]"
}

func sortedList(values []string) string {
	copyValues := append([]string(nil), values...)
	sort.Strings(copyValues)
	return strings.Join(copyValues, ",")
}
