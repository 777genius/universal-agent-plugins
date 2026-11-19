"""Genuine Devin Local 3000.12.57 Project installation smoke on an isolated hosted runner.

Runs the public source-built UAP CLI and real hash-pinned Devin without vendor
credentials or model calls. Only file installation and native listing qualify.
"""
import base64
from contextlib import contextmanager
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.request import Request, urlopen

from native_client_research import kill_group, run_probe

VERSION = "3000.12.57"
# CognitionAI/devin-cli's own publish_npm.py builds this version-specific
# platform archive from the hash-checked vendor manifest. The CDN original
# currently returns HTTP 403 on hosted runners, so use the official npm copy
# without relaxing the independently pinned original executable digest.
PACKAGE_VERSION = VERSION + "-linux-x64"
METADATA_URL = "https://registry.npmjs.org/devin/" + PACKAGE_VERSION
TARBALL_URL = "https://registry.npmjs.org/devin/-/devin-" + PACKAGE_VERSION + ".tgz"
BINARY_SHA = "3f3fb99f472d92f7852be3775ac2b66b5d60baf26f3441b02d54c56b160bc30d"
MAX_ARCHIVE = 96 << 20
MAX_EXPANDED = 256 << 20
SKILL = "uap-e2e-project-v57"
STDIO = "uap_e2e_project_57_stdio"
HTTP = "uap_e2e_project_57_http"
RAW_SOURCE = "https://github.com/777genius/universal-agent-plugins/tree/b5d80336b53397be5bbb72c84f680e5885a6ac0d/tests/fixtures/uap_apm/skills"
RAW_SKILL = "benchmark-note"
RAW_DESCRIPTION = "An inert note for checking Agent Skills 1.0 installation."
RAW_SKILL_SHA = "53e39c4f1dcaa4c2ed1196654678ede091b71fb837f77a1beb07ba556abc1198"
RAW_REFERENCE_SHA = "674bbf87f1469969103f15f60ea1389829fe431dfca5439da1fc57da211d1f2f"
RAW_MANIFEST_SHA = "c9bbd0b5bbb39d2ae168390ee53a28d4e825de8c0b5334dbc4a55e300b095ab0"


def digest(data):
    return hashlib.sha256(data).hexdigest()


def command_diagnostic(body, env):
    """Reuse the bounded redacted-tail policy; redact before truncation."""
    text = body.decode("utf-8", errors="replace")
    for key, value in env.items():
        if isinstance(value, str) and len(value) >= 4 and re.search(
                r"(?i)token|password|secret|api[_-]?key|authorization|credential", key):
            text = text.replace(value, "<redacted>")
            text = text.replace(json.dumps(value)[1:-1], "<redacted>")
    text = re.sub(r"""(?i)(["']?Authorization["']?\s*:\s*["']?(?:Basic|Bearer)\s+)[^\s"',;}]+""",
                  r"\1<redacted>", text)
    text = re.sub(r"""(?i)((?:token|password|secret|api[_-]?key|credential)["']?\s*[:=]\s*)(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s,;}]+)""",
                  r"\1<redacted>", text)
    return text[-2048:]


