package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Every effect capable of contacting a source, probing a client or refreshing
// security is installed and forbidden. The user-scope check also has no SDK.
type dependencyCheckForbiddenEffects struct{}

func (dependencyCheckForbiddenEffects) AcquireLocal(context.Context, string) (domain.PackageSnapshot, error) {
	panic("offline check acquired a local source")
}
func (dependencyCheckForbiddenEffects) DiscoverGitHubPackages(context.Context, string, string) ([]string, error) {
	panic("offline check discovered remote packages")
}
func (dependencyCheckForbiddenEffects) AcquireGitHub(context.Context, string, string, string) (domain.PackageSnapshot, error) {
	panic("offline check downloaded a source")
}
func (dependencyCheckForbiddenEffects) AcquireGitHubVerified(context.Context, string, string, string, string) (domain.PackageSnapshot, error) {
	panic("offline check downloaded an exact source")
}
func (dependencyCheckForbiddenEffects) Load(context.Context, uint64) (directoryv1.VerifiedBundle, error) {
	panic("offline check refreshed the Directory")
}
func (dependencyCheckForbiddenEffects) Detect(context.Context) ([]domain.DetectedClient, error) {
	panic("offline check detected native clients")
}
func (dependencyCheckForbiddenEffects) Evaluate(context.Context, domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	panic("offline check ran a security evaluator")
}

