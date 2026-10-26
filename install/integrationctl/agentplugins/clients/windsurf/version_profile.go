package windsurf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ProjectVersionRequirement   = (*Adapter)(nil)
	_ clients.HostVersionProbeEnvironment = (*Adapter)(nil)
)

func (*Adapter) RequiresProjectVersion(client domain.DetectedClient) bool {
	return client.ClientID == domain.ClientWindsurf && primaryDevinExecutable(client.ExecutablePath)
}

func primaryDevinExecutable(executable string) bool {
	name := strings.ToLower(filepath.Base(executable))
	return filepath.IsAbs(executable) && filepath.Clean(executable) == executable && (name == "devin" || name == "devin.exe")
}

// ConfigRoot is Cascade mutation authority and can intentionally be empty.
// Bind the version child only to original host facts already held by Detector.
func (adapter *Adapter) VersionProbeEnvironmentForHost(host clients.Host, client domain.DetectedClient) ([]string, error) {
	if !adapter.RequiresProjectVersion(client) {
		return nil, nil
	}
	if err := devinVersionDirectory(host, host.HomeDir(), true); err != nil {
		return nil, fmt.Errorf("devin original home: %w", err)
	}
	environment := []string{"HOME=" + host.HomeDir()}
	var keys []string
	switch host.GOOS() {
	case "linux", "darwin":
		keys = []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"}
	case "windows":
		if host.Env("APPDATA") == "" {
			return nil, fmt.Errorf("devin original APPDATA is required for version probing")
		}
		environment = append(environment, "USERPROFILE="+host.HomeDir())
		keys = []string{"APPDATA", "LOCALAPPDATA"}
	default:
		return nil, fmt.Errorf("devin version environment is unsupported on this platform")
	}
	for _, key := range keys {
		value := host.Env(key)
		if value == "" {
			continue
		}
		if err := devinVersionDirectory(host, value, false); err != nil {
			return nil, fmt.Errorf("devin original %s: %w", key, err)
		}
		environment = append(environment, key+"="+value)
	}
	return environment, nil
}

func devinVersionDirectory(host clients.Host, path string, required bool) error {
	if path == "" || strings.TrimSpace(path) != path || strings.ContainsAny(path, "\x00\r\n") || !utf8.ValidString(path) || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("expected a clean absolute original directory")
	}
	canonical, err := host.CanonicalDirectory(path)
	if err != nil {
		return err
	}
	if canonical != path {
		return fmt.Errorf("original directory contains an alias")
	}
	info, err := host.Lstat(path)
	if err != nil {
		if required || !os.IsNotExist(err) {
			return err
		}
		info, err = host.Lstat(filepath.Dir(path))
		if err != nil {
			return err
		}
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("original directory or its parent is not ordinary")
	}
	return nil
}
