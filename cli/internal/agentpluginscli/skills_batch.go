package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

var ErrSkillsPlanStale = errors.New("skill installation plan is stale")

type SkillsSelection struct {
	Repository, Commit, Path, Name, Alias, CandidateID string
	Website                                            string
	ExpectedIndexSHA256, ExpectedRawTreeDigest         string
	Selector                                           environment.Selector
}
type SkillsBatchRequest struct {
	Selections []SkillsSelection
	TargetIDs  []string
}
type SkillsAction struct{ Alias, ClientID, Action, TargetPath, CandidateID, FromPath, ToPath string }
type SkillsBatchItem struct {
	Alias, ClientID, Action, TargetPath, CandidateID, Status, Reason string
	Client                                                           installer.ClientResult
	ManualActions                                                    []string
	// failure explains a binding that did not succeed; nil otherwise.
	failure *installFailure
}
type SkillsProgress struct {
	Alias, ClientID, Action, Phase string
	Result                         installer.Result
}
type SkillsBatchResult struct {
	Status             string
	Items              []SkillsBatchItem
	RepairInstructions []string
}
type PreparedSkillsBatch struct {
	warnings        []string
	selections      []SkillsSelection
	app, preview    App
	request         dependencySyncRequest
	actions         []SkillsAction
	cleanup         func()
	mu              sync.Mutex
	closed, applied bool
}

func (p *PreparedSkillsBatch) Actions() []SkillsAction { return append([]SkillsAction{}, p.actions...) }
func (p *PreparedSkillsBatch) Warnings() []string      { return append([]string{}, p.warnings...) }
func (p *PreparedSkillsBatch) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.closed = true
		p.cleanup()
	}
}

func (p *PreparedSkillsBatch) Check(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.applied {
		return ErrSkillsPlanStale
	}
	return p.check(ctx)
}

func (p *PreparedSkillsBatch) checkInputs() error {
	if err := p.app.checkSkillsContext(nil); err != nil {
		return fmt.Errorf("%w: %w", ErrSkillsPlanStale, err)
	}
	if err := p.request.checkInputs(); err != nil {
		return fmt.Errorf("%w: %w", ErrSkillsPlanStale, err)
	}
	return nil
}

func (p *PreparedSkillsBatch) check(ctx context.Context) error {
	if err := p.checkInputs(); err != nil {
		return err
	}
	actions, detected, paths, err := prepareSkillsBindings(ctx, p.preview, p.request.requirements, p.request.lock, p.request.aliases, p.request.prepared, p.request.targets)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSkillsPlanStale, err)
	}
	for _, action := range p.actions {
		if path := paths[action.Alias+"\x00"+action.ClientID]; path != action.TargetPath {
			return ErrSkillsPlanStale
		}
		old, current := p.request.detected[domain.ClientID(action.ClientID)], detected[domain.ClientID(action.ClientID)]
		if old.ConfigRoot != current.ConfigRoot || old.ExecutablePath != current.ExecutablePath {
			return ErrSkillsPlanStale
		}
	}
	if !reflect.DeepEqual(actions, p.request.prepared) {
		return ErrSkillsPlanStale
	}
	return nil
}

// Apply is called only after explicit confirmation by the caller. The existing
// environment lock/rechecks and installer receipts own all durable effects.
func (p *PreparedSkillsBatch) Apply(ctx context.Context) (SkillsBatchResult, error) {
	return p.ApplyWithProgress(ctx, nil)
}

