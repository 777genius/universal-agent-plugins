//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

func (c *controller) trustedDriverEnvelope(body []byte, stage, trust string, driver isolationProof) (map[string]any, isolationProof, error) {
	value, err := uniqueJSON(body)
	if err != nil {
		return nil, isolationProof{}, err
	}
	outer, err := objectKeys(value, strings.Fields("type stage nonce trustPromptAccepted servers publisherProof publisherExit result"), nil)
	f := c.trustedState
	if err != nil || outer["type"] != "driver-result" || outer["stage"] != stage || outer["nonce"] != f.Nonce || outer["trustPromptAccepted"] != (trust == "prompt") {
		return nil, isolationProof{}, fmt.Errorf("trusted driver envelope differs")
	}
	proofBytes, _ := json.Marshal(outer["publisherProof"])
	var publisher isolationProof
	if err := json.Unmarshal(proofBytes, &publisher); err != nil {
		return nil, isolationProof{}, err
	}
	if err := validateIsolationProof(publisher, c.request.Input.UID, c.request.Input.GID, c.parentNS, nil); err != nil {
		return nil, isolationProof{}, err
	}
	if publisher.Namespace != driver.Namespace || publisher.PID == driver.PID {
		return nil, isolationProof{}, fmt.Errorf("publisher and driver process/namespace relationship differs")
	}
	exit, err := objectKeys(outer["publisherExit"], []string{"code", "signal"}, nil)
	if err != nil {
		return nil, isolationProof{}, err
	}
	if (exit["code"] != json.Number("0") || exit["signal"] != nil) && (exit["code"] != nil || exit["signal"] != "SIGTERM") {
		return nil, isolationProof{}, fmt.Errorf("publisher shutdown differs")
	}
	return outer, publisher, nil
}

func (c *controller) trustedObserverIdentity(outer map[string]any, stage, scope string, request map[string]any, driver, publisher isolationProof) (map[string]any, int64, error) {
	observed, err := objectKeys(outer["result"], strings.Fields("schemaVersion mode stage nonce collected version appRoot appHost folder home trusted observerPID networkNamespace skills definitions calls"), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("native observer incomplete or unknown: %w", err)
	}
	if err := c.trustedObserverContext(observed, stage, scope, request); err != nil {
		return nil, 0, err
	}
	pid, err := trustedNumber(observed["observerPID"])
	ns, nsErr := trustedNumber(observed["networkNamespace"])
	if err != nil || nsErr != nil || pid <= 0 || pid == int64(driver.PID) || pid == int64(publisher.PID) || ns <= 0 || uint64(ns) != driver.Namespace {
		return nil, 0, fmt.Errorf("actual extension host identity differs")
	}
	return observed, pid, nil
}

func (c *controller) trustedObserverContext(observed map[string]any, stage, scope string, request map[string]any) error {
	f := c.trustedState
	if observed["schemaVersion"] != json.Number("1") || observed["mode"] != trustedProjectMode || observed["stage"] != stage || observed["nonce"] != f.Nonce || observed["collected"] != true || observed["version"] != publisherVersion || observed["appRoot"] != request["appRoot"] || observed["appHost"] != "desktop" || observed["folder"] != f.Projects[scope] || observed["home"] != f.Home || observed["trusted"] != true {
		return fmt.Errorf("native observer identity differs")
	}
	return nil
}

func trustedObservedSkills(observed, request map[string]any) error {
	// Normalize only our independently authored expectation to the same JSON
	// number/array representation; never replace it with observed native bytes.
	expectedBody, _ := json.Marshal(request["skills"])
	expectedSkills, err := uniqueJSON(expectedBody)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(observed["skills"], expectedSkills) {
		return fmt.Errorf("native selected skills differ")
	}
	return nil
}

