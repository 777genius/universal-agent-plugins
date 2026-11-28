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

# Exact package-lock.json in the signed Playwright 0.0.80 package, revision
# d9c8d778200d51ae941d86bb69af23534cd1c8f7, io.github.777genius.agentplugins/runtime.
# npm ci creates these three relative bin links before any native observation.
PLAYWRIGHT_LOCK_SHA256 = "1c10c6b0e0c7009fdc88fff6d4e295293d1f4f99c4b38bb7412a37504bf4ab7c"
PLAYWRIGHT_BIN_LINKS = {"playwright": "../playwright/cli.js",
                        "playwright-core": "../playwright-core/cli.js",
                        "playwright-mcp": "../@playwright/mcp/cli.js"}


def runtime_document(audit, case, path):
    body, error = audit["copilot_settings_read"](path, case)
    require(error is None and type(body) is str)
    return decode(body.encode()), body


def owned_playwright_data(audit, case, anchor, scope, physical, pins):
    """Bind retained data to its signed installation and existing owner receipt."""
    state, _ = runtime_document(audit, case, anchor / "state-v2.json")
    require(type(state) is dict and type(state.get("schema_version")) is int
            and state["schema_version"] == 5 and type(state.get("installations")) is list)
    distribution = "777genius/playwright-bridge"
    pin = pins[distribution]
    rows = [row for row in state["installations"] if type(row) is dict
            and type(row.get("directory")) is dict
            and row["directory"].get("distribution_id") == distribution]
    require(len(rows) == 1)
    row = rows[0]
    require(row.get("origin_mode") == "directory" and row.get("declared_name") == "playwright"
            and type(row["directory"].get("desired_release_sequence")) is int
            and row["directory"].get("desired_release_sequence") == pin["release_sequence"]
            and row.get("package", {}).get("version") == pin["package_version"]
            and row["package"].get("manifest_digest") == pin["manifest_digest"]
            and all(row.get("source", {}).get(key) == pin[field] for key, field in (
                ("repository", "repository"), ("package_subpath", "package_path"),
                ("resolved_revision", "resolved_revision"), ("tree_digest", "tree_digest"))))
    require(type(row.get("installation_id")) is str and bool(row["installation_id"]))
    receipt_id = "data_" + hashlib.sha256(
        "\0".join((row["installation_id"], physical, scope)).encode()).hexdigest()[:24]
    data = anchor / "plugin-data" / physical
    expected = {"data_receipt_id": receipt_id, "physical_backend_id": physical,
                "scope": scope, "locator": str(data), "state": "owned",
                "ownership_digest": "sha256:" + hashlib.sha256(
                    ("agentplugins-plugin-data-v1\0" + receipt_id + "\0" + str(data)).encode()).hexdigest()}
    receipt = row.get("data_receipts", {}).get(receipt_id)
    marker, _ = runtime_document(audit, case, data / ".agentplugins-data-owner.json")
    require(type(receipt) is dict and type(marker) is dict
            and all(receipt.get(key) == marker.get(key) == value for key, value in expected.items()))
    clients = row.get("clients")
    require(type(clients) is dict and (
        not clients and row.get("data_retained") is True
        or any(type(binding) is dict and binding.get("client_id") == "claude"
               and binding.get("scope") == scope and binding.get("physical_artifact_id") == physical
               and binding.get("data_receipt_id") == receipt_id for binding in clients.values())))
    return data


