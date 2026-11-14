package codex

import (
	"context"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

type packageMeta struct {
	Author     *author  `yaml:"author,omitempty"`
	Homepage   string   `yaml:"homepage,omitempty"`
	Repository string   `yaml:"repository,omitempty"`
	License    string   `yaml:"license,omitempty"`
	Keywords   []string `yaml:"keywords,omitempty"`
}

type author struct {
	Name  string `yaml:"name,omitempty"`
	Email string `yaml:"email,omitempty"`
	URL   string `yaml:"url,omitempty"`
}

func (a Adapter) syncManagedPlugin(ctx context.Context, manifest domain.IntegrationManifest, sourceRoot, pluginRoot string) error {
	root := filepath.Clean(sourceRoot)
	parent := filepath.Dir(pluginRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return domain.NewError(domain.ErrMutationApply, "prepare Codex plugin parent", err)
	}
	tmpRoot, err := os.MkdirTemp(parent, manifest.IntegrationID+".tmp-*")
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "create Codex materialization temp root", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpRoot)
		}
	}()
	if authoredpath.HasManifest(root) {
		if err := a.materializeAuthoredCodexSource(ctx, manifest, root, tmpRoot); err != nil {
			return err
		}
	} else if fileExists(filepath.Join(root, ".codex-plugin", "plugin.json")) {
		if err := copyNativeCodexPackage(root, tmpRoot); err != nil {
			return err
		}
	} else {
		if err := a.materializeAuthoredCodexSource(ctx, manifest, root, tmpRoot); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(pluginRoot); err != nil && !os.IsNotExist(err) {
		return domain.NewError(domain.ErrMutationApply, "replace Codex managed plugin root", err)
	}
	if err := os.Rename(tmpRoot, pluginRoot); err != nil {
		return domain.NewError(domain.ErrMutationApply, "activate Codex managed plugin root", err)
	}
	cleanup = false
	return nil
}
