package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type packagedMCPLayout int

const (
	packagedMCPSidecarWrapped packagedMCPLayout = iota
	packagedMCPInline
)

const (
	packagedPluginSkillsRef = "./skills/"
	packagedPluginMCPRef    = "./.mcp.json"
	kimiRootManifestRel     = "kimi.plugin.json"
)

type packagedPluginAdapter struct {
	id          string
	displayName string
	manifestRel string
	// manifestAlt lists native manifest paths consulted before manifestRel when
	// reading: Kimi publishes kimi.plugin.json at the plugin root, and that copy
	// wins when both exist.
	manifestAlt []string
	mcpLayout   packagedMCPLayout
}

func newGrokPluginAdapter() packagedPluginAdapter {
	return packagedPluginAdapter{
		id:          "grok",
		displayName: "Grok",
		manifestRel: ".grok-plugin/plugin.json",
		mcpLayout:   packagedMCPSidecarWrapped,
	}
}

func newKimiPluginAdapter() packagedPluginAdapter {
	return packagedPluginAdapter{
		id:          "kimi",
		displayName: "Kimi",
		manifestRel: ".kimi-plugin/plugin.json",
		manifestAlt: []string{kimiRootManifestRel},
		mcpLayout:   packagedMCPInline,
	}
}

func (a packagedPluginAdapter) manifestCandidates() []string {
	rels := make([]string, 0, len(a.manifestAlt)+1)
	rels = append(rels, a.manifestAlt...)
	return append(rels, a.manifestRel)
}

// manifestWriteRel returns the manifest path Generate must update so that
// readManifest keeps resolving the same file after generation.
func (a packagedPluginAdapter) manifestWriteRel(root string) string {
	for _, rel := range a.manifestCandidates() {
		if fileExists(filepath.Join(root, rel)) {
			return rel
		}
	}
	return a.manifestRel
}

func (a packagedPluginAdapter) ID() string { return a.id }

func (a packagedPluginAdapter) DetectNative(root string) bool {
	for _, rel := range a.manifestCandidates() {
		if fileExists(filepath.Join(root, rel)) {
			return true
		}
	}
	return false
}

func (a packagedPluginAdapter) RefineDiscovery(root string, state *pluginmodel.TargetState) error {
	authoredRoot := authoredRootHint(*state, pluginmodel.NewPortableComponents())
	targetDir := filepath.Join(root, authoredRoot, "targets", a.id)
	entries, err := os.ReadDir(targetDir)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	case len(entries) == 0:
		return nil
	default:
		return fmt.Errorf("target %s does not support %s/targets/%s/... in phase 1: use %s/skills/** and %s/mcp/servers.yaml", a.id, authoredRoot, a.id, authoredRoot, authoredRoot)
	}
}

func (a packagedPluginAdapter) ManagedPaths(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]string, error) {
	if a.mcpLayout == packagedMCPSidecarWrapped {
		// The sidecar is managed even when portable MCP is absent: it becomes
		// stale (and must be removed) when portable MCP is dropped, otherwise a
		// later import resurrects servers from the orphaned sidecar.
		return []string{".mcp.json"}, nil
	}
	return nil, nil
}

func (a packagedPluginAdapter) Generate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]pluginmodel.Artifact, error) {
	doc := map[string]any{
		"name":        graph.Manifest.Name,
		"version":     graph.Manifest.Version,
		"description": graph.Manifest.Description,
	}
	if len(graph.Portable.Paths("skills")) > 0 {
		doc["skills"] = packagedPluginSkillsRef
	}
	var artifacts []pluginmodel.Artifact
	if graph.Portable.MCP != nil {
		projected, err := renderPortableMCPForTarget(graph.Portable.MCP, a.id)
		if err != nil {
			return nil, err
		}
		switch a.mcpLayout {
		case packagedMCPInline:
			doc["mcpServers"] = projected
		default:
			doc["mcpServers"] = packagedPluginMCPRef
			body, err := marshalJSON(map[string]any{"mcpServers": projected})
			if err != nil {
				return nil, err
			}
			artifacts = append(artifacts, pluginmodel.Artifact{
				RelPath: ".mcp.json",
				Content: body,
			})
		}
	}
	body, err := marshalJSON(doc)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, pluginmodel.Artifact{
		RelPath: a.manifestWriteRel(root),
		Content: body,
	})
	skillArtifacts, err := renderPortableSkills(root, graph.Portable.Paths("skills"), "skills")
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, skillArtifacts...)
	return compactArtifacts(artifacts), nil
}

