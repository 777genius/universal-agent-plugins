package portablemcp

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

var aliasRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func normalizeServer(alias string, server Server) (Server, error) {
	server.Type = strings.ToLower(strings.TrimSpace(server.Type))
	server.Targets = normalizeTargets(server.Targets)
	switch server.Type {
	case "stdio":
		if server.Stdio == nil {
			return Server{}, domain.NewError(domain.ErrManifestLoad, fmt.Sprintf("portable MCP server %q requires stdio config", alias), nil)
		}
		server.Stdio.Command = strings.TrimSpace(server.Stdio.Command)
		server.Stdio.Args = normalizeStrings(server.Stdio.Args)
		server.Stdio.Env = normalizeStringMap(server.Stdio.Env)
		if server.Stdio.Command == "" {
			return Server{}, domain.NewError(domain.ErrManifestLoad, fmt.Sprintf("portable MCP server %q requires stdio.command", alias), nil)
		}
		if server.Remote != nil {
			return Server{}, domain.NewError(domain.ErrManifestLoad, fmt.Sprintf("portable MCP server %q type stdio may not define remote config", alias), nil)
		}
	case "remote":
		if server.Remote == nil {
			return Server{}, domain.NewError(domain.ErrManifestLoad, fmt.Sprintf("portable MCP server %q requires remote config", alias), nil)
		}
		server.Remote.Protocol = strings.ToLower(strings.TrimSpace(server.Remote.Protocol))
		server.Remote.URL = strings.TrimSpace(server.Remote.URL)
		server.Remote.Headers = normalizeStringMap(server.Remote.Headers)
		if server.Remote.URL == "" {
			return Server{}, domain.NewError(domain.ErrManifestLoad, fmt.Sprintf("portable MCP server %q requires remote.url", alias), nil)
		}
		if server.Stdio != nil {
			return Server{}, domain.NewError(domain.ErrManifestLoad, fmt.Sprintf("portable MCP server %q type remote may not define stdio config", alias), nil)
		}
	default:
		return Server{}, domain.NewError(domain.ErrManifestLoad, fmt.Sprintf("portable MCP server %q type must be stdio or remote", alias), nil)
	}
	return server, nil
}

func normalizeTargets(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	sort.Strings(out)
	uniq := out[:0]
	for _, value := range out {
		if len(uniq) == 0 || uniq[len(uniq)-1] != value {
			uniq = append(uniq, value)
		}
	}
	return uniq
}

func normalizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func normalizeStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func matchesTarget(targets []string, target domain.TargetID) bool {
	if len(targets) == 0 {
		return true
	}
	want := string(target)
	for _, item := range targets {
		if item == want {
			return true
		}
	}
	return false
}
