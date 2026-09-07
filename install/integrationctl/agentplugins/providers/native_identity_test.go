package providers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type identityRunner struct {
	result   legacyports.CommandResult
	commands [][]string
	last     legacyports.Command
}

type blockingIdentityRunner struct {
	returned chan struct{}
}

func (runner blockingIdentityRunner) Run(ctx context.Context, _ legacyports.Command) (legacyports.CommandResult, error) {
	<-ctx.Done()
	close(runner.returned)
	return legacyports.CommandResult{}, ctx.Err()
}

func (runner *identityRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.commands = append(runner.commands, append([]string(nil), command.Argv...))
	runner.last = command
	return runner.result, nil
}

func TestNativeIdentityCursorReadsEveryAuthoritativeLocalManifest(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".cursor", "plugins", "local")
	writeIdentityFile(t, filepath.Join(root, "foreign-path", ".cursor-plugin", "plugin.json"), `{"name":"demo"}`)
	plan := identityPlan(root)
	observation, err := (NativeIdentityObserver{}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCursor}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("observation = %+v, err = %v", observation, err)
	}

	writeIdentityFile(t, filepath.Join(root, "foreign-path", ".cursor-plugin", "plugin.json"), `{"name":`)
	observation, err = (NativeIdentityObserver{}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCursor}, plan, nil)
	if observation.State != domain.NativeIdentityIndeterminate || err == nil {
		t.Fatalf("malformed observation = %+v, err = %v", observation, err)
	}
}

// TestNativeIdentityUnqualifiedPluginRootIgnoresForeignNonDirectoryEntries
// reproduces a real defect found reviewing Claude Code's own equivalent scan:
// a plain OS-generated file such as .DS_Store sitting in a shared plugins
// root (created automatically the moment a user browses the folder in
// Finder) made every entry indeterminate, refusing repair/update for every
// unrelated, healthy plugin sharing that root. A plain file cannot contain
// the manifest this scheme requires, so it can never claim a competing
// plugin identity and must not block classification of real entries.
func TestNativeIdentityUnqualifiedPluginRootIgnoresForeignNonDirectoryEntries(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".cursor", "plugins", "local")
	plan := identityPlan(root)
	if err := os.MkdirAll(plan.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeIdentityFile(t, filepath.Join(plan.ActivePath, ".cursor-plugin", "plugin.json"), `{"name":"demo"}`)
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte{0}, 0o600); err != nil {
		t.Fatal(err)
	}
	managed := &domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "sha256:owned"}}}
	observer := NativeIdentityObserver{Stager: acceptingPackageVerifier{}}
	observation, err := observer.ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCursor}, plan, managed)
	if err != nil || observation.State != domain.NativeIdentityManaged {
		t.Fatalf("foreign non-directory entry blocked classification: observation = %+v, err = %v", observation, err)
	}

	// A genuine competing directory entry must still be caught.
	writeIdentityFile(t, filepath.Join(root, "foreign-path", ".cursor-plugin", "plugin.json"), `{"name":"demo"}`)
	observation, err = observer.ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCursor}, plan, managed)
	if observation.State != domain.NativeIdentityUnmanaged || err != nil {
		t.Fatalf("real collision was not caught alongside a foreign file: observation = %+v, err = %v", observation, err)
	}

	// A symlink entry must still fail closed.
	if err := os.RemoveAll(filepath.Join(root, "foreign-path")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(plan.ActivePath, filepath.Join(root, "suspicious-link")); err != nil {
		t.Fatal(err)
	}
	observation, err = observer.ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCursor}, plan, managed)
	if observation.State != domain.NativeIdentityIndeterminate {
		t.Fatalf("symlink entry was not refused: observation = %+v, err = %v", observation, err)
	}
}

