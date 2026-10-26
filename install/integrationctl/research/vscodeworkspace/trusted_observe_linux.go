//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func (c *controller) trustedRequests(stage, scope, operation string, installed bool) (map[string]any, []any, error) {
	f := c.trustedState
	project := f.Projects[scope]
	digest, err := hashFile(f.UserSkill, 1<<20)
	if err != nil {
		return nil, nil, err
	}
	skills := []any{map[string]any{"name": "uap-vscode-user", "path": f.UserSkill, "sha256": digest, "source": "user"}}
	servers, ui := []any{}, []any{}
	absentSkills, absentServers := []string{"uapvscodeuser"}, []string{}
	for _, other := range []string{"a", "b"} {
		if other != scope || !installed {
			absentSkills = append(absentSkills, f.Packages[other].Name)
			absentServers = append(absentServers, f.Packages[other].Name)
		}
	}
	if installed {
		if err := c.trustedReceipt(scope); err != nil {
			return nil, nil, err
		}
		p := f.Packages[scope]
		digest, err := hashFile(p.Skill, 1<<20)
		if err != nil {
			return nil, nil, err
		}
		skills = append(skills, map[string]any{"name": p.Name, "path": p.Skill, "sha256": digest, "source": "local"})
		config := filepath.Join(project, ".vscode", "mcp.json")
		body, err := readOrdinary(config, 1<<20)
		if err != nil {
			return nil, nil, err
		}
		value, err := uniqueJSON(body)
		object, ok := value.(map[string]any)
		if err != nil || !ok {
			return nil, nil, fmt.Errorf("native installed document malformed")
		}
		entries, ok := object["servers"].(map[string]any)
		expected := trustedExpectedServer(f, p, scope)
		if !ok || !reflect.DeepEqual(entries[p.Name], expected) {
			return nil, nil, fmt.Errorf("native projection differs from independent selected receipt/package/data")
		}
		facts := map[string]any{"nonce": f.Nonce, "revision": p.Revision, "cwd": p.Binding.TargetLocator,
			"argv": []string{"argument with spaces", p.Binding.TargetLocator + "/argument", "${UNKNOWN}"}, "plugin_root": p.Binding.TargetLocator, "plugin_data": p.Data,
			"literal": "literal ${UNKNOWN} $HOME", "once": p.Binding.TargetLocator + "/${UNKNOWN}", "marker": f.Marker}
		servers = append(servers, map[string]any{"label": p.Name, "command": expected["command"], "args": expected["args"], "env": expected["env"], "cwd": "", "facts": facts})
		ui = append(ui, map[string]any{"label": p.Name, "originUri": (&url.URL{Scheme: "file", Path: config}).String()})
	}
	return map[string]any{"mode": trustedProjectMode, "stage": stage, "nonce": f.Nonce, "project": project, "home": f.Home,
		"appRoot": filepath.Join(c.request.Base, "publisher", publisherPrefix, "resources", "app"), "skills": skills, "absentSkills": absentSkills,
		"servers": servers, "absentServers": absentServers, "operation": operation, "marker": f.Marker}, ui, nil
}

func trustedEvents(path string) ([]byte, error) {
	body, err := readOrdinary(path, 2<<20)
	if os.IsNotExist(err) {
		return []byte{}, nil
	}
	return body, err
}

// The trusted driver atomically renames this one closed result. It deliberately
// does not use or weaken the older observer's observation.ready protocol.
func trustedReadyResult(path string) ([]byte, bool, error) {
	body, err := readOrdinary(path, resultLimit)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return body, true, nil
}
func trustedNumber(value any) (int64, error) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, fmt.Errorf("exact integer required")
	}
	return number.Int64()
}

