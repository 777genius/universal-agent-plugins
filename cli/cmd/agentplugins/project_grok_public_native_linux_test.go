//go:build linux && grokpublicnative

package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var (
	grokPublicNative    = flag.String("grok-public-native", "", "original pinned Grok executable")
	grokPublicInventory = flag.String("grok-public-inventory-test", "", "compiled production readonly metadata test")
)

const grokNativeForeignConfig = "# retained foreign settings\n[marketplace]\ndefault_skills_installs_purged=true\n[features]\nturn_summary=false\n[plugins]\nenabled=['foreign-enabled']\ndisabled=['foreign-disabled']\n"

// This tagged acceptance uses the production composer, registry, process runner,
// lifecycle, transactions and source codecs. Only the disposable ZIP origin's CA
// is supplied explicitly; no native response or process is replaced by a fixture.
func grokNativePublicApp(t *testing.T, journey projectJourney) agentpluginscli.App {
	t.Helper()
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(journey.home, journey.cache, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	var configure func(agentpluginscli.App) agentpluginscli.App
	configure = func(app agentpluginscli.App) agentpluginscli.App {
		app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
		app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: journey.cache, HTTPClient: journey.transport.server.Client(), Runner: sourceacquisition.OSRunner{}}
		app.SourceAcquirerForDryRun = func(root string) agentpluginscli.SourceAcquirer {
			return sourceacquisition.Acquirer{TempRoot: root, HTTPClient: journey.transport.server.Client(), Runner: sourceacquisition.OSRunner{}}
		}
		compose := app.ForScope
		app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
			scoped, err := compose(scope, root)
			if err != nil {
				return scoped, err
			}
			return configure(scoped), nil
		}
		return app
	}
	return configure(app)
}

