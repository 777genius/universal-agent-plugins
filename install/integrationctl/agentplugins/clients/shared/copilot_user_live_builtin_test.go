package shared

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type copilotUserLiveStep struct {
	argv   []string
	tree   bool
	result legacyports.CommandResult
	err    error
	cancel context.CancelFunc
}

type copilotUserLiveRunner struct {
	t     *testing.T
	steps []copilotUserLiveStep
	calls int
}

func (r *copilotUserLiveRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	return r.next(command, false)
}

func (r *copilotUserLiveRunner) RunWithTreeExitGrace(_ context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	r.t.Helper()
	if grace != time.Second {
		r.t.Fatal("original native registry tree exit grace changed")
	}
	return r.next(command, true)
}

func (r *copilotUserLiveRunner) next(command legacyports.Command, tree bool) (legacyports.CommandResult, error) {
	r.t.Helper()
	if r.calls >= len(r.steps) {
		r.t.Fatal("unexpected inert command; no executable may run")
	}
	step := r.steps[r.calls]
	r.calls++
	if tree != step.tree || !reflect.DeepEqual(command, legacyports.Command{Argv: step.argv}) {
		r.t.Fatalf("held command/runner changed: %+v tree=%v", command, tree)
	}
	if step.cancel != nil {
		step.cancel()
	}
	return step.result, step.err
}

func copilotUserLiveRequest(t *testing.T) domain.ActivationRequest {
	t.Helper()
	root := t.TempDir()
	executable := filepath.Join(root, "never-executed", "copilot")
	const physical = "demo-owned-0123456789ab"
	plan := domain.DeliveryPlan{ClientID: domain.ClientCopilot, Scope: domain.ScopeUser,
		TargetContext: domain.TargetContext{Scope: domain.ScopeUser}, DeclaredName: "demo",
		DeclaredVersion: "1.2.3", PhysicalArtifactID: physical, ActivePath: filepath.Join(root, "managed", physical),
		NativeRegistryExecutable: executable, Activation: domain.ActivationPrepared,
		Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid}
	if !filepath.IsAbs(executable) || filepath.Clean(executable) != executable || !filepath.IsAbs(plan.ActivePath) || filepath.Clean(plan.ActivePath) != plan.ActivePath {
		t.Fatal("inert positive fixture must use native canonical absolute paths")
	}
	return domain.ActivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientCopilot, ExecutablePath: executable},
		Plan: plan, Delivery: domain.StagedDelivery{ClientID: domain.ClientCopilot, ActivePath: plan.ActivePath},
		DeclaredName: plan.DeclaredName, BackendExecutable: executable, VerifyOnly: true}
}

func copilotUserLiveDocument(request domain.ActivationRequest) []byte {
	identity := request.DeclaredName + "@" + copilotUserLiveExpectedMarketplace(request.Plan.PhysicalArtifactID)
	return []byte("Built-in plugins:\n  • browser-use (v0.0.4) [disabled]\n" +
		"Live Plugins (loaded from a local marketplace directory, never copied):\n  • " + identity +
		" (v" + request.Plan.DeclaredVersion + ") (enabled)\n      from " + request.Plan.ActivePath + "\n")
}

func copilotUserLiveExpectedMarketplace(physical string) string {
	sum := sha256.Sum256([]byte(physical))
	return fmt.Sprintf("agentplugins-%x", sum[:6])
}

func copilotUserLiveListStep(request domain.ActivationRequest, body []byte, tree bool) copilotUserLiveStep {
	return copilotUserLiveStep{argv: []string{request.BackendExecutable, "plugin", "list"}, tree: tree,
		result: legacyports.CommandResult{Stdout: body}}
}

func TestCopilotUserLiveBuiltinExactObservationAndOwnership(t *testing.T) {
	r := copilotUserLiveRequest(t)
	body := copilotUserLiveDocument(r)
	if copilotUserLiveExpectedMarketplace(r.Plan.PhysicalArtifactID) != "agentplugins-b73014d8eb6c" {
		t.Fatal("independent fixed marketplace identity changed")
	}
	if len(copilotUserLiveBuiltinPrefix) != 56 || fmt.Sprintf("%x", sha256.Sum256([]byte(copilotUserLiveBuiltinPrefix))) != "e83a8da5c004a1bfd6aa6645aacbe882ff506bf311b6a6c8c34833533e25caa2" {
		t.Fatal("independently authored fixed measured prefix changed")
	}
	if !copilotUserLiveActivation(body, nil, r) {
		t.Fatal("held exact observation refused")
	}
	for _, owned := range []bool{false, true} {
		runner := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{copilotUserLiveListStep(r, body, true)}}
		var managed *domain.ClientBinding
		want := clients.RegistryCollision
		if owned {
			managed = &domain.ClientBinding{ClientID: "copilot", Scope: "user", PhysicalArtifact: r.Plan.PhysicalArtifactID, TargetLocator: r.Plan.ActivePath}
			want = clients.RegistryExpected
		}
		got, err := InspectCopilotRegistry(context.Background(), clients.Env{Runner: runner}, r.Plan, managed)
		if err != nil || got != want || runner.calls != 1 {
			t.Fatalf("owned=%v finding=%v err=%v calls=%d", owned, got, err, runner.calls)
		}
	}
}

