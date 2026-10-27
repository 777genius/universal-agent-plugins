package skillsource

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type ScanPolicy struct {
	MaxEntries, MaxMetadataBytes, MaxDepth, MaxCandidates, MaxDocumentBytes, MaxAggregateBytes int
	Deadline                                                                                   time.Duration
}

func (p ScanPolicy) bounded() ScanPolicy {
	d := ScanPolicy{100000, 16 << 20, 64, 2000, 1 << 20, 16 << 20, 120 * time.Second}
	a := []*int{&p.MaxEntries, &p.MaxMetadataBytes, &p.MaxDepth, &p.MaxCandidates, &p.MaxDocumentBytes, &p.MaxAggregateBytes}
	b := []int{d.MaxEntries, d.MaxMetadataBytes, d.MaxDepth, d.MaxCandidates, d.MaxDocumentBytes, d.MaxAggregateBytes}
	for i, v := range a {
		if *v <= 0 || *v > b[i] {
			*v = b[i]
		}
	}
	if p.Deadline <= 0 || p.Deadline > d.Deadline {
		p.Deadline = d.Deadline
	}
	return p
}

type Candidate struct {
	ID, Path, Name, Description, Status string
	Diagnostics                         []string
}
type Catalog struct {
	Repository, Commit, ScopePath, Digest string
	Complete                              bool
	IncompleteReasons                     []string
	Candidates                            []Candidate
	Folders                               []string
	Discovered, Validated                 int
	Bytes                                 int64
}
type Resource struct {
	Path string
	// Size is absent unless its object metadata is already cached. Asking Git
	// for the size of a promised resource would download the entire resource.
	Size       *int64
	Executable bool
}
type Preview struct {
	Path, Markdown, RawContent string
	Resources                  []Resource
	Skill                      domain.Skill
	// Hosted text-only consumers must not infer completeness from Resources,
	// which intentionally omits unsupported Git entries for legacy UI callers.
	HasUnsupportedResources bool
}
type Session struct {
	source      Source
	tree        *sourceacquisition.GitTree
	policy      ScanPolicy
	mu          sync.Mutex
	sizes       map[string]int64
	bodies      map[string][]byte
	cachedBytes int
}

func (a Acquirer) Open(ctx context.Context, s Source) (*Session, error) {
	repository, err := sourceacquisition.CanonicalGitHubRepository(s.Repository)
	if err != nil {
		return nil, err
	}
	s.Repository = repository
	openCtx, cancel := context.WithTimeout(ctx, a.Policy.bounded().Deadline)
	defer cancel()
	t, e := a.OpenGitHubTree(openCtx, s.Repository, s.Commit)
	if e != nil {
		return nil, e
	}
	return &Session{source: s, tree: t, policy: a.Policy.bounded(), sizes: map[string]int64{}, bodies: map[string][]byte{}}, nil
}
func (s *Session) Close() error { return s.tree.Close() }
func (s *Session) candidate(e sourceacquisition.TreeEntry) Candidate {
	p := path.Dir(e.Path)
	if p == "." {
		p = ""
	}
	return Candidate{ID: digest([]byte(s.source.Repository + "\x00" + s.source.Commit + "\x00" + p)), Path: p, Status: "unavailable", Diagnostics: []string{}}
}
func (s *Session) name(p string) string {
	if p != "" {
		return path.Base(p)
	}
	return path.Base(s.source.Repository)
}
func (s *Session) cachedBlobSize(oid string) *int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	size, known := s.sizes[oid]
	if !known {
		return nil
	}
	return &size
}
func (s *Session) blobSize(ctx context.Context, oid string) (int64, error) {
	if cached := s.cachedBlobSize(oid); cached != nil {
		return *cached, ctx.Err()
	}
	n, e := s.tree.BlobSize(ctx, oid)
	if e == nil {
		s.mu.Lock()
		if len(s.sizes) < s.policy.MaxCandidates {
			s.sizes[oid] = n
		}
		s.mu.Unlock()
	}
	return n, e
}
func (s *Session) blob(ctx context.Context, oid string) ([]byte, error) {
	s.mu.Lock()
	body, ok := s.bodies[oid]
	s.mu.Unlock()
	if ok {
		return body, ctx.Err()
	}
	body, e := s.tree.Blob(ctx, oid, s.policy.MaxDocumentBytes)
	if e == nil {
		s.mu.Lock()
		if _, present := s.bodies[oid]; !present && s.cachedBytes+len(body) <= s.policy.MaxAggregateBytes {
			s.bodies[oid] = body
			s.cachedBytes += len(body)
		}
		s.mu.Unlock()
	}
	return body, e
}

