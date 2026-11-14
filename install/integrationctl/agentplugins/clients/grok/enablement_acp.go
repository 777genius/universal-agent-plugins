package grok

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const grokACPFrameLimit = 64 << 10

type grokACPInventory struct {
	initialized, notified bool
	plugins               []grokACPPlugin
	complete              bool
}

// This is the measured sessionless route. It never creates or resumes a
// session, submits a prompt, authorizes trust, or invokes a plugin action.
func readEnablementInventory(ctx context.Context, runner ports.CommandRunner, inputs *enablementInventoryInputs, expected nativeconfig.FileSnapshot) (plugins []grokACPPlugin, resultErr error) {
	duplex, ok := runner.(ports.ReadonlyDuplexCommandRunner)
	if !ok || inputs == nil {
		return nil, fmt.Errorf("grok enablement requires contained readonly duplex execution")
	}
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := inputs.revalidate(bounded, expected); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, inputs.revalidate(bounded, expected)) }()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	project := inputs.inputs.Project.Path
	command := legacyports.Command{Argv: []string{inputs.inputs.Executable.Path, "agent", "--no-leader", "stdio"}, Dir: project,
		Env: grokMetadataEnvironment(inputs.inputs.Home.Path)}
	var tail *grokACPTail
	var pending []byte
	var inventory grokACPInventory
	defer func() { resultErr = errors.Join(resultErr, tail.close()) }()
	err := duplex.RunReadonlyDuplexWithPlannedShutdown(bounded, command, inputs.inputs, func(stdin io.Writer, stdout io.Reader) error {
		var err error
		tail, err = captureGrokACPTail(stdout)
		if err != nil {
			return err
		}
		reader := bufio.NewReaderSize(stdout, grokACPFrameLimit+1)
		inventory, err = exchangeEnablementInventory(stdin, reader, "uap-enablement-"+hex.EncodeToString(nonce[:]), project)
		if err == nil {
			pending, err = reader.Peek(reader.Buffered())
			pending = bytes.Clone(pending)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if !inventory.complete {
		return nil, fmt.Errorf("grok ACP inventory response is incomplete")
	}
	if err := tail.drain(bounded, pending); err != nil {
		return nil, err
	}
	return inventory.plugins, nil
}

func grokMetadataEnvironment(home string) []string {
	// Explicit HOME is constructor authority. No ambient GROK_HOME, credentials,
	// provider overrides or PATH values are copied into this metadata process.
	return []string{"HOME=" + home, "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "TERM=dumb", "NO_COLOR=1", "GROK_DISABLE_AUTOUPDATER=1",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0",
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"), "XDG_DATA_HOME=" + filepath.Join(home, ".local", "share"),
		"XDG_CACHE_HOME=" + filepath.Join(home, ".cache"), "XDG_STATE_HOME=" + filepath.Join(home, ".local", "state")}
}

func exchangeEnablementInventory(stdin io.Writer, reader *bufio.Reader, sessionID, project string) (grokACPInventory, error) {
	state := grokACPInventory{}
	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{
		"protocolVersion": 1, "clientCapabilities": map[string]any{}, "clientInfo": map[string]any{"name": "uap-inventory-admission", "version": "1"}}}); err != nil {
		return state, err
	}
	for range 4 {
		line, err := reader.ReadSlice('\n')
		if err != nil || len(line) > grokACPFrameLimit {
			return state, errors.Join(err, fmt.Errorf("grok ACP response is incomplete or exceeds its bound"))
		}
		value, err := shared.DecodeStrictJSONObject(line)
		if err != nil {
			return state, err
		}
		if err := state.accept(value, project); err != nil {
			return state, err
		}
		if state.complete {
			return state, nil
		}
		if value["id"] == json.Number("1") {
			if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "_x.ai/plugins/list", "params": map[string]any{"sessionId": sessionID}}); err != nil {
				return state, err
			}
		}
	}
	return state, fmt.Errorf("grok ACP inventory exceeded its finite response sequence")
}

func (state *grokACPInventory) accept(value map[string]any, project string) error {
	if value["jsonrpc"] != "2.0" || state.complete {
		return fmt.Errorf("grok ACP response envelope differs")
	}
	if _, present := value["method"]; present {
		return state.acceptNotification(value)
	}
	if len(value) != 3 {
		return fmt.Errorf("grok ACP response contains unknown envelope members")
	}
	result, ok := value["result"].(map[string]any)
	if !ok {
		return fmt.Errorf("grok ACP response lacks a result")
	}
	if !state.initialized {
		return state.acceptInitialization(value["id"], result, project)
	}
	if value["id"] != json.Number("2") || len(result) != 1 {
		return fmt.Errorf("grok ACP listing response identity differs")
	}
	listing, ok := result["result"].(map[string]any)
	if !ok || len(listing) != 1 {
		return fmt.Errorf("grok ACP listing wrapper differs")
	}
	plugins, err := parseEnablementPlugins(listing["plugins"])
	if err != nil {
		return err
	}
	state.plugins, state.complete = plugins, true
	return nil
}

func (state *grokACPInventory) acceptNotification(value map[string]any) error {
	params, ok := value["params"].(map[string]any)
	servers, list := params["mcpServers"].([]any)
	if len(value) != 3 || value["method"] != "_x.ai/mcp/servers_updated" || !ok || len(params) != 1 || !list || len(servers) != 0 || state.notified {
		return fmt.Errorf("grok ACP notification is not the measured empty metadata event")
	}
	state.notified = true
	return nil
}

func (state *grokACPInventory) acceptInitialization(id any, result map[string]any, project string) error {
	meta, ok := result["_meta"].(map[string]any)
	if id != json.Number("1") || result["protocolVersion"] != json.Number("1") || !ok ||
		(meta["agentVersion"] != "1.0.70" && meta["agentVersion"] != "1.0.70 (f50904eaee6c)") || meta["currentWorkingDirectory"] != project {
		return fmt.Errorf("grok ACP initialization version or project differs")
	}
	state.initialized = true
	return nil
}
