package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"github.com/pelletier/go-toml/v2"
)

type packageVerifier interface {
	Verify(context.Context, string, string) error
}

// NativeIdentityObserver combines package ownership with the client's native
// registry. A manager-owned path is not proof that the logical identity is
// free: another prepared marketplace, local plugin, installed CLI entry, or
// native config entry can already claim the same manifest identity.
// Native registry discovery executes trusted, short-lived list commands. A
// tree-aware runner is used when available so Linux requires atomic cgroup
// containment and Windows uses a Job Object. Plain runners remain supported as
// injected test/provider implementations, but OS execution never claims cleanup
// based on descendant sampling.
type NativeIdentityObserver struct {
	Stager           packageVerifier
	Runner           CommandRunner
	DiscoveryTimeout time.Duration
}

type treeCommandRunner interface {
	RunWithTreeExitGrace(context.Context, legacyports.Command, time.Duration) (legacyports.CommandResult, error)
}

const defaultNativeDiscoveryTimeout = 15 * time.Second

type registryFinding uint8

const (
	registryClear registryFinding = iota
	registryExpected
	registryCollision
	registryIndeterminate
)

func (observer NativeIdentityObserver) ObserveNativeIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	return observer.observeIdentity(ctx, client, plan, managed, true)
}

// ObservePreparedIdentity inspects only filesystem-backed package identities.
// It deliberately excludes native CLI discovery so dry-run cannot launch the
// detected client executable.
func (observer NativeIdentityObserver) ObservePreparedIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	return observer.observeIdentity(ctx, client, plan, managed, false)
}

func (observer NativeIdentityObserver) observeIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding, includeNativeRegistry bool) (domain.NativeIdentityObservation, error) {
	if err := ctx.Err(); err != nil {
		return domain.NativeIdentityObservation{}, err
	}
	name := strings.TrimSpace(plan.DeclaredName)
	if name == "" {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, nil
	}

	prepared, preparedErr := inspectPreparedRegistry(plan, name, managed != nil)
	if client.ClientID == domain.ClientClaude {
		prepared, preparedErr = inspectClaudeSkillsRegistry(plan, name, managed != nil)
	}
	if preparedErr != nil {
		prepared = registryIndeterminate
	}
	nativeAttempted := includeNativeRegistry && observer.Runner != nil && strings.TrimSpace(plan.NativeRegistryExecutable) != "" &&
		(client.ClientID == domain.ClientCodex || client.ClientID == domain.ClientClaude || client.ClientID == domain.ClientCopilot || client.ClientID == domain.ClientVSCode)
	native := registryClear
	if includeNativeRegistry {
		nativeCtx := ctx
		cancelNative := func() {}
		if nativeAttempted {
			timeout := observer.DiscoveryTimeout
			if timeout <= 0 {
				timeout = defaultNativeDiscoveryTimeout
			}
			nativeCtx, cancelNative = context.WithTimeout(ctx, timeout)
		}
		var err error
		native, err = observer.inspectNativeRegistry(nativeCtx, client, plan, managed)
		cancelNative()
		if err != nil {
			return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate, NativeDiscoveryState: domain.NativeIdentityIndeterminate,
				NativeDiscoveryAttempted: nativeAttempted}, err
		}
	}
	discoveryState := domain.NativeIdentityAbsent
	discoveryReconciled := false
	switch native {
	case registryExpected:
		discoveryState = domain.NativeIdentityManaged
		discoveryReconciled = managed != nil
	case registryCollision:
		discoveryState = domain.NativeIdentityUnmanaged
	case registryIndeterminate:
		discoveryState = domain.NativeIdentityIndeterminate
	}
	observed := func(state domain.NativeIdentityState) domain.NativeIdentityObservation {
		return domain.NativeIdentityObservation{State: state, NativeDiscoveryState: discoveryState,
			NativeDiscoveryReconciled: discoveryReconciled, NativeDiscoveryAttempted: nativeAttempted}
	}
	if preparedErr != nil {
		return observed(domain.NativeIdentityIndeterminate), preparedErr
	}
	if prepared == registryCollision || native == registryCollision {
		return observed(domain.NativeIdentityUnmanaged), nil
	}
	if prepared == registryIndeterminate || native == registryIndeterminate {
		return observed(domain.NativeIdentityIndeterminate), nil
	}
	if managed == nil && (prepared == registryExpected || native == registryExpected) {
		return observed(domain.NativeIdentityUnmanaged), nil
	}

	_, statErr := os.Lstat(plan.ActivePath)
	if os.IsNotExist(statErr) {
		return observed(domain.NativeIdentityAbsent), nil
	}
	if statErr != nil {
		return observed(domain.NativeIdentityIndeterminate), statErr
	}
	if managed == nil {
		return observed(domain.NativeIdentityUnmanaged), nil
	}
	expected := managedPackageDigest(*managed)
	if expected == "" || observer.Stager == nil {
		return observed(domain.NativeIdentityIndeterminate), nil
	}
	if err := observer.Stager.Verify(ctx, plan.ActivePath, expected); err != nil {
		var verification *ports.VerificationError
		if errors.As(err, &verification) && verification.Kind == ports.VerificationDigestMismatch {
			result := observed(domain.NativeIdentityIndeterminate)
			result.Digest = verification.ActualDigest
			return result, nil
		}
		return observed(domain.NativeIdentityIndeterminate), err
	}
	result := observed(domain.NativeIdentityManaged)
	result.Digest = expected
	result.ReceiptReconciled = true
	return result, nil
}

