package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This opt-in contract runs only in newly-created test projects and empty
// profiles. It initializes a local stdio MCP fixture without threads or turns.
func TestLocalCodexPluginNativeReadiness(t *testing.T) {
	executable := os.Getenv("AGENTPLUGINS_LOCAL_CODEX")
	if executable == "" {
		t.Skip("set AGENTPLUGINS_LOCAL_CODEX to the installed absolute Codex executable")
	}
	if runtime.GOOS == "windows" {
		t.Skip("this local fixture uses /usr/bin/python3")
	}
	env, request, _ := projectDiscoveryFixture(t)
	env.Runner = processadapter.OS{}
	request.BackendExecutable = executable
	t.Setenv("HOME", tempRoot(t))
	t.Setenv("TMPDIR", tempRoot(t))
	t.Setenv("CODEX_HOME", request.Client.ConfigRoot)
	if err := os.Remove(request.Client.ConfigRoot); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(tempRoot(t), "methods.jsonl")
	server := filepath.Join(request.Delivery.ActivePath, "server.py")
	body := `import sys,json,pathlib
log=pathlib.Path(sys.argv[1])
for line in sys.stdin:
 m=json.loads(line)
 with log.open('a') as f:f.write(m['method']+'\n')
 if 'id' not in m:continue
 if m['method']=='initialize':result={'protocolVersion':'2024-11-05','capabilities':{'tools':{}},'serverInfo':{'name':'local-uap-native','version':'1'}}
 elif m['method']=='tools/list':result={'tools':[]}
 else:result={}
 print(json.dumps({'jsonrpc':'2.0','id':m['id'],'result':result}),flush=True)
`
	if err := os.WriteFile(server, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	projected, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"local": map[string]any{"command": "/usr/bin/python3", "args": []string{server, log}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(request.Delivery.ActivePath, ".mcp.json"), projected, 0600); err != nil {
		t.Fatal(err)
	}
	selected, err := readCodexProjectServer(request.Delivery.ActivePath, "local")
	if err != nil {
		t.Fatal(err)
	}
	object, err := codexProjectLayout(request.Plan.NativeRegistryRoot).MCPObject("local", selected, codexProjectConfigPath(request.Plan.NativeRegistryRoot))
	if err != nil {
		t.Fatal(err)
	}
	request.Delivery.NativeObjects = append(request.Delivery.NativeObjects, object)
	request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportNative})
	outcome, err := New().Activate(t.Context(), env, request)
	if err != nil || outcome.Activation != domain.ActivationActive || outcome.Authentication != domain.AuthenticationNotRequired {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	methods, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(methods), "initialize\n") || !strings.Contains(string(methods), "tools/list\n") || strings.Contains(string(methods), "tools/call") {
		t.Fatalf("unexpected fixture requests: %s", methods)
	}
	trustPath := filepath.Join(request.Client.ConfigRoot, "config.toml")
	before, err := os.ReadFile(trustPath)
	if err != nil {
		t.Fatal(err)
	}
	request.VerifyOnly = true
	outcome, err = New().Activate(t.Context(), env, request)
	if err != nil || outcome.Activation != domain.ActivationActive {
		t.Fatalf("verify=%+v err=%v", outcome, err)
	}
	after, err := os.ReadFile(trustPath)
	if err != nil || string(after) != string(before) {
		t.Fatal("verification changed trust")
	}
}
