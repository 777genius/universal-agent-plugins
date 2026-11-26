"""Inert native-output contracts only; these tests never establish native E2E."""
import copy
import json
import os
from pathlib import Path
import subprocess
from tempfile import TemporaryDirectory
import textwrap
import unittest
from unittest.mock import patch

import marketing_kit_claude_discovery as discovery


class ClaudeRemovalDiscoveryContracts(unittest.TestCase):
    def test_workflow_filters_execute_suites_and_classify_each_new_input(self):
        root = Path(__file__).resolve().parents[1]
        workflow = (root / ".github/workflows/marketing-kit-project-release-e2e.yml").read_text()
        classifier = (root / ".github/workflows/pr-changes.yml").read_text()
        paths = ("scripts/marketing_kit_claude_retirement.py", "scripts/marketing_kit_claude_discovery.py",
                 "scripts/test_marketing_kit_claude_retirement.py", "scripts/test_marketing_kit_claude_discovery.py")
        self.assertIn("python3 -m unittest discover -s scripts -p 'test_marketing_kit_claude_*.py'", workflow)
        script = textwrap.dedent(classifier.split("        run: |\n", 1)[1])
        with TemporaryDirectory() as temporary:
            location = Path(temporary)
            gh = location / "gh"
            gh.write_text('#!/bin/sh\nprintf "%s\\n" "$CLAUDE_CHANGED_PATH"\n')
            gh.chmod(0o700)
            for path in paths:
                with self.subTest(path=path):
                    self.assertEqual(workflow.count("      - '" + path + "'"), 2)
                    output = location / "output"
                    output.write_text("")
                    env = dict(os.environ, PATH=str(location) + os.pathsep + os.environ["PATH"],
                        CLAUDE_CHANGED_PATH=path, GITHUB_EVENT_NAME="pull_request", CHANGED_FILES="1",
                        GITHUB_OUTPUT=str(output), GITHUB_REPOSITORY="inert/repository", PR_NUMBER="1")
                    result = subprocess.run(["bash", "-c", script], cwd=location, env=env,
                                            capture_output=True, timeout=10, check=False)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    values = dict(row.split("=", 1) for row in output.read_text().splitlines())
                    self.assertEqual(values["live_clients"], '["claude"]')
                    self.assertEqual(values["native_clients"], '[]')

    def fixture(self, root):
        home, project = root / "home", root / "project"
        home.mkdir(); project.mkdir(); (home / ".claude").mkdir()
        binary = root / "inert-claude"
        binary.write_text("inert test artifact; never executed\n")
        subject = discovery.RemovalDiscovery(binary, root)
        env = {"HOME": str(home), "CLAUDE_CONFIG_DIR": str(home / ".claude"),
               "ANTHROPIC_API_KEY": "must-not-inherit"}
        foreign = {"id": "foreign@vendor", "scope": "user", "enabled": True,
                   "installPath": str(home / "foreign")}
        rows = [dict(foreign)] + [{"id": name + "@agentplugins-deadbeef0000",
                "scope": "project", "enabled": True, "installPath": str(home / name)}
                for name in ("marketing-kit", "notion", "hubspot-crm", "playwright")]
        commands = [{"name": "foreign-skill", "builtin": False},
                    {"name": "marketing-kit:campaign-planning", "builtin": False}]
        state = {"removed": False, "mutation": None, "calls": []}

        def run(argv, cwd, native_env, output, label, **options):
            self.assertEqual(cwd, project)
            self.assertNotIn("ANTHROPIC_API_KEY", native_env)
            self.assertEqual(native_env["CLAUDE_CONFIG_DIR"], str(home / ".claude"))
            self.assertTrue(options["memory_only"])
            self.assertEqual(options["timeout"], 60)
            state["calls"].append(argv[1:])
            mutation = state["mutation"]
            if argv[1:] == ["--version"]:
                body = b"2.1.263 (Claude Code)\n"
                if mutation == "version": body = b"2.1.264 (Claude Code)\n"
            elif argv[1:] == ["mcp", "list"]:
                body = state.get("mcp_body", b"unclassified genuine MCP diagnostic\n")
            elif argv[1:3] == ["plugin", "list"]:
                selected = [dict(foreign)] if state["removed"] else copy.deepcopy(rows)
                if state["removed"] and mutation == "registration": selected.append(rows[1])
                if mutation == "duplicate": selected.append(selected[0])
                if mutation == "malformed": selected[0]["enabled"] = 1
                if mutation == "foreign-registration": selected[0]["enabled"] = False
                body = json.dumps(selected).encode()
                if mutation == "truncated": body = body[:-1]
                if mutation == "extra-document": body += b"[]"
            else:
                self.assertEqual(argv[1:], list(discovery.INITIALIZE_ARGS))
                self.assertEqual(options["input_bytes"], discovery.INITIALIZE)
                selected = [dict(commands[0])] if state["removed"] else copy.deepcopy(commands)
                if state["removed"] and mutation == "skill": selected.append(commands[1])
                if mutation == "foreign-skill": selected[0]["builtin"] = True
                if mutation == "skill-duplicate": selected.append(selected[0])
                payload = {"account": {"tokenSource": "none"}, "commands": selected}
                response = {"subtype": "success", "request_id": "native-discovery", "response": payload}
                if mutation == "credentials": payload["account"]["tokenSource"] = "apiKey"
                if mutation == "request": response["request_id"] = "different"
                if mutation == "response": response["subtype"] = "error"
                if mutation == "missing-commands": del payload["commands"]
                body = json.dumps({"type": "control_response", "response": response}).encode()
                if mutation == "json-duplicate": body = body.replace(b'"type":', b'"type":"forged","type":', 1)
            record = {"probe": label, "exit": 0, "error": None, "stdout_bytes": len(body),
                      "stderr_bytes": 0, "stdout_sha256": "0" * 64, "stderr_sha256": "0" * 64}
            stderr = b""
            if mutation == "timeout": record["error"] = "probe deadline exceeded"
            if mutation == "exit": record["exit"] = 1
            if mutation == "stderr": stderr = b"native warning"
            if argv[1:] == ["mcp", "list"] and state.get("mcp_exit"):
                record["exit"] = state["mcp_exit"]
            return record, body, stderr
        return subject, env, project, state, run

    def test_fresh_before_and_after_require_native_version_registration_and_skills(self):
        with TemporaryDirectory() as temporary:
            subject, env, project, state, run = self.fixture(Path(temporary).resolve())
            with patch.dict(discovery._PROBE, {"run_probe": run}):
                before = subject.before_remove(env, project, "project", "B")
                state["removed"] = True
                proof = subject.after_remove(env, project, before, "B")
            self.assertEqual(len(state["calls"]), 7)
            self.assertTrue(proof["plugin_registration_absence_qualified"])
            self.assertTrue(proof["skill_discovery_absence_qualified"])
            self.assertTrue(proof["foreign_native_inventory_preserved"])
            for flag in ("physical_absence_qualified", "writer_closure_qualified",
                         "mcp_discovery_absence_qualified", "full_native_removal_qualified"):
                self.assertIs(proof[flag], False)

    def test_effective_mcp_outputs_remain_raw_observations_and_never_qualify_absence(self):
        import base64
        for body, code in ((b"", 0), (b"No MCP servers configured.\n", 0),
                           (b"plugin:notion:notion: needs authentication\n", 0),
                           (b"unknown diagnostic\n", 1)):
            with self.subTest(body=body, code=code), TemporaryDirectory() as temporary:
                subject, env, project, state, run = self.fixture(Path(temporary).resolve())
                with patch.dict(discovery._PROBE, {"run_probe": run}):
                    before = subject.before_remove(env, project, "project", "B")
                    state.update(removed=True, mcp_body=body, mcp_exit=code)
                    proof = subject.after_remove(env, project, before, "B")
                self.assertIs(proof["mcp_discovery_absence_qualified"], False)
                self.assertIs(proof["full_native_removal_qualified"], False)
                self.assertEqual(base64.b64decode(subject.evidence[-1]["stdout_base64"]), body)
                self.assertEqual(subject.evidence[-1]["exit"], code)

    def test_active_malformed_partial_and_foreign_changes_never_qualify(self):
        mutations = ("registration", "skill", "version", "duplicate", "malformed", "truncated",
            "extra-document", "foreign-registration", "foreign-skill", "skill-duplicate", "credentials",
            "request", "response", "missing-commands", "json-duplicate", "timeout", "exit", "stderr")
        for mutation in mutations:
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                subject, env, project, state, run = self.fixture(Path(temporary).resolve())
                with patch.dict(discovery._PROBE, {"run_probe": run}):
                    before = subject.before_remove(env, project, "project", "B")
                    state.update(removed=True, mutation=mutation)
                    with self.assertRaises((ValueError, json.JSONDecodeError)):
                        subject.after_remove(env, project, before, "B")

    def test_profile_scope_and_native_executable_cannot_be_substituted(self):
        for mutation in ("home", "project", "profile", "binary", "scope"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                subject, env, project, state, run = self.fixture(root)
                with patch.dict(discovery._PROBE, {"run_probe": run}):
                    before = subject.before_remove(env, project, "project", "B")
                    state["removed"] = True
                    if mutation == "home": before["home"] += "-other"
                    elif mutation == "project": before["project"] += "-other"
                    elif mutation == "profile": env["CLAUDE_CONFIG_DIR"] += "-other"
                    elif mutation == "binary": subject.executable.write_text("substituted native executable")
                    else:
                        state["removed"] = False
                        with self.assertRaises(ValueError):
                            subject.before_remove(env, project, "user", "B")
                        continue
                    with self.assertRaises(ValueError):
                        subject.after_remove(env, project, before, "B")

    def test_main_keeps_cold_restore_order_and_refuses_own_or_peer_native_writes(self):
        from test_live_install_audit_env import IsolatedClientEnvironmentTest
        fixture = IsolatedClientEnvironmentTest()
        code, report, phases = fixture._exercise_marketing_kit_scope_report("claude")
        self.assertEqual(code, 0, report.get("failure"))
        self.assertLess(phases.index("cold locked restore"), phases.index("claude-before:remove B"))
        self.assertLess(phases.index("User cold locked restore C"), phases.index("claude-before:User C remove"))
        for mutation in (("after", "B", "B", False), ("after", "A", "A", False),
                         ("before", "B", "A", False), ("after", "B", "A", True)):
            with self.subTest(mutation=mutation):
                code, report, phases = fixture._exercise_marketing_kit_scope_report(
                    "claude", claude_native_mutation=mutation)
                phase, case, _, _ = mutation
                if phase == "before":
                    self.assertNotIn("claude-capture:" + case, phases)
                else:
                    self.assertIn("claude-capture:" + case, phases)
                    self.assertNotIn("claude-finish:" + case, phases)
                    self.assertIn("claude-close:" + case, phases)
                self.assertEqual(code, 1)
                self.assertFalse(report["two_scope_lifecycle_qualified"])
                self.assertIn("Claude native", report["failure"])
                self.assertFalse(report["native_removal_qualification"]["full_native_removal_qualified"])


if __name__ == "__main__":
    unittest.main()
