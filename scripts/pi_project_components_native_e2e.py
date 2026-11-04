#!/usr/bin/env python3
"""Genuine Pi project Skills/MCP E2E; reuses the pinned official Pi test host."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from pi_project_native_e2e import PINS, checked_release, require_hosted, run, write
from pi_local_model_fixture import ScriptedPiModel


class PiHTTPMCPProbe:
    """Single bounded, stateless HTTP MCP fixture for both native Pi scopes."""
    def __init__(self):
        self.lock = threading.Lock()
        self.observed = []
        outer = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_args):
                pass

            def do_POST(self):
                if self.path != "/mcp":
                    self.send_error(404)
                    return
                try:
                    length = int(self.headers.get("Content-Length", "0"))
                    if length <= 0 or length > 1 << 16:
                        raise ValueError("HTTP MCP body size")
                    request = json.loads(self.rfile.read(length))
                    method = request.get("method")
                    if request.get("jsonrpc") != "2.0" or not isinstance(method, str):
                        raise ValueError("not JSON-RPC")
                except (ValueError, TypeError, json.JSONDecodeError):
                    self.send_error(400)
                    return
                with outer.lock:
                    outer.observed.append(method)
                    if len(outer.observed) > 128:
                        self.send_error(429)
                        return
                if "id" not in request:
                    self.send_response(202)
                    self.end_headers()
                    return
                if method == "initialize":
                    result = {"protocolVersion": "2025-11-25", "capabilities": {"tools": {}},
                              "serverInfo": {"name": "pi-http-probe", "version": "1.0.0"}}
                elif method == "tools/list":
                    result = {"tools": [{"name": "ping", "description": "Inert HTTP proof",
                                         "inputSchema": {"type": "object", "properties": {}}}]}
                elif method == "tools/call" and request.get("params", {}).get("name") == "ping":
                    result = {"content": [{"type": "text", "text": "pi-http-e2e"}]}
                else:
                    self.send_error(400)
                    return
                data = json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": result}).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def do_GET(self):
                self.send_response(405)
                self.send_header("Allow", "POST")
                self.end_headers()

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    @property
    def url(self):
        return f"http://127.0.0.1:{self.server.server_port}/mcp"

    def methods(self):
        with self.lock:
            return tuple(self.observed)

    def __enter__(self):
        return self

    def __exit__(self, *_exc):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=3)


def checked_rpc(pi, project, env, approved, stage):
    command = [str(pi), "--mode", "rpc", "--no-session", "--offline",
               "--approve" if approved else "--no-approve"]
    output = run(command, project, env=env,
                 input_bytes=(json.dumps({"id":stage, "type":"get_commands"})+"\n").encode(),
                 timeout=55)
    responses = [json.loads(line) for line in output.splitlines() if line.strip()]
    matches = [value for value in responses if value.get("type") == "response" and
               value.get("command") == "get_commands" and value.get("id") == stage and
               value.get("success") is True]
    if len(matches) != 1:
        raise RuntimeError("Pi get_commands did not return one successful bounded response")
    commands = matches[0].get("data", {}).get("commands")
    if not isinstance(commands, list) or len(commands) > 200:
        raise RuntimeError("Pi skill command inventory malformed or unbounded")
    return commands

def main():
    if len(sys.argv) != 3 or sys.argv[1] not in PINS:
        raise SystemExit("usage: pi_project_components_native_e2e.py TARGET OUTPUT")
    target, destination = sys.argv[1], Path(sys.argv[2]).resolve()
    require_hosted(target)
    source = Path(__file__).resolve().parents[1]
    commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=source, text=True).strip()
    if not re.fullmatch(r"[0-9a-f]{40}", commit) or commit != os.environ.get("EXPECTED_COMMIT"):
        raise RuntimeError("exact UAP checkout required")
    if subprocess.check_output(["git", "status", "--porcelain"], cwd=source):
        raise RuntimeError("UAP checkout must be clean")
    destination.mkdir(parents=True, exist_ok=False)
    state = {"status":"failed", "sha":commit, "target":target, "stages":[],
             "real_model_qualified":False, "mcp_tools_call_qualified":False}
    try:
        with PiHTTPMCPProbe() as http_probe, tempfile.TemporaryDirectory(prefix="uap-pi-components-", dir=os.environ["RUNNER_TEMP"]) as tmp:
            root = Path(tmp).resolve()
            acquire_env = {key:value for key,value in os.environ.items() if key in
                           ("PATH","GH_TOKEN","SystemRoot","COMSPEC")}
            pi, release = checked_release(root, target, acquire_env)
            state["release"] = release
            driver = root / ("driver.exe" if target.startswith("windows") else "driver")
            run(["go","build","-trimpath","-o",str(driver),"./clients/pi/testdata/project-driver"],
                source / "install/integrationctl/agentplugins",timeout=240)
            user, project_a, project_b = root/"home/.pi/agent",root/"A",root/"B"
            package = root/"source"
            for p in (user,project_a,project_b,package): p.mkdir(parents=True)
            fixture = 'export default function() {}\n'
            # Foreign entries are visible to UAP but never claimed.
            foreign = '{"mcpServers":{"foreign":{"url":"https://foreign.invalid/mcp","enabled":false}},"owner":"user"}\n'
            for project in (project_a,project_b):
                write(project,".pi/mcp.json",foreign)
            write(user,"mcp.json",foreign)
            write(package,"plugin.json",json.dumps({
                "$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                "name":"pi-component-proof","version":"1.0.0"},separators=(",",":")))
            def skill(revision):
                write(package,"skills/pi-managed/SKILL.md",
                      "---\nname: pi-managed\ndescription: Qualified project Pi Skill E2E\n---\n"+revision+"\n")
            skill("first")
            # Node runs as a true Pi-spawned stdio MCP server with only
            # package and data authority. It logs protocol methods, not data.
            server = '''
import readline from "node:readline";
import { appendFileSync } from "node:fs";
import { join } from "node:path";
for await (const line of readline.createInterface({input:process.stdin,crlfDelay:Infinity})) {
  let req; try { req=JSON.parse(line); } catch { continue; }
  if (!req || typeof req.method !== "string") continue;
  if (req.method !== "notifications/initialized" && process.env.PLUGIN_DATA)
    appendFileSync(join(process.env.PLUGIN_DATA,"pi-mcp-methods.log"),req.method+"\\n");
  if (req.id === undefined) continue;
  let result;
  if (req.method === "initialize")
    result={protocolVersion:"2025-11-25",capabilities:{tools:{}},serverInfo:{name:"pi-uap-probe",version:"1.0.0"}};
  else if (req.method === "tools/list")
    result={tools:[{name:"echo",description:"Read-only inert E2E probe",inputSchema:{type:"object",properties:{}}}]};
  else if (req.method === "tools/call")
    result={content:[{type:"text",text:"pi-e2e"}]};
  else result={};
  process.stdout.write(JSON.stringify({jsonrpc:"2.0",id:req.id,result})+"\\n");
}
'''
            write(package,"probe.mjs",server)
            mcp_body={"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
                "mcpServers":{"pi-managed":{
                    "type":"stdio","command":"node",
                    "args":["${PLUGIN_ROOT}/probe.mjs"],"cwd":"${PLUGIN_ROOT}","env":{}},
                    "pi-http":{"type":"streamable-http","url":http_probe.url}}}
            write(package,"mcp.json",json.dumps(mcp_body,separators=(",",":")))
            def op(project, verb, scope="project", profile=None, state_root=None, name=None):
                profile = profile or user
                state_root = state_root or project/".uap"
                name = name or "TEST-pi-components"
                receipt=root/(project.name+"-"+scope+"-"+verb+"-"+name+".json")
                args=[str(driver),str(state_root),str(profile),str(project),str(package),
                      verb,name,str(receipt),scope]
                run(args,project,timeout=95)
                outcome=json.loads(receipt.read_text())
                if outcome.get("outcome") not in ("completed","unchanged") or outcome.get("binding",{}).get("scope")!=scope:
                    raise RuntimeError("UAP Pi scoped transaction did not publish exact receipt")
                return outcome
            for project in (project_a,project_b): op(project,"install")
            state["stages"].append("UAP_project_A_B_installed")
            def pi_env(profile=user):
                env={"HOME":str(root/"home"),"USERPROFILE":str(root/"home"),
                     "PI_CODING_AGENT_DIR":str(profile),"PI_OFFLINE":"1",
                     "PATH":os.environ.get("PATH","")}
                if target.startswith("windows"):
                    for key in ("SystemRoot","COMSPEC","WINDIR","PATHEXT"):
                        if os.environ.get(key): env[key]=os.environ[key]
                    env["APPDATA"]=str(root/"home/AppData/Roaming")
                    env["LOCALAPPDATA"]=str(root/"home/AppData/Local")
                    Path(env["APPDATA"]).mkdir(parents=True,exist_ok=True)
                    Path(env["LOCALAPPDATA"]).mkdir(parents=True,exist_ok=True)
                return env
            env=pi_env()
            untrusted=checked_rpc(pi,project_a,env,False,"untrusted")
            if any(x.get("name")=="skill:pi-managed" for x in untrusted):
                raise RuntimeError("untrusted Pi loaded project Skill")
            trusted=checked_rpc(pi,project_a,env,True,"trusted")
            if sum(1 for x in trusted if x.get("source")=="skill" and x.get("name")=="skill:pi-managed")!=1:
                raise RuntimeError("trusted Pi did not discover managed project Skill")
            state["stages"].append("real_Pi_trust_gated_skill_discovery")
            # The "pi mcp" shell subcommand bypasses session-only --approve
            # and reads its own trust store. Explicitly authorize ONLY
            # disposable project A in the disposable test profile; no UAP
            # installer action, real user profile or project B is trusted.
            write(user,"trust.json",json.dumps({
                str(project_a.resolve()):True, str(project_b.resolve()):False
            },sort_keys=True)+chr(10))
            # The official v1.4.0 shell grammar is "pi mcp list --json";
            # placing --approve before mcp routes to the generic CLI parser
            # and rejects --json before any native MCP exchange.
            args=[str(pi),"mcp","list","--json"]
            before_project_http=len(http_probe.methods())
            output=run(args,project_a,env=env,timeout=55)
            if len(output)>1<<20:
                raise RuntimeError("native project MCP inventory exceeds limit")
            catalog=json.loads(output)
            entries=catalog.get("servers",[])
            for name, tool in (("pi-managed","echo"),("pi-http","ping")):
                matching=[entry for entry in entries if entry.get("name")==name]
                if len(matching)!=1 or matching[0].get("scope")!="project" or (
                    matching[0].get("state")!="connected") or tool not in matching[0].get("tools",[]):
                    raise RuntimeError("real Pi did not connect and enumerate exact project MCP "+name)
            # A connected inventory alone cannot qualify HTTP protocol methods.
            # Require the actual pinned Pi child to initialize and list tools.
            if not {"initialize","tools/list"}.issubset(
                    set(http_probe.methods()[before_project_http:])):
                raise RuntimeError("native project HTTP MCP initialize/tools-list absent")
            other=json.loads(run(args,project_b,env=env,timeout=55))
            if any(entry.get("name") in ("pi-managed","pi-http") for entry in other.get("servers",[])):
                raise RuntimeError("untrusted Pi project B discovered project A MCP server")
            methods_path=project_a/".uap/plugin-data"
            logs=list(methods_path.glob("**/pi-mcp-methods.log"))
            if len(logs)!=1: raise RuntimeError("real Pi stdio MCP method log absent")
            methods=logs[0].read_text().splitlines()
            if "initialize" not in methods or "tools/list" not in methods:
                raise RuntimeError("real Pi MCP initialize/tools-list missing")
            state["stages"].append("real_Pi_MCP_initialize_and_tools_list")
            # Only the installed Pi process may dispatch the genuine MCP
            # tools/call. Each scripted local model requests one exact tool.
            def native_call(profile, project, env, tool):
                fixture_model = ScriptedPiModel(tool)
                try:
                    write(profile, "models.json", json.dumps({"providers": {"uap-fixture": {
                        "name": "UAP local CI model",
                        "baseUrl": fixture_model.base_url,
                        "api": "openai-completions",
                        "apiKey": "local-test-only",
                        "models": [{"id": "uap-scripted", "name": "Deterministic",
                                    "reasoning": False, "input": ["text"],
                                    "contextWindow": 32768, "maxTokens": 1024}]
                    }}}, separators=(",", ":")))
                    child = [str(pi), "--print", "--offline", "--no-session",
                             "--approve", "--tools", "+codemode",
                             "--model", "uap-fixture/uap-scripted",
                             "Call the available Pi MCP "+tool+" exactly once."]
                    run(child, project, env=env, timeout=95, max_output=1<<20)
                    requests, seen, calls, results, invalid = fixture_model.stats()
                finally:
                    fixture_model.close()
                if requests == 0 or seen == 0 or calls != 1 or results == 0 or invalid != 0:
                    raise RuntimeError("real Pi "+tool+" model-driven MCP call not qualified")

            native_call(user, project_a, env, "echo")
            if logs[0].read_text().splitlines().count("tools/call") != 1:
                raise RuntimeError("Pi did not issue exactly one native project stdio tools/call")
            before_http = http_probe.methods().count("tools/call")
            native_call(user, project_a, env, "ping")
            if http_probe.methods().count("tools/call") != before_http+1:
                raise RuntimeError("Pi did not issue exactly one native project HTTP tools/call")
            state["stages"].append("real_Pi_project_stdio_and_HTTP_initialize_list_call")

            # Exercise global resources through the same native pinned Pi
            # executable. Project C contains no project override; A/B are
            # intentionally left installed while both global profiles cycle.
            project_c = root/"C"
            project_c.mkdir()
            for label, profile in (("default",user), ("custom",root/"custom-pi-profile")):
                profile.mkdir(parents=True, exist_ok=True)
                state_root=root/("global-"+label)
                name="TEST-pi-global-"+label
                if label=="custom":
                    write(profile,"mcp.json",foreign)
                    # This original selected profile must be handed to both
                    # UAP and the actual Pi process, never inferred from HOME.
                    original_user_mcp=(user/"mcp.json").read_bytes()
                installed=op(project_c,"install",scope="user",profile=profile,
                             state_root=state_root,name=name)
                binding=installed["binding"]
                active=Path(binding.get("target_path",""))
                if active.parent!=profile/"extensions" or (active/"index.ts").exists():
                    raise RuntimeError("Pi global owned package escaped profile or became an extension")
                global_skill=profile/"skills/pi-managed/SKILL.md"
                if not global_skill.exists() or "first" not in global_skill.read_text():
                    raise RuntimeError("Pi global Skill was not copied into original selected profile")
                global_config=json.loads((profile/"mcp.json").read_text())
                if not all(key in global_config.get("mcpServers",{}) for key in ("foreign","pi-managed","pi-http")):
                    raise RuntimeError("Pi global MCP ownership does not retain foreign entries")
                if label=="custom" and (user/"mcp.json").read_bytes()!=original_user_mcp:
                    raise RuntimeError("custom Pi Global unexpectedly wrote the default profile")
                native_env=pi_env(profile)
                observed_commands=checked_rpc(pi,project_c,native_env,True,label+"-global-skill")
                if sum(1 for item in observed_commands if item.get("source")=="skill" and
                       item.get("name")=="skill:pi-managed")!=1:
                    raise RuntimeError("genuine Pi did not discover scoped global Skill")
                before_global_http=len(http_probe.methods())
                observed_mcp=json.loads(run([str(pi),"mcp","list","--json"],
                                           project_c,env=native_env,timeout=55))
                # Check every selected profile independently: previous project
                # or default-profile protocol exchanges are not Global proof.
                if not {"initialize","tools/list"}.issubset(
                        set(http_probe.methods()[before_global_http:])):
                    raise RuntimeError("native "+label+" Global HTTP MCP initialize/tools-list absent")
                for server_name, tool in (("pi-managed","echo"),("pi-http","ping")):
                    entries=[entry for entry in observed_mcp.get("servers",[])
                             if entry.get("name")==server_name]
                    # Pinned upstream Pi v1.4.0 reports `global`, not UAP's `user`.
                    # The native shell contract is explicit in upstream
                    # packages/coding-agent/src/extensions/mcp/{config,cli}.ts.
                    if len(entries)!=1 or entries[0].get("scope")!="global" or (
                            entries[0].get("state")!="connected") or tool not in entries[0].get("tools",[]):
                        # Preserve the strict native gate. Report only bounded,
                        # non-secret status fields to distinguish a scope-name
                        # mismatch from a failed genuine MCP connection.
                        observed=[{
                            "name": item.get("name"), "scope": item.get("scope"),
                            "state": item.get("state"),
                            "tools": item.get("tools",[])[:12]
                                if isinstance(item.get("tools"), list) else None,
                        } for item in entries[:2]]
                        raise RuntimeError("genuine Pi global MCP server not connected: "+
                                           server_name+" observed="+json.dumps(observed,sort_keys=True)[:600])
                global_logs=list((state_root/"plugin-data").glob("**/pi-mcp-methods.log"))
                if len(global_logs)!=1 or not all(x in global_logs[0].read_text().splitlines()
                                                 for x in ("initialize","tools/list")):
                    raise RuntimeError("Pi user stdio MCP initialize/list absent")
                native_call(profile,project_c,native_env,"echo")
                if global_logs[0].read_text().splitlines().count("tools/call")!=1:
                    raise RuntimeError("Pi global stdio tools/call missing")
                before_http=http_probe.methods().count("tools/call")
                native_call(profile,project_c,native_env,"ping")
                if http_probe.methods().count("tools/call")!=before_http+1:
                    raise RuntimeError("Pi global HTTP tools/call missing")
                skill("global-"+label)
                op(project_c,"update",scope="user",profile=profile,
                   state_root=state_root,name=name)
                if "global-"+label not in global_skill.read_text():
                    raise RuntimeError("Pi global Skill update missing")
                global_skill.parent.joinpath("SKILL.md").unlink()
                global_skill.parent.rmdir()
                op(project_c,"repair",scope="user",profile=profile,
                   state_root=state_root,name=name)
                if "global-"+label not in global_skill.read_text():
                    raise RuntimeError("Pi global Skill repair missing")
                op(project_c,"remove",scope="user",profile=profile,
                   state_root=state_root,name=name)
                if global_skill.exists() or any(key in json.loads((profile/"mcp.json").read_text()).get("mcpServers",{})
                                                for key in ("pi-managed","pi-http")):
                    raise RuntimeError("Pi global removal left owned Skill/MCP")
                if "foreign" not in json.loads((profile/"mcp.json").read_text()).get("mcpServers",{}):
                    raise RuntimeError("Pi global removal destroyed foreign MCP")
                if (project_b/".pi/skills/pi-managed/SKILL.md").exists() is False:
                    raise RuntimeError("global cycle mutated project B")
                state["stages"].append(label+"_Pi_global_skill_stdio_HTTP_call_update_repair_remove")
                skill("first")

            state["real_model_qualified"] = True
            state["mcp_tools_call_qualified"] = True
            skill("second")
            op(project_a,"update")
            if not any("second" in p.read_text() for p in (project_a/".pi/skills").glob("*/SKILL.md")):
                raise RuntimeError("Pi managed Skill update not installed")
            op(project_a,"remove")
            if list((project_a/".pi/skills").glob("pi-managed/SKILL.md")):
                raise RuntimeError("removed Pi project Skill remains visible")
            if not (project_b/".pi/skills/pi-managed/SKILL.md").exists():
                raise RuntimeError("project B Skill removed with A")
            # Global install/remove may legitimately reserialize the JSON.
            # Preserve the exact foreign document semantically, not its old whitespace.
            if json.loads((user/"mcp.json").read_text()) != json.loads(foreign):
                raise RuntimeError("user Pi MCP foreign content changed after Global lifecycle")
            remaining = json.loads((project_a/".pi/mcp.json").read_text()).get("mcpServers", {})
            if not isinstance(remaining, dict) or "foreign" not in remaining:
                raise RuntimeError("foreign project MCP was removed")
            if "pi-managed" in remaining:
                raise RuntimeError("owned Pi project MCP survived removal")
            state["stages"].append("A_update_remove_B_and_foreign_preserved")
            state["status"]="passed"
    finally:
        (destination/"pi-project-components-evidence.json").write_text(json.dumps(state,indent=2,sort_keys=True)+"\n")

if __name__=="__main__": main()
