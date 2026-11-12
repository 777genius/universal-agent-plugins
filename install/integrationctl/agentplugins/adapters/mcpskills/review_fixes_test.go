package mcpskills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLargeFrontmatterIntegerMustMatchExactly(t *testing.T) {
	t.Parallel()
	const actualRevision = "9007199254740992"
	const claimedRevision = "9007199254740993"
	body := []byte("---\nname: review\ndescription: Review changed files\nrevision: " + actualRevision + "\n---\n# Review\n")
	digest := sha256.Sum256(body)
	resource := Resource{URI: sampleURI, Digest: "sha256:" + hex.EncodeToString(digest[:]), Size: int64(len(body))}
	for _, tc := range []struct {
		name, revision string
		wantValid      bool
	}{
		{"exact integer", actualRevision, true},
		{"rounded float collision", claimedRevision, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded := fmt.Sprintf(`{"uri":%q,"frontmatter":{"name":"review","description":"Review changed files","revision":%s},"resources":[]}`, sampleURI, tc.revision)
			var skill Skill
			if err := json.Unmarshal([]byte(encoded), &skill); err != nil {
				t.Fatal(err)
			}
			if _, ok := skill.Frontmatter["revision"].(json.Number); !ok {
				t.Fatalf("JSON integer lost its exact representation: %T", skill.Frontmatter["revision"])
			}
			err := verifyDocument(context.Background(), skill, resource, body)
			if (err == nil) != tc.wantValid {
				t.Fatalf("frontmatter validation = %v; want valid=%v", err, tc.wantValid)
			}
		})
	}
}

func TestStreamingSSEReturnsBeforeEOF(t *testing.T) {
	t.Parallel()
	reader, writer := io.Pipe()
	defer func() {
		_ = reader.Close()
		_ = writer.Close()
	}()
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       reader,
	}
	resultCh := make(chan error, 1)
	go func() {
		body, err := decodeHTTPResponse(context.Background(), response)
		if err == nil && !strings.Contains(string(body), `"resultType":"complete"`) {
			err = errors.New("incorrect final SSE result")
		}
		resultCh <- err
	}()
	go func() {
		_, _ = io.WriteString(writer, "data: "+`{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"}}`+"\n\n")
		// Deliberately leave the pipe open: the final reply must not need EOF.
	}()
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SSE client waited for EOF after receiving the final response")
	}
}

func TestDNSFallbackNeverRedialsHostOrPrivateIP(t *testing.T) {
	t.Parallel()
	ips := []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("1.1.1.1")}}
	var calls []string
	var peer net.Conn
	connection, err := dialPublicAddresses(context.Background(), "443", ips,
		func(_ context.Context, network, address string) (net.Conn, error) {
			if network != "tcp" {
				t.Fatalf("unexpected network: %s", network)
			}
			calls = append(calls, address)
			if address == "8.8.8.8:443" {
				return nil, errors.New("unreachable first IP")
			}
			client, server := net.Pipe()
			peer = server
			return client, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	_ = peer.Close()
	if got := strings.Join(calls, ","); got != "8.8.8.8:443,1.1.1.1:443" {
		t.Fatalf("unexpected dial order or hostname re-resolution: %q", got)
	}
	private := append(ips, net.IPAddr{IP: net.ParseIP("127.0.0.1")})
	called := false
	if _, err := dialPublicAddresses(context.Background(), "443", private,
		func(context.Context, string, string) (net.Conn, error) {
			called = true
			return nil, errors.New("must not dial")
		}); err == nil || called {
		t.Fatalf("mixed public/private DNS response was dialed: %v", err)
	}
}

func TestDNSFallbackStopsOnContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	_, err := dialPublicAddresses(ctx, "443",
		[]net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("1.1.1.1")}},
		func(context.Context, string, string) (net.Conn, error) {
			calls++
			cancel()
			return nil, errors.New("unreachable")
		})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("fallback exceeded canceled budget: calls=%d err=%v", calls, err)
	}
}
