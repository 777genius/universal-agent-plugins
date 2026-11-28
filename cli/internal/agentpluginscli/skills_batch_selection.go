package agentpluginscli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// SkillsSelectionError associates a failed source with the immutable selection.
// Presentation layers choose a safe message; the cause remains classifiable.
type SkillsSelectionError struct {
	CandidateID string
	Cause       error
}

func (e *SkillsSelectionError) Error() string { return "selected skill could not be prepared" }
func (e *SkillsSelectionError) Unwrap() error { return e.Cause }

func (candidate *skillsBatchCandidate) selectSkills(ctx context.Context, app App, selections []SkillsSelection, targets []string) error {
	prefetch := candidate.prefetchRawSkills(ctx, app, selections, targets)
	defer prefetch.close()
	seen := map[string]bool{}
	for _, selection := range selections {
		requirement, err := skillSelectionRequirement(candidate.requirements, selection)
		if err != nil {
			return &SkillsSelectionError{CandidateID: selection.CandidateID, Cause: err}
		}
		key := mustSourceKey(requirement)
		if seen[key] {
			return &SkillsSelectionError{CandidateID: selection.CandidateID, Cause: errors.New("duplicate selected skill")}
		}
		seen[key] = true
		if err = candidate.selectSkill(ctx, app, selection, requirement, targets, prefetch); err != nil {
			return &SkillsSelectionError{CandidateID: selection.CandidateID, Cause: err}
		}
	}
	return nil
}

// prefetchRawSkills starts the fresh GitHub Skill fetches that selectSkill will
// request, with exactly the arguments resolveSkill passes for a new selection.
// Selections that keep a saved requirement or fail validation are not fetched.
func (candidate *skillsBatchCandidate) prefetchRawSkills(ctx context.Context, app App, selections []SkillsSelection, targets []string) *rawSkillPrefetch {
	acquirer, err := app.rawSkillAcquirer()
	if err != nil {
		return nil
	}
	keys := []rawSkillKey{}
	for _, selection := range selections {
		req, err := skillSelectionRequirement(candidate.requirements, selection)
		if err != nil || req.Git == nil {
			continue
		}
		if err := app.checkEnterpriseRequirement(ctx, req, targets); err != nil {
			continue
		}
		// resolveSkill rejects this selection before downloading; do not either.
		if skillDirectoryMismatch(req.Git.Skill, req.Git.Path) {
			continue
		}
		if old, ok := candidate.requirements.Dependencies[req.Alias]; ok && mustSourceKey(old) == mustSourceKey(req) {
			continue
		}
		keys = append(keys, rawSkillKey{repository: req.Git.Repository, url: req.Git.URL, commit: selection.Commit, path: req.Git.Path})
	}
	if len(keys) < 2 {
		return nil
	}
	return startRawSkillPrefetch(ctx, acquirer, keys, app.prefetchGuard())
}

func (candidate *skillsBatchCandidate) selectSkill(ctx context.Context, app App, selection SkillsSelection, req environment.Requirement, targets []string, prefetch *rawSkillPrefetch) error {
	alias := req.Alias
	// Re-add preserves its prior selector and pin; normal saved update refreshes it.
	if old, ok := candidate.requirements.Dependencies[alias]; ok && mustSourceKey(old) == mustSourceKey(req) {
		req = old
	} else {
		resolver := dependencyResolver{app: app, requirements: candidate.requirements, dryRun: true, rawSkills: prefetch}
		var pin environment.LockedPackage
		var err error
		if req.Local != nil {
			pin, err = resolver.resolveLocalSkill(ctx, req, targets)
		} else if req.Web != nil {
			pin, err = resolver.resolveWebSkill(ctx, req, targets)
		} else {
			pin, err = resolver.resolveSkill(ctx, req, targets, nil, selection.Commit, "", "", nil)
		}
		if err != nil {
			return err
		}
		if selection.ExpectedIndexSHA256 != "" && (pin.Source.Web == nil || pin.Source.Web.IndexSHA256 != selection.ExpectedIndexSHA256) {
			return ErrSkillsPlanStale
		}
		if selection.ExpectedRawTreeDigest != "" && (pin.Source.Web == nil || pin.Source.Web.RawTreeDigest != selection.ExpectedRawTreeDigest) {
			return ErrSkillsPlanStale
		}
		candidate.previous.Packages = append(candidate.previous.Packages, pin)
	}
	candidate.requirements.Dependencies[alias] = req
	candidate.aliases = append(candidate.aliases, alias)
	candidate.candidateIDs[alias] = selection.CandidateID
	for _, target := range targets {
		if !slices.Contains(candidate.requirements.Targets[target], alias) {
			candidate.requirements.Targets[target] = append(candidate.requirements.Targets[target], alias)
		}
	}
	return nil
}

