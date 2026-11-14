package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

type fakeBundlePublisher struct {
	findRelease   *domain.Release
	findErr       error
	createRelease *domain.Release
	createErr     error
	createdDraft  *bool
	updateDrafts  []bool
	updateIDs     []int64
	updateRelease *domain.Release
	updateErr     error
	uploads       []fakeUploadedAsset
	deletes       []int64
}

type fakeUploadedAsset struct {
	UploadURL string
	Name      string
	Body      string
}

func (f *fakeBundlePublisher) FindReleaseByTag(_ context.Context, owner, repo, tag string) (*domain.Release, error) {
	if owner == "" || repo == "" || tag == "" {
		return nil, domain.NewError(domain.ExitRelease, "bad ref")
	}
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.findRelease == nil {
		return nil, domain.NewError(domain.ExitRelease, "missing release")
	}
	return f.findRelease, nil
}

func (f *fakeBundlePublisher) CreateRelease(_ context.Context, owner, repo, tag string, draft bool) (*domain.Release, error) {
	if owner == "" || repo == "" || tag == "" {
		return nil, domain.NewError(domain.ExitRelease, "bad ref")
	}
	f.createdDraft = new(bool)
	*f.createdDraft = draft
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createRelease == nil {
		return nil, domain.NewError(domain.ExitRelease, "missing created release")
	}
	return f.createRelease, nil
}

func (f *fakeBundlePublisher) UpdateReleaseDraftState(_ context.Context, owner, repo string, releaseID int64, draft bool) (*domain.Release, error) {
	if owner == "" || repo == "" || releaseID == 0 {
		return nil, domain.NewError(domain.ExitRelease, "bad update ref")
	}
	f.updateIDs = append(f.updateIDs, releaseID)
	f.updateDrafts = append(f.updateDrafts, draft)
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updateRelease == nil {
		return nil, domain.NewError(domain.ExitRelease, "missing updated release")
	}
	return f.updateRelease, nil
}

func (f *fakeBundlePublisher) UploadReleaseAsset(_ context.Context, uploadURL, name string, body []byte, _ string) (*domain.Asset, error) {
	f.uploads = append(f.uploads, fakeUploadedAsset{UploadURL: uploadURL, Name: name, Body: string(body)})
	return &domain.Asset{ID: int64(len(f.uploads)), Name: name}, nil
}

func (f *fakeBundlePublisher) DeleteReleaseAsset(_ context.Context, _ string, _ string, assetID int64) error {
	f.deletes = append(f.deletes, assetID)
	return nil
}

func fakeBundleExportFunc(t *testing.T, root string, metadata exportMetadata, extraFiles ...string) func(PluginExportOptions) (PluginExportResult, error) {
	t.Helper()
	baseFiles := []string{"plugin/plugin.yaml", "plugin/launcher.yaml"}
	baseFiles = append(baseFiles, extraFiles...)
	return func(opts PluginExportOptions) (PluginExportResult, error) {
		if opts.Root != root {
			t.Fatalf("export root = %q want %q", opts.Root, root)
		}
		if opts.Platform != metadata.Platform {
			t.Fatalf("export platform = %q want %q", opts.Platform, metadata.Platform)
		}
		if err := writeExportArchive(root, opts.Output, baseFiles, metadata); err != nil {
			return PluginExportResult{}, err
		}
		return PluginExportResult{Lines: []string{"Exported bundle: " + opts.Output}}, nil
	}
}

