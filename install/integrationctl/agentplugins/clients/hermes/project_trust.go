package hermes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Hermes 0.23 keeps project trust as skills.trusted_project_dirs in the
// profile config that a bare `hermes` selects. A sticky active profile can
// redirect that choice, so UAP never edits the YAML itself and never uses
// `hermes config set`: only the user's own CLI writes it. That CLI exits 0 even
// when it saved nothing, so every precondition and an independent read-back
// must pass before UAP reports trust. Trust covers the whole Git root, is a
// persistent user preference and is never revoked by UAP removal.
const (
	hermesCompletedAction     = "Restart Hermes and review project trust before using installed Skills"
	hermesTrustCommandTimeout = 60 * time.Second
	hermesTrustOutputLimit    = 64 << 10
	hermesTrustFileLimit      = 1 << 20
)

// Pinned Hermes treats these HERMES_MANAGED values as an unmanaged install.
var hermesUnmanagedValues = map[string]bool{"": true, "false": true, "0": true, "no": true, "off": true, "brew": true, "homebrew": true}

type hermesTrustGrant struct {
	runner     ports.CommandRunner
	lookup     func(string) (string, error)
	kernel     nativeconfig.Kernel
	executable string
	// profile is the canonical Hermes home UAP selected for this user.
	profile string
	project string
	// environ is the environment the user's own Hermes runs with. UAP does not
	// select a named Hermes profile, so `-p` is never passed.
	environ []string
}

type hermesTrustReport struct {
	root      string
	granted   bool
	attempted bool
	reason    string
	missing   []string
}

// hermesTrustApplies selects confirmed Project activations whose Skills
// depend on Hermes project trust. Read-only verification never qualifies.
func hermesTrustApplies(req domain.ActivationRequest) bool {
	return !req.VerifyOnly && req.Plan.Scope == domain.ScopeProject &&
		req.Plan.InstallIntent == domain.InstallIntentAutomatic &&
		shared.ComponentKindPresent(req.Plan.Components, domain.ComponentSkill)
}

// completeHermesProjectTrust runs only after a confirmed activation installed
// the exact project Skills. Its failure never undoes those files: the result
// either reports verified trust or the manual command for the user to run.
// Only an explicit add carries consent; update, repair and dependency sync
// must not override a user's `hermes skills untrust`.
func completeHermesProjectTrust(ctx context.Context, env clients.Env, req domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	project := req.Plan.TargetContext.ProjectRoot
	report := hermesTrustReport{root: project, reason: "UAP grants it only during an explicit add, so update, repair and dependency sync never override `hermes skills untrust`"}
	var err error
	switch {
	case !req.ProjectTrustConsent:
	case req.Plan.TargetContext.UserConfigRoot != "" && req.Plan.TargetContext.UserConfigRoot != req.Client.ConfigRoot:
		report.reason = "the project's recorded Hermes profile differs from the detected Hermes profile"
	default:
		report, err = hermesTrustGrant{
			runner: env.Runner, lookup: env.LookupExecutable, kernel: env.NativeConfig,
			executable: req.Client.ExecutablePath, profile: req.Client.ConfigRoot,
			project: project, environ: os.Environ(),
		}.run(ctx)
	}
	actions := outcome.UserActions[:0]
	for _, action := range outcome.UserActions {
		if action != hermesCompletedAction {
			actions = append(actions, action)
		}
	}
	user, local := report.actions()
	actions = append(actions, user)
	outcome.UserActions = actions
	outcome.LocalActions = append(outcome.LocalActions, local...)
	if err != nil && domain.IsUnclosedClientProbe(err) {
		outcome.NativeEffect = domain.NativeEffectUncertain
	}
	return outcome, err
}

