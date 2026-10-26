//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

// Only inert data is authored here. No public CLI, publisher, probe or vendor
// executable is executed; these regressions exercise receipt and held-file IO.
func trustedReceiptFixture(t *testing.T, scope string) (*controller, domain.StateFileV2, string) {
	t.Helper()
	base := t.TempDir()
	c := &controller{request: sealedRequest{Base: base, Input: inputRequest{UID: os.Getuid(), GID: os.Getgid(), Trusted: &trustedInput{}}}, work: context.Background()}
	f := &trustedLifecycle{Root: filepath.Join(base, "trusted"), Projects: map[string]string{}, Packages: map[string]*trustedPackage{}, Protected: map[string]map[string]string{}, Foreign: map[string]any{}, Nonce: "fixed-nonce", Marker: "fixed-marker"}
	c.trustedState = f
	f.Projects[scope] = filepath.Join(f.Root, "project-"+scope)
	for _, path := range []string{filepath.Join(base, "executables"), f.Root, f.Projects[scope]} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	probe, installer := []byte("inert probe identity\n"), []byte("inert installer identity\n")
	for name, body := range map[string][]byte{"native-probe": probe, "agentplugins": installer} {
		if err := os.WriteFile(filepath.Join(base, "executables", name), body, 0700); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		if name == "native-probe" {
			c.request.Input.Trusted.ProbeSHA256 = hex.EncodeToString(sum[:])
		} else {
			c.request.Input.Trusted.InstallerSHA256 = hex.EncodeToString(sum[:])
		}
	}
	if err := c.trustedSource(scope, scope+"-one"); err != nil {
		t.Fatal(err)
	}
	p := f.Packages[scope]
	installationID := "12345678-1234-4234-9234-123456789012"
	physical := domain.ComputePhysicalArtifactID(p.Name, installationID)
	active := filepath.Join(f.Projects[scope], ".uap", "managed", "clients", "vscode", physical)
	statePath := filepath.Join(f.Projects[scope], ".uap", "state-v2.json")
	if scope == "user" {
		active = filepath.Join(f.Root, "installer-state", "managed", "clients", "vscode", physical)
		statePath = filepath.Join(f.Root, "installer-state", "state-v2.json")
	}
	for _, relative := range []string{"plugin.json", "skills/" + p.Name + "/SKILL.md"} {
		body, err := os.ReadFile(filepath.Join(p.Source, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.trustedWrite(filepath.Join(active, filepath.FromSlash(relative)), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	b := domain.ClientBinding{ClientBindingID: "client-fixture", ClientID: "vscode", Scope: scope, PhysicalArtifact: physical, TargetLocator: active, Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled,
		PackageRevision: &domain.ClientPackageRevision{TreeDigest: p.SourceDigest, ManifestDigest: p.ManifestDigest}, NativeProfileRoot: filepath.Join(f.Projects[scope], ".vscode")}
	i := domain.Installation{InstallationID: installationID, DeclaredName: p.Name, Source: domain.SourceBinding{RequestedSource: p.Source, CanonicalSource: p.Source, SourceBindingID: domain.ComputeSourceBindingID(domain.SourceIdentity{CanonicalSource: p.Source}), TreeDigest: p.SourceDigest}, Package: domain.PackageBinding{DeclaredName: p.Name, Version: "1.0.0", ManifestDigest: p.ManifestDigest}, DataReceipts: map[string]domain.DataReceipt{}}
	if scope != "user" {
		b.Scope = "project"
		b.DataReceiptID = "data-fixture"
		p.Data = filepath.Join(f.Projects[scope], ".uap", "plugin-data", "data-fixture")
		if err := c.trustedDirectory(p.Data); err != nil {
			t.Fatal(err)
		}
		i.DataReceipts[b.DataReceiptID] = domain.DataReceipt{DataReceiptID: b.DataReceiptID, State: domain.DataReceiptOwned, Scope: "project", OwnershipDigest: "sha256:owned", PhysicalBackend: physical, Locator: p.Data}
		p.Skill = filepath.Join(f.Projects[scope], ".github", "skills", p.Name, "SKILL.md")
		skill, err := os.ReadFile(filepath.Join(p.Source, "skills", p.Name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.trustedWrite(p.Skill, skill, 0600); err != nil {
			t.Fatal(err)
		}
		for path, body := range map[string][]byte{filepath.Join(active, "bin", "probe"): probe, filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName): installer} {
			if err := c.trustedWrite(path, body, 0700); err != nil {
				t.Fatal(err)
			}
		}
		p.Binding = b
		expected := trustedExpectedServer(f, p, scope)
		config := filepath.Join(f.Projects[scope], ".vscode", "mcp.json")
		if err := c.trustedJSON(config, map[string]any{"servers": map[string]any{p.Name: expected}}); err != nil {
			t.Fatal(err)
		}
		args, env := []string{}, map[string]string{}
		for _, value := range expected["args"].([]any) {
			args = append(args, value.(string))
		}
		for key, value := range expected["env"].(map[string]any) {
			env[key] = value.(string)
		}
		receipt, err := nativeconfig.DesiredReceipt(config, nativeconfig.CodecVSCode, p.Name, nativeconfig.Server{Type: "stdio", Command: expected["command"].(string), Args: args, Env: env, StdioValuesResolved: true, CWDResolved: true}, nativeconfig.Placeholders{})
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(append(append([]byte("f\x00SKILL.md\x00-\x00"), skill...), 0))
		b.NativeObjects = []domain.NativeObjectOwnership{
			{ObjectID: "vscode_project_skill_directory:" + p.Name, Kind: "vscode_project_skill_directory", LogicalName: p.Name, Path: filepath.Dir(p.Skill), SourceRelative: "skills/" + p.Name, ManagedDigest: "sha256:" + hex.EncodeToString(sum[:]), ProtectionClass: "managed"},
			{ObjectID: "vscode_project_mcp_server:" + p.Name, Kind: "vscode_project_mcp_server", LogicalName: p.Name, Path: config, ManagedDigest: receipt.Digest, ProtectionClass: "managed"},
		}
	}
	digest, err := trustedPackageDigest(c.work, active)
	if err != nil {
		t.Fatal(err)
	}
	b.NativeObjects = append(b.NativeObjects, domain.NativeObjectOwnership{ObjectID: "package:vscode:" + physical, Kind: "managed_package_directory", LogicalName: p.Name, Path: active, ManagedDigest: digest, ProtectionClass: "managed"})
	i.Clients = map[string]domain.ClientBinding{b.ClientBindingID: b}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{i}}
	if err := c.trustedJSON(statePath, state); err != nil {
		t.Fatal(err)
	}
	if err := c.trustedReceipt(scope); err != nil {
		t.Fatalf("valid independent receipt: %v", err)
	}
	return c, state, statePath
}

func TestTrustedReceiptRejectsWrongSourceObjectsAndInstalledProbe(t *testing.T) {
	for _, kind := range []string{"source-tree", "manifest", "source-binding", "revision", "object-digest", "object-id", "object-path", "object-extra", "probe-adoption", "native-extra"} {
		t.Run(kind, func(t *testing.T) {
			c, state, path := trustedReceiptFixture(t, "a")
			i := &state.Installations[0]
			b := i.Clients["client-fixture"]
			switch kind {
			case "source-tree":
				i.Source.TreeDigest = "sha256:wrong"
			case "manifest":
				i.Package.ManifestDigest = "sha256:wrong"
			case "source-binding":
				i.Source.SourceBindingID = "src_other"
			case "revision":
				b.PackageRevision.TreeDigest = "sha256:wrong"
			case "object-digest":
				b.NativeObjects[0].ManagedDigest = "sha256:wrong"
			case "object-id":
				b.NativeObjects[0].ObjectID = "other"
			case "object-path":
				b.NativeObjects[0].Path = filepath.Join(c.trustedState.Root, "other")
			case "object-extra":
				b.NativeObjects = append(b.NativeObjects, domain.NativeObjectOwnership{Kind: "unknown", ObjectID: "unknown"})
			case "probe-adoption":
				if err := os.WriteFile(filepath.Join(b.TargetLocator, "bin", "probe"), []byte("changed native payload"), 0700); err != nil {
					t.Fatal(err)
				}
				digest, err := trustedPackageDigest(c.work, b.TargetLocator)
				if err != nil {
					t.Fatal(err)
				}
				b.NativeObjects[len(b.NativeObjects)-1].ManagedDigest = digest
			case "native-extra":
				p := c.trustedState.Packages["a"]
				server := trustedExpectedServer(c.trustedState, p, "a")
				server["cwd"] = b.TargetLocator
				if err := c.trustedJSON(filepath.Join(c.trustedState.Projects["a"], ".vscode", "mcp.json"), map[string]any{"servers": map[string]any{p.Name: server}}); err != nil {
					t.Fatal(err)
				}
			}
			i.Clients["client-fixture"] = b
			if err := c.trustedJSON(path, state); err != nil {
				t.Fatal(err)
			}
			if err := c.trustedReceipt("a"); err == nil {
				t.Fatal("unqualified source/ownership/probe accepted", kind)
			}
		})
	}
}

func TestTrustedSourcesAndPreparedUserAreFrozenBeforeNativeObservation(t *testing.T) {
	for _, kind := range []string{"authored-source", "user-package", "user-receipt", "user-data"} {
		t.Run(kind, func(t *testing.T) {
			c, _, statePath := trustedReceiptFixture(t, "user")
			p := c.trustedState.Packages["user"]
			p.Installation.DataReceipts = map[string]domain.DataReceipt{}
			data := filepath.Join(c.trustedState.Root, "user-data")
			if err := c.trustedDirectory(data); err != nil {
				t.Fatal(err)
			}
			p.Installation.DataReceipts["user-data"] = domain.DataReceipt{Locator: data}
			if err := c.trustedPreserveUser(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(p.Source, "plugin.json")
			if kind == "user-package" {
				path = filepath.Join(p.Binding.TargetLocator, "plugin.json")
			}
			if kind == "user-receipt" {
				path = statePath
			}
			if kind == "user-data" {
				path = filepath.Join(data, "unexpected")
			}
			if err := os.WriteFile(path, []byte("must not be adopted before installed-a"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := c.trustedPreserved(); err == nil {
				t.Fatal("changed pre-native baseline accepted", kind)
			}
		})
	}
}

func TestTrustedInstalledPackageCannotAdoptExtraFileAndRewrittenReceipt(t *testing.T) {
	c, state, statePath := trustedReceiptFixture(t, "a")
	p := c.trustedState.Packages["a"]
	if err := c.trustedProtect(p.Binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Binding.TargetLocator, "unreviewed-extra"), []byte("native added payload"), 0600); err != nil {
		t.Fatal(err)
	}
	digest, err := trustedPackageDigest(c.work, p.Binding.TargetLocator)
	if err != nil {
		t.Fatal(err)
	}
	b := state.Installations[0].Clients["client-fixture"]
	for index := range b.NativeObjects {
		if b.NativeObjects[index].Kind == "managed_package_directory" {
			b.NativeObjects[index].ManagedDigest = digest
		}
	}
	state.Installations[0].Clients["client-fixture"] = b
	if err := c.trustedJSON(statePath, state); err != nil {
		t.Fatal(err)
	}
	if err := c.trustedReceipt("a"); err == nil {
		t.Fatal("native extra and rewritten receipt were adopted after public install")
	}
}

func TestTrustedHeldMutationRejectsLeafSwapAndParentAlias(t *testing.T) {
	for _, directory := range []bool{false, true} {
		root := t.TempDir()
		path := filepath.Join(root, "owned")
		foreign := filepath.Join(t.TempDir(), "foreign")
		if directory {
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(foreign, 0700); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(path, []byte("owned"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(foreign, []byte("foreign unchanged"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		before, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		parent, err := trustedParent(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(foreign, path); err != nil {
			t.Fatal(err)
		}
		if file, err := trustedOpenHeld(parent, "owned", before, directory); err == nil {
			_ = file.Close()
			t.Fatal("post-check alias opened for root mutation")
		}
		_ = parent.Close()
		if _, err := trustedParent(filepath.Join(path, "child")); err == nil {
			t.Fatal("aliased parent accepted")
		}
		if !directory {
			body, err := os.ReadFile(foreign)
			if err != nil || !bytes.Equal(body, []byte("foreign unchanged")) {
				t.Fatal("foreign file changed", err)
			}
		}
	}
}

func TestTrustedCapturedPackageDigestMatchesInstallerFraming(t *testing.T) {
	c, _, _ := trustedReceiptFixture(t, "a")
	root := c.trustedState.Packages["a"].Source
	got, err := trustedPackageDigest(c.work, root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := (packagedigest.Builder{TempRoot: t.TempDir()}).Snapshot(c.work, root, domain.SourceIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	defer packagedigest.Remove(snapshot)
	if got != snapshot.TreeDigest {
		t.Fatal("captured digest differs from real installer framing")
	}
}
