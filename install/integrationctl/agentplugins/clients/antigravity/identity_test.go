package antigravity

import (
 "context"
 "os"
 "path/filepath"
 "testing"

 "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
 "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
 "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func writeAntigravityIdentity(t *testing.T, root, folder, name string) string {
 t.Helper()
 path := filepath.Join(root, folder)
 if err := os.MkdirAll(path, 0o700); err != nil { t.Fatal(err) }
 if err := shared.WriteJSON(filepath.Join(path, ".antigravity-plugin", "plugin.json"),
  map[string]any{"name": name, "$schema": "https://antigravity.google/schemas/v1/plugin.json"}); err != nil { t.Fatal(err) }
 return path
}

func TestAntigravityNativeCollisionAndManagedOwnership(t *testing.T) {
 t.Parallel()
 root := t.TempDir()
 foreign := writeAntigravityIdentity(t, root, "foreign-plugin", "same-name")
 managed := filepath.Join(root, "managed-plugin")
 finding, err := inspectNativePluginDirectory(root, "same-name", managed, false)
 if err != nil || finding != clients.RegistryCollision { t.Fatalf("foreign identity not rejected: %v %v", finding, err) }
 finding, err = inspectNativePluginDirectory(root, "different-name", managed, false)
 if err != nil || finding != clients.RegistryClear { t.Fatalf("unrelated identity obstructed installation: %v %v", finding, err) }
 if err := os.RemoveAll(foreign); err != nil { t.Fatal(err) }
 writeAntigravityIdentity(t, root, "managed-plugin", "same-name")
 finding, err = inspectNativePluginDirectory(root, "same-name", managed, true)
 if err != nil || finding != clients.RegistryExpected { t.Fatalf("owned identity not recognized: %v %v", finding, err) }
 finding, err = inspectNativePluginDirectory(root, "same-name", managed, false)
 if err != nil || finding != clients.RegistryCollision { t.Fatalf("unowned managed path was adopted: %v %v", finding, err) }
}

func TestAntigravityNativeSymlinkAndMissingManifestFailClosed(t *testing.T) {
 t.Parallel()
 root := t.TempDir()
 managed := filepath.Join(root, "managed")
 if err := os.MkdirAll(filepath.Join(root, "orphan"), 0o700); err != nil { t.Fatal(err) }
 if f, err := inspectNativePluginDirectory(root, "demo", managed, false); err == nil || f != clients.RegistryIndeterminate {
  t.Fatalf("unknown native directory must not mean no collision: %v %v", f, err)
 }
 if err := os.RemoveAll(filepath.Join(root, "orphan")); err != nil { t.Fatal(err) }
 if err := os.Symlink(t.TempDir(), filepath.Join(root, "foreign-link")); err == nil {
  if f, err := inspectNativePluginDirectory(root, "demo", managed, false); err == nil || f != clients.RegistryIndeterminate {
   t.Fatalf("native symlink must fail closed: %v %v", f, err)
  }
 }
}

func TestAntigravityNativeProjectAuthorityIsolated(t *testing.T) {
 t.Parallel()
 user := t.TempDir()
 project := t.TempDir()
 native := filepath.Join(project, ".agents")
 root := filepath.Join(native, "plugins")
 writeAntigravityIdentity(t, root, "my-plugin", "my-plugin")
 plan := domain.DeliveryPlan{
  ClientID: domain.ClientAntigravity, Scope: domain.ScopeProject,
  NativeRegistryRoot: native, TargetRoot: root,
  ActivePath: filepath.Join(root, "my-plugin"), DeclaredName: "my-plugin",
  TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: user},
 }
 detector := domain.DetectedClient{ClientID: domain.ClientAntigravity, ConfigRoot: user}
 finding, err := New().InspectNativeRegistry(context.Background(), clients.Env{}, detector, plan, nil)
 if err != nil || finding != clients.RegistryCollision { t.Fatalf("unexpected native inspection: %v %v", finding, err) }
 plan.NativeRegistryRoot = user
 if f, err := New().InspectNativeRegistry(context.Background(), clients.Env{}, detector, plan, nil); err == nil || f != clients.RegistryIndeterminate {
  t.Fatalf("project-to-user fallback was allowed: %v %v", f, err)
 }
}
