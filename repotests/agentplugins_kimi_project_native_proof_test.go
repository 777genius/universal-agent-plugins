//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

type kimiProjectProofFile struct {
	Body       []byte
	Executable bool
}

// The recipe is frozen before add/update. No field below is obtained from the
// installed package, a returned ownership receipt or a later source reread.
type kimiProjectProof struct {
	scope, source, name, version, root, home, project, nonce, revision string
	installerSHA                                                       string
	authored                                                           map[string]kimiProjectProofFile
	sourceIdentity                                                     domain.SourceBinding
	packageIdentity                                                    domain.PackageBinding
	stable                                                             *kimiProjectProofAuthority
}

type kimiProjectProofAuthority struct {
	installation, binding, artifact, active, profile string
	dataID, data, dataDigest                         string
}

type kimiProjectValidated struct {
	authority  kimiProjectProofAuthority
	binding    domain.ClientBinding
	skill      string
	identities map[string]string
}

func kimiProjectProofSHA(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func kimiProjectProofState(body []byte) (domain.StateFileV2, error) {
	var state domain.StateFileV2
	if _, err := kimiProjectObject(body); err != nil {
		return state, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return state, err
	}
	if state.SchemaVersion != domain.StateSchemaVersion {
		return state, fmt.Errorf("Kimi state schema differs")
	}
	return state, nil
}

func newKimiProjectProof(ctx context.Context, scope, source, name, version, root, home, project, nonce, revision string, authored map[string]kimiProjectProofFile, installerSHA string) (*kimiProjectProof, error) {
	p := &kimiProjectProof{scope: scope, source: source, name: name, version: version, root: root, home: home, project: project, nonce: nonce, revision: revision, installerSHA: installerSHA, authored: map[string]kimiProjectProofFile{}}
	for path, file := range authored {
		p.authored[path] = kimiProjectProofFile{Body: bytes.Clone(file.Body), Executable: file.Executable}
	}
	entries := kimiProjectProofEntries(p.authored)
	digest, err := packagedigest.DigestCaptured(ctx, entries)
	if err != nil {
		return nil, err
	}
	p.sourceIdentity = domain.SourceBinding{SourceBindingID: domain.ComputeSourceBindingID(domain.SourceIdentity{CanonicalSource: source}), RequestedSource: source, CanonicalSource: source, TreeDigest: digest}
	inventory := domain.ComponentInventory{Skills: []string{name}}
	if scope != "user" {
		inventory.MCPPresent, inventory.MCPEnabled, inventory.MCPServers = true, true, []string{name}
	}
	p.packageIdentity = domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: name, Version: version, ManifestDigest: "sha256:" + kimiProjectProofSHA(p.authored["plugin.json"].Body), Inventory: inventory}
	if err := p.validateSource(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

func kimiProjectProofEntries(files map[string]kimiProjectProofFile) []packagedigest.CapturedEntry {
	directories := map[string]bool{}
	entries := make([]packagedigest.CapturedEntry, 0, len(files))
	for path, file := range files {
		entries = append(entries, packagedigest.CapturedEntry{Path: path, Kind: "file", Executable: file.Executable, Content: bytes.Clone(file.Body)})
		for parent := filepath.ToSlash(filepath.Dir(path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			directories[parent] = true
		}
	}
	for path := range directories {
		entries = append(entries, packagedigest.CapturedEntry{Path: path, Kind: "directory"})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

// Bounded ordinary-file captures retain bytes, executable bits and the complete
// directory closure. The held reader rejects links, aliases and file drift.
func kimiProjectProofCapture(ctx context.Context, root string) ([]packagedigest.CapturedEntry, error) {
	if err := projectNativeRuntimeParents(filepath.Join(root, "leaf")); err != nil {
		return nil, err
	}
	var captured []packagedigest.CapturedEntry
	var total int64
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return err
		}
		count++
		if count > 4096 || len(strings.Split(rel, string(filepath.Separator))) > 32 {
			return fmt.Errorf("Kimi captured closure entry/depth bound")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if err := projectNativeRuntimeParents(filepath.Join(path, "leaf")); err != nil {
				return err
			}
			captured = append(captured, packagedigest.CapturedEntry{Path: filepath.ToSlash(rel), Kind: "directory"})
			return nil
		}
		total += info.Size()
		if !info.Mode().IsRegular() || info.Size() < 0 || total > 256<<20 {
			return fmt.Errorf("Kimi captured ordinary-file/byte bound")
		}
		body, err := projectNativeRuntimeRead(path, 256<<20)
		if err != nil {
			return err
		}
		after, err := os.Lstat(path)
		if err != nil || !projectNativeRuntimeStableStat(info.Sys(), after.Sys()) {
			return fmt.Errorf("Kimi capture executable identity drift")
		}
		captured = append(captured, packagedigest.CapturedEntry{Path: filepath.ToSlash(rel), Kind: "file", Executable: info.Mode().Perm()&0111 != 0, Content: body})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, err := packagedigest.DigestCaptured(ctx, captured); err != nil {
		return nil, err
	}
	return captured, nil
}

func (p *kimiProjectProof) validateSource(ctx context.Context) error {
	actual, err := kimiProjectProofCapture(ctx, p.source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, kimiProjectProofEntries(p.authored)) {
		return fmt.Errorf("Kimi source closure differs from frozen authored bytes/modes/directories")
	}
	return nil
}

func (p *kimiProjectProof) authority(i domain.Installation) (kimiProjectProofAuthority, domain.ClientBinding, error) {
	var a kimiProjectProofAuthority
	var b domain.ClientBinding
	if !projectNativeRuntimeText(i.InstallationID, 64) || i.InstallationID == "" || i.DeclaredName != p.name || !reflect.DeepEqual(i.Source, p.sourceIdentity) || !reflect.DeepEqual(i.Package, p.packageIdentity) || len(i.Clients) != 1 {
		return a, b, fmt.Errorf("Kimi source/tree/package/manifest identity differs from authored recipe")
	}
	a.installation, a.artifact = i.InstallationID, domain.ComputePhysicalArtifactID(p.name, i.InstallationID)
	scope := "user"
	activation := domain.ActivationActive
	a.active = filepath.Join(p.home, ".kimi-code", "plugins", "managed", a.artifact)
	if p.scope != "user" {
		scope, activation = "project", domain.ActivationPrepared
		a.profile = filepath.Join(p.project, ".kimi-code")
		a.active = filepath.Join(p.project, ".uap", "managed", "clients", "kimi", a.artifact)
		a.dataID = "data_" + kimiProjectProofSHA([]byte(strings.Join([]string{i.InstallationID, a.artifact, scope}, "\x00")))[:24]
		a.data = filepath.Join(p.project, ".uap", "plugin-data", a.artifact)
		a.dataDigest = "sha256:" + kimiProjectProofSHA([]byte("agentplugins-plugin-data-v1\x00"+a.dataID+"\x00"+a.data))
	}
	a.binding = domain.ComputeClientBindingID(i.InstallationID, "kimi", scope, a.active)
	var ok bool
	b, ok = i.Clients[a.binding]
	wantRevision := &domain.ClientPackageRevision{Version: p.version, TreeDigest: p.sourceIdentity.TreeDigest, ManifestDigest: p.packageIdentity.ManifestDigest}
	if !ok || b.ClientBindingID != a.binding || b.ClientID != "kimi" || b.Scope != scope || b.PhysicalArtifact != a.artifact || b.TargetLocator != a.active || b.NativeProfileRoot != a.profile || b.DataReceiptID != a.dataID || b.Materialization != domain.MaterializationMaterialized || b.Activation != activation || b.Verification != domain.VerificationInstalled || !reflect.DeepEqual(b.PackageRevision, wantRevision) {
		return a, b, fmt.Errorf("Kimi selected binding identity/scope/locator/revision/status differs")
	}
	if p.scope == "user" {
		if len(i.DataReceipts) != 0 {
			return a, b, fmt.Errorf("Kimi skill-only user package acquired unknown data authority")
		}
	} else {
		d := i.DataReceipts[a.dataID]
		if len(i.DataReceipts) != 1 || d.DataReceiptID != a.dataID || d.PhysicalBackend != a.artifact || d.Scope != scope || d.Locator != a.data || d.OwnershipDigest != a.dataDigest || d.State != domain.DataReceiptOwned {
			return a, b, fmt.Errorf("Kimi exact selected data authority differs")
		}
	}
	if p.stable != nil && *p.stable != a {
		return a, b, fmt.Errorf("Kimi original installation/binding/artifact/data authority changed")
	}
	return a, b, nil
}

// No identities or persistent baseline escape this boundary on partial proof.
func (p *kimiProjectProof) validate(ctx context.Context, i domain.Installation) (kimiProjectValidated, error) {
	var refused kimiProjectValidated
	if err := p.validateSource(ctx); err != nil {
		return refused, err
	}
	a, b, err := p.authority(i)
	if err != nil {
		return refused, err
	}
	entries, err := kimiProjectProofCapture(ctx, a.active)
	if err != nil {
		return refused, err
	}
	if err := p.validateInstalled(entries, a); err != nil {
		return refused, err
	}
	digest, err := projectNativeRuntimeInstalledDigest(ctx, entries)
	if err != nil {
		return refused, err
	}
	expected, err := p.expectedObjects(a, digest)
	if err != nil {
		return refused, err
	}
	if err := kimiProjectProofObjects(b.NativeObjects, expected); err != nil {
		return refused, err
	}
	skill := filepath.Join(a.active, "skills", p.name, "SKILL.md")
	identities := map[string]string{}
	if p.scope != "user" {
		skill = filepath.Join(a.profile, "skills", p.name, "SKILL.md")
		nativeSkill, err := kimiProjectProofCapture(ctx, filepath.Dir(skill))
		wantSkill := []packagedigest.CapturedEntry{{Path: "SKILL.md", Kind: "file", Content: p.authored["skills/"+p.name+"/SKILL.md"].Body}}
		if err != nil || !reflect.DeepEqual(nativeSkill, wantSkill) {
			return refused, fmt.Errorf("Kimi delivered skill closure differs from frozen authored skill")
		}
		if err := p.validateNativeMCP(a); err != nil {
			return refused, err
		}
		if _, err := projectNativeRuntimeRead(filepath.Join(a.data, ".agentplugins-data-owner.json"), kimiProjectNativeLimit); err != nil {
			return refused, err
		}
		if err := (providers.PluginDataManager{Base: filepath.Join(p.project, ".uap", "plugin-data")}).ValidateData(ctx, i.DataReceipts[a.dataID]); err != nil {
			return refused, err
		}
		identities[filepath.Join(a.active, "bin", "probe")] = kimiProjectProofSHA(p.authored["bin/probe"].Body)
		identities[filepath.Join(a.active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)] = p.installerSHA
	}
	if p.stable == nil {
		copyAuthority := a
		p.stable = &copyAuthority
	}
	return kimiProjectValidated{authority: a, binding: b, skill: skill, identities: identities}, nil
}

func kimiProjectProofObjects(actual []domain.NativeObjectOwnership, expected map[string]domain.NativeObjectOwnership) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("Kimi exact ownership count differs")
	}
	seen := map[string]bool{}
	for _, object := range actual {
		want, ok := expected[object.Kind]
		if !ok || seen[object.Kind] || !reflect.DeepEqual(object, want) {
			return fmt.Errorf("Kimi ownership kind/identity/path/digest differs")
		}
		seen[object.Kind] = true
	}
	return nil
}

func (p *kimiProjectProof) expectedObjects(a kimiProjectProofAuthority, digest string) (map[string]domain.NativeObjectOwnership, error) {
	objects := map[string]domain.NativeObjectOwnership{
		"managed_package_directory": {ObjectID: "package:kimi:" + a.artifact, Kind: "managed_package_directory", LogicalName: p.name, Path: a.active, ManagedDigest: digest, ProtectionClass: "managed"},
	}
	if p.scope == "user" {
		return objects, nil
	}
	// This one-file skill has the existing neutral skill framing, independently
	// assembled from authored bytes rather than the delivered skill directory.
	skill := p.authored["skills/"+p.name+"/SKILL.md"].Body
	skillDigest := "sha256:" + kimiProjectProofSHA(append(append([]byte("f\x00SKILL.md\x00-\x00"), skill...), 0))
	objects["kimi_project_skill_directory"] = domain.NativeObjectOwnership{ObjectID: "kimi_project_skill_directory:" + p.name, Kind: "kimi_project_skill_directory", LogicalName: p.name, Path: filepath.Join(a.profile, "skills", p.name), SourceRelative: "skills/" + p.name, ManagedDigest: skillDigest, ProtectionClass: "managed"}
	body, err := json.Marshal(p.expectedServer(a))
	if err != nil {
		return nil, err
	}
	var server nativeconfig.Server
	if err := json.Unmarshal(body, &server); err != nil {
		return nil, err
	}
	server.Type, server.StdioValuesResolved, server.CWDResolved = "stdio", true, true
	path := filepath.Join(a.profile, "mcp.json")
	receipt, err := nativeconfig.DesiredReceipt(path, nativeconfig.CodecKimi, p.name, server, nativeconfig.Placeholders{})
	if err != nil {
		return nil, err
	}
	objects["kimi_project_mcp_server"] = domain.NativeObjectOwnership{ObjectID: "kimi_project_mcp_server:" + p.name, Kind: "kimi_project_mcp_server", LogicalName: p.name, Path: path, ManagedDigest: receipt.Digest, ProtectionClass: "managed"}
	return objects, nil
}

func (p *kimiProjectProof) expectedServer(a kimiProjectProofAuthority) map[string]any {
	selected := &kimiProjectNativePackage{Revision: p.revision, Data: a.data, Binding: domain.ClientBinding{TargetLocator: a.active}}
	return kimiProjectExpectedServer(selected, p.scope, p.root, p.nonce)
}

func (p *kimiProjectProof) validateNativeMCP(a kimiProjectProofAuthority) error {
	body, err := projectNativeRuntimeRead(filepath.Join(a.profile, "mcp.json"), kimiProjectNativeLimit)
	if err != nil {
		return err
	}
	doc, err := kimiProjectObject(body)
	if err != nil {
		return err
	}
	entries, ok := doc["mcpServers"].(map[string]any)
	if !ok || !reflect.DeepEqual(entries[p.name], p.expectedServer(a)) {
		return fmt.Errorf("Kimi native MCP entry differs from independent authored projection")
	}
	return nil
}
