package usecase

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/plugininstall/adapters/archive"
	"github.com/777genius/plugin-kit-ai/plugininstall/adapters/fs"
	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
	"github.com/777genius/plugin-kit-ai/plugininstall/internal/checksum"
	"github.com/777genius/plugin-kit-ai/plugininstall/ports"
)

type fakeGH struct {
	rel       *domain.Release
	latestRel *domain.Release
	blobs     map[string][]byte
}

type testResolver struct{}

func (testResolver) Resolve(path string) (string, error) {
	return filepath.Abs(path)
}

type testSelector struct{}

func (testSelector) Pick(assets []domain.Asset, target ports.TargetPlatform) (*domain.Asset, bool, error) {
	return domain.PickInstallAsset(assets, target.GOOS, target.GOARCH)
}

type testChecksums struct{}

func (testChecksums) Expected(checksumsFile []byte, assetName string) ([]byte, error) {
	return checksum.ExpectedSum(checksumsFile, assetName)
}

func (testChecksums) Verify(payload []byte, expected []byte, assetName string) error {
	got := sha256.Sum256(payload)
	if len(expected) != len(got) {
		return domain.NewError(domain.ExitChecksum, "internal checksum length")
	}
	for i := range expected {
		if expected[i] != got[i] {
			return domain.NewError(domain.ExitChecksum, "sha256 mismatch for "+assetName)
		}
	}
	return nil
}

type testPerms struct{}

func (testPerms) FileMode(target ports.TargetPlatform) uint32 {
	if target.GOOS == "windows" {
		return 0o644
	}
	return 0o755
}

func testTarget() ports.TargetPlatform {
	return ports.TargetPlatform{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
}

func newTestInstaller(fake *fakeGH) *Installer {
	return &Installer{
		GitHub:    fake,
		Archive:   archive.TarGzExtractor{},
		FS:        fs.OS{},
		Resolver:  testResolver{},
		Selector:  testSelector{},
		Checksums: testChecksums{},
		Perms:     testPerms{},
	}
}

func (f *fakeGH) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*domain.Release, error) {
	return f.rel, nil
}

func (f *fakeGH) GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error) {
	if f.latestRel != nil {
		return f.latestRel, nil
	}
	return f.rel, nil
}

func (f *fakeGH) DownloadAsset(ctx context.Context, url string) ([]byte, string, error) {
	b, ok := f.blobs[url]
	if !ok {
		return nil, "", domain.NewError(domain.ExitNetwork, "unknown url "+url)
	}
	return b, "application/octet-stream", nil
}

func TestInstaller_Run_happyPath(t *testing.T) {
	t.Parallel()
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGzFixture(t, binName, []byte("binarydata"))
	sum := sha256.Sum256(tarGz)
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)
	checksumsBody := []byte(line)
	if _, err := checksum.ExpectedSum(checksumsBody, archName); err != nil {
		t.Fatal(err)
	}

	base := "https://example.test"
	rel := &domain.Release{
		TagName: "v1.0.0",
		Assets: []domain.Asset{
			{Name: "checksums.txt", BrowserDownloadURL: base + "/c"},
			{Name: archName, BrowserDownloadURL: base + "/a"},
		},
	}
	fake := &fakeGH{
		rel: rel,
		blobs: map[string][]byte{
			base + "/c": checksumsBody,
			base + "/a": tarGz,
		},
	}
	dir := t.TempDir()
	inst := newTestInstaller(fake)
	got, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v1.0.0",
		InstallDir: dir,
		Force:      true,
		Target:     testTarget(),
	})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, binName)
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "binarydata" {
		t.Fatalf("got %q", b)
	}
	if got.ResolvedInstallPath != out {
		t.Fatalf("resolved path = %q, want %q", got.ResolvedInstallPath, out)
	}
	if got.InstalledFileName != binName {
		t.Fatalf("installed name = %q, want %q", got.InstalledFileName, binName)
	}
	if got.ReleaseRef != "v1.0.0" || got.ReleaseSource != "tag" {
		t.Fatalf("release = %q (%q)", got.ReleaseRef, got.ReleaseSource)
	}
	if got.AssetName != archName {
		t.Fatalf("asset = %q, want %q", got.AssetName, archName)
	}
	if got.TargetGOOS != runtime.GOOS || got.TargetGOARCH != runtime.GOARCH {
		t.Fatalf("target = %s/%s", got.TargetGOOS, got.TargetGOARCH)
	}
	if got.Overwrote {
		t.Fatal("did not expect overwrite")
	}
	if got.PayloadKind != "tar.gz" {
		t.Fatalf("payload kind = %q", got.PayloadKind)
	}
}

