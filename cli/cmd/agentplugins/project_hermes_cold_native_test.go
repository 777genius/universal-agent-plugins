package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestHermesColdDeadlineDoesNotRenewCallerBudget(t *testing.T) {
	now := time.UnixMilli(1794614400000)
	encoded := strconv.FormatInt(now.Add(90*time.Second).UnixMilli(), 10)
	hard := now.Add(6 * time.Minute)
	first, err := hermesColdDeadline(encoded, now, hard)
	if err != nil || !first.Equal(now.Add(time.Minute)) {
		t.Fatal("caller deadline lost its exact cleanup reserve", first, err)
	}
	later, err := hermesColdDeadline(encoded, now.Add(10*time.Second), hard)
	if err != nil || !later.Equal(first) {
		t.Fatal("elapsed setup renewed the caller budget", later, err)
	}
	shorter, err := hermesColdDeadline(encoded, now, now.Add(45*time.Second))
	if err != nil || !shorter.Equal(now.Add(15*time.Second)) {
		t.Fatal("test deadline did not retain its cleanup reserve", shorter, err)
	}
	for _, invalid := range []string{"", "+" + encoded, "0" + encoded, encoded + " ", "-1", "9999999999999999999",
		strconv.FormatInt(now.Add(30*time.Second).UnixMilli(), 10), strconv.FormatInt(now.Add(361*time.Second).UnixMilli(), 10)} {
		if _, err := hermesColdDeadline(invalid, now, hard); err == nil {
			t.Fatalf("invalid or exhausted caller deadline accepted: %q", invalid)
		}
	}
}

func TestHermesColdCallerOwnsFixtureRemoval(t *testing.T) {
	caller, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var owned string
	t.Run("completed-scope", func(t *testing.T) {
		for _, key := range []string{"TMPDIR", "TEMP", "TMP"} {
			t.Setenv(key, caller)
		}
		transport := &projectJourneyTransport{tempRoot: hermesColdDirectory(t)}
		owned = transport.temporary(t)
		projectJourneyWrite(t, filepath.Join(owned, "retained.txt"), "caller owns cleanup\n")
	})
	if filepath.Dir(owned) != caller || projectJourneyRead(t, filepath.Join(owned, "retained.txt")) != "caller owns cleanup\n" {
		t.Fatal("test cleanup removed or escaped a caller-owned native fixture")
	}
}

