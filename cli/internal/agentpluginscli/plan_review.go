package agentpluginscli

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

const (
	defaultInstallReviewWidth = 92
	minimumInstallReviewWidth = 40

	catalogNotTestedWarning        = "catalog_not_tested"
	catalogRuntimeNotTestedWarning = "catalog_runtime_not_tested"
	verifySelectedClientAction     = "verify the plugin in the selected client before relying on it"
)

var (
	githubNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	gitCommitPattern  = regexp.MustCompile(`^(?:[0-9A-Fa-f]{40}|[0-9A-Fa-f]{64})$`)
)

type installReviewStyles struct {
	enabled                        bool
	title, label, success, warning lipgloss.Style
	failure                        lipgloss.Style
}

type installReviewRenderer struct {
	width  int
	styles installReviewStyles
}

type sharedInstallReview struct {
	catalogNotTested, runtimeNotTested, verifySelected bool
	warnings, actions                                  []string
}

func renderInstallReview(writer io.Writer, envelope domain.PackageEnvelope, results []usecase.AddResult) error {
	return renderInstallReviewAtWidth(writer, envelope, results, installReviewWidth(writer))
}

func renderInstallReviewAtWidth(writer io.Writer, envelope domain.PackageEnvelope, results []usecase.AddResult, width int) error {
	checked := &planWriter{writer: writer}
	renderer := newInstallReviewRenderer(width, terminaltheme.For(writer).Enabled)
	_, err := fmt.Fprintln(checked, renderer.render(envelope, results))
	if err != nil {
		return err
	}
	return checked.err
}

func newInstallReviewRenderer(width int, color bool) installReviewRenderer {
	width = max(minimumInstallReviewWidth, width)
	plain := lipgloss.NewStyle()
	styles := installReviewStyles{title: plain, label: plain, success: plain, warning: plain, failure: plain}
	if color {
		styles.enabled = true
		styles.title = plain.Bold(true)
		styles.label = plain.Bold(true).Foreground(terminaltheme.Color(terminaltheme.Label))
		styles.success = plain.Bold(true).Foreground(terminaltheme.Color(terminaltheme.Success))
		styles.warning = plain.Bold(true).Foreground(terminaltheme.Color(terminaltheme.Warning))
		styles.failure = plain.Bold(true).Foreground(terminaltheme.Color(terminaltheme.Error))
	}
	return installReviewRenderer{width: width, styles: styles}
}

func (r installReviewRenderer) render(envelope domain.PackageEnvelope, results []usecase.AddResult) string {
	results, shared := prepareInstallReview(results)
	var b strings.Builder
	b.WriteString(r.styles.title.Render("Install plan"))
	b.WriteString("\n\n")

	identity := strings.TrimSpace(prompt.SafeText(string(envelope.Manifest.Name)) + " " + prompt.SafeText(string(envelope.Manifest.Version)))
	if identity == "" {
		identity = "Plugin"
	}
	b.WriteString(r.styles.title.Render(identity))
	b.WriteString("\n")
	scope := reviewScope(results)
	b.WriteString(r.labeledLine("Scope", fmt.Sprintf("%s  ·  Clients %d", scope, len(results))))
	b.WriteString("\n")
	b.WriteString(r.labeledLine("Source", r.sourceValue(envelope.Source)))

	revision := shortReviewDigest(envelope.Source.ResolvedRevision)
	tree := shortReviewDigest(envelope.TreeDigest)
	if revision != "" || tree != "" {
		b.WriteString("\n")
		if r.width < 60 && revision != "" && tree != "" {
			b.WriteString(r.labeledLine("Commit", revision))
			b.WriteString("\n")
			b.WriteString(r.labeledLine("Tree", tree))
		} else {
			var provenance []string
			if revision != "" {
				provenance = append(provenance, r.styles.label.Render("Commit")+" "+revision)
			}
			if tree != "" {
				provenance = append(provenance, r.styles.label.Render("Tree")+" "+tree)
			}
			b.WriteString(r.wrap(strings.Join(provenance, "  ·  "), 0))
		}
	}

	for _, result := range results {
		b.WriteString("\n\n")
		b.WriteString(r.card(result))
	}
	if note := r.sharedNote(shared, len(results)); note != "" {
		b.WriteString("\n\n")
		b.WriteString(note)
	}
	return b.String()
}

