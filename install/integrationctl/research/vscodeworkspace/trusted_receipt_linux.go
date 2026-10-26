//go:build linux

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

func (c *controller) trustedReceipt(scope string) error {
	p := c.trustedState.Packages[scope]
	if err := c.trustedPreserved(); err != nil {
		return err
	}
	i, err := c.trustedReceiptInstallation(scope, p)
	if err != nil {
		return err
	}
	if err := trustedReceiptSource(p, i); err != nil {
		return err
	}
	for _, binding := range i.Clients {
		if err := c.trustedReceiptBinding(scope, p, i, binding); err != nil {
			return err
		}
	}
	p.Installation = i
	want, err := readOrdinary(filepath.Join(p.Source, "skills", p.Name, "SKILL.md"), 1<<20)
	if err != nil {
		return err
	}
	got, err := readOrdinary(p.Skill, 1<<20)
	if err != nil || !bytes.Equal(got, want) {
		return fmt.Errorf("public native skill bytes differ")
	}
	return c.trustedReceiptObjects(scope, p, want)
}

func (c *controller) trustedReceiptObjects(scope string, p *trustedPackage, skill []byte) error {
	binding := p.Binding
	active := binding.TargetLocator
	digest, err := c.trustedReceiptFiles(p)
	if err != nil {
		return err
	}
	want := map[string]domain.NativeObjectOwnership{
		"managed_package_directory": {ObjectID: "package:vscode:" + binding.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: p.Name, Path: active, ManagedDigest: digest, ProtectionClass: "managed"},
	}
	if scope != "user" {
		if err := c.trustedProjectObjects(scope, p, skill, want); err != nil {
			return err
		}
	}
	if len(binding.NativeObjects) != len(want) {
		return fmt.Errorf("exact native ownership inventory differs")
	}
	seen := map[string]bool{}
	for _, object := range binding.NativeObjects {
		expected, ok := want[object.Kind]
		if !ok || seen[object.Kind] || !reflect.DeepEqual(object, expected) {
			return fmt.Errorf("native ownership identity/path/digest differs")
		}
		seen[object.Kind] = true
	}
	return nil
}

// Call immediately after the actual public user add, before project installs
// or any installed native host. No later observation may reset this baseline.
func (c *controller) trustedPreserveUser() error {
	f := c.trustedState
	p := f.Packages["user"]
	paths := []string{filepath.Join(f.Root, "installer-state", "state-v2.json"), p.Binding.TargetLocator}
	for _, data := range p.Installation.DataReceipts {
		if !trustedContained(f.Root, data.Locator) {
			return fmt.Errorf("user data escapes fixture")
		}
		paths = append(paths, data.Locator)
	}
	for _, path := range paths {
		if err := c.trustedProtect(path); err != nil {
			return err
		}
	}
	return nil
}

// Expected bytes derive from the authored package and selected receipt. They
// are never adopted from the native document under observation.
func trustedExpectedServer(f *trustedLifecycle, p *trustedPackage, scope string) map[string]any {
	active := p.Binding.TargetLocator
	return map[string]any{"type": "stdio", "command": filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName),
		"args": []any{"--internal-stdio-v1", active, p.Data, active, "plugin", "--", "./bin/probe", "argument with spaces", active + "/argument", "${UNKNOWN}"},
		"env":  map[string]any{"PLUGIN_ROOT": active, "PLUGIN_DATA": p.Data, "UAP_TEST_ROOT": f.Root, "UAP_TEST_EVENTS": filepath.Join(f.Root, scope+"-events.jsonl"), "UAP_TEST_NONCE": f.Nonce, "UAP_TEST_REVISION": p.Revision, "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": active + "/${UNKNOWN}"}}
}

