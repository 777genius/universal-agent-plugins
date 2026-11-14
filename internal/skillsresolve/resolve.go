// Package skillsresolve owns immutable, text-only catalog source previews.
package skillsresolve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxResponseBytes = 256 << 10

var ErrInvalidRequest = errors.New("invalid source request")

type Request struct {
	Repository    string  `json:"repository"`
	SkillNameHint *string `json:"skillNameHint,omitempty"`
	Commit        *string `json:"commit,omitempty"`
	Path          *string `json:"path,omitempty"`
}

type Candidate struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Skill struct {
	Candidate
	RawContent   string `json:"rawContent"`
	RawSHA256    string `json:"rawSha256"`
	HasResources bool   `json:"hasResources"`
}

type Result struct {
	Version    int         `json:"version"`
	Status     string      `json:"status"`
	Repository string      `json:"repository"`
	Commit     string      `json:"commit,omitempty"`
	Candidates []Candidate `json:"candidates,omitempty"`
	Skill      *Skill      `json:"skill,omitempty"`
	Reasons    []string    `json:"reasons,omitempty"`
}

type Discovery struct {
	Complete   bool
	Candidates []Candidate
	Reasons    []string
}

type Preview struct {
	Candidate
	RawContent              string
	HasResources            bool
	HasUnsupportedResources bool
}

type Session interface {
	Discover(context.Context) (Discovery, error)
	Preview(context.Context, string) (Preview, error)
	Close() error
}

// Source validates coordinates before any network work and opens one immutable
// commit. Neither application policy nor the HTTP layer knows about Git.
type Source interface {
	Open(context.Context, Request) (Session, string, string, error)
}

type Service struct{ source Source }

func New(source Source) *Service { return &Service{source: source} }

func ValidateRequest(r Request) error {
	if !validText(r.Repository, 256, false) || (r.Commit == nil) != (r.Path == nil) {
		return ErrInvalidRequest
	}
	if r.SkillNameHint != nil && (!validText(*r.SkillNameHint, 512, false) || r.Commit != nil) {
		return ErrInvalidRequest
	}
	if r.Commit != nil && (!validCommit(*r.Commit) || !validText(*r.Path, 4096, true)) {
		return ErrInvalidRequest
	}
	return nil
}

func validText(s string, limit int, empty bool) bool {
	return len(s) <= limit && utf8.ValidString(s) && (empty || strings.TrimSpace(s) != "") && !strings.ContainsFunc(s, unicode.IsControl)
}

func validCommit(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func (s *Service) Resolve(ctx context.Context, request Request) (r Result, err error) {
	r = Result{Version: 1, Repository: request.Repository, Status: "incomplete"}
	if err := ValidateRequest(request); err != nil {
		return r, err
	}
	session, repository, commit, err := s.source.Open(ctx, request)
	if err != nil {
		return r, err
	}
	defer func() {
		if cleanupErr := session.Close(); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()
	r.Repository, r.Commit = repository, commit
	if request.Path != nil {
		return selected(ctx, session, r, *request.Path)
	}
	discovery, err := session.Discover(ctx)
	if err != nil {
		return r, err
	}
	if !discovery.Complete {
		r.Reasons = discovery.Reasons
		return r, nil
	}
	return choose(ctx, session, r, discovery.Candidates, request.SkillNameHint)
}

func choose(ctx context.Context, session Session, r Result, candidates []Candidate, hint *string) (Result, error) {
	if len(candidates) == 0 {
		r.Status = "not_found"
		return r, nil
	}
	if hint == nil && len(candidates) == 1 {
		return selected(ctx, session, r, candidates[0].Path)
	}
	matches := []Candidate{}
	for _, candidate := range candidates {
		if hint != nil && candidate.Name == *hint {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 1 {
		return selected(ctx, session, r, matches[0].Path)
	}
	// A stale provider hint is not a path or a declared name. Let the user choose
	// from the actual validated catalog instead of guessing or hiding it.
	r.Status, r.Candidates = "selection_required", candidates
	return r, nil
}

func selected(ctx context.Context, session Session, r Result, path string) (Result, error) {
	preview, err := session.Preview(ctx, path)
	if err != nil {
		return r, err
	}
	if preview.HasUnsupportedResources || !utf8.ValidString(preview.RawContent) {
		r.Reasons = []string{"unsupported_source_content"}
		return r, nil
	}
	digest := sha256.Sum256([]byte(preview.RawContent))
	r.Status = "resolved"
	r.Skill = &Skill{Candidate: preview.Candidate, RawContent: preview.RawContent,
		RawSHA256: hex.EncodeToString(digest[:]), HasResources: preview.HasResources}
	return r, nil
}