func (s *Session) readCandidate(ctx context.Context, e sourceacquisition.TreeEntry) (Candidate, []byte) {
	c := s.candidate(e)
	if e.Type != "blob" || (e.Mode != "100644" && e.Mode != "100755") {
		c.Status = "invalid"
		c.Diagnostics = append(c.Diagnostics, "skill_document_wrong_kind")
		return c, nil
	}
	n, err := s.blobSize(ctx, e.Object)
	if err != nil {
		c.Diagnostics = append(c.Diagnostics, "skill_document_unavailable")
		return c, nil
	}
	if n > int64(s.policy.MaxDocumentBytes) {
		c.Diagnostics = append(c.Diagnostics, "skill_document_too_large")
		return c, nil
	}
	body, err := s.blob(ctx, e.Object)
	if err != nil || int64(len(body)) != n {
		c.Diagnostics = append(c.Diagnostics, "skill_document_unavailable")
		return c, nil
	}
	skill, err := conformance.ParseSkill(ctx, s.name(c.Path), body)
	if err != nil {
		code, unavailable := conformance.SkillErrorCode(err)
		c.Status = "invalid"
		if unavailable {
			c.Status = "unavailable"
		}
		c.Diagnostics = append(c.Diagnostics, code)
		return c, body
	}
	c.Status = "available"
	c.Name = skill.Name
	c.Description = skill.Description
	return c, body
}

