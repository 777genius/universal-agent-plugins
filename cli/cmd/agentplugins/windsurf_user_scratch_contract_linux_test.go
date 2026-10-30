//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type windUserScratchDetector struct{ profile string }

func (d windUserScratchDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientWindsurf, ConfigRoot: d.profile, Status: domain.DetectionDetected}}, nil
}

func windUserScratchApp(app agentpluginscli.App, profile, state string) agentpluginscli.App {
	app.Detector = windUserScratchDetector{profile}
	app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
	app.DirectoryClientForDryRun, app.CommitDirectoryPreview = nil, nil
	app.SecurityEvaluator = projectJourneySecurity{}
	app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return projectJourneySecurity{} }
	app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: state}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return windUserScratchApp(selected, profile, state), nil
	}
	return app
}

func TestWindsurfPublicUserAddUsesDefaultSnapshotScratch(t *testing.T) {
	root := projectJourneyTemp(t)
	home, state, scratch, source := filepath.Join(root, "home"), filepath.Join(root, "installer-state"), filepath.Join(root, "tmp"), filepath.Join(root, "source")
	profile := filepath.Join(home, ".codeium", "windsurf")
	for _, path := range []string{state, scratch} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"TMPDIR", "TEMP", "TMP"} {
		t.Setenv(name, scratch)
	}
	config := filepath.Join(profile, "mcp_config.json")
	projectJourneyWrite(t, config, `{"mcpServers":{"foreign-user":{"serverUrl":"http://127.0.0.1:9/foreign-user","disabled":true}},"foreignPreference":"keep"}`)
	lock := config + ".agentplugins.lock"
	projectJourneyWrite(t, lock, "")
	foreignSkill := filepath.Join(profile, "skills", "foreign-user", "SKILL.md")
	projectJourneyWrite(t, foreignSkill, "foreign User skill; never executed\n")
	projectJourneyWrite(t, filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"uap-windsurf-component","version":"1.0.0"}`)
	projectJourneyWrite(t, filepath.Join(source, "skills", "uap-windsurf-component", "SKILL.md"), "---\nname: uap-windsurf-component\ndescription: Inert scratch reproduction\n---\nNever executed\n")
	projectJourneyWrite(t, filepath.Join(source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"uap-windsurf-component":{"type":"streamable-http","url":"http://127.0.0.1:9/no-execution"}}}`)
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(home, state, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	app = windUserScratchApp(app, profile, state)
	windUserScratchExecute(t, app, source, scratch)
	var document struct {
		Servers    map[string]json.RawMessage `json:"mcpServers"`
		Preference string                     `json:"foreignPreference"`
	}
	raw, err := os.ReadFile(config)
	if err != nil || json.Unmarshal(raw, &document) != nil || len(document.Servers) != 2 || document.Preference != "keep" {
		t.Fatal("ordinary User add native config/foreign preservation", err)
	}
	var foreign map[string]any
	if json.Unmarshal(document.Servers["foreign-user"], &foreign) != nil || !reflect.DeepEqual(foreign, map[string]any{"serverUrl": "http://127.0.0.1:9/foreign-user", "disabled": true}) || !bytes.Contains(document.Servers["uap-windsurf-component"], []byte("http://127.0.0.1:9/no-execution")) {
		t.Fatal("selected native server or original foreign server differs")
	}
	if projectJourneyRead(t, foreignSkill) != "foreign User skill; never executed\n" {
		t.Fatal("foreign skill changed")
	}
}

func windUserScratchExecute(t *testing.T, app agentpluginscli.App, source, scratch string) {
	t.Helper()
	lock := filepath.Join(app.UserHome, ".codeium", "windsurf", "mcp_config.json.agentplugins.lock")
	before, err := os.Lstat(scratch)
	if err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.Lstat(lock)
	if err != nil {
		t.Fatal(err)
	}
	var output, stderr bytes.Buffer
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), &output, &stderr, false
	command := agentpluginscli.NewRoot(app)
	command.SetArgs([]string{"add", source, "--no-save", "--scope", "user", "--target", "windsurf", "--format", "json"})
	time.Sleep(20 * time.Millisecond)
	if err := command.Execute(); err != nil {
		t.Fatalf("ordinary User add: %v\n%s\n%s", err, output.String(), stderr.String())
	}
	var result map[string]any
	if json.Unmarshal(output.Bytes(), &result) != nil || result["command"] != "add" || result["result"] != "success" {
		t.Fatal("public add result differs", output.String())
	}
	after, err := os.Lstat(scratch)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		t.Fatal("ordinary User add scratch authority/membership differs", err)
	}
	oldStat, oldOK := before.Sys().(*syscall.Stat_t)
	newStat, newOK := after.Sys().(*syscall.Stat_t)
	if !oldOK || !newOK || oldStat.Uid != newStat.Uid || oldStat.Gid != newStat.Gid || (oldStat.Mtim == newStat.Mtim && oldStat.Ctim == newStat.Ctim) {
		t.Fatal("actual User scratch timestamp-only effect differs")
	}
	lockAfter, err := os.Lstat(lock)
	if err != nil || !os.SameFile(lockBefore, lockAfter) || lockAfter.Mode() != 0600 || lockAfter.Size() != 0 {
		t.Fatal("existing native lock identity changed", err)
	}
	t.Logf("actual User scratch: size_equal=%t modified_equal=%t changed_equal=%t", before.Size() == after.Size(), oldStat.Mtim == newStat.Mtim, oldStat.Ctim == newStat.Ctim)
}
