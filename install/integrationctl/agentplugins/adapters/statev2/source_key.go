package statev2

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func installedSourceKey(installation domain.Installation) string {
	sourceKey := "github:" + strings.ToLower(installation.Source.Repository) + ":" + installation.Source.PackageSubpath
	if installation.Source.SourceKind != "" {
		sourceKey = sourcespec.Key(sourcespec.Kind(installation.Source.SourceKind), installation.Source.SourceURL, installation.Source.PackageSubpath)
	}
	if installation.Directory != nil {
		sourceKey = "directory:" + strings.ToLower(installation.Directory.DistributionID)
	}
	return sourceKey
}
