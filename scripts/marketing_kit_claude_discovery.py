"""Fresh Claude registration/Skill observations; no MCP absence or writer claim.

The initialize protocol and exact response validation follow
repotests/agentplugins_claude_project_initialize_test.go. Its empty strict MCP
configuration isolates Skill discovery and explicitly does not prove MCP absence.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
from runpy import run_path
import stat

_PROBE = run_path(str(Path(__file__).with_name("native_client_research.py")))
ALIASES = frozenset(("marketing-kit", "notion", "hubspot-crm", "playwright", "playwright-bridge"))
INITIALIZE = b'{"type":"control_request","request_id":"native-discovery","request":{"subtype":"initialize"}}\n'
INITIALIZE_ARGS = ("-p", "--input-format", "stream-json", "--output-format", "stream-json",
                  "--verbose", "--permission-mode", "dontAsk", "--strict-mcp-config",
                  "--mcp-config", '{"mcpServers":{}}')


def require(value):
    if not value:
        raise ValueError("fresh Claude removal discovery is unresolved")


def unique(pairs):
    result = {}
    for name, value in pairs:
        require(name not in result)
        result[name] = value
    return result


def decode(body):
    require(type(body) is bytes and 0 < len(body) <= 1 << 20)
    return json.loads(body.decode("utf-8"), object_pairs_hook=unique,
                      parse_constant=lambda _: require(False))


def registrations(body):
    rows = decode(body)
    require(type(rows) is list)
    seen = set()
    for row in rows:
        require(type(row) is dict and type(row.get("id")) is str and "@" in row["id"]
                and type(row.get("scope")) is str and row["scope"] in ("user", "project", "local")
                and type(row.get("enabled")) is bool and type(row.get("installPath")) is str
                and Path(row["installPath"]).is_absolute())
        key = (row["id"], row["scope"], row["installPath"])
        require(key not in seen)
        seen.add(key)
    return rows


def commands(body):
    message = decode(body)
    require(type(message) is dict and message.get("type") == "control_response")
    response = message.get("response")
    require(type(response) is dict and response.get("request_id") == "native-discovery"
            and response.get("subtype") == "success")
    result = response.get("response")
    require(type(result) is dict and type(result.get("account")) is dict
            and result["account"].get("tokenSource") == "none"
            and type(result.get("commands")) is list)
    found = {}
    for command in result["commands"]:
        require(type(command) is dict and type(command.get("name")) is str
                and command["name"] and command["name"] not in found)
        found[command["name"]] = command
    return found


def owned(name):
    return name.split("@", 1)[0].split(":", 1)[0].casefold() in ALIASES


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)


class RemovalDiscovery:
    """Bind fresh native processes to the prebound executable and disposable case."""

    def __init__(self, executable, case, evidence=None):
        self.executable, self.case = Path(executable), Path(case)
        self.evidence = evidence if evidence is not None else []
        self.mcp_observed = False
        require(self.executable.is_absolute() and self.case.resolve(strict=True) == self.case)
        self.executable_identity = self.binary_identity()

    def binary_identity(self):
        before = self.executable.lstat()
        # This is the trusted, prebound npm client executable, which may be
        # a native binary rather than cli.js. Hash it as a stream; package-tree
        # snapshot limits are not executable-size policy.
        require(stat.S_ISREG(before.st_mode) and before.st_size > 0
                and self.executable.resolve(strict=True) == self.executable)
        with self.executable.open("rb") as stream:
            held = os.fstat(stream.fileno())
            require(os.path.samestat(before, held))
            digest = hashlib.file_digest(stream, "sha256").hexdigest()
            after = os.fstat(stream.fileno())
        fields = lambda info: (info.st_mode, info.st_dev, info.st_ino, info.st_size,
                               info.st_mtime_ns, info.st_ctime_ns)
        require(fields(before) == fields(after) == fields(self.executable.lstat()))
        return fields(before), digest

    def capture(self, env, project, args, label, input_bytes=None, observational=False):
        home, project = Path(env["HOME"]), Path(project)
        profile = home / ".claude"
        require(home.resolve(strict=True) == home and project.resolve(strict=True) == project
                and home.is_relative_to(self.case) and project.is_relative_to(self.case)
                and env.get("CLAUDE_CONFIG_DIR") == str(profile)
                and profile.resolve(strict=True) == profile and stat.S_ISDIR(profile.lstat().st_mode)
                and self.binary_identity() == self.executable_identity)
        # Match the production adapter's bounded credential-free environment.
        allowed = {"HOME", "USER", "LOGNAME", "USERPROFILE", "PATH", "TMPDIR", "TMP",
                   "TEMP", "LANG", "SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT", "APPDATA", "LOCALAPPDATA"}
        native_env = {key: value for key, value in env.items() if key in allowed or key.startswith("LC_")}
        native_env.update(CLAUDE_CONFIG_DIR=str(home / ".claude"),
                          AGENTPLUGINS_CLAUDE_PROFILE=str(home / ".claude"),
                          DISABLE_AUTOUPDATER="1", CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1")
        require(all(len(value) <= 4096 and "\0" not in value for value in native_env.values())
                and sum(len(key) + len(value) + 2 for key, value in native_env.items()) <= 32768)
        record, stdout, stderr = _PROBE["run_probe"](
            [str(self.executable), *args], project, native_env, self.case, label,
            timeout=60, memory_only=True, input_bytes=input_bytes)
        evidence = {key: record[key] for key in (
            "probe", "exit", "error", "stdout_bytes", "stderr_bytes", "stdout_sha256", "stderr_sha256")}
        evidence.update(stdout_base64=base64.b64encode(stdout).decode("ascii"),
                        stderr_base64=base64.b64encode(stderr).decode("ascii"),
                        native_absence_qualified=False)
        self.evidence.append(evidence)
        require(self.binary_identity() == self.executable_identity)
        if not observational:
            require(record.get("exit") == 0 and record.get("error") is None and stderr == b"")
        # Qualification uses held buffers; exported evidence never becomes input.
        return stdout, {key: record[key] for key in (
            "probe", "exit", "error", "stdout_bytes", "stderr_bytes", "stdout_sha256", "stderr_sha256")}

    def observe(self, env, project, label):
        version, _ = self.capture(env, project, ("--version",), label + "-version")
        require(version.strip() == b"2.1.263 (Claude Code)")
        listing, listing_record = self.capture(env, project, ("plugin", "list", "--json"), label + "-registration")
        initialized, initialized_record = self.capture(env, project, INITIALIZE_ARGS,
            label + "-skills", INITIALIZE)
        return registrations(listing), commands(initialized), [listing_record, initialized_record]

    def before_remove(self, env, project, scope, label):
        rows, skills, records = self.observe(env, project, label + "-before")
        selected = [row for row in rows if owned(row["id"])]
        require(len(selected) == 4 and {row["id"].split("@", 1)[0] for row in selected}
                == {"marketing-kit", "notion", "hubspot-crm", "playwright"}
                and all(row["scope"] == scope for row in selected)
                and "marketing-kit:campaign-planning" in skills
                and skills["marketing-kit:campaign-planning"].get("builtin") is not True)
        return {"home": env["HOME"], "project": str(project), "scope": scope,
                "foreign_registrations": [row for row in rows if not owned(row["id"])],
                "foreign_skills": {key: value for key, value in skills.items() if not owned(key)},
                "records": records}

    def after_remove(self, env, project, before, label):
        require(before["home"] == env["HOME"] and before["project"] == str(project))
        rows, skills, records = self.observe(env, project, label + "-after")
        # The existing repo mcp-list helper discards the exit error and only
        # checks positive transport substrings. No exact absence parser exists.
        # Retain the genuine effective-discovery output for hosted review;
        # neither exit zero nor an empty stream qualifies absence.
        if not self.mcp_observed:
            self.capture(env, project, ("mcp", "list"), label + "-mcp-observational", observational=True)
            self.mcp_observed = True
        require(not any(owned(row["id"]) for row in rows) and not any(owned(name) for name in skills)
                and canonical(rows) == canonical(before["foreign_registrations"])
                and canonical(skills) == canonical(before["foreign_skills"]))
        return {"status": "verified-registration-and-skill-absence", "scope": before["scope"],
                "plugin_registration_absence_qualified": True, "skill_discovery_absence_qualified": True,
                "foreign_native_inventory_preserved": True, "mcp_discovery_absence_qualified": False,
                "mcp_discovery_boundary": "Skill initialize uses an empty strict MCP configuration; no effective MCP absence claim",
                "physical_absence_qualified": False, "writer_closure_qualified": False,
                "full_native_removal_qualified": False,
                "native_executable_sha256": self.executable_identity[1],
                "observations": before["records"] + records}
