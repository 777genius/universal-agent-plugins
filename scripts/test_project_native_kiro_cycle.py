"""Inert exact-source contracts for the authenticated Kiro lifecycle reader."""
import copy
import datetime
import json
from pathlib import Path
import unittest

import project_native_kiro_cycle as cycle


def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def fixture():
    root, nonce = "/owned/native-fixture", "a" * 32
    info = {"steps": list(cycle.STEPS), "complete": True, "positive_sessions": 4,
            "nonce": nonce, "root": root, "projects": {"a": root + "/project", "b": root + "/project-b"},
            "home": root + "/home", "source": root + "/package", "name": "uap-kiro-attempt"}
    native = "/owned/acquired/kirocli/bin/kiro-cli"
    source = {"source_commit": "b" * 40, "source_tree": "c" * 40, "scanner_archive_sha256": "d" * 64,
              "client_identity": {"client": "kiro", "acquisition": {"sha256": cycle.PIN,
                  "pin_verified": True, "members": [{"name": "kirocli/bin/kiro-cli", "kind": "file",
                  "bytes": 43027744, "sha256": "94cae7e349f76869a19eae335a5b392fba8515743b1c2822b9ea33128c067cc6"}]},
                  "runtime_files": [{"path": native, "sha256": "94cae7e349f76869a19eae335a5b392fba8515743b1c2822b9ea33128c067cc6"}]}}
    summary = {"client": "kiro", "version": "2.34.0", "runtime_attempt_complete": True,
               "activation": "prepared", "vendor_authentication_qualified": True, "real_model_qualified": True,
               "qualification": {key: True for key in cycle.FLAGS},
               "source_identity": {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                   "installer_source_state": "committed", "installer_patch_sha256": []},
               "acquisition": {"client": "kiro", "acquisition": {"sha256": cycle.PIN, "pin_verified": True,
                   "members": [{"Name": "kirocli/bin/kiro-cli", "Kind": "file", "Bytes": 43027744,
                       "sha256": "94cae7e349f76869a19eae335a5b392fba8515743b1c2822b9ea33128c067cc6"}]},
                   "node": None, "runtime_files": source["client_identity"]["runtime_files"]},
               "scanner": {"version": "0.1.3", "source_tag": "v0.1.3", "binary_sha256": "e" * 64,
                   "archive_sha256": source["scanner_archive_sha256"], "platform": "linux-amd64",
                   "route": "prepopulated supported scanner cache; ordinary CLI security assessment"},
               "test_failed": False, "elapsed_ms": 40000, "capture_mode": cycle.MODE, "kiro_cycle": info}
    roles = {key: make_role(info, key) for key in ("a", "b")}
    proofs = []
    for stage in ("add-a", "add-b", "repair-a", "remove-a", "remove-b"):
        key = stage[-1]
        selected = copy.deepcopy(roles[key])
        if stage == "repair-a":
            selected["state"]["installations"][0]["updated_at"] = "after-repair"
            selected["state"]["installations"][0]["clients"]["kiro"]["updated_at"] = "after-repair"
            selected["binding"]["updated_at"] = "after-repair"
            roles[key] = copy.deepcopy(selected)
        removed = stage.startswith("remove-")
        if removed:
            selected["state"]["installations"][0]["clients"] = {}
            selected["state"]["installations"][0]["data_retained"] = True
            selected["config"] = {"mcpServers": {}}
        state, config = encoded(selected["state"]), encoded(selected["config"])
        proofs.append({"stage": stage, "role": key, "project": info["projects"][key],
                       "state_json": state.decode(), "state_sha256": cycle.sha(state),
                       "config_json": config.decode(), "config_sha256": cycle.sha(config),
                       "data": selected["data"], "removed": removed, "binding": selected["binding"]})
    bodies = {"runtime-attempt.json": encoded(summary), "kiro-cycle-proof.json": encoded(proofs)}
    for key in ("a", "b"):
        selector = "@uap-kiro-attempt/inspect_runtime"
        bodies["kiro-agent-" + key + ".json"] = encoded({
            "name": "uap-kiro-native-" + key, "description": "Disposable UAP native MCP fixture",
            "prompt": "Use only the named disposable MCP inspect_runtime tool, exactly as the user requests. Never run shell, file, web, or subagent tools.",
            "tools": [selector], "allowedTools": [selector], "mcpServers": {},
            "resources": [], "hooks": {}, "toolAliases": {}})
    events = b""
    for index, label in enumerate(cycle.BASE + cycle.STEPS):
        key = "b" if label.endswith("-b") else "a"
        project = info["projects"][key]
        cwd, argv = command(info, roles, native, label)
        row = {"argv": argv, "cwd": cwd, "exit": 0, "error": False, "deadline": False,
               "elapsed_ms": 1000, "cleanup": "full descendant supervision; forced cleanup fails"}
        if label not in cycle.BASE:
            row.update(started_at=stamp(index * 2), ended_at=stamp(index * 2 + 1), event_offset=len(events))
        if label in cycle.POSITIVE:
            events += event_rows(info, roles[key], index)
        bodies[label + ".command.json"] = encoded(row)
        bodies[label + ".stdout.log"] = b""
        bodies[label + ".stderr.log"] = b""
        if label in cycle.PROOFS:
            bodies[label + ".stdout.log"] = encoded({"schema_version": 1, "command": label.split("-")[0],
                                                    "result": "success", "data": {"result": {"mutated": True}}})
    bodies["version.stdout.log"] = b"kiro-cli 2.34.0\n"
    bodies["chat-help.stdout.log"] = b"--v2 --no-interactive --agent --trust-tools\n"
    bodies["agent-help.stdout.log"] = b"--path\n"
    bodies["probe-events.jsonl"] = events
    return bodies, source


