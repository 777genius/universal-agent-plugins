package codex

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestDesktopPluginLinkUsesLocalDetailURL(t *testing.T) {
	root := filepath.Join(tempRoot(t), "managed plugin")
	link, err := desktopPluginLink(root, "stripe")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "codex" || u.Host != "plugins" || u.Path != "/stripe" {
		t.Fatalf("unexpected desktop URL %q: %v", link, err)
	}
	want := filepath.Join(root, ".agents", "plugins", "marketplace.json")
	if values := u.Query()["marketplacePath"]; len(values) != 1 || values[0] != want {
		t.Fatalf("wrong marketplace path: %q", link)
	}
	if strings.Contains(link, "managed plugin") || strings.Contains(link, "\n") {
		t.Fatal("unencoded marketplace path")
	}
}

func TestDesktopPluginLinkRejectsUnsafeInputs(t *testing.T) {
	root := filepath.Join(tempRoot(t), "managed")
	cases := []struct{ name, path, plugin string }{
		{"relative root", "relative/path", "stripe"},
		{"unclean root", root + string(filepath.Separator) + "..", "stripe"},
		{"question mark", root, "stripe?mode=share"},
		{"fragment", root, "stripe#fragment"},
		{"path traversal", root, "../stripe"},
		{"empty plugin", root, ""},
		{"noncanonical plugin", root, "Stripe Plugin"},
		{"control in root", root + "\nother", "stripe"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if link, err := desktopPluginLink(tt.path, tt.plugin); err == nil || link != "" {
				t.Fatalf("unsafe link %q, error %v", link, err)
			}
		})
	}
}

func TestDesktopHandoffRequiresExactManagedMarketplace(t *testing.T) {
	root := filepath.Join(tempRoot(t), "managed")
	if got := desktopPluginHandoff(root, "stripe", "artifact"); got != "" {
		t.Fatalf("unprepared marketplace produced link: %q", got)
	}
	path := filepath.Join(root, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := desktopPluginHandoff(root, "stripe", "artifact"); got != "" {
		t.Fatalf("foreign marketplace produced link: %q", got)
	}
	marketplace := shared.ManagedMarketplaceName("artifact")
	payload := fmt.Sprintf(`{"name":%q,"plugins":[{"name":"stripe","source":{"source":"local","path":"./"}}]}`, marketplace)
	if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	got := desktopPluginHandoff(root, "stripe", "artifact")
	if !strings.Contains(got, "codex://plugins/stripe?marketplacePath=") ||
		!strings.Contains(got, "finish installation or enablement") ||
		!strings.Contains(got, "does not prove ChatGPT runtime use") {
		t.Fatalf("missing honest desktop guidance: %q", got)
	}
	if got := desktopPluginHandoff(root, "other", "artifact"); got != "" {
		t.Fatalf("foreign package received link: %q", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		foreign := filepath.Join(tempRoot(t), "other.json")
		if err := os.WriteFile(foreign, []byte(payload), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(foreign, path); err != nil {
			t.Fatal(err)
		}
		if got := desktopPluginHandoff(root, "stripe", "artifact"); got != "" {
			t.Fatalf("symlink marketplace received link: %q", got)
		}
	}
}

func TestDesktopHandoffDoesNotUpgradeCodexVerification(t *testing.T) {
	root, profile := filepath.Join(tempRoot(t), "active"), tempRoot(t)
	marketplace := filepath.Join(root, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(marketplace), 0700); err != nil {
		t.Fatal(err)
	}
	payload := fmt.Sprintf(`{"name":%q,"plugins":[{"name":"demo","source":{"source":"local","path":"./"}}]}`, shared.ManagedMarketplaceName("artifact"))
	if err := os.WriteFile(marketplace, []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runner := &profileRunner{}
	client := domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: profile}
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientCodex, PhysicalArtifactID: "artifact", DeclaredName: "demo",
		NativeRegistryRoot: profile, NativeRegistryExecutable: executable,
	}
	outcome, err := New().Activate(context.Background(), clients.Env{Runner: runner}, domain.ActivationRequest{
		Client: client, Plan: plan, Delivery: domain.StagedDelivery{ClientID: domain.ClientCodex, ActivePath: root},
		DeclaredName: "demo", BackendExecutable: executable,
	})
	if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("Codex lifecycle changed: %+v / %v", outcome, err)
	}
	if len(outcome.LocalActions) == 0 || !strings.HasPrefix(outcome.LocalActions[0], "start a new Codex session") {
		t.Fatalf("Codex's native next step must remain first: %+v", outcome.LocalActions)
	}
	if !strings.Contains(strings.Join(outcome.LocalActions, "\n"), "codex://plugins/demo?marketplacePath=") {
		t.Fatalf("desktop user guidance missing from verified Codex installation: %+v", outcome)
	}
	// The native listing is successful but the optional Desktop step does not
	// assert that a signed-in ChatGPT runtime has executed the plugin.
	if !strings.Contains(strings.Join(outcome.LocalActions, "\n"), "does not prove ChatGPT runtime use") {
		t.Fatal("ChatGPT runtime caveat disappeared")
	}
}

func TestManualDesktopHandoffRemainsVisibleInFirstCodexNextAction(t *testing.T) {
	root, profile := filepath.Join(tempRoot(t), "active"), tempRoot(t)
	path := filepath.Join(root, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	document := fmt.Sprintf(`{"name":%q,"plugins":[{"name":"demo","source":{"source":"local","path":"./"}}]}`, shared.ManagedMarketplaceName("artifact"))
	if err := os.WriteFile(path, []byte(document), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := New().Activate(context.Background(), clients.Env{}, domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: profile},
		Plan: domain.DeliveryPlan{
			ClientID: domain.ClientCodex, Scope: domain.ScopeUser,
			PhysicalArtifactID: "artifact", DeclaredName: "demo",
			NativeRegistryRoot: profile,
		},
		Delivery: domain.StagedDelivery{ClientID: domain.ClientCodex, ActivePath: root},
		DeclaredName: "demo",
	})
	if err != nil || out.Activation != domain.ActivationManual {
		t.Fatalf("manual Codex path changed: %+v %v", out, err)
	}
	if len(out.LocalActions) != 1 || !strings.HasPrefix(out.LocalActions[0], "in Codex, install") ||
		!strings.Contains(out.LocalActions[0], "codex://plugins/demo?marketplacePath=") {
		t.Fatalf("optional Desktop link not visible in first local next action: %+v", out.LocalActions)
	}
}

func TestDesktopHandoffRejectsOversizedLocalManifest(t *testing.T) {
	root := filepath.Join(tempRoot(t), "active")
	path := filepath.Join(root, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 64<<10+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if action := desktopPluginHandoff(root, "stripe", "artifact"); action != "" {
		t.Fatalf("oversized local manifest should not produce a link: %q", action)
	}
}
