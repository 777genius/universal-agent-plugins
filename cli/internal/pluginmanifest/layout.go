package pluginmanifest

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type authoredLayout struct {
	RootRel string
}

func (l authoredLayout) IsCanonical() bool {
	return filepath.ToSlash(strings.TrimSpace(l.RootRel)) == pluginmodel.SourceDirName
}

func (l authoredLayout) Path(rel string) string {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" {
		return filepath.ToSlash(strings.TrimSpace(l.RootRel))
	}
	if strings.TrimSpace(l.RootRel) == "" {
		return rel
	}
	return filepath.ToSlash(filepath.Join(l.RootRel, rel))
}
