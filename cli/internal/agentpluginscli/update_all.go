package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type updateAllInstallation struct {
	Saved          bool                  `json:"saved,omitempty"`
	Actions        []dependencyActionDTO `json:"actions,omitempty"`
	InstallationID string                `json:"installation_id"`
	Name           string                `json:"name"`
	Status         string                `json:"status"`
	Reason         string                `json:"reason,omitempty"`
	Plan           *updateMultiResult    `json:"plan,omitempty"`
}

type updateAllResult struct {
	Batch         bool                    `json:"batch"`
	Status        string                  `json:"status"`
	DryRun        bool                    `json:"dry_run"`
	Planned       int                     `json:"planned"`
	Updated       int                     `json:"updated"`
	Skipped       int                     `json:"skipped"`
	Failed        int                     `json:"failed"`
	Installations []updateAllInstallation `json:"installations"`
}

type preparedUpdateAllItem struct {
	installation domain.Installation
	prepared     *preparedUpdateMany
	resultIndex  int
}

type updateAllSession struct {
	saved           *preparedSavedUpdateAll
	ctx             context.Context
	cmd             *cobra.Command
	app             App
	opts            *options
	installations   []domain.Installation
	ownedAliases    map[string][]string
	result          updateAllResult
	preparedItems   []preparedUpdateAllItem
	preflightFailed bool
	preflightCauses []error
	bundle          directoryv1.VerifiedBundle
	bundleOK        bool
	detected        map[domain.ClientID]domain.DetectedClient
	detectionErr    error
}

func runUpdateAll(ctx context.Context, cmd *cobra.Command, app App, opts *options) error {
	staging := *opts
	staging.dryRun = true
	preview, cleanup, err := dependencyPreviewApp(app, &staging)
	if err != nil {
		return err
	}
	defer cleanup()
	session := &updateAllSession{ctx: ctx, cmd: cmd, app: preview, opts: opts}
	empty, err := session.load()
	if err != nil {
		return err
	}
	if empty {
		return renderUpdateAll(cmd, opts, session.result)
	}
	defer session.cleanupPrepared()
	if err := session.preflightAll(); err != nil {
		return err
	}
	if opts.dryRun {
		return renderUpdateAll(cmd, opts, session.result)
	}
	return session.applyAll()
}

func (session *updateAllSession) load() (bool, error) {
	state, err := session.app.StateStore.Load()
	if err != nil {
		return false, err
	}
	installations := append([]domain.Installation(nil), state.Installations...)
	sort.Slice(installations, func(i, j int) bool {
		left, right := strings.ToLower(installations[i].DeclaredName), strings.ToLower(installations[j].DeclaredName)
		if left != right {
			return left < right
		}
		return installations[i].InstallationID < installations[j].InstallationID
	})
	session.installations = installations
	session.ownedAliases = map[string][]string{}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID != session.app.environmentID() {
			continue
		}
		aliases := session.ownedAliases[receipt.InstallationID]
		found := false
		for _, alias := range aliases {
			if alias == receipt.Alias {
				found = true
			}
		}
		if !found {
			session.ownedAliases[receipt.InstallationID] = append(aliases, receipt.Alias)
		}
	}
	unmanaged := make([]domain.Installation, 0, len(installations))
	for _, installation := range installations {
		if len(session.ownedAliases[installation.InstallationID]) == 0 {
			unmanaged = append(unmanaged, installation)
		}
	}
	session.result = updateAllResult{
		Batch: true, Status: "planned", DryRun: session.opts.dryRun,
		Installations: make([]updateAllInstallation, 0, len(installations)),
	}
	manifest, _ := session.app.dependencyPaths()
	_, manifestErr := os.Stat(manifest)
	if len(installations) == 0 && os.IsNotExist(manifestErr) {
		session.result.Status = "no_installations"
		return true, nil
	}
	bundle, bundleOK, detected, detectionErr, err := updateAllDirectoryContext(session.ctx, session.app, state, unmanaged)
	if err != nil {
		return false, err
	}
	session.bundle, session.bundleOK, session.detected, session.detectionErr = bundle, bundleOK, detected, detectionErr
	session.preparedItems = make([]preparedUpdateAllItem, 0, len(installations))
	return false, nil
}

