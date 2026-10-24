package agentpluginscli

import (
	"context"
	"path/filepath"
	"sort"
	"sync"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

// ManagementBackend adapts shared application behavior to the generated wire
// contract. Prepared handles and catalog selections remain private Go objects.
type ManagementBackend struct {
	app         App
	config      management.Startup
	mu          sync.Mutex
	catalogs    map[string]*skillsCatalogSession
	webCatalogs map[string]managementWebCatalog
	closed      bool
}

func NewManagementBackend(app App, input management.Startup) (*ManagementBackend, error) {
	if input.StateRoot == "" {
		input.StateRoot = filepath.Dir(app.ManagedRoot)
	}
	if input.DependenciesRoot == "" {
		input.DependenciesRoot = app.DependenciesRoot
	}
	cfg, err := NormalizeManagementStartup(input)
	if err != nil {
		return nil, err
	}
	app, err = ConfigureManagementApp(app, cfg)
	if err != nil {
		return nil, err
	}
	return &ManagementBackend{app: app, config: cfg, catalogs: map[string]*skillsCatalogSession{}, webCatalogs: map[string]managementWebCatalog{}}, nil
}
func (b *ManagementBackend) ContextID() string     { return b.config.ContextId }
func (b *ManagementBackend) EnvironmentID() string { return ManagementEnvironmentID(b.config) }
func (b *ManagementBackend) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for id, catalog := range b.catalogs {
		catalog.cleanup()
		delete(b.catalogs, id)
	}
	clear(b.webCatalogs)
}
func (b *ManagementBackend) Session(ctx context.Context) (management.Session, error) {
	discovered, err := b.app.skillsTargets(ctx)
	if err != nil {
		return management.Session{}, err
	}
	byClient := map[string]management.Target{}
	for _, target := range discovered {
		reason := ""
		if target.Reason != nil {
			reason = *target.Reason
		}
		byClient[target.Id] = management.Target{Client: target.Id, Label: target.Label, Scope: b.config.Scope, DisplayPath: target.DisplayPath, Available: target.Available, Reason: reason}
	}
	out := management.Session{ApiVersion: management.N1, EngineVersion: b.app.Version, ContextId: b.ContextID(), EnvironmentId: b.EnvironmentID(), AuthoringRoots: append([]management.LocalSkillRoot{}, b.config.AuthoringRoots...), Scope: b.config.Scope, Targets: []management.Target{}, Capabilities: []management.Capability{}}
	for _, captured := range b.config.Targets {
		target := byClient[captured.Client]
		target.Id = captured.Id
		if reason := managementExecutableReason(captured, b.app.Installer.RequiresClientExecutable(captured.Client)); reason != "" {
			target.Available, target.Reason = false, reason
		}
		out.Targets = append(out.Targets, target)
	}
	for _, action := range []string{"install", "install_skills", "update", "sync", "repair", "remove", "recover", "source_git", "source_archive", "source_https_skill"} {
		out.Capabilities = append(out.Capabilities, management.Capability{Action: action, Supported: true})
	}
	if len(b.config.AuthoringRoots) > 0 {
		out.Capabilities = append(out.Capabilities, management.Capability{Action: "local_skill", Supported: true})
	}
	for _, action := range []string{"project_scope", "local_scope", "activation_attestation", "authentication_attestation"} {
		out.Capabilities = append(out.Capabilities, management.Capability{Action: action, Reason: "This engine has not qualified this management capability"})
	}
	return out, nil
}
func (b *ManagementBackend) clientTargets(ids []string, required bool) ([]string, error) {
	if required && len(ids) == 0 {
		return nil, invalidManagementRequest("Select at least one captured target for installation")
	}
	out := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, invalidManagementRequest("Select each target only once")
		}
		seen[id] = true
		client, err := b.selectedManagementClient(id, required)
		if err != nil {
			return nil, err
		}
		out = append(out, client)
	}
	sort.Strings(out)
	return out, nil
}
func (b *ManagementBackend) targetID(client string) string {
	for _, target := range b.config.Targets {
		if target.Client == client {
			return target.Id
		}
	}
	return ""
}
func managementDiagnostic(code, message string) management.Diagnostic {
	return management.Diagnostic{Code: code, Message: message, Severity: management.DiagnosticSeverityWarning}
}
