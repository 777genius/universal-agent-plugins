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
}

func dependencyActions(view installer.Inspection, requirements environment.Requirements, lock environment.Lock) ([]dependencyAction, error) {
	if view.Recovery.Required {
		return nil, installer.ErrRecoveryRequired
	}
	if err := rejectForeignDependencies(view, lock); err != nil {
		return nil, err
	}
	var observations []environment.BindingObservation
	for _, receipt := range view.EnvironmentReceipts {
		if receipt.EnvironmentID != environment.EnvironmentUserDefault {
			continue
		}
		current := environment.BindingObservation{Alias: receipt.Alias, ClientID: receipt.ClientID, InstallationID: receipt.InstallationID, BindingID: receipt.ClientBindingID, SourceKey: receipt.SourceKey, PackageDigest: receipt.PackageDigest, Managed: true, Receipt: &receipt}
		binding, found := findInspectedEnvironmentBinding(view, receipt)
		if !found {
			return nil, errors.New("partial: environment receipt has no live binding")
		}
		current.Incomplete = bindingLifecycleIncomplete(binding)
		current.Revision = binding.Revision
		current.Drift = binding.ProjectionStatus != ""
		current.Pending = binding.Materialization != string(domain.MaterializationMaterialized)
		if binding.TreeDigest != receipt.PackageDigest {
			return nil, errors.New("receipt/live binding package mismatch")
		}
		observations = append(observations, current)
	}
	prepared, err := (environment.Coordinator{}).PrepareSync(requirements, lock, observations)
	return prepared.Bindings, err
}

func runDependencySync(cmd *cobra.Command, app App, opts *options, lockedOnly, yes bool) error {
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
	requirements, err := readDependencyManifest(manifestPath)
	if err != nil {
		return err
	}
	requirements, err = app.canonicalizeDependencyRequirements(cmd.Context(), requirements)
	if err != nil {
		return err
	}
	lock, err := selectDependencySyncLock(cmd.Context(), app, opts, lockedOnly, lockPath, requirements)
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
	detected, err := detectDependencyClients(cmd.Context(), app)
	if err != nil {
		return err
	}
	prepared, err := preflightDependencyActions(cmd, app, opts, actions, detected)
	if err != nil {
		return err
	}
	if opts.dryRun {
		return renderDependencySync(cmd, opts, prepared, true)
	}
	request := dependencySyncRequest{requirements: requirements, lock: lock, manifestPath: manifestPath, lockPath: lockPath, manifestBytes: manifestBytes, lockBytes: lockBytes, prepared: prepared, detected: detected}
	return applyDependencySync(cmd, app, opts, request, yes)
}

func renderDependencySync(cmd *cobra.Command, opts *options, actions []dependencyAction, dryRun bool) error {
	if opts.format == "json" {
		return renderJSON(cmd.OutOrStdout(), map[string]any{"actions": actions, "dry_run": dryRun})
	}
	for _, action := range actions {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s/%s\n", action.Action, action.Alias, action.ClientID); err != nil {
			return err
		}
	}
	return nil
}

func rejectForeignDependencies(view installer.Inspection, lock environment.Lock) error {
	for _, pkg := range lock.Packages {
		for _, installed := range view.Installations {
			if installed.SourceKey != pkg.SourceKey {
				continue
			}
			for _, binding := range installed.Bindings {
				for _, target := range pkg.Targets {
					if binding.ClientID == target && (binding.Owner == nil || binding.Owner.EnvironmentID != environment.EnvironmentUserDefault || binding.Owner.Alias != pkg.Alias) {
						return fmt.Errorf("foreign: %s/%s already has a live binding outside this environment alias", pkg.Alias, target)
					}
				}
			}
		}
	}
	return nil
}

func applyDependencySync(cmd *cobra.Command, app App, opts *options, request dependencySyncRequest, yes bool) error {
	requirements, lock, prepared, detected := request.requirements, request.lock, request.prepared, request.detected
	manifestPath, lockPath, manifestBytes, lockBytes := request.manifestPath, request.lockPath, request.manifestBytes, request.lockBytes
	var actions []dependencyAction
	if !yes && app.Terminal && opts.format == "human" {
		reader := bufio.NewReader(cmd.InOrStdin())
		confirmed, promptErr := promptYesNo(cmd.Context(), reader, cmd.OutOrStdout(), cmd.ErrOrStderr(), "Apply the preflighted dependency sync? [y/N]")
		if promptErr != nil {
			return promptErr
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
		if _, err = preflightDependencyActions(cmd, app, opts, actions, detected); err != nil {
			return fmt.Errorf("dependency sync changed after preflight: %w", err)
		}
	}
	if !reflect.DeepEqual(actions, prepared) {
		return installer.ErrPlanChanged
	}
	if err := ensureDependencyStore(cmd.Context(), app, lock); err != nil {
		return err
	}
	adapter := &dependencyInstaller{app: app, opts: opts, detected: detected, cmd: cmd, beforeApply: func() error {
		m, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		l, err := os.ReadFile(lockPath)
		if err != nil {
			return err
		}
		if string(m) != string(manifestBytes) || string(l) != string(lockBytes) {
			return installer.ErrPlanChanged
		}
		return nil
	}}
	results, applyErr := (environment.Coordinator{Mutator: adapter}).ApplySync(cmd.Context(), environment.PreparedSync{EnvironmentID: environment.EnvironmentUserDefault, ManifestDigest: lock.ManifestDigest, Bindings: actions}, true)
	if applyErr != nil {
		if opts.format == "json" {
			_ = renderJSON(cmd.OutOrStdout(), map[string]any{"outcome": "partial", "results": results, "actions": actions})
		}
		return fmt.Errorf("dependency sync stopped; inspect results and recover pending work before retry: %w", applyErr)
	}
	return renderDependencySync(cmd, opts, prepared, false)
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
