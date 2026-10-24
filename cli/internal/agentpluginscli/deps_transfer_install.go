package agentpluginscli

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func runDependencyTransferInstall(cmd *cobra.Command, app App, opts *options, transfer *dependencyTransfer, yes bool) error {
	if err := requireDependencyTransferReady(cmd, app); err != nil {
		return err
	}
	if app.Installer == nil {
		return errors.New("typed SDK installer is unavailable")
	}
	// Preparation uses disposable acquisition/cache roots even for a real import.
	// WithTemporaryRoot retains the authoritative installer state and Apply policy.
	stagingOptions := *opts
	stagingOptions.dryRun = true
	preview, cleanup, err := dependencyPreviewApp(app, &stagingOptions)
	if err != nil {
		return err
	}
	defer cleanup()
	request, err := prepareDependencyTransferInstall(cmd, preview, opts, transfer)
	if err != nil {
		return err
	}
	transfer.prepared = request.prepared
	transfer.output.Actions = dependencyActionOutput(request.prepared, opts.dependencyComponents)
	if err := transfer.check(); err != nil {
		return err
	}
	if opts.dryRun {
		return renderDependencyTransferInstallPlan(cmd, opts, transfer)
	}
	if dependencySyncUnchanged(cmd.Context(), preview, request) {
		transfer.output.Outcome, transfer.output.AgreementSaved = "unchanged", true
		return renderDependencyTransfer(cmd, opts, transfer.output)
	}
	confirmed, err := confirmDependencyTransfer(cmd, app, opts, transfer, yes)
	if err != nil || !confirmed {
		return err
	}
	return commitDependencyTransferInstall(cmd, app, preview, opts, transfer, request)
}

func commitDependencyTransferInstall(cmd *cobra.Command, original App, staged App, opts *options, transfer *dependencyTransfer, request dependencySyncRequest) error {
	release, err := lockDependencyTransfer(cmd, original, transfer)
	if err != nil {
		return err
	}
	defer release()
	if err := requireDependencyTransferReady(cmd, original); err != nil {
		return err
	}
	if err := transfer.check(); err != nil {
		return err
	}
	actions, err := request.recheck(cmd, staged, opts)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actions, request.prepared) {
		return installer.ErrPlanChanged
	}
	if err := transfer.publish(cmd.Context()); err != nil {
		return renderDependencyTransferFailure(cmd, opts, transfer, err)
	}
	request.manifestBytes, request.lockBytes = transfer.source.files[0].body, transfer.source.files[1].body
	if err := cmd.Context().Err(); err != nil {
		return finishDependencyTransferInstall(cmd, opts, transfer, request, nil, err)
	}
	if original.CommitDirectoryPreview != nil {
		if err := original.CommitDirectoryPreview(staged.DirectoryClient); err != nil {
			return finishDependencyTransferInstall(cmd, opts, transfer, request, nil, err)
		}
	}
	staged.DirectoryClient, staged.SecurityEvaluator = original.DirectoryClient, original.SecurityEvaluator
	return applyDependencyTransferInstall(cmd, staged, opts, transfer, request, actions)
}

func prepareDependencyTransferInstall(cmd *cobra.Command, app App, opts *options, transfer *dependencyTransfer) (dependencySyncRequest, error) {
	source, destination := transfer.source, transfer.destination
	request := dependencySyncRequest{requirements: source.requirements, lock: source.lock, manifestPath: destination.files[0].path, lockPath: destination.files[1].path, manifestBytes: destination.files[0].body, lockBytes: destination.files[1].body, candidateManifest: source.files[0].body, candidateLock: source.files[1].body}
	if err := verifyDependencyDeclarations(cmd.Context(), app, request.lock); err != nil {
		return request, err
	}
	state, err := app.Installer.Inspect(cmd.Context())
	if err != nil {
		return request, err
	}
	actions, err := dependencyActions(state, request.requirements, request.lock, app.environmentID())
	if err != nil {
		return request, err
	}
	if err := validateDependencyRemovals(opts, actions); err != nil {
		return request, err
	}
	request.detected, err = detectDependencyClients(cmd.Context(), app)
	if err != nil {
		return request, err
	}
	request.consent = map[string]bool{}
	request.prepared, err = preflightDependencyActions(cmd, app, opts, actions, request.detected, request.consent)
	return request, err
}

func applyDependencyTransferInstall(cmd *cobra.Command, app App, opts *options, transfer *dependencyTransfer, request dependencySyncRequest, actions []dependencyAction) error {
	// Once saved, this pair is desired state; later installation failures retain it.
	if err := ensureDependencyStore(cmd.Context(), app, request.selectedLock()); err != nil {
		return finishDependencyTransferInstall(cmd, opts, transfer, request, nil, err)
	}
	adapter := &dependencyInstaller{app: app, opts: opts, detected: request.detected, cmd: cmd, consent: request.consent, beforeApply: request.checkInputs}
	results, applyErr := (environment.Coordinator{EnvironmentID: app.environmentID(), Mutator: adapter}).ApplySync(cmd.Context(), environment.PreparedSync{EnvironmentID: app.environmentID(), ManifestDigest: request.lock.ManifestDigest, Bindings: actions}, true)
	return finishDependencyTransferInstall(cmd, opts, transfer, request, results, applyErr)
}

func finishDependencyTransferInstall(cmd *cobra.Command, opts *options, transfer *dependencyTransfer, request dependencySyncRequest, results []installer.Result, applyErr error) error {
	renderErr := renderDependencyTransferResults(cmd, opts, transfer, request, results, applyErr)
	if applyErr != nil {
		return errors.Join(fmt.Errorf("agreement saved; locked installation stopped; inspect results, use deps recover if required, then retry agentplugins install --locked: %w", applyErr), renderErr)
	}
	return renderErr
}

// The existing environment lock creates its parent. Adopt only that newly
// observed directory; the approved absent file preimages remain unchanged.
func (transfer *dependencyTransfer) observeImportLockDirectory() error {
	if transfer.destination.directoryInfo != nil {
		return nil
	}
	info, err := os.Stat(transfer.destination.directory)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return installer.ErrPlanChanged
	}
	transfer.destination.directoryInfo = info
	return nil
}
