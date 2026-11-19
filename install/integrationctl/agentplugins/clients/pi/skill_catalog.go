package pi

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const piSkillObservationTimeout = 30 * time.Second
const piSkillCatalogLimit = 1 << 20

func piSkillCLI(request domain.ActivationRequest) bool {
	executable := request.Client.ExecutablePath
	if request.Client.Status != domain.DetectionDetected || !filepath.IsAbs(executable) ||
		filepath.Clean(executable) != executable || strings.ContainsAny(executable, "\x00\r\n") ||
		(request.BackendExecutable != "" && request.BackendExecutable != executable) {
		return false
	}
	// Pi has no IDE observer here. An explicitly supplied SDK client may have
	// no surfaces; known non-CLI surfaces do not authorize a native process.
	if len(request.Client.Surfaces) == 0 {
		return true
	}
	for _, surface := range request.Client.Surfaces {
		if surface.ID == "pi_cli" && surface.Detected {
			return true
		}
	}
	return false
}

func piSkillCommand(request domain.ActivationRequest) legacyports.Command {
	environment := []string{"PI_CODING_AGENT_DIR=" + request.Client.ConfigRoot, "PI_OFFLINE=1"}
	for _, key := range []string{"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL",
		"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME"} {
		if value, ok := os.LookupEnv(key); ok {
			environment = append(environment, key+"="+value)
		}
	}
	cwd := request.Client.ConfigRoot
	if request.Plan.Scope == domain.ScopeProject {
		cwd = request.Plan.TargetContext.ProjectRoot
	}
	return legacyports.Command{Argv: []string{request.Client.ExecutablePath, "--mode", "rpc", "--no-session", "--offline"},
		Dir: cwd, Env: environment, StdoutLimitBytes: piSkillCatalogLimit}
}

func runPiSkillCatalog(ctx context.Context, runner ports.CompletionCommandRunner, request domain.ActivationRequest) ([]any, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	id := "uap-skills-" + hex.EncodeToString(nonce[:])
	var observed []any
	err := runner.RunDuplexToCompletion(ctx, piSkillCommand(request), func(stdin io.Writer, stdout io.Reader) error {
		if err := json.NewEncoder(stdin).Encode(map[string]string{"id": id, "type": "get_commands"}); err != nil {
			return err
		}
		// Pi1.4/1.5 emits one response for this one command. Returning only after
		// that response lets the existing runner send EOF, then require natural
		// native shutdown. No prompt, approval flag or signal is used.
		body, err := bufio.NewReader(io.LimitReader(stdout, piSkillCatalogLimit+1)).ReadBytes('\n')
		if err != nil {
			return err
		}
		if len(body) > piSkillCatalogLimit {
			return errors.New("pi Skill catalog exceeds byte limit")
		}
		observed, err = piSkillCatalogResponse(body, id)
		return err
	})
	return observed, errors.Join(err, ctx.Err())
}

func piSkillCatalogResponse(body []byte, id string) ([]any, error) {
	envelope, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return nil, err
	}
	if envelope["type"] != "response" || envelope["command"] != "get_commands" || envelope["id"] != id ||
		envelope["success"] != true {
		return nil, errors.New("pi did not return the selected get_commands response")
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		return nil, errors.New("pi Skill response has no data object")
	}
	commands, ok := data["commands"].([]any)
	if !ok || len(commands) > 512 {
		return nil, errors.New("pi Skill response has no bounded command inventory")
	}
	return commands, nil
}

func piSkillCatalogMatches(rows []any, expected map[string]piObservedSkill, scope domain.InstallScope, base string) bool {
	seen := make(map[string]bool, len(expected))
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return false
		}
		name, ok := row["name"].(string)
		if !ok || name == "" {
			return false
		}
		for logical, skill := range expected {
			if shared.FoldJSONKey(name) != shared.FoldJSONKey("skill:"+logical) {
				continue
			}
			if seen[logical] || name != "skill:"+logical || !piSkillCatalogEntry(row, skill, scope, base) {
				return false
			}
			seen[logical] = true
		}
	}
	return len(expected) > 0 && len(seen) == len(expected)
}

func piSkillCatalogEntry(row map[string]any, skill piObservedSkill, scope domain.InstallScope, base string) bool {
	source, ok := row["sourceInfo"].(map[string]any)
	if !ok || row["source"] != "skill" || row["description"] != skill.description {
		return false
	}
	for key, want := range map[string]string{"path": skill.path, "source": "auto", "scope": string(scope), "origin": "top-level", "baseDir": base} {
		if source[key] != want {
			return false
		}
	}
	// A settings.packages resource is a different authority; never normalize it
	// into the direct auto-discovered User/Project object.
	if _, ok := source["changelogPath"]; ok {
		return false
	}
	return true
}
