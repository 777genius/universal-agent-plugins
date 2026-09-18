package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

type Stager struct {
	LauncherSource  *managedstdio.Source
	SnapshotBuilder packagesnapshot.Builder
	PluginDataRoot  string
}

func (stager Stager) Discard(ctx context.Context, delivery domain.StagedDelivery) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	stagingBase := filepath.Clean(delivery.OwnedBase)
	if delivery.ClientID == domain.ClientClaude || delivery.ClientID == domain.ClientCursor {
		stagingBase = filepath.Dir(stagingBase)
	}
	if filepath.Dir(filepath.Clean(delivery.StagingPath)) != stagingBase ||
		!strings.HasPrefix(filepath.Base(delivery.StagingPath), ".agentplugins-staging-") {
		return fmt.Errorf("refuse unsafe staged delivery cleanup")
	}
	return removeStaging(stagingBase, delivery.StagingPath)
}

func (stager Stager) Verify(ctx context.Context, root, expectedDigest string) error {
	if strings.TrimSpace(expectedDigest) == "" {
		return fmt.Errorf("expected artifact digest is required")
	}
	if err := rejectExcludedOwnershipMarkers(root); err != nil {
		kind := ports.VerificationIndeterminate
		var marker *excludedOwnershipMarkerError
		if errors.As(err, &marker) {
			kind = ports.VerificationExcludedMarker
		} else if errors.Is(err, os.ErrNotExist) {
			kind = ports.VerificationAbsent
		}
		return &ports.VerificationError{Kind: kind, Err: err}
	}
	artifact, err := stager.SnapshotBuilder.Build(ctx, root)
	if err != nil {
		kind := ports.VerificationIndeterminate
		if errors.Is(err, os.ErrNotExist) {
			kind = ports.VerificationAbsent
		}
		return &ports.VerificationError{Kind: kind, Err: err}
	}
	digest := artifact.Digest
	if closeErr := artifact.Close(); closeErr != nil {
		return &ports.VerificationError{Kind: ports.VerificationIndeterminate, Err: closeErr}
	}
	if digest != expectedDigest {
		return &ports.VerificationError{Kind: ports.VerificationDigestMismatch, ActualDigest: digest, Err: fmt.Errorf("artifact digest mismatch")}
	}
	return nil
}

type excludedOwnershipMarkerError struct{ name string }

func (err *excludedOwnershipMarkerError) Error() string {
	return fmt.Sprintf("managed artifact contains excluded ownership marker %q", err.name)
}

func rejectExcludedOwnershipMarkers(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filepath.Clean(path) == filepath.Clean(root) {
			return nil
		}
		if entry.Name() == ".git" || entry.Name() == ".plugin-kit-ai.lock" {
			return &excludedOwnershipMarkerError{name: entry.Name()}
		}
		return nil
	})
}

func (stager Stager) Stage(
	ctx context.Context,
	envelope domain.PackageEnvelope,
	plan domain.DeliveryPlan,
	operationID string,
	hints domain.CompatibilityHints,
) (delivery domain.StagedDelivery, err error) {
	return stager.stage(ctx, envelope, plan, operationID, hints, stager.pluginDataPath(plan))
}

// StageWithPluginData binds projections to the exact owned locator recorded by
// lifecycle state. It is an optional extension to ports.PackageStager so older
// non-stdio test doubles remain source-compatible.
func (stager Stager) StageWithPluginData(
	ctx context.Context,
	envelope domain.PackageEnvelope,
	plan domain.DeliveryPlan,
	operationID string,
	hints domain.CompatibilityHints,
	pluginDataPath string,
) (delivery domain.StagedDelivery, err error) {
	if strings.TrimSpace(pluginDataPath) == "" || !filepath.IsAbs(pluginDataPath) {
		return domain.StagedDelivery{}, fmt.Errorf("owned PLUGIN_DATA path must be absolute")
	}
	return stager.stage(ctx, envelope, plan, operationID, hints, filepath.Clean(pluginDataPath))
}