func (session *updateAllSession) cleanupPrepared() {
	if session.saved != nil {
		session.saved.cleanup()
	}
	for index := range session.preparedItems {
		session.preparedItems[index].prepared.cleanup()
	}
}

func (session *updateAllSession) preflightAll() error {
	if err := session.prepareSaved(); err != nil {
		session.preflightFailed = true
		session.preflightCauses = append(session.preflightCauses, err)
		session.result.Failed++
		session.result.Installations = append(session.result.Installations, updateAllInstallation{Name: "saved environment", Saved: true, Status: "preflight_failed", Reason: err.Error()})
	}
	for _, installation := range session.installations {
		session.classifyInstallation(installation)
	}
	if !session.preflightFailed {
		if err := session.checkPrepared(); err != nil {
			session.preflightFailed = true
			session.preflightCauses = append(session.preflightCauses, err)
			session.result.Failed++
			session.result.Installations = append(session.result.Installations, updateAllInstallation{Name: "batch", Status: "preflight_failed", Reason: err.Error()})
		}
	}
	if session.preflightFailed {
		session.result.Status = "preflight_failed"
		renderErr := renderUpdateAll(session.cmd, session.opts, session.result)
		return installFailureWithCause{
			summary: "update --all preflight failed; no installation was changed",
			// This batch owns the attempted result, including a partial write.
			cause: &declaredInstallResultError{cause: errors.Join(
				errors.Join(session.preflightCauses...), installOutputError(renderErr),
			)},
		}
	}
	return nil
}

func (session *updateAllSession) classifyInstallation(installation domain.Installation) {
	if session.savedContains(installation.InstallationID) {
		return
	}
	item := updateAllInstallation{InstallationID: installation.InstallationID, Name: installation.DeclaredName}
	if aliases := session.ownedAliases[installation.InstallationID]; len(aliases) > 0 {
		sort.Strings(aliases)
		actions := make([]string, 0, len(aliases))
		for _, alias := range aliases {
			actions = append(actions, "agentplugins update "+alias)
		}
		item.Status, item.Reason = "skipped", "saved dependency; run "+strings.Join(actions, " or ")
		session.result.Skipped++
		session.result.Installations = append(session.result.Installations, item)
		return
	}
	if session.skipDirectoryStatus(installation, &item) {
		session.result.Installations = append(session.result.Installations, item)
		return
	}
	targets := installationTargets(installation, session.opts.scope)
	if installation.OriginMode == domain.OriginModeDirectory {
		// A batch selects recorded owners, not their implicit physical siblings.
		targets = installedBindingTargets(installation, session.opts.scope)
	}
	// Keep every imperative preview process-inert until the complete mixed batch passes.
	previewOpts := *session.opts
	previewOpts.dryRun = true
	previewApp := session.app
	previewApp.Lifecycle.PrepareHostsForPreview = false
	prepared, prepareErr := prepareUpdateMany(session.ctx, previewApp, &previewOpts, installation, targets, false)
	if prepared != nil {
		prepared.dryRun = session.opts.dryRun
		prepared.result.DryRun = session.opts.dryRun
	}
	if prepareErr != nil {
		session.recordPreflightFailure(&item, prepared, prepareErr)
		return
	}
	if prepared.noChange {
		session.recordSkipCurrent(prepared, &item)
		return
	}
	if err := session.authorizePrepared(prepared); err != nil {
		session.recordPreflightFailure(&item, prepared, err)
		return
	}
	session.recordPlanned(installation, prepared, &item)
}

