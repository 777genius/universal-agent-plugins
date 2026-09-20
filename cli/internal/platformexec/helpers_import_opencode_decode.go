package platformexec

func decodeImportedOpenCodeConfig(body []byte) (importedOpenCodeConfig, error) {
	raw, err := decodeImportedOpenCodeConfigRaw(body)
	if err != nil {
		return importedOpenCodeConfig{}, err
	}
	out := importedOpenCodeConfig{}
	if err := decodeOpenCodePluginField(raw, &out); err != nil {
		return importedOpenCodeConfig{}, err
	}
	if err := decodeOpenCodeObjectField(raw, "mcp", &out.MCPProvided, &out.MCP); err != nil {
		return importedOpenCodeConfig{}, err
	}
	if err := decodeOpenCodeObjectField(raw, "command", &out.CommandsProvided, &out.Commands); err != nil {
		return importedOpenCodeConfig{}, err
	}
	if err := decodeOpenCodeObjectField(raw, "agent", &out.AgentsProvided, &out.Agents); err != nil {
		return importedOpenCodeConfig{}, err
	}
	if err := decodeOpenCodeDefaultAgentField(raw, &out); err != nil {
		return importedOpenCodeConfig{}, err
	}
	if err := decodeOpenCodeInstructionsField(raw, &out); err != nil {
		return importedOpenCodeConfig{}, err
	}
	if err := decodeOpenCodePermissionField(raw, &out); err != nil {
		return importedOpenCodeConfig{}, err
	}
	if err := mergeOpenCodeLegacyModeField(raw, &out); err != nil {
		return importedOpenCodeConfig{}, err
	}
	out.Extra = extractOpenCodeExtra(raw)
	return out, nil
}
