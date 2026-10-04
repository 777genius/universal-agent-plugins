package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
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