def command(argv, cwd, env, timeout=90, receipt=None):
    # Share the repository's bounded, supervised process runner with all other
    # native collectors. A timed-out CLI cannot leave orphaned subprocesses.
    record, stdout, stderr = run_probe(
        argv, Path(cwd), env, Path("/tmp"), "devin-project-e2e",
        timeout=timeout, limit=1 << 20, memory_only=True)
    if receipt is not None:
        receipt["result"] = {
            "exit_code": record["exit"],
            "capture_error": (command_diagnostic(str(record["error"]).encode(), env)
                              if record["error"] is not None else None),
            "elapsed_seconds": record.get("elapsed_seconds"),
            "stdout_bytes": len(stdout), "stdout_sha256": digest(stdout),
            "stderr_bytes": len(stderr), "stderr_sha256": digest(stderr),
        }
        if record["error"] is not None or record["exit"] != 0:
            receipt["result"]["stderr_diagnostic"] = command_diagnostic(stderr, env)
            try:
                public = json.loads(stdout)
            except (ValueError, UnicodeError):
                public = None
            allowed = {"command": ("skills add", "remove"), "result": ("success", "failure"),
                       "status": ("succeeded", "failed", "partial", "interrupted"),
                       "readiness": ("ready", "needs_action", "failed", "unknown")}
            if isinstance(public, dict):
                receipt["result"]["public_status"] = {
                    key: public[key] for key, values in allowed.items()
                    if isinstance(public.get(key), str) and public[key] in values}
    if record["error"] is not None or record["exit"] != 0:
        stage = receipt.get("stage", "unspecified") if receipt is not None else "unspecified"
        capture = (command_diagnostic(str(record["error"]).encode(), env)[:128]
                   if record["error"] is not None else None)
        diagnostic = command_diagnostic(stderr, env)
        raise RuntimeError(
            f"command {Path(argv[0]).name} stage={stage} failed, exit={record['exit']}, "
            f"capture={capture}, stderr={diagnostic[:512]}, stderr_sha256={digest(stderr)}")
    return stdout.decode("utf-8", errors="strict")


def official_npm_bytes(url, limit):
    with urlopen(Request(url, headers={"User-Agent": "uap-devin-native-e2e/1.0"}), timeout=90) as response:
        body = response.read(limit + 1)
    if len(body) > limit:
        raise RuntimeError("official Devin npm response exceeded its fixed bound")
    return body


def npm_archive():
    metadata = json.loads(official_npm_bytes(METADATA_URL, 1 << 16))
    if (not isinstance(metadata, dict) or metadata.get("name") != "devin" or
            metadata.get("version") != PACKAGE_VERSION or
            metadata.get("os") != ["linux"] or metadata.get("cpu") != ["x64"]):
        raise RuntimeError("official Devin platform metadata differs")
    dist = metadata.get("dist")
    if not isinstance(dist, dict) or dist.get("tarball") != TARBALL_URL:
        raise RuntimeError("Devin npm tarball source is not the pinned platform")
    integrity = dist.get("integrity")
    if not isinstance(integrity, str) or not integrity.startswith("sha512-"):
        raise RuntimeError("Devin npm platform is missing SHA-512 integrity")
    try:
        expected = base64.b64decode(integrity[7:], validate=True)
    except ValueError as error:
        raise RuntimeError("invalid Devin npm integrity encoding") from error
    if len(expected) != 64:
        raise RuntimeError("invalid Devin npm integrity length")
    archive = official_npm_bytes(TARBALL_URL, MAX_ARCHIVE)
    if hashlib.sha512(archive).digest() != expected:
        raise RuntimeError("official Devin npm platform tarball integrity differs")
    return archive, integrity


