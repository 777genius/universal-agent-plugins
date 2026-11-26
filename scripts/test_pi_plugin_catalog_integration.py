"""Real case orchestration with inert public processes and a native-observer seam."""
from copy import deepcopy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import test_live_install_audit_env as live_fixtures


class PiPluginCatalogIntegrationTests(unittest.TestCase):
    def run_case(self, *, enabled=True, fault=None, native_refusal=None):
        audit = live_fixtures.load_audit()
        fixtures = json.loads(audit.FIXTURES.read_text())
        pins = audit.effective_signed_release_pins(fixtures, "pi")
        node = deepcopy(live_fixtures.PiContext7SignedFallbackContracts.NODE)
        added = deepcopy(live_fixtures.PiContext7SignedFallbackContracts.INSTALL)
        events, observers = [], []
        state = {"installed": False, "adds": 0, "removed": False}

        class Observer:
            def __init__(self, home, project, data, env, version, api, *, source, index):
                if source != "context7" or index != fixtures["plugin_aliases"].index(source):
                    raise AssertionError("native observation is not bound to the selected source")
                self.home = Path(home)
                self.evidence = {"status": "unqualified", "checkpoints": {}}
                observers.append(self)

            def checked(self, method, value):
                phase = value if method == "checkpoint" else "select"
                events.append("native:" + phase)
                if phase == native_refusal:
                    error = ValueError("injected native inventory refusal")
                    error.pi_plugin_registration_proof = self.evidence
                    raise error
                if method == "checkpoint":
                    self.evidence["checkpoints"][phase] = {"registration_verified": True}
                    if phase == "after-remove":
                        if fault == "late-native-foreign-change":
                            (self.home / "foreign-untouched.txt").write_text("changed by native probe")
                        self.evidence["status"] = "native-registration-lifecycle-verified"

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            index = fixtures["plugin_aliases"].index("context7")
            case = root / f"plugin-{index:02d}"
            config = case / "home/.pi/agent/mcp.json"
            data = case / "uap-data"

            def public(argv, env, cwd, timeout=120, **kwargs):
                operation = argv[1]
                events.append("public:" + operation)
                self.assertEqual(argv[0], "/inert/agentplugins")
                self.assertEqual(cwd, case / "project")
                self.assertEqual(env["HOME"], str(case / "home"))
                code, error = 0, ""
                if operation == "add":
                    state["adds"] += 1
                    if fault == "install-refused" or (fault == "repeat-failed" and state["adds"] == 2):
                        code, error = 1, "installation refused\n"
                    elif not state["installed"]:
                        config.parent.mkdir(parents=True, exist_ok=True)
                        config.write_text('{"mcpServers":{"context7":{"command":"never-executed"}}}')
                        (data / "uap.toml").write_text("# inert desired source\n")
                        (data / "uap.lock").write_text('{"packages":[{"alias":"context7"}]}')
                        state["installed"] = True
                    document = added
                elif operation == "remove":
                    if fault == "remove-failed":
                        code, error = 1, "removal refused\n"
                    else:
                        config.unlink()
                        (data / "uap.lock").write_text('{"packages":[]}')
                        state.update(installed=False, removed=True)
                    document = {"data": {}}
                else:
                    self.assertEqual(operation, "list")
                    selected = deepcopy(node)
                    if fault == "wrong-source":
                        selected["source"] = "foreign/repository/plugin"
                    rows = [selected] if state["installed"] and fault != "missing-source" else []
                    if fault == "post-list-failed" and state["removed"]:
                        code, error = 1, "inventory refused\n"
                    document = {"schema_version": 1, "command": "list", "result": "success",
                                "data": {"installations": rows}}
                return {"exit_code": code, "timeout": False, "stdout_tail": json.dumps(document),
                        "stderr_tail": error, "_json": deepcopy(document)}

            with patch.object(audit, "call", side_effect=public), patch.object(
                    audit, "PI_PLUGIN_CATALOG_AUDIT", {"PiPluginCatalogAudit": Observer}):
                try:
                    row = audit.do_case("/inert/agentplugins", "pi", "plugin", "context7", index,
                        root, fixtures["plugin_declared_names"], pins,
                        client_version={"status": "reported", "value": "1.5.0", "exit_code": 0},
                        pi_registration=enabled)
                except ValueError as error:
                    if native_refusal is None:
                        raise
                    return error, events, observers
        return row, events, observers

    def test_opt_in_observes_all_stages_without_promoting_default_native_status(self):
        row, events, observers = self.run_case()
        self.assertEqual(events, ["native:before-add", "public:add", "public:list", "native:select",
            "native:after-add", "public:add", "public:list", "native:repeat-add",
            "public:remove", "public:list", "native:after-remove"])
        self.assertEqual(row["status"], "package-installed-native-unqualified")
        self.assertEqual(row["pi_plugin_registration_proof"]["status"], "native-registration-lifecycle-verified")
        self.assertEqual(len(observers), 1)
        for field in ("listed_identity", "repeat_verified", "removal_verified", "sentinels_preserved"):
            self.assertIs(row[field], True, field)
        default, calls, created = self.run_case(enabled=False)
        self.assertEqual(calls, ["public:" + step for step in ("add", "list", "add", "list", "remove", "list")])
        self.assertEqual(created, [])
        self.assertNotIn("pi_plugin_registration_proof", default)
        self.assertEqual(default["status"], row["status"])

    def test_failed_public_gates_never_complete_native_registration_proof(self):
        expected = {"install-refused": ["before-add"], "missing-source": ["before-add"],
            "wrong-source": ["before-add"], "repeat-failed": ["before-add", "after-add"],
            "remove-failed": ["before-add", "after-add", "repeat-add"],
            "post-list-failed": ["before-add", "after-add", "repeat-add"]}
        for fault, phases in expected.items():
            with self.subTest(fault=fault):
                row, _, _ = self.run_case(fault=fault)
                proof = row["pi_plugin_registration_proof"]
                self.assertEqual(list(proof["checkpoints"]), phases)
                self.assertEqual(proof["status"], "unqualified")
                self.assertNotEqual(row["status"], "passed")

    def test_native_refusal_propagates_without_later_commands_or_final_proof(self):
        for phase in ("before-add", "after-add", "repeat-add", "after-remove"):
            with self.subTest(phase=phase):
                error, events, _ = self.run_case(native_refusal=phase)
                self.assertIsInstance(error, ValueError)
                self.assertEqual(events[-1], "native:" + phase)
                self.assertEqual(error.pi_plugin_registration_proof["status"], "unqualified")

    def test_final_native_probe_cannot_hide_a_foreign_home_change(self):
        row, _, _ = self.run_case(fault="late-native-foreign-change")
        self.assertEqual(row["pi_plugin_registration_proof"]["status"], "native-registration-lifecycle-verified")
        self.assertIs(row["sentinels_preserved"], False)
        self.assertNotEqual(row["status"], "passed")


if __name__ == "__main__":
    unittest.main()