func (c *controller) trustedObservedServers(outer, observed, request map[string]any, scope string, pid int64, driver, publisher isolationProof, started time.Time, before []byte) error {
	f := c.trustedState
	selected := request["servers"].([]any)
	definitions, ok := observed["definitions"].([]any)
	calls, callsOK := observed["calls"].([]any)
	ui, uiOK := outer["servers"].([]any)
	if !ok || !callsOK || !uiOK || len(definitions) != len(selected) || len(calls) != len(selected) || len(ui) != len(selected) {
		return fmt.Errorf("native selected server cardinality differs")
	}
	for index, raw := range selected {
		expected := raw.(map[string]any)
		if err := c.trustedObservedDefinition(expected, definitions[index], ui[index], scope); err != nil {
			return err
		}
		facts, session, err := c.trustedObservedCall(calls[index], expected, request, pid, driver, publisher)
		if err != nil {
			return err
		}
		after, err := trustedEvents(filepath.Join(f.Root, scope+"-events.jsonl"))
		if err != nil {
			return err
		}
		if !bytes.HasPrefix(after, before) {
			return fmt.Errorf("native event history rewritten")
		}
		if err := validateTrustedProbeEvents(after[len(before):], facts, started, time.Now()); err != nil {
			return err
		}
		f.Sessions[session] = true
	}
	if len(selected) == 0 {
		after, err := trustedEvents(filepath.Join(f.Root, scope+"-events.jsonl"))
		if err != nil || !bytes.Equal(after, before) {
			return fmt.Errorf("absent project started probe")
		}
	}
	return nil
}

func (c *controller) trustedObservedDefinition(expected map[string]any, definition, server any, scope string) error {
	f := c.trustedState
	projection := map[string]any{}
	for _, key := range []string{"label", "command", "args", "env", "cwd"} {
		projection[key] = expected[key]
	}
	encoded, _ := json.Marshal(projection)
	want, err := uniqueJSON(encoded)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(definition, want) {
		return fmt.Errorf("native definition command/argv/env/cwd differs")
	}
	startedServer, err := objectKeys(server, strings.Fields("label originUri trustPromptAccepted nativeState"), nil)
	origin := (&url.URL{Scheme: "file", Path: filepath.Join(f.Projects[scope], ".vscode", "mcp.json")}).String()
	if err != nil || startedServer["label"] != expected["label"] || startedServer["originUri"] != origin || startedServer["nativeState"] != "Running" {
		return fmt.Errorf("native started server origin/state differs")
	}
	if _, ok := startedServer["trustPromptAccepted"].(bool); !ok {
		return fmt.Errorf("native server trust decision missing")
	}
	return nil
}

func (c *controller) trustedObservedCall(raw any, expected, request map[string]any, pid int64, driver, publisher isolationProof) (map[string]any, string, error) {
	call, err := objectKeys(raw, strings.Fields("label initialize tools facts operation gatewaySessionSha256 gatewayEndpointSha256"), nil)
	if err != nil || call["label"] != expected["label"] || call["operation"] != request["operation"] {
		return nil, "", fmt.Errorf("vendor gateway call identity differs")
	}
	for _, key := range []string{"gatewaySessionSha256", "gatewayEndpointSha256"} {
		text, ok := call[key].(string)
		if !ok || !shaPattern.MatchString(text) {
			return nil, "", fmt.Errorf("private gateway identity hash missing")
		}
	}
	facts, err := objectKeys(call["facts"], strings.Fields("nonce revision cwd argv plugin_root plugin_data literal once marker pid session"), nil)
	if err != nil {
		return nil, "", err
	}
	session, err := c.trustedProbeIdentity(facts, pid, driver, publisher)
	if err != nil {
		return nil, "", err
	}
	projection := map[string]any{}
	for key, item := range facts {
		if key != "pid" && key != "session" {
			projection[key] = item
		}
	}
	encoded, _ := json.Marshal(expected["facts"])
	want, err := uniqueJSON(encoded)
	if err != nil || !reflect.DeepEqual(projection, want) {
		return nil, "", fmt.Errorf("returned native probe facts differ")
	}
	return facts, session, nil
}

func (c *controller) trustedProbeIdentity(facts map[string]any, pid int64, driver, publisher isolationProof) (string, error) {
	f := c.trustedState
	probePID, err := trustedNumber(facts["pid"])
	session, ok := facts["session"].(string)
	if err != nil || probePID <= 0 || probePID == pid || probePID == int64(driver.PID) || probePID == int64(publisher.PID) || !ok || session == "" || len(session) > 256 || f.Sessions[session] {
		return "", fmt.Errorf("fresh native probe identity missing")
	}
	return session, nil
}
