package agentpluginscli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// newInstallCommand is the everyday converge verb. It installs or repairs the
// declared environment from uap.toml and uap.lock through the exact deps sync
// machinery: first runs install everything declared, intact items are left
// alone, and missing or drifted environment-owned projections are restored
// from their recorded pin before the machine is converged to the locked state.
// With a positional source it retains the historical add alias contract.
func newInstallCommand(app App, opts *options) *cobra.Command {
	var locked, yes, activationComplete, authComplete bool
	command := &cobra.Command{
		Use:   "install [name-or-source]",
		Short: "Install or repair the declared dependency environment",
		Long:  "Install/repair the declared environment from uap.toml and uap.lock: install everything declared on a first run, leave already-matching items alone, and converge missing or drifted state to the exact locked pins. With no source, a uap.toml in the current directory selects project scope (no parent search); explicit --scope wins. An existing uap.lock is validated strictly; only a missing lock is resolved and created. Use update to change pins. The preview shows the selected scope and manifest/lock paths. With a source argument, install behaves as agentplugins add for that one package.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := selectInstallScope(cmd, opts); err != nil {
					return err
				}
			}
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if len(args) == 1 {
				return executeAddCommand(cmd, app, opts, args[0], activationComplete, authComplete)
			}
			if err := validateEnvironmentOptions("install", opts); err != nil {
				return err
			}
			manifestPath, lockPath := app.dependencyPaths()
			lockedOnly, err := installLockedOnly(lockPath, locked)
			if err != nil {
				return err
			}
			opts.installSelection = &installSelection{Scope: string(app.scope()), ProjectRoot: app.ProjectRoot, ManifestPath: manifestPath, LockPath: lockPath}
			app.Lifecycle.PrepareHostsForPreview = true
			return runDependencySyncConverge(cmd, app, opts, lockedOnly, yes, func(convergeApp App) (bool, error) {
				return repairDeclaredEnvironmentDrift(cmd, convergeApp, opts, yes)
			})
		},
	}
	// Retain the add flags so the positional-source alias is flag-compatible.
	command.Flags().BoolVar(&opts.noSave, "no-save", false, "install without saving portable dependency intent")
	command.Flags().StringVar(&opts.alias, "alias", "", "saved dependency alias")
	command.Flags().StringVar(&opts.chatGPTAppID, "chatgpt-app-id", "", "personal Context7 registration ID copied from ChatGPT Developer Mode")
	command.Flags().BoolVar(&activationComplete, "activation-complete", false, "attest that manual client activation is complete")
	command.Flags().BoolVar(&authComplete, "auth-complete", false, "attest that required authentication is complete or none is required after review")
	command.Flags().BoolVar(&opts.externalUninstalled, "external-uninstalled", false, "attest removed targets were uninstalled manually or never imported")
	command.Flags().BoolVar(&locked, "locked", false, "require an exact existing lock and never resolve refs")
	command.Flags().BoolVar(&yes, "yes", false, "confirm the complete preflighted install without an interactive prompt")
	return command
}

// repairDeclaredEnvironmentDrift restores environment-owned projections that
// are missing or drifted from their recorded per-binding pin, reusing the exact
// repair machinery of the repair command. Foreign bindings and pending
// transactions keep the fail-closed sync behavior untouched. It reports whether
// the caller should continue with the sync convergence.
func repairDeclaredEnvironmentDrift(cmd *cobra.Command, app App, opts *options, yes bool) (bool, error) {
	if opts.dryRun {
		// Dry runs never mutate; sync's own fail-closed drift refusal is the
		// truthful no-changes plan for a damaged projection.
		return true, nil
	}
	view, err := app.Installer.Inspect(cmd.Context())
	if err != nil {
		return false, err
	}
	drifted := driftedEnvironmentBindings(view)
	if len(drifted) == 0 {
		return true, nil
	}
	// The repair command authorizes online host preparation for its mutation;
	// recorded-pin repairs keep that exact flow.
	app.Lifecycle.PrepareHostsForPreview = true
	actions, err := buildDeclaredEnvironmentRepairs(cmd, app, drifted)
	if err != nil {
		return false, err
	}
	if len(actions) == 0 {
		return true, nil
	}
	return true, applyDeclaredEnvironmentRepairs(cmd, app, opts, actions, yes)
}

