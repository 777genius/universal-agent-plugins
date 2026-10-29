package agentpluginscli

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type preparedSavedUpdateAll struct {
	app     App
	request dependencySyncRequest
	cleanup func()
	rows    map[string]int
}

func (session *updateAllSession) prepareSaved() error {
	manifest, _ := session.app.dependencyPaths()
	if _, err := os.Stat(manifest); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	staging := *session.opts
	staging.dryRun = true
	preview, cleanup, err := dependencyPreviewApp(session.app, &staging)
	if err != nil {
		return err
	}
	owned := false
	defer func() {
		if !owned {
			cleanup()
		}
	}()
	snapshot, err := snapshotEnvironmentContext(session.ctx, preview, false)
	if err != nil {
		return err
	}
	request, aliases, err := session.prepareSavedGraph(preview, snapshot)
	if err != nil {
		return err
	}
	if len(aliases) == 0 {
		return nil
	}
	session.saved = &preparedSavedUpdateAll{app: preview, request: request, cleanup: cleanup, rows: map[string]int{}}
	owned = true
	session.appendSavedRows(request, aliases)
	return nil
}

func (session *updateAllSession) prepareSavedGraph(preview App, snapshot environmentSnapshot) (dependencySyncRequest, []string, error) {
	aliases := make([]string, 0, len(snapshot.Requirements.Dependencies))
	for alias := range snapshot.Requirements.Dependencies {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	if len(aliases) == 0 {
		return dependencySyncRequest{}, aliases, nil
	}
	requirements, locked, err := prepareSavedUpdates(session.ctx, preview, snapshot, aliases, nil)
	if err != nil {
		return dependencySyncRequest{}, aliases, err
	}
	body, err := editEnvironmentIntent(snapshot, requirements)
	if err != nil {
		return dependencySyncRequest{}, aliases, err
	}
	request, err := prepareSavedSync(session.cmd, preview, session.opts, snapshot, requirements, locked, body, declaredNodeAliases(requirements, locked, aliases), nil, nil)
	return request, aliases, err
}

func (session *updateAllSession) appendSavedRows(request dependencySyncRequest, aliases []string) {
	for _, action := range request.prepared {
		row, exists := session.saved.rows[action.Alias]
		if !exists {
			item := savedUpdateAllRow(action, request, aliases)
			row = len(session.result.Installations)
			session.saved.rows[action.Alias] = row
			session.result.Installations = append(session.result.Installations, item)
		}
		item := &session.result.Installations[row]
		item.Actions = append(item.Actions, dependencyActionOutput([]dependencyAction{action}, session.opts.dependencyComponents)...)
		if action.Action != environment.ActionUnchanged {
			item.Status = "planned"
			item.Reason = "saved dependency graph prepared"
		}
	}
	for _, row := range session.saved.rows {
		if session.result.Installations[row].Status == "planned" {
			session.result.Planned++
		} else {
			session.result.Skipped++
		}
	}
}

func savedUpdateAllRow(action dependencyAction, request dependencySyncRequest, aliases []string) updateAllInstallation {
	item := updateAllInstallation{InstallationID: action.InstallationID, Name: action.Alias, Saved: true, Status: "skipped", Reason: "saved dependency is current"}
	for _, alias := range aliases {
		if dependencyNodeAlias(request.requirements, request.lock, alias) == action.Alias && savedRequirementPinned(request.requirements.Dependencies[alias]) {
			item.Reason = "exact intent retained"
		}
	}
	return item
}

func (session *updateAllSession) savedContains(id string) bool {
	if session.saved == nil || id == "" {
		return false
	}
	for _, row := range session.saved.rows {
		if session.result.Installations[row].InstallationID == id {
			return true
		}
	}
	return false
}
func (session *updateAllSession) checkPrepared() error {
	if session.saved != nil {
		request := session.saved.request
		if err := request.checkInputs(); err != nil {
			return err
		}
		actions, err := request.recheck(session.cmd, session.saved.app, session.opts)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(actions, request.prepared) {
			return installer.ErrPlanChanged
		}
	}
	for _, item := range session.preparedItems {
		if err := checkPreparedImperativeUpdate(session.ctx, item.prepared); err != nil {
			return err
		}
	}
	return session.ctx.Err()
}
func checkPreparedImperativeUpdate(ctx context.Context, prepared *preparedUpdateMany) error {
	state, err := prepared.service.StateStore.Load()
	if err != nil {
		return err
	}
	matching := false
	for _, old := range prepared.state.Installations {
		if old.InstallationID != prepared.installationID {
			continue
		}
		for _, current := range state.Installations {
			if current.InstallationID == old.InstallationID && reflect.DeepEqual(current, old) {
				matching = true
			}
		}
	}
	if !matching {
		return installer.ErrPlanChanged
	}
	// State equality cannot prove the managed tree, physical profile or native
	// planning facts stayed unchanged. Reuse the process-inert group boundary.
	reviewed, err := prepared.service.UpdateGroup(ctx, usecase.GroupInput{Targets: prepared.inputs, CompatibilityChecks: prepared.compatibility, OperationGroupID: prepared.result.OperationID, DryRun: true})
	if err != nil {
		return err
	}
	if len(reviewed.Targets) != len(prepared.result.Targets) {
		return installer.ErrPlanChanged
	}
	for i, target := range reviewed.Targets {
		if !reflect.DeepEqual(target.Plan, prepared.result.Targets[i].Output.Result.Plan) {
			return installer.ErrPlanChanged
		}
	}
	return nil
}

func (session *updateAllSession) applySaved() error {
	if session.saved == nil {
		return nil
	}
	saved := session.saved
	if dependencySyncUnchanged(session.ctx, saved.app, saved.request) {
		return nil
	}
	confirmed, err := confirmDependencySync(session.cmd, session.app, session.opts, mutationConfirmed(session.app, session.opts))
	if err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("update all canceled before effects")
	}
	adapter := &dependencyInstaller{app: saved.app, opts: session.opts, detected: saved.request.detected, cmd: session.cmd, consent: saved.request.consent}
	applied, applyErr := applyPreparedDependencies(session.ctx, saved.app, &saved.request, adapter, func(_ context.Context) error { return session.checkPrepared() })
	session.recordSavedBindings(applied)
	if applyErr != nil {
		session.result.Status = "partial_failure"
		if !applied.Published && len(applied.Results) == 0 {
			session.result.Status = "apply_failed"
		}
		// Publication itself is a durable effect even if every binding was unattempted.
		if applied.Published {
			for _, row := range saved.rows {
				item := &session.result.Installations[row]
				if item.Status == "not_attempted" {
					item.Reason = "saved intent was published; materialization not attempted; inspect and run deps sync"
				}
			}
		}
		if session.result.Failed == 0 {
			session.result.Failed++
		}
		return applyErr
	}
	return nil
}