func TestInstaller_prereleaseRejected(t *testing.T) {
	t.Parallel()
	fake := &fakeGH{
		rel: &domain.Release{
			TagName:    "v0-rc",
			Prerelease: true,
			Assets:     []domain.Asset{{Name: "checksums.txt"}},
		},
		blobs: map[string][]byte{},
	}
	inst := newTestInstaller(fake)
	_, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v0-rc",
		InstallDir: t.TempDir(),
		Force:      true,
		Target:     testTarget(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ExitRelease {
		t.Fatalf("got %v", err)
	}
}

func TestInstaller_prereleaseAllowedWithPre(t *testing.T) {
	t.Parallel()
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGzFixture(t, binName, []byte("x"))
	sum := sha256.Sum256(tarGz)
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)
	checksumsBody := []byte(line)
	base := "https://example.test"
	rel := &domain.Release{
		TagName:    "v0-rc",
		Prerelease: true,
		Assets: []domain.Asset{
			{Name: "checksums.txt", BrowserDownloadURL: base + "/c"},
			{Name: archName, BrowserDownloadURL: base + "/a"},
		},
	}
	fake := &fakeGH{
		rel: rel,
		blobs: map[string][]byte{
			base + "/c": checksumsBody,
			base + "/a": tarGz,
		},
	}
	dir := t.TempDir()
	inst := newTestInstaller(fake)
	got, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v0-rc",
		InstallDir:      dir,
		Force:           true,
		AllowPrerelease: true,
		Target:          testTarget(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(filepath.Join(dir, binName)); err != nil {
		t.Fatal(err)
	}
	if got.ReleaseRef != "v0-rc" || got.ReleaseSource != "tag" || got.PayloadKind != "tar.gz" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestInstaller_outputName(t *testing.T) {
	t.Parallel()
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	wantName := "myplugin"
	tarGz := mustTarGzFixture(t, binName, []byte("data"))
	sum := sha256.Sum256(tarGz)
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)
	base := "https://example.test"
	rel := &domain.Release{
		TagName: "v1",
		Assets: []domain.Asset{
			{Name: "checksums.txt", BrowserDownloadURL: base + "/c"},
			{Name: archName, BrowserDownloadURL: base + "/a"},
		},
	}
	fake := &fakeGH{
		rel: rel,
		blobs: map[string][]byte{
			base + "/c": []byte(line),
			base + "/a": tarGz,
		},
	}
	dir := t.TempDir()
	inst := newTestInstaller(fake)
	got, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v1",
		InstallDir: dir,
		Force:      true,
		OutputName: wantName,
		Target:     testTarget(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, binName)); err == nil {
		t.Fatal("expected archive name not used as path")
	}
	b, err := os.ReadFile(filepath.Join(dir, wantName))
	if err != nil || string(b) != "data" {
		t.Fatalf("read %v: %q", err, b)
	}
	if got.InstalledFileName != wantName || got.ResolvedInstallPath != filepath.Join(dir, wantName) {
		t.Fatalf("result = %+v", got)
	}
}

func TestInstaller_invalidOutputName(t *testing.T) {
	t.Parallel()
	fake := &fakeGH{
		rel: &domain.Release{
			TagName: "v1",
			Assets: []domain.Asset{
				{Name: "checksums.txt", BrowserDownloadURL: "https://x/c"},
				{Name: fmt.Sprintf("p_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH), BrowserDownloadURL: "https://x/a"},
			},
		},
		blobs: map[string][]byte{},
	}
	inst := newTestInstaller(fake)
	_, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v1",
		InstallDir: t.TempDir(),
		OutputName: "evil/bin",
		Target:     testTarget(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ExitUsage {
		t.Fatalf("got %v", err)
	}
}

func TestInstaller_missingChecksums(t *testing.T) {
	t.Parallel()
	fake := &fakeGH{
		rel: &domain.Release{
			TagName: "v1",
			Assets: []domain.Asset{
				{Name: fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH), BrowserDownloadURL: "https://x/a"},
			},
		},
		blobs: map[string][]byte{},
	}
	inst := newTestInstaller(fake)
	_, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v1",
		InstallDir: t.TempDir(),
		Target:     testTarget(),
	})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ExitChecksum {
		t.Fatalf("got %v", err)
	}
}

