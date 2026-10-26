//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const trustedProjectMode = "trusted-project-v1"
const trustedManifestLimit = 1 << 20
const trustedToolsLimit = 384 << 20

// This is a fixed test-tool handoff, never a general executable request.
type trustedInput struct {
	ToolsRoot           string `json:"tools_root"`
	ToolsManifestSHA256 string `json:"tools_manifest_sha256"`
	InstallerPath       string `json:"installer_path"`
	InstallerSHA256     string `json:"installer_sha256"`
	ProbePath           string `json:"probe_path"`
	ProbeSHA256         string `json:"probe_sha256"`
}

type trustedToolFile struct {
	Path       string `json:"path"`
	Bytes      int64  `json:"bytes"`
	SHA256     string `json:"sha256"`
	Executable bool   `json:"executable"`
}

type trustedToolManifest struct {
	SchemaVersion     int               `json:"schema_version"`
	NodeVersion       string            `json:"node_version"`
	TypeScriptVersion string            `json:"typescript_version"`
	PublisherCommit   string            `json:"publisher_commit"`
	Files             []trustedToolFile `json:"files"`
}

func trustedRelativeFile(name string) bool {
	if name == "" || len(name) > 512 || strings.ContainsAny(name, "\\\x00:") || strings.HasPrefix(name, "/") || filepath.ToSlash(filepath.Clean(name)) != name {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || part == ".bin" {
			return false
		}
	}
	switch name {
	case "node", "observer/package.json", "observer/extension.js", "observer/runner.js", "driver/package.json", "driver/driver.js":
		return true
	}
	return strings.HasPrefix(name, "driver/node_modules/playwright-core/") && !strings.Contains(strings.TrimPrefix(name, "driver/node_modules/playwright-core/"), "/node_modules/")
}

func decodeTrustedManifest(body []byte) (trustedToolManifest, error) {
	var manifest trustedToolManifest
	if len(body) > trustedManifestLimit {
		return manifest, fmt.Errorf("trusted tool manifest exceeds bound")
	}
	if _, err := uniqueJSON(body); err != nil {
		return manifest, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, err
	}
	if manifest.SchemaVersion != 1 || manifest.NodeVersion != "24.21.0" || manifest.TypeScriptVersion != "7.0.2" || manifest.PublisherCommit != publisherCommit || len(manifest.Files) < 7 || len(manifest.Files) > 4096 {
		return manifest, fmt.Errorf("trusted tool version/closure contract differs")
	}
	seen, total := map[string]bool{}, int64(0)
	for _, file := range manifest.Files {
		if !trustedRelativeFile(file.Path) || seen[file.Path] || !shaPattern.MatchString(file.SHA256) || file.Bytes < 0 || file.Bytes > 256<<20 || file.Executable != (file.Path == "node") {
			return manifest, fmt.Errorf("trusted tool member is duplicate, unsafe or unpinned")
		}
		seen[file.Path] = true
		total += file.Bytes
		if total > trustedToolsLimit {
			return manifest, fmt.Errorf("trusted tool aggregate exceeds bound")
		}
	}
	for _, name := range []string{"node", "observer/package.json", "observer/extension.js", "observer/runner.js", "driver/package.json", "driver/driver.js", "driver/node_modules/playwright-core/package.json"} {
		if !seen[name] {
			return manifest, fmt.Errorf("trusted tool runtime closure is incomplete: %s", name)
		}
	}
	return manifest, nil
}

func decodePinnedTrustedManifest(body []byte, expected string) (trustedToolManifest, error) {
	digest := sha256.Sum256(body)
	if !shaPattern.MatchString(expected) || hex.EncodeToString(digest[:]) != expected {
		return trustedToolManifest{}, fmt.Errorf("decoded trusted manifest bytes differ from supplied pin")
	}
	return decodeTrustedManifest(body)
}

