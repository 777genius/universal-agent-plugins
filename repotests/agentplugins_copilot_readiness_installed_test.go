//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
)

func copilotReadinessDigest(body []byte) string {
	h := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(h[:])
}
func (f *copilotProjectAttempt) installedReadiness() error {
	root, data := f.binding.TargetLocator, f.data
	entry := map[string]any{
		"type": "stdio", "tools": []any{"*"},
		"command": filepath.Join(root, "io.github.777genius.agentplugins", "managed-stdio-v1", "agentplugins"),
		"args":    []any{"--internal-stdio-v1", root, data, root, "plugin", "--", "./bin/probe", "argument with spaces", root + "/argument", "${UNKNOWN}"},
		"env":     map[string]any{"PLUGIN_ROOT": root, "PLUGIN_DATA": data, "UAP_TEST_ROOT": f.base.Root, "UAP_TEST_EVENTS": f.base.Events, "UAP_TEST_NONCE": f.base.Nonce, "UAP_TEST_REVISION": "A-one", "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": root + "/${UNKNOWN}"},
	}
	observed, err := projectNativeRuntimeJSON(f.files["mcp.json.installed"])
	if err != nil || !reflect.DeepEqual(observed, map[string]any{"mcpServers": map[string]any{"uapcopilota": entry}}) {
		return fmt.Errorf("Copilot readiness independently authored installed MCP entry")
	}
	skill := []byte("---\nname: uapcopilota\ndescription: Disposable Copilot project proof\n---\nUAP_COPILOT_A_ONE_" + f.base.Nonce + "\n")
	if !bytes.Equal(skill, f.files["uapcopilota-SKILL.md.installed"]) {
		return fmt.Errorf("Copilot readiness independently authored installed skill body")
	}
	entryBody, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	entryBody = append([]byte("agentplugins-native-mcp-v1\x00copilot-mcpServers\x00uapcopilota\x00"), entryBody...)
	// Independent one-file non-executable native skill-directory framing.
	framed := append([]byte{'f', 0}, []byte("SKILL.md\x00-\x00")...)
	framed = append(framed, skill...)
	framed = append(framed, 0)
	want := map[string][2]string{
		"copilot_project_mcp_server":      {filepath.Join(f.base.Project, ".github", "mcp.json"), copilotReadinessDigest(entryBody)},
		"copilot_project_skill_directory": {filepath.Join(f.base.Project, ".github", "skills", "uapcopilota"), copilotReadinessDigest(framed)},
	}
	seen := map[string]bool{}
	for _, object := range f.binding.NativeObjects {
		if expected, has := want[object.Kind]; has {
			if seen[object.Kind] || object.ObjectID != object.Kind+":uapcopilota" || object.Path != expected[0] || object.ManagedDigest != expected[1] || object.LogicalName != "uapcopilota" || object.ProtectionClass != "managed" {
				return fmt.Errorf("Copilot readiness native ownership digest/authority")
			}
			seen[object.Kind] = true
		}
	}
	if len(seen) != len(want) {
		return fmt.Errorf("Copilot readiness missing native ownership")
	}
	return nil
}