// TestNativeIdentityOpenCodeIgnoresForeignNonDirectoryEntries confirms the
// same fix protects OpenCode too: OpenCode's own native registry check
// (inspectNativeRegistry) never scans a directory, but its prepared-identity
// check still goes through the shared inspectUnqualifiedPluginRoot exactly
// like Cursor's does (observeIdentity calls inspectPreparedRegistry
// unconditionally for every client before any client-specific override), so
// a foreign .DS_Store in OpenCode's managed clients root would have hit the
// identical bug if inspectUnqualifiedPluginRoot had not already been fixed.
func TestNativeIdentityOpenCodeIgnoresForeignNonDirectoryEntries(t *testing.T) {
	root := filepath.Join(t.TempDir(), "managed", "clients", "opencode")
	plan := identityPlan(root)
	if err := os.MkdirAll(plan.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeIdentityFile(t, filepath.Join(plan.ActivePath, "plugin.json"), `{"name":"demo"}`)
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte{0}, 0o600); err != nil {
		t.Fatal(err)
	}
	managed := &domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "sha256:owned"}}}
	observer := NativeIdentityObserver{Stager: acceptingPackageVerifier{}}
	observation, err := observer.ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientOpenCode}, plan, managed)
	if err != nil || observation.State != domain.NativeIdentityManaged {
		t.Fatalf("foreign non-directory entry blocked OpenCode classification: observation = %+v, err = %v", observation, err)
	}
}

// TestNativeIdentityClaudeSkillsRegistryIgnoresForeignNonDirectoryEntries is
// the Claude-specific counterpart: reproduces the exact reported scenario
// (a bare .DS_Store in <CLAUDE_CONFIG_DIR>/skills blocking every plugin's
// repair) against inspectClaudeSkillsRegistry directly.
func TestNativeIdentityClaudeSkillsRegistryIgnoresForeignNonDirectoryEntries(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	plan := identityPlan(root)
	if err := os.MkdirAll(plan.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeIdentityFile(t, filepath.Join(plan.ActivePath, ".claude-plugin", "plugin.json"), `{"name":"demo"}`)
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte{0}, 0o600); err != nil {
		t.Fatal(err)
	}
	managed := &domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "sha256:owned"}}}
	observer := NativeIdentityObserver{Stager: acceptingPackageVerifier{}}
	observation, err := observer.ObservePreparedIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude}, plan, managed)
	if err != nil || observation.State != domain.NativeIdentityManaged {
		t.Fatalf("foreign .DS_Store blocked Claude skill classification: observation = %+v, err = %v", observation, err)
	}

	// A directory that legitimately has no manifest and no SKILL.md is still
	// genuinely ambiguous and must remain refused (unchanged behavior).
	if err := os.MkdirAll(filepath.Join(root, "unrelated-empty-dir"), 0o700); err != nil {
		t.Fatal(err)
	}
	observation, err = observer.ObservePreparedIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude}, plan, managed)
	if observation.State != domain.NativeIdentityIndeterminate {
		t.Fatalf("ambiguous foreign directory was not refused: observation = %+v, err = %v", observation, err)
	}
}

func TestNativeIdentityQualifiedPreparedMarketplaceCoexistsOnlyWithPositiveNamespace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "prepared")
	writeIdentityFile(t, filepath.Join(root, "foreign", ".agents", "plugins", "marketplace.json"), `{"name":"foreign-market","plugins":[{"name":"demo"}]}`)
	plan := identityPlan(root)
	plan.NativeRegistryRoot = filepath.Join(t.TempDir(), "missing-codex-root")
	observation, err := (NativeIdentityObserver{}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCodex}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityAbsent {
		t.Fatalf("qualified observation = %+v, err = %v", observation, err)
	}

	writeIdentityFile(t, filepath.Join(root, "unqualified", "plugin.json"), `{"name":"demo"}`)
	observation, err = (NativeIdentityObserver{}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCodex}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("unqualified observation = %+v, err = %v", observation, err)
	}
}

func TestNativeIdentityCodexUsesExactCLIRegistryIdentity(t *testing.T) {
	plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
	plan.NativeRegistryExecutable = "/test/bin/codex"
	marketplace := managedMarketplaceName(plan.PhysicalArtifactID)
	runner := &identityRunner{result: legacyports.CommandResult{Stdout: []byte(`{"installed":[{"pluginId":"demo@foreign","name":"demo","marketplaceName":"foreign","installed":true,"enabled":true}]}`)}}
	observation, err := (NativeIdentityObserver{Runner: runner}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCodex}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityAbsent {
		t.Fatalf("foreign namespace observation = %+v, err = %v", observation, err)
	}
	if !reflect.DeepEqual(runner.commands, [][]string{{"/test/bin/codex", "plugin", "list", "--json"}}) {
		t.Fatalf("commands = %#v", runner.commands)
	}

	runner.result.Stdout = []byte(`{"installed":[{"pluginId":"demo@` + marketplace + `","name":"demo","marketplaceName":"` + marketplace + `","installed":true,"enabled":true}]}`)
	observation, err = (NativeIdentityObserver{Runner: runner}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCodex}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("occupied managed namespace observation = %+v, err = %v", observation, err)
	}
}

