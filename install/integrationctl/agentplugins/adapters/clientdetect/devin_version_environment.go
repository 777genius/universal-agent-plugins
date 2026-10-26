package clientdetect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

func primaryDevinExecutable(executable string) bool {
	name := strings.ToLower(filepath.Base(executable))
	return filepath.IsAbs(executable) && filepath.Clean(executable) == executable && (name == "devin" || name == "devin.exe")
}

// ConfigRoot is Cascade mutation authority and can intentionally be empty.
// Bind the version child only to original host facts already held by Detector.
func (detector Detector) devinVersionEnvironment() ([]string, error) {
	if err := detector.devinVersionDirectory(detector.HomeDir, true); err != nil {
		return nil, fmt.Errorf("Devin original home: %w", err)
	}
	environment := []string{"HOME=" + detector.HomeDir}
	var keys []string
	switch detector.GOOS {
	case "linux", "darwin":
		keys = []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"}
	case "windows":
		if detector.Environment["APPDATA"] == "" {
			return nil, fmt.Errorf("Devin original APPDATA is required for version probing")
		}
		environment = append(environment, "USERPROFILE="+detector.HomeDir)
		keys = []string{"APPDATA", "LOCALAPPDATA"}
	default:
		return nil, fmt.Errorf("Devin version environment is unsupported on this platform")
	}
	for _, key := range keys {
		value := detector.Environment[key]
		if value == "" {
			continue
		}
		if err := detector.devinVersionDirectory(value, false); err != nil {
			return nil, fmt.Errorf("Devin original %s: %w", key, err)
		}
		environment = append(environment, key+"="+value)
	}
	return environment, nil
}

func (detector Detector) devinVersionDirectory(path string, required bool) error {
	if path == "" || strings.TrimSpace(path) != path || strings.ContainsAny(path, "\x00\r\n") || !utf8.ValidString(path) || !filepath.IsAbs(path) || filepath.Clean(path) != path || detector.Lstat == nil {
		return fmt.Errorf("expected a clean absolute original directory")
	}
	canonical, err := clients.CanonicalDirectory(path, detector.Lstat, detector.EvalSymlinks)
	if err != nil {
		return err
	}
	if canonical != path {
		return fmt.Errorf("original directory contains an alias")
	}
	info, err := detector.Lstat(path)
	if err != nil {
		if required || !os.IsNotExist(err) {
			return err
		}
		info, err = detector.Lstat(filepath.Dir(path))
		if err != nil {
			return err
		}
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("original directory or its parent is not ordinary")
	}
	return nil
}
