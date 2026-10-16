package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func newDepsCommand(app App, opts *options) *cobra.Command {
	command := &cobra.Command{Use: "deps", Short: "Manage the portable user dependency environment"}
	var locked, yes bool
	command.AddCommand(&cobra.Command{
		Use: "init", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateDepsOptions(opts); err != nil {
				return err
			}
			return initializeDependencies(cmd, app, opts)
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
	command.AddCommand(newDependencyRecoverCommand(app, opts, &yes))
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
	if err := requireDependencyMutationReady(app); err != nil {
		return dependencyLockResult{}, err
	}
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
	requirements, err := readDependencyManifestBytes(manifestPath, manifestBefore)
	if err != nil {
		return dependencyLockResult{}, err
	}
	if err := validateDependencyTargets(app, requirements); err != nil {
		return dependencyLockResult{}, err
	}
	preview, cleanup, err := dependencyPreviewApp(app, opts)
	if err != nil {
		return dependencyLockResult{}, err
	}
	defer cleanup()
	app = preview
	requirements, previous, locked, err := resolveDependencyLock(ctx, app, opts, requirements, lockPath, updateAlias)
	if err != nil {
		return dependencyLockResult{}, err
	}
	hadPrevious := previous.LockVersion != 0
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

func decodeCompatibleDependencyLockBytes(body []byte, requirements environment.Requirements) (environment.Lock, error) {
	return decodeDependencyLockBytes(body, requirements, true)
}

func equalDependencyLock(left, right environment.Lock) bool {
	leftBody, _ := jsonMarshal(left)
	rightBody, _ := jsonMarshal(right)
	return string(leftBody) == string(rightBody)
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func dependencyPreviewApp(app App, opts *options) (App, func(), error) {
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

func initializeDependencies(cmd *cobra.Command, app App, opts *options) error {
	manifest, _ := app.dependencyPaths()
	if _, err := os.Lstat(manifest); err == nil {
		return fmt.Errorf("%s already exists; init never rewrites it", manifest)
	} else if !os.IsNotExist(err) {
		return err
	}
	if opts.dryRun {
		return renderJSON(cmd.OutOrStdout(), map[string]any{"dry_run": true, "action": "create_manifest"})
	}
	if err := os.MkdirAll(filepath.Dir(manifest), 0o700); err != nil {
		return err
	}
	if err := writeInitialDependencyManifest(manifest); err != nil {
		return err
	}
	if opts.format == "json" {
		return renderJSON(cmd.OutOrStdout(), map[string]any{"created": true, "manifest_path": manifest})
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "Created %s\nNext: edit dependencies and targets, then run deps lock.\n", manifest)
	return err
}

func validateDependencyTargets(app App, requirements environment.Requirements) error {
	for target := range requirements.Targets {
		if definition, ok := domain.ClientDefinitionFor(domain.ClientID(target)); ok && (definition.PlansWithoutHostPresence || definition.Traits.RequiresPersonalMappingForPrepare) {
			return fmt.Errorf("%s is prepared manually and cannot be a saved dependency target; use agentplugins add <source> --target %s (not saved)", definition.DisplayName, target)
		}
		if app.Installer == nil || !app.Installer.SupportsClient(target) {
			return fmt.Errorf("unsupported dependency target %q; supported targets: %v", target, supportedDependencyTargets(app))
		}
	}
	return nil
}

func resolveDependencyLock(ctx context.Context, app App, opts *options, requirements environment.Requirements, lockPath, updateAlias string) (environment.Requirements, environment.Lock, environment.Lock, error) {
	var err error
	var previous environment.Lock
	hadPrevious := false
	if body, readErr := os.ReadFile(lockPath); readErr == nil {
		previous, err = decodeCompatibleDependencyLockBytes(body, requirements)
		if err != nil {
			return requirements, environment.Lock{}, environment.Lock{}, err
		}
		hadPrevious = err == nil
	} else if !os.IsNotExist(readErr) {
		return requirements, environment.Lock{}, environment.Lock{}, readErr
	}
	canonicalizeRequirementsFromLock(&requirements, previous)
	requirements, err = app.canonicalizeDependencyRequirements(ctx, requirements)
	if err != nil {
		return requirements, environment.Lock{}, environment.Lock{}, err
	}
	resolver := dependencyResolver{app: app, dryRun: opts.dryRun, requirements: requirements}
	var previousPin *environment.Lock
	if hadPrevious {
		previousPin = &previous
	}
	locked, err := (environment.Coordinator{Resolver: resolver}).Resolve(ctx, requirements, derefLock(previousPin), updateAlias)
	if err != nil {
		return requirements, environment.Lock{}, environment.Lock{}, err
	}
	if !opts.dryRun {
		if err := ensureDependencyStore(ctx, app, locked); err != nil {
			return requirements, environment.Lock{}, environment.Lock{}, err
		}
	}
	return requirements, previous, locked, nil
}

func newDependencyRecoverCommand(app App, opts *options, yes *bool) *cobra.Command {
	recovery := &cobra.Command{
		Use: "recover", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateDepsOptions(opts); err != nil {
				return err
			}
			return runDependencyRecover(cmd, app, opts, *yes)
		},
	}
	recovery.Flags().BoolVar(yes, "yes", false, "confirm the complete preflighted recovery without an interactive prompt")
	return recovery
}

func supportedDependencyTargets(app App) []string {
	out := []string{}
	for _, target := range domain.SupportedClientIDs() {
		if app.Installer != nil && app.Installer.SupportsClient(string(target)) {
			out = append(out, string(target))
		}
	}
	return out
}