func (r installReviewRenderer) sourceValue(source domain.SourceIdentity) string {
	target, label, ok := githubReviewLink(source)
	if !ok {
		if label != "" {
			return prompt.SafeText(label)
		}
		return prompt.SafeText(publicPackageSource(source))
	}
	if !r.styles.enabled {
		return target
	}
	return ansi.SetHyperlink(target) + prompt.SafeText(label) + " ↗" + ansi.ResetHyperlink()
}

func (r installReviewRenderer) labeledLine(label, value string) string {
	prefix := r.styles.label.Render(label) + " "
	available := max(8, r.width-ansi.StringWidth(prefix))
	wrapped := ansi.Wrap(value, available, " /")
	lines := strings.Split(wrapped, "\n")
	for index := range lines {
		if index == 0 {
			lines[index] = prefix + lines[index]
		} else {
			lines[index] = strings.Repeat(" ", ansi.StringWidth(prefix)) + lines[index]
		}
	}
	return strings.Join(lines, "\n")
}

func (r installReviewRenderer) card(result usecase.AddResult) string {
	plan := result.Plan
	outerWidth := r.width
	styleWidth := max(22, outerWidth-2)
	// Lip Gloss applies its own grapheme-aware reflow. Keep two spare cells so
	// styled labels and wide glyphs never force a surprise continuation row.
	innerWidth := max(18, styleWidth-4)

	status, statusStyle := reviewPlanStatus(plan, r.styles)
	clientName := reviewClientName(plan.ClientID)
	nameWidth := max(8, innerWidth-ansi.StringWidth(status)-2)
	clientName = ansi.Truncate(prompt.SafeText(clientName), nameWidth, "…")
	headerName := r.styles.title.Render(clientName)
	headerGap := max(1, innerWidth-ansi.StringWidth(headerName)-ansi.StringWidth(status))
	lines := []string{headerName + strings.Repeat(" ", headerGap) + statusStyle.Render(status)}

	lines = append(lines, r.cardLabeledRows("Target", prompt.SafeText(string(plan.ClientID)), innerWidth)...)
	packageValue := strings.TrimSuffix(reviewPackageMode(plan.PackageMode), " package")
	if components := reviewComponentSummary(plan.Components); components != "" {
		if packageValue != "" {
			packageValue += "  ·  "
		}
		packageValue += components
	}
	if packageValue != "" {
		lines = append(lines, r.cardLabeledRows("Package", packageValue, innerWidth)...)
	}
	lines = append(lines, r.cardLabeledRows("Authentication", reviewStateName(string(plan.Authentication)), innerWidth)...)
	lines = append(lines, r.cardLabeledRows("Verification", reviewStateName(string(plan.Verification)), innerWidth)...)

	for _, component := range plan.Components {
		componentLine := prompt.SafeText(component.Name) + "  ·  " + reviewSupport(component.Support)
		lines = append(lines, r.cardLabeledRows(reviewComponentKind(component.Kind), componentLine, innerWidth)...)
		if component.Reason != "" {
			lines = append(lines, r.cardLabeledRows("Reason", prompt.SafeText(component.Reason), innerWidth)...)
		}
	}

	diagnosticCodes := make(map[string]bool, len(plan.Diagnostics))
	for _, diagnostic := range plan.Diagnostics {
		diagnosticCodes[diagnostic.Code] = true
		diagnosticCodes[diagnostic.Message] = true
		message := prompt.SafeText(diagnostic.Message)
		if message == "" {
			message = prompt.SafeText(diagnostic.Code)
		}
		lines = append(lines, r.cardLabeledRows(r.diagnosticLabel(diagnostic.Severity), message, innerWidth)...)
		if diagnostic.Code != "" && diagnostic.Message != "" {
			lines = append(lines, r.cardLabeledRows("Code", prompt.SafeText(diagnostic.Code), innerWidth)...)
		}
	}
	for _, warning := range plan.Warnings {
		if diagnosticCodes[warning] {
			continue
		}
		lines = append(lines, r.cardLabeledRows("Warning", reviewWarningText(warning), innerWidth)...)
	}
	for _, action := range append(append([]string(nil), plan.UserActions...), plan.LocalActions...) {
		lines = append(lines, r.cardLabeledRows("Next", prompt.SafeText(action), innerWidth)...)
	}

	body := strings.Join(lines, "\n")
	style := lipgloss.NewStyle().Width(styleWidth).Padding(0, 1).Border(lipgloss.RoundedBorder())
	return style.Render(body)
}