func (observer NativeIdentityObserver) inspectNativeRegistry(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	switch client.ClientID {
	case domain.ClientCodex:
		if strings.TrimSpace(plan.NativeRegistryExecutable) != "" {
			return observer.inspectCodexCLI(ctx, plan, managed)
		}
		return inspectCodexFiles(plan, managed)
	case domain.ClientClaude:
		if strings.TrimSpace(plan.NativeRegistryExecutable) == "" || observer.Runner == nil {
			return registryIndeterminate, nil
		}
		configRoot := strings.TrimSpace(plan.TargetAnchor)
		if configRoot == "" {
			configRoot = strings.TrimSpace(client.ConfigRoot)
		}
		if configRoot == "" && strings.TrimSpace(plan.TargetRoot) != "" {
			configRoot = filepath.Dir(filepath.Clean(plan.TargetRoot))
		}
		command, err := claudeListCommand(plan.NativeRegistryExecutable, configRoot, plan.ActivePath)
		if err != nil {
			return registryIndeterminate, err
		}
		result, err := runClaudeListCommand(ctx, observer.Runner, command)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return registryIndeterminate, ctxErr
			}
			return registryIndeterminate, err
		}
		if result.ExitCode != 0 {
			return registryIndeterminate, fmt.Errorf("Claude Code plugin registry command failed with exit code %d", result.ExitCode)
		}
		switch claudePluginStatus(result.Stdout, plan.DeclaredName, plan.ActivePath) {
		case claudeStatusInstalled:
			if managed == nil {
				return registryCollision, nil
			}
			return registryExpected, nil
		case claudeStatusAbsent:
			return registryClear, nil
		case claudeStatusCollision:
			return registryCollision, nil
		default:
			return registryIndeterminate, nil
		}
	case domain.ClientChatGPT:
		// ChatGPT's installed-plugin registry is remote and this adapter has no
		// authenticated read-only API. A signed explicit app binding authorizes
		// only preparation of a new local package, while an existing ownership
		// receipt authorizes replacement of that owned local package. Neither is
		// treated as proof of remote activation or registry availability.
		if plan.LocalPreparationAuthorized || managed != nil {
			return registryClear, nil
		}
		return registryIndeterminate, nil
	case domain.ClientCursor:
		// TargetRoot is Cursor's authoritative plugins/local registry and was
		// inspected as the prepared/native boundary above.
		return registryClear, nil
	case domain.ClientCopilot, domain.ClientVSCode:
		if strings.TrimSpace(plan.NativeRegistryExecutable) != "" {
			return observer.inspectCopilotCLI(ctx, plan, managed)
		}
		root := strings.TrimSpace(plan.NativeRegistryRoot)
		if root == "" {
			return registryIndeterminate, nil
		}
		if _, err := os.Lstat(root); os.IsNotExist(err) {
			return registryClear, nil
		} else if err != nil {
			return registryIndeterminate, err
		}
		// Copilot and VS Code share a backend, but its on-disk registry contract
		// is not implemented by these adapters. An existing config root without
		// the CLI therefore cannot prove absence.
		return registryIndeterminate, nil
	case domain.ClientKiro:
		return inspectKiroRegistry(plan, managed)
	case domain.ClientOpenCode:
		// OpenCode MCP entries are keyed by individual server names rather than
		// the package identity. Exact entry collision and ownership checks happen
		// transactionally in the native config provider after staging.
		return registryClear, nil
	case domain.ClientCline:
		// Cline MCP identities are per-server and protected by exact receipts;
		// its skill paths are checked before the all-or-none native config batch.
		return registryClear, nil
	case domain.ClientGemini:
		return inspectGeminiRegistry(plan, managed)
	case domain.ClientWindsurf:
		return inspectWindsurfRegistry(plan, managed)
	default:
		return registryIndeterminate, nil
	}
}