def make_role(info, key):
    project = info["projects"][key]
    active, data = project + "/.uap/managed/clients/kiro/artifact-" + key, project + "/.uap/plugin-data/data-" + key
    binding = {"client_binding_id": "binding-" + key, "client_id": "kiro", "scope": "project",
               "target_locator": active, "physical_artifact_id": "artifact-" + key,
               "materialization": "materialized", "activation": "prepared", "authentication": "not_required",
               "policy": "not_required", "verification": "installed", "data_receipt_id": "data-" + key,
               "native_profile_root": project + "/.kiro", "updated_at": "before"}
    allocation = {"data_receipt_id": "data-" + key, "physical_backend_id": "artifact-" + key,
                  "scope": "project", "locator": data, "ownership_digest": "sha256:" + "f" * 64, "state": "owned"}
    installed = {"installation_id": "installation-" + key, "declared_name": "uap-kiro-attempt",
                 "source": {"source_binding_id": "source-" + key, "canonical_source": info["source"], "tree_digest": "sha256:" + "1" * 64},
                 "package": {"declared_name": "uap-kiro-attempt", "version": "1.0.0", "manifest_digest": "sha256:" + "2" * 64},
                 "clients": {"kiro": copy.deepcopy(binding)}, "data_receipts": {"data-" + key: allocation},
                 "created_at": "before", "updated_at": "before"}
    entry = {"command": active + "/io.github.777genius.agentplugins/managed-stdio-v1/agentplugins",
             "args": ["--internal-stdio-v1", active, data, active, "plugin", "--", "./bin/probe", "Owned kiro", active + "/argument"],
             "env": {"PLUGIN_ROOT": active, "PLUGIN_DATA": data, "UAP_TEST_ROOT": info["root"],
                     "UAP_TEST_EVENTS": info["root"] + "/events.jsonl", "UAP_TEST_NONCE": info["nonce"], "UAP_TEST_REVISION": "a-one"}}
    return {"key": key, "binding": binding, "data": data,
            "state": {"schema_version": 5, "installations": [installed]}, "config": {"mcpServers": {"uap-kiro-attempt": entry}}}


def stamp(second, fraction=""):
    return "2026-01-01T00:00:" + str(second).zfill(2) + ("." + fraction if fraction else "") + "Z"


