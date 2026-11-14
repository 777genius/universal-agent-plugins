package cursor

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (Adapter) ID() domain.TargetID { return domain.TargetCursor }

func (Adapter) Capabilities(context.Context) (ports.Capabilities, error) {
	return ports.Capabilities{
		InstallMode:               "plugin_materialization_or_config_projection",
		SupportsNativeUpdate:      false,
		SupportsNativeRemove:      true,
		SupportsRepair:            true,
		SupportsScopeUser:         true,
		SupportsScopeProject:      true,
		MayTriggerInteractiveAuth: true,
		SupportedSourceKinds:      []string{"local_path", "github_repo_path", "git_url"},
		EvidenceKey:               "target.cursor.native_surface",
	}, nil
}
