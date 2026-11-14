package claude

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (Adapter) ID() domain.TargetID { return domain.TargetClaude }

func (Adapter) Capabilities(context.Context) (ports.Capabilities, error) {
	return ports.Capabilities{
		InstallMode:              "native_cli",
		SupportsNativeUpdate:     true,
		SupportsNativeRemove:     true,
		SupportsAutoUpdatePolicy: true,
		SupportsScopeUser:        true,
		SupportsScopeProject:     true,
		RequiresReload:           true,
		SupportedSourceKinds:     []string{"local_path", "github_repo_path", "git_url"},
		EvidenceKey:              "target.claude.native_surface",
	}, nil
}
