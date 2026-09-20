package pluginmanifest

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/targetcontracts"
)

const (
	FileName         = pluginmodel.FileName
	LauncherFileName = pluginmodel.LauncherFileName
	APIVersionV1     = pluginmodel.APIVersionV1
)

type WarningKind = pluginmodel.WarningKind

const (
	WarningUnknownField  = pluginmodel.WarningUnknownField
	WarningIgnoredImport = pluginmodel.WarningIgnoredImport
	WarningFidelity      = pluginmodel.WarningFidelity
)

type Warning = pluginmodel.Warning
type Manifest = pluginmodel.Manifest
type Launcher = pluginmodel.Launcher
type PortableMCP = pluginmodel.PortableMCP
type PortableComponents = pluginmodel.PortableComponents
type NativeDocFormat = pluginmodel.NativeDocFormat

const (
	NativeDocFormatJSON = pluginmodel.NativeDocFormatJSON
	NativeDocFormatTOML = pluginmodel.NativeDocFormatTOML
)

type NativeExtraDoc = pluginmodel.NativeExtraDoc
type TargetState = pluginmodel.TargetState
type TargetComponents = pluginmodel.TargetState

type GeminiSetting struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	EnvVar      string `yaml:"env_var" json:"envVar"`
	Sensitive   bool   `yaml:"sensitive" json:"sensitive"`
}

type PackageGraph = pluginmodel.PackageGraph
type Artifact = pluginmodel.Artifact

type InspectTarget struct {
	Target              string                    `json:"target"`
	PlatformFamily      string                    `json:"platform_family"`
	TargetClass         string                    `json:"target_class"`
	LauncherRequirement string                    `json:"launcher_requirement"`
	TargetNoun          string                    `json:"target_noun,omitempty"`
	ProductionClass     string                    `json:"production_class"`
	RuntimeContract     string                    `json:"runtime_contract"`
	InstallModel        string                    `json:"install_model,omitempty"`
	DevModel            string                    `json:"dev_model,omitempty"`
	ActivationModel     string                    `json:"activation_model,omitempty"`
	NativeRoot          string                    `json:"native_root,omitempty"`
	PortableKinds       []string                  `json:"portable_kinds"`
	TargetNativeKinds   []string                  `json:"target_native_kinds"`
	NativeDocPaths      map[string]string         `json:"native_doc_paths,omitempty"`
	NativeSurfaces      []targetcontracts.Surface `json:"native_surfaces,omitempty"`
	NativeSurfaceTiers  map[string]string         `json:"native_surface_tiers,omitempty"`
	ManagedArtifacts    []string                  `json:"managed_artifacts"`
	UnsupportedKinds    []string                  `json:"unsupported_kinds,omitempty"`
}

type InspectLayout struct {
	AuthoredRoot      string              `json:"authored_root,omitempty"`
	AuthoredInputs    []string            `json:"authored_inputs"`
	BoundaryDocs      []string            `json:"boundary_docs,omitempty"`
	GeneratedGuide    string              `json:"generated_guide,omitempty"`
	GeneratedOutputs  []string            `json:"generated_outputs"`
	GeneratedByTarget map[string][]string `json:"generated_by_target,omitempty"`
}

type Inspection struct {
	Manifest    Manifest               `json:"manifest"`
	Launcher    *Launcher              `json:"launcher,omitempty"`
	Portable    PortableComponents     `json:"portable"`
	Publication publicationmodel.Model `json:"publication"`
	Targets     []InspectTarget        `json:"targets"`
	Layout      InspectLayout          `json:"layout"`
	SourceFiles []string               `json:"source_files"`
}

type RenderResult struct {
	Artifacts  []Artifact
	StalePaths []string
}

func Load(root string) (Manifest, error) {
	return loadManifest(root)
}

func LoadLauncher(root string) (Launcher, error) {
	return loadLauncher(root)
}

func LoadWithWarnings(root string) (Manifest, []Warning, error) {
	return loadManifestWithWarnings(root)
}

func LoadLauncherWithWarnings(root string) (Launcher, []Warning, error) {
	return loadLauncherWithWarnings(root)
}

func Parse(body []byte) (Manifest, error) {
	return parseManifest(body)
}

func ParseLauncher(body []byte) (Launcher, error) {
	return parseLauncher(body)
}

func Analyze(body []byte) (Manifest, []Warning, error) {
	return analyzeManifest(body)
}

func AnalyzeLauncher(body []byte) (Launcher, []Warning, error) {
	return analyzeLauncher(body)
}

func ValidateGeminiExtensionName(name string) error {
	return pluginmodel.ValidateGeminiExtensionName(name)
}

func Default(projectName, platform, runtime, description string, _ bool) Manifest {
	return defaultManifest(projectName, platform, runtime, description)
}

func DefaultLauncher(projectName, runtime string) Launcher {
	return defaultLauncher(projectName, runtime)
}

func Save(root string, manifest Manifest, force bool) error {
	return saveManifest(root, manifest, force)
}

func SaveLauncher(root string, launcher Launcher, force bool) error {
	return saveLauncher(root, launcher, force)
}

func Normalize(root string, force bool) ([]Warning, error) {
	return normalizePackage(root, force)
}

func Discover(root string) (PackageGraph, []Warning, error) {
	return discoverPackage(root)
}

func Inspect(root string, target string) (Inspection, []Warning, error) {
	return inspectPackage(root, target)
}

func Generate(root string, target string) (RenderResult, error) {
	return generatePackage(root, target)
}

func WriteArtifacts(root string, artifacts []Artifact) error {
	return writeArtifacts(root, artifacts)
}

func RemoveArtifacts(root string, relPaths []string) error {
	return removeArtifacts(root, relPaths)
}

func Drift(root string, target string) ([]string, error) {
	return driftPackage(root, target)
}

func Import(root string, from string, force bool, includeUserScope bool) (Manifest, []Warning, error) {
	return importPackage(root, from, force, includeUserScope)
}

func ImportFromSource(root string, sourceRef string, from string, force bool, includeUserScope bool) (Manifest, []Warning, error) {
	return ImportSource(root, sourceRef, from, force, includeUserScope)
}

func InspectSourceRef(sourceRef string, from string, target string, includeUserScope bool) (SourceInspection, []Warning, error) {
	return InspectSource(sourceRef, from, target, includeUserScope)
}

func ValidateClaudeHookEntrypoints(body []byte, entrypoint string) ([]string, error) {
	return validateClaudeHookEntrypoints(body, entrypoint)
}

func LoadNativeExtraDoc(root, rel string, format NativeDocFormat) (NativeExtraDoc, error) {
	return pluginmodel.LoadNativeExtraDoc(root, rel, format)
}

func ValidateNativeExtraDocConflicts(doc NativeExtraDoc, label string, managedPaths []string) error {
	return pluginmodel.ValidateNativeExtraDocConflicts(doc, label, managedPaths)
}

func DiscoveredTargetKinds(tc TargetComponents) []string {
	return discoveredTargetKinds(tc)
}
