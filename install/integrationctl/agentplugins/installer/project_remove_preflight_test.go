package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectRemovalForeignFilesRefuseBeforeNativeAttempt(t *testing.T) {
	for _, variant := range []string{"single-cline", "single-codex", "group-first", "group-second"} {
		for _, kind := range []string{"skill", "mcp"} {
			for _, afterPrepare := range []bool{false, true} {
				name := variant + "/" + kind
				if afterPrepare {
					name += "/after-prepare"
				}
				t.Run(name, func(t *testing.T) {
					f, path := projectRemovalForeignFixture(t, variant, kind)
					var handle *PreparedOperation
					if afterPrepare {
						req := f.req
						req.Operation, req.OperationID = OpRemove, "foreign-after-prepare"
						var err error
						handle, err = f.engine.Prepare(t.Context(), req)
						if err != nil {
							t.Fatal(err)
						}
						defer func() {
							if err := handle.Close(); err != nil {
								t.Error(err)
							}
						}()
					}
					foreign := "foreign skill bytes"
					if kind == "mcp" {
						original := deathRead(t, path)
						foreign = strings.ReplaceAll(original, "http://127.0.0.1:9/inert", "https://foreign.example/mcp")
						if foreign == original {
							t.Fatal("actual projected marker missing")
						}
					}
					deathWrite(t, path, foreign)
					before, other := deathTree(t, f.project), f.untouched(t)
					var result Result
					var err error
					if afterPrepare {
						result, err = f.engine.Apply(t.Context(), handle, Decision{Confirmed: true})
					} else {
						result, err = f.execute(t, OpRemove)
					}
					if err == nil || result.Mutated || !reflect.DeepEqual(before, deathTree(t, f.project)) {
						t.Fatalf("foreign %s refusal changed project/state/journal: mutation=%t error=%v", kind, result.Mutated, err)
					}
					f.assertUntouched(t, other)
				})
			}
		}
	}
}

func projectRemovalForeignFixture(t *testing.T, variant, kind string) (*projectFilesFixture, string) {
	t.Helper()
	var f *projectFilesFixture
	if strings.HasPrefix(variant, "group-") {
		f = projectRemovalGroup(t)
	} else {
		f = newProjectFilesFixture(t, strings.TrimPrefix(variant, "single-"), "disabled-with-runner")
		f.apply(t, OpInstall)
	}
	path := f.skill
	if kind == "mcp" {
		path = f.config
	}
	if variant == "group-second" {
		path = filepath.Join(f.project, ".agents", "skills", "docs", "SKILL.md")
		if kind == "mcp" {
			path = filepath.Join(f.project, ".codex", "config.toml")
		}
	}
	return f, path
}

func TestProjectRemovalAllowsAbsentOwnedNativeFiles(t *testing.T) {
	for _, id := range []string{"cline", "codex"} {
		t.Run(id, func(t *testing.T) {
			f := newProjectFilesFixture(t, id, "disabled-with-runner")
			f.apply(t, OpInstall)
			other := f.untouched(t)
			for _, path := range []string{filepath.Dir(f.skill), f.config} {
				if err := os.RemoveAll(path); err != nil {
					t.Fatal(err)
				}
			}
			result := f.apply(t, OpRemove)
			if !result.Mutated {
				t.Fatal("owned package removal lost mutation")
			}
			f.assertUntouched(t, other)
		})
	}
}

type removalFileInspector struct {
	finding clients.RegistryFinding
	err     error
	calls   int
	check   func(clients.Env, domain.DetectedClient, domain.DeliveryPlan, *domain.ClientBinding)
}

func (*removalFileInspector) ID() domain.ClientID                { return domain.ClientCline }
func (*removalFileInspector) UsesNativeRegistryExecutable() bool { return true }
func (*removalFileInspector) InspectPreparedRegistry(domain.DeliveryPlan, string, bool) (clients.RegistryFinding, error) {
	return clients.RegistryClear, nil
}
func (a *removalFileInspector) InspectNativeRegistry(_ context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, binding *domain.ClientBinding) (clients.RegistryFinding, error) {
	a.calls++
	if a.check != nil {
		a.check(env, client, plan, binding)
	}
	return a.finding, a.err
}

