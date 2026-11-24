//go:build linux

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Project acquisition must not change persistent User cache timestamps by
// creating and deleting its disposable Git checkout under that cache root.
func TestProjectSourceScratchPreservesUserCacheRoot(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "user-cache")
	projectRoot := filepath.Join(home, "project")
	scratchBase := filepath.Join(home, "scratch")
	for _, path := range []string{cache, projectRoot, scratchBase} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TMPDIR", scratchBase)
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	user, err := newAgentpluginsCLIApp(home, cache, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	userSource, ok := user.SourceAcquirer.(lazySourceAcquirer)
	if !ok || userSource.acquirer.TempRoot != cache {
		t.Fatal("User acquisition scratch root changed")
	}
	project, err := user.ForScope(domain.ScopeProject, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	projectSource, ok := project.SourceAcquirer.(lazySourceAcquirer)
	if !ok || projectSource.dataRoot != cache || projectSource.acquirer.TempRoot != "" {
		t.Fatal("Project acquisition still targets persistent User cache scratch")
	}
	before, err := os.Stat(cache)
	if err != nil {
		t.Fatal(err)
	}
	temporary, err := os.MkdirTemp(projectSource.acquirer.TempRoot, "agentplugins-git-*")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(temporary) != scratchBase {
		t.Fatal("Project source workspace did not use the isolated OS temp parent")
	}
	if err := os.RemoveAll(temporary); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(cache)
	if err != nil {
		t.Fatal(err)
	}
	first, okFirst := before.Sys().(*syscall.Stat_t)
	last, okLast := after.Sys().(*syscall.Stat_t)
	if !okFirst || !okLast || !os.SameFile(before, after) ||
		before.ModTime() != after.ModTime() || first.Ctim != last.Ctim {
		t.Fatal("Project source workspace changed held User cache identity or metadata")
	}
}
