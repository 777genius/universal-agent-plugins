package integrationctl

import (
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/cursor"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/evidence"
	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/journal"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/jsonstate"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/locks"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/manifest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/safemutate"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/source"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/workspacelock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/usecase"
)

func newService() (usecase.Service, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return usecase.Service{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return usecase.Service{}, err
	}
	repoRoot := discoverRepoRoot(cwd)
	fs := fsadapter.OS{}
	mutator := safemutate.OS{}
	service := usecase.Service{
		SourceResolver:       source.Resolver{Runner: process.OS{}},
		ManifestLoader:       manifest.Loader{},
		StateStore:           jsonstate.Store{FS: fs, Path: filepath.Join(home, ".plugin-kit-ai", "state.json")},
		WorkspaceLock:        workspacelock.Store{FS: fs, File: filepath.Join(repoRoot, domain.WorkspaceLockFileName)},
		LockManager:          locks.FileLock{BaseDir: filepath.Join(home, ".plugin-kit-ai", "locks")},
		Journal:              journal.FileJournal{FS: fs, BaseDir: filepath.Join(home, ".plugin-kit-ai", "operations")},
		Evidence:             evidence.Registry{FS: fs, Path: filepath.Join(repoRoot, "docs", "generated", "integrationctl_evidence_registry.json")},
		CurrentWorkspaceRoot: cwd,
		Adapters:             newTargetAdapters(fs, mutator, cwd, home),
	}
	return service, nil
}

func newTargetAdapters(fs ports.FileSystem, mutator safemutate.OS, cwd string, home string) map[domain.TargetID]ports.TargetAdapter {
	return map[domain.TargetID]ports.TargetAdapter{
		domain.TargetClaude:   claude.Adapter{Runner: process.OS{}, FS: fs, ProjectRoot: cwd, UserHome: home},
		domain.TargetCodex:    codex.Adapter{FS: fs, ProjectRoot: cwd, UserHome: home},
		domain.TargetGemini:   gemini.Adapter{Runner: process.OS{}, FS: fs, UserHome: home},
		domain.TargetCursor:   cursor.Adapter{FS: fs, SafeMutator: mutator, ProjectRoot: cwd, UserHome: home},
		domain.TargetOpenCode: opencode.Adapter{FS: fs, SafeMutator: mutator, ProjectRoot: cwd, UserHome: home},
	}
}
