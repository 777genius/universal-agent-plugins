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
		client, ok := session.detected[target]
		if !ok {
			client = domain.DetectedClient{ClientID: target, DisplayName: string(target), Status: domain.DetectionNotDetected}
			session.detected[target] = client
		}
		inputs = append(inputs, usecase.RemoveInput{
			Selector: session.installationSelector, Client: client, Scope: domain.ScopeUser,
			DryRun: true, Interactive: false, ExternalUninstalled: session.opts.externalUninstalled,
			BackendExecutable: backendExecutable(client, session.detected),
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
	return nil
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

func (session *removeManySession) apply() error {
	session.result.Status = "applying"
	session.result.Succeeded = 0
	session.result.Failed = 0
	session.result.Targets = session.result.Targets[:0]
	for index := range session.inputs {
		session.inputs[index].Confirmed = true
	}
	appliedGroup, groupErr := session.service.RemoveGroup(session.ctx, usecase.RemoveGroupInput{
		Selector: session.installationSelector, Targets: session.inputs, OperationGroupID: session.result.OperationID,
		Confirmed: true, PurgeData: session.opts.purgeData,
	})
	appliedResults := appliedGroup.Targets
	if len(appliedResults) > len(session.inputs) || len(appliedResults) != len(session.inputs) && groupErr == nil {
		return fmt.Errorf("install engine returned %d remove results for %d targets", len(appliedResults), len(session.inputs))
	}
	session.appendAppliedTargets(appliedResults)
	if groupErr != nil {
		session.result.Status, session.result.Failed = groupFailureStatus(appliedGroup.Phase), len(session.inputs)-session.result.Succeeded
		_ = renderRemoveMultiResult(session.cmd, session.opts, session.result)
		return groupErr
	}
	session.result.Status = string(appliedGroup.Phase)
	session.recordRetainedData()
	return renderRemoveMultiResult(session.cmd, session.opts, session.result)
}

func (session *removeManySession) appendAppliedTargets(appliedResults []usecase.RemoveResult) {
	for index, applied := range appliedResults {
		output := newRemoveResultData(session.installation, applied, false)
		output.OperationID = session.result.OperationID
		status := string(applied.GroupPhase)
		if status == "" {
			status = "apply_failed"
		}
		session.result.Targets = append(session.result.Targets, removeTargetResult{Target: string(session.selected[index].ClientID), Status: status, Output: output})
		if applied.GroupPhase == usecase.GroupTargetExternalCompleted {
			session.result.Succeeded++
		}
	}
}

func (session *removeManySession) recordRetainedData() {
	after, loadErr := session.app.StateStore.Load()
	if loadErr != nil {
		return
	}
	retained, ok := locallyMatchedInstallation(after, session.installation.InstallationID)
	if !ok || !retained.DataRetained {
		return
	}
	session.result.DataRetained = true
	session.result.Status = "data_retained"
	for _, receipt := range retained.DataReceipts {
		session.result.RetainedData = append(session.result.RetainedData, retainedDataResult{
			DataReceiptID: receipt.DataReceiptID, PhysicalBackend: receipt.PhysicalBackend,
			Scope: receipt.Scope, State: receipt.State,
		})
	}
	session.result.RetainedDataAction = fmt.Sprintf("run `agentplugins remove %s --purge-data` to delete retained plugin data", session.installation.InstallationID)
}

func renderRemoveMultiResult(cmd *cobra.Command, opts *options, result removeMultiResult) error {
	if opts.format == "json" {
		overall := "success"
		if result.Failed > 0 {
			overall = "failure"
		}
		return writeJSONResult(cmd.OutOrStdout(), "remove", overall, result)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Remove %s: %s\n", result.Plugin, result.Status); err != nil {
		return err
	}
	for _, target := range result.Targets {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", target.Target, target.Status); err != nil {
			return err
		}
	}
	if result.PluginDataPreserved {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "  PLUGIN_DATA: preserved"); err != nil {
			return err
		}
	}
	if result.RetainedDataAction != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Next: %s\n", result.RetainedDataAction); err != nil {
			return err
		}
	}
	return nil
}