func (observer NativeIdentityObserver) inspectCodexCLI(ctx context.Context, plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	if observer.Runner == nil {
		return registryIndeterminate, nil
	}
	result, err := observer.runNativeRegistry(ctx, legacyports.Command{Argv: []string{plan.NativeRegistryExecutable, "plugin", "list", "--json"}})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return registryIndeterminate, ctxErr
		}
		return registryIndeterminate, err
	}
	if result.ExitCode != 0 {
		if diagnostic := boundedNativeDiagnostic(result.Stdout, result.Stderr); diagnostic != "" {
			return registryIndeterminate, fmt.Errorf("Codex plugin registry command failed with exit code %d: %s", result.ExitCode, diagnostic)
		}
		return registryIndeterminate, fmt.Errorf("Codex plugin registry command failed with exit code %d", result.ExitCode)
	}
	return codexRegistryFinding(result.Stdout, plan.DeclaredName, managedMarketplaceName(plan.PhysicalArtifactID), managed != nil), nil
}

// nativeDiagnosticLimit bounds the excerpt kept from a failed native CLI
// invocation's own output. It exists purely for operator diagnosis of a
// discovery failure and is never treated as proof of package absence.
const nativeDiagnosticLimit = 2048

// boundedNativeDiagnostic returns a short, sanitized excerpt of a failed
// native CLI invocation's own stdout/stderr. It never includes argv,
// environment, or configuration; only the bounded child-process output, with
// control characters stripped and length capped.
func boundedNativeDiagnostic(stdout, stderr []byte) string {
	parts := make([]string, 0, 2)
	if text := sanitizeNativeDiagnosticText(stderr); text != "" {
		parts = append(parts, text)
	}
	if text := sanitizeNativeDiagnosticText(stdout); text != "" {
		parts = append(parts, text)
	}
	text := strings.Join(parts, " | ")
	if len(text) > nativeDiagnosticLimit {
		// Re-validate after the byte-index cut: it may have split a multibyte
		// rune, which strings.ToValidUTF8 would otherwise leave as U+FFFD.
		text = strings.ToValidUTF8(text[:nativeDiagnosticLimit], "") + "...(truncated)"
	}
	return text
}

