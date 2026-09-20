package app

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

func inspectGeminiPublicationChannel(root string) (publicationmodel.Channel, error) {
	inspection, _, err := pluginmanifest.Inspect(root, "gemini")
	if err != nil {
		return publicationmodel.Channel{}, err
	}
	publication := inspection.Publication
	if _, ok := publicationPackageForTarget(publication, "gemini"); !ok {
		return publicationmodel.Channel{}, fmt.Errorf("target %s is not publication-capable", "gemini")
	}
	channel, ok := publicationChannelForFamily(publication, "gemini-gallery")
	if !ok {
		return publicationmodel.Channel{}, fmt.Errorf("target %s requires authored publication channel metadata under %s", "gemini", publishschema.GeminiGalleryRel)
	}
	return channel, nil
}
