package repostate

import (
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
)

type State struct {
	GitAvailable    bool
	InGitRepo       bool
	RepoRoot        string
	HasOriginRemote bool
	OriginURL       string
	OriginHost      string
	OriginIsGitHub  bool
}

func Inspect(root string) State {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	out, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		if isGitMissing(err) {
			return State{}
		}
		return State{GitAvailable: true}
	}
	state := State{
		GitAvailable: true,
		InGitRepo:    true,
		RepoRoot:     filepath.Clean(strings.TrimSpace(string(out))),
	}
	remote, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").CombinedOutput()
	if err != nil {
		return state
	}
	state.HasOriginRemote = true
	state.OriginURL = strings.TrimSpace(string(remote))
	state.OriginHost, state.OriginIsGitHub = parseOrigin(state.OriginURL)
	return state
}

func isGitMissing(err error) bool {
	if err == exec.ErrNotFound {
		return true
	}
	if ee, ok := err.(*exec.Error); ok && ee.Err == exec.ErrNotFound {
		return true
	}
	return false
}

func parseOrigin(raw string) (host string, github bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if strings.HasPrefix(raw, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(raw, "git@"), ":", 2)
		if len(parts) != 2 {
			return "", false
		}
		host = strings.ToLower(strings.TrimSpace(parts[0]))
		return host, host == "github.com"
	}
	if strings.HasPrefix(raw, "ssh://") || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", false
		}
		host = strings.ToLower(strings.TrimSpace(u.Hostname()))
		return host, host == "github.com"
	}
	return "", false
}
