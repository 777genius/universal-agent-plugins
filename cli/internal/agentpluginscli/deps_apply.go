package agentpluginscli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type dependencySyncRequest struct {
	requirements             environment.Requirements
	lock                     environment.Lock
	manifestPath, lockPath   string
	manifestBytes, lockBytes []byte
	prepared                 []dependencyAction
	detected                 map[domain.ClientID]domain.DetectedClient
	consent                  map[string]bool
}

func dependencyActions(view installer.Inspection, requirements environment.Requirements, lock environment.Lock) ([]dependencyAction, error) {
	observations, err := environment.Observe(view)
	if err != nil {
		return nil, err
	}
	prepared, err := (environment.Coordinator{}).PrepareSync(requirements, lock, observations)
	return prepared.Bindings, err
}

func runDependencySync(cmd *cobra.Command, app App, opts *options, lockedOnly, yes bool) error {
	if err := requireDependencyMutationReady(app); err != nil {
		return err
	}
	if app.Installer == nil {
		return errors.New("typed SDK installer is unavailable")
	}
	preview, cleanup, previewErr := dependencyPreviewApp(app, opts)
	if previewErr != nil {
		return previewErr
	}
	defer cleanup()
	app = preview
	manifestPath, lockPath := app.dependencyPaths()
	requirements, lock, err := prepareDependencySyncLock(cmd.Context(), app, opts, lockedOnly, manifestPath, lockPath)
	if err != nil {
		return err
	}
	manifestBytes, lockBytes, err := readDependencySyncInputs(manifestPath, lockPath, opts.dryRun)
	if err != nil {
		return err
	}
	state, err := app.Installer.Inspect(cmd.Context())
	if err != nil {
		return err
	}
	actions, err := dependencyActions(state, requirements, lock)
	if err != nil {
		return err
	}
	if err := validateDependencyRemovals(opts, actions); err != nil {
		return err
	}
	detected, err := detectDependencyClients(cmd.Context(), app)
	if err != nil {
		return err
	}
	consent := map[string]bool{}
	prepared, err := preflightDependencyActions(cmd, app, opts, actions, detected, consent)
	if err != nil {
		return err
	}
	if opts.dryRun {
		return renderDependencySync(cmd, opts, prepared, true)
	}
	request := dependencySyncRequest{requirements: requirements, lock: lock, manifestPath: manifestPath, lockPath: lockPath, manifestBytes: manifestBytes, lockBytes: lockBytes, prepared: prepared, detected: detected, consent: consent}
	return applyDependencySync(cmd, app, opts, request, yes)
}

func renderDependencySync(cmd *cobra.Command, opts *options, actions []dependencyAction, dryRun bool) error {
	if opts.format == "json" {
		return renderJSON(cmd.OutOrStdout(), map[string]any{"actions": dependencyActionOutput(actions), "dry_run": dryRun})
	}
	for _, action := range actions {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s/%s\n", action.Action, action.Alias, action.ClientID); err != nil {
			return err
		}
	}
	return nil
}

func applyDependencySync(cmd *cobra.Command, app App, opts *options, request dependencySyncRequest, yes bool) error {
	requirements, lock, prepared, detected := request.requirements, request.lock, request.prepared, request.detected
	var actions []dependencyAction
	if err := renderDependencyConfirmation(cmd, opts, prepared); err != nil {
		return err
	}
	if !yes && app.Terminal && opts.format == "human" {
		reader := bufio.NewReader(cmd.InOrStdin())
		confirmed, promptErr := promptYesNo(cmd.Context(), reader, cmd.OutOrStdout(), cmd.ErrOrStderr(), "Apply the preflighted dependency sync? [y/N]")
		if promptErr != nil {
			return promptErr
		}
		if !confirmed {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
			return err
		}
		yes = confirmed
	}
	if !yes {
		return errors.New("dependency sync requires --yes or explicit interactive confirmation")
	}
	unlock, err := processlock.Lock{Path: app.EnvironmentLockPath}.Acquire(cmd.Context())
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	{
		state, err := app.Installer.Inspect(cmd.Context())
		if err != nil {
			return err
		}
		actions, err = dependencyActions(state, requirements, lock)
		if err != nil {
			return err
		}
		if _, err = preflightDependencyActions(cmd, app, opts, actions, detected, request.consent); err != nil {
			return fmt.Errorf("dependency sync changed after preflight: %w", err)
		}
	}
	if !reflect.DeepEqual(actions, prepared) {
		return installer.ErrPlanChanged
	}
	if err := ensureDependencyStore(cmd.Context(), app, lock); err != nil {
		return err
	}
	adapter := &dependencyInstaller{app: app, opts: opts, detected: detected, cmd: cmd, consent: request.consent, beforeApply: request.checkInputs}
	results, applyErr := (environment.Coordinator{Mutator: adapter}).ApplySync(cmd.Context(), environment.PreparedSync{EnvironmentID: environment.EnvironmentUserDefault, ManifestDigest: lock.ManifestDigest, Bindings: actions}, true)
	if renderErr := renderDependencyResults(cmd, opts, prepared, results, applyErr); renderErr != nil && applyErr == nil {
		return renderErr
	}
	if applyErr != nil {
		return fmt.Errorf("dependency sync stopped; inspect binding results and next actions: %w", applyErr)
	}
	return nil
}

