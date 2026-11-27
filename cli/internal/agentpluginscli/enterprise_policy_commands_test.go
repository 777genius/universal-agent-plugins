package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func executeEnterprisePolicy(t *testing.T, fixture cliFixture, ok bool, args ...string) enterprisePolicyReport {
	t.Helper()
	argv := append([]string{"policy"}, args...)
	argv = append(argv, "--format", "json")
	out, stderr, err := fixture.execute(false, argv...)
	if ok != (err == nil) {
		t.Fatalf("policy %v: err=%v stdout=%s stderr=%s", args, err, out, stderr)
	}
	var result struct {
		SchemaVersion int                    `json:"schema_version"`
		Result        string                 `json:"result"`
		Data          enterprisePolicyReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("policy output was not one JSON document: %v\n%s", err, out)
	}
	if result.SchemaVersion != 1 || (result.Result == "success") != ok {
		t.Fatalf("policy envelope does not match exit result: %+v", result)
	}
	return result.Data
}

func setEnterpriseCommandFiles(fixture *cliFixture, paths ...enterprisepolicy.Path) {
	files := &enterprisepolicy.Files{Paths: paths}
	fixture.app.PolicyFiles, fixture.app.InstallationPolicy = files, files
}

func TestEnterprisePolicyTemplateAndConfigurationCLI(t *testing.T) {
	f := newCLIFixture(t, nil)
	path := filepath.Join(f.root, "uap-policy.yaml")
	setEnterpriseCommandFiles(&f, enterprisepolicy.Path{Layer: "project", Path: path},
		enterprisepolicy.Path{Layer: "user", Path: filepath.Join(f.root, "absent-policy.yaml")})
	beforeInit := observeDependencyCheckFiles(t, f.root)
	planned := executeEnterprisePolicy(t, f, true, "init", path, "--dry-run")
	if planned.Outcome != "planned" {
		t.Fatalf("dry-run did not plan template creation: %+v", planned)
	}
	assertDependencyCheckFilesUnchanged(t, f.root, beforeInit)
	created := executeEnterprisePolicy(t, f, true, "init", path)
	if created.Outcome != "created" || created.Path != path {
		t.Fatalf("template destination not reported: %+v", created)
	}
	validated := executeEnterprisePolicy(t, f, true, "validate", path)
	if validated.RestrictionCount != 0 {
		t.Fatalf("template unexpectedly enabled restrictions: %+v", validated)
	}
	before := observeDependencyCheckFiles(t, f.root)
	refused := executeEnterprisePolicy(t, f, false, "init", path)
	if refused.Code != "POLICY_FILE_EXISTS" {
		t.Fatalf("existing policy was not preserved: %+v", refused)
	}
	unrestricted := executeEnterprisePolicy(t, f, false, "status", "--require-policy")
	if unrestricted.Code != "POLICY_REQUIRED" || len(unrestricted.Layers) != 2 || unrestricted.Layers[1].Status != "absent" {
		t.Fatalf("configuration presence was mistaken for enforcement: %+v", unrestricted)
	}
	assertDependencyCheckFilesUnchanged(t, f.root, before)
	dependencyWrite(t, path, "schema_version: 2\ninstall:\n  allowed_scopes: [project]\n")
	active := executeEnterprisePolicy(t, f, true, "status", "--require-policy")
	if active.RestrictionCount != 1 || len(active.Layers[0].Rules) != 1 || active.Layers[0].Rules[0].Name != "install.allowed_scopes" {
		t.Fatalf("active rule origin was not reported: %+v", active)
	}
	human, _, err := f.execute(false, "policy", "status")
	if err != nil || !strings.Contains(human, path) || !strings.Contains(human, "install.allowed_scopes") || !strings.Contains(human, "absent") {
		t.Fatalf("human policy status is incomplete: %v %s", err, human)
	}
}

