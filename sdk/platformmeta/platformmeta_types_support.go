package platformmeta

// SupportStatus describes the lifecycle status of a platform profile.
type SupportStatus string

// TransportMode describes how a plugin talks to the target platform.
type TransportMode string

// PlatformFamily groups targets by their broad integration model.
type PlatformFamily string

// LauncherRequirement indicates whether a launcher file is required.
type LauncherRequirement string

// NativeDocRole classifies target-native docs as structured or extra.
type NativeDocRole string

// NativeDocFormat identifies the file format of target-native docs.
type NativeDocFormat string

// ManagedArtifactKind describes how plugin-kit-ai manages generated artifacts.
type ManagedArtifactKind string

// ContextStrategy describes how contextual files are selected or projected.
type ContextStrategy string

// SurfaceTier describes the maturity of a target-native surface area.
type SurfaceTier string

const (
	// StatusRuntimeSupported means the platform has a supported runtime contract.
	StatusRuntimeSupported SupportStatus = "runtime_supported"
	// StatusScaffoldOnly means the platform supports scaffold output but not runtime dispatch.
	StatusScaffoldOnly SupportStatus = "scaffold_only"
	// StatusDeferred means the platform is modeled but intentionally deferred.
	StatusDeferred SupportStatus = "deferred"
)

const (
	// TransportProcess uses direct process execution for hook dispatch.
	TransportProcess TransportMode = "process"
	// TransportHybrid combines process execution with target-specific helpers.
	TransportHybrid TransportMode = "hybrid"
	// TransportDaemon uses a long-lived daemon or service boundary.
	TransportDaemon TransportMode = "daemon"
)

const (
	// FamilyPackagedRuntime describes packaged runtime plugins.
	FamilyPackagedRuntime PlatformFamily = "packaged_runtime"
	// FamilyExtensionPackage describes extension or IDE package targets.
	FamilyExtensionPackage PlatformFamily = "extension_package"
	// FamilyCodePlugin describes repo-local code plugins.
	FamilyCodePlugin PlatformFamily = "code_plugin"
	// FamilyIDEPlugin describes IDE plugin targets with dedicated shells.
	FamilyIDEPlugin PlatformFamily = "ide_plugin"
)

const (
	// LauncherRequired means launcher.yaml is required for the target.
	LauncherRequired LauncherRequirement = "required"
	// LauncherOptional means launcher.yaml may be present but is not required.
	LauncherOptional LauncherRequirement = "optional"
	// LauncherIgnored means launcher.yaml is not used for the target.
	LauncherIgnored LauncherRequirement = "ignored"
)

const (
	// NativeDocRoleStructured marks primary structured target config files.
	NativeDocRoleStructured NativeDocRole = "structured"
	// NativeDocRoleExtra marks extra passthrough docs not fully modeled by the SDK.
	NativeDocRoleExtra NativeDocRole = "extra"
)

const (
	// NativeDocYAML identifies YAML native docs.
	NativeDocYAML NativeDocFormat = "yaml"
	// NativeDocJSON identifies JSON native docs.
	NativeDocJSON NativeDocFormat = "json"
	// NativeDocTOML identifies TOML native docs.
	NativeDocTOML NativeDocFormat = "toml"
	// NativeDocMarkdown identifies Markdown native docs.
	NativeDocMarkdown NativeDocFormat = "md"
)

const (
	// ManagedArtifactStatic describes a static generated file.
	ManagedArtifactStatic ManagedArtifactKind = "static"
	// ManagedArtifactMirror describes a mirrored source-to-output tree.
	ManagedArtifactMirror ManagedArtifactKind = "mirror"
	// ManagedArtifactPortableMCP describes a portable MCP manifest artifact.
	ManagedArtifactPortableMCP ManagedArtifactKind = "portable_mcp"
	// ManagedArtifactPortableSkills describes generated portable skill output.
	ManagedArtifactPortableSkills ManagedArtifactKind = "portable_skills"
	// ManagedArtifactSelectedContext describes context files selected from source material.
	ManagedArtifactSelectedContext ManagedArtifactKind = "selected_context"
)

const (
	// ContextStrategyGeminiPrimaryRoot selects Gemini's primary context root strategy.
	ContextStrategyGeminiPrimaryRoot ContextStrategy = "gemini_primary_root"
)

const (
	// SurfaceTierStable marks a stable public surface.
	SurfaceTierStable SurfaceTier = "stable"
	// SurfaceTierBeta marks a beta public surface.
	SurfaceTierBeta SurfaceTier = "beta"
	// SurfaceTierPreview marks a preview-only surface.
	SurfaceTierPreview SurfaceTier = "preview"
	// SurfaceTierPassthroughOnly marks config surfaces that are preserved but not modeled as first-class authored APIs.
	SurfaceTierPassthroughOnly SurfaceTier = "passthrough_only"
	// SurfaceTierUnsupported marks unsupported surfaces that should not be relied on.
	SurfaceTierUnsupported SurfaceTier = "unsupported"
)
