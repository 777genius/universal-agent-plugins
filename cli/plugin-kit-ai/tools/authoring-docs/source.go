package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Rebase requires explicit source revalidation; this is not a release identity.
const preparedSourceSHA = "070663efb27f69ecae8609e6b839f86f843efbb0"

type sourcePin struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func git(checkout string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", checkout}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	body, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("source checkout verification failed")
	}
	return body, nil
}
func validateSource(checkout, sha string) ([]sourcePin, error) {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(sha) || sha != preparedSourceSHA {
		return nil, fmt.Errorf("source identity must be the exact audited preparation SHA %s", preparedSourceSHA)
	}
	head, err := git(checkout, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(head)) != sha {
		return nil, fmt.Errorf("source identity differs from checkout HEAD")
	}
	// Includes staged changes. New adapter/docs files are allowed at this base.
	changed, err := git(checkout, "diff", "--name-only", sha, "--")
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(changed)) != 0 {
		return nil, fmt.Errorf("tracked checkout differs from source identity")
	}
	added, err := git(checkout, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Split(string(added), "\x00") {
		if strings.HasSuffix(name, ".go") && !strings.HasPrefix(name, "cli/plugin-kit-ai/tools/authoring-docs/") {
			return nil, fmt.Errorf("untracked Go input prevents exact source verification")
		}
	}
	pins := make([]sourcePin, 0, len(factoryPins))
	for _, pin := range factoryPins {
		body, err := os.ReadFile(filepath.Join(checkout, filepath.FromSlash(pin.Path)))
		if err != nil {
			return nil, fmt.Errorf("source input unavailable: %s", pin.Path)
		}
		if fmt.Sprintf("%x", sha256.Sum256(body)) != pin.SHA256 {
			return nil, fmt.Errorf("source input mismatch: %s", pin.Path)
		}
		pins = append(pins, pin)
	}
	return pins, nil
}

// Exact command/flag/composition and dependency source inputs.
var factoryPins = []sourcePin{
	{"cli/plugin-kit-ai/cmd/agentplugins/release_root.go", "c0465f90903c7ad3fcdc2283c241558d1b73bd9633f0e6af247ccd692a0e155e"},
	{"cli/plugin-kit-ai/cmd/plugin-kit-ai/release_compat.go", "d2f11a845c116160114ddd1b0ba8b825e9e5a4794354742749db7122c0c674c8"},
	{"cli/plugin-kit-ai/go.mod", "c19d163f4cab4b7a3cdd6c71bec885809c81beaf4aee705071b90780bc9a15eb"},
	{"cli/plugin-kit-ai/go.sum", "8d0b8892b2f6528b9bfecf83d8efda5533ae696d2506c061df9bf4d88ede3eb0"},
	{"cli/plugin-kit-ai/internal/agentpluginscli/root.go", "97dac1600168a64835cc9aeeb998f56d5796e566572d9530f3f6537f8ad5ae04"},
	{"cli/plugin-kit-ai/internal/authoring/commands/commands.go", "6f581be57435b8f269b80a6c1efca62e8369a1a7ca39dd215cecf15a1e59d568"},
	{"cli/plugin-kit-ai/internal/authoring/commands/public_contract.go", "39c79f491f0733d352ffc0fa3a8ff4eaa169612e8876e92e5fb74c856663ec65"},
	{"cli/plugin-kit-ai/internal/authoring/commands/skills.go", "566e36e02d58c5d76e92369987d901feb1bcb6f3e59d3225e02961fa041d606b"},
	{"cli/plugin-kit-ai/internal/authoring/commands/version.go", "ffe6cfef352faeb9a3c00722a628a2093876c14120cd6725131b3e105fda6b29"},
	{"cli/plugin-kit-ai/internal/authoringcli/command.go", "eaf18b18c8aeaecbc2e63c780a1472097855315a4bcdc22e7eb0386aa1375012"},
	{"cli/plugin-kit-ai/internal/authoringcli/flags.go", "5f745c810e90586899cff2170433dbc58483ba17496167738387388f29be49c0"},
	{"cli/plugin-kit-ai/internal/authoringcli/release.go", "45cb7a7000a31bb8477d1301fc70ce4521140b9f2a4760d5f84da3283134877f"},
}
