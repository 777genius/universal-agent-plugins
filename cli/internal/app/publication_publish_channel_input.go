package app

import "fmt"

func newLocalPublishInput(opts PluginPublishOptions, channel string) (PluginPublicationMaterializeOptions, error) {
	target, err := publishTargetForChannel(channel)
	if err != nil {
		return PluginPublicationMaterializeOptions{}, err
	}
	return PluginPublicationMaterializeOptions{
		Root:        opts.Root,
		Target:      target,
		Dest:        opts.Dest,
		PackageRoot: opts.PackageRoot,
		DryRun:      opts.DryRun,
	}, nil
}

func publishTargetForChannel(channel string) (string, error) {
	switch channel {
	case "codex-marketplace":
		return "codex-package", nil
	case "claude-marketplace":
		return "claude", nil
	default:
		return "", fmt.Errorf("unsupported publish channel %q", channel)
	}
}
