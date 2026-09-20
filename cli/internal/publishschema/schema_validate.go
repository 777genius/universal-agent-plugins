package publishschema

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

var authPolicyRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func (doc CodexMarketplace) Validate() error {
	if doc.APIVersion != APIVersionV1 {
		return fmt.Errorf("api_version must be %q", APIVersionV1)
	}
	if err := scaffold.ValidateProjectName(doc.MarketplaceName); err != nil {
		return fmt.Errorf("invalid marketplace_name: %w", err)
	}
	if err := validateSourceRoot(doc.SourceRoot); err != nil {
		return err
	}
	if strings.TrimSpace(doc.Category) == "" {
		return fmt.Errorf("category required")
	}
	switch doc.InstallationPolicy {
	case "AVAILABLE", "INSTALLED_BY_DEFAULT", "NOT_AVAILABLE":
	default:
		return fmt.Errorf("installation_policy must be one of AVAILABLE, INSTALLED_BY_DEFAULT, NOT_AVAILABLE")
	}
	if !authPolicyRe.MatchString(doc.AuthenticationPolicy) {
		return fmt.Errorf("authentication_policy must use uppercase snake case")
	}
	return nil
}

func (doc ClaudeMarketplace) Validate() error {
	if doc.APIVersion != APIVersionV1 {
		return fmt.Errorf("api_version must be %q", APIVersionV1)
	}
	if err := scaffold.ValidateProjectName(doc.MarketplaceName); err != nil {
		return fmt.Errorf("invalid marketplace_name: %w", err)
	}
	if strings.TrimSpace(doc.OwnerName) == "" {
		return fmt.Errorf("owner_name required")
	}
	if err := validateSourceRoot(doc.SourceRoot); err != nil {
		return err
	}
	return nil
}

func (doc GeminiGallery) Validate() error {
	if doc.APIVersion != APIVersionV1 {
		return fmt.Errorf("api_version must be %q", APIVersionV1)
	}
	switch doc.Distribution {
	case "git_repository", "github_release":
	default:
		return fmt.Errorf("distribution must be one of git_repository, github_release")
	}
	if doc.RepositoryVisibility != "public" {
		return fmt.Errorf("repository_visibility must be %q", "public")
	}
	if doc.GitHubTopic != "gemini-cli-extension" {
		return fmt.Errorf("github_topic must be %q", "gemini-cli-extension")
	}
	switch doc.ManifestRoot {
	case "repository_root", "release_archive_root":
	default:
		return fmt.Errorf("manifest_root must be one of repository_root, release_archive_root")
	}
	if doc.Distribution == "git_repository" && doc.ManifestRoot != "repository_root" {
		return fmt.Errorf("manifest_root must be %q when distribution is %q", "repository_root", "git_repository")
	}
	return nil
}

func validateSourceRoot(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("source_root required")
	}
	if strings.HasPrefix(value, "/") {
		return fmt.Errorf("source_root must stay relative to the publication root")
	}
	if !strings.HasPrefix(value, "./") {
		return fmt.Errorf("source_root must start with ./")
	}
	trimmed := strings.TrimPrefix(value, "./")
	if trimmed == ".." || strings.HasPrefix(trimmed, "../") || strings.Contains(trimmed, "/../") {
		return fmt.Errorf("source_root may not escape the publication root")
	}
	return nil
}
