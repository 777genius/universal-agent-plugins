//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kimi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

type kimiProjectProofContract struct {
	ctx          context.Context
	fixture      *kimiProjectNativeFixture
	proof        *kimiProjectProof
	installation domain.Installation
}

// All executables here are inert bytes, never invoked. Source acquisition,
// schema loading, Kimi planning, Stager projection and file activation are the
// actual selected production implementations, not a second expected renderer.
func newKimiProjectProofContract(t *testing.T, scope string) *kimiProjectProofContract {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	root := t.TempDir()
	home, project := filepath.Join(root, "home"), filepath.Join(root, "A")
	for _, path := range []string{filepath.Join(home, ".kimi-code"), filepath.Join(project, ".git")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	probe, installer := filepath.Join(root, "probe"), filepath.Join(root, "installer")
	projectNativeRuntimeWrite(t, probe, []byte("inert authored probe; never run\n"), 0700)
	projectNativeRuntimeWrite(t, installer, []byte("inert public installer; never run\n"), 0700)
	f := &kimiProjectNativeFixture{ctx: ctx, base: &nativeFixture{Root: root, Home: home, Probe: probe, Nonce: "inert-proof-nonce"}, installer: installer, projects: map[string]string{scope: project}, packages: map[string]*kimiProjectNativePackage{}, immutable: map[string]string{probe: nativeSHA(t, probe), installer: nativeSHA(t, installer)}}
	f.source(t, scope, scope+"-one")
	p := f.packages[scope].Proof
	snapshot, err := (packagedigest.Builder{TempRoot: root}).Snapshot(ctx, p.source, domain.SourceIdentity{RequestedSource: p.source, CanonicalSource: p.source})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := packagedigest.Remove(snapshot); err != nil {
			t.Error(err)
		}
	})
	schemas, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := (loader.Loader{Registry: schemas}).LoadSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	i := domain.Installation{InstallationID: "01234567-89ab-4cde-8fab-0123456789ab", DeclaredName: p.name, Source: domain.SourceBinding{SourceBindingID: domain.ComputeSourceBindingID(envelope.Source), RequestedSource: envelope.Source.RequestedSource, CanonicalSource: envelope.Source.CanonicalSource, TreeDigest: envelope.TreeDigest}, Package: domain.PackageBinding{LoaderKind: envelope.LoaderKind, FormatID: envelope.FormatID, SchemaURI: envelope.SchemaURI, DeclaredName: envelope.Manifest.Name, Version: envelope.Manifest.Version, ManifestDigest: envelope.ManifestDigest, Inventory: envelope.Inventory}, Clients: map[string]domain.ClientBinding{}}
	// The native fixture consumes state JSON, whose omitempty inventory slices
	// deserialize to nil. Exercise that actual persistence boundary here too.
	stateBytes, err := json.Marshal(i)
	if err != nil {
		t.Fatal(err)
	}
	i = domain.Installation{}
	if err := json.Unmarshal(stateBytes, &i); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(i.Source, p.sourceIdentity) || !reflect.DeepEqual(i.Package, p.packageIdentity) {
		t.Fatalf("frozen recipe differs from actual acquired/loaded source identity: source=%+v want=%+v package=%+v want=%+v", i.Source, p.sourceIdentity, i.Package, p.packageIdentity)
	}
	adapter := kimi.New()
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientKimi, ConfigRoot: filepath.Join(home, ".kimi-code"), ExecutablePath: filepath.Join(root, "kimi"), Version: "2.3.2", Status: domain.DetectionDetected}
	installScope := domain.ScopeUser
	managed, projectRoot := filepath.Join(root, "installer-state", "managed"), ""
	if scope != "user" {
		installScope, projectRoot, managed = domain.ScopeProject, project, filepath.Join(project, ".uap", "managed")
	}
	planning := planner.Planner{ProjectRoot: projectRoot, ManagedRoot: managed, Paths: pathpolicy.Policy{}, Registry: registry}
	plan, err := planning.Plan(ctx, domain.PlanRequest{Envelope: envelope, Client: client, Scope: installScope, ProjectRoot: projectRoot, PhysicalArtifactID: domain.ComputePhysicalArtifactID(p.name, i.InstallationID)})
	if err != nil || plan.Status != domain.PlanReady {
		t.Fatalf("actual selected Kimi plan: %+v, %v", plan, err)
	}
	launcher, err := managedstdio.NewSource(installer, "0.1.0-development")
	if err != nil {
		t.Fatal(err)
	}
	stager := providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}, LauncherSource: launcher}
	if scope != "user" {
		stager.PluginDataRoot = filepath.Join(project, ".uap", "plugin-data")
		data, _, err := (providers.PluginDataManager{Base: filepath.Join(project, ".uap", "plugin-data")}).EnsureData(ctx, i.InstallationID, plan.PhysicalArtifactID, "project")
		if err != nil {
			t.Fatal(err)
		}
		i.DataReceipts = map[string]domain.DataReceipt{data.DataReceiptID: data}
	}
	delivery, err := stager.Stage(ctx, envelope, plan, "inert-kimi-proof", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, delivery.ActivePath); err != nil {
		t.Fatal(err)
	}
	request := domain.ActivationRequest{Client: client, Plan: plan, Delivery: delivery, DeclaredName: p.name}
	out, err := adapter.Activate(ctx, clients.Env{NativeConfig: nativeconfig.New()}, request)
	if err != nil || out.Verification != domain.VerificationInstalled {
		t.Fatalf("actual inert Kimi file activation: %+v, %v", out, err)
	}
	b := domain.ClientBinding{ClientID: "kimi", Scope: string(installScope), PhysicalArtifact: plan.PhysicalArtifactID, TargetLocator: plan.ActivePath, Materialization: domain.MaterializationMaterialized, Activation: out.Activation, Verification: out.Verification, NativeObjects: delivery.NativeObjects, PackageRevision: &domain.ClientPackageRevision{Version: envelope.Manifest.Version, TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest}}
	b.ClientBindingID = domain.ComputeClientBindingID(i.InstallationID, b.ClientID, b.Scope, b.TargetLocator)
	if scope != "user" {
		b.NativeProfileRoot, b.NativeObjects = plan.NativeRegistryRoot, out.NativeObjects
		for id := range i.DataReceipts {
			b.DataReceiptID = id
		}
	}
	i.Clients[b.ClientBindingID] = b
	return &kimiProjectProofContract{ctx: ctx, fixture: f, proof: p, installation: i}
}

