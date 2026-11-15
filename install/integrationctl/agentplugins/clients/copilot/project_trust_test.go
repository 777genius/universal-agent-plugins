package copilot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func trustFixture(t *testing.T) (clients.Env, domain.ActivationRequest, string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project, profile := filepath.Join(base, "project"), filepath.Join(base, "selected-profile")
	for _, path := range []string{project, profile} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	target, err := domain.NewTargetContext(domain.ScopeProject, project, profile)
	if err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{ClientID: domain.ClientCopilot, Scope: domain.ScopeProject, TargetContext: target,
		NativeRegistryRoot: project, PhysicalArtifactID: "fixture", ActivePath: filepath.Join(project, ".uap", "managed", "clients", "copilot", "fixture")}
	return clients.Env{NativeConfig: nativeconfig.New()}, domain.ActivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientCopilot, ConfigRoot: profile}, Plan: plan}, filepath.Join(profile, "config.json")
}

func trustRead(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestProjectCopilotTrustSelectedProfileAndReadOnlyVerification(t *testing.T) {
	env, request, path := trustFixture(t)
	foreign := []byte("// retained\n{\"foreign\":9007199254740993,\"trustedFolders\":[],}\n")
	if err := os.WriteFile(path, foreign, 0600); err != nil {
		t.Fatal(err)
	}
	amb := t.TempDir()
	t.Setenv("COPILOT_HOME", amb)
	if err := verifyCopilotTrust(env, request, false); err != nil {
		t.Fatal(err)
	}
	if err := verifyCopilotTrust(env, request, true); err == nil {
		t.Fatal("missing trust verified")
	}
	if !bytes.Equal(foreign, trustRead(t, path)) {
		t.Fatal("verification mutated config")
	}
	call := func() error { return verifyCopilotTrust(env, request, true) }
	if err := withCopilotTrust(t.Context(), env, request, call); err != nil {
		t.Fatal(err)
	}
	first := trustRead(t, path)
	if !bytes.Contains(first, []byte("// retained")) || !bytes.Contains(first, []byte("9007199254740993")) {
		t.Fatal("foreign config changed")
	}
	if err := withCopilotTrust(t.Context(), env, request, call); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, trustRead(t, path)) {
		t.Fatal("repeat grant rewrote native config")
	}
	if entries, err := os.ReadDir(amb); err != nil || len(entries) != 0 {
		t.Fatal("ambient COPILOT_HOME was used")
	}
	request.Client.ConfigRoot = amb
	if err := verifyCopilotTrust(env, request, false); err == nil {
		t.Fatal("foreign selected profile admitted")
	}
}

func TestProjectCopilotTrustRollbackAbsentExistingAndCancellation(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "existing"}[existing], func(t *testing.T) {
			env, request, path := trustFixture(t)
			before := []byte("/* keep */ {\"trustedFolders\":[], \"foreign\":true}\n")
			if existing {
				if err := os.WriteFile(path, before, 0600); err != nil {
					t.Fatal(err)
				}
			}
			cause := errors.New("component failed")
			err := withCopilotTrust(t.Context(), env, request, func() error {
				if err := verifyCopilotTrust(env, request, true); err != nil {
					t.Fatal(err)
				}
				return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: cause}
			})
			var observed *shared.NativeEffectError
			if !errors.Is(err, cause) || !errors.As(err, &observed) || observed.Effect != domain.NativeEffectUnchanged {
				t.Fatalf("rollback effect: %v", err)
			}
			if existing {
				if !bytes.Equal(before, trustRead(t, path)) {
					t.Fatal("original bytes not restored")
				}
			} else if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatal("new grant survived failed components")
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if err := withCopilotTrust(ctx, env, request, func() error { t.Fatal("canceled operation ran components"); return nil }); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}