def command(info, roles, native, label):
    key = "b" if label.endswith("-b") else "a"
    project = info["projects"][key]
    if label.startswith("git-"):
        return project, ["/usr/bin/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"]
    if label == "version":
        return info["root"] + "/outside", [native, "--version"]
    if label in ("chat-help", "agent-help"):
        return info["root"] + "/outside", [native, *(["chat", "--help"] if label == "chat-help" else ["agent", "validate", "--help"])]
    if label.startswith("validate-"):
        return project, [native, "agent", "validate", "--path", project + "/.kiro/agents/uap-kiro-native-" + key + ".json"]
    if label.startswith(("add-", "repair-", "remove-")):
        verb = label.split("-")[0]
        target = info["source"] if verb == "add" else "installation-" + key
        args = ["/owned/uap", verb, target]
        if verb == "add":
            args += ["--no-save"]
        args += ["--scope", "project", "--project-root", project, "--target", "kiro", "--format", "json"]
        if verb != "add":
            args += ["--yes"]
        return project, args
    operation = "write" if label.startswith("native-write-") else "read"
    prompt = ("Call @uap-kiro-attempt/inspect_runtime exactly once with nonce " + info["nonce"]
              + ", operation " + operation + " and marker kiro-" + key + "-" + info["nonce"] + ". Do not use any other tool.")
    if label.startswith("native-removed-"):
        prompt = ("Call @uap-kiro-attempt/inspect_runtime only if that tool is available, with nonce " + info["nonce"]
                  + " and operation read. If unavailable, finish without using any tool. Do not use any other tool.")
    return project, [native, "chat", "--v2", "--no-interactive", "--agent", "uap-kiro-native-" + key,
                     "--trust-tools", "@uap-kiro-attempt/inspect_runtime", prompt]


def event_rows(info, role, index):
    pid = 31 + index
    seconds = int(datetime.datetime(2026, 1, 1, tzinfo=datetime.timezone.utc).timestamp()) + index * 2
    session = str(pid) + "-" + str(seconds * 1_000_000_000 + 1)
    active = role["binding"]["target_locator"]
    facts = {"nonce": info["nonce"], "revision": "a-one", "cwd": active,
             "argv": ["Owned kiro", active + "/argument"], "plugin_root": active, "plugin_data": role["data"],
             "literal": "", "once": "", "marker": "kiro-" + role["key"] + "-" + info["nonce"], "pid": pid, "session": session}
    return b"".join(encoded({"method": method, "pid": pid, "session": session, "nonce": info["nonce"],
                             "timestamp": stamp(index * 2, "00" + str(position + 1)), "facts": facts if method == "tools/call" else None}) + b"\n"
                    for position, method in enumerate(("initialize", "tools/list", "tools/call")))



def replace_event_rows(bodies, rows):
    """Keep the command-offset joins exact while mutating an event predicate."""
    raw, cursor = b"", 0
    for label in cycle.STEPS:
        command_row = json.loads(bodies[label + ".command.json"])
        command_row["event_offset"] = len(raw)
        bodies[label + ".command.json"] = encoded(command_row)
        if label in cycle.POSITIVE:
            raw += b"".join(encoded(row) + b"\n" for row in rows[cursor:cursor + 3])
            cursor += 3
    bodies["probe-events.jsonl"] = raw


