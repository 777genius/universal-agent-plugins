//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

func (c *controller) ownedDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	return os.Chown(path, c.request.Input.UID, c.request.Input.GID)
}
func writeSealed(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0444)
}
func skillBody(name, marker string) []byte {
	return []byte("---\nname: " + name + "\ndescription: " + marker + "\ndisable-model-invocation: true\n---\nInert research marker. No command or tool request.\n")
}
func markerMCP(marker string) []byte {
	return []byte(`{"servers":{"uap-` + strings.ToLower(marker) + `":{"type":"http","url":"http://127.0.0.1:9/UAP-` + marker + `"}}}` + "\n")
}
func (c *controller) fixtures() error {
	if err := c.homeFixtures(); err != nil {
		return err
	}
	if err := c.projectFixtures(); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(c.request.Base, "probes"), 0755); err != nil {
		return err
	}
	for _, label := range probeLabels {
		if err := c.probeFixture(label); err != nil {
			return err
		}
	}
	return nil
}
func (c *controller) homeFixtures() error {
	base := c.request.Base
	if err := os.Mkdir(filepath.Join(base, "home"), 0755); err != nil {
		return err
	}
	for _, name := range []string{"config", "cache", "state", "data", "npm-cache"} {
		if err := c.ownedDirectory(filepath.Join(base, "home", name)); err != nil {
			return err
		}
	}
	for _, name := range []string{"npm-user-config", "npm-global-config", "git-config"} {
		if err := writeSealed(filepath.Join(base, "home", name), nil); err != nil {
			return err
		}
	}
	if err := c.ownedDirectory(filepath.Join(base, "tmp")); err != nil {
		return err
	}
	for _, item := range []struct{ name, marker string }{{"uap-user", "USER"}, {"uap-shared", "USER-SHARED"}} {
		if err := writeSealed(filepath.Join(base, "home", ".copilot", "skills", item.name, "SKILL.md"), skillBody(item.name, item.marker)); err != nil {
			return err
		}
	}
	return nil
}
func (c *controller) projectFixtures() error {
	for _, label := range []string{"A", "B"} {
		folder := filepath.Join(c.request.Base, "projects", label)
		if err := writeSealed(filepath.Join(folder, ".vscode", "mcp.json"), markerMCP(label)); err != nil {
			return err
		}
		for _, item := range []struct{ name, marker string }{{"uap-" + strings.ToLower(label), label}, {"uap-shared", label + "-SHARED"}} {
			if err := writeSealed(filepath.Join(folder, ".github", "skills", item.name, "SKILL.md"), skillBody(item.name, item.marker)); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *controller) probeFixture(label string) error {
	root := filepath.Join(c.request.Base, "probes", label)
	if err := os.Mkdir(root, 0755); err != nil {
		return err
	}
	for _, name := range []string{"profile", "extensions", "results"} {
		if err := c.ownedDirectory(filepath.Join(root, name)); err != nil {
			return err
		}
	}
	settings := []byte(`{"update.mode":"none","telemetry.telemetryLevel":"off","extensions.autoUpdate":false,"extensions.autoCheckUpdates":false,"workbench.startupEditor":"none","window.restoreWindows":"none","security.workspace.trust.enabled":true,"chat.mcp.autostart":"never"}` + "\n")
	if err := writeSealed(filepath.Join(root, "profile", "User", "settings.json"), settings); err != nil {
		return err
	}
	if err := writeSealed(filepath.Join(root, "profile", "User", "mcp.json"), markerMCP("USER")); err != nil {
		return err
	}
	// Only these declared cache leaves are writable; authored User files/parent remain sealed.
	for _, cache := range []string{"globalStorage", "workspaceStorage", "History", "snippets"} {
		if err := c.ownedDirectory(filepath.Join(root, "profile", "User", cache)); err != nil {
			return err
		}
	}
	return nil
}
func (c *controller) sourcePaths(label string) []string {
	base := c.request.Base
	return []string{filepath.Join(base, "observer"), filepath.Join(base, "projects"), filepath.Join(base, "home", ".copilot", "skills"), filepath.Join(base, "probes", label, "profile", "User", "settings.json"), filepath.Join(base, "probes", label, "profile", "User", "mcp.json")}
}
func (c *controller) checkPublisher() error {
	if err := immutableAncestors(filepath.Join(c.request.Base, "observer")); err != nil {
		return err
	}
	self := filepath.Join(c.request.Base, "control", "controller")
	if err := immutableAncestors(filepath.Dir(self)); err != nil {
		return err
	}
	if digest, err := hashFile(self, 64<<20); err != nil || digest != c.request.Input.HelperSHA {
		return errors.Join(fmt.Errorf("sealed controller binary changed"), err)
	}
	for _, name := range requiredPublisher {
		path := filepath.Join(c.request.Base, "publisher", publisherPrefix, name)
		if err := immutableAncestors(filepath.Dir(path)); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		uid, err := statUID(info)
		mode := os.FileMode(0444)
		if name == "code" || name == rgExecutable {
			mode = 0555
		}
		if err != nil || uid != 0 || info.Mode().Perm() != mode {
			return fmt.Errorf("sealed publisher owner/mode differs")
		}
		digest, err := hashFile(path, 512<<20)
		if err != nil || digest != c.hashes[name] {
			return errors.Join(fmt.Errorf("sealed publisher digest changed"), err)
		}
	}
	return nil
}
func (c *controller) setupDisplay() error {
	if err := c.prepareDisplayParent(); err != nil {
		return err
	}
	if err := c.systemIdentities(); err != nil {
		return err
	}
	return c.setupAuthority()
}
func (c *controller) prepareDisplayParent() error {
	parent := "/tmp/.X11-unix"
	if _, err := os.Lstat(parent); os.IsNotExist(err) {
		if err = os.Mkdir(parent, 0700); err != nil {
			return err
		}
		if err = os.Chmod(parent, os.ModeSticky|0777); err != nil {
			return err
		}
		c.createdDisplayParent, err = os.Lstat(parent)
		if err != nil {
			return err
		}
	}
	parentInfo, err := exactDirectory(parent, 0)
	if err != nil {
		return err
	}
	if parentInfo.Mode().Perm()&0002 != 0 && parentInfo.Mode()&os.ModeSticky == 0 {
		return fmt.Errorf("display parent is not protected by sticky ownership")
	}
	for _, name := range []string{"/tmp/.X93-lock", "/tmp/.X11-unix/X93"} {
		if _, err := os.Lstat(name); !os.IsNotExist(err) {
			return fmt.Errorf("display/lock collision; refusing to attach: %s", name)
		}
	}
	return nil
}
func (c *controller) systemIdentities() error {
	for _, path := range []string{"/usr/bin/Xvfb", "/usr/bin/xauth"} {
		if err := trustedSystem(path); err != nil {
			return err
		}
	}
	identities := map[string]any{}
	for _, path := range []string{"/usr/bin/Xvfb", "/usr/bin/xauth"} {
		digest, err := hashFile(path, 128<<20)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		identities[path] = map[string]any{"sha256": digest, "bytes": info.Size(), "version": "not inferred from binary identity"}
	}
	identityBody, err := encode(identities, controlLimit)
	if err != nil {
		return err
	}
	if err = c.keep("system-executables.json", identityBody); err != nil {
		return err
	}
	return nil
}
func (c *controller) setupAuthority() error {
	var err error
	auth := filepath.Join(c.request.Base, "auth")
	if err = os.Mkdir(auth, 0700); err != nil {
		return err
	}
	c.cookie, err = randomCookie()
	if err != nil {
		return err
	}
	file := filepath.Join(auth, "authority")
	if err = c.system("xauth", "/usr/bin/xauth", []string{"-f", file, "source", "-"}, []byte("add "+display+" . "+c.cookie+"\n"), true); err != nil {
		return err
	}
	authority, _, err := openOrdinary(file, 16<<10)
	if err != nil {
		return err
	}
	if err = authority.Close(); err != nil {
		return err
	}
	if err = os.Chown(auth, 0, c.request.Input.GID); err != nil {
		return err
	}
	if err = os.Chmod(auth, 0710); err != nil {
		return err
	}
	if err = os.Chown(file, 0, c.request.Input.GID); err != nil {
		return err
	}
	return os.Chmod(file, 0440)
}
func (c *controller) displayReady(ctx context.Context, proof isolationProof) error {
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		lock, err := readOrdinary("/tmp/.X93-lock", 64)
		if err == nil {
			lockInfo, le := os.Lstat("/tmp/.X93-lock")
			if le != nil {
				return le
			}
			info, se := os.Lstat("/tmp/.X11-unix/X93")
			if se == nil {
				if e := validateDisplayReady(lock, lockInfo, info, proof); e != nil {
					return e
				}
				after, ae := os.Lstat("/tmp/.X11-unix/X93")
				if ae != nil || !sameFile(info, after) {
					return fmt.Errorf("display socket changed during readiness")
				}
				c.displayAssets = map[string]os.FileInfo{"/tmp/.X93-lock": lockInfo, "/tmp/.X11-unix/X93": info}
				return nil
			}
			if !os.IsNotExist(se) {
				return se
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("display readiness exceeded ten seconds")
		case <-tick.C:
		}
	}
}
func (c *controller) removeA() error {
	base := filepath.Join(c.request.Base, "projects", "A")
	for _, path := range []string{filepath.Join(base, ".vscode", "mcp.json"), filepath.Join(base, ".github", "skills", "uap-a", "SKILL.md"), filepath.Join(base, ".github", "skills", "uap-shared", "SKILL.md")} {
		info, err := os.Lstat(path)
		if err != nil || !regular(info, 1<<20) {
			return errors.Join(fmt.Errorf("declared removal fixture differs"), err)
		}
		if err = os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}
func validateObservation(body []byte, base, folder string) (map[string]any, error) {
	value, err := uniqueJSON(body)
	if err != nil {
		return nil, err
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("observer result is not an object")
	}
	if err := validateObservationEnvelope(m); err != nil {
		return nil, err
	}
	if m["collected"] != true {
		return m, fmt.Errorf("observer stage failed: %.2048v", m["stage_error"])
	}
	if err := validateObservationContext(m, base, folder); err != nil {
		return m, err
	}
	if err := validateObservationPolicy(m); err != nil {
		return m, err
	}
	if err := validateObserverIdentity(m); err != nil {
		return m, err
	}
	if err := metadataRows(m["mcp"], metadataMCP); err != nil {
		return m, err
	}
	if err := metadataRows(m["skills"], metadataSkill); err != nil {
		return m, err
	}
	return m, nil
}
func (c *controller) observe(label string) error {
	deadline, err := deadlineAt(c.request.Input.StartNS, 420*time.Second)
	if err != nil || time.Until(deadline) < 65*time.Second {
		return fmt.Errorf("insufficient reserved budget for another GUI")
	}
	if label == "A-after-removal" {
		if err = c.removeA(); err != nil {
			return err
		}
	}
	folderLabel := label
	if label != "B" {
		folderLabel = "A"
	}
	folder := filepath.Join(c.request.Base, "projects", folderLabel)
	before, err := snapshot(c.sourcePaths(label))
	if err != nil {
		return err
	}
	if err = c.checkPublisher(); err != nil {
		return err
	}
	root := filepath.Join(c.request.Base, "probes", label)
	result := fixedResultPath(c.request.Base, label)
	if err = requireFreshResultPaths(result); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.work, 45*time.Second)
	defer cancel()
	var observed map[string]any
	_, runErr := c.strict(ctx, label, guiArgv(c.request.Base, folder, filepath.Join(root, "profile")), folder, c.environment(folder, result, filepath.Join(root, "profile")), func(exchange context.Context, _ isolationProof) error {
		var err error
		observed, err = c.waitObservation(exchange, label, result, folder)
		return err
	})
	after, afterErr := snapshot(c.sourcePaths(label))
	if afterErr != nil || !reflect.DeepEqual(before, after) {
		runErr = errors.Join(runErr, errors.Join(fmt.Errorf("authored source/fixture changed"), afterErr))
	}
	if e := c.checkPublisher(); e != nil {
		runErr = errors.Join(runErr, e)
	}
	if observed != nil {
		c.observations = append(c.observations, map[string]any{"probe": label, "metadata_collected": runErr == nil, "attribution": metadataAttribution(observed, folder, label == "A-after-removal"), "error": clipped(runErr)})
	}
	return runErr
}
func (c *controller) execute() (resultErr error) {
	attemptedDisplay := false
	reapedDisplay := false
	defer func() {
		if !attemptedDisplay {
			c.displayCleanupErr = c.cleanupCreatedDisplayParent()
		} else if reapedDisplay {
			c.displayCleanupErr = c.cleanupDisplay()
		} else {
			c.displayCleanupErr = c.unprovedDisplayCleanup()
		}
		resultErr = errors.Join(resultErr, c.displayCleanupErr)
	}()
	if err := c.fixtures(); err != nil {
		return err
	}
	if err := c.setupDisplay(); err != nil {
		return err
	}
	if err := c.work.Err(); err != nil {
		return err
	}
	env := c.environment(filepath.Join(c.request.Base, "projects", "A"), "", "")
	argv := []string{"/usr/bin/Xvfb", display, "-screen", "0", "1024x768x24", "-nolisten", "tcp", "-auth", filepath.Join(c.request.Base, "auth", "authority")}
	displayCtx, cancel := context.WithDeadline(context.Background(), c.deadline)
	defer cancel()
	attemptedDisplay = true
	record, err := runDisplayTable(c.work, func(exchange func(context.Context, isolationProof) error) (childRecord, error) {
		return c.strict(displayCtx, "Xvfb", argv, c.request.Base, env, exchange)
	}, c.displayReady, c.observe)
	reapedDisplay = record.TreeReaped
	return err
}
func (c *controller) waitObservation(ctx context.Context, label, result, folder string) (map[string]any, error) {
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		body, ready, err := admitReadyResult(result)
		if err != nil {
			return nil, err
		}
		if ready {
			if c.cookie != "" && strings.Contains(string(body), c.cookie) {
				return nil, fmt.Errorf("task cookie appeared in result; raw body withheld")
			}
			if err = c.keep(label+".observation.json", body); err != nil {
				return nil, err
			}
			return validateObservation(body, c.request.Base, folder)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tick.C:
		}
	}
}
func validateObservationEnvelope(m map[string]any) error {
	if m["collected"] == true {
		if _, present := m["stage_error"]; present {
			return fmt.Errorf("successful observer envelope contains stage_error")
		}
	}
	allowed := map[string]bool{}
	for _, key := range []string{"schema_version", "collected", "stage_error", "version", "app_root", "app_host", "observer", "folder", "trusted", "policy", "mcp", "skills", "native_project_qualified", "native_mcp_initialization_qualified", "native_runtime_qualified"} {
		allowed[key] = true
	}
	for key := range m {
		if !allowed[key] {
			return fmt.Errorf("unknown observer result field")
		}
	}
	if m["schema_version"] != json.Number("1") || m["native_project_qualified"] != false || m["native_mcp_initialization_qualified"] != false || m["native_runtime_qualified"] != false {
		return fmt.Errorf("observer schema/qualification differs")
	}
	return nil
}
func validateObservationContext(m map[string]any, base, folder string) error {
	if m["version"] != publisherVersion || m["app_root"] != filepath.Join(base, "publisher", publisherPrefix, "resources", "app") || m["app_host"] != "desktop" || m["trusted"] != false {
		return fmt.Errorf("publisher/context differs")
	}
	if err := metadataURI(m["folder"]); err != nil {
		return err
	}
	selected, ok := m["folder"].(map[string]any)
	if !ok || selected["scheme"] != "file" || selected["authority"] != "" || selected["path"] != folder || selected["value"] != "file://"+folder {
		return fmt.Errorf("selected native folder differs")
	}
	return nil
}
func validateObservationPolicy(m map[string]any) error {
	policy, policyErr := objectKeys(m["policy"], []string{"skills_enabled", "mcp_access", "mcp_autostart"}, nil)
	ok := policyErr == nil
	if !ok || policy["mcp_autostart"] != "never" {
		return fmt.Errorf("actual restrictive policy missing")
	}
	if _, ok = policy["skills_enabled"].(bool); !ok {
		return fmt.Errorf("unknown skills policy type")
	}
	if value := policy["mcp_access"]; value != "all" && value != "none" && value != "registry" {
		return fmt.Errorf("unknown MCP policy type")
	}
	return nil
}
func validateObserverIdentity(m map[string]any) error {
	observer, observerErr := objectKeys(m["observer"], []string{"id", "mode", "proposals"}, nil)
	ok := observerErr == nil
	if !ok || observer["id"] != "uap-research.workspace-observer" || observer["mode"] != json.Number("3") || !reflect.DeepEqual(observer["proposals"], []any{"mcpServerDefinitions", "chatPromptFiles"}) {
		return fmt.Errorf("observer identity/proposals differ")
	}
	return nil
}