func kimiProjectProofRequireRefused(t *testing.T, f *kimiProjectProofContract) {
	t.Helper()
	result, err := f.proof.validate(f.ctx, f.installation)
	if err == nil || !reflect.DeepEqual(result, kimiProjectValidated{}) || f.proof.stable != nil {
		t.Fatalf("partial/corrupt first proof admitted identities or baseline: %+v, %v", result, err)
	}
}

func TestKimiProjectProofMatchesActualUserAndMixedStager(t *testing.T) {
	for _, scope := range []string{"user", "a"} {
		t.Run(scope, func(t *testing.T) {
			f := newKimiProjectProofContract(t, scope)
			validated, err := f.proof.validate(f.ctx, f.installation)
			if err != nil || f.proof.stable == nil {
				t.Fatalf("actual Kimi Stager/activation rejected: %v", err)
			}
			entries, err := kimiProjectProofCapture(f.ctx, validated.authority.active)
			if err != nil {
				t.Fatal(err)
			}
			wrongFraming, err := packagedigest.DigestCaptured(f.ctx, entries)
			if err != nil {
				t.Fatal(err)
			}
			for _, object := range validated.binding.NativeObjects {
				if object.Kind == "managed_package_directory" && object.ManagedDigest == wrongFraming {
					t.Fatal("source framing adopted as installed package ownership")
				}
			}
			wantIdentities := map[string]string{}
			if scope != "user" {
				wantIdentities[filepath.Join(validated.authority.active, "bin", "probe")] = f.fixture.immutable[f.fixture.base.Probe]
				wantIdentities[filepath.Join(validated.authority.active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)] = f.fixture.immutable[f.fixture.installer]
			}
			if !reflect.DeepEqual(validated.identities, wantIdentities) {
				t.Fatal("observer identities did not originate from independent frozen executables")
			}
		})
	}
}