func TestInstaller_checksumMismatch(t *testing.T) {
	t.Parallel()
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGzFixture(t, binName, []byte("payload"))
	base := "https://example.test"
	fake := &fakeGH{
		rel: &domain.Release{
			TagName: "v1",
			Assets: []domain.Asset{
				{Name: "checksums.txt", BrowserDownloadURL: base + "/c"},
				{Name: archName, BrowserDownloadURL: base + "/a"},
			},
		},
		blobs: map[string][]byte{
			base + "/c": []byte("deadbeef  " + archName + "\n"),
			base + "/a": tarGz,
		},
	}
	inst := newTestInstaller(fake)
	_, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v1",
		InstallDir: t.TempDir(),
		Target:     testTarget(),
	})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ExitChecksum {
		t.Fatalf("got %v", err)
	}
}

func TestInstaller_rawBinaryLikeNotificationsPlugin(t *testing.T) {
	t.Parallel()
	rawName := fmt.Sprintf("claude-notifications-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		rawName += ".exe"
	}
	payload := []byte("fake-binary-payload")
	sum := sha256.Sum256(payload)
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), rawName)
	base := "https://example.test"
	rel := &domain.Release{
		TagName: "v1.34.0",
		Assets: []domain.Asset{
			{Name: "checksums.txt", BrowserDownloadURL: base + "/c"},
			{Name: rawName, BrowserDownloadURL: base + "/b"},
		},
	}
	fake := &fakeGH{
		rel: rel,
		blobs: map[string][]byte{
			base + "/c": []byte(line),
			base + "/b": payload,
		},
	}
	dir := t.TempDir()
	inst := newTestInstaller(fake)
	got, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v1.34.0",
		InstallDir: dir,
		Force:      true,
		Target:     testTarget(),
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, rawName))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Fatalf("content %q", data)
	}
	if got.AssetName != rawName || got.PayloadKind != "raw" || got.ReleaseSource != "tag" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestInstaller_useLatest(t *testing.T) {
	t.Parallel()
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGzFixture(t, binName, []byte("z"))
	sum := sha256.Sum256(tarGz)
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)
	base := "https://example.test"
	rel := &domain.Release{TagName: "v2.0.0", Assets: []domain.Asset{
		{Name: "checksums.txt", BrowserDownloadURL: base + "/c"},
		{Name: archName, BrowserDownloadURL: base + "/a"},
	}}
	fake := &fakeGH{
		latestRel: rel,
		blobs: map[string][]byte{
			base + "/c": []byte(line),
			base + "/a": tarGz,
		},
	}
	dir := t.TempDir()
	inst := newTestInstaller(fake)
	got, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", UseLatest: true,
		InstallDir: dir,
		Force:      true,
		Target:     testTarget(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, binName)); err != nil || string(b) != "z" {
		t.Fatalf("read %v %q", err, b)
	}
	if got.ReleaseRef != "v2.0.0" || got.ReleaseSource != "latest" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestInstaller_existingFileWithoutForceRejected(t *testing.T) {
	t.Parallel()
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGzFixture(t, binName, []byte("new-data"))
	sum := sha256.Sum256(tarGz)
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)
	base := "https://example.test"
	fake := &fakeGH{
		rel: &domain.Release{
			TagName: "v1",
			Assets: []domain.Asset{
				{Name: "checksums.txt", BrowserDownloadURL: base + "/c"},
				{Name: archName, BrowserDownloadURL: base + "/a"},
			},
		},
		blobs: map[string][]byte{
			base + "/c": []byte(line),
			base + "/a": tarGz,
		},
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, binName)
	if err := os.WriteFile(dest, []byte("old-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	inst := newTestInstaller(fake)
	_, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v1",
		InstallDir: dir,
		Target:     testTarget(),
	})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ExitFS {
		t.Fatalf("got %v", err)
	}
}

