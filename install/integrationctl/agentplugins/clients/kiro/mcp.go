package kiro

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	SkillObjectKind     = "kiro_global_skill_directory"
	MCPObjectKind       = "kiro_global_mcp_server"
	kiroSkillObjectKind = SkillObjectKind
	kiroMCPObjectKind   = MCPObjectKind
)

func ReadMCPConfig(path string) (servers map[string]any, original []byte, mode os.FileMode, exists bool, err error) {
	snapshot, err := nativeconfig.New().ReadExactFile(path)
	if err != nil {
		return nil, nil, 0600, false, fmt.Errorf("read Kiro MCP configuration: %w", err)
	}
	if !snapshot.Exists {
		return map[string]any{}, nil, 0600, false, nil
	}
	servers, err = decodeKiroMCPConfig(snapshot.Body)
	return servers, snapshot.Body, snapshot.Mode, true, err
}

func decodeKiroMCPConfig(body []byte) (map[string]any, error) {
	standard, err := hujson.Standardize(bytes.Clone(body))
	if err != nil {
		return nil, fmt.Errorf("decode Kiro MCP configuration: %w", err)
	}
	document, err := shared.DecodeStrictJSONObject(standard)
	if err != nil {
		return nil, fmt.Errorf("decode Kiro MCP configuration: %w", err)
	}
	for key := range document {
		if strings.EqualFold(key, "mcpServers") && key != "mcpServers" {
			return nil, kiroErrorf("Kiro mcpServers key has incorrect case")
		}
	}
	if raw, present := document["mcpServers"]; present {
		servers, ok := raw.(map[string]any)
		if !ok {
			return nil, kiroErrorf("Kiro mcpServers must be an object")
		}
		return servers, nil
	}
	return map[string]any{}, nil
}

// Keep the HuJSON tree for rendering and the existing strict decoder for
// ownership: duplicate/case collisions and out-of-range numbers still fail.
func encodeKiroMCPConfig(original []byte, servers map[string]any) ([]byte, error) {
	if len(original) == 0 {
		original = []byte("{}\n")
	}
	before, err := decodeKiroMCPConfig(original)
	if err != nil {
		return nil, err
	}
	ast, err := hujson.Parse(original)
	if err != nil {
		return nil, err
	}
	entries := kiroMCPEntries(ast.Value.(*hujson.Object))
	if err := patchKiroMCPEntries(entries, before, servers); err != nil {
		return nil, err
	}
	body := ast.Pack()
	if _, err := decodeKiroMCPConfig(body); err != nil {
		return nil, err
	}
	return body, nil
}

func kiroMCPEntries(root *hujson.Object) *hujson.Object {
	var entries *hujson.Object
	for i := range root.Members {
		if root.Members[i].Name.Value.(hujson.Literal).String() == "mcpServers" {
			entries = root.Members[i].Value.Value.(*hujson.Object)
		}
	}
	if entries == nil {
		entries = &hujson.Object{}
		root.Members = append(root.Members, hujson.ObjectMember{Name: hujson.Value{Value: hujson.String("mcpServers")}, Value: hujson.Value{Value: entries}})
	}
	return entries
}

func patchKiroMCPEntries(entries *hujson.Object, before, servers map[string]any) error {
	remaining := make(map[string]any, len(servers))
	for name, server := range servers {
		remaining[name] = server
	}
	for i := 0; i < len(entries.Members); {
		member := &entries.Members[i]
		name := member.Name.Value.(hujson.Literal).String()
		server, keep := remaining[name]
		if !keep {
			// Preserve comments adjacent to the removed member as collection comments.
			extra := append(bytes.Clone(member.Name.BeforeExtra), member.Name.AfterExtra...)
			extra = append(extra, member.Value.BeforeExtra...)
			extra = append(extra, member.Value.AfterExtra...)
			entries.Members = append(entries.Members[:i], entries.Members[i+1:]...)
			if i < len(entries.Members) {
				entries.Members[i].Name.BeforeExtra = append(extra, entries.Members[i].Name.BeforeExtra...)
			} else {
				entries.AfterExtra = append(extra, entries.AfterExtra...)
			}
			continue
		}
		oldBody, _ := json.Marshal(before[name])
		nextBody, err := json.Marshal(server)
		if err != nil {
			return err
		}
		if !bytes.Equal(oldBody, nextBody) {
			next, err := hujson.Parse(nextBody)
			if err != nil {
				return err
			}
			next.BeforeExtra, next.AfterExtra = member.Value.BeforeExtra, member.Value.AfterExtra
			member.Value = next
		}
		delete(remaining, name)
		i++
	}
	return appendKiroMCPEntries(entries, remaining)
}

func appendKiroMCPEntries(entries *hujson.Object, remaining map[string]any) error {
	names := make([]string, 0, len(remaining))
	for name := range remaining {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		body, err := json.Marshal(remaining[name])
		if err != nil {
			return err
		}
		value, err := hujson.Parse(body)
		if err != nil {
			return err
		}
		entries.Members = append(entries.Members, hujson.ObjectMember{Name: hujson.Value{Value: hujson.String(name)}, Value: value})
	}
	return nil
}

func NativeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == kiroSkillObjectKind || object.Kind == kiroMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func previousMCPObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind == kiroMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func hasKiroSkillObjects(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == kiroSkillObjectKind {
			return true
		}
	}
	return false
}
