package platformmeta

// TargetContractMeta describes the author-facing contract for a platform target.
type TargetContractMeta struct {
	// PlatformFamily groups the target into a broad integration family.
	PlatformFamily PlatformFamily
	// TargetClass names the internal target class used by docs and scaffolds.
	TargetClass string
	// TargetNoun is the user-facing noun for the produced artifact.
	TargetNoun string
	// ProductionClass summarizes the intended production posture.
	ProductionClass string
	// RuntimeContract describes the public runtime support promise.
	RuntimeContract string
	// InstallModel describes how the target is installed by end users.
	InstallModel string
	// DevModel describes the expected local development loop.
	DevModel string
	// ActivationModel describes how changes become active in the target.
	ActivationModel string
	// NativeRoot points to the target's native install or config root.
	NativeRoot string
	// ImportSupport reports whether `plugin-kit-ai import` is supported.
	ImportSupport bool
	// GenerateSupport reports whether `plugin-kit-ai generate` is supported.
	GenerateSupport bool
	// ValidateSupport reports whether `plugin-kit-ai validate` is supported.
	ValidateSupport bool
	// PortableComponentKinds lists portable authoring components the target consumes.
	PortableComponentKinds []string
	// TargetComponentKinds lists native target components generated for the target.
	TargetComponentKinds []string
	// Summary provides the high-level target description used in docs.
	Summary string
}

// SDKMeta describes the runtime SDK package associated with a platform target.
type SDKMeta struct {
	// PublicPackage is the public SDK package name for the target.
	PublicPackage string
	// InternalPackage is the internal runtime package name used by generators.
	InternalPackage string
	// InternalImport is the import path for the internal runtime package.
	InternalImport string
	// Status is the support status for the target's runtime lane.
	Status SupportStatus
	// TransportModes lists the supported runtime transport modes.
	TransportModes []TransportMode
	// LiveTestProfile names the live integration test profile for the target.
	LiveTestProfile string
}

// LauncherMeta describes whether a launcher is required for the target.
type LauncherMeta struct {
	Requirement LauncherRequirement
}

// NativeDocSpec describes one target-native config file or manifest surface.
type NativeDocSpec struct {
	// Kind is the normalized component kind represented by the native file.
	Kind string
	// Path is the target-relative path to the native file.
	Path string
	// Format is the native file format.
	Format NativeDocFormat
	// Role describes whether the file is structured or extra.
	Role NativeDocRole
	// ManagedKeys lists keys that plugin-kit-ai manages in extra docs.
	ManagedKeys []string
}

// ManagedArtifactSpec describes an artifact managed or mirrored by the toolchain.
type ManagedArtifactSpec struct {
	// Kind describes how the artifact is managed.
	Kind ManagedArtifactKind
	// Path is the file path for single-file artifacts.
	Path string
	// ComponentKind identifies the component family for mirrored artifacts.
	ComponentKind string
	// SourceRoot is the source directory for mirrored artifacts.
	SourceRoot string
	// OutputRoot is the destination directory for mirrored artifacts.
	OutputRoot string
	// ContextMode controls how contextual sources are selected.
	ContextMode ContextStrategy
}

// SurfaceSupport describes the maturity tier for one surface kind.
type SurfaceSupport struct {
	// Kind is the normalized surface kind name.
	Kind string
	// Tier is the maturity tier for that surface kind.
	Tier SurfaceTier
}

// PlatformProfile collects the public metadata for one supported target platform.
type PlatformProfile struct {
	// ID is the normalized platform identifier.
	ID string
	// Contract describes the author-facing target contract.
	Contract TargetContractMeta
	// SDK describes the runtime SDK metadata for the target.
	SDK SDKMeta
	// Launcher describes launcher requirements for the target.
	Launcher LauncherMeta
	// NativeDocs enumerates target-native config files and manifests.
	NativeDocs []NativeDocSpec
	// SurfaceTiers enumerates maturity tiers for target-native surfaces.
	SurfaceTiers []SurfaceSupport
	// ManagedArtifacts enumerates generated or mirrored artifacts.
	ManagedArtifacts []ManagedArtifactSpec
	// Scaffold describes `init` output for the target.
	Scaffold ScaffoldMeta
	// Validate describes `validate` rules for the target.
	Validate ValidateMeta
}