// TestNativeIdentityCodexAbsentRecoveryMatchesRealCLIFailureThenSucceedsAfterRestoration
// reproduces run05's exact defect against the real production observer: Codex's
// own `plugin list --json` fails outright (its configured local marketplace
// source is absent) before the managed directory is reconstructed, then
// succeeds once the directory exists again. The caller (usecase/group.go)
// relies on ObservePreparedIdentity never invoking this failing command and on
// ObserveNativeIdentity's error carrying a bounded, sanitized excerpt of the
// CLI's own output for diagnosis.
func TestNativeIdentityCodexAbsentRecoveryMatchesRealCLIFailureThenSucceedsAfterRestoration(t *testing.T) {
	t.Parallel()
	plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
	plan.NativeRegistryExecutable = "/test/bin/codex"
	marketplace := managedMarketplaceName(plan.PhysicalArtifactID)
	managed := &domain.ClientBinding{TargetLocator: plan.ActivePath, NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "sha256:owned"}}}
	client := domain.DetectedClient{ClientID: domain.ClientCodex}

	failingRunner := &identityRunner{result: legacyports.CommandResult{ExitCode: 1, Stderr: []byte("Error: failed to load marketplace snapshot for " + marketplace + "\n")}}
	observer := NativeIdentityObserver{Runner: failingRunner, Stager: acceptingPackageVerifier{}}

	// The recovery-eligibility gate (usecase's preparedIdentityObservation) must
	// never invoke this failing command at all.
	prepared, err := observer.ObservePreparedIdentity(context.Background(), client, plan, managed)
	if err != nil || prepared.State != domain.NativeIdentityAbsent {
		t.Fatalf("prepared observation = %+v, err = %v", prepared, err)
	}
	if len(failingRunner.commands) != 0 {
		t.Fatalf("prepared observation launched the native CLI: %v", failingRunner.commands)
	}

	// The full, CLI-inclusive check must surface the CLI's own failure with a
	// bounded diagnostic excerpt, and must not be mistaken for proof of absence.
	_, err = observer.ObserveNativeIdentity(context.Background(), client, plan, managed)
	if err == nil {
		t.Fatal("failing native registry command was not surfaced as an error")
	}
	if !strings.Contains(err.Error(), "exit code 1") || !strings.Contains(err.Error(), marketplace) {
		t.Fatalf("native discovery error missing bounded diagnostic: %v", err)
	}

	// Once the directory is reconstructed and the CLI reports it, the same
	// observer call must confirm managed ownership with the recorded digest.
	writeIdentityFile(t, filepath.Join(plan.ActivePath, "plugin.json"), `{"name":"demo"}`)
	failingRunner.result = legacyports.CommandResult{Stdout: []byte(`{"installed":[{"pluginId":"demo@` + marketplace + `","name":"demo","marketplaceName":"` + marketplace + `","installed":true,"enabled":true}]}`)}
	observation, err := observer.ObserveNativeIdentity(context.Background(), client, plan, managed)
	if err != nil || observation.State != domain.NativeIdentityManaged || observation.Digest != "sha256:owned" {
		t.Fatalf("post-restoration observation = %+v, err = %v", observation, err)
	}
}

func TestPreparedIdentityInspectsFilesWithoutLaunchingNativeCLI(t *testing.T) {
	root := filepath.Join(t.TempDir(), "prepared")
	plan := identityPlan(root)
	plan.NativeRegistryExecutable = "/test/bin/codex"
	writeIdentityFile(t, filepath.Join(root, "foreign", "plugin.json"), `{"name":"demo"}`)
	runner := &identityRunner{}

	observation, err := (NativeIdentityObserver{Runner: runner}).ObservePreparedIdentity(
		context.Background(), domain.DetectedClient{ClientID: domain.ClientCodex}, plan, nil,
	)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("observation = %+v, err = %v", observation, err)
	}
	if len(runner.commands) != 0 || observation.NativeDiscoveryAttempted {
		t.Fatalf("prepared observation launched native discovery: commands=%#v observation=%+v", runner.commands, observation)
	}
}

func TestNativeIdentityCodexManualModeReadsAuthoritativeConfig(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), ".codex")
	plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
	plan.NativeRegistryRoot = configRoot
	marketplace := managedMarketplaceName(plan.PhysicalArtifactID)
	writeIdentityFile(t, filepath.Join(configRoot, "config.toml"), "[plugins.\"demo@"+marketplace+"\"]\nenabled = true\n")

	observation, err := (NativeIdentityObserver{}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCodex}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("observation = %+v, err = %v", observation, err)
	}
}