func TestProjectCopilotTrustRetainsForeignConcurrentWriteAndCommittedCleanup(t *testing.T) {
	env, request, path := trustFixture(t)
	foreign := []byte(`{"trustedFolders":[],"foreign":"concurrent"}`)
	err := withCopilotTrust(t.Context(), env, request, func() error {
		if err := os.WriteFile(path, foreign, 0600); err != nil {
			t.Fatal(err)
		}
		return errors.New("component failure")
	})
	var observed *shared.NativeEffectError
	if !errors.As(err, &observed) || observed.Effect != domain.NativeEffectUncertain || !bytes.Equal(foreign, trustRead(t, path)) {
		t.Fatalf("foreign write not retained: %v", err)
	}
	cleanup := errors.New("component lock cleanup")
	err = withCopilotTrust(t.Context(), env, request, func() error { return &nativeconfig.CommittedCleanupError{Err: cleanup} })
	if !nativeconfig.IsCommittedCleanup(err) || !errors.Is(err, cleanup) {
		t.Fatal(err)
	}
	if err := verifyCopilotTrust(env, request, true); err != nil {
		t.Fatal(err)
	}
}

func TestProjectCopilotTrustRefusesUnsafePathsAndDocuments(t *testing.T) {
	for _, body := range []string{"", "{", `{"trustedFolders":true}`, `{"trustedFolders":["ok",1]}`} {
		t.Run("document-"+body, func(t *testing.T) {
			env, request, path := trustFixture(t)
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if err := withCopilotTrust(t.Context(), env, request, func() error { t.Fatal("invalid config applied"); return nil }); err == nil {
				t.Fatal("unsafe config admitted")
			}
			if string(trustRead(t, path)) != body {
				t.Fatal("invalid config changed")
			}
		})
	}
	env, request, path := trustFixture(t)
	if err := verifyCopilotTrust(env, request, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("read-only preflight created config")
	}
	for _, root := range []string{request.Client.ConfigRoot, filepath.Dir(request.Client.ConfigRoot), filepath.VolumeName(path) + string(filepath.Separator)} {
		r := request
		r.Plan.TargetContext.ProjectRoot = root
		r.Plan.NativeRegistryRoot = root
		r.Plan.ActivePath = filepath.Join(root, ".uap", "managed", "clients", "copilot", "fixture")
		if _, err := copilotTrustPath(r.Client, r.Plan); err == nil {
			t.Fatalf("unsafe root admitted: %s", root)
		}
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "foreign"), path); err != nil {
		if strings.Contains(err.Error(), "privilege") {
			t.Skip("symlink unavailable")
		}
		t.Fatal(err)
	}
	if err := verifyCopilotTrust(env, request, false); err == nil {
		t.Fatal("symlink config admitted")
	}
}

func TestProjectCopilotTrustLockCleanupAndAcquisitionFaults(t *testing.T) {
	for _, acquireFailure := range []bool{false, true} {
		env, request, path := trustFixture(t)
		cause := errors.New("trust lock fault")
		env.NativeConfig = nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
			if acquireFailure {
				return nil, cause
			}
			return func() error { return cause }, nil
		})
		called := false
		err := withCopilotTrust(t.Context(), env, request, func() error { called = true; return nil })
		if !errors.Is(err, cause) {
			t.Fatal(err)
		}
		if acquireFailure {
			if called {
				t.Fatal("components started without trust lock")
			}
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatal("failed acquisition wrote trust")
			}
		} else {
			if !called || !nativeconfig.IsCommittedCleanup(err) {
				t.Fatal("committed effect lost")
			}
			if err := verifyCopilotTrust(env, request, true); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestProjectCopilotDetectionBindsSelectedHome(t *testing.T) {
	home := t.TempDir()
	selected := filepath.Join(home, "chosen")
	host := clients.NewHost(clients.HostProbes{HomeDir: home, Environment: map[string]string{"COPILOT_HOME": selected}, Lstat: os.Lstat, EvalSymlinks: filepath.EvalSymlinks})
	detected := New().DetectSurfaces(host)
	if detected.Err != nil || detected.ConfigRoot != selected {
		t.Fatalf("selected COPILOT_HOME not captured: %+v", detected)
	}
	host = clients.NewHost(clients.HostProbes{HomeDir: home, Environment: map[string]string{"COPILOT_HOME": "relative"}, Lstat: os.Lstat, EvalSymlinks: filepath.EvalSymlinks})
	if New().DetectSurfaces(host).Err == nil {
		t.Fatal("relative profile admitted")
	}
}
