package agentpluginscli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// SkillsPickerRequest selects one public source; PathSet distinguishes an exact
// repository root Skill from an unrestricted repository scan.
type SkillsPickerRequest struct {
	Input, Ref, Path string
	PathSet          bool
}

type skillsCatalogSession struct {
	nonce      string
	app        App
	source     skillsource.Source
	session    *skillsource.Session
	cleanup    func()
	mu         sync.Mutex
	catalog    skillspicker.Catalog
	candidates map[string]skillsource.Candidate
	scanned    bool
	progress   skillspicker.ScanProgress
}

func (app App) skillSourceAcquirer(scratch string) (skillsource.Acquirer, error) {
	var a sourceacquisition.Acquirer
	switch value := app.SourceAcquirer.(type) {
	case sourceacquisition.Acquirer:
		a = value
	case *sourceacquisition.Acquirer:
		a = *value
	case skillsource.Acquirer:
		value.TempRoot = scratch
		return value, nil
	case *skillsource.Acquirer:
		acquirer := *value
		acquirer.TempRoot = scratch
		return acquirer, nil
	case interface {
		SkillsAcquirer(string) skillsource.Acquirer
	}:
		return value.SkillsAcquirer(scratch), nil
	default:
		return skillsource.Acquirer{}, errors.New("source acquisition does not support raw Skills")
	}
	a.TempRoot = scratch
	return skillsource.Acquirer{Acquirer: a}, nil
}
func (app App) openSkillsCatalog(ctx context.Context, input SkillsPickerRequest) (*skillsCatalogSession, error) {
	scratch, err := os.MkdirTemp("", "agentplugins-skills-")
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.RemoveAll(scratch) }
	acquirer, err := app.skillSourceAcquirer(scratch)
	if err != nil {
		cleanup()
		return nil, err
	}
	source, err := acquirer.Resolve(ctx, input.Input, input.Ref)
	if err != nil {
		cleanup()
		return nil, err
	}
	if input.PathSet {
		if source.RequestedPath != "" && source.RequestedPath != input.Path {
			cleanup()
			return nil, errors.New("--path conflicts with the source URL path")
		}
		if err := sourceacquisition.ValidateSkillPath(input.Path); err != nil {
			cleanup()
			return nil, err
		}
		source.RequestedPath = input.Path
		source.ExactPath = true
	}
	session, err := acquirer.Open(ctx, source)
	if err != nil {
		cleanup()
		return nil, err
	}
	// A folder URL with an immediate Skill is the same explicit selection as a
	// file URL. A category remains a catalog scope. This lookup never enumerates
	// siblings and its bounded metadata is reused by the source session.
	if !source.ExactPath && source.RequestedPath != "" {
		if _, directErr := session.CandidateAt(ctx, source.RequestedPath); directErr == nil {
			source.ExactPath = true
		}
	}
	return &skillsCatalogSession{nonce: newSkillCatalogID(), app: app, source: source, session: session, cleanup: func() { _ = session.Close(); cleanup() }, candidates: map[string]skillsource.Candidate{}}, nil
}

