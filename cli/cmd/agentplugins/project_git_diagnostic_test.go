package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

var projectGitDiagnosticLabels = []string{"init", "remote-add", "config", "ls-remote", "check-ref-format", "fetch", "rev-parse", "ls-tree", "sparse-init", "sparse-set", "checkout", "show", "unknown"}
var projectGitDiagnosticURI = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s"'<>]+`)
var projectGitDiagnosticCredential = regexp.MustCompile(`(?i)(authorization|proxy-authorization|password|passwd|token|secret|cookie|credential)[ \t]*[:=]`)
var projectGitDiagnosticSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

type projectGitDiagnostic struct {
	mu                 sync.Mutex
	counts             map[string]uint16
	failures           uint64
	logged             int
	lastConfiguredRepo string
	endpoint           string
	log                func(string)
}

type projectGitDiagnosticRunner struct {
	recorder *projectGitDiagnostic
	root     string
	delegate sourceacquisition.Runner
}

func (r projectGitDiagnosticRunner) Run(ctx context.Context, command sourceacquisition.Command) ([]byte, error) {
	delegate := r.delegate
	if delegate == nil {
		delegate = sourceacquisition.OSRunner{}
	}
	body, err := delegate.Run(ctx, command)
	phase, known, endpointExplicit := projectGitDiagnosticOperation(command, r.recorder.endpoint)
	d := r.recorder
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.counts[phase] < 65535 {
		d.counts[phase]++
	}
	if !known {
		d.lastConfiguredRepo = ""
	}
	owned := projectGitDiagnosticWorkspace(r.root, command.Dir)
	repo := filepath.Join(command.Dir, "repository")
	if phase == "remote-add" {
		d.lastConfiguredRepo = ""
		if err == nil && owned && projectGitDiagnosticEndpoint(d.endpoint) {
			d.lastConfiguredRepo = repo
		}
	}
	if err == nil {
		return body, nil
	}
	d.failures++
	if d.logged >= 4 {
		return body, err
	}
	d.logged++
	cause := "cause_omitted"
	// Non-native OSRunner errors include argv/streams: never inspect Error().
	if command.NativeCredentials && known && owned && projectGitDiagnosticEndpoint(d.endpoint) && (endpointExplicit || phase == "fetch" && d.lastConfiguredRepo == repo) {
		cause = projectGitDiagnosticCause(err, d.endpoint, r.root, command.Dir, repo)
	}
	d.log(fmt.Sprintf("project Git diagnostic phase=%s class=%s cause=%s counts=%s", phase, projectGitDiagnosticClass(err), cause, d.countSnapshot()))
	return body, err
}

func (d *projectGitDiagnostic) failureSequence() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.failures
}

func (d *projectGitDiagnostic) outsideInvocations(sequence uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failures != sequence || d.logged >= 4 {
		return
	}
	d.logged++
	d.log("project Git diagnostic: no observed Runner failure; remaining source phase undetermined; counts=" + d.countSnapshot())
}

func (d *projectGitDiagnostic) countSnapshot() string {
	values := make([]string, 0, len(projectGitDiagnosticLabels))
	for _, label := range projectGitDiagnosticLabels {
		values = append(values, fmt.Sprintf("%s:%d", label, d.counts[label]))
	}
	return strings.Join(values, ",")
}

func projectGitDiagnosticClass(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "context_deadline"
	case errors.Is(err, processadapter.ErrOrdinaryContainmentUnsupported):
		return "ordinary_unsupported"
	case errors.Is(err, processadapter.ErrStdoutLimitExceeded):
		return "stdout_limit"
	default:
		return "other_git_or_process"
	}
}

func projectGitDiagnosticEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() != "127.0.0.1" || u.User != nil || strings.ContainsAny(endpoint, "?#") || u.Path != "/repo.git" || u.RawPath != "" {
		return false
	}
	port, err := strconv.Atoi(u.Port())
	return err == nil && port > 0 && port <= 65535
}

func projectGitDiagnosticWorkspace(root, dir string) bool {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(dir) != root || filepath.Clean(dir) != dir {
		return false
	}
	name := filepath.Base(dir)
	if !strings.HasPrefix(name, "agentplugins-git-") && !strings.HasPrefix(name, "agentplugins-direct-ref-") && !strings.HasPrefix(name, "agentplugins-discovery-") {
		return false
	}
	physical, err := filepath.EvalSymlinks(dir)
	return err == nil && physical == dir
}

