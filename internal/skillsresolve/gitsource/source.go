// Package gitsource adapts UAP's existing immutable Skill source reader.
package gitsource

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/internal/skillsresolve"
)

var Policy = skillsource.ScanPolicy{
	MaxEntries: 20000, MaxMetadataBytes: 4 << 20, MaxDepth: 32,
	MaxCandidates: 128, MaxDocumentBytes: 64 << 10, MaxAggregateBytes: 2 << 20,
	Deadline: 30 * time.Second,
}

type Source struct{ Acquirer skillsource.Acquirer }

func (s Source) Open(ctx context.Context, request skillsresolve.Request) (skillsresolve.Session, string, string, error) {
	if err := skillsresolve.ValidateRequest(request); err != nil {
		return nil, "", "", err
	}
	repository, err := sourceacquisition.CanonicalGitHubRepository(request.Repository)
	if err != nil || strings.HasSuffix(repository, ".git") {
		return nil, "", "", skillsresolve.ErrInvalidRequest
	}
	source := skillsource.Source{Repository: repository}
	if request.Commit != nil {
		if err = sourceacquisition.ValidateSkillPath(*request.Path); err != nil || !safePath(*request.Path) {
			return nil, "", "", skillsresolve.ErrInvalidRequest
		}
		source.Commit = *request.Commit
	} else {
		source, err = s.Acquirer.Resolve(ctx, repository, "")
		if err != nil {
			return nil, "", "", err
		}
	}
	session, err := s.Acquirer.Open(ctx, source)
	if err != nil {
		return nil, "", "", err
	}
	return gitSession{session}, repository, source.Commit, nil
}

type gitSession struct{ session *skillsource.Session }

func (s gitSession) Close() error { return s.session.Close() }

func (s gitSession) Discover(ctx context.Context) (skillsresolve.Discovery, error) {
	catalog, err := s.session.Scan(ctx, "")
	if err != nil {
		return skillsresolve.Discovery{}, err
	}
	r := skillsresolve.Discovery{Complete: catalog.Complete, Reasons: catalog.IncompleteReasons}
	for _, candidate := range catalog.Candidates {
		if candidate.Status == "available" {
			if !safePath(candidate.Path) {
				r.Complete = false
				r.Reasons = append(r.Reasons, "unsupported_source_path")
				continue
			}
			r.Candidates = append(r.Candidates, skillsresolve.Candidate{Path: candidate.Path, Name: candidate.Name, Description: candidate.Description})
		}
	}
	return r, nil
}

func (s gitSession) Preview(ctx context.Context, path string) (skillsresolve.Preview, error) {
	preview, err := s.session.Preview(ctx, path)
	if err != nil {
		return skillsresolve.Preview{}, err
	}
	unsupported := preview.HasUnsupportedResources
	for _, resource := range preview.Resources {
		unsupported = unsupported || !safePath(resource.Path)
	}
	return skillsresolve.Preview{
		Candidate:  skillsresolve.Candidate{Path: preview.Path, Name: preview.Skill.Name, Description: preview.Skill.Description},
		RawContent: preview.RawContent, HasResources: len(preview.Resources) != 0,
		HasUnsupportedResources: unsupported,
	}, nil
}

func safePath(path string) bool {
	return len(path) <= 4096 && utf8.ValidString(path) && !strings.ContainsFunc(path, unicode.IsControl) && strings.Count(path, "/") < Policy.MaxDepth
}

// hostedRunner only receives trusted core-generated Git arguments. No user
// input can switch transports, redirects, helpers or native credentials.
type hostedRunner struct {
	mu       sync.Mutex
	runner   sourceacquisition.Runner
	poisoned bool
}

func (r *hostedRunner) Run(ctx context.Context, command sourceacquisition.Command) ([]byte, error) {
	// Discovery may request two documents concurrently. Admission and cleanup
	// evidence share one lock so queued work cannot launch after an uncertain exit.
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.poisoned {
		return nil, sourceacquisition.ErrGitCleanupUnconfirmed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if command.NativeCredentials {
		return nil, skillsresolve.ErrInvalidRequest
	}
	command.Args = append([]string{"-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "http.followRedirects=false"}, command.Args...)
	runner := r.runner
	if runner == nil {
		runner = sourceacquisition.OSRunner{ReportCleanupEvidence: true}
	}
	output, err := runner.Run(ctx, command)
	if errors.Is(err, sourceacquisition.ErrGitCleanupUnconfirmed) {
		r.poisoned = true
	}
	return output, err
}
