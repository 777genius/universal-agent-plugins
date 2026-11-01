#!/usr/bin/env python3
"""Genuine Pi project Skills/MCP E2E; reuses the pinned official Pi test host."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

from pi_project_native_e2e import PINS, checked_release, require_hosted, run, write

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
        with tempfile.TemporaryDirectory(prefix="uap-pi-components-", dir=os.environ["RUNNER_TEMP"]) as tmp:
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
                    "args":["${PLUGIN_ROOT}/probe.mjs"],"cwd":"${PLUGIN_ROOT}","env":{}}}}
            write(package,"mcp.json",json.dumps(mcp_body,separators=(",",":")))
            def op(project, verb):
                receipt=root/(project.name+"-"+verb+".json")
                args=[str(driver),str(project/".uap"),str(user),str(project),str(package),
                      verb,"TEST-pi-components",str(receipt)]
                run(args,project,timeout=95)
                outcome=json.loads(receipt.read_text())
                if outcome.get("outcome") not in ("completed","unchanged") or outcome.get("binding",{}).get("scope")!="project":
                    raise RuntimeError("UAP Pi project transaction did not publish exact receipt")
                return outcome
            for project in (project_a,project_b): op(project,"install")
            state["stages"].append("UAP_project_A_B_installed")
            def pi_env():
                env={"HOME":str(root/"home"),"USERPROFILE":str(root/"home"),
                     "PI_CODING_AGENT_DIR":str(user),"PI_OFFLINE":"1",
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
            output=run(args,project_a,env=env,timeout=55)
            if len(output)>1<<20:
                raise RuntimeError("native project MCP inventory exceeds limit")
            catalog=json.loads(output)
            entries=catalog.get("servers",[])
            matching=[entry for entry in entries if entry.get("name")=="pi-managed"]
            if len(matching)!=1 or matching[0].get("scope")!="project" or (
                matching[0].get("state")!="connected") or "echo" not in matching[0].get("tools",[]):
                raise RuntimeError("real Pi did not connect and enumerate exact project MCP tool")
            other=json.loads(run(args,project_b,env=env,timeout=55))
            if any(entry.get("name")=="pi-managed" for entry in other.get("servers",[])):
                raise RuntimeError("untrusted Pi project B discovered project A MCP server")
            methods_path=project_a/".uap/plugin-data"
            logs=list(methods_path.glob("**/pi-mcp-methods.log"))
            if len(logs)!=1: raise RuntimeError("real Pi stdio MCP method log absent")
            methods=logs[0].read_text().splitlines()
            if "initialize" not in methods or "tools/list" not in methods:
                raise RuntimeError("real Pi MCP initialize/tools-list missing")
            state["stages"].append("real_Pi_MCP_initialize_and_tools_list")
            skill("second")
            op(project_a,"update")
            if not any("second" in p.read_text() for p in (project_a/".pi/skills").glob("*/SKILL.md")):
                raise RuntimeError("Pi managed Skill update not installed")
            op(project_a,"remove")
            if list((project_a/".pi/skills").glob("pi-managed/SKILL.md")):
                raise RuntimeError("removed Pi project Skill remains visible")
            if not (project_b/".pi/skills/pi-managed/SKILL.md").exists():
                raise RuntimeError("project B Skill removed with A")
            if (user/"mcp.json").read_text()!=foreign: raise RuntimeError("user Pi MCP changed")
            if '"foreign"' not in (project_a/".pi/mcp.json").read_text():
                raise RuntimeError("foreign project MCP was removed")
            state["stages"].append("A_update_remove_B_and_foreign_preserved")
            state["status"]="passed"
    finally:
        (destination/"pi-project-components-evidence.json").write_text(json.dumps(state,indent=2,sort_keys=True)+"\n")

if __name__=="__main__": main()
