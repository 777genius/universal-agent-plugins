//go:build linux

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type trustedPackage struct {
	Source, Name, Revision, Data, Skill string
	SourceDigest, ManifestDigest        string
	Installation                        domain.Installation
	Binding                             domain.ClientBinding
}

// Hold every parent without following aliases before any privileged mutation.
// A renamed ordinary fixture ancestor cannot redirect this descriptor to a
// foreign path. The final linked identity is checked again after the write.
func trustedParent(path string) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return nil, fmt.Errorf("canonical fixture leaf required")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(filepath.Dir(path), "/"), "/") {
		if part == "" {
			continue
		}
		next, openErr := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		_ = unix.Close(fd)
		if openErr != nil {
			return nil, openErr
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), filepath.Dir(path)), nil
}

// In particular, an existing file is opened WITHOUT O_TRUNC. No root write,
// ownership or mode mutation happens until the held inode matches the checked
// single-link file (or ordinary directory) exactly.
func trustedOpenHeld(parent *os.File, name string, before os.FileInfo, directory bool) (*os.File, error) {
	if filepath.Base(name) != name || name == "." || name == ".." {
		return nil, fmt.Errorf("fixture leaf required")
	}
	flags := unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC | unix.O_WRONLY
	if directory {
		flags = unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC | unix.O_RDONLY | unix.O_DIRECTORY
	} else if before == nil {
		flags |= unix.O_CREAT | unix.O_EXCL
	}
	fd, err := unix.Openat(int(parent.Fd()), name, flags, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	held, err := file.Stat()
	if err != nil || (directory && !held.IsDir()) || (!directory && !regular(held, 128<<20)) || (before != nil && !sameFile(before, held)) {
		_ = file.Close()
		return nil, fmt.Errorf("held fixture identity differs")
	}
	return file, nil
}

type trustedLifecycle struct {
	Root, Home, Profile, Extensions, Nonce, Marker, UserSkill string
	Projects                                                  map[string]string
	Packages                                                  map[string]*trustedPackage
	Protected                                                 map[string]map[string]string
	Foreign                                                   map[string]any
	Sessions                                                  map[string]bool
	Stages                                                    []string
	Complete                                                  bool
}

func trustedContained(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Only fixture-owned descendants are chowned. Sealed controller, executable and
// request ancestors always remain outside this writable tree.
func (c *controller) trustedDirectory(path string) (resultErr error) {
	if c.scannerCache != nil && c.scannerCache.sealed {
		if node := c.scannerCache.node(path); node != nil {
			return node.verify()
		}
	}
	root := c.trustedState.Root
	if path != root && !trustedContained(root, path) {
		return fmt.Errorf("fixture directory escapes owned root")
	}
	if path != root {
		if _, err := os.Lstat(filepath.Dir(path)); os.IsNotExist(err) {
			if err := c.trustedDirectory(filepath.Dir(path)); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	parent, err := trustedParent(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parent.Close()) }()
	if err := unix.Mkdirat(int(parent.Fd()), filepath.Base(path), 0700); err != nil && err != unix.EEXIST {
		return err
	}
	info, err := anchoredInfo(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("ordinary fixture directory required")
	}
	held, err := trustedOpenHeld(parent, filepath.Base(path), info, true)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, held.Close()) }()
	if err := held.Chown(c.request.Input.UID, c.request.Input.GID); err != nil {
		return err
	}
	after, err := held.Stat()
	linked, linkErr := anchoredInfo(path)
	if err != nil || linkErr != nil || !sameFile(after, linked) {
		return fmt.Errorf("fixture directory link changed")
	}
	return nil
}
func (c *controller) trustedWrite(path string, body []byte, mode os.FileMode) (resultErr error) {
	if c.scannerCache != nil && c.scannerCache.sealed && c.scannerCache.protects(path) {
		return fmt.Errorf("sealed scanner ancestry cannot be rewritten")
	}
	if !trustedContained(c.trustedState.Root, path) {
		return fmt.Errorf("fixture write outside owned root")
	}
	if err := c.trustedDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	parent, err := trustedParent(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parent.Close()) }()
	var before os.FileInfo
	if info, err := anchoredInfo(path); err == nil {
		if !regular(info, 128<<20) {
			return fmt.Errorf("fixture replacement is not ordinary")
		}
		before = info
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := trustedOpenHeld(parent, filepath.Base(path), before, false)
	if err != nil {
		return err
	}
	if err = file.Truncate(0); err == nil {
		_, err = file.Write(body)
	}
	if err == nil {
		err = file.Chmod(mode)
	}
	if err == nil {
		err = file.Chown(c.request.Input.UID, c.request.Input.GID)
	}
	after, statErr := file.Stat()
	linked, linkErr := anchoredInfo(path)
	if err == nil && (statErr != nil || linkErr != nil || !sameFile(after, linked)) {
		err = fmt.Errorf("fixture file link changed")
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func (c *controller) trustedJSON(path string, value any) error {
	body, err := encode(value, 1<<20)
	if err != nil {
		return err
	}
	return c.trustedWrite(path, body, 0600)
}

func (c *controller) trustedFixtures() error {
	root := filepath.Join(c.request.Base, "trusted")
	nonce, err := randomCookie()
	if err != nil {
		return err
	}
	f := &trustedLifecycle{Root: root, Home: filepath.Join(root, "home"), Nonce: nonce,
		Marker: "vscode-retained-" + nonce, Projects: map[string]string{}, Packages: map[string]*trustedPackage{},
		Protected: map[string]map[string]string{}, Foreign: map[string]any{}, Sessions: map[string]bool{}}
	c.trustedState = f
	f.Profile, f.Extensions = filepath.Join(f.Home, "config", "Code"), filepath.Join(root, "extensions")
	foreign, err := c.trustedHomeFixtures()
	if err != nil {
		return err
	}
	if err := c.trustedProjectFixtures(foreign); err != nil {
		return err
	}
	if err := c.prepareTrustedScanner(); err != nil {
		return err
	}
	for _, path := range []string{f.UserSkill, filepath.Join(f.Profile, "User", "settings.json"), filepath.Join(f.Profile, "User", "mcp.json")} {
		if err := c.trustedProtect(path); err != nil {
			return err
		}
	}
	for _, scope := range []string{"a", "b"} {
		for _, relative := range []string{".github/mcp.json", ".github/skills/uap-foreign-" + scope} {
			if err := c.trustedProtect(filepath.Join(f.Projects[scope], relative)); err != nil {
				return err
			}
		}
	}
	return c.checkTrustedAssets()
}

func (c *controller) trustedHomeFixtures() (map[string]any, error) {
	f := c.trustedState
	for _, path := range []string{f.Root, f.Home, f.Profile, filepath.Join(f.Profile, "User"), f.Extensions,
		filepath.Join(f.Root, "tmp"), filepath.Join(f.Root, "sources"), filepath.Join(f.Root, "results"), filepath.Join(f.Root, "installer-state")} {
		if err := c.trustedDirectory(path); err != nil {
			return nil, err
		}
	}
	// The display supervisor retains its existing fixed task temporary directory.
	if err := c.ownedDirectory(filepath.Join(c.request.Base, "tmp")); err != nil {
		return nil, err
	}
	for _, name := range []string{"config", "data", "state", "cache"} {
		if err := c.trustedDirectory(filepath.Join(f.Home, name)); err != nil {
			return nil, err
		}
	}
	settings := map[string]any{"update.mode": "none", "telemetry.telemetryLevel": "off", "extensions.autoUpdate": "off",
		"extensions.autoCheckUpdates": false, "workbench.startupEditor": "none", "window.restoreWindows": "none",
		"workbench.welcomePage.experimentalOnboarding": false,
		"security.workspace.trust.enabled":             true, "security.workspace.trust.startupPrompt": "always", "chat.mcp.autostart": "never"}
	if err := c.trustedJSON(filepath.Join(f.Profile, "User", "settings.json"), settings); err != nil {
		return nil, err
	}
	foreign := map[string]any{"type": "http", "url": "http://127.0.0.1:9/preserved-foreign"}
	if err := c.trustedJSON(filepath.Join(f.Profile, "User", "mcp.json"), map[string]any{"servers": map[string]any{"uap-foreign-user": foreign}}); err != nil {
		return nil, err
	}
	f.UserSkill = filepath.Join(f.Home, ".copilot", "skills", "uap-vscode-user", "SKILL.md")
	if err := c.trustedWrite(f.UserSkill, skillBody("uap-vscode-user", "PREEXISTING-USER-"+f.Nonce), 0600); err != nil {
		return nil, err
	}
	return foreign, nil
}

func (c *controller) trustedProjectFixtures(foreign map[string]any) error {
	f := c.trustedState
	for _, scope := range []string{"a", "b", "user"} {
		project := filepath.Join(f.Root, "project-"+scope)
		f.Projects[scope] = project
		if err := c.trustedDirectory(project); err != nil {
			return err
		}
		if scope != "user" {
			f.Foreign[scope] = foreign
			if err := c.trustedJSON(filepath.Join(project, ".vscode", "mcp.json"), map[string]any{"servers": map[string]any{"uap-foreign-" + scope: foreign}, "inputs": []any{}}); err != nil {
				return err
			}
			if err := c.trustedWrite(filepath.Join(project, ".github", "mcp.json"), []byte("untouched Copilot control "+scope+"\n"), 0600); err != nil {
				return err
			}
			if err := c.trustedWrite(filepath.Join(project, ".github", "skills", "uap-foreign-"+scope, "SKILL.md"), skillBody("uap-foreign-"+scope, "FOREIGN-"+scope), 0600); err != nil {
				return err
			}
		}
		if err := c.trustedSource(scope, scope+"-one"); err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) trustedSource(scope, revision string) error {
	f := c.trustedState
	name := "uapvscode" + scope
	source := filepath.Join(f.Root, "sources", scope)
	version := "1.0.0"
	if revision == "a-two" {
		version = "1.0.1"
	}
	if err := c.trustedJSON(filepath.Join(source, "plugin.json"), map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": name, "version": version}); err != nil {
		return err
	}
	if err := c.trustedWrite(filepath.Join(source, "skills", name, "SKILL.md"), skillBody(name, "UAP_VSCODE_"+revision+"_"+f.Nonce), 0600); err != nil {
		return err
	}
	if scope != "user" {
		probe, err := readOrdinary(filepath.Join(c.request.Base, "executables", "native-probe"), 128<<20)
		if err != nil {
			return err
		}
		if err = c.trustedWrite(filepath.Join(source, "bin", "probe"), probe, 0700); err != nil {
			return err
		}
		server := map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"argument with spaces", "${PLUGIN_ROOT}/argument", "${UNKNOWN}"}, "cwd": "${PLUGIN_ROOT}",
			"env": map[string]string{"UAP_TEST_ROOT": f.Root, "UAP_TEST_EVENTS": filepath.Join(f.Root, scope+"-events.jsonl"), "UAP_TEST_NONCE": f.Nonce, "UAP_TEST_REVISION": revision, "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": "${PLUGIN_ROOT}/${UNKNOWN}"}}
		if err = c.trustedJSON(filepath.Join(source, "mcp.json"), map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{name: server}}); err != nil {
			return err
		}
	}
	digest, err := trustedPackageDigest(c.work, source)
	if err != nil {
		return err
	}
	manifest, err := hashFile(filepath.Join(source, "plugin.json"), 1<<20)
	if err != nil {
		return err
	}
	f.Packages[scope] = &trustedPackage{Source: source, Name: name, Revision: revision, SourceDigest: digest, ManifestDigest: "sha256:" + manifest}
	// Freeze before even the empty native baseline; later receipt reads cannot
	// adopt a native-mutated authored package as their independent expectation.
	return c.trustedProtect(source)
}

func trustedInventory(root string) (map[string]string, error) {
	result := map[string]string{}
	count := 0
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		count++
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if count > 4096 || strings.Count(rel, string(filepath.Separator)) > 32 {
			return fmt.Errorf("protected tree bound")
		}
		info, err := anchoredInfo(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			result[rel] = fmt.Sprintf("directory:%o", info.Mode())
			return nil
		}
		if !regular(info, 128<<20) {
			return fmt.Errorf("protected tree contains nonordinary member")
		}
		total += info.Size()
		if total > 256<<20 {
			return fmt.Errorf("protected tree byte bound")
		}
		digest, err := hashFile(path, 128<<20)
		if err != nil {
			return err
		}
		result[rel] = fmt.Sprintf("%o:%d:%s", info.Mode(), info.Size(), digest)
		return nil
	})
	return result, err
}
func (c *controller) trustedProtect(path string) error {
	if _, exists := c.trustedState.Protected[path]; exists {
		return fmt.Errorf("protected baseline already exists")
	}
	value, err := trustedInventory(path)
	if err != nil {
		return err
	}
	c.trustedState.Protected[path] = value
	return nil
}
func (c *controller) trustedPreserved() error {
	for path, want := range c.trustedState.Protected {
		got, err := trustedInventory(path)
		if err != nil || !reflect.DeepEqual(got, want) {
			return fmt.Errorf("protected original-user/neighbor/native bytes changed: %s", path)
		}
	}
	for scope, want := range c.trustedState.Foreign {
		body, err := readOrdinary(filepath.Join(c.trustedState.Projects[scope], ".vscode", "mcp.json"), 1<<20)
		if err != nil {
			return err
		}
		value, err := uniqueJSON(body)
		object, ok := value.(map[string]any)
		if err != nil || !ok {
			return fmt.Errorf("native foreign config malformed")
		}
		servers, ok := object["servers"].(map[string]any)
		if !ok || !reflect.DeepEqual(servers["uap-foreign-"+scope], want) {
			return fmt.Errorf("unrelated native server changed")
		}
	}
	return nil
}

