//go:build linux

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type windDirectCacheFact struct {
	Device, Inode, Links uint64
	UID, GID, Mode       uint32
	Size                 int64
	Modified, Changed    syscall.Timespec
	Digest               string
}

type windDirectCacheProbe struct {
	root       string
	scanner    *windUserCacheScanner
	before     map[string]windDirectCacheFact
	keys       map[string]bool
	stage      int
	start, end time.Time
}

func windDirectCacheSHA(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func newWindDirectCacheJourney(t *testing.T) (devinProjectJourney, *windDirectCacheProbe) {
	t.Helper()
	f := devinProjectJourney{projectJourney{home: devinJourneyTemp(t), a: devinJourneyTemp(t), b: devinJourneyTemp(t)}}
	f.cache = filepath.Join(f.home, "uap")
	for _, root := range []string{"", f.a, f.b} {
		native := f.devinRoot(root)
		projectJourneyWrite(t, filepath.Join(native, "mcp_config.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"foreignPreference":"keep"}`)
		projectJourneyWrite(t, filepath.Join(native, "skills", "foreign", "SKILL.md"), "foreign skill\n")
		projectJourneyWrite(t, filepath.Join(native, "hooks.json"), "foreign hooks\n")
	}
	scratch := devinJourneyTemp(t)
	for _, name := range []string{"TMPDIR", "TEMP", "TMP"} {
		t.Setenv(name, scratch)
	}
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(f.home, f.cache, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	scanner := &windUserCacheScanner{}
	f.app = windDirectCacheApp(t, app, f.home, scanner)
	if err := os.MkdirAll(filepath.Join(f.cache, "security", "assessments"), 0700); err != nil {
		t.Fatal(err)
	}
	projectJourneyWrite(t, filepath.Join(f.cache, "security", "lintai", "caller-owned.txt"), "original scanner cache marker; never executed\n")
	probe := &windDirectCacheProbe{root: f.cache, scanner: scanner, keys: map[string]bool{}}
	probe.before = windDirectCacheFrame(t, f.cache)
	return f, probe
}

func windDirectCacheApp(t *testing.T, app agentpluginscli.App, home string, scanner *windUserCacheScanner) agentpluginscli.App {
	t.Helper()
	app.Detector = devinProjectDetector{root: filepath.Join(home, ".devin")}
	app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
	app.DirectoryClientForDryRun, app.CommitDirectoryPreview = nil, nil
	evaluator, ok := app.SecurityEvaluator.(securityscan.Evaluator)
	if !ok {
		t.Fatal("default evaluator absent")
	}
	cache, ok := evaluator.Cache.(securityscan.FileCache)
	if !ok || cache.Root != filepath.Join(home, "uap", "security", "assessments") || evaluator.Requirement != securityscan.DefaultRequirement() {
		t.Fatal("default file cache absent")
	}
	acquirer, ok := app.SourceAcquirer.(lazySourceAcquirer)
	if !ok || acquirer.dataRoot != filepath.Join(home, "uap") || acquirer.acquirer.TempRoot != acquirer.dataRoot {
		t.Fatal("default lazy acquirer absent")
	}
	evaluator.Scanner = scanner
	app.SecurityEvaluator = evaluator
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return windDirectCacheApp(t, selected, home, scanner), nil
	}
	return app
}

func windDirectCacheInfo(t *testing.T, info os.FileInfo) windDirectCacheFact {
	t.Helper()
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != uint32(os.Getuid()) || s.Gid != uint32(os.Getgid()) || s.Ino == 0 || (!info.IsDir() && (!info.Mode().IsRegular() || s.Nlink != 1)) {
		t.Fatal("ordinary owned cache authority required")
	}
	return windDirectCacheFact{Device: s.Dev, Inode: s.Ino, Links: s.Nlink, UID: s.Uid, GID: s.Gid, Mode: s.Mode, Size: s.Size, Modified: s.Mtim, Changed: s.Ctim}
}

func windDirectCacheFrame(t *testing.T, root string) map[string]windDirectCacheFact {
	t.Helper()
	frame := map[string]windDirectCacheFact{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		fact := windDirectCacheInfo(t, info)
		if !entry.IsDir() {
			fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
			if err != nil {
				return err
			}
			file := os.NewFile(uintptr(fd), path)
			body, readErr := io.ReadAll(io.LimitReader(file, (1<<20)+1))
			after, statErr := file.Stat()
			closeErr := file.Close()
			if readErr != nil || statErr != nil || closeErr != nil || len(body) > 1<<20 || !os.SameFile(info, after) || fact != windDirectCacheInfo(t, after) {
				t.Fatal("cache read frame changed")
			}
			fact.Digest = windDirectCacheSHA(body)
		}
		frame[filepath.ToSlash(name)] = fact
		if len(frame) > 32 {
			t.Fatal("finite cache inventory exceeded")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func (probe *windDirectCacheProbe) observe(t *testing.T, stage string, raw []byte, installation domain.Installation) {
	t.Helper()
	stages := []string{"add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b"}
	scans := []int{1, 2, 2, 2, 2, 3, 3, 3, 3, 3}
	if probe.stage >= len(stages) || stages[probe.stage] != stage || probe.scanner.calls != scans[probe.stage] {
		t.Fatalf("measured scan ledger differs: stage=%s index=%d calls=%d", stage, probe.stage, probe.scanner.calls)
	}
	requirement := securityscan.DefaultRequirement()
	subject := domain.SecuritySubject{TreeDigest: installation.Source.TreeDigest, ManifestDigest: installation.Package.ManifestDigest}
	key := securityscan.CacheKey(subject, requirement)
	_, existed := probe.keys[key]
	probe.keys[key] = true
	after := windDirectCacheFrame(t, probe.root)
	for name, fact := range after {
		old, present := probe.before[name]
		if present && old == fact {
			continue
		}
		if name == "." || name == "security/assessments" && !existed {
			left, right := old, fact
			left.Modified, left.Changed, right.Modified, right.Changed = syscall.Timespec{}, syscall.Timespec{}, syscall.Timespec{}, syscall.Timespec{}
			if name == "security/assessments" {
				left.Size, right.Size = 0, 0
			}
			if !present || left != right || fact.Size < 0 || fact.Size > 1<<20 {
				t.Fatal("cache parent changed unrelated authority", name)
			}
			for _, stamp := range []syscall.Timespec{fact.Modified, fact.Changed} {
				when := time.Unix(stamp.Sec, stamp.Nsec)
				if stamp.Nsec < 0 || stamp.Nsec >= 1e9 || when.Before(probe.start) || when.After(probe.end) {
					t.Fatal("cache parent timestamp outside actual command")
				}
			}
			continue
		}
		if !present && !existed && name == "security/assessments/"+key+".json" {
			continue
		}
		t.Fatal("cache lifecycle changed unrelated original record", name)
	}
	for name := range probe.before {
		if _, ok := after[name]; !ok {
			t.Fatal("cache lifecycle removed prior record", name)
		}
	}
	entries, err := os.ReadDir(filepath.Join(probe.root, "security", "assessments"))
	if err != nil || len(entries) != len(probe.keys) {
		t.Fatal("finite assessment membership differs", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if len(name) != 69 || filepath.Ext(name) != ".json" || !probe.keys[name[:64]] {
			t.Fatal("unexpected cache key")
		}
	}
	cache := securityscan.FileCache{Root: filepath.Join(probe.root, "security", "assessments")}
	assessment, found := cache.Load(key)
	want := domain.SecurityAssessment{SchemaVersion: 1, Subject: subject, Scanner: requirement.Scanner, Policy: requirement.Policy, Outcome: domain.SecurityNoBlockingFindings, ScannedFiles: 3, ReportDigest: "sha256:" + windDirectCacheSHA([]byte(windUserCacheReport))}
	if !found || assessment.Validate(requirement, subject) != nil || !reflect.DeepEqual(assessment, want) {
		t.Fatal("exact cache subject/policy/report differs")
	}
	expected, err := json.Marshal(assessment)
	path := filepath.Join(cache.Root, key+".json")
	file := after["security/assessments/"+key+".json"]
	if err != nil || projectJourneyRead(t, path) != string(expected) || file.Mode != 0100600 || file.Size != int64(len(expected)) || file.Digest != windDirectCacheSHA(expected) || file.Device != after["security/assessments"].Device {
		t.Fatal("canonical assessment cache differs", err)
	}
	if !existed {
		for _, stamp := range []syscall.Timespec{file.Modified, file.Changed} {
			when := time.Unix(stamp.Sec, stamp.Nsec)
			if stamp.Nsec < 0 || stamp.Nsec >= 1e9 || when.Before(probe.start) || when.After(probe.end) {
				t.Fatal("cache file timestamps outside actual command")
			}
		}
	}
	paths := []string{}
	if len(raw) > 0 {
		var value any
		if json.Unmarshal(raw, &value) != nil {
			t.Fatal("public JSON")
		}
		probe.security(t, value, "$", subject, requirement, &paths)
	}
	sort.Strings(paths)
	t.Logf("%s: scanner_calls=%d cache_files=%d security=%v", stage, probe.scanner.calls, len(entries), paths)
	probe.before, probe.stage = after, probe.stage+1
}

func (probe *windDirectCacheProbe) security(t *testing.T, value any, path string, subject domain.SecuritySubject, requirement domain.SecurityRequirement, paths *[]string) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			if key == "security" && item != nil {
				raw, err := json.Marshal(item)
				var assessment domain.SecurityAssessment
				decoder := json.NewDecoder(bytes.NewReader(raw))
				decoder.DisallowUnknownFields()
				if err != nil || decoder.Decode(&assessment) != nil || assessment.Validate(requirement, subject) != nil || (assessment.Evidence != domain.SecurityEvidenceCache && assessment.Evidence != domain.SecurityEvidenceLocalScan) {
					t.Fatal("public security subject/evidence differs")
				}
				stored, found := (securityscan.FileCache{Root: filepath.Join(probe.root, "security", "assessments")}).Load(securityscan.CacheKey(subject, requirement))
				cleared := assessment
				cleared.Evidence = ""
				if !found || !reflect.DeepEqual(cleared, stored) {
					t.Fatal("public security does not join the exact cache assessment")
				}
				*paths = append(*paths, path+"."+key+"="+string(assessment.Evidence))
			}
			probe.security(t, item, path+"."+key, subject, requirement, paths)
		}
	case []any:
		for _, item := range value {
			probe.security(t, item, path+"[]", subject, requirement, paths)
		}
	}
}
