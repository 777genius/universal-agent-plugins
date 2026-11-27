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


    def test_initialize_diagnostic_seam_is_exact_and_keeps_native_refusals(self):
        for mutation in (None, "exit", "timeout", "stderr"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                subject, env, project, state, run = self.fixture(Path(temporary).resolve())
                boundaries = []
                def initialize(action):
                    self.assertEqual(state["calls"], [["--version"], ["plugin", "list", "--json"]])
                    boundaries.append("before")
                    try:
                        return action()
                    finally:
                        boundaries.append("after")
                def selected_run(*args, **kwargs):
                    state["mutation"] = mutation if args[0][1:] == list(discovery.INITIALIZE_ARGS) else None
                    return run(*args, **kwargs)
                with patch.dict(discovery._PROBE, {"run_probe": selected_run}):
                    if mutation is None:
                        subject.before_remove(env, project, "project", "A", initialize=initialize)
                    else:
                        with self.assertRaises(ValueError):
                            subject.before_remove(env, project, "project", "A", initialize=initialize)
                self.assertEqual(boundaries, ["before", "after"])
                self.assertEqual(state["calls"][-1], list(discovery.INITIALIZE_ARGS))

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



# Immutable published lock bytes are fixture input, not generated npm state.
PLAYWRIGHT_LOCK = '''{
  "name": "agentplugins-runtime-playwright",
  "version": "1.0.0",
  "lockfileVersion": 3,
  "requires": true,
  "packages": {
    "": {
      "name": "agentplugins-runtime-playwright",
      "version": "1.0.0",
      "dependencies": {
        "@playwright/mcp": "0.0.80"
      }
    },
    "node_modules/@playwright/mcp": {
      "version": "0.0.80",
      "resolved": "https://registry.npmjs.org/@playwright/mcp/-/mcp-0.0.80.tgz",
      "integrity": "sha512-FOPXHm2SvFhAQylm10jMZ35B/SR2TaMLVkavAlwoG4N2qCb5RqbvhQYcu3zmXNyxR2DW0Ooxe+9XPVt5UjKRCQ==",
      "license": "Apache-2.0",
      "dependencies": {
        "playwright": "1.63.0-alpha-2026-08-31",
        "playwright-core": "1.63.0-alpha-2026-08-31"
      },
      "bin": {
        "playwright-mcp": "cli.js"
      },
      "engines": {
        "node": ">=18"
      }
    },
    "node_modules/playwright": {
      "version": "1.63.0-alpha-2026-08-31",
      "resolved": "https://registry.npmjs.org/playwright/-/playwright-1.63.0-alpha-2026-08-31.tgz",
      "integrity": "sha512-3XAsuznfu8jBVJ4QxdGvBkt0+b8ZFwuwJYyOfiIw5ZjUOrNLNRhKxzLzLuydou3gJ9c6eMwVqgzdiOwhy54Kzw==",
      "license": "Apache-2.0",
      "dependencies": {
        "playwright-core": "1.63.0-alpha-2026-08-31"
      },
      "bin": {
        "playwright": "cli.js"
      },
      "engines": {
        "node": ">=20"
      }
    },
    "node_modules/playwright-core": {
      "version": "1.63.0-alpha-2026-08-31",
      "resolved": "https://registry.npmjs.org/playwright-core/-/playwright-core-1.63.0-alpha-2026-08-31.tgz",
      "integrity": "sha512-1ek0Lyr12h6jcs/WTcNoVtzZkQp7D/90PsMuBW/Rm6h3AsWAbzpqj0geMv8+8Tzzr9CSUYvg9kznrVZINQMXXw==",
      "license": "Apache-2.0",
      "bin": {
        "playwright-core": "cli.js"
      },
      "engines": {
        "node": ">=20"
      }
    }
  }
}
'''


class ClaudeOwnedRuntimeLinkContracts(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        from runpy import run_path
        cls.runner = run_path(str(Path(__file__).with_name("marketing-kit-project-release-e2e.py")))

    def install_runtime(self, runner, env, project, scope, *, retained=False):
        import hashlib
        anchor = project / ".uap" if scope == "project" else Path(env["AGENTPLUGINS_HOME"])
        physical = "playwright-01aeb6209cf5"
        data = anchor / "plugin-data" / physical
        runtime = data / "npm-runtime" / discovery.PLAYWRIGHT_LOCK_SHA256
        bins = runtime / "node_modules/.bin"
        bins.mkdir(parents=True)
        links, targets = {}, {}
        for name, relative in discovery.PLAYWRIGHT_BIN_LINKS.items():
            target = runtime / "node_modules" / relative.removeprefix("../")
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text("inert npm entrypoint fixture; never executed\n")
            link = bins / name
            link.symlink_to(relative)
            links[name], targets[name] = link, target
        (runtime / "package-lock.json").write_text(PLAYWRIGHT_LOCK)
        installation = scope + "-playwright-installation"
        receipt_id = "data_" + hashlib.sha256(
            "\0".join((installation, physical, scope)).encode()).hexdigest()[:24]
        receipt = {"data_receipt_id": receipt_id, "physical_backend_id": physical,
                   "scope": scope, "locator": str(data), "state": "owned",
                   "ownership_digest": "sha256:" + hashlib.sha256(
                       ("agentplugins-plugin-data-v1\0" + receipt_id + "\0" + str(data)).encode()).hexdigest()}
        owner = data / ".agentplugins-data-owner.json"
        owner.write_text(json.dumps(receipt))
        pin = runner["FIXED_SIGNED_CLOSURE"]["777genius/playwright-bridge"]
        record = {"installation_id": installation, "declared_name": "playwright", "origin_mode": "directory",
                  "directory": {"distribution_id": "777genius/playwright-bridge",
                                "desired_release_sequence": pin["release_sequence"]},
                  "source": {key: pin[field] for key, field in (
                      ("repository", "repository"), ("package_subpath", "package_path"),
                      ("resolved_revision", "resolved_revision"), ("tree_digest", "tree_digest"))},
                  "package": {"version": pin["package_version"], "manifest_digest": pin["manifest_digest"]},
                  "data_receipts": {receipt_id: receipt}, "data_retained": retained,
                  "clients": {} if retained else {"claude": {
                      "client_id": "claude", "scope": scope, "physical_artifact_id": physical,
                      "data_receipt_id": receipt_id}}}
        state = anchor / "state-v2.json"
        state.write_text(json.dumps({"schema_version": 5, "installations": [record]}))
        return {"runtime": runtime, "links": links, "targets": targets, "owner": owner,
                "state": state, "record": record, "receipt": receipt}

    def fixture(self, case, scopes=("project",)):
        home, data, project = (case / name for name in ("home", "data", "project"))
        for path in (home, data, project):
            path.mkdir()
        env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
        installed = {scope: self.install_runtime(self.runner, env, project, scope,
                     retained=scope == "project") for scope in scopes}
        return env, project, installed

    def snapshot(self, env, project, case):
        return self.runner["claude_observation_snapshot"](env, project, case)

    def test_signed_project_and_user_bin_links_remain_in_complete_snapshot(self):
        import hashlib
        self.assertEqual(hashlib.sha256(PLAYWRIGHT_LOCK.encode()).hexdigest(), discovery.PLAYWRIGHT_LOCK_SHA256)
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            env, project, installed = self.fixture(case, ("project", "user"))
            before = self.snapshot(env, project, case)
            self.assertEqual(before, self.runner["profile_snapshot"](env, project))
            self.assertEqual(sum(key.startswith("link:") for key in before), 6)
            self.assertEqual(sum(value == "symlink" for value in before.values()), 6)
            for scope, paths in installed.items():
                root = project if scope == "project" else Path(env["AGENTPLUGINS_HOME"])
                for target in paths["targets"].values():
                    self.assertIn(root.name + "/" + target.relative_to(root).as_posix(), before)

    def test_foreign_redirected_or_unowned_links_fail_before_observation(self):
        mutations = ("absolute", "redirected", "escape", "unknown-bin", "dangling", "target-link",
                     "target-hardlink", "target-parent", "bin-parent", "runtime-parent", "foreign-home",
                     "foreign-namespace", "state-link", "owner-link", "owner-mismatch", "source-mismatch",
                     "lock-mismatch", "retained-mismatch")
        for mutation in mutations:
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                case = Path(temporary).resolve()
                env, project, installed = self.fixture(case)
                item = installed["project"]
                link, target = item["links"]["playwright"], item["targets"]["playwright"]
                outside = case / "unowned"
                outside.write_text("foreign bytes must not become runtime authority")
                if mutation in ("absolute", "redirected", "escape"):
                    link.unlink()
                    link.symlink_to(str(target) if mutation == "absolute" else
                                    "../playwright-core/cli.js" if mutation == "redirected" else
                                    "../../../../../../../../unowned")
                elif mutation == "unknown-bin":
                    (link.parent / "foreign").symlink_to("../playwright/cli.js")
                elif mutation == "dangling":
                    target.unlink()
                elif mutation in ("target-link", "target-hardlink"):
                    target.unlink()
                    target.symlink_to(outside) if mutation == "target-link" else os.link(outside, target)
                elif mutation in ("target-parent", "bin-parent", "runtime-parent"):
                    parent = target.parent if mutation == "target-parent" else link.parent if mutation == "bin-parent" else item["runtime"]
                    moved = case / "moved"
                    parent.rename(moved)
                    parent.symlink_to(moved, target_is_directory=True)
                elif mutation == "foreign-home":
                    (Path(env["HOME"]) / "foreign").symlink_to(target)
                elif mutation == "foreign-namespace":
                    foreign = project / ".uap/plugin-data/foreign/npm-runtime" / discovery.PLAYWRIGHT_LOCK_SHA256 / "node_modules/.bin/playwright"
                    foreign.parent.mkdir(parents=True)
                    foreign.symlink_to("../playwright/cli.js")
                elif mutation in ("state-link", "owner-link"):
                    path = item["state"] if mutation == "state-link" else item["owner"]
                    moved = case / "moved"
                    path.rename(moved)
                    path.symlink_to(moved)
                elif mutation == "owner-mismatch":
                    item["owner"].write_text(json.dumps({**item["receipt"], "locator": str(case)}))
                elif mutation == "lock-mismatch":
                    (item["runtime"] / "package-lock.json").write_text(PLAYWRIGHT_LOCK + "\n")
                else:
                    record = item["record"]
                    if mutation == "source-mismatch":
                        record["source"]["resolved_revision"] = "0" * 40
                    else:
                        record["data_retained"] = False
                    item["state"].write_text(json.dumps({"schema_version": 5, "installations": [record]}))
                with self.assertRaises((AssertionError, ValueError)):
                    self.snapshot(env, project, case)

    def test_native_mutations_of_admitted_links_and_targets_fail_even_on_action_error(self):
        for mutation in ("target-bytes", "target-inode", "link-text", "link-inode"):
            for action_error in (False, True):
                with self.subTest(mutation=mutation, action_error=action_error), TemporaryDirectory() as temporary:
                    case = Path(temporary).resolve()
                    env, project, installed = self.fixture(case, ("project", "user"))
                    item = installed["user"]
                    link, target = item["links"]["playwright"], item["targets"]["playwright"]
                    before = self.snapshot(env, project, case)
                    def action():
                        if mutation == "target-bytes":
                            target.write_text("changed by native action")
                        elif mutation == "target-inode":
                            body = target.read_bytes()
                            target.rename(case / "saved-target")
                            target.write_bytes(body)
                        elif mutation == "link-text":
                            link.unlink()
                            link.symlink_to("../playwright-core/cli.js")
                        else:
                            link.rename(case / "saved-link")
                            link.symlink_to("../playwright/cli.js")
                        if action_error:
                            raise RuntimeError("injected native action failure")
                    with self.assertRaisesRegex(AssertionError, "Claude native observation"):
                        self.runner["guarded_native_attestation"](action, (
                            lambda: self.runner["check_profile_unchanged"](
                                env, project, before, "Claude native observation"),))

    def test_main_lifecycle_accepts_owned_runtime_without_dropping_snapshot_guards(self):
        from test_live_install_audit_env import IsolatedClientEnvironmentTest
        def install(runner, env, project, label):
            if label in ("add", "cold locked restore", "User add", "User cold locked restore C"):
                self.install_runtime(runner, env, project, "user" if label.startswith("User ") else "project")
        code, report, phases = IsolatedClientEnvironmentTest()._exercise_marketing_kit_scope_report(
            "claude", claude_runtime_fixture=install)
        self.assertEqual(code, 0, report.get("failure"))
        self.assertTrue(report["two_scope_lifecycle_qualified"])
        self.assertIn("claude-after:User A remove", phases)


if __name__ == "__main__":
    unittest.main()
