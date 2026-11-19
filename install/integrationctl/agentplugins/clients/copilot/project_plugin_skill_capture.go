package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"slices"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type copilotWholeSkillProof struct {
	objects                                    []domain.NativeObjectOwnership
	skills                                     map[string]copilotObservedSkill
	trust, settings                            nativeconfig.FileSnapshot
	authentication                             domain.AuthenticationState
	name, marketplace, active, version, digest string
	versionPresent                             bool
}

// Whole-plugin ownership is a package receipt plus a settings pair, never a
// loose Skill receipt. Read metadata only from a package seal matching that
// receipt, retaining the original native paths in the expected catalog.
func captureCopilotWholeSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest) (proof *copilotWholeSkillProof) {
	if copilotVerifyWholePlugin(env, request) != nil || !copilotWholeSkillPackageReceipt(request) {
		return nil
	}
	sealed, closeSnapshot, err := env.Snapshots.CaptureDirectory(ctx, request.Plan.ActivePath)
	if err != nil || closeSnapshot == nil {
		return nil
	}
	defer func() {
		if closeSnapshot() != nil {
			proof = nil
		}
	}()
	digest, err := env.Snapshots.Digest(ctx, sealed)
	if err != nil || digest != request.Delivery.ArtifactDigest {
		return nil
	}
	p := &copilotWholeSkillProof{objects: slices.Clone(request.Delivery.NativeObjects), skills: map[string]copilotObservedSkill{},
		authentication: shared.ObservedSkillInstallationAuthentication(request.Plan), name: request.Plan.DeclaredName,
		marketplace: shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID), active: request.Plan.ActivePath, digest: digest}
	if !captureCopilotWholeManifest(env, sealed, p) {
		return nil
	}
	for _, component := range request.Plan.Components {
		skill, ok := captureCopilotWholeSkill(ctx, env, component.Name, sealed, p.active)
		if !ok {
			return nil
		}
		p.skills[component.Name] = skill
	}
	if !captureCopilotWholeProfile(env, request, p) {
		return nil
	}
	current, err := env.Snapshots.Digest(ctx, p.active)
	if err != nil || current != p.digest || ctx.Err() != nil {
		return nil
	}
	return p
}

func copilotWholeSkillPackageReceipt(request domain.ActivationRequest) bool {
	if len(request.Delivery.NativeObjects) != 2 || request.Delivery.ArtifactDigest == "" {
		return false
	}
	expected := domain.NativeObjectOwnership{ObjectID: "package:copilot:" + request.Plan.PhysicalArtifactID,
		Kind: "managed_package_directory", LogicalName: request.Plan.DeclaredName, Path: request.Plan.ActivePath,
		ManagedDigest: request.Delivery.ArtifactDigest, ProtectionClass: "managed"}
	found := 0
	for _, object := range request.Delivery.NativeObjects {
		if object.Kind == "managed_package_directory" {
			if object != expected {
				return false
			}
			found++
		}
	}
	return found == 1
}

func captureCopilotWholeSkill(ctx context.Context, env clients.Env, name, sealed, active string) (copilotObservedSkill, bool) {
	if pathpolicy.ValidateLeafID(name) != nil {
		return copilotObservedSkill{}, false
	}
	path := filepath.Join(sealed, "skills", name, "SKILL.md")
	file, err := env.NativeConfig.ReadExactFileBounded(path, 1<<20)
	if err != nil || !file.Exists {
		return copilotObservedSkill{}, false
	}
	skill, ok := copilotProjectSkillMetadata(ctx, env, name, filepath.Join(active, "skills", name), file.Body)
	skill.source = "plugin"
	return skill, ok
}

func captureCopilotWholeProfile(env clients.Env, request domain.ActivationRequest, proof *copilotWholeSkillProof) bool {
	trust, err := copilotTrustPath(request.Client, request.Plan)
	if err != nil {
		return false
	}
	proof.trust, err = env.NativeConfig.ReadExactFileBounded(trust, 1<<20)
	if err != nil {
		return false
	}
	settings, err := copilotProjectPluginSettingsPath(request.Plan.NativeRegistryRoot)
	if err != nil {
		return false
	}
	proof.settings, err = env.NativeConfig.ReadExactFileBounded(settings, 1<<20)
	return err == nil
}

func copilotCatalogArray(body []byte) ([]any, bool) {
	if len(body) > 1<<20 || !utf8.Valid(body) {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return nil, false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, false
	}
	rows, ok := value.([]any)
	return rows, ok
}

func copilotWholePluginListed(body []byte, proof *copilotWholeSkillProof) bool {
	rows, ok := copilotCatalogArray(body)
	if !ok || len(rows) > 64 {
		return false
	}
	seen := map[string]bool{}
	found := 0
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || !copilotWholePluginRow(row) {
			return false
		}
		name, market := row["name"].(string), row["marketplace"].(string)
		key := shared.FoldJSONKey(name + "@" + market)
		if seen[key] {
			return false
		}
		seen[key] = true
		if name == proof.name && market == proof.marketplace {
			if row["enabled"] != true || row["source"] != "live" || row["installedFrom"] != proof.active ||
				(proof.versionPresent && row["version"] != proof.version) {
				return false
			}
			found++
		}
	}
	return found == 1
}

func copilotWholePluginRow(row map[string]any) bool {
	if len(row) != 5 && len(row) != 6 {
		return false
	}
	if len(row) == 6 {
		if _, ok := row["installedFrom"]; !ok {
			return false
		}
	}
	for _, key := range []string{"name", "marketplace", "version", "source"} {
		if _, ok := row[key].(string); !ok {
			return false
		}
	}
	if _, ok := row["enabled"].(bool); !ok || row["name"] == "" {
		return false
	}
	if path, present := row["installedFrom"]; present {
		text, ok := path.(string)
		if !ok || !filepath.IsAbs(text) || filepath.Clean(text) != text {
			return false
		}
	}
	return row["source"] == "live" || row["source"] == "builtin"
}

func captureCopilotWholeManifest(env clients.Env, sealed string, p *copilotWholeSkillProof) bool {
	manifest, err := env.NativeConfig.ReadExactFileBounded(filepath.Join(sealed, "plugin.json"), 1<<20)
	if err != nil || !manifest.Exists {
		return false
	}
	doc, err := shared.DecodeStrictJSONObject(manifest.Body)
	if err != nil || doc["name"] != p.name || doc["$schema"] != domain.PluginSchemaV1 {
		return false
	}
	if version, present := doc["version"]; present {
		p.versionPresent = true
		var ok bool
		if p.version, ok = version.(string); !ok {
			return false
		}
	}
	return true
}