def install_genuine_devin(root):
    archive, integrity = npm_archive()
    target = root / "vendor-devin"
    target.mkdir(mode=0o700)
    with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as package:
        members = package.getmembers()
        if not 3 <= len(members) <= 512:
            raise RuntimeError("unexpected Devin npm platform member count")
        seen, total = set(), 0
        for member in members:
            name = member.name
            parts = name.split("/")
            if (not name.startswith("package/") or
                    any(part in ("", ".", "..") for part in parts) or
                    len(parts) > 16 or name in seen or
                    not (member.isdir() or member.isfile()) or
                    member.size < 0 or member.size > MAX_EXPANDED):
                raise RuntimeError("unsafe Devin npm platform member")
            seen.add(name)
            total += member.size
            if total > MAX_EXPANDED:
                raise RuntimeError("Devin npm platform exceeds expanded size bound")
            path = target.joinpath(*parts[1:])
            if member.isdir():
                path.mkdir(mode=0o700, parents=True, exist_ok=False)
                continue
            path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
            stream = package.extractfile(member)
            if stream is None:
                raise RuntimeError("Devin npm member is not readable")
            with stream, path.open("xb") as output:
                remaining = member.size
                while remaining:
                    chunk = stream.read(min(1 << 20, remaining))
                    if not chunk:
                        raise RuntimeError("short Devin npm platform member")
                    output.write(chunk)
                    remaining -= len(chunk)
                if stream.read(1):
                    raise RuntimeError("oversized Devin npm platform member")
    manifest = json.loads((target / "package.json").read_text())
    if (manifest.get("name") != "devin" or
            manifest.get("version") != PACKAGE_VERSION or
            manifest.get("os") != ["linux"] or manifest.get("cpu") != ["x64"] or
            manifest.get("repository", {}).get("url") !=
            "git+https://github.com/CognitionAI/devin-cli.git" or
            (target / "distribution").read_text() != "npm"):
        raise RuntimeError("Devin npm platform manifest/authority differs")
    path = target / "bin" / "devin"
    if not path.is_file():
        raise RuntimeError("genuine Devin executable absent from npm platform")
    with path.open("rb") as stream:
        computed = hashlib.file_digest(stream, "sha256").hexdigest()
    if computed != BINARY_SHA:
        raise RuntimeError("npm executable differs from independently pinned original Devin binary")
    path.chmod(0o700)
    return path, integrity


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


def serve_http(port_file):
    # The server runs in its own process; run_probe uses preexec_fn and must
    # never fork from a parent with a live HTTP thread.
    with ThreadingHTTPServer(("127.0.0.1", 0), InertMCP) as server:
        port_file.write_text(str(server.server_port))
        server.serve_forever()


@contextmanager
def external_http_fixture(root):
    port_file = root / "http-port"
    child = subprocess.Popen(
        [sys.executable, str(Path(__file__).resolve()), "--serve-http", str(port_file)],
        cwd=root, env={"PATH": "/usr/bin:/bin", "PYTHONUNBUFFERED": "1"},
        stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        start_new_session=True)
    try:
        for _ in range(100):
            if child.poll() is not None:
                raise RuntimeError("isolated Devin HTTP fixture exited during startup")
            if port_file.is_file():
                raw = port_file.read_text()
                if not raw.isdecimal() or not 1 <= int(raw) <= 65535:
                    raise RuntimeError("isolated Devin HTTP fixture returned an invalid port")
                yield int(raw)
                break
            time.sleep(0.05)
        else:
            raise RuntimeError("isolated Devin HTTP fixture startup deadline exceeded")
    finally:
        if child.poll() is None:
            kill_group(child)
        child.wait(timeout=5)


def available_skill(body, name):
    data = json.loads(body)
    if isinstance(data, dict):
        data = data.get("skills")
    if not isinstance(data, list):
        raise RuntimeError("genuine Devin Skill inventory has an unknown shape")
    return any(isinstance(row, dict) and row.get("name") == name for row in data)


def available_mcp(body, name, config_path, transport):
    # Pinned genuine Devin 3000.12.57 exposes structured MCP inventory rows.
    # A diagnostic mentioning a name is not proof of a configured server.
    rows = json.loads(body)
    if not isinstance(rows, list) or any(not isinstance(row, dict) for row in rows):
        raise RuntimeError("genuine Devin MCP JSON inventory has an unknown shape")
    matching = [row for row in rows if row.get("name") == name]
    if len(matching) > 1:
        raise RuntimeError("genuine Devin MCP inventory has duplicate selected names")
    if not matching:
        return False
    row = matching[0]
    schema = {"name", "enabled", "disabled_reason", "transport", "command", "args",
              "url", "disabled_tools", "source"}
    if (set(row) != schema or row["enabled"] is not True or
            row["disabled_reason"] is not None or row["transport"] != transport or
            row["source"] != str(config_path)):
        raise RuntimeError("genuine Devin MCP selected source or enabled state differs")
    return True



