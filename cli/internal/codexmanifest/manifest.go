package codexmanifest

import (
	"fmt"
	"path/filepath"
)

const (
	PluginDir      = ".codex-plugin"
	PluginFileName = "plugin.json"
	AppFileName    = ".app.json"
	MCPFileName    = ".mcp.json"
	SkillsRef      = "./skills/"
	MCPServersRef  = "./.mcp.json"
	AppsRef        = "./.app.json"
)

func PluginManifestPath() string {
	return filepath.ToSlash(filepath.Join(PluginDir, PluginFileName))
}

func AppManifestPath() string {
	return filepath.ToSlash(AppFileName)
}

func MCPManifestPath() string {
	return filepath.ToSlash(MCPFileName)
}

type Author struct {
	Name  string `yaml:"name,omitempty" json:"name,omitempty"`
	Email string `yaml:"email,omitempty" json:"email,omitempty"`
	URL   string `yaml:"url,omitempty" json:"url,omitempty"`
}

type PackageMeta struct {
	Author     *Author  `yaml:"author,omitempty" json:"author,omitempty"`
	Homepage   string   `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Repository string   `yaml:"repository,omitempty" json:"repository,omitempty"`
	License    string   `yaml:"license,omitempty" json:"license,omitempty"`
	Keywords   []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`
}

type ImportedPluginManifest struct {
	Name          string
	Version       string
	Description   string
	PackageMeta   PackageMeta
	SkillsPath    string
	MCPServersRef string
	AppsRef       string
	Interface     map[string]any
	Extra         map[string]any
}

type PluginDirLayoutError struct {
	Path string
}

func (e *PluginDirLayoutError) Error() string {
	return fmt.Sprintf("Codex plugin directory %s may only contain %s (unexpected %s)", PluginDir, PluginFileName, e.Path)
}

func AppManifestEnabled(doc map[string]any) bool {
	return len(doc) > 0
}
