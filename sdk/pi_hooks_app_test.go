package pluginkitai

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/pi"
)

func piTestFrame(t *testing.T) []byte {
	t.Helper()
	value := pi.ObservationEvent{SchemaVersion: 1, Event: "agent_settled", ObserverID: "TEST-observer", ObservationID: "TEST-observation", SessionID: "TEST-session", CWD: filepath.Clean(t.TempDir()), Mode: "tui", PiPID: 123,
		LoadedRevision: pi.LoadedRevision{ProjectionDigest: "sha256:" + strings.Repeat("a", 64), CanonicalDigest: "sha256:" + strings.Repeat("b", 64), BootstrapVersion: 1, BindingID: "TEST-binding", RuntimeRoot: filepath.Clean(t.TempDir()), PrimarySHA256: strings.Repeat("c", 64)}, Settled: &pi.Settled{Aborted: false, BoundaryOutcome: "completed"}}
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestPiObservationDispatchIsNeutralAndCancellable(t *testing.T) {
	iox := &testIO{in: piTestFrame(t)}
	app := New(Config{Name: "TEST-pi", Args: []string{"TEST-helper", "pi-event"}, IO: iox, Env: testEnv{}})
	called := 0
	app.Pi().OnObservationContext(func(ctx context.Context, e *pi.ObservationEvent) (*pi.ObservationResponse, error) {
		called++
		if e.Event != "agent_settled" || ctx.Err() != nil {
			t.Fatal("wrong invocation")
		}
		return nil, nil
	})
	if code := app.Run(); code != 0 || called != 1 || iox.out.String() != "{}" {
		t.Fatalf("dispatch: code=%d called=%d stdout=%s stderr=%s", code, called, iox.out.String(), iox.err.String())
	}
}

func TestPiObservationRejectsMalformedOrPrivateFrames(t *testing.T) {
	body := string(piTestFrame(t))
	for name, invalid := range map[string]string{
		"duplicate":       strings.Replace(body, `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1),
		"trailing":        body + `{}`,
		"private prompt":  strings.Replace(body, `"event":"agent_settled"`, `"event":"agent_settled","prompt_text":"private"`, 1),
		"missing aborted": strings.Replace(body, `"aborted":false,`, "", 1),
		"unknown outcome": strings.Replace(body, `"completed"`, `"unknown"`, 1),
		"cross event":     strings.Replace(body, `"agent_settled"`, `"ui_prompt_start"`, 1),
		"oversize":        strings.Repeat(" ", 65537),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := pi.DecodeObservation([]byte(invalid)); err == nil {
				t.Fatal("invalid frame admitted")
			}
		})
	}
	var value pi.ObservationEvent
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatal(err)
	}
	value.Settled = nil
	value.Event = "ui_prompt_start"
	value.Prompt = &pi.Prompt{Kind: "confirm"}
	question, _ := json.Marshal(value)
	if _, err := pi.DecodeObservation(question); err != nil {
		t.Fatal("question rejected", err)
	}
}
