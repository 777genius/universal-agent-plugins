"""Pinned Kimi help and local API research; product qualification stays false.

The shared frontend owns hosted admission, process/HTTP bounds, private capture,
token redaction and cleanup. This leaf uses only the reviewed native web API.
It does not install a product plugin, configure a model or issue a tool call.
"""
import codecs
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import stat
import time
from urllib.parse import urlencode

CLIENT = "kimi"
VERSION = "2.3.2"
AVAILABLE_PHASES = ("acquire", "help", "observe")
NEEDS_NATIVE_PROBE = True
ACQUISITION = {
    "url": "https://github.com/MoonshotAI/kimi-code/releases/download/"
           "%40moonshot-ai/kimi-code%402.3.2/kimi-code-linux-x64.tar.gz",
    "integrity": "sha256:4715ccd1a591b64683929a14b26b3d07ac4c484e4da3881b22a2ccca56304042",
    "max_bytes": 96 << 20,
    "expected_bytes": 65_938_009,
    "format": "tar.gz",
    "entrypoint": "kimi",
    "runtime": "native",
}

# The exact release's produce-manifest.mjs packages the sole executable as
# `tar -C <work> -czf <archive> kimi`. The frontend must verify the archive pin
# and regular entry before executing it. Never reuse a 2.3.1 binary digest.
SOURCE_COMMIT = "b8dd3a134060c6e80ba4a1659335cd63b68483f2"
PROBES = (
    ("kimi-version", ("--version",), None),
    ("kimi-help", ("--help",), ""),
    ("kimi-doctor-help", ("doctor", "--help"), "doctor"),
    ("kimi-acp-help", ("acp", "--help"), "acp"),
    ("kimi-web-help", ("web", "--help"), "web"),
)


def _accepted_streams(record, stdout, stderr):
    if (not isinstance(record, dict) or type(record.get("exit")) is not int
            or record["exit"] != 0 or "error" not in record
            or record["error"] not in (None, "")):
        raise ValueError("Kimi help probe failed; bounded capture retained")
    if not isinstance(stdout, bytes) or not isinstance(stderr, bytes) or stderr:
        raise ValueError("Kimi help probe emitted unexpected stream data")
    try:
        return stdout.decode("utf-8", errors="strict")
    except UnicodeError as error:
        raise ValueError("Kimi help probe is not UTF-8") from error


def _validate_help(text, command):
    name = "kimi" + (" " + command if command else "")
    if not re.search(r"^Usage: " + re.escape(name) + r"(?:[ \t]+\[|[ \t]*$)", text, re.MULTILINE):
        raise ValueError("Kimi help usage does not identify the requested command")
    if "--help" not in text:
        raise ValueError("Kimi command help is missing its help option")
    if not command and any(word not in text for word in ("--version", "doctor", "acp", "web")):
        raise ValueError("Kimi main help is missing the selected command surface")


def _help(ctx):
    observations = {}
    for label, arguments, command in PROBES:
        record, stdout, stderr = ctx.probe(label, list(arguments), project="a", timeout=30)
        text = _accepted_streams(record, stdout, stderr)
        if command is None:
            if stdout not in (b"2.3.2", b"2.3.2\n", b"2.3.2\r\n"):
                raise ValueError("Kimi executable version differs from the frozen 2.3.2 pin")
        else:
            _validate_help(text, command)
        observations[label] = {"surface": "version" if command is None else "help",
                               "arguments": list(arguments)}
    return {
        "observations": observations,
        "cases": [{"id": "kimi.help", "status": "passed",
                   "reason": "Exact pinned Kimi 2.3.2 version and fixed main/doctor/ACP/web help observed; this help case makes no service or project discovery claim"}],
        "unobserved": ["kimi.scope-observation", "kimi.project-lifecycle",
                       "kimi.native-restart-removal", "kimi.mcp-initialize",
                       "kimi.mcp-tools-list", "kimi.mcp-tools-call"],
    }


PREFIX = "uap-kimi-research-"
SCOPES = ("a", "b", "c", "user")
WORKSPACES = ("a", "b", "c")
TOKEN = re.compile(rb"[A-Za-z0-9_-]{43}\Z")
ENV_KEYS = ("UAP_TEST_EVENTS", "UAP_TEST_NONCE", "UAP_TEST_REVISION", "UAP_TEST_ROOT")


