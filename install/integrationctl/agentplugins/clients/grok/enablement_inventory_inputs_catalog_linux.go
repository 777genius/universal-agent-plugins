//go:build linux

package grok

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// The initial supported closure is an ordinary selected Git root with the
// production HOME-relative XDG environment. External-capable discovery layers
// are refused, not hidden behind a successful empty native inventory.
func (lease *enablementInventoryInputs) captureDiscovery(ctx context.Context) error {
	home, project := lease.inputs.Home.Path, lease.inputs.Project.Path
	if err := lease.captureOutsideDiscovery(ctx); err != nil {
		return err
	}
	absent := []string{"/etc/grok/managed_config.toml", "/etc/grok/requirements.toml",
		"/etc/claude-code/managed-settings.json"}
	for _, root := range []string{home, project} {
		for _, relative := range []string{".grok/managed_config.toml", ".grok/requirements.toml",
			".grok/installed-plugins/registry.json", ".claude/plugins/known_marketplaces.json",
			".claude/plugins/installed_plugins.json", ".claude/settings.json", ".claude/settings.local.json"} {
			absent = append(absent, filepath.Join(root, relative))
		}
	}
	absent = append(absent, filepath.Join(project, ".grok", "config.toml"))
	for _, path := range absent {
		if err := lease.captureAbsent(ctx, path); err != nil {
			return err
		}
	}
	if err := lease.captureGit(ctx); err != nil {
		return err
	}
	count := 0
	for _, root := range []string{home, project} {
		for _, surface := range []string{".grok", ".claude"} {
			if err := lease.capturePlugins(ctx, filepath.Join(root, surface, "plugins"), &count); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

// Unmounted ancestors must not carry discovery markers that the child view
// would otherwise hide. HOME-contained ancestors remain in the original view.
func (lease *enablementInventoryInputs) captureOutsideDiscovery(ctx context.Context) error {
	home := lease.inputs.Home.Path
	path := filepath.Dir(lease.inputs.Project.Path)
	for range 32 {
		if err := ctx.Err(); err != nil {
			return err
		}
		if path != home && !strings.HasPrefix(path, home+string(filepath.Separator)) {
			for _, marker := range []string{".grok", ".claude"} {
				if err := lease.captureAbsent(ctx, filepath.Join(path, marker)); err != nil {
					return err
				}
			}
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ctx.Err()
		}
		path = parent
	}
	return errEnablementInventoryInputs
}

func (lease *enablementInventoryInputs) captureGit(ctx context.Context) error {
	root := filepath.Join(lease.inputs.Project.Path, ".git")
	pin, _, err := lease.captureInput(ctx, root, true)
	if err != nil {
		return err
	}
	if pin.file == nil {
		return errEnablementInventoryInputs
	}
	for _, relative := range []string{"objects", "refs"} {
		child, _, err := lease.captureInput(ctx, filepath.Join(root, relative), true)
		if err != nil || child.file == nil {
			return errors.Join(err, errEnablementInventoryInputs)
		}
	}
	for _, relative := range []string{"commondir", "gitdir", "config.worktree", "worktrees", "objects/info/alternates", "objects/info/http-alternates"} {
		if err := lease.captureAbsent(ctx, filepath.Join(root, relative)); err != nil {
			return err
		}
	}
	config, body, err := lease.captureInput(ctx, filepath.Join(root, "config"), false)
	if err != nil {
		return err
	}
	if config.file == nil || !inventoryInputGitConfig(body) {
		return errEnablementInventoryInputs
	}
	head, body, err := lease.captureInput(ctx, filepath.Join(root, "HEAD"), false)
	if err != nil {
		return err
	}
	if head.file == nil || !inventoryInputGitHEAD(body) {
		return errEnablementInventoryInputs
	}
	return ctx.Err()
}

func inventoryInputGitHEAD(body []byte) bool {
	text := string(body)
	if !strings.HasPrefix(text, "ref: refs/heads/") || !strings.HasSuffix(text, "\n") {
		return false
	}
	branch := strings.TrimSuffix(strings.TrimPrefix(text, "ref: refs/heads/"), "\n")
	if branch == "" || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") || strings.Contains(branch, "//") {
		return false
	}
	for _, ch := range branch {
		if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') && ch != '-' && ch != '_' && ch != '/' {
			return false
		}
	}
	return true
}

func inventoryInputGitConfig(body []byte) bool {
	core := false
	seen := map[string]bool{}
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if line == "[core]" && !core {
			core = true
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !core || !ok || seen[key] {
			return false
		}
		seen[key] = true
		switch key {
		case "repositoryformatversion":
			if value != "0" {
				return false
			}
		case "bare":
			if value != "false" {
				return false
			}
		case "filemode", "logallrefupdates", "ignorecase", "precomposeunicode":
			if value != "true" && value != "false" {
				return false
			}
		default:
			return false
		}
	}
	return core && seen["repositoryformatversion"] && seen["bare"]
}

func inventoryInputConfig(body []byte) error {
	document, _, err := membershipDocument(body)
	if err != nil {
		return err
	}
	for _, key := range []string{"version_overrides", "include", "includes", "import", "imports"} {
		if _, present := document[key]; present || membershipAliases(document, key) {
			return errEnablementInventoryInputs
		}
	}
	if raw, present := document["plugins"]; present {
		table := raw.(map[string]any) // The existing strict membership parser proved this type.
		for key := range table {
			if key != "enabled" && key != "disabled" {
				return errEnablementInventoryInputs
			}
		}
	}
	return nil
}

func (lease *enablementInventoryInputs) capturePlugins(ctx context.Context, path string, count *int) error {
	pin, _, err := lease.captureInput(ctx, path, true)
	if err != nil || pin.file == nil {
		return err
	}
	entries, err := pin.file.ReadDir(projectDirectoryLimit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(entries) > projectDirectoryLimit {
		return errEnablementInventoryInputs
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return errEnablementInventoryInputs
		}
		*count++
		if *count > projectDirectoryLimit {
			return errEnablementInventoryInputs
		}
		root := filepath.Join(path, entry.Name())
		for _, layout := range []string{".grok-plugin", ".claude-plugin"} {
			manifest, body, err := lease.captureInput(ctx, filepath.Join(root, layout, "plugin.json"), false)
			if err != nil {
				return err
			}
			if manifest.file != nil {
				if err := inventoryInputManifest(body); err != nil {
					return err
				}
			}
		}
	}
	return pin.revalidate(ctx)
}

func inventoryInputManifest(body []byte) error {
	object, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return err
	}
	for _, key := range []string{"skills", "commands", "agents", "hooks", "mcpServers", "lspServers"} {
		raw, present := object[key]
		if !present {
			continue
		}
		values, ok := raw.([]any)
		if !ok {
			values = []any{raw}
		}
		if len(values) > projectDirectoryLimit {
			return errEnablementInventoryInputs
		}
		for _, value := range values {
			path, ok := value.(string)
			if !ok || !inventoryInputRelative(path) {
				return errEnablementInventoryInputs
			}
		}
	}
	return nil
}

func inventoryInputRelative(path string) bool {
	if !membershipText(path) || filepath.IsAbs(path) || strings.ContainsAny(path, "\\$~%:") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			return false
		}
	}
	return path != "." && path != "./"
}
