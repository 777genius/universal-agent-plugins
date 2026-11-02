//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

// Projection JSON uses the existing fixed WriteJSON encoding. All unchanged
// payload bytes and modes come from the immutable authored recipe.
func (p *kimiProjectProof) installedFiles(a kimiProjectProofAuthority) map[string]kimiProjectProofFile {
	files := make(map[string]kimiProjectProofFile, len(p.authored)+3)
	for path, file := range p.authored {
		files[path] = file
	}
	if p.scope == "user" {
		files[".kimi-plugin/plugin.json"] = kimiProjectProofFile{Body: kimiProjectProofJSON(map[string]any{"name": p.name, "version": p.version, "skills": "./skills/"})}
		return files
	}
	files["mcp.json"] = kimiProjectProofFile{Body: kimiProjectProofJSON(map[string]any{"mcpServers": map[string]any{p.name: p.expectedServer(a)}})}
	directory := managedstdio.RelativeDirectory
	files[directory+"/"+managedstdio.ExecutableName] = kimiProjectProofFile{Executable: true}
	metadata, _ := json.Marshal(struct {
		Protocol int    `json:"protocol"`
		SHA256   string `json:"sha256"`
		Version  string `json:"cliVersion"`
	}{1, p.installerSHA, "0.1.0-development"})
	files[directory+"/metadata.json"] = kimiProjectProofFile{Body: metadata}
	return files
}

func kimiProjectProofJSON(value any) []byte {
	body, _ := json.MarshalIndent(value, "", "  ")
	return append(body, '\n')
}

func (p *kimiProjectProof) validateInstalled(actual []packagedigest.CapturedEntry, a kimiProjectProofAuthority) error {
	expected := kimiProjectProofEntries(p.installedFiles(a))
	if len(actual) != len(expected) {
		return fmt.Errorf("Kimi installed closure has missing or extra members")
	}
	launcher := filepath.ToSlash(filepath.Join(managedstdio.RelativeDirectory, managedstdio.ExecutableName))
	for index, entry := range actual {
		want := expected[index]
		if entry.Path != want.Path || entry.Kind != want.Kind || entry.Executable != want.Executable || entry.Target != want.Target {
			return fmt.Errorf("Kimi installed path/type/mode closure differs")
		}
		if p.scope != "user" && entry.Path == launcher {
			if kimiProjectProofSHA(entry.Content) != p.installerSHA {
				return fmt.Errorf("Kimi installed launcher differs from frozen public installer")
			}
		} else if !bytes.Equal(entry.Content, want.Content) {
			return fmt.Errorf("Kimi installed source/probe/projection/metadata differs: %s", entry.Path)
		}
	}
	return nil
}