// ApplyWithProgress observes the same transaction as Apply. An observer is
// presentation only; its panic cannot change native effects or receipts.
func (p *PreparedSkillsBatch) ApplyWithProgress(ctx context.Context, progress func(SkillsProgress)) (SkillsBatchResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := SkillsBatchResult{Status: "failed", Items: []SkillsBatchItem{}, RepairInstructions: []string{}}
	if p.closed || p.applied {
		return result, errors.New("skill plan closed or already applied")
	}
	// Check the original authority before creating a lock in the selected root.
	if err := p.checkInputs(); err != nil {
		return result, err
	}
	p.applied = true
	adapter := &dependencyInstaller{app: p.app, opts: p.app.skillsOptions(), detected: p.request.detected, beforeApply: p.checkInputs}
	adapter.onPrepare = func(action environment.PreparedBinding) {
		emitSkillsProgress(progress, SkillsProgress{Alias: action.Alias, ClientID: action.ClientID, Action: string(action.Action), Phase: "preparing"})
	}
	adapter.onApply = func(action environment.PreparedBinding) {
		emitSkillsProgress(progress, SkillsProgress{Alias: action.Alias, ClientID: action.ClientID, Action: string(action.Action), Phase: "installing"})
	}
	adapter.onResult = func(action environment.PreparedBinding, result installer.Result) {
		emitSkillsProgress(progress, SkillsProgress{Alias: action.Alias, ClientID: action.ClientID, Action: string(action.Action), Phase: "finished", Result: result})
	}
	var unchanged map[string]installer.ClientResult
	checked := func(ctx context.Context) error {
		if err := p.check(ctx); err != nil {
			return err
		}
		var err error
		unchanged, err = p.observeUnchangedSkills(ctx)
		return err
	}
	applied, applyErr := applyPreparedDependencies(ctx, p.app, &p.request, adapter, checked)
	var mutated bool
	result.Items, mutated = skillsBatchItems(p.actions, applied.Outcomes, applyErr)
	for i := range result.Items {
		item := &result.Items[i]
		if item.Action == string(environment.ActionUnchanged) {
			item.Client = unchanged[skillsBindingKey(item.Alias, item.ClientID, item.Action)]
		}
	}
	if applyErr != nil {
		if mutated || applied.Published {
			result.Status = "partial"
		}
		result.RepairInstructions = append(result.RepairInstructions, "Inspect agentplugins deps status and follow existing recovery/repair instructions.")
		return result, applyErr
	}
	result.Status = "succeeded"
	return result, nil
}

func skillsBatchItems(actions []SkillsAction, outcomes []environment.BindingOutcome, applyErr error) ([]SkillsBatchItem, bool) {
	byBinding := make(map[string]environment.BindingOutcome, len(outcomes))
	mutated := false
	for _, outcome := range outcomes {
		binding := outcome.Binding
		byBinding[skillsBindingKey(binding.Alias, binding.ClientID, string(binding.Action))] = outcome
		mutated = mutated || outcome.Status == environment.BindingApplied || (outcome.Result != nil && outcome.Result.Mutated)
	}
	items := make([]SkillsBatchItem, 0, len(actions))
	for _, action := range actions {
		item := SkillsBatchItem{Alias: action.Alias, ClientID: action.ClientID, Action: action.Action, TargetPath: action.TargetPath, CandidateID: action.CandidateID, Status: "not_attempted"}
		outcome, observed := byBinding[skillsBindingKey(action.Alias, action.ClientID, action.Action)]
		switch {
		case action.Action == "unchanged":
			item.Status = "unchanged"
		case observed:
			item.Status, item.Reason = outcomeStatusReason(outcome)
			item.failure = describeBindingFailure(outcome)
			if outcome.Result != nil {
				item.Client = outcome.Result.Client
				item.ManualActions = append([]string(nil), outcome.Result.ManualActions...)
			}
		case applyErr != nil:
			// Nothing ran: the sync failed before reaching this binding.
			item.Reason = applyErr.Error()
			item.failure = describeSyncFailure(applyErr)
		}
		items = append(items, item)
	}
	return items, mutated
}

func emitSkillsProgress(observer func(SkillsProgress), event SkillsProgress) {
	if observer == nil {
		return
	}
	defer func() { _ = recover() }()
	// The observer receives detached presentation facts, never a mutable native
	// authority handle or slices shared with the transaction's returned result.
	native := event.Result
	client := native.Client
	client.ProfileAuthority, client.SelectedDelivery = nil, domain.SelectedDelivery{}
	client.RequiredComponents = append([]string(nil), client.RequiredComponents...)
	if client.Failure != nil {
		failure := *client.Failure
		client.Failure = &failure
	}
	event.Result = installer.Result{Outcome: native.Outcome, Client: client, ManualActions: append([]string(nil), native.ManualActions...), Reason: native.Reason}
	observer(event)
}

func skillsBindingKey(alias, client, action string) string {
	return client + "\x00" + alias + "\x00" + action
}