// driftedEnvironmentBindings maps each installation ID to the clients whose
// environment-owned bindings report projection drift.
func driftedEnvironmentBindings(view installer.Inspection) map[string][]domain.ClientID {
	drifted := map[string][]domain.ClientID{}
	for _, installation := range view.Installations {
		for _, binding := range installation.Bindings {
			if binding.Owner == nil || binding.ProjectionStatus == "" {
				continue
			}
			drifted[installation.InstallationID] = append(drifted[installation.InstallationID], domain.ClientID(binding.ClientID))
		}
	}
	return drifted
}

func buildDeclaredEnvironmentRepairs(cmd *cobra.Command, app App, drifted map[string][]domain.ClientID) ([]dependencyAction, error) {
	state, err := app.StateStore.Load()
	if err != nil {
		return nil, err
	}
	var actions []dependencyAction
	for _, installation := range state.Installations {
		targets := drifted[installation.InstallationID]
		if len(targets) == 0 {
			continue
		}
		repairs, err := buildDependencyRepairs(cmd.Context(), app, state, installation, targets)
		if err != nil {
			return nil, err
		}
		actions = append(actions, repairs...)
	}
	return actions, nil
}

func applyDeclaredEnvironmentRepairs(cmd *cobra.Command, app App, opts *options, actions []dependencyAction, yes bool) error {
	detected, err := detectDependencyClients(cmd.Context(), app)
	if err != nil {
		return err
	}
	consent := map[string]bool{}
	prepared, err := preflightDependencyActions(cmd, app, opts, actions, detected, consent)
	if err != nil {
		return err
	}
	// Removal attestation stays solely with the sync plan; this phase only shows
	// the exact recorded-pin repairs it is about to apply.
	if opts.format == "human" {
		if err := renderDependencySync(cmd, opts, prepared, false); err != nil {
			return err
		}
	}
	confirmed, err := confirmDependencySync(cmd, app, opts, yes)
	if err != nil || !confirmed {
		return err
	}
	adapter := &dependencyInstaller{app: app, opts: opts, cmd: cmd, detected: detected, consent: consent}
	results, applyErr := (environment.Coordinator{EnvironmentID: app.environmentID(), Mutator: adapter}).ApplySync(cmd.Context(), environment.PreparedSync{EnvironmentID: app.environmentID(), Bindings: prepared}, true)
	if renderErr := renderDependencyResults(cmd, opts, prepared, results, applyErr); renderErr != nil && applyErr == nil {
		return renderErr
	}
	if applyErr != nil {
		return fmt.Errorf("dependency install stopped; inspect binding results and next actions: %w", applyErr)
	}
	return nil
}

// Only the argumentless install verb infers a project, and only in the exact
// working directory. Explicit scope selection always takes precedence.
func selectInstallScope(cmd *cobra.Command, opts *options) error {
	if cmd.Flags().Changed("scope") || opts.projectRoot != "" {
		return nil
	}
	info, err := os.Lstat("uap.toml")
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("detect project manifest: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("detect project manifest: current-directory uap.toml must be a regular non-symlink file; select --scope explicitly")
	}
	root, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	opts.scope, opts.projectRoot = "project", root
	return nil
}

func installLockedOnly(lockPath string, explicit bool) (bool, error) {
	if explicit {
		return true, nil
	}
	_, err := os.Lstat(lockPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect dependency lock: %w", err)
	}
	return true, nil
}

type installSelection struct {
	Scope        string `json:"scope"`
	ProjectRoot  string `json:"project_root,omitempty"`
	ManifestPath string `json:"manifest_path"`
	LockPath     string `json:"lock_path"`
}
