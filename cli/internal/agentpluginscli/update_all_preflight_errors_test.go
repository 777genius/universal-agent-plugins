package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// Keep the normal acquisition ports for the peer and fail only the selected
// producer. The dry-run factory below carries this seam through both nested
// dependency previews; replacing SourceAcquirer alone would miss that boundary.
type preflightSourceFailure struct {
	SourceAcquirer
	sourceacquisition.DirectAcquirer
	origin string
	local  string
	fail   func(context.Context) error
	calls  int
}

func (source *preflightSourceFailure) AcquireLocal(ctx context.Context, path string) (domain.PackageSnapshot, error) {
	if source.origin == "imperative" && filepath.Clean(path) == filepath.Clean(source.local) {
		source.calls++
		return domain.PackageSnapshot{}, source.fail(ctx)
	}
	return source.SourceAcquirer.AcquireLocal(ctx, path)
}

func (source *preflightSourceFailure) AcquireArchive(ctx context.Context, archive sourcespec.Archive, digest, tree string) (domain.PackageSnapshot, error) {
	if source.origin == "saved" {
		source.calls++
		return domain.PackageSnapshot{}, source.fail(ctx)
	}
	return source.DirectAcquirer.AcquireArchive(ctx, archive, digest, tree)
}

// Accept a real prefix and fail once. This distinguishes an incomplete result
// document from a writer which refuses every byte, and exposes accidental
// entrypoint attempts to append a second failure envelope.
type preflightPartialWriter struct {
	bytes.Buffer
	cause error
	calls int
}

func (writer *preflightPartialWriter) Write(body []byte) (int, error) {
	writer.calls++
	if writer.calls == 1 {
		n := len(body) / 2
		if n == 0 {
			n = len(body)
		}
		_, _ = writer.Buffer.Write(body[:n])
		return n, writer.cause
	}
	return writer.Buffer.Write(body)
}

