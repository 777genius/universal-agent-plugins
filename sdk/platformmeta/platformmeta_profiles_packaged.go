package platformmeta

func packagedProfiles() []PlatformProfile {
	return []PlatformProfile{
		claudeProfile(),
		codexPackageProfile(),
		codexRuntimeProfile(),
	}
}