func sanitizeNativeDiagnosticText(output []byte) string {
	text := strings.ToValidUTF8(string(output), "")
	text = strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return ' '
		case r < 0x20 || r == 0x7f:
			return -1
		default:
			return r
		}
	}, text)
	return strings.Join(strings.Fields(text), " ")
}

func codexRegistryFinding(body []byte, name, expectedMarketplace string, owned bool) registryFinding {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	value, err := decodeUniqueJSONValue(decoder)
	if err != nil {
		return registryIndeterminate
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return registryIndeterminate
	}
	document, ok := value.(map[string]any)
	if !ok {
		return registryIndeterminate
	}
	entries, ok := document["installed"].([]any)
	if !ok {
		return registryIndeterminate
	}
	finding := registryClear
	seen := map[string]bool{}
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			return registryIndeterminate
		}
		entryName, nameOK := entry["name"].(string)
		marketplace, marketplaceOK := entry["marketplaceName"].(string)
		pluginID, idOK := entry["pluginId"].(string)
		_, installedOK := entry["installed"].(bool)
		_, enabledOK := entry["enabled"].(bool)
		if !nameOK || !marketplaceOK || !idOK || !installedOK || !enabledOK || entryName == "" || marketplace == "" || pluginID != entryName+"@"+marketplace || seen[pluginID] {
			return registryIndeterminate
		}
		seen[pluginID] = true
		if entryName != name {
			continue
		}
		if marketplace == expectedMarketplace {
			if !owned {
				return registryCollision
			}
			finding = registryExpected
		}
		// A different non-empty marketplace is positive namespace evidence and
		// can coexist with the managed marketplace.
	}
	return finding
}

func (observer NativeIdentityObserver) inspectCopilotCLI(ctx context.Context, plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	if observer.Runner == nil {
		return registryIndeterminate, nil
	}
	result, err := observer.runNativeRegistry(ctx, legacyports.Command{Argv: []string{plan.NativeRegistryExecutable, "plugin", "list"}})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return registryIndeterminate, ctxErr
		}
		return registryIndeterminate, err
	}
	if result.ExitCode != 0 {
		return registryIndeterminate, fmt.Errorf("Copilot plugin registry command failed with exit code %d", result.ExitCode)
	}
	expectedPath := plan.ActivePath
	if managed != nil && strings.TrimSpace(managed.TargetLocator) != "" {
		expectedPath = managed.TargetLocator
	}
	return copilotRegistryFindingAt(result.Stdout, plan.DeclaredName, managedMarketplaceName(plan.PhysicalArtifactID), copilotMarketplaceVersion(plan.DeclaredVersion), expectedPath, managed != nil), nil
}

func (observer NativeIdentityObserver) runNativeRegistry(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if runner, ok := observer.Runner.(treeCommandRunner); ok {
		return runner.RunWithTreeExitGrace(ctx, command, time.Second)
	}
	return observer.Runner.Run(ctx, command)
}

func copilotRegistryFinding(stdout []byte, name, expectedMarketplace, expectedVersion string, owned bool) registryFinding {
	return copilotRegistryFindingAt(stdout, name, expectedMarketplace, expectedVersion, "", owned)
}