func (c *controller) trustedReceiptInstallation(scope string, p *trustedPackage) (domain.Installation, error) {
	f := c.trustedState
	statePath := filepath.Join(f.Projects[scope], ".uap", "state-v2.json")
	if scope == "user" {
		statePath = filepath.Join(f.Root, "installer-state", "state-v2.json")
	}
	body, err := readOrdinary(statePath, 1<<20)
	if err != nil {
		return domain.Installation{}, err
	}
	if _, err := uniqueJSON(body); err != nil {
		return domain.Installation{}, err
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(body, &state); err != nil {
		return domain.Installation{}, err
	}
	selected := []domain.Installation{}
	for _, i := range state.Installations {
		if i.DeclaredName == p.Name {
			selected = append(selected, i)
		}
	}
	if len(selected) != 1 || len(selected[0].Clients) != 1 {
		return domain.Installation{}, fmt.Errorf("exact public receipt selection differs")
	}
	i := selected[0]
	return i, nil
}

func trustedReceiptSource(p *trustedPackage, i domain.Installation) error {
	wantVersion := "1.0.0"
	if p.Revision == "a-two" {
		wantVersion = "1.0.1"
	}
	if p.SourceDigest == "" || p.ManifestDigest == "" || i.Source.CanonicalSource != p.Source || i.Source.RequestedSource != p.Source || i.Source.SourceBindingID != domain.ComputeSourceBindingID(domain.SourceIdentity{CanonicalSource: p.Source}) || i.Source.TreeDigest != p.SourceDigest || i.Source.Repository != "" || i.Source.PackageSubpath != "" || i.Source.ResolvedRevision != "" || i.Package.DeclaredName != p.Name || i.Package.ManifestDigest != p.ManifestDigest || i.Package.Version != wantVersion {
		return fmt.Errorf("public source/package identity differs")
	}
	return nil
}

func (c *controller) trustedReceiptBinding(scope string, p *trustedPackage, i domain.Installation, binding domain.ClientBinding) error {
	f := c.trustedState
	wantScope := "project"
	if scope == "user" {
		wantScope = "user"
	}
	if binding.ClientID != "vscode" || binding.Scope != wantScope || binding.Activation != domain.ActivationPrepared || binding.TargetLocator == "" || binding.PhysicalArtifact != domain.ComputePhysicalArtifactID(p.Name, i.InstallationID) || binding.PackageRevision == nil || binding.PackageRevision.TreeDigest != p.SourceDigest || binding.PackageRevision.ManifestDigest != p.ManifestDigest {
		return fmt.Errorf("public receipt scope/Prepared state differs")
	}
	if !trustedContained(f.Root, binding.TargetLocator) {
		return fmt.Errorf("receipt package escaped fixture")
	}
	if scope != "user" {
		if err := c.trustedProjectReceipt(scope, p, i, binding); err != nil {
			return err
		}
	} else {
		p.Skill = filepath.Join(binding.TargetLocator, "skills", p.Name, "SKILL.md")
	}
	p.Binding = binding
	return nil
}

func (c *controller) trustedProjectReceipt(scope string, p *trustedPackage, i domain.Installation, binding domain.ClientBinding) error {
	f := c.trustedState
	if binding.TargetLocator != filepath.Join(f.Projects[scope], ".uap", "managed", "clients", "vscode", binding.PhysicalArtifact) || binding.NativeProfileRoot != filepath.Join(f.Projects[scope], ".vscode") || binding.Verification != domain.VerificationInstalled {
		return fmt.Errorf("project receipt authority differs")
	}
	data := i.DataReceipts[binding.DataReceiptID]
	if data.DataReceiptID == "" || data.DataReceiptID != binding.DataReceiptID || data.State != domain.DataReceiptOwned || data.Scope != "project" || data.OwnershipDigest == "" || data.PhysicalBackend != binding.PhysicalArtifact || !trustedContained(filepath.Join(f.Projects[scope], ".uap", "plugin-data"), data.Locator) {
		return fmt.Errorf("persistent data authority differs")
	}
	p.Data = data.Locator
	p.Skill = filepath.Join(f.Projects[scope], ".github", "skills", p.Name, "SKILL.md")
	return nil
}

func (c *controller) trustedReceiptFiles(p *trustedPackage) (string, error) {
	active := p.Binding.TargetLocator
	// Preserve every untransformed source file, especially the actual probe.
	for _, relative := range []string{"plugin.json", "skills/" + p.Name + "/SKILL.md"} {
		want, err := readOrdinary(filepath.Join(p.Source, filepath.FromSlash(relative)), 1<<20)
		if err != nil {
			return "", err
		}
		got, err := readOrdinary(filepath.Join(active, filepath.FromSlash(relative)), 1<<20)
		if err != nil || !bytes.Equal(got, want) {
			return "", fmt.Errorf("managed package differs from frozen authored source")
		}
	}
	digest, err := trustedInstalledPackageDigest(c.work, active)
	if err != nil {
		return "", err
	}
	return digest, nil
}

func (c *controller) trustedProjectObjects(scope string, p *trustedPackage, skill []byte, want map[string]domain.NativeObjectOwnership) error {
	binding := p.Binding
	active := binding.TargetLocator
	for path, expected := range map[string]string{filepath.Join(active, "bin", "probe"): c.request.Input.Trusted.ProbeSHA256, filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName): c.request.Input.Trusted.InstallerSHA256} {
		info, err := anchoredInfo(path)
		if err != nil || !regular(info, 128<<20) || info.Mode().Perm()&0111 == 0 {
			return fmt.Errorf("installed probe/launcher executable identity differs")
		}
		actual, err := hashFile(path, 128<<20)
		if err != nil || actual != expected {
			return fmt.Errorf("installed probe/launcher differs from sealed exact binary")
		}
	}
	sum := sha256.Sum256(append(append([]byte("f\x00SKILL.md\x00-\x00"), skill...), 0))
	kind := "vscode_project_skill_directory"
	want[kind] = domain.NativeObjectOwnership{ObjectID: kind + ":" + p.Name, Kind: kind, LogicalName: p.Name, Path: filepath.Dir(p.Skill), SourceRelative: "skills/" + p.Name, ManagedDigest: "sha256:" + hex.EncodeToString(sum[:]), ProtectionClass: "managed"}
	object, err := c.trustedMCPObject(scope, p)
	if err != nil {
		return err
	}
	want[object.Kind] = object
	return nil
}