func validateTrustedPackages(root string) error {
	for _, item := range []struct{ path, name string }{{"observer/package.json", "project-runtime-observer"}, {"driver/package.json", "project-runtime-driver"}} {
		body, err := readOrdinary(filepath.Join(root, item.path), 16<<10)
		if err != nil {
			return err
		}
		value, err := uniqueJSON(body)
		object, ok := value.(map[string]any)
		if err != nil || !ok || object["name"] != item.name || object["type"] != "commonjs" || object["private"] != true {
			return fmt.Errorf("trusted package identity/module mode differs: %s", item.path)
		}
		if item.name == "project-runtime-observer" && (object["publisher"] != "uap-research" || object["main"] != "./extension.js") {
			return fmt.Errorf("trusted observer extension identity differs")
		}
	}
	body, err := readOrdinary(filepath.Join(root, "driver/node_modules/playwright-core/package.json"), 128<<10)
	if err != nil {
		return err
	}
	value, err := uniqueJSON(body)
	object, ok := value.(map[string]any)
	if err != nil || !ok || object["name"] != "playwright-core" || object["version"] != "1.64.1" {
		return fmt.Errorf("trusted Playwright runtime package differs")
	}
	return nil
}

// Validate the entire supplied tree against the manifest before copying it.
// The destination receives only checked regular files with root-owned modes.
func copyTrustedTools(ctx context.Context, in trustedInput, target string, uid int) (trustedToolManifest, error) {
	var manifest trustedToolManifest
	if _, err := exactDirectory(in.ToolsRoot, uid); err != nil {
		return manifest, err
	}
	manifestPath := filepath.Join(in.ToolsRoot, "manifest.json")
	digest, err := hashFile(manifestPath, trustedManifestLimit)
	if err != nil || digest != in.ToolsManifestSHA256 || !shaPattern.MatchString(in.ToolsManifestSHA256) {
		return manifest, fmt.Errorf("trusted tool manifest identity differs")
	}
	body, err := readOrdinary(manifestPath, trustedManifestLimit)
	if err != nil {
		return manifest, err
	}
	manifest, err = decodePinnedTrustedManifest(body, in.ToolsManifestSHA256)
	if err != nil {
		return manifest, err
	}
	expected := map[string]trustedToolFile{}
	for _, entry := range manifest.Files {
		expected[entry.Path] = entry
	}
	root, err := os.OpenRoot(in.ToolsRoot)
	if err != nil {
		return manifest, err
	}
	defer func() { _ = root.Close() }()
	seen, entries := map[string]bool{}, 0
	err = fs.WalkDir(root.FS(), ".", func(relative string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > 8192 || strings.Count(relative, "/") > 32 {
			return fmt.Errorf("trusted tool tree exceeds bound")
		}
		path := filepath.Join(in.ToolsRoot, relative)
		info, err := anchoredInfo(path)
		if err != nil {
			return err
		}
		owner, err := statUID(info)
		if err != nil || int64(owner) != int64(uid) {
			return fmt.Errorf("trusted tool input owner differs")
		}
		if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			return os.Mkdir(filepath.Join(target, relative), 0755)
		}
		if relative == "manifest.json" {
			if !regular(info, trustedManifestLimit) {
				return fmt.Errorf("trusted manifest is not ordinary")
			}
			return nil
		}
		entry, ok := expected[filepath.ToSlash(relative)]
		if !ok || !regular(info, 256<<20) || info.Size() != entry.Bytes {
			return fmt.Errorf("unlisted, aliased or changed trusted tool file")
		}
		actual, err := copyOrdinary(ctx, path, filepath.Join(target, relative), uid, entry.Executable)
		if err != nil || actual != entry.SHA256 {
			return fmt.Errorf("trusted tool bytes differ: %s", relative)
		}
		seen[entry.Path] = true
		return nil
	})
	if err != nil {
		return manifest, err
	}
	if len(seen) != len(expected) {
		return manifest, fmt.Errorf("trusted tool manifest member missing")
	}
	if err := validateTrustedPackages(target); err != nil {
		return manifest, err
	}
	// Recheck the manifest after the complete input walk; never adopt replacement.
	if actual, err := hashFile(manifestPath, trustedManifestLimit); err != nil || actual != digest {
		return manifest, fmt.Errorf("trusted tool manifest changed during copy")
	}
	if err := os.WriteFile(filepath.Join(target, "manifest.json"), body, 0444); err != nil {
		return manifest, err
	}
	return manifest, nil
}
