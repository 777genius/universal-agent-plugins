package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/spf13/cobra"
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

func runUpdateAll(ctx context.Context, cmd *cobra.Command, app App, opts *options) error {
	state, err := app.StateStore.Load()
	if err != nil {
		return err
	}
	installations := append([]domain.Installation(nil), state.Installations...)
	sort.Slice(installations, func(i, j int) bool {
		left, right := strings.ToLower(installations[i].DeclaredName), strings.ToLower(installations[j].DeclaredName)
		if left != right {
			return left < right
		}
		return installations[i].InstallationID < installations[j].InstallationID
	})
	result := updateAllResult{Batch: true, Status: "planned", DryRun: opts.dryRun,
		Installations: make([]updateAllInstallation, 0, len(installations))}
	if len(installations) == 0 {
		result.Status = "no_installations"
		return renderUpdateAll(cmd, opts, result)
	}

	bundle, bundleOK, detected, detectionErr, err := updateAllDirectoryContext(ctx, app, state, installations)
	if err != nil {
		return err
	}
	preparedItems := make([]preparedUpdateAllItem, 0, len(installations))
	defer func() {
		for index := range preparedItems {
			preparedItems[index].prepared.cleanup()
		}
	}()

	preflightFailed := false
	for _, installation := range installations {
		item := updateAllInstallation{InstallationID: installation.InstallationID, Name: installation.DeclaredName}
		if installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil {
			status := inspectOutdatedInstallation(bundle, bundleOK, detected, detectionErr, app.Version, installation, opts.scope)
			switch status.Status {
			case "current":
				item.Status, item.Reason = "skipped", status.Reason
				result.Skipped++
				result.Installations = append(result.Installations, item)
				continue
			case "blocked", "unknown":
				item.Status, item.Reason = "preflight_failed", status.Reason
				result.Failed++
				preflightFailed = true
				result.Installations = append(result.Installations, item)
				continue
			}
		}
		targets := installationTargets(installation, opts.scope)
		prepared, prepareErr := prepareUpdateMany(ctx, app, opts, installation, targets, !opts.dryRun)
		if prepareErr != nil {
			item.Status, item.Reason = "preflight_failed", prepareErr.Error()
			if prepared != nil {
				plan := prepared.result
				item.Plan = &plan
				prepared.cleanup()
			}
			result.Failed++
			preflightFailed = true
			result.Installations = append(result.Installations, item)
			continue
		}
		if prepared.noChange {
			item.Status, item.Reason = "skipped", "installed package and every selected target are current"
			plan := prepared.result
			item.Plan = &plan
			prepared.cleanup()
			result.Skipped++
			result.Installations = append(result.Installations, item)
			continue
		}
		item.Status = "planned"
		plan := prepared.result
		item.Plan = &plan
		resultIndex := len(result.Installations)
		result.Installations = append(result.Installations, item)
		preparedItems = append(preparedItems, preparedUpdateAllItem{installation: installation, prepared: prepared, resultIndex: resultIndex})
		result.Planned++
	}
	if preflightFailed {
		result.Status = "preflight_failed"
		_ = renderUpdateAll(cmd, opts, result)
		return fmt.Errorf("update --all preflight failed; no installation was changed")
	}
	if opts.dryRun {
		return renderUpdateAll(cmd, opts, result)
	}
	result.Status = "completed"
	for index := range preparedItems {
		item := &preparedItems[index]
		applied, applyErr := applyPreparedUpdate(ctx, item.prepared)
		view := &result.Installations[item.resultIndex]
		view.Plan = &applied
		if applyErr != nil {
			view.Status, view.Reason = "apply_failed", applyErr.Error()
			result.Failed++
			result.Status = "partial_failure"
			markUnattemptedUpdateAll(&result, preparedItems, index+1)
			_ = renderUpdateAll(cmd, opts, result)
			return applyErr
		}
		view.Status = "updated"
		result.Updated++
	}
	return renderUpdateAll(cmd, opts, result)
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
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(writer, "Update all: %s (planned=%d updated=%d skipped=%d failed=%d)\n",
		result.Status, result.Planned, result.Updated, result.Skipped, result.Failed)
	return err
}
