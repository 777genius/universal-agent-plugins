"""Independent Pi User component discovery; never authenticates or calls a tool."""
import base64
import json
from pathlib import Path
import re
import uuid

ACCEPTANCE = "pi-plugin-native-registration"


def require(value):
    if not value:
        raise ValueError("Pi plugin native registration proof is unresolved")


class PiPluginCatalogAudit:
    """Observe the original disposable profile with the pinned native CLI."""

    def __init__(self, home, project, data, env, client_version, api, *, source, index):
        require(client_version == {"status": "reported", "value": "1.5.0", "exit_code": 0}
                and type(client_version.get("exit_code")) is int)
        require(type(source) is str and source and type(index) is int and 0 <= index < 30)
        self.home, self.project, self.data = map(Path, (home, project, data))
        self.case, self.api = self.project.parent, api
        self.profile = self.home / ".pi/agent"
        require(self.home.parent == self.data.parent == self.case
                and env.get("HOME") == str(self.home)
                and env.get("PI_CODING_AGENT_DIR") == str(self.profile))
        require(api["path_inside_case"](self.profile, self.case))
        self.profile.mkdir(parents=True, exist_ok=True)
        require(all(api["path_inside_case"](path, self.case) and path.is_dir()
                    and not path.is_symlink() for path in (self.profile, self.project, self.data)))
        self.roots = (self.home / ".pi", self.project, self.data)
        self.env = {key: value for key, value in env.items() if key in (
            "PATH", "HOME", "USERPROFILE", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL",
            "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME")}
        self.env.update(PI_CODING_AGENT_DIR=str(self.profile), PI_OFFLINE="1")
        self.binary = api["AGENT_BINARIES"].get("pi")
        require(type(self.binary) is str and Path(self.binary).is_absolute())
        self.expected = {"skill": {}, "mcp_server": {}}
        self.installed_bytes = {}
        self.evidence = {"schema_version": 1, "status": "unqualified",
                         "source": source, "index": index, "case_root": str(self.case.resolve(strict=True)),
                         "qualification": "native-registration-only", "native_ready": False,
                         "vendor_authentication": False, "model_calls": False,
                         "tools_call_qualified": False, "checkpoints": {}}

    def checked(self, method, *args):
        require(method in ("select", "checkpoint"))
        try:
            return getattr(self, method)(*args)
        except (OSError, ValueError, KeyError, TypeError, AttributeError) as error:
            self.evidence["refusal"] = {"stage": method, "type": type(error).__name__}
            error.pi_plugin_registration_proof = self.evidence
            raise

    def read(self, path):
        text, error = self.api["copilot_settings_read"](path, self.case)
        require(error is None and type(text) is str)
        return text

    def document(self, text):
        return json.loads(text, object_pairs_hook=self.api["_unique_json_object"],
                          parse_constant=lambda _: require(False))

    def _probe(self, phase, kind, argv, payload=None):
        before = self.api["scoped_snapshot"](self.roots)
        def complete(body):
            value = self.document(body.decode())
            require(value.get("id") == json.loads(payload)["id"]
                    and value.get("command") == "get_commands" and value.get("success") is True)
            return True
        options = ({"input_bytes": payload, "input_complete": complete} if payload else {})
        record, stdout, stderr = self.api["COPILOT_CATALOG_PROBE"](
            argv, self.project, self.env, self.case, "pi-plugin-" + phase + "-" + kind,
            timeout=45, limit=1 << 20, memory_only=True, **options)
        capture = {**record, "stdout_base64": base64.b64encode(stdout).decode(),
                   "stderr_base64": base64.b64encode(stderr).decode()}
        if payload:
            capture["stdin_base64"] = base64.b64encode(payload).decode()
        self.evidence["checkpoints"].setdefault(phase, {})[kind] = capture
        require(record.get("error") is None and type(record.get("exit")) is int
                and stderr == b"" and 0 < len(stdout) <= 1 << 20)
        if payload:
            require(record["exit"] == 0 and record.get("stdin_written") is True
                    and record.get("stdin_response_confirmed") is True
                    and len(stdout.splitlines()) == 1 and stdout.endswith(b"\n"))
        after = self.api["scoped_snapshot"](self.roots)
        # Official Pi 1.5.0 creates these empty stores on first native use.
        # Existing stores, configuration, receipts and all other bytes stay exact.
        created = []
        for name in ("auth.json", "models-store.json", "mcp-auth.json"):
            key, metadata = ".pi/agent/" + name, "metadata:0:agent/" + name
            if key not in before and key in after:
                require(self.read(self.profile / name) == "{}")
                after.pop(key)
                after.pop(metadata)
                created.append(name)
        require(before == after)
        capture["empty_native_stores_created"] = created
        return self.document(stdout.decode()), record["exit"]

    def select(self, added):
        require(not self.installed_bytes and set(self.evidence["checkpoints"]) == {"before-add"})
        actions = added["data"]["actions"]
        require(type(actions) is list and actions)
        names = {"skill": set(), "mcp_server": set()}
        for action in actions:
            require(action.get("client_id") == "pi")
            for component in action["component_support"]:
                if component.get("support") != "native":
                    continue
                kind, name = component.get("kind"), component.get("name")
                require(kind in names and type(name) is str
                        and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,127}", name))
                require(name not in names[kind])
                names[kind].add(name)
        require(any(names.values()))
        for name in names["skill"]:
            path = self.profile / "skills" / name / "SKILL.md"
            self.installed_bytes[str(path)] = self.read(path)
            self.expected["skill"][name] = str(path)
        config = self.profile / "mcp.json"
        servers = self.document(self.read(config)).get("mcpServers") if config.exists() else {}
        require(type(servers) is dict and set(servers) == names["mcp_server"])
        for name, server in servers.items():
            require(type(server) is dict)
            transport = server.get("url")
            if transport is None:
                require(type(server.get("command")) is str and type(server.get("args", [])) is list
                        and all(type(arg) is str for arg in server.get("args", [])))
                transport = " ".join([server["command"], *server.get("args", [])])
            require(type(transport) is str and transport)
            self.expected["mcp_server"][name] = {"name": name, "scope": "global",
                "source": str(config), "enabled": server.get("enabled") is not False,
                "exposure": server.get("exposure", "codemode"), "transport": transport}
        if servers:
            self.installed_bytes[str(config)] = self.read(config)
        self.evidence["expected_components"] = {
            kind: sorted(values) for kind, values in self.expected.items()}

    def checkpoint(self, phase):
        order = ("before-add", "after-add", "repeat-add", "after-remove")
        require(phase in order and list(self.evidence["checkpoints"]) == list(order[:order.index(phase)]))
        present = phase in ("after-add", "repeat-add")
        for path, original in self.installed_bytes.items():
            if present:
                require(self.read(Path(path)) == original)
        expected = self.expected if present else {"skill": {}, "mcp_server": {}}
        nonce = "pi-plugins-" + uuid.uuid4().hex
        payload = (json.dumps({"id": nonce, "type": "get_commands"}) + "\n").encode()
        value, _ = self._probe(phase, "skills", [self.binary, "--mode", "rpc", "--no-session", "--offline"], payload)
        require(value.get("type") == "response" and value.get("id") == nonce)
        commands = value["data"]["commands"]
        require(type(commands) is list and len(commands) <= 512
                and all(type(row) is dict for row in commands))
        skills = [row for row in commands if row.get("source") == "skill"]
        require(len(skills) == len(expected["skill"]))
        seen = set()
        for row in skills:
            name = row.get("name", "").removeprefix("skill:")
            require(name in expected["skill"] and name not in seen and row["name"] == "skill:" + name)
            require(row.get("sourceInfo") == {"path": expected["skill"][name], "source": "auto",
                    "scope": "user", "origin": "top-level", "baseDir": str(self.profile)})
            seen.add(name)
        value, exit_code = self._probe(phase, "mcp", [self.binary, "mcp", "list", "--json"])
        require(type(value) is dict and value.get("errors") == [] and "note" not in value)
        rows = value.get("servers")
        require(type(rows) is list and len(rows) == len(expected["mcp_server"])
                and all(type(row) is dict for row in rows))
        seen, failed = set(), False
        for row in rows:
            name = row.get("name")
            require(name in expected["mcp_server"] and name not in seen
                    and all(row.get(key) == item for key, item in expected["mcp_server"][name].items()))
            require(row.get("state") in ("connected", "failed", "needs-auth", "disabled", "disconnected")
                    and type(row.get("tools")) is list and "override" not in row)
            failed = failed or (row["enabled"] and row["state"] != "connected")
            seen.add(name)
        require(exit_code == (1 if failed else 0))
        self.evidence["checkpoints"][phase]["registration_verified"] = True
        if phase == "after-remove":
            self.evidence["status"] = "native-registration-lifecycle-verified"

    def verified(self):
        return self.evidence["status"] == "native-registration-lifecycle-verified"