// actions keeps public JSON path-free. Host paths and the exact command go to
// LocalActions, which only human output renders.
func (report hermesTrustReport) actions() (string, []string) {
	if report.granted {
		return "Hermes now trusts this project's Git repository root for Skills in .hermes/skills and .agents/skills, including Skills added later, until `hermes skills untrust`; Skills that Hermes skills_guard flags as dangerous stay quarantined and do not load. Start a new Hermes session to load them; installed files alone do not prove discovery",
			[]string{"Hermes trusted Git repository root: " + report.root}
	}
	prefix := "UAP did not change Hermes project trust: "
	if report.attempted {
		prefix = "UAP could not confirm Hermes project trust: "
	}
	local := []string{hermesTrustCommandAction(report.root)}
	if len(report.missing) > 0 {
		local = append(local, "Previously trusted Hermes entries missing afterwards: "+strings.Join(report.missing, ", "))
	}
	return prefix + report.reason + ". To trust the whole Git repository (.hermes/skills and .agents/skills, including future content), run `hermes skills trust` inside it, then start a new Hermes session", local
}

// hermesTrustCommandAction prints a runnable command only for a root that no
// supported shell can expand or split. Any other root is shown as quoted data,
// never as a command a user could copy and run.
func hermesTrustCommandAction(root string) string {
	if command, ok := hermesTrustCommand(root); ok {
		return "Exact Hermes trust command for this project: " + command
	}
	return "The repository path contains shell-sensitive characters, so no command is printed; run `hermes skills trust` from inside the repository, then start a new Hermes session. Repository path (not a command): " + strconv.Quote(root)
}

// POSIX shells take single quotes. On Windows, double quotes work in both
// PowerShell and cmd.exe because a safe root has no $, backtick, % or quote.
func hermesTrustCommand(root string) (string, bool) {
	if !hermesSafeCommandPath(root) {
		return "", false
	}
	if runtime.GOOS == "windows" {
		return `hermes skills trust "` + root + `"`, true
	}
	return "hermes skills trust '" + root + "'", true
}

// hermesSafeCommandPath admits letters (including non-ASCII), digits, space
// and `/ \ : . _ - + @ ,` only, and refuses a leading dash that could read as
// a flag. Canonical absolute roots begin with a separator or a drive letter.
func hermesSafeCommandPath(root string) bool {
	if root == "" || strings.HasPrefix(root, "-") {
		return false
	}
	for _, r := range root {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune(` /\:._-+@,`, r) {
			return false
		}
	}
	return true
}

// run returns an error only when a native process was not proven closed or the
// caller canceled; every other failure is a reported, unclaimed grant.
func (grant hermesTrustGrant) run(ctx context.Context) (hermesTrustReport, error) {
	report := hermesTrustReport{root: grant.project}
	refuse := func(reason string) (hermesTrustReport, error) {
		report.reason = reason
		return report, nil
	}
	if grant.runner == nil || grant.lookup == nil {
		return refuse("the native Hermes command runner is unavailable")
	}
	if !cleanRoot(grant.project) {
		return refuse("the selected project is not an exact absolute directory")
	}
	if !cleanRoot(grant.profile) {
		return refuse("the detected Hermes home is not an exact absolute directory")
	}
	if !cleanRoot(grant.executable) || runtime.GOOS == "windows" && !strings.EqualFold(filepath.Ext(grant.executable), ".exe") {
		return refuse("the Hermes CLI executable was not detected exactly")
	}
	values := hermesEnvironmentValues(grant.environ)
	if reason := hermesTrustPolicyRefusal(grant.profile, grant.project, values); reason != "" {
		return refuse(reason)
	}
	root, reason, err := grant.gitRoot(ctx, values)
	if err != nil || reason != "" {
		report.reason = reason
		return report, err
	}
	report.root = root
	file, err := grant.kernel.BeginExactFile(filepath.Join(grant.profile, "config.yaml"))
	if err != nil {
		return refuse("UAP could not take its exclusive lock on the Hermes config.yaml, which must be a regular file, not a symlink, that no other UAP operation is changing")
	}
	report, err = grant.trustLocked(ctx, report)
	if closeErr := file.Close(); closeErr != nil && report.granted {
		report.granted, report.reason = false, "the Hermes configuration lock was not released cleanly"
	}
	return report, err
}