func TestCopilotUserLiveBuiltinInitialAndReadOnlyActivation(t *testing.T) {
	for _, verifyOnly := range []bool{false, true} {
		r := copilotUserLiveRequest(t)
		r.VerifyOnly = verifyOnly
		steps := []copilotUserLiveStep{}
		if !verifyOnly {
			steps = append(steps, copilotUserLiveStep{argv: []string{r.BackendExecutable, "plugin", "marketplace", "add", r.Plan.ActivePath}},
				copilotUserLiveStep{argv: []string{r.BackendExecutable, "plugin", "install", r.DeclaredName + "@" + copilotUserLiveExpectedMarketplace(r.Plan.PhysicalArtifactID)}})
		}
		steps = append(steps, copilotUserLiveListStep(r, copilotUserLiveDocument(r), false))
		runner := &copilotUserLiveRunner{t: t, steps: steps}
		got, err := CompleteCopilotActivation(context.Background(), clients.Env{Runner: runner}, r, StartedActivation(r))
		if err != nil || got.Activation != domain.ActivationActive || got.Verification != domain.VerificationInstalled || got.Authentication != domain.AuthenticationNotChecked || got.Policy != domain.PolicyAllowed || got.ActivationAttested || runner.calls != len(steps) {
			t.Fatalf("verifyOnly=%v outcome=%+v err=%v calls=%d", verifyOnly, got, err, runner.calls)
		}
	}
}

func TestCopilotUserLiveBuiltinKeepsCoherentVSCodeAndExplicitAttestationSeparate(t *testing.T) {
	for _, attested := range []bool{false, true} {
		r := copilotUserLiveRequest(t)
		r.Client.ClientID, r.Plan.ClientID, r.Delivery.ClientID = domain.ClientVSCode, domain.ClientVSCode, domain.ClientVSCode
		r.ActivationComplete = attested
		body := copilotUserLiveDocument(r)
		if copilotUserLiveActivation(body, nil, r) {
			t.Fatal("coherent VS Code request reached new Copilot-only recognition")
		}
		runner := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{copilotUserLiveListStep(r, body, false)}}
		outcome, err := CompleteCopilotActivation(context.Background(), clients.Env{Runner: runner}, r, StartedActivation(r))
		want := domain.ActivationManual
		if attested {
			want = domain.ActivationActive
		}
		if err != nil || outcome.Activation != want || outcome.ActivationAttested != attested || runner.calls != 1 {
			t.Fatal("old explicit attestation/manual distinction changed", outcome, err)
		}
	}
}