def raw_skill_present(body, expected_path):
    """Qualify the selected raw Skill, never a Cascade import or name alone."""
    rows = json.loads(body)
    if not isinstance(rows, list) or len(rows) > 128 or any(not isinstance(row, dict) for row in rows):
        raise RuntimeError("raw Skill inventory has an unknown shape")
    matches = [row for row in rows if row.get("name") == RAW_SKILL]
    if not matches:
        return False
    expected = {"name": RAW_SKILL, "description": RAW_DESCRIPTION, "provider": "Devin",
                "base_dir": str(expected_path), "display_name": RAW_SKILL,
                "triggers": ["user", "model"], "warnings": [], "errors": []}
    if matches != [expected]:
        raise RuntimeError("raw Skill native metadata or selected Project path differs")
    return True


def raw_ready_output(body, repeated=False):
    result = json.loads(body)
    if (not isinstance(result, dict) or result.get("schema_version") != 1
            or result.get("command") != "skills add" or result.get("result") != "success"
            or result.get("status") != "succeeded" or result.get("readiness") != "ready"
            or not isinstance(result.get("items"), list) or len(result["items"]) != 1
            or result.get("next_actions")):
        raise RuntimeError("public raw Project Skills did not establish automatic native Ready")
    item = result["items"][0]
    if (not isinstance(item, dict) or item.get("alias") != RAW_SKILL
            or item.get("targetId") != "windsurf"
            or item.get("status") != ("unchanged" if repeated else "succeeded")):
        raise RuntimeError("raw Project Skills selected item or repeat status differs")
    return result


def raw_repeat_snapshot(managed, native, project_root):
    """Finite owned payload/state/lock identities, without exporting their bodies."""
    paths = [project_root / ".uap" / "state-v2.json", project_root / "uap.toml", project_root / "uap.lock"]
    for root in (managed, native):
        paths.append(root)
        for path in root.rglob("*"):
            paths.append(path)
            if len(paths) > 256:
                raise RuntimeError("raw repeat owned snapshot exceeds its entry bound")
    result, total = {}, 0
    for path in paths:
        info = path.lstat()
        if path.is_symlink() or not (path.is_file() or path.is_dir()) or info.st_size > 1 << 20:
            raise RuntimeError("raw repeat snapshot requires ordinary bounded owned paths")
        row = {"mode": info.st_mode, "device": info.st_dev, "inode": info.st_ino}
        if path.is_file():
            with path.open("rb") as stream:
                body = stream.read((1 << 20) + 1)
            total += len(body)
            if len(body) > 1 << 20 or total > 4 << 20:
                raise RuntimeError("raw repeat owned snapshot exceeds its byte bound")
            row.update(bytes=len(body), sha256=digest(body), modified_ns=info.st_mtime_ns)
        result[str(path)] = row
    return result


def raw_state_installations(document):
    if (not isinstance(document, dict) or type(document.get("schema_version")) is not int
            or document["schema_version"] != 5 or not isinstance(document.get("installations"), list)):
        raise RuntimeError("raw Project saved state schema differs")
    seen = set()
    for item in document["installations"]:
        if (not isinstance(item, dict) or not isinstance(item.get("installation_id"), str)
                or not item["installation_id"] or item["installation_id"] in seen
                or not isinstance(item.get("clients"), dict)):
            raise RuntimeError("raw Project saved installation shape differs")
        seen.add(item["installation_id"])
        for key, client in item["clients"].items():
            if (not isinstance(key, str) or not key or not isinstance(client, dict)
                    or client.get("client_binding_id") != key or client.get("client_id") != "windsurf"
                    or client.get("scope") != "project" or not isinstance(client.get("target_locator"), str)
                    or not client["target_locator"]):
                raise RuntimeError("raw Project saved client shape differs")
    return document["installations"]


