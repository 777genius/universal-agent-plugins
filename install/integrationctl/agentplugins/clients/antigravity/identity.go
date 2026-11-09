package antigravity

import (
 "context"
 "fmt"
 "os"
 "path/filepath"
 "strings"

 "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
 "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
 "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
 "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
 _ clients.PreparedRegistryInspector = (*Adapter)(nil)
 _ clients.RegistryInspector = (*Adapter)(nil)
)

func (*Adapter) UsesNativeRegistryExecutable() bool { return false }

// The user and workspace discovery directories are the native registration
// authority documented by Antigravity. These observations prove only filesystem
// identity, not whether an agy process has loaded or enabled the plugin.
func (*Adapter) InspectNativeRegistry(ctx context.Context, _ clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, binding *domain.ClientBinding) (clients.RegistryFinding, error) {
 if err := ctx.Err(); err != nil { return clients.RegistryIndeterminate, err }
 if client.ClientID != domain.ClientAntigravity {
  return clients.RegistryIndeterminate, fmt.Errorf("Antigravity selected client identity differs")
 }
 if err := selectedNativePluginRoot(plan); err != nil { return clients.RegistryIndeterminate, err }
 if plan.Scope == domain.ScopeUser && client.ConfigRoot != plan.NativeRegistryRoot {
  return clients.RegistryIndeterminate, fmt.Errorf("Antigravity user native profile changed")
 }
 if plan.Scope == domain.ScopeProject && plan.TargetContext.UserConfigRoot != "" &&
   client.ConfigRoot != plan.TargetContext.UserConfigRoot {
  return clients.RegistryIndeterminate, fmt.Errorf("Antigravity project uses a different detected user profile")
 }
 if binding != nil && binding.TargetLocator != plan.ActivePath {
  return clients.RegistryIndeterminate, fmt.Errorf("Antigravity recorded plugin target differs from active path")
 }
 return inspectNativePluginDirectory(plan.TargetRoot, plan.DeclaredName, plan.ActivePath, binding != nil)
}

func (*Adapter) InspectPreparedRegistry(plan domain.DeliveryPlan, name string, owned bool) (clients.RegistryFinding, error) {
 if err := selectedNativePluginRoot(plan); err != nil { return clients.RegistryIndeterminate, err }
 return inspectNativePluginDirectory(plan.TargetRoot, name, plan.ActivePath, owned)
}

func inspectNativePluginDirectory(root, name, activePath string, owned bool) (clients.RegistryFinding, error) {
 if !cleanAbsolute(root) || !cleanAbsolute(activePath) || strings.TrimSpace(name) == "" {
  return clients.RegistryIndeterminate, fmt.Errorf("Antigravity native plugin identity requires selected roots and name")
 }
 entries, err := os.ReadDir(root)
 if os.IsNotExist(err) { return clients.RegistryClear, nil }
 if err != nil { return clients.RegistryIndeterminate, err }
 finding := clients.RegistryClear
 for _, entry := range entries {
  if strings.HasPrefix(entry.Name(), ".agentplugins-staging-") { continue }
  path := filepath.Join(root, entry.Name())
  info, err := os.Lstat(path)
  if err != nil { return clients.RegistryIndeterminate, err }
  if info.Mode()&os.ModeSymlink != 0 { return clients.RegistryIndeterminate, fmt.Errorf("Antigravity native plugin root contains a symlink") }
  if !info.IsDir() {
   if path == activePath { return clients.RegistryIndeterminate, fmt.Errorf("Antigravity owned plugin path is not a directory") }
   continue
  }
  identity, err := antigravityManifestName(path)
  if err != nil { return clients.RegistryIndeterminate, err }
  if identity != name { continue }
  if shared.SameCleanPath(path, activePath) && owned {
   finding = clients.RegistryExpected
   continue
  }
  return clients.RegistryCollision, nil
 }
 return finding, nil
}

func antigravityManifestName(root string) (string, error) {
 for _, relative := range []string{filepath.Join(".antigravity-plugin", "plugin.json"), "plugin.json"} {
  path := filepath.Join(root, relative)
  body, err := atomicfile.ReadBoundedRegularFile(path, maxNativeManifestBytes)
  if os.IsNotExist(err) { continue }
  if err != nil { return "", fmt.Errorf("read Antigravity native manifest: %w", err) }
  doc, err := shared.DecodeStrictJSONObject(body)
  if err != nil { return "", fmt.Errorf("decode Antigravity native manifest: %w", err) }
  name, ok := doc["name"].(string)
  if !ok || !nativePluginID.MatchString(name) { return "", fmt.Errorf("invalid Antigravity native manifest name") }
  return name, nil
 }
 return "", fmt.Errorf("native Antigravity plugin directory has no manifest")
}
