package gitsource

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

// NewHosted refuses to expose Git work without the declared runtime envelope.
// Tests instantiate the same core adapter with local Git fixtures separately.
func NewHosted(tempDir string) (Source, error) {
	if !filepath.IsAbs(tempDir) || os.Geteuid() == 0 {
		return Source{}, fmt.Errorf("resolver requires an unprivileged bounded runtime")
	}
	info, err := os.Lstat(tempDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Source{}, fmt.Errorf("resolver scratch directory unavailable")
	}
	if err = checkRuntimeEnvelope(tempDir); err != nil {
		return Source{}, err
	}
	return Source{Acquirer: skillsource.Acquirer{
		Acquirer: sourceacquisition.Acquirer{TempRoot: tempDir, Runner: hostedRunner{}}, Policy: Policy,
	}}, nil
}