func copilotRegistryFindingAt(stdout []byte, name, expectedMarketplace, expectedVersion, expectedPath string, owned bool) registryFinding {
	expected := name + "@" + expectedMarketplace
	if status, recognized := copilotLivePluginStatus(stdout, expected, expectedVersion, expectedPath); recognized {
		switch status {
		case copilotStatusInstalled:
			if !owned {
				return registryCollision
			}
			return registryExpected
		case copilotStatusAbsent:
			return registryClear
		default:
			return registryIndeterminate
		}
	}
	normalized := strings.ReplaceAll(string(stdout), "\r\n", "\n")
	document := strings.TrimSuffix(normalized, "\n")
	if document == "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin." {
		return registryClear
	}
	inInstalled := false
	recognizedHeader := false
	recognizedEmpty := false
	recognizedEntry := false
	finding := registryClear
	seen := map[string]bool{}
	for _, line := range strings.Split(document, "\n") {
		if strings.TrimSpace(line) == "Installed plugins:" {
			if inInstalled || recognizedHeader {
				return registryIndeterminate
			}
			inInstalled, recognizedHeader = true, true
			continue
		}
		if !inInstalled {
			return registryIndeterminate
		}
		if line != "" && line[0] != ' ' && line[0] != '\t' {
			return registryIndeterminate
		}
		match := copilotInstalledEntry.FindStringSubmatch(line)
		if len(match) == 3 {
			if recognizedEmpty {
				return registryIndeterminate
			}
			recognizedEntry = true
			identity := match[1]
			if seen[identity] {
				return registryIndeterminate
			}
			seen[identity] = true
			parts := strings.Split(identity, "@")
			if len(parts) != 2 {
				return registryIndeterminate
			}
			if parts[0] == name && parts[1] == expectedMarketplace {
				if match[2] != expectedVersion {
					return registryIndeterminate
				}
				if !owned {
					return registryCollision
				}
				finding = registryExpected
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if (trimmed == "No plugins installed." || trimmed == "No plugins installed") && !recognizedEntry && !recognizedEmpty {
			recognizedEmpty = true
			continue
		}
		return registryIndeterminate
	}
	if !recognizedHeader || (!recognizedEntry && !recognizedEmpty) {
		return registryIndeterminate
	}
	return finding
}

func inspectCodexFiles(plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" {
		return registryIndeterminate, nil
	}
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return registryClear, nil
	} else if err != nil {
		return registryIndeterminate, err
	}
	expectedMarketplace := managedMarketplaceName(plan.PhysicalArtifactID)
	finding := registryClear
	configPath := filepath.Join(root, "config.toml")
	body, err := os.ReadFile(configPath)
	if err == nil {
		var config struct {
			Plugins map[string]map[string]any `toml:"plugins"`
		}
		if err := toml.Unmarshal(body, &config); err != nil {
			return registryIndeterminate, err
		}
		for identity := range config.Plugins {
			parts := strings.Split(identity, "@")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return registryIndeterminate, nil
			}
			if parts[0] == plan.DeclaredName && parts[1] == expectedMarketplace {
				if managed == nil {
					return registryCollision, nil
				}
				finding = registryExpected
			}
		}
	} else if !os.IsNotExist(err) {
		return registryIndeterminate, err
	}
	cacheFinding, err := inspectCodexCache(filepath.Join(root, "plugins", "cache"), plan.DeclaredName, expectedMarketplace, managed != nil)
	if err != nil {
		return registryIndeterminate, err
	}
	if cacheFinding == registryCollision || cacheFinding == registryIndeterminate {
		return cacheFinding, nil
	}
	if cacheFinding == registryExpected {
		finding = registryExpected
	}
	return finding, nil
}

func inspectCodexCache(root, name, expectedMarketplace string, owned bool) (registryFinding, error) {
	markets, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return registryClear, nil
	}
	if err != nil {
		return registryIndeterminate, err
	}
	finding := registryClear
	for _, market := range markets {
		if market.Type()&os.ModeSymlink != 0 || !market.IsDir() {
			return registryIndeterminate, nil
		}
		plugins, err := os.ReadDir(filepath.Join(root, market.Name()))
		if err != nil {
			return registryIndeterminate, err
		}
		for _, plugin := range plugins {
			if plugin.Type()&os.ModeSymlink != 0 || !plugin.IsDir() {
				return registryIndeterminate, nil
			}
			manifest := filepath.Join(root, market.Name(), plugin.Name(), "local", ".codex-plugin", "plugin.json")
			manifestName, err := readJSONManifestName(manifest)
			if os.IsNotExist(err) {
				return registryIndeterminate, nil
			}
			if err != nil {
				return registryIndeterminate, err
			}
			if manifestName != name {
				continue
			}
			if market.Name() == expectedMarketplace {
				if !owned {
					return registryCollision, nil
				}
				finding = registryExpected
			}
		}
	}
	return finding, nil
}

