package pluginmodel

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

const (
	FileName            = "plugin.yaml"
	LauncherFileName    = "launcher.yaml"
	SourceDirName       = platformmeta.CanonicalAuthoredRoot
	LegacySourceDirName = "src"
	APIVersionV1        = "v1"
)

type WarningKind string

const (
	WarningUnknownField  WarningKind = "unknown_field"
	WarningIgnoredImport WarningKind = "ignored_import"
	WarningFidelity      WarningKind = "fidelity"
)

type Warning struct {
	Kind    WarningKind
	Path    string
	Message string
}

type Author struct {
	Name  string `yaml:"name,omitempty" json:"name,omitempty"`
	Email string `yaml:"email,omitempty" json:"email,omitempty"`
	URL   string `yaml:"url,omitempty" json:"url,omitempty"`
}

type Manifest struct {
	APIVersion  string   `yaml:"api_version,omitempty" json:"api_version,omitempty"`
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version" json:"version"`
	Description string   `yaml:"description" json:"description"`
	Author      *Author  `yaml:"author,omitempty" json:"author,omitempty"`
	Homepage    string   `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Repository  string   `yaml:"repository,omitempty" json:"repository,omitempty"`
	License     string   `yaml:"license,omitempty" json:"license,omitempty"`
	Keywords    []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`
	Targets     []string `yaml:"targets" json:"targets"`
}

type Launcher struct {
	Runtime    string `yaml:"runtime" json:"runtime"`
	Entrypoint string `yaml:"entrypoint" json:"entrypoint"`
}

type PortableMCP struct {
	Path    string           `json:"path"`
	Servers map[string]any   `json:"servers"`
	File    *PortableMCPFile `json:"file,omitempty"`
}

type PortableComponents struct {
	Items map[string][]string `json:"items"`
	MCP   *PortableMCP        `json:"mcp,omitempty"`
}

type NativeDocFormat string

const (
	NativeDocFormatJSON NativeDocFormat = "json"
	NativeDocFormatTOML NativeDocFormat = "toml"
)

type NativeExtraDoc struct {
	Path   string
	Format NativeDocFormat
	Raw    []byte
	Fields map[string]any
}

type TargetState struct {
	Target     string
	Docs       map[string]string
	Components map[string][]string
}

type PackageGraph struct {
	Manifest    Manifest
	Launcher    *Launcher
	Portable    PortableComponents
	Targets     map[string]TargetState
	SourceFiles []string
}

type Artifact struct {
	RelPath string
	Content []byte
}

func NewPortableComponents() PortableComponents {
	return PortableComponents{Items: map[string][]string{}}
}

func (p *PortableComponents) Add(kind string, paths ...string) {
	if len(paths) == 0 {
		return
	}
	if p.Items == nil {
		p.Items = map[string][]string{}
	}
	p.Items[kind] = append(p.Items[kind], paths...)
}

func (p PortableComponents) Paths(kind string) []string {
	return append([]string(nil), p.Items[kind]...)
}

func (p PortableComponents) Kinds() []string {
	out := make([]string, 0, len(p.Items))
	for kind, paths := range p.Items {
		if len(paths) == 0 {
			continue
		}
		out = append(out, kind)
	}
	slices.Sort(out)
	return out
}

func NewTargetState(target string) TargetState {
	return TargetState{
		Target:     target,
		Docs:       map[string]string{},
		Components: map[string][]string{},
	}
}

func (tc *TargetState) SetDoc(kind, path string) {
	if tc.Docs == nil {
		tc.Docs = map[string]string{}
	}
	tc.Docs[kind] = filepath.ToSlash(path)
}

func (tc TargetState) DocPath(kind string) string {
	return strings.TrimSpace(tc.Docs[kind])
}

func (tc *TargetState) AddComponent(kind string, paths ...string) {
	if tc.Components == nil {
		tc.Components = map[string][]string{}
	}
	tc.Components[kind] = append(tc.Components[kind], paths...)
}

func (tc TargetState) ComponentPaths(kind string) []string {
	return append([]string(nil), tc.Components[kind]...)
}

func IsAuthoredRoot(path string) bool {
	path = filepath.ToSlash(strings.TrimSpace(path))
	return path == SourceDirName
}

func CanonicalAuthoredPath(path string) string {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." || path == "" {
		return ""
	}
	if IsAuthoredRoot(path) {
		return path
	}
	if path == LegacySourceDirName {
		return SourceDirName
	}
	if strings.HasPrefix(path, LegacySourceDirName+"/") {
		return filepath.ToSlash(filepath.Join(SourceDirName, strings.TrimPrefix(path, LegacySourceDirName+"/")))
	}
	if strings.HasPrefix(path, SourceDirName+"/") {
		return path
	}
	return filepath.ToSlash(filepath.Join(SourceDirName, path))
}

func RebaseAuthoredPath(path string, authoredRoot string) string {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	authoredRoot = filepath.ToSlash(filepath.Clean(strings.TrimSpace(authoredRoot)))
	if path == "." || path == "" {
		return ""
	}
	if authoredRoot == "." || authoredRoot == "" {
		authoredRoot = SourceDirName
	}
	switch {
	case path == SourceDirName:
		return authoredRoot
	case strings.HasPrefix(path, SourceDirName+"/"):
		return filepath.ToSlash(filepath.Join(authoredRoot, strings.TrimPrefix(path, SourceDirName+"/")))
	default:
		return path
	}
}
