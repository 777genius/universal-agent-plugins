package platformexec

import "fmt"

func validateGeminiMCPTransport(path, serverName string, server map[string]any) (string, bool, []Diagnostic) {
	command, hasCommand := geminiOptionalString(server["command"])
	_, hasURL := geminiOptionalString(server["url"])
	_, hasHTTPURL := geminiOptionalString(server["httpUrl"])
	if countTruthy(hasCommand, hasURL, hasHTTPURL) == 1 {
		return command, hasCommand, nil
	}
	return command, hasCommand, []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeManifestInvalid,
		Path:     path,
		Target:   "gemini",
		Message:  fmt.Sprintf("Gemini extension MCP server %q must define exactly one transport via command, url, or httpUrl", serverName),
	}}
}

func countTruthy(values ...bool) int {
	total := 0
	for _, value := range values {
		if value {
			total++
		}
	}
	return total
}
