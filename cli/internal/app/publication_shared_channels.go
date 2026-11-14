package app

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func channelsNeedLocalDest(channels []publicationmodel.Channel) bool {
	for _, channel := range channels {
		switch channel.Family {
		case "codex-marketplace", "claude-marketplace":
			return true
		}
	}
	return false
}

func cleanedDestForMulti(dest string, channels []publicationmodel.Channel) string {
	if !channelsNeedLocalDest(channels) {
		return ""
	}
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return ""
	}
	return filepath.Clean(dest)
}

func publicationPackageForTarget(model publicationmodel.Model, target string) (publicationmodel.Package, bool) {
	for _, pkg := range model.Packages {
		if pkg.Target == target {
			return pkg, true
		}
	}
	return publicationmodel.Package{}, false
}

func publicationChannelForTarget(model publicationmodel.Model, target string) (publicationmodel.Channel, bool) {
	for _, channel := range model.Channels {
		if slices.Contains(channel.PackageTargets, target) {
			return channel, true
		}
	}
	return publicationmodel.Channel{}, false
}

func publicationChannelForFamily(model publicationmodel.Model, family string) (publicationmodel.Channel, bool) {
	for _, channel := range model.Channels {
		if channel.Family == family {
			return channel, true
		}
	}
	return publicationmodel.Channel{}, false
}
