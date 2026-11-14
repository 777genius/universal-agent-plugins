package targetcontracts

import (
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func fromProfile(profile platformmeta.PlatformProfile) Entry {
	rules := managedArtifactRules(profile)
	return Entry{
		Target:                 profile.ID,
		PlatformFamily:         string(profile.Contract.PlatformFamily),
		TargetClass:            profile.Contract.TargetClass,
		LauncherRequirement:    string(profile.Launcher.Requirement),
		TargetNoun:             profile.Contract.TargetNoun,
		ProductionClass:        profile.Contract.ProductionClass,
		RuntimeContract:        profile.Contract.RuntimeContract,
		InstallModel:           profile.Contract.InstallModel,
		DevModel:               profile.Contract.DevModel,
		ActivationModel:        profile.Contract.ActivationModel,
		NativeRoot:             profile.Contract.NativeRoot,
		ImportSupport:          profile.Contract.ImportSupport,
		GenerateSupport:        profile.Contract.GenerateSupport,
		ValidateSupport:        profile.Contract.ValidateSupport,
		PortableComponentKinds: cloneStrings(profile.Contract.PortableComponentKinds),
		TargetComponentKinds:   cloneStrings(profile.Contract.TargetComponentKinds),
		NativeDocs:             nativeDocs(profile.NativeDocs),
		NativeDocPaths:         nativeDocPaths(profile.NativeDocs),
		NativeSurfaces:         fromSurfaceSupport(profile.SurfaceTiers),
		NativeSurfaceTiers:     nativeSurfaceTiers(profile.SurfaceTiers),
		ManagedArtifactRules:   rules,
		ManagedArtifacts:       managedArtifactStrings(rules),
		Summary:                profile.Contract.Summary,
	}
}

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	return append([]string{}, items...)
}
