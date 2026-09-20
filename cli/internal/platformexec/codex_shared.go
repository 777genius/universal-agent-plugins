package platformexec

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func codexPackageMetaEqual(left, right codexPackageMeta) bool {
	left.Normalize()
	right.Normalize()
	if (left.Author == nil) != (right.Author == nil) {
		return false
	}
	if left.Author != nil && right.Author != nil {
		if left.Author.Name != right.Author.Name || left.Author.Email != right.Author.Email || left.Author.URL != right.Author.URL {
			return false
		}
	}
	return left.Homepage == right.Homepage &&
		left.Repository == right.Repository &&
		left.License == right.License &&
		slices.Equal(left.Keywords, right.Keywords)
}

func manifestAuthorToCodex(author *pluginmodel.Author) *codexmanifest.Author {
	if author == nil {
		return nil
	}
	out := &codexmanifest.Author{
		Name:  strings.TrimSpace(author.Name),
		Email: strings.TrimSpace(author.Email),
		URL:   strings.TrimSpace(author.URL),
	}
	out.Normalize()
	if out.Empty() {
		return nil
	}
	return out
}

func mergeCodexPackageMeta(dst *codexPackageMeta, override codexPackageMeta) {
	if dst == nil {
		return
	}
	override.Normalize()
	if override.Author != nil && !override.Author.Empty() {
		dst.Author = override.Author
	}
	if strings.TrimSpace(override.Homepage) != "" {
		dst.Homepage = override.Homepage
	}
	if strings.TrimSpace(override.Repository) != "" {
		dst.Repository = override.Repository
	}
	if strings.TrimSpace(override.License) != "" {
		dst.License = override.License
	}
	if len(override.Keywords) > 0 {
		dst.Keywords = append([]string(nil), override.Keywords...)
	}
}

func cloneStringMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	body, _ := json.Marshal(values)
	out := map[string]any{}
	_ = json.Unmarshal(body, &out)
	return out
}