func TestPublicUpdateAllPreflightRetainsCausesAndSoleResult(t *testing.T) {
	for _, origin := range []string{"saved", "imperative"} {
		for _, failure := range []string{"source", "usage", "foreign", "cancel"} {
			for _, output := range []string{"json", "human", "refused-json", "partial-json"} {
				t.Run(origin+"/"+failure+"/"+output, func(t *testing.T) {
					journey, _, local := savedMixedUpdateJourney(t)
					app := journey.cli.app
					activator, ok := app.Lifecycle.Activator.(*cliObservedActivator)
					if !ok {
						t.Fatal("mixed journey lost its observed activation port")
					}
					activationCalls := activator.calls
					clients, detectErr := app.Detector.Detect(t.Context())
					if detectErr != nil || len(clients) == 0 {
						t.Fatal("mixed journey native profiles unavailable", detectErr)
					}
					profileRoots := make([]string, 0, len(clients))
					for _, client := range clients {
						profileRoots = append(profileRoots, client.ConfigRoot)
					}
					profilesBefore := freshRestoreFiles(t, profileRoots...)
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					sentinel := errors.New("opaque-preflight-source-184731")
					cause := fmt.Errorf("%w: %w", sentinel, fs.ErrPermission)
					switch failure {
					case "usage":
						cause = CLIUsageError{Code: "TIMEOUT", Message: "opaque-preflight-usage-184731"}
					case "foreign":
						cause = &earlyForeignBindingError{cause: sentinel, scope: domain.ScopeUser}
					case "cancel":
						cause = context.Canceled
					}
					base := app.SourceAcquirer
					// Production preview factories may own a distinct acquirer. Keep
					// their construction and wrap the returned port rather than
					// silently bypassing their source policy.
					factory := app.SourceAcquirerForDryRun
					var injected []*preflightSourceFailure
					app.SourceAcquirerForDryRun = func(root string) SourceAcquirer {
						delegate := base
						if factory != nil {
							delegate = factory(root)
						}
						direct, ok := delegate.(sourceacquisition.DirectAcquirer)
						if !ok {
							t.Fatal("saved mixed fixture lacks its archive acquisition port")
						}
						wrapped := &preflightSourceFailure{SourceAcquirer: delegate, DirectAcquirer: direct, origin: origin, local: local}
						wrapped.fail = func(active context.Context) error {
							if active.Err() != nil {
								t.Errorf("preflight was canceled before the acquisition witness: %v", active.Err())
							}
							if failure == "cancel" {
								cancel()
								return active.Err()
							}
							return cause
						}
						injected = append(injected, wrapped)
						return wrapped
					}
					foreign := filepath.Join(journey.cli.root, "unrelated-preflight-user-file.txt")
					if err := os.WriteFile(foreign, []byte("unrelated user bytes must survive preflight\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					before := savedCheckFiles(t, journey.cli.root)
					var stdout, stderr bytes.Buffer
					var writer io.Writer = &stdout
					writerCause := errors.New("opaque-preflight-writer-592817")
					var refused *diagnosticResultWriter
					var partial *preflightPartialWriter
					switch output {
					case "refused-json":
						refused = &diagnosticResultWriter{fail: func([]byte) bool { return true }, err: writerCause}
						writer = refused
					case "partial-json":
						partial = &preflightPartialWriter{cause: writerCause}
						writer = partial
					}
					format := "json"
					if output == "human" {
						format = "human"
					}
					app.Output, app.ErrorOutput = writer, &stderr
					command := NewRoot(app)
					args := []string{"update", "--all", "--format", format}
					command.SetArgs(args)
					err := command.ExecuteContext(ctx)
					calls := 0
					for _, source := range injected {
						calls += source.calls
					}
					if calls == 0 {
						t.Fatal("public preflight never reached the selected acquisition failure")
					}
					if !reflect.DeepEqual(before, savedCheckFiles(t, journey.cli.root)) {
						t.Error("failed preflight changed persistent installation, intent or foreign files")
					}
					if activator.calls != activationCalls || !reflect.DeepEqual(profilesBefore, freshRestoreFiles(t, profileRoots...)) {
						t.Error("failed preflight invoked activation or changed native profile bytes")
					}
					if !errors.Is(err, cause) {
						t.Errorf("public preflight discarded original %s cause: %v", failure, err)
					}
					if failure == "source" && !errors.Is(err, fs.ErrPermission) {
						t.Errorf("filesystem classification was lost: %v", err)
					}
					const summary = "update --all preflight failed; no installation was changed"
					if err == nil || err.Error() != summary {
						t.Errorf("public preflight error lost its exact safe summary: %v", err)
					}
					// Emulate the executable's opt-in fallback exactly. Retaining
					// usage/ownership causes must not give the entrypoint another
					// JSON document after an owned (even partly written) result.
					if ShouldWriteAgentFailure(err) {
						t.Error("already reported batch opted into entrypoint failure output")
						_ = WriteAgentFailure(writer, err)
					}
					if refused != nil {
						if !errors.Is(err, writerCause) || refused.failures != 1 {
							t.Errorf("result write refusal was lost or retried: failures=%d err=%v", refused.failures, err)
						}
					} else if partial != nil {
						if !errors.Is(err, writerCause) || partial.calls != 1 || partial.Len() == 0 {
							t.Errorf("partial result failure was lost or appended: calls=%d bytes=%d err=%v", partial.calls, partial.Len(), err)
						}
					} else if format == "json" {
						envelope := decodeOneDiagnosticEnvelope(t, stdout.String())
						data, ok := envelope["data"].(map[string]any)
						if !ok || data["status"] != "preflight_failed" || data["updated"] != float64(0) {
							t.Errorf("failed preflight lost its compatible batch result: %s", stdout.String())
						}
					} else if !strings.Contains(stdout.String(), "Update all: preflight_failed") {
						t.Errorf("human preflight summary missing: %q", stdout.String())
					}
				})
			}
		}
	}
}

func TestPublicUpdateAllPreflightSuccessfulPreviewOutputFailure(t *testing.T) {
	for _, failOutput := range []bool{false, true} {
		t.Run(fmt.Sprint(failOutput), func(t *testing.T) {
			journey, _, _ := savedMixedUpdateJourney(t)
			before := savedCheckFiles(t, journey.cli.root)
			app := journey.cli.app
			activator, ok := app.Lifecycle.Activator.(*cliObservedActivator)
			if !ok {
				t.Fatal("mixed journey lost its observed activation port")
			}
			activationCalls := activator.calls
			clients, detectErr := app.Detector.Detect(t.Context())
			if detectErr != nil || len(clients) == 0 {
				t.Fatal("mixed journey native profiles unavailable", detectErr)
			}
			profileRoots := make([]string, 0, len(clients))
			for _, client := range clients {
				profileRoots = append(profileRoots, client.ConfigRoot)
			}
			profilesBefore := freshRestoreFiles(t, profileRoots...)
			var output bytes.Buffer
			cause := errors.New("preview writer refused")
			var writer io.Writer = &output
			if failOutput {
				writer = &diagnosticResultWriter{fail: func([]byte) bool { return true }, err: cause}
			}
			app.Output, app.ErrorOutput = writer, io.Discard
			command := NewRoot(app)
			command.SetArgs([]string{"update", "--all", "--dry-run", "--format", "json"})
			err := command.ExecuteContext(t.Context())
			if failOutput {
				if !errors.Is(err, cause) {
					t.Fatalf("successful preflight discarded its result-output cause: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				decoder := json.NewDecoder(&output)
				var envelope struct {
					Data updateAllResult `json:"data"`
				}
				if err := decoder.Decode(&envelope); err != nil {
					t.Fatal(err)
				}
				var extra any
				if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
					t.Fatalf("successful preview emitted extra output: %v", err)
				}
				if envelope.Data.Status != "planned" || envelope.Data.Planned != 2 || envelope.Data.Updated != 0 {
					t.Fatalf("successful preflight changed batch planning: %+v", envelope.Data)
				}
			}
			if !reflect.DeepEqual(before, savedCheckFiles(t, journey.cli.root)) {
				t.Fatal("preflight preview or output failure applied an update")
			}
			if activator.calls != activationCalls || !reflect.DeepEqual(profilesBefore, freshRestoreFiles(t, profileRoots...)) {
				t.Fatal("preflight preview or output failure invoked activation or changed native profile bytes")
			}
		})
	}
}

// This controlled discovery port fails only after the Directory installation
// exists; the public batch must retain its cause when projecting unknown status.
type preflightFailingDetector struct {
	calls int
	cause error
}

func (detector *preflightFailingDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	detector.calls++
	return nil, detector.cause
}

func TestUpdateAllDirectoryPreflightRetainsDetectionCause(t *testing.T) {
	fixture := newProjectionDirectoryFixture(t, true)
	clients, err := fixture.cli.app.Detector.Detect(t.Context())
	if err != nil || len(clients) != 1 {
		t.Fatal("seeded native profile unavailable", clients, err)
	}
	profilesBefore := freshRestoreFiles(t, clients[0].ConfigRoot)
	before := savedCheckFiles(t, fixture.cli.root)
	cause := &os.PathError{Op: "detect", Path: "opaque-detection-path-438921", Err: fs.ErrPermission}
	detector := &preflightFailingDetector{cause: cause}
	app := fixture.cli.app
	app.Detector = detector
	var stdout, stderr bytes.Buffer
	app.Output, app.ErrorOutput = &stdout, &stderr
	command := NewRoot(app)
	command.SetArgs([]string{"update", "--all", "--format", "json"})
	err = command.ExecuteContext(t.Context())
	if detector.calls != 1 {
		t.Errorf("expected one witnessed batch discovery failure, got %d", detector.calls)
	}
	if !errors.Is(err, cause) || !errors.Is(err, fs.ErrPermission) {
		t.Errorf("Directory batch discarded original typed discovery cause: %v", err)
	}
	const summary = "update --all preflight failed; no installation was changed"
	if err == nil || err.Error() != summary {
		t.Errorf("discovery refusal lost exact safe summary: %v", err)
	}
	if ShouldWriteAgentFailure(err) {
		t.Error("reported Directory batch opted into entrypoint failure output")
		_ = WriteAgentFailure(&stdout, err)
	}
	envelope := decodeOneDiagnosticEnvelope(t, stdout.String())
	data, ok := envelope["data"].(map[string]any)
	if !ok || data["status"] != "preflight_failed" || data["updated"] != float64(0) || data["failed"] != float64(1) {
		t.Errorf("discovery refusal lost one truthful batch failure: %s", stdout.String())
	}
	if !reflect.DeepEqual(before, savedCheckFiles(t, fixture.cli.root)) ||
		!reflect.DeepEqual(profilesBefore, freshRestoreFiles(t, clients[0].ConfigRoot)) {
		t.Fatal("Directory discovery failure changed persistent or native files")
	}
}
