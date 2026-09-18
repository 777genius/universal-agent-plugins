package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type updateAllInstallation struct {
	InstallationID string             `json:"installation_id"`
	Name           string             `json:"name"`
	Status         string             `json:"status"`
	Reason         string             `json:"reason,omitempty"`
	Plan           *updateMultiResult `json:"plan,omitempty"`
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
	ctx             context.Context
	cmd             *cobra.Command
	app             App
	opts            *options
	installations   []domain.Installation
	result          updateAllResult
	preparedItems   []preparedUpdateAllItem
	preflightFailed bool
	bundle          directoryv1.VerifiedBundle
	bundleOK        bool
	detected        map[domain.ClientID]domain.DetectedClient
	detectionErr    error
}

func runUpdateAll(ctx context.Context, cmd *cobra.Command, app App, opts *options) error {
	session := &updateAllSession{ctx: ctx, cmd: cmd, app: app, opts: opts}
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
	session.result = updateAllResult{
		Batch: true, Status: "planned", DryRun: session.opts.dryRun,
		Installations: make([]updateAllInstallation, 0, len(installations)),
	}
	if len(installations) == 0 {
		session.result.Status = "no_installations"
		return true, nil
	}
	bundle, bundleOK, detected, detectionErr, err := updateAllDirectoryContext(session.ctx, session.app, state, installations)
	if err != nil {
		return false, err
	}
	session.bundle, session.bundleOK, session.detected, session.detectionErr = bundle, bundleOK, detected, detectionErr
	session.preparedItems = make([]preparedUpdateAllItem, 0, len(installations))
	return false, nil
}

func (session *updateAllSession) cleanupPrepared() {
	for index := range session.preparedItems {
		session.preparedItems[index].prepared.cleanup()
	}
}

func (session *updateAllSession) preflightAll() error {
	for _, installation := range session.installations {
		session.classifyInstallation(installation)
	}
	if session.preflightFailed {
		session.result.Status = "preflight_failed"
		_ = renderUpdateAll(session.cmd, session.opts, session.result)
		return fmt.Errorf("update --all preflight failed; no installation was changed")
	}
	return nil
}

func (session *updateAllSession) classifyInstallation(installation domain.Installation) {
	item := updateAllInstallation{InstallationID: installation.InstallationID, Name: installation.DeclaredName}
	if session.skipDirectoryStatus(installation, &item) {
		session.result.Installations = append(session.result.Installations, item)
		return
	}
	targets := installationTargets(installation, session.opts.scope)
	prepared, prepareErr := prepareUpdateMany(session.ctx, session.app, session.opts, installation, targets, !session.opts.dryRun)
	if prepareErr != nil {
		session.recordPreflightFailure(&item, prepared, prepareErr)
		return
	}
	if prepared.noChange {
		session.recordSkipCurrent(prepared, &item)
		return
	}
	session.recordPlanned(installation, prepared, &item)
}

func (session *updateAllSession) skipDirectoryStatus(installation domain.Installation, item *updateAllInstallation) bool {
	if installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil {
		return false
	}
	status := inspectOutdatedInstallation(session.bundle, session.bundleOK, session.detected, session.detectionErr, session.app.Version, installation, session.opts.scope)
	switch status.Status {
	case "current":
		item.Status, item.Reason = "skipped", status.Reason
		session.result.Skipped++
		return true
	case "blocked", "unknown":
		item.Status, item.Reason = "preflight_failed", status.Reason
		session.result.Failed++
		session.preflightFailed = true
		return true
	}
	return false
}

func (session *updateAllSession) recordPreflightFailure(item *updateAllInstallation, prepared *preparedUpdateMany, prepareErr error) {
	item.Status, item.Reason = "preflight_failed", prepareErr.Error()
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
	for index := range session.preparedItems {
		item := &session.preparedItems[index]
		applied, applyErr := applyPreparedUpdate(session.ctx, item.prepared)
		view := &session.result.Installations[item.resultIndex]
		view.Plan = &applied
		if applyErr != nil {
			view.Status, view.Reason = "apply_failed", applyErr.Error()
			session.result.Failed++
			session.result.Status = "partial_failure"
			markUnattemptedUpdateAll(&session.result, session.preparedItems, index+1)
			_ = renderUpdateAll(session.cmd, session.opts, session.result)
			return applyErr
		}
		view.Status = "updated"
		session.result.Updated++
	}
	return renderUpdateAll(session.cmd, session.opts, session.result)
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
	return renderUpdateAllHuman(cmd.OutOrStdout(), result)
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
