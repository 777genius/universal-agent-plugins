"""Genuine Devin Local 3000.12.57 Project installation smoke on an isolated hosted runner.

Runs the public source-built UAP CLI and real hash-pinned Devin without vendor
credentials or model calls. Only file installation and native listing qualify.
"""
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import sys
import tarfile
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.request import urlopen

from native_client_research import run_probe

VERSION = "3000.12.57"
URL = f"https://static.devin.ai/cli/{VERSION}/devin-{VERSION}-x86_64-unknown-linux.tar.gz"
ARCHIVE_SHA = "0946784ef886d716e82a8026f76933bf0a6e4f0a804a23722bb62047e8c07d4a"
BINARY_SHA = "3f3fb99f472d92f7852be3775ac2b66b5d60baf26f3441b02d54c56b160bc30d"
SKILL = "uap-e2e-project-v57"
STDIO = "uap_e2e_project_57_stdio"
HTTP = "uap_e2e_project_57_http"


def digest(data):
    return hashlib.sha256(data).hexdigest()


def command(argv, cwd, env, timeout=90):
    # Share the repository's bounded, supervised process runner with all other
    # native collectors. A timed-out CLI cannot leave orphaned subprocesses.
    record, stdout, stderr = run_probe(
        argv, Path(cwd), env, Path("/tmp"), "devin-project-e2e",
        timeout=timeout, limit=1 << 20, memory_only=True)
    if record["error"] is not None or record["exit"] != 0:
        raise RuntimeError(
            f"command {Path(argv[0]).name} failed, exit={record['exit']}, "
            f"capture={record['error']}, stderr_sha256={digest(stderr)}")
    return stdout.decode("utf-8", errors="strict")


def install_genuine_devin(root):
    with urlopen(URL, timeout=60) as response:
        archive = response.read((96 << 20) + 1)
    if digest(archive) != ARCHIVE_SHA:
        raise RuntimeError("pinned Devin archive digest differs")
    with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as package:
        member = package.getmember("bin/devin")
        if not member.isfile() or member.size <= 0 or member.size > 128 << 20:
            raise RuntimeError("Devin archive executable is not an ordinary bounded file")
        stream = package.extractfile(member)
        if stream is None:
            raise RuntimeError("Devin archive has no executable payload")
        executable = stream.read((128 << 20) + 1)
    if digest(executable) != BINARY_SHA:
        raise RuntimeError("pinned Devin executable digest differs")
    path = root / "bin" / "devin"
    path.parent.mkdir()
    path.write_bytes(executable)
    path.chmod(0o700)
    return path


class InertMCP(BaseHTTPRequestHandler):
    def do_POST(self):
        size = int(self.headers.get("Content-Length", "0"))
        if self.path != "/mcp" or not 0 < size <= 65536:
            self.send_error(400)
            return
        request = json.loads(self.rfile.read(size))
        method = request.get("method")
        if method == "initialize":
            data = {"protocolVersion": "2025-03-26", "capabilities": {"tools": {}},
                    "serverInfo": {"name": "uap-devin-install-only", "version": "1.0.0"}}
        elif method == "tools/list":
            data = {"tools": []}
        else:
            data = {}
        body = json.dumps({"jsonrpc": "2.0", "id": request.get("id"), "result": data}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_args):
        return


def available_skill(body, name):
    data = json.loads(body)
    if isinstance(data, dict):
        data = data.get("skills")
    if not isinstance(data, list):
        raise RuntimeError("genuine Devin Skill inventory has an unknown shape")
    return any(isinstance(row, dict) and row.get("name") == name for row in data)


def available_mcp(body, name):
    return re.search(r"(?<![A-Za-z0-9_])" + re.escape(name) + r"(?![A-Za-z0-9_])", body) is not None