func (c *controller) trustedMCPObject(scope string, p *trustedPackage) (domain.NativeObjectOwnership, error) {
	f := c.trustedState
	expected := trustedExpectedServer(f, p, scope)
	args, env := make([]string, 0, len(expected["args"].([]any))), map[string]string{}
	for _, value := range expected["args"].([]any) {
		args = append(args, value.(string))
	}
	for key, value := range expected["env"].(map[string]any) {
		env[key] = value.(string)
	}
	config := filepath.Join(f.Projects[scope], ".vscode", "mcp.json")
	body, err := readOrdinary(config, 1<<20)
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	value, err := uniqueJSON(body)
	document, ok := value.(map[string]any)
	if err != nil || !ok {
		return domain.NativeObjectOwnership{}, fmt.Errorf("installed native config is malformed")
	}
	entries, ok := document["servers"].(map[string]any)
	if !ok || !reflect.DeepEqual(entries[p.Name], expected) {
		return domain.NativeObjectOwnership{}, fmt.Errorf("installed native projection differs from independent source/receipt")
	}
	receipt, err := nativeconfig.DesiredReceipt(config, nativeconfig.CodecVSCode, p.Name, nativeconfig.Server{Type: "stdio", Command: expected["command"].(string), Args: args, Env: env, StdioValuesResolved: true, CWDResolved: true}, nativeconfig.Placeholders{})
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	kind := "vscode_project_mcp_server"
	return domain.NativeObjectOwnership{ObjectID: kind + ":" + p.Name, Kind: kind, LogicalName: p.Name, Path: config, ManagedDigest: receipt.Digest, ProtectionClass: "managed"}, nil
}
