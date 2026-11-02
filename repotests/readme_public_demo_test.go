package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The README poster is a link into the site's real Mux demo, not a second
// decorative video card. The browser playback/fullscreen contract is qualified
// separately by landing/tests/browser/hero-demo-video.spec.ts.
func TestREADMEHasOneRealDemoThumbnail(t *testing.T) {
	root := RepoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(body)
	demoAnchor := regexp.MustCompile(`(?s)<a\s+href="https://universal-agent-plugins\.com/#demo"[^>]*>\s*<img\b[^>]*>\s*</a>`)
	matches := demoAnchor.FindAllString(readme, -1)
	if len(matches) != 1 || strings.Count(readme, `href="https://universal-agent-plugins.com/#demo"`) != 1 {
		t.Fatalf("README needs one clickable image linking to the real demo, found %d", len(matches))
	}
	poster := matches[0]
	if strings.Count(poster, "<img") != 1 ||
		!strings.Contains(poster, `alt="Universal Agent Plugins video preview`) ||
		!strings.Contains(poster, `src="https://github.com/user-attachments/assets/`) ||
		strings.Contains(readme, "assets/readme-demo.svg") {
		t.Fatal("README demo poster is missing, inaccessible or duplicated by a decorative placeholder")
	}
	player, err := os.ReadFile(filepath.Join(root, "landing", "components", "registry", "HeroDemoVideo.vue"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(player), `id="demo"`) ||
		!strings.Contains(string(player), `@click="playDemo"`) {
		t.Fatal("README demo link does not target the real playable landing control")
	}
}

// README paths are part of the public install and evidence journey. Keep
// renamed docs and local client icons from silently producing broken links.
// Remote web URLs are intentionally not tested by this offline contract.
func TestREADMELocalReferencesExist(t *testing.T) {
	root := RepoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	markdown := regexp.MustCompile(`\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	media := regexp.MustCompile(`\b(?:src|srcset)="([^"]+)"`)
	var references []string
	for _, match := range markdown.FindAllStringSubmatch(string(body), -1) {
		references = append(references, match[1])
	}
	for _, match := range media.FindAllStringSubmatch(string(body), -1) {
		references = append(references, match[1])
	}
	seen := map[string]bool{}
	for _, reference := range references {
		if strings.HasPrefix(reference, "#") || strings.HasPrefix(reference, "https://") ||
			strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "mailto:") {
			continue
		}
		path := strings.SplitN(strings.SplitN(reference, "#", 2)[0], "?", 2)[0]
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		clean := filepath.Clean(filepath.FromSlash(path))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			t.Errorf("README reference escapes repository: %q", reference)
			continue
		}
		if _, err := os.Stat(filepath.Join(root, clean)); err != nil {
			t.Errorf("README local reference %q cannot be opened: %v", reference, err)
		}
	}
	if len(seen) < 20 {
		t.Fatalf("README reference test selected only %d local paths, check parser", len(seen))
	}
}
