// Package mcpskills implements a read-only proof of concept for the MCP Skills
// extension on modern (2026-07-28) MCP servers. It does not install or run Skills.
package mcpskills

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	maxResources  = 512
	maxSkillBytes = 16 << 20
	maxURIBytes   = 4096
)

var ErrInvalidEntry = errors.New("invalid MCP Skill manifest")

// Resource is a manifest's content-bound file reference, not a download URL.
type Resource struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

// A missing size is not the same as an explicitly declared zero-length file.
func (r *Resource) UnmarshalJSON(data []byte) error {
	var wire struct {
		URI    string `json:"uri"`
		Digest string `json:"digest"`
		Size   *int64 `json:"size"`
	}
	if err := json.Unmarshal(data, &wire); err != nil || wire.Size == nil {
		return ErrInvalidEntry
	}
	*r = Resource{URI: wire.URI, Digest: wire.Digest, Size: *wire.Size}
	return nil
}

// Skill identifies one remote Skill by its exact URI *within its MCP server*.
// Resources is either a complete Resource array or the string "dynamic".
type Skill struct {
	URI         string          `json:"uri"`
	Frontmatter map[string]any  `json:"frontmatter"`
	Resources   json.RawMessage `json:"resources"`
}

func (s Skill) name() string {
	name, _ := s.Frontmatter["name"].(string)
	return name
}

func (s Skill) description() string {
	description, _ := s.Frontmatter["description"].(string)
	return description
}

func (s Skill) resources() ([]Resource, bool, error) {
	if string(s.Resources) == `"dynamic"` {
		return nil, true, nil
	}
	if len(s.Resources) == 0 || s.Resources[0] != '[' {
		return nil, false, ErrInvalidEntry
	}
	var resources []Resource
	if err := json.Unmarshal(s.Resources, &resources); err != nil {
		return nil, false, ErrInvalidEntry
	}
	return resources, false, nil
}

func validateURI(value string) error {
	if len(value) == 0 || len(value) > maxURIBytes || !utf8.ValidString(value) ||
		strings.ContainsAny(value, "\\\r\n\t") || strings.TrimSpace(value) != value {
		return ErrInvalidEntry
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.User != nil || parsed.Fragment != "" ||
		parsed.RawQuery != "" || parsed.Opaque != "" || parsed.ForceQuery {
		return ErrInvalidEntry
	}
	// Preserve the URI identity byte-for-byte, and refuse path ambiguity.
	lower := strings.ToLower(parsed.EscapedPath())
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") ||
		strings.Contains(lower, "%00") || strings.Contains(value, "#") || strings.Contains(value, "?") {
		return ErrInvalidEntry
	}
	for _, component := range strings.Split(parsed.Path, "/") {
		if component == "." || component == ".." {
			return ErrInvalidEntry
		}
	}
	if strings.Contains(parsed.Path, "//") {
		return ErrInvalidEntry
	}
	return nil
}

func validateSkill(s Skill) ([]Resource, bool, error) {
	if err := validateURI(s.URI); err != nil || !strings.HasSuffix(s.URI, "/SKILL.md") {
		return nil, false, ErrInvalidEntry
	}
	name := s.name()
	description := s.description()
	if !domain.ValidSkillName(name) || description == "" || utf8.RuneCountInString(description) > 1024 ||
		len(s.Frontmatter) == 0 || len(s.Frontmatter) > 128 {
		return nil, false, ErrInvalidEntry
	}
	root := strings.TrimSuffix(s.URI, "/SKILL.md")
	parsed, _ := url.Parse(root)
	parts := strings.Split(strings.TrimSuffix(parsed.Host+parsed.Path, "/"), "/")
	if len(parts) == 0 || parts[len(parts)-1] != name {
		return nil, false, ErrInvalidEntry
	}
	resources, dynamic, err := s.resources()
	if err != nil || dynamic {
		return resources, dynamic, err
	}
	if len(resources) == 0 || len(resources) > maxResources {
		return nil, false, ErrInvalidEntry
	}
	seen := make(map[string]bool, len(resources))
	var total int64
	found := false
	for _, resource := range resources {
		if err := validateURI(resource.URI); err != nil || !strings.HasPrefix(resource.URI, root+"/") ||
			resource.Size < 0 || resource.Size > maxSkillBytes || !validDigest(resource.Digest) || seen[resource.URI] {
			return nil, false, ErrInvalidEntry
		}
		seen[resource.URI] = true
		if resource.URI == s.URI {
			found = true
		}
		total += resource.Size
		if total > maxSkillBytes {
			return nil, false, ErrInvalidEntry
		}
	}
	if !found {
		return nil, false, ErrInvalidEntry
	}
	return resources, false, nil
}

func validDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, ch := range value[7:] {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f') {
			return false
		}
	}
	return true
}

func verifyDocument(ctx context.Context, s Skill, resource Resource, contents []byte) error {
	if int64(len(contents)) != resource.Size {
		return ErrInvalidEntry
	}
	digest := sha256.Sum256(contents)
	if "sha256:"+hex.EncodeToString(digest[:]) != resource.Digest {
		return ErrInvalidEntry
	}
	fields := make(map[string]struct{}, len(s.Frontmatter))
	for k := range s.Frontmatter {
		fields[k] = struct{}{}
	}
	front, err := conformance.DecodeFrontmatter(ctx, contents, conformance.DefaultLimits(), fields)
	if err != nil || len(front.Keys) != len(s.Frontmatter) {
		return ErrInvalidEntry
	}
	for _, key := range front.Keys {
		if _, ok := s.Frontmatter[key]; !ok {
			return ErrInvalidEntry
		}
	}
	got, err := json.Marshal(front.Values)
	if err != nil {
		return ErrInvalidEntry
	}
	expected, err := json.Marshal(s.Frontmatter)
	if err != nil || string(got) != string(expected) {
		return ErrInvalidEntry
	}
	return nil
}

func readResourceBytes(uri string, content readContent) ([]byte, error) {
	if content.URI != uri || (content.Text == nil) == (content.Blob == nil) {
		return nil, ErrInvalidEntry
	}
	if content.Text != nil {
		return []byte(*content.Text), nil
	}
	data, err := base64.StdEncoding.Strict().DecodeString(*content.Blob)
	if err != nil {
		return nil, ErrInvalidEntry
	}
	return data, nil
}

// Summary is safe to display: it never includes Skill instructions or scripts.
type Summary struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Resources   int    `json:"resource_count"`
	Integrity   string `json:"integrity"`
	Origin      string `json:"server"`
	Digest      string `json:"skill_digest,omitempty"`
}

func summarize(s Skill, origin, integrity string) (Summary, error) {
	resources, dynamic, err := validateSkill(s)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{URI: s.URI, Name: s.name(), Description: s.description(), Resources: len(resources), Integrity: integrity, Origin: origin}
	if dynamic {
		summary.Integrity = "unverifiable_dynamic"
	}
	if !dynamic {
		for _, r := range resources {
			if r.URI == s.URI {
				summary.Digest = r.Digest
				break
			}
		}
	}
	return summary, nil
}

func unavailable(kind string) error { return fmt.Errorf("%s: %w", kind, ErrInvalidEntry) }