def raw_pinned_resources(managed, native):
    skill_files = {"SKILL.md": RAW_SKILL_SHA, "references/fixture.txt": RAW_REFERENCE_SHA}
    expected = ({"plugin.json": RAW_MANIFEST_SHA,
                 **{"skills/" + RAW_SKILL + "/" + name: sha for name, sha in skill_files.items()}}, skill_files)
    evidence = {}
    for root, pinned in zip((managed, native), expected):
        if root.is_symlink() or not root.is_dir():
            raise RuntimeError("raw Project pinned tree is not an ordinary directory")
        found = {}
        for index, path in enumerate(root.rglob("*")):
            if index >= 32 or path.is_symlink():
                raise RuntimeError("raw Project pinned tree exceeds bounds or contains a symlink")
            if path.is_dir():
                continue
            relative = path.relative_to(root).as_posix()
            if not path.is_file() or relative not in pinned or path.stat().st_size > 1 << 20:
                raise RuntimeError("raw Project pinned file set differs")
            found[relative] = digest(path.read_bytes())
        if found != pinned:
            raise RuntimeError("raw Project pinned resources differ")
        evidence[str(root)] = found
    return evidence


def raw_project_skills(root, report, binary, cli, base_env):
    # Reuse the same pinned native runtime, public CLI and bounded runner. A
    # separate owned profile makes the exact read-only global policy explicit.
    case = root / "raw-project-skills"
    home, config = case / "home", case / "config"
    global_root = config / "devin"
    projects = [case / "project-a", case / "project-b"]
    outside = case / "outside"
    for path in (home, global_root, outside, *projects):
        path.mkdir(parents=True)
    env = dict(base_env, HOME=str(home), XDG_CONFIG_HOME=str(config),
               XDG_DATA_HOME=str(case / "data"), XDG_STATE_HOME=str(case / "state"),
               XDG_CACHE_HOME=str(case / "cache"))
    protected = {}
    def seed(path, body):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(body)
        path.chmod(0o600)
        protected[path] = (path.read_bytes(), path.stat().st_mode)
    seed(global_root / "config.json", '{"version":1,"auto_update":false}\n')
    for index, path in enumerate((global_root, home / ".codeium" / "windsurf", *(p / ".devin" for p in projects))):
        seed(path / "mcp_config.json", '{"mcpServers":{"foreign":{"url":"https://foreign.invalid/mcp","disabled":true}}}\n')
        name = "foreign-raw-case-" + str(index)
        seed(path / "skills" / name / "SKILL.md",
             f"---\nname: {name}\ndescription: Inert foreign Skill\n---\nLeave unchanged.\n")
    evidence = {"source": RAW_SOURCE, "skill_sha256": RAW_SKILL_SHA,
                "commands": [], "public": {}, "inventories": []}
    report["raw_project_skills"] = evidence
    def run(label, argv, cwd, timeout=90):
        receipt = {"stage": label, "argv": argv, "cwd": str(cwd)}
        evidence["commands"].append(receipt)
        return command(argv, cwd, env, timeout=timeout, receipt=receipt)
    def preserved():
        for path, (body, mode) in protected.items():
            if path.is_symlink() or path.read_bytes() != body or path.stat().st_mode != mode:
                raise RuntimeError("raw Project Skills changed foreign, Global or Cascade bytes/mode")
    def inventory(label, present):
        for index, cwd in enumerate([*projects, outside]):
            body = run(label + "-" + str(index), [str(binary), "skills", "list", "--json"], cwd, 45)
            evidence["inventories"].append({"stage": label, "scope": ("a", "b", "user")[index],
                                            "cwd": str(cwd), "rows": json.loads(body)})
            if raw_skill_present(body, projects[0] / ".devin" / "skills" / RAW_SKILL) != (present and index == 0):
                raise RuntimeError("raw Project Skill escaped A or is absent from its exact native source")
        preserved()
    # An explicit Project root, not the caller cwd, selects the delivery target.
    add_argv = [str(cli), "skills", "add", RAW_SOURCE, "--skill", RAW_SKILL,
                "--target", "windsurf", "--scope", "project", "--project-root", str(projects[0]),
                "--yes", "--no-ui", "--format", "json"]
    inventory("before-add", False)
    body = run("skills-add", add_argv, outside)
    evidence["public"]["add"] = json.loads(body)
    raw_ready_output(body)
    document = json.loads((projects[0] / ".uap" / "state-v2.json").read_text())
    installations = raw_state_installations(document)
    if len(installations) != 1:
        raise RuntimeError("raw Project saved installation shape differs")
    installation = installations[0]
    if installation.get("source", {}).get("resolved_revision") != "b5d80336b53397be5bbb72c84f680e5885a6ac0d":
        raise RuntimeError("raw Project saved source differs from the immutable public commit")
    bindings = list(installation.get("clients", {}).values())
    if len(bindings) != 1:
        raise RuntimeError("raw Project saved client count differs")
    binding = bindings[0]
    expected = {"client_id": "windsurf", "scope": "project", "activation": "active",
                "authentication": "not_required", "policy": "allowed", "verification": "installation_verified",
                "native_profile_root": str(projects[0] / ".devin")}
    if any(binding.get(key) != value for key, value in expected.items()):
        raise RuntimeError("raw Project binding lacks exact automatic native proof")
    managed = Path(binding.get("target_locator", ""))
    if managed.parent != projects[0] / ".uap" / "managed" / "clients" / "windsurf" or managed.is_symlink() or not managed.is_dir():
        raise RuntimeError("raw Project managed target escaped its selected scope")
    skill = projects[0] / ".devin" / "skills" / RAW_SKILL / "SKILL.md"
    if skill.is_symlink() or digest(skill.read_bytes()) != RAW_SKILL_SHA:
        raise RuntimeError("raw Project Skill differs from pinned public source bytes")
    evidence["pinned_resources"] = raw_pinned_resources(managed, skill.parent)
    evidence["binding"] = binding
    evidence["installation_source"] = installation.get("source")
    inventory("after-add", True)
    state_root = projects[0] / ".uap"
    before = raw_repeat_snapshot(managed, skill.parent, projects[0])
    body = run("skills-repeat", add_argv, outside)
    evidence["public"]["repeat"] = json.loads(body)
    raw_ready_output(body, repeated=True)
    repeated_rows = raw_state_installations(json.loads((state_root / "state-v2.json").read_text()))
    if len(repeated_rows) != 1:
        raise RuntimeError("raw Project repeated installation count differs")
    repeated = repeated_rows[0]
    after = raw_repeat_snapshot(managed, skill.parent, projects[0])
    evidence["repeat_continuity"] = {"before": before, "after": after,
        "installation_id": repeated.get("installation_id"), "binding": list(repeated.get("clients", {}).values())}
    if (repeated.get("installation_id") != installation["installation_id"]
            or list(repeated.get("clients", {}).values()) != [binding] or before != after):
        raise RuntimeError("raw Project repeat changed installation, binding, payload, state or lock")
    inventory("after-repeat", True)
    removed = json.loads(run("remove", [str(cli), "remove", installation["installation_id"],
        "--target", "windsurf", "--scope", "project", "--project-root", str(projects[0]),
        "--yes", "--format", "json"], outside))
    evidence["public"]["remove"] = removed
    if removed.get("result") != "success":
        raise RuntimeError("raw Project ordinary removal failed")
    inventory("after-remove", False)
    if managed.exists() or managed.is_symlink() or skill.parent.exists() or skill.parent.is_symlink():
        raise RuntimeError("raw Project removal retained owned files")
    remaining = json.loads((projects[0] / ".uap" / "state-v2.json").read_text())
    if any(item["clients"] for item in raw_state_installations(remaining)):
        raise RuntimeError("raw Project removal retained an active client binding")
    report["raw_project_skills_native_ready"] = True


