package hermes

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// This is a distribution/capability boundary for an explicitly selected trusted
// client. It is not a signature verifier or a portable build-preparation receipt.
const hermesBundleRef = "2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19"

type hermesBundle struct {
	executable, root, digest string
}

func qualifyHermesBundle(executable string) (hermesBundle, error) {
	if !cleanRoot(executable) {
		return hermesBundle{}, fmt.Errorf("hermes executable must be absolute and clean")
	}
	executable, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return hermesBundle{}, err
	}
	root := filepath.Dir(filepath.Dir(executable))
	original, err := os.Stat(root)
	if err != nil {
		return hermesBundle{}, err
	}
	if !original.IsDir() {
		return hermesBundle{}, fmt.Errorf("hermes bundle root is not a directory")
	}
	name := "hermes"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if executable != filepath.Join(root, "bin", name) {
		return hermesBundle{}, fmt.Errorf("source-installed Hermes is not qualified for a Project session")
	}
	manifest, err := atomicfile.ReadBoundedRegularFile(filepath.Join(root, "manifest.json"), 64<<10)
	if err != nil {
		return hermesBundle{}, err
	}
	if err := validateHermesBundle(root, name, manifest); err != nil {
		return hermesBundle{}, err
	}
	launcher, err := atomicfile.ReadBoundedRegularFile(executable, 2<<20)
	if err != nil {
		return hermesBundle{}, err
	}
	current, err := os.Stat(root)
	if err != nil || !os.SameFile(original, current) {
		return hermesBundle{}, fmt.Errorf("hermes bundle root changed")
	}
	digest := fmt.Sprintf("%x:%x", sha256.Sum256(manifest), sha256.Sum256(launcher))
	return hermesBundle{executable: executable, root: root, digest: digest}, nil
}

func validateHermesBundle(root, launcher string, body []byte) error {
	doc, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return err
	}
	if err := hermesBundleHeader(doc); err != nil {
		return err
	}
	selected, ok := doc["runtime"].(map[string]any)
	if !ok || len(selected) != 5 || selected["repoDir"] != "hermes-agent" || selected["toolsDir"] != "tools" {
		return fmt.Errorf("hermes contained runtime selection differs")
	}
	if err := hermesBundleLaunchers(doc["launchers"], selected["commands"], launcher); err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(root, "hermes-agent", ".git")); !os.IsNotExist(err) {
		return fmt.Errorf("hermes Project sessions require a contained snapshot")
	}
	for _, key := range []string{"repoDir", "toolsDir", "storePython", "sitePackages"} {
		if err := hermesBundlePath(root, key, selected[key]); err != nil {
			return err
		}
	}
	if _, err := os.Lstat(filepath.Join(root, "hermes-agent", ".env")); !os.IsNotExist(err) {
		return fmt.Errorf("hermes bundle has a dotenv source or unavailable dotenv authority")
	}
	return nil
}

func hermesBundleHeader(doc map[string]any) error {
	target := map[string]string{"linux/amd64": "linux-x64", "darwin/arm64": "darwin-arm64", "windows/amd64": "win32-x64"}[runtime.GOOS+"/"+runtime.GOARCH]
	if target == "" || len(doc) != 8 || doc["schema"] != json.Number("1") ||
		doc["ref"] != hermesBundleRef || doc["target"] != target ||
		doc["repo"] != "hermes-agent" || doc["venv"] != "venv" || doc["store"] != "tools" {
		return fmt.Errorf("hermes bundle revision, target or layout is unqualified")
	}
	return nil
}

func hermesBundleLaunchers(names, mapping any, launcher string) error {
	commands, ok := mapping.(map[string]any)
	launchers, listOK := names.([]any)
	if !ok || !listOK || len(launchers) == 0 || len(launchers) > 32 ||
		len(commands) != len(launchers) || commands["hermes"] != "bin/"+launcher {
		return fmt.Errorf("hermes bundle launcher selection differs")
	}
	seen := map[string]bool{}
	suffix := strings.TrimPrefix(launcher, "hermes")
	for _, entry := range launchers {
		name, valid := entry.(string)
		if !valid || pathpolicy.ValidateLeafID(name) != nil || seen[name] || commands[name] != "bin/"+name+suffix {
			return fmt.Errorf("hermes bundle launcher list differs")
		}
		seen[name] = true
	}
	return nil
}

func hermesBundlePath(root, key string, value any) error {
	relative, ok := value.(string)
	if !ok || relative == "" || filepath.IsAbs(relative) || filepath.ToSlash(filepath.Clean(relative)) != relative {
		return fmt.Errorf("hermes runtime path is not a clean relative path")
	}
	if key == "storePython" && !strings.HasPrefix(relative, "tools/") ||
		key == "sitePackages" && !strings.HasPrefix(relative, "venv/") {
		return fmt.Errorf("hermes interpreter or dependencies are outside the contained layout")
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return err
	}
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if key == "storePython" && !info.Mode().IsRegular() || key != "storePython" && !info.IsDir() {
		return fmt.Errorf("hermes runtime member has the wrong kind")
	}
	return nil
}