func TestEnterprisePolicyValidateSurvivesInvalidActivePolicy(t *testing.T) {
	f := newCLIFixture(t, nil)
	broken := filepath.Join(f.root, "machine.yaml")
	valid := filepath.Join(f.root, "candidate.yaml")
	dependencyWrite(t, broken, "schema_version: 2\nunknown: DO_NOT_LEAK_YAML_SECRET\n")
	dependencyWrite(t, valid, "schema_version: 2\ncomponents:\n  denied: [mcp]\n")
	setEnterpriseCommandFiles(&f, enterprisepolicy.Path{Layer: "machine", Path: broken})
	executeEnterprisePolicy(t, f, true, "validate", valid)
	report := executeEnterprisePolicy(t, f, false, "status")
	if report.Code != "POLICY_INVALID" || report.Layers[0].Status != "invalid" || strings.Contains(report.Message, "DO_NOT_LEAK") {
		t.Fatalf("bad policy diagnostics lost origin or leaked content: %+v", report)
	}
}

func newEnterpriseSourceCheckFixture(t *testing.T) cliFixture {
	t.Helper()
	f := newCLIFixture(t, nil)
	f.app.Detector = dependencyCheckForbiddenEffects{}
	f.app.SourceAcquirerForDryRun = func(root string) SourceAcquirer {
		return sourceacquisition.Acquirer{TempRoot: root}
	}
	return f
}

func TestEnterprisePolicyCandidateCannotWeakenMachineCLI(t *testing.T) {
	f := newEnterpriseSourceCheckFixture(t)
	plugin := writeCLIPlugin(t)
	machine := filepath.Join(f.root, "machine.yaml")
	candidate := filepath.Join(f.root, "candidate.yaml")
	dependencyWrite(t, machine, "schema_version: 2\nsources:\n  allowed_kinds: [git]\n")
	dependencyWrite(t, candidate, "schema_version: 2\nsources:\n  allowed_kinds: [local]\n")
	setEnterpriseCommandFiles(&f, enterprisepolicy.Path{Layer: "machine", Path: machine})
	before := observeDependencyCheckFiles(t, f.root)
	report := executeEnterprisePolicy(t, f, false, "check", plugin, "--target", "codex", "--policy", candidate)
	if report.Outcome != "denied" || len(report.Violations) == 0 || report.Violations[0].PolicyPath != machine {
		t.Fatalf("candidate weakened machine restrictions: %+v", report)
	}
	assertDependencyCheckFilesUnchanged(t, f.root, before)
}

func TestEnterprisePolicySourceCheckCoverageAndNoMutationCLI(t *testing.T) {
	f := newEnterpriseSourceCheckFixture(t)
	plugin := writeCLIPlugin(t)
	policy := filepath.Join(f.root, "policy.yaml")
	dependencyWrite(t, policy, "schema_version: 2\nsources:\n  allowed_kinds: [local]\n")
	setEnterpriseCommandFiles(&f, enterprisepolicy.Path{Layer: "user", Path: policy})
	before := observeDependencyCheckFiles(t, f.root)
	allowed := executeEnterprisePolicy(t, f, true, "check", plugin, "--target", "codex", "--require-policy")
	if allowed.Outcome != "allowed" || allowed.Coverage != "package_only" {
		t.Fatalf("package coverage missing: %+v", allowed)
	}
	assertDependencyCheckFilesUnchanged(t, f.root, before)
	dependencyWrite(t, filepath.Join(plugin, "uap.toml"), "schema_version = 1\n[dependencies.child]\ngit = \"acme/child\"\ncommit = \""+strings.Repeat("b", 40)+"\"\n")
	incomplete := executeEnterprisePolicy(t, f, false, "check", plugin, "--target", "codex")
	if incomplete.Outcome != "incomplete" || incomplete.Code != "DEPENDENCIES_NOT_CHECKED" {
		t.Fatalf("root package approved unresolved dependencies: %+v", incomplete)
	}
	assertDependencyCheckFilesUnchanged(t, f.root, before)
}