func inspectKiroRegistry(plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" {
		return registryIndeterminate, nil
	}
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return registryClear, nil
	} else if err != nil {
		return registryIndeterminate, err
	}
	if managed != nil {
		if err := verifyKiroNativeObjects(root, managed.NativeObjects, true); err != nil {
			return registryIndeterminate, err
		}
	}
	finding := registryClear
	mcp := map[string]any{}
	if hasSupportedMCP(plan.Components) {
		mcpPath := filepath.Join(root, "settings", "mcp.json")
		if err := validateKiroNativePath(root, mcpPath); err != nil {
			return registryIndeterminate, err
		}
		var err error
		mcp, _, _, _, err = readKiroMCPConfig(mcpPath)
		if err != nil {
			return registryIndeterminate, err
		}
	}
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		var exists bool
		switch component.Kind {
		case domain.ComponentSkill:
			skillPath := filepath.Join(root, "skills", component.Name)
			if err := validateKiroNativePath(root, skillPath); err != nil {
				return registryIndeterminate, err
			}
			_, statErr := os.Lstat(skillPath)
			exists = statErr == nil
			if statErr != nil && !os.IsNotExist(statErr) {
				return registryIndeterminate, statErr
			}
		case domain.ComponentMCPServer:
			_, exists = mcp[component.Name]
		default:
			continue
		}
		if !exists {
			continue
		}
		if managed == nil {
			return registryCollision, nil
		}
		if !managedKiroObjectExists(managed.NativeObjects, component.Kind, component.Name) {
			return registryIndeterminate, nil
		}
		finding = registryExpected
	}
	return finding, nil
}

func managedKiroObjectExists(objects []domain.NativeObjectOwnership, kind domain.ComponentKind, name string) bool {
	want := kiroSkillObjectKind
	if kind == domain.ComponentMCPServer {
		want = kiroMCPObjectKind
	}
	for _, object := range objects {
		if object.Kind == want && object.LogicalName == name {
			return true
		}
	}
	return false
}

func hasSupportedMCP(components []domain.ComponentDecision) bool {
	for _, component := range components {
		if component.Kind == domain.ComponentMCPServer && component.Support != domain.SupportUnsupported {
			return true
		}
	}
	return false
}

func inspectPreparedRegistry(plan domain.DeliveryPlan, name string, owned bool) (registryFinding, error) {
	return inspectUnqualifiedPluginRoot(plan.TargetRoot, name, plan.ActivePath, owned)
}

func inspectClaudeSkillsRegistry(plan domain.DeliveryPlan, name string, owned bool) (registryFinding, error) {
	root := strings.TrimSpace(plan.TargetRoot)
	if root == "" {
		return registryIndeterminate, nil
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return registryClear, nil
	}
	if err != nil {
		return registryIndeterminate, err
	}
	finding := registryClear
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return registryIndeterminate, nil
		}
		if !entry.IsDir() {
			// A plain file cannot contain the .claude-plugin/plugin.json this
			// scheme requires, so it can never claim a competing plugin
			// identity. OS-generated artifacts such as .DS_Store are common
			// in a Finder-browsed skills directory and must not block every
			// other plugin's repair/update.
			continue
		}
		path := filepath.Join(root, entry.Name())
		manifest := filepath.Join(path, ".claude-plugin", "plugin.json")
		manifestName, readErr := readJSONManifestName(manifest)
		if os.IsNotExist(readErr) {
			// Plain skills legitimately share this directory and do not claim a
			// plugin identity.
			if _, skillErr := os.Lstat(filepath.Join(path, "SKILL.md")); skillErr == nil {
				continue
			} else if !os.IsNotExist(skillErr) {
				return registryIndeterminate, skillErr
			}
			return registryIndeterminate, nil
		}
		if readErr != nil {
			return registryIndeterminate, readErr
		}
		if manifestName != name {
			continue
		}
		if sameCleanPath(path, plan.ActivePath) && owned {
			finding = registryExpected
			continue
		}
		return registryCollision, nil
	}
	return finding, nil
}