func (stager Stager) stage(
	ctx context.Context,
	envelope domain.PackageEnvelope,
	plan domain.DeliveryPlan,
	operationID string,
	hints domain.CompatibilityHints,
	pluginDataPath string,
) (delivery domain.StagedDelivery, err error) {
	if err := ctx.Err(); err != nil {
		return domain.StagedDelivery{}, err
	}
	if plan.Status == domain.PlanUnsupported {
		return domain.StagedDelivery{}, fmt.Errorf("cannot stage an unsupported delivery plan")
	}
	if strings.TrimSpace(envelope.SnapshotRoot) == "" {
		return domain.StagedDelivery{}, fmt.Errorf("package snapshot root is required")
	}
	if err := pathpolicy.ValidateLeafID(operationID); err != nil {
		return domain.StagedDelivery{}, fmt.Errorf("invalid staging operation id: %w", err)
	}
	if err := validatePlanPaths(plan); err != nil {
		return domain.StagedDelivery{}, err
	}
	if err := validateReservedStdioEnvironment(envelope, plan); err != nil {
		return domain.StagedDelivery{}, err
	}
	if err := os.MkdirAll(plan.TargetRoot, 0o700); err != nil {
		return domain.StagedDelivery{}, fmt.Errorf("create client target root: %w", err)
	}
	if err := validatePlanPaths(plan); err != nil {
		return domain.StagedDelivery{}, err
	}
	suffix := sha256.Sum256([]byte(operationID))
	stagingBase := shared.DefaultStagingLayout{}.StagingBase(plan)
	// Claude Code and Cursor both discover plugin-shaped directories directly
	// below a watched in-place slot (`skills/` and `plugins/local`). Keep the
	// transaction staging directory beside that watched root so a reload
	// cannot treat staging as an installed plugin. Claude's TargetAnchor is
	// already the parent of `skills/`; Cursor's TargetAnchor is the config
	// root, so the parent of `plugins/local` is the matching sibling.
	// dirswap still performs the final atomic rename on the same filesystem.
	if plan.ClientID == domain.ClientClaude {
		stagingBase = plan.TargetAnchor
	}
	if plan.ClientID == domain.ClientCursor {
		stagingBase = filepath.Dir(plan.TargetRoot)
	}
	if err := os.MkdirAll(stagingBase, 0o700); err != nil {
		return domain.StagedDelivery{}, fmt.Errorf("create client staging root: %w", err)
	}
	stagingPath := filepath.Join(stagingBase, ".agentplugins-staging-"+hex.EncodeToString(suffix[:8]))
	if err := pathpolicy.RequireContainedChild(stagingBase, stagingPath); err != nil {
		return domain.StagedDelivery{}, fmt.Errorf("unsafe staging path: %w", err)
	}
	if _, statErr := os.Lstat(stagingPath); statErr == nil {
		return domain.StagedDelivery{}, fmt.Errorf("staging path already exists")
	} else if !os.IsNotExist(statErr) {
		return domain.StagedDelivery{}, fmt.Errorf("inspect staging path: %w", statErr)
	}
	defer func() {
		if err != nil {
			_ = removeStaging(stagingBase, stagingPath)
		}
	}()
	if err := filetree.CopyDir(envelope.SnapshotRoot, stagingPath); err != nil {
		return domain.StagedDelivery{}, fmt.Errorf("copy package snapshot to staging: %w", err)
	}
	if err := sanitizePackage(stagingPath, envelope, plan); err != nil {
		return domain.StagedDelivery{}, err
	}
	if plan.PackageMode == domain.PackageProjection {
		switch plan.ClientID {
		case domain.ClientCodex:
			if err := projectOpenAI(stagingPath, envelope, plan, hints, pluginDataPath); err != nil {
				return domain.StagedDelivery{}, err
			}
		case domain.ClientClaude:
			if err := stager.deliverManagedStdio(stagingPath, envelope, plan); err != nil {
				return domain.StagedDelivery{}, err
			}
			if err := projectClaude(stagingPath, envelope, plan, pluginDataPath); err != nil {
				return domain.StagedDelivery{}, err
			}
		case domain.ClientChatGPT:
			if err := projectChatGPT(stagingPath, envelope, plan, hints, pluginDataPath); err != nil {
				return domain.StagedDelivery{}, err
			}
		}
		if plan.ClientID == domain.ClientCodex || plan.ClientID == domain.ClientChatGPT {
			if err := projectCodexMarketplace(stagingPath, envelope, plan); err != nil {
				return domain.StagedDelivery{}, err
			}
		}
	}
	if plan.ClientID == domain.ClientKiro {
		if err := projectKiroMCP(stagingPath, envelope, plan, pluginDataPath); err != nil {
			return domain.StagedDelivery{}, err
		}
	}
	var geminiObjects []domain.NativeObjectOwnership
	if plan.ClientID == domain.ClientGemini {
		var err error
		geminiObjects, err = buildGeminiNativeObjects(stagingPath, envelope, plan, pluginDataPath)
		if err != nil {
			return domain.StagedDelivery{}, err
		}
	}
	if plan.ClientID == domain.ClientCursor {
		if err := projectCursor(stagingPath, envelope, plan, pluginDataPath); err != nil {
			return domain.StagedDelivery{}, err
		}
	}
	if plan.ClientID == domain.ClientOpenCode {
		if err := projectOpenCodeNative(stagingPath, envelope, plan, pluginDataPath); err != nil {
			return domain.StagedDelivery{}, err
		}
	}
	if plan.ClientID == domain.ClientCline {
		if err := projectClineNative(stagingPath, envelope, plan, pluginDataPath); err != nil {
			return domain.StagedDelivery{}, err
		}
	}
	if plan.ClientID == domain.ClientCopilot || plan.ClientID == domain.ClientVSCode {
		if err := projectCopilotMarketplace(stagingPath, envelope, plan); err != nil {
			return domain.StagedDelivery{}, err
		}
	}
	if plan.ClientID == domain.ClientWindsurf {
		if err := stager.deliverManagedStdio(stagingPath, envelope, plan); err != nil {
			return domain.StagedDelivery{}, err
		}
		if err := projectWindsurfMCP(stagingPath, envelope, plan, pluginDataPath); err != nil {
			return domain.StagedDelivery{}, err
		}
	}
	artifact, err := stager.SnapshotBuilder.Build(ctx, stagingPath)
	if err != nil {
		return domain.StagedDelivery{}, fmt.Errorf("verify staged artifact: %w", err)
	}
	artifactDigest := artifact.Digest
	if closeErr := artifact.Close(); closeErr != nil {
		return domain.StagedDelivery{}, fmt.Errorf("clean staged verification snapshot: %w", closeErr)
	}
	objects := []domain.NativeObjectOwnership{
		{
			ObjectID:        "package:" + string(plan.ClientID) + ":" + plan.PhysicalArtifactID,
			Kind:            "managed_package_directory",
			LogicalName:     envelope.Manifest.Name,
			Path:            plan.ActivePath,
			ManagedDigest:   artifactDigest,
			ProtectionClass: "managed",
		},
	}
	if plan.ClientID == domain.ClientKiro {
		kiroObjects, err := buildKiroNativeObjects(stagingPath, envelope, plan)
		if err != nil {
			return domain.StagedDelivery{}, err
		}
		objects = append(objects, kiroObjects...)
	}
	if plan.ClientID == domain.ClientOpenCode {
		openCodeObjects, err := buildOpenCodeNativeObjects(stagingPath, envelope, plan)
		if err != nil {
			return domain.StagedDelivery{}, err
		}
		objects = append(objects, openCodeObjects...)
	}
	if plan.ClientID == domain.ClientCline {
		clineObjects, err := buildClineNativeObjects(stagingPath, envelope, plan)
		if err != nil {
			return domain.StagedDelivery{}, err
		}
		objects = append(objects, clineObjects...)
	}
	objects = append(objects, geminiObjects...)
	if plan.ClientID == domain.ClientWindsurf {
		windsurfObjects, err := buildWindsurfNativeObjects(stagingPath, plan)
		if err != nil {
			return domain.StagedDelivery{}, err
		}
		objects = append(objects, windsurfObjects...)
	}
	return domain.StagedDelivery{
		ClientID:       plan.ClientID,
		OwnedBase:      plan.TargetRoot,
		ActivePath:     plan.ActivePath,
		StagingPath:    stagingPath,
		ArtifactDigest: artifactDigest,
		NativeObjects:  objects,
	}, nil
}