def qualified(report):
    return report.get("installation_discovery_e2e") is True and report.get("raw_project_skills_native_ready") is True


def main(root, report):
    checkout = Path(os.environ["GITHUB_WORKSPACE"])
    expected = os.environ["EXPECTED_COMMIT"]
    actual = command(["git", "rev-parse", "HEAD"], checkout, os.environ).strip()
    if not re.fullmatch(r"[0-9a-f]{40}", expected) or actual != expected:
        raise RuntimeError("hosted source differs from exact PR commit")
    binary, integrity = install_genuine_devin(root)
    report["npm_tarball_integrity"] = integrity
    report["verified_binary_sha256"] = BINARY_SHA
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
    with external_http_fixture(root) as http_port:
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
                          HTTP: {"type":"streamable-http","url":f"http://127.0.0.1:{http_port}/mcp"}}}))
        cli = root / "agentplugins"
        command(["go", "build", "-trimpath", "-o", str(cli), "./cli/cmd/agentplugins"], checkout, os.environ, timeout=200)
        report["uap_binary_sha256"] = digest(cli.read_bytes())

        def inventory(stage, present):
            observations = []
            for index, project in enumerate(projects):
                skill_output = command([str(binary), "skills", "list", "--json"], project, env, timeout=45)
                mcp_output = command([str(binary), "mcp", "list", "--json"], project, env, timeout=45)
                selected_mcp = project / ".devin" / "mcp_config.json"
                found = [available_skill(skill_output, SKILL),
                         available_mcp(mcp_output, STDIO, selected_mcp, "stdio"),
                         available_mcp(mcp_output, HTTP, selected_mcp, "http")]
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

        native_a_path = projects[0] / ".devin" / "mcp_config.json"
        state_path = projects[0] / ".uap" / "state-v2.json"

        def binding_for(iid, required=True):
            document = json.loads(state_path.read_text())
            items = document.get("installations")
            if document.get("schema_version") != 5 or not isinstance(items, list):
                raise RuntimeError("UAP Project saved state version or shape differs")
            selected = [item for item in items if item.get("installation_id") == iid]
            if len(selected) != 1 or not isinstance(selected[0].get("clients"), dict):
                raise RuntimeError("UAP Project installation or clients differ")
            bindings = [value for value in selected[0]["clients"].values()
                        if isinstance(value, dict) and value.get("client_id") == "windsurf"]
            if not required:
                if bindings:
                    raise RuntimeError("removed Project still has an owned client binding")
                return None
            if len(bindings) != 1:
                raise RuntimeError("UAP Project has missing/duplicate selected bindings")
            binding = bindings[0]
            path = Path(binding.get("target_locator", ""))
            root = projects[0] / ".uap" / "managed" / "clients" / "windsurf"
            if (binding.get("scope") != "project" or
                    binding.get("native_profile_root") != str(projects[0] / ".devin") or
                    path.parent != root or not path.is_dir() or path.is_symlink()):
                raise RuntimeError("UAP Project binding or managed package path differs")
            return binding

        def preserved(stage):
            if (cascade.read_bytes() != original_cascade or
                    (global_root / "mcp_config.json").read_bytes() != original_global or
                    (projects[1] / ".devin" / "mcp_config.json").read_bytes() != original_b):
                raise RuntimeError(stage + " changed Cascade, Global or Project B")
            for project in projects:
                foreign = project / ".devin" / "skills" / "foreign-skill" / "SKILL.md"
                if foreign.is_symlink() or not foreign.is_file() or foreign.read_bytes() != original_skill:
                    raise RuntimeError(stage + " changed an original foreign Skill")
            current = json.loads(native_a_path.read_text())
            if current.get("mcpServers", {}).get("foreign_original") != original_a["mcpServers"]["foreign_original"]:
                raise RuntimeError(stage + " changed an original Project A MCP entry")

        def installation_id(data, *, unchanged=False):
            if not isinstance(data, dict):
                raise RuntimeError("UAP add has no structured response")
            if unchanged:
                # The public singleton no-change result deliberately has no
                # batch operation or targets; see project_native_windsurf_cycle.
                if "targets" in data or "operation_id" in data or data.get("dry_run") is not False:
                    raise RuntimeError("unchanged UAP add used an unexpected batch shape")
                output = data
            else:
                targets = data.get("targets")
                if (not isinstance(targets, list) or len(targets) != 1 or
                        not isinstance(targets[0], dict) or targets[0].get("target") != "windsurf" or
                        targets[0].get("status") != "external_completed"):
                    raise RuntimeError("UAP first add target receipt differs")
                output = targets[0].get("output")
            if not isinstance(output, dict) or not isinstance(output.get("result"), dict):
                raise RuntimeError("UAP add has no client result")
            outcome = output["result"]
            if unchanged and (outcome.get("no_change") is not True or
                              outcome.get("mutated") is not False or
                              outcome.get("requires_confirmation") is not False):
                raise RuntimeError("UAP unchanged add falsely reports a mutation")
            identifier = outcome.get("installation_id")
            if not isinstance(identifier, str) or not re.fullmatch(r"[0-9a-f-]{36}", identifier):
                raise RuntimeError("UAP add has no canonical installation ID")
            return identifier

        inventory("before-add", False)
        added = cli_call("add", str(source), "--no-save")
        iid = installation_id(added)
        installed_binding = binding_for(iid)
        managed = Path(installed_binding["target_locator"])
        installed_native = native_a_path.read_bytes()
        original_state = state_path.read_bytes()
        inventory("after-add", True)
        preserved("after-add")

        replayed = cli_call("add", str(source), "--no-save")
        if (installation_id(replayed, unchanged=True) != iid or binding_for(iid) != installed_binding or
                native_a_path.read_bytes() != installed_native or state_path.read_bytes() != original_state):
            raise RuntimeError("unchanged Project add rewrote binding/state or native config")
        inventory("after-unchanged-add", True)
        preserved("after-unchanged-add")

        owned_skill = projects[0] / ".devin" / "skills" / SKILL
        if owned_skill.is_symlink() or not (owned_skill / "SKILL.md").is_file():
            raise RuntimeError("UAP-owned Project Skill is absent or aliased")
        shutil.rmtree(owned_skill)
        cli_call("repair", iid)
        repaired = binding_for(iid)
        if repaired["client_binding_id"] != installed_binding["client_binding_id"] or repaired["target_locator"] != str(managed):
            raise RuntimeError("Project repair changed original binding authority")
        inventory("after-repair", True)
        preserved("after-repair")

        cli_call("remove", iid)
        inventory("after-remove", False)
        binding_for(iid, required=False)
        if managed.exists() or managed.is_symlink():
            raise RuntimeError("Project removal retained the managed package")
        if json.loads(native_a_path.read_text()) != original_a or owned_skill.exists() or owned_skill.is_symlink():
            raise RuntimeError("Project A removal changed original MCP or retained owned objects")
        preserved("after-remove")
        report["installation_discovery_e2e"] = True
    raw_project_skills(root, report, binary, cli, env)



if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "--serve-http":
        serve_http(Path(sys.argv[2]))
        sys.exit(0)
    if len(sys.argv) != 1:
        raise SystemExit("unsupported Devin E2E invocation")
    result = {"commit":os.environ.get("EXPECTED_COMMIT"), "version":VERSION, "npm_package_version": PACKAGE_VERSION,
              "stages":[], "installation_discovery_e2e":False, "raw_project_skills_native_ready":False, "model_runtime_qualified":False}
    with tempfile.TemporaryDirectory(prefix="devin-project-v57-", dir=os.environ.get("RUNNER_TEMP")) as temp:
        directory = Path(temp)
        try:
            main(directory, result)
        except Exception as error:
            result["failure"] = f"{type(error).__name__}: {str(error)[:320]}"
        report_path = Path(os.environ["RUNNER_TEMP"]) / "devin-project-v57-e2e.json"
        report_path.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
        print(json.dumps({"commit": result["commit"], "installation_discovery_e2e":result["installation_discovery_e2e"],
                          "raw_project_skills_native_ready":result["raw_project_skills_native_ready"],
                          "failure":result.get("failure")}, sort_keys=True))
    sys.exit(0 if qualified(result) else 1)
