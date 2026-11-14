package codexmanifest

import "encoding/json"

func DecodeImportedPluginManifest(body []byte) (ImportedPluginManifest, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return ImportedPluginManifest{}, err
	}
	out := ImportedPluginManifest{}
	if value, ok, err := decodeJSONStringField(raw, "name"); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		out.Name = value
	}
	if value, ok, err := decodeJSONStringField(raw, "version"); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		out.Version = value
	}
	if value, ok, err := decodeJSONStringField(raw, "description"); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		out.Description = value
	}
	if value, ok, err := decodeAuthorField(raw); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		out.PackageMeta.Author = value
	}
	if value, ok, err := decodeJSONStringField(raw, "homepage"); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		out.PackageMeta.Homepage = value
	}
	if value, ok, err := decodeJSONStringField(raw, "repository"); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		out.PackageMeta.Repository = value
	}
	if value, ok, err := decodeJSONStringField(raw, "license"); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		out.PackageMeta.License = value
	}
	if values, ok, err := decodeJSONStringArrayField(raw, "keywords"); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		out.PackageMeta.Keywords = values
	}
	if value, ok, err := decodeJSONStringField(raw, "skills"); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		out.SkillsPath = value
	}
	if value, ok, err := decodeJSONStringField(raw, "mcpServers"); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		out.MCPServersRef = value
	}
	if value, ok, err := decodeJSONStringField(raw, "apps"); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		out.AppsRef = value
	}
	if value, ok, err := decodeJSONObjectField(raw, "interface"); err != nil {
		return ImportedPluginManifest{}, err
	} else if ok {
		if err := ValidateInterfaceDoc(value); err != nil {
			return ImportedPluginManifest{}, err
		}
		out.Interface = value
	}

	deleteImportedPluginCanonicalFields(raw)

	out.PackageMeta.Normalize()
	if len(raw) > 0 {
		out.Extra = raw
	}
	return out, nil
}

func deleteImportedPluginCanonicalFields(raw map[string]any) {
	delete(raw, "name")
	delete(raw, "version")
	delete(raw, "description")
	delete(raw, "author")
	delete(raw, "homepage")
	delete(raw, "repository")
	delete(raw, "license")
	delete(raw, "keywords")
	delete(raw, "skills")
	delete(raw, "mcpServers")
	delete(raw, "apps")
	delete(raw, "interface")
}