func TestNativeIdentityCopilotAndVSCodeUseSharedAuthoritativeBackend(t *testing.T) {
	for _, clientID := range []domain.ClientID{domain.ClientCopilot, domain.ClientVSCode} {
		t.Run(string(clientID), func(t *testing.T) {
			plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
			plan.NativeRegistryExecutable = "/test/bin/copilot"
			marketplace := managedMarketplaceName(plan.PhysicalArtifactID)
			runner := &identityRunner{result: legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  • demo@" + marketplace + " (v1.0.0)\n")}}
			observation, err := (NativeIdentityObserver{Runner: runner}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: clientID}, plan, nil)
			if err != nil || observation.State != domain.NativeIdentityUnmanaged {
				t.Fatalf("observation = %+v, err = %v", observation, err)
			}
			if !reflect.DeepEqual(runner.commands, [][]string{{"/test/bin/copilot", "plugin", "list"}}) {
				t.Fatalf("commands = %#v", runner.commands)
			}
		})
	}
}

func TestCopilotRegistryFindingRequiresExactNativeIdentity(t *testing.T) {
	marketplace := "agentplugins-demo-0123456789ab"
	tests := []struct {
		name string
		body string
		want registryFinding
	}{
		{name: "exact", body: "Installed plugins:\n  • demo@" + marketplace + " (v1.0.0)\n", want: registryExpected},
		{name: "wrong_version", body: "Installed plugins:\n  • demo@" + marketplace + " (v1.0.1)\n", want: registryIndeterminate},
		{name: "unrelated", body: "Installed plugins:\n  • other@" + marketplace + " (v1.0.0)\n", want: registryClear},
		{name: "duplicate", body: "Installed plugins:\n  • demo@" + marketplace + " (v1.0.0)\n  • demo@" + marketplace + " (v1.0.0)\n", want: registryIndeterminate},
		{name: "absent_short", body: "No plugins installed.\n", want: registryIndeterminate},
		{name: "absent_official_1_0_80", body: "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\n", want: registryClear},
		{name: "absent_official_with_prefix", body: "\nNo plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\n", want: registryIndeterminate},
		{name: "absent_official_with_suffix", body: "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\n\n", want: registryIndeterminate},
		{name: "unknown", body: "Copilot plugins are unavailable right now\n", want: registryIndeterminate},
		{name: "unknown_prefix", body: "experimental output\nInstalled plugins:\n  • demo@" + marketplace + " (v1.0.0)\n", want: registryIndeterminate},
		{name: "unknown_suffix", body: "Installed plugins:\n  • demo@" + marketplace + " (v1.0.0)\nupdate available\n", want: registryIndeterminate},
		{name: "unknown_unindented_entry", body: "Installed plugins:\ndemo@" + marketplace + " (v1.0.0)\n", want: registryIndeterminate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := copilotRegistryFinding([]byte(test.body), "demo", marketplace, "1.0.0", true); got != test.want {
				t.Fatalf("finding = %d, want %d", got, test.want)
			}
		})
	}
}