func projectGitDiagnosticCause(err error, locators ...string) string {
	value := err.Error()
	if len(value) > 8192 || !utf8.ValidString(value) {
		return "cause_omitted"
	}
	// process.OS can attach stderr beneath the native transport wrapper.
	if strings.Contains(value, "client diagnostic:") {
		return "cause_omitted"
	}
	if projectGitDiagnosticCredential.MatchString(value) {
		return "cause_omitted"
	}
	for i := len(locators) - 1; i >= 0; i-- {
		locator := locators[i]
		if len(locator) >= 4 {
			value = strings.ReplaceAll(value, locator, "<fixture-locator>")
		}
	}
	value = projectGitDiagnosticURI.ReplaceAllString(value, "<redacted-uri>")
	quoted := strconv.QuoteToASCII(value)
	if len(quoted) > 2048 {
		quoted = quoted[:2028] + "...[truncated]"
	}
	return quoted
}

func projectGitDiagnosticOperation(command sourceacquisition.Command, endpoint string) (phase string, known, explicit bool) {
	args := command.Args
	repo := filepath.Join(command.Dir, "repository")
	if slices.Equal(args, []string{"init", "--quiet", repo}) {
		return "init", true, false
	}
	if len(args) < 3 || args[0] != "-C" || args[1] != repo {
		return "unknown", false, false
	}
	args = args[2:]
	switch args[0] {
	case "remote":
		if slices.Equal(args, []string{"remote", "add", "origin", endpoint}) {
			return "remote-add", true, false
		}
	case "config":
		if len(args) == 3 {
			want, ok := map[string]string{"core.autocrlf": "false", "core.filemode": "true", "core.symlinks": "true", "fetch.recurseSubmodules": "false"}[args[1]]
			if ok && want == args[2] {
				return "config", true, false
			}
		}
	case "fetch":
		if projectGitDiagnosticFetch(args, endpoint) {
			return "fetch", true, slices.Contains(args, endpoint)
		}
	case "ls-remote":
		if slices.Equal(args, []string{"ls-remote", "--symref", "--", endpoint, "HEAD"}) {
			return "ls-remote", true, true
		}
		if len(args) >= 5 && len(args) <= 6 && args[1] == "--refs" && args[2] == "--" && args[3] == endpoint && projectGitDiagnosticRefs(args[4:]) {
			return "ls-remote", true, true
		}
	case "check-ref-format":
		if len(args) == 2 && projectGitDiagnosticRefs(args[1:]) {
			return "check-ref-format", true, false
		}
	case "rev-parse":
		if slices.Equal(args, []string{"rev-parse", "--verify", "FETCH_HEAD^{commit}"}) {
			return "rev-parse", true, false
		}
	case "checkout":
		if len(args) == 4 && slices.Equal(args[:3], []string{"checkout", "--quiet", "--detach"}) && projectGitDiagnosticSHA.MatchString(args[3]) {
			return "checkout", true, false
		}
	case "ls-tree":
		if len(args) == 6 && slices.Equal(args[:3], []string{"ls-tree", "-r", "-z"}) && projectGitDiagnosticSHA.MatchString(args[3]) && args[4] == "--" && projectGitDiagnosticPath(args[5]) {
			return "ls-tree", true, false
		}
	case "sparse-checkout":
		if slices.Equal(args, []string{"sparse-checkout", "init", "--cone"}) {
			return "sparse-init", true, false
		}
		if len(args) == 5 && slices.Equal(args[:4], []string{"sparse-checkout", "set", "--cone", "--"}) && projectGitDiagnosticPath(args[4]) {
			return "sparse-set", true, false
		}
	case "show":
		if len(args) == 2 {
			revision, path, ok := strings.Cut(args[1], ":")
			if ok && projectGitDiagnosticSHA.MatchString(revision) && projectGitDiagnosticPath(path) {
				return "show", true, false
			}
		}
	}
	return "unknown", false, false
}

func projectGitDiagnosticFetch(args []string, endpoint string) bool {
	if len(args) < 7 || !slices.Equal(args[:3], []string{"fetch", "--quiet", "--depth=1"}) {
		return false
	}
	tail := args[3:]
	if len(tail) > 0 && tail[0] == "--filter=blob:none" {
		tail = tail[1:]
	}
	if len(tail) == 4 && slices.Equal(tail[:3], []string{"--no-tags", "--no-recurse-submodules", "origin"}) {
		return projectGitDiagnosticSHA.MatchString(tail[3])
	}
	return len(tail) == 5 && slices.Equal(tail[:4], []string{"--no-tags", "--no-recurse-submodules", "--", endpoint}) && projectGitDiagnosticSHA.MatchString(tail[4])
}

func projectGitDiagnosticRefs(refs []string) bool {
	for _, ref := range refs {
		if (!strings.HasPrefix(ref, "refs/heads/") && !strings.HasPrefix(ref, "refs/tags/")) || strings.ContainsAny(ref, "\x00\r\n ") || strings.Contains(ref, "..") {
			return false
		}
	}
	return len(refs) > 0
}

func projectGitDiagnosticPath(path string) bool {
	if path == "." {
		return true
	}
	if path == "" || strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\x00\r\n\\") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
