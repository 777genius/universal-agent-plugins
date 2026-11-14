package pluginkitairepo_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type installCompatibilityMatrix struct {
	Cases []installCompatibilityCase `json:"cases"`
}

type installCompatibilityCase struct {
	Name          string                          `json:"name"`
	ReleaseSource string                          `json:"release_source"`
	ReleaseTag    string                          `json:"release_tag"`
	ChecksumsMode string                          `json:"checksums_mode"`
	Assets        []installCompatibilityAsset     `json:"assets"`
	Expect        installCompatibilityExpectation `json:"expect"`
}

type installCompatibilityAsset struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	BinaryName string `json:"binary_name"`
	Body       string `json:"body"`
}

type installCompatibilityExpectation struct {
	ExitCode            int    `json:"exit_code"`
	InstalledName       string `json:"installed_name"`
	InstalledBody       string `json:"installed_body"`
	AssetName           string `json:"asset_name"`
	ReleaseSource       string `json:"release_source"`
	DiagnosticSubstring string `json:"diagnostic_substring"`
}

type materializedCompatibilityAsset struct {
	Name string
	Body []byte
}

func TestPluginKitAIInstall_CompatibilityMatrix(t *testing.T) {
	t.Parallel()
	requireBindTests(t)

	matrix := loadInstallCompatibilityMatrix(t)
	pluginKitAIBin := buildPluginKitAI(t)

	for _, tc := range matrix.Cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			srv, payloads := newCompatibilityGitHubServer(t, tc)
			t.Cleanup(srv.Close)

			outDir := t.TempDir()
			args := []string{"--dir", outDir, "--force"}
			switch tc.ReleaseSource {
			case "latest":
				args = append([]string{"--latest"}, args...)
			default:
				args = append([]string{"--tag", tc.ReleaseTag}, args...)
			}

			code, out := runInstall(t, pluginKitAIBin, "", srv.URL, args...)
			if code != tc.Expect.ExitCode {
				t.Fatalf("want exit %d, got %d: %s", tc.Expect.ExitCode, code, out)
			}

			if code == 0 {
				installedName := expandCompatibilityPlaceholders(tc.Expect.InstalledName)
				assetName := expandCompatibilityPlaceholders(tc.Expect.AssetName)
				assertInstallSummary(
					t,
					string(out),
					filepath.Join(outDir, installedName),
					tc.ReleaseTag,
					tc.Expect.ReleaseSource,
					assetName,
					runtime.GOOS,
					runtime.GOARCH,
					false,
				)
				got, err := os.ReadFile(filepath.Join(outDir, installedName))
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != tc.Expect.InstalledBody {
					t.Fatalf("installed body = %q, want %q", got, tc.Expect.InstalledBody)
				}
				return
			}

			if sub := strings.TrimSpace(tc.Expect.DiagnosticSubstring); sub != "" && !strings.Contains(string(out), sub) {
				t.Fatalf("want diagnostic %q in output:\n%s", sub, out)
			}

			if tc.ChecksumsMode == "missing" {
				if _, ok := payloads["checksums.txt"]; ok {
					t.Fatal("unexpected checksums payload for missing-checksums case")
				}
			}
		})
	}
}

func loadInstallCompatibilityMatrix(t *testing.T) installCompatibilityMatrix {
	t.Helper()
	path := filepath.Join(RepoRoot(t), "repotests", "testdata", "install_compatibility", "matrix.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var matrix installCompatibilityMatrix
	if err := json.Unmarshal(body, &matrix); err != nil {
		t.Fatal(err)
	}
	if len(matrix.Cases) == 0 {
		t.Fatal("empty compatibility matrix")
	}
	return matrix
}

func newCompatibilityGitHubServer(t *testing.T, tc installCompatibilityCase) (*httptest.Server, map[string][]byte) {
	t.Helper()

	payloads := make(map[string][]byte)
	assets := make([]materializedCompatibilityAsset, 0, len(tc.Assets))
	for _, asset := range tc.Assets {
		mat := materializeCompatibilityAsset(t, asset)
		assets = append(assets, mat)
		payloads[mat.Name] = mat.Body
	}
	if tc.ChecksumsMode != "missing" {
		checksums := buildChecksumsFile(t, tc.ChecksumsMode, assets)
		payloads["checksums.txt"] = checksums
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/repos/o/r/releases/tags/%s", tc.ReleaseTag) && tc.ReleaseSource != "latest":
			writeCompatibilityReleaseJSON(w, tc.ReleaseTag, base, assets, tc.ChecksumsMode)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/releases/latest" && tc.ReleaseSource == "latest":
			writeCompatibilityReleaseJSON(w, tc.ReleaseTag, base, assets, tc.ChecksumsMode)
		case r.Method == http.MethodGet && r.URL.Path == "/checksums":
			if body, ok := payloads["checksums.txt"]; ok {
				_, _ = w.Write(body)
				return
			}
			http.NotFound(w, r)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/asset/"):
			name := strings.TrimPrefix(r.URL.Path, "/asset/")
			body, ok := payloads[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))

	return srv, payloads
}

func writeCompatibilityReleaseJSON(w http.ResponseWriter, tag, base string, assets []materializedCompatibilityAsset, checksumsMode string) {
	type ghAsset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	}
	type ghRelease struct {
		TagName    string    `json:"tag_name"`
		Draft      bool      `json:"draft"`
		Prerelease bool      `json:"prerelease"`
		Assets     []ghAsset `json:"assets"`
	}

	release := ghRelease{
		TagName:    tag,
		Draft:      false,
		Prerelease: false,
	}
	if checksumsMode != "missing" {
		release.Assets = append(release.Assets, ghAsset{
			Name:               "checksums.txt",
			BrowserDownloadURL: base + "/checksums",
		})
	}
	for _, asset := range assets {
		release.Assets = append(release.Assets, ghAsset{
			Name:               asset.Name,
			BrowserDownloadURL: base + "/asset/" + asset.Name,
			Size:               int64(len(asset.Body)),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(release)
}

func materializeCompatibilityAsset(t *testing.T, asset installCompatibilityAsset) materializedCompatibilityAsset {
	t.Helper()
	name := expandCompatibilityPlaceholders(asset.Name)
	switch asset.Kind {
	case "tar.gz":
		return materializedCompatibilityAsset{
			Name: name,
			Body: mustCompatibilityTarGz(t, asset.BinaryName, []byte(asset.Body)),
		}
	case "raw", "zip", "text":
		return materializedCompatibilityAsset{Name: name, Body: []byte(asset.Body)}
	default:
		t.Fatalf("unsupported compatibility asset kind %q", asset.Kind)
		return materializedCompatibilityAsset{}
	}
}

func buildChecksumsFile(t *testing.T, mode string, assets []materializedCompatibilityAsset) []byte {
	t.Helper()
	var lines []string
	for _, asset := range assets {
		sum := sha256.Sum256(asset.Body)
		hash := hex.EncodeToString(sum[:])
		if mode == "corrupt" {
			hash = strings.Repeat("0", len(hash))
		}
		lines = append(lines, fmt.Sprintf("%s  %s", hash, asset.Name))
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func expandCompatibilityPlaceholders(s string) string {
	repl := strings.NewReplacer(
		"{{goos}}", runtime.GOOS,
		"{{goarch}}", runtime.GOARCH,
		"{{exe_suffix}}", compatibilityExeSuffix(),
	)
	return repl.Replace(s)
}

func compatibilityExeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func mustCompatibilityTarGz(t *testing.T, name string, body []byte) []byte {
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