func (grant hermesTrustGrant) trustLocked(ctx context.Context, report hermesTrustReport) (hermesTrustReport, error) {
	path, err := grant.hermes(ctx, "config", "path")
	if err != nil || path == nil {
		report.reason = "Hermes did not report its configuration path"
		return report, err
	}
	if reason := grant.checkConfigPath(path.Stdout); reason != "" {
		report.reason = reason
		return report, nil
	}
	before, reason, err := grant.trustedDirs(ctx)
	if err != nil || reason != "" {
		report.reason = "the existing Hermes trust list could not be read"
		return report, err
	}
	report.attempted = true
	result, err := grant.hermes(ctx, "skills", "trust", report.root)
	if err != nil {
		report.reason = "the Hermes trust command did not complete"
		return report, err
	}
	failure := ""
	if result == nil {
		failure = "the Hermes trust command did not complete"
	} else if !hermesTrustConfirmed(*result, report.root) {
		failure = "Hermes did not confirm the trust grant"
	}
	// Read back even after a refused or unconfirmed grant so entries a managed
	// save deleted are reported to the user.
	report, err = grant.verifyTrust(ctx, report, before)
	if failure != "" {
		report.granted, report.reason = false, failure
	}
	return report, err
}

// The pinned CLI exits 0 for refused or unsaved grants and reports the real
// outcome only in its output, so require the exact confirmation line.
func hermesTrustConfirmed(result legacyports.CommandResult, root string) bool {
	line, _, _ := strings.Cut(string(result.Stdout), "\n")
	line = strings.TrimSuffix(line, "\r")
	return len(result.Stderr) == 0 && (line == "Trusted: "+root || line == "Already trusted: "+root)
}

// verifyTrust independently reads the saved list back: it must hold the root
// and every earlier entry, and project discovery must still be enabled.
func (grant hermesTrustGrant) verifyTrust(ctx context.Context, report hermesTrustReport, before []string) (hermesTrustReport, error) {
	after, reason, err := grant.trustedDirs(ctx)
	if err != nil || reason != "" {
		report.reason = "the Hermes trust list could not be read back"
		return report, err
	}
	for _, entry := range before {
		if !slices.Contains(after, entry) {
			report.missing = append(report.missing, entry)
		}
	}
	if len(report.missing) > 0 || !slices.Contains(after, report.root) {
		report.reason = "the Hermes trust list does not contain the expected entries afterwards"
		return report, nil
	}
	discovery, err := grant.hermes(ctx, "config", "get", "--json", "skills.project_discovery")
	if err != nil || discovery == nil {
		report.reason = "Hermes project Skill discovery could not be read"
		return report, err
	}
	value, ok := hermesSingleJSON(discovery.Stdout)
	if !ok {
		report.reason = "Hermes project Skill discovery could not be read"
		return report, nil
	}
	if enabled, isBool := value.(bool); isBool && !enabled {
		report.reason = "Hermes project Skill discovery is disabled (skills.project_discovery is false)"
		return report, nil
	}
	report.granted, report.reason = true, ""
	return report, nil
}

// hermes runs one bounded CLI invocation. A nil result with a nil error is a
// closed but unusable invocation; an error is an unclosed process or cancel.
func (grant hermesTrustGrant) hermes(ctx context.Context, args ...string) (*legacyports.CommandResult, error) {
	return grant.command(ctx, append([]string{grant.executable}, args...))
}

func (grant hermesTrustGrant) command(ctx context.Context, argv []string) (*legacyports.CommandResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, hermesTrustCommandTimeout)
	defer cancel()
	result, err := shared.RunNativeRegistry(bounded, grant.runner, legacyports.Command{
		Argv: argv, Dir: grant.project, Env: grant.environment(), StdoutLimitBytes: hermesTrustOutputLimit,
	})
	if domain.IsUnclosedClientProbe(err) {
		return nil, err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil || result.ExitCode != 0 || len(result.Stdout) >= hermesTrustOutputLimit {
		return nil, nil
	}
	return &result, nil
}

