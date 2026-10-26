//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
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
func (c *controller) trustedDirectory(path string) error {
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
	defer parent.Close()
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
	defer held.Close()
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
func (c *controller) trustedWrite(path string, body []byte, mode os.FileMode) error {
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
	defer parent.Close()
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
	for _, path := range []string{root, f.Home, f.Profile, filepath.Join(f.Profile, "User"), f.Extensions,
		filepath.Join(root, "tmp"), filepath.Join(root, "sources"), filepath.Join(root, "results"), filepath.Join(root, "installer-state")} {
		if err := c.trustedDirectory(path); err != nil {
			return err
		}
	}
	// The display supervisor retains its existing fixed task temporary directory.
	if err := c.ownedDirectory(filepath.Join(c.request.Base, "tmp")); err != nil {
		return err
	}
	for _, name := range []string{"config", "data", "state", "cache"} {
		if err := c.trustedDirectory(filepath.Join(f.Home, name)); err != nil {
			return err
		}
	}
	settings := map[string]any{"update.mode": "none", "telemetry.telemetryLevel": "off", "extensions.autoUpdate": "off",
		"extensions.autoCheckUpdates": false, "workbench.startupEditor": "none", "window.restoreWindows": "none",
		"workbench.welcomePage.experimentalOnboarding": false,
		"security.workspace.trust.enabled":             true, "security.workspace.trust.startupPrompt": "always", "chat.mcp.autostart": "never"}
	if err := c.trustedJSON(filepath.Join(f.Profile, "User", "settings.json"), settings); err != nil {
		return err
	}
	foreign := map[string]any{"type": "http", "url": "http://127.0.0.1:9/preserved-foreign"}
	if err := c.trustedJSON(filepath.Join(f.Profile, "User", "mcp.json"), map[string]any{"servers": map[string]any{"uap-foreign-user": foreign}}); err != nil {
		return err
	}
	f.UserSkill = filepath.Join(f.Home, ".copilot", "skills", "uap-vscode-user", "SKILL.md")
	if err := c.trustedWrite(f.UserSkill, skillBody("uap-vscode-user", "PREEXISTING-USER-"+nonce), 0600); err != nil {
		return err
	}
	for _, scope := range []string{"a", "b", "user"} {
		project := filepath.Join(root, "project-"+scope)
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

func trustedPackageDigest(ctx context.Context, root string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	captured, err := trustedCapturePackage(ctx, root)
	if err != nil {
		return "", err
	}
	return packagedigest.DigestCaptured(ctx, captured)
}

// Installed managed receipts use the existing provider Stager's framing. The
// acquired source's v1 digest above remains a separate immutable identity.
func trustedInstalledPackageDigest(ctx context.Context, root string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	captured, err := trustedCapturePackage(ctx, root)
	if err != nil {
		return "", err
	}
	if _, err := packagedigest.DigestCaptured(ctx, captured); err != nil {
		return "", err
	}
	sort.Slice(captured, func(i, j int) bool { return captured[i].Path < captured[j].Path })
	hash := sha256.New()
	for _, item := range captured {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		kind := "file"
		if item.Kind == "directory" {
			kind = "dir"
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%t\x00%d\x00", kind, item.Path, item.Executable, len(item.Content))
		for body := item.Content; len(body) > 0; {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			n := min(len(body), 32*1024)
			_, _ = hash.Write(body[:n])
			body = body[n:]
		}
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func trustedCapturePackage(ctx context.Context, root string) ([]packagedigest.CapturedEntry, error) {
	parent, err := trustedParent(filepath.Join(root, "leaf"))
	if err != nil {
		return nil, err
	}
	_ = parent.Close()
	captured := []packagedigest.CapturedEntry{}
	count, total := 0, int64(0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if count > 4096 || len(strings.Split(rel, string(filepath.Separator))) > 32 {
			return fmt.Errorf("package capture entry/depth bound")
		}
		info, err := anchoredInfo(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			parent, err := trustedParent(filepath.Join(path, "leaf"))
			if err != nil {
				return err
			}
			_ = parent.Close()
			if rel != "." {
				captured = append(captured, packagedigest.CapturedEntry{Path: filepath.ToSlash(rel), Kind: "directory"})
			}
			return nil
		}
		if !regular(info, 128<<20) {
			return fmt.Errorf("package capture nonordinary file")
		}
		total += info.Size()
		if total > 256<<20 {
			return fmt.Errorf("package capture byte bound")
		}
		body, err := readOrdinary(path, 128<<20)
		if err != nil {
			return err
		}
		captured = append(captured, packagedigest.CapturedEntry{Path: filepath.ToSlash(rel), Kind: "file", Executable: info.Mode().Perm()&0111 != 0, Content: body})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return captured, nil
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

func (c *controller) trustedReceipt(scope string) error {
	f := c.trustedState
	p := f.Packages[scope]
	if err := c.trustedPreserved(); err != nil {
		return err
	}
	statePath := filepath.Join(f.Projects[scope], ".uap", "state-v2.json")
	if scope == "user" {
		statePath = filepath.Join(f.Root, "installer-state", "state-v2.json")
	}
	body, err := readOrdinary(statePath, 1<<20)
	if err != nil {
		return err
	}
	if _, err := uniqueJSON(body); err != nil {
		return err
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(body, &state); err != nil {
		return err
	}
	selected := []domain.Installation{}
	for _, i := range state.Installations {
		if i.DeclaredName == p.Name {
			selected = append(selected, i)
		}
	}
	if len(selected) != 1 || len(selected[0].Clients) != 1 {
		return fmt.Errorf("exact public receipt selection differs")
	}
	i := selected[0]
	wantVersion := "1.0.0"
	if p.Revision == "a-two" {
		wantVersion = "1.0.1"
	}
	if p.SourceDigest == "" || p.ManifestDigest == "" || i.Source.CanonicalSource != p.Source || i.Source.RequestedSource != p.Source || i.Source.SourceBindingID != domain.ComputeSourceBindingID(domain.SourceIdentity{CanonicalSource: p.Source}) || i.Source.TreeDigest != p.SourceDigest || i.Source.Repository != "" || i.Source.PackageSubpath != "" || i.Source.ResolvedRevision != "" || i.Package.DeclaredName != p.Name || i.Package.ManifestDigest != p.ManifestDigest || i.Package.Version != wantVersion {
		return fmt.Errorf("public source/package identity differs")
	}
	for _, binding := range i.Clients {
		wantScope := "project"
		if scope == "user" {
			wantScope = "user"
		}
		if binding.ClientID != "vscode" || binding.Scope != wantScope || binding.Activation != domain.ActivationPrepared || binding.TargetLocator == "" || binding.PhysicalArtifact != domain.ComputePhysicalArtifactID(p.Name, i.InstallationID) || binding.PackageRevision == nil || binding.PackageRevision.TreeDigest != p.SourceDigest || binding.PackageRevision.ManifestDigest != p.ManifestDigest {
			return fmt.Errorf("public receipt scope/Prepared state differs")
		}
		if !trustedContained(f.Root, binding.TargetLocator) {
			return fmt.Errorf("receipt package escaped fixture")
		}
		if scope != "user" {
			if binding.TargetLocator != filepath.Join(f.Projects[scope], ".uap", "managed", "clients", "vscode", binding.PhysicalArtifact) || binding.NativeProfileRoot != filepath.Join(f.Projects[scope], ".vscode") || binding.Verification != domain.VerificationInstalled {
				return fmt.Errorf("project receipt authority differs")
			}
			data := i.DataReceipts[binding.DataReceiptID]
			if data.DataReceiptID == "" || data.DataReceiptID != binding.DataReceiptID || data.State != domain.DataReceiptOwned || data.Scope != "project" || data.OwnershipDigest == "" || data.PhysicalBackend != binding.PhysicalArtifact || !trustedContained(filepath.Join(f.Projects[scope], ".uap", "plugin-data"), data.Locator) {
				return fmt.Errorf("persistent data authority differs")
			}
			p.Data = data.Locator
			p.Skill = filepath.Join(f.Projects[scope], ".github", "skills", p.Name, "SKILL.md")
		} else {
			p.Skill = filepath.Join(binding.TargetLocator, "skills", p.Name, "SKILL.md")
		}
		p.Binding = binding
	}
	p.Installation = i
	want, err := readOrdinary(filepath.Join(p.Source, "skills", p.Name, "SKILL.md"), 1<<20)
	if err != nil {
		return err
	}
	got, err := readOrdinary(p.Skill, 1<<20)
	if err != nil || !bytes.Equal(got, want) {
		return fmt.Errorf("public native skill bytes differ")
	}
	return c.trustedReceiptObjects(scope, p, want)
}

func (c *controller) trustedReceiptObjects(scope string, p *trustedPackage, skill []byte) error {
	f, binding := c.trustedState, p.Binding
	active := binding.TargetLocator
	// Preserve every untransformed source file, especially the actual probe.
	for _, relative := range []string{"plugin.json", "skills/" + p.Name + "/SKILL.md"} {
		want, err := readOrdinary(filepath.Join(p.Source, filepath.FromSlash(relative)), 1<<20)
		if err != nil {
			return err
		}
		got, err := readOrdinary(filepath.Join(active, filepath.FromSlash(relative)), 1<<20)
		if err != nil || !bytes.Equal(got, want) {
			return fmt.Errorf("managed package differs from frozen authored source")
		}
	}
	digest, err := trustedInstalledPackageDigest(c.work, active)
	if err != nil {
		return err
	}
	want := map[string]domain.NativeObjectOwnership{
		"managed_package_directory": {ObjectID: "package:vscode:" + binding.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: p.Name, Path: active, ManagedDigest: digest, ProtectionClass: "managed"},
	}
	if scope != "user" {
		for path, expected := range map[string]string{filepath.Join(active, "bin", "probe"): c.request.Input.Trusted.ProbeSHA256, filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName): c.request.Input.Trusted.InstallerSHA256} {
			info, err := anchoredInfo(path)
			if err != nil || !regular(info, 128<<20) || info.Mode().Perm()&0111 == 0 {
				return fmt.Errorf("installed probe/launcher executable identity differs")
			}
			actual, err := hashFile(path, 128<<20)
			if err != nil || actual != expected {
				return fmt.Errorf("installed probe/launcher differs from sealed exact binary")
			}
		}
		sum := sha256.Sum256(append(append([]byte("f\x00SKILL.md\x00-\x00"), skill...), 0))
		kind := "vscode_project_skill_directory"
		want[kind] = domain.NativeObjectOwnership{ObjectID: kind + ":" + p.Name, Kind: kind, LogicalName: p.Name, Path: filepath.Dir(p.Skill), SourceRelative: "skills/" + p.Name, ManagedDigest: "sha256:" + hex.EncodeToString(sum[:]), ProtectionClass: "managed"}
		expected := trustedExpectedServer(f, p, scope)
		args, env := []string{}, map[string]string{}
		for _, value := range expected["args"].([]any) {
			args = append(args, value.(string))
		}
		for key, value := range expected["env"].(map[string]any) {
			env[key] = value.(string)
		}
		config := filepath.Join(f.Projects[scope], ".vscode", "mcp.json")
		body, err := readOrdinary(config, 1<<20)
		if err != nil {
			return err
		}
		value, err := uniqueJSON(body)
		document, ok := value.(map[string]any)
		if err != nil || !ok {
			return fmt.Errorf("installed native config is malformed")
		}
		entries, ok := document["servers"].(map[string]any)
		if !ok || !reflect.DeepEqual(entries[p.Name], expected) {
			return fmt.Errorf("installed native projection differs from independent source/receipt")
		}
		receipt, err := nativeconfig.DesiredReceipt(config, nativeconfig.CodecVSCode, p.Name, nativeconfig.Server{Type: "stdio", Command: expected["command"].(string), Args: args, Env: env, StdioValuesResolved: true, CWDResolved: true}, nativeconfig.Placeholders{})
		if err != nil {
			return err
		}
		kind = "vscode_project_mcp_server"
		want[kind] = domain.NativeObjectOwnership{ObjectID: kind + ":" + p.Name, Kind: kind, LogicalName: p.Name, Path: config, ManagedDigest: receipt.Digest, ProtectionClass: "managed"}
	}
	if len(binding.NativeObjects) != len(want) {
		return fmt.Errorf("exact native ownership inventory differs")
	}
	seen := map[string]bool{}
	for _, object := range binding.NativeObjects {
		expected, ok := want[object.Kind]
		if !ok || seen[object.Kind] || !reflect.DeepEqual(object, expected) {
			return fmt.Errorf("native ownership identity/path/digest differs")
		}
		seen[object.Kind] = true
	}
	return nil
}

// Call immediately after the actual public user add, before project installs
// or any installed native host. No later observation may reset this baseline.
func (c *controller) trustedPreserveUser() error {
	f := c.trustedState
	p := f.Packages["user"]
	paths := []string{filepath.Join(f.Root, "installer-state", "state-v2.json"), p.Binding.TargetLocator}
	for _, data := range p.Installation.DataReceipts {
		if !trustedContained(f.Root, data.Locator) {
			return fmt.Errorf("user data escapes fixture")
		}
		paths = append(paths, data.Locator)
	}
	for _, path := range paths {
		if err := c.trustedProtect(path); err != nil {
			return err
		}
	}
	return nil
}

// Expected bytes derive from the authored package and selected receipt. They
// are never adopted from the native document under observation.
func trustedExpectedServer(f *trustedLifecycle, p *trustedPackage, scope string) map[string]any {
	active := p.Binding.TargetLocator
	return map[string]any{"type": "stdio", "command": filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName),
		"args": []any{"--internal-stdio-v1", active, p.Data, active, "plugin", "--", "./bin/probe", "argument with spaces", active + "/argument", "${UNKNOWN}"},
		"env":  map[string]any{"PLUGIN_ROOT": active, "PLUGIN_DATA": p.Data, "UAP_TEST_ROOT": f.Root, "UAP_TEST_EVENTS": filepath.Join(f.Root, scope+"-events.jsonl"), "UAP_TEST_NONCE": f.Nonce, "UAP_TEST_REVISION": p.Revision, "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": active + "/${UNKNOWN}"}}
}