func (a packagedPluginAdapter) Import(root string, seed ImportSeed) (ImportResult, error) {
	result := ImportResult{
		Manifest: seed.Manifest,
		Launcher: seed.Launcher,
	}
	manifest, path, ok, err := a.readManifest(root)
	if err != nil {
		return ImportResult{}, err
	}
	if !ok {
		return ImportResult{}, fmt.Errorf("%s plugin import requires %s", a.displayName, strings.Join(a.manifestCandidates(), " or "))
	}
	if name := stringMapField(manifest, "name"); name != "" {
		result.Manifest.Name = name
	}
	if version := stringMapField(manifest, "version"); version != "" {
		result.Manifest.Version = version
	}
	if description := stringMapField(manifest, "description"); description != "" {
		result.Manifest.Description = description
	}
	if servers, ok, err := a.importedMCP(root, path, manifest); err != nil {
		return ImportResult{}, err
	} else if ok {
		artifact, err := importedPortableMCPArtifact(a.id, servers)
		if err != nil {
			return ImportResult{}, err
		}
		result.Artifacts = append(result.Artifacts, artifact)
	}
	skillArtifacts, err := copyArtifactDirs(root, artifactDir{src: "skills", dst: "skills"})
	if err != nil {
		return ImportResult{}, err
	}
	result.Artifacts = append(result.Artifacts, skillArtifacts...)
	result.Artifacts = compactArtifacts(result.Artifacts)
	return result, nil
}

func (a packagedPluginAdapter) Validate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]Diagnostic, error) {
	manifest, manifestRel, diagnostics := a.validateManifestRead(root)
	if len(diagnostics) > 0 {
		return diagnostics, nil
	}
	a.manifestRel = manifestRel // diagnostics must name the manifest actually read (Kimi root manifest wins)
	diagnostics = append(diagnostics, a.validateIdentity(manifest, graph)...)
	diagnostics = append(diagnostics, a.validateSkills(root, manifest, graph)...)
	mcpDiagnostics, err := a.validateMCP(root, manifest, graph)
	if err != nil {
		return nil, err
	}
	return append(diagnostics, mcpDiagnostics...), nil
}

func (a packagedPluginAdapter) readManifest(root string) (map[string]any, string, bool, error) {
	for _, rel := range a.manifestCandidates() {
		body, err := readRegularFile(filepath.Join(root, rel), rel)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, rel, false, err
		}
		doc, err := decodeJSONObject(body, a.displayName+" plugin manifest "+rel)
		if err != nil {
			return nil, rel, false, err
		}
		return doc, rel, true, nil
	}
	return nil, "", false, nil
}

func (a packagedPluginAdapter) importedMCP(root, manifestRel string, manifest map[string]any) (map[string]any, bool, error) {
	value, ok := manifest["mcpServers"]
	if !ok || value == nil {
		return nil, false, nil
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed, true, nil
	case string:
		if a.mcpLayout == packagedMCPInline {
			return nil, false, fmt.Errorf("%s plugin manifest %s field %q must be an inline object; %s does not resolve MCP file refs", a.displayName, manifestRel, "mcpServers", a.displayName)
		}
		ref := strings.TrimSpace(typed)
		if ref == "" {
			return nil, false, nil
		}
		rel, err := containedPluginRel(ref)
		if err != nil {
			return nil, false, fmt.Errorf("unsupported %s plugin manifest %s mcpServers ref %q: %w", a.displayName, manifestRel, ref, err)
		}
		servers, ok, err := readWrappedMCPServers(filepath.Join(root, rel), rel)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, fmt.Errorf("%s plugin manifest %s mcpServers ref %q does not exist", a.displayName, manifestRel, ref)
		}
		return servers, true, nil
	default:
		return nil, false, fmt.Errorf("%s plugin manifest %s field %q must be a string ref or object", a.displayName, manifestRel, "mcpServers")
	}
}