func TestEnterprisePolicyOfflineClosureUsesCandidateAndCachedGraphCLI(t *testing.T) {
	f := newDependencyCheckFixture(t)
	child := seedDependencyCheckPackage(t, f, "child", "")
	root := seedDependencyCheckPackage(t, f, "root", "schema_version = 1\n[dependencies.child]\ngit = \"acme/child\"\ncommit = \""+strings.Repeat("a", 40)+"\"\n")
	child.RequiredBy = []environment.RequiredBy{{Alias: "root", Selector: child.Source.Git.Requested}}
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{root, child}, "root")
	candidate := filepath.Join(f.root, "candidate.yaml")
	dependencyWrite(t, candidate, "schema_version: 2\nsources:\n  deny_repositories: [github.com/acme/child]\n")
	setEnterpriseCommandFiles(&f)
	before := observeDependencyCheckFiles(t, f.root)
	allowed := executeEnterprisePolicy(t, f, true, "check")
	if allowed.Outcome != "allowed" || allowed.Coverage != "declared_closure" {
		t.Fatalf("offline closure not checked: %+v", allowed)
	}
	denied := executeEnterprisePolicy(t, f, false, "check", "--policy", candidate)
	if denied.Outcome != "denied" || len(denied.Violations) == 0 || denied.Violations[0].Layer != "candidate" {
		t.Fatalf("transitive candidate restriction did not apply: %+v", denied)
	}
	assertDependencyCheckFilesUnchanged(t, f.root, before)
	if err := os.RemoveAll(f.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	incomplete := executeEnterprisePolicy(t, f, false, "check", "--policy", candidate)
	if incomplete.Outcome != "incomplete" || incomplete.Code != "CLOSURE_NOT_VERIFIED" {
		t.Fatalf("missing cache became policy approval: %+v", incomplete)
	}
}

type enterpriseCommandCustomGate struct{ calls int }

func (gate *enterpriseCommandCustomGate) Check(_ context.Context, request domain.InstallPolicyRequest) (string, error) {
	gate.calls++
	if request.Envelope == nil {
		return "custom", nil
	}
	return "", &domain.InstallPolicyError{Violations: []domain.InstallPolicyViolation{{Code: "custom_rule", Layer: "injected", Subject: "plugin", Message: "Embedded host denied this package."}}}
}

func TestEnterprisePolicyCheckPreservesInjectedGateCLI(t *testing.T) {
	f := newEnterpriseSourceCheckFixture(t)
	candidate := filepath.Join(f.root, "candidate.yaml")
	dependencyWrite(t, candidate, "schema_version: 2\n")
	setEnterpriseCommandFiles(&f)
	gate := &enterpriseCommandCustomGate{}
	f.app.InstallationPolicy = gate
	report := executeEnterprisePolicy(t, f, false, "check", writeCLIPlugin(t), "--target", "codex", "--policy", candidate)
	if report.Outcome != "denied" || len(report.Violations) != 1 || report.Violations[0].Layer != "injected" || gate.calls == 0 {
		t.Fatalf("candidate replaced injected authority: %+v calls=%d", report, gate.calls)
	}
}

func TestEnterprisePolicyCheckRetainsInspectedRestrictions(t *testing.T) {
	f := newCLIFixture(t, nil)
	path := filepath.Join(f.root, "policy.yaml")
	dependencyWrite(t, path, "schema_version: 2\ncomponents:\n  denied: [mcp]\n")
	setEnterpriseCommandFiles(&f, enterprisepolicy.Path{Layer: "user", Path: path})
	snapshot, err := f.app.PolicyFiles.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	gate := enterpriseCommandGate(f.app, "", snapshot)
	dependencyWrite(t, path, "schema_version: 2\n")
	envelope := domain.PackageEnvelope{}
	envelope.MCP.Present = true
	_, err = gate.Check(t.Context(), domain.InstallPolicyRequest{Envelope: &envelope, Scope: domain.ScopeUser, Targets: []domain.ClientID{domain.ClientCodex}})
	var denied *domain.InstallPolicyError
	if !errors.As(err, &denied) || len(denied.Violations) == 0 || denied.Violations[0].PolicyPath != path {
		t.Fatalf("inspection weakened its initial restrictions after a file change: %v", err)
	}
}
