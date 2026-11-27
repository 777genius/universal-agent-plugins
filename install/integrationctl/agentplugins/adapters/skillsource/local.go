package skillsource

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
)

// LocalSkillPath validates observed ancestry without resolving symlinks. The
// immutable acquisition path remains responsible for validating installed bytes.
func LocalSkillPath(root, relative string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if relative == "." {
		relative = ""
	}
	if err := sourceacquisition.ValidateSkillPath(relative); err != nil {
		return "", err
	}
	selected := filepath.Join(absolute, filepath.FromSlash(relative))
	if err := localDirectoryAncestry(selected); err != nil {
		return "", err
	}
	info, err := os.Lstat(filepath.Join(selected, "SKILL.md"))
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("local Skill requires a regular SKILL.md")
	}
	return selected, nil
}

func localDirectoryAncestry(directory string) error {
	for current := directory; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("local Skill path component %q is not a regular directory (symlinks are rejected); pass the resolved path", current)
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}

type localScanner struct {
	root              *os.Root
	policy            ScanPolicy
	catalog           Catalog
	entries, metadata int
	stopped           bool
}

// ScanLocal reads only bounded Skill documents through a pinned local root.
// Observed links are rejected; rooted operations contain concurrent substitutions.
func ScanLocal(ctx context.Context, directory string, policy ScanPolicy) (Catalog, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return Catalog{}, err
	}
	if err = localDirectoryAncestry(absolute); err != nil {
		return Catalog{}, err
	}
	root, err := os.OpenRoot(absolute)
	if err != nil {
		return Catalog{}, err
	}
	defer func() { _ = root.Close() }()
	policy = policy.bounded()
	ctx, cancel := context.WithTimeout(ctx, policy.Deadline)
	defer cancel()
	s := localScanner{root: root, policy: policy, catalog: Catalog{Repository: absolute, Complete: true, Candidates: []Candidate{}, Folders: []string{}, IncompleteReasons: []string{}}}
	if _, err = root.Lstat("SKILL.md"); err == nil {
		s.document(ctx, "")
	} else if !os.IsNotExist(err) {
		return Catalog{}, err
	} else if err = s.walk(ctx, ".", 0); err != nil {
		return Catalog{}, err
	}
	if err = ctx.Err(); err != nil {
		return Catalog{}, err
	}
	sort.Slice(s.catalog.Candidates, func(i, j int) bool { return s.catalog.Candidates[i].Path < s.catalog.Candidates[j].Path })
	return finalize(s.catalog), nil
}

func (s *localScanner) walk(ctx context.Context, directory string, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > s.policy.MaxDepth {
		s.catalog.incomplete("depth_limit")
		return nil
	}
	handle, err := s.openDirectory(directory)
	if err != nil {
		s.catalog.incomplete("directory_unavailable")
		return nil
	}
	defer func() { _ = handle.Close() }()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		remaining := s.policy.MaxEntries - s.entries
		if remaining <= 0 {
			s.catalog.incomplete("entry_limit")
			s.stopped = true
			return nil
		}
		entries, readErr := handle.ReadDir(min(128, remaining))
		for _, entry := range entries {
			if s.stopped {
				return nil
			}
			if err := s.visit(ctx, directory, entry, depth); err != nil {
				return err
			}
		}

		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			s.catalog.incomplete("directory_unavailable")
			return nil
		}
	}
}

