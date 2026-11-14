package platformmeta

// All returns the full set of public platform profiles.
func All() []PlatformProfile {
	profiles := make([]PlatformProfile, 0, len(packagedProfiles())+len(toolingProfiles()))
	profiles = append(profiles, packagedProfiles()...)
	profiles = append(profiles, toolingProfiles()...)
	return profiles
}

// Lookup resolves a platform profile by normalized platform name.
func Lookup(name string) (PlatformProfile, bool) {
	name = normalizeName(name)
	for _, profile := range All() {
		if profile.ID == name {
			return profile, true
		}
	}
	return PlatformProfile{}, false
}

// IDs returns the normalized identifiers for every known platform profile.
func IDs() []string {
	profiles := All()
	out := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, profile.ID)
	}
	return out
}
