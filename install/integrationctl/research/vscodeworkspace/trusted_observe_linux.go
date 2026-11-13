//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
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
	outer, publisher, err := c.trustedDriverEnvelope(body, stage, trust, driver)
	if err != nil {
		return nil, err
	}
	observed, pid, err := c.trustedObserverIdentity(outer, stage, scope, request, driver, publisher)
	if err != nil {
		return nil, err
	}
	if err := trustedObservedSkills(observed, request); err != nil {
		return nil, err
	}
	if err := c.trustedObservedServers(outer, observed, request, scope, pid, driver, publisher, started, before); err != nil {
		return nil, err
	}
	return observed, nil
}

func (c *controller) trustedObserve(stage, scope, trust, operation string, installed bool) error {
	deadline, err := inputDeadline(c.request.Input, "work")
	if err != nil || time.Until(deadline) < 75*time.Second {
		return fmt.Errorf("insufficient original work budget for GUI")
	}
	if err := c.checkTrustedAssets(); err != nil {
		return err
	}
	prepared, err := c.trustedObservationRequest(stage, scope, trust, operation, installed)
	if err != nil {
		return err
	}
	f := c.trustedState
	ctx, cancel := context.WithTimeout(c.work, 65*time.Second)
	defer cancel()
	started := time.Now()
	var observed map[string]any
	_, runErr := c.strictInput(ctx, stage, []string{filepath.Join(c.request.Base, "tools", "node"), filepath.Join(c.request.Base, "tools", "driver", "driver.js")}, prepared.project, c.trustedEnvironment(), prepared.frame, func(exchange context.Context, proof isolationProof) error {
		var err error
		observed, err = c.waitTrustedObservation(exchange, prepared.driverResult, stage, scope, trust, prepared.request, proof, started, prepared.before)
		return err
	})
	if runErr != nil {
		runErr = c.trustedPartialObservation(stage, prepared.result, runErr)
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

type trustedDriverRequest struct {
	request                       map[string]any
	project, result, driverResult string
	frame, before                 []byte
}

func (c *controller) trustedObservationRequest(stage, scope, trust, operation string, installed bool) (trustedDriverRequest, error) {
	f := c.trustedState
	request, servers, err := c.trustedRequests(stage, scope, operation, installed)
	if err != nil {
		return trustedDriverRequest{}, err
	}
	requestPath := filepath.Join(c.request.Base, "requests", stage+".json")
	body, err := encode(request, controlLimit)
	if err != nil {
		return trustedDriverRequest{}, err
	}
	if err := writeSealed(requestPath, body); err != nil {
		return trustedDriverRequest{}, err
	}
	resultRoot := filepath.Join(f.Root, "results", stage)
	if err := c.trustedDirectory(resultRoot); err != nil {
		return trustedDriverRequest{}, err
	}
	ready, result, driverResult := filepath.Join(resultRoot, "ready.json"), filepath.Join(resultRoot, "observer.json"), filepath.Join(resultRoot, "driver.json")
	for _, path := range []string{ready, result, driverResult} {
		if err := requireFreshResultPaths(path); err != nil {
			return trustedDriverRequest{}, err
		}
	}
	project := f.Projects[scope]
	frame, err := c.trustedDriverFrame(stage, scope, trust, requestPath, ready, result, driverResult, servers)
	if err != nil {
		return trustedDriverRequest{}, err
	}
	before, err := trustedEvents(filepath.Join(f.Root, scope+"-events.jsonl"))
	if err != nil {
		return trustedDriverRequest{}, err
	}
	return trustedDriverRequest{request: request, project: project, result: result, driverResult: driverResult, frame: frame, before: before}, nil
}

func (c *controller) trustedDriverFrame(stage, scope, trust, requestPath, ready, result, driverResult string, servers []any) ([]byte, error) {
	f := c.trustedState
	project := f.Projects[scope]
	publisher := filepath.Join(c.request.Base, "publisher", publisherPrefix, "code")
	observer := filepath.Join(c.request.Base, "tools", "observer")
	args := []string{"--new-window", "--disable-crash-reporter", "--locale", "en", "--user-data-dir", f.Profile, "--extensions-dir", f.Extensions,
		"--extensionDevelopmentPath", observer, "--enable-proposed-api", "uap-research.project-runtime-observer", "--remote-debugging-port=0", project}
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
		return nil, err
	}
	frame = append(frame, '\n')
	return frame, nil
}

func (c *controller) waitTrustedObservation(exchange context.Context, driverResult, stage, scope, trust string, request map[string]any, proof isolationProof, started time.Time, before []byte) (map[string]any, error) {
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		body, ready, err := trustedReadyResult(driverResult)
		if err != nil {
			return nil, err
		}
		if ready {
			if err := c.keep(stage+".observation.json", body); err != nil {
				return nil, err
			}
			return c.validateTrustedObservation(body, stage, scope, trust, request, proof, started, before)
		}
		select {
		case <-exchange.Done():
			return nil, exchange.Err()
		case <-tick.C:
		}
	}
}

func (c *controller) trustedPartialObservation(stage, result string, runErr error) error {
	f := c.trustedState
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
	return runErr
}