func (session *updateAllSession) recordSavedBindings(applied dependencyApplicationResult) {
	cursor := 0
	for _, action := range session.saved.request.prepared {
		row := session.saved.rows[action.Alias]
		item := &session.result.Installations[row]
		if action.Action == environment.ActionUnchanged {
			continue
		}
		if cursor >= len(applied.Results) {
			if item.Status == "updated" {
				item.Status = "apply_failed"
				item.Reason = "saved dependency was only partially materialized; inspect binding receipts and run deps sync or recovery"
			} else if item.Status != "apply_failed" {
				item.Status = "not_attempted"
				item.Reason = "saved graph materialization stopped before this binding"
			}
			continue
		}
		result := applied.Results[cursor]
		cursor++
		if result.Outcome != installer.OutcomeCompleted && result.Outcome != installer.OutcomeUnchanged {
			item.Status = "apply_failed"
			item.Reason = "saved intent may have been published; inspect binding receipts and run deps sync or recovery"
		} else if item.Status != "apply_failed" {
			item.Status = "updated"
			item.Reason = "saved dependency bindings updated"
		}
	}
	for _, row := range session.saved.rows {
		item := &session.result.Installations[row]
		if item.Status == "updated" {
			session.result.Updated++
		}
		if item.Status == "apply_failed" {
			session.result.Failed++
		}
	}
}

func (session *updateAllSession) recordSavedPreApplyFailure() {
	if session.result.Status != "completed" {
		return
	}
	session.result.Status = "apply_failed"
	session.result.Failed++
	for _, row := range session.saved.rows {
		item := &session.result.Installations[row]
		if item.Status == "planned" {
			item.Status = "not_attempted"
			item.Reason = "saved dependency application stopped before effects"
		}
	}
}
