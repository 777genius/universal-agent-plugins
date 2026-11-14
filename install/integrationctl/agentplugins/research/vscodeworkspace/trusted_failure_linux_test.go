//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func closedFailureFixture() map[string]any {
	return map[string]any{"type": "driver-failure", "stage": "empty-a", "nonce": "inert-nonce", "phase": "publisher-startup", "code": "closed-stage-contract-refused", "visibleNativeUI": []any{map[string]any{"kind": "dialog", "messagePresent": false}, map[string]any{"kind": "quick-input"}, map[string]any{"kind": "quick-label"}}, "diagnostics": map[string]any{
		"mcp": map[string]any{"startSelected": false, "trustAccepted": false, "lastState": "none", "launchNotification": false, "outputVisible": false, "launchError": "none", "postStartInput": "unobserved", "awaitBoundary": "unobserved", "elapsedMs": 0, "stopped": 0, "starting": 0, "running": 0, "other": 0}, "reason": "publisher-startup-failed", "captureReason": "none", "publisherExitCode": nil, "publisherSignal": "none", "proofSeen": true, "endpointSeen": false, "publisherMergedBytes": 0, "preVerifierStderrBytes": 0, "mergedBytesCapped": false, "stderrBytesCapped": false,
		"startupSignatures": map[string]any{"sandboxUnavailable": false, "namespaceDenied": false, "sharedLibraryMissing": false, "glibcVersionMissing": false, "displayUnavailable": false}}}
}
func failureBody(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestTrustedFailureClosedEnvelopeRefusesForeignAndMalformedFacts(t *testing.T) {
	for _, mutation := range []string{"valid", "type", "nonce", "stage", "phase", "code", "extra", "reason", "capture", "signal", "count", "exit", "boolean", "signature", "signature-extra", "mcp-state", "mcp-count", "mcp-bool", "mcp-extra"} {
		t.Run(mutation, func(t *testing.T) {
			value := closedFailureFixture()
			facts := value["diagnostics"].(map[string]any)
			switch mutation {
			case "type", "nonce", "stage", "phase", "code":
				value[mutation] = "foreign-secret"
			case "extra":
				value["private"] = "ws://private-cookie"
			case "reason":
				facts["reason"] = "private-error"
			case "capture":
				facts["captureReason"] = "private-error"
			case "signal":
				facts["publisherSignal"] = "private-signal"
			case "count":
				facts["publisherMergedBytes"] = (1 << 20) + 2
			case "exit":
				facts["publisherExitCode"] = -1
			case "boolean":
				facts["proofSeen"] = "true"
			case "signature":
				facts["startupSignatures"].(map[string]any)["sandboxUnavailable"] = "secret"
			case "mcp-state":
				facts["mcp"].(map[string]any)["lastState"] = "private text"
			case "mcp-count":
				facts["mcp"].(map[string]any)["stopped"] = 256
			case "mcp-bool":
				facts["mcp"].(map[string]any)["trustAccepted"] = 1
			case "mcp-extra":
				facts["mcp"].(map[string]any)["raw"] = "secret"
			case "signature-extra":
				facts["startupSignatures"].(map[string]any)["raw"] = "secret"
			}
			failure, err := admitTrustedFailure(failureBody(t, value), "empty-a", "inert-nonce")
			if mutation == "valid" {
				if err != nil || failure == nil {
					t.Fatal(failure, err)
				}
			} else if err == nil {
				t.Fatal("malformed failure admitted", failure)
			}
		})
	}
	for _, body := range []string{`{"type":"driver-failure"`, `{"type":"driver-failure","type":"driver-result"}`, strings.Repeat("x", resultLimit+1)} {
		if _, err := admitTrustedFailure([]byte(body), "empty-a", "inert-nonce"); err == nil {
			t.Fatal("malformed body admitted")
		}
	}
}

func TestTrustedFailureAtomicHandoffIsPromptAndNeverSuccess(t *testing.T) {
	c := testController(t, nil)
	c.trustedState = &trustedLifecycle{Nonce: "inert-nonce"}
	path := filepath.Join(t.TempDir(), "driver.json")
	body := failureBody(t, closedFailureFixture())
	if err := os.WriteFile(path+".pending", body, 0600); err != nil {
		t.Fatal(err)
	}
	if _, ready, err := trustedReadyResult(path); err != nil || ready {
		t.Fatal("partial failure admitted", err)
	}
	if err := os.Rename(path+".pending", path); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	start := time.Now()
	observed, err := c.waitTrustedObservation(ctx, path, "empty-a", "a", "prompt", nil, isolationProof{}, time.Now(), nil)
	var failure *trustedDriverFailure
	if !errors.As(err, &failure) || observed != nil || len(c.trustedState.Stages) != 0 || c.trustedState.Complete || time.Since(start) > 500*time.Millisecond {
		t.Fatal("failure became success or waited for timeout", observed, err)
	}
	if !strings.Contains(string(c.files["empty-a.observation.json"]), `"type":"driver-failure"`) {
		t.Fatal("closed failure lost")
	}
}

func TestTrustedFailureRetainsStageCauseWhileSupervisorProvesCleanup(t *testing.T) {
	for _, bad := range []bool{false, true} {
		runner := &failureCleanupRunner{inertDuplexRunner: inertDuplexRunner{body: testProof()}}
		if bad {
			runner.err = errors.New("inert cleanup failed")
		}
		c := testController(t, runner)
		stageErr := error(nil)
		record, cleanupErr := c.strict(t.Context(), "failure", []string{"/inert"}, "/", nil, func(context.Context, isolationProof) error {
			var err error
			err, stageErr = trustedFailureForShutdown(&trustedDriverFailure{phase: "publisher-startup", reason: "publisher-startup-failed"})
			return err
		})
		if stageErr == nil || record.TreeReaped == bad || (cleanupErr == nil) == bad {
			t.Fatal("protocol/cleanup conflated", stageErr, record, cleanupErr)
		}
	}
	ordinary := errors.New("malformed handoff")
	exchangeErr, retained := trustedFailureForShutdown(ordinary)
	if !errors.Is(exchangeErr, ordinary) || retained != nil {
		t.Fatal("unadmitted failure requested planned shutdown")
	}
}

func TestTrustedDisplayRetainsSequenceFailureAndCleanupFailure(t *testing.T) {
	sequenceErr := errors.New("inert stage failed")
	for _, cleanupFailed := range []bool{false, true} {
		runner := &failureCleanupRunner{inertDuplexRunner: inertDuplexRunner{body: testProof()}}
		if cleanupFailed {
			runner.err = errors.New("inert display cleanup failed")
		}
		c := testController(t, runner)
		record, err := runTrustedDisplay(func(exchange func(context.Context, isolationProof) error) (childRecord, error) {
			return c.strict(t.Context(), "Xvfb", []string{"/inert"}, "/", nil, exchange)
		}, func(context.Context, isolationProof) error { return nil }, func(context.Context) error { return sequenceErr })
		if !errors.Is(err, sequenceErr) || record.TreeReaped == cleanupFailed {
			t.Fatal("sequence failure lost or cleanup invented", record, err)
		}
	}
}

func TestTrustedDisplayRetainsReadinessFailureWithoutRunningSequence(t *testing.T) {
	readyErr := errors.New("inert display readiness refused")
	for _, cleanupFailed := range []bool{false, true} {
		runner := &failureCleanupRunner{inertDuplexRunner: inertDuplexRunner{body: testProof()}}
		if cleanupFailed {
			runner.err = errors.New("inert display cleanup failed")
		}
		c := testController(t, runner)
		sequenceCalled := false
		record, err := runTrustedDisplay(func(exchange func(context.Context, isolationProof) error) (childRecord, error) {
			return c.strict(t.Context(), "Xvfb", []string{"/inert"}, "/", nil, exchange)
		}, func(context.Context, isolationProof) error { return readyErr }, func(context.Context) error {
			sequenceCalled = true
			return nil
		})
		if !errors.Is(err, readyErr) || sequenceCalled || record.TreeReaped == cleanupFailed || record.PlannedStop == cleanupFailed {
			t.Fatal("readiness refusal lost, sequence ran, or cleanup inferred", record, err)
		}
	}
}

type failureCleanupRunner struct {
	inertDuplexRunner
	err error
}

func (r *failureCleanupRunner) RunDuplexWithPlannedShutdown(ctx context.Context, command ports.Command, exchange func(io.Writer, io.Reader) error) error {
	return errors.Join(r.inertDuplexRunner.RunDuplexWithPlannedShutdown(ctx, command, exchange), r.err)
}

func TestTrustedFailureUIRejectsMalformedAndPrivateRows(t *testing.T) {
	for _, value := range []any{
		nil, "private", []any{"private"}, []any{map[string]any{"kind": "unknown"}},
		[]any{map[string]any{"kind": "dialog", "messagePresent": "secret"}},
		[]any{map[string]any{"kind": "dialog"}},
		[]any{map[string]any{"kind": "dialog", "messagePresent": true, "text": "private"}},
		[]any{map[string]any{"kind": "quick-label", "text": "secret"}},
		[]any{map[string]any{"kind": "quick-input", "messagePresent": true}},
		make([]any, 17), []any{map[string]any{"kind": "quick-label", "text": strings.Repeat("x", 4097)}},
	} {
		envelope := closedFailureFixture()
		envelope["visibleNativeUI"] = value
		if _, err := admitTrustedFailure(failureBody(t, envelope), "empty-a", "inert-nonce"); err == nil {
			t.Fatal("malformed UI admitted")
		}
	}
}

func TestTrustedFailureMCPSubstepsRemainClosedAndNegative(t *testing.T) {
	for _, phase := range strings.Fields("mcp-command-palette mcp-server-picker mcp-server-selection mcp-action-picker mcp-start-action mcp-trust mcp-state") {
		value := closedFailureFixture()
		value["phase"] = phase
		value["diagnostics"].(map[string]any)["reason"] = "picker-input-deadline"
		failure, err := admitTrustedFailure(failureBody(t, value), "empty-a", "inert-nonce")
		if err != nil || failure == nil || failure.phase != phase {
			t.Fatal("closed negative MCP substep refused", failure, err)
		}
		value["phase"] = phase + " private"
		if _, err := admitTrustedFailure(failureBody(t, value), "empty-a", "inert-nonce"); err == nil {
			t.Fatal("arbitrary substep admitted")
		}
	}
}

func TestTrustedFailureLaunchDiagnosticPrivacy(t *testing.T) {
	if _, err := admitTrustedFailure(failureBody(t, closedFailureFixture()), "empty-a", "inert-nonce"); err != nil {
		t.Fatal("unmodified serialized fixture refused", err)
	}
	for _, key := range []string{"launchNotification", "outputVisible", "launchError", "postStartInput", "private"} {
		value := closedFailureFixture()
		facts := value["diagnostics"].(map[string]any)
		mcp := facts["mcp"].(map[string]any)
		mcp[key] = "private-cookie-path"
		if _, err := admitTrustedFailure(failureBody(t, value), "empty-a", "inert-nonce"); err == nil {
			t.Fatal("private diagnostic admitted", key)
		}
	}
}

func TestTrustedFailurePostStartInputAndNativeErrorEnums(t *testing.T) {
	for _, input := range strings.Fields("unobserved none action-picker trust-picker other multiple") {
		for _, launch := range strings.Fields("native-start command-not-found") {
			value := closedFailureFixture()
			mcp := value["diagnostics"].(map[string]any)["mcp"].(map[string]any)
			mcp["postStartInput"], mcp["launchError"] = input, launch
			if _, err := admitTrustedFailure(failureBody(t, value), "empty-a", "inert-nonce"); err != nil {
				t.Fatal("closed diagnostic enum refused", input, launch, err)
			}
		}
	}
	for _, input := range []any{nil, true, 1, "PRIVATE_INPUT", map[string]any{"text": "PRIVATE"}} {
		value := closedFailureFixture()
		value["diagnostics"].(map[string]any)["mcp"].(map[string]any)["postStartInput"] = input
		if _, err := admitTrustedFailure(failureBody(t, value), "empty-a", "inert-nonce"); err == nil {
			t.Fatal("unknown input diagnostic admitted")
		}
	}
}

func TestTrustedFailureAwaitBoundaryAndElapsedRemainClosed(t *testing.T) {
	for _, boundary := range strings.Fields(failureAwaitBoundaries) {
		for _, elapsed := range []int{0, 15000, 15001} {
			value := closedFailureFixture()
			mcp := value["diagnostics"].(map[string]any)["mcp"].(map[string]any)
			mcp["awaitBoundary"], mcp["elapsedMs"] = boundary, elapsed
			if failure, err := admitTrustedFailure(failureBody(t, value), "empty-a", "inert-nonce"); err != nil || failure == nil {
				t.Fatal("closed negative await observation refused", boundary, elapsed, err)
			}
		}
	}
	for key, invalid := range map[string][]any{
		"awaitBoundary": {nil, true, 1, "PRIVATE_BOUNDARY", "input-count PRIVATE"},
		"elapsedMs":     {nil, true, "1", -1, 1.5, json.Number("1.0"), 15002},
	} {
		for _, changed := range invalid {
			value := closedFailureFixture()
			value["diagnostics"].(map[string]any)["mcp"].(map[string]any)[key] = changed
			if _, err := admitTrustedFailure(failureBody(t, value), "empty-a", "inert-nonce"); err == nil {
				t.Fatal("invalid await observation admitted", key, changed)
			}
		}
		value := closedFailureFixture()
		delete(value["diagnostics"].(map[string]any)["mcp"].(map[string]any), key)
		if _, err := admitTrustedFailure(failureBody(t, value), "empty-a", "inert-nonce"); err == nil {
			t.Fatal("missing await observation admitted", key)
		}
	}
}