// StartSkillsPicker resolves a real immutable Git source and joins discovery and
// the same prepared batch installer used by terminal selection. Closing the
// server releases all source and review scratch workspaces.
func (app App) StartSkillsPicker(ctx context.Context, input SkillsPickerRequest) (*skillspicker.Server, error) {
	session, err := app.openSkillsCatalog(ctx, input)
	if err != nil {
		return nil, err
	}
	revision := session.source.Commit
	server, err := skillspicker.Start(ctx, skillspicker.Options{Source: skillspicker.Source{Repository: session.source.Repository, SourceRevision: &revision, Scope: session.source.RequestedPath}, Callbacks: session.callbacks(), OnClose: session.cleanup})
	if err != nil {
		session.cleanup()
		return nil, err
	}
	return server, nil
}
func (s *skillsCatalogSession) callbacks() skillspicker.Callbacks {
	return skillspicker.Callbacks{Scan: s.scan, Preview: s.preview, Targets: s.app.skillsTargets, Prepare: s.prepare, Progress: s.scanProgress}
}
func (s *skillsCatalogSession) scan(ctx context.Context, scope string) (skillspicker.Catalog, error) {
	s.mu.Lock()
	first := !s.scanned
	s.scanned = true
	s.mu.Unlock()
	var result skillsource.Catalog
	var err error
	if first && scope == s.source.RequestedPath && (scope != "" || s.source.ExactPath) {
		result, err = s.session.ExactCatalog(ctx, scope)
		if err != nil && !s.source.ExactPath {
			result, err = s.session.Scan(ctx, scope)
		}
	} else {
		result, err = s.session.Scan(ctx, scope)
	}
	if err != nil {
		return skillspicker.Catalog{}, err
	}
	catalog := skillspicker.Catalog{CatalogId: newSkillCatalogID(), CatalogDigest: result.Digest, Repository: result.Repository, SourceRevision: result.Commit, Scope: result.ScopePath, Complete: result.Complete, Candidates: []skillspicker.Candidate{}, Diagnostics: []skillspicker.Diagnostic{}}
	candidates := map[string]skillsource.Candidate{}
	for _, c := range result.Candidates {
		hash := sha256.Sum256([]byte(s.nonce + "\x00" + c.ID))
		c.ID = "candidate-" + hex.EncodeToString(hash[:])
		candidates[c.ID] = c
		status := skillspicker.CandidateStatus(c.Status)
		if !status.Valid() {
			status = skillspicker.Unavailable
		}
		catalog.Candidates = append(catalog.Candidates, skillspicker.Candidate{Id: c.ID, Name: c.Name, Description: c.Description, Path: c.Path, Status: status, DiagnosticCodes: append([]string{}, c.Diagnostics...)})
		catalog.Counts.Total++
		switch status {
		case skillspicker.Available:
			catalog.Counts.Available++
		case skillspicker.Invalid:
			catalog.Counts.Invalid++
		default:
			catalog.Counts.Unavailable++
		}
	}
	for _, reason := range result.IncompleteReasons {
		catalog.Diagnostics = append(catalog.Diagnostics, skillspicker.Diagnostic{Code: reason, Severity: skillspicker.Warning, Message: "Catalog scan is incomplete; explicitly scan a source folder to discover nested Skills"})
	}
	s.mu.Lock()
	s.catalog = catalog
	s.progress = skillspicker.ScanProgress{Discovered: result.Discovered, Validated: result.Validated, Bytes: int(result.Bytes)}
	s.candidates = candidates
	s.mu.Unlock()
	return catalog, nil
}
func newSkillCatalogID() string {
	id, err := newOperationGroupID()
	if err != nil {
		panic(err)
	}
	return id
}
func (s *skillsCatalogSession) preview(ctx context.Context, id string) (skillspicker.Preview, error) {
	s.mu.Lock()
	candidate, ok := s.candidates[id]
	catalogID := s.catalog.CatalogId
	s.mu.Unlock()
	if !ok {
		return skillspicker.Preview{}, errors.New("unknown catalog candidate")
	}
	result, err := s.session.Preview(ctx, candidate.Path)
	if err != nil {
		return skillspicker.Preview{}, err
	}
	preview := skillspicker.Preview{CatalogId: catalogID, CandidateId: id, Path: result.Path, Content: result.Markdown, RawContent: &result.RawContent, Format: skillspicker.Markdown, Resources: []skillspicker.Resource{}, Diagnostics: []skillspicker.Diagnostic{}}
	for _, resource := range result.Resources {
		preview.Resources = append(preview.Resources, skillspicker.Resource{Path: resource.Path, Size: int(resource.Size), Executable: resource.Executable})
	}
	return preview, nil
}
func (app App) skillsTargets(ctx context.Context) ([]skillspicker.Target, error) {
	if app.Detector == nil {
		return nil, errors.New("client detector unavailable")
	}
	detected, err := app.Detector.Detect(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[domain.ClientID]domain.DetectedClient{}
	for _, client := range detected {
		byID[client.ClientID] = client
	}
	targets := []skillspicker.Target{}
	for _, id := range domain.SupportedClientIDs() {
		if !supportsDependencyTarget(app, string(id)) {
			continue
		}
		definition, _ := domain.ClientDefinitionFor(id)
		client, ok := byID[id]
		available := ok && client.DetectionError == nil && client.Status == domain.DetectionDetected
		target := skillspicker.Target{Id: string(id), Label: definition.DisplayName, Scope: "user", DisplayPath: client.ConfigRoot, Available: available}
		if !available {
			reason := "This client is not detected; install or configure it before selecting it"
			if client.DetectionError != nil {
				reason = "Client configuration could not be inspected safely"
			}
			target.Reason = &reason
		}
		targets = append(targets, target)
	}
	return targets, nil
}
func (s *skillsCatalogSession) prepare(ctx context.Context, input skillspicker.PlanRequest) (skillspicker.Prepared, error) {
	s.mu.Lock()
	if input.CatalogId != s.catalog.CatalogId || input.CatalogDigest != s.catalog.CatalogDigest {
		s.mu.Unlock()
		return nil, errors.New("catalog changed")
	}
	selections := []SkillsSelection{}
	for _, id := range input.SelectedIds {
		candidate, ok := s.candidates[id]
		if !ok || candidate.Status != "available" {
			s.mu.Unlock()
			return nil, errors.New("invalid selected candidate")
		}
		selections = append(selections, s.selection(candidate))
	}
	s.mu.Unlock()
	prepared, err := s.app.PrepareSkills(ctx, SkillsBatchRequest{Selections: selections, TargetIDs: input.TargetIds})
	if err != nil {
		return nil, s.reviewRefusal(err)
	}
	return skillsPickerPrepared{prepared}, nil
}
func (s *skillsCatalogSession) selection(candidate skillsource.Candidate) SkillsSelection {
	selector := environment.Selector{Kind: environment.SelectorRef, Value: s.source.RequestedSelector}
	switch s.source.RequestedSelector {
	case "default":
		selector = environment.Selector{Kind: environment.SelectorDefault, Value: ""}
	case s.source.Commit:
		selector = environment.Selector{Kind: environment.SelectorCommit, Value: s.source.Commit}
	}
	return SkillsSelection{Repository: s.source.Repository, Commit: s.source.Commit, Path: candidate.Path, Name: candidate.Name, CandidateID: candidate.ID, Selector: selector}
}

type skillsPickerPrepared struct{ *PreparedSkillsBatch }

func (p skillsPickerPrepared) Review() skillspicker.Plan {
	review := skillspicker.Plan{Actions: []skillspicker.Action{}, Conflicts: []skillspicker.Diagnostic{}, Warnings: []skillspicker.Diagnostic{}, Confirmable: true}
	for _, a := range p.Actions() {
		action := skillspicker.Action{CandidateId: a.CandidateID, TargetId: a.ClientID, Alias: a.Alias, Action: skillspicker.ActionAction(a.Action), DestinationPath: a.TargetPath, ReasonCodes: []string{}}
		if a.FromPath != "" || a.ToPath != "" {
			action.SourceMoved = &skillspicker.SourceMoved{From: a.FromPath, To: a.ToPath}
		}
		review.Actions = append(review.Actions, action)
	}
	for _, warning := range p.Warnings() {
		review.Warnings = append(review.Warnings, skillspicker.Diagnostic{Code: "INSTALLER_WARNING", Severity: skillspicker.Warning, Message: warning})
	}
	return review
}
func (p skillsPickerPrepared) Check(ctx context.Context) error {
	err := p.PreparedSkillsBatch.Check(ctx)
	if errors.Is(err, ErrSkillsPlanStale) {
		return skillspicker.ErrPlanStale
	}
	return err
}
func (p skillsPickerPrepared) Apply(ctx context.Context) (skillspicker.Outcome, error) {
	result, err := p.PreparedSkillsBatch.Apply(ctx)
	if errors.Is(err, ErrSkillsPlanStale) {
		err = skillspicker.ErrPlanStale
	}
	return skillBatchOutcome(result), err
}
func skillBatchOutcome(result SkillsBatchResult) skillspicker.Outcome {
	outcome := skillspicker.Outcome{State: skillspicker.OperationState(result.Status), Items: []skillspicker.OperationItem{}, Diagnostics: []skillspicker.Diagnostic{}, Result: skillspicker.OperationResult{Message: fmt.Sprintf("Skills installation %s", result.Status), RepairInstructions: append([]string{}, result.RepairInstructions...)}}
	for _, item := range result.Items {
		alias, path := item.Alias, item.TargetPath
		message := skillOperationMessage(skillOperationItemStatus(item.Status), item.Reason)
		outcome.Items = append(outcome.Items, skillspicker.OperationItem{CandidateId: item.CandidateID, TargetId: item.ClientID, Alias: &alias, DestinationPath: &path, Message: &message, Status: skillOperationItemStatus(item.Status)})
	}
	return outcome
}

func skillOperationItemStatus(status string) skillspicker.OperationItemStatus {
	switch status {
	case "completed", "succeeded":
		return skillspicker.OperationItemStatusSucceeded
	case "unchanged":
		return skillspicker.OperationItemStatusUnchanged
	case "not_attempted", "skipped":
		return skillspicker.OperationItemStatusSkipped
	default:
		return skillspicker.OperationItemStatusFailed
	}
}

func (s *skillsCatalogSession) scanProgress() skillspicker.ScanProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}
