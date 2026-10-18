package agentpluginscli

import (
	"bufio"
	"context"
	"encoding/json"
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
	requirements                     environment.Requirements
	lock                             environment.Lock
	manifestPath, lockPath           string
	manifestBytes, lockBytes         []byte
	prepared                         []dependencyAction
	detected                         map[domain.ClientID]domain.DetectedClient
	consent                          map[string]bool
	candidateManifest, candidateLock []byte
	aliases                          []string
	attestation                      *environment.Attestation
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
	return runDependencySyncConverge(cmd, app, opts, lockedOnly, yes, nil)
}

// dependencyConverge runs after the manifest and lock candidate is validated
// but before any planned action is derived, so a fail-closed candidate refusal
// can never race a converge-side repair write.
type dependencyConverge func(app App) (bool, error)

func runDependencySyncConverge(cmd *cobra.Command, app App, opts *options, lockedOnly, yes bool, converge dependencyConverge) error {
	if err := requireDependencyMutationReady(app); err != nil {
		return err
	}
	if app.Installer == nil {
		return errors.New("typed SDK installer is unavailable")
	}
	view, inspectErr := app.Installer.Inspect(cmd.Context())
	if inspectErr != nil {
		return inspectErr
	}
	if view.Recovery.Required {
		return installer.ErrRecoveryRequired
	}
	preview, cleanup, previewErr := dependencyPreviewApp(app, opts)
	if previewErr != nil {
		return previewErr
	}
	defer cleanup()
	app = preview
	request, err := prepareDependencySyncCandidate(cmd, app, opts, lockedOnly)
	if err != nil {
		return err
	}
	if converge != nil {
		proceed, convergeErr := converge(app)
		if convergeErr != nil || !proceed {
			return convergeErr
		}
	}
	requirements, lock := request.requirements, request.lock
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
	request.prepared = prepared
	request.detected = detected
	request.consent = consent
	return applyDependencySync(cmd, app, opts, request, yes)
}

func renderDependencySync(cmd *cobra.Command, opts *options, actions []dependencyAction, dryRun bool) error {
	if opts.format == "json" {
		data := map[string]any{"actions": dependencyActionOutput(actions, opts.dependencyComponents), "dry_run": dryRun}
		if opts.environmentOutput != nil {
			data["environment"] = opts.environmentOutput
			return writeJSONResult(cmd.OutOrStdout(), "add", "success", data)
		}
		return renderJSON(cmd.OutOrStdout(), data)
	}
	for _, action := range actions {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s/%s\n", action.Action, action.Alias, action.ClientID); err != nil {
			return err
		}
	}
	return renderDependencyComponents(cmd, opts, actions)
}

