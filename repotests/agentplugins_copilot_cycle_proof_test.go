//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type copilotCycleNativeSnapshot struct {
	Active           string                         `json:"active"`
	ManagedFiles     map[string]string              `json:"managed_files"`
	Objects          []domain.NativeObjectOwnership `json:"objects"`
	SkillPath        string                         `json:"skill_path"`
	Skill            *copilotCycleBytes             `json:"skill"`
	ConfigPath       string                         `json:"config_path"`
	Config           *copilotCycleBytes             `json:"config"`
	SkillFiles       map[string]string              `json:"skill_files"`
	LauncherMetadata *copilotCycleBytes             `json:"launcher_metadata"`
}

type copilotCycleDataSnapshot struct {
	Allocated bool                `json:"allocated"`
	Path      string              `json:"path"`
	Receipt   *domain.DataReceipt `json:"receipt"`
	Owner     *copilotCycleBytes  `json:"owner"`
	Marker    *copilotCycleBytes  `json:"marker"`
	Files     map[string]string   `json:"files"`
}

func copilotCycleSource(i domain.Installation, r *copilotCycleRecipe) error {
	source := domain.SourceBinding{SourceBindingID: "src_" + copilotProjectConfigHash([]byte(r.Source + "\x00\x00"))[:24], RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: r.Digest}
	inventory := domain.ComponentInventory{Skills: []string{r.Skill}}
	if r.Key != "user" {
		inventory.MCPPresent, inventory.MCPEnabled, inventory.MCPServers = true, true, []string{r.Skill}
	}
	pkg := domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1,
		DeclaredName: "native-proof", Version: r.Version, ManifestDigest: "sha256:" + copilotProjectConfigHash(r.Authored["plugin.json"]), Inventory: inventory}
	if !copilotReadinessID(i.InstallationID) || i.DeclaredName != "native-proof" || !reflect.DeepEqual(i.Source, source) || !reflect.DeepEqual(i.Package, pkg) {
		return fmt.Errorf("Copilot cycle source/package differs from independently authored recipe")
	}
	return nil
}

func (c *copilotProjectCycle) proof(t *testing.T, r *copilotCycleRecipe, stage string) {
	t.Helper()
	raw, err := projectNativeRuntimeRead(c.statePath(r), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	state, err := clineCycleState(raw)
	if err != nil || len(state.Installations) != 1 {
		t.Fatal("exact single Copilot cycle installation required", err)
	}
	i := state.Installations[0]
	if err := copilotCycleSource(i, r); err != nil {
		t.Fatal(err)
	}
	if r.Installation.InstallationID != "" && i.InstallationID != r.Installation.InstallationID {
		t.Fatal("Copilot cycle logical installation identity changed")
	}
	if r.Removed {
		if len(i.Clients) != 0 || !i.DataRetained {
			t.Fatal("Copilot default removal did not retain its data-only installation")
		}
	} else {
		c.bindingProof(t, r, i)
	}
	data := c.dataSnapshot(t, r, i)
	native := c.nativeSnapshot(t, r)
	r.Installation = i
	c.append(t, "cycle-proof.jsonl", stage, map[string]any{"scope": r.Key, "revision": r.Revision, "version": r.Version, "removed": r.Removed,
		"state": copilotCycleStream(t, raw), "binding": r.Binding, "source_digest": r.Digest, "native": native, "data": data})
	c.f.protected[c.statePath(r)] = copilotProjectConfigHash(raw)
	c.check(t)
}

func (c *copilotProjectCycle) bindingProof(t *testing.T, r *copilotCycleRecipe, i domain.Installation) {
	t.Helper()
	if len(i.Clients) != 1 || i.DataRetained {
		t.Fatal("Copilot cycle single materialized binding required")
	}
	var b domain.ClientBinding
	for key, value := range i.Clients {
		if key != value.ClientBindingID {
			t.Fatal("Copilot cycle binding map identity differs")
		}
		b = value
	}
	physical := "native-proof-" + copilotProjectConfigHash([]byte(i.InstallationID))[:12]
	active := filepath.Join(filepath.Dir(c.statePath(r)), "managed", "clients", "copilot", physical)
	profile, activation := r.Project, domain.ActivationPrepared
	if r.Key == "user" {
		profile, activation = filepath.Join(c.f.base.Home, ".copilot"), domain.ActivationActive
	}
	id := "client_" + copilotProjectConfigHash([]byte(strings.Join([]string{i.InstallationID, "copilot", copilotCycleScope(r), active}, "\x00")))[:24]
	revision := &domain.ClientPackageRevision{Version: r.Version, TreeDigest: r.Digest, ManifestDigest: "sha256:" + copilotProjectConfigHash(r.Authored["plugin.json"])}
	if b.ClientID != "copilot" || b.Scope != copilotCycleScope(r) || b.TargetLocator != active || b.PhysicalArtifact != physical || b.NativeProfileRoot != profile ||
		b.ClientBindingID != id || b.Materialization != domain.MaterializationMaterialized || b.Activation != activation || b.Verification != domain.VerificationInstalled ||
		b.Authentication != domain.AuthenticationNotChecked || b.Policy != domain.PolicyAllowed || !reflect.DeepEqual(b.PackageRevision, revision) {
		t.Fatal("Copilot cycle binding authority/activation/revision differs")
	}
	if r.Binding.ClientBindingID != "" && (b.ClientBindingID != r.Binding.ClientBindingID || b.PhysicalArtifact != r.Binding.PhysicalArtifact || b.DataReceiptID != r.Binding.DataReceiptID) {
		t.Fatal("Copilot update/repair changed stable binding/data identity")
	}
	for key, other := range c.recipes {
		if key != r.Key && other.Installation.InstallationID != "" && (other.Installation.InstallationID == i.InstallationID || other.Binding.ClientBindingID == id || other.Binding.PhysicalArtifact == physical) {
			t.Fatal("Copilot same-name scopes share physical or logical authority")
		}
	}
	r.Binding, r.Active = b, active
	r.SkillPath = filepath.Join(active, "skills", r.Skill, "SKILL.md")
	if r.Key != "user" {
		r.SkillPath, r.Config = filepath.Join(r.Project, ".github", "skills", r.Skill, "SKILL.md"), filepath.Join(r.Project, ".github", "mcp.json")
	}
}

func (c *copilotProjectCycle) suspend(r *copilotCycleRecipe) {
	delete(c.f.protected, c.statePath(r))
	if r.SkillPath != "" {
		delete(c.f.protected, r.SkillPath)
	}
	if r.Config != "" {
		delete(c.f.protected, r.Config)
	}
	if r.Active != "" {
		delete(c.trees, r.Active)
	}
	if r.Key != "user" && r.SkillPath != "" {
		delete(c.trees, filepath.Dir(filepath.Dir(r.SkillPath)))
	}
}

func copilotCycleOwner(raw []byte, expected domain.DataReceipt) bool {
	if _, err := projectNativeRuntimeJSON(raw); err != nil {
		return false
	}
	var actual domain.DataReceipt
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	return d.Decode(&actual) == nil && reflect.DeepEqual(actual, expected)
}
