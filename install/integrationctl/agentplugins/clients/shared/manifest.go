package shared

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ManifestOption selects how the envelope metadata is projected.
type ManifestOption func(*manifestOptions)

type manifestOptions struct {
	author authorProjection
}

type authorProjection uint8

const (
	authorOmitted authorProjection = iota
	authorObject
	authorNameEmail
)

// WithAuthorObject copies the authored author value through unchanged.
func WithAuthorObject() ManifestOption {
	return func(options *manifestOptions) { options.author = authorObject }
}

// WithAuthorNameEmail projects the author as a flat {name, email} object,
// omitting an empty email and the author entirely when there is no name. It is
// for clients whose manifest schema does not accept the authored shape.
func WithAuthorNameEmail() ManifestOption {
	return func(options *manifestOptions) { options.author = authorNameEmail }
}

// ManifestFromEnvelope builds the plugin manifest skeleton every projecting
// client starts from: the package name plus the optional metadata members that
// are actually set.
func ManifestFromEnvelope(envelope domain.PackageEnvelope, options ...ManifestOption) map[string]any {
	manifest := map[string]any{"name": envelope.Manifest.Name}
	ApplyManifestMetadata(manifest, envelope, options...)
	return manifest
}

// ApplyManifestMetadata overlays the envelope metadata onto an existing
// manifest. The name is left alone: a preserved upstream manifest keeps the
// identity it was published with.
func ApplyManifestMetadata(manifest map[string]any, envelope domain.PackageEnvelope, options ...ManifestOption) {
	settings := manifestOptions{}
	for _, option := range options {
		option(&settings)
	}
	for _, member := range []struct{ key, value string }{
		{"version", envelope.Manifest.Version},
		{"description", envelope.Manifest.Description},
		{"homepage", envelope.Manifest.Homepage},
		{"repository", envelope.Manifest.Repository},
		{"license", envelope.Manifest.License},
	} {
		if strings.TrimSpace(member.value) != "" {
			manifest[member.key] = member.value
		}
	}
	applyManifestAuthor(manifest, envelope, settings.author)
	if len(envelope.Manifest.Keywords) > 0 {
		manifest["keywords"] = envelope.Manifest.Keywords
	}
}

func applyManifestAuthor(manifest map[string]any, envelope domain.PackageEnvelope, projection authorProjection) {
	author := envelope.Manifest.Author
	if author == nil {
		return
	}
	switch projection {
	case authorObject:
		manifest["author"] = author
	case authorNameEmail:
		if strings.TrimSpace(author.Name) == "" {
			return
		}
		flat := map[string]string{"name": author.Name}
		if strings.TrimSpace(author.Email) != "" {
			flat["email"] = author.Email
		}
		manifest["author"] = flat
	case authorOmitted:
	}
}

// PreservedOpenAIManifest returns the upstream OpenAI plugin manifest when the
// package was authored in that format, so a publisher's own document survives
// the projection instead of being rebuilt from the normalized envelope.
func PreservedOpenAIManifest(envelope domain.PackageEnvelope) (map[string]any, bool, error) {
	if envelope.FormatID != domain.FormatIDOpenAIPlugin || len(envelope.Manifest.Raw) == 0 {
		return nil, false, nil
	}
	var manifest map[string]any
	if err := json.Unmarshal(envelope.Manifest.Raw, &manifest); err != nil || manifest == nil {
		return nil, false, fmt.Errorf("decode preserved OpenAI plugin manifest: %w", err)
	}
	return manifest, true, nil
}
