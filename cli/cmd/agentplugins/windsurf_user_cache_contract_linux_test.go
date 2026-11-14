//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const windUserCacheReport = `{"schema_version":1,"tool":{"name":"lintai","version":"0.1.3"},"policy":{"id":"agent-plugin-install","version":2,"presets":["recommended","preview","threat-review","supply-chain","advisory"]},"stats":{"scanned_files":3,"skipped_files":0},"findings":[],"diagnostics":[],"runtime_errors":[]}`

type windUserCacheScanner struct{ calls int }

func (s *windUserCacheScanner) Scan(context.Context, string) ([]byte, error) {
	s.calls++
	return []byte(windUserCacheReport), nil
}

// Preserve the real evaluator, cache, policy, lazy source acquirer and scoped
// composer. Only host/service discovery and the scanner response are inert.
func windUserCacheApp(t *testing.T, app agentpluginscli.App, profile string, scanner *windUserCacheScanner) agentpluginscli.App {
	t.Helper()
	app.Detector = windUserScratchDetector{profile}
	app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
	app.DirectoryClientForDryRun, app.CommitDirectoryPreview = nil, nil
	evaluator, ok := app.SecurityEvaluator.(securityscan.Evaluator)
	if !ok {
		t.Fatal("default public security evaluator missing")
	}
	if _, ok := evaluator.Cache.(securityscan.FileCache); !ok {
		t.Fatal("default public assessment file cache missing")
	}
	if _, ok := app.SourceAcquirer.(lazySourceAcquirer); !ok {
		t.Fatal("default lazy source acquirer missing")
	}
	evaluator.Scanner = scanner
	app.SecurityEvaluator = evaluator
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return windUserCacheApp(t, selected, profile, scanner), nil
	}
	return app
}

func TestWindsurfPublicUserAddUsesDefaultSecurityCache(t *testing.T) {
	f := newWindUserScratchFixture(t)
	marker := filepath.Join(f.state, "security", "lintai", "caller-owned.txt")
	projectJourneyWrite(t, marker, "inert original scanner-cache marker; never executed\n")
	scanner := &windUserCacheScanner{}
	app := windUserCacheApp(t, f.app, f.profile, scanner)
	before := securityFixtureTree(t, filepath.Join(f.state, "security"))
	raw := windUserScratchExecute(t, app, f.source, f.scratch)
	state, err := app.StateStore.Load()
	if err != nil || len(state.Installations) != 1 || scanner.calls != 1 {
		t.Fatal("actual public evaluator did not scan one exact package", err)
	}
	i := state.Installations[0]
	subject := domain.SecuritySubject{TreeDigest: i.Source.TreeDigest, ManifestDigest: i.Package.ManifestDigest}
	requirement := securityscan.DefaultRequirement()
	key := securityscan.CacheKey(subject, requirement)
	root := filepath.Join(f.state, "security", "assessments")
	cache := app.SecurityEvaluator.(securityscan.Evaluator).Cache.(securityscan.FileCache)
	assessment, found := cache.Load(key)
	if cache.Root != root || !found || assessment.Validate(requirement, subject) != nil || assessment.Evidence != "" {
		t.Fatal("real default cache subject/policy differs")
	}
	reportHash := sha256.Sum256([]byte(windUserCacheReport))
	want := domain.SecurityAssessment{SchemaVersion: 1, Subject: subject, Scanner: requirement.Scanner, Policy: requirement.Policy, Outcome: domain.SecurityNoBlockingFindings, ScannedFiles: 3, ReportDigest: "sha256:" + hex.EncodeToString(reportHash[:])}
	expected, err := json.Marshal(want)
	if err != nil || !reflect.DeepEqual(assessment, want) || projectJourneyRead(t, filepath.Join(root, key+".json")) != string(expected) {
		t.Fatal("real cache bytes differ from exact inert report", err)
	}
	var public struct {
		Data struct {
			Security domain.SecurityAssessment `json:"security"`
		} `json:"data"`
	}
	want.Evidence = domain.SecurityEvidenceLocalScan
	if json.Unmarshal(raw, &public) != nil || !reflect.DeepEqual(public.Data.Security, want) {
		t.Fatal("public User result does not bind the same local scan")
	}
	entries, err := os.ReadDir(root)
	info, statErr := os.Lstat(filepath.Join(root, key+".json"))
	if err != nil || statErr != nil || len(entries) != 1 || entries[0].Name() != key+".json" || !info.Mode().IsRegular() || info.Mode() != 0600 {
		t.Fatal("finite source-defined assessment cache membership/mode differs")
	}
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != uint32(os.Getuid()) || s.Gid != uint32(os.Getgid()) || s.Nlink != 1 {
		t.Fatal("assessment cache owner/link differs")
	}
	after := securityFixtureTree(t, filepath.Join(f.state, "security"))
	for name, value := range before {
		if after[name] != value {
			t.Fatal("default security cache changed original scanner-cache bytes/mode")
		}
	}
	if len(after) != len(before)+2 || projectJourneyRead(t, marker) != "inert original scanner-cache marker; never executed\n" {
		t.Fatal("default public source/cache added an unrelated security entry")
	}
	t.Log("actual default User evaluator: one new assessments directory and one exact source/policy-bound cache file")
}
