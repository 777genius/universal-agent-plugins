package gemini

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type Adapter struct {
	Runner   ports.ProcessRunner
	FS       ports.FileSystem
	UserHome string
}

func (Adapter) ID() domain.TargetID { return domain.TargetGemini }

func (Adapter) Capabilities(context.Context) (ports.Capabilities, error) {
	return ports.Capabilities{
		InstallMode:              "native_cli",
		SupportsNativeUpdate:     true,
		SupportsNativeRemove:     true,
		SupportsLinkMode:         true,
		SupportsAutoUpdatePolicy: true,
		SupportsScopeUser:        true,
		SupportsScopeProject:     true,
		SupportsRepair:           true,
		RequiresRestart:          true,
		SupportedSourceKinds:     []string{"local_path", "github_repo_path", "git_url"},
		EvidenceKey:              "target.gemini.native_surface",
	}, nil
}