func selectDependencySyncLock(ctx context.Context, app App, opts *options, lockedOnly bool, lockPath string, requirements environment.Requirements) (environment.Lock, error) {
	var lock environment.Lock
	if lockedOnly {
		var err error
		lock, err = readDependencyLock(lockPath, requirements)
		if err != nil {
			return environment.Lock{}, fmt.Errorf("--locked requires an exact current uap.lock: %w", err)
		}
	} else {
		result, lockErr := runDependencyLock(ctx, app, opts, "", true)
		if lockErr != nil {
			return environment.Lock{}, lockErr
		}
		lock = result.Lock
	}
	return lock, nil
}

func detectDependencyClients(ctx context.Context, app App) (map[domain.ClientID]domain.DetectedClient, error) {
	clients, err := app.Detector.Detect(ctx)
	if err != nil {
		return nil, err
	}
	detected := map[domain.ClientID]domain.DetectedClient{}
	for _, client := range clients {
		detected[client.ClientID] = client
	}
	return detected, nil
}

func readDependencySyncInputs(manifestPath, lockPath string, dryRun bool) ([]byte, []byte, error) {
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, nil, err
	}
	var lockBytes []byte
	if !dryRun {
		lockBytes, err = os.ReadFile(lockPath)
		if err != nil {
			return nil, nil, err
		}
	}
	return manifestBytes, lockBytes, nil
}

func (app App) canonicalizeDependencySyncRequirements(ctx context.Context, requirements environment.Requirements, lockPath string, lockedOnly bool) (environment.Requirements, error) {
	readLock := readCompatibleDependencyLock
	if lockedOnly {
		readLock = readDependencyLock
	}
	lock, err := readLock(lockPath, requirements)
	if err == nil {
		canonicalizeRequirementsFromLock(&requirements, lock)
	} else if lockedOnly || !os.IsNotExist(err) {
		return environment.Requirements{}, fmt.Errorf("read dependency lock: %w", err)
	}
	return app.canonicalizeDependencyRequirements(ctx, requirements)
}

func requireDependencyMutationReady(app App) error {
	if guard, ok := app.StateStore.(interface{ RequireMutationReady() error }); ok {
		return guard.RequireMutationReady()
	}
	return nil
}

func validateDependencyRemovals(opts *options, actions []dependencyAction) error {
	if !opts.externalUninstalled {
		return nil
	}
	for _, action := range actions {
		if action.Action == environment.ActionRemove {
			return nil
		}
	}
	return errors.New("--external-uninstalled requires at least one removal")
}

func renderDependencyConfirmation(cmd *cobra.Command, opts *options, prepared []dependencyAction) error {
	if opts.format == "human" {
		if err := renderDependencySync(cmd, opts, prepared, false); err != nil {
			return err
		}
	}
	if opts.externalUninstalled {
		removals := 0
		for _, action := range prepared {
			if action.Action == environment.ActionRemove {
				removals++
				if opts.format == "human" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Attest external uninstall: %s/%s\n", action.Alias, action.ClientID)
				}
			}
		}
		if removals == 0 {
			return errors.New("--external-uninstalled requires at least one removal")
		}
	}
	return nil
}

func (request dependencySyncRequest) checkInputs() error {
	m, err := os.ReadFile(request.manifestPath)
	if err != nil {
		return err
	}
	l, err := os.ReadFile(request.lockPath)
	if err != nil {
		return err
	}
	if string(m) != string(request.manifestBytes) || string(l) != string(request.lockBytes) {
		return installer.ErrPlanChanged
	}
	return nil
}

func prepareDependencySyncLock(ctx context.Context, app App, opts *options, lockedOnly bool, manifestPath, lockPath string) (environment.Requirements, environment.Lock, error) {
	requirements, err := readDependencyManifest(manifestPath)
	if err != nil {
		return environment.Requirements{}, environment.Lock{}, err
	}
	if err := validateDependencyTargets(app, requirements); err != nil {
		return environment.Requirements{}, environment.Lock{}, err
	}
	requirements, err = app.canonicalizeDependencySyncRequirements(ctx, requirements, lockPath, lockedOnly)
	if err != nil {
		return environment.Requirements{}, environment.Lock{}, err
	}
	lock, err := selectDependencySyncLock(ctx, app, opts, lockedOnly, lockPath, requirements)
	if err != nil {
		return environment.Requirements{}, environment.Lock{}, err
	}
	return requirements, lock, nil
}
