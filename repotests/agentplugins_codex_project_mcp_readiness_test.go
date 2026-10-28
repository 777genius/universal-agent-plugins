package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

type codexProjectMCPInventory struct {
	Data []struct {
		Name, PluginID, RuntimeStatus string
		Tools                         map[string]json.RawMessage
	} `json:"data"`
	NextCursor *string `json:"nextCursor"`
}

func codexProjectMCPReady(ctx context.Context, request func(context.Context) (json.RawMessage, error), present bool) (codexProjectMCPInventory, error) {
	var inventory codexProjectMCPInventory
	for {
		body, err := request(ctx)
		if err != nil {
			return inventory, err
		}
		inventory = codexProjectMCPInventory{}
		if err := json.Unmarshal(body, &inventory); err != nil || inventory.NextCursor != nil {
			return inventory, fmt.Errorf("incomplete native MCP inventory: %s", body)
		}
		if !present {
			return inventory, nil
		}
		ready, err := codexProjectMCPConnected(inventory)
		if err != nil || ready {
			return inventory, err
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return inventory, fmt.Errorf("native project MCP startup did not connect: %w; inventory: %+v", ctx.Err(), inventory)
		case <-timer.C:
		}
	}
}

func codexProjectMCPConnected(inventory codexProjectMCPInventory) (bool, error) {
	if len(inventory.Data) != 2 {
		return false, fmt.Errorf("native project MCP servers missing: %+v", inventory)
	}
	seen, ready := map[string]bool{}, true
	for _, server := range inventory.Data {
		if server.PluginID != "" || (server.Name != "default" && server.Name != "explicit") || seen[server.Name] {
			return false, fmt.Errorf("unexpected project MCP server: %+v", server)
		}
		seen[server.Name] = true
		switch server.RuntimeStatus {
		case "connected":
		case "starting":
			ready = false
		default:
			return false, fmt.Errorf("project MCP server did not connect: %+v", server)
		}
	}
	return ready, nil
}

func codexProjectMCPTestInventory(status string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"data":[{"name":"default","runtimeStatus":"connected","tools":{"inspect_runtime":{}}},{"name":"explicit","runtimeStatus":%q,"tools":{"inspect_runtime":{}}}],"nextCursor":null}`, status))
}

func TestCodexProjectMCPReadinessAwaitsConnected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	inventory, err := codexProjectMCPReady(ctx, func(context.Context) (json.RawMessage, error) {
		calls++
		if calls == 1 {
			return codexProjectMCPTestInventory("starting"), nil
		}
		return codexProjectMCPTestInventory("connected"), nil
	}, true)
	if err != nil || calls != 2 || inventory.Data[1].RuntimeStatus != "connected" {
		t.Fatalf("starting with a tool must await connected: calls=%d inventory=%+v err=%v", calls, inventory, err)
	}
}

func TestCodexProjectMCPReadinessRejectsInvalidInventory(t *testing.T) {
	for name, body := range map[string]json.RawMessage{
		"failed":     codexProjectMCPTestInventory("failed"),
		"unknown":    codexProjectMCPTestInventory("unknown"),
		"malformed":  json.RawMessage(`{"data":`),
		"missing":    json.RawMessage(`{"data":[]}`),
		"foreign":    json.RawMessage(`{"data":[{"name":"default","runtimeStatus":"connected"},{"name":"foreign","runtimeStatus":"starting"}]}`),
		"duplicate":  json.RawMessage(`{"data":[{"name":"default","runtimeStatus":"connected"},{"name":"default","runtimeStatus":"starting"}]}`),
		"plugin":     json.RawMessage(`{"data":[{"name":"default","runtimeStatus":"connected"},{"name":"explicit","pluginId":"other","runtimeStatus":"starting"}]}`),
		"pagination": json.RawMessage(`{"data":[],"nextCursor":"more"}`),
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			_, err := codexProjectMCPReady(context.Background(), func(context.Context) (json.RawMessage, error) {
				calls++
				return body, nil
			}, true)
			if err == nil || calls != 1 {
				t.Fatalf("invalid inventory must fail on its first read: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestCodexProjectMCPReadinessKeepsOriginalDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	calls := 0
	inventory, err := codexProjectMCPReady(ctx, func(observed context.Context) (json.RawMessage, error) {
		if observed != ctx {
			t.Fatal("readiness replaced the original context")
		}
		calls++
		return codexProjectMCPTestInventory("starting"), nil
	}, true)
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 || inventory.Data[1].RuntimeStatus != "starting" {
		t.Fatalf("starting must not qualify at the deadline: calls=%d inventory=%+v err=%v", calls, inventory, err)
	}
}
