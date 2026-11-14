package opencode

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"gopkg.in/yaml.v3"
)

func (a Adapter) copyOwnedFiles(files []copyFile) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(files))
	for _, item := range files {
		body, err := os.ReadFile(item.Source)
		if err != nil {
			return nil, domain.NewError(domain.ErrMutationApply, "read OpenCode source asset", err)
		}
		if err := a.fs().WriteFileAtomic(context.Background(), item.Destination, body, 0o644); err != nil {
			return nil, domain.NewError(domain.ErrMutationApply, "write OpenCode projected asset", err)
		}
		out = append(out, item.Destination)
	}
	sort.Strings(out)
	return out, nil
}

func collectCopyFiles(sourceRoot, assetsRoot string) ([]copyFile, error) {
	type pair struct{ src, dst string }
	pairs := []pair{
		{authoredpath.Join(sourceRoot, "targets", "opencode", "commands"), filepath.Join(assetsRoot, "commands")},
		{authoredpath.Join(sourceRoot, "targets", "opencode", "agents"), filepath.Join(assetsRoot, "agents")},
		{authoredpath.Join(sourceRoot, "targets", "opencode", "themes"), filepath.Join(assetsRoot, "themes")},
		{authoredpath.Join(sourceRoot, "targets", "opencode", "tools"), filepath.Join(assetsRoot, "tools")},
		{authoredpath.Join(sourceRoot, "targets", "opencode", "plugins"), filepath.Join(assetsRoot, "plugins")},
		{authoredpath.Join(sourceRoot, "skills"), filepath.Join(assetsRoot, "skills")},
	}
	var out []copyFile
	for _, pair := range pairs {
		if _, err := os.Stat(pair.src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err := filepath.WalkDir(pair.src, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(pair.src, path)
			if err != nil {
				return err
			}
			out = append(out, copyFile{
				Source:      path,
				Destination: filepath.Join(pair.dst, rel),
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if pkg := authoredpath.Join(sourceRoot, "targets", "opencode", "package.json"); fileExists(pkg) {
		out = append(out, copyFile{Source: pkg, Destination: filepath.Join(assetsRoot, "package.json")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Destination < out[j].Destination })
	return out, nil
}

func readPlugins(path string) ([]pluginRef, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, domain.NewError(domain.ErrManifestLoad, "read OpenCode package metadata", err)
	}
	var meta packageMeta
	if err := yaml.Unmarshal(body, &meta); err != nil {
		return nil, domain.NewError(domain.ErrManifestLoad, "parse OpenCode package metadata", err)
	}
	var out []pluginRef
	for _, plugin := range meta.Plugins {
		plugin.Name = strings.TrimSpace(plugin.Name)
		if plugin.Name == "" {
			continue
		}
		out = append(out, plugin)
	}
	return out, nil
}
