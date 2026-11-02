// Package sourceacquisition resolves direct package sources into one sealed,
// digest-verified local snapshot. It never executes package content.
package sourceacquisition

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var (
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38}[A-Za-z0-9])?/[A-Za-z0-9](?:[A-Za-z0-9._-]{0,98}[A-Za-z0-9])?$`)
	fullSHA           = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

const (
	maxRootManifestMetadataBytes    = 4 << 10
	maxGitHubDiscoveryMetadataBytes = 1 << 20
)

type Command struct {
	Dir            string
	Args           []string
	Stdin          []byte
	MaxOutputBytes int
	// NativeCredentials is restricted to direct user-selected Git transport commands.
	NativeCredentials bool
	// mappedLocal is restricted to the explicit URLForRepo test fixture seam.
	mappedLocal bool
}

type Runner interface {
	Run(context.Context, Command) ([]byte, error)
}

type OSRunner struct {
	// ReportCleanupEvidence lets hosted callers refuse further launches after
	// uncertain cleanup without changing the default CLI error contract.
	ReportCleanupEvidence bool
}

func (runner OSRunner) Run(ctx context.Context, command Command) (body []byte, resultErr error) {
	if len(command.Stdin) != 0 {
		return nil, fmt.Errorf("git stdin is unsupported by the contained runner")
	}
	environment := isolatedGitEnvironment(os.Environ(), command.Dir)
	args := command.Args
	if command.NativeCredentials {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		environment = nativeGitEnvironment(os.Environ())
		environment = withBatchSSH(ctx, environment, command.Dir)
		options, policyErr := nativeVerificationOptions(ctx, environment, command)
		if policyErr != nil {
			return nil, policyErr
		}
		if command.mappedLocal {
			options = append(options, "-c", "protocol.file.allow=always")
			environment = append(environment, "GIT_ALLOW_PROTOCOL=https:ssh:file")
		}
		args = append(options, args...)
	}
	result, err := (processadapter.OS{}).RunWithTreeExitGrace(ctx, legacyports.Command{
		Argv: append([]string{"git"}, args...), Dir: command.Dir,
		Env: environment, StdoutLimitBytes: command.MaxOutputBytes,
	}, 5*time.Second)
	defer func() { resultErr = runner.withCleanupEvidence(result, resultErr) }()
	output := append(append([]byte(nil), result.Stdout...), result.Stderr...)
	if err != nil {
		if processadapter.IsOnlyStdoutLimitExceeded(err) ||
			processadapter.IsExactStdoutLimitExitCode(err, 141) {
			return nil, processadapter.ErrStdoutLimitExceeded
		}
		if command.NativeCredentials {
			return nil, fmt.Errorf("native Git transport failed: %w", ctxOrError(ctx, err))
		}
		return nil, fmt.Errorf("git %s failed: %w: %s", strings.Join(command.Args, " "), err, strings.TrimSpace(string(output)))
	}
	if result.ExitCode != 0 {
		return nil, gitTransportExitFailure(command, result.ExitCode, output)
	}
	// Blob callers require byte-exact stdout; diagnostic stderr is never content.
	return append([]byte(nil), result.Stdout...), nil
}

func isolatedGitEnvironment(environ []string, isolatedHome string) []string {
	clean := make([]string, 0, len(environ)+9)
	for _, item := range environ {
		key, _, _ := strings.Cut(item, "=")
		switch upper := strings.ToUpper(key); {
		case upper == "HOME", upper == "USERPROFILE", upper == "XDG_CONFIG_HOME":
			continue
		case upper == "NETRC", upper == "SSH_AUTH_SOCK", upper == "SSH_ASKPASS":
			continue
		case strings.HasPrefix(upper, "GIT_"):
			continue
		}
		clean = append(clean, item)
	}
	clean = append(clean,
		"HOME="+isolatedHome,
		"USERPROFILE="+isolatedHome,
		"XDG_CONFIG_HOME="+isolatedHome,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=credential.helper",
		"GIT_CONFIG_VALUE_0=",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_LFS_SKIP_SMUDGE=1",
	)
	return clean
}

type Acquirer struct {
	// HTTPClient is optional test/host transport configuration; redirects still obey archive policy.
	HTTPClient *http.Client
	TempRoot   string
	Runner     Runner
	Digester   packagedigest.Builder
	URLForRepo func(string) string
	Now        func() time.Time
}

// DiscoverGitHubPackages returns repository-relative package roots at an
// immutable revision without checking out or executing repository content.
// An empty string is returned when a portable or native package manifest exists
// at repository root; callers must then validate that root and must not silently
// fall back to a nested package when the root manifest is malformed.
func (acquirer Acquirer) DiscoverGitHubPackages(ctx context.Context, repository, revision string) ([]string, error) {
	if !repositoryPattern.MatchString(repository) {
		return nil, fmt.Errorf("github repository must be owner/repo")
	}
	if !fullSHA.MatchString(revision) {
		return nil, fmt.Errorf("immutable GitHub source requires a full 40-character lowercase commit SHA")
	}
	url := "https://github.com/" + repository + ".git"
	if acquirer.URLForRepo != nil {
		url = acquirer.URLForRepo(repository)
	}
	return acquirer.discoverGitPackages(ctx, url, repository, revision, false)
}

func (acquirer Acquirer) discoverGitPackages(ctx context.Context, url, repository, revision string, native bool) ([]string, error) {
	tempRoot, err := os.MkdirTemp(acquirer.TempRoot, "agentplugins-discovery-*")
	if err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "create temporary discovery workspace")
	}
	defer func() { _ = os.RemoveAll(tempRoot) }()
	repoRoot := filepath.Join(tempRoot, "repository")
	run := func(args ...string) ([]byte, error) {
		transport := native && len(args) > 2 && args[2] == "fetch"
		return acquirer.runner().Run(ctx, Command{Dir: tempRoot, Args: args, NativeCredentials: transport, mappedLocal: transport && acquirer.mappedLocalEndpoint(url)})
	}
	if _, err := run("init", "--quiet", repoRoot); err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "initialize discovery repository")
	}
	if _, err := run("-C", repoRoot, "remote", "add", "origin", url); err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "configure discovery remote")
	}
	for _, setting := range [][]string{{"fetch.recurseSubmodules", "false"}, {"core.autocrlf", "false"}} {
		if _, err := run("-C", repoRoot, "config", setting[0], setting[1]); err != nil {
			return nil, gitAcquisitionFailure(repository, revision, "configure discovery repository")
		}
	}
	fetchArgs := []string{"-C", repoRoot, "fetch", "--quiet", "--depth=1", "--no-tags", "--no-recurse-submodules", "origin", revision}
	if !native {
		fetchArgs = append(fetchArgs[:5], append([]string{"--filter=blob:none"}, fetchArgs[5:]...)...)
	}
	if _, err := run(fetchArgs...); err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "fetch immutable revision for package discovery")
	}
	resolved, err := run("-C", repoRoot, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil || strings.TrimSpace(string(resolved)) != revision {
		return nil, gitAcquisitionFailure(repository, revision, "verify immutable discovery revision")
	}
	rootTree, err := acquirer.runner().Run(ctx, Command{Dir: tempRoot,
		Args: []string{"-C", repoRoot, "ls-tree", "-z", revision, "--", "plugin.json", ".codex-plugin/plugin.json"}, MaxOutputBytes: maxRootManifestMetadataBytes})
	if err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "inspect repository root for Agent Plugins package")
	}
	rootPaths, err := packagePathsFromTree(rootTree)
	if err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "parse repository root package candidate")
	}
	if len(rootPaths) == 1 && rootPaths[0] == "" {
		return rootPaths, nil
	}
	tree, err := acquirer.runner().Run(ctx, Command{Dir: tempRoot,
		Args: []string{"-C", repoRoot, "ls-tree", "-r", "-z", revision}, MaxOutputBytes: maxGitHubDiscoveryMetadataBytes})
	if err != nil {
		if err == processadapter.ErrStdoutLimitExceeded {
			return nil, fmt.Errorf("repository package metadata exceeds the safe auto-discovery limit; choose a package explicitly with //path")
		}
		return nil, gitAcquisitionFailure(repository, revision, "inspect repository for Agent Plugins packages")
	}
	paths, err := packagePathsFromTree(tree)
	if err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "parse repository package candidates")
	}
	return paths, nil
}