func TestKimiProjectProofRefusesForgedReceiptBeforeFirstBaseline(t *testing.T) {
	changes := map[string]func(*domain.Installation){
		"source binding": func(i *domain.Installation) { i.Source.SourceBindingID = "src_nonempty-forgery" },
		"source tree":    func(i *domain.Installation) { i.Source.TreeDigest = "sha256:" + strings.Repeat("a", 64) },
		"manifest":       func(i *domain.Installation) { i.Package.ManifestDigest = "sha256:" + strings.Repeat("b", 64) },
		"inventory":      func(i *domain.Installation) { i.Package.Inventory.Skills = []string{"other"} },
		"binding ID": func(i *domain.Installation) {
			for key, b := range i.Clients {
				b.ClientBindingID = "client_nonempty-forgery"
				i.Clients[key] = b
			}
		},
		"target path": func(i *domain.Installation) {
			for key, b := range i.Clients {
				b.TargetLocator += "-other"
				i.Clients[key] = b
			}
		},
		"data": func(i *domain.Installation) {
			for key, d := range i.DataReceipts {
				d.OwnershipDigest = "sha256:" + strings.Repeat("c", 64)
				i.DataReceipts[key] = d
			}
		},
		"unknown object": func(i *domain.Installation) {
			for key, b := range i.Clients {
				b.NativeObjects[0].Kind = "unknown"
				i.Clients[key] = b
			}
		},
		"missing object": func(i *domain.Installation) {
			for key, b := range i.Clients {
				b.NativeObjects = b.NativeObjects[1:]
				i.Clients[key] = b
			}
		},
		"duplicate object": func(i *domain.Installation) {
			for key, b := range i.Clients {
				b.NativeObjects[1] = b.NativeObjects[0]
				i.Clients[key] = b
			}
		},
		"extra object": func(i *domain.Installation) {
			for key, b := range i.Clients {
				b.NativeObjects = append(b.NativeObjects, b.NativeObjects[0])
				i.Clients[key] = b
			}
		},
		"object path": func(i *domain.Installation) {
			for key, b := range i.Clients {
				b.NativeObjects[0].Path += "-other"
				i.Clients[key] = b
			}
		},
		"object digest": func(i *domain.Installation) {
			for key, b := range i.Clients {
				b.NativeObjects[0].ManagedDigest = "sha256:" + strings.Repeat("d", 64)
				i.Clients[key] = b
			}
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			f := newKimiProjectProofContract(t, "a")
			change(&f.installation)
			kimiProjectProofRequireRefused(t, f)
		})
	}
}