func containedPluginRel(ref string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(ref))
	cleaned = strings.TrimPrefix(cleaned, "."+string(filepath.Separator))
	if cleaned == "" || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || filepath.IsAbs(cleaned) || filepath.VolumeName(cleaned) != "" {
		return "", fmt.Errorf("path must stay inside the plugin root")
	}
	return filepath.ToSlash(cleaned), nil
}

func readWrappedMCPServers(path, label string) (map[string]any, bool, error) {
	body, err := readRegularFile(path, label)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	doc, err := decodeJSONObject(body, "plugin MCP config "+label)
	if err != nil {
		return nil, false, err
	}
	if value, ok := doc["mcpServers"]; ok {
		servers, ok := value.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("plugin MCP config %s field %q must be a JSON object", label, "mcpServers")
		}
		return servers, true, nil
	}
	return doc, true, nil
}

func (a packagedPluginAdapter) validateManifestRead(root string) (map[string]any, string, []Diagnostic) {
	manifest, manifestRel, ok, err := a.readManifest(root)
	if err != nil {
		if manifestRel == "" {
			manifestRel = a.manifestRel
		}
		return nil, manifestRel, []Diagnostic{a.manifestDiagnostic(CodeManifestInvalid, fmt.Sprintf("%s plugin manifest %s is invalid: %v", a.displayName, manifestRel, err))}
	}
	if !ok {
		return nil, a.manifestRel, []Diagnostic{a.manifestDiagnostic(CodeGeneratedContractInvalid, fmt.Sprintf("%s plugin manifest %s is not readable", a.displayName, a.manifestRel))}
	}
	return manifest, manifestRel, nil
}

func (a packagedPluginAdapter) validateIdentity(manifest map[string]any, graph pluginmodel.PackageGraph) []Diagnostic {
	return append(
		append(
			a.validateIdentityField(manifest, "name", strings.TrimSpace(graph.Manifest.Name)),
			a.validateIdentityField(manifest, "version", strings.TrimSpace(graph.Manifest.Version))...,
		),
		a.validateIdentityField(manifest, "description", strings.TrimSpace(graph.Manifest.Description))...,
	)
}

func (a packagedPluginAdapter) validateIdentityField(manifest map[string]any, field, expected string) []Diagnostic {
	got := stringMapField(manifest, field)
	if got == expected {
		return nil
	}
	return []Diagnostic{a.manifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("%s plugin manifest %s sets %s %q; expected %q from plugin.yaml", a.displayName, a.manifestRel, field, got, expected),
	)}
}

func (a packagedPluginAdapter) validateSkills(root string, manifest map[string]any, graph pluginmodel.PackageGraph) []Diagnostic {
	paths := graph.Portable.Paths("skills")
	if len(paths) == 0 {
		if value, ok := manifest["skills"]; !ok || value == nil {
			return nil
		}
		return []Diagnostic{a.manifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("%s plugin manifest %s may not define skills when portable skills are absent", a.displayName, a.manifestRel),
		)}
	}
	diagnostics := a.validateSkillsRef(manifest)
	for _, rel := range paths {
		diagnostics = append(diagnostics, a.validateSkillProjection(root, rel)...)
	}
	return diagnostics
}

func (a packagedPluginAdapter) validateSkillsRef(manifest map[string]any) []Diagnostic {
	ref, ok := manifest["skills"]
	if !ok {
		return []Diagnostic{a.manifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("%s plugin manifest %s must reference %q when portable skills are authored", a.displayName, a.manifestRel, packagedPluginSkillsRef),
		)}
	}
	refText, ok := ref.(string)
	if !ok || strings.TrimSpace(refText) != packagedPluginSkillsRef {
		return []Diagnostic{a.manifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("%s plugin manifest %s must use %q for skills when portable skills are authored", a.displayName, a.manifestRel, packagedPluginSkillsRef),
		)}
	}
	return nil
}

