package defs

import (
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func adaptProfile(profile platformmeta.PlatformProfile) PlatformProfile {
	platformID := runtime.PlatformID(profile.ID)
	if profile.ID == "codex-runtime" {
		platformID = "codex"
	}
	return PlatformProfile{
		Platform:        platformID,
		Status:          adaptStatus(profile.SDK.Status),
		PublicPackage:   profile.SDK.PublicPackage,
		InternalPackage: profile.SDK.InternalPackage,
		InternalImport:  profile.SDK.InternalImport,
		TransportModes:  adaptTransportModes(profile.SDK.TransportModes),
		LiveTestProfile: profile.SDK.LiveTestProfile,
		Scaffold: ScaffoldMeta{
			RequiredFiles:  append([]string(nil), profile.Scaffold.RequiredFiles...),
			OptionalFiles:  append([]string(nil), profile.Scaffold.OptionalFiles...),
			ForbiddenFiles: append([]string(nil), profile.Scaffold.ForbiddenFiles...),
			TemplateFiles:  adaptTemplateFiles(profile.Scaffold.TemplateFiles),
		},
		Validate: ValidateMeta{
			RequiredFiles:  append([]string(nil), profile.Validate.RequiredFiles...),
			ForbiddenFiles: append([]string(nil), profile.Validate.ForbiddenFiles...),
			BuildTargets:   append([]string(nil), profile.Validate.BuildTargets...),
		},
	}
}

func adaptStatus(status platformmeta.SupportStatus) runtime.SupportStatus {
	switch status {
	case platformmeta.StatusRuntimeSupported:
		return runtime.StatusRuntimeSupported
	case platformmeta.StatusScaffoldOnly:
		return runtime.StatusScaffoldOnly
	default:
		return runtime.StatusDeferred
	}
}

func adaptTransportModes(modes []platformmeta.TransportMode) []runtime.TransportMode {
	out := make([]runtime.TransportMode, 0, len(modes))
	for _, mode := range modes {
		switch mode {
		case platformmeta.TransportHybrid:
			out = append(out, runtime.HybridMode)
		case platformmeta.TransportDaemon:
			out = append(out, runtime.DaemonMode)
		default:
			out = append(out, runtime.ProcessMode)
		}
	}
	return out
}

func adaptTemplateFiles(files []platformmeta.TemplateFile) []TemplateFile {
	out := make([]TemplateFile, 0, len(files))
	for _, file := range files {
		out = append(out, TemplateFile{
			Path:     file.Path,
			Template: file.Template,
			Extra:    file.Extra,
		})
	}
	return out
}
