package agentpluginscli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"sync"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
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
	if err := app.checkSkillsContext(nil); err != nil {
		return nil, err
	}
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
