package agentpluginscli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type updateTargetResult struct {
	Target     string        `json:"target"`
	Selected   bool          `json:"selected"`
	Status     string        `json:"status"`
	NextAction string        `json:"next_action,omitempty"`
	Output     addResultData `json:"output"`
}

type updateMultiResult struct {
	OperationID string               `json:"operation_id,omitempty"`
	Batch       bool                 `json:"batch"`
	Status      string               `json:"status"`
	Succeeded   int                  `json:"succeeded"`
	Failed      int                  `json:"failed"`
	Plugin      string               `json:"plugin"`
	Version     string               `json:"version,omitempty"`
	Source      string               `json:"source"`
	Revision    string               `json:"revision,omitempty"`
	TreeDigest  string               `json:"tree_digest,omitempty"`
	DryRun      bool                 `json:"dry_run"`
	Targets     []updateTargetResult `json:"targets"`
}

type preparedUpdateMany struct {
	loaded         loadedPackage
	service        usecase.Service
	inputs         []usecase.AddInput
	compatibility  []usecase.AddInput
	selected       []domain.ClientID
	result         updateMultiResult
	noChange       bool
	dryRun         bool
	state          domain.StateFileV2
	installationID string
}

func runUpdateMany(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector string, targets []domain.ClientID) error {
	state, err := app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return err
	}
	prepared, err := prepareUpdateMany(ctx, app, opts, installation, targets, !opts.dryRun)
	if prepared != nil {
		defer prepared.cleanup()
	}
	if err != nil {
		if prepared != nil {
			_ = renderUpdateMultiResult(cmd, opts, prepared.result)
			return fmt.Errorf("group update preflight failed; no target was changed: %w%s", err, groupNextAction(prepared.result.Targets))
		}
		return err
	}
	if err := authorizeSecurityAssessment(cmd, app, opts, &prepared.loaded); err != nil {
		return err
	}
	if opts.dryRun {
		return renderUpdateMultiResult(cmd, opts, prepared.result)
	}
	result, applyErr := applyPreparedUpdate(ctx, prepared)
	if renderErr := renderUpdateMultiResult(cmd, opts, result); renderErr != nil && applyErr == nil {
		applyErr = renderErr
	}
	return applyErr
}

func prepareUpdateMany(ctx context.Context, app App, opts *options, installation domain.Installation, targets []domain.ClientID, probeVersion bool) (*preparedUpdateMany, error) {
	if err := validateUpdateTargets(installation, opts.scope, targets); err != nil {
		return nil, err
	}
	detected, installedClients, loaded, err := loadUpdatePackage(ctx, app, opts, installation, targets, probeVersion)
	if err != nil {
		return nil, err
	}
	state, stateErr := app.StateStore.Load()
	if stateErr != nil {
		if loaded.cleanup != nil {
			_ = loaded.cleanup()
		}
		return nil, stateErr
	}
	prepared := &preparedUpdateMany{loaded: loaded, dryRun: opts.dryRun, state: state, installationID: installation.InstallationID}
	if err := prepared.checkSourceIdentity(installation); err != nil {
		prepared.cleanup()
		return nil, err
	}
	if err := prepared.buildInputs(app, installedClients, targets, detected); err != nil {
		prepared.cleanup()
		return nil, err
	}
	return prepared.planUpdate(ctx)
}

func validateUpdateTargets(installation domain.Installation, scope string, targets []domain.ClientID) error {
	if installation.NeedsRebind || installation.Package.LoaderKind != domain.LoaderKindAgentPlugins {
		return fmt.Errorf("update requires a bound Agent Plugins installation")
	}
	for _, target := range targets {
		if !installationHasTarget(installation, target, scope) {
			return fmt.Errorf("plugin is not installed for target %q in %s scope; no target was changed", target, scope)
		}
	}
	return nil
}

func loadUpdatePackage(ctx context.Context, app App, opts *options, installation domain.Installation, targets []domain.ClientID, probeVersion bool) (map[domain.ClientID]domain.DetectedClient, []domain.DetectedClient, loadedPackage, error) {
	// Signed compatibility retains recorded independent targets and explicit
	// selections. Implicit siblings of a native owner remain physically tracked
	// but are not independently selected targets.
	allTargets := append(directoryInstallationTargets(installation, opts.scope), targets...)
	_, detected, err := preflightProjectLifecycleTargets(ctx, app, opts, targets, nil, probeVersion && installation.OriginMode == domain.OriginModeDirectory, lifecycleInstallIntents(installation, opts.scope, nil))
	if err != nil {
		return nil, nil, loadedPackage{}, err
	}
	installedClients, err := preflightInstalledBindings(installedBindingTargets(installation, opts.scope), detected, app.scope())
	if err != nil {
		return nil, nil, loadedPackage{}, err
	}
	writeProgress(app, opts.format, "Resolving and validating one updated Agent Plugin package for every selected target...")
	loaded, err := resolveUpdatePackage(ctx, app, installation, allTargets, targets, opts.scope, detected)
	if err != nil {
		return nil, nil, loadedPackage{}, err
	}
	return detected, installedClients, loaded, nil
}

