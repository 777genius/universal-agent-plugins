package mcpskills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
)

const (
	maxCatalogPages   = 32
	maxCatalogEntries = 2048
)

// Client is one read-only MCP endpoint. It never stores credentials or state.
type Client struct {
	endpoint   string
	origin     string
	httpClient *http.Client
}

func New(endpoint string) (*Client, error) {
	return NewWithHTTPClient(endpoint, publicHTTPClient())
}

// NewWithHTTPClient permits a caller-owned authenticated/test transport only
// through explicit dependency injection. Production CLI always uses New.
func NewWithHTTPClient(endpoint string, client *http.Client) (*Client, error) {
	origin, err := normalizeEndpoint(endpoint)
	if err != nil || client == nil {
		return nil, ErrProtocol
	}
	clone := *client
	// Even an injected caller must not be redirected to another endpoint.
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	sum := sha256.Sum256([]byte(endpoint))
	return &Client{endpoint: endpoint, origin: origin + "#" + hex.EncodeToString(sum[:8]), httpClient: &clone}, nil
}

func (c *Client) Origin() string { return c.origin }

func (c *Client) discover(ctx context.Context) error {
	body, err := c.rpcCall(ctx, "server/discover", nil)
	if err != nil {
		return err
	}
	var result struct {
		cacheFields
		SupportedVersions []string `json:"supportedVersions"`
		Capabilities      struct {
			Resources  json.RawMessage            `json:"resources"`
			Extensions map[string]json.RawMessage `json:"extensions"`
		} `json:"capabilities"`
	}
	if json.Unmarshal(body, &result) != nil || !result.cacheFields.valid() ||
		len(result.Capabilities.Resources) < 2 || result.Capabilities.Resources[0] != '{' {
		return ErrProtocol
	}
	ext, present := result.Capabilities.Extensions[extensionID]
	if !present || len(ext) < 2 || ext[0] != '{' {
		return ErrProtocol
	}
	for _, ver := range result.SupportedVersions {
		if ver == protocolVersion {
			return nil
		}
	}
	return ErrProtocol
}

// List retrieves metadata only. A partial/empty listing does not imply the
// server has no other Skills (clients may query exact URIs via Inspect).
func (c *Client) List(ctx context.Context) ([]Summary, error) {
	if err := c.discover(ctx); err != nil {
		return nil, err
	}
	var summaries []Summary
	seenURIs := make(map[string]bool)
	seenCursors := make(map[string]bool)
	var cursor, cacheScope string
	for page := 0; page < maxCatalogPages; page++ {
		args := map[string]any{}
		if cursor != "" {
			args["cursor"] = cursor
		}
		payload, err := c.rpcCall(ctx, "skills/list", args)
		if err != nil {
			return nil, err
		}
		var result struct {
			cacheFields
			Skills     []Skill `json:"skills"`
			NextCursor string  `json:"nextCursor"`
		}
		if json.Unmarshal(payload, &result) != nil || !result.cacheFields.valid() || result.Skills == nil {
			return nil, ErrProtocol
		}
		if cacheScope != "" && cacheScope != result.CacheScope {
			return nil, ErrProtocol
		}
		cacheScope = result.CacheScope
		if len(summaries)+len(result.Skills) > maxCatalogEntries {
			return nil, ErrProtocol
		}
		for _, skill := range result.Skills {
			if seenURIs[skill.URI] {
				return nil, ErrProtocol
			}
			seenURIs[skill.URI] = true
			summary, err := summarize(skill, c.origin, "manifest_only")
			if err != nil {
				return nil, err
			}
			summaries = append(summaries, summary)
		}
		if result.NextCursor == "" {
			return summaries, nil
		}
		if seenCursors[result.NextCursor] {
			return nil, ErrProtocol
		}
		seenCursors[result.NextCursor] = true
		cursor = result.NextCursor
	}
	return nil, errors.Join(ErrProtocol, errors.New("MCP Skills page limit exceeded"))
}

// Inspect obtains one exact URI and verifies only its SKILL.md. Dynamic entries
// remain visible but cannot be content-bound. No Skill content is returned.
func (c *Client) Inspect(ctx context.Context, uri string) (Summary, error) {
	if validateURI(uri) != nil {
		return Summary{}, ErrInvalidEntry
	}
	if err := c.discover(ctx); err != nil {
		return Summary{}, err
	}
	payload, err := c.rpcCall(ctx, "skills/get", map[string]any{"uri": uri})
	if err != nil {
		return Summary{}, err
	}
	var result struct {
		cacheFields
		Skill Skill `json:"skill"`
	}
	if json.Unmarshal(payload, &result) != nil || !result.cacheFields.valid() || result.Skill.URI != uri {
		return Summary{}, ErrProtocol
	}
	summary, err := summarize(result.Skill, c.origin, "verified")
	if err != nil || summary.Integrity == "unverifiable_dynamic" {
		return summary, err
	}
	resources, _, err := result.Skill.resources()
	if err != nil {
		return Summary{}, err
	}
	var document Resource
	for _, resource := range resources {
		if resource.URI == uri {
			document = resource
			break
		}
	}
	readPayload, err := c.rpcCall(ctx, "resources/read", map[string]any{"uri": uri})
	if err != nil {
		return Summary{}, err
	}
	var read struct {
		cacheFields
		Contents []readContent `json:"contents"`
	}
	if json.Unmarshal(readPayload, &read) != nil || !read.cacheFields.valid() || len(read.Contents) != 1 {
		return Summary{}, ErrProtocol
	}
	content, err := readResourceBytes(uri, read.Contents[0])
	if err != nil {
		return Summary{}, err
	}
	if err := verifyDocument(ctx, result.Skill, document, content); err != nil {
		return Summary{}, err
	}
	return summary, nil
}

type readContent struct {
	URI  string  `json:"uri"`
	Text *string `json:"text"`
	Blob *string `json:"blob"`
}
