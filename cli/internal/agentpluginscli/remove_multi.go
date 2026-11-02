package agentpluginscli

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type removeTargetResult struct {
	Target string           `json:"target"`
	Status string           `json:"status"`
	Output removeResultData `json:"output"`
}

// retainedDataResult proves that owned plugin data remains without exposing
// its host-specific locator. The actionable locator remains in private state.
type retainedDataResult struct {
	DataReceiptID   string                  `json:"data_receipt_id"`
	PhysicalBackend string                  `json:"physical_backend_id"`
	Scope           string                  `json:"scope"`
	State           domain.DataReceiptState `json:"state"`
}

type removeMultiResult struct {
	OperationID         string               `json:"operation_id,omitempty"`
	Batch               bool                 `json:"batch"`
	Status              string               `json:"status"`
	Succeeded           int                  `json:"succeeded"`
	Failed              int                  `json:"failed"`
	Plugin              string               `json:"plugin"`
	PluginDataPreserved bool                 `json:"plugin_data_preserved"`
	DataRetained        bool                 `json:"data_retained"`
	RetainedData        []retainedDataResult `json:"retained_data,omitempty"`
	RetainedDataAction  string               `json:"retained_data_action,omitempty"`
	DryRun              bool                 `json:"dry_run"`
	Targets             []removeTargetResult `json:"targets"`
}

type removeManySession struct {
	ctx                  context.Context
	cmd                  *cobra.Command
	app                  App
	opts                 *options
	installation         domain.Installation
	installationSelector string
	targets              []domain.ClientID
	detected             map[domain.ClientID]domain.DetectedClient
	selected             []domain.DetectedClient
	inputs               []usecase.RemoveInput
	service              usecase.Service
	result               removeMultiResult
}

func runRemoveMany(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector string, targets []domain.ClientID) error {
	session := &removeManySession{ctx: ctx, cmd: cmd, app: app, opts: opts, targets: targets}
	if err := session.prepare(selector); err != nil {
		return err
	}
	if err := session.plan(); err != nil {
		return err
	}
	if opts.dryRun {
		return renderRemoveMultiResult(cmd, opts, session.result)
	}
	return session.apply()
}

func (session *removeManySession) prepare(selector string) error {
	state, err := session.app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return err
	}
	if installation.Package.LoaderKind == domain.LoaderKindLegacy {
		return fmt.Errorf("legacy removal is integration-wide; use --target legacy-all")
	}
	session.installation = installation
	session.installationSelector = installation.InstallationID
	if err := session.detectClients(); err != nil {
		return err
	}
	return session.buildInputs()
}

func (session *removeManySession) detectClients() error {
	clients, err := session.app.Detector.Detect(session.ctx)
	if err != nil {
		return fmt.Errorf("detect AI clients: %w", err)
	}
	detected := make(map[domain.ClientID]domain.DetectedClient, len(clients)+1)
	for _, client := range clients {
		detected[client.ClientID] = client
	}
	session.detected = detected
	session.service = lifecycleService(session.app, detected)
	session.result = removeMultiResult{
		Batch: true, Status: "planned", Plugin: session.installation.DeclaredName,
		PluginDataPreserved: !session.opts.purgeData, DryRun: session.opts.dryRun,
		Targets: make([]removeTargetResult, 0, len(session.targets)),
	}
	return nil
}

func (session *removeManySession) buildInputs() error {
	inputs := make([]usecase.RemoveInput, 0, len(session.targets))
	selected := make([]domain.DetectedClient, 0, len(session.targets))
	for _, target := range session.targets {
		if !installationHasTarget(session.installation, target, session.opts.scope) {
			return fmt.Errorf("plugin is not installed for target %q in %s scope; no target was changed", target, session.opts.scope)
		}
		if err := selectedDetectionError(target, session.detected); err != nil {
			return err
		}
		state, err := session.app.StateStore.Load()
		if err != nil {
			return err
		}
		authority, err := environmentAuthorityForBinding(state, session.installation.InstallationID, target, "remove")
		if err != nil {
			return err
		}
		client, ok := session.detected[target]
		if !ok {
			client = domain.DetectedClient{ClientID: target, DisplayName: string(target), Status: domain.DetectionNotDetected}
			session.detected[target] = client
		}
		inputs = append(inputs, usecase.RemoveInput{
			Selector: session.installationSelector, Client: client, Scope: session.app.scope(),
			DryRun: true, Interactive: false, ExternalUninstalled: session.opts.externalUninstalled,
			BackendExecutable:    backendExecutable(client, session.detected, session.app.scope()),
			EnvironmentAuthority: authority,
		})
		selected = append(selected, client)
	}
	session.inputs = inputs
	session.selected = selected
	return nil
}

func (session *removeManySession) plan() error {
	operationID, err := newOperationGroupID()
	if err != nil {
		return err
	}
	session.result.OperationID = operationID
	plannedGroup, planErr := session.service.RemoveGroup(session.ctx, usecase.RemoveGroupInput{
		Selector: session.installationSelector, Targets: session.inputs, OperationGroupID: operationID,
		DryRun: true, PurgeData: session.opts.purgeData,
	})
	session.appendPlannedTargets(plannedGroup.Targets, operationID)
	session.result.Succeeded = len(plannedGroup.Targets)
	if planErr != nil {
		session.result.Status, session.result.Failed, session.result.Succeeded = "preflight_failed", len(session.inputs), 0
		_ = renderRemoveMultiResult(session.cmd, session.opts, session.result)
		return fmt.Errorf("group remove preflight failed; no target was changed: %w", planErr)
	}
	blocked := session.blockedRemovalTargets(plannedGroup.Targets)
	if len(blocked) > 0 && !session.opts.dryRun {
		session.result.Status, session.result.Failed, session.result.Succeeded = "manual_step_required", len(session.inputs), 0
		for index := range session.result.Targets {
			if session.result.Targets[index].Status == "planned" {
				session.result.Targets[index].Status = "not_applied"
			}
		}
		if err := renderRemoveMultiResult(session.cmd, session.opts, session.result); err != nil {
			return fmt.Errorf("render grouped removal result: %w", err)
		}
		return fmt.Errorf("removal requires a manual client step for %v; complete the displayed uninstall step, then rerun with --external-uninstalled; no target was changed", blocked)
	}
	return nil
}

func (session *removeManySession) blockedRemovalTargets(planned []usecase.RemoveResult) []domain.ClientID {
	blocked := make([]domain.ClientID, 0, len(planned))
	for index, target := range planned {
		if !target.Deactivation.ArtifactRemovalAllowed {
			blocked = append(blocked, session.selected[index].ClientID)
		}
	}
	return blocked
}

func (session *removeManySession) appendPlannedTargets(planned []usecase.RemoveResult, operationID string) {
	for index, target := range planned {
		status := "planned"
		if !target.Deactivation.ArtifactRemovalAllowed {
			status = "blocked"
		}
		output := newRemoveResultData(session.installation, target, true)
		output.OperationID = operationID
		session.result.Targets = append(session.result.Targets, removeTargetResult{Target: string(session.selected[index].ClientID), Status: status, Output: output})
	}
}
