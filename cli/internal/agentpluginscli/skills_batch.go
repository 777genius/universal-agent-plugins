package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

var ErrSkillsPlanStale = errors.New("skill installation plan is stale")

type SkillsSelection struct {
	Repository, Commit, Path, Name, Alias, CandidateID string
	Website                                            string
	Selector                                           environment.Selector
}
type SkillsBatchRequest struct {
	Selections []SkillsSelection
	TargetIDs  []string
}
type SkillsAction struct{ Alias, ClientID, Action, TargetPath, CandidateID, FromPath, ToPath string }
type SkillsBatchItem struct{ Alias, ClientID, TargetPath, CandidateID, Status, Reason string }
type SkillsBatchResult struct {
	Status             string
	Items              []SkillsBatchItem
	RepairInstructions []string
}
type PreparedSkillsBatch struct {
	warnings        []string
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

func (p *PreparedSkillsBatch) check(ctx context.Context) error {
	if err := p.request.checkInputs(); err != nil {
		return fmt.Errorf("%w: %w", ErrSkillsPlanStale, err)
	}
	actions, detected, paths, err := prepareSkillsBindings(ctx, p.preview, p.request.requirements, p.request.lock, p.request.aliases, p.request.prepared)
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
	p.mu.Lock()
	defer p.mu.Unlock()
	result := SkillsBatchResult{Status: "failed", Items: []SkillsBatchItem{}, RepairInstructions: []string{}}
	if p.closed || p.applied {
		return result, errors.New("skill plan closed or already applied")
	}
	unlock, err := (processlock.Lock{Path: p.app.EnvironmentLockPath}).Acquire(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = unlock() }()
	if err = p.check(ctx); err != nil {
		return result, err
	}
	if err = ensureDependencyStore(ctx, p.app, p.request.selectedLock()); err != nil {
		return result, err
	}
	if err = p.request.publish(); err != nil {
		return result, err
	}
	p.applied = true
	adapter := &dependencyInstaller{app: p.app, opts: &options{}, detected: p.request.detected, beforeApply: p.request.checkInputs}
	results, applyErr := (environment.Coordinator{Mutator: adapter}).ApplySync(ctx, environment.PreparedSync{EnvironmentID: environment.EnvironmentUserDefault, ManifestDigest: p.request.lock.ManifestDigest, Bindings: p.request.prepared}, true)
	cursor := 0
	failedItem := false
	for _, action := range p.actions {
		item := SkillsBatchItem{Alias: action.Alias, ClientID: action.ClientID, TargetPath: action.TargetPath, CandidateID: action.CandidateID, Status: "not_attempted"}
		if action.Action == string(environment.ActionUnchanged) {
			item.Status = "unchanged"
		} else if cursor < len(results) {
			r := results[cursor]
			cursor++
			item.Status = string(r.Outcome)
			item.Reason = r.Reason
		} else if applyErr != nil && !failedItem {
			item.Status = "failed"
			item.Reason = applyErr.Error()
			failedItem = true
		}
		result.Items = append(result.Items, item)
	}
	if applyErr != nil {
		if len(results) > 0 {
			result.Status = "partial"
		}
		result.RepairInstructions = append(result.RepairInstructions, "Inspect agentplugins deps status and follow existing recovery/repair instructions.")
		return result, applyErr
	}
	result.Status = "succeeded"
	return result, nil
}