func validateReservedStdioEnvironment(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	for _, name := range domain.SelectedMCPNames(plan) {
		server := envelope.MCP.Servers[name]
		if server.Type != "stdio" {
			continue
		}
		switch env := server.Decoded["env"].(type) {
		case map[string]any:
			if _, ok := env["PLUGIN_ROOT"]; ok {
				return fmt.Errorf("stdio MCP server %s defines reserved PLUGIN_ROOT", name)
			}
			if _, ok := env["PLUGIN_DATA"]; ok {
				return fmt.Errorf("stdio MCP server %s defines reserved PLUGIN_DATA", name)
			}
		case map[string]string:
			if _, ok := env["PLUGIN_ROOT"]; ok {
				return fmt.Errorf("stdio MCP server %s defines reserved PLUGIN_ROOT", name)
			}
			if _, ok := env["PLUGIN_DATA"]; ok {
				return fmt.Errorf("stdio MCP server %s defines reserved PLUGIN_DATA", name)
			}
		}
	}
	return nil
}

func (stager Stager) pluginDataPath(plan domain.DeliveryPlan) string {
	base := stager.PluginDataRoot
	if strings.TrimSpace(base) == "" {
		base = filepath.Join(plan.TargetAnchor, "plugin-data")
	}
	return filepath.Join(base, plan.PhysicalArtifactID)
}