func (c *controller) checkTrustedAssets() error {
	if c.scannerCache == nil {
		return fmt.Errorf("trusted scanner cache admission missing")
	}
	if err := c.scannerCache.verify(); err != nil {
		return err
	}
	if err := c.checkPublisher(); err != nil {
		return err
	}
	in := c.request.Input.Trusted
	root := filepath.Join(c.request.Base, "tools")
	body, err := readOrdinary(filepath.Join(root, "manifest.json"), trustedManifestLimit)
	if err != nil {
		return err
	}
	manifest, err := decodePinnedTrustedManifest(body, in.ToolsManifestSHA256)
	if err != nil {
		return err
	}
	for _, item := range manifest.Files {
		path := filepath.Join(root, item.Path)
		if err := immutableAncestors(filepath.Dir(path)); err != nil {
			return err
		}
		info, err := anchoredInfo(path)
		if err != nil || info.Size() != item.Bytes {
			return fmt.Errorf("sealed tool size differs")
		}
		uid, err := statUID(info)
		if err != nil || uid != 0 || info.Mode().Perm()&0222 != 0 {
			return fmt.Errorf("sealed tool writable")
		}
		actual, err := hashFile(path, 256<<20)
		if err != nil || actual != item.SHA256 {
			return fmt.Errorf("sealed complete runtime closure changed")
		}
	}
	for name, want := range map[string]string{"agentplugins": in.InstallerSHA256, "native-probe": in.ProbeSHA256, "lintai": in.ScannerSHA256} {
		path := filepath.Join(c.request.Base, "executables", name)
		actual, err := hashFile(path, 128<<20)
		if err != nil || actual != want {
			return fmt.Errorf("sealed fixture executable changed")
		}
	}
	return c.trustedPreserved()
}
