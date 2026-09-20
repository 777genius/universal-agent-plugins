package publishschema

import (
	"path/filepath"
	"strings"
)

func normalizeCodexMarketplace(doc *CodexMarketplace) {
	doc.APIVersion = strings.TrimSpace(doc.APIVersion)
	doc.MarketplaceName = strings.TrimSpace(doc.MarketplaceName)
	doc.DisplayName = strings.TrimSpace(doc.DisplayName)
	doc.SourceRoot = normalizeSourceRoot(doc.SourceRoot)
	doc.Category = strings.TrimSpace(doc.Category)
	doc.InstallationPolicy = normalizePolicy(doc.InstallationPolicy, "AVAILABLE")
	doc.AuthenticationPolicy = normalizePolicy(doc.AuthenticationPolicy, "ON_INSTALL")
}

func normalizeClaudeMarketplace(doc *ClaudeMarketplace) {
	doc.APIVersion = strings.TrimSpace(doc.APIVersion)
	doc.MarketplaceName = strings.TrimSpace(doc.MarketplaceName)
	doc.OwnerName = strings.TrimSpace(doc.OwnerName)
	doc.SourceRoot = normalizeSourceRoot(doc.SourceRoot)
}

func normalizeGeminiGallery(doc *GeminiGallery) {
	doc.APIVersion = strings.TrimSpace(doc.APIVersion)
	doc.Distribution = strings.ToLower(strings.TrimSpace(doc.Distribution))
	if doc.Distribution == "" {
		doc.Distribution = "git_repository"
	}
	doc.RepositoryVisibility = strings.ToLower(strings.TrimSpace(doc.RepositoryVisibility))
	if doc.RepositoryVisibility == "" {
		doc.RepositoryVisibility = "public"
	}
	doc.GitHubTopic = strings.TrimSpace(doc.GitHubTopic)
	if doc.GitHubTopic == "" {
		doc.GitHubTopic = "gemini-cli-extension"
	}
	doc.ManifestRoot = strings.ToLower(strings.TrimSpace(doc.ManifestRoot))
	if doc.ManifestRoot == "" {
		doc.ManifestRoot = "repository_root"
	}
}

func normalizeSourceRoot(value string) string {
	value = strings.TrimSpace(value)
	switch value {
	case "", ".":
		return "./"
	default:
		if value == "./" {
			return value
		}
		return filepath.ToSlash(value)
	}
}

func normalizePolicy(value, fallback string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func setOf(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[strings.TrimSpace(value)] = true
	}
	return out
}
