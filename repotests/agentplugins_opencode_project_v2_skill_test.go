package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// os/exec writes stderr from its copy goroutine. Limit bytes while the
// native child is alive, and read the buffer only after Wait has joined it.
type openCodeV2BoundedStderr struct {
	data      bytes.Buffer
	truncated bool
}

func (w *openCodeV2BoundedStderr) Write(body []byte) (int, error) {
	incoming := len(body)
	remaining := max(0, openCodeProjectLimit-w.data.Len())
	if remaining < len(body) {
		w.truncated = true
		body = body[:remaining]
	}
	if _, err := w.data.Write(body); err != nil {
		return 0, err
	}
	return incoming, nil
}

func (w *openCodeV2BoundedStderr) Len() int      { return w.data.Len() }
func (w *openCodeV2BoundedStderr) Bytes() []byte { return w.data.Bytes() }
func (w *openCodeV2BoundedStderr) String() string { return w.data.String() }

// V2 removed "debug skill". Its official /api/skill endpoint exposes the
// actual process-local skill registry. Never replace this proof with a file
// inventory or a synthetic successful native response.
func (f *openCodeProjectFixture) skillsV2(t *testing.T, label, project string, required ...string) map[string]openCodeProjectSkill {
	t.Helper()
	commandLabel := label + "-skill"
	if f.table[commandLabel] != "native" || f.used[commandLabel] != "" || len(f.used) >= 60 {
		t.Fatal("V2 skill API observation not admitted by finite native command table")
	}
	f.used[commandLabel] = "native"
	before := f.recorder.counts()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(f.ctx, 16*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, f.client, "serve", "--hostname", "127.0.0.1", "--port", strconv.Itoa(port))
	command.WaitDelay = time.Second
	command.Dir = project
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	password := hex.EncodeToString(key)
	// V2 eagerly POSTs to configured MCP servers while the native skill API
	// starts. Preserve the real layered project/user configuration; the
	// loopback recorder verifies every attempted route, but a 503 response
	// cannot qualify MCP initialize or tool execution.
	command.Env = append(f.env(), "OPENCODE_PASSWORD="+password)
	command.Stdout = io.Discard
	var stderr openCodeV2BoundedStderr
	command.Stderr = &stderr
	started := time.Now()
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()

	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport, Timeout: 1300 * time.Millisecond}
	requestURL := fmt.Sprintf("http://127.0.0.1:%d/api/skill?location%%5Bdirectory%%5D=%s", port, url.QueryEscape(project))
	var body []byte
	var lastError error
	for ctx.Err() == nil {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		request.SetBasicAuth("opencode", password)
		response, err := httpClient.Do(request)
		if err == nil {
			body, err = io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
			if closeErr := response.Body.Close(); closeErr != nil {
				err = closeErr
			}
			if err == nil && response.StatusCode == http.StatusOK && len(body) <= 1<<20 {
				// V2 registers two built-in skills before project/user
				// resources finish loading. A nonempty registry therefore
				// proves nothing: require every source-bound expected skill
				// before observing the native project result.
				registered, observed := openCodeProjectV2Skills(body, project)
				ready := observed == nil && len(required) > 0
				for _, name := range required {
					if registered[name].Name == "" {
						ready = false
					}
				}
				if ready {
					lastError = nil
					break
				}
				lastError = fmt.Errorf("V2 source-bound project skills not yet registered or malformed: %v", observed)
			} else {
				lastError = fmt.Errorf("native V2 skill route not ready (status %d)", response.StatusCode)
			}
		} else {
			lastError = err
		}
		select {
		case <-ctx.Done():
		case <-time.After(110 * time.Millisecond):
		}
	}
	// This proof uses a one-time Basic Auth secret and must not silently
	// pass if the public skill endpoint is exposed without authentication.
	if lastError == nil {
		unauthenticated, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		response, err := httpClient.Do(unauthenticated)
		if err != nil {
			t.Fatal("native V2 unauthenticated skill negative control failed", err)
		}
		_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("native V2 skill API did not reject missing Basic Auth (status %d)", response.StatusCode)
		}
	}
	// Wait for the child's stderr copier before reading or hashing any
	// captured bytes, even on a successful HTTP API observation.
	_ = command.Process.Kill()
	_ = command.Wait()
	if lastError != nil || len(body) == 0 || len(body) > 1<<20 || stderr.truncated {
		t.Fatalf("V2 native skill HTTP observation failed: %v (stderr overflow=%t); bounded startup stderr: %q", lastError, stderr.truncated, strings.TrimSpace(stderr.String()))
	}
	f.keep(t, commandLabel+".stdout.log", body)
	if len(stderr.Bytes()) > openCodeProjectLimit {
		t.Fatal("native V2 server wrote unbounded stderr")
	}
	f.keep(t, commandLabel+".stderr.log", stderr.Bytes())
	digest := func(b []byte) string {
		sum := sha256.Sum256(b)
		return hex.EncodeToString(sum[:])
	}
	f.records = append(f.records, map[string]any{
		"class": "native", "label": commandLabel,
		"argv": []string{filepath.Base(f.client), "serve", "--hostname", "127.0.0.1", "--port", "<bound-port>"},
		"cwd": project, "exit": 0, "elapsed_ms": time.Since(started).Milliseconds(),
		"stdout_file": commandLabel+".stdout.log", "stdout_bytes": len(body),
		"stdout_sha256": digest(body), "stderr_file": commandLabel+".stderr.log",
		"stderr_bytes": stderr.Len(), "stderr_sha256": digest(stderr.Bytes()),
		"recorder_before": before, "recorder_after": f.recorder.counts(),
		"native_observation": "official_V2_api_skill_not_file_inventory",
	})
	after := f.recorder.counts()
	if after[0] < before[0] || after[1] < before[1] || after[0] > 128 || after[1] > 128 || f.recorder.unexpected.Load() != 0 {
		t.Fatal("native V2 startup accessed an unregistered MCP endpoint or exceeded its bound")
	}
	parsed, err := openCodeProjectV2Skills(body, project)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