func validatePlanPaths(plan domain.DeliveryPlan) error {
	if strings.TrimSpace(plan.TargetAnchor) == "" || strings.TrimSpace(plan.TargetRoot) == "" || strings.TrimSpace(plan.ActivePath) == "" {
		return fmt.Errorf("delivery plan target paths are incomplete")
	}
	if err := pathpolicy.RequireContainedChild(plan.TargetAnchor, plan.TargetRoot); err != nil {
		return fmt.Errorf("unsafe delivery target root: %w", err)
	}
	if plan.ClientID == domain.ClientClaude && filepath.Clean(plan.TargetRoot) != filepath.Join(filepath.Clean(plan.TargetAnchor), "skills") {
		return fmt.Errorf("Claude delivery target root must be the exact configured skills directory")
	}
	if filepath.Dir(filepath.Clean(plan.ActivePath)) != filepath.Clean(plan.TargetRoot) {
		return fmt.Errorf("delivery active path must be a direct child of target root")
	}
	if err := pathpolicy.RequireContainedChild(plan.TargetRoot, plan.ActivePath); err != nil {
		return fmt.Errorf("unsafe delivery active path: %w", err)
	}
	return nil
}

func sanitizePackage(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	if err := removeUnsupportedPortableHooks(root); err != nil {
		return err
	}
	if err := removeInvalidAndUnsupportedSkills(root, envelope, plan); err != nil {
		return err
	}
	if err := writeSanitizedMCP(root, envelope, plan); err != nil {
		return err
	}
	if err := writeSanitizedApp(root, envelope, plan); err != nil {
		return err
	}
	return writeSanitizedExtensions(root, envelope, plan)
}

func removeUnsupportedPortableHooks(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("inspect staged package root: %w", err)
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), "hooks") {
			continue
		}
		candidate := filepath.Join(root, entry.Name())
		if err := pathpolicy.RequireContainedChild(root, candidate); err != nil {
			return fmt.Errorf("unsafe staged hooks path: %w", err)
		}
		if err := os.RemoveAll(candidate); err != nil {
			return fmt.Errorf("remove unsupported staged hooks: %w", err)
		}
	}
	return nil
}

func writeSanitizedApp(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	path := filepath.Join(root, ".app.json")
	if plan.ClientID != domain.ClientChatGPT || !envelope.App.Enabled || len(envelope.App.Raw) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove unsupported .app.json: %w", err)
		}
		return nil
	}
	mode := os.FileMode(0o644)
	if envelope.LocalChatGPTMapping != nil {
		mode = 0o600
	}
	return atomicfile.Write(path, append([]byte(nil), envelope.App.Raw...), mode)
}

