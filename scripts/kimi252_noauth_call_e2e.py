"""Genuine pinned Kimi calls UAP-installed MCP through a local scripted provider."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import secrets
import selectors
import shutil
import signal
import stat
import subprocess
import tempfile
import time
import urllib.parse

import kimi_252_native_web_mcp_probe as native
import kimi_stable_project_mcp_smoke as installed
from grok_project_protocol_client import document, installation_state, read_owned, require, retained_data
from grok_project_protocol_e2e import snapshot, write
from kimi252_noauth_provider import LIMIT, MODEL, admit, approval, original, provider
from native_client_research import run_probe

NAME = native.SERVER_NAME
SOURCE = Path(__file__).resolve().parent.parent
# Node 24's standard WebSocket client. It observes only; Python validates and
# answers the one exact interaction. Token arrives on private stdin, never argv.
BRIDGE = r'''
let input='';
for await(const part of process.stdin){input+=part;if(Buffer.byteLength(input)>4096)process.exit(2);}
let cfg;try{cfg=JSON.parse(input);}catch{process.exit(2);}
const url=new URL(cfg.base);
if(url.protocol!=='http:'||url.hostname!=='127.0.0.1'||url.pathname!=='/'||url.search||url.hash||url.username||url.password||!url.port||!/^[A-Za-z0-9_-]{16,128}$/.test(cfg.token)||typeof cfg.session!=='string')process.exit(2);
const ws=new WebSocket(cfg.base.replace('http:','ws:')+'/api/v1/ws',['kimi-code.bearer.'+cfg.token]);
let count=0,size=0;
const timer=setTimeout(()=>{ws.close();process.exit(3);},60000);
ws.addEventListener('open',()=>{ws.send(JSON.stringify({type:'client_hello',id:'owned-hello',payload:{client_id:'uap-owned-test'}}));ws.send(JSON.stringify({type:'subscribe_v2',id:'owned-subscribe',payload:{session_id:cfg.session,transcript:{'*':'block'}}}));});
ws.addEventListener('message',event=>{const raw=String(event.data);size+=Buffer.byteLength(raw);count++;if(size>1048576||count>256||raw.includes(cfg.token)){ws.close();process.exit(4);}try{JSON.parse(raw);}catch{ws.close();process.exit(5);}process.stdout.write(raw+'\n');});
ws.addEventListener('error',()=>process.exit(6));
ws.addEventListener('close',()=>{clearTimeout(timer);process.exit(0);});
'''


def kill(process):
    if process is not None:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait(timeout=5)


def stop_runtime(stream, bridge, process):
    try:
        if stream is not None:
            stream.close()
    finally:
        try:
            kill(bridge)
        finally:
            kill(process)


def marker_identity(path, root, expected):
    require(read_owned(path, root).decode() == expected, "owned marker bytes differ")
    info = path.stat()
    return [info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_ctime_ns]


class Journey:
    def __init__(self, root, installer, probe, client, node, evidence):
        self.root, self.installer, self.probe = root, installer, probe
        self.client, self.node, self.evidence = client, node, evidence
        self.home, self.a, self.b, self.package = [root / part for part in ("home", "a", "b", "package")]
        self.log, self.nonce, self.protected = root / "mcp-events.jsonl", secrets.token_hex(16), {}
        for path in (self.home, self.a, self.b, self.package / "bin", root / "tmp"):
            path.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.env = {"HOME": str(self.home), "USERPROFILE": str(self.home),
            "KIMI_CODE_HOME": str(self.home / ".kimi-code"), "KIMI_CODE_NO_AUTO_UPDATE": "1",
            "PATH": str(client.parent) + ":/usr/bin:/bin", "TMPDIR": str(root / "tmp"),
            "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8", "TERM": "dumb", "NO_COLOR": "1",
            "AGENTPLUGINS_HOME": str(root / "installer-state"), "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_TERMINAL_PROMPT": "0"}
        for part in ("CONFIG", "DATA", "CACHE", "STATE"):
            path = root / ("xdg-" + part.lower())
            path.mkdir(mode=0o700)
            self.env["XDG_" + part + "_HOME"] = str(path)
        (self.home / ".kimi-code").mkdir(mode=0o700)

    def check(self):
        for path, digest in self.protected.items():
            require(hashlib.sha256(read_owned(path, self.root)).hexdigest() == digest,
                    "protected User/foreign/config changed")

    def command(self, label, argv, cwd=None, timeout=90):
        self.check()
        row, out, err = run_probe([str(x) for x in argv], cwd or self.a, self.env, self.root,
                                  label, timeout=timeout, limit=LIMIT, memory_only=True)
        self.evidence["commands"].append({**row, "stdout": original(out), "stderr": original(err)})
        self.check()
        require(row["error"] is None and row["exit"] == 0, "native/App command failed: " + label)
        return out

    def cli(self, label, verb, selection):
        args = [self.installer, verb, selection, "--target", "kimi", "--scope", "project",
                "--project-root", self.a, "--format", "json"]
        if verb == "add":
            args.append("--no-save")
        result = document(self.command(label, args, timeout=120))
        require(result.get("result") == "success", "public App did not report success")

    def author(self, revision):
        write(self.package / "plugin.json", {"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
              "name": NAME, "version": "1.0." + revision})
        if not (self.package / "bin/probe").exists():
            shutil.copyfile(self.probe, self.package / "bin/probe")
            (self.package / "bin/probe").chmod(0o700)
        write(self.package / "mcp.json", {"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
              "mcpServers": {NAME: {"type": "stdio", "command": "./bin/probe", "args": [],
              "env": {"UAP_TEST_ROOT": str(self.root), "UAP_TEST_EVENTS": str(self.log),
                      "UAP_TEST_NONCE": self.nonce, "UAP_TEST_REVISION": revision,
                      "UAP_TEST_REPORT_OPERATION": "1"}}}})

    def selected(self):
        rows = [row for row in installation_state(self.a)["installations"] if row.get("declared_name") == NAME]
        require(len(rows) == 1, "owned installation missing or duplicated")
        item = rows[0]
        bindings = item.get("clients", {})
        require(isinstance(bindings, dict) and len(bindings) == 1, "one selected Kimi binding required")
        binding = next(iter(bindings.values()))
        require(binding.get("client_id") == "kimi" and binding.get("scope") == "project"
                and binding.get("native_profile_root") == str(self.a / ".kimi-code"),
                "native binding scope/profile differs")
        active = Path(binding.get("target_locator", ""))
        artifact = binding.get("physical_artifact_id")
        require(active.is_absolute() and active.resolve() == active and active.is_dir()
                and active.is_relative_to(self.a / ".uap") and isinstance(artifact, str)
                and Path(artifact).name == artifact and artifact not in ("", ".", ".."),
                "managed package authority differs")
        data = self.a / ".uap/plugin-data" / artifact
        receipt = item.get("data_receipts", {}).get(binding.get("data_receipt_id"), {})
        require(receipt.get("state") == "owned" and receipt.get("scope") == "project"
                and receipt.get("locator") == str(data), "owned data authority differs")
        installed.verify(self.a / ".kimi-code/mcp.json", foreign=self.foreign)
        return item, binding, active, data

    def configure(self, url):
        self.check()
        path = self.home / ".kimi-code/config.toml"
        body = ('default_model="owned"\ndefault_permission_mode="manual"\ntelemetry=false\n'
                'auto_session_title=false\n[providers.local]\ntype="openai"\nbase_url=' + json.dumps(url)
                + '\napi_key="inert-local-only"\n[models.owned]\nprovider="local"\nmodel=' + json.dumps(MODEL)
                + '\nmax_context_size=64000\ncapabilities=["tool_use"]\n')
        write(path, body)
        self.protected[path] = hashlib.sha256(body.encode()).hexdigest()
        origin = url.removesuffix("/v1")
        self.env.update(HTTP_PROXY=origin, HTTPS_PROXY=origin, NO_PROXY="127.0.0.1,localhost")

    def frames(self, bridge, token, record):
        selector = selectors.DefaultSelector()
        selector.register(bridge.stdout, selectors.EVENT_READ)
        pending, total = b"", 0
        try:
            while True:
                if b"\n" in pending:
                    line, pending = pending.split(b"\n", 1)
                    require(token.encode() not in line, "private native bearer in event")
                    record.setdefault("websocket_originals", []).append(original(line))
                    yield document(line)
                    continue
                require(selector.select(.2) or bridge.poll() is None, "native bridge exited")
                if not selector.select(.01):
                    yield None
                    continue
                chunk = os.read(bridge.stdout.fileno(), 65536)
                require(chunk, "native bridge closed")
                total += len(chunk)
                require(total <= LIMIT, "native event bound")
                pending += chunk
        finally:
            selector.close()

    def session(self, label, expected=None, reject=False, project=None):
        project = project or self.a
        row = {"stage": label, "requests": [], "passed": False}
        self.evidence["native_sessions"].append(row)
        config_path = project / ".kimi-code/mcp.json"
        config_before = read_owned(config_path, self.root) if os.path.lexists(config_path) else None
        row["native_project_config"] = {"path": str(config_path), "present": config_before is not None}
        if config_before is not None:
            row["native_project_config"]["original"] = original(config_before)
        before = read_owned(self.log, self.root) if self.log.exists() else b""
        marker_before = None
        if expected is not None:
            marker = Path(expected["plugin_data"]) / "native-marker.txt"
            if expected["operation"] == "write":
                require(not os.path.lexists(marker), "initial marker already exists")
            else:
                marker_before = marker_identity(marker, self.root, expected["marker"])
        with provider(expected, reject) as (url, driver):
            row["requests"] = driver.requests
            self.configure(url)
            process = bridge = stream = None
            trust_path = token = None
            pending_trust = False
            try:
                base, token, process = native.start_server(self.client, self.env, project)
                driver.secret = token
                require(NAME not in native.list_servers(base, token, project), "unexpected preexisting trust")
                workspace = native.response("/api/v1/workspaces", base, token, "POST",
                                            {"root": str(project), "name": "UAP owned native E2E"})
                trust_path = "/api/v1/workspaces/" + urllib.parse.quote(workspace["id"], safe="")
                require(native.response(trust_path + "/trust", base, token) == {"trusted": False},
                        "native project already trusted")
                # Absence must be observed AFTER ordinary explicit trust too;
                # untrusted-workspace suppression cannot prove source removal.
                pending_trust = True
                require(native.response(trust_path + "/trust", base, token, "POST", {}) == {"trusted": True},
                        "native trust grant failed")
                row["trust_granted"] = True
                catalog = native.list_servers(base, token, project)
                if expected is not None:
                    native.ensure_project_server(catalog, config_path)
                else:
                    require(NAME not in catalog, "trusted absent source advertised owned server")
                session = native.response("/api/v1/sessions", base, token, "POST", {"workspace_id": workspace["id"]})
                sid = session["id"]
                row["native_session_id"] = sid
                bridge = subprocess.Popen([str(self.node), "--input-type=module", "-e", BRIDGE],
                    stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                    cwd=project, env=self.env, start_new_session=True)
                bridge.stdin.write(json.dumps({"base": base, "token": token, "session": sid}).encode())
                bridge.stdin.close()
                stream = self.frames(bridge, token, row)
                deadline = time.monotonic() + 10
                for frame in stream:
                    require(time.monotonic() < deadline, "native subscribe deadline")
                    if frame and frame.get("type") == "ack" and frame.get("id") == "owned-subscribe":
                        require(type(frame.get("code")) is int and frame["code"] == 0, "native subscribe failed")
                        break
                path = "/api/v1/sessions/" + urllib.parse.quote(sid, safe="")
                submitted = native.response(path + "/prompts", base, token, "POST", {
                    "model": "owned", "permission_mode": "manual", "content": [{"type": "text",
                    "text": "Use only the owned fixture MCP tool once, then stop."}]})
                require(submitted.get("status") == "running", "native prompt did not start")
                deadline, answered, manual = time.monotonic() + 35, False, False
                for frame in stream:
                    require(time.monotonic() < deadline and driver.error is None, "native turn deadline/protocol failure")
                    if frame and frame.get("type") == "agent":
                        require(frame.get("session_id") == sid and frame.get("agent_id") == "main"
                                and frame.get("permission_mode") == "manual", "native permission mode differs")
                        manual = True
                    if frame and frame.get("type") == "interaction" and frame.get("status") == "pending":
                        require(expected is not None and not answered and manual, "unexpected or repeated native approval")
                        identity = approval(frame, sid, expected)
                        result = native.response(path + "/interactions/" + urllib.parse.quote(identity, safe=""),
                            base, token, "POST", {"kind": "approval", "decision": "rejected" if reject else "approved"})
                        require(result.get("outcome") == ("rejected" if reject else "approved"), "native consent failed")
                        row["approval"] = frame
                        row["approval_result"] = result
                        answered = True
                    if driver.completed:
                        state = native.response(path, base, token)
                        if not state.get("busy"):
                            require(state.get("last_turn_reason") == "completed" and state.get("permission_rules") == []
                                    and state.get("agent_config", {}).get("model") == "owned" and manual,
                                    "native turn/permission state differs")
                            row["final_session"] = state
                            break
                require(driver.completed and (expected is None or answered), "native call/consent incomplete")
                after = read_owned(self.log, self.root) if self.log.exists() else b""
                require(after.startswith(before), "original MCP log replaced")
                events = [document(line) for line in after[len(before):].splitlines()]
                row["original_events"] = events
                calls = [event for event in events if event.get("method") == "tools/call"]
                if expected is None:
                    require(not events and not driver.called, "absent source executed native MCP")
                elif reject:
                    require(not calls and not os.path.lexists(marker), "rejected native call caused marker or MCP effect")
                else:
                    require([event.get("method") for event in events] == ["initialize", "tools/list", "tools/call"],
                            "native MCP event order/count differs")
                    require(len(calls) == 1 and calls[0].get("facts") == driver.result, "native/provider fact join differs")
                    require(all(event.get("nonce") == self.nonce and event.get("pid") == driver.result["pid"]
                                and event.get("session") == driver.result["session"] for event in events),
                            "native probe process/session differs")
                    observed = marker_identity(marker, self.root, expected["marker"])
                    require(marker_before is None or observed == marker_before, "native read mutated marker")
                row["native_result"] = driver.result
            finally:
                try:
                    if pending_trust:
                        native.response(trust_path + "/untrust", base, token, "POST", {})
                        require(native.response(trust_path + "/trust", base, token) == {"trusted": False}
                                and NAME not in native.list_servers(base, token, project), "native revoke not verified")
                        row["trust_revoked"] = True
                finally:
                    try:
                        stop_runtime(stream, bridge, process)
                    finally:
                        row["provider_error"] = driver.error
                        current = read_owned(config_path, self.root) if os.path.lexists(config_path) else None
                        require(current == config_before, "native Project config changed")
                        self.check()
        row["provider_error"] = driver.error
        admit(driver)
        self.check()
        row["passed"] = True

    def execute(self):
        require(self.command("kimi-version", [self.client, "--version"]) == b"2.5.2\n", "native version differs")
        require(self.command("node-version", [self.node, "--version"]).startswith(b"v24."), "Node24 required")
        for label, project in (("a", self.a), ("b", self.b)):
            self.command("git-init-" + label, ["/usr/bin/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"], project)
        for path in (self.home / "foreign.txt", self.a / "foreign.txt", self.b / "foreign.txt"):
            write(path, self.nonce)
            self.protected[path] = hashlib.sha256(read_owned(path, self.root)).hexdigest()
        untouched_b = snapshot(self.b)
        user_config = self.home / ".kimi-code/mcp.json"
        write(user_config, {"mcpServers": {"foreign-user-disabled": {
            "transport": "http", "url": "http://127.0.0.1:9/user-disabled", "enabled": False}}})
        self.protected[user_config] = hashlib.sha256(read_owned(user_config, self.root)).hexdigest()
        self.foreign = {"transport": "http", "url": "http://127.0.0.1:9/foreign-disabled", "enabled": False}
        config = self.a / ".kimi-code/mcp.json"
        write(config, {"mcpServers": {installed.FOREIGN_NAME: self.foreign}})
        self.author("0")
        self.cli("add", "add", self.package)
        first, binding, active, data = self.selected()
        held, receipt = snapshot(active), read_owned(self.a / ".uap/state-v2.json", self.root)
        self.cli("repeat", "add", self.package)
        require(snapshot(active) == held and read_owned(self.a / ".uap/state-v2.json", self.root) == receipt,
                "repeat changed package or ownership")
        marker = "owned-" + self.nonce
        expected = {"nonce": self.nonce, "revision": "0", "marker": marker, "operation": "write",
                    "cwd": str(active), "plugin_root": str(active), "plugin_data": str(data)}
        self.session("declined-owned-call", expected, reject=True)
        self.session("installed", expected)
        self.session("project-b-absent", project=self.b)
        self.author("1")
        self.cli("update", "update", self.package)
        updated, updated_binding, replacement, _ = self.selected()
        require(replacement == active and updated_binding["client_binding_id"] == binding["client_binding_id"]
                and updated["source"]["tree_digest"] != first["source"]["tree_digest"], "update authority differs")
        expected.update(revision="1", operation="read")
        self.session("updated", expected)
        shutil.rmtree(active)
        self.cli("repair-missing", "repair", NAME)
        repaired, repaired_binding, repaired_active, repaired_data = self.selected()
        require(repaired_active == active and repaired_data == data and repaired["source"] == updated["source"]
                and repaired_binding["client_binding_id"] == binding["client_binding_id"]
                and repaired["data_receipts"] == updated["data_receipts"], "repair authority differs")
        self.session("repaired", expected)
        self.cli("remove", "remove", NAME)
        installed.verify_removed(config, foreign=self.foreign)
        require(not os.path.lexists(active), "removed package survives")
        self.session("removed-absent")
        self.evidence["retained_installation"] = retained_data(self.a, repaired)
        marker_identity(data / "native-marker.txt", self.root, marker)
        require(snapshot(self.b) == untouched_b,
                "Project B/User MCP changed")
        self.check()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for key in ("installer", "probe", "node", "output"):
        parser.add_argument("--" + key, type=Path, required=True)
    args = parser.parse_args()
    require(os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted",
            "genuine disposable hosted runner required")
    host = (platform.system(), platform.machine())
    pins = {("Linux", "x86_64"): ("linux-x64", 58177868, "6614a511048219d53996454cac8594c9541bbb6cf6e8f0a081d4e878be511ce2"),
            ("Darwin", "arm64"): ("darwin-arm64", 54792334, "974500f5360f21c00fb73ab978b7d362f44d6acfdb7110b975b8d7a44ff0da9c")}
    require(host in pins, "unqualified native host")
    expected = os.environ.get("EXPECTED_COMMIT", "")
    def git(*parts):
        return subprocess.check_output(["/usr/bin/git", *parts], cwd=SOURCE, timeout=10).decode().strip()
    require(len(expected) == 40 and git("rev-parse", "HEAD") == expected
            and not git("status", "--porcelain", "--untracked-files=normal"), "clean exact source required")
    for path in (args.installer, args.probe, args.node):
        require(path.is_absolute() and path.resolve() == path and path.is_file()
                and stat.S_ISREG(path.stat().st_mode) and path.stat().st_uid == os.getuid(), "owned exact executable required")
    executables = {key: hashlib.sha256(getattr(args, key).read_bytes()).hexdigest()
                   for key in ("installer", "probe", "node")}
    args.output.mkdir(mode=0o700, parents=True, exist_ok=False)
    evidence = {"schema_version": 1, "source_commit": expected, "source_tree": git("rev-parse", "HEAD^{tree}"),
        "run_id": os.environ.get("GITHUB_RUN_ID"), "run_attempt": os.environ.get("GITHUB_RUN_ATTEMPT"),
        "event": os.environ.get("GITHUB_EVENT_NAME"), "platform": list(host), "passed": False,
        "vendor_authentication": False, "server_issued_loopback_authentication": True,
        "actual_model": False, "scripted_local_provider": True,
        "commands": [], "native_sessions": [], "compiled_executables_sha256": executables}
    try:
        from project_native_research import Budget, acquire, materialize, verify_files
        suffix, size, digest = pins[host]
        spec = {"url": "https://github.com/MoonshotAI/kimi-code/releases/download/%40moonshot-ai/kimi-code%402.5.2/kimi-code-" + suffix + ".tar.gz",
            "integrity": "sha256:" + digest, "expected_bytes": size, "max_bytes": 96 << 20,
            "max_expanded_bytes": 256 << 20, "format": "tar.gz", "entrypoint": "kimi", "runtime": "native"}
        with tempfile.TemporaryDirectory(prefix="kimi252-native-call-", dir=os.environ["RUNNER_TEMP"]) as tmp:
            root = Path(tmp).resolve()
            budget = Budget(1200)
            receipt, contents = acquire(spec, budget)
            client, identities = materialize(root / "bin", spec, receipt, contents)
            evidence["acquisition"] = receipt
            evidence["binary_sha256"] = identities[client]
            journey = Journey(root, args.installer, args.probe, client, args.node, evidence)
            try:
                journey.execute()
                verify_files(identities, budget)
            finally:
                if journey.log.exists():
                    evidence["mcp_original"] = original(read_owned(journey.log, root))
        require(git("rev-parse", "HEAD") == expected
                and git("rev-parse", "HEAD^{tree}") == evidence["source_tree"]
                and not git("status", "--porcelain", "--untracked-files=normal"), "source changed")
        require(all(hashlib.sha256(getattr(args, key).read_bytes()).hexdigest() == digest
                    for key, digest in executables.items()), "compiled executable changed")
        evidence["passed"] = True
    except Exception as error:
        evidence["error"] = type(error).__name__ + ": " + str(error)
        raise
    finally:
        write(args.output / "evidence.json", evidence)
        print(json.dumps({"passed": evidence["passed"]}))


if __name__ == "__main__":
    main()
