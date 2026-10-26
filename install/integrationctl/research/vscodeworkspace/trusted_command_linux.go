//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
)

func (c *controller) trustedEnvironment() map[string]string {
	f := c.trustedState
	env := map[string]string{"HOME": f.Home, "USERPROFILE": f.Home, "PATH": "/usr/bin:/bin", "LANG": "C.UTF-8",
		"TMPDIR": filepath.Join(f.Root, "tmp"), "DISPLAY": display, "XAUTHORITY": filepath.Join(c.request.Base, "auth", "authority"), "NO_COLOR": "1", "TERM": "dumb"}
	for key, dir := range map[string]string{"XDG_CONFIG_HOME": "config", "XDG_DATA_HOME": "data", "XDG_CACHE_HOME": "cache", "XDG_STATE_HOME": "state"} {
		env[key] = filepath.Join(f.Home, dir)
	}
	return env
}

// Finite public fixture operations use the existing ordinary whole-tree
// supervisor. Its natural leader exit differs from the long-running UI driver's
// explicitly planned shutdown and must not be reclassified as that operation.
func (c *controller) trustedCommand(label, cwd string, argv []string, expectedExit int) ([]byte, error) {
	if err := c.checkTrustedAssets(); err != nil {
		return nil, err
	}
	env := c.trustedEnvironment()
	env["PATH"] = filepath.Join(c.request.Base, "executables") + ":/usr/bin:/bin"
	env["AGENTPLUGINS_HOME"] = filepath.Join(c.trustedState.Root, "installer-state")
	env["GIT_CONFIG_NOSYSTEM"], env["GIT_CONFIG_GLOBAL"], env["GIT_TERMINAL_PROMPT"] = "1", "/dev/null", "0"
	command, err := c.isolationCommand(argv, cwd, env)
	if err != nil {
		return nil, err
	}
	command.StdoutLimitBytes = streamLimit
	ctx, cancel := context.WithTimeout(c.work, 30*time.Second)
	defer cancel()
	r, runErr := (process.OS{}).RunWithDescendantExitGrace(ctx, command, 5*time.Second)
	if ctx.Err() != nil || len(r.Stdout) >= streamLimit || len(r.Stderr) != 0 || r.ExitCode != expectedExit {
		runErr = errors.Join(runErr, fmt.Errorf("public operation exit/capture/budget differs"), ctx.Err())
	}
	newline := bytes.IndexByte(r.Stdout, '\n')
	var proof isolationProof
	var output []byte
	if newline < 0 || newline > 4096 || !bytes.HasPrefix(r.Stdout, []byte(proofPrefix)) {
		runErr = errors.Join(runErr, fmt.Errorf("public child proof missing"))
	} else {
		body := r.Stdout[len(proofPrefix):newline]
		_, proofErr := uniqueJSON(body)
		if proofErr == nil {
			proofErr = json.Unmarshal(body, &proof)
		}
		proofErr = validateIsolationProof(proof, c.request.Input.UID, c.request.Input.GID, c.parentNS, proofErr)
		if !proof.LoopbackReady {
			proofErr = errors.Join(proofErr, fmt.Errorf("public child loopback admission missing"))
		}
		runErr = errors.Join(runErr, proofErr)
		output = r.Stdout[newline+1:]
	}
	merged := append(append([]byte(nil), r.Stdout...), r.Stderr...)
	if c.cookie != "" {
		merged = bytes.ReplaceAll(merged, []byte(c.cookie), []byte("[task-cookie-redacted]"))
	}
	digest := sha256.Sum256(merged)
	record := childRecord{Label: label, Argv: argv, Cwd: cwd, Proof: proof, Diagnostics: fmt.Sprintf("ordinary public child; exact exit %d; verifier-prefixed merged stdout/stderr", r.ExitCode),
		MergedFile: label + ".merged.log", MergedBytes: len(merged), MergedSHA: hex.EncodeToString(digest[:]), TreeReaped: runErr == nil, Error: clipped(runErr)}
	if e := c.keep(record.MergedFile, merged); e != nil {
		runErr = errors.Join(runErr, e)
		record.Error = clipped(runErr)
	}
	c.children = append(c.children, record)
	if err := c.checkTrustedAssets(); err != nil {
		runErr = errors.Join(runErr, err)
	}
	return output, runErr
}

func trustedPublicOutcome(body []byte, mutated bool) error {
	value, err := uniqueJSON(body)
	object, ok := value.(map[string]any)
	if err != nil || !ok || object["schema_version"] != json.Number("1") || object["result"] != "success" {
		return fmt.Errorf("public CLI success envelope differs")
	}
	data, ok := object["data"].(map[string]any)
	if !ok {
		return fmt.Errorf("public CLI data missing")
	}
	results, ok := data["targets"].([]any)
	if !ok || len(results) != 1 {
		return fmt.Errorf("single public target outcome missing")
	}
	result, ok := results[0].(map[string]any)
	if !ok {
		return fmt.Errorf("public target outcome malformed")
	}
	output, ok := result["output"].(map[string]any)
	if !ok {
		return fmt.Errorf("public target output missing")
	}
	outcome, ok := output["result"].(map[string]any)
	if !ok || outcome["mutated"] != mutated {
		return fmt.Errorf("public target mutation result differs")
	}
	return nil
}

func (c *controller) trustedUAP(label, scope string, mutated, absent bool, args ...string) error {
	f := c.trustedState
	p := f.Packages[scope]
	cwd := f.Projects[scope]
	argv := append([]string{filepath.Join(c.request.Base, "executables", "agentplugins")}, args...)
	argv = append(argv, "--target", "vscode", "--format", "json")
	if scope == "user" {
		argv = append(argv, "--scope", "user")
	} else {
		argv = append(argv, "--scope", "project", "--project-root", cwd)
	}
	exit := 0
	if absent {
		exit = 1
	}
	body, err := c.trustedCommand(label, cwd, argv, exit)
	if err != nil {
		return err
	}
	if absent {
		first := "agentplugins: installation \"" + p.Name + "\" was not found\n"
		second := "agentplugins: plugin is not installed for target \"vscode\" in project scope; no target was changed\n"
		if string(body) != first && string(body) != second {
			return fmt.Errorf("repeat removal is not exact absent refusal")
		}
		return nil
	}
	progress := ""
	switch args[0] {
	case "add":
		progress = "Resolving and validating one Agent Plugin package for every selected target...\n"
		if strings.HasPrefix(label, "repeat-") {
			progress += "Applying transactional client package...\n"
		} else {
			progress += "Installing " + p.Name + " for 1 selected clients...\n"
		}
	case "update":
		progress = "Resolving and validating one updated Agent Plugin package for every selected target...\n"
	case "repair":
		progress = "Resolving and validating each unique exact installed package revision once...\n"
	}
	if !bytes.HasPrefix(body, []byte(progress)) {
		return fmt.Errorf("public progress diagnostics differ")
	}
	return trustedPublicOutcome(body[len(progress):], mutated)
}