// The user's unchanged environment selects the same Hermes home, .env layers
// and backend a bare `hermes` uses. Pinned Hermes output ignores color and TERM.
func (grant hermesTrustGrant) environment() []string {
	return slices.Clone(grant.environ)
}

func hermesEnvironmentValues(environ []string) map[string]string {
	values := make(map[string]string, len(environ))
	for _, entry := range environ {
		key, value, _ := strings.Cut(entry, "=")
		if runtime.GOOS == "windows" {
			key = strings.ToUpper(key)
		}
		values[key] = value
	}
	return values
}

// Managed, container and alternate-backend installs either ignore or discard a
// CLI trust write. Any managed config.yaml makes every Hermes save delete each
// administrator-pinned key from the user's file, so its presence refuses too.
func hermesTrustPolicyRefusal(profile, project string, values map[string]string) string {
	if value, ok := values["HERMES_MANAGED"]; ok && !hermesUnmanagedValues[strings.ToLower(strings.TrimSpace(value))] {
		return "Hermes is managed by HERMES_MANAGED"
	}
	for _, marker := range []string{".managed", ".container-mode"} {
		if _, err := os.Lstat(filepath.Join(profile, marker)); !os.IsNotExist(err) {
			return "the Hermes home has a " + marker + " marker"
		}
	}
	managed := filepath.Join(filepath.VolumeName(project)+string(os.PathSeparator), "etc", "hermes")
	if value := strings.TrimSpace(values["HERMES_MANAGED_DIR"]); value != "" {
		if !filepath.IsAbs(value) {
			return "HERMES_MANAGED_DIR is not an absolute directory"
		}
		managed = filepath.Clean(value)
	}
	if hermesNonFileBackend(values["HERMES_CONFIG_BACKEND"]) {
		return "Hermes uses a non-file configuration backend"
	}
	for _, path := range []string{filepath.Join(profile, ".env"), filepath.Join(profile, ".op.env"), filepath.Join(managed, ".env")} {
		body, err := readHermesTrustFile(path)
		if err != nil {
			return "a Hermes environment file could not be inspected"
		}
		if hermesDotenvSelectsBackend(body) {
			return "Hermes uses a non-file configuration backend"
		}
	}
	body, err := readHermesTrustFile(filepath.Join(managed, "config.yaml"))
	if err != nil {
		return "the managed Hermes configuration could not be inspected"
	}
	if len(bytes.TrimSpace(body)) != 0 {
		return "a managed Hermes configuration would remove administrator-pinned keys from the user configuration on save"
	}
	return ""
}

func hermesNonFileBackend(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value != "" && value != "file"
}

// Any assignment that is not exactly the file backend refuses, including
// unparseable quoting; a false refusal only prints the manual command.
func hermesDotenvSelectsBackend(body []byte) bool {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "HERMES_CONFIG_BACKEND") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return true
		}
		if strings.TrimSpace(key) != "HERMES_CONFIG_BACKEND" {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if hermesNonFileBackend(value) {
			return true
		}
	}
	return false
}

