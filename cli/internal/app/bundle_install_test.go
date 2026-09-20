package app

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginServiceBundleInstallInstallsPythonBundleIntoDestination(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "demo.tar.gz")
	metadata := exportMetadata{
		PluginName:         "demo",
		Platform:           "codex-runtime",
		Runtime:            "python",
		Manager:            "requirements.txt (pip)",
		BootstrapModel:     "repo-local .venv",
		RuntimeRequirement: "Python 3.10+ installed on the machine running the plugin",
		RuntimeInstallHint: "Go is the recommended path when you want users to avoid installing Python before running the plugin",
		Next: []string{
			"plugin-kit-ai doctor .",
			"plugin-kit-ai bootstrap .",
			"plugin-kit-ai validate . --platform codex-runtime --strict",
		},
		BundleFormat: "tar.gz",
		GeneratedBy:  "plugin-kit-ai export",
	}
	writeBundleArchive(t, bundle, metadata, map[string]bundleEntry{
		".plugin-kit-ai-export.json": {mode: 0o644, body: mustJSON(t, metadata)},
		"plugin/plugin.yaml":         {mode: 0o644, body: []byte("name: demo\n")},
		"plugin/launcher.yaml":       {mode: 0o644, body: []byte("runtime: python\nentrypoint: ./bin/demo\n")},
		"bin/demo":                   {mode: 0o755, body: []byte("#!/usr/bin/env bash\n")},
		"plugin/main.py":             {mode: 0o644, body: []byte("print('ok')\n")},
	})

	dest := filepath.Join(dir, "installed")
	var svc PluginService
	result, err := svc.BundleInstall(PluginBundleInstallOptions{
		Archive: bundle,
		Dest:    dest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "plugin/plugin.yaml")); err != nil {
		t.Fatalf("expected installed plugin.yaml: %v", err)
	}
	text := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Bundle: plugin=demo platform=codex-runtime runtime=python manager=requirements.txt (pip)",
		"Bundle source: " + bundle,
		"Installed path: " + dest,
		"Runtime requirement: Python 3.10+ installed on the machine running the plugin",
		"Runtime install hint: Go is the recommended path when you want users to avoid installing Python before running the plugin",
		"plugin-kit-ai doctor " + dest,
		"plugin-kit-ai bootstrap " + dest,
		"plugin-kit-ai validate " + dest + " --platform codex-runtime --strict",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("result missing %q:\n%s", want, text)
		}
	}
}

func TestPluginServiceBundleInstallRejectsUnsupportedRuntime(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "demo.tar.gz")
	metadata := exportMetadata{
		PluginName:     "demo",
		Platform:       "codex-runtime",
		Runtime:        "shell",
		BundleFormat:   "tar.gz",
		GeneratedBy:    "plugin-kit-ai export",
		Manager:        "none",
		BootstrapModel: "launcher plus executable shell scripts",
	}
	writeBundleArchive(t, bundle, metadata, map[string]bundleEntry{
		".plugin-kit-ai-export.json": {mode: 0o644, body: mustJSON(t, metadata)},
	})

	var svc PluginService
	_, err := svc.BundleInstall(PluginBundleInstallOptions{
		Archive: bundle,
		Dest:    filepath.Join(dir, "dest"),
	})
	if err == nil || !strings.Contains(err.Error(), "python/node") {
		t.Fatalf("error = %v", err)
	}
}

func TestPluginServiceBundleInstallRejectsRemoteURL(t *testing.T) {
	var svc PluginService
	_, err := svc.BundleInstall(PluginBundleInstallOptions{
		Archive: "https://example.com/demo.tar.gz",
		Dest:    filepath.Join(t.TempDir(), "dest"),
	})
	if err == nil || !strings.Contains(err.Error(), "remote URLs") {
		t.Fatalf("error = %v", err)
	}
}

func TestPluginServiceBundleInstallRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "demo.tar.gz")
	metadata := exportMetadata{
		PluginName:   "demo",
		Platform:     "codex-runtime",
		Runtime:      "python",
		BundleFormat: "tar.gz",
		GeneratedBy:  "plugin-kit-ai export",
	}
	writeBundleArchive(t, bundle, metadata, map[string]bundleEntry{
		".plugin-kit-ai-export.json": {mode: 0o644, body: mustJSON(t, metadata)},
		"../escape.txt":              {mode: 0o644, body: []byte("nope")},
	})

	var svc PluginService
	_, err := svc.BundleInstall(PluginBundleInstallOptions{
		Archive: bundle,
		Dest:    filepath.Join(dir, "dest"),
	})
	if err == nil || !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("error = %v", err)
	}
}

func TestPluginServiceBundleInstallDestinationRequiresForceForNonEmptyPath(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "demo.tar.gz")
	metadata := exportMetadata{
		PluginName:   "demo",
		Platform:     "codex-runtime",
		Runtime:      "node",
		BundleFormat: "tar.gz",
		GeneratedBy:  "plugin-kit-ai export",
		Manager:      "npm",
	}
	writeBundleArchive(t, bundle, metadata, map[string]bundleEntry{
		".plugin-kit-ai-export.json": {mode: 0o644, body: mustJSON(t, metadata)},
		"plugin/plugin.yaml":         {mode: 0o644, body: []byte("name: demo\n")},
	})
	dest := filepath.Join(dir, "dest")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	var svc PluginService
	if _, err := svc.BundleInstall(PluginBundleInstallOptions{Archive: bundle, Dest: dest}); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error = %v", err)
	}
	if _, err := svc.BundleInstall(PluginBundleInstallOptions{Archive: bundle, Dest: dest, Force: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "keep.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected overwrite to replace destination, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "plugin/plugin.yaml")); err != nil {
		t.Fatalf("expected new bundle content, err=%v", err)
	}
}

type bundleEntry struct {
	mode int64
	body []byte
}

func writeBundleArchive(t *testing.T, path string, metadata exportMetadata, entries map[string]bundleEntry) {
	t.Helper()
	if _, ok := entries[".plugin-kit-ai-export.json"]; !ok {
		entries[".plugin-kit-ai-export.json"] = bundleEntry{mode: 0o644, body: mustJSON(t, metadata)}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	for name, entry := range entries {
		hdr := &tar.Header{
			Name:     name,
			Mode:     entry.mode,
			Size:     int64(len(entry.body)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