class KiroAuthenticatedCycleContracts(unittest.TestCase):
    def test_complete_receipts_require_four_causal_sessions(self):
        bodies, source = fixture()
        result = cycle.members(bodies, source)
        self.assertTrue(result["runtime_attempt_complete"])
        self.assertTrue(all(result["qualification"].values()))
        self.assertEqual(result["activation"], "prepared")
        self.assertEqual(len(bodies), 59)
        self.assertNotIn(b"KIRO_API_KEY", b"".join(bodies.values()))

    def test_removed_controls_require_success_and_cleanup(self):
        for label in ("native-removed-a", "native-removed-b", "native-remaining-b"):
            for key, value in (("exit", 1), ("error", True), ("deadline", True), ("cleanup", "unknown")):
                with self.subTest(label=label, field=key):
                    bodies, source = fixture()
                    row = json.loads(bodies[label + ".command.json"])
                    row[key] = value
                    bodies[label + ".command.json"] = encoded(row)
                    with self.assertRaises(ValueError):
                        cycle.members(bodies, source)
        bodies, source = fixture()
        bodies["native-remaining-b.command.json"] = bodies["native-removed-a.command.json"]
        with self.assertRaises(ValueError):
            cycle.members(bodies, source)

    def test_nonce_scope_session_order_and_birth_are_joined(self):
        for field, value in (("nonce", "foreign"), ("plugin_data", "/foreign/data"),
                             ("pid", True), ("marker", "foreign"), ("session", "31-1")):
            with self.subTest(field=field):
                bodies, source = fixture()
                rows = [json.loads(line) for line in bodies["probe-events.jsonl"].splitlines()]
                rows[2]["facts"][field] = value
                replace_event_rows(bodies, rows)
                with self.assertRaises(ValueError):
                    cycle.members(bodies, source)
        bodies, source = fixture()
        rows = [json.loads(line) for line in bodies["probe-events.jsonl"].splitlines()]
        rows[0]["method"] = "tools/list"
        replace_event_rows(bodies, rows)
        with self.assertRaises(ValueError):
            cycle.members(bodies, source)


        bodies, source = fixture()
        rows = [json.loads(line) for line in bodies["probe-events.jsonl"].splitlines()]
        for row in rows[:3]:
            row["session"] = str(row["pid"]) + "-1"
            if row["facts"] is not None:
                row["facts"]["session"] = row["session"]
        replace_event_rows(bodies, rows)
        with self.assertRaises(ValueError):
            cycle.members(bodies, source)

    def test_state_data_members_and_public_results_are_closed(self):
        for field in ("data", "identity", "config", "hash"):
            with self.subTest(field=field):
                bodies, source = fixture()
                rows = json.loads(bodies["kiro-cycle-proof.json"])
                if field == "data":
                    state = json.loads(rows[2]["state_json"])
                    state["installations"][0]["data_receipts"]["data-a"]["locator"] = "/foreign/data"
                    rows[2]["state_json"] = encoded(state).decode()
                    rows[2]["state_sha256"] = cycle.sha(rows[2]["state_json"].encode())
                elif field == "identity":
                    state = json.loads(rows[2]["state_json"])
                    state["installations"][0]["installation_id"] = "foreign"
                    rows[2]["state_json"] = encoded(state).decode()
                    rows[2]["state_sha256"] = cycle.sha(rows[2]["state_json"].encode())
                elif field == "config":
                    rows[3]["config_json"] = rows[0]["config_json"]
                    rows[3]["config_sha256"] = rows[0]["config_sha256"]
                else:
                    rows[0]["state_sha256"] = "0" * 64
                bodies["kiro-cycle-proof.json"] = encoded(rows)
                with self.assertRaises(ValueError):
                    cycle.members(bodies, source)
        bodies, source = fixture()
        bodies["foreign.log"] = b""
        with self.assertRaises(ValueError):
            cycle.members(bodies, source)
        bodies, source = fixture()
        bodies["remove-a.stdout.log"] = encoded({"schema_version": 1, "command": "remove", "result": "failure", "data": {}})
        with self.assertRaises(ValueError):
            cycle.members(bodies, source)

    def test_only_exact_agent_and_version_declared_syntax_are_admitted(self):
        for kind in ("agent", "argv", "help"):
            with self.subTest(kind=kind):
                bodies, source = fixture()
                if kind == "agent":
                    row = json.loads(bodies["kiro-agent-a.json"])
                    row["tools"].append("execute_bash")
                    bodies["kiro-agent-a.json"] = encoded(row)
                elif kind == "argv":
                    row = json.loads(bodies["native-write-a.command.json"])
                    row["argv"][row["argv"].index("--trust-tools")] = "--trust-all-tools"
                    bodies["native-write-a.command.json"] = encoded(row)
                else:
                    bodies["chat-help.stdout.log"] = b"--no-interactive --agent --trust-tools\n"
                with self.assertRaises(ValueError):
                    cycle.members(bodies, source)

    def test_incomplete_attempt_keeps_all_qualification_false(self):
        bodies, source = fixture()
        result = json.loads(bodies["runtime-attempt.json"])
        result.update(runtime_attempt_complete=False, test_failed=True, vendor_authentication_qualified=False, real_model_qualified=False)
        result["qualification"] = {key: False for key in cycle.FLAGS}
        result["kiro_cycle"].update(steps=[], complete=False, positive_sessions=0, name="")
        partial = {"runtime-attempt.json": encoded(result), "kiro-cycle-proof.json": b"[]"}
        self.assertFalse(cycle.members(partial, source)["runtime_attempt_complete"])
        result["vendor_authentication_qualified"] = True
        partial["runtime-attempt.json"] = encoded(result)
        with self.assertRaises(ValueError):
            cycle.members(partial, source)

    def test_existing_workflow_keeps_manual_auth_guard_and_ordinary_compile(self):
        workflow = (Path(__file__).resolve().parents[1] / ".github/workflows/project-native-runtime.yml").read_text()
        self.assertIn("github.event_name == 'workflow_dispatch' && inputs.kiro_auth_research == true && matrix.client == 'kiro' && secrets.KIRO_API_KEY || ''", workflow)
        self.assertIn("if not allow_kiro_auth:", workflow)
        self.assertIn("go test -json -p=1 -count=1 -timeout=120s ./repotests -run '^(TestCopilot|TestProjectNativeRuntime|TestKiroCycle)'", workflow)


if __name__ == "__main__":
    unittest.main()
