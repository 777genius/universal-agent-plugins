package agentpluginscli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func newDepsCommand(app App, opts *options) *cobra.Command {
	command := &cobra.Command{Use: "deps", Short: "Manage the portable user dependency environment"}
	var locked, yes bool
	command.AddCommand(&cobra.Command{
		Use: "init", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateDepsOptions(opts); err != nil {
				return err
			}
			manifest, _ := app.dependencyPaths()
			if _, err := os.Lstat(manifest); err == nil {
				return fmt.Errorf("%s already exists; init never rewrites it", filepath.Base(manifest))
			} else if !os.IsNotExist(err) {
				return err
			}
			if opts.dryRun {
				return renderJSON(cmd.OutOrStdout(), map[string]any{"dry_run": true, "action": "create_manifest"})
			}
			return writeInitialDependencyManifest(manifest)
		},
	})
	command.AddCommand(&cobra.Command{
		Use: "lock", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateDepsOptions(opts); err != nil {
				return err
			}
			result, err := runDependencyLock(cmd.Context(), app, opts, "", true)
			if err != nil {
				return err
			}
			return renderDependencyLock(cmd, opts, result)
		},
	})
	command.AddCommand(&cobra.Command{
		Use: "update <alias>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDepsOptions(opts); err != nil {
				return err
			}
			result, err := runDependencyLock(cmd.Context(), app, opts, args[0], true)
			if err != nil {
				return err
			}
			return renderDependencyLock(cmd, opts, result)
		},
	})
	command.AddCommand(&cobra.Command{
		Use: "status", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateDepsOptions(opts); err != nil {
				return err
			}
			return runDependencyStatus(cmd, app, opts)
		},
	})
	sync := &cobra.Command{
		Use: "sync", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateDepsOptions(opts); err != nil {
				return err
			}
			return runDependencySync(cmd, app, opts, locked, yes)
		},
	}
	sync.Flags().BoolVar(&opts.externalUninstalled, "external-uninstalled", false, "attest removed targets were uninstalled manually or never imported")
	sync.Flags().BoolVar(&locked, "locked", false, "require an exact existing lock and never resolve refs")
	sync.Flags().BoolVar(&yes, "yes", false, "confirm the complete preflighted sync without an interactive prompt")
	command.AddCommand(sync)
	recover := &cobra.Command{
		Use: "recover", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateDepsOptions(opts); err != nil {
				return err
			}
			return runDependencyRecover(cmd, app, opts, yes)
		},
	}
	recover.Flags().BoolVar(&yes, "yes", false, "confirm the complete preflighted recovery without an interactive prompt")
	command.AddCommand(recover)
	return command
}

func validateDepsOptions(opts *options) error {
	if opts.target != "" {
		return errors.New("deps uses uap.toml targets only; inherited --target is not allowed")
	}
	if opts.scope != "user" {
		return errors.New("deps supports --scope user only")
	}
	return validateCommonOptions(opts)
}

type dependencyLockResult struct {
	Manifest environment.Requirements `json:"-"`
	Lock     environment.Lock         `json:"lock"`
	LockPath string                   `json:"-"`
	Changed  bool                     `json:"changed"`
	DryRun   bool                     `json:"dry_run"`
}