func (s *Session) Scan(ctx context.Context, scope string) (Catalog, error) {
	c := Catalog{Repository: s.source.Repository, Commit: s.source.Commit, ScopePath: scope, Complete: true, Candidates: []Candidate{}, Folders: []string{}, IncompleteReasons: []string{}}
	if err := sourceacquisition.ValidateSkillPath(scope); err != nil {
		return c, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.policy.Deadline)
	defer cancel()
	documents, err := s.scanDocuments(ctx, scope, &c)
	if err != nil {
		return c, err
	}
	readable := s.reserveDocuments(ctx, documents, &c)
	if err = s.decodeDocuments(ctx, documents, readable, &c); err != nil {
		return c, err
	}
	return finalize(c), nil
}
func (c *Catalog) incomplete(reason string) {
	c.Complete = false
	for _, existing := range c.IncompleteReasons {
		if existing == reason {
			return
		}
	}
	c.IncompleteReasons = append(c.IncompleteReasons, reason)
}
func (s *Session) scanDocuments(ctx context.Context, scope string, c *Catalog) ([]sourceacquisition.TreeEntry, error) {
	entries, err := s.tree.List(ctx, scope, s.policy.MaxMetadataBytes)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		c.incomplete("tree_metadata_unavailable")
		return nil, nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	if len(entries) > s.policy.MaxEntries {
		entries = entries[:s.policy.MaxEntries]
		c.incomplete("tree_entry_limit")
	}
	var documents []sourceacquisition.TreeEntry
	for _, e := range entries {
		relative := strings.TrimPrefix(strings.TrimPrefix(e.Path, scope), "/")
		if strings.Count(relative, "/")+1 > s.policy.MaxDepth {
			c.incomplete("tree_depth_limit")
			continue
		}
		if path.Base(e.Path) != "SKILL.md" {
			continue
		}
		if len(documents) >= s.policy.MaxCandidates {
			c.incomplete("candidate_limit")
			continue
		}
		documents = append(documents, e)
	}
	return documents, nil
}
func (s *Session) reserveDocuments(ctx context.Context, documents []sourceacquisition.TreeEntry, c *Catalog) []bool {
	c.Candidates = make([]Candidate, len(documents))
	readable := make([]bool, len(documents))
	aggregate := int64(0)
	// Reserve bytes in canonical path order before concurrent reads. The aggregate
	// bound and partial catalog are therefore independent of process scheduling.
	for i, e := range documents {
		c.Candidates[i] = s.candidate(e)
		if e.Type != "blob" || (e.Mode != "100644" && e.Mode != "100755") {
			c.Candidates[i].Status = "invalid"
			c.Candidates[i].Diagnostics = []string{"skill_document_wrong_kind"}
			continue
		}
		n, e := s.blobSize(ctx, e.Object)
		if e != nil {
			c.Candidates[i].Diagnostics = []string{"skill_document_unavailable"}
			c.incomplete("document_unavailable")
			continue
		}
		if n > int64(s.policy.MaxDocumentBytes) {
			c.Candidates[i].Diagnostics = []string{"skill_document_too_large"}
			c.incomplete("document_bytes_limit")
			continue
		}
		if aggregate+n > int64(s.policy.MaxAggregateBytes) {
			c.Candidates[i].Diagnostics = []string{"skill_aggregate_bytes_limit"}
			c.incomplete("aggregate_bytes_limit")
			continue
		}
		aggregate += n
		readable[i] = true
	}
	return readable
}
func (s *Session) decodeDocuments(ctx context.Context, documents []sourceacquisition.TreeEntry, readable []bool, c *Catalog) error {
	readBytes := make([]int, len(documents))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(2)
	for i, e := range documents {
		if !readable[i] {
			continue
		}
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			candidate, body := s.readCandidate(gctx, e)
			readBytes[i] = len(body)
			c.Candidates[i] = candidate
			return gctx.Err()
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for i, candidate := range c.Candidates {
		c.Bytes += int64(readBytes[i])
		if candidate.Status == "unavailable" {
			c.incomplete("document_unavailable")
		}
	}
	return nil
}
func finalize(c Catalog) Catalog {
	c.Discovered = len(c.Candidates)
	for _, v := range c.Candidates {
		if v.Status != "unavailable" {
			c.Validated++
		}
	}
	folders := map[string]bool{}
	for _, v := range c.Candidates {
		for p := v.Path; p != "" && p != "."; p = path.Dir(p) {
			folders[p] = true
		}
	}
	for p := range folders {
		c.Folders = append(c.Folders, p)
	}
	sort.Strings(c.Folders)
	sort.Strings(c.IncompleteReasons)
	body, _ := json.Marshal(c)
	c.Digest = digest(body)
	return c
}

// CandidateAt verifies a selected known path without a repository catalog scan.
func (s *Session) CandidateAt(ctx context.Context, p string) (Candidate, error) {
	ctx, cancel := context.WithTimeout(ctx, s.policy.Deadline)
	defer cancel()
	if err := sourceacquisition.ValidateSkillPath(p); err != nil {
		return Candidate{}, err
	}
	e, err := s.tree.Entry(ctx, path.Join(p, "SKILL.md"))
	if err != nil {
		return Candidate{}, err
	}
	c, _ := s.readCandidate(ctx, e)
	if err := ctx.Err(); err != nil {
		return c, err
	}
	return c, nil
}
func (s *Session) Preview(ctx context.Context, p string) (Preview, error) {
	var preview Preview
	if err := sourceacquisition.ValidateSkillPath(p); err != nil {
		return preview, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.policy.Deadline)
	defer cancel()
	e, err := s.tree.Entry(ctx, path.Join(p, "SKILL.md"))
	if err != nil {
		return preview, err
	}
	c, body := s.readCandidate(ctx, e)
	if err := ctx.Err(); err != nil {
		return preview, err
	}
	if c.Status != "available" {
		return preview, fmt.Errorf("selected Skill metadata is unavailable or invalid")
	}
	skill, err := conformance.ParseSkill(ctx, s.name(p), body)
	if err != nil {
		return preview, err
	}
	frontmatter, err := conformance.DecodeFrontmatter(ctx, body, conformance.DefaultLimits(), nil)
	if err != nil {
		return preview, err
	}
	preview = Preview{Path: p, Markdown: string(frontmatter.Body), RawContent: string(body), Resources: []Resource{}, Skill: skill}
	entries, err := s.tree.List(ctx, p, s.policy.MaxMetadataBytes)
	if err != nil {
		return preview, err
	}
	if len(entries) > s.policy.MaxEntries {
		return preview, fmt.Errorf("resource inventory exceeds tree entry limit")
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return preview, err
		}
		if e.Type != "blob" || (e.Mode != "100644" && e.Mode != "100755" && e.Mode != "120000") {
			preview.HasUnsupportedResources = true
			continue
		}
		if e.Mode == "120000" {
			preview.HasUnsupportedResources = true
		}
		rel := e.Path
		if p != "" {
			rel = strings.TrimPrefix(rel, p+"/")
		}
		if rel == "SKILL.md" {
			continue
		}
		preview.Resources = append(preview.Resources, Resource{Path: rel, Size: s.cachedBlobSize(e.Object), Executable: e.Mode == "100755"})
	}
	sort.Slice(preview.Resources, func(i, j int) bool { return preview.Resources[i].Path < preview.Resources[j].Path })
	return preview, nil
}
func (a Acquirer) ResolveGitHubSkill(ctx context.Context, repository, sha, expected, requested, last string) (string, error) {
	s, err := a.Open(ctx, Source{Repository: repository, Commit: sha})
	if err != nil {
		return "", err
	}
	defer func() { _ = s.Close() }()
	paths := []string{last}
	if requested != last {
		paths = append(paths, requested)
	}
	for _, p := range paths {
		candidate, err := s.CandidateAt(ctx, p)
		if err == nil && candidate.Status == "available" && candidate.Name == expected {
			return p, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	catalog, err := s.Scan(ctx, "")
	if err != nil {
		return "", err
	}
	if !catalog.Complete {
		return "", fmt.Errorf("skill relocation requires complete repository discovery")
	}
	var matches []string
	for _, c := range catalog.Candidates {
		if c.Status == "available" && c.Name == expected {
			matches = append(matches, c.Path)
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("skill relocation requires exactly one valid exact-name match; found %d", len(matches))
	}
	return matches[0], nil
}

// ExactCatalog describes one explicitly selected Skill without enumerating its
// subtree. Complete remains false because nested Skills were not enumerated;
// the selected available candidate can still proceed through normal preflight.
func (s *Session) ExactCatalog(ctx context.Context, p string) (Catalog, error) {
	ctx, cancel := context.WithTimeout(ctx, s.policy.Deadline)
	defer cancel()
	c := Catalog{Repository: s.source.Repository, Commit: s.source.Commit, ScopePath: p, Complete: false, Candidates: []Candidate{}, Folders: []string{}, IncompleteReasons: []string{"exact_skill_selection"}}
	if err := sourceacquisition.ValidateSkillPath(p); err != nil {
		return c, err
	}
	e, err := s.tree.Entry(ctx, path.Join(p, "SKILL.md"))
	if err != nil {
		return c, err
	}
	candidate, body := s.readCandidate(ctx, e)
	if err := ctx.Err(); err != nil {
		return c, err
	}
	c.Candidates = append(c.Candidates, candidate)
	c.Bytes = int64(len(body))
	return finalize(c), nil
}