func TestCopilotLiveRegistryFindingRequiresExactIdentityStatusAndPath(t *testing.T) {
	t.Parallel()
	marketplace := "agentplugins-demo-0123456789ab"
	path := filepath.Join(t.TempDir(), "managed", "demo")
	header := copilotLiveHeader + "\n"
	entry := func(name, version, status, from string) string {
		return header + "  • " + name + "@" + marketplace + " (v" + version + ") (" + status + ")\n      from " + from + "\n"
	}
	tests := []struct {
		name  string
		body  string
		owned bool
		want  registryFinding
	}{
		{name: "managed", body: entry("demo", "1.7.0-uap.1", "enabled", path), owned: true, want: registryExpected},
		{name: "managed with unrelated", body: entry("other", "9.9.9", "enabled", path) + strings.TrimPrefix(entry("demo", "1.7.0-uap.1", "enabled", path), header), owned: true, want: registryExpected},
		{name: "managed CRLF", body: strings.ReplaceAll(entry("demo", "1.7.0-uap.1", "enabled", path), "\n", "\r\n"), owned: true, want: registryExpected},
		{name: "unowned collision", body: entry("demo", "1.7.0-uap.1", "enabled", path), want: registryCollision},
		{name: "unrelated", body: entry("other", "9.9.9", "enabled", path), owned: true, want: registryClear},
		{name: "disabled", body: entry("demo", "1.7.0-uap.1", "disabled", path), owned: true, want: registryIndeterminate},
		{name: "wrong version", body: entry("demo", "1.7.0-uap.0", "enabled", path), owned: true, want: registryIndeterminate},
		{name: "wrong path", body: entry("demo", "1.7.0-uap.1", "enabled", filepath.Join(t.TempDir(), "other")), owned: true, want: registryIndeterminate},
		{name: "dot segment path", body: entry("demo", "1.7.0-uap.1", "enabled", filepath.Dir(path)+string(filepath.Separator)+"alias"+string(filepath.Separator)+".."+string(filepath.Separator)+filepath.Base(path)), owned: true, want: registryIndeterminate},
		{name: "duplicate", body: entry("demo", "1.7.0-uap.1", "enabled", path) + strings.TrimPrefix(entry("demo", "1.7.0-uap.1", "enabled", path), header), owned: true, want: registryIndeterminate},
		{name: "missing from", body: header + "  • demo@" + marketplace + " (v1.7.0-uap.1) (enabled)\n", owned: true, want: registryIndeterminate},
		{name: "extra suffix", body: entry("demo", "1.7.0-uap.1", "enabled", path) + "unexpected\n", owned: true, want: registryIndeterminate},
		{name: "ansi prefix", body: "\x1b[32m" + entry("demo", "1.7.0-uap.1", "enabled", path), owned: true, want: registryIndeterminate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := copilotRegistryFindingAt([]byte(test.body), "demo", marketplace, "1.7.0-uap.1", path, test.owned); got != test.want {
				t.Fatalf("finding=%d want=%d", got, test.want)
			}
		})
	}
}

func TestNativeIdentityCopilotAcceptsExactLiveManagedRegistration(t *testing.T) {
	t.Parallel()
	plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
	plan.DeclaredVersion = "1.7.0-uap.1"
	plan.NativeRegistryExecutable = "/test/bin/copilot"
	if err := os.MkdirAll(plan.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeIdentityFile(t, filepath.Join(plan.ActivePath, "plugin.json"), `{"name":"demo"}`)
	marketplace := managedMarketplaceName(plan.PhysicalArtifactID)
	managed := &domain.ClientBinding{TargetLocator: plan.ActivePath, NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "sha256:owned"}}}
	listing := copilotLiveHeader + "\n  • demo@" + marketplace + " (v1.7.0-uap.1) (enabled)\n      from " + plan.ActivePath + "\n"
	runner := &identityRunner{result: legacyports.CommandResult{Stdout: []byte(listing)}}
	observation, err := (NativeIdentityObserver{Runner: runner, Stager: acceptingPackageVerifier{}}).ObserveNativeIdentity(
		context.Background(), domain.DetectedClient{ClientID: domain.ClientCopilot}, plan, managed,
	)
	if err != nil || observation.State != domain.NativeIdentityManaged || !observation.NativeDiscoveryReconciled {
		t.Fatalf("observation=%+v err=%v", observation, err)
	}
}

func TestNativeIdentityObservationExposesReceiptAndExactDiscovery(t *testing.T) {
	plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
	plan.NativeRegistryExecutable = "/test/bin/copilot"
	if err := os.MkdirAll(plan.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeIdentityFile(t, filepath.Join(plan.ActivePath, "plugin.json"), `{"name":"demo"}`)
	marketplace := managedMarketplaceName(plan.PhysicalArtifactID)
	digest := "sha256:owned"
	managed := &domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: digest}}}
	runner := &identityRunner{result: legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  • demo@" + marketplace + " (v1.0.0)\n")}}
	observation, err := (NativeIdentityObserver{Runner: runner, Stager: acceptingPackageVerifier{}}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCopilot}, plan, managed)
	if err != nil || observation.State != domain.NativeIdentityManaged || !observation.ReceiptReconciled || !observation.NativeDiscoveryReconciled || observation.NativeDiscoveryState != domain.NativeIdentityManaged {
		t.Fatalf("observation = %+v, err = %v", observation, err)
	}
}