// Keep the batch response as the sole JSON owner while reusing the ordinary
// security decision with the original input, context and human output channels.
func (session *updateAllSession) authorizePrepared(prepared *preparedUpdateMany) error {
	for index := range prepared.result.Targets {
		prepared.result.Targets[index].Output.Security = prepared.loaded.security
	}
	command := session.cmd
	if session.opts.format == "json" {
		command = &cobra.Command{Use: session.cmd.Name()}
		command.SetContext(session.cmd.Context())
		command.SetIn(session.cmd.InOrStdin())
		command.SetOut(io.Discard)
		command.SetErr(session.cmd.ErrOrStderr())
	}
	return authorizeSecurityAssessment(command, session.app, session.opts, &prepared.loaded)
}

func (session *updateAllSession) skipDirectoryStatus(installation domain.Installation, item *updateAllInstallation) bool {
	if installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil {
		return false
	}
	status := inspectOutdatedInstallation(session.bundle, session.bundleOK, session.detected, session.detectionErr, session.app.Version, installation, session.opts.scope)
	if status.Status != "blocked" && status.Status != "unknown" {
		return false
	}
	item.Status, item.Reason = "preflight_failed", status.Reason
	session.result.Failed++
	session.preflightFailed = true
	if status.Status == "unknown" && session.bundleOK && session.detectionErr != nil {
		session.preflightCauses = append(session.preflightCauses, session.detectionErr)
	}
	return true
}

func (session *updateAllSession) recordPreflightFailure(item *updateAllInstallation, prepared *preparedUpdateMany, prepareErr error) {
	item.Status, item.Reason = "preflight_failed", prepareErr.Error()
	session.preflightCauses = append(session.preflightCauses, prepareErr)
	if prepared != nil {
		plan := prepared.result
		item.Plan = &plan
		prepared.cleanup()
	}
	session.result.Failed++
	session.preflightFailed = true
	session.result.Installations = append(session.result.Installations, *item)
}

func (session *updateAllSession) recordSkipCurrent(prepared *preparedUpdateMany, item *updateAllInstallation) {
	item.Status, item.Reason = "skipped", "installed package and every selected target are current"
	plan := prepared.result
	item.Plan = &plan
	prepared.cleanup()
	session.result.Skipped++
	session.result.Installations = append(session.result.Installations, *item)
}

func (session *updateAllSession) recordPlanned(installation domain.Installation, prepared *preparedUpdateMany, item *updateAllInstallation) {
	item.Status = "planned"
	plan := prepared.result
	item.Plan = &plan
	resultIndex := len(session.result.Installations)
	session.result.Installations = append(session.result.Installations, *item)
	session.preparedItems = append(session.preparedItems, preparedUpdateAllItem{installation: installation, prepared: prepared, resultIndex: resultIndex})
	session.result.Planned++
}

func (session *updateAllSession) applyAll() error {
	session.result.Status = "completed"
	if err := session.applySaved(); err != nil {
		session.recordSavedPreApplyFailure()
		markUnattemptedUpdateAll(&session.result, session.preparedItems, 0)
		renderErr := renderUpdateAll(session.cmd, session.opts, session.result)
		return errors.Join(humanInstallApplyError(session.opts.format, session.opts.installResult, err), installOutputError(renderErr))
	}
	for index := range session.preparedItems {
		item := &session.preparedItems[index]
		if err := checkPreparedImperativeUpdate(session.ctx, item.prepared); err != nil {
			return session.failImperativeUpdate(index, nil, err)
		}
		applied, applyErr := applyPreparedUpdate(session.ctx, item.prepared)
		view := &session.result.Installations[item.resultIndex]
		view.Plan = &applied
		if applyErr != nil {
			return session.failImperativeUpdate(index, &applied, applyErr)
		}
		if selectedUpdatesUnchanged(applied) {
			view.Status, view.Reason = "skipped", "installed package and every selected target are current"
			session.result.Skipped++
		} else {
			view.Status = "updated"
			session.result.Updated++
		}
	}
	return renderUpdateAll(session.cmd, session.opts, session.result)
}