func newDependencyCheckFixture(t *testing.T) cliFixture {
	t.Helper()
	f := newCLIFixture(t, nil)
	f.app.DependenciesRoot = filepath.Join(f.root, "data")
	f.app.EnvironmentStoreRoot = filepath.Join(f.root, "data", "store")
	f.app.EnvironmentLockPath = filepath.Join(f.root, "data", "environment.lock")
	if err := os.MkdirAll(f.app.DependenciesRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	f.app.SourceAcquirer = dependencyCheckForbiddenEffects{}
	f.app.DirectoryClient = dependencyCheckForbiddenEffects{}
	f.app.Detector = dependencyCheckForbiddenEffects{}
	f.app.SecurityEvaluator = dependencyCheckForbiddenEffects{}
	return f
}

func seedDependencyCheckPackage(t *testing.T, f cliFixture, name, sidecar string) environment.LockedPackage {
	t.Helper()
	root := t.TempDir()
	manifest := fmt.Sprintf(`{"$schema":%q,"name":%q,"version":"1.0.0","description":"Offline check fixture"}`, domain.PluginSchemaV1, name)
	dependencyWrite(t, filepath.Join(root, "plugin.json"), manifest)
	if sidecar != "" {
		dependencyWrite(t, filepath.Join(root, "uap.toml"), sidecar)
	}
	revision := strings.Repeat("a", 40)
	source := domain.SourceIdentity{Repository: "acme/" + name, ResolvedRevision: revision}
	snapshot, err := (packagedigest.Builder{}).SnapshotWithExecutables(context.Background(), root, source, []string{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	envelope, err := f.app.PackageLoader.Load(context.Background(), domain.LoadInput{SnapshotRoot: snapshot.Root, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: source})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.dependencyStore().Put(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	return environment.LockedPackage{Alias: name, Version: "1.0.0", SourceKey: "github:acme/" + name + ":", Targets: []string{"codex"}, DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: snapshot.TreeDigest, ManifestDigest: envelope.ManifestDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: "acme/" + name, Commit: revision, Requested: environment.Selector{Kind: environment.SelectorCommit, Value: revision}}}}
}

func writeDependencyCheckAgreement(t *testing.T, f cliFixture, pins []environment.LockedPackage, roots ...string) environment.Lock {
	t.Helper()
	body := "schema_version = 1\n"
	for _, root := range roots {
		body += fmt.Sprintf("[dependencies.%s]\ngit = %q\ncommit = %q\n", root, "acme/"+root, strings.Repeat("a", 40))
	}
	if len(roots) > 0 {
		quoted := make([]string, len(roots))
		for i, root := range roots {
			quoted[i] = fmt.Sprintf("%q", root)
		}
		body += "[targets]\ncodex = [" + strings.Join(quoted, ", ") + "]\n"
	}
	mp, lp := f.app.dependencyPaths()
	if err := os.MkdirAll(filepath.Dir(mp), 0o700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, mp, body)
	requirements, err := readDependencyManifest(mp)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := environment.ManifestDigest(requirements)
	if err != nil {
		t.Fatal(err)
	}
	lock := environment.Lock{LockVersion: environment.LockVersion, ManifestDigest: digest, Packages: pins}
	if err := writeDependencyLock(lp, lock); err != nil {
		t.Fatal(err)
	}
	return lock
}

func executeDependencyCheck(t *testing.T, f cliFixture, outcome dependencyCheckOutcome, args ...string) (dependencyCheckReport, string) {
	t.Helper()
	command := append([]string{"deps", "check", "--format", "json"}, args...)
	out, stderr, err := f.execute(false, command...)
	if (outcome == dependencyCheckValid) != (err == nil) {
		t.Fatalf("check outcome=%s err=%v stdout=%s stderr=%s", outcome, err, out, stderr)
	}
	if err != nil && !errors.Is(err, errDependencyCheckFailed) {
		t.Fatalf("check failed without a structured verdict: %v", err)
	}
	var report dependencyCheckReport
	if err := json.Unmarshal([]byte(out), &report); err != nil || report.SchemaVersion != 1 || report.Outcome != outcome {
		t.Fatalf("check report: %+v error=%v body=%s", report, err, out)
	}
	return report, out
}

func requireDependencyCheckCode(t *testing.T, report dependencyCheckReport, code string) {
	t.Helper()
	for _, check := range report.Checks {
		if check.Code == code {
			return
		}
	}
	t.Fatalf("missing diagnostic %q: %+v", code, report)
}

func TestDependencyCheckOfflineCLI(t *testing.T) {
	for _, empty := range []bool{true, false} {
		t.Run(fmt.Sprintf("empty=%t", empty), func(t *testing.T) {
			f := newDependencyCheckFixture(t)
			if empty {
				writeDependencyCheckAgreement(t, f, nil)
			} else {
				pin := seedDependencyCheckPackage(t, f, "demo", "")
				writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
			}
			before := observeDependencyCheckFiles(t, f.root)
			_, first := executeDependencyCheck(t, f, dependencyCheckValid)
			_, second := executeDependencyCheck(t, f, dependencyCheckValid, "--dry-run")
			if first != second {
				t.Fatalf("offline checks are nondeterministic: %s\n%s", first, second)
			}
			out, _, err := f.execute(false, "deps", "check")
			if err != nil || !strings.Contains(out, "Local dependency integrity (user): valid") || !strings.Contains(out, "No package sources were fetched") {
				t.Fatalf("human check: %v %s", err, out)
			}
			assertDependencyCheckFilesUnchanged(t, f.root, before)
		})
	}
}

func TestDependencyCheckDamagedStateCLI(t *testing.T) {
	for _, name := range []string{"manifest_missing", "lock_missing", "manifest_invalid", "lock_invalid", "lock_conflicted", "lock_stale", "cache_missing", "metadata_corrupt", "payload_corrupt", "pin_inventory", "pin_version"} {
		t.Run(name, func(t *testing.T) {
			f := newDependencyCheckFixture(t)
			pin := seedDependencyCheckPackage(t, f, "demo", "")
			lock := writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
			mp, lp := f.app.dependencyPaths()
			stored, err := f.app.dependencyStore().Get(context.Background(), pin.DigestAlgorithm, pin.TreeDigest)
			if err != nil {
				t.Fatal(err)
			}
			outcome, code := dependencyCheckInvalid, name
			switch name {
			case "manifest_missing":
				err = os.Remove(mp)
				outcome = dependencyCheckUnavailable
			case "lock_missing":
				err = os.Remove(lp)
				outcome = dependencyCheckUnavailable
			case "manifest_invalid":
				dependencyWrite(t, mp, "schema_version = \"invalid\"\n")
			case "lock_invalid":
				dependencyWrite(t, lp, `{"lock_version":1,"lock_version":1}`)
			case "lock_conflicted":
				dependencyWrite(t, lp, "<<<<<<< HEAD\r\n{}\r\n=======\r\n{}\r\n>>>>>>> other\r\n")
				code = "lock_invalid"
			case "lock_stale":
				lock.ManifestDigest = "sha256:" + strings.Repeat("b", 64)
				err = writeDependencyLock(lp, lock)
			case "cache_missing":
				err = os.RemoveAll(f.app.EnvironmentStoreRoot)
				outcome, code = dependencyCheckUnavailable, "package_unavailable"
			case "metadata_corrupt":
				dependencyWrite(t, filepath.Join(filepath.Dir(stored.Root), "metadata.json"), `{"schema_version":1,"schema_version":1}`)
				code = "store_mismatch"
			case "payload_corrupt":
				dependencyWrite(t, filepath.Join(stored.Root, "plugin.json"), "{}")
				code = "store_mismatch"
			case "pin_inventory":
				lock.Packages[0].ExecutableFiles = []string{"ghost.sh"}
				err = writeDependencyLock(lp, lock)
				code = "package_invalid"
			case "pin_version":
				lock.Packages[0].Version = "9.0.0"
				err = writeDependencyLock(lp, lock)
				code = "package_invalid"
			}
			if err != nil {
				t.Fatal(err)
			}
			before := observeDependencyCheckFiles(t, f.root)
			report, _ := executeDependencyCheck(t, f, outcome)
			requireDependencyCheckCode(t, report, code)
			assertDependencyCheckFilesUnchanged(t, f.root, before)
		})
	}
}

func TestDependencyCheckDeclaredGraphCLI(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "spurious"} {
		t.Run(mode, func(t *testing.T) {
			f := newDependencyCheckFixture(t)
			sidecar := fmt.Sprintf("schema_version = 1\n[dependencies.child]\ngit = \"acme/child\"\ncommit = %q\n", strings.Repeat("a", 40))
			if mode == "spurious" {
				sidecar = ""
			}
			parent := seedDependencyCheckPackage(t, f, "parent", sidecar)
			pins := []environment.LockedPackage{parent}
			if mode != "missing" {
				child := seedDependencyCheckPackage(t, f, "child", "")
				child.RequiredBy = []environment.RequiredBy{{Alias: "parent", Selector: child.Source.Git.Requested}}
				pins = append(pins, child)
			}
			writeDependencyCheckAgreement(t, f, pins, "parent")
			before := observeDependencyCheckFiles(t, f.root)
			outcome := dependencyCheckInvalid
			if mode == "valid" {
				outcome = dependencyCheckValid
			}
			report, _ := executeDependencyCheck(t, f, outcome)
			if mode != "valid" {
				requireDependencyCheckCode(t, report, "graph_declarations_mismatch")
			}
			assertDependencyCheckFilesUnchanged(t, f.root, before)
		})
	}
}

type dependencyCheckHookLoader struct {
	delegate ports.PackageLoader
	hook     func(domain.LoadInput)
}

func (loader dependencyCheckHookLoader) Load(ctx context.Context, input domain.LoadInput) (domain.PackageEnvelope, error) {
	loaded, err := loader.delegate.Load(ctx, input)
	loader.hook(input)
	return loaded, err
}

func TestDependencyCheckRechecksAgreementAndCleansSnapshot(t *testing.T) {
	f := newDependencyCheckFixture(t)
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	mp, _ := f.app.dependencyPaths()
	var snapshotRoot string
	f.app.PackageLoader = dependencyCheckHookLoader{delegate: f.app.PackageLoader, hook: func(input domain.LoadInput) {
		snapshotRoot = input.SnapshotRoot
		if strings.HasPrefix(snapshotRoot, f.app.EnvironmentStoreRoot+string(filepath.Separator)) {
			t.Fatal("offline check parsed the mutable store tree instead of its verified snapshot")
		}
		dependencyWrite(t, mp, "schema_version = 1\n# concurrent external intent\n")
	}}
	report, _ := executeDependencyCheck(t, f, dependencyCheckError)
	requireDependencyCheckCode(t, report, "agreement_changed")
	if !strings.Contains(dependencyRead(t, mp), "concurrent external intent") {
		t.Fatal("check rewrote external agreement edit")
	}
	if _, err := os.Stat(snapshotRoot); !os.IsNotExist(err) {
		t.Fatalf("verified snapshot leaked after check: %s %v", snapshotRoot, err)
	}
}

func TestDependencyCheckCancellationAndOperationalClassification(t *testing.T) {
	f := newDependencyCheckFixture(t)
	writeDependencyCheckAgreement(t, f, nil)
	before := observeDependencyCheckFiles(t, f.root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	f.app.Output = &output
	command := NewRoot(f.app)
	command.SetArgs([]string{"deps", "check", "--format", "json"})
	if err := command.ExecuteContext(ctx); !errors.Is(err, errDependencyCheckFailed) {
		t.Fatalf("canceled check did not fail: %v", err)
	}
	var report dependencyCheckReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.Outcome != dependencyCheckCanceled {
		t.Fatalf("canceled report: %s %v", output.String(), err)
	}
	assertDependencyCheckFilesUnchanged(t, f.root, before)
	for _, err := range []error{&os.PathError{Op: "open", Path: "metadata.json", Err: os.ErrPermission}, &os.PathError{Op: "read", Path: "metadata.json", Err: errors.New("I/O failure")}} {
		issue := dependencyPackageCheckFailure("demo", err)
		if issue.Outcome != dependencyCheckError || issue.Code != "store_unreadable" {
			t.Fatalf("operational error was reclassified as missing/invalid: %+v", issue)
		}
	}
	if dependencyPackageCheckFailure("demo", context.Canceled).Outcome != dependencyCheckCanceled {
		t.Fatal("cancellation lost its distinct outcome")
	}
}

// The policy verdict must be emitted through the existing offline, versioned
// dependency integrity contract; missing or denied policy never returns success.
func TestDependencyCheckOfflinePolicyGateCLI(t *testing.T) {
	f := newDependencyCheckFixture(t)
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	path := filepath.Join(f.root, "policy.yml")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("schema_version: 1\nsources:\n  allow_hosts: [github.com]\n")
	before := observeDependencyCheckFiles(t, f.root)
	report, _ := executeDependencyCheck(t, f, dependencyCheckValid, "--policy", path)
	found := false
	for _, check := range report.Checks {
		if check.Stage == "policy" && check.Outcome == dependencyCheckValid {
			found = true
		}
	}
	if !found { t.Fatal("offline policy approval did not produce a policy check") }
	assertDependencyCheckFilesUnchanged(t, f.root, before)

	write("schema_version: 1\nsources:\n  deny_hosts: [github.com]\n")
	before = observeDependencyCheckFiles(t, f.root)
	report, _ = executeDependencyCheck(t, f, dependencyCheckInvalid, "--policy", path)
	requireDependencyCheckCode(t, report, "policy_denied")
	assertDependencyCheckFilesUnchanged(t, f.root, before)

	if err := os.Remove(path); err != nil { t.Fatal(err) }
	before = observeDependencyCheckFiles(t, f.root)
	report, _ = executeDependencyCheck(t, f, dependencyCheckUnavailable, "--policy", path)
	requireDependencyCheckCode(t, report, "policy_unavailable")
	assertDependencyCheckFilesUnchanged(t, f.root, before)
}
