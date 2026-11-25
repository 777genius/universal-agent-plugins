package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
)

type browserSessionPrepared struct {
	started chan context.Context
	release chan struct{}
	effects atomic.Int32
}

func (prepared *browserSessionPrepared) Review() skillspicker.Plan {
	return skillspicker.Plan{Confirmable: true, Actions: []skillspicker.Action{{CandidateId: "candidate", TargetId: "codex", Alias: "review", Action: "install", DestinationPath: "/isolated/review"}}}
}
func (*browserSessionPrepared) Check(context.Context) error { return nil }
func (prepared *browserSessionPrepared) Apply(ctx context.Context) (skillspicker.Outcome, error) {
	prepared.effects.Add(1)
	prepared.started <- ctx
	select {
	case <-prepared.release:
		return skillspicker.Outcome{State: "succeeded"}, nil
	case <-ctx.Done():
		return skillspicker.Outcome{}, ctx.Err()
	}
}
func (*browserSessionPrepared) Close() {}

type browserSessionFixture struct {
	server        *skillspicker.Server
	prepared      *browserSessionPrepared
	origin, token string
	client        *http.Client
}

func newBrowserSessionFixture(t *testing.T) browserSessionFixture {
	t.Helper()
	prepared := &browserSessionPrepared{started: make(chan context.Context, 1), release: make(chan struct{})}
	callbacks := skillspicker.Callbacks{
		Scan: func(context.Context, string) (skillspicker.Catalog, error) {
			return skillspicker.Catalog{CatalogId: "catalog", CatalogDigest: "digest", Complete: true, Candidates: []skillspicker.Candidate{{Id: "candidate", Name: "review", Path: "review", Status: skillspicker.Available}}}, nil
		},
		Preview: func(context.Context, string) (skillspicker.Preview, error) {
			return skillspicker.Preview{}, nil
		},
		Targets: func(context.Context) ([]skillspicker.Target, error) {
			return []skillspicker.Target{{Id: "codex", Available: true}}, nil
		},
		Prepare: func(context.Context, skillspicker.PlanRequest) (skillspicker.Prepared, error) { return prepared, nil },
	}
	server, err := skillspicker.Start(context.Background(), skillspicker.Options{Callbacks: callbacks})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	parsed, err := url.Parse(server.URL())
	if err != nil {
		t.Fatal(err)
	}
	token, _ := url.ParseQuery(parsed.Fragment)
	parsed.Fragment = ""
	fixture := browserSessionFixture{server: server, prepared: prepared, origin: strings.TrimSuffix(parsed.String(), "/"), token: token.Get("capability"), client: &http.Client{Timeout: 2 * time.Second}}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var session skillspicker.Session
		fixture.request(t, "GET", "/api/v1/session", "", &session)
		if session.State == "ready" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("picker scan did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	return fixture
}

func (fixture browserSessionFixture) request(t *testing.T, method, path, body string, result any) {
	t.Helper()
	request, err := http.NewRequest(method, fixture.origin+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+fixture.token)
	request.Header.Set("Origin", fixture.origin)
	request.Header.Set("Content-Type", "application/json")
	response, err := fixture.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("request %s: %d %s", path, response.StatusCode, data)
	}
	if result != nil {
		if err := json.NewDecoder(response.Body).Decode(result); err != nil {
			t.Fatal(err)
		}
	}
}
func (fixture browserSessionFixture) accept(t *testing.T) context.Context {
	t.Helper()
	var plan skillspicker.Plan
	fixture.request(t, "POST", "/api/v1/plans", `{"catalogId":"catalog","catalogDigest":"digest","selectedIds":["candidate"],"targetIds":["codex"]}`, &plan)
	fixture.request(t, "POST", "/api/v1/plans/"+plan.PlanId+"/apply", "", nil)
	select {
	case ctx := <-fixture.prepared.started:
		return ctx
	case <-time.After(2 * time.Second):
		t.Fatal("accepted worker never started")
		return nil
	}
}
func (fixture browserSessionFixture) complete(t *testing.T) {
	t.Helper()
	close(fixture.prepared.release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		var session skillspicker.Session
		fixture.request(t, "GET", "/api/v1/session", "", &session)
		if session.OperationId != nil {
			var operation skillspicker.Operation
			fixture.request(t, "GET", "/api/v1/operations/"+*session.OperationId, "", &operation)
			if operation.State == "succeeded" {
				return
			}
			if operation.State == "failed" {
				t.Fatalf("accepted work failed: %+v", operation)
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("accepted work did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}
func browserSessionCommand(ctx context.Context, output io.Writer) *cobra.Command {
	command := &cobra.Command{}
	command.SetContext(ctx)
	command.SetOut(output)
	command.SetErr(output)
	return command
}

func TestBrowserLaunchFailureHandsUnusedSessionToTerminal(t *testing.T) {
	fixture := newBrowserSessionFixture(t)
	var output bytes.Buffer
	app := App{reviewOutput: &output, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		if !strings.Contains(output.String(), "Picker URL:") {
			t.Fatal("fallback question preceded usable local URL")
		}
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}}
	handled, err := runSkillsBrowserSession(browserSessionCommand(t.Context(), &output), app, &options{}, fixture.server, func(context.Context, string) error { return errors.New("fixture opener unavailable") })
	if handled || err != nil || fixture.prepared.effects.Load() != 0 {
		t.Fatalf("handled=%t err=%v effects=%d", handled, err, fixture.prepared.effects.Load())
	}
	select {
	case <-fixture.server.Done():
	default:
		t.Fatal("terminal takeover retained live browser authority")
	}
}

func TestBrowserAdmissionDuringFallbackQuestionRetainsBrowserOwner(t *testing.T) {
	fixture := newBrowserSessionFixture(t)
	var output bytes.Buffer
	promptDone := make(chan context.Context, 1)
	app := App{reviewOutput: &output, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		worker := fixture.accept(t)
		promptDone <- worker
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}}
	finished := make(chan struct {
		handled bool
		err     error
	}, 1)
	go func() {
		handled, err := runSkillsBrowserSession(browserSessionCommand(t.Context(), &output), app, &options{}, fixture.server, func(context.Context, string) error { return errors.New("fixture opener") })
		finished <- struct {
			handled bool
			err     error
		}{handled, err}
	}()
	var worker context.Context
	select {
	case worker = <-promptDone:
	case <-time.After(2 * time.Second):
		t.Fatal("fallback question did not admit browser operation")
	}
	if worker.Err() != nil {
		t.Fatalf("browser operation cancelled during fallback: %v", worker.Err())
	}
	select {
	case result := <-finished:
		t.Fatalf("browser ownership returned early: %+v", result)
	default:
	}
	fixture.complete(t)
	fixture.server.Close()
	result := <-finished
	if !result.handled || result.err != nil || fixture.prepared.effects.Load() != 1 {
		t.Fatalf("handled=%t err=%v effects=%d", result.handled, result.err, fixture.prepared.effects.Load())
	}
	if !strings.Contains(output.String(), "terminal selection will not start another installation") {
		t.Fatalf("missing takeover explanation: %s", output.String())
	}
}

func TestBrowserInteractionFailuresPreserveAlreadyAcceptedWork(t *testing.T) {
	for _, kind := range []string{"opened-output", "url-output", "prompt"} {
		t.Run(kind, func(t *testing.T) {
			fixture := newBrowserSessionFixture(t)
			worker := fixture.accept(t)
			cause := errors.New("fixture interaction failed")
			output := io.Writer(shortPromptWriter{})
			if kind == "prompt" {
				output = &bytes.Buffer{}
			}
			app := App{reviewOutput: output, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) { return prompt.ConfirmationResult{}, cause }}}
			entered := make(chan struct{})
			finished := make(chan error, 1)
			go func() {
				_, err := runSkillsBrowserSession(browserSessionCommand(t.Context(), output), app, &options{}, fixture.server, func(context.Context, string) error {
					close(entered)
					if kind == "opened-output" {
						return nil
					}
					return errors.New("fixture opener")
				})
				finished <- err
			}()
			<-entered
			if worker.Err() != nil {
				t.Fatalf("interaction failure cancelled accepted work: %v", worker.Err())
			}
			select {
			case err := <-finished:
				t.Fatalf("interaction failure abandoned browser: %v", err)
			default:
			}
			fixture.complete(t)
			fixture.server.Close()
			err := <-finished
			want := error(io.ErrShortWrite)
			if kind == "prompt" {
				want = cause
			}
			if !errors.Is(err, want) || fixture.prepared.effects.Load() != 1 {
				t.Fatalf("err=%v effects=%d", err, fixture.prepared.effects.Load())
			}
		})
	}
}

func TestBrowserOpenerChildContextDoesNotOwnAcceptedWork(t *testing.T) {
	fixture := newBrowserSessionFixture(t)
	worker := fixture.accept(t)
	var output bytes.Buffer
	var opener context.Context
	entered := make(chan struct{})
	finished := make(chan error, 1)
	app := App{reviewOutput: &output, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) { return prompt.ConfirmationResult{Accepted: false}, nil }}}
	go func() {
		_, err := runSkillsBrowserSession(browserSessionCommand(t.Context(), &output), app, &options{}, fixture.server, func(ctx context.Context, _ string) error {
			opener = ctx
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > skillsBrowserLaunchTimeout {
				t.Error("opener is unbounded")
			}
			close(entered)
			return context.DeadlineExceeded
		})
		finished <- err
	}()
	<-entered
	deadline := time.Now().Add(2 * time.Second)
	for opener.Err() == nil {
		if time.Now().After(deadline) {
			t.Fatal("opener child context was not released")
		}
		time.Sleep(time.Millisecond)
	}
	if worker.Err() != nil {
		t.Fatalf("opener timeout cancelled accepted work: %v", worker.Err())
	}
	fixture.complete(t)
	fixture.server.Close()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestBrowserWaitHonorsExplicitCommandCancellation(t *testing.T) {
	fixture := newBrowserSessionFixture(t)
	fixture.server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// Both channels are ready. The session closing cannot hide cancellation.
	for range 20 {
		if err := waitForSkillsPicker(ctx, fixture.server); !errors.Is(err, context.Canceled) {
			t.Fatalf("closed session hid command cancellation: %v", err)
		}
	}
}