def _identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_nlink,
            info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def _parent(root, path, create=False):
    """Open owned ancestors with directory FDs; aliases cannot redirect I/O."""
    root, path = Path(root), Path(path)
    if not root.is_absolute() or root.resolve() != root or not path.is_absolute():
        raise ValueError("Kimi fixture root is not canonical")
    try:
        relative = path.relative_to(root)
    except ValueError:
        raise ValueError("Kimi fixture path is outside the owned root") from None
    if not relative.parts or any(part in ("", ".", "..") for part in relative.parts):
        raise ValueError("Kimi fixture member is not an owned child")
    flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
    descriptor = os.open(root, flags)
    parents = []
    try:
        current = root
        for part in relative.parts[:-1]:
            info = os.fstat(descriptor)
            if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid():
                raise ValueError("Kimi fixture ancestor is not an owned directory")
            parents.append((current, info.st_dev, info.st_ino))
            if create:
                try:
                    os.mkdir(part, mode=0o700, dir_fd=descriptor)
                except FileExistsError:
                    pass
            child = os.open(part, flags, dir_fd=descriptor)
            os.close(descriptor)
            descriptor = child
            current = current / part
        info = os.fstat(descriptor)
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid():
            raise ValueError("Kimi fixture parent is not an owned directory")
        parents.append((current, info.st_dev, info.st_ino))
        _confirm_parents(parents)
        return descriptor, relative.name, parents
    except BaseException:
        os.close(descriptor)
        raise


def _confirm_parents(parents):
    for path, device, inode in parents:
        info = path.lstat()
        if (not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid()
                or (info.st_dev, info.st_ino) != (device, inode)):
            raise ValueError("Kimi fixture ancestor identity changed")


def _read(root, path, limit=1 << 20, *, mode=None, missing=False):
    parent, name, parents = _parent(root, path)
    try:
        try:
            before = os.stat(name, dir_fd=parent, follow_symlinks=False)
        except FileNotFoundError:
            _confirm_parents(parents)
            if missing:
                return b""
            raise
        if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1
                or before.st_uid != os.getuid() or not 0 <= before.st_size <= limit
                or (mode is not None and stat.S_IMODE(before.st_mode) != mode)):
            raise ValueError("Kimi owned file type, mode or size refused")
        descriptor = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent)
        try:
            if _identity(os.fstat(descriptor)) != _identity(before):
                raise ValueError("Kimi owned file opened identity differs")
            chunks, left = [], before.st_size + 1
            while left:
                chunk = os.read(descriptor, min(left, 65536))
                if not chunk:
                    break
                chunks.append(chunk)
                left -= len(chunk)
            body = b"".join(chunks)
            if len(body) != before.st_size or _identity(os.fstat(descriptor)) != _identity(before):
                raise ValueError("Kimi owned file changed during held read")
        finally:
            os.close(descriptor)
        if _identity(os.stat(name, dir_fd=parent, follow_symlinks=False)) != _identity(before):
            raise ValueError("Kimi owned file path identity changed")
        _confirm_parents(parents)
        return body
    finally:
        os.close(parent)


def _write_new(root, path, body):
    if not isinstance(body, bytes) or len(body) > 65536:
        raise ValueError("Kimi fixture body refused")
    parent, name, parents = _parent(root, path, create=True)
    try:
        descriptor = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                             0o600, dir_fd=parent)
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(body)
            stream.flush()
            os.fsync(stream.fileno())
        _confirm_parents(parents)
    finally:
        os.close(parent)
    if _read(root, path, mode=0o600) != body:
        raise ValueError("Kimi fixture write verification differs")


