//go:build linux || darwin

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

type acpMetadataWriter struct {
	bytes.Buffer
	closes int
}

func (w *acpMetadataWriter) Close() error {
	w.closes++
	return nil
}

func acpMetadataPositive() []string {
	return []string{
		`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentCapabilities":{},"_meta":{"unknown":"preserved"}}}`,
		`{"jsonrpc":"2.0","id":2,"result":{"sessionId":"native-session"}}`,
		`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"native-session","update":{"sessionUpdate":"available_commands_update","availableCommands":[{"name":"uap-acp-metadata-marker","description":"inert"}]}}}`,
	}
}

func TestACPMetadataExchangeUsesOnlyTwoRequestsAndNaturalEOF(t *testing.T) {
	p := &acpMetadataExchange{}
	w := &acpMetadataWriter{}
	if err := p.exchange(w, strings.NewReader(strings.Join(acpMetadataPositive(), "\n")+"\n"), "/owned/project"); err != nil {
		t.Fatal(err)
	}
	if !p.StdoutEOF || !p.MarkerListed || p.SessionID != "native-session" || w.closes != 1 || len(p.Sent) != 2 {
		t.Fatal("finite metadata exchange did not close naturally", p, w.closes)
	}
	wire := bytes.Split(bytes.TrimSuffix(w.Bytes(), []byte{'\n'}), []byte{'\n'})
	expected := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{"fs":{"readTextFile":false,"writeTextFile":false},"terminal":false},"clientInfo":{"name":"uap-acp-metadata-observer","version":"1"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":"/owned/project","mcpServers":[]}}`,
	}
	if len(wire) != len(expected) || !bytes.HasSuffix(w.Bytes(), []byte{'\n'}) {
		t.Fatal("actual ACP stdin must contain exactly two newline-terminated requests")
	}
	for index, line := range wire {
		got, err := shared.DecodeStrictJSONObject(line)
		want, wantErr := shared.DecodeStrictJSONObject([]byte(expected[index]))
		if err != nil || wantErr != nil || !reflect.DeepEqual(got, want) {
			t.Fatal("actual ACP stdin request or parameters differ", index, err, wantErr)
		}
	}
	// Self-recorded Sent remains supplementary to the actual writer bytes.
	for index, method := range []string{"initialize", "session/new"} {
		if p.Sent[index]["method"] != method || p.Sent[index]["id"] != index+1 {
			t.Fatal("extra authority or incorrect request", p.Sent)
		}
	}
	last := p.Sent[1]["params"].(map[string]any)
	if last["cwd"] != "/owned/project" || len(last["mcpServers"].([]any)) != 0 {
		t.Fatal("selected cwd or empty session MCP input changed")
	}
}

func TestACPMetadataExchangeRefusesMalformedOrUncorrelatedFrames(t *testing.T) {
	for _, fault := range []string{"duplicate-key", "wrong-id", "duplicate-response", "early-session", "wrong-version", "foreign-session", "oversize", "early-eof"} {
		t.Run(fault, func(t *testing.T) {
			frames := acpMetadataPositive()
			switch fault {
			case "duplicate-key":
				frames[0] = strings.Replace(frames[0], `"id":1`, `"id":1,"ID":1`, 1)
			case "wrong-id":
				frames[0] = strings.Replace(frames[0], `"id":1`, `"id":3`, 1)
			case "duplicate-response":
				frames = append(frames[:1], append([]string{frames[0]}, frames[1:]...)...)
			case "early-session":
				frames[0], frames[1] = frames[1], frames[0]
			case "wrong-version":
				frames[0] = strings.Replace(frames[0], `"protocolVersion":1`, `"protocolVersion":2`, 1)
			case "foreign-session":
				frames[2] = strings.ReplaceAll(frames[2], "native-session", "foreign-session")
			case "oversize":
				frames[0] = strings.Repeat("x", acpMetadataLimit+1)
			case "early-eof":
				frames = frames[:1]
			}
			if err := (&acpMetadataExchange{}).exchange(&acpMetadataWriter{}, strings.NewReader(strings.Join(frames, "\n")+"\n"), "/owned/project"); err == nil {
				t.Fatal("invalid metadata exchange accepted", fault)
			}
		})
	}
}

func TestACPMetadataExchangeStopsClientRequestsWithoutAnyReply(t *testing.T) {
	for _, method := range []string{"_kiro/auth/getAccessToken", "session/request_permission", "terminal/create", "unknown/client-request"} {
		t.Run(method, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 99, "method": method,
				"params": map[string]string{"token": "private-auth-canary"}})
			if err != nil {
				t.Fatal(err)
			}
			p, w := &acpMetadataExchange{}, &acpMetadataWriter{}
			if err := p.exchange(w, bytes.NewReader(append(body, '\n')), "/owned/project"); err != nil {
				t.Fatal(err)
			}
			if len(p.Sent) != 1 || p.StoppedMethod != method || w.closes != 1 || !p.StdoutEOF {
				t.Fatal("request advanced the session or did not close", p)
			}
			saved, err := json.Marshal(p)
			if err != nil || bytes.Contains(saved, []byte("private-auth-canary")) {
				t.Fatal("client request parameters leaked")
			}
		})
	}
}

func TestACPMetadataCatalogReplacementCannotRetainRemovedMarker(t *testing.T) {
	frames := append(acpMetadataPositive(), `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"native-session","update":{"sessionUpdate":"available_commands_update","availableCommands":[]}}}`)
	p := &acpMetadataExchange{}
	if err := p.exchange(&acpMetadataWriter{}, strings.NewReader(strings.Join(frames, "\n")+"\n"), "/owned/project"); err != nil {
		t.Fatal(err)
	}
	if p.MarkerListed {
		t.Fatal("replaced command catalog retained old marker")
	}
}

func TestACPMetadataCompletionClassDoesNotPublishNativeErrorContents(t *testing.T) {
	for _, sample := range []struct{ message, class string }{
		{"native token=private-error-canary forced supervised cleanup", "forced-cleanup"},
		{"unrecognized native token=private-error-canary", "completion-refused"},
	} {
		class := acpMetadataCompletionClass(fmt.Errorf("%s", sample.message))
		if class != sample.class || strings.Contains(class, "private-error-canary") {
			t.Fatal("completion diagnostic exposed native error content", class)
		}
	}
}
