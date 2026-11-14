package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (r Resolver) runner() ports.ProcessRunner {
	if r.Runner != nil {
		return r.Runner
	}
	return process.OS{}
}

type clonedSource struct {
	PackageRoot string
	CleanupRoot string
	Commit      string
}

func (source clonedSource) cleanup() error {
	if strings.TrimSpace(source.CleanupRoot) == "" {
		return nil
	}
	return os.RemoveAll(source.CleanupRoot)
}

func (r Resolver) cloneGitHub(ctx context.Context, ownerRepo, subdir, gitRef string) (clonedSource, error) {
	return r.clone(ctx, "https://github.com/"+ownerRepo+".git", subdir, gitRef)
}

func (r Resolver) cloneURL(ctx context.Context, raw string, gitRef string) (clonedSource, error) {
	return r.clone(ctx, raw, "", gitRef)
}

func (r Resolver) clone(ctx context.Context, repoURL, subdir, gitRef string) (clonedSource, error) {
	normalizedSubdir := ""
	if strings.TrimSpace(subdir) != "" {
		var err error
		normalizedSubdir, err = normalizePackageSubdir(subdir)
		if err != nil {
			return clonedSource{}, err
		}
	}
	tmp, err := os.MkdirTemp("", "integrationctl-source-*")
	if err != nil {
		return clonedSource{}, err
	}
	cloneResult, err := r.runner().Run(ctx, ports.Command{
		Argv: []string{"git", "clone", "--depth", "1", "--", repoURL, tmp},
	})
	if err != nil {
		_ = os.RemoveAll(tmp)
		if isCommandNotFound(err) {
			return clonedSource{}, fmt.Errorf("git not found")
		}
		return clonedSource{}, err
	}
	if cloneResult.ExitCode != 0 {
		_ = os.RemoveAll(tmp)
		return clonedSource{}, fmt.Errorf("git clone failed: %s", commandOutput(cloneResult))
	}
	if strings.TrimSpace(gitRef) != "" {
		if err := r.checkoutRef(ctx, tmp, gitRef); err != nil {
			_ = os.RemoveAll(tmp)
			return clonedSource{}, err
		}
	}
	revResult, err := r.runner().Run(ctx, ports.Command{
		Argv: []string{"git", "-C", tmp, "rev-parse", "HEAD"},
	})
	if err != nil {
		_ = os.RemoveAll(tmp)
		if isCommandNotFound(err) {
			return clonedSource{}, fmt.Errorf("git not found")
		}
		return clonedSource{}, err
	}
	if revResult.ExitCode != 0 {
		_ = os.RemoveAll(tmp)
		return clonedSource{}, fmt.Errorf("git rev-parse failed: %s", commandOutput(revResult))
	}
	root := tmp
	if normalizedSubdir != "" {
		root = filepath.Join(tmp, filepath.FromSlash(normalizedSubdir))
		if err := validateContainedDirectory(tmp, root); err != nil {
			_ = os.RemoveAll(tmp)
			return clonedSource{}, fmt.Errorf("unsafe source subdir %q: %w", normalizedSubdir, err)
		}
	}
	if info, err := os.Lstat(root); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		_ = os.RemoveAll(tmp)
		return clonedSource{}, fmt.Errorf("source subdir not found: %s", normalizedSubdir)
	}
	return clonedSource{
		PackageRoot: root,
		CleanupRoot: tmp,
		Commit:      strings.TrimSpace(string(revResult.Stdout)),
	}, nil
}

func (r Resolver) checkoutRef(ctx context.Context, repoRoot, gitRef string) error {
	fetchResult, err := r.runner().Run(ctx, ports.Command{
		Argv: []string{"git", "-C", repoRoot, "fetch", "--depth", "1", "origin", gitRef},
	})
	if err != nil {
		if isCommandNotFound(err) {
			return fmt.Errorf("git not found")
		}
		return err
	}
	if fetchResult.ExitCode != 0 {
		return fmt.Errorf("git fetch %q failed: %s", gitRef, commandOutput(fetchResult))
	}
	checkoutResult, err := r.runner().Run(ctx, ports.Command{
		Argv: []string{"git", "-C", repoRoot, "checkout", "FETCH_HEAD"},
	})
	if err != nil {
		if isCommandNotFound(err) {
			return fmt.Errorf("git not found")
		}
		return err
	}
	if checkoutResult.ExitCode != 0 {
		return fmt.Errorf("git checkout %q failed: %s", gitRef, commandOutput(checkoutResult))
	}
	return nil
}