func removeInvalidAndUnsupportedSkills(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	names := append([]string(nil), envelope.Inventory.InvalidSkills...)
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentSkill && component.Support == domain.SupportUnsupported {
			names = append(names, component.Name)
		}
	}
	skillsRoot := filepath.Join(root, "skills")
	if envelope.Inventory.InvalidSkillsRoot {
		if err := pathpolicy.RequireExactPath(filepath.Join(root, "skills"), skillsRoot); err != nil {
			return err
		}
		if err := os.RemoveAll(skillsRoot); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove invalid skills root: %w", err)
		}
		return nil
	}
	for _, name := range names {
		candidate := filepath.Join(skillsRoot, name)
		if filepath.Dir(filepath.Clean(candidate)) != filepath.Clean(skillsRoot) {
			return fmt.Errorf("unsafe invalid skill path %q", name)
		}
		if err := pathpolicy.RequireContainedChild(skillsRoot, candidate); err != nil {
			return fmt.Errorf("unsafe invalid skill path %q: %w", name, err)
		}
		if err := os.RemoveAll(candidate); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove skipped skill %q: %w", name, err)
		}
	}
	return nil
}

func writeSanitizedMCP(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	path := filepath.Join(root, "mcp.json")
	if !envelope.MCP.Present || !envelope.MCP.Enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove disabled mcp.json: %w", err)
		}
		return nil
	}
	supported := shared.SupportedMCPNames(plan)
	servers := make(map[string]json.RawMessage, len(supported))
	for _, name := range supported {
		server, ok := envelope.MCP.Servers[name]
		if ok {
			servers[name] = append(json.RawMessage(nil), server.Raw...)
		}
	}
	if len(servers) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove unsupported mcp.json: %w", err)
		}
		return nil
	}
	document := struct {
		Schema     string                     `json:"$schema"`
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}{Schema: domain.MCPSchemaV1, MCPServers: servers}
	return shared.WriteJSON(path, document)
}

func writeSanitizedExtensions(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	var unsupported []string
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentExtension && component.Support == domain.SupportUnsupported {
			unsupported = append(unsupported, component.Name)
		}
	}
	ignoredInvalid := false
	for _, diagnostic := range envelope.Diagnostics {
		if diagnostic.Code == "plugin_extensions_ignored" {
			ignoredInvalid = true
			break
		}
	}
	if len(unsupported) == 0 && !ignoredInvalid {
		return nil
	}
	path := filepath.Join(root, "plugin.json")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read staged plugin.json: %w", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(body, &document); err != nil {
		return fmt.Errorf("decode staged plugin.json: %w", err)
	}
	if ignoredInvalid {
		delete(document, "extensions")
		return shared.WriteJSON(path, document)
	}
	var extensions map[string]json.RawMessage
	if raw := document["extensions"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &extensions); err != nil {
			return fmt.Errorf("decode staged plugin extensions: %w", err)
		}
	}
	for _, name := range unsupported {
		delete(extensions, name)
	}
	if len(extensions) == 0 {
		delete(document, "extensions")
	} else {
		raw, err := json.Marshal(extensions)
		if err != nil {
			return err
		}
		document["extensions"] = raw
	}
	return shared.WriteJSON(path, document)
}

func projectOpenAI(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, hints domain.CompatibilityHints, dataPath string) error {
	manifest, err := projectedOpenAIManifest(envelope)
	if err != nil {
		return err
	}
	// A preserved upstream manifest may already declare these members against a
	// layout this projection does not produce, so they are dropped and then
	// re-declared from what the plan actually selected.
	delete(manifest, "apps")
	delete(manifest, "skills")
	delete(manifest, "mcpServers")
	shared.ApplyManifestMetadata(manifest, envelope, shared.WithAuthorObject())
	if shared.ComponentKindPresent(plan.Components, domain.ComponentSkill) {
		manifest["skills"] = "./skills/"
	}
	serverNames := shared.SupportedMCPNames(plan)
	if len(serverNames) > 0 {
		manifest["mcpServers"] = "./.mcp.json"
	}
	manifestPath := filepath.Join(root, ".codex-plugin", "plugin.json")
	if err := shared.WriteJSON(manifestPath, manifest); err != nil {
		return fmt.Errorf("write OpenAI compatibility manifest: %w", err)
	}
	return projectOpenAIMCP(root, envelope, serverNames, hints, plan.ActivePath, dataPath)
}