def summary(report, fixtures, negative):
    """Require the complete pinned catalog; connection failures remain non-PASS."""
    result = {"pi_native_registration_passed": 0, "pi_native_registration_negative": 0,
              "pi_native_registration_qualified": False, "native_ready": False,
              "tools_call_qualified": False, "vendor_authentication": False, "model_calls": False}
    try:
        require(report.get("acceptance") == ACCEPTANCE and report.get("target") == "pi"
                and report.get("kind") == "plugin" and report.get("platform") == "linux"
                and report.get("architecture") == "x86_64" and report.get("passed") == 0
                and report.get("source_selection_complete") is True
                and report.get("fixture_count") == report.get("cases_attempted") == 30)
        for field, version in (("client_version", "1.5.0"), ("cli_version", "agentplugins 0.1.115")):
            require(report.get(field) == {"status": "reported", "value": version, "exit_code": 0}
                    and type(report[field].get("exit_code")) is int)
        rows, sources = report["results"], fixtures["plugin_aliases"]
        require(len(rows) == len(sources) == 30 and len(set(sources)) == 30)
        cases = set()
        for index, (row, source) in enumerate(zip(rows, sources)):
            require(row.get("index") == index and type(row.get("index")) is int
                    and row.get("source") == source and row.get("kind") == "plugin"
                    and row.get("target") == "pi"
                    and row.get("identity") == fixtures["plugin_declared_names"].get(source, source))
            if negative(row):
                require(index == 26 and source ==
                    "github:obra/the-elements-of-style@05fc4f0d2b97b7c042dd9949ad658568e4a1324e")
                result["pi_native_registration_negative"] += 1
                continue
            require(row.get("status") == "package-installed-native-unqualified"
                and all(row.get(key) is True for key in (
                "listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                "removal_verified", "post_list_schema_valid", "sentinels_preserved")))
            require(row.get("native_residuals") == [] and all(row.get(key) is False for key in (
                "failed_install_registered", "failed_install_residue", "post_remove_identity_registered")))
            for command in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
                require(type(row[command].get("exit_code")) is int and row[command]["exit_code"] == 0
                        and row[command].get("timeout") is False)
            proof = row["pi_plugin_registration_proof"]
            case = Path(proof["case_root"])
            require(case.is_absolute() and str(case) == proof["case_root"] and ".." not in case.parts
                    and case.name == f"plugin-{index:02d}" and str(case) not in cases
                    and proof.get("source") == source and proof.get("index") == index
                    and type(proof.get("index")) is int and proof.get("schema_version") == 1)
            cases.add(str(case))
            require(proof.get("status") == "native-registration-lifecycle-verified"
                    and proof.get("qualification") == "native-registration-only" and "refusal" not in proof
                    and all(proof.get(key) is False for key in (
                        "native_ready", "tools_call_qualified", "vendor_authentication", "model_calls")))
            checkpoints = proof["checkpoints"]
            require(list(checkpoints) == ["before-add", "after-add", "repeat-add", "after-remove"])
            for value in checkpoints.values():
                require(value.get("registration_verified") is True)
                for kind in ("skills", "mcp"):
                    capture = value[kind]
                    require(capture.get("error") is None and type(capture.get("exit")) is int
                            and capture.get("cwd") == str(case / "project")
                            and capture["exit"] in ((0,) if kind == "skills" else (0, 1))
                            and 0 < len(base64.b64decode(capture["stdout_base64"], validate=True)) <= 1 << 20
                            and base64.b64decode(capture["stderr_base64"], validate=True) == b"")
            result["pi_native_registration_passed"] += 1
        result["pi_native_registration_qualified"] = (
            result["pi_native_registration_passed"] == 29 and result["pi_native_registration_negative"] == 1)
    except (KeyError, ValueError, TypeError, AttributeError):
        pass
    return result