// readHermesTrustFile treats absence as empty. Anything else that cannot be
// read as one bounded regular file is an error, never permission.
func readHermesTrustFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > hermesTrustFileLimit {
		return nil, fmt.Errorf("hermes trust policy input is not a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(file, hermesTrustFileLimit+1))
	if err := errors.Join(readErr, file.Close()); err != nil || len(body) > hermesTrustFileLimit {
		return nil, fmt.Errorf("hermes trust policy input could not be read")
	}
	return body, nil
}

// gitRoot returns the absolute physical root Git and Hermes both select. It
// must be the nearest ancestor holding .git, never the home or volume root.
func (grant hermesTrustGrant) gitRoot(ctx context.Context, values map[string]string) (string, string, error) {
	const unconfirmed = "the project Git repository root could not be confirmed"
	marker := nearestGitAncestor(grant.project)
	if marker == "" {
		return "", unconfirmed, nil
	}
	git := grant.lookupGit()
	if git == "" {
		return "", "Git is unavailable to confirm the project repository root", nil
	}
	result, err := grant.command(ctx, []string{git, "-C", grant.project, "rev-parse", "--show-toplevel"})
	if err != nil || result == nil {
		return "", unconfirmed, err
	}
	line := strings.TrimSuffix(strings.TrimSuffix(string(result.Stdout), "\n"), "\r")
	root := filepath.FromSlash(line)
	if !cleanRoot(root) || strings.ContainsAny(line, "\r\n") || strings.IndexFunc(line, unicode.IsControl) >= 0 ||
		!physicalDirectory(root) || !sameDirectory(root, marker) {
		return "", unconfirmed, nil
	}
	home := hermesHome(values)
	if home == "" {
		return "", "the home directory cannot be determined, so Hermes trust could cover it", nil
	}
	if sameDirectory(root, home) || containsDirectory(root, home) {
		return "", "Hermes trust would cover the entire home directory", nil
	}
	return root, "", nil
}

// hermesHome is the existing directory Hermes treats as Path.home(), or empty
// when the environment does not name one exactly.
func hermesHome(values map[string]string) string {
	home := values["HOME"]
	if runtime.GOOS == "windows" {
		home = values["USERPROFILE"]
	}
	if !cleanRoot(home) {
		return ""
	}
	if info, err := os.Stat(home); err != nil || !info.IsDir() {
		return ""
	}
	return home
}

// containsDirectory reports whether root is a physical ancestor of path.
func containsDirectory(root, path string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(root, resolved)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (grant hermesTrustGrant) lookupGit() string {
	git, err := grant.lookup("git")
	if err != nil || !cleanRoot(git) {
		return ""
	}
	return git
}

// nearestGitAncestor mirrors Hermes' project root walk, refusing a symlinked
// or special .git marker. An empty result means no safe marker exists.
func nearestGitAncestor(project string) string {
	for directory := project; ; directory = filepath.Dir(directory) {
		info, err := os.Lstat(filepath.Join(directory, ".git"))
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
				return ""
			}
			return directory
		}
		if !os.IsNotExist(err) || filepath.Dir(directory) == directory {
			return ""
		}
	}
}

func physicalDirectory(path string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	return err == nil && resolved == path
}

func sameDirectory(left, right string) bool {
	a, errA := os.Stat(left)
	b, errB := os.Stat(right)
	return errA == nil && errB == nil && a.IsDir() && b.IsDir() && os.SameFile(a, b)
}

// A different directory means a sticky active profile or another selector
// redirects a bare `hermes`; its write would land outside the selected home.
func (grant hermesTrustGrant) checkConfigPath(stdout []byte) string {
	const differs = "Hermes selects a different configuration than UAP (check the Hermes active profile)"
	line := strings.TrimSuffix(strings.TrimSuffix(string(stdout), "\n"), "\r")
	path := filepath.Clean(line)
	if line == "" || strings.ContainsAny(line, "\r\n") || !filepath.IsAbs(line) || filepath.Base(path) != "config.yaml" {
		return differs
	}
	canonical, err := clients.CanonicalDirectory(filepath.Dir(path), os.Lstat, filepath.EvalSymlinks)
	if err != nil || canonical != grant.profile {
		return differs
	}
	return ""
}

func (grant hermesTrustGrant) trustedDirs(ctx context.Context) ([]string, string, error) {
	result, err := grant.hermes(ctx, "config", "get", "--json", "skills.trusted_project_dirs")
	if err != nil || result == nil {
		return nil, "unreadable", err
	}
	value, ok := hermesSingleJSON(result.Stdout)
	if !ok {
		return nil, "unreadable", nil
	}
	switch typed := value.(type) {
	case nil:
		return []string{}, "", nil
	case string:
		return []string{typed}, "", nil
	case []any:
		entries := make([]string, 0, len(typed))
		for _, item := range typed {
			entry, isString := item.(string)
			if !isString {
				return nil, "unreadable", nil
			}
			entries = append(entries, entry)
		}
		return entries, "", nil
	default:
		return nil, "unreadable", nil
	}
}

func hermesSingleJSON(body []byte) (any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return nil, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, false
	}
	return value, true
}
