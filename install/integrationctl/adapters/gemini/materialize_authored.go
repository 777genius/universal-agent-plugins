package gemini

import (
	"context"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

type packageMeta struct {
	ContextFileName string   `yaml:"context_file_name,omitempty"`
	ExcludeTools    []string `yaml:"exclude_tools,omitempty"`
	PlanDirectory   string   `yaml:"plan_directory,omitempty"`
}

func (a Adapter) materializeAuthoredGeminiSource(ctx context.Context, manifest domain.IntegrationManifest, sourceRoot, destRoot string) error {
	meta, err := loadPackageMeta(authoredpath.Join(sourceRoot, "targets", "gemini", "package.yaml"))
	if err != nil {
		return err
	}
	doc, err := a.materializedGeminiManifest(ctx, manifest, sourceRoot, meta)
	if err != nil {
		return err
	}
	if contextName, err := materializeAuthoredContexts(sourceRoot, destRoot, meta); err != nil {
		return err
	} else if contextName != "" {
		doc["contextFileName"] = contextName
	}
	body, err := marshalJSON(doc)
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "marshal Gemini manifest", err)
	}
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return domain.NewError(domain.ErrMutationApply, "prepare Gemini materialized root", err)
	}
	if err := os.WriteFile(filepath.Join(destRoot, "gemini-extension.json"), body, 0o644); err != nil {
		return domain.NewError(domain.ErrMutationApply, "write Gemini materialized manifest", err)
	}
	return a.copyAuthoredGeminiSupportFiles(sourceRoot, destRoot)
}
