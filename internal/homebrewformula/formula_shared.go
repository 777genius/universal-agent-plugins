package homebrewformula

import (
	"fmt"
	"strings"
)

func mustAsset(assets []Asset, goos, goarch string) Asset {
	for _, asset := range assets {
		if asset.GOOS == goos && asset.GOARCH == goarch {
			return asset
		}
	}
	panic(fmt.Sprintf("missing asset for %s/%s", goos, goarch))
}

func normalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	if strings.HasPrefix(tag, "plugin-kit-ai-v") || strings.HasPrefix(tag, "v") {
		return tag
	}
	return "plugin-kit-ai-v" + tag
}

func versionFromTag(tag string) string {
	if strings.HasPrefix(tag, "plugin-kit-ai-v") {
		return strings.TrimPrefix(tag, "plugin-kit-ai-v")
	}
	return strings.TrimPrefix(tag, "v")
}