func runDependencyLock(ctx context.Context, app App, opts *options, updateAlias string, allowWrite bool) (dependencyLockResult, error) {
	if allowWrite && !opts.dryRun {
		release, lockErr := processlock.Lock{Path: app.EnvironmentLockPath}.Acquire(ctx)
		if lockErr != nil {
			return dependencyLockResult{}, lockErr
		}
		defer func() { _ = release() }()
	}
	if app.Installer != nil {
		view, err := app.Installer.Inspect(ctx)
		if err != nil {
			return dependencyLockResult{}, err
		}
		if view.Recovery.Required {
			return dependencyLockResult{}, installer.ErrRecoveryRequired
		}
	}
	manifestPath, lockPath := app.dependencyPaths()
	manifestBefore, manifestReadErr := os.ReadFile(manifestPath)
	if manifestReadErr != nil {
		return dependencyLockResult{}, manifestReadErr
	}
	requirements, err := readDependencyManifest(manifestPath)
	if err != nil {
		return dependencyLockResult{}, err
	}
	for target := range requirements.Targets {
		if app.ClientRegistry == nil {
			return dependencyLockResult{}, errors.New("client registry unavailable")
		}
		if _, ok := app.ClientRegistry.Lookup(domain.ClientID(target)); !ok {
			return dependencyLockResult{}, fmt.Errorf("unsupported target %q", target)
		}
	}
	preview, cleanup, err := dependencyPreviewApp(ctx, app, opts)
	if err != nil {
		return dependencyLockResult{}, err
	}
	defer cleanup()
	app = preview
	requirements, err = app.canonicalizeDependencyRequirements(ctx, requirements)
	if err != nil {
		return dependencyLockResult{}, err
	}
	var previous environment.Lock
	hadPrevious := false
	if body, readErr := os.ReadFile(lockPath); readErr == nil {
		previous, err = decodeCompatibleDependencyLockBytes(body, requirements)
		if err != nil {
			return dependencyLockResult{}, err
		}
		hadPrevious = err == nil
	} else if !os.IsNotExist(readErr) {
		return dependencyLockResult{}, readErr
	}
	resolver := dependencyResolver{app: app, dryRun: opts.dryRun, requirements: requirements}
	var previousPin *environment.Lock
	if hadPrevious {
		previousPin = &previous
	}
	locked, err := (environment.Coordinator{Resolver: resolver}).Resolve(ctx, requirements, derefLock(previousPin), updateAlias)
	if err != nil {
		return dependencyLockResult{}, err
	}
	if !opts.dryRun {
		if err := ensureDependencyStore(ctx, app, locked); err != nil {
			return dependencyLockResult{}, err
		}
	}
	changed := !hadPrevious || !equalDependencyLock(previous, locked)
	if allowWrite && changed && !opts.dryRun {
		manifestAfter, readErr := os.ReadFile(manifestPath)
		if readErr != nil || string(manifestAfter) != string(manifestBefore) {
			return dependencyLockResult{}, errors.New("uap.toml changed during lock resolution; rerun deps lock")
		}
		if err := writeDependencyLock(lockPath, locked); err != nil {
			return dependencyLockResult{}, err
		}
	}
	return dependencyLockResult{Manifest: requirements, Lock: locked, LockPath: lockPath, Changed: changed, DryRun: opts.dryRun}, nil
}

func derefLock(lock *environment.Lock) environment.Lock {
	if lock == nil {
		return environment.Lock{}
	}
	return *lock
}

