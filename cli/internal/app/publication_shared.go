package app

import (
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func publicationModeLabel(dryRun bool) string {
	if dryRun {
		return "dry-run"
	}
	return "apply"
}

func orderedPublicationChannels(model publicationmodel.Model) []publicationmodel.Channel {
	out := append([]publicationmodel.Channel(nil), model.Channels...)
	slices.SortFunc(out, func(a, b publicationmodel.Channel) int {
		oa, oka := publicationChannelOrder(a.Family)
		ob, okb := publicationChannelOrder(b.Family)
		switch {
		case oka && okb && oa != ob:
			return oa - ob
		case oka && !okb:
			return -1
		case !oka && okb:
			return 1
		default:
			return strings.Compare(a.Family, b.Family)
		}
	})
	return out
}

func publicationChannelOrder(family string) (int, bool) {
	switch family {
	case "codex-marketplace":
		return 0, true
	case "claude-marketplace":
		return 1, true
	case "gemini-gallery":
		return 2, true
	default:
		return 0, false
	}
}
