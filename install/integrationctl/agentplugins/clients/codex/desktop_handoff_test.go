package codex

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDesktopPluginLinkIsAnEncodedLocalDetailHandoff(t *testing.T) {
	root := filepath.Join(tempRoot(t), "managed plugin")
	link, err := desktopPluginLink(root, "stripe")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "codex" || u.Host != "plugins" || u.Path != "/stripe" {
		t.Fatalf("unexpected desktop plugin URL: %q %v", link, err)
	}
	if values := u.Query()["marketplacePath"]; len(values) != 1 || values[0] != filepath.Join(root, ".agents", "plugins", "marketplace.json") {
		t.Fatalf("wrong marketplace path: %q", link)
	}
	if strings.Contains(link, "managed plugin") || strings.Contains(link, "\n") {
		t.Fatal("marketplace root was not URL-encoded")
	}
}

func TestDesktopPluginLinkRejectsForeignOrMalformedData(t *testing.T) {
	root := filepath.Join(tempRoot(t), "managed")
	cases := []struct{ name, path, plugin string }{
		{"relative root", "relative/path", "stripe"},
		{"parent escape", filepath.Join(root, "..", "managed", ".."), "stripe"},
		{"query injection", root, "stripe?mode=share"},
		{"fragment injection", root, "stripe#fragment"},
		{"noncanonical name", root, "../stripe"},
		{"empty name", root, ""},
		{"space in name", root, "stripe plugin"},
		{"newline in path", root + "\nother", "stripe"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if link, err := desktopPluginLink(tt.path, tt.plugin); err == nil || link != "" {
				t.Fatalf("unexpected link %q, error %v", link, err)
			}
		})
	}
}

func TestDesktopPluginHandoffRefusesMissingAndForeignMarketplace(t *testing.T) {
	root := filepath.Join(tempRoot(t), "managed")
	if got := desktopPluginHandoff(root, "stripe"); got != "" {
		t.Fatalf("unprepared marketplace produced a link: %q", got)
	}
	path := filepath.Join(root, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"name":"agentplugins-test","plugins":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	out := desktopPluginHandoff(root, "stripe")
	if !strings.Contains(out, "codex://plugins/stripe?marketplacePath=") ||
		!strings.Contains(out, "finish installation or enablement") ||
		!strings.Contains(out, "does not prove ChatGPT runtime use") {
		t.Fatalf("missing bounded local-desktop step: %q", out)
	}
	// A foreign replacement must not be advertised as an owned catalog.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		foreign := filepath.Join(tempRoot(t), "other.json")
		if err := os.WriteFile(foreign, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(foreign, path); err != nil {
			t.Fatal(err)
		}
		if out := desktopPluginHandoff(root, "stripe"); out != "" {
			t.Fatalf("symlink marketplace produced a link: %q", out)
		}
	}
}