func grokNativeCommand(t *testing.T, home, project string, argv ...string) []byte {
	t.Helper()
	result, err := (process.OS{}).Run(t.Context(), legacyports.Command{Argv: argv, Dir: project,
		Env: []string{"HOME=" + home, "GROK_DISABLE_AUTOUPDATER=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}, StdoutLimitBytes: 1 << 20})
	if err != nil || result.ExitCode != 0 || !result.CleanupComplete || len(result.Stderr) != 0 {
		t.Fatalf("real native command %v: %v exit=%d cleanup=%t\n%s\n%s", argv, err, result.ExitCode, result.CleanupComplete, result.Stdout, result.Stderr)
	}
	return result.Stdout
}

func newGrokNativePublicJourney(t *testing.T, transport *projectJourneyTransport) projectJourney {
	t.Helper()
	journey := projectJourney{home: projectJourneyTemp(t), cache: projectJourneyTemp(t), a: projectJourneyTemp(t), b: projectJourneyTemp(t), transport: transport}
	for _, project := range []string{journey.a, journey.b} {
		projectJourneyGit(t, project, "init", "--quiet")
	}
	for _, name := range []string{"HOME", "USERPROFILE"} {
		t.Setenv(name, journey.home)
	}
	for _, name := range []string{"CONFIG", "DATA", "CACHE", "STATE"} {
		t.Setenv("XDG_"+name+"_HOME", filepath.Join(journey.home, ".xdg-"+strings.ToLower(name)))
	}
	t.Setenv("PATH", filepath.Dir(*grokPublicNative)+":/usr/bin:/bin")
	t.Setenv("GROK_DISABLE_AUTOUPDATER", "1")
	config := filepath.Join(journey.home, ".grok", "config.toml")
	projectJourneyWrite(t, config, "[mcp_servers.uap_inert_lock]\ncommand='/usr/bin/false'\nenabled=false\n")
	grokNativeCommand(t, journey.home, journey.a, *grokPublicNative, "mcp", "disable", "uap_inert_lock")
	projectJourneyWrite(t, config, grokNativeForeignConfig)
	projectJourneyWrite(t, filepath.Join(journey.home, ".grok", "keep-user"), "original User sentinel\n")
	projectJourneyWrite(t, filepath.Join(journey.home, ".grok", "plugins", "foreign-disabled", ".grok-plugin", "plugin.json"), `{"name":"foreign-disabled","version":"1.0.0","description":"Inert foreign disabled User plugin"}`)
	projectJourneyWrite(t, filepath.Join(journey.b, "foreign-sentinel"), "independent project B\n")
	journey.app = grokNativePublicApp(t, journey)
	return journey
}

func grokNativePublicInventory(t *testing.T, journey projectJourney, binding domain.ClientBinding, enabled bool) {
	t.Helper()
	// Execute the existing production readonly reader, retaining its two exact
	// identity/cleanup checks. No parallel protocol client is introduced here.
	output := grokNativeCommand(t, journey.home, journey.a, *grokPublicInventory,
		"-test.run=^TestEnablementReadonlyNativeInventory$", "-test.v", "-test.count=1", "-test.timeout=40s",
		"-grok-readonly-home="+journey.home, "-grok-readonly-project="+journey.a,
		"-grok-readonly-native="+*grokPublicNative, "-grok-readonly-path="+binding.TargetLocator,
		"-grok-readonly-name=project-demo", "-grok-readonly-version="+binding.PackageRevision.Version,
		fmt.Sprintf("-grok-readonly-enabled=%t", enabled))
	if !strings.Contains(string(output), "GROK_READONLY_NATIVE_INVENTORY_PASS\n") {
		t.Fatal("native inventory test did not complete", string(output))
	}
}

func grokNativePublicRun(t *testing.T, journey projectJourney, project string, args ...string) string {
	t.Helper()
	output := journey.run(t, project, args...)
	t.Logf("GROK_PUBLIC_NATIVE_COMMAND args=%q project=%q\n%s", args, project, output)
	return output
}

func grokNativeEnabledObject(t *testing.T, binding domain.ClientBinding) domain.NativeObjectOwnership {
	t.Helper()
	var result domain.NativeObjectOwnership
	for _, object := range binding.NativeObjects {
		if object.Kind == "grok_enabled_plugin" {
			if result.ObjectID != "" {
				t.Fatal("duplicate native membership ownership")
			}
			result = object
		}
	}
	if result.ObjectID == "" || binding.NativeActivationAttempt != "" || binding.Activation != domain.ActivationPrepared {
		t.Fatalf("native enablement lacks settled Prepared ownership: %+v", binding)
	}
	return result
}

type grokNativeDataAuthority struct {
	installationID string
	receipt        domain.DataReceipt
	directory      os.FileInfo
}

func grokNativeCaptureData(t *testing.T, journey projectJourney, installation domain.Installation, binding domain.ClientBinding) grokNativeDataAuthority {
	t.Helper()
	receipt, ok := installation.DataReceipts[binding.DataReceiptID]
	expected := filepath.Join(journey.a, ".uap", "plugin-data", binding.PhysicalArtifact)
	if !ok || len(installation.DataReceipts) != 1 || receipt.DataReceiptID == "" || receipt.State != domain.DataReceiptOwned || receipt.Scope != "project" || receipt.PhysicalBackend != binding.PhysicalArtifact || receipt.Locator != expected {
		t.Fatalf("native stdio lost exact persistent data authority: %+v", receipt)
	}
	info, err := os.Stat(receipt.Locator)
	if err != nil || !info.IsDir() {
		t.Fatal("native data directory missing", err)
	}
	projectJourneyWrite(t, filepath.Join(receipt.Locator, "native-user-data"), "retain actual native User data\n")
	return grokNativeDataAuthority{installation.InstallationID, receipt, info}
}

func (data grokNativeDataAuthority) check(t *testing.T, journey projectJourney, removed bool) {
	t.Helper()
	state := journey.state(t, journey.a)
	found := false
	for _, installation := range state.Installations {
		if installation.InstallationID != data.installationID {
			continue
		}
		found = true
		if installation.DataReceipts[data.receipt.DataReceiptID] != data.receipt || removed && (len(installation.Clients) != 0 || !installation.DataRetained) {
			t.Fatal("persistent native receipt or retention changed", installation)
		}
	}
	info, err := os.Stat(data.receipt.Locator)
	if !found || err != nil || !os.SameFile(data.directory, info) || projectJourneyRead(t, filepath.Join(data.receipt.Locator, "native-user-data")) != "retain actual native User data\n" {
		t.Fatal("persistent native data was replaced or lost", err)
	}
}

func TestProjectGrokNativeSavedEnablementLifecycle(t *testing.T) {
	for _, path := range []string{*grokPublicNative, *grokPublicInventory} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			t.Fatal("original absolute native inputs required")
		}
	}
	nativeBytes, err := os.ReadFile(*grokPublicNative)
	if err != nil {
		t.Fatal(err)
	}
	originalDigest := fmt.Sprintf("%x", sha256.Sum256(nativeBytes))
	if originalDigest != "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9" {
		t.Fatal("original pinned Grok binary differs")
	}
	transport := newProjectJourneyTransport(t, "zip")
	// Keep MCP prepared and untrusted: this test never executes a tool or grants
	// trust. Existing public source/stdio tests own their separate runtime proof.
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), grokNativePreparedMCP)
	transport.mu.Lock()
	transport.archive = projectJourneyArchive(t, transport.repo)
	transport.mu.Unlock()
	sourceInitial := securityFixtureTree(t, transport.repo)
	journey := newGrokNativePublicJourney(t, transport)
	if string(grokNativeCommand(t, journey.home, journey.a, *grokPublicNative, "--version")) != "grok 1.0.70 (f50904eaee6c)\n" {
		t.Fatal("native version differs")
	}
	config := filepath.Join(journey.home, ".grok", "config.toml")
	bBefore := securityFixtureTree(t, journey.b)
	grokNativePublicRun(t, journey, journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
	installation, binding := journey.owned(t, journey.a)
	data := grokNativeCaptureData(t, journey, installation, binding)
	userForeign := securityFixtureTree(t, filepath.Join(journey.home, ".grok", "plugins", "foreign-disabled"))
	grokNativePublicInventory(t, journey, binding, false)
	grokNativePublicRun(t, journey, journey.a, "repair", installation.InstallationID, "--target", "grok", "--yes")
	_, binding = journey.owned(t, journey.a)
	configBefore, stateBefore := projectJourneyRead(t, config), projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json"))
	grokNativePublicRun(t, journey, journey.a, "repair", installation.InstallationID, "--target", "grok", "--confirm-plugin-enable", "--dry-run", "--yes")
	if projectJourneyRead(t, config) != configBefore || projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json")) != stateBefore {
		t.Fatal("enablement preview changed original config or state")
	}
	output := grokNativePublicRun(t, journey, journey.a, "repair", installation.InstallationID, "--target", "grok", "--confirm-plugin-enable", "--yes")
	if !strings.Contains(output, "selected Grok project plugin is enabled") {
		t.Fatal("public result lacks native enablement readback", output)
	}
	_, enabled := journey.owned(t, journey.a)
	owned := grokNativeEnabledObject(t, enabled)
	grokNativePublicInventory(t, journey, enabled, true)
	enabledConfig := projectJourneyRead(t, config)
	grokNativePublicRun(t, journey, journey.a, "repair", installation.InstallationID, "--target", "grok", "--confirm-plugin-enable", "--yes")
	_, repeated := journey.owned(t, journey.a)
	if grokNativeEnabledObject(t, repeated) != owned || projectJourneyRead(t, config) != enabledConfig {
		t.Fatal("repeated enable changed settled member")
	}
	if !reflect.DeepEqual(sourceInitial, securityFixtureTree(t, transport.repo)) {
		t.Fatal("native install or enablement changed the portable source")
	}
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	transport.mu.Lock()
	transport.archive = projectJourneyArchive(t, transport.repo)
	transport.mu.Unlock()
	sourceUpdated := securityFixtureTree(t, transport.repo)
	grokNativePublicRun(t, journey, journey.a, "update", "docs", "--target", "grok")
	updated, current := journey.owned(t, journey.a)
	data.check(t, journey, false)
	if updated.Source.TreeDigest == installation.Source.TreeDigest || current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator || grokNativeEnabledObject(t, current) != owned {
		t.Fatal("update lost exact source, binding or native member")
	}
	grokNativePublicRun(t, journey, journey.a, "repair", installation.InstallationID, "--target", "grok", "--yes")
	if err := os.RemoveAll(current.TargetLocator); err != nil {
		t.Fatal(err)
	}
	grokNativePublicRun(t, journey, journey.a, "repair", installation.InstallationID, "--target", "grok", "--yes")
	_, repaired := journey.owned(t, journey.a)
	data.check(t, journey, false)
	if grokNativeEnabledObject(t, repaired) != owned || projectJourneyRead(t, config) != enabledConfig || projectJourneyRead(t, filepath.Join(current.TargetLocator, "skills", "docs", "SKILL.md")) != projectJourneyAdvancedSkill {
		t.Fatal("exact absent-package repair lost member or bytes")
	}
	grokNativePublicInventory(t, journey, repaired, true)
	// A cold portable import installs Prepared files in a fresh profile. It must
	// not inherit an enabled member from the exporting machine's User config.
	exchange := projectJourneyTemp(t)
	grokNativePublicRun(t, journey, journey.a, "deps", "export", exchange, "--yes")
	pair := journey.assertPortablePair(t, exchange)
	cold := newGrokNativePublicJourney(t, transport)
	coldB := securityFixtureTree(t, cold.b)
	grokNativePublicRun(t, cold, cold.a, "deps", "import", exchange, "--install", "--yes")
	restored, coldBinding := cold.owned(t, cold.a)
	coldData := grokNativeCaptureData(t, cold, restored, coldBinding)
	if coldData.receipt.Locator == data.receipt.Locator || os.SameFile(coldData.directory, data.directory) {
		t.Fatal("cold restore reused exporting environment data")
	}
	coldForeign := securityFixtureTree(t, filepath.Join(cold.home, ".grok", "plugins", "foreign-disabled"))
	if restored.Source.SourceBindingID != updated.Source.SourceBindingID || restored.Source.TreeDigest != updated.Source.TreeDigest || pair != cold.assertPortablePair(t, cold.a) {
		t.Fatal("cold locked import changed pinned source")
	}
	if projectJourneyRead(t, filepath.Join(cold.home, ".grok", "config.toml")) != grokNativeForeignConfig {
		t.Fatal("cold restore inherited native enablement")
	}
	grokNativePublicInventory(t, cold, coldBinding, false)
	grokNativePublicRun(t, cold, cold.a, "deps", "sync", "--locked", "--yes")
	grokNativePublicRun(t, cold, cold.a, "repair", restored.InstallationID, "--target", "grok", "--confirm-plugin-enable", "--yes")
	_, coldEnabled := cold.owned(t, cold.a)
	grokNativeEnabledObject(t, coldEnabled)
	grokNativePublicInventory(t, cold, coldEnabled, true)
	cold.app = grokNativePublicApp(t, cold)
	grokNativePublicRun(t, cold, cold.a, "deps", "sync", "--locked", "--yes")
	coldData.check(t, cold, false)
	coldConfig := filepath.Join(cold.home, ".grok", "config.toml")
	projectJourneyWrite(t, coldConfig, strings.Replace(projectJourneyRead(t, coldConfig), "'foreign-enabled'", "'foreign-enabled','later-foreign'", 1))
	grokNativePublicRun(t, cold, cold.a, "remove", "docs", "--target", "grok")
	coldData.check(t, cold, true)
	if !reflect.DeepEqual(coldForeign, securityFixtureTree(t, filepath.Join(cold.home, ".grok", "plugins", "foreign-disabled"))) || projectJourneyRead(t, filepath.Join(cold.home, ".grok", "keep-user")) != "original User sentinel\n" {
		t.Fatal("cold lifecycle changed foreign User plugin or sentinel")
	}
	if projectJourneyRead(t, coldConfig) != strings.Replace(grokNativeForeignConfig, "'foreign-enabled'", "'foreign-enabled','later-foreign'", 1) {
		t.Fatal("cold removal changed foreign membership")
	}
	projectJourneyAbsent(t, coldEnabled.TargetLocator)
	if !reflect.DeepEqual(coldB, securityFixtureTree(t, cold.b)) {
		t.Fatal("cold lifecycle changed Project B")
	}
	// Return to the original native process environment before its public remove.
	for _, name := range []string{"HOME", "USERPROFILE"} {
		t.Setenv(name, journey.home)
	}
	for _, name := range []string{"CONFIG", "DATA", "CACHE", "STATE"} {
		t.Setenv("XDG_"+name+"_HOME", filepath.Join(journey.home, ".xdg-"+strings.ToLower(name)))
	}
	journey.app = grokNativePublicApp(t, journey)
	projectJourneyWrite(t, config, strings.Replace(projectJourneyRead(t, config), "'foreign-enabled'", "'foreign-enabled','later-foreign'", 1))
	grokNativePublicRun(t, journey, journey.a, "remove", "docs", "--target", "grok")
	data.check(t, journey, true)
	if !reflect.DeepEqual(userForeign, securityFixtureTree(t, filepath.Join(journey.home, ".grok", "plugins", "foreign-disabled"))) || projectJourneyRead(t, filepath.Join(journey.home, ".grok", "keep-user")) != "original User sentinel\n" {
		t.Fatal("warm lifecycle changed foreign User plugin or sentinel")
	}
	projectJourneyAbsent(t, repaired.TargetLocator)
	if projectJourneyRead(t, config) != strings.Replace(grokNativeForeignConfig, "'foreign-enabled'", "'foreign-enabled','later-foreign'", 1) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) {
		t.Fatal("native removal changed foreign User settings or Project B")
	}
	if !reflect.DeepEqual(sourceUpdated, securityFixtureTree(t, transport.repo)) {
		t.Fatal("native update, repair or removal changed the portable source")
	}
	after, err := os.ReadFile(*grokPublicNative)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(after)) != originalDigest {
		t.Fatal("native binary changed", err)
	}
	t.Log("GROK_PUBLIC_NATIVE_SAVED_LIFECYCLE_PASS")
}