func copilotUserLiveAlteredDocuments(request domain.ActivationRequest) map[string]string {
	body := string(copilotUserLiveDocument(request))
	parent, leaf := filepath.Dir(request.Plan.ActivePath), filepath.Base(request.Plan.ActivePath)
	pathLine := "      from " + request.Plan.ActivePath + "\n"
	foreign := filepath.Join(parent, "foreign", leaf)
	unclean := parent + string(filepath.Separator) + "." + string(filepath.Separator) + leaf
	return map[string]string{
		"builtin-enabled":       strings.ReplaceAll(body, "[disabled]", "[enabled]"),
		"builtin-unknown":       strings.ReplaceAll(body, "browser-use", "other-builtin"),
		"builtin-version":       strings.ReplaceAll(body, "v0.0.4", "v0.0.5"),
		"builtin-state-missing": strings.ReplaceAll(body, " [disabled]", ""),
		"live-disabled":         strings.ReplaceAll(body, "(enabled)", "(disabled)"),
		"live-unknown-status":   strings.ReplaceAll(body, "(enabled)", "(pending)"),
		"live-version":          strings.ReplaceAll(body, "v1.2.3", "v1.2.4"),
		"live-name":             strings.ReplaceAll(body, "demo@", "other@"),
		"live-marketplace":      strings.ReplaceAll(body, "@agentplugins-", "@other-"),
		"live-path":             strings.Replace(body, pathLine, "      from "+foreign+"\n", 1),
		"relative-path":         strings.Replace(body, pathLine, "      from "+filepath.Join("managed", leaf)+"\n", 1),
		"unclean-path":          strings.Replace(body, pathLine, "      from "+unclean+"\n", 1),
		"extra-heading":         "Built-in plugins:\n" + body,
		"extra-builtin":         strings.ReplaceAll(body, "Live Plugins", "  • browser-use (v0.0.4) [disabled]\nLive Plugins"),
		"extra-entry":           body + "  • other@foreign (v1.0.0) (enabled)\n      from " + foreign + "\n",
		"duplicate-entry":       body + body[strings.Index(body, "  • demo@"):],
		"wrong-heading":         strings.ReplaceAll(body, "Live Plugins", "Live Plugin"),
		"leading-junk":          "unexpected\n" + body,
		"trailing-junk":         body + "unexpected\n",
		"extra-lf":              body + "\n",
		"missing-lf":            strings.TrimSuffix(body, "\n"),
		"crlf":                  strings.ReplaceAll(body, "\n", "\r\n"),
		"spacing":               strings.ReplaceAll(body, "      from ", "     from "),
		"ansi":                  "\x1b[32m" + body,
		"nul":                   body + "\x00",
		"oversized":             body + strings.Repeat("x", 8<<10),
		"empty":                 "",
	}
}

func TestCopilotUserLiveBuiltinWholeByteRefusalsKeepManualAndIndeterminate(t *testing.T) {
	r := copilotUserLiveRequest(t)
	for name, altered := range copilotUserLiveAlteredDocuments(r) {
		t.Run(name, func(t *testing.T) {
			body := []byte(altered)
			if altered == string(copilotUserLiveDocument(r)) {
				t.Fatal("negative whole-document fixture did not mutate its original bytes")
			}
			if copilotUserLiveActivation(body, nil, r) {
				t.Fatal("altered whole observation admitted")
			}
			inspect := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{copilotUserLiveListStep(r, body, true)}}
			finding, err := InspectCopilotRegistry(context.Background(), clients.Env{Runner: inspect}, r.Plan, nil)
			if err != nil || finding != clients.RegistryIndeterminate || inspect.calls != 1 {
				t.Fatal("altered prefixed observation grants registry authority", finding, err)
			}
			verify := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{copilotUserLiveListStep(r, body, false)}}
			outcome, err := CompleteCopilotActivation(context.Background(), clients.Env{Runner: verify}, r, StartedActivation(r))
			if err != nil || outcome.Activation != domain.ActivationManual || outcome.Verification != domain.VerificationPackageValid || verify.calls != 1 {
				t.Fatal("unknown observation lost manual fallback", outcome, err)
			}
		})
	}
}

func copilotUserLiveContextMutations() map[string]func(*domain.ActivationRequest) {
	return map[string]func(*domain.ActivationRequest){
		"vscode-plan":     func(r *domain.ActivationRequest) { r.Plan.ClientID = domain.ClientVSCode },
		"other-plan":      func(r *domain.ActivationRequest) { r.Plan.ClientID = domain.ClientCline },
		"empty-plan-id":   func(r *domain.ActivationRequest) { r.Plan.ClientID = "" },
		"vscode-client":   func(r *domain.ActivationRequest) { r.Client.ClientID = domain.ClientVSCode },
		"vscode-delivery": func(r *domain.ActivationRequest) { r.Delivery.ClientID = domain.ClientVSCode },
		"project":         func(r *domain.ActivationRequest) { r.Plan.Scope = domain.ScopeProject },
		"local":           func(r *domain.ActivationRequest) { r.Plan.Scope = domain.ScopeLocal },
		"empty-scope":     func(r *domain.ActivationRequest) { r.Plan.Scope = "" },
		"project-context": func(r *domain.ActivationRequest) { r.Plan.TargetContext.Scope = domain.ScopeProject },
		"local-context":   func(r *domain.ActivationRequest) { r.Plan.TargetContext.Scope = domain.ScopeLocal },
		"project-root":    func(r *domain.ActivationRequest) { r.Plan.TargetContext.ProjectRoot = filepath.Dir(r.Plan.ActivePath) },
		"request-name":    func(r *domain.ActivationRequest) { r.DeclaredName = "other" },
		"plan-name":       func(r *domain.ActivationRequest) { r.Plan.DeclaredName = "other" },
		"backend": func(r *domain.ActivationRequest) {
			r.BackendExecutable = filepath.Join(filepath.Dir(r.BackendExecutable), "other-copilot")
		},
		"detected-executable": func(r *domain.ActivationRequest) {
			r.Client.ExecutablePath = filepath.Join(filepath.Dir(r.Client.ExecutablePath), "other-copilot")
		},
		"registry-executable": func(r *domain.ActivationRequest) {
			r.Plan.NativeRegistryExecutable = filepath.Join(filepath.Dir(r.Plan.NativeRegistryExecutable), "other-copilot")
		},
		"delivery-path": func(r *domain.ActivationRequest) {
			r.Delivery.ActivePath = filepath.Join(filepath.Dir(r.Plan.ActivePath), "other", r.Plan.PhysicalArtifactID)
		},
		"physical-id": func(r *domain.ActivationRequest) { r.Plan.PhysicalArtifactID = "other-owned" },
	}
}

