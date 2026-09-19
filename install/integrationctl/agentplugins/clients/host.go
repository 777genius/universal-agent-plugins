package clients

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// HostProbes is the raw environment a Host is built from: the values an adapter
// may read and the three filesystem probes it observes through. The surface
// constructors are deliberately not part of it - they translate a probe result
// into an evidence string, which is a cross-client output contract and stays in
// one implementation shared by the detector and by contracttest.
type HostProbes struct {
	HomeDir               string
	GOOS                  string
	Environment           map[string]string
	SystemApplicationsDir string
	WindowsProgramFiles   []string
	LinuxApplicationDirs  []string

	LookPath func(name string) (string, error)
	Lstat    func(path string) (fs.FileInfo, error)
	ReadDir  func(path string) ([]os.DirEntry, error)
}

// NewHost builds the probing environment handed to an adapter.
func NewHost(probes HostProbes) Host { return host{probes: probes} }

type host struct{ probes HostProbes }

func (h host) HomeDir() string                { return h.probes.HomeDir }
func (h host) GOOS() string                   { return h.probes.GOOS }
func (h host) Env(name string) string         { return h.probes.Environment[name] }
func (h host) SystemApplicationsDir() string  { return h.probes.SystemApplicationsDir }
func (h host) WindowsProgramFiles() []string  { return h.probes.WindowsProgramFiles }
func (h host) LinuxApplicationDirs() []string { return h.probes.LinuxApplicationDirs }

func (h host) LookPath(binary string) string {
	if h.probes.LookPath == nil {
		return ""
	}
	path, err := h.probes.LookPath(binary)
	if err != nil || strings.TrimSpace(path) == "" {
		return ""
	}
	return path
}

// Lstat reports an error rather than falling back to the real filesystem: a
// host built without the probe must not observe the machine it runs on.
func (h host) Lstat(path string) (fs.FileInfo, error) {
	if h.probes.Lstat == nil {
		return nil, fmt.Errorf("host has no Lstat probe")
	}
	return h.probes.Lstat(path)
}

func (h host) ReadDir(path string) ([]os.DirEntry, error) {
	if h.probes.ReadDir == nil {
		return nil, fmt.Errorf("host has no ReadDir probe")
	}
	return h.probes.ReadDir(path)
}

func (h host) BinarySurface(id, binary string) domain.ClientSurface {
	return h.ResolvedBinarySurface(id, h.LookPath(binary))
}

func (h host) ResolvedBinarySurface(id, executablePath string) domain.ClientSurface {
	found := executablePath != ""
	return domain.ClientSurface{ID: id, Detected: found, Evidence: evidence(found, "executable_on_path")}
}

func (h host) DirectorySurface(id, path string) domain.ClientSurface {
	detected := h.realDirectory(path)
	return domain.ClientSurface{ID: id, Detected: detected, Evidence: evidence(detected, "configuration_directory")}
}

func (h host) AppSurface(id, appName string) domain.ClientSurface {
	paths := []string{filepath.Join(h.HomeDir(), "Applications", appName)}
	if strings.TrimSpace(h.SystemApplicationsDir()) != "" {
		paths = append(paths, filepath.Join(h.SystemApplicationsDir(), appName))
	}
	for _, path := range paths {
		if h.realDirectory(path) {
			return domain.ClientSurface{ID: id, Detected: true, Evidence: "application_bundle"}
		}
	}
	return domain.ClientSurface{ID: id}
}

func (h host) WindowsAppSurface(id, userRelativePath, systemRelativePath string) domain.ClientSurface {
	paths := []string{}
	if root := strings.TrimSpace(h.Env("LOCALAPPDATA")); root != "" {
		paths = append(paths, filepath.Join(root, userRelativePath))
	}
	for _, root := range h.WindowsProgramFiles() {
		if strings.TrimSpace(root) != "" {
			paths = append(paths, filepath.Join(root, systemRelativePath))
		}
	}
	for _, path := range paths {
		if h.realFile(path) {
			return domain.ClientSurface{ID: id, Detected: true, Evidence: "application_installation"}
		}
	}
	return domain.ClientSurface{ID: id}
}

func (h host) LinuxDesktopSurface(id string, filenames ...string) domain.ClientSurface {
	for _, root := range h.LinuxApplicationDirs() {
		for _, filename := range filenames {
			if h.realFile(filepath.Join(root, filename)) {
				return domain.ClientSurface{ID: id, Detected: true, Evidence: "desktop_entry"}
			}
		}
	}
	return domain.ClientSurface{ID: id}
}

func (h host) ExtensionSurface(id, root string, extensionIDs ...string) domain.ClientSurface {
	if !h.realDirectory(root) {
		return domain.ClientSurface{ID: id}
	}
	entries, err := h.ReadDir(root)
	if err != nil {
		return domain.ClientSurface{ID: id}
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		name := strings.ToLower(entry.Name())
		for _, extensionID := range extensionIDs {
			extensionID = strings.ToLower(extensionID)
			if name == extensionID || strings.HasPrefix(name, extensionID+"-") {
				return domain.ClientSurface{ID: id, Detected: true, Evidence: "editor_extension"}
			}
		}
	}
	return domain.ClientSurface{ID: id}
}

func (h host) VSCodeConfigRoot() string {
	switch h.GOOS() {
	case "darwin":
		return filepath.Join(h.HomeDir(), "Library", "Application Support", "Code", "User")
	case "windows":
		if root := strings.TrimSpace(h.Env("APPDATA")); root != "" {
			return filepath.Join(root, "Code", "User")
		}
	default:
		if root := strings.TrimSpace(h.Env("XDG_CONFIG_HOME")); root != "" {
			return filepath.Join(root, "Code", "User")
		}
		return filepath.Join(h.HomeDir(), ".config", "Code", "User")
	}
	return filepath.Join(h.HomeDir(), ".config", "Code", "User")
}

func (h host) EditorChannelConfigRoot(channel string) string {
	switch h.GOOS() {
	case "darwin":
		return filepath.Join(h.HomeDir(), "Library", "Application Support", channel, "User")
	case "windows":
		if root := strings.TrimSpace(h.Env("APPDATA")); root != "" {
			return filepath.Join(root, channel, "User")
		}
	}
	return filepath.Join(h.xdgConfigBase(), channel, "User")
}

func (h host) XDGConfigRoot(name string) string {
	return filepath.Join(h.xdgConfigBase(), name)
}

func (h host) xdgConfigBase() string {
	if root := strings.TrimSpace(h.Env("XDG_CONFIG_HOME")); root != "" {
		return root
	}
	return filepath.Join(h.HomeDir(), ".config")
}

func (h host) realDirectory(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := h.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

func (h host) realFile(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := h.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0
}

func evidence(ok bool, value string) string {
	if ok {
		return value
	}
	return ""
}