def _fixtures(ctx):
    probe = Path(ctx.native_probe)
    if not probe.is_absolute() or probe.resolve() != probe:
        raise ValueError("Kimi native probe is not the supplied canonical executable")
    fixtures, protected = {}, {}
    for scope in SCOPES:
        root = ctx.home if scope == "user" else ctx.projects[scope]
        cwd = ctx.projects["user"] if scope == "user" else root
        name = PREFIX + scope
        skill = root / ".kimi-code" / "skills" / name / "SKILL.md"
        description = "Owned Kimi API research marker for " + scope
        body = ("---\nname: " + name + "\ndescription: " + description
                + "\n---\n\nBenign owned " + scope + " marker.\n").encode()
        event = ctx.private / ("kimi-" + scope + "-events.jsonl")
        config = {
            "transport": "stdio", "command": str(probe), "args": [name], "cwd": str(cwd),
            "env": {"UAP_TEST_ROOT": str(ctx.root), "UAP_TEST_EVENTS": str(event),
                    "UAP_TEST_NONCE": secrets.token_hex(24), "UAP_TEST_REVISION": scope},
            "startupTimeoutMs": 3000, "toolTimeoutMs": 3000,
        }
        mcp = root / ".kimi-code" / "mcp.json"
        document = json.dumps({"mcpServers": {name: config}}, sort_keys=True).encode() + b"\n"
        for path, content in ((skill, body), (mcp, document)):
            _write_new(ctx.root, path, content)
            protected[path] = content
        fixtures[scope] = {"name": name, "root": root, "cwd": cwd, "skill": skill,
                           "description": description, "mcp": mcp, "config": config,
                           "event": event, "nonce": config["env"]["UAP_TEST_NONCE"]}
    for path, body in ((ctx.home / ".kimi-code" / "config.toml", b"telemetry = false\n"),
                       (ctx.private / "kimi-foreign-preservation.txt", b"owned foreign bytes\n")):
        _write_new(ctx.root, path, body)
        protected[path] = body
    return fixtures, protected


def _preserved(ctx, protected):
    for path, body in protected.items():
        if _read(ctx.root, path, mode=0o600) != body:
            raise ValueError("Kimi protected fixture bytes changed")


def _json(body):
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError("Kimi API JSON has duplicate keys")
            result[key] = value
        return result

    def invalid(_):
        raise ValueError("Kimi API JSON has a nonfinite number")

    if not isinstance(body, bytes) or not 0 < len(body) <= 1 << 20:
        raise ValueError("Kimi API body size refused")
    try:
        return json.loads(body.decode("utf-8", "strict"), object_pairs_hook=pairs, parse_constant=invalid)
    except (UnicodeError, json.JSONDecodeError, RecursionError):
        raise ValueError("Kimi API response is not whole bounded JSON") from None


def _object(value, required, optional=()):
    if not isinstance(value, dict) or not set(required) <= value.keys() or value.keys() - set(required) - set(optional):
        raise ValueError("Kimi API object shape differs from pinned source")
    return value


def _string(value, limit=4096, empty=False, multiline=False):
    if (not isinstance(value, str) or not (0 if empty else 1) <= len(value) <= limit
            or any(ord(c) < 32 and not (multiline and c in "\r\n\t") for c in value)):
        raise ValueError("Kimi API string shape refused")
    return value


