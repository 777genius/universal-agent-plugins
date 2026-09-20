package pluginmodel

func (m *PortableMCP) RenderForTarget(target string) (map[string]any, error) {
	return renderPortableMCPForTarget(m, target)
}

func (file PortableMCPFile) RenderLegacyProjection(target string) map[string]any {
	return renderPortableMCPLegacyProjection(file, target)
}

func (server PortableMCPServer) appliesTo(target string) bool {
	return portableMCPServerAppliesTo(server, target)
}

func (server PortableMCPServer) generate(target string) map[string]any {
	return generatePortableMCPServer(server, target)
}

func renderPortableMCPStdio(target string, stdio *PortableMCPStdio) map[string]any {
	return renderPortableMCPStdioTransport(target, stdio)
}

func renderPortableMCPRemote(target string, remote *PortableMCPRemote) map[string]any {
	return renderPortableMCPRemoteTransport(target, remote)
}

func translatePortableMCPValue(target string, value any) any {
	return translatePortableMCPProjectionValue(target, value)
}

func translatePortableMCPString(target, value string) string {
	return translatePortableMCPProjectionString(target, value)
}

func portableMCPVariableReplacements(target string) map[string]string {
	return portableMCPProjectionVariableReplacements(target)
}

func mustPortableMCPMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}