def main(root, report):
    checkout = Path(os.environ["GITHUB_WORKSPACE"])
    expected = os.environ["EXPECTED_COMMIT"]
    actual = command(["git", "rev-parse", "HEAD"], checkout, os.environ).strip()
    if not re.fullmatch(r"[0-9a-f]{40}", expected) or actual != expected:
        raise RuntimeError("hosted source differs from exact PR commit")
    binary = install_genuine_devin(root)
    home = root / "home"
    config = home / "xdg-config"
    global_root = config / "devin"
    projects = [root / "project-a", root / "project-b"]
    state = root / "uap-state"
    scratch = root / "scratch"
    for path in (home, config, global_root, state, scratch, home / "xdg-data", home / "xdg-cache", home / "xdg-state", *projects):
        path.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, HOME=str(home), XDG_CONFIG_HOME=str(config),
               XDG_DATA_HOME=str(home / "xdg-data"), XDG_STATE_HOME=str(home / "xdg-state"),
               XDG_CACHE_HOME=str(home / "xdg-cache"), AGENTPLUGINS_HOME=str(state),
               TMPDIR=str(scratch), PATH=str(binary.parent) + os.pathsep + os.environ["PATH"])
    version = command([str(binary), "--version"], projects[0], env)
    if not re.fullmatch(r"devin 3000\.12\.57(?: \([0-9a-f]+\))?\s*", version):
        raise RuntimeError("selected genuine Devin version output differs")
    cascade = home / ".codeium" / "windsurf" / "mcp_config.json"
    cascade.parent.mkdir(parents=True)
    cascade.write_text('{"mcpServers":{"cascade_original":{"serverUrl":"https://foreign.invalid/mcp"}}}\n')
    original_cascade = cascade.read_bytes()
    (global_root / "mcp_config.json").write_text('{"mcpServers":{"global_original":{"url":"https://foreign.invalid/mcp"}}}\n')
    original_global = (global_root / "mcp_config.json").read_bytes()
    for project in projects:
        native = project / ".devin"
        (native / "skills" / "foreign-skill").mkdir(parents=True)
        (native / "skills" / "foreign-skill" / "SKILL.md").write_text("---\nname: foreign-skill\ndescription: foreign\n---\n")
        (native / "mcp_config.json").write_text('{"mcpServers":{"foreign_original":{"url":"https://foreign.invalid/mcp"}}}\n')
    original_a = json.loads((projects[0] / ".devin" / "mcp_config.json").read_text())
    original_b = (projects[1] / ".devin" / "mcp_config.json").read_bytes()
    original_skill = (projects[0] / ".devin" / "skills" / "foreign-skill" / "SKILL.md").read_bytes()
    with ThreadingHTTPServer(("127.0.0.1", 0), InertMCP) as http_server:
        thread = threading.Thread(target=http_server.serve_forever, daemon=True)
        thread.start()
        try:
            service = root / "service.py"
            service.write_text(
                'import json,sys\n'
                'for line in sys.stdin:\n'
                ' try:\n'
                '  request=json.loads(line)\n'
                '  method=request.get("method")\n'
                '  result={"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"uap-deven","version":"1"}} if method=="initialize" else {"tools":[]} if method=="tools/list" else {}\n'
                '  if "id" in request: print(json.dumps({"jsonrpc":"2.0","id":request["id"],"result":result}),flush=True)\n'
                ' except (ValueError,TypeError): pass\n')
            source = root / "plugin"
            skill_file = source / "skills" / SKILL / "SKILL.md"
            skill_file.parent.mkdir(parents=True)
            skill_file.write_text(f"---\nname: {SKILL}\ndescription: Inert Project installation proof\n---\nNo model action required\n")
            (source / "plugin.json").write_text(json.dumps({"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                                                              "name": SKILL, "version":"1.0.0"}))
            (source / "mcp.json").write_text(json.dumps({"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
               "mcpServers": {STDIO: {"type":"stdio","command":"python3","args":[str(service)]},
                              HTTP: {"type":"streamable-http","url":f"http://127.0.0.1:{http_server.server_port}/mcp"}}}))
            cli = root / "agentplugins"
            command(["go", "build", "-trimpath", "-o", str(cli), "./cli/cmd/agentplugins"], checkout, os.environ, timeout=200)
            report["uap_binary_sha256"] = digest(cli.read_bytes())

            def inventory(stage, present):
                observations = []
                for index, project in enumerate(projects):
                    skill_output = command([str(binary), "skills", "list", "--json"], project, env, timeout=45)
                    mcp_output = command([str(binary), "mcp", "list"], project, env, timeout=45)
                    found = [available_skill(skill_output, SKILL), available_mcp(mcp_output, STDIO), available_mcp(mcp_output, HTTP)]
                    want = present if index == 0 else False
                    if found != [want] * 3:
                        raise RuntimeError(f"genuine Devin {stage} Project {index} Skills/MCP discovery mismatched: {found}")
                    observations.append({"project":index, "owned_skill_stdio_http": found})
                report["stages"].append({"stage":stage,"inventory":observations})

            def cli_call(stage, *arguments):
                output = command([str(cli), stage, *arguments, "--target", "windsurf",
                                  "--scope", "project", "--format", "json"],
                                 projects[0], env, timeout=90)
                data = json.loads(output)
                if data.get("command") != stage or data.get("result") != "success":
                    raise RuntimeError(f"UAP public {stage} did not report success")
                return data["data"]

            inventory("before-add", False)
            added = cli_call("add", str(source), "--no-save")
            targets = added.get("targets")
            if not isinstance(targets, list) or len(targets) != 1 or targets[0].get("target") != "windsurf":
                raise RuntimeError("UAP add target receipt differs")
            result = targets[0].get("output", {}).get("result", {})
            iid = result.get("installation_id") if isinstance(result, dict) else None
            if not isinstance(iid, str) or not re.fullmatch(r"[0-9a-f-]{36}", iid):
                raise RuntimeError("UAP add lacks an installation ID")
            inventory("after-add", True)
            owned_skill = projects[0] / ".devin" / "skills" / SKILL
            if owned_skill.is_symlink() or not (owned_skill / "SKILL.md").is_file():
                raise RuntimeError("UAP-owned Project Skill is absent or aliased")
            shutil.rmtree(owned_skill)
            cli_call("repair", iid)
            inventory("after-repair", True)
            cli_call("remove", iid)
            inventory("after-remove", False)
            native_a = json.loads((projects[0] / ".devin" / "mcp_config.json").read_text())
            if native_a != original_a or owned_skill.exists() or owned_skill.is_symlink():
                raise RuntimeError("Project A removal changed foreign MCP or retained owned native objects")
            if (cascade.read_bytes() != original_cascade or
                (global_root / "mcp_config.json").read_bytes() != original_global or
                (projects[1] / ".devin" / "mcp_config.json").read_bytes() != original_b):
                raise RuntimeError("Project lifecycle mutated Cascade, Global or Project B authority")
            for project in projects:
                foreign_skill = project / ".devin" / "skills" / "foreign-skill" / "SKILL.md"
                if not foreign_skill.is_file() or foreign_skill.is_symlink() or foreign_skill.read_bytes() != original_skill:
                    raise RuntimeError("Project lifecycle changed foreign Skill")
            report["installation_discovery_e2e"] = True
        finally:
            http_server.shutdown()
            thread.join(timeout=5)


if __name__ == "__main__":
    result = {"commit":os.environ.get("EXPECTED_COMMIT"), "version":VERSION, "archive_sha256":ARCHIVE_SHA,
              "stages":[], "installation_discovery_e2e":False, "model_runtime_qualified":False}
    with tempfile.TemporaryDirectory(prefix="devin-project-v57-", dir=os.environ.get("RUNNER_TEMP")) as temp:
        directory = Path(temp)
        try:
            main(directory, result)
        except Exception as error:
            result["failure"] = f"{type(error).__name__}: {str(error)[:320]}"
        report_path = Path(os.environ["RUNNER_TEMP"]) / "devin-project-v57-e2e.json"
        report_path.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
        print(json.dumps({"commit": result["commit"], "installation_discovery_e2e":result["installation_discovery_e2e"],
                          "failure":result.get("failure")}, sort_keys=True))
    sys.exit(0 if result["installation_discovery_e2e"] else 1)