func (c *controller) validateTrustedObservation(body []byte, stage, scope, trust string, request map[string]any, driver isolationProof, started time.Time, before []byte) (map[string]any, error) {
	value, err := uniqueJSON(body)
	if err != nil {
		return nil, err
	}
	outer, err := objectKeys(value, strings.Fields("type stage nonce trustPromptAccepted servers publisherProof publisherExit result"), nil)
	f := c.trustedState
	if err != nil || outer["type"] != "driver-result" || outer["stage"] != stage || outer["nonce"] != f.Nonce || outer["trustPromptAccepted"] != (trust == "prompt") {
		return nil, fmt.Errorf("trusted driver envelope differs")
	}
	proofBytes, _ := json.Marshal(outer["publisherProof"])
	var publisher isolationProof
	if err := json.Unmarshal(proofBytes, &publisher); err != nil {
		return nil, err
	}
	if err := validateIsolationProof(publisher, c.request.Input.UID, c.request.Input.GID, c.parentNS, nil); err != nil {
		return nil, err
	}
	if publisher.Namespace != driver.Namespace || publisher.PID == driver.PID {
		return nil, fmt.Errorf("publisher and driver process/namespace relationship differs")
	}
	exit, err := objectKeys(outer["publisherExit"], []string{"code", "signal"}, nil)
	if err != nil {
		return nil, err
	}
	if !((exit["code"] == json.Number("0") && exit["signal"] == nil) || (exit["code"] == nil && exit["signal"] == "SIGTERM")) {
		return nil, fmt.Errorf("publisher shutdown differs")
	}
	observed, err := objectKeys(outer["result"], strings.Fields("schemaVersion mode stage nonce collected version appRoot appHost folder home trusted observerPID networkNamespace skills definitions calls"), nil)
	if err != nil {
		return nil, fmt.Errorf("native observer incomplete or unknown: %w", err)
	}
	if observed["schemaVersion"] != json.Number("1") || observed["mode"] != trustedProjectMode || observed["stage"] != stage || observed["nonce"] != f.Nonce || observed["collected"] != true || observed["version"] != publisherVersion || observed["appRoot"] != request["appRoot"] || observed["appHost"] != "desktop" || observed["folder"] != f.Projects[scope] || observed["home"] != f.Home || observed["trusted"] != true {
		return nil, fmt.Errorf("native observer identity differs")
	}
	pid, err := trustedNumber(observed["observerPID"])
	ns, nsErr := trustedNumber(observed["networkNamespace"])
	if err != nil || nsErr != nil || pid <= 0 || pid == int64(driver.PID) || pid == int64(publisher.PID) || ns <= 0 || uint64(ns) != driver.Namespace {
		return nil, fmt.Errorf("actual extension host identity differs")
	}
	// Normalize only our independently authored expectation to the same JSON
	// number/array representation; never replace it with observed native bytes.
	expectedBody, _ := json.Marshal(request["skills"])
	expectedSkills, err := uniqueJSON(expectedBody)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(observed["skills"], expectedSkills) {
		return nil, fmt.Errorf("native selected skills differ")
	}
	selected := request["servers"].([]any)
	definitions, ok := observed["definitions"].([]any)
	calls, callsOK := observed["calls"].([]any)
	ui, uiOK := outer["servers"].([]any)
	if !ok || !callsOK || !uiOK || len(definitions) != len(selected) || len(calls) != len(selected) || len(ui) != len(selected) {
		return nil, fmt.Errorf("native selected server cardinality differs")
	}
	for index, raw := range selected {
		expected := raw.(map[string]any)
		projection := map[string]any{}
		for _, key := range []string{"label", "command", "args", "env", "cwd"} {
			projection[key] = expected[key]
		}
		encoded, _ := json.Marshal(projection)
		want, err := uniqueJSON(encoded)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(definitions[index], want) {
			return nil, fmt.Errorf("native definition command/argv/env/cwd differs")
		}
		startedServer, err := objectKeys(ui[index], strings.Fields("label originUri trustPromptAccepted nativeState"), nil)
		origin := (&url.URL{Scheme: "file", Path: filepath.Join(f.Projects[scope], ".vscode", "mcp.json")}).String()
		if err != nil || startedServer["label"] != expected["label"] || startedServer["originUri"] != origin || startedServer["nativeState"] != "Running" {
			return nil, fmt.Errorf("native started server origin/state differs")
		}
		if _, ok := startedServer["trustPromptAccepted"].(bool); !ok {
			return nil, fmt.Errorf("native server trust decision missing")
		}
		call, err := objectKeys(calls[index], strings.Fields("label initialize tools facts operation gatewaySessionSha256 gatewayEndpointSha256"), nil)
		if err != nil || call["label"] != expected["label"] || call["operation"] != request["operation"] {
			return nil, fmt.Errorf("vendor gateway call identity differs")
		}
		for _, key := range []string{"gatewaySessionSha256", "gatewayEndpointSha256"} {
			text, ok := call[key].(string)
			if !ok || !shaPattern.MatchString(text) {
				return nil, fmt.Errorf("private gateway identity hash missing")
			}
		}
		facts, err := objectKeys(call["facts"], strings.Fields("nonce revision cwd argv plugin_root plugin_data literal once marker pid session"), nil)
		if err != nil {
			return nil, err
		}
		probePID, err := trustedNumber(facts["pid"])
		session, ok := facts["session"].(string)
		if err != nil || probePID <= 0 || probePID == pid || probePID == int64(driver.PID) || probePID == int64(publisher.PID) || !ok || session == "" || len(session) > 256 || f.Sessions[session] {
			return nil, fmt.Errorf("fresh native probe identity missing")
		}
		projection = map[string]any{}
		for key, item := range facts {
			if key != "pid" && key != "session" {
				projection[key] = item
			}
		}
		encoded, _ = json.Marshal(expected["facts"])
		want, err = uniqueJSON(encoded)
		if err != nil || !reflect.DeepEqual(projection, want) {
			return nil, fmt.Errorf("returned native probe facts differ")
		}
		after, err := trustedEvents(filepath.Join(f.Root, scope+"-events.jsonl"))
		if err != nil {
			return nil, err
		}
		if !bytes.HasPrefix(after, before) {
			return nil, fmt.Errorf("native event history rewritten")
		}
		if err := validateTrustedProbeEvents(after[len(before):], facts, started, time.Now()); err != nil {
			return nil, err
		}
		f.Sessions[session] = true
	}
	if len(selected) == 0 {
		after, err := trustedEvents(filepath.Join(f.Root, scope+"-events.jsonl"))
		if err != nil || !bytes.Equal(after, before) {
			return nil, fmt.Errorf("absent project started probe")
		}
	}
	return observed, nil
}

