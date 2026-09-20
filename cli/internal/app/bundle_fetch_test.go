package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

type fakeBundleDownloader struct {
	bodies map[string][]byte
	errs   map[string]error
}

func (f fakeBundleDownloader) Download(_ context.Context, url string) ([]byte, string, error) {
	if err, ok := f.errs[url]; ok {
		return nil, "", err
	}
	body, ok := f.bodies[url]
	if !ok {
		return nil, "", fmt.Errorf("missing body for %s", url)
	}
	return body, "application/octet-stream", nil
}

type fakeBundleReleaseSource struct {
	byTag  *domain.Release
	latest *domain.Release
	bodies map[string][]byte
	errs   map[string]error
}

func (f fakeBundleReleaseSource) GetReleaseByTag(_ context.Context, owner, repo, tag string) (*domain.Release, error) {
	if owner == "" || repo == "" || tag == "" {
		return nil, errors.New("bad release ref")
	}
	if f.byTag == nil {
		return nil, errors.New("missing tagged release")
	}
	return f.byTag, nil
}

func (f fakeBundleReleaseSource) FindReleaseByTag(_ context.Context, owner, repo, tag string) (*domain.Release, error) {
	return f.GetReleaseByTag(context.Background(), owner, repo, tag)
}

func (f fakeBundleReleaseSource) GetLatestRelease(_ context.Context, owner, repo string) (*domain.Release, error) {
	if owner == "" || repo == "" {
		return nil, errors.New("bad latest ref")
	}
	if f.latest == nil {
		return nil, errors.New("missing latest release")
	}
	return f.latest, nil
}

func (f fakeBundleReleaseSource) DownloadAsset(_ context.Context, url string) ([]byte, string, error) {
	if err, ok := f.errs[url]; ok {
		return nil, "", err
	}
	body, ok := f.bodies[url]
	if !ok {
		return nil, "", fmt.Errorf("missing asset for %s", url)
	}
	return body, "application/octet-stream", nil
}

func TestPluginServiceBundleFetchURLInstallsPythonBundleWithExplicitChecksum(t *testing.T) {
	dir := t.TempDir()
	bundle := mustBundleArchiveBytes(t, exportMetadata{
		PluginName:     "demo",
		Platform:       "codex-runtime",
		Runtime:        "python",
		Manager:        "requirements.txt (pip)",
		BootstrapModel: "repo-local .venv",
		Next: []string{
			"plugin-kit-ai doctor .",
			"plugin-kit-ai bootstrap .",
			"plugin-kit-ai validate . --platform codex-runtime --strict",
		},
		BundleFormat: "tar.gz",
		GeneratedBy:  "plugin-kit-ai export",
	}, map[string]bundleEntry{
		"plugin/plugin.yaml":   {mode: 0o644, body: []byte("name: demo\n")},
		"plugin/launcher.yaml": {mode: 0o644, body: []byte("runtime: python\nentrypoint: ./bin/demo\n")},
		"bin/demo":             {mode: 0o755, body: []byte("#!/usr/bin/env bash\n")},
		"plugin/main.py":       {mode: 0o644, body: []byte("print('ok')\n")},
	})
	sum := sha256.Sum256(bundle)
	result, err := bundleFetch(context.Background(), PluginBundleFetchOptions{
		URL:    "https://example.com/demo_bundle.tar.gz",
		Dest:   filepath.Join(dir, "installed"),
		SHA256: hex.EncodeToString(sum[:]),
	}, bundleFetchDeps{
		URLDownloader: fakeBundleDownloader{
			bodies: map[string][]byte{
				"https://example.com/demo_bundle.tar.gz": bundle,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "installed", "plugin/plugin.yaml")); err != nil {
		t.Fatalf("expected installed plugin.yaml: %v", err)
	}
	text := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Bundle source: https://example.com/demo_bundle.tar.gz",
		"Checksum source: flag --sha256",
		"plugin-kit-ai doctor " + filepath.Join(dir, "installed"),
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("result missing %q:\n%s", want, text)
		}
	}
}

