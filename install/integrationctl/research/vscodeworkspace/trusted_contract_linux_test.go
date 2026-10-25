//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func trustedContractFixture(t *testing.T) (string, trustedToolManifest) {
	t.Helper()
	root := t.TempDir()
	bodies := map[string][]byte{
		"node":                  []byte("\x7fELFinert executable header; never run"),
		"observer/package.json": []byte(`{"name":"project-runtime-observer","publisher":"uap-research","private":true,"type":"commonjs","main":"./extension.js"}`),
		"observer/extension.js": []byte("exports.activate=()=>({});\n"),
		"observer/runner.js":    []byte("exports.run=async()=>{};\n"),
		"driver/package.json":   []byte(`{"name":"project-runtime-driver","private":true,"type":"commonjs"}`),
		"driver/driver.js":      []byte("throw Error('inert contract only');\n"),
		"driver/node_modules/playwright-core/package.json": []byte(`{"name":"playwright-core","version":"1.64.1"}`),
		"driver/node_modules/playwright-core/runtime.js":   []byte("inert runtime closure\n"),
	}
	manifest := trustedToolManifest{SchemaVersion: 1, NodeVersion: "24.21.0", TypeScriptVersion: "5.9.3", PublisherCommit: publisherCommit}
	for path, body := range bodies {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, body, 0600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		manifest.Files = append(manifest.Files, trustedToolFile{Path: path, Bytes: int64(len(body)), SHA256: hex.EncodeToString(digest[:]), Executable: path == "node"})
	}
	return root, manifest
}

func trustedContractWriteManifest(t *testing.T, root string, manifest trustedToolManifest) trustedInput {
	t.Helper()
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	return trustedInput{ToolsRoot: root, ToolsManifestSHA256: hex.EncodeToString(digest[:])}
}

func TestTrustedDecodedManifestMustMatchPinnedBytes(t *testing.T) {
	root, manifest := trustedContractFixture(t)
	input := trustedContractWriteManifest(t, root, manifest)
	changed := manifest
	changed.Files = append([]trustedToolFile(nil), manifest.Files...)
	changed.Files[0].SHA256 = strings.Repeat("0", 64)
	body, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	// Even if a caller has separately observed the expected path digest before
	// and after reading, the decoded body itself must be bound to that pin.
	if _, err := decodePinnedTrustedManifest(body, input.ToolsManifestSHA256); err == nil {
		t.Fatal("different valid manifest body accepted under original manifest pin")
	}
}

func TestTrustedToolContractRejectsUnlistedAliasedAndChangedInputs(t *testing.T) {
	for _, kind := range []string{"unlisted", "symlink", "changed", "manifest"} {
		t.Run(kind, func(t *testing.T) {
			root, manifest := trustedContractFixture(t)
			input := trustedContractWriteManifest(t, root, manifest)
			switch kind {
			case "unlisted":
				if err := os.WriteFile(filepath.Join(root, "ambient.js"), []byte("unlisted"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				path := filepath.Join(root, "driver/node_modules/playwright-core/runtime.js")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("package.json", path); err != nil {
					t.Fatal(err)
				}
			case "changed":
				if err := os.WriteFile(filepath.Join(root, "observer/runner.js"), []byte("changed bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			case "manifest":
				input.ToolsManifestSHA256 = strings.Repeat("0", 64)
			}
			_, err := copyTrustedTools(context.Background(), input, filepath.Join(t.TempDir(), "sealed"), os.Getuid())
			if err == nil {
				t.Fatal("unsafe trusted tool input was adopted")
			}
		})
	}
}

func TestTrustedToolContractCopiesPinnedCompleteRuntimeAndModuleManifests(t *testing.T) {
	root, manifest := trustedContractFixture(t)
	input := trustedContractWriteManifest(t, root, manifest)
	target := filepath.Join(t.TempDir(), "sealed")
	actual, err := copyTrustedTools(context.Background(), input, target, os.Getuid())
	if err != nil {
		t.Fatal(err)
	}
	if len(actual.Files) != len(manifest.Files) {
		t.Fatal("runtime closure was reduced")
	}
	for _, file := range actual.Files {
		path := filepath.Join(target, file.Path)
		digest, err := hashFile(path, 256<<20)
		if err != nil || digest != file.SHA256 {
			t.Fatalf("sealed bytes: %s %v", file.Path, err)
		}
		info, err := os.Lstat(path)
		want := os.FileMode(0444)
		if file.Path == "node" {
			want = 0555
		}
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("sealed mode: %s", file.Path)
		}
	}
	if err := validateTrustedPackages(target); err != nil {
		t.Fatal(err)
	}
}

func TestTrustedManifestRejectsModeDriftDuplicatePathsAndIncompleteClosure(t *testing.T) {
	_, original := trustedContractFixture(t)
	for _, kind := range []string{"version", "duplicate", "traversal", "bin-link", "missing-playwright", "executable-js", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			manifest := original
			manifest.Files = append([]trustedToolFile(nil), original.Files...)
			switch kind {
			case "version":
				manifest.TypeScriptVersion = "7.0.2"
			case "duplicate":
				manifest.Files = append(manifest.Files, manifest.Files[0])
			case "traversal":
				manifest.Files[0].Path = "driver/../node"
			case "bin-link":
				manifest.Files[0].Path = "driver/node_modules/.bin/playwright"
			case "missing-playwright":
				for i, file := range manifest.Files {
					if file.Path == "driver/node_modules/playwright-core/package.json" {
						manifest.Files = append(manifest.Files[:i], manifest.Files[i+1:]...)
						break
					}
				}
			case "executable-js":
				for i, file := range manifest.Files {
					if file.Path == "driver/driver.js" {
						manifest.Files[i].Executable = true
					}
				}
			}
			body, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "unknown" {
				body = append(body[:len(body)-1], []byte(`,"runtime":"ambient"}`)...)
			}
			if _, err := decodeTrustedManifest(body); err == nil {
				t.Fatal("unreviewed trusted runtime manifest accepted")
			}
		})
	}
}
