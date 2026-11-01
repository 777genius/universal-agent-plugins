package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
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

// V2 removed "debug skill". Its official /api/skill endpoint exposes the
// actual process-local skill registry. Never replace this proof with a file
// inventory or a synthetic successful native response.
func (f *openCodeProjectFixture) skillsV2(t *testing.T, label, project string) map[string]openCodeProjectSkill {
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
	command.Dir = project
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	password := hex.EncodeToString(key)
	// This invocation proves Skills, not MCP transport. Suppress the two
	// known fixture remote MCP entries in the process-only V2 overlay so
	// their initial connection attempts cannot masquerade as a skill result.
	disabled := map[string]any{"type": "remote", "url": "http://127.0.0.1:9/disabled", "disabled": true}
	overlay, err := json.Marshal(map[string]any{
		"mcp": map[string]any{"servers": map[string]any{
			"scope-marker": disabled, "uap-user-control": disabled,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	command.Env = append(f.env(), "OPENCODE_PASSWORD="+password, "OPENCODE_CONFIG_CONTENT="+string(overlay))
	command.Stdout = io.Discard
	var stderr bytes.Buffer
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
				lastError = nil
				break
			}
			lastError = fmt.Errorf("native V2 skill route not ready (status %d)", response.StatusCode)
		} else {
			lastError = err
		}
		select {
		case <-ctx.Done():
		case <-time.After(110 * time.Millisecond):
		}
	}
	if lastError != nil || len(body) == 0 || len(body) > 1<<20 {
		t.Fatalf("V2 native skill HTTP observation failed: %v; bounded startup stderr: %q", lastError, strings.TrimSpace(stderr.String()))
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
	if before != [2]int64{} || f.recorder.counts() != [2]int64{} {
		t.Fatal("V2 skill-only native observation unexpectedly contacted passive MCP probe")
	}
	parsed, err := openCodeProjectV2Skills(body)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func openCodeProjectV2Skills(body []byte) (map[string]openCodeProjectSkill, error) {
	value, err := openCodeProjectJSON(body)
	if err != nil {
		return nil, err
	}
	rows, ok := value.([]any)
	if !ok || len(rows) > 200 {
		return nil, fmt.Errorf("V2 native skill API response is not a bounded array")
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
