package conformance

import (
	"context"
	"errors"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// DecodePlugin is staged so installer policy can retain its early-fatal precedence.
// Author reports validate the original schema independently of the loading view.
func (d Decoder) DecodePlugin(ctx context.Context, doc Document) (Facts, error) {
	f := d.newFacts()
	l := d.Limits.bounded()
	ready, state, err := d.observe(ctx, &f, doc, "plugin.json", l.PluginBytes, domain.BoundaryPlugin, false)
	f.Coverage.Plugin = state
	if err != nil {
		return f, err
	}
	if !ready {
		f.finish(l)
		return f, nil
	}
	duplicates, state, err := d.structure(ctx, &f, doc, domain.BoundaryPlugin)
	f.Coverage.Plugin = state
	if err != nil {
		return f, err
	}
	if state != Pass {
		f.finish(l)
		return f, nil
	}
	_, decoded, err := decodeJSONObject(doc.body)
	if err != nil { // Root duplicate makes the canonical identity unavailable.
		f.Coverage.Plugin = NotEvaluated
		if len(duplicates) == 0 {
			f.Coverage.Plugin = Fail
			f.add("plugin_object_required", "plugins/5.2", "plugin.json", "", "", Normative, domain.BoundaryPlugin)
		}
		f.finish(l)
		return f, nil
	}
	schema, ok := decoded["$schema"].(string)
	if !ok || strings.TrimSpace(schema) == "" {
		f.add("plugin_schema_missing", "plugins/5.2", "plugin.json", "/$schema", "", Normative, domain.BoundaryPlugin)
		f.Coverage.Plugin = Fail
		f.finish(l)
		return f, nil
	}
	if schema != domain.PluginSchemaV1 {
		f.add("plugin_schema_unsupported", "plugins/5.2", "plugin.json", "/$schema", "", InstallerPolicy, domain.BoundaryPlugin)
		f.Coverage.Plugin = NotEvaluated
		f.finish(l)
		return f, nil
	}
	f.SchemaIDs = []string{schema}
	if !d.registryReady(schema) {
		f.add("schema_registry_unavailable", "engine/schema", "plugin.json", "", "", InstallerPolicy, domain.BoundaryPlugin)
		f.Coverage.Plugin = NotEvaluated
		f.finish(l)
		return f, nil
	}
	if err = d.Registry.Validate(schema, decoded); err != nil {
		if schemaViolation(err) {
			f.add("plugin_original_schema_invalid", "plugins/5.2", "plugin.json", "", "", Normative, domain.BoundaryPlugin)
			f.Coverage.Plugin = Fail
		} else {
			f.add("schema_registry_unavailable", "engine/schema", "plugin.json", "", "", InstallerPolicy, domain.BoundaryPlugin)
			f.Coverage.Plugin = NotEvaluated
			f.finish(l)
			return f, nil
		}
	}
	manifest, diagnostics, digest, err := (InstallerDecoder{Registry: d.Registry}).Plugin(doc.body)
	if err := ctx.Err(); err != nil {
		return f, err
	}
	if err != nil {
		var load *domain.LoadError
		if errors.As(err, &load) {
			layer := Normative
			if len(duplicates) > 0 {
				layer = HostSafety
			}
			if load.Diagnostic.Code == "plugin_manifest_decode_failed" || (load.Diagnostic.Code == "plugin_schema_invalid" && load.Cause != nil && !schemaViolation(load.Cause)) {
				layer = InstallerPolicy
			}
			f.add(load.Diagnostic.Code, "plugins/5.2", "plugin.json", "", "", layer, domain.BoundaryPlugin)
			if layer != Normative {
				f.Coverage.Plugin = mergeOutcome(f.Coverage.Plugin, NotEvaluated)
			} else {
				f.Coverage.Plugin = Fail
			}
		}
	} else {
		f.Package = &domain.PackageEnvelope{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: schema, SchemaVersion: "1.0.0", ManifestSchema: domain.SchemaIdentity{URI: schema, Version: "1.0.0"}, Manifest: manifest, ManifestDigest: digest, Skills: map[string]domain.Skill{}}
		for _, v := range diagnostics {
			f.add(v.Code, "plugins/5.2", "plugin.json", "", v.Item, Normative, v.Boundary)
		}
	}
	if len(duplicates) > 0 {
		f.Coverage.Plugin = mergeOutcome(f.Coverage.Plugin, NotEvaluated)
	}
	f.finish(l)
	return f, nil
}
func (d Decoder) structure(ctx context.Context, f *Facts, doc Document, boundary domain.FailureBoundary) ([]duplicate, Outcome, error) {
	duplicates, err := scanJSON(ctx, doc.body, d.Limits.bounded(), duplicateRecord)
	if err != nil {
		if ctx.Err() != nil {
			return nil, NotEvaluated, ctx.Err()
		}
		var limit *parseFailure
		if errors.As(err, &limit) {
			f.add(limit.Code, "host/bounds", reportPath(boundary), "", "", HostSafety, boundary)
			return nil, NotEvaluated, nil
		}
		rule := "plugins/5.2"
		if boundary == domain.BoundaryMCP {
			rule = "plugins/7.2.1"
		}
		f.add("document_json_invalid", rule, reportPath(boundary), "", "", Normative, boundary)
		return nil, Fail, nil
	}
	for ordinal, dup := range duplicates {
		b := boundary
		item := ""
		pointer := ""
		if boundary == domain.BoundaryMCP && len(dup.keys) >= 3 && dup.keys[0] == "mcpServers" {
			b = domain.BoundaryMCPServer
			item = dup.keys[1]
			pointer = "/mcpServers"
		}
		before := len(f.Findings)
		f.add("json_duplicate_key", "host/duplicates", reportPath(b), pointer, item, HostSafety, b)
		if len(f.Findings) > before {
			f.Findings[len(f.Findings)-1].Ordinal = ordinal + 1
		}
	}
	return duplicates, Pass, nil
}

// DecodeMCP keeps document and individual-server failure boundaries. Paths are
// already observed containment values; omitted observations never mean pass.
func (d Decoder) DecodeMCP(ctx context.Context, doc Document, pluginSchema string, paths []PathObservation) (Facts, error) {
	f := d.newFacts()
	l := d.Limits.bounded()
	if len(paths) > l.Members {
		return d.limitFacts("input_member_limit"), ctx.Err()
	}
	observations := map[[2]string]Outcome{}
	observationBytes := 0
	for _, p := range paths {
		observationBytes += len(p.Item) + len(p.Field)
		if observationBytes > l.AggregateBytes {
			return d.limitFacts("input_aggregate_limit"), ctx.Err()
		}
		key := [2]string{p.Item, p.Field}
		if old, exists := observations[key]; exists {
			observations[key] = mergeOutcome(old, validOutcome(p.State))
		} else {
			observations[key] = validOutcome(p.State)
		}
	}
	ready, state, err := d.observe(ctx, &f, doc, "mcp.json", l.MCPBytes, domain.BoundaryMCP, true)
	f.Coverage.MCP = state
	if err != nil {
		return f, err
	}
	if !ready {
		if doc.Path == "mcp.json" && doc.State == WrongKind {
			f.Package = &domain.PackageEnvelope{MCP: domain.MCPComponent{Present: true}}
		}
		f.finish(l)
		return f, nil
	}
	duplicates, state, err := d.structure(ctx, &f, doc, domain.BoundaryMCP)
	f.Coverage.MCP = state
	if err != nil {
		return f, err
	}
	if state != Pass {
		f.Package = &domain.PackageEnvelope{MCP: domain.MCPComponent{Present: true, Raw: doc.Bytes()}}
		f.finish(l)
		return f, nil
	}
	if !d.registryReady(domain.MCPSchemaV1) {
		f.add("schema_registry_unavailable", "engine/schema", "mcp.json", "", "", InstallerPolicy, domain.BoundaryMCP)
		f.Coverage.MCP = NotEvaluated
		f.finish(l)
		return f, nil
	}
	if pluginSchema != domain.PluginSchemaV1 {
		f.add("mcp_parent_schema_unavailable", "plugins/10.1", "mcp.json", "/$schema", "", InstallerPolicy, domain.BoundaryMCP)
		f.Coverage.MCP = NotEvaluated
		f.finish(l)
		return f, nil
	}
	duplicateServer := map[string]bool{}
	documentDuplicate := false
	for _, v := range duplicates {
		if len(v.keys) >= 3 && v.keys[0] == "mcpServers" {
			duplicateServer[v.keys[1]] = true
		} else {
			documentDuplicate = true
		}
	}
	observer := func(code, item string, cause error) {
		serverBoundary := strings.HasPrefix(code, "server_")
		layer := Normative
		rule := "plugins/7.2.1"
		switch code {
		case "mcp_schema_unsupported":
			layer = InstallerPolicy
		case "mcp_schema_mismatch":
			rule = "plugins/10.1"
		case "mcp_malformed", "mcp_servers_invalid":
			if documentDuplicate {
				layer = HostSafety
			}
		case "server_json":
			if duplicateServer[item] {
				layer = HostSafety
			}
		case "server_schema", "mcp_schema_invalid":
			if !schemaViolation(cause) {
				layer = InstallerPolicy
				code = "schema_registry_unavailable"
			}
		case "server_semantics":
			var semantic *semanticFailure
			if errors.As(cause, &semantic) {
				code = semantic.Code
			}
		}
		f.add(code, rule, "mcp.json", "", item, layer, func() domain.FailureBoundary {
			if serverBoundary {
				return domain.BoundaryMCPServer
			}
			return domain.BoundaryMCP
		}())
		if layer == Normative {
			f.Coverage.MCP = Fail
		} else {
			f.Coverage.MCP = mergeOutcome(f.Coverage.MCP, NotEvaluated)
		}
	}
	component, _ := (InstallerDecoder{Registry: d.Registry, issue: observer}).MCP(doc.body, pluginSchema, func(config map[string]any) (*domain.StdioRequirement, error) {
		return ParseStdio(config, normativeCommand, normativeCWD)
	})
	if err := ctx.Err(); err != nil {
		return f, err
	}
	// Domain diagnostics may contain raw errors. They remain private to the legacy
	// facade; the author model carries only allowlisted finding codes.
	for name, old := range component.InvalidServer {
		old.Message = "invalid server configuration"
		component.InvalidServer[name] = old
	}
	for _, name := range sortedKeys(component.Servers) {
		server := component.Servers[name]
		if server.Type != "stdio" {
			continue
		}
		for _, field := range []string{"command", "cwd"} {
			value, _ := server.Decoded[field].(string)
			required := field == "command" && server.StdioRequirement.Kind == domain.ExecutableBundled || field == "cwd" && value != "" && value != "${PLUGIN_ROOT}" && value != "${PLUGIN_DATA}"
			if !required {
				continue
			}
			observation, exists := observations[[2]string{name, field}]
			if !exists {
				observation = NotEvaluated
			}
			if observation != Pass {
				layer := HostSafety
				code := "path_containment_unavailable"
				if observation == Fail {
					layer = Normative
					code = "path_containment_invalid"
					delete(component.Servers, name)
					component.InvalidServer[name] = domain.Diagnostic{Code: code, Boundary: domain.BoundaryMCPServer, Severity: domain.SeverityError, Path: "mcp.json", Item: name, Message: "invalid server containment"}
				}
				f.add(code, "plugins/4.1", "mcp.json", "/mcpServers", name, layer, domain.BoundaryMCPServer)
				f.Coverage.MCP = mergeOutcome(f.Coverage.MCP, observation)
			}
		}
	}
	if len(duplicates) > 0 {
		f.Coverage.MCP = mergeOutcome(f.Coverage.MCP, NotEvaluated)
	}
	f.Package = &domain.PackageEnvelope{MCP: component}
	if component.SchemaURI == domain.MCPSchemaV1 {
		f.SchemaIDs = []string{domain.MCPSchemaV1}
	}
	f.finish(l)
	return f, nil
}
func normativeCommand(value string) (string, error) { return ParseCommandPath(value) }
func normativeCWD(value string) error               { _, _, err := ParseCWDPath(value); return err }