func TestCopilotUserLiveBuiltinRequiresHeldActivationContext(t *testing.T) {
	baseline := copilotUserLiveRequest(t)
	body := copilotUserLiveDocument(baseline)
	if !copilotUserLiveActivation(body, nil, baseline) {
		t.Fatal("held context baseline must be positive before any mutation")
	}
	for name, mutate := range copilotUserLiveContextMutations() {
		t.Run(name, func(t *testing.T) {
			r := baseline
			mutate(&r)
			if reflect.DeepEqual(r, baseline) {
				t.Fatal("negative context fixture did not alter the held request")
			}
			if copilotUserLiveActivation(body, nil, r) {
				t.Fatal("unheld activation context admitted")
			}
			runner := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{copilotUserLiveListStep(r, body, false)}}
			if err := VerifyCopilotPlugin(context.Background(), clients.Env{Runner: runner}, r); !errors.Is(err, ErrCopilotListContractUnknown) || runner.calls != 1 {
				t.Fatal("unheld prefixed listing unexpectedly verified", err)
			}
		})
	}
}

func TestCopilotUserLiveBuiltinRejectsWrongOwnedBinding(t *testing.T) {
	r := copilotUserLiveRequest(t)
	mutations := []func(*domain.ClientBinding){
		func(b *domain.ClientBinding) { b.ClientID = "vscode" },
		func(b *domain.ClientBinding) { b.Scope = "project" },
		func(b *domain.ClientBinding) { b.PhysicalArtifact = "other-owned" },
		func(b *domain.ClientBinding) {
			b.TargetLocator = filepath.Join(filepath.Dir(r.Plan.ActivePath), "foreign", r.Plan.PhysicalArtifactID)
		},
	}
	for index, mutate := range mutations {
		binding := domain.ClientBinding{ClientID: "copilot", Scope: "user", PhysicalArtifact: r.Plan.PhysicalArtifactID, TargetLocator: r.Plan.ActivePath}
		mutate(&binding)
		runner := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{copilotUserLiveListStep(r, copilotUserLiveDocument(r), true)}}
		finding, err := InspectCopilotRegistry(context.Background(), clients.Env{Runner: runner}, r.Plan, &binding)
		if err != nil || finding != clients.RegistryIndeterminate || runner.calls != 1 {
			t.Fatalf("wrong owned binding %d grants finding=%v err=%v", index, finding, err)
		}
	}
}

func TestCopilotUserLiveBuiltinIdentityAndByteBounds(t *testing.T) {
	for _, name := range []string{"", " demo", "demo\n", "browser-use", "BROWSER-USE", "Browser-Use", "é", strings.Repeat("d", 257)} {
		r := copilotUserLiveRequest(t)
		r.DeclaredName, r.Plan.DeclaredName = name, name
		if copilotUserLiveActivation(copilotUserLiveDocument(r), nil, r) {
			t.Fatal("invalid or same-builtin declared name admitted", name)
		}
	}
	for _, version := range []string{"*", "1.2", "1.2.3\nextra", strings.Repeat("1", 257)} {
		r := copilotUserLiveRequest(t)
		r.Plan.DeclaredVersion = version
		if copilotUserLiveActivation(copilotUserLiveDocument(r), nil, r) {
			t.Fatal("invalid or oversized exact version admitted")
		}
	}
	baseline := copilotUserLiveRequest(t)
	paths := append(copilotUserLiveInvalidPaths(t, baseline.Plan.ActivePath), filepath.VolumeName(baseline.Plan.ActivePath)+string(filepath.Separator))
	for _, path := range paths {
		r := baseline
		r.Plan.ActivePath, r.Delivery.ActivePath = path, path
		if copilotUserLiveActivation(copilotUserLiveDocument(r), nil, r) {
			t.Fatal("unheld/canonicality/oversized managed path admitted")
		}
	}
}