func validateTrustedProbeEvents(body []byte, facts map[string]any, started, ended time.Time) error {
	lines := bytes.Split(bytes.TrimSpace(body), []byte{'\n'})
	if len(lines) < 3 || len(lines) > 32 {
		return fmt.Errorf("fresh probe event inventory differs")
	}
	initialized, listed, called := false, false, false
	initCount, listCount, callCount := 0, 0, 0
	for _, line := range lines {
		value, err := uniqueJSON(line)
		if err != nil {
			return err
		}
		event, err := objectKeys(value, strings.Fields("method pid session nonce timestamp facts"), nil)
		if err != nil || event["pid"] != facts["pid"] || event["session"] != facts["session"] || event["nonce"] != facts["nonce"] {
			return fmt.Errorf("probe process/session/nonce does not join native result")
		}
		timestamp, ok := event["timestamp"].(string)
		when, err := time.Parse(time.RFC3339Nano, timestamp)
		if !ok || err != nil || when.Before(started) || when.After(ended) {
			return fmt.Errorf("probe event is outside actual native stage")
		}
		switch event["method"] {
		case "initialize":
			if initialized || listed || called || event["facts"] != nil {
				return fmt.Errorf("probe initialize ordering differs")
			}
			initialized = true
			initCount++
		case "tools/list":
			if !initialized || called || event["facts"] != nil {
				return fmt.Errorf("probe catalog ordering differs")
			}
			listed = true
			listCount++
		case "tools/call":
			if !initialized || !listed || called || !reflect.DeepEqual(event["facts"], facts) {
				return fmt.Errorf("probe call does not join exact returned facts")
			}
			called = true
			callCount++
		default:
			return fmt.Errorf("unknown probe event")
		}
	}
	if initCount != 1 || listCount < 1 || callCount != 1 {
		return fmt.Errorf("native probe initialize/list/call incomplete")
	}
	return nil
}