func TestHermesColdCancellationSettlesActualChild(t *testing.T) {
	const key = "UAP_HERMES_COLD_CANCEL_CONTRACT"
	if root := os.Getenv(key); root != "" {
		if err := os.WriteFile(filepath.Join(root, "ready"), []byte("ready"), 0600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Minute)
		return
	}
	root, err := os.MkdirTemp("", "uap-cold-cancel-")
	if err != nil {
		t.Fatal(err)
	}
	settled := false
	defer func() {
		if settled {
			if err := os.RemoveAll(root); err != nil {
				t.Error(err)
			}
		}
	}()
	executable, err := os.Executable()
	if err != nil {
		settled = true
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	type outcome struct {
		result legacyports.CommandResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := (processadapter.OS{}).Run(ctx, legacyports.Command{
			Argv: []string{executable, "-test.run=^TestHermesColdCancellationSettlesActualChild$"},
			Env:  append(os.Environ(), key+"="+root), StdoutLimitBytes: 4096,
		})
		done <- outcome{result, err}
	}()
	limit, poll := time.NewTimer(10*time.Second), time.NewTicker(10*time.Millisecond)
	defer limit.Stop()
	defer poll.Stop()
	ready := false
	for {
		select {
		case result := <-done:
			settled = result.result.CleanupComplete || result.result.NoChildStarted
			if !ready || !result.result.CleanupComplete || result.result.NoChildStarted || !errors.Is(result.err, context.Canceled) {
				t.Fatalf("actual child cancellation was not settled: ready=%t cleanup=%t no-child=%t err=%v", ready, result.result.CleanupComplete, result.result.NoChildStarted, result.err)
			}
			return
		case <-poll.C:
			if !ready {
				if _, err := os.Stat(filepath.Join(root, "ready")); err == nil {
					ready = true
					cancel()
				}
			}
		case <-limit.C:
			t.Fatal("child settlement unknown; retained caller fixture", root)
		}
	}
}

// Real TLS Git acquisition and the production saved CLI/store/native kernel
// restore only the portable agreement. Native inspection then uses the actual
// public Cobra composition and official bundled Hermes, without a fake client,
// runner or detected version. The final session callback uses process.OS in
// place of Unix exec; the separate public UAP binary is provenance here, not an
// executed Cold entrypoint. This does not exercise model/tool calls.
func TestProjectHermesBundleColdRestoreAndRepair(t *testing.T) {
	input := hermesColdSelection(t)
	transport := newProjectJourneyTransportIn(input.ctx, t, "git", input.root)
	input.isolate(t)
	var calls atomic.Int64
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "inventory must not call an MCP transport", http.StatusForbidden)
	}))
	t.Cleanup(endpoint.Close)
	hermesColdSource(t, transport, endpoint.URL+"/mcp")
	original := newHermesColdJourney(t, input, transport)
	original.run(t, original.a, "add", transport.endpoint, "--alias", "docs", "--target", "hermes", "--yes")
	installed, _, servers := hermesColdOwned(t, original, endpoint.URL+"/mcp")
	input.unchanged(t)
	input.inspect(t, original, "original-inspect", servers)
	pair := original.assertPortablePair(t, original.a)
	exchange := transport.temporary(t)
	original.run(t, original.a, "deps", "export", exchange, "--yes")
	hermesColdExchange(t, original, exchange, pair)
	input.protected[original.a] = securityFixtureTree(t, original.a)
	hermesColdAdvance(t, transport)
	cold := newHermesColdJourney(t, input, transport)
	projectJourneyAbsent(t, cold.cache, filepath.Join(cold.a, ".uap"))
	cold.run(t, cold.a, "deps", "import", exchange, "--install", "--yes")
	restored, _, servers := hermesColdOwned(t, cold, endpoint.URL+"/mcp")
	if restored.Source != installed.Source || restored.Package.ManifestDigest != installed.Package.ManifestDigest ||
		restored.InstallationID == installed.InstallationID || pair != cold.assertPortablePair(t, cold.a) {
		t.Fatal("cold restore changed exact source/pins or reused the original installation")
	}
	input.unchanged(t)
	input.inspect(t, cold, "cold-inspect", servers)
	for _, receipt := range restored.DataReceipts {
		projectJourneyWrite(t, filepath.Join(receipt.Locator, "cold-retained.txt"), "retained cold project data\n")
	}
	hermesColdRepeatAndRepair(t, input, cold, restored, endpoint.URL+"/mcp", pair)
	restored, binding, _ := hermesColdOwned(t, cold, endpoint.URL+"/mcp")
	publications := cold.state(t, cold.a).TransactionReceipts
	hermesColdRemove(t, input, cold)
	if calls.Load() != 0 || len(input.commands) != 3 {
		t.Fatal("native configuration inspection unexpectedly called MCP or omitted a selected process")
	}
	// CGI uses the original outer process group, not a context-aware runner.
	// Its actual server join is required before any successful report. A blocked
	// join reaches the parent's hard failure while retaining the entire tree.
	endpoint.Close()
	transport.server.Close()
	if err := input.ctx.Err(); err != nil {
		t.Fatal("native cold lifecycle canceled before completed server settlement", err)
	}
	input.writeReport(t, installed, restored, binding, publications, cold.state(t, cold.a))
}

func hermesColdOwned(t *testing.T, journey projectJourney, endpoint string) (domain.Installation, domain.ClientBinding, map[string]any) {
	t.Helper()
	installation, binding := journey.owned(t, journey.a)
	if binding.ClientID != "hermes" || binding.Scope != "project" ||
		binding.NativeProfileRoot != filepath.Join(journey.a, ".hermes") ||
		binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled ||
		binding.PendingNativeIntent != nil || binding.NativeActivationAttempt != "" || len(binding.Receipts) == 0 {
		t.Fatalf("Hermes cold binding lost scoped ownership or conservative status: %+v", binding)
	}
	config := filepath.Join(journey.a, ".hermes", "uap-session", "config.yaml")
	found := 0
	for _, object := range binding.NativeObjects {
		if object.Kind == "hermes_project_session_mcp" {
			found++
			if object.LogicalName != "docs" || object.Path != config || object.ManagedDigest == "" {
				t.Fatal("Hermes native MCP object is not joined to the selected Project")
			}
		}
	}
	if found != 1 || len(installation.Source.ResolvedRevision) != 40 || installation.Source.TreeDigest == "" {
		t.Fatal("Hermes cold restore lacks its exact Git/native source identity")
	}
	for _, path := range []string{
		filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"),
		filepath.Join(journey.a, ".hermes", "skills", "docs", "SKILL.md"),
	} {
		if projectJourneyRead(t, path) != projectJourneySkill {
			t.Fatal("Hermes cold restore followed a moving source or lost the recorded Skill")
		}
	}
	servers := hermesColdServers(t, config)
	entry, ok := servers["docs"].(map[string]any)
	if len(servers) != 1 || !ok || entry["url"] != endpoint || entry["command"] != nil {
		t.Fatal("Hermes cold config is not the one owned HTTP source")
	}
	journey.inspectNoRecovery(t, journey.a)
	hermesColdRootAuthority(t, journey, binding)
	return installation, binding, servers
}

