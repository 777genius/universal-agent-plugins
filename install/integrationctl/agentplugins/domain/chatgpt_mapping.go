package domain

import (
	"fmt"
	"regexp"
)

const Context7OAuthURL = "https://mcp.context7.com/mcp/oauth"
const ChatGPTRegistrationAction = "In ChatGPT Developer Mode, register a remote connection using https://mcp.context7.com/mcp/oauth, complete OAuth, and copy its plugin_asdk_app_ ID. Rerun add context7 --target chatgpt --prepare --chatgpt-app-id <ID>. Then install the prepared plugin from the personal marketplace in ChatGPT desktop and verify its tools in a new chat. Official steps: https://developers.openai.com/plugins/build/plugins. Remote connection and tool calls have not been verified. The personal registration receipt is retained locally after remove, including --purge-data, for reinstall."

// ChatGPTMappedPreparationAction applies only after the personal mapping exists.
const ChatGPTMappedPreparationAction = "Prepare the personal local marketplace, then install it in ChatGPT desktop and verify OAuth and tools in a new chat. Remote OAuth and tool calls have not been verified. The personal registration receipt is retained locally after remove, including --purge-data, for reinstall."

// ChatGPTPreparedAction names the local artifact after successful preparation.
func ChatGPTPreparedAction(path, name string) string {
	return fmt.Sprintf("In ChatGPT desktop, add the personal local marketplace at %s (.agents/plugins/marketplace.json), install %s, and verify the registered OAuth connection and tool calls in a new chat. Remote OAuth and tool calls have not been verified. The registration receipt is retained locally after remove, including --purge-data, for reinstall.", path, name)
}

// ChatGPTLocalMapping is a personal registration receipt, never catalog evidence.
// It is retained in private installer state across remove, including purge-data.
// Reinstallation revalidates its source and endpoint before projecting it.
type ChatGPTLocalMapping struct {
	ProductID   string `json:"product_id"`
	Repository  string `json:"repository"`
	PackagePath string `json:"package_path"`
	Server      string `json:"server"`
	URL         string `json:"url"`
	AppID       string `json:"app_id"`
}

var chatGPTAppIDPattern = regexp.MustCompile(`^plugin_asdk_app_[0-9a-f]{32}$`)

func ValidateChatGPTAppID(id string) error {
	if !chatGPTAppIDPattern.MatchString(id) {
		return fmt.Errorf("invalid ChatGPT app ID: copy the plugin_asdk_app_ ID from the official registration UI")
	}
	return nil
}
func (m ChatGPTLocalMapping) Validate() error {
	if m.ProductID != "context7" || m.Repository != "upstash/context7" || m.PackagePath != "plugins/agent-plugins/context7" || m.Server != "context7" || m.URL != Context7OAuthURL {
		return fmt.Errorf("personal ChatGPT mapping has a different Context7 identity or OAuth endpoint")
	}
	return ValidateChatGPTAppID(m.AppID)
}
func (m ChatGPTLocalMapping) ValidatePackage(e PackageEnvelope) error {
	if err := m.Validate(); err != nil {
		return err
	}
	return ValidateContext7PreparationPackage(e)
}

func ValidateContext7PreparationPackage(e PackageEnvelope) error {
	m := ChatGPTLocalMapping{ProductID: "context7", Repository: "upstash/context7", PackagePath: "plugins/agent-plugins/context7", Server: "context7", URL: Context7OAuthURL}
	server, ok := e.MCP.Servers[m.Server]
	if e.Manifest.Name != m.ProductID || e.Source.Repository != m.Repository || e.Source.PackageSubpath != m.PackagePath || !e.MCP.Enabled || len(e.MCP.Servers) != 1 || !ok || server.Type != "streamable-http" || server.Decoded["url"] != m.URL {
		return fmt.Errorf("personal ChatGPT mapping does not match canonical Context7 package and OAuth server")
	}
	return nil
}