func TestPluginServiceBundlePublishCreatesPublishedReleaseByDefault(t *testing.T) {
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: python\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "main.py"), "print('ok')\n")

	publisher := &fakeBundlePublisher{
		findErr: domain.NewError(domain.ExitRelease, "release tag \"v1\" not found"),
		createRelease: &domain.Release{
			ID:        1,
			TagName:   "v1",
			Draft:     false,
			UploadURL: "https://uploads.example/releases/1/assets{?name,label}",
		},
	}
	result, err := bundlePublish(context.Background(), PluginBundlePublishOptions{
		Root:     dir,
		Platform: "codex-runtime",
		Repo:     "o/r",
		Tag:      "v1",
	}, bundlePublishDeps{
		GitHub: publisher,
		Export: fakeBundleExportFunc(t, dir, exportMetadata{
			PluginName:     "demo",
			Platform:       "codex-runtime",
			Runtime:        "python",
			Manager:        "requirements",
			BootstrapModel: "repo-local .venv",
			Next:           []string{"plugin-kit-ai doctor ."},
			BundleFormat:   "tar.gz",
			GeneratedBy:    "plugin-kit-ai export",
		}, filepath.ToSlash(filepath.Join("plugin", "main.py"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(publisher.uploads) != 2 {
		t.Fatalf("uploads = %#v", publisher.uploads)
	}
	if publisher.createdDraft == nil || *publisher.createdDraft {
		t.Fatalf("createdDraft = %#v", publisher.createdDraft)
	}
	if publisher.uploads[0].Name != "demo_codex-runtime_python_bundle.tar.gz" {
		t.Fatalf("bundle upload = %#v", publisher.uploads[0])
	}
	if publisher.uploads[1].Name != "demo_codex-runtime_python_bundle.tar.gz.sha256" {
		t.Fatalf("sidecar upload = %#v", publisher.uploads[1])
	}
	if !strings.Contains(publisher.uploads[1].Body, "demo_codex-runtime_python_bundle.tar.gz") {
		t.Fatalf("sidecar body = %q", publisher.uploads[1].Body)
	}
	text := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Release state: created published release",
		"plugin-kit-ai bundle fetch o/r --tag v1 --platform codex-runtime --runtime python --dest <path>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("result missing %q:\n%s", want, text)
		}
	}
}

func TestPluginServiceBundlePublishCreatesDraftReleaseWhenRequested(t *testing.T) {
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: python\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "main.py"), "print('ok')\n")

	publisher := &fakeBundlePublisher{
		findErr: domain.NewError(domain.ExitRelease, "release tag \"v1\" not found"),
		createRelease: &domain.Release{
			ID:        1,
			TagName:   "v1",
			Draft:     true,
			UploadURL: "https://uploads.example/releases/1/assets{?name,label}",
		},
	}
	result, err := bundlePublish(context.Background(), PluginBundlePublishOptions{
		Root:     dir,
		Platform: "codex-runtime",
		Repo:     "o/r",
		Tag:      "v1",
		Draft:    true,
	}, bundlePublishDeps{
		GitHub: publisher,
		Export: fakeBundleExportFunc(t, dir, exportMetadata{
			PluginName:     "demo",
			Platform:       "codex-runtime",
			Runtime:        "python",
			Manager:        "requirements",
			BootstrapModel: "repo-local .venv",
			Next:           []string{"plugin-kit-ai doctor ."},
			BundleFormat:   "tar.gz",
			GeneratedBy:    "plugin-kit-ai export",
		}, filepath.ToSlash(filepath.Join("plugin", "main.py"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if publisher.createdDraft == nil || !*publisher.createdDraft {
		t.Fatalf("createdDraft = %#v", publisher.createdDraft)
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "Release state: created draft release") {
		t.Fatalf("result = %v", result.Lines)
	}
}

func TestPluginServiceBundlePublishPromotesExistingDraftReleaseToPublished(t *testing.T) {
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: node\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, "package.json", `{"name":"demo","scripts":{"build":"tsc -p tsconfig.json"}}`)
	writeBootstrapProjectFile(t, dir, "tsconfig.json", `{"compilerOptions":{"outDir":"dist"}}`)
	writeBootstrapProjectFile(t, dir, filepath.Join("dist", "main.js"), "console.log('ok')\n")

	publisher := &fakeBundlePublisher{
		findRelease: &domain.Release{
			ID:        2,
			TagName:   "v2",
			Draft:     true,
			UploadURL: "https://uploads.example/releases/2/assets{?name,label}",
		},
		updateRelease: &domain.Release{
			ID:        2,
			TagName:   "v2",
			Draft:     false,
			UploadURL: "https://uploads.example/releases/2/assets{?name,label}",
		},
	}
	result, err := bundlePublish(context.Background(), PluginBundlePublishOptions{
		Root:     dir,
		Platform: "codex-runtime",
		Repo:     "o/r",
		Tag:      "v2",
	}, bundlePublishDeps{
		GitHub: publisher,
		Export: fakeBundleExportFunc(t, dir, exportMetadata{
			PluginName:     "demo",
			Platform:       "codex-runtime",
			Runtime:        "node",
			Manager:        "npm",
			BootstrapModel: "package-manager install + build",
			Next:           []string{"plugin-kit-ai doctor ."},
			BundleFormat:   "tar.gz",
			GeneratedBy:    "plugin-kit-ai export",
		}, "package.json", "tsconfig.json", filepath.ToSlash(filepath.Join("dist", "main.js"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(publisher.updateIDs) != 1 || publisher.updateIDs[0] != 2 {
		t.Fatalf("updateIDs = %#v", publisher.updateIDs)
	}
	if len(publisher.updateDrafts) != 1 || publisher.updateDrafts[0] {
		t.Fatalf("updateDrafts = %#v", publisher.updateDrafts)
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "Release state: promoted draft release to published") {
		t.Fatalf("result = %v", result.Lines)
	}
}

func TestPluginServiceBundlePublishReusesExistingDraftReleaseWhenRequested(t *testing.T) {
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: node\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, "package.json", `{"name":"demo","scripts":{"build":"tsc -p tsconfig.json"}}`)
	writeBootstrapProjectFile(t, dir, "tsconfig.json", `{"compilerOptions":{"outDir":"dist"}}`)
	writeBootstrapProjectFile(t, dir, filepath.Join("dist", "main.js"), "console.log('ok')\n")

	publisher := &fakeBundlePublisher{
		findRelease: &domain.Release{
			ID:        2,
			TagName:   "v2",
			Draft:     true,
			UploadURL: "https://uploads.example/releases/2/assets{?name,label}",
		},
	}
	result, err := bundlePublish(context.Background(), PluginBundlePublishOptions{
		Root:     dir,
		Platform: "codex-runtime",
		Repo:     "o/r",
		Tag:      "v2",
		Draft:    true,
	}, bundlePublishDeps{
		GitHub: publisher,
		Export: fakeBundleExportFunc(t, dir, exportMetadata{
			PluginName:     "demo",
			Platform:       "codex-runtime",
			Runtime:        "node",
			Manager:        "npm",
			BootstrapModel: "package-manager install + build",
			Next:           []string{"plugin-kit-ai doctor ."},
			BundleFormat:   "tar.gz",
			GeneratedBy:    "plugin-kit-ai export",
		}, "package.json", "tsconfig.json", filepath.ToSlash(filepath.Join("dist", "main.js"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(publisher.updateIDs) != 0 {
		t.Fatalf("unexpected draft promotion: %#v", publisher.updateIDs)
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "Release state: reused existing draft release") {
		t.Fatalf("result = %v", result.Lines)
	}
}

func TestPluginServiceBundlePublishReusesExistingPublishedReleaseWithForce(t *testing.T) {
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: node\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, "package.json", `{"name":"demo","scripts":{"build":"tsc -p tsconfig.json"}}`)
	writeBootstrapProjectFile(t, dir, "tsconfig.json", `{"compilerOptions":{"outDir":"dist"}}`)
	writeBootstrapProjectFile(t, dir, filepath.Join("dist", "main.js"), "console.log('ok')\n")

	publisher := &fakeBundlePublisher{
		findRelease: &domain.Release{
			ID:        2,
			TagName:   "v2",
			Draft:     false,
			UploadURL: "https://uploads.example/releases/2/assets{?name,label}",
			Assets: []domain.Asset{
				{ID: 11, Name: "demo_codex-runtime_node_bundle.tar.gz"},
				{ID: 12, Name: "demo_codex-runtime_node_bundle.tar.gz.sha256"},
			},
		},
	}
	result, err := bundlePublish(context.Background(), PluginBundlePublishOptions{
		Root:     dir,
		Platform: "codex-runtime",
		Repo:     "o/r",
		Tag:      "v2",
		Force:    true,
	}, bundlePublishDeps{
		GitHub: publisher,
		Export: fakeBundleExportFunc(t, dir, exportMetadata{
			PluginName:     "demo",
			Platform:       "codex-runtime",
			Runtime:        "node",
			Manager:        "npm",
			BootstrapModel: "package-manager install + build",
			Next:           []string{"plugin-kit-ai doctor ."},
			BundleFormat:   "tar.gz",
			GeneratedBy:    "plugin-kit-ai export",
		}, "package.json", "tsconfig.json", filepath.ToSlash(filepath.Join("dist", "main.js"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{11, 12}; len(publisher.deletes) != len(want) || publisher.deletes[0] != want[0] || publisher.deletes[1] != want[1] {
		t.Fatalf("deletes = %#v", publisher.deletes)
	}
	if len(publisher.uploads) != 2 {
		t.Fatalf("uploads = %#v", publisher.uploads)
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "Release state: reused existing published release") {
		t.Fatalf("result = %v", result.Lines)
	}
}

func TestPluginServiceBundlePublishFailsWhenAssetExistsWithoutForce(t *testing.T) {
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: node\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, "package.json", `{"name":"demo","scripts":{"build":"tsc -p tsconfig.json"}}`)
	writeBootstrapProjectFile(t, dir, "tsconfig.json", `{"compilerOptions":{"outDir":"dist"}}`)
	writeBootstrapProjectFile(t, dir, filepath.Join("dist", "main.js"), "console.log('ok')\n")

	publisher := &fakeBundlePublisher{
		findRelease: &domain.Release{
			ID:        2,
			TagName:   "v2",
			Draft:     false,
			UploadURL: "https://uploads.example/releases/2/assets{?name,label}",
			Assets: []domain.Asset{
				{ID: 11, Name: "demo_codex-runtime_node_bundle.tar.gz"},
			},
		},
	}
	_, err := bundlePublish(context.Background(), PluginBundlePublishOptions{
		Root:     dir,
		Platform: "codex-runtime",
		Repo:     "o/r",
		Tag:      "v2",
	}, bundlePublishDeps{
		GitHub: publisher,
		Export: fakeBundleExportFunc(t, dir, exportMetadata{
			PluginName:     "demo",
			Platform:       "codex-runtime",
			Runtime:        "node",
			Manager:        "npm",
			BootstrapModel: "package-manager install + build",
			Next:           []string{"plugin-kit-ai doctor ."},
			BundleFormat:   "tar.gz",
			GeneratedBy:    "plugin-kit-ai export",
		}, "package.json", "tsconfig.json", filepath.ToSlash(filepath.Join("dist", "main.js"))),
	})
	if err == nil || !strings.Contains(err.Error(), "use --force to replace") {
		t.Fatalf("error = %v", err)
	}
}

func TestPluginServiceBundlePublishRejectsShellRuntime(t *testing.T) {
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: shell\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexec \"$ROOT/scripts/main.sh\" \"$@\"\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("scripts", "main.sh"), "#!/usr/bin/env bash\nexit 0\n")
	mustChmodBootstrapExecutable(t, filepath.Join(dir, "bin", "demo"))
	mustChmodBootstrapExecutable(t, filepath.Join(dir, "scripts", "main.sh"))
	renderExportTarget(t, dir, "codex-runtime")

	_, err := bundlePublish(context.Background(), PluginBundlePublishOptions{
		Root:     dir,
		Platform: "codex-runtime",
		Repo:     "o/r",
		Tag:      "v1",
	}, bundlePublishDeps{
		GitHub: &fakeBundlePublisher{},
		Export: fakeBundleExportFunc(t, dir, exportMetadata{
			PluginName:     "demo",
			Platform:       "codex-runtime",
			Runtime:        "shell",
			Manager:        "none",
			BootstrapModel: "shell",
			Next:           []string{"plugin-kit-ai doctor ."},
			BundleFormat:   "tar.gz",
			GeneratedBy:    "plugin-kit-ai export",
		}, filepath.ToSlash(filepath.Join("scripts", "main.sh"))),
	})
	if err == nil || !strings.Contains(err.Error(), `supports only exported python/node bundles`) {
		t.Fatalf("error = %v", err)
	}
}
