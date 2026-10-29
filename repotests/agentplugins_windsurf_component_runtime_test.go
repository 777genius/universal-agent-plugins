//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const windRuntimeLogs = "xdg-data\x00devin/cli/logs"
const windRuntimeLock = windRuntimeLogs + "/.log-maintenance.lock"
const windRuntimeTelemetry = "xdg-cache\x00devin/telemetry_state.json"
const windRuntimeTelemetryBody = `{"is_zdr":false}`
const windRuntimePlugins = "xdg-data\x00devin/cli/plugins"
const windRuntimePluginLock = windRuntimePlugins + "/lock.json"
const windRuntimePluginLockSHA = "cbce3a5464482eb4cfb7b152ad15183bcb5db5c5557c625ee0cec1edb582aa88"

var windRuntimeLogName = regexp.MustCompile(`^devin_(\d{8}-\d{6})_([1-9]\d{0,9})\.log$`)

func (w *windComponentFixture) prepareRuntime(t *testing.T) {
	t.Helper()
	f := w.f
	path := filepath.Join(f.base.Root, "xdg-config", "devin", "config.json")
	_, original := projectNativeCLIUpdater("windsurf")
	body, err := projectNativeRuntimeRead(path, 4096)
	if err != nil || len(f.used) != 0 || !bytes.Equal(body, original) || f.protected[path] != windsurfConfigDigest(original) {
		t.Fatal("Windsurf startup recipe requires the untouched pre-child config")
	}
	// Version 1 takes the pinned loader's no-schema-write branch. This is
	// deliberate initial fixture authoring, before any vendor child or baseline.
	body = append([]byte("{\"version\":1,"), original[1:]...)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	f.protected[path] = windsurfConfigDigest(body)
	id, err := domain.NewInstallationID()
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"xdg-config/devin/cli", "xdg-cache/devin/cli", "xdg-data/devin/cli/logs", "xdg-data/devin/cli/plugins", "xdg-data/devin/mcp/oauth"} {
		if err := os.MkdirAll(filepath.Join(f.base.Root, relative), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for relative, raw := range map[string][]byte{
		"xdg-data/devin/cli/installation_id":        []byte(id),
		"xdg-data/devin/cli/plugins/lock.json.lock": {},
		"xdg-cache/devin/telemetry_state.json":      []byte(windRuntimeTelemetryBody),
		"xdg-data/devin/cli/plugins/lock.json": []byte(`{
  "requirements": [],
  "resolved": [],
  "edges": []
}`),
	} {
		path := filepath.Join(f.base.Root, relative)
		projectNativeRuntimeWrite(t, path, raw, 0644)
		f.protected[path] = windsurfConfigDigest(raw)
	}
	for _, relative := range []string{"xdg-config/devin", "xdg-cache/devin", "xdg-data/devin"} {
		w.freeze(t, filepath.Join(f.base.Root, relative))
	}
}

// Copy only the already validated held frame. A later filesystem observation
// must match this authority; it can never become a replacement baseline.
func (w *windComponentFixture) freezeRuntime(frame map[string]windProfileRecord) {
	const prefix = "xdg-data\x00devin"
	tree := map[string]windProfileRecord{}
	for key, record := range frame {
		if key == prefix {
			tree["."] = record
		} else if strings.HasPrefix(key, prefix+"/") {
			tree[strings.TrimPrefix(key, prefix+"/")] = record
		}
	}
	w.trees[filepath.Join(w.f.base.Root, "xdg-data", "devin")] = tree
	w.trees[filepath.Join(w.f.base.Root, "xdg-cache", "devin")]["telemetry_state.json"] = frame[windRuntimeTelemetry]
}

func windRuntimeAuthority(a, b windProfileRecord) bool {
	return a.Kind == b.Kind && a.Device == b.Device && a.Inode == b.Inode && a.Mode == b.Mode && a.UID == b.UID && a.GID == b.GID && a.Links == b.Links
}

func windRuntimeFile(record windProfileRecord, limit int64) bool {
	return record.Kind == "file" && record.Mode == 0100644 && record.UID == uint32(os.Getuid()) && record.GID == uint32(os.Getgid()) && record.Links == 1 && record.Size >= 0 && record.Size <= limit
}

func windRuntimeCommandTimes(record windProfileRecord, start, end time.Time) bool {
	for _, stamp := range [][2]int64{record.Modified, record.Changed} {
		if stamp[1] < 0 || stamp[1] >= int64(time.Second) {
			return false
		}
		when := time.Unix(stamp[0], stamp[1])
		if when.Before(start) || when.After(end) {
			return false
		}
	}
	return true
}

func windRuntimeEmptyRegistry(old, now windProfileRecord, start, end time.Time) bool {
	return windRuntimeFile(old, 57) && windRuntimeFile(now, 57) && old.Inode > 0 && now.Inode > 0 && old.Device == now.Device && old.Size == 57 && now.Size == 57 && old.SHA256 == windRuntimePluginLockSHA && now.SHA256 == old.SHA256 && windRuntimeCommandTimes(now, start, end)
}

func windRuntimeRegistryParent(old, now windProfileRecord, start, end time.Time) bool {
	return windRuntimeAuthority(old, now) && old.Kind == "directory" && old.Mode == 040755 && old.UID == uint32(os.Getuid()) && old.GID == uint32(os.Getgid()) && old.Size == now.Size && old.SHA256 == now.SHA256 && windRuntimeCommandTimes(now, start, end)
}

func windRuntimePID(key string, start, end time.Time) (string, error) {
	name := strings.TrimPrefix(key, windRuntimeLogs+"/")
	parts := windRuntimeLogName.FindStringSubmatch(name)
	if len(parts) != 3 || key != windRuntimeLogs+"/"+name || end.Before(start) || end.Sub(start) > 10*time.Second {
		return "", fmt.Errorf("Windsurf native log name/interval")
	}
	stamp, err := time.Parse("20060102-150405", parts[1])
	pid, pidErr := strconv.ParseUint(parts[2], 10, 31)
	if err != nil || pidErr != nil || pid == 0 || stamp.Before(start.Truncate(time.Second)) || stamp.After(end) {
		return "", fmt.Errorf("Windsurf native log calendar/PID/window")
	}
	return parts[2], nil
}

func (w *windComponentFixture) runtimeClosure(before, after map[string]windProfileRecord, start, end time.Time) error {
	registryChanged := before[windRuntimePluginLock] != after[windRuntimePluginLock] && windRuntimeEmptyRegistry(before[windRuntimePluginLock], after[windRuntimePluginLock], start, end)
	for key, old := range before {
		now, ok := after[key]
		if !ok {
			return fmt.Errorf("Windsurf native removed original profile entry")
		}
		if old == now {
			continue
		}
		if key == windRuntimeTelemetry && windRuntimeAuthority(old, now) && windRuntimeFile(now, 16) && old.Size == 16 && now.Size == 16 && old.SHA256 == windsurfConfigDigest([]byte(windRuntimeTelemetryBody)) && now.SHA256 == old.SHA256 {
			continue // The pinned native cache writer may refresh only its timestamps.
		}
		if registryChanged && (key == windRuntimePluginLock || key == windRuntimePlugins && windRuntimeRegistryParent(old, now, start, end)) {
			continue // Only the pinned empty registry may be atomically replaced.
		}
		if key != windRuntimeLogs && key != windRuntimeLock || !windRuntimeAuthority(old, now) {
			return fmt.Errorf("Windsurf native changed original profile authority")
		}
	}
	pid, count, total := "", 0, int64(0)
	for key, record := range after {
		if strings.HasPrefix(key, windRuntimeLogs+"/") && key != windRuntimeLock {
			if !windRuntimeFile(record, 1<<20) {
				return fmt.Errorf("Windsurf native log ordinary file/bound")
			}
			total += record.Size
		}
		if _, existed := before[key]; existed {
			continue
		}
		if key == windRuntimeLock {
			continue
		}
		var err error
		pid, err = windRuntimePID(key, start, end)
		if err != nil || !windRuntimeFile(record, 1<<20) {
			return fmt.Errorf("Windsurf native added unknown profile entry")
		}
		count++
	}
	lock, ok := after[windRuntimeLock]
	if count != 1 || total > 16<<20 || !ok || !windRuntimeFile(lock, 11) {
		return fmt.Errorf("Windsurf native complete log/lock ledger")
	}
	body, err := projectNativeRuntimeRead(filepath.Join(w.f.base.Root, "xdg-data", "devin", "cli", "logs", ".log-maintenance.lock"), 11)
	if err != nil || windsurfConfigDigest(body) != lock.SHA256 || string(body) != pid+"\n" {
		return fmt.Errorf("Windsurf native maintenance self-PID differs")
	}
	return nil
}
