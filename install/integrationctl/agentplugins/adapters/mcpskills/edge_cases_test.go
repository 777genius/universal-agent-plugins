package mcpskills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRPCEnvelopeAdversarialResponses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body string
		want       error
	}{
		{"wrong ID", `{"jsonrpc":"2.0","id":2,"result":{}}`, ErrProtocol},
		{"no ID", `{"jsonrpc":"2.0","result":{}}`, ErrProtocol},
		{"wrong version", `{"jsonrpc":"1.0","id":1,"result":{}}`, ErrProtocol},
		{"missing result", `{"jsonrpc":"2.0","id":1}`, ErrProtocol},
		{"null result", `{"jsonrpc":"2.0","id":1,"result":null}`, ErrProtocol},
		{"method in response", `{"jsonrpc":"2.0","id":1,"method":"tools/call","result":{}}`, ErrProtocol},
		{"duplicate JSON key", `{"jsonrpc":"2.0","id":1,"result":{},"result":{}}`, ErrProtocol},
		{"server error", `{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"private credential"}}`, ErrRemote},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeResponse([]byte(tc.body))
			if !errors.Is(err, tc.want) || strings.Contains(err.Error(), "private credential") {
				t.Fatalf("response accepted or error leaked: %v", err)
			}
		})
	}
}

func TestSSERejectsMalformedAndAmbiguousResponses(t *testing.T) {
	t.Parallel()
	const response = `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"}}`
	cases := map[string]string{
		"empty stream": ": keepalive\n\n",
		"wrong final ID": "data: {\"jsonrpc\":\"2.0\",\"id\":8,\"result\":{}}\n\n",
		"unexpected server request": "data: {\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\"}\n\n",
		"duplicate final": "data: " + response + "\n\ndata: " + response + "\n\n",
		"notification after final": "data: " + response + "\n\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\n",
		"duplicate key": "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"id\":1,\"result\":{}}\n\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeEvents([]byte(body)); !errors.Is(err, ErrProtocol) {
				t.Fatalf("accepted malformed SSE: %v", err)
			}
		})
	}
}

func TestMCPHTTPRejectsOversizeAndUnexpectedEncodings(t *testing.T) {
	t.Parallel()
	good := `{"jsonrpc":"2.0","id":1,"result":{}}`
	cases := []struct {
		name, media, encoding, payload string
		length                         int64
	}{
		{"bad media type", "text/html", "", good, -1},
		{"content encoding", "application/json", "gzip", good, -1},
		{"oversize header", "application/json", "", good, maxResponseBytes + 1},
		{"oversize body", "application/json", "", strings.Repeat("x", maxResponseBytes + 1), -1},
		{"non-OK status", "application/json", "", good, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := http.StatusOK
			if tc.name == "non-OK status" {
				status = http.StatusBadRequest
			}
			header := http.Header{}
			header.Set("Content-Type", tc.media)
			if tc.encoding != "" {
				header.Set("Content-Encoding", tc.encoding)
			}
			response := &http.Response{StatusCode: status, ContentLength: tc.length, Header: header,
				Body: io.NopCloser(strings.NewReader(tc.payload))}
			if _, err := decodeHTTPResponse(context.Background(), response); err == nil {
				t.Fatal("accepted unexpected HTTP response")
			}
		})
	}
}

func TestManifestRejectsIntegrityAndPathAmbiguity(t *testing.T) {
	t.Parallel()
	body := []byte(sampleSkill)
	sum := sha256.Sum256(body)
	good := Resource{URI: sampleURI, Digest: "sha256:" + hex.EncodeToString(sum[:]), Size: int64(len(body))}
	cases := []struct {
		name string
		file Resource
	}{
		{"wrong root", Resource{URI: "skill://other/review/SKILL.md", Digest: good.Digest, Size: good.Size}},
		{"oversize", Resource{URI: sampleURI, Digest: good.Digest, Size: maxSkillBytes + 1}},
		{"negative size", Resource{URI: sampleURI, Digest: good.Digest, Size: -1}},
		{"uppercase digest", Resource{URI: sampleURI, Digest: "sha256:" + strings.Repeat("A", 64), Size: good.Size}},
		{"wrong algorithm", Resource{URI: sampleURI, Digest: "md5:" + strings.Repeat("a", 64), Size: good.Size}},
		{"encoded traversal", Resource{URI: "skill://acme/review/%2e%2e/SKILL.md", Digest: good.Digest, Size: good.Size}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateResourceCollection(sampleURI, "skill://acme/review", []Resource{tc.file}); !errors.Is(err, ErrInvalidEntry) {
				t.Fatalf("accepted invalid resource: %v", err)
			}
		})
	}
	skill := Skill{URI: sampleURI, Frontmatter: map[string]any{"name": "review",
		"description": "Review changed files", "metadata": map[string]any{"owner": "changed"}}}
	if err := verifyDocument(context.Background(), skill, good, body); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("accepted incorrect nested frontmatter: %v", err)
	}
	skill.Frontmatter["metadata"] = map[string]any{"owner": "tests"}
	if err := verifyDocument(context.Background(), skill, good, body); err != nil {
		t.Fatalf("rejected matching frontmatter: %v", err)
	}
}

func TestMCPResourceContentsAndMethodsFailClosed(t *testing.T) {
	t.Parallel()
	body := "data"
	invalid := []readContent{{URI: "skill://other/SKILL.md", Text: &body},
		{URI: sampleURI}, {URI: sampleURI, Text: &body, Blob: &body},
		{URI: sampleURI, Blob: ptrValue("not-base64%%%%")}}
	for _, item := range invalid {
		if _, err := readResourceBytes(sampleURI, item); !errors.Is(err, ErrInvalidEntry) {
			t.Fatalf("accepted invalid resource: %+v, %v", item, err)
		}
	}
	client := &Client{endpoint: "https://example.com/mcp", httpClient: http.DefaultClient}
	if _, err := client.rpcCall(context.Background(), "tools/call", map[string]any{"name": "danger"}); !errors.Is(err, ErrProtocol) {
		t.Fatalf("unexpected MCP tool call: %v", err)
	}
}

func ptrValue(value string) *string { return &value }

func TestMissingOrNullResourceSize(t *testing.T) {
	t.Parallel()
	for _, document := range []string{
		`{"uri":"skill://acme/review/SKILL.md","digest":"sha256:` + strings.Repeat("a", 64) + `"}`,
		`{"uri":"skill://acme/review/SKILL.md","digest":"sha256:` + strings.Repeat("a", 64) + `","size":null}`,
	} {
		var resource Resource
		if err := json.Unmarshal([]byte(document), &resource); !errors.Is(err, ErrInvalidEntry) {
			t.Fatalf("accepted resource without a size: %v", err)
		}
	}
}
