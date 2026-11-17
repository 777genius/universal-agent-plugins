package agentpluginscli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type interruptedClientChange struct {
	Target        string `json:"target"`
	NativeAttempt string `json:"native_attempt"`
	Status        string `json:"status"`
	NextAction    string `json:"next_action"`
}

type interruptedClientChangePreview struct {
	DryRun  bool                      `json:"dry_run"`
	Plugin  string                    `json:"plugin"`
	Status  string                    `json:"status"`
	Targets []interruptedClientChange `json:"targets"`
}

// confirmInterruptedClientChanges runs before repair or removal plans
// anything. A killed run can leave a client change whose effect no intent
// records, and every command refuses that binding. When the client proves
// that no command of that run is alive and recognizes its own plugin state,
// --yes closes the attempt and the command continues. A dry run performs the
// same check and stops: nothing can be planned while the attempt is open.
func confirmInterruptedClientChanges(cmd *cobra.Command, app App, opts *options, selector, command string) (bool, error) {
	// Automated calls confirm only explicitly named targets; without one the
	// command keeps its own target checks and refusal.
	if automatedMutation(app, opts) && strings.TrimSpace(opts.target) == "" {
		return false, nil
	}
	state, installation, found := selectedInstallationState(app, selector)
	bindings := interruptedClientBindings(app, opts, state, installation)
	if !found || len(bindings) == 0 {
		return false, nil
	}
	if !opts.dryRun && !opts.yes {
		return true, CLIUsageError{Code: "CONFIRMATION_REQUIRED", Message: fmt.Sprintf("the interrupted %s change (native attempt %s) must be confirmed first; rerun with --yes, or with --dry-run to preview the check",
			publicCLITargetName(domain.ClientID(bindings[0].ClientID)), bindings[0].NativeActivationAttempt)}
	}
	detected, err := app.Detector.Detect(cmd.Context())
	if err != nil {
		return true, fmt.Errorf("detect AI clients: %w", err)
	}
	clients := detectedClientMap(detected)
	preview := interruptedClientChangePreview{DryRun: true, Plugin: installation.DeclaredName, Status: "confirmable"}
	for _, binding := range bindings {
		target := publicCLITargetName(domain.ClientID(binding.ClientID))
		if err := confirmInterruptedClientChange(cmd, app, opts, state, installation, binding, clients); err != nil {
			return true, fmt.Errorf("cannot confirm the interrupted %s change (native attempt %s): %w", target, binding.NativeActivationAttempt, err)
		}
		if !opts.dryRun {
			writeProgress(app, opts.format, fmt.Sprintf("Confirmed the interrupted %s change (native attempt %s) from the client's own plugin state.", target, binding.NativeActivationAttempt))
			continue
		}
		preview.Targets = append(preview.Targets, interruptedClientChange{Target: target, NativeAttempt: binding.NativeActivationAttempt, Status: "confirmable",
			NextAction: fmt.Sprintf("rerun %s with --yes: it confirms this change first, then continues", command)})
	}
	if !opts.dryRun {
		return false, nil
	}
	return true, renderInterruptedClientChangePreview(cmd, opts, command, preview)
}

// selectedInstallationState loads the state and the selected installation.
// The command itself reports an unreadable state or an unknown selector.
func selectedInstallationState(app App, selector string) (domain.StateFileV2, domain.Installation, bool) {
	state, err := app.StateStore.Load()
	if err != nil {
		return state, domain.Installation{}, false
	}
	installation, err := selectInstallation(state, selector)
	return state, installation, err == nil
}

// confirmInterruptedClientChange confirms one binding under the repair
// authority of its saved environment, if any.
func confirmInterruptedClientChange(cmd *cobra.Command, app App, opts *options, state domain.StateFileV2, installation domain.Installation, binding domain.ClientBinding, clients map[domain.ClientID]domain.DetectedClient) error {
	id := domain.ClientID(binding.ClientID)
	client, found := clients[id]
	if !found {
		return fmt.Errorf("the client is not detected")
	}
	authority, err := environmentAuthorityForBinding(state, installation.InstallationID, id, "repair")
	if err != nil {
		return err
	}
	_, err = lifecycleService(app, clients).ConfirmInterruptedNativeAttempt(cmd.Context(), usecase.InterruptedAttemptInput{
		InstallationID: installation.InstallationID, Client: client, Scope: app.scope(), BackendExecutable: backendExecutable(client, clients, app.scope()),
		EnvironmentAuthority: authority, DryRun: opts.dryRun, Confirmed: opts.yes,
	})
	return err
}

// interruptedClientBindings selects the bindings of the requested targets
// whose journal-free attempt the client can confirm in the current scope.
func interruptedClientBindings(app App, opts *options, state domain.StateFileV2, installation domain.Installation) []domain.ClientBinding {
	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return nil
	}
	unresolved := unresolvedNativeAttempts(app, state)
	var bindings []domain.ClientBinding
	for _, binding := range installation.Clients {
		id := domain.ClientID(binding.ClientID)
		if binding.Scope == string(app.scope()) && unresolved[binding.NativeActivationAttempt] &&
			app.Lifecycle.ConfirmsInterruptedAttempt(id, app.scope()) && (len(targets) == 0 || slices.Contains(targets, id)) {
			bindings = append(bindings, binding)
		}
	}
	slices.SortFunc(bindings, func(a, b domain.ClientBinding) int { return strings.Compare(a.ClientID, b.ClientID) })
	return bindings
}

func renderInterruptedClientChangePreview(cmd *cobra.Command, opts *options, command string, preview interruptedClientChangePreview) error {
	if opts.format == "json" {
		return writeJSONOutput(cmd.OutOrStdout(), command, preview)
	}
	var text strings.Builder
	fmt.Fprintf(&text, "Dry run: %s has an interrupted client change to confirm before %s.\n", preview.Plugin, command)
	for _, target := range preview.Targets {
		fmt.Fprintf(&text, "  %s: native attempt %s can be confirmed: no client command of that run is running and the client's own plugin state matches the recorded ownership.\n", target.Target, target.NativeAttempt)
		fmt.Fprintf(&text, "    Next: %s\n", target.NextAction)
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), text.String())
	return err
}