def _instant(value):
    _string(value, 64)
    if not re.fullmatch(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?(?:Z|[+-]\d\d:\d\d)", value):
        raise ValueError("Kimi API timestamp is not ISO datetime")
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        raise ValueError("Kimi API timestamp is invalid") from None


def _envelope(status, body, *, expected_status=200, code=0):
    value = _object(_json(body), ("code", "msg", "data", "request_id"), ("details", "stack"))
    if type(status) is not int or status != expected_status or type(value["code"]) is not int or value["code"] != code:
        raise ValueError("Kimi API status or exact envelope code differs")
    _string(value["request_id"], 256)
    _string(value["msg"], 1024)
    if code == 0:
        if value["msg"] != "success" or value.get("stack") is not None or value.get("details") is not None:
            raise ValueError("Kimi API success envelope differs")
    else:
        if value["data"] is not None:
            raise ValueError("Kimi API negative response contains success data")
        if "stack" in value:
            _string(value["stack"], 32768, empty=True, multiline=True)
    return value["data"]


def _ready(ctx, service):
    registered = set()
    while service.remaining() > 0:
        stdout, stderr = service.snapshot()
        if not all(isinstance(stream, bytes) and len(stream) <= 1 << 20 for stream in (stdout, stderr)):
            raise ValueError("Kimi readiness stream bounds refused")
        # Register every recognizable native token before later parsing can fail.
        for candidate in re.findall(rb"(?:#token=|  Token:    )([A-Za-z0-9_-]{43})", stdout + b"\n" + stderr):
            if candidate not in registered:
                ctx.register_secret(candidate)
                registered.add(candidate)
        try:
            text = codecs.getincrementaldecoder("utf-8")().decode(stdout, final=False)
        except UnicodeError:
            raise ValueError("Kimi readiness is not UTF-8") from None
        lines = [line.rstrip("\r\n") for line in text.splitlines(keepends=True) if line.endswith("\n")]
        local = [line for line in lines if line.startswith("  Local:")]
        tokens = [line for line in lines if line.startswith("  Token:")]
        headers = [line for line in lines if "Kimi server ready" in line]
        if local and tokens and headers:
            if len(local) != 1 or len(tokens) != 1 or len(headers) != 1 or not headers[0].endswith("Kimi server ready  2.3.2"):
                raise ValueError("Kimi readiness banner identity differs")
            match = re.fullmatch(r"  Local:    (http://127\.0\.0\.1:([1-9][0-9]{0,4}))/#token=([A-Za-z0-9_-]{43})", local[0])
            token = re.fullmatch(r"  Token:    ([A-Za-z0-9_-]{43})", tokens[0])
            if not match or not token or int(match[2]) > 65535 or match[3] != token[1]:
                raise ValueError("Kimi readiness loopback/token grammar differs")
            native_path = ctx.home / ".kimi-code" / "server.token"
            if stat.S_IMODE(native_path.parent.lstat().st_mode) != 0o700:
                raise ValueError("Kimi native token parent mode differs")
            body = _read(ctx.root, native_path, 43, mode=0o600)
            if TOKEN.fullmatch(body):
                ctx.register_secret(body)
            if body != token[1].encode("ascii"):
                raise ValueError("Kimi native token differs from readiness")
            service.bind_origin(match[1])
            service.capture_ready()
            return body.decode("ascii")
        time.sleep(min(0.02, max(0, service.remaining())))
    raise ValueError("Kimi native readiness deadline expired")


def _meta(data):
    _object(data, ("server_version", "capabilities", "server_id", "started_at", "open_in_apps",
                   "dangerous_bypass_auth", "backend"), ("experimental_flags", "features", "web_title"))
    if (data["server_version"] != VERSION or data["dangerous_bypass_auth"] is not False
            or data["backend"] != "v2" or data["open_in_apps"] != []):
        raise ValueError("Kimi native server metadata identity differs")
    _string(data["server_id"], 256)
    _instant(data["started_at"])
    keys = ("websocket", "file_upload", "fs_query", "mcp", "tasks", "terminal")
    _object(data["capabilities"], keys)
    if any(value is not True for value in data["capabilities"].values()):
        raise ValueError("Kimi native capabilities differ")
    if "experimental_flags" in data:
        flags = data["experimental_flags"]
        if not isinstance(flags, dict) or len(flags) > 256 or any(not isinstance(k, str) or type(v) is not bool for k, v in flags.items()):
            raise ValueError("Kimi native experimental flag shape differs")
    if "features" in data:
        if not isinstance(data["features"], list) or len(data["features"]) > 256:
            raise ValueError("Kimi native feature count refused")
        for item in data["features"]:
            _object(item, ("name", "state", "meta"))
            _string(item["name"], 256)
            if item["state"] not in ("Pending", "Activating", "Active", "Unloading", "Failed") or not isinstance(item["meta"], dict):
                raise ValueError("Kimi native feature shape differs")
    if "web_title" in data:
        _string(data["web_title"], empty=True)


def _workspace(data, fixture):
    _object(data, ("id", "root", "name", "created_at", "last_opened_at", "session_count"))
    if (not isinstance(data["id"], str) or not re.fullmatch(r"wd_[a-z0-9._-]+_[0-9a-f]{12}", data["id"])
            or len(data["id"]) > 256 or data["root"] != str(fixture["root"])
            or data["name"] != fixture["name"] or type(data["session_count"]) is not int
            or data["session_count"] != 0):
        raise ValueError("Kimi workspace root/name/identity differs")
    _instant(data["created_at"])
    _instant(data["last_opened_at"])
    return data["id"]


def _trust(data, expected):
    _object(data, ("trusted",))
    if data["trusted"] is not expected:
        raise ValueError("Kimi native workspace trust state differs")


def _skills(data, scope, fixtures):
    _object(data, ("skills",))
    rows = data["skills"]
    if not isinstance(rows, list) or len(rows) > 512:
        raise ValueError("Kimi skill catalog count refused")
    found = {}
    for row in rows:
        _object(row, ("name", "description", "path", "source"), ("type", "disable_model_invocation", "scopes"))
        for key in ("name", "description", "path"):
            _string(row[key], empty=key == "description", multiline=key == "description")
        if row["source"] not in ("project", "user", "extra", "builtin"):
            raise ValueError("Kimi skill source differs")
        if "type" in row:
            _string(row["type"], empty=True)
        if "disable_model_invocation" in row and type(row["disable_model_invocation"]) is not bool:
            raise ValueError("Kimi skill invocation flag differs")
        if "scopes" in row and (not isinstance(row["scopes"], list) or len(row["scopes"]) > 2 or any(v not in ("tui", "web") for v in row["scopes"])):
            raise ValueError("Kimi skill scope shape differs")
        if row["name"].casefold().startswith(PREFIX):
            if row["name"] in found:
                raise ValueError("Kimi research skill is duplicated")
            found[row["name"]] = row
    expected = {fixtures[key]["name"]: key for key in (scope, "user")}
    if found.keys() != expected.keys():
        raise ValueError("Kimi skill catalog crosses project or user isolation")
    for name, key in expected.items():
        row, fixture = found[name], fixtures[key]
        if (row["path"] != str(fixture["skill"]) or row["description"] != fixture["description"]
                or row["source"] != ("user" if key == "user" else "project")):
            raise ValueError("Kimi research skill origin or marker differs")
    return sorted(found)


def _catalog(data, scope, fixtures):
    if not isinstance(data, list) or len(data) > 256:
        raise ValueError("Kimi MCP catalog count refused")
    found = {}
    for row in data:
        _object(row, ("name", "config", "source", "origin", "mutable"), ("plugin",))
        _string(row["name"])
        _string(row["origin"])
        if not isinstance(row["config"], dict) or row["source"] not in ("global", "plugin", "caller") or type(row["mutable"]) is not bool:
            raise ValueError("Kimi MCP managed entry shape differs")
        if "plugin" in row:
            _object(row["plugin"], ("id", "name"))
            _string(row["plugin"]["id"])
            _string(row["plugin"]["name"])
        if row["name"].casefold().startswith(PREFIX):
            if row["name"] in found:
                raise ValueError("Kimi research MCP entry is duplicated")
            found[row["name"]] = row
    scopes = (scope, "user") if scope in ("a", "b") else ("user",)
    expected = {fixtures[key]["name"]: key for key in scopes}
    if found.keys() != expected.keys():
        raise ValueError("Kimi MCP catalog crosses trusted project or user isolation")
    for name, key in expected.items():
        row, fixture = found[name], fixtures[key]
        config = dict(fixture["config"])
        if key != "user":
            del config["env"]
            config["envKeys"] = sorted(ENV_KEYS)
        if (row["source"] != "global" or row["origin"] != str(fixture["mcp"])
                or row["mutable"] is not (key == "user") or "plugin" in row
                or row["config"] != config):
            raise ValueError("Kimi research MCP origin/config differs")
    return sorted(found)


def _events(ctx, fixture):
    body = _read(ctx.root, fixture["event"], 2 << 20, mode=0o600, missing=True)
    if body and not body.endswith(b"\n"):
        raise ValueError("Kimi probe event stream is incomplete")
    lines = body.splitlines()
    if len(lines) > 64:
        raise ValueError("Kimi probe event count refused")
    rows = []
    for line in lines:
        row = _object(_json(line), ("method", "pid", "session", "nonce", "timestamp", "facts"))
        if (row["method"] not in ("initialize", "tools/list") or type(row["pid"]) is not int
                or row["pid"] <= 0 or row["nonce"] != fixture["nonce"] or row["facts"] is not None
                or not isinstance(row["session"], str)
                or not re.fullmatch(str(row["pid"]) + r"-[0-9]{15,20}", row["session"])):
            raise ValueError("Kimi probe method/nonce/process/session differs")
        _instant(row["timestamp"])
        rows.append(row)
    return body, rows


def _observe(ctx):
    def stage(label):
        # Only fixed local stage labels survive the shared service's safe errors.
        if isinstance(getattr(ctx, "summary", None), dict):
            ctx.summary["collector_stage"] = label

    stage("kimi-fixtures")
    fixtures, protected = _fixtures(ctx)

    def observe(service):
        stage("kimi-readiness")
        token = _ready(ctx, service)
        calls = 0

        def request(label, method, path, body=None, *, auth=True, status=200, code=0):
            nonlocal calls
            calls += 1
            if calls > 36 or service.remaining() <= 0:
                raise ValueError("Kimi fixed API request budget exhausted")
            stage(label)
            return _envelope(*service.http(label, method, path, token=token if auth else None,
                                          json_body=body, timeout=min(5, service.remaining())),
                             expected_status=status, code=code)

        def catalog(scope, suffix):
            path = "/api/v2/mcp/servers?" + urlencode({"cwd": str(ctx.projects[scope])})
            return _catalog(request("kimi-mcp-" + scope + suffix, "GET", path), scope, fixtures)

        health = request("kimi-health", "GET", "/api/v1/healthz", auth=False)
        if _object(health, ("ok",))["ok"] is not True:
            raise ValueError("Kimi native health response differs")
        request("kimi-meta-unauthorized", "GET", "/api/v1/meta", auth=False, status=401, code=40101)
        meta = request("kimi-meta", "GET", "/api/v1/meta")
        _meta(meta)
        ids = {}
        for scope in WORKSPACES:
            fixture = fixtures[scope]
            data = request("kimi-workspace-" + scope, "POST", "/api/v1/workspaces",
                           {"root": str(fixture["root"]), "name": fixture["name"]})
            ids[scope] = _workspace(data, fixture)
        if len(set(ids.values())) != 3:
            raise ValueError("Kimi native workspace identities are not distinct")
        for scope in WORKSPACES:
            _trust(request("kimi-trust-initial-" + scope, "GET", "/api/v1/workspaces/" + ids[scope] + "/trust"), False)
        for scope in ("a", "b"):
            path = "/api/v1/workspaces/" + ids[scope] + "/trust"
            _trust(request("kimi-trust-owned-" + scope, "POST", path, {}), True)
            _trust(request("kimi-trust-confirm-" + scope, "GET", path), True)
        _trust(request("kimi-trust-confirm-c", "GET", "/api/v1/workspaces/" + ids["c"] + "/trust"), False)
        # Native approval of only A/B precedes the preservation baseline.
        _preserved(ctx, protected)
        baseline = {str(path.relative_to(ctx.root)): hashlib.sha256(body).hexdigest() for path, body in protected.items()}
        skill_names, catalogs = {}, {}
        for scope in WORKSPACES:
            skill_names[scope] = _skills(request("kimi-skills-" + scope, "GET", "/api/v1/workspaces/" + ids[scope] + "/skills"), scope, fixtures)
        for scope in (*WORKSPACES, "user"):
            catalogs[scope] = catalog(scope, "")
        probes, sessions = {}, set()
        for scope in ("a", "b"):
            fixture = fixtures[scope]
            before, _ = _events(ctx, fixture)
            started = datetime.now(timezone.utc)
            result = request("kimi-test-" + scope, "POST", "/api/v2/mcp/servers:test",
                             {"name": fixture["name"], "cwd": str(fixture["cwd"])})
            ended = datetime.now(timezone.utc)
            _object(result, ("success", "output"))
            expected = ('Connected to MCP server "' + fixture["name"] + '".\nAvailable tools: 1\n'
                        '- inspect_runtime: Inspect only disposable fixture runtime facts.')
            if result["success"] is not True or result["output"] != expected:
                raise ValueError("Kimi native named MCP test did not enumerate the supplied probe")
            after, rows = _events(ctx, fixture)
            if not after.startswith(before):
                raise ValueError("Kimi probe events were replaced during named test")
            fresh = rows[len(before.splitlines()):]
            pairs = {(row["pid"], row["session"]) for row in fresh}
            if (not fresh or {row["method"] for row in fresh} != {"initialize", "tools/list"}
                    or fresh[0]["method"] != "initialize"
                    or sum(row["method"] == "initialize" for row in fresh) != 1
                    or len(pairs) != 1 or pairs & sessions
                    or any(not started <= _instant(row["timestamp"]) <= ended for row in fresh)):
                raise ValueError("Kimi named-test request window lacks fresh correlated probe events")
            sessions.update(pairs)
            probes[scope] = fresh
        before_c, _ = _events(ctx, fixtures["c"])
        if before_c:
            raise ValueError("Kimi untrusted C probe ran before the negative test")
        request("kimi-test-untrusted-c", "POST", "/api/v2/mcp/servers:test",
                {"name": fixtures["c"]["name"], "cwd": str(fixtures["c"]["cwd"])}, code=40408)
        if _events(ctx, fixtures["c"])[0] or _events(ctx, fixtures["user"])[0]:
            raise ValueError("Kimi untrusted or unselected user probe unexpectedly ran")
        for scope in WORKSPACES:
            if _skills(request("kimi-skills-" + scope + "-repeat", "GET", "/api/v1/workspaces/" + ids[scope] + "/skills"), scope, fixtures) != skill_names[scope]:
                raise ValueError("Kimi repeated skill catalog differs")
            if catalog(scope, "-repeat") != catalogs[scope]:
                raise ValueError("Kimi repeated MCP catalog differs")
        for scope in SCOPES:
            body, _ = _events(ctx, fixtures[scope])
            if scope in ("c", "user") and body:
                raise ValueError("Kimi untrusted or unselected probe ran during repeated catalogs")
        _preserved(ctx, protected)
        shutdown = request("kimi-shutdown", "POST", "/api/v1/shutdown", {})
        if _object(shutdown, ("ok",))["ok"] is not True:
            raise ValueError("Kimi native shutdown response differs")
        service.mark_shutdown()
        return {"server_version": meta["server_version"], "backend": meta["backend"],
                "workspace_ids": ids, "native_trust_setup": {"a": True, "b": True, "c": False},
                "skill_markers": skill_names, "mcp_markers": catalogs, "probe_events": probes,
                "protected_sha256": baseline, "api_request_count": calls,
                "tools_call_count": 0, "provider_configured": False}

    result = ctx.service("kimi-web-api", ["web", "--no-open", "--host", "127.0.0.1", "--port", "0"],
                         observe, project="a", timeout=60)
    _preserved(ctx, protected)
    for scope in SCOPES:
        body, _ = _events(ctx, fixtures[scope])
        if scope in ("c", "user") and body:
            raise ValueError("Kimi untrusted or unselected probe ran during native shutdown")
    stage("kimi-api-complete")
    return result


def collect(ctx):
    """Use only the explicitly selected reviewed phase; never imply runtime use."""
    if ctx.phase not in ("help", "observe"):
        raise ValueError("Kimi collector requires an explicit help or observe phase")
    result = _help(ctx)
    if ctx.phase == "observe":
        result["observations"]["kimi-web-api"] = _observe(ctx)
        result["cases"].append({"id": "kimi.scope-observation", "status": "passed",
                                "reason": "Pinned native API admitted its generated bearer, exact owned A/B trust, separate A/B/C/user skill and MCP catalogs, and named A/B MCP initialize/tools/list with fresh correlated probe events; C refused; no product lifecycle or tool call qualified"})
        result["unobserved"].remove("kimi.scope-observation")
        result["unobserved"].remove("kimi.mcp-initialize")
        result["unobserved"].remove("kimi.mcp-tools-list")
    return result
