package codex

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (Adapter) ID() domain.TargetID { return domain.TargetCodex }

func (Adapter) Capabilities(context.Context) (ports.Capabilities, error) {
	return ports.Capabilities{
		InstallMode:          "marketplace_prepare",
		SupportsNativeUpdate: false,
		SupportsNativeRemove: false,
		SupportsScopeUser:    true,
		SupportsScopeProject: true,
		SupportsRepair:       true,
		RequiresRestart:      true,
		RequiresNewThread:    true,
		SupportedSourceKinds: []string{"local_path", "github_repo_path", "git_url"},
		EvidenceKey:          "target.codex.native_surface",
	}, nil
}
