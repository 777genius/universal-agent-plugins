package targetcontracts

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func managedArtifactRules(profile platformmeta.PlatformProfile) []ManagedArtifact {
	out := make([]ManagedArtifact, 0, len(profile.ManagedArtifacts))
	for _, item := range profile.ManagedArtifacts {
		switch item.Kind {
		case platformmeta.ManagedArtifactStatic:
			out = append(out, ManagedArtifact{
				Path:      item.Path,
				Condition: managedArtifactCondition(item),
			})
		case platformmeta.ManagedArtifactPortableMCP:
			out = append(out, ManagedArtifact{
				Path:      item.Path,
				Condition: "when portable MCP is authored",
			})
		case platformmeta.ManagedArtifactPortableSkills:
			out = append(out, ManagedArtifact{
				Path:      item.OutputRoot + "/**",
				Condition: "when portable skills are authored",
			})
		case platformmeta.ManagedArtifactMirror:
			path := managedMirrorArtifactPath(profile, item)
			if strings.TrimSpace(path) == "" {
				continue
			}
			out = append(out, ManagedArtifact{
				Path:      path,
				Condition: managedArtifactCondition(item),
			})
		case platformmeta.ManagedArtifactSelectedContext:
			out = append(out, ManagedArtifact{
				Path:      "GEMINI.md or selected root context",
				Condition: "when contexts are authored",
			})
		}
	}
	return out
}

func managedMirrorArtifactPath(profile platformmeta.PlatformProfile, item platformmeta.ManagedArtifactSpec) string {
	if item.OutputRoot != "" {
		return item.OutputRoot + "/**"
	}
	for _, doc := range profile.NativeDocs {
		if doc.Kind == item.ComponentKind {
			return filepath.Base(doc.Path)
		}
	}
	return ""
}

func managedArtifactStrings(items []ManagedArtifact) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		label := item.Path
		if strings.TrimSpace(item.Condition) != "" {
			label += " (" + item.Condition + ")"
		}
		out = append(out, label)
	}
	return out
}

func managedArtifactCondition(item platformmeta.ManagedArtifactSpec) string {
	switch {
	case item.Path == ".app.json":
		return "when app_manifest is enabled"
	case item.ComponentKind != "":
		return authoredCondition(item.ComponentKind)
	case item.OutputRoot != "":
		return authoredCondition(strings.Trim(filepath.Base(item.OutputRoot), "/"))
	default:
		return ""
	}
}

func authoredCondition(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return ""
	}
	verb := "is"
	if strings.HasSuffix(kind, "s") {
		verb = "are"
	}
	return "when " + kind + " " + verb + " authored"
}
