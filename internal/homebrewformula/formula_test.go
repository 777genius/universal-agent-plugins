package homebrewformula

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSelectsAllSupportedAssets(t *testing.T) {
	t.Parallel()
	checksumsPath := filepath.Join(t.TempDir(), "checksums.txt")
	body := strings.Join([]string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  plugin-kit-ai_1.2.3_darwin_amd64.tar.gz",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  plugin-kit-ai_1.2.3_darwin_arm64.tar.gz",
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc  plugin-kit-ai_1.2.3_linux_amd64.tar.gz",
		"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd  plugin-kit-ai_1.2.3_linux_arm64.tar.gz",
	}, "\n") + "\n"
	if err := os.WriteFile(checksumsPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Build("plugin-kit-ai-v1.2.3", "777genius/plugin-kit-ai", checksumsPath, "https://github.com/777genius/plugin-kit-ai/releases/download/plugin-kit-ai-v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "1.2.3" {
		t.Fatalf("version = %q", got.Version)
	}
	if got.Tag != "plugin-kit-ai-v1.2.3" {
		t.Fatalf("tag = %q", got.Tag)
	}
	if len(got.Assets) != 4 {
		t.Fatalf("assets len = %d", len(got.Assets))
	}
}

func TestBuildFailsWhenChecksumsMissingAsset(t *testing.T) {
	t.Parallel()
	checksumsPath := filepath.Join(t.TempDir(), "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  plugin-kit-ai_1.2.3_darwin_amd64.tar.gz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Build("v1.2.3", "777genius/plugin-kit-ai", checksumsPath, "https://github.com/777genius/plugin-kit-ai/releases/download/v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "missing asset") {
		t.Fatalf("err = %v", err)
	}
}

func TestRenderPinsExpectedURLsAndChecksums(t *testing.T) {
	t.Parallel()
	f := Formula{
		Version:   "1.2.3",
		ClassName: "PluginKitAi",
		Desc:      "AI CLI plugin runtime with a first-class Go SDK",
		Homepage:  "https://github.com/777genius/plugin-kit-ai",
		Assets: []Asset{
			{GOOS: "darwin", GOARCH: "amd64", URL: "https://example/plugin-kit-ai_1.2.3_darwin_amd64.tar.gz", SHA256: strings.Repeat("a", 64)},
			{GOOS: "darwin", GOARCH: "arm64", URL: "https://example/plugin-kit-ai_1.2.3_darwin_arm64.tar.gz", SHA256: strings.Repeat("b", 64)},
			{GOOS: "linux", GOARCH: "amd64", URL: "https://example/plugin-kit-ai_1.2.3_linux_amd64.tar.gz", SHA256: strings.Repeat("c", 64)},
			{GOOS: "linux", GOARCH: "arm64", URL: "https://example/plugin-kit-ai_1.2.3_linux_arm64.tar.gz", SHA256: strings.Repeat("d", 64)},
		},
	}
	body, err := Generate(f)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		`version "1.2.3"`,
		`url "https://example/plugin-kit-ai_1.2.3_darwin_arm64.tar.gz"`,
		`sha256 "` + strings.Repeat("b", 64) + `"`,
		`url "https://example/plugin-kit-ai_1.2.3_linux_amd64.tar.gz"`,
		`sha256 "` + strings.Repeat("c", 64) + `"`,
		`bin.install "plugin-kit-ai"`,
		`assert_match version.to_s, shell_output("#{bin}/plugin-kit-ai version")`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formula missing %q:\n%s", want, text)
		}
	}
}