func resolveUpdatePackage(
	ctx context.Context,
	app App,
	installation domain.Installation,
	allTargets, targets []domain.ClientID,
	scope string,
	detected map[domain.ClientID]domain.DetectedClient,
) (loadedPackage, error) {
	operation := domain.DirectoryUpdate
	if selectedTargetsNeedDesiredRelease(installation, targets, scope) {
		// A prior partial rollout already accepted the installation-wide desired
		// release. Resolve that exact release so remaining bindings converge to it
		// instead of incorrectly requiring a still newer release.
		operation = domain.DirectoryNewTarget
	}
	loaded, err := app.loadInstalledPackage(ctx, installation, allTargets, operation, 0, installation.Source.ResolvedRevision, detected)
	if operation == domain.DirectoryUpdate && errors.Is(err, domain.ErrDirectoryNoSafeUpdate) {
		// "No later release" means the tracked immutable release is already the
		// newest safe choice, not that update itself is invalid. Resolve that
		// exact recorded release again so the lifecycle can report an idempotent
		// no-change result while retaining revocation and integrity checks.
		loaded, err = app.loadInstalledPackage(ctx, installation, allTargets, domain.DirectoryNewTarget, 0, installation.Source.ResolvedRevision, detected)
	}
	return loaded, err
}

func (prepared *preparedUpdateMany) checkSourceIdentity(installation domain.Installation) error {
	if installation.OriginMode != domain.OriginModeDirectory && domain.ComputeSourceBindingID(prepared.loaded.envelope.Source) != installation.Source.SourceBindingID {
		return fmt.Errorf("resolved source identity changed; use agentplugins switch after reviewing provenance")
	}
	return nil
}

func (prepared *preparedUpdateMany) buildInputs(app App, installedClients []domain.DetectedClient, targets []domain.ClientID, detected map[domain.ClientID]domain.DetectedClient) error {
	prepared.service = lifecycleService(app, detected)
	if err := prepared.appendCompatibilityInputs(installedClients, detected); err != nil {
		return err
	}
	return prepared.appendSelectedInputs(targets, detected)
}

func (prepared *preparedUpdateMany) appendCompatibilityInputs(installedClients []domain.DetectedClient, detected map[domain.ClientID]domain.DetectedClient) error {
	compatibility := make([]usecase.AddInput, 0, len(installedClients))
	for _, client := range installedClients {
		input, err := prepared.clientInput(client, detected)
		if err != nil {
			return err
		}
		compatibility = append(compatibility, input)
	}
	prepared.compatibility = compatibility
	return nil
}

func (prepared *preparedUpdateMany) appendSelectedInputs(targets []domain.ClientID, detected map[domain.ClientID]domain.DetectedClient) error {
	inputs := make([]usecase.AddInput, 0, len(targets))
	selected := make([]domain.ClientID, 0, len(targets))
	for _, target := range targets {
		input, err := prepared.clientInput(detected[target], detected)
		if err != nil {
			return err
		}
		inputs = append(inputs, input)
		selected = append(selected, target)
	}
	prepared.inputs = inputs
	prepared.selected = selected
	return nil
}

func (prepared *preparedUpdateMany) clientInput(client domain.DetectedClient, detected map[domain.ClientID]domain.DetectedClient) (usecase.AddInput, error) {
	target := client.ClientID
	clientPackage := cloneLoadedPackage(prepared.loaded)
	if err := prepareLoadedPackageForClient(&clientPackage, target); err != nil {
		return usecase.AddInput{}, fmt.Errorf("preflight target %s: %w; no target was changed", target, err)
	}
	authority, err := environmentAuthorityForBinding(prepared.state, prepared.installationID, target, "update")
	if err != nil {
		return usecase.AddInput{}, err
	}
	return usecase.AddInput{
		Envelope: clientPackage.envelope, Client: client, Scope: scopeFromProjectRoot(prepared.service.ProjectRoot),
		Interactive: false, Hints: clientPackage.hints, BackendExecutable: backendExecutable(client, detected, scopeFromProjectRoot(prepared.service.ProjectRoot)),
		OriginMode: prepared.loaded.origin, DirectoryResolution: cloneDirectoryOrigin(prepared.loaded.directory),
		DistributionSuspended: prepared.loaded.distributionSuspended, ReleaseRevoked: prepared.loaded.releaseRevoked,
		EnvironmentAuthority: authority,
	}, nil
}
