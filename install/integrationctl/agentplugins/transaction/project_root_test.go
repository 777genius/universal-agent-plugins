package transaction

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func projectRootFixture(t *testing.T) (ProjectRootPublisher, domain.ActivationRequest, ports.NativeProjectRoot, statev2.Store) {
	t.Helper()
	kernel, mutation, store := transactionFixture(t, "native-attempt")
	project := filepath.Dir(store.Path)
	native := filepath.Join(project, ".fixture")
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installation := &state.Installations[0]
	binding := onlyClient(*installation)
	binding.ClientID, binding.Scope = string(domain.ClientHermes), string(domain.ScopeProject)
	binding.ClientBindingID = domain.ComputeClientBindingID(installation.InstallationID, binding.ClientID, binding.Scope, mutation.ActivePath)
	binding.NativeProfileRoot, binding.TargetLocator = native, mutation.ActivePath
	binding.NativeActivationAttempt = "native-attempt"
	installation.Clients = map[string]domain.ClientBinding{binding.ClientBindingID: binding}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	req := domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientHermes},
		Plan: domain.DeliveryPlan{ClientID: domain.ClientHermes, Scope: domain.ScopeProject,
			ActivePath: mutation.ActivePath, NativeRegistryRoot: native,
			TargetContext: domain.TargetContext{ProjectRoot: project}},
		NativeAttempt: domain.NativeAttemptIdentity{OperationID: binding.NativeActivationAttempt,
			InstallationID: installation.InstallationID, BindingID: binding.ClientBindingID, NativeRoot: native},
	}
	body := []byte("fixture selected root\n")
	root := filepath.Join(native, "selected")
	spec := ports.NativeProjectRoot{Path: root, MarkerBody: body, Marker: domain.NativeObjectOwnership{
		Kind: "fixture_root", ObjectID: "fixture_root", Path: filepath.Join(root, ".owner"), ProtectionClass: "owned",
		ManagedDigest: fmt.Sprintf("%x", sha256.Sum256(body)),
	}}
	return ProjectRootPublisher{Kernel: kernel}, req, spec, store
}

func TestProjectRootPublicationAndSharedReceiptReuse(t *testing.T) {
	for _, parentExists := range []bool{false, true} {
		t.Run(fmt.Sprint(parentExists), func(t *testing.T) {
			publisher, request, spec, store := projectRootFixture(t)
			if parentExists {
				writeTransactionBody(t, filepath.Dir(spec.Path), "foreign parent bytes")
			}
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if err := publisher.PublishProjectRoot(t.Context(), request, spec); err != nil {
				t.Fatal(err)
			}
			if err := publisher.PublishProjectRoot(t.Context(), request, spec); err != nil {
				t.Fatalf("exact root could not be reused: %v", err)
			}
			after, err := store.Load()
			if err != nil || !reflect.DeepEqual(before.Installations, after.Installations) || len(after.TransactionReceipts) != 1 {
				t.Fatalf("publication changed binding or duplicated receipt: %v", err)
			}
			if parentExists {
				assertTransactionBody(t, filepath.Dir(spec.Path), "foreign parent bytes")
			}
			after.Installations = nil
			if err := store.Save(after); err != nil {
				t.Fatal(err)
			}
			if err := publisher.VerifyProjectRoot(t.Context(), spec.Path, spec.Marker); err != nil {
				t.Fatalf("removed first installation lost root proof: %v", err)
			}
			second := before.Installations[0]
			second.InstallationID = "00000000-0000-4000-8000-000000000002"
			binding := onlyClient(second)
			binding.ClientBindingID = domain.ComputeClientBindingID(second.InstallationID, binding.ClientID, binding.Scope, binding.TargetLocator)
			binding.PhysicalArtifact = domain.ComputePhysicalArtifactID(second.DeclaredName, second.InstallationID)
			binding.NativeActivationAttempt = "second-native-attempt"
			second.Clients = map[string]domain.ClientBinding{binding.ClientBindingID: binding}
			after.Installations = []domain.Installation{second}
			if err := store.Save(after); err != nil {
				t.Fatal(err)
			}
			request.NativeAttempt.InstallationID, request.NativeAttempt.BindingID = second.InstallationID, binding.ClientBindingID
			request.NativeAttempt.OperationID = binding.NativeActivationAttempt
			if err := publisher.PublishProjectRoot(t.Context(), request, spec); err != nil {
				t.Fatalf("second installation could not reuse retained proof: %v", err)
			}
		})
	}
}

func TestProjectRootRejectsForeignOrCopiedMarker(t *testing.T) {
	for _, change := range []string{"foreign-root", "replaced-root", "replaced-parent", "marker", "missing-root"} {
		t.Run(change, func(t *testing.T) {
			publisher, request, spec, store := projectRootFixture(t)
			if change == "foreign-root" {
				if err := os.MkdirAll(spec.Path, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(spec.Marker.Path, spec.MarkerBody, 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := publisher.PublishProjectRoot(t.Context(), request, spec); err != nil {
				t.Fatal(err)
			}
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "replaced-root", "missing-root":
				if err := os.Rename(spec.Path, spec.Path+"-previous"); err != nil {
					t.Fatal(err)
				}
				if change == "replaced-root" {
					if err := os.Mkdir(spec.Path, 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(spec.Marker.Path, spec.MarkerBody, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			case "replaced-parent":
				parent := filepath.Dir(spec.Path)
				if err := os.Rename(parent, parent+"-previous"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(parent, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(parent+"-previous", filepath.Base(spec.Path)), spec.Path); err != nil {
					t.Fatal(err)
				}
			case "marker":
				if err := os.WriteFile(spec.Marker.Path, []byte("foreign"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := publisher.PublishProjectRoot(t.Context(), request, spec); err == nil {
				t.Fatal("accepted unproven selected root")
			}
			after, err := store.Load()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("refusal modified ownership: %v", err)
			}
		})
	}
}
