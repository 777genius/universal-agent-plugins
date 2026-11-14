package generator

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func renderTargetSupportMatrixRow(profile platformmeta.PlatformProfile) string {
	return "| " + profile.ID + " | " +
		string(profile.Contract.PlatformFamily) + " | " +
		profile.Contract.TargetClass + " | " +
		string(profile.Launcher.Requirement) + " | " +
		profile.Contract.TargetNoun + " | " +
		profile.Contract.InstallModel + " | " +
		profile.Contract.DevModel + " | " +
		profile.Contract.ActivationModel + " | " +
		profile.Contract.NativeRoot + " | " +
		profile.Contract.ProductionClass + " | " +
		profile.Contract.RuntimeContract + " | " +
		boolString(profile.Contract.ImportSupport) + " | " +
		boolString(profile.Contract.GenerateSupport) + " | " +
		boolString(profile.Contract.ValidateSupport) + " | " +
		joinStrings(profile.Contract.PortableComponentKinds) + " | " +
		joinStrings(profile.Contract.TargetComponentKinds) + " | " +
		joinNativeDocs(profile.NativeDocs) + " | " +
		joinSurfaceSupport(profile.SurfaceTiers) + " | " +
		joinManagedArtifacts(profile) + " | " +
		profile.Contract.Summary + " |\n"
}

func boolString(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func joinStrings(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ", ")
}

func joinSurfaceSupport(items []platformmeta.SurfaceSupport) string {
	if len(items) == 0 {
		return "-"
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Kind+"="+string(item.Tier))
	}
	return strings.Join(out, ", ")
}

func joinNativeDocs(items []platformmeta.NativeDocSpec) string {
	if len(items) == 0 {
		return "-"
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Path) == "" {
			continue
		}
		out = append(out, item.Kind+"="+authoringDocPath(item.Path))
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ", ")
}

func authoringDocPath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	if path == "src" {
		return platformmeta.CanonicalAuthoredRoot
	}
	if strings.HasPrefix(path, "src/") {
		return filepath.ToSlash(filepath.Join(platformmeta.CanonicalAuthoredRoot, strings.TrimPrefix(path, "src/")))
	}
	if path == platformmeta.CanonicalAuthoredRoot || strings.HasPrefix(path, platformmeta.CanonicalAuthoredRoot+"/") {
		return path
	}
	return filepath.ToSlash(filepath.Join(platformmeta.CanonicalAuthoredRoot, path))
}

func joinManagedArtifacts(profile platformmeta.PlatformProfile) string {
	if len(profile.ManagedArtifacts) == 0 {
		return "-"
	}
	out := make([]string, 0, len(profile.ManagedArtifacts))
	for _, item := range profile.ManagedArtifacts {
		label := managedArtifactLabel(profile, item)
		if strings.TrimSpace(label) == "" {
			continue
		}
		if condition := managedArtifactCondition(item); strings.TrimSpace(condition) != "" {
			label += " (" + condition + ")"
		}
		out = append(out, label)
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ", ")
}

func managedArtifactLabel(profile platformmeta.PlatformProfile, item platformmeta.ManagedArtifactSpec) string {
	switch item.Kind {
	case platformmeta.ManagedArtifactStatic, platformmeta.ManagedArtifactPortableMCP:
		return item.Path
	case platformmeta.ManagedArtifactPortableSkills:
		return item.OutputRoot + "/**"
	case platformmeta.ManagedArtifactMirror:
		if item.OutputRoot != "" {
			return item.OutputRoot + "/**"
		}
		return managedArtifactMirrorDocLabel(profile, item.ComponentKind)
	case platformmeta.ManagedArtifactSelectedContext:
		return "GEMINI.md or selected root context"
	default:
		return ""
	}
}

func managedArtifactMirrorDocLabel(profile platformmeta.PlatformProfile, kind string) string {
	for _, doc := range profile.NativeDocs {
		if doc.Kind == kind {
			if strings.TrimSpace(doc.Path) == "" {
				return ""
			}
			return filepath.Base(doc.Path)
		}
	}
	return ""
}

func managedArtifactCondition(item platformmeta.ManagedArtifactSpec) string {
	switch {
	case item.Kind == platformmeta.ManagedArtifactPortableMCP:
		return "when portable MCP is authored"
	case item.Kind == platformmeta.ManagedArtifactPortableSkills:
		return "when portable skills are authored"
	case item.Kind == platformmeta.ManagedArtifactSelectedContext:
		return "when contexts are authored"
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
