package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cursor"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/cursorhooks"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type physicalEditor struct {
	*codex.Adapter
	preflight func(*domain.ProfileAuthority)
	verifier  func(*domain.ProfileAuthority)
}

func (a physicalEditor) PreflightActivation(_ clients.Env, r domain.ActivationRequest) error {
	if a.preflight != nil {
		a.preflight(r.Client.ProfileAuthority)
	}
	return nil
}
func (a physicalEditor) VerifierAvailable(c domain.DetectedClient, p domain.DeliveryPlan, exe string) bool {
	if a.verifier != nil {
		a.verifier(c.ProfileAuthority)
	}
	return a.Adapter.VerifierAvailable(c, p, exe)
}

func (a physicalEditor) CaptureProfileAuthority(ctx context.Context, c domain.DetectedClient) (domain.ProfileAuthority, error) {
	return profileauthority.Capture(ctx, c.ConfigRoot)
}
func (a physicalEditor) RevalidateProfileAuthority(ctx context.Context, _ domain.ClientID, p domain.ProfileAuthority) error {
	return profileauthority.Revalidate(ctx, p)
}
func physicalEngine(t *testing.T) (*Engine, Request) {
	t.Helper()
	root := physicalTempDir(t)
	profile := filepath.Join(root, "TEST-profile")
	pkg := filepath.Join(root, "TEST-package")
	for _, dir := range []string{profile, filepath.Join(pkg, "skills", "TEST")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(pkg, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"physical-test","version":"1.0.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "skills", "TEST", "SKILL.md"), []byte("---\nname: TEST\ndescription: TEST fixture\n---\nTEST\n"), 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(physicalEditor{Adapter: codex.New()})
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	writePackage(t, pkg, helper)
	e, err := New(Config{StateRoot: filepath.Join(root, "TEST-state"), HelperExecutable: helper, Registry: registry, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	return e, Request{Operation: OpInstall, PackageRoot: pkg, ClientID: "codex", ClientConfigRoot: profile, ClientExecutable: helper, InstallationID: "00000000-0000-4000-8000-000000000092", OperationID: "TEST-operation"}
}

// Regression: absent profile/ancestor reaches snapshot creation; reservation must remain inert.
func TestPhysicalProfileBootstrapBeforePrepare(t *testing.T) {
	for _, missing := range []string{"profile", "ancestor"} {
		t.Run(missing, func(t *testing.T) {
			e, req := physicalEngine(t)
			req.ClientConfigRoot = filepath.Join(filepath.Dir(req.ClientConfigRoot), "TEST-missing", missing)
			if _, err := e.ReserveIdentity(IdentityRequest{ClientID: req.ClientID, InstallationID: req.InstallationID, DeclaredName: "sample-notify", ClientConfigRoot: req.ClientConfigRoot}); err != nil {
				t.Fatal(err)
			}
			unexpected, prepareErr := e.Prepare(t.Context(), req)
			if unexpected != nil {
				defer func() {
					if err := unexpected.Close(); err != nil {
						t.Errorf("close prepared operation: %v", err)
					}
				}()
			}
			if _, err := os.Lstat(e.cfg.StateRoot); !os.IsNotExist(err) {
				t.Fatalf("bootstrap/reservation created namespace: prepare=%v stat=%v", prepareErr, err)
			}
			if !errors.Is(prepareErr, directoryidentity.ErrBootstrapRequired) {
				t.Fatalf("missing profile did not return bootstrap refusal: %v", prepareErr)
			}
		})
	}
}

// Regression: pointer mutation/alias retarget recaptures authority or selects another owner.
func TestPhysicalProfileExactOwnerAndImmutableAlias(t *testing.T) {
	e, req := physicalEngine(t)
	canonical := req.ClientConfigRoot
	alias := canonical + "-alias"
	if err := os.Symlink(canonical, alias); err != nil {
		t.Fatal(err)
	}
	req.ClientConfigRoot = alias
	h, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := h.Close(); err != nil {
			t.Errorf("close prepared operation: %v", err)
		}
	}()
	p := h.Plan()
	if p.ProfileAuthority == nil || p.ProfileAuthority.Facts().CanonicalRoot != canonical {
		t.Fatal("prepared token did not freeze canonical spelling")
	}
	saved := *p.ProfileAuthority
	*p.ProfileAuthority = domain.ProfileAuthority{}
	other := canonical + "-other"
	if err := os.Mkdir(other, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, alias); err != nil {
		t.Fatal(err)
	}
	if !h.Plan().ProfileAuthority.Equal(saved) {
		t.Fatal("caller pointer changed prepared authority")
	}
	result, err := e.Apply(t.Context(), h, Decision{Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Binding.ProfileAuthority == nil || !result.Binding.ProfileAuthority.Equal(saved) {
		t.Fatal("committed result lost frozen token")
	}
	if err := e.VerifyProfileAuthority(t.Context(), result.InstallationID, result.Binding.BindingID); err != nil {
		t.Fatal(err)
	}
	if err := e.VerifyProfileAuthority(t.Context(), "TEST-other-installation", result.Binding.BindingID); err == nil {
		t.Fatal("another installation verified")
	}
	before, err := os.ReadFile(e.cfg.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(canonical, canonical+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(canonical, 0700); err != nil {
		t.Fatal(err)
	}
	if err := e.VerifyProfileAuthority(t.Context(), result.InstallationID, result.Binding.BindingID); err == nil {
		t.Fatal("same spelling replacement verified")
	}
	after, err := os.ReadFile(e.cfg.StateFile)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("read-only verification changed state")
	}
	foreign, err := New(Config{StateRoot: e.cfg.StateRoot + "-other", Registry: e.cfg.Registry})
	if err != nil {
		t.Fatal(err)
	}
	foreign.store = e.store
	if err := foreign.VerifyProfileAuthority(t.Context(), result.InstallationID, result.Binding.BindingID); err == nil {
		t.Fatal("foreign namespace verified recorded owner")
	}
}

// Regression: host callback profile drift reaches the following native effect or state acknowledgement.
func TestPhysicalProfileHandoffDriftRetainsCommit(t *testing.T) {
	e, req := physicalEngine(t)
	calls := 0
	e.cfg.OnCommittedBinding = func(_ context.Context, f BindingFacts) error {
		calls++
		if f.ProfileAuthority == nil {
			t.Fatal("handoff lost token")
		}
		if err := os.Rename(req.ClientConfigRoot, req.ClientConfigRoot+"-old"); err != nil {
			t.Fatal(err)
		}
		return os.Mkdir(req.ClientConfigRoot, 0700)
	}
	h, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := h.Close(); err != nil {
			t.Errorf("close prepared operation: %v", err)
		}
	}()
	result, err := e.Apply(t.Context(), h, Decision{Confirmed: true})
	if err == nil || calls != 1 {
		t.Fatalf("callback did not fence next effect: calls=%d %v", calls, err)
	}
	state, loadErr := e.store.Load()
	if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 || !result.Mutated {
		t.Fatalf("authorized package commit was lost: %+v %v", result, loadErr)
	}
}

// Regression: a late missing opted target lets an earlier selected target create snapshots.
type physicalClaude struct{ *claude.Adapter }

func (a physicalClaude) CaptureProfileAuthority(ctx context.Context, c domain.DetectedClient) (domain.ProfileAuthority, error) {
	return profileauthority.Capture(ctx, c.ConfigRoot)
}
func (a physicalClaude) RevalidateProfileAuthority(ctx context.Context, _ domain.ClientID, p domain.ProfileAuthority) error {
	return profileauthority.Revalidate(ctx, p)
}
func TestPhysicalProfileLateGroupTargetBeforePrepare(t *testing.T) {
	e, req := physicalEngine(t)
	registry, err := clients.NewRegistry(codex.New(), physicalClaude{claude.New()})
	if err != nil {
		t.Fatal(err)
	}
	e.cfg.Registry = registry
	req.Targets = []ClientTarget{{ClientID: "codex", ClientConfigRoot: req.ClientConfigRoot, ClientExecutable: req.ClientExecutable}, {ClientID: "claude", ClientConfigRoot: filepath.Join(filepath.Dir(req.ClientConfigRoot), "TEST-late-missing"), ClientExecutable: req.ClientExecutable}}
	unexpected, prepareErr := e.Prepare(t.Context(), req)
	if unexpected != nil {
		defer func() {
			if err := unexpected.Close(); err != nil {
				t.Errorf("close prepared operation: %v", err)
			}
		}()
	}
	if _, err := os.Lstat(e.cfg.StateRoot); !os.IsNotExist(err) {
		t.Fatalf("late invalid target permitted earlier namespace effects: %v", prepareErr)
	}
	if !errors.Is(prepareErr, directoryidentity.ErrBootstrapRequired) {
		t.Fatalf("late target did not refuse bootstrap: %v", prepareErr)
	}
}

// Regression: projection callback drift reaches Stage and erases already permitted EnsureData effects.
func TestPhysicalProfileProjectionDriftBeforeStage(t *testing.T) {
	e, req := physicalEngine(t)
	e.cfg.ServerName = "sample-notify"
	calls, dataRoot := 0, ""
	e.cfg.ProjectArgs = func(f BindingFacts) ([]string, error) {
		calls++
		dataRoot = f.DataRoot
		if f.ProfileAuthority == nil || dataRoot == "" {
			t.Fatal("projection lost frozen profile/data facts")
		}
		if err := os.Rename(req.ClientConfigRoot, req.ClientConfigRoot+"-old"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(req.ClientConfigRoot, 0700); err != nil {
			t.Fatal(err)
		}
		return []string{"TEST-projection"}, nil
	}
	h, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := h.Close(); err != nil {
			t.Errorf("close prepared operation: %v", err)
		}
	}()
	if _, err := e.Apply(t.Context(), h, Decision{Confirmed: true}); err == nil || calls != 1 {
		t.Fatalf("projection drift reached next effect: %d %v", calls, err)
	}
	raw, err := os.ReadFile(filepath.Join(dataRoot, ".agentplugins-data-owner.json"))
	if err != nil {
		t.Fatalf("authorized EnsureData was lost: %v", err)
	}
	var observed domain.DataReceipt
	if json.Unmarshal(raw, &observed) != nil || observed.Locator != dataRoot || observed.State != domain.DataReceiptOwned || observed.DataReceiptID == "" {
		t.Fatal("data effect has no real owner receipt")
	}
	if _, err := os.Lstat(e.cfg.StateFile); !os.IsNotExist(err) {
		t.Fatal("projection drift saved binding state")
	}
	entries, err := os.ReadDir(e.cfg.OperationsDir)
	if err != nil || len(entries) != 0 {
		t.Fatal("projection drift created intent")
	}
	managed, err := os.ReadDir(e.cfg.ManagedRoot)
	if err != nil || len(managed) != 0 {
		t.Fatal("projection drift staged or published package")
	}
}

// Regression: a callback rewrites a shared token after replacing the real profile,
// making the next EnsureData accept a newly captured owner.
func TestPhysicalProfileCallbackCannotReplaceFrozenAuthority(t *testing.T) {
	for _, phase := range []string{"preflight", "verifier"} {
		t.Run(phase, func(t *testing.T) {
			e, req := physicalEngine(t)
			armed, calls := false, 0
			mutate := func(token *domain.ProfileAuthority) {
				if !armed || token == nil || calls != 0 {
					return
				}
				calls++
				if err := os.Rename(req.ClientConfigRoot, req.ClientConfigRoot+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(req.ClientConfigRoot, 0700); err != nil {
					t.Fatal(err)
				}
				replacement, err := profileauthority.Capture(t.Context(), req.ClientConfigRoot)
				if err != nil {
					t.Fatal(err)
				}
				*token = replacement
			}
			a := physicalEditor{Adapter: codex.New()}
			if phase == "preflight" {
				a.preflight = mutate
			} else {
				a.verifier = mutate
			}
			registry, err := clients.NewRegistry(a)
			if err != nil {
				t.Fatal(err)
			}
			e.cfg.Registry = registry
			h, err := e.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := h.Close(); err != nil {
					t.Errorf("close prepared operation: %v", err)
				}
			}()
			var stateBefore []byte
			if phase == "verifier" {
				if _, err := e.Apply(t.Context(), h, Decision{Confirmed: true}); err != nil {
					t.Fatal(err)
				}
				if err := h.Close(); err != nil {
					t.Fatal(err)
				}
				req.OperationID = "TEST-verifier-resume"
				h, err = e.Prepare(t.Context(), req)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := h.Close(); err != nil {
						t.Errorf("close prepared operation: %v", err)
					}
				}()
				stateBefore, err = os.ReadFile(e.cfg.StateFile)
				if err != nil {
					t.Fatal(err)
				}
			}
			armed = true
			_, applyErr := e.Apply(t.Context(), h, Decision{Confirmed: true})
			if calls != 1 || (phase == "preflight" && applyErr == nil) {
				t.Fatalf("replacement did not fence callback: calls=%d err=%v", calls, applyErr)
			}
			if phase == "verifier" {
				after, err := os.ReadFile(e.cfg.StateFile)
				if err != nil || !bytes.Equal(stateBefore, after) {
					t.Fatal("verifier callback changed durable binding")
				}
				facts := h.Plan()
				if err := e.VerifyProfileAuthority(t.Context(), facts.InstallationID, facts.BindingID); err == nil {
					t.Fatal("verifier callback granted replacement owner")
				}
			} else {
				if entries, err := os.ReadDir(e.cfg.PluginDataBase); err != nil && !os.IsNotExist(err) || len(entries) != 0 {
					t.Fatalf("callback authorized plugin data: %v", err)
				}
				if _, err := os.Lstat(e.cfg.StateFile); !os.IsNotExist(err) {
					t.Fatal("callback authorized state Save")
				}
			}
		})
	}
}

// Regression: real death after the state commit bypasses the public recovery fence.
type physicalDeathStore struct{ transaction.StateStore }

func (s physicalDeathStore) Save(state domain.StateFileV2) error {
	if err := s.StateStore.Save(state); err != nil {
		return err
	}
	for _, i := range state.Installations {
		for _, b := range i.Clients {
			for _, r := range b.Receipts {
				if r.Phase == transaction.ReceiptPhaseStateCommitted {
					os.Exit(71)
				}
			}
		}
	}
	return nil
}
func TestPhysicalProfilePublicRecoverAfterChildDeath(t *testing.T) {
	if raw := os.Getenv("TEST_PHYSICAL_DEATH_REQUEST"); raw != "" {
		var req Request
		if err := json.Unmarshal([]byte(raw), &req); err != nil {
			t.Fatal(err)
		}
		registry, err := clients.NewRegistry(physicalEditor{Adapter: codex.New()})
		if err != nil {
			t.Fatal(err)
		}
		e, err := New(Config{StateRoot: os.Getenv("TEST_PHYSICAL_DEATH_STATE"), HelperExecutable: req.ClientExecutable, Registry: registry, TrustedLocalPackages: true})
		if err != nil {
			t.Fatal(err)
		}
		e.store = physicalDeathStore{e.store}
		h, err := e.Prepare(t.Context(), req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := h.Close(); err != nil {
				t.Errorf("close prepared operation: %v", err)
			}
		}()
		_, err = e.Apply(t.Context(), h, Decision{Confirmed: true})
		t.Fatalf("child never reached committed-state death: %v", err)
	}
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			e, req := physicalEngine(t)
			// Death bypasses snapshot Close; make only this fixture's orphan scratch removable after assertions.
			t.Cleanup(func() {
				scratch := filepath.Join(e.cfg.StateRoot, "tmp")
				if err := filepath.WalkDir(scratch, func(path string, entry os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() {
						return os.Chmod(path, 0700)
					}
					return nil
				}); err != nil {
					t.Error(err)
				}
			})
			raw, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), req.ClientExecutable, "-test.run=^TestPhysicalProfilePublicRecoverAfterChildDeath$", "-test.timeout=20s")
			cmd.Env = append(os.Environ(), "TEST_PHYSICAL_DEATH_REQUEST="+string(raw), "TEST_PHYSICAL_DEATH_STATE="+e.cfg.StateRoot)
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 71 {
				t.Fatalf("not a committed-state death: %v %s", err, output)
			}
			pending := func() bool {
				state, err := e.store.Load()
				if err != nil {
					t.Fatal(err)
				}
				for _, installation := range state.Installations {
					for _, binding := range installation.Clients {
						for _, receipt := range binding.Receipts {
							if receipt.Phase == "state_committed" && len(receipt.ProfileOwners) > 0 && len(receipt.DirectoryProof) > 0 {
								return true
							}
						}
					}
				}
				return false
			}
			before, err := os.ReadFile(e.cfg.StateFile)
			if err != nil || !pending() {
				t.Fatalf("no durable pending decision: %v", err)
			}
			observed, err := e.Inspect(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if replace {
				if err := os.Rename(req.ClientConfigRoot, req.ClientConfigRoot+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(req.ClientConfigRoot, 0700); err != nil {
					t.Fatal(err)
				}
			}
			_, err = e.Recover(t.Context(), observed)
			if (err != nil) != replace {
				t.Fatalf("public recovery replacement=%t: %v", replace, err)
			}
			after, readErr := os.ReadFile(e.cfg.StateFile)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if replace && !bytes.Equal(before, after) {
				t.Fatal("refusal altered durable state")
			}
			if !replace && pending() {
				t.Fatal("public recovery left decision pending")
			}
		})
	}
}

func physicalTempDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// This seam observes the real planner and activation-preflight inputs, while
// retaining Cursor's public preparation adapter. It does not qualify native Stop.
type physicalCompatibilityCursor struct {
	*cursor.Adapter
	observe func(string, domain.DetectedClient) error
}

func (a physicalCompatibilityCursor) CaptureProfileAuthority(ctx context.Context, c domain.DetectedClient) (domain.ProfileAuthority, error) {
	return profileauthority.Capture(ctx, c.ConfigRoot)
}
func (a physicalCompatibilityCursor) RevalidateProfileAuthority(ctx context.Context, _ domain.ClientID, p domain.ProfileAuthority) error {
	return profileauthority.Revalidate(ctx, p)
}
func (a physicalCompatibilityCursor) RefinePlan(ctx context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if err := a.observe("planner", in.Client); err != nil {
		return err
	}
	return a.Adapter.RefinePlan(ctx, in, plan)
}
func (a physicalCompatibilityCursor) PreflightActivation(_ clients.Env, r domain.ActivationRequest) error {
	return a.observe("activation-preflight", r.Client)
}

// Regression: rebuilding an observational sibling drops its original token
// before the real public Update compatibility planner or activation preflight.
func TestPhysicalProfileUpdateCompatibilityForwarding(t *testing.T) {
	e, req := physicalEngine(t)
	// Only actual full-ancestry Capture can admit the affirmative fixture.
	token, err := profileauthority.Capture(t.Context(), req.ClientConfigRoot)
	if errors.Is(err, directoryidentity.ErrUnsupported) {
		t.Skipf("NOT_RUN positive compatibility forwarding: full ancestry unsupported: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("physical authority ancestry: %+v", token.Facts())
	cursorRoot := filepath.Join(filepath.Dir(req.ClientConfigRoot), "TEST-cursor")
	if err := os.Mkdir(cursorRoot, 0700); err != nil {
		t.Fatal(err)
	}
	var expected *domain.ProfileAuthority
	var namespace string
	calls := map[string]int{}
	observe := func(phase string, c domain.DetectedClient) error {
		if expected == nil { // Initial public installation establishes the owner.
			return nil
		}
		if c.ProfileAuthority == nil || !c.ProfileAuthority.Equal(*expected) || c.ProfileNamespace != namespace {
			return fmt.Errorf("%s lost original recorded Cursor authority/namespace", phase)
		}
		if err := profileauthority.Revalidate(t.Context(), *c.ProfileAuthority); err != nil {
			return err
		}
		calls[phase]++
		return nil
	}
	registry, err := clients.NewRegistry(physicalEditor{Adapter: codex.New()}, physicalCompatibilityCursor{cursor.New(), observe})
	if err != nil {
		t.Fatal(err)
	}
	e.cfg.Registry = registry
	install := func(r Request) {
		t.Helper()
		h, err := e.Prepare(t.Context(), r)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := h.Close(); err != nil {
				t.Error(err)
			}
		}()
		if _, err := e.Apply(t.Context(), h, Decision{Confirmed: true}); err != nil {
			t.Fatal(err)
		}
	}
	install(req)
	cursorReq := req
	cursorReq.ClientID, cursorReq.ClientConfigRoot, cursorReq.OperationID = "cursor", cursorRoot, "TEST-cursor-install"
	install(cursorReq)
	state, err := e.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installation, ok := findInstall(state, req.InstallationID)
	if !ok {
		t.Fatal("public install was not persisted")
	}
	cursorBefore, _, ok := findBinding(installation, domain.ClientCursor)
	if !ok || cursorBefore.ProfileAuthority == nil {
		t.Fatal("public install did not record Cursor authority")
	}
	expected, namespace = domain.CloneProfileAuthority(cursorBefore.ProfileAuthority), cursorBefore.ProfileNamespace
	if namespace != e.cfg.StateRoot {
		t.Fatal("recorded namespace differs")
	}
	// Candidate r2 is independent input bytes; only Codex is selected for Update.
	if err := os.WriteFile(filepath.Join(req.PackageRoot, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sample-notify","version":"1.0.1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	req.Operation, req.OperationID = OpUpdate, "TEST-codex-update"
	req.KnownTargets = []TargetFacts{{ClientID: "cursor", BindingID: cursorBefore.ClientBindingID, ConfigRoot: cursorRoot, Executable: req.ClientExecutable}}
	h, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := h.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, phase := range []string{"planner", "activation-preflight"} {
		if calls[phase] == 0 {
			t.Fatalf("public Update never reached sibling %s", phase)
		}
	}
	// A boundary clone must not alias the Store.Load result.
	check, err := e.compatibilityBindingCheck(cursorBefore, cursorRoot, req.ClientExecutable, usecase.AddInput{Client: h.client})
	if err != nil {
		t.Fatal(err)
	}
	if check.Client.ProfileAuthority == cursorBefore.ProfileAuthority || check.Client.ProfileAuthority == nil || !check.Client.ProfileAuthority.Equal(*expected) {
		t.Fatal("sibling forwarding did not clone the original token")
	}
	codexBefore, _, ok := findBinding(installation, domain.ClientCodex)
	if !ok {
		t.Fatal("selected binding missing")
	}
	check, err = e.compatibilityBindingCheck(codexBefore, req.ClientConfigRoot, req.ClientExecutable, usecase.AddInput{Client: h.client})
	if err != nil {
		t.Fatal(err)
	}
	if check.Client.ProfileAuthority != h.client.ProfileAuthority || !reflect.DeepEqual(check.Client, h.client) {
		t.Fatal("selected compatibility client lost prepared-client precedence")
	}
	// Prepare is an observational compatibility check, not a sibling mutation.
	after, err := e.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	current, ok := findInstall(after, req.InstallationID)
	if !ok {
		t.Fatal("installation disappeared")
	}
	cursorAfter, _, ok := findBinding(current, domain.ClientCursor)
	if !ok || !reflect.DeepEqual(cursorBefore, cursorAfter) {
		t.Fatal("compatibility preview changed Cursor r1")
	}
	t.Log("PASS positive public Update planner + activation-preflight original-token forwarding; selected prepared-client precedence")
}

func TestPhysicalProfileCompatibilityLegacyNil(t *testing.T) {
	e, req := physicalEngine(t)
	check, err := e.compatibilityBindingCheck(domain.ClientBinding{ClientID: "codex"}, req.ClientConfigRoot, req.ClientExecutable, usecase.AddInput{})
	if err != nil {
		t.Fatal(err)
	}
	if check.Client.ProfileAuthority != nil || check.Client.ProfileNamespace != "" {
		t.Fatal("legacy compatibility client gained physical authority")
	}
}

// The selected public adapter contract uses real Capture, Cursor's projector,
// pure hook planning and ExactFile acknowledgement. No editor process is run.
type physicalSelectedCursor struct {
	physicalCompatibilityCursor
	selector string
}

func (a physicalSelectedCursor) RefinePlan(ctx context.Context, in clients.PlanInput, p *domain.DeliveryPlan) error {
	if err := a.Adapter.RefinePlan(ctx, in, p); err != nil {
		return err
	}
	if in.Client.ProfileAuthority == nil {
		return fmt.Errorf("missing physical qualification")
	}
	original, err := nativeconfig.New().ReadExactFile(filepath.Join(in.Client.ConfigRoot, "hooks.json"))
	if err != nil {
		return err
	}
	if info, err := os.Lstat(in.Client.ExecutablePath); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("TEST executable is not a regular file: %v", err)
	}
	p.SelectedDelivery, err = physicalCursorSelection(in.Client.ConfigRoot, in.Client.ExecutablePath, a.selector, in.Envelope.TreeDigest, original.Body, original.Exists)
	return err
}

func physicalCursorSelection(root, executable, selector, digest string, raw []byte, exists bool) (domain.SelectedDelivery, error) {
	planned, err := cursorhooks.Plan(cursorhooks.Request{Operation: cursorhooks.Install, Document: raw, Shell: cursorhooks.LinuxUserShell32212, ExecutableVerified: true, Specs: []cursorhooks.HookSpec{{Executable: executable, Selector: selector}}})
	if err != nil {
		return domain.SelectedDelivery{}, err
	}
	r := planned.Receipt
	receipt := domain.CursorHookReceipt{Version: r.Version, Event: r.Event, Executable: r.Spec.Executable, Selector: r.Spec.Selector, Shell: string(r.Shell), EntryDigest: r.EntryDigest, RemainderDigest: r.RemainderDigest}
	return domain.NewCursorDelivery(domain.CursorDeliveryFacts{ProfileRoot: root, HooksPath: filepath.Join(root, "hooks.json"), ProfileIdentity: "TEST-physical", CursorVersion: "2026.09.28-64d2043", TargetOS: "linux", TargetArch: "amd64", QualificationID: "TEST-projection-contract", Executable: executable, Selector: selector, Shell: receipt.Shell, ObjectID: "TEST-stop", EntryDigest: receipt.EntryDigest, CanonicalDigest: digest, PlannedReceipt: receipt, OriginalExists: exists, OriginalRawDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(raw))})
}

func (a physicalSelectedCursor) Activate(ctx context.Context, _ clients.Env, r domain.ActivationRequest) (domain.ActivationOutcome, error) {
	f, ok := r.Plan.SelectedDelivery.CursorFacts()
	if !ok || r.Client.ProfileAuthority == nil {
		return domain.ActivationOutcome{}, fmt.Errorf("missing selection/token")
	}
	if err := profileauthority.Revalidate(ctx, *r.Client.ProfileAuthority); err != nil {
		return domain.ActivationOutcome{}, err
	}
	file, err := nativeconfig.New().BeginExactFile(f.HooksPath)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	defer file.Close()
	original := file.Original()
	if original.Exists != f.OriginalExists || fmt.Sprintf("sha256:%x", sha256.Sum256(original.Body)) != f.OriginalRawDigest {
		return domain.ActivationOutcome{}, fmt.Errorf("hook basis changed")
	}
	planned, err := cursorhooks.Plan(cursorhooks.Request{Operation: cursorhooks.Install, Document: original.Body, Shell: cursorhooks.LinuxUserShell32212, ExecutableVerified: true, Specs: []cursorhooks.HookSpec{{Executable: f.Executable, Selector: f.Selector}}})
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := file.Apply(planned.Desired); err != nil {
		return domain.ActivationOutcome{}, errors.Join(err, file.Rollback())
	}
	current, err := nativeconfig.New().ReadExactFile(f.HooksPath)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := cursorhooks.VerifyOwned(current.Body, planned.Receipt); err != nil {
		return domain.ActivationOutcome{}, err
	}
	receipt := f.PlannedReceipt
	if receipt.EntryDigest != planned.Receipt.EntryDigest || receipt.RemainderDigest != planned.Receipt.RemainderDigest {
		return domain.ActivationOutcome{}, fmt.Errorf("acknowledgement differs")
	}
	return domain.ActivationOutcome{Activation: domain.ActivationManual, NativeEffect: domain.NativeEffectCommitted, NativeObjects: []domain.NativeObjectOwnership{r.Plan.SelectedDelivery.CursorOwnership(receipt)}}, nil
}

// Plausible RED: selectedNativeOnly suppresses ProjectArgs, installing [] argv
// even though the host reserved a selector before Prepare. Canonical input must
// retain empty args; only the projected copy may contain the selector.
func TestPhysicalProfileSelectedCursorMCPProjection(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
			t.Skip("NOT_RUN selected Cursor projection: fixed Linux amd64 TEST tuple required")
		}
		e, req := physicalEngine(t)
		token, err := profileauthority.Capture(t.Context(), req.ClientConfigRoot)
		if errors.Is(err, directoryidentity.ErrUnsupported) {
			t.Skipf("NOT_RUN selected Cursor Prepare/Apply/Store: full ancestry unsupported: %v", err)
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("observed full ancestry: %+v", token.Facts())
		if err := os.WriteFile(filepath.Join(req.ClientConfigRoot, "hooks.json"), []byte(`{"version":1,"hooks":{}}`), 0600); err != nil {
			t.Fatal(err)
		}
		req.ClientID, req.ClientExecutable = "cursor", filepath.Join(req.PackageRoot, "bin", "probe")
		selector := filepath.Join(filepath.Dir(req.ClientConfigRoot), "TEST-selector")
		a := physicalSelectedCursor{physicalCompatibilityCursor: physicalCompatibilityCursor{Adapter: cursor.New(), observe: func(_ string, c domain.DetectedClient) error {
			if c.ProfileAuthority == nil || !c.ProfileAuthority.Equal(token) || c.ProfileNamespace != e.cfg.StateRoot {
				return fmt.Errorf("original token/namespace lost")
			}
			return profileauthority.Revalidate(t.Context(), *c.ProfileAuthority)
		}}, selector: selector}
		e.cfg.Registry, err = clients.NewRegistry(a)
		if err != nil {
			t.Fatal(err)
		}
		reservation, err := e.ReserveIdentity(IdentityRequest{ClientID: req.ClientID, InstallationID: req.InstallationID, DeclaredName: "sample-notify", ClientConfigRoot: req.ClientConfigRoot})
		if err != nil || reservation.TargetPath == "" {
			t.Fatalf("reservation: %+v %v", reservation, err)
		}
		data := filepath.Join(e.cfg.PluginDataBase, domain.ComputePhysicalArtifactID("sample-notify", req.InstallationID))
		canonical, err := e.LocalPackageTreeDigest(t.Context(), req.PackageRoot)
		if err != nil {
			t.Fatal(err)
		}
		originalBytes := physicalByteDigest(t, req.PackageRoot)
		manifest, err := os.ReadFile(filepath.Join(req.PackageRoot, "plugin.json"))
		if err != nil {
			t.Fatal(err)
		}
		manifestDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(manifest))
		originalMCP, err := os.ReadFile(filepath.Join(req.PackageRoot, "mcp.json"))
		if err != nil || !bytes.Contains(originalMCP, []byte(`"args":[]`)) {
			t.Fatalf("canonical args must start empty: %s %v", originalMCP, err)
		}
		calls := 0
		e.cfg.ServerName = "sample-notify"
		e.cfg.ProjectArgs = func(f BindingFacts) ([]string, error) {
			calls++
			if f.DataRoot != data || f.TargetPath != reservation.TargetPath || f.BindingID != reservation.BindingID || f.TreeDigest != canonical || f.ProfileAuthority == nil || !f.ProfileAuthority.Equal(token) {
				return nil, fmt.Errorf("foreign data/selector/token")
			}
			// Mutating the callback's clone cannot replace the frozen profile token.
			*f.ProfileAuthority = domain.ProfileAuthority{}
			return []string{"--binding", selector}, nil
		}
		h, err := e.Prepare(t.Context(), req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := h.Close(); err != nil {
				t.Error(err)
			}
		}()
		before := h.Plan().SelectedDelivery
		result, err := e.Apply(t.Context(), h, Decision{Confirmed: true})
		if err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("selected projection callback calls=%d, want 1", calls)
		}
		state, err := e.store.Load()
		if err != nil {
			t.Fatal(err)
		}
		installation, ok := findInstall(state, req.InstallationID)
		if !ok {
			t.Fatal("real Store commit missing")
		}
		b, _, ok := findBinding(installation, domain.ClientCursor)
		if !ok || b.PackageRevision == nil || b.PackageRevision.TreeDigest != canonical || b.PackageRevision.ManifestDigest != manifestDigest || b.SelectedDelivery.CanonicalDigest() != canonical {
			t.Fatal("selected canonical revision changed")
		}
		if b.ProfileAuthority == nil || !b.ProfileAuthority.Equal(token) || b.ProfileNamespace != e.cfg.StateRoot {
			t.Fatal("original token/namespace changed")
		}
		expected, _ := before.CursorFacts()
		sealed, _ := b.SelectedDelivery.CursorFacts()
		expected.ProjectionDigest = physicalByteDigest(t, result.Binding.TargetPath)
		if expected != sealed || sealed.ProjectionDigest == canonical {
			t.Fatal("seal lost original packet or projection byte digest")
		}
		if physicalByteDigest(t, req.PackageRoot) != originalBytes {
			t.Fatal("canonical tree mutated")
		}
		actual, err := os.ReadFile(filepath.Join(result.Binding.TargetPath, "mcp.json"))
		if err != nil {
			t.Fatal(err)
		}
		var mcp struct {
			Servers map[string]struct {
				Args []string `json:"args"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal(actual, &mcp); err != nil || !reflect.DeepEqual(mcp.Servers["sample-notify"].Args, []string{"--binding", selector}) {
			t.Fatalf("installed argv bytes: %s %v", actual, err)
		}
		raw, err := os.ReadFile(filepath.Join(req.PackageRoot, "mcp.json"))
		if err != nil || !bytes.Equal(raw, originalMCP) {
			t.Fatal("canonical MCP bytes changed")
		}
		ack := false
		for _, object := range b.NativeObjects {
			if object.Kind == "cursor_user_stop" {
				ack = object.CursorReceipt == sealed.PlannedReceipt
			}
		}
		if !ack || b.SelectedDelivery.ValidateCursorObjects(b.NativeObjects) != nil {
			t.Fatal("actual hooks acknowledgement lost")
		}
		if err := e.VerifyProfileAuthority(t.Context(), req.InstallationID, b.ClientBindingID); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("refusals", func(t *testing.T) {
		root := physicalTempDir(t)
		digest := "sha256:" + fmt.Sprintf("%x", sha256.Sum256(nil))
		selected, err := physicalCursorSelection(root, filepath.Join(root, "TEST-executable"), filepath.Join(root, "TEST-selector"), digest, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		envelope := domain.PackageEnvelope{}
		envelope.MCP.Servers = map[string]domain.MCPServer{"sample-notify": {Type: "stdio", Decoded: map[string]any{"args": []any{}}}}
		good := BindingFacts{ClientID: "cursor", DataRoot: filepath.Join(root, "TEST-data"), SelectedDelivery: selected}
		calls := 0
		callback := func(BindingFacts) ([]string, error) {
			calls++
			return nil, fmt.Errorf("foreign selector/data conflict")
		}
		for _, name := range []string{"no-data", "relative-data", "root-data", "foreign-client", "missing-MCP", "non-stdio", "unknown-mode", "callback-conflict"} {
			t.Run(name, func(t *testing.T) {
				f, input := good, envelope
				switch name {
				case "no-data":
					f.DataRoot = ""
				case "relative-data":
					f.DataRoot = "TEST-relative"
				case "root-data":
					f.DataRoot = string(filepath.Separator)
				case "foreign-client":
					f.ClientID = "codex"
				case "missing-MCP":
					input.MCP.Servers = nil
				case "non-stdio":
					input.MCP.Servers = map[string]domain.MCPServer{"sample-notify": {Type: "http", Decoded: map[string]any{"url": "https://TEST.invalid"}}}
				case "unknown-mode":
					if err := json.Unmarshal([]byte(`{"mode":"cursor-unknown"}`), &f.SelectedDelivery); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := projectArgs(input, "sample-notify", callback, f); err == nil {
					t.Fatal("unsafe projection accepted")
				}
			})
		}
		if calls != 1 {
			t.Fatalf("invalid inputs reached callback: %d", calls)
		}
		f, _ := selected.CursorFacts()
		collision, err := cursorhooks.Plan(cursorhooks.Request{Operation: cursorhooks.Install, Shell: cursorhooks.LinuxUserShell32212, ExecutableVerified: true, Specs: []cursorhooks.HookSpec{{Executable: f.Executable, Selector: f.Selector}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := physicalCursorSelection(root, f.Executable, f.Selector, digest, collision.Desired, true); !errors.Is(err, cursorhooks.ErrConflict) {
			t.Fatalf("foreign selector collision accepted: %v", err)
		}
		foreignData := filepath.Join(root, "TEST-foreign-data")
		if err := os.Mkdir(foreignData, 0700); err != nil {
			t.Fatal(err)
		}
		if _, _, err := (providers.PluginDataManager{Base: root}).EnsureData(t.Context(), "TEST-installation", "TEST-foreign-data", "user"); err == nil {
			t.Fatal("foreign data without owner marker accepted")
		}
		seam := seamStager{serverName: "sample-notify", projectArgs: callback}
		plan := domain.DeliveryPlan{SelectedDelivery: selected, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "sample-notify", Support: domain.SupportPrepared}}}
		if _, err := seam.Stage(t.Context(), envelope, plan, "TEST", domain.CompatibilityHints{}); err == nil {
			t.Fatal("data-free Stage accepted selected Cursor callback")
		}
		seam.serverName = "TEST-unselected"
		if _, err := seam.StageWithPluginData(t.Context(), envelope, plan, "TEST", domain.CompatibilityHints{}, good.DataRoot); err == nil {
			t.Fatal("unselected MCP reached Stage")
		}
		plan.Components = nil
		seam.serverName = "sample-notify"
		_, _ = seam.StageWithPluginData(t.Context(), envelope, plan, "TEST", domain.CompatibilityHints{}, "")
		if calls != 1 {
			t.Fatal("Stop-only path invoked MCP callback")
		}
	})
}

// Independent implementation of the documented snapshot byte format, without
// calling Stager.Verify or the snapshot builder that produced the sealed digest.
func physicalByteDigest(t *testing.T, root string) string {
	t.Helper()
	var paths []string
	if err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if path != root {
			paths = append(paths, path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		kind, executable, size := "dir", false, int64(0)
		if !info.IsDir() {
			kind, executable, size = "file", info.Mode()&0111 != 0, info.Size()
		}
		fmt.Fprintf(hash, "%s\x00%s\x00%t\x00%d\x00", kind, filepath.ToSlash(rel), executable, size)
		if !info.IsDir() {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			hash.Write(body)
		}
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil))
}