func TestNativeIdentityBoundsHungAuthoritativeDiscovery(t *testing.T) {
	plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
	plan.NativeRegistryExecutable = "/test/bin/copilot"
	returned := make(chan struct{})
	observation, err := (NativeIdentityObserver{
		Runner: blockingIdentityRunner{returned: returned}, DiscoveryTimeout: 10 * time.Millisecond,
	}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCopilot}, plan, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	if observation.State != domain.NativeIdentityIndeterminate || observation.NativeDiscoveryState != domain.NativeIdentityIndeterminate ||
		!observation.NativeDiscoveryAttempted || observation.NativeDiscoveryReconciled {
		t.Fatalf("observation = %+v", observation)
	}
	select {
	case <-returned:
	default:
		t.Fatal("hung discovery runner remained active after timeout")
	}
}

func TestNativeIdentityPreservesNativeDiscoveryWhenPreparedRegistryBlocksOverallIdentity(t *testing.T) {
	for _, test := range []struct {
		name      string
		manifest  string
		wantState domain.NativeIdentityState
		wantError bool
	}{
		{name: "collision", manifest: `{"name":"demo"}`, wantState: domain.NativeIdentityUnmanaged},
		{name: "malformed", manifest: `{"name":`, wantState: domain.NativeIdentityIndeterminate, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
			plan.NativeRegistryExecutable = "/test/bin/copilot"
			writeIdentityFile(t, filepath.Join(plan.TargetRoot, "foreign", "plugin.json"), test.manifest)
			marketplace := managedMarketplaceName(plan.PhysicalArtifactID)
			managed := &domain.ClientBinding{}
			runner := &identityRunner{result: legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  • demo@" + marketplace + " (v1.0.0)\n")}}

			observation, err := (NativeIdentityObserver{Runner: runner}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCopilot}, plan, managed)
			if (err != nil) != test.wantError {
				t.Fatalf("err = %v, want error %t", err, test.wantError)
			}
			if observation.State != test.wantState || !observation.NativeDiscoveryAttempted || !observation.NativeDiscoveryReconciled || observation.NativeDiscoveryState != domain.NativeIdentityManaged {
				t.Fatalf("observation = %+v", observation)
			}
			if !reflect.DeepEqual(runner.commands, [][]string{{"/test/bin/copilot", "plugin", "list"}}) {
				t.Fatalf("commands = %#v", runner.commands)
			}
		})
	}
}

type acceptingPackageVerifier struct{}

func (acceptingPackageVerifier) Verify(context.Context, string, string) error { return nil }

func TestNativeIdentityManualRemoteAndUnknownSharedBackendsFailClosed(t *testing.T) {
	plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
	for _, test := range []struct {
		client domain.ClientID
		root   string
	}{
		{client: domain.ClientChatGPT},
		{client: domain.ClientCopilot, root: filepath.Join(t.TempDir(), ".copilot")},
		{client: domain.ClientVSCode, root: filepath.Join(t.TempDir(), ".copilot")},
	} {
		if test.root != "" {
			if err := os.MkdirAll(test.root, 0700); err != nil {
				t.Fatal(err)
			}
		}
		plan.NativeRegistryRoot = test.root
		observation, err := (NativeIdentityObserver{}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: test.client}, plan, nil)
		if err != nil || observation.State != domain.NativeIdentityIndeterminate {
			t.Fatalf("%s observation = %+v, err = %v", test.client, observation, err)
		}
	}
}

func TestNativeIdentityKiroReadsGlobalMCPRegistry(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), ".kiro")
	writeIdentityFile(t, filepath.Join(configRoot, "settings", "mcp.json"), `{"mcpServers":{"docs":{"command":"foreign"}}}`)
	plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
	plan.NativeRegistryRoot = configRoot
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportNative}}
	observation, err := (NativeIdentityObserver{}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientKiro}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("observation = %+v, err = %v", observation, err)
	}
}

func TestNativeIdentityKiroManualPowerAuthorizesOnlyLocalPreparation(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), ".kiro")
	plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
	plan.NativeRegistryRoot = configRoot
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative}}

	observation, err := (NativeIdentityObserver{}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientKiro}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityAbsent {
		t.Fatalf("observation = %+v, err = %v", observation, err)
	}
}

func identityPlan(root string) domain.DeliveryPlan {
	artifact := "demo-0123456789ab"
	return domain.DeliveryPlan{
		DeclaredName:       "demo",
		DeclaredVersion:    "1.0.0",
		PhysicalArtifactID: artifact,
		TargetRoot:         root,
		ActivePath:         filepath.Join(root, artifact),
	}
}

func writeIdentityFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