func skillSelectionRequirement(wanted environment.Requirements, selection SkillsSelection) (environment.Requirement, error) {
	if err := validateLocalSkillSelection(selection); err != nil {
		return environment.Requirement{}, err
	}
	selector := selection.Selector
	if selector.Kind == "" {
		selector = environment.Selector{Kind: environment.SelectorCommit, Value: selection.Commit}
	}
	req := environment.Requirement{Alias: defaultSkillAlias(selection), Git: &environment.GitSource{Kind: environment.SkillKind, Skill: selection.Name, Repository: strings.ToLower(selection.Repository), URL: selection.URL, Path: selection.Path, Selector: selector}}
	if selection.LocalPath != "" {
		req.Git = nil
		req.Local = &environment.LocalSource{Path: selection.LocalPath, Skill: selection.Name}
	}
	if selection.Website != "" {
		req.Git = nil
		endpoint, err := environment.NormalizeWebURL(selection.Website)
		if err != nil {
			return req, err
		}
		req.Web = &environment.WebSource{URL: endpoint, Skill: selection.Name}
	}
	key, err := req.SourceKey()
	if err != nil {
		return req, err
	}
	for alias, old := range wanted.Dependencies {
		if mustSourceKey(old) != key {
			continue
		}
		if selection.Alias != "" && selection.Alias != alias {
			return req, fmt.Errorf("source already saved under alias %s", alias)
		}
		req.Alias = alias
		return req, nil
	}
	if old, ok := wanted.Dependencies[req.Alias]; ok && mustSourceKey(old) != key {
		if selection.Alias != "" {
			return req, fmt.Errorf("alias %s belongs to another source", req.Alias)
		}
		sum := sha256.Sum256([]byte(key))
		req.Alias += "-" + hex.EncodeToString(sum[:4])
		if len(req.Alias) > 64 {
			req.Alias = "skill-" + hex.EncodeToString(sum[:8])
		}
		if old, ok := wanted.Dependencies[req.Alias]; ok && mustSourceKey(old) != key {
			return req, errors.New("skill alias conflict")
		}
	}
	return req, nil
}

func defaultSkillAlias(selection SkillsSelection) string {
	if selection.Alias != "" {
		return selection.Alias
	}
	probe := environment.Requirement{Alias: selection.Name, Git: &environment.GitSource{Repository: selection.Repository, URL: selection.URL, Selector: environment.Selector{Kind: environment.SelectorDefault}}}
	if selection.LocalPath != "" {
		probe.Git = nil
		probe.Local = &environment.LocalSource{Path: selection.LocalPath, Skill: selection.Name}
	}
	if selection.Website != "" {
		probe.Git = nil
		// An invalid publisher URL normalizes to "", which fails Validate and keeps the hashed alias.
		endpoint, _ := environment.NormalizeWebURL(selection.Website)
		probe.Web = &environment.WebSource{URL: endpoint, Skill: selection.Name}
	}
	if probe.Validate() == nil {
		return selection.Name
	}
	sum := sha256.Sum256([]byte(selection.Name + "\x00" + selection.Path))
	return "skill-" + hex.EncodeToString(sum[:6])
}