func TestKimiProjectProofRefusesSourceAndInstalledCorruptionBeforeFirstBaseline(t *testing.T) {
	for _, name := range []string{"source bytes", "source extra directory", "probe", "launcher", "metadata", "projection field", "manifest bytes", "extra file", "extra directory", "executable mode", "symlink", "hardlink", "native skill", "native MCP"} {
		t.Run(name, func(t *testing.T) {
			f := newKimiProjectProofContract(t, "a")
			a, _, err := f.proof.authority(f.installation)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(a.active, "bin", "probe")
			switch name {
			case "source bytes":
				path = filepath.Join(f.proof.source, "plugin.json")
			case "source extra directory":
				path = filepath.Join(f.proof.source, "unknown")
			case "launcher":
				path = filepath.Join(a.active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
			case "metadata":
				path = filepath.Join(a.active, filepath.FromSlash(managedstdio.RelativeDirectory), "metadata.json")
			case "projection field":
				path = filepath.Join(a.active, "mcp.json")
			case "manifest bytes":
				path = filepath.Join(a.active, "plugin.json")
			case "extra file", "extra directory":
				path = filepath.Join(a.active, "unknown")
			case "native skill":
				path = filepath.Join(a.profile, "skills", f.proof.name, "SKILL.md")
			case "native MCP":
				path = filepath.Join(a.profile, "mcp.json")
			}
			switch name {
			case "source extra directory", "extra directory":
				err = os.Mkdir(path, 0700)
			case "executable mode":
				err = os.Chmod(path, 0600)
			case "symlink", "hardlink":
				if err = os.Remove(path); err == nil {
					if name == "symlink" {
						err = os.Symlink(f.fixture.base.Probe, path)
					} else {
						err = os.Link(f.fixture.base.Probe, path)
					}
				}
			case "projection field":
				body, readErr := projectNativeRuntimeRead(path, 1<<20)
				if readErr != nil {
					t.Fatal(readErr)
				}
				var value map[string]any
				if err := json.Unmarshal(body, &value); err != nil {
					t.Fatal(err)
				}
				value["unknown"] = true
				err = os.WriteFile(path, kimiProjectProofJSON(value), 0600)
			default:
				err = os.WriteFile(path, []byte("corrupt before first baseline\n"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			kimiProjectProofRequireRefused(t, f)
		})
	}
}

func TestKimiProjectProofRetainsAuthorityAcrossSourceRevision(t *testing.T) {
	f := newKimiProjectProofContract(t, "a")
	if _, err := f.proof.validate(f.ctx, f.installation); err != nil {
		t.Fatal(err)
	}
	original := *f.proof.stable
	f.fixture.source(t, "a", "a-two")
	next := f.fixture.packages["a"].Proof
	if next.stable == nil || *next.stable != original || next.sourceIdentity.TreeDigest == f.proof.sourceIdentity.TreeDigest || next.packageIdentity.Version != "1.0.1" {
		t.Fatal("source update reset original authority or lost new authored source identity")
	}
	changed := f.installation
	changed.Source, changed.Package = next.sourceIdentity, next.packageIdentity
	changed.Clients = map[string]domain.ClientBinding{}
	var binding domain.ClientBinding
	for key, b := range f.installation.Clients {
		b.PackageRevision = &domain.ClientPackageRevision{Version: next.version, TreeDigest: next.sourceIdentity.TreeDigest, ManifestDigest: next.packageIdentity.ManifestDigest}
		binding = b
		changed.Clients[key] = b
	}
	if a, _, err := next.authority(changed); err != nil || a != original {
		t.Fatalf("new authored revision refused original identity: %v", err)
	}
	// A fully self-consistent replacement identity must still fail the first
	// validated authority retained across source revisions.
	changed.InstallationID = "fedcba98-7654-4321-8abc-fedcba987654"
	binding.PhysicalArtifact = domain.ComputePhysicalArtifactID(next.name, changed.InstallationID)
	binding.TargetLocator = filepath.Join(next.project, ".uap", "managed", "clients", "kimi", binding.PhysicalArtifact)
	binding.ClientBindingID = domain.ComputeClientBindingID(changed.InstallationID, "kimi", "project", binding.TargetLocator)
	binding.DataReceiptID = "data_" + kimiProjectProofSHA([]byte(strings.Join([]string{changed.InstallationID, binding.PhysicalArtifact, "project"}, "\x00")))[:24]
	data := domain.DataReceipt{DataReceiptID: binding.DataReceiptID, PhysicalBackend: binding.PhysicalArtifact, Scope: "project", Locator: filepath.Join(next.project, ".uap", "plugin-data", binding.PhysicalArtifact), State: domain.DataReceiptOwned}
	data.OwnershipDigest = "sha256:" + kimiProjectProofSHA([]byte("agentplugins-plugin-data-v1\x00"+data.DataReceiptID+"\x00"+data.Locator))
	changed.Clients = map[string]domain.ClientBinding{binding.ClientBindingID: binding}
	changed.DataReceipts = map[string]domain.DataReceipt{data.DataReceiptID: data}
	withoutBaseline := *next
	withoutBaseline.stable = nil
	if _, _, err := withoutBaseline.authority(changed); err != nil {
		t.Fatalf("replacement candidate is not internally coherent: %v", err)
	}
	if _, _, err := next.authority(changed); err == nil {
		t.Fatal("new revision accepted replacement installation authority")
	}
}

func TestKimiProjectProofUserProjectionAndCanceledCaptureStayUnadmitted(t *testing.T) {
	for _, name := range []string{"user projection", "user extra object", "canceled capture"} {
		t.Run(name, func(t *testing.T) {
			f := newKimiProjectProofContract(t, "user")
			switch name {
			case "user projection":
				a, _, err := f.proof.authority(f.installation)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(a.active, ".kimi-plugin", "plugin.json")
				if err := os.WriteFile(path, []byte(`{"name":"nonempty-foreign","version":"1.0.0","skills":"./skills/","extra":true}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "user extra object":
				for key, b := range f.installation.Clients {
					b.NativeObjects = append(b.NativeObjects, domain.NativeObjectOwnership{Kind: "unknown", ManagedDigest: "nonempty"})
					f.installation.Clients[key] = b
				}
			case "canceled capture":
				ctx, cancel := context.WithCancel(f.ctx)
				cancel()
				f.ctx = ctx
			}
			kimiProjectProofRequireRefused(t, f)
		})
	}
}

func TestKimiProjectProofStateRefusesUnknownAndDuplicateFacts(t *testing.T) {
	for _, body := range []string{
		`{"schema_version":2,"installations":[],"unknown":true}`,
		`{"schema_version":2,"installations":[{"source":{"unknown":"nonempty"}}]}`,
		`{"schema_version":2,"installations":[{"clients":{"id":{"native_objects":[{"kind":"managed_package_directory","unknown":true}]}}}]}`,
		`{"schema_version":2,"schema_version":2,"installations":[]}`,
		`{"schema_version":3,"installations":[]}`,
	} {
		if _, err := kimiProjectProofState([]byte(body)); err == nil {
			t.Fatal("unknown, duplicate or unsupported state facts accepted")
		}
	}
}