func TestPluginServiceBundleFetchURLUsesSidecarChecksum(t *testing.T) {
	bundle := mustBundleArchiveBytes(t, exportMetadata{
		PluginName:   "demo",
		Platform:     "codex-runtime",
		Runtime:      "python",
		BundleFormat: "tar.gz",
		GeneratedBy:  "plugin-kit-ai export",
	}, map[string]bundleEntry{
		"plugin/plugin.yaml": {mode: 0o644, body: []byte("name: demo\n")},
	})
	sum := sha256.Sum256(bundle)
	sidecar := []byte(hex.EncodeToString(sum[:]) + "  demo_bundle.tar.gz\n")
	result, err := bundleFetch(context.Background(), PluginBundleFetchOptions{
		URL:  "https://example.com/demo_bundle.tar.gz",
		Dest: filepath.Join(t.TempDir(), "installed"),
	}, bundleFetchDeps{
		URLDownloader: fakeBundleDownloader{
			bodies: map[string][]byte{
				"https://example.com/demo_bundle.tar.gz":        bundle,
				"https://example.com/demo_bundle.tar.gz.sha256": sidecar,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "Checksum source: https://example.com/demo_bundle.tar.gz.sha256") {
		t.Fatalf("result missing sidecar checksum source:\n%s", strings.Join(result.Lines, "\n"))
	}
}

func TestPluginServiceBundleFetchURLRejectsHTTP(t *testing.T) {
	_, err := bundleFetch(context.Background(), PluginBundleFetchOptions{
		URL:  "http://example.com/demo_bundle.tar.gz",
		Dest: filepath.Join(t.TempDir(), "installed"),
	}, bundleFetchDeps{})
	if err == nil || !strings.Contains(err.Error(), "https://") {
		t.Fatalf("error = %v", err)
	}
}

func TestPluginServiceBundleFetchURLFailsChecksumMismatch(t *testing.T) {
	bundle := mustBundleArchiveBytes(t, exportMetadata{
		PluginName:   "demo",
		Platform:     "codex-runtime",
		Runtime:      "python",
		BundleFormat: "tar.gz",
		GeneratedBy:  "plugin-kit-ai export",
	}, map[string]bundleEntry{
		"plugin/plugin.yaml": {mode: 0o644, body: []byte("name: demo\n")},
	})
	_, err := bundleFetch(context.Background(), PluginBundleFetchOptions{
		URL:    "https://example.com/demo_bundle.tar.gz",
		Dest:   filepath.Join(t.TempDir(), "installed"),
		SHA256: strings.Repeat("a", 64),
	}, bundleFetchDeps{
		URLDownloader: fakeBundleDownloader{
			bodies: map[string][]byte{
				"https://example.com/demo_bundle.tar.gz": bundle,
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "checksum verification failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestPluginServiceBundleFetchGitHubInstallsNodeBundleFromChecksumsTxt(t *testing.T) {
	dir := t.TempDir()
	bundle := mustBundleArchiveBytes(t, exportMetadata{
		PluginName:     "demo",
		Platform:       "claude",
		Runtime:        "node",
		Manager:        "npm",
		BootstrapModel: "npm install",
		BundleFormat:   "tar.gz",
		GeneratedBy:    "plugin-kit-ai export",
	}, map[string]bundleEntry{
		"plugin/plugin.yaml":         {mode: 0o644, body: []byte("name: demo\n")},
		"plugin/launcher.yaml":       {mode: 0o644, body: []byte("runtime: node\nentrypoint: ./bin/demo\n")},
		"package.json":               {mode: 0o644, body: []byte(`{"name":"demo","scripts":{"build":"tsc"}}`)},
		"dist/main.js":               {mode: 0o644, body: []byte("console.log('ok')\n")},
		"bin/demo":                   {mode: 0o755, body: []byte("#!/usr/bin/env bash\n")},
		".claude-plugin/plugin.json": {mode: 0o644, body: []byte("{}\n")},
	})
	sum := sha256.Sum256(bundle)
	release := &domain.Release{
		TagName: "v1.2.3",
		Assets: []domain.Asset{
			{Name: "checksums.txt", BrowserDownloadURL: "https://api.example/c"},
			{Name: "demo_claude_node_bundle.tar.gz", BrowserDownloadURL: "https://api.example/a"},
		},
	}
	result, err := bundleFetch(context.Background(), PluginBundleFetchOptions{
		Ref:      "demo/demo",
		Tag:      "v1.2.3",
		Dest:     filepath.Join(dir, "installed"),
		Platform: "claude",
		Runtime:  "node",
	}, bundleFetchDeps{
		GitHub: fakeBundleReleaseSource{
			byTag: release,
			bodies: map[string][]byte{
				"https://api.example/a": bundle,
				"https://api.example/c": []byte(hex.EncodeToString(sum[:]) + "  demo_claude_node_bundle.tar.gz\n"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "installed", "dist", "main.js")); err != nil {
		t.Fatalf("expected installed dist/main.js: %v", err)
	}
	text := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Bundle source: github release demo/demo@v1.2.3 (tag) asset=demo_claude_node_bundle.tar.gz",
		"Checksum source: release asset checksums.txt",
		"runtime=node",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("result missing %q:\n%s", want, text)
		}
	}
}

func TestPluginServiceBundleFetchGitHubFallsBackToSidecarChecksum(t *testing.T) {
	bundle := mustBundleArchiveBytes(t, exportMetadata{
		PluginName:   "demo",
		Platform:     "codex-runtime",
		Runtime:      "python",
		BundleFormat: "tar.gz",
		GeneratedBy:  "plugin-kit-ai export",
	}, map[string]bundleEntry{
		"plugin/plugin.yaml": {mode: 0o644, body: []byte("name: demo\n")},
	})
	sum := sha256.Sum256(bundle)
	release := &domain.Release{
		Assets: []domain.Asset{
			{Name: "demo_codex-runtime_python_bundle.tar.gz", BrowserDownloadURL: "https://api.example/a"},
			{Name: "demo_codex-runtime_python_bundle.tar.gz.sha256", BrowserDownloadURL: "https://api.example/s"},
		},
	}
	result, err := bundleFetch(context.Background(), PluginBundleFetchOptions{
		Ref:  "demo/demo",
		Tag:  "v1.0.0",
		Dest: filepath.Join(t.TempDir(), "installed"),
	}, bundleFetchDeps{
		GitHub: fakeBundleReleaseSource{
			byTag: release,
			bodies: map[string][]byte{
				"https://api.example/a": bundle,
				"https://api.example/s": []byte(hex.EncodeToString(sum[:]) + "\n"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "Checksum source: release asset demo_codex-runtime_python_bundle.tar.gz.sha256") {
		t.Fatalf("result missing checksum sidecar source:\n%s", strings.Join(result.Lines, "\n"))
	}
}

func TestPluginServiceBundleFetchGitHubUsesLatestRelease(t *testing.T) {
	dir := t.TempDir()
	bundle := mustBundleArchiveBytes(t, exportMetadata{
		PluginName:     "demo",
		Platform:       "claude",
		Runtime:        "node",
		Manager:        "npm",
		BootstrapModel: "npm install",
		BundleFormat:   "tar.gz",
		GeneratedBy:    "plugin-kit-ai export",
	}, map[string]bundleEntry{
		"plugin/plugin.yaml":         {mode: 0o644, body: []byte("name: demo\n")},
		"plugin/launcher.yaml":       {mode: 0o644, body: []byte("runtime: node\nentrypoint: ./bin/demo\n")},
		"package.json":               {mode: 0o644, body: []byte(`{"name":"demo"}`)},
		"dist/main.js":               {mode: 0o644, body: []byte("console.log('ok')\n")},
		"bin/demo":                   {mode: 0o755, body: []byte("#!/usr/bin/env bash\n")},
		".claude-plugin/plugin.json": {mode: 0o644, body: []byte("{}\n")},
	})
	sum := sha256.Sum256(bundle)
	release := &domain.Release{
		TagName: "v9.0.0",
		Assets: []domain.Asset{
			{Name: "checksums.txt", BrowserDownloadURL: "https://api.example/c"},
			{Name: "demo_claude_node_bundle.tar.gz", BrowserDownloadURL: "https://api.example/a"},
		},
	}
	result, err := bundleFetch(context.Background(), PluginBundleFetchOptions{
		Ref:      "demo/demo",
		Latest:   true,
		Dest:     filepath.Join(dir, "installed"),
		Platform: "claude",
		Runtime:  "node",
	}, bundleFetchDeps{
		GitHub: fakeBundleReleaseSource{
			latest: release,
			bodies: map[string][]byte{
				"https://api.example/a": bundle,
				"https://api.example/c": []byte(hex.EncodeToString(sum[:]) + "  demo_claude_node_bundle.tar.gz\n"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Bundle source: github release demo/demo@v9.0.0 (latest) asset=demo_claude_node_bundle.tar.gz",
		"Checksum source: release asset checksums.txt",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("result missing %q:\n%s", want, text)
		}
	}
}

func TestSelectBundleReleaseAssetRejectsAmbiguous(t *testing.T) {
	_, err := selectBundleReleaseAsset(&domain.Release{
		Assets: []domain.Asset{
			{Name: "a_codex-runtime_python_bundle.tar.gz"},
			{Name: "b_claude_node_bundle.tar.gz"},
		},
	}, "", "", "")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("error = %v", err)
	}
}

func TestSelectBundleReleaseAssetUsesPlatformRuntime(t *testing.T) {
	asset, err := selectBundleReleaseAsset(&domain.Release{
		Assets: []domain.Asset{
			{Name: "a_codex-runtime_python_bundle.tar.gz"},
			{Name: "b_claude_node_bundle.tar.gz"},
		},
	}, "", "claude", "node")
	if err != nil {
		t.Fatal(err)
	}
	if asset.Name != "b_claude_node_bundle.tar.gz" {
		t.Fatalf("asset = %q", asset.Name)
	}
}

func TestSelectBundleReleaseAssetUsesExactAssetName(t *testing.T) {
	asset, err := selectBundleReleaseAsset(&domain.Release{
		Assets: []domain.Asset{
			{Name: "a_codex-runtime_python_bundle.tar.gz"},
			{Name: "b_claude_node_bundle.tar.gz"},
		},
	}, "a_codex-runtime_python_bundle.tar.gz", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if asset.Name != "a_codex-runtime_python_bundle.tar.gz" {
		t.Fatalf("asset = %q", asset.Name)
	}
}

func TestPluginServiceBundleFetchGitHubRejectsMetadataMismatch(t *testing.T) {
	bundle := mustBundleArchiveBytes(t, exportMetadata{
		PluginName:   "demo",
		Platform:     "codex-runtime",
		Runtime:      "python",
		BundleFormat: "tar.gz",
		GeneratedBy:  "plugin-kit-ai export",
	}, map[string]bundleEntry{
		"plugin/plugin.yaml": {mode: 0o644, body: []byte("name: demo\n")},
	})
	sum := sha256.Sum256(bundle)
	release := &domain.Release{
		TagName: "v1.0.0",
		Assets: []domain.Asset{
			{Name: "demo_codex-runtime_python_bundle.tar.gz", BrowserDownloadURL: "https://api.example/a"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://api.example/c"},
		},
	}
	_, err := bundleFetch(context.Background(), PluginBundleFetchOptions{
		Ref:       "demo/demo",
		Tag:       "v1.0.0",
		Dest:      filepath.Join(t.TempDir(), "installed"),
		AssetName: "demo_codex-runtime_python_bundle.tar.gz",
		Platform:  "claude",
		Runtime:   "node",
	}, bundleFetchDeps{
		GitHub: fakeBundleReleaseSource{
			byTag: release,
			bodies: map[string][]byte{
				"https://api.example/a": bundle,
				"https://api.example/c": []byte(hex.EncodeToString(sum[:]) + "  demo_codex-runtime_python_bundle.tar.gz\n"),
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `does not match requested platform "claude"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestParseBundleChecksumSelectsNamedEntry(t *testing.T) {
	body := []byte(strings.Join([]string{
		strings.Repeat("a", 64) + "  first_bundle.tar.gz",
		strings.Repeat("b", 64) + "  wanted_bundle.tar.gz",
	}, "\n"))

	sum, err := parseBundleChecksum(body, "wanted_bundle.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(sum); got != strings.Repeat("b", 64) {
		t.Fatalf("checksum = %q", got)
	}
}

func TestValidateBundleFetchModeRequiresPlatformAndRuntimeTogether(t *testing.T) {
	err := validateBundleFetchMode(PluginBundleFetchOptions{
		Ref:      "demo/demo",
		Tag:      "v1.0.0",
		Dest:     "/tmp/demo",
		Platform: "claude",
	})
	if err == nil || !strings.Contains(err.Error(), "--platform and --runtime together") {
		t.Fatalf("error = %v", err)
	}
}

func mustBundleArchiveBytes(t *testing.T, metadata exportMetadata, entries map[string]bundleEntry) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "demo.tar.gz")
	writeBundleArchive(t, path, metadata, entries)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