func (a packagedPluginAdapter) validateSkillProjection(root, rel string) []Diagnostic {
	child, err := portableSkillChildPath(rel)
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     filepath.ToSlash(rel),
			Target:   a.id,
			Message:  fmt.Sprintf("%s portable skill path %s is invalid: %v", a.displayName, rel, err),
		}}
	}
	renderedRel := filepath.ToSlash(filepath.Join("skills", child))
	sourceBody, err := readRegularFile(filepath.Join(root, rel), filepath.ToSlash(rel))
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     filepath.ToSlash(rel),
			Target:   a.id,
			Message:  fmt.Sprintf("%s portable skill source %s is not readable: %v", a.displayName, rel, err),
		}}
	}
	renderedBody, err := readRegularFile(filepath.Join(root, renderedRel), renderedRel)
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     renderedRel,
			Target:   a.id,
			Message:  fmt.Sprintf("%s generated skill %s is not readable", a.displayName, renderedRel),
		}}
	}
	if string(sourceBody) == string(renderedBody) {
		return nil
	}
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     renderedRel,
		Target:   a.id,
		Message:  fmt.Sprintf("%s generated skill %s does not match authored portable skill %s", a.displayName, renderedRel, rel),
	}}
}

func (a packagedPluginAdapter) validateMCP(root string, manifest map[string]any, graph pluginmodel.PackageGraph) ([]Diagnostic, error) {
	if graph.Portable.MCP == nil {
		if value, ok := manifest["mcpServers"]; !ok || value == nil {
			return nil, nil
		}
		return []Diagnostic{a.manifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("%s plugin manifest %s may not define mcpServers when portable MCP is absent", a.displayName, a.manifestRel),
		)}, nil
	}
	projected, err := renderPortableMCPForTarget(graph.Portable.MCP, a.id)
	if err != nil {
		return nil, err
	}
	rendered, diagnostics := a.readGeneratedMCP(root, manifest)
	if len(diagnostics) > 0 {
		return diagnostics, nil
	}
	if jsonDocumentsEqual(projected, rendered) {
		return nil, nil
	}
	path := a.manifestRel
	if a.mcpLayout == packagedMCPSidecarWrapped {
		path = ".mcp.json"
	}
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     path,
		Target:   a.id,
		Message:  fmt.Sprintf("%s plugin MCP projection does not match authored portable MCP", a.displayName),
	}}, nil
}

func (a packagedPluginAdapter) readGeneratedMCP(root string, manifest map[string]any) (map[string]any, []Diagnostic) {
	value, ok := manifest["mcpServers"]
	if !ok {
		return nil, []Diagnostic{a.manifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("%s plugin manifest %s must declare mcpServers when portable MCP is authored", a.displayName, a.manifestRel),
		)}
	}
	switch a.mcpLayout {
	case packagedMCPInline:
		servers, ok := value.(map[string]any)
		if !ok {
			return nil, []Diagnostic{a.manifestDiagnostic(
				CodeGeneratedContractInvalid,
				fmt.Sprintf("%s plugin manifest %s field %q must be an inline MCP object", a.displayName, a.manifestRel, "mcpServers"),
			)}
		}
		return servers, nil
	default:
		refText, ok := value.(string)
		if !ok || strings.TrimSpace(refText) != packagedPluginMCPRef {
			return nil, []Diagnostic{a.manifestDiagnostic(
				CodeGeneratedContractInvalid,
				fmt.Sprintf("%s plugin manifest %s must use %q for mcpServers when portable MCP is authored", a.displayName, a.manifestRel, packagedPluginMCPRef),
			)}
		}
		servers, readable, err := readWrappedMCPServers(filepath.Join(root, ".mcp.json"), ".mcp.json")
		if err != nil {
			return nil, []Diagnostic{{
				Severity: SeverityFailure,
				Code:     CodeGeneratedContractInvalid,
				Path:     ".mcp.json",
				Target:   a.id,
				Message:  fmt.Sprintf("%s plugin MCP config .mcp.json is invalid: %v", a.displayName, err),
			}}
		}
		if !readable {
			return nil, []Diagnostic{{
				Severity: SeverityFailure,
				Code:     CodeGeneratedContractInvalid,
				Path:     ".mcp.json",
				Target:   a.id,
				Message:  fmt.Sprintf("%s plugin MCP config .mcp.json is not readable", a.displayName),
			}}
		}
		return servers, nil
	}
}

func (a packagedPluginAdapter) manifestDiagnostic(code, message string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     code,
		Path:     a.manifestRel,
		Target:   a.id,
		Message:  message,
	}
}
