package gemini

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) copyAuthoredGeminiSupportFiles(sourceRoot, destRoot string) error {
	for _, pair := range [][2]string{
		{authoredpath.Join(sourceRoot, "targets", "gemini", "commands"), filepath.Join(destRoot, "commands")},
		{authoredpath.Join(sourceRoot, "targets", "gemini", "policies"), filepath.Join(destRoot, "policies")},
		{authoredpath.Join(sourceRoot, "targets", "gemini", "agents"), filepath.Join(destRoot, "agents")},
		{authoredpath.Join(sourceRoot, "skills"), filepath.Join(destRoot, "skills")},
	} {
		if err := copyDirIfExists(pair[0], pair[1]); err != nil {
			return domain.NewError(domain.ErrMutationApply, "copy Gemini authored directory", err)
		}
	}
	return copyAuthoredGeminiHooks(sourceRoot, destRoot)
}

func copyAuthoredGeminiHooks(sourceRoot, destRoot string) error {
	hooksSrc := authoredpath.Join(sourceRoot, "targets", "gemini", "hooks", "hooks.json")
	if !fileExists(hooksSrc) {
		return nil
	}
	if err := copyFile(hooksSrc, filepath.Join(destRoot, "hooks", "hooks.json")); err != nil {
		return domain.NewError(domain.ErrMutationApply, "copy Gemini hooks", err)
	}
	return nil
}