func decodeDependencyLockBytes(body []byte, requirements environment.Requirements) (environment.Lock, error) {
	temp, err := os.CreateTemp("", "uap-lock-*.json")
	if err != nil {
		return environment.Lock{}, err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(body); err == nil {
		err = temp.Close()
	} else {
		_ = temp.Close()
	}
	if err != nil {
		return environment.Lock{}, err
	}
	return readDependencyLock(name, requirements)
}

func decodeCompatibleDependencyLockBytes(body []byte, requirements environment.Requirements) (environment.Lock, error) {
	temp, err := os.CreateTemp("", "uap-lock-*.json")
	if err != nil {
		return environment.Lock{}, err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(body); err == nil {
		err = temp.Close()
	} else {
		_ = temp.Close()
	}
	if err != nil {
		return environment.Lock{}, err
	}
	return readCompatibleDependencyLock(name, requirements)
}

func equalDependencyLock(left, right environment.Lock) bool {
	leftBody, _ := jsonMarshal(left)
	rightBody, _ := jsonMarshal(right)
	return string(leftBody) == string(rightBody)
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func dependencyPreviewApp(ctx context.Context, app App, opts *options) (App, func(), error) {
	noop := func() {}
	if !opts.dryRun {
		return app, noop, nil
	}
	root, err := os.MkdirTemp("", "uap-dependency-preview-")
	if err != nil {
		return app, noop, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	if app.SecurityEvaluatorForDryRun != nil {
		app.SecurityEvaluator = app.SecurityEvaluatorForDryRun(root)
	}
	if app.DirectoryClientForDryRun != nil {
		app.DirectoryClient, err = app.DirectoryClientForDryRun(root)
		if err != nil {
			cleanup()
			return app, noop, err
		}
	}
	if app.SourceAcquirerForDryRun != nil {
		app.SourceAcquirer = app.SourceAcquirerForDryRun(root)
	}
	if app.Installer != nil {
		app.Installer, err = app.Installer.WithTemporaryRoot(root)
		if err != nil {
			cleanup()
			return app, noop, err
		}
	}
	return app, cleanup, nil
}

func renderJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func ensureDependencyStore(ctx context.Context, app App, lock environment.Lock) error {
	store := app.dependencyStore()
	for _, pkg := range lock.Packages {
		if _, err := store.Get(ctx, pkg.DigestAlgorithm, pkg.TreeDigest); err == nil {
			continue
		} else if !errors.Is(err, environmentstore.ErrMissing) && !errors.Is(err, environmentstore.ErrMismatch) {
			return err
		}
		loaded, err := materializeLockedPackage(ctx, app, pkg, true, nil)
		if err != nil {
			return err
		}
		if loaded.cleanup != nil {
			_ = loaded.cleanup()
		}
	}
	return nil
}

func renderDependencyLock(cmd *cobra.Command, opts *options, result dependencyLockResult) error {
	if opts.format == "json" {
		return renderJSON(cmd.OutOrStdout(), result)
	}
	state := "unchanged"
	if result.Changed {
		state = "updated"
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "Dependency lock %s: %d packages, manifest %s\n", state, len(result.Lock.Packages), result.Lock.ManifestDigest)
	return err
}

type dependencyAction = environment.PreparedBinding

func dependencyActions(view installer.Inspection, requirements environment.Requirements, lock environment.Lock) ([]dependencyAction, error) {
	if view.Recovery.Required {
		return nil, installer.ErrRecoveryRequired
	}
	for _, pkg := range lock.Packages {
		for _, installed := range view.Installations {
			if installed.SourceKey != pkg.SourceKey {
				continue
			}
			for _, binding := range installed.Bindings {
				for _, target := range pkg.Targets {
					if binding.ClientID == target && (binding.Owner == nil || binding.Owner.EnvironmentID != environment.EnvironmentUserDefault || binding.Owner.Alias != pkg.Alias) {
						return nil, fmt.Errorf("foreign: %s/%s already has a live binding outside this environment alias", pkg.Alias, target)
					}
				}
			}
		}
	}
	var observations []environment.BindingObservation
	for _, receipt := range view.EnvironmentReceipts {
		if receipt.EnvironmentID != environment.EnvironmentUserDefault {
			continue
		}
		current := environment.BindingObservation{Alias: receipt.Alias, ClientID: receipt.ClientID, InstallationID: receipt.InstallationID, BindingID: receipt.ClientBindingID, SourceKey: receipt.SourceKey, PackageDigest: receipt.PackageDigest, Managed: true, Receipt: &receipt}
		found := false
		for _, installation := range view.Installations {
			if installation.InstallationID != receipt.InstallationID {
				continue
			}
			for _, binding := range installation.Bindings {
				if binding.BindingID != receipt.ClientBindingID {
					continue
				}
				found = true
				current.Revision = binding.Revision
				current.Drift = binding.ProjectionStatus != ""
				current.Pending = binding.Materialization != string(domain.MaterializationMaterialized)
				if binding.TreeDigest != receipt.PackageDigest {
					return nil, errors.New("receipt/live binding package mismatch")
				}
			}
		}
		if !found {
			return nil, errors.New("partial: environment receipt has no live binding")
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
	preview, cleanup, previewErr := dependencyPreviewApp(cmd.Context(), app, opts)
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
	var lock environment.Lock
	if lockedOnly {
		lock, err = readDependencyLock(lockPath, requirements)
		if err != nil {
			return fmt.Errorf("--locked requires an exact current uap.lock: %w", err)
		}
	} else {
		result, lockErr := runDependencyLock(cmd.Context(), app, opts, "", true)
		if lockErr != nil {
			return lockErr
		}
		lock = result.Lock
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var lockBytes []byte
	if !opts.dryRun {
		lockBytes, err = os.ReadFile(lockPath)
		if err != nil {
			return err
		}
	}
	state, err := app.Installer.Inspect(cmd.Context())
	if err != nil {
		return err
	}
	actions, err := dependencyActions(state, requirements, lock)
	if err != nil {
		return err
	}
	clients, err := app.Detector.Detect(cmd.Context())
	if err != nil {
		return err
	}
	detected := map[domain.ClientID]domain.DetectedClient{}
	for _, client := range clients {
		detected[client.ClientID] = client
	}
	prepared, err := preflightDependencyActions(cmd, app, opts, actions, detected)
	if err != nil {
		return err
	}
	if opts.dryRun {
		return renderDependencySync(cmd, opts, prepared, true)
	}
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
	var unlock func() error
	if !opts.dryRun {
		unlock, err = processlock.Lock{Path: app.EnvironmentLockPath}.Acquire(cmd.Context())
		if err != nil {
			return err
		}
		defer func() { _ = unlock() }()
		state, err = app.Installer.Inspect(cmd.Context())
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
