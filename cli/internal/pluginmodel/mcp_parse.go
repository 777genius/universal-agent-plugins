package pluginmodel

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func ParsePortableMCP(rel string, body []byte) (ParsedPortableMCP, error) {
	raw := map[string]any{}
	switch strings.ToLower(filepathExt(rel)) {
	case ".json":
		if err := json.Unmarshal(body, &raw); err != nil {
			return ParsedPortableMCP{}, fmt.Errorf("parse %s: %w", rel, err)
		}
	default:
		if err := yaml.Unmarshal(body, &raw); err != nil {
			return ParsedPortableMCP{}, fmt.Errorf("parse %s: %w", rel, err)
		}
	}
	if raw == nil {
		raw = map[string]any{}
	}
	_, hasAPIVersion := raw["api_version"]
	_, hasFormat := raw["format"]
	_, hasVersion := raw["version"]
	switch {
	case hasAPIVersion:
		if hasFormat || hasVersion {
			return ParsedPortableMCP{}, fmt.Errorf("portable MCP file %s must not mix api_version with legacy format/version markers", rel)
		}
	case hasFormat || hasVersion:
		if !hasFormat {
			return ParsedPortableMCP{}, fmt.Errorf("portable MCP file %s legacy schema must declare %q", rel, PortableMCPLegacyFormatMarker)
		}
		if !hasVersion {
			return ParsedPortableMCP{}, fmt.Errorf("portable MCP file %s legacy schema must declare version", rel)
		}
	default:
		return ParsedPortableMCP{}, fmt.Errorf("portable MCP file %s must declare api_version", rel)
	}
	return parsePortableMCPFile(rel, body)
}

func parsePortableMCPFile(rel string, body []byte) (ParsedPortableMCP, error) {
	var file PortableMCPFile
	switch strings.ToLower(filepathExt(rel)) {
	case ".json":
		if err := json.Unmarshal(body, &file); err != nil {
			return ParsedPortableMCP{}, fmt.Errorf("parse %s: %w", rel, err)
		}
	default:
		if err := yaml.Unmarshal(body, &file); err != nil {
			return ParsedPortableMCP{}, fmt.Errorf("parse %s: %w", rel, err)
		}
	}
	normalizePortableMCPFile(&file)
	if err := file.Validate(); err != nil {
		return ParsedPortableMCP{}, fmt.Errorf("parse %s: %w", rel, err)
	}
	return ParsedPortableMCP{
		Servers: mustPortableMCPMap(file.RenderLegacyProjection("")),
		File:    &file,
	}, nil
}