func (r installReviewRenderer) cardLabeledRows(label, value string, width int) []string {
	styled := r.styles.label.Render(label) + ": "
	if label == "Warning" {
		styled = r.styles.warning.Render("!") + " " + r.styles.label.Render(label) + ": "
	} else if label == "Error" {
		styled = r.styles.failure.Render("✗") + " " + r.styles.label.Render(label) + ": "
	}
	prefixWidth := ansi.StringWidth(styled)
	wrapped := wrapReviewText(value, max(8, width-prefixWidth), 0)
	for index := range wrapped {
		if index == 0 {
			wrapped[index] = styled + wrapped[index]
		} else {
			wrapped[index] = strings.Repeat(" ", prefixWidth) + wrapped[index]
		}
	}
	return wrapped
}

func (r installReviewRenderer) diagnosticLabel(severity domain.Severity) string {
	if severity == domain.SeverityError {
		return "Error"
	}
	return "Warning"
}

func (r installReviewRenderer) sharedNote(shared sharedInstallReview, clients int) string {
	if !shared.catalogNotTested && !shared.runtimeNotTested && !shared.verifySelected && len(shared.warnings) == 0 && len(shared.actions) == 0 {
		return ""
	}
	var lines []string
	lines = append(lines, r.styles.warning.Render("! VERIFICATION"))
	switch {
	case shared.catalogNotTested && shared.runtimeNotTested:
		lines = append(lines, r.wrap("Catalog and runtime testing evidence is not published for this release.", 2))
	case shared.catalogNotTested:
		lines = append(lines, r.wrap("Catalog testing evidence is not published for this release.", 2))
	case shared.runtimeNotTested:
		lines = append(lines, r.wrap("Runtime testing evidence is not published for this release.", 2))
	}
	if shared.verifySelected {
		message := "Verify the plugin in the selected client before relying on it."
		if clients != 1 {
			message = "Verify the plugin once in each selected client before relying on it."
		}
		lines = append(lines, r.wrap(message, 2))
	}
	for _, warning := range shared.warnings {
		lines = append(lines, r.wrap("Warning: "+reviewWarningText(warning), 2))
	}
	for _, action := range shared.actions {
		lines = append(lines, r.wrap("Next for all: "+prompt.SafeText(action), 2))
	}
	return strings.Join(lines, "\n")
}

func (r installReviewRenderer) wrap(value string, indent int) string {
	return strings.Join(wrapReviewText(value, r.width, indent), "\n")
}

func prepareInstallReview(results []usecase.AddResult) ([]usecase.AddResult, sharedInstallReview) {
	prepared := make([]usecase.AddResult, len(results))
	for index, result := range results {
		result = withOpenCodeRuntimeNotice(result)
		result.Plan.Components = append([]domain.ComponentDecision(nil), result.Plan.Components...)
		result.Plan.Diagnostics = append([]domain.Diagnostic(nil), result.Plan.Diagnostics...)
		result.Plan.Warnings = uniqueReviewStrings(result.Plan.Warnings)
		result.Plan.UserActions = uniqueReviewStrings(result.Plan.UserActions)
		result.Plan.LocalActions = uniqueReviewStrings(result.Plan.LocalActions)
		prepared[index] = result
	}

	shared := sharedInstallReview{}
	for index := range prepared {
		var warnings []string
		for _, warning := range prepared[index].Plan.Warnings {
			switch warning {
			case catalogNotTestedWarning:
				shared.catalogNotTested = true
			case catalogRuntimeNotTestedWarning:
				shared.runtimeNotTested = true
			default:
				warnings = append(warnings, warning)
			}
		}
		prepared[index].Plan.Warnings = warnings

		var actions []string
		for _, action := range prepared[index].Plan.UserActions {
			if action == verifySelectedClientAction {
				shared.verifySelected = true
				continue
			}
			actions = append(actions, action)
		}
		prepared[index].Plan.UserActions = actions
	}

	if len(prepared) > 1 {
		shared.warnings = commonReviewStrings(prepared, func(result usecase.AddResult) []string { return result.Plan.Warnings })
		shared.actions = commonReviewStrings(prepared, func(result usecase.AddResult) []string { return result.Plan.UserActions })
		for index := range prepared {
			prepared[index].Plan.Warnings = removeReviewStrings(prepared[index].Plan.Warnings, shared.warnings)
			prepared[index].Plan.UserActions = removeReviewStrings(prepared[index].Plan.UserActions, shared.actions)
		}
	}
	return prepared, shared
}