func applyDependencySync(cmd *cobra.Command, app App, opts *options, request dependencySyncRequest, yes bool) error {
	lock, prepared, detected := request.lock, request.prepared, request.detected
	var actions []dependencyAction
	if err := renderDependencyConfirmation(cmd, opts, prepared); err != nil {
		return err
	}
	if dependencySyncUnchanged(cmd.Context(), app, request) {
		return renderDependencyResults(cmd, opts, prepared, nil, nil)
	}
	confirmed, err := confirmDependencySync(cmd, app, opts, yes)
	if err != nil || !confirmed {
		return err
	}
	unlock, err := processlock.Lock{Path: app.EnvironmentLockPath}.Acquire(cmd.Context())
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	if err := requireDependencyMutationReady(app); err != nil {
		return err
	}
	if err := request.checkInputs(); err != nil {
		return err
	}
	actions, err = request.recheck(cmd, app, opts)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actions, prepared) {
		return installer.ErrPlanChanged
	}
	storeLock := request.selectedLock()
	if err := ensureDependencyStore(cmd.Context(), app, storeLock); err != nil {
		return err
	}
	if err := request.publish(); err != nil {
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

func dependencySyncUnchanged(ctx context.Context, app App, request dependencySyncRequest) bool {
	for _, action := range request.prepared {
		if action.Action != environment.ActionUnchanged {
			return false
		}
		if action.Package != nil {
			if _, err := app.dependencyStore().Get(ctx, action.Package.DigestAlgorithm, action.Package.TreeDigest); err != nil {
				return false
			}
		}
	}
	if request.candidateManifest != nil && string(request.candidateManifest) != string(request.manifestBytes) {
		return false
	}
	return request.candidateLock == nil || string(request.candidateLock) == string(request.lockBytes)
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
	if !sameEnvironmentFile(request.manifestPath, request.manifestBytes) || !sameEnvironmentFile(request.lockPath, request.lockBytes) {
		return installer.ErrPlanChanged
	}
	return nil
}

func (request *dependencySyncRequest) publish() error {
	if request.candidateManifest != nil && string(request.candidateManifest) != string(request.manifestBytes) {
		if err := publishEnvironmentFile(request.manifestPath, request.candidateManifest); err != nil {
			return err
		}
		request.manifestBytes = request.candidateManifest
	}
	if request.candidateLock != nil && string(request.candidateLock) != string(request.lockBytes) {
		if err := publishEnvironmentFile(request.lockPath, request.candidateLock); err != nil {
			return fmt.Errorf("uap.toml saved, uap.lock not written: %w; run agentplugins deps lock, then retry, or agentplugins deps sync", err)
		}
		request.lockBytes = request.candidateLock
	}
	return nil
}

func prepareDependencySyncCandidate(cmd *cobra.Command, app App, opts *options, lockedOnly bool) (dependencySyncRequest, error) {
	manifestPath, lockPath := app.dependencyPaths()
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return dependencySyncRequest{}, err
	}
	requirements, err := readDependencyManifestBytes(manifestPath, manifestBytes)
	if err != nil {
		return dependencySyncRequest{}, err
	}
	if err := validateDependencyTargets(app, requirements); err != nil {
		return dependencySyncRequest{}, err
	}
	lockBytes, err := os.ReadFile(lockPath)
	if err != nil && !os.IsNotExist(err) {
		return dependencySyncRequest{}, err
	}
	if os.IsNotExist(err) {
		lockBytes = nil
	}
	var previous environment.Lock
	if len(lockBytes) > 0 {
		previous, err = decodeDependencyLockBytes(lockBytes, requirements, !lockedOnly)
		if err != nil {
			return dependencySyncRequest{}, err
		}
		canonicalizeRequirementsFromLock(&requirements, previous)
	}
	var lock environment.Lock
	if lockedOnly {
		lock = previous
		if err := lock.Validate(requirements); err != nil {
			return dependencySyncRequest{}, fmt.Errorf("--locked requires an exact current uap.lock: %w", err)
		}
	} else {
		requirements, err = app.canonicalizeDependencyRequirements(cmd.Context(), requirements)
		if err != nil {
			return dependencySyncRequest{}, err
		}
		lock, err = (environment.Coordinator{Resolver: dependencyResolver{app: app, requirements: requirements, dryRun: opts.dryRun}}).Resolve(cmd.Context(), requirements, previous, "")
		if err != nil {
			return dependencySyncRequest{}, err
		}
	}
	candidateLock, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return dependencySyncRequest{}, err
	}
	candidateLock = append(candidateLock, '\n')
	if equalDependencyLock(previous, lock) {
		candidateLock = lockBytes
	}

	return dependencySyncRequest{requirements: requirements, lock: lock, manifestPath: manifestPath, lockPath: lockPath, manifestBytes: manifestBytes, lockBytes: lockBytes, candidateLock: candidateLock}, nil

}

func (request dependencySyncRequest) recheck(cmd *cobra.Command, app App, opts *options) ([]dependencyAction, error) {
	var actions []dependencyAction
	requirements, lock, detected := request.requirements, request.lock, request.detected
	state, err := app.Installer.Inspect(cmd.Context())
	if err != nil {
		return nil, err
	}
	if len(request.aliases) > 0 {
		var observed []environment.BindingObservation
		observed, err = environment.Observe(state)
		if err == nil {
			var selected environment.PreparedSync
			selected, err = (environment.Coordinator{}).PrepareSyncFor(requirements, lock, observed, request.aliases, request.attestation)
			actions = selected.Bindings
		}
	} else {
		actions, err = dependencyActions(state, requirements, lock)
	}
	if err != nil {
		return nil, err
	}
	if _, err = preflightDependencyActions(cmd, app, opts, actions, detected, request.consent); err != nil {
		return nil, fmt.Errorf("dependency sync changed after preflight: %w", err)
	}
	return actions, nil
}

func (request dependencySyncRequest) selectedLock() environment.Lock {
	lock := request.lock
	storeLock := lock
	if len(request.aliases) > 0 {
		storeLock.Packages = nil
		for _, pkg := range lock.Packages {
			for _, alias := range request.aliases {
				if alias == pkg.Alias {
					storeLock.Packages = append(storeLock.Packages, pkg)
				}
			}
		}
	}

	return storeLock
}

func confirmDependencySync(cmd *cobra.Command, app App, opts *options, yes bool) (bool, error) {
	if !yes && app.Terminal && opts.format == "human" {
		reader := bufio.NewReader(cmd.InOrStdin())
		confirmed, promptErr := promptYesNo(cmd.Context(), reader, cmd.OutOrStdout(), cmd.ErrOrStderr(), "Apply the preflighted dependency sync? [y/N]")
		if promptErr != nil {
			return false, promptErr
		}
		if !confirmed {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
			return false, err
		}
		yes = confirmed
	}
	if !yes {
		return false, errors.New("dependency sync requires --yes or explicit interactive confirmation")
	}

	return true, nil
}
