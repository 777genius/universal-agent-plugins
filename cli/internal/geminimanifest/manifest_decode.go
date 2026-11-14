package geminimanifest

import "fmt"

func decodeImportedExtensionObject(raw map[string]any) (ImportedExtension, error) {
	out := ImportedExtension{}

	name, err := optionalNonEmptyStringField(raw, "name")
	if err != nil {
		return ImportedExtension{}, err
	}
	out.Name = name

	version, err := optionalNonEmptyStringField(raw, "version")
	if err != nil {
		return ImportedExtension{}, err
	}
	out.Version = version

	description, err := optionalNonEmptyStringField(raw, "description")
	if err != nil {
		return ImportedExtension{}, err
	}
	out.Description = description

	contextFileName, err := optionalNonEmptyStringField(raw, "contextFileName")
	if err != nil {
		return ImportedExtension{}, err
	}
	out.Meta.ContextFileName = contextFileName

	migratedTo, err := optionalNonEmptyStringField(raw, "migratedTo")
	if err != nil {
		return ImportedExtension{}, err
	}
	out.Meta.MigratedTo = migratedTo

	if servers, ok := raw["mcpServers"]; ok {
		value, ok := servers.(map[string]any)
		if !ok {
			return ImportedExtension{}, fmt.Errorf("Gemini extension field %q must be a JSON object", "mcpServers")
		}
		if len(value) > 0 {
			out.MCPServers = value
		}
	}

	if values, ok := raw["excludeTools"]; ok {
		items, err := stringArrayField(values, "excludeTools")
		if err != nil {
			return ImportedExtension{}, err
		}
		out.Meta.ExcludeTools = items
	}

	if err := decodeImportedPlan(raw, &out); err != nil {
		return ImportedExtension{}, err
	}
	if err := decodeImportedSettings(raw, &out); err != nil {
		return ImportedExtension{}, err
	}
	if err := decodeImportedThemes(raw, &out); err != nil {
		return ImportedExtension{}, err
	}

	deleteImportedCanonicalFields(raw)
	if len(raw) > 0 {
		out.Extra = raw
	}
	return out, nil
}

func decodeImportedPlan(raw map[string]any, out *ImportedExtension) error {
	planValue, ok := raw["plan"]
	if !ok {
		return nil
	}
	plan, ok := planValue.(map[string]any)
	if !ok {
		return fmt.Errorf("Gemini extension field %q must be a JSON object", "plan")
	}
	if directory, ok := plan["directory"]; ok {
		text, ok := directory.(string)
		if !ok || stringsTrimSpace(text) == "" {
			return fmt.Errorf("Gemini extension field %q must be a non-empty string when provided", "plan.directory")
		}
		out.Meta.PlanDirectory = text
		delete(plan, "directory")
	}
	if len(plan) == 0 {
		delete(raw, "plan")
	} else {
		raw["plan"] = plan
	}
	return nil
}

func decodeImportedSettings(raw map[string]any, out *ImportedExtension) error {
	values, ok := raw["settings"]
	if !ok {
		return nil
	}
	items, err := objectArrayField(values, "settings")
	if err != nil {
		return err
	}
	for i, item := range items {
		doc, _ := item.(map[string]any)
		if err := validateImportedSettingObject(doc); err != nil {
			return fmt.Errorf("Gemini extension field %q contains an invalid object at index %d: %w", "settings", i, err)
		}
	}
	out.Settings = items
	return nil
}

func decodeImportedThemes(raw map[string]any, out *ImportedExtension) error {
	values, ok := raw["themes"]
	if !ok {
		return nil
	}
	items, err := objectArrayField(values, "themes")
	if err != nil {
		return err
	}
	for i, item := range items {
		doc, _ := item.(map[string]any)
		if err := validateImportedThemeObject(doc); err != nil {
			return fmt.Errorf("Gemini extension field %q contains an invalid object at index %d: %w", "themes", i, err)
		}
	}
	out.Themes = items
	return nil
}

func deleteImportedCanonicalFields(raw map[string]any) {
	delete(raw, "name")
	delete(raw, "version")
	delete(raw, "description")
	delete(raw, "mcpServers")
	delete(raw, "contextFileName")
	delete(raw, "excludeTools")
	delete(raw, "migratedTo")
	delete(raw, "settings")
	delete(raw, "themes")
	if plan, ok := raw["plan"].(map[string]any); ok && len(plan) == 0 {
		delete(raw, "plan")
	}
}
