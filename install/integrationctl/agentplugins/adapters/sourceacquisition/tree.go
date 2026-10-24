package sourceacquisition

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GitTree owns an isolated, blob-filtered immutable repository. It exposes only
// bounded object reads; no checkout, Git hooks or repository code is executed.
type GitTree struct {
	root, repository, revision string
	runner                     Runner
}
type TreeEntry struct{ Path, Mode, Type, Object string }

func (a Acquirer) OpenGitHubTree(ctx context.Context, repository, revision string) (*GitTree, error) {
	if !repositoryPattern.MatchString(repository) || !fullSHA.MatchString(revision) {
		return nil, fmt.Errorf("immutable GitHub tree requires owner/repo and full lowercase commit SHA")
	}
	root, err := os.MkdirTemp(a.TempRoot, "agentplugins-tree-*")
	if err != nil {
		return nil, err
	}
	t := &GitTree{root: root, repository: repository, revision: revision, runner: a.runner()}
	fail := func(operation string) (*GitTree, error) {
		_ = t.Close()
		return nil, gitAcquisitionFailure(repository, revision, operation)
	}
	url := "https://github.com/" + repository + ".git"
	if a.URLForRepo != nil {
		url = a.URLForRepo(repository)
	}
	for _, args := range [][]string{{"init", "--quiet", root}, {"remote", "add", "origin", url}, {"config", "fetch.recurseSubmodules", "false"}, {"fetch", "--quiet", "--depth=1", "--filter=blob:none", "--no-tags", "--no-recurse-submodules", "origin", revision}} {
		if _, err = t.run(ctx, 4096, args...); err != nil {
			return fail("prepare immutable tree")
		}
	}
	out, err := t.run(ctx, 128, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil || strings.TrimSpace(string(out)) != revision {
		return fail("verify immutable tree")
	}
	return t, nil
}
func (t *GitTree) Close() error { return os.RemoveAll(t.root) }
func (t *GitTree) run(ctx context.Context, limit int, args ...string) ([]byte, error) {
	return t.runner.Run(ctx, Command{Dir: t.root, Args: args, MaxOutputBytes: limit})
}

// List resolves the subtree before enumeration, so callers' budgets apply to
// the selected scope even when the repository's root exceeds those budgets.
func (t *GitTree) List(ctx context.Context, scope string, limit int) ([]TreeEntry, error) {
	if err := ValidateSkillPath(scope); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, fmt.Errorf("tree output bound is required")
	}
	object := t.revision
	if scope != "" {
		out, err := t.run(ctx, 128, "rev-parse", "--verify", t.revision+":"+scope)
		if err != nil {
			return nil, fmt.Errorf("scope is not a tracked directory")
		}
		object = strings.TrimSpace(string(out))
		if !fullSHA.MatchString(object) {
			return nil, fmt.Errorf("invalid subtree object")
		}
		kind, e := t.run(ctx, 32, "cat-file", "-t", object)
		if e != nil || strings.TrimSpace(string(kind)) != "tree" {
			return nil, fmt.Errorf("scope is not a tracked directory")
		}
	}
	out, err := t.run(ctx, limit, "ls-tree", "-r", "-z", object)
	if err != nil {
		return nil, err
	}
	result := make([]TreeEntry, 0)
	for _, record := range bytes.Split(out, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		meta, name, ok := bytes.Cut(record, []byte{'\t'})
		f := bytes.Fields(meta)
		if !ok || len(f) != 3 {
			return nil, fmt.Errorf("invalid Git tree record")
		}
		p := string(name)
		if scope != "" {
			p = scope + "/" + p
		}
		if err := ValidateSkillPath(p); err != nil {
			return nil, fmt.Errorf("unsafe Git tree path")
		}
		if !fullSHA.Match(f[2]) {
			return nil, fmt.Errorf("invalid Git object")
		}
		result = append(result, TreeEntry{Path: p, Mode: string(f[0]), Type: string(f[1]), Object: string(f[2])})
	}
	return result, nil
}
func (t *GitTree) BlobSize(ctx context.Context, oid string) (int64, error) {
	if !fullSHA.MatchString(oid) {
		return 0, fmt.Errorf("invalid blob ID")
	}
	out, err := t.run(ctx, 128, "cat-file", "-s", oid)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid blob size")
	}
	return n, nil
}
func (t *GitTree) Blob(ctx context.Context, oid string, limit int) ([]byte, error) {
	if !fullSHA.MatchString(oid) || limit <= 0 {
		return nil, fmt.Errorf("invalid bounded blob request")
	}
	return t.run(ctx, limit, "cat-file", "blob", oid)
}

// ValidateSkillPath rejects ambiguous spellings instead of cleaning them into
// another source. It shares acquisition's portable path containment rules.
func ValidateSkillPath(value string) error {
	if value == "" {
		return nil
	}
	if value != strings.TrimSpace(value) || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return fmt.Errorf("skill path must be an exact repository-relative directory")
	}
	normalized, err := normalizeSubpath(value)
	if err != nil || normalized != value {
		return fmt.Errorf("skill path must be a safe repository-relative directory")
	}
	if strings.Contains(value, "\x00") || filepath.VolumeName(value) != "" {
		return fmt.Errorf("unsafe skill path")
	}
	return nil
}

// Entry observes exactly one tracked path without enumerating its siblings.
func (t *GitTree) Entry(ctx context.Context, p string) (TreeEntry, error) {
	if p == "" {
		return TreeEntry{}, fmt.Errorf("entry path required")
	}
	if err := ValidateSkillPath(p); err != nil {
		return TreeEntry{}, err
	}
	parent, name := "", p
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		parent, name = p[:i], p[i+1:]
	}
	object := t.revision
	if parent != "" {
		out, e := t.run(ctx, 128, "rev-parse", "--verify", t.revision+":"+parent)
		if e != nil {
			return TreeEntry{}, fmt.Errorf("tracked parent absent")
		}
		object = strings.TrimSpace(string(out))
		if !fullSHA.MatchString(object) {
			return TreeEntry{}, fmt.Errorf("invalid parent object")
		}

		kind, e := t.run(ctx, 32, "cat-file", "-t", object)
		if e != nil || strings.TrimSpace(string(kind)) != "tree" {
			return TreeEntry{}, fmt.Errorf("entry parent is not a tracked directory")
		}
	}
	out, e := t.run(ctx, 8192, "ls-tree", "-z", object, "--", name)
	if e != nil {
		return TreeEntry{}, e
	}
	records := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	if len(records) != 1 || len(records[0]) == 0 {
		return TreeEntry{}, fmt.Errorf("tracked entry absent")
	}
	meta, filename, ok := bytes.Cut(records[0], []byte{'\t'})
	f := bytes.Fields(meta)
	if !ok || len(f) != 3 || string(filename) != name || !fullSHA.Match(f[2]) {
		return TreeEntry{}, fmt.Errorf("invalid tracked entry")
	}
	return TreeEntry{Path: p, Mode: string(f[0]), Type: string(f[1]), Object: string(f[2])}, nil
}