func selectedUpdatesUnchanged(applied updateMultiResult) bool {
	selected := false
	for _, target := range applied.Targets {
		if !target.Selected {
			continue
		}
		selected = true
		if !target.Output.Result.NoChange || target.Output.Result.Mutated {
			return false
		}
	}
	return selected
}

func markUnattemptedUpdateAll(result *updateAllResult, prepared []preparedUpdateAllItem, start int) {
	for index := start; index < len(prepared); index++ {
		view := &result.Installations[prepared[index].resultIndex]
		view.Status = "not_attempted"
		view.Reason = "batch stopped after an earlier apply failure"
	}
}

func updateAllDirectoryContext(ctx context.Context, app App, state domain.StateFileV2, installations []domain.Installation) (directoryv1.VerifiedBundle, bool, map[domain.ClientID]domain.DetectedClient, error, error) {
	needsDirectory := false
	for _, installation := range installations {
		if installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil {
			needsDirectory = true
			break
		}
	}
	if !needsDirectory {
		return directoryv1.VerifiedBundle{}, false, nil, nil, nil
	}
	bundle, ok, err := directoryBundleForRead(ctx, app, state, true)
	if err != nil {
		return directoryv1.VerifiedBundle{}, false, nil, nil, fmt.Errorf("load signed Directory: %w", err)
	}
	if !ok {
		return directoryv1.VerifiedBundle{}, false, nil, nil, nil
	}
	return detectUpdateAllClients(ctx, app, bundle)
}

func detectUpdateAllClients(ctx context.Context, app App, bundle directoryv1.VerifiedBundle) (directoryv1.VerifiedBundle, bool, map[domain.ClientID]domain.DetectedClient, error, error) {
	clients, err := detectClientsForLifecycleResolution(ctx, app.Detector, false)
	if err != nil {
		return bundle, true, nil, fmt.Errorf("detect AI clients: %w", err), nil
	}
	detected := make(map[domain.ClientID]domain.DetectedClient, len(clients))
	for _, client := range clients {
		detected[client.ClientID] = client
	}
	return bundle, true, detected, nil, nil
}

func renderUpdateAll(cmd *cobra.Command, opts *options, result updateAllResult) error {
	if opts.format == "json" {
		overall := "success"
		if result.Failed > 0 {
			overall = "failure"
		}
		return writeJSONResult(cmd.OutOrStdout(), "update", overall, result)
	}
	if err := renderUpdateAllHuman(cmd.OutOrStdout(), updateAllHumanFailureResult(result, opts.installResult)); err != nil {
		return installOutputError(err)
	}
	return renderInstallAttention(cmd, opts)
}

func renderUpdateAllHuman(writer io.Writer, result updateAllResult) error {
	for _, installation := range result.Installations {
		if err := renderUpdateAllInstallation(writer, installation); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(writer, "Update all: %s (planned=%d updated=%d skipped=%d failed=%d)\n",
		result.Status, result.Planned, result.Updated, result.Skipped, result.Failed)
	return err
}

func renderUpdateAllInstallation(writer io.Writer, installation updateAllInstallation) error {
	for _, action := range installation.Actions {
		if action.Action != "update" {
			continue
		}
		from, to := dependencyTransition(action)
		if _, err := fmt.Fprintf(writer, "  %s/%s: %s → %s\n", action.Alias, action.ClientID, from, to); err != nil {
			return err
		}
	}
	if installation.Plan != nil {
		for _, target := range installation.Plan.Targets {
			if err := renderOpenCodeRuntimeNotice(writer, target.Output.Result); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(writer, "%s: %s", installation.Name, installation.Status); err != nil {
		return err
	}
	if installation.Reason != "" {
		if _, err := fmt.Fprintf(writer, " - %s", installation.Reason); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(writer)
	return err
}