func copilotUserLiveSizedPath(t *testing.T, root, leaf string, size int) string {
	t.Helper()
	separator := string(filepath.Separator)
	prefix := filepath.VolumeName(root) + separator
	padding := size - len(prefix) - len(leaf)
	if padding < 3 {
		t.Fatal("inert long-path fixture has insufficient component space")
	}
	middle := strings.Repeat("q"+separator, padding/2)
	if padding%2 != 0 {
		middle = "qq" + separator + strings.Repeat("q"+separator, (padding-3)/2)
	}
	path := prefix + middle + leaf
	if len(path) != size || !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) != leaf {
		t.Fatal("inert long-path fixture is not exact-sized/native/canonical")
	}
	return path
}

func copilotUserLiveInvalidPaths(t *testing.T, path string) []string {
	t.Helper()
	separator := string(filepath.Separator)
	parent, leaf := filepath.Dir(path), filepath.Base(path)
	return []string{"", filepath.Join("relative", leaf),
		parent + separator + "." + separator + leaf,
		parent + separator + "bad\x00" + separator + leaf,
		parent + separator + "bad\r" + separator + leaf,
		parent + separator + "bad\n" + separator + leaf,
		copilotUserLiveSizedPath(t, path, leaf, 4097)}
}

func TestCopilotUserLiveBuiltinPositiveDeclaredBoundsAndDefaultContext(t *testing.T) {
	r := copilotUserLiveRequest(t)
	r.Plan.TargetContext.Scope = ""
	r.DeclaredName, r.Plan.DeclaredName = strings.Repeat("d", 256), strings.Repeat("d", 256)
	r.Plan.PhysicalArtifactID = strings.Repeat("p", 255)
	r.Plan.ActivePath = copilotUserLiveSizedPath(t, r.Plan.ActivePath, r.Plan.PhysicalArtifactID, 4096)
	r.Delivery.ActivePath = r.Plan.ActivePath
	if len(r.Plan.ActivePath) != 4096 || !copilotUserLiveActivation(copilotUserLiveDocument(r), nil, r) {
		t.Fatal("canonical boundary-sized positive with default user context refused")
	}
	r.Plan.ActivePath = copilotUserLiveSizedPath(t, r.Plan.ActivePath, r.Plan.PhysicalArtifactID, 4097)
	r.Delivery.ActivePath = r.Plan.ActivePath
	if copilotUserLiveActivation(copilotUserLiveDocument(r), nil, r) {
		t.Fatal("4097-byte path admitted")
	}
	if !copilotUserLiveName(strings.Repeat("p", 256)) || copilotUserLiveName(strings.Repeat("p", 257)) {
		t.Fatal("physical namespace byte boundary changed")
	}
}

func TestCopilotUserLiveBuiltinExecutableMustStayHeldAndCanonical(t *testing.T) {
	baseline := copilotUserLiveRequest(t)
	for _, executable := range copilotUserLiveInvalidPaths(t, baseline.BackendExecutable) {
		r := baseline
		r.Plan.NativeRegistryExecutable, r.Client.ExecutablePath, r.BackendExecutable = executable, executable, executable
		if copilotUserLiveActivation(copilotUserLiveDocument(r), nil, r) {
			t.Fatal("noncanonical or oversized held executable admitted")
		}
	}
}

