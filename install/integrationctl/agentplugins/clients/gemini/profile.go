package gemini

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ProfileResolver         = (*Adapter)(nil)
	_ clients.ProfileBindingValidator = (*Adapter)(nil)
	_ clients.VersionProbeEnvironment = (*Adapter)(nil)
)

func profileSpelling(root string) error {
	if root == "" || strings.TrimSpace(root) != root || strings.ContainsAny(root, "\x00\r\n") ||
		!utf8.ValidString(root) || !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
		return fmt.Errorf("Gemini config root must be an explicit clean absolute directory")
	}
	return nil
}

// ResolveProfileRoot resolves an explicit config directory itself, not the
// native GEMINI_CLI_HOME parent. Reject ambiguous spelling before following
// aliases; missing suffixes remain missing and are never created here.
func (*Adapter) ResolveProfileRoot(root string) (string, error) {
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	canonical, err := clients.CanonicalDirectory(root, os.Lstat, filepath.EvalSymlinks)
	if err != nil {
		return "", err
	}
	if err := profileSpelling(canonical); err != nil {
		return "", err
	}
	return canonical, nil
}

// v0.62.0 paths.ts homedir() returns any nonempty GEMINI_CLI_HOME unchanged;
// storage.ts getGlobalGeminiDir() appends .gemini. Relative spelling therefore
// uses the original native cwd, and padding is literal directory data.
func detectedProfileRoot(host clients.Host) (string, error) {
	parent := host.Env("GEMINI_CLI_HOME")
	if parent == "" {
		parent = host.HomeDir()
	}
	if parent == "" || strings.ContainsAny(parent, "\x00\r\n") || !utf8.ValidString(parent) {
		return "", fmt.Errorf("Gemini native home parent is unavailable or unsupported")
	}
	if !filepath.IsAbs(parent) {
		cwd := host.WorkingDir()
		if !filepath.IsAbs(cwd) || filepath.Clean(cwd) != cwd || strings.ContainsAny(cwd, "\x00\r\n") || !utf8.ValidString(cwd) {
			return "", fmt.Errorf("relative GEMINI_CLI_HOME requires the original absolute working directory")
		}
		// Drive-relative/rooted Windows paths need process drive state that Host
		// does not supply. Refuse rather than guess another profile.
		if filepath.VolumeName(parent) != "" || strings.HasPrefix(parent, string(filepath.Separator)) ||
			filepath.Separator == '\\' && strings.HasPrefix(parent, "/") {
			return "", fmt.Errorf("ambiguous GEMINI_CLI_HOME directory")
		}
		parent = filepath.Join(host.WorkingDir(), parent)
	}
	root := filepath.Join(parent, ".gemini")
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	root, err := host.CanonicalDirectory(root)
	if err != nil {
		return "", err
	}
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	return root, nil
}

func validateProfile(root, planned string) error {
	canonical, err := New().ResolveProfileRoot(root)
	if err != nil {
		return err
	}
	if canonical != root {
		return fmt.Errorf("Gemini config root must be canonical before native operations")
	}
	if planned != "" && planned != root {
		return fmt.Errorf("Gemini planned registry root differs from client config root")
	}
	return nil
}

// VersionProbeEnvironment pins the probe to selected physical file authority.
// Native v0.62.0 can express only parent/.gemini. An arbitrary config directory
// cannot be safely encoded as GEMINI_CLI_HOME=<config-root>: that selects a
// nested .gemini. This environment is not evidence of native startup readiness.
func (*Adapter) VersionProbeEnvironment(root string) ([]string, error) {
	if err := validateProfile(root, ""); err != nil {
		return nil, err
	}
	if filepath.Base(root) != ".gemini" {
		return nil, fmt.Errorf("selected Gemini config root cannot be represented by native GEMINI_CLI_HOME")
	}
	return []string{"GEMINI_CLI_HOME=" + filepath.Dir(root)}, nil
}

// ValidateBindingProfile accepts recorded physical authority or exact existing
// native path/digest ownership. Legacy names, package locators and ambient home
// are not profile evidence. Missing legacy objects cannot establish authority.
func (*Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	return validateBindingProfile(root, binding, nativeconfig.New())
}

func validateBindingProfile(root string, binding domain.ClientBinding, kernel nativeconfig.Kernel) error {
	if err := validateProfile(root, ""); err != nil {
		return err
	}
	if binding.NativeProfileRoot != "" {
		if err := validateProfile(binding.NativeProfileRoot, root); err != nil {
			return fmt.Errorf("recorded Gemini profile mismatch: %w", err)
		}
		return validateGeminiObjectPaths(root, binding.NativeObjects)
	}
	objects := GeminiObjects(binding.NativeObjects)
	if len(objects) == 0 {
		return fmt.Errorf("legacy Gemini binding has no exact native ownership evidence")
	}
	if err := VerifyGeminiNativeObjects(root, objects, false, kernel); err != nil {
		return fmt.Errorf("legacy Gemini binding profile is unproven: %w", err)
	}
	return nil
}
