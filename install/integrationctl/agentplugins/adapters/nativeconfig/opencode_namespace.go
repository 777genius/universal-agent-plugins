package nativeconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/tailscale/hujson"
)

var ErrOpenCodeV2Namespace = errors.New("OpenCode mcp.servers format is not yet supported by namespace preflight")

// OpenCodeNamespaceConflict is a conservative server-name collision: the
// servers could expose tools with the same callable ID. Tool catalogs are not
// queried, so this does not claim that a particular tool has collided.
type OpenCodeNamespaceConflict struct {
	Proposed string
	Existing string
	Reason   string
}

func (e *OpenCodeNamespaceConflict) Error() string {
	return fmt.Sprintf("OpenCode MCP server %q may share callable tool IDs with %q (%s); rename one server and retry", e.Proposed, e.Existing, e.Reason)
}

// OpenCode uses JavaScript's /[^a-zA-Z0-9_-]/g, which replaces each UTF-16
// code unit independently (including both halves of a surrogate pair).
func openCodeToolNamePart(name string) string {
	var out strings.Builder
	for _, unit := range utf16.Encode([]rune(name)) {
		if unit >= 'a' && unit <= 'z' || unit >= 'A' && unit <= 'Z' || unit >= '0' && unit <= '9' || unit == '_' || unit == '-' {
			out.WriteByte(byte(unit))
		} else {
			out.WriteByte('_')
		}
	}
	return out.String()
}

func openCodeNamesMayCollide(a, b string) bool {
	return openCodeCollisionReason(a, b) != ""
}

func openCodeCollisionReason(a, b string) string {
	a, b = openCodeToolNamePart(a), openCodeToolNamePart(b)
	if a == b {
		return "server names normalize identically"
	}
	if strings.HasPrefix(a, b+"_") || strings.HasPrefix(b, a+"_") {
		return "one normalized server name contains the other followed by _"
	}
	return ""
}

// checkOpenCodeNamespace is the one policy used for both read-only preflight
// and the final in-memory document under the cooperating writer lock.
func checkOpenCodeNamespace(active []string, proposed []string) error {
	sort.Strings(active)
	sort.Strings(proposed)
	for i, name := range proposed {
		for _, other := range active {
			if reason := openCodeCollisionReason(name, other); reason != "" {
				return &OpenCodeNamespaceConflict{Proposed: name, Existing: other, Reason: reason}
			}
		}
		for _, other := range proposed[i+1:] {
			if reason := openCodeCollisionReason(name, other); reason != "" {
				return &OpenCodeNamespaceConflict{Proposed: name, Existing: other, Reason: reason}
			}
		}
	}
	return nil
}

func openCodeActiveMCPNames(entries *hujson.Object) ([]string, error) {
	return openCodeActiveMCPNamesForCodec(entries, CodecOpenCode)
}

func openCodeActiveMCPNamesForCodec(entries *hujson.Object, codec Codec) ([]string, error) {
	if entries == nil {
		return nil, nil
	}
	names := make([]string, 0, len(entries.Members))
	for i := range entries.Members {
		member := &entries.Members[i]
		name := member.Name.Value.(hujson.Literal).String()
		body, err := entryCanonical(member)
		if err != nil {
			return nil, fmt.Errorf("%w: OpenCode MCP server %q: %w", ErrMalformed, name, err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
			return nil, fmt.Errorf("%w: OpenCode MCP server %q must be an object", ErrMalformed, name)
		}
		if codec == CodecOpenCode && name == "servers" && openCodeNestedServers(fields) {
			return nil, ErrOpenCodeV2Namespace
		}
		activityKey := "enabled"
		if codec == CodecOpenCodeV2 {
			activityKey = "disabled"
		}
		if activity, ok := fields[activityKey]; ok {
			var value bool
			if err := json.Unmarshal(activity, &value); err != nil || string(activity) == "null" {
				return nil, fmt.Errorf("%w: OpenCode MCP server %q has invalid %s value", ErrMalformed, name, activityKey)
			}
			if codec == CodecOpenCode && !value || codec == CodecOpenCodeV2 && value {
				continue
			}
		}
		names = append(names, name)
	}
	return names, nil
}

func openCodeNestedServers(fields map[string]json.RawMessage) bool {
	for _, key := range []string{"type", "enabled"} {
		if value, ok := fields[key]; ok && len(value) > 0 && value[0] != '{' && value[0] != '[' {
			return false // A flat server literally named "servers".
		}
	}
	return true
}

// CheckOpenCodeNamespace observes only the selected user-level config. It is
// process-inert and never modifies the config or starts OpenCode/MCP servers.
// previous names are omitted only when the same operation removes them.
func (kernel Kernel) CheckOpenCodeNamespace(paths Paths, proposed, previous []string) error {
	return kernel.CheckOpenCodeNamespaceForCodec(paths, CodecOpenCode, proposed, previous)
}

// CheckOpenCodeNamespaceForCodec is the closed dialect-aware preflight for a
// future prepared host. The legacy helper stays V1. This is a conservative name
// collision policy, not proof of a running host's tool catalog or mixed roots.
// proposed contains only names that the prepared operation will leave active.
func (kernel Kernel) CheckOpenCodeNamespaceForCodec(paths Paths, codec Codec, proposed, previous []string) error {
	if codec != CodecOpenCode && codec != CodecOpenCodeV2 {
		return fmt.Errorf("unsupported OpenCode namespace codec %q", codec)
	}
	if err := kernel.RequireFileIO(); err != nil {
		return err
	}
	if err := validateExactPath(paths.JSON, "JSON native config path"); err != nil {
		return err
	}
	if paths.JSONC != "" {
		if err := validateExactPath(paths.JSONC, "JSONC native config path"); err != nil {
			return err
		}
	}
	if filepath.Clean(paths.JSON) == filepath.Clean(paths.JSONC) {
		return fmt.Errorf("OpenCode JSON and JSONC config paths must differ")
	}
	file, err := kernel.resolve(paths)
	if err != nil {
		return err
	}
	var entries *hujson.Object
	if file.exists {
		doc, err := parseDocument(file.body, file.jsonc)
		if err != nil {
			return err
		}
		if codec == CodecOpenCodeV2 {
			if err := requireOpenCodeV2Root(doc); err != nil {
				return err
			}
		}
		entries, err = codecCollection(doc, codec, false)
		if err != nil {
			return err
		}
	}
	active, err := openCodeActiveMCPNamesForCodec(entries, codec)
	if err != nil {
		return err
	}
	removed := make(map[string]bool, len(previous))
	for _, name := range previous {
		removed[name] = true
	}
	remaining := active[:0]
	for _, name := range active {
		if !removed[name] {
			remaining = append(remaining, name)
		}
	}
	return checkOpenCodeNamespace(remaining, append([]string(nil), proposed...))
}
