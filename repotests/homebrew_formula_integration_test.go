package pluginkitairepo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomebrewFormulaGeneratorFromChecksums(t *testing.T) {
	t.Parallel()
	root := RepoRoot(t)
	tmp := t.TempDir()
	checksumsPath := filepath.Join(tmp, "checksums.txt")
	outputPath := filepath.Join(tmp, "plugin-kit-ai.rb")
	body := strings.Join([]string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  plugin-kit-ai_1.2.3_darwin_amd64.tar.gz",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  plugin-kit-ai_1.2.3_darwin_arm64.tar.gz",
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc  plugin-kit-ai_1.2.3_linux_amd64.tar.gz",
		"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd  plugin-kit-ai_1.2.3_linux_arm64.tar.gz",
	}, "\n") + "\n"
	if err := os.WriteFile(checksumsPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("find go in PATH: %v", err)
	}

	cmd := exec.Command(goBin, "run", "./cmd/plugin-kit-ai-homebrew-gen",
		"--tag", "v1.2.3",
		"--repo", "777genius/plugin-kit-ai",
		"--checksums", checksumsPath,
		"--download-base", "https://github.com/777genius/plugin-kit-ai/releases/download/v1.2.3",
		"--output", outputPath,
	)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run homebrew generator: %v\n%s", err, out)
	}

	formulaBody, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	formula := string(formulaBody)
	for _, want := range []string{
		`class PluginKitAi < Formula`,
		`version "1.2.3"`,
		`url "https://github.com/777genius/plugin-kit-ai/releases/download/v1.2.3/plugin-kit-ai_1.2.3_darwin_arm64.tar.gz"`,
		`sha256 "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"`,
		`url "https://github.com/777genius/plugin-kit-ai/releases/download/v1.2.3/plugin-kit-ai_1.2.3_linux_amd64.tar.gz"`,
		`sha256 "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"`,
		`bin.install "plugin-kit-ai"`,
	} {
		if !strings.Contains(formula, want) {
			t.Fatalf("formula missing %q:\n%s", want, formula)
		}
	}
}
