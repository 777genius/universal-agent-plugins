package agentpluginscli

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	githubNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	gitCommitPattern  = regexp.MustCompile(`^(?:[0-9A-Fa-f]{40}|[0-9A-Fa-f]{64})$`)
)

func reviewWarningText(code string) string {
	if message, ok := map[string]string{
		"authentication_not_catalog_verified":                          "Authentication requirements are not verified by catalog evidence.",
		"authentication_requirement_unknown":                           "Authentication requirements are unknown.",
		"client_compatibility_not_catalog_verified":                    "Client compatibility is not verified by catalog evidence.",
		"components_skipped_local_readiness":                           "Some components were skipped because local requirements are not ready.",
		"no_supported_components":                                      "No supported components are available for this client.",
		"managed_component_removal_required":                           "Existing managed components must be removed before this plan can proceed.",
		"direct_source_digest_matches_known_revoked_directory_release": "This direct source matches a known revoked Directory release.",
		"catalog_client_unsupported":                                   "The catalog does not list this client as supported.",
		"catalog_package_mode_mismatch":                                "The catalog package mode does not match the generated package.",
		"trusted_claude_cli_required":                                  "A trusted Claude CLI installation is required.",
		"personal_registration_requires_account_install":               "Personal registration must be completed in the target account.",
		"chatgpt_app_binding_required":                                 "A registered ChatGPT app connection is required.",
		"windsurf_skills_prepared_only":                                "Windsurf skills are prepared for manual activation only.",
	}[code]; ok {
		return message
	}
	return reviewStateName(code)
}

func reviewComponentKind(kind domain.ComponentKind) string {
	switch kind {
	case domain.ComponentMCPServer:
		return "MCP server"
	case domain.ComponentSkill:
		return "Skill"
	case domain.ComponentApp:
		return "App"
	case domain.ComponentExtension:
		return "Extension"
	default:
		return reviewStateName(string(kind))
	}
}

func reviewComponentSummary(components []domain.ComponentDecision) string {
	if len(components) == 0 {
		return ""
	}
	counts := make(map[domain.ComponentKind]int)
	var order []domain.ComponentKind
	for _, component := range components {
		if counts[component.Kind] == 0 {
			order = append(order, component.Kind)
		}
		counts[component.Kind]++
	}
	var summary []string
	for _, kind := range order {
		count := counts[kind]
		label := reviewComponentKind(kind)
		if count != 1 {
			switch kind {
			case domain.ComponentMCPServer:
				label = "MCP servers"
			default:
				label = strings.ToLower(label) + "s"
			}
		}
		summary = append(summary, fmt.Sprintf("%d %s", count, label))
	}
	return strings.Join(summary, " · ")
}

func wrapReviewText(value string, width, indent int) []string {
	width = max(8, width)
	prefix := strings.Repeat(" ", indent)
	wrapped := ansi.Wrap(value, max(8, width-indent), " ")
	lines := strings.Split(wrapped, "\n")
	for index := range lines {
		lines[index] = prefix + lines[index]
	}
	return lines
}

func shortReviewDigest(value string) string {
	value = prompt.SafeText(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "sha256:")
	runes := []rune(value)
	if len(runes) > 12 {
		return string(runes[:12])
	}
	return value
}

func githubReviewLink(source domain.SourceIdentity) (target, label string, ok bool) {
	repository := strings.Trim(source.Repository, "/")
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || !githubNamePattern.MatchString(parts[0]) || !githubNamePattern.MatchString(parts[1]) || parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." {
		return "", "", false
	}

	labelParts := []string{"github.com", parts[0], parts[1]}
	if subpath := validGithubSubpath(source.PackageSubpath); len(subpath) > 0 {
		labelParts = append(labelParts, subpath...)
	}
	label = strings.Join(labelParts, "/")

	revision := strings.TrimSpace(source.ResolvedRevision)
	if !gitCommitPattern.MatchString(revision) {
		return "", label, false
	}
	pathParts := []string{parts[0], parts[1], "tree", revision}
	if subpath := validGithubSubpath(source.PackageSubpath); len(subpath) > 0 {
		pathParts = append(pathParts, subpath...)
	}

	targetURL := url.URL{Scheme: "https", Host: "github.com", Path: "/" + strings.Join(pathParts, "/")}
	return targetURL.String(), label, true
}

func validGithubSubpath(value string) []string {
	value = strings.Trim(value, "/")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\\\x00\r\n") {
			return nil
		}
	}
	return parts
}

func installReviewWidth(writer io.Writer) int {
	for {
		if sized, ok := writer.(interface{ InstallReviewWidth() int }); ok && sized.InstallReviewWidth() > 0 {
			return sized.InstallReviewWidth()
		}
		unwrappedPlanWriter := unwrapPlanWriters(writer)
		if unwrappedPlanWriter != writer {
			writer = unwrappedPlanWriter
			continue
		}
		unwrapped := terminaltheme.Unwrap(writer)
		if unwrapped != writer {
			writer = unwrapped
			continue
		}
		break
	}
	if file, ok := writer.(*os.File); ok {
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			return max(minimumInstallReviewWidth, width-1)
		}
	}
	return defaultInstallReviewWidth
}

func unwrapPlanWriters(writer io.Writer) io.Writer {
	for {
		checked, ok := writer.(*planWriter)
		if !ok {
			return writer
		}
		writer = checked.writer
	}
}