def owned_runtime_link_entries(audit, env, project, case, complete, pins):
    """Admit finite owned npm links; keep every entry in the protected snapshot."""
    allowed, checked = set(), set()
    roots = (Path(env["HOME"]), Path(env["AGENTPLUGINS_HOME"]), project)
    for key, value in complete.items():
        if not key.startswith("link:"):
            continue
        _, index, relative = key.split(":", 2)
        if index not in ("1", "2"):
            continue
        index = int(index)
        parts = Path(relative).parts
        prefix = (".uap", "plugin-data") if index == 2 else ("plugin-data",)
        tail = parts[len(prefix):]
        if (parts[:len(prefix)] != prefix or len(tail) != 6
                or tail[1:5] != ("npm-runtime", PLAYWRIGHT_LOCK_SHA256, "node_modules", ".bin")
                or tail[5] not in PLAYWRIGHT_BIN_LINKS
                or value != PLAYWRIGHT_BIN_LINKS[tail[5]]):
            continue
        physical = tail[0]
        require(physical not in ("", ".", ".."))
        anchor = project / ".uap" if index == 2 else roots[1]
        runtime = anchor / "plugin-data" / physical / "npm-runtime" / PLAYWRIGHT_LOCK_SHA256
        link = roots[index] / relative
        # No candidate path is resolved through its link. The shared reader
        # walks ordinary ancestors through descriptors and opens without follow.
        require(audit["path_inside_case"](link.parent, case)
                and stat.S_ISDIR(link.parent.lstat().st_mode))
        if runtime not in checked:
            owned_playwright_data(audit, case, anchor, "project" if index == 2 else "user", physical, pins)
            _, body = runtime_document(audit, case, runtime / "package-lock.json")
            require(hashlib.sha256(body.encode()).hexdigest() == PLAYWRIGHT_LOCK_SHA256)
            require({entry.name for entry in link.parent.iterdir()} == set(PLAYWRIGHT_BIN_LINKS))
            checked.add(runtime)
        target = runtime / "node_modules" / value.removeprefix("../")
        _, error = audit["copilot_settings_read"](target, case)
        identity = link.lstat()
        require(error is None and stat.S_ISLNK(identity.st_mode) and os.readlink(link) == value
                and complete.get(f"metadata:{index}:{relative}")
                    == (identity.st_mode, identity.st_dev, identity.st_ino))
        entry = f"{roots[index].name}/{relative}"
        require(complete.get(entry) == "symlink")
        allowed.update((key, entry))
    return allowed


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
            timeout=60, memory_only=True, input_bytes=input_bytes, umask=0o077)
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

    def observe(self, env, project, label, *, initialize=None):
        version, _ = self.capture(env, project, ("--version",), label + "-version")
        require(version.strip() == b"2.1.263 (Claude Code)")
        listing, listing_record = self.capture(env, project, ("plugin", "list", "--json"), label + "-registration")
        rows = registrations(listing)
        def capture_initialize():
            return self.capture(env, project, INITIALIZE_ARGS, label + "-skills", INITIALIZE)
        if initialize is None:
            initialized, initialized_record = capture_initialize()
        else:
            initialized, initialized_record = initialize(capture_initialize, rows)
            require(type(initialized_record) is dict and initialized_record.get("exit") == 0
                    and initialized_record.get("error") is None
                    and initialized_record.get("stderr_bytes") == 0)
        return rows, commands(initialized), [listing_record, initialized_record]

    def before_remove(self, env, project, scope, label, *, initialize=None):
        options = {"initialize": initialize} if initialize is not None else {}
        rows, skills, records = self.observe(env, project, label + "-before", **options)
        selected = [row for row in rows if owned(row["id"])]
        require(len(selected) == 4 and {row["id"].split("@", 1)[0] for row in selected}
                == {"marketing-kit", "notion", "hubspot-crm", "playwright"}
                and all(row["scope"] == scope for row in selected)
                and "marketing-kit:campaign-planning" in skills
                and skills["marketing-kit:campaign-planning"].get("builtin") is not True)
        return {"home": env["HOME"], "project": str(project), "scope": scope,
                "native_rows": rows,
                "foreign_registrations": [row for row in rows if not owned(row["id"])],
                "foreign_skills": {key: value for key, value in skills.items() if not owned(key)},
                "records": records}

    def after_remove(self, env, project, before, label, *, initialize=None):
        require(before["home"] == env["HOME"] and before["project"] == str(project))
        options = {"initialize": initialize} if initialize is not None else {}
        rows, skills, records = self.observe(env, project, label + "-after", **options)
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
                "home": env["HOME"], "project": str(project), "native_rows": rows,
                "plugin_registration_absence_qualified": True, "skill_discovery_absence_qualified": True,
                "foreign_native_inventory_preserved": True, "mcp_discovery_absence_qualified": False,
                "mcp_discovery_boundary": "Skill initialize uses an empty strict MCP configuration; no effective MCP absence claim",
                "physical_absence_qualified": False, "writer_closure_qualified": False,
                "full_native_removal_qualified": False,
                "native_executable_sha256": self.executable_identity[1],
                "observations": before["records"] + records}
