package app

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"

func discoverPublicationBaseContext(root, target, dest string) (publicationContext, error) {
	graph, _, err := pluginmanifest.Discover(root)
	if err != nil {
		return publicationContext{}, err
	}
	if _, err := graph.Manifest.SelectedTargets(target); err != nil {
		return publicationContext{}, err
	}
	inspection, _, err := pluginmanifest.Inspect(root, target)
	if err != nil {
		return publicationContext{}, err
	}
	return publicationContext{
		root:       root,
		target:     target,
		dest:       dest,
		graph:      graph,
		inspection: inspection,
	}, nil
}