func commonReviewStrings(results []usecase.AddResult, values func(usecase.AddResult) []string) []string {
	if len(results) == 0 {
		return nil
	}
	var common []string
	for _, candidate := range uniqueReviewStrings(values(results[0])) {
		present := true
		for _, result := range results[1:] {
			if !containsReviewString(values(result), candidate) {
				present = false
				break
			}
		}
		if present {
			common = append(common, candidate)
		}
	}
	return common
}

func uniqueReviewStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func removeReviewStrings(values, removed []string) []string {
	if len(removed) == 0 {
		return values
	}
	set := make(map[string]bool, len(removed))
	for _, value := range removed {
		set[value] = true
	}
	result := values[:0]
	for _, value := range values {
		if !set[value] {
			result = append(result, value)
		}
	}
	return result
}

func containsReviewString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func reviewPlanStatus(plan domain.DeliveryPlan, styles installReviewStyles) (string, lipgloss.Style) {
	if plan.Status == domain.PlanUnsupported || plan.Authentication == domain.AuthenticationFailed || plan.Verification == domain.VerificationFailed {
		return "✗ BLOCKED", styles.failure
	}
	if plan.Status == domain.PlanManualActivationRequired || plan.Status == domain.PlanPrepared || plan.Activation == domain.ActivationManual || plan.Activation == domain.ActivationPrepared {
		return "! MANUAL STEP", styles.warning
	}
	needsReview := plan.Authentication == domain.AuthenticationNotChecked || plan.Authentication == domain.AuthenticationPending || plan.Verification == domain.VerificationNotRun || len(plan.Diagnostics) > 0 || len(plan.Warnings) > 0
	for _, component := range plan.Components {
		needsReview = needsReview || component.Support == domain.SupportUnsupported
	}
	if needsReview {
		return "! REVIEW", styles.warning
	}
	return "✓ AUTO", styles.success
}

func reviewClientName(id domain.ClientID) string {
	if definition, ok := domain.ClientDefinitionFor(id); ok {
		return definition.DisplayName
	}
	return string(id)
}

func reviewScope(results []usecase.AddResult) string {
	if len(results) == 0 {
		return "user"
	}
	scope := string(results[0].Plan.Scope)
	if scope == "" {
		scope = "user"
	}
	for _, result := range results[1:] {
		candidate := string(result.Plan.Scope)
		if candidate == "" {
			candidate = "user"
		}
		if candidate != scope {
			return "mixed"
		}
	}
	return prompt.SafeText(scope)
}

func reviewPackageMode(mode domain.PackageMode) string {
	switch mode {
	case domain.PackageNative:
		return "Native package"
	case domain.PackageProjection:
		return "Projected package"
	case domain.PackageBridge:
		return "Client bridge"
	case domain.PackagePrepared:
		return "Prepared package"
	case "":
		return ""
	default:
		return reviewStateName(string(mode)) + " package"
	}
}

func reviewStateName(value string) string {
	if value == "" {
		return "not specified"
	}
	return strings.ReplaceAll(prompt.SafeText(value), "_", " ")
}

func reviewSupport(value domain.SupportLevel) string {
	return reviewStateName(string(value))
}

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
		if checked, ok := writer.(*planWriter); ok {
			writer = checked.writer
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