func inspectUnqualifiedPluginRoot(root, name, activePath string, owned bool) (registryFinding, error) {
	if strings.TrimSpace(root) == "" {
		return registryIndeterminate, nil
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return registryClear, nil
	}
	if err != nil {
		return registryIndeterminate, err
	}
	finding := registryClear
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".agentplugins-staging-") {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return registryIndeterminate, nil
		}
		if !entry.IsDir() {
			// A plain file cannot contain the manifest this scheme requires,
			// so it can never claim a competing plugin identity. OS-generated
			// artifacts such as .DS_Store are common here and must not block
			// every other plugin's repair/update.
			continue
		}
		path := filepath.Join(root, entry.Name())
		manifestName, qualified, namespace, err := nativeManifestIdentity(path)
		if err != nil {
			return registryIndeterminate, err
		}
		if manifestName != name {
			continue
		}
		if qualified && namespace != "" && namespace != managedMarketplaceName(filepath.Base(activePath)) {
			continue
		}
		if activePath != "" && sameCleanPath(path, activePath) && owned {
			finding = registryExpected
			continue
		}
		return registryCollision, nil
	}
	return finding, nil
}

func nativeManifestIdentity(root string) (name string, qualified bool, namespace string, err error) {
	for _, marketplace := range []string{filepath.Join(root, ".agents", "plugins", "marketplace.json"), filepath.Join(root, ".github", "plugin", "marketplace.json")} {
		body, readErr := os.ReadFile(marketplace)
		if readErr == nil {
			value, decodeErr := decodeStrictJSONObject(body)
			if decodeErr != nil {
				return "", false, "", decodeErr
			}
			namespace, _ = value["name"].(string)
			plugins, ok := value["plugins"].([]any)
			if namespace == "" || !ok || len(plugins) != 1 {
				return "", false, "", fmt.Errorf("invalid prepared marketplace identity")
			}
			plugin, ok := plugins[0].(map[string]any)
			if !ok {
				return "", false, "", fmt.Errorf("invalid prepared marketplace plugin identity")
			}
			name, ok = plugin["name"].(string)
			if !ok || name == "" {
				return "", false, "", fmt.Errorf("invalid prepared marketplace plugin name")
			}
			return name, true, namespace, nil
		}
		if !os.IsNotExist(readErr) {
			return "", false, "", readErr
		}
	}
	for _, manifest := range []string{filepath.Join(root, ".claude-plugin", "plugin.json"), filepath.Join(root, "plugin.json"), filepath.Join(root, ".cursor-plugin", "plugin.json"), filepath.Join(root, ".codex-plugin", "plugin.json")} {
		name, readErr := readJSONManifestName(manifest)
		if readErr == nil {
			return name, false, "", nil
		}
		if !os.IsNotExist(readErr) {
			return "", false, "", readErr
		}
	}
	return "", false, "", fmt.Errorf("native package has no recognized authoritative manifest")
}

func readJSONManifestName(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value, err := decodeStrictJSONObject(body)
	if err != nil {
		return "", err
	}
	name, ok := value["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("manifest %s has no valid name", path)
	}
	return name, nil
}

func decodeStrictJSONObject(body []byte) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	value, err := decodeUniqueJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("JSON document has trailing data")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON document must be an object")
	}
	return object, nil
}

func sameCleanPath(left, right string) bool {
	leftAbsolute, leftErr := filepath.Abs(left)
	rightAbsolute, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbsolute) == filepath.Clean(rightAbsolute)
}

func managedPackageDigest(client domain.ClientBinding) string {
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" {
			return object.ManagedDigest
		}
	}
	return ""
}