// The collector must not bypass Write through an embedded ReaderFrom or
// retain unbounded child output before the post-Wait evidence checks.
func TestOpenCodeV2BoundedStderrNeverStoresBeyondLimit(t *testing.T) {
	var output openCodeV2BoundedStderr
	chunk := bytes.Repeat([]byte("x"), openCodeProjectLimit+257)
	n, err := output.Write(chunk)
	if err != nil || n != len(chunk) || output.Len() != openCodeProjectLimit || !output.truncated {
		t.Fatal("native V2 stderr writer did not cap input")
	}
	if n, err := output.Write([]byte("late")); err != nil || n != 4 || output.Len() != openCodeProjectLimit {
		t.Fatal("native V2 stderr writer exceeded cap after overflow")
	}
}

func openCodeProjectV2Skills(body []byte, project string) (map[string]openCodeProjectSkill, error) {
	value, err := openCodeProjectJSON(body)
	if err != nil {
		return nil, err
	}
	envelope, ok := value.(map[string]any)
	if !ok || len(envelope) != 2 {
		return nil, fmt.Errorf("V2 native skill API did not return a source-bound envelope")
	}
	location, ok := envelope["location"].(map[string]any)
	if !ok || len(location) != 1 || location["directory"] != project {
		return nil, fmt.Errorf("V2 native skill API selected another project")
	}
	rows, ok := envelope["data"].([]any)
	if !ok || len(rows) > 200 {
		return nil, fmt.Errorf("V2 native skill API data is not a bounded skill array")
	}
	result := make(map[string]openCodeProjectSkill, len(rows))
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("V2 native skill entry is not an object")
		}
		id, okID := row["id"].(string)
		name, okName := row["name"].(string)
		path, okPath := row["path"].(string)
		content, okContent := row["content"].(string)
		if !okID || !okName || !okPath || !okContent || id == "" || name == "" || path == "" || len(content) > 1<<18 || result[id].Name != "" {
			return nil, fmt.Errorf("V2 native skill row is missing identity, path, body, or is duplicated")
		}
		result[id] = openCodeProjectSkill{Name: id, Location: path, Content: content}
	}
	return result, nil
}