func (c *controller) trustedObserve(stage, scope, trust, operation string, installed bool) error {
	deadline, err := inputDeadline(c.request.Input, "work")
	if err != nil || time.Until(deadline) < 75*time.Second {
		return fmt.Errorf("insufficient original work budget for GUI")
	}
	if err := c.checkTrustedAssets(); err != nil {
		return err
	}
	f := c.trustedState
	request, servers, err := c.trustedRequests(stage, scope, operation, installed)
	if err != nil {
		return err
	}
	requestPath := filepath.Join(c.request.Base, "requests", stage+".json")
	body, err := encode(request, controlLimit)
	if err != nil {
		return err
	}
	if err := writeSealed(requestPath, body); err != nil {
		return err
	}
	resultRoot := filepath.Join(f.Root, "results", stage)
	if err := c.trustedDirectory(resultRoot); err != nil {
		return err
	}
	ready, result, driverResult := filepath.Join(resultRoot, "ready.json"), filepath.Join(resultRoot, "observer.json"), filepath.Join(resultRoot, "driver.json")
	for _, path := range []string{ready, result, driverResult} {
		if err := requireFreshResultPaths(path); err != nil {
			return err
		}
	}
	project := f.Projects[scope]
	publisher := filepath.Join(c.request.Base, "publisher", publisherPrefix, "code")
	observer := filepath.Join(c.request.Base, "tools", "observer")
	args := []string{"--new-window", "--disable-crash-reporter", "--locale", "en", "--user-data-dir", f.Profile, "--extensions-dir", f.Extensions,
		"--extensionDevelopmentPath", observer, "--extensionTestsPath", filepath.Join(observer, "runner.js"), "--enable-proposed-api", "uap-research.project-runtime-observer", "--remote-debugging-port=0", project}
	env := c.trustedEnvironment()
	env["UAP_VSCODE_WORKSPACE_FOLDER"] = project
	env["UAP_VSCODE_TRUSTED_REQUEST"] = requestPath
	env["UAP_VSCODE_TRUSTED_READY"] = ready
	env["UAP_VSCODE_TRUSTED_RESULT"] = result
	input := map[string]any{"mode": trustedProjectMode, "stage": stage, "project": project, "publisher": publisher, "publisherArgs": args, "publisherEnv": env,
		"originalNetworkNamespace": strconv.FormatUint(c.parentNS, 10), "uid": c.request.Input.UID, "gid": c.request.Input.GID, "expectedServers": servers,
		"triggerFile": ready, "resultFile": result, "driverResultFile": driverResult, "nonce": f.Nonce, "timeoutMs": 60000, "trustExpected": trust}
	frame, err := json.Marshal(input)
	if err != nil {
		return err
	}
	frame = append(frame, '\n')
	before, err := trustedEvents(filepath.Join(f.Root, scope+"-events.jsonl"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.work, 65*time.Second)
	defer cancel()
	started := time.Now()
	var observed map[string]any
	_, runErr := c.strictInput(ctx, stage, []string{filepath.Join(c.request.Base, "tools", "node"), filepath.Join(c.request.Base, "tools", "driver", "driver.js")}, project, c.trustedEnvironment(), frame, func(exchange context.Context, proof isolationProof) error {
		tick := time.NewTicker(25 * time.Millisecond)
		defer tick.Stop()
		for {
			body, ready, err := trustedReadyResult(driverResult)
			if err != nil {
				return err
			}
			if ready {
				if err := c.keep(stage+".observation.json", body); err != nil {
					return err
				}
				observed, err = c.validateTrustedObservation(body, stage, scope, trust, request, proof, started, before)
				return err
			}
			select {
			case <-exchange.Done():
				return exchange.Err()
			case <-tick.C:
			}
		}
	})
	if runErr != nil {
		// An observer can return useful bounded native facts before the driver
		// refuses a stage. Preserve them without treating a failed stage as proof.
		if _, exists := c.files[stage+".observation.json"]; !exists {
			if partial, ready, readErr := trustedReadyResult(result); readErr == nil && ready {
				if value, decodeErr := uniqueJSON(partial); decodeErr == nil {
					body, encodeErr := encode(map[string]any{"type": "observer-partial", "stage": stage, "nonce": f.Nonce, "result": value}, resultLimit)
					if encodeErr == nil {
						runErr = errors.Join(runErr, c.keep(stage+".observation.json", body))
					}
				}
			}
		}
	}
	if err := c.checkTrustedAssets(); err != nil {
		runErr = errors.Join(runErr, err)
	}
	if runErr == nil {
		f.Stages = append(f.Stages, stage)
	}
	c.observations = append(c.observations, map[string]any{"stage": stage, "project": scope, "complete": runErr == nil, "observation": observed, "error": clipped(runErr)})
	return runErr
}