func TestInstaller_overwriteExistingFileReportsOverwrite(t *testing.T) {
	t.Parallel()
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGzFixture(t, binName, []byte("new-data"))
	sum := sha256.Sum256(tarGz)
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)
	base := "https://example.test"
	fake := &fakeGH{
		rel: &domain.Release{
			TagName: "v1",
			Assets: []domain.Asset{
				{Name: "checksums.txt", BrowserDownloadURL: base + "/c"},
				{Name: archName, BrowserDownloadURL: base + "/a"},
			},
		},
		blobs: map[string][]byte{
			base + "/c": []byte(line),
			base + "/a": tarGz,
		},
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, binName)
	if err := os.WriteFile(dest, []byte("old-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	inst := newTestInstaller(fake)
	got, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v1",
		InstallDir: dir,
		Force:      true,
		Target:     testTarget(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Overwrote {
		t.Fatal("expected overwrite result")
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "new-data" {
		t.Fatalf("got %q", b)
	}
}

func TestInstaller_destinationDirectoryRejectedEvenWithForce(t *testing.T) {
	t.Parallel()
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGzFixture(t, binName, []byte("new-data"))
	sum := sha256.Sum256(tarGz)
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)
	base := "https://example.test"
	fake := &fakeGH{
		rel: &domain.Release{
			TagName: "v1",
			Assets: []domain.Asset{
				{Name: "checksums.txt", BrowserDownloadURL: base + "/c"},
				{Name: archName, BrowserDownloadURL: base + "/a"},
			},
		},
		blobs: map[string][]byte{
			base + "/c": []byte(line),
			base + "/a": tarGz,
		},
	}
	dir := t.TempDir()
	destDir := filepath.Join(dir, binName)
	if err := os.Mkdir(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	inst := newTestInstaller(fake)
	_, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v1",
		InstallDir: dir,
		Force:      true,
		Target:     testTarget(),
	})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ExitFS {
		t.Fatalf("got %v", err)
	}
}

func TestInstaller_installDirAsFileRejected(t *testing.T) {
	t.Parallel()
	installPath := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(installPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	inst := newTestInstaller(&fakeGH{})
	_, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v1",
		InstallDir: installPath,
		Target:     testTarget(),
	})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ExitFS {
		t.Fatalf("got %v", err)
	}
}

func TestInstaller_tagAndLatestRejected(t *testing.T) {
	t.Parallel()
	inst := newTestInstaller(&fakeGH{})
	_, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r", Tag: "v1", UseLatest: true,
		InstallDir: t.TempDir(),
		Target:     testTarget(),
	})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ExitUsage {
		t.Fatalf("got %v", err)
	}
}

func TestInstaller_rawBinary_skipsCompanionUtilities(t *testing.T) {
	t.Parallel()
	goos, goarch := runtime.GOOS, runtime.GOARCH
	sfx := fmt.Sprintf("-%s-%s", goos, goarch)
	if goos == "windows" {
		sfx += ".exe"
	}
	assets := []domain.Asset{
		{Name: "sound-preview" + sfx},
		{Name: "list-devices" + sfx},
		{Name: "myplugin" + sfx},
	}
	payload, fromTar, err := domain.PickInstallAsset(assets, goos, goarch)
	if err != nil {
		t.Fatal(err)
	}
	if fromTar || payload == nil || payload.Name != "myplugin"+sfx {
		t.Fatalf("got %+v tar=%v", payload, fromTar)
	}
}

func TestInstaller_neitherTagNorLatest(t *testing.T) {
	t.Parallel()
	inst := newTestInstaller(&fakeGH{})
	_, err := inst.Run(context.Background(), Input{
		Owner: "o", Repo: "r",
		InstallDir: t.TempDir(),
		Target:     testTarget(),
	})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ExitUsage {
		t.Fatalf("got %v", err)
	}
}

func mustTarGzFixture(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