func hermesColdServers(t *testing.T, path string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(projectJourneyRead(t, path)), &doc); err != nil || len(doc) != 1 {
		t.Fatal("Hermes selected YAML is not a single closed MCP source", err)
	}
	servers, ok := doc["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatal("Hermes selected MCP collection is missing")
	}
	body, err := json.Marshal(servers)
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		t.Fatal(err)
	}
	return normalized
}

func hermesColdExchange(t *testing.T, original projectJourney, exchange string, pair [2]string) {
	t.Helper()
	entries, err := os.ReadDir(exchange)
	if err != nil || len(entries) != 2 || entries[0].Name() != "uap.lock" || entries[1].Name() != "uap.toml" ||
		pair != original.assertPortablePair(t, exchange) {
		t.Fatal("cold transfer contains more than the unchanged portable agreement", err)
	}
}

func hermesColdAdvance(t *testing.T, transport *projectJourneyTransport) {
	t.Helper()
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	transport.git(t, transport.repo, "add", ".")
	transport.git(t, transport.repo, "commit", "-m", "advance remote after frozen Hermes export")
	transport.git(t, transport.repo, "push", "--quiet", transport.remote, "main")
}

func hermesColdRepeatAndRepair(t *testing.T, input *hermesColdInputs, journey projectJourney, original domain.Installation, endpoint string, pair [2]string) {
	t.Helper()
	statePath := filepath.Join(journey.a, ".uap", "state-v2.json")
	before := projectJourneyRead(t, statePath)
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	if projectJourneyRead(t, statePath) != before || pair != journey.assertPortablePair(t, journey.a) {
		t.Fatal("unchanged locked replay rewrote its source agreement or receipts")
	}
	input.unchanged(t)
	_, binding, _ := hermesColdOwned(t, journey, endpoint)
	for _, path := range []string{
		filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"),
		filepath.Join(journey.a, ".hermes", "uap-session", "config.yaml"),
	} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	journey.run(t, journey.a, "repair", original.InstallationID, "--target", "hermes", "--yes")
	repaired, _, servers := hermesColdOwned(t, journey, endpoint)
	if repaired.Source != original.Source || !reflect.DeepEqual(repaired.Package, original.Package) ||
		pair != journey.assertPortablePair(t, journey.a) {
		t.Fatal("missing repair moved the immutable source or saved agreement")
	}
	input.unchanged(t)
	input.inspect(t, journey, "repaired-inspect", servers)
}

func hermesColdRemove(t *testing.T, input *hermesColdInputs, journey projectJourney) {
	t.Helper()
	foreign := filepath.Join(journey.a, ".hermes", "skills", "foreign")
	before := securityFixtureTree(t, foreign)
	installation, binding := journey.owned(t, journey.a)
	rootBefore := hermesColdRootAuthority(t, journey, binding)
	journey.run(t, journey.a, "remove", "docs", "--target", "hermes", "--yes")
	state := journey.state(t, journey.a)
	rootAfter := hermesColdRootAuthority(t, journey, binding)
	if !reflect.DeepEqual(rootBefore.receipt, rootAfter.receipt) || rootBefore.marker != rootAfter.marker ||
		!os.SameFile(rootBefore.root, rootAfter.root) || !os.SameFile(rootBefore.parent, rootAfter.parent) ||
		rootBefore.root.Mode() != rootAfter.root.Mode() || rootBefore.parent.Mode() != rootAfter.parent.Mode() {
		t.Fatal("Hermes removal replaced or relinquished the committed session root authority")
	}
	if len(state.EnvironmentReceipts) != 0 {
		t.Fatal("Hermes removal retained an installed cold binding")
	}
	for _, item := range state.Installations {
		if len(item.Clients) != 0 {
			t.Fatal("Hermes removal retained a client binding")
		}
	}
	projectJourneyAbsent(t, binding.TargetLocator, filepath.Join(journey.a, ".hermes", "skills", "docs"))
	config := filepath.Join(journey.a, ".hermes", "uap-session", "config.yaml")
	if _, err := os.Stat(config); err == nil {
		if servers := hermesColdServers(t, config); len(servers) != 0 {
			t.Fatal("Hermes removal retained owned MCP")
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, securityFixtureTree(t, foreign)) ||
		projectJourneyRead(t, filepath.Join(journey.a, ".hermes", "foreign.txt")) != "project-a-foreign\n" {
		t.Fatal("Hermes removal changed foreign Project files")
	}
	for _, receipt := range installation.DataReceipts {
		if projectJourneyRead(t, filepath.Join(receipt.Locator, "cold-retained.txt")) != "retained cold project data\n" {
			t.Fatal("Hermes removal consumed retained Project data")
		}
	}
	input.unchanged(t)
}
