package domain

import (
	"fmt"
	"regexp"
)

const Context7OAuthURL = "https://mcp.context7.com/mcp/oauth"
const Context7ChatGPTURL = "https://mcp.context7.com/mcp"
const ChatGPTRegistrationAction = "ChatGPT setup required:\n1. In Developer Mode, create and connect an app for https://mcp.context7.com/mcp with No authentication.\n2. Copy its asdk_app_ ID from app settings.\n3. Run: npx universal-agent-plugins add context7 --target chatgpt --chatgpt-app-id <ID>\n4. Install Context7 from your personal marketplace and select it in a new chat.\nGuide: https://developers.openai.com/plugins/build/plugins"

// ChatGPTMappedPreparationAction applies only after the personal mapping exists.
const ChatGPTMappedPreparationAction = "Install Context7 from your personal marketplace in ChatGPT, then select it in a new chat."

// ChatGPTPreparedAction names the local artifact after successful preparation.
func ChatGPTPreparedAction(path, name string) string {
	return fmt.Sprintf("ChatGPT: add the personal marketplace at %s (.agents/plugins/marketplace.json), install %s, then select it in a new chat.", path, name)
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

var chatGPTAppIDPattern = regexp.MustCompile(`^asdk_app_[0-9a-f]{32}$`)
var legacyChatGPTAppIDPattern = regexp.MustCompile(`^plugin_asdk_app_[0-9a-f]{32}$`)

func ValidateChatGPTAppID(id string) error {
	if !chatGPTAppIDPattern.MatchString(id) {
		return fmt.Errorf("invalid ChatGPT app ID: copy the asdk_app_ ID from ChatGPT app settings")
	}
	return nil
}

// IsLegacyContext7Registration identifies receipts written by the broken
// 0.1.56 OAuth guidance. They may be replaced only by an explicit new app ID.
func (m ChatGPTLocalMapping) IsLegacyContext7Registration() bool {
	return m.ProductID == "context7" && m.Repository == "upstash/context7" && m.PackagePath == "plugins/agent-plugins/context7" && m.Server == "context7" && m.URL == Context7OAuthURL && legacyChatGPTAppIDPattern.MatchString(m.AppID)
}

func (m ChatGPTLocalMapping) Validate() error {
	if m.ProductID != "context7" || m.Repository != "upstash/context7" || m.PackagePath != "plugins/agent-plugins/context7" || m.Server != "context7" || m.URL != Context7ChatGPTURL {
		return fmt.Errorf("personal ChatGPT mapping has a different Context7 identity or ChatGPT endpoint")
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
	m := ChatGPTLocalMapping{ProductID: "context7", Repository: "upstash/context7", PackagePath: "plugins/agent-plugins/context7", Server: "context7", URL: Context7ChatGPTURL}
	server, ok := e.MCP.Servers[m.Server]
	if e.Manifest.Name != m.ProductID || e.Source.Repository != m.Repository || e.Source.PackageSubpath != m.PackagePath || !e.MCP.Enabled || len(e.MCP.Servers) != 1 || !ok || server.Type != "streamable-http" || server.Decoded["url"] != Context7OAuthURL {
		return fmt.Errorf("personal ChatGPT mapping does not match the canonical Context7 package")
	}
	return nil
}