func TestProjectRemovalObservationUsesFrozenRecordedSet(t *testing.T) {
	for _, finding := range []clients.RegistryFinding{clients.RegistryExpected, clients.RegistryClear, clients.RegistryCollision, clients.RegistryIndeterminate, clients.RegistryFinding(255)} {
		t.Run(fmt.Sprintf("finding-%d", finding), func(t *testing.T) {
			f := newProjectFilesFixture(t, "cline", "disabled-with-runner")
			f.apply(t, OpInstall)
			binding := f.binding(t)
			objects := append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...)
			client := domain.DetectedClient{ClientID: domain.ClientCline, ConfigRoot: f.user}
			target, err := f.engine.planner().ResolveTarget(t.Context(), client, domain.ScopeProject, binding.PhysicalArtifact)
			if err != nil {
				t.Fatal(err)
			}
			inspector := &removalFileInspector{finding: finding}
			inspector.check = func(env clients.Env, got domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) {
				if env.Runner != nil || got.ClientID != client.ClientID || got.ConfigRoot != f.user || plan.DeclaredName != "files-observer" || plan.TargetContext != target.TargetContext || plan.TargetAnchor != target.TargetAnchor || plan.TargetRoot != target.TargetRoot || plan.ActivePath != target.ActivePath || plan.NativeRegistryRoot != target.NativeRegistryRoot || len(plan.Components) != 0 || !reflect.DeepEqual(plan.PreviousNativeObjects, objects) || !reflect.DeepEqual(managed.NativeObjects, objects) {
					t.Fatal("removal observation lost exact frozen authority or acquired a native runner")
				}
				// Each external packet owns independent slices. A callback cannot
				// change the plan/binding used by subsequent confirmation fences.
				plan.PreviousNativeObjects[0].Path = "callback-only"
				if !reflect.DeepEqual(managed.NativeObjects, objects) {
					t.Fatal("plan shares managed ownership")
				}
				managed.NativeObjects[len(managed.NativeObjects)-1].LogicalName = "callback-only"
			}
			registry, err := clients.NewRegistry(inspector)
			if err != nil {
				t.Fatal(err)
			}
			f.engine.cfg.Registry = registry
			before, other := deathTree(t, f.project), f.untouched(t)
			err = f.engine.confirmProjectRemovalFiles(t.Context(), client, "files-observer", binding, target, managedPackageDigest(binding))
			wantError := finding != clients.RegistryExpected && finding != clients.RegistryClear
			if (err != nil) != wantError || inspector.calls != 1 || !reflect.DeepEqual(binding.NativeObjects, objects) || !reflect.DeepEqual(before, deathTree(t, f.project)) {
				t.Fatalf("finding=%v error=%v calls=%d", finding, err, inspector.calls)
			}
			f.assertUntouched(t, other)
		})
	}
}

func TestProjectRemovalObservationRejectsMissingOrFailedInspector(t *testing.T) {
	for _, missing := range []bool{false, true} {
		f := newProjectFilesFixture(t, "cline", "disabled-with-runner")
		f.apply(t, OpInstall)
		binding := f.binding(t)
		client := domain.DetectedClient{ClientID: domain.ClientCline, ConfigRoot: f.user}
		target, err := f.engine.planner().ResolveTarget(t.Context(), client, domain.ScopeProject, binding.PhysicalArtifact)
		if err != nil {
			t.Fatal(err)
		}
		failure := errors.New("inert inspector failure")
		var adapter clients.Adapter = &removalFileInspector{finding: clients.RegistryClear, err: failure}
		if missing {
			adapter = &handoffCaptureAdapter{id: domain.ClientCline}
		}
		registry, err := clients.NewRegistry(adapter)
		if err != nil {
			t.Fatal(err)
		}
		f.engine.cfg.Registry = registry
		before := deathTree(t, f.project)
		err = f.engine.confirmProjectRemovalFiles(t.Context(), client, "files-observer", binding, target, managedPackageDigest(binding))
		if err == nil || !missing && !errors.Is(err, failure) || !reflect.DeepEqual(before, deathTree(t, f.project)) {
			t.Fatalf("missing=%t error=%v", missing, err)
		}
	}
}

func TestProjectRemovalObservationPreservesOtherLifecycles(t *testing.T) {
	f := newProjectFilesFixture(t, "cline", "disabled-with-runner")
	f.apply(t, OpInstall)
	inspector := &removalFileInspector{finding: clients.RegistryCollision}
	registry, err := clients.NewRegistry(inspector)
	if err != nil {
		t.Fatal(err)
	}
	f.engine.cfg.Registry = registry
	before := deathTree(t, f.project)
	for _, id := range []domain.ClientID{domain.ClientClaude, domain.ClientGrok} {
		if err := f.engine.confirmProjectRemovalFiles(t.Context(), domain.DetectedClient{ClientID: id}, "", domain.ClientBinding{}, domain.DeliveryTarget{}, ""); err != nil {
			t.Fatal(err)
		}
	}
	f.engine.cfg.ProjectRoot = ""
	if err := f.engine.confirmProjectRemovalFiles(t.Context(), domain.DetectedClient{ClientID: domain.ClientCline}, "", domain.ClientBinding{}, domain.DeliveryTarget{}, ""); err != nil {
		t.Fatal(err)
	}
	if inspector.calls != 0 || !reflect.DeepEqual(before, deathTree(t, f.project)) {
		t.Fatal("file observer entered a historical lifecycle")
	}
}

func TestProjectRemovalObservationRejectsContradictoryPacket(t *testing.T) {
	for _, bad := range []string{"name", "target-scope", "user-profile", "digest", "owned-receipt"} {
		t.Run(bad, func(t *testing.T) {
			f := newProjectFilesFixture(t, "cline", "disabled-with-runner")
			f.apply(t, OpInstall)
			binding := f.binding(t)
			client := domain.DetectedClient{ClientID: domain.ClientCline, ConfigRoot: f.user}
			target, err := f.engine.planner().ResolveTarget(t.Context(), client, domain.ScopeProject, binding.PhysicalArtifact)
			if err != nil {
				t.Fatal(err)
			}
			name, digest := "files-observer", managedPackageDigest(binding)
			switch bad {
			case "name":
				name = ""
			case "target-scope":
				target.TargetContext.Scope = domain.ScopeUser
			case "user-profile":
				target.TargetContext.UserConfigRoot = filepath.Join(f.base, "B")
			case "digest":
				digest = "sha256:foreign"
			case "owned-receipt":
				binding.NativeObjects = nil
			}
			before, other := deathTree(t, f.project), f.untouched(t)
			if err := f.engine.confirmProjectRemovalFiles(t.Context(), client, name, binding, target, digest); err == nil || !reflect.DeepEqual(before, deathTree(t, f.project)) {
				t.Fatalf("bad %s admitted or mutated: %v", bad, err)
			}
			f.assertUntouched(t, other)
		})
	}
}