func projectOpenAIMCP(root string, envelope domain.PackageEnvelope, serverNames []string, hints domain.CompatibilityHints, pluginRoot, dataPath string) error {
	return shared.ProjectMCPServers(shared.MCPProjection{
		Root:       root,
		Envelope:   envelope,
		Names:      serverNames,
		Dialect:    shared.MCPDialectOpenAI,
		PluginRoot: pluginRoot,
		DataPath:   dataPath,
		Hints:      hints,
	})
}

func projectKiroMCP(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	return shared.ProjectMCPServers(shared.MCPProjection{
		Root:       root,
		Envelope:   envelope,
		Names:      shared.SupportedMCPNames(plan),
		Dialect:    shared.MCPDialectKiro,
		PluginRoot: plan.ActivePath,
		DataPath:   dataPath,
	})
}

func projectCursor(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	manifest := shared.ManifestFromEnvelope(envelope, shared.WithAuthorNameEmail())
	if shared.ComponentKindPresent(plan.Components, domain.ComponentSkill) {
		manifest["skills"] = "./skills/"
	}
	serverNames := shared.SupportedMCPNames(plan)
	if len(serverNames) > 0 {
		manifest["mcpServers"] = "./mcp.json"
	}
	if err := shared.WriteJSON(filepath.Join(root, ".cursor-plugin", "plugin.json"), manifest); err != nil {
		return fmt.Errorf("write Cursor plugin manifest: %w", err)
	}
	return projectCursorMCP(root, envelope, serverNames, plan.ActivePath, dataPath)
}

func projectCursorMCP(root string, envelope domain.PackageEnvelope, serverNames []string, pluginRoot, dataPath string) error {
	return shared.ProjectMCPServers(shared.MCPProjection{
		Root:       root,
		Envelope:   envelope,
		Names:      serverNames,
		Dialect:    shared.MCPDialectCursor,
		PluginRoot: pluginRoot,
		DataPath:   dataPath,
	})
}

func projectChatGPT(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, hints domain.CompatibilityHints, dataPath string) error {
	manifest, err := projectedOpenAIManifest(envelope)
	if err != nil {
		return err
	}
	serverNames := shared.SupportedMCPNames(plan)
	if len(serverNames) > 0 {
		manifest["mcpServers"] = "./.mcp.json"
	} else {
		delete(manifest, "mcpServers")
	}
	if shared.ComponentKindPresent(plan.Components, domain.ComponentSkill) {
		manifest["skills"] = "./skills/"
	} else {
		delete(manifest, "skills")
	}
	if envelope.App.Enabled && shared.ComponentKindPresent(plan.Components, domain.ComponentApp) {
		manifest["apps"] = "./.app.json"
	} else {
		delete(manifest, "apps")
	}
	if err := shared.WriteJSON(filepath.Join(root, ".codex-plugin", "plugin.json"), manifest); err != nil {
		return fmt.Errorf("write ChatGPT plugin manifest: %w", err)
	}
	if err := projectOpenAIMCP(root, envelope, serverNames, hints, plan.ActivePath, dataPath); err != nil {
		return err
	}
	for _, portableManifest := range []string{"plugin.json", "mcp.json"} {
		if err := os.Remove(filepath.Join(root, portableManifest)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove portable %s from official ChatGPT projection: %w", portableManifest, err)
		}
	}
	return nil
}

func projectedOpenAIManifest(envelope domain.PackageEnvelope) (map[string]any, error) {
	preserved, ok, err := shared.PreservedOpenAIManifest(envelope)
	if err != nil {
		return nil, err
	}
	if ok {
		return preserved, nil
	}
	return shared.ManifestFromEnvelope(envelope, shared.WithAuthorObject()), nil
}

func pathContainedBy(root, candidate string) bool {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	return err == nil && !filepath.IsAbs(relative) && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func removeStaging(base, path string) error {
	if err := pathpolicy.RequireContainedChild(base, path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}