func TestCopilotUserLiveBuiltinLegacyFallbackResultsRemainExact(t *testing.T) {
	r := copilotUserLiveRequest(t)
	identity := r.DeclaredName + "@" + copilotUserLiveExpectedMarketplace(r.Plan.PhysicalArtifactID)
	measured := copilotUserLiveDocument(r)
	if status, recognized := CopilotLivePluginStatus(measured, identity, r.Plan.DeclaredVersion, r.Plan.ActivePath); recognized || status != CopilotStatusUnknown {
		t.Fatal("generic live grammar broadened")
	}
	if CopilotPluginStatus(measured, identity, r.Plan.DeclaredVersion, r.Plan.ActivePath) != CopilotStatusUnknown {
		t.Fatal("generic measured-document status changed")
	}
	legacy := "Installed plugins:\n  • " + identity + " (v1.2.3)\n"
	for _, body := range [][]byte{[]byte(legacy), append(append([]byte{}, measured...), []byte(legacy)...), []byte(string(measured) + strings.Repeat("x", 8<<10) + "\n" + legacy)} {
		copilotUserLiveLegacyVerify(t, r, body)
	}
	if CopilotRegistryFindingAt(append(measured, []byte(legacy)...), r.DeclaredName, copilotUserLiveExpectedMarketplace(r.Plan.PhysicalArtifactID), r.Plan.DeclaredVersion, r.Plan.ActivePath, true) != clients.RegistryIndeterminate {
		t.Fatal("old registry preamble refusal changed")
	}
}

func copilotUserLiveLegacyVerify(t *testing.T, request domain.ActivationRequest, body []byte) {
	t.Helper()
	if copilotUserLiveActivation(body, []byte("old diagnostic"), request) {
		t.Fatal("new branch accepted independent legacy grammar")
	}
	identity := request.DeclaredName + "@" + copilotUserLiveExpectedMarketplace(request.Plan.PhysicalArtifactID)
	if CopilotPluginStatus(body, identity, request.Plan.DeclaredVersion, request.Plan.ActivePath) != CopilotStatusInstalled {
		t.Fatal("old Installed grammar no longer skips its preamble")
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	step := copilotUserLiveListStep(request, body, false)
	step.result.Stderr, step.cancel = []byte("old diagnostic"), cancel
	runner := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{step}}
	if err := VerifyCopilotPlugin(ctx, clients.Env{Runner: runner}, request); err != nil || runner.calls != 1 {
		t.Fatal("preserved legacy verification stream/context behavior changed", err)
	}
}

func TestCopilotUserLiveBuiltinUnexpectedStderrDoesNotGrantNewAuthority(t *testing.T) {
	r := copilotUserLiveRequest(t)
	for _, tree := range []bool{false, true} {
		step := copilotUserLiveListStep(r, copilotUserLiveDocument(r), tree)
		step.result.Stderr = []byte("unexpected")
		runner := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{step}}
		if tree {
			finding, err := InspectCopilotRegistry(context.Background(), clients.Env{Runner: runner}, r.Plan, nil)
			if err != nil || finding != clients.RegistryIndeterminate {
				t.Fatal("diagnostic stream granted registry authority", finding, err)
			}
		} else if err := VerifyCopilotPlugin(context.Background(), clients.Env{Runner: runner}, r); !errors.Is(err, ErrCopilotListContractUnknown) {
			t.Fatal("new prefixed verification accepted diagnostic stream", err)
		}
		if runner.calls != 1 {
			t.Fatal("unexpected repeated native command")
		}
	}
}

func TestCopilotUserLiveBuiltinCommandAndOriginalContextFailures(t *testing.T) {
	for _, tree := range []bool{false, true} {
		for _, kind := range []string{"runner", "exit", "canceled-before", "canceled-after"} {
			copilotUserLiveFailure(t, tree, kind)
		}
	}
}

func copilotUserLiveFailure(t *testing.T, tree bool, kind string) {
	t.Helper()
	r := copilotUserLiveRequest(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	step := copilotUserLiveListStep(r, copilotUserLiveDocument(r), tree)
	problem := errors.New("inert native command failure")
	wantCalls := 1
	switch kind {
	case "runner":
		step.err = problem
	case "exit":
		step.result.ExitCode = 7
	case "canceled-before":
		cancel()
		if tree {
			wantCalls = 0
		}
	case "canceled-after":
		step.cancel = cancel
	}
	runner := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{step}}
	var err error
	if tree {
		var finding clients.RegistryFinding
		finding, err = InspectCopilotRegistry(ctx, clients.Env{Runner: runner}, r.Plan, nil)
		if finding != clients.RegistryIndeterminate {
			t.Fatal("failed command/context granted a registry finding", finding)
		}
	} else {
		err = VerifyCopilotPlugin(ctx, clients.Env{Runner: runner}, r)
	}
	if err == nil || runner.calls != wantCalls {
		t.Fatalf("tree=%v kind=%s err=%v calls=%d", tree, kind, err, runner.calls)
	}
	if kind == "runner" && !errors.Is(err, problem) || strings.HasPrefix(kind, "canceled-") && !errors.Is(err, context.Canceled) {
		t.Fatal("original command/context error lost", err)
	}
}