func (s *localScanner) document(ctx context.Context, directory string) {
	if directory == "." {
		directory = ""
	}
	if len(s.catalog.Candidates) >= s.policy.MaxCandidates {
		s.catalog.incomplete("candidate_limit")
		return
	}
	c := Candidate{ID: digest([]byte(s.catalog.Repository + "\x00" + directory)), Path: directory, Status: "unavailable", Diagnostics: []string{}}
	body, code := s.readDocument(ctx, path.Join(directory, "SKILL.md"))
	if code != "" {
		c.Diagnostics = append(c.Diagnostics, code)
		if code == "skill_document_wrong_kind" {
			c.Status = "invalid"
		} else {
			s.catalog.incomplete(code)
		}
	} else {
		name := path.Base(directory)
		if directory == "" {
			name = filepath.Base(s.catalog.Repository)
		}
		skill, err := conformance.ParseSkill(ctx, name, body)
		if err != nil {
			reason, unavailable := conformance.SkillErrorCode(err)
			c.Status = "invalid"
			if unavailable {
				c.Status = "unavailable"
				s.catalog.incomplete(reason)
			}
			c.Diagnostics = append(c.Diagnostics, reason)
		} else {
			c.Status = "available"
			c.Name = skill.Name
			c.Description = skill.Description
		}
	}
	s.catalog.Candidates = append(s.catalog.Candidates, c)
}

func (s *localScanner) readDocument(ctx context.Context, name string) ([]byte, string) {
	if ctx.Err() != nil {
		return nil, "skill_document_unavailable"
	}
	if !s.regularAncestry(path.Dir(name)) {
		return nil, "skill_document_unavailable"
	}
	before, err := s.root.Lstat(name)
	if err != nil {
		return nil, "skill_document_unavailable"
	}
	if !before.Mode().IsRegular() {
		return nil, "skill_document_wrong_kind"
	}
	if before.Size() > int64(s.policy.MaxDocumentBytes) {
		return nil, "skill_document_too_large"
	}
	remaining := int64(s.policy.MaxAggregateBytes) - s.catalog.Bytes
	if before.Size() > remaining {
		return nil, "skill_aggregate_bytes_limit"
	}
	file, err := atomicfile.OpenRootReadFile(s.root, name)
	if err != nil {
		return nil, "skill_document_unavailable"
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil || !localSameObservation(before, opened) {
		return nil, "skill_document_unavailable"
	}
	body, err := io.ReadAll(io.LimitReader(file, min(int64(s.policy.MaxDocumentBytes), remaining)+1))
	s.catalog.Bytes += int64(len(body))
	after, statErr := file.Stat()
	observed, pathErr := s.root.Lstat(name)
	if err != nil || statErr != nil || pathErr != nil || !localSameObservation(before, after) || !localSameObservation(before, observed) || int64(len(body)) != before.Size() {
		return nil, "skill_document_unavailable"
	}
	return body, ""
}

func localSameObservation(a, b os.FileInfo) bool {
	return a.Mode().IsRegular() && b.Mode().IsRegular() && os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func (s *localScanner) regularAncestry(directory string) bool {
	for current := directory; current != "."; current = path.Dir(current) {
		info, err := s.root.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}

func (s *localScanner) visit(ctx context.Context, directory string, entry os.DirEntry, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.entries >= s.policy.MaxEntries {
		s.catalog.incomplete("entry_limit")
		s.stopped = true
		return nil
	}
	cost := len(entry.Name()) + 64
	if cost > s.policy.MaxMetadataBytes-s.metadata {
		s.catalog.incomplete("metadata_limit")
		s.stopped = true
		return nil
	}
	s.entries++
	s.metadata += cost
	child := path.Join(directory, entry.Name())
	if entry.Name() == "SKILL.md" {
		s.document(ctx, path.Dir(child))
		return nil
	}
	if entry.Type()&os.ModeSymlink != 0 {
		s.catalog.incomplete("symlink_entry")
		return nil
	}
	if entry.IsDir() {
		return s.walk(ctx, child, depth+1)
	}
	return nil
}

// openDirectory requires directory admission before any potentially blocking open.
func (s *localScanner) openDirectory(directory string) (*os.File, error) {
	info, err := s.root.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("local directory is not regular")
	}
	pinned, err := s.root.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer func() { _ = pinned.Close() }()
	handle, err := pinned.Open(".")
	if err != nil {
		return nil, err
	}
	opened, err := handle.Stat()
	if err != nil || !os.SameFile(info, opened) {
		_ = handle.Close()
		return nil, fmt.Errorf("local directory changed during discovery")
	}
	return handle, nil
}
