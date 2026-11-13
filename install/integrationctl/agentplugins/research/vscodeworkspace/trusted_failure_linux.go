//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// A terminal protocol failure is distinct from a supervisor cleanup failure.
// Only this validated private handoff may request planned teardown on failure.
type trustedDriverFailure struct{ phase, reason string }

func (e *trustedDriverFailure) Error() string {
	return "trusted driver failure: " + e.phase + "/" + e.reason
}

func exactFailureKeys(value map[string]any, names string) bool {
	keys := strings.Fields(names)
	if len(value) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := value[key]; !ok {
			return false
		}
	}
	return true
}
func failureChoice(value any, choices string) bool {
	s, ok := value.(string)
	return ok && strings.Contains(" "+choices+" ", " "+s+" ") && s != "" && !strings.ContainsAny(s, " \t\r\n")
}

const failureReasons = "unclassified publisher-startup-failed publisher-spawn-failed publisher-stream-overflow publisher-proof-bound publisher-proof-missing publisher-proof-mismatch publisher-stream-malformed publisher-pre-verifier-stderr publisher-endpoint-ambiguous publisher-endpoint-refused publisher-port-refused stage-deadline operation-deadline browser-context-mismatch workbench-pages-ambiguous workbench-page-missing publisher-shutdown-incomplete publisher-terminal-result-mismatch observer-deadline driver-output-bound picker-input-deadline picker-input-ambiguous picker-label-missing picker-label-ambiguous picker-row-bound ui-selection-refused ui-dialog-unexpected ui-dialog-unplanned mcp-trust-origin mcp-state-refused mcp-start-deadline trigger-publication-refused"

func admitTrustedFailure(body []byte, stage, nonce string) (*trustedDriverFailure, error) {
	value, err := uniqueJSON(body)
	if err != nil {
		return nil, err
	}
	outer, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("trusted terminal envelope is not an object")
	}
	if outer["type"] == "driver-result" {
		return nil, nil // success retains its complete existing validator
	}
	if outer["type"] != "driver-failure" {
		return nil, fmt.Errorf("unknown trusted terminal envelope")
	}
	if !exactFailureKeys(outer, "type stage nonce phase code visibleNativeUI diagnostics") || outer["stage"] != stage || outer["nonce"] != nonce || outer["code"] != "closed-stage-contract-refused" ||
		!failureChoice(outer["phase"], "publisher-startup native-devtools-admission workbench-focus workspace-trust native-mcp-start mcp-command-palette mcp-server-picker mcp-server-selection mcp-action-picker mcp-start-action mcp-trust mcp-state ui-trigger-publication observer-result publisher-shutdown driver-result-publication parent-planned-shutdown") {
		return nil, fmt.Errorf("closed driver failure context refused")
	}
	if err := validateTrustedFailureUI(outer["visibleNativeUI"]); err != nil {
		return nil, err
	}
	facts, ok := outer["diagnostics"].(map[string]any)
	if !ok || !exactFailureKeys(facts, "mcp reason captureReason publisherExitCode publisherSignal proofSeen endpointSeen publisherMergedBytes preVerifierStderrBytes mergedBytesCapped stderrBytesCapped startupSignatures") ||
		!failureChoice(facts["reason"], failureReasons) || !failureChoice(facts["captureReason"], "none "+failureReasons) || !failureChoice(facts["publisherSignal"], "none other SIGTERM SIGKILL SIGABRT SIGTRAP SIGSEGV SIGILL SIGBUS SIGFPE SIGHUP") {
		return nil, fmt.Errorf("closed driver failure diagnostics refused")
	}
	if err := validateTrustedFailureFields(facts); err != nil {
		return nil, err
	}

	return &trustedDriverFailure{phase: outer["phase"].(string), reason: facts["reason"].(string)}, nil
}

func validateTrustedMCPState(value any) error {
	mcp, err := objectKeys(value, strings.Fields("startSelected trustAccepted lastState stopped starting running other launchNotification outputVisible launchError"), nil)
	if err != nil || !failureChoice(mcp["lastState"], "none Stopped Starting Running other") || !failureChoice(mcp["launchError"], "none variable-resolution other diagnostic-refused") {
		return fmt.Errorf("closed MCP state refused")
	}
	for _, key := range []string{"startSelected", "trustAccepted", "launchNotification", "outputVisible"} {
		if _, ok := mcp[key].(bool); !ok {
			return fmt.Errorf("closed MCP boolean refused")
		}
	}
	for _, key := range []string{"stopped", "starting", "running", "other"} {
		n, err := trustedNumber(mcp[key])
		if err != nil || n < 0 || n > 255 {
			return fmt.Errorf("closed MCP count refused")
		}
	}

	return nil
}

func validateTrustedFailureFields(facts map[string]any) error {
	if err := validateTrustedMCPState(facts["mcp"]); err != nil {
		return err
	}

	for _, key := range []string{"proofSeen", "endpointSeen", "mergedBytesCapped", "stderrBytesCapped"} {
		if _, ok := facts[key].(bool); !ok {
			return fmt.Errorf("closed driver failure boolean refused")
		}
	}
	for _, key := range []string{"publisherMergedBytes", "preVerifierStderrBytes"} {
		n, err := trustedNumber(facts[key])
		if err != nil || n < 0 || n > (1<<20)+1 {
			return fmt.Errorf("closed driver failure count refused")
		}
	}
	if facts["publisherExitCode"] != nil {
		n, err := trustedNumber(facts["publisherExitCode"])
		if err != nil || n < 0 || n > 255 {
			return fmt.Errorf("closed driver failure exit refused")
		}
	}
	signatures, ok := facts["startupSignatures"].(map[string]any)
	if !ok || !exactFailureKeys(signatures, "sandboxUnavailable namespaceDenied sharedLibraryMissing glibcVersionMissing displayUnavailable") {
		return fmt.Errorf("closed driver failure signatures refused")
	}
	for _, value := range signatures {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("closed driver failure signature refused")
		}
	}
	return nil
}

func trustedFailureForShutdown(err error) (error, error) {
	var failure *trustedDriverFailure
	if errors.As(err, &failure) {
		return nil, err
	}
	return err, nil
}
func runTrustedDisplay(outer func(func(context.Context, isolationProof) error) (childRecord, error), ready func(context.Context, isolationProof) error, sequence func(context.Context) error) (childRecord, error) {
	var sequenceErr error
	record, err := outer(func(ctx context.Context, proof isolationProof) error {
		sequenceErr = ready(ctx, proof)
		if sequenceErr == nil {
			sequenceErr = sequence(ctx)
		}
		return nil
	})
	return record, errors.Join(sequenceErr, err)
}

func validateTrustedFailureUI(value any) error {
	rows, ok := value.([]any)
	body, err := json.Marshal(value)
	if !ok || len(rows) > 16 || err != nil || len(body) > 4096 {
		return fmt.Errorf("closed driver failure UI bound refused")
	}
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("closed driver failure UI row refused")
		}
		switch row["kind"] {
		case "dialog":
			_, boolean := row["messagePresent"].(bool)
			if !exactFailureKeys(row, "kind messagePresent") || !boolean {
				return fmt.Errorf("closed driver failure dialog refused")
			}
		case "quick-input", "quick-label":
			if !exactFailureKeys(row, "kind") {
				return fmt.Errorf("closed driver failure quick UI refused")
			}
		default:
			return fmt.Errorf("closed driver failure UI kind refused")
		}
	}
	return nil
}
