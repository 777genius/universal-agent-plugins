"""Prevent the live acceptance harness from touching ambient client profiles."""
import importlib.util
from contextlib import closing, contextmanager
from datetime import datetime, timezone, timedelta
import hashlib
import json
import os
import sqlite3
import subprocess
import sys
import textwrap
from pathlib import Path, PurePosixPath, PureWindowsPath
from runpy import run_path
from tempfile import TemporaryDirectory
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch
from contextlib import contextmanager
from datetime import datetime, timezone
import hashlib
import runpy
import struct
import time

ROOT = Path(__file__).resolve().parents[1]
COPILOT_TRUST_FIXTURE_SUPPORTED = (hasattr(os, "getuid") and os.open in os.supports_dir_fd
                                   and hasattr(os, "O_NOFOLLOW") and hasattr(os, "O_DIRECTORY"))


def load_audit():
    file = Path(__file__).resolve().parent / "live-install-audit.py"
    spec = importlib.util.spec_from_file_location("live_install_audit", file)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    # Unit fixtures inject native executables explicitly, never from the host.
    module.AGENT_BINARIES.clear()
    return module


class PiContext7SignedFallbackContracts(unittest.TestCase):
    # Literal original public row: no expected pin is learned from this DTO.
    NODE = json.loads(r'''{"installation_id":"ae532983-081d-4238-98ad-ea0cf8adfbf3","name":"context7","version":"0.2.5","source":"777genius/universal-agent-plugins-registry/plugins/context7","clients":[{"client_id":"pi","scope":"user","materialization":"materialized","activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"installation_verified","package_revision":{"version":"0.2.5","resolved_revision":"8360159c07140d17cbd4060a110553092c9cd245","distribution_id":"777genius/context7","release_sequence":7,"tree_digest":"sha256:635e673ec3aa8440a2826dc3a677ebe652b63e542db2e861b61d2df7f55e48b3","manifest_digest":"sha256:1c1b8633e99d8ae10ed804b2c98532c4e08049b95f6229a6dd0b3746bdaea496"},"affected_surfaces":["pi"]}],"directory":{"product_id":"context7","recorded_distribution":"777genius/context7","current_distribution":"777genius/context7","recorded_revision":"8360159c07140d17cbd4060a110553092c9cd245","current_revision":"8360159c07140d17cbd4060a110553092c9cd245","recorded_release_sequence":7,"current_release_sequence":7,"recorded_snapshot_sequence":49},"mixed_version":false}''')
    INSTALL = json.loads(r'''{"schema_version":1,"command":"add","result":"success","data":{"actions":[{"to_version":"0.2.5","component_support":[{"kind":"mcp_server","name":"context7","support":"native"}],"alias":"context7","client_id":"pi","action":"install","reason":"no matching environment-owned binding","package_digest":"sha256:635e673ec3aa8440a2826dc3a677ebe652b63e542db2e861b61d2df7f55e48b3","revision":"8360159c07140d17cbd4060a110553092c9cd245"}],"command":"agentplugins add","environment":{"alias":"context7","intent_change":true,"lock_path":"/tmp/uap-live-30x30-b7gj262v/plugin-01/uap-data/uap.lock","manifest_path":"/tmp/uap-live-30x30-b7gj262v/plugin-01/uap-data/uap.toml","saved":true,"scope":"user","scope_selection_reason":"explicit","selected_targets":["pi"]},"outcome":"completed","result":"success","results":[{"installation_id":"ae532983-081d-4238-98ad-ea0cf8adfbf3","client_id":"pi","operation":"install","outcome":"completed","manual_actions":["Reload resources or restart Pi to discover user Skills and MCP; installed files do not prove an authenticated connection or model tool call."]}],"schema_version":1,"scope":"user","scope_selection_reason":"explicit"}}''')
    EXPECTED = {
        "distribution": "777genius/context7", "repository": "777genius/universal-agent-plugins-registry",
        "path": "plugins/context7", "revision": "8360159c07140d17cbd4060a110553092c9cd245", "sequence": 7,
        "tree_digest": "sha256:635e673ec3aa8440a2826dc3a677ebe652b63e542db2e861b61d2df7f55e48b3",
        "manifest_digest": "sha256:1c1b8633e99d8ae10ed804b2c98532c4e08049b95f6229a6dd0b3746bdaea496",
    }

    def inputs(self):
        import copy
        audit = load_audit()
        doc = json.loads(audit.FIXTURES.read_text())
        defaults = doc['signed_release_pins']
        # Pi selects the signed community release7 through the per-target pin
        # override; matching never derives an expected release from a row.
        selected = audit.effective_signed_release_pins(doc, 'pi')
        return audit, defaults, selected, copy.deepcopy(self.NODE)

    def matching(self, audit, pins, node, target='pi', alias='context7'):
        return audit.matching_installations({'data': {'installations': [node]}},
                                           'context7', target, 'plugin', alias, pins)

    def test_actual_target_fallback_row_requires_the_full_immutable_pin(self):
        import copy
        audit, defaults, selected, node = self.inputs()
        self.assertEqual(self.matching(audit, selected, node), [node])
        # Raw default pins name the inactive upstream distribution for pi.
        self.assertEqual(self.matching(audit, defaults, node), [])
        self.assertEqual(selected['context7'], self.EXPECTED)
        self.assertEqual(node['clients'][0]['activation'], 'prepared')
        self.assertEqual(node['clients'][0]['authentication'], 'not_checked')
        mutations = [
            ('source', 'other/repository/plugins/context7'),
            ('directory.product_id', 'other'), ('directory.recorded_distribution', 'upstash/context7'),
            ('directory.recorded_revision', '0' * 40), ('directory.recorded_release_sequence', 6),
            ('directory.recorded_release_sequence', True),
            ('client.scope', 'project'), ('client.client_id', 'codex'), ('client.verification', 'failed'),
            ('revision.distribution_id', 'upstash/context7'), ('revision.resolved_revision', '0' * 40),
            ('revision.release_sequence', 6), ('revision.tree_digest', 'sha256:' + '0' * 64),
            ('revision.manifest_digest', 'sha256:' + '0' * 64),
        ]
        for key, value in mutations:
            with self.subTest(field=key, value=value):
                bad = copy.deepcopy(node)
                if key.startswith('directory.'): bad['directory'][key.split('.')[1]] = value
                elif key.startswith('client.'): bad['clients'][0][key.split('.')[1]] = value
                elif key.startswith('revision.'): bad['clients'][0]['package_revision'][key.split('.')[1]] = value
                else: bad[key] = value
                self.assertEqual(self.matching(audit, selected, bad), [])
        current_only = copy.deepcopy(node)
        for key in ('recorded_distribution', 'recorded_revision', 'recorded_release_sequence'):
            del current_only['directory'][key]
        self.assertEqual(self.matching(audit, selected, current_only), [])

    def test_default_authority_and_all_other_clients_aliases_stay_exact(self):
        import copy
        audit, defaults, selected, node = self.inputs()
        doc = json.loads(audit.FIXTURES.read_text())
        original = copy.deepcopy(defaults)
        for target in audit.NATIVE_ROOTS:
            canonical = audit.audit_client_id(target, 'plugin')
            target_pins = audit.effective_signed_release_pins(doc, canonical)
            for alias in defaults:
                if (canonical, alias) == ('pi', 'context7'): continue
                self.assertEqual(target_pins[alias], defaults[alias])
        for target in audit.NATIVE_ROOTS:
            if target == 'pi': continue
            foreign = copy.deepcopy(node)
            foreign['clients'][0]['client_id'] = target
            self.assertEqual(self.matching(audit, defaults, foreign, target=target), [])
        upstream = copy.deepcopy(node)
        pin = defaults['context7']
        upstream['source'] = pin['repository'] + '/' + pin['path']
        upstream['directory'].update(recorded_distribution=pin['distribution'],
            recorded_revision=pin['revision'], recorded_release_sequence=pin['sequence'])
        upstream['clients'][0]['package_revision'].update(distribution_id=pin['distribution'],
            resolved_revision=pin['revision'], release_sequence=pin['sequence'],
            tree_digest=pin['tree_digest'], manifest_digest=pin['manifest_digest'])
        upstream['clients'][0]['client_id'] = 'codex'
        self.assertEqual(self.matching(audit, defaults, upstream, target='codex'), [upstream])
        upstream['clients'][0]['client_id'] = 'pi'
        self.assertEqual(self.matching(audit, selected, upstream), [])
        for mutation in ('missing', 'extra', 'revision', 'bool-sequence', 'string-sequence'):
            with self.subTest(mutation=mutation):
                bad = copy.deepcopy(selected)
                if mutation == 'missing': del bad['context7']
                elif mutation == 'extra':
                    bad['context7']['unreviewed'] = True
                    doc_bad = json.loads(audit.FIXTURES.read_text())
                    doc_bad['signed_release_pins_by_target']['pi']['context7'] = copy.deepcopy(bad['context7'])
                    with self.assertRaisesRegex(ValueError, 'frozen signed release pin'):
                        audit.validate_fixtures(doc_bad)
                    continue
                elif mutation == 'revision': bad['context7']['revision'] = '0' * 40
                elif mutation == 'bool-sequence': bad['context7']['sequence'] = True
                else: bad['context7']['sequence'] = '1'
                self.assertEqual(self.matching(audit, bad, node), [])
        self.assertEqual(self.matching(audit, selected, node, alias='notion'), [])
        self.assertEqual(defaults, original)

    def test_real_case_orchestration_keeps_prepared_state_and_all_repeat_removal_gates(self):
        import copy
        import shutil
        audit, defaults, selected, node = self.inputs()
        calls, installed = [], [False]
        def perform(argv, env, project, timeout=105):
            operation = argv[1]
            calls.append(operation)
            home = Path(env['HOME'])
            owned = home / '.pi/agent/extensions/context7-test-owned'
            aggregate = home / '.pi/agent/mcp.json'
            if operation == 'add':
                installed[0] = True
                owned.mkdir(parents=True, exist_ok=True)
                (owned / 'plugin.json').write_text('{"name":"context7","version":"0.2.5"}')
                (owned / 'mcp.json').write_text('{"mcpServers":{"context7":{"url":"https://inert.test/mcp"}}}')
                aggregate.write_text('{"mcpServers":{"context7":{"url":"https://inert.test/mcp"}}}')
                response = copy.deepcopy(self.INSTALL)
            elif operation == 'list':
                response = {'schema_version': 1, 'command': 'list', 'result': 'success',
                            'data': {'installations': [copy.deepcopy(node)] if installed[0] else []}}
            else:
                self.assertEqual(operation, 'remove')
                self.assertEqual(argv[2], 'context7')
                installed[0] = False
                shutil.rmtree(owned)
                aggregate.unlink()
                response = {'schema_version': 1, 'result': 'success'}
            return {'exit_code': 0, 'stdout_tail': json.dumps(response), 'stderr_tail': '',
                    'timeout': False, '_json': response}
        with TemporaryDirectory() as temporary, patch.object(audit, 'call', side_effect=perform):
            result = audit.do_case('/inert/uap', 'pi', 'plugin', 'context7', 1,
                                  Path(temporary).resolve(), {}, selected)
        # Pi component support and owned files do not provide independent
        # before/add/repeat/remove native inventories in this catalog lane.
        self.assertEqual(result['status'], 'package-installed-native-unqualified')
        self.assertEqual(calls, ['add', 'list', 'add', 'list', 'remove', 'list'])
        self.assertEqual(result['matched_installation_ids'], result['repeat_installation_ids'])
        self.assertTrue(result['repeat_verified'])
        self.assertTrue(result['removal_verified'])
        self.assertTrue(result['sentinels_preserved'])
        self.assertEqual(result['public_target_states']['after-add']['states'][0]['activation'], 'prepared')
        self.assertEqual(result['public_target_states']['after-add']['states'][0]['authentication'], 'not_checked')


class QoderPreparedRoutingContracts(unittest.TestCase):
    def test_prepared_proof_cannot_route_another_target_kind_or_host(self):
        audit = load_audit()
        self.assertIsNone(audit.QODER_PREPARED_AUDIT)
        with TemporaryDirectory() as temporary, patch.object(audit, "call") as command:
            root = Path(temporary)
            for target, kind in (("pi", "skill"), ("qoder", "plugin")):
                with self.subTest(target=target, kind=kind), self.assertRaises(ValueError):
                    audit.do_case("/inert/uap", target, kind, {}, 0, root, {}, qoder_prepared=True)
            with patch.object(audit.sys, "platform", "win32"), self.assertRaises(ValueError):
                audit.do_case("/inert/uap", "qoder", "skill", {}, 0, root, {}, qoder_prepared=True)
            command.assert_not_called()
            self.assertEqual(list(root.iterdir()), [])


class IsolatedClientEnvironmentTest(unittest.TestCase):

    def test_copilot_canonical_remove_public_envelope(self):
        audit = load_audit()
        receipt = {"schema_version": 1, "command": "agentplugins remove", "result": "success"}
        self.assertTrue(audit.copilot_skill_success_response(receipt, "agentplugins remove"))
        for mutated in ({"schema_version": 1, "command": "remove", "result": "success"},
                        {"schema_version": True, "command": "agentplugins remove", "result": "success"},
                        {"schema_version": 1, "command": "agentplugins remove", "result": "failure"}):
            with self.subTest(mutated=mutated):
                self.assertFalse(audit.copilot_skill_success_response(mutated, "agentplugins remove"))

    def test_copilot_skill_repeat_only_owns_empty_new_process_logs(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home = case / "home"
            logs = home / ".copilot" / "logs"
            logs.mkdir(parents=True, mode=0o700)
            roots = [home / ".copilot", home / ".config" / "copilot"]
            before = audit.snapshot(roots)
            first = logs / "process-123-456.log"
            second = logs / "process-789-101.log"
            first.write_bytes(b"")
            after = audit.snapshot(roots)
            self.assertTrue(audit.copilot_skill_repeat_log_only(before, after, home, case))
            second.write_bytes(b"")
            self.assertTrue(audit.copilot_skill_repeat_log_only(
                before, audit.snapshot(roots), home, case))
            (logs / "process-222-333.log").write_bytes(b"")
            self.assertFalse(audit.copilot_skill_repeat_log_only(
                before, audit.snapshot(roots), home, case))
            (logs / "process-222-333.log").unlink()
            first.write_bytes(b"changed")
            self.assertFalse(audit.copilot_skill_repeat_log_only(
                before, audit.snapshot(roots), home, case))
            first.write_bytes(b"")
            second.unlink()
            first.unlink()
            link = logs / "process-123-456.log"
            link.symlink_to(case)
            self.assertFalse(audit.copilot_skill_repeat_log_only(
                before, audit.snapshot(roots), home, case))

    def test_marketing_current_vscode_boundary_preserves_authenticated_managed_user_grants(self):
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        classify = runner["current_vscode_boundary_kind"]
        managed = {client: {"scopes": ["user"], "delivery": "managed"}
                   for client in ("codex", "cursor", "copilot", "claude", "gemini", "opencode")}
        extra = {client: {"scopes": ["user"], "delivery": "managed"}
                 for client in ("hermes", "antigravity", "pi")}
        vscode = {"vscode": {"scopes": ["user"], "delivery": "prepared"}}
        policy = {"source_mode": "production_default", "snapshot_sequence": 49,
                  "snapshot_digest": "sha256:" + "a" * 64,
                  "closure": runner["FIXED_SIGNED_CLOSURE"]}
        for roots, result in ((managed, "signed_grant_refusal"),
                              ({**managed, **extra}, "signed_grant_refusal"),
                              ({**managed, **vscode}, "saved_management_capability"),
                              ({**managed, **extra, **vscode}, "saved_management_capability"),
                              ({**managed, **extra, "other": {"scopes": ["user"], "delivery": "managed"}},
                               "signed_grant_refusal")):
            self.assertEqual(classify({**policy, "root_target_compatibility": roots}), result)
        for invalid in ({**managed, **extra, "vscode": {"scopes": ["user"], "delivery": "managed"}},
                        {**managed, "hermes": {"scopes": ["project"], "delivery": "managed"}},
                        {**managed, **extra, "other": {"scopes": ["user"], "delivery": "prepared"}}):
            with self.subTest(invalid=invalid), self.assertRaisesRegex(
                    AssertionError, "unqualified signed target shape"):
                classify({**policy, "root_target_compatibility": invalid})

    def test_remotion_diagnostic_has_only_sanitized_source_coordinates(self):
        runner = run_path(str(Path(__file__).resolve().parent /
                              "remotion_published_windows_long_temp_e2e.py"))
        try:
            raise ValueError("private-value-never-in-report")
        except ValueError as error:
            site = runner["sanitized_failure_site"](error, Path(__file__).resolve().parents[1])
        self.assertEqual(site["file"], "test_live_install_audit_env.py")
        self.assertIsInstance(site["line"], int)
        self.assertNotIn("private-value-never-in-report", str(site))
        self.assertNotIn(str(Path(__file__).resolve().parents[1]), str(site))
        self.assertEqual(runner["sanitized_failure_site"](
            ValueError("other-secret"), Path(__file__).resolve().parents[1])["file"], "unqualified")


    def test_gemini_repeat_snapshot_tracks_ordinary_empty_directory_deltas(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            profile = Path(temporary) / ".gemini"
            chats = profile / "tmp" / "project" / "chats"
            chats.mkdir(parents=True)
            before = audit.snapshot([profile], include_directories=True)
            ordinary_before = audit.snapshot([profile])
            untracked = profile / "unexpected-empty"
            untracked.mkdir()
            after = audit.snapshot([profile], include_directories=True)
            self.assertNotEqual(before, after)
            self.assertIn("directory:.gemini/unexpected-empty", after)
            # Other client snapshots intentionally keep the old contract.
            self.assertEqual(audit.snapshot([profile]), ordinary_before)
            untracked.rmdir()
            # A valid journal under the same chats parents changes one file,
            # not the directory identities used to reject foreign deltas.
            (chats / "record.json").write_text("{}\\n")
            journal = audit.snapshot([profile], include_directories=True)
            self.assertEqual(set(journal) - set(before),
                             {".gemini/tmp/project/chats/record.json"})
            (chats / "record.json").unlink()
            self.assertEqual(audit.snapshot([profile], include_directories=True), before)

    def test_marketing_kit_rejects_non_success_json_even_with_zero_exit(self):
        # A zero process exit with a refusal/partial response is not lifecycle PASS.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["verified"]
        accepted = {"exit_code": 0, "timeout": False,
                    "_json": {"result": "success", "data": {}}}
        self.assertEqual(check(accepted, "list", "list"), accepted["_json"])
        completed = {**accepted, "_json": {"result": "success",
                                          "outcome": "completed", "data": {}}}
        self.assertEqual(check(completed, "list", "list"), completed["_json"])
        for invalid in (
                {"exit_code": 1},
                {"timeout": True},
                {"_json": None},
                {"_json": {"data": {}}},
                {"_json": {"result": "failure", "error": "not authorized"}},
                {"_json": {"result": "partial", "data": {}}},
                # 'success' must not override an explicitly incomplete outcome.
                {"_json": {"result": "success", "outcome": "pending"}},
                {"_json": {"result": "success", "outcome": "manual_action_required"}},
                {"_json": {"result": "success", "outcome": "unexpected"}},
                {"_json": {"result": "success", "outcome": 42}}):
            with self.subTest(invalid=invalid):
                with self.assertRaisesRegex(AssertionError, "CLI did not succeed"):
                    check({**accepted, **invalid}, "add")

    def test_marketing_kit_add_requires_scoped_dependency_receipts(self):
        # A published add response has two JSON layers. The outer success is
        # not proof that the inner signed dependency closure was applied.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["verified"]
        aliases = ("marketing-kit", "notion", "hubspot-crm", "playwright")
        actions = [{"alias": alias, "client_id": "codex", "action": "install"}
                   for alias in aliases]
        results = [{"client_id": "codex", "operation": "install",
                    "installation_id": alias + "-id", "outcome": "completed"}
                   for alias in aliases]
        closure = {"result": "success", "outcome": "completed",
                   "scope": "project", "actions": actions, "results": results}
        envelope = {"result": "success", "data": closure}
        wrapped = {"exit_code": 0, "timeout": False, "_json": envelope}
        self.assertEqual(check(wrapped, "add", "add", "codex", "project"), envelope)
        # A caller-less or scope-less result cannot qualify an add even if
        # every nested signed dependency action otherwise looks complete.
        for chosen_target, chosen_scope in ((None, None), ("codex", None),
                                            (None, "project")):
            with self.subTest(target=chosen_target, scope=chosen_scope):
                with self.assertRaisesRegex(AssertionError, "CLI did not succeed"):
                    check(wrapped, "add", "add", chosen_target, chosen_scope)
        # A public wrapper may itself report completed, but not pending/failure.
        completed = {**envelope, "outcome": "completed"}
        self.assertEqual(check({**wrapped, "_json": completed},
                               "add", "add", "codex", "project"), completed)
        # Repeated add can legitimately converge without active actions.
        no_op = {**closure, "actions": [{**a, "action": "unchanged"} for a in actions],
                 "results": []}
        self.assertEqual(check({**wrapped, "_json": {**envelope, "data": no_op}},
                               "repeat add", "add", "codex", "project"),
                         {**envelope, "data": no_op})
        for bad in (
                *({**envelope, "outcome": status} for status in
                  ("failure", "pending", "manual_action_required", "unexpected", 42)),
                {"result": "success", "data": {}},
                {"result": "success"},
                {**envelope, "data": {**closure, "outcome": "partial"}},
                {**envelope, "data": {**closure, "result": "failure"}},
                {**envelope, "data": {**closure, "scope": "user"}},
                {**envelope, "data": {**closure, "results": results[:-1]}},
                # Duplicate receipts for two different signed child aliases
                # cannot establish that each dependency was actually applied.
                {**envelope, "data": {**closure, "results": [
                    results[0], {**results[1],
                                 "installation_id": results[0]["installation_id"]},
                    *results[2:]]}},
                {**envelope, "data": {**closure, "actions": actions[:-1]}},
                {**envelope, "data": {**closure, "actions": [
                    {**actions[0], "client_id": "claude"}, *actions[1:]]}},
        ):
            with self.subTest(bad=bad), self.assertRaisesRegex(
                    AssertionError, "CLI did not succeed"):
                check({**wrapped, "_json": bad}, "add", "add", "codex", "project")

    def test_marketing_kit_add_receipts_match_native_scope_inventory(self):
        # Cold add plans do not have installation IDs. Even four unique
        # successful result IDs cannot prove they belong to the four installs.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        aliases = ("marketing-kit", "notion", "hubspot-crm", "playwright")
        distributions = runner["REQUIRED_DISTRIBUTIONS"]
        actions = [{"alias": alias, "client_id": "codex", "action": "install"}
                   for alias in aliases]
        results = [{"client_id": "codex", "operation": "install",
                    "installation_id": alias + "-id", "outcome": "completed"}
                   for alias in aliases]
        for scope in ("user", "project"):
            with self.subTest(scope=scope):
                envelope = {"result": "success", "data": {
                    "result": "success", "outcome": "completed",
                    "scope": scope, "actions": actions, "results": results}}
                receipt = {"exit_code": 0, "timeout": False, "_json": envelope}
                installed = {distributions[alias]: alias + "-id"
                             for alias in aliases}
                with patch.dict(execute.__globals__, {
                        "invoke": lambda *args: receipt,
                        "inventory": lambda *args: None,
                        "identities": lambda *args: installed}):
                    args = ("add", "marketing-kit", "--target", "codex",
                            "--scope", scope)
                    if scope == "project":
                        args += ("--project-root", str(Path.cwd()),)
                    self.assertEqual(
                        execute("agentplugins", {}, Path.cwd(), "codex",
                                "add", *args)["result"], "passed")
                    for changed in (
                            # Unique, plausible receipts swapped between
                            # dependencies were previously accepted.
                            [results[1], results[0], *results[2:]],
                            [{**results[0], "installation_id": "foreign-id"},
                             *results[1:]]):
                        bad_receipt = {**receipt, "_json": {**envelope, "data": {
                            **envelope["data"], "results": changed}}}
                        with patch.dict(execute.__globals__, {
                                "invoke": lambda *args: bad_receipt}):
                            with self.assertRaisesRegex(
                                    AssertionError, "receipt does not match"):
                                execute("agentplugins", {}, Path.cwd(), "codex",
                                        "add", *args)

    def test_marketing_kit_unchanged_dependency_actions_match_scoped_inventory(self):
        # A no-op command must not claim four foreign durable receipts while
        # the real selected-scope inventory and signed lock remain unchanged.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        aliases = tuple(runner["REQUIRED_DISTRIBUTIONS"])
        installed = {distribution: alias + "-id"
                     for alias, distribution in runner["REQUIRED_DISTRIBUTIONS"].items()}
        actions = [{"alias": alias, "client_id": "codex", "action": "unchanged",
                    "installation_id": alias + "-id"} for alias in aliases]
        for scope in ("user", "project"):
            for operation in ("add", "update"):
                with self.subTest(scope=scope, operation=operation):
                    data = {"result": "success", "outcome": "completed",
                            "scope": scope, "actions": actions, "results": []}
                    envelope = ({"result": "success", "data": data}
                                if operation == "add" else data)
                    response = {"exit_code": 0, "timeout": False, "_json": envelope}
                    args = (operation, "marketing-kit", "--target", "codex",
                            "--scope", scope)
                    if scope == "project":
                        args += ("--project-root", str(Path.cwd()),)
                    with patch.dict(execute.__globals__, {
                            "invoke": lambda *argv: response,
                            "inventory": lambda *argv: [],
                            "identities": lambda *argv: installed}):
                        self.assertEqual(execute(
                            "agentplugins", {}, Path.cwd(), "codex",
                            "no-op", *args)["result"], "passed")
                        for wrong_id in (None, "", "foreign-installation",
                                         actions[0]["installation_id"]):
                            changed = [dict(action) for action in actions]
                            changed[1]["installation_id"] = wrong_id
                            changed_data = {**data, "actions": changed}
                            bad = ({"result": "success", "data": changed_data}
                                   if operation == "add" else changed_data)
                            with patch.dict(execute.__globals__, {
                                    "invoke": lambda *argv: {
                                        **response, "_json": bad}}):
                                with self.assertRaisesRegex(
                                        AssertionError, "unchanged action does not match"):
                                    execute("agentplugins", {}, Path.cwd(),
                                            "codex", "no-op", *args)

    def test_marketing_kit_lock_requires_real_selected_scope_dependency_graph(self):
        # Four correct source keys can still conceal detached mandatory children.
        from copy import deepcopy
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["lock_ids"]
        required = runner["REQUIRED_DISTRIBUTIONS"]
        selectors = runner["REQUIRED_CHILD_SELECTORS"]
        pins = []
        for alias, distribution in required.items():
            pins.append({
                "alias": alias, "source_key": "directory:" + distribution,
                "source": {"registry": {"distribution": distribution}},
                "targets": ["codex"], "tree_digest": "sha256:" + "a" * 64,
                "manifest_digest": "sha256:" + "b" * 64,
                "required_by": ([] if alias == "marketing-kit" else
                                [{"alias": "marketing-kit",
                                  "selector": dict(selectors[alias])}]),
            })

        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            lock = root / "uap.lock"

            def check(packages, target="codex"):
                lock.write_text(json.dumps({"packages": packages}), encoding="utf-8")
                return verify(root, target)

            self.assertEqual(check(pins),
                             {"directory:" + d for d in required.values()})

            def swap_source_keys(rows):
                # The overall set remains correct, but each signed pin points
                # to a different distribution than its declared alias/source.
                rows[0]["source_key"], rows[1]["source_key"] = (
                    rows[1]["source_key"], rows[0]["source_key"])

            # The old graph check accepted this connected but reparented
            # closure even though Marketing Kit no longer directly requires
            # Notion as recorded in the signed package manifest.
            chained = deepcopy(pins)
            chained[1]["required_by"][0]["alias"] = "hubspot-crm"
            with self.assertRaisesRegex(AssertionError, "direct required dependencies"):
                check(chained)

            # A redundant child-to-child edge still leaves all three signed
            # children direct under Marketing Kit; reachability alone passes.
            extra_edge = deepcopy(pins)
            extra_edge[2]["required_by"].append({"alias": "notion", "selector": {
                "kind": "release", "value": "1"}})
            with self.assertRaisesRegex(AssertionError, "child dependency edge"):
                check(extra_edge)

            for mutate in (
                    swap_source_keys,
                    lambda rows: rows[1].update(required_by=[]),
                    lambda rows: rows[1].update(targets=["claude"]),
                    lambda rows: rows[1]["source"]["registry"].update(
                        distribution="someone/foreign"),
                    lambda rows: rows[1].update(tree_digest="sha256:invalid"),
                    lambda rows: rows[1]["required_by"][0].update(alias="missing"),
                    lambda rows: rows[1]["required_by"][0]["selector"].update(value="0.2.0"),
                    lambda rows: rows[2]["required_by"][0]["selector"].update(
                        kind="release", value="1"),
                    lambda rows: rows[3]["required_by"][0].pop("selector"),
                    lambda rows: rows[1].update(alias="marketing-kit"),
                    lambda rows: rows[0].update(required_by=[
                        {"alias": "notion", "selector": {
                            "kind": "release", "value": "1"}}]),
            ):
                with self.subTest(change=mutate):
                    bad = deepcopy(pins)
                    mutate(bad)
                    with self.assertRaises(AssertionError):
                        check(bad)
            with self.assertRaisesRegex(AssertionError, "signed lock pin"):
                check(pins, "claude")

    def test_marketing_kit_negative_locked_restore_fixture_changes_one_digest(self):
        # The refusal must originate from a genuinely corrupt pin, not a
        # missing lock, invalid command or unavailable upstream package.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        alter = runner["corrupt_locked_child_digest"]
        original = {"packages": [
            {"alias": "marketing-kit", "tree_digest": "sha256:" + "a" * 64},
            {"alias": "notion", "tree_digest": "sha256:" + "b" * 64},
            {"alias": "hubspot-crm", "tree_digest": "sha256:" + "c" * 64},
            {"alias": "playwright", "tree_digest": "sha256:" + "d" * 64}]}
        with TemporaryDirectory() as temp:
            locked = Path(temp) / "uap.lock"
            locked.write_text(json.dumps(original) + "\n", encoding="utf-8")
            alter(locked)
            changed = json.loads(locked.read_text(encoding="utf-8"))
            self.assertEqual(
                changed["packages"][1]["tree_digest"], "sha256:0" + "b" * 63)
            self.assertEqual(changed["packages"][0], original["packages"][0])
            self.assertEqual(changed["packages"][2:], original["packages"][2:])
            for bad in ({"packages": []}, {"packages": [
                    {"alias": "notion", "tree_digest": "not-sha256"}]}):
                locked.write_text(json.dumps(bad), encoding="utf-8")
                with self.assertRaisesRegex(AssertionError, "negative.test|signed child"):
                    alter(locked)


    def test_marketing_kit_negative_lock_corruption_rejects_replaced_paths(self):
        # The fixture itself must not overwrite a foreign file between its
        # preflight lstat and the deliberate one-digest corruption.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        alter = runner["corrupt_locked_child_digest"]
        original = json.dumps({"packages": [
            {"alias": "notion", "tree_digest": "sha256:" + "b" * 64}]}) + "\n"
        for scope in ("user", "project"):
            for variant in ("replaced", "symlink", "hardlink"):
                with self.subTest(scope=scope, variant=variant), TemporaryDirectory() as temp:
                    root = Path(temp)
                    locked = root / scope / "uap.lock"
                    locked.parent.mkdir()
                    locked.write_text(original, encoding="utf-8")
                    expected_identity = locked.lstat()
                    foreign = root / "foreign"
                    if variant == "replaced":
                        locked.replace(root / "old-lock")
                        locked.write_text(original, encoding="utf-8")
                    elif variant == "symlink":
                        locked.replace(root / "old-lock")
                        foreign.write_text("foreign bytes", encoding="utf-8")
                        try:
                            locked.symlink_to(foreign)
                        except (OSError, NotImplementedError) as error:
                            self.skipTest(f"host cannot create symlinks: {error}")
                    else:
                        try:
                            os.link(locked, foreign)
                        except (OSError, NotImplementedError) as error:
                            self.skipTest(f"host cannot create hardlinks: {error}")
                    with self.assertRaisesRegex(AssertionError, "negative-test lock path"):
                        alter(locked, expected_identity)
                    if variant == "symlink":
                        self.assertEqual(foreign.read_text(encoding="utf-8"), "foreign bytes")
                    else:
                        self.assertEqual(locked.read_text(encoding="utf-8"), original)
                        if variant == "hardlink":
                            self.assertEqual(foreign.read_text(encoding="utf-8"), original)


    def test_marketing_kit_corrupt_locked_restore_refuses_user_and_project(self):
        # Both scopes must reach the real locked CLI refusal (not an invented
        # missing-file error), restore exact portable bytes, and run guards.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_corrupt_lock_refusal"]
        original = {"packages": [
            {"alias": "marketing-kit", "tree_digest": "sha256:" + "a" * 64},
            {"alias": "notion", "tree_digest": "sha256:" + "b" * 64}]}
        for scope in ("project", "user"):
            with self.subTest(scope=scope), TemporaryDirectory() as temporary:
                root = Path(temporary)
                home, data, project = (root / name for name in ("home", "data", "project"))
                for path in (home, data, project):
                    path.mkdir()
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                lock_file = (project if scope == "project" else data) / "uap.lock"
                lock_file.write_text(json.dumps(original) + "\n", encoding="utf-8")
                original_bytes = lock_file.read_bytes()
                invoked, checked = [], []

                def refuse(command, *_):
                    invoked.append(command)
                    self.assertEqual(json.loads(lock_file.read_text())["packages"][1][
                        "tree_digest"], "sha256:0" + "b" * 63)
                    return {"exit_code": 1, "timeout": False,
                            "stderr_tail": (
                                "preflight notion/codex: acquired package tree digest "
                                + "sha256:" + "b" * 64 + " does not match expected "
                                + "sha256:0" + "b" * 63)}

                def preserved():
                    checked.append(lock_file.read_bytes())

                with patch.dict(verify.__globals__, {
                        "invoke": refuse,
                        "inventory": lambda *args: [],
                        "identities": lambda *args: {}}):
                    self.assertEqual(verify("agentplugins", env, project, "codex",
                                            scope, (preserved,)), 1)
                    self.assertEqual(lock_file.read_bytes(), original_bytes)
                    self.assertEqual(checked, [original_bytes])
                    self.assertEqual(invoked[0].count("--scope"), 1)
                    self.assertEqual(invoked[0][invoked[0].index("--scope") + 1], scope)
                    self.assertEqual("--project-root" in invoked[0], scope == "project")
                    for failed in (
                            {"exit_code": 0, "stderr_tail": "digest mismatch"},
                            {"exit_code": 1, "timeout": True,
                             "stderr_tail": "digest mismatch"},
                            {"exit_code": 1, "stderr_tail": "native client unavailable"},
                            {"exit_code": 1, "stderr_tail": "launcher cache lock timed out"},
                            {"exit_code": 1, "stderr_tail": "signed lock digest mismatch"},
                            {"exit_code": 1, "stderr_tail": "verify notion pinned manifest: remote download failed"},
                            {"exit_code": 1, "stderr_tail": "verify hubspot-crm pinned manifest; signed Directory publication differs from exact lock"},
                            {"exit_code": 1, "stderr_tail": (
                                "preflight notion/codex: acquired package tree digest "
                                + "sha256:" + "a" * 64 + " does not match expected "
                                + "sha256:0" + "b" * 63)},
                            {"exit_code": 1, "stderr_tail": (
                                "preflight notion/codex: acquired package tree digest "
                                + "sha256:" + "b" * 64 + " does not match expected "
                                + "sha256:1" + "b" * 63)}):
                        with self.subTest(failed=failed), patch.dict(
                                verify.__globals__, {
                                    "invoke": lambda *args: failed}):
                            with self.assertRaisesRegex(AssertionError, "corrupt"):
                                verify("agentplugins", env, project, "codex",
                                       scope, (preserved,))
                            self.assertEqual(lock_file.read_bytes(), original_bytes)
                    self.assertEqual(len(checked), 10)

    def test_marketing_kit_refusal_allows_only_derived_own_cache_additions(self):
        from runpy import run_path
        import hashlib

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        snapshot, check = runner["refusal_profile_snapshot"], runner["check_refusal_profile_unchanged"]
        for absent in (False, True):
            with self.subTest(absent_data=absent), TemporaryDirectory() as temporary:
                root = Path(temporary)
                home, data, project = (root / name for name in ("home", "data", "project"))
                home.mkdir()
                project.mkdir()
                if not absent:
                    data.mkdir(mode=0o700)
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                pin = {"tree_digest": "sha256:" + "b" * 64,
                       "manifest_digest": "sha256:" + "c" * 64}
                before = snapshot(env, project, {"packages": [pin]})
                own_before = runner["profile_snapshot"](env, project)
                if absent:
                    data.mkdir(mode=0o700)
                name, subject = next(iter(before["subjects"].items()))
                assessment = {"schema_version": 1, "subject": subject,
                              "scanner": {"id": "lintai", "version": "0.1.3"},
                              "policy": runner["REFUSAL_CACHE_POLICY"]}
                scanner = data / runner["REFUSAL_SCANNER_PATH"]
                files = {data / "directory-v1-cache.json": b"verified signed cache",
                         data / "directory-v1-cache.json.lock": b"",
                         data / name: json.dumps(assessment).encode(), scanner: b"pinned test binary"}
                for path, body in files.items():
                    for parent in reversed(path.relative_to(data).parents):
                        (data / parent).mkdir(mode=0o700, exist_ok=True)
                    path.write_bytes(body)
                    path.chmod(0o700 if path == scanner else 0o600)
                with patch.dict(check.__globals__, {
                        "REFUSAL_SCANNER_SHA256": hashlib.sha256(files[scanner]).hexdigest()}):
                    check(env, project, before)
                    # The peer guard keeps its full snapshot contract; even
                    # legitimate additions must fail if attributed to a peer.
                    with self.assertRaisesRegex(AssertionError, "isolated profile changed"):
                        runner["check_profile_unchanged"](env, project, own_before, "peer")
                    protected = snapshot(env, project, {"packages": [pin]})
                    cache = data / name
                    replacement = data / "replacement"
                    replacement.write_bytes(cache.read_bytes())
                    replacement.chmod(0o600)
                    replacement.replace(cache)
                    with self.assertRaisesRegex(AssertionError, "existing data entry"):
                        check(env, project, protected)

    def test_marketing_kit_refusal_rejects_state_native_foreign_and_unknown_cache(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        mutations = (
            "data/state-v2.json", "data/store/package", "data/managed/package",
            "data/operations-v2/receipt", "data/uap.toml", "data/uap.lock",
            "home/.codex/config.toml", "project/.uap/state-v2.json",
            "home/foreign.txt", "project/foreign.txt",
            "data/security/unknown", "data/security/assessments/" + "a" * 64 + ".json",
            "data/security/assessments/.assessment-temp.tmp",
            "data/security/lintai/0.1.4/linux-amd64/lintai",
        )
        for relative in mutations:
            with self.subTest(path=relative), TemporaryDirectory() as temporary:
                root = Path(temporary)
                home, data, project = (root / name for name in ("home", "data", "project"))
                for directory in (home, data, project):
                    directory.mkdir(mode=0o700)
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                path = root / relative
                if path.name in ("foreign.txt", "uap.toml", "uap.lock"):
                    path.write_text("original")
                before = runner["refusal_profile_snapshot"](env, project, {})
                path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
                path.write_text("unexpected")
                path.chmod(0o600)
                with self.assertRaises(AssertionError):
                    runner["check_refusal_profile_unchanged"](env, project, before)

    def test_marketing_kit_refusal_rejects_unsafe_cache_entries(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        for mutation in ("symlink-file", "symlink-directory", "fifo", "scanner",
                         "assessment-subject", "assessment-policy", "mode", "root-mode"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary)
                home, data, project = (root / name for name in ("home", "data", "project"))
                for directory in (home, data, project):
                    directory.mkdir(mode=0o700)
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                pin = {"tree_digest": "sha256:" + "b" * 64,
                       "manifest_digest": "sha256:" + "c" * 64}
                before = runner["refusal_profile_snapshot"](env, project, {"packages": [pin]})
                if mutation == "root-mode":
                    data.chmod(0o755)
                elif mutation == "symlink-directory":
                    outside = root / "outside"
                    outside.mkdir()
                    try:
                        (data / "security").symlink_to(outside, target_is_directory=True)
                    except (OSError, NotImplementedError) as error:
                        self.skipTest(f"host cannot create symlinks: {error}")
                elif mutation.startswith("assessment-"):
                    name, subject = next(iter(before["subjects"].items()))
                    path = data / name
                    for parent in reversed(path.relative_to(data).parents):
                        (data / parent).mkdir(mode=0o700, exist_ok=True)
                    body = {"schema_version": 1, "subject": subject,
                            "scanner": {"id": "lintai", "version": "0.1.3"},
                            "policy": dict(runner["REFUSAL_CACHE_POLICY"])}
                    if mutation == "assessment-subject":
                        body["subject"] = {**subject, "tree_digest": "sha256:" + "d" * 64}
                    else:
                        body["policy"]["version"] = 99
                    path.write_text(json.dumps(body))
                    path.chmod(0o600)
                else:
                    path = data / (runner["REFUSAL_SCANNER_PATH"] if mutation == "scanner"
                                   else "directory-v1-cache.json")
                    for parent in reversed(path.relative_to(data).parents):
                        (data / parent).mkdir(mode=0o700, exist_ok=True)
                    if mutation == "symlink-file":
                        outside = root / "outside"
                        outside.write_text("foreign")
                        try:
                            path.symlink_to(outside)
                        except (OSError, NotImplementedError) as error:
                            self.skipTest(f"host cannot create symlinks: {error}")
                    elif mutation == "fifo":
                        if not hasattr(os, "mkfifo"):
                            self.skipTest("host cannot create FIFO")
                        os.mkfifo(path, mode=0o600)
                    else:
                        path.write_text("untrusted")
                        path.chmod(0o700 if mutation == "scanner" else 0o644)
                with self.assertRaises(AssertionError):
                    runner["check_refusal_profile_unchanged"](env, project, before)

    def test_marketing_kit_vscode_user_policy_refusal_requires_exact_diagnostic(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_vscode_user_policy_refusal"]
        diagnostic = ('Resolving and validating one Agent Plugin package for every selected target...\n'
                      'agentplugins: no eligible directory release for "marketing-kit": '
                      '777genius/marketing-kit: release does not support complete '
                      'target set; missing vscode')
        accepted = {"exit_code": 1, "timeout": False, "stderr_tail": diagnostic + "\n",
                    "stdout_tail": "", "_json": None}
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            home, data, project = (case / name for name in ("home", "data", "project"))
            for directory in (home, data, project):
                directory.mkdir(mode=0o700)
            (project / "uap.toml").write_text("retained Project agreement")
            (project / "uap.lock").write_text("retained exact Project lock")
            env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
            before = runner["profile_snapshot"](env, project)
            commands, guards = [], []

            def invoke(command, *_):
                commands.append(command)
                return accepted

            with patch.dict(verify.__globals__, {"invoke": invoke,
                    "inventory": lambda *_: []}):
                stage = verify("agentplugins", env, project,
                               (lambda: guards.append("checked"),))
            self.assertEqual(commands, [["agentplugins", "add", "marketing-kit",
                "--target", "vscode", "--scope", "user", "--no-save", "--yes", "--format", "json"]])
            self.assertEqual(stage["scope"], "user")
            self.assertEqual(stage["contract"], "negative_policy")
            self.assertEqual(stage["result"], "expected_refusal")
            self.assertNotEqual(stage["result"], "passed")
            self.assertEqual(guards, ["checked"])
            self.assertEqual(runner["profile_snapshot"](env, project), before)
            for ending in ("", "\r\n"):
                with patch.dict(verify.__globals__, {
                        "invoke": lambda *_: {**accepted, "stderr_tail": diagnostic.replace("\n", "\r\n") + ending if ending else diagnostic},
                        "inventory": lambda *_: []}):
                    self.assertEqual(verify("agentplugins", env, project, ())["result"], "expected_refusal")
            for invalid in ({"exit_code": 0}, {"exit_code": -9}, {"exit_code": None},
                            {"exit_code": True}, {"timeout": True}, {"timeout": None},
                            {"stderr_tail": "network unavailable"},
                            {"stderr_tail": diagnostic.split("\n", 1)[1]},
                            {"stderr_tail": 'agentplugins: target vscode cannot be saved; retry agentplugins add marketing-kit --target vscode --no-save\n'},
                            {"stderr_tail": diagnostic.replace("vscode", "copilot")},
                            {"stderr_tail": diagnostic.replace('"marketing-kit"', '"notion"')},
                            {"stderr_tail": diagnostic + "\nremote download failed"},
                            {"stderr_tail": " " + diagnostic},
                            {"stderr_tail": diagnostic + " "},
                            {"stdout_tail": '{"result":"success"}'},
                            {"stdout_tail": " "}, {"stdout_tail": None},
                            {"_json": {"result": "success"}}):
                with self.subTest(invalid=invalid), patch.dict(verify.__globals__, {
                        "invoke": lambda *_: {**accepted, **invalid},
                        "inventory": lambda *_: []}):
                    checked = []
                    with self.assertRaisesRegex(AssertionError, "exact missing-vscode"):
                        verify("agentplugins", env, project, (lambda: checked.append(True),))
                    self.assertEqual(checked, [True])
                    self.assertEqual(runner["profile_snapshot"](env, project), before)

    def test_marketing_kit_vscode_user_policy_refusal_rejects_own_mutations(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_vscode_user_policy_refusal"]
        response = {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": (
            'Resolving and validating one Agent Plugin package for every selected target...\n'
            'agentplugins: no eligible directory release for "marketing-kit": '
            '777genius/marketing-kit: release does not support complete target set; missing vscode')}
        for relative in ("home/.copilot/settings.json", "home/foreign.txt", "project/uap.lock",
                         "data/uap.toml", "data/uap.lock", "data/state-v2.json",
                         "data/managed/plugin", "data/store/package", "data/operations-v2/receipt",
                         "data/directory-v1-cache.json"):
            for during_inventory in (False, True):
                with self.subTest(path=relative, inventory=during_inventory), TemporaryDirectory() as temporary:
                    case = Path(temporary)
                    home, data, project = (case / name for name in ("home", "data", "project"))
                    for directory in (home, data, project):
                        directory.mkdir(mode=0o700)
                    env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                    path = case / relative
                    if path.name in ("foreign.txt", "uap.lock", "state-v2.json", "directory-v1-cache.json"):
                        path.write_text("original")
                    checked = []

                    def change():
                        path.parent.mkdir(parents=True, exist_ok=True)
                        path.write_text("unexpected")

                    def invoke(*_):
                        if not during_inventory:
                            change()
                        return response

                    def inventory(*_):
                        if during_inventory:
                            change()
                        return []

                    with patch.dict(verify.__globals__, {"invoke": invoke, "inventory": inventory}):
                        with self.assertRaises(AssertionError):
                            verify("agentplugins", env, project, (lambda: checked.append(True),))
                    self.assertEqual(checked, [True])

    def test_marketing_kit_vscode_user_policy_refusal_checks_inventory_and_peers(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_vscode_user_policy_refusal"]
        response = {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": (
            'Resolving and validating one Agent Plugin package for every selected target...\n'
            'agentplugins: no eligible directory release for "marketing-kit": '
            '777genius/marketing-kit: release does not support complete target set; missing vscode')}
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            env, project, foreign, *_ = runner["run_case"]("unused", "copilot", case, "A")
            peer_env, peer_project, *_ = runner["run_case"]("unused", "copilot", case, "B")
            peer_before = runner["profile_snapshot"](peer_env, peer_project)
            pinned = runner["pin_foreign"]([(foreign, "unmanaged user data\n")], case)
            with patch.dict(verify.__globals__, {"invoke": lambda *_: response,
                    "inventory": lambda *_: ["unexpected"]}):
                with self.assertRaisesRegex(AssertionError, "retained an installation"):
                    verify("agentplugins", env, project, ())
            marker = Path(peer_env["HOME"]) / "unexpected-native.json"

            def mutate_peer(*_):
                marker.write_text("changed")
                return response

            with patch.dict(verify.__globals__, {"invoke": mutate_peer, "inventory": lambda *_: []}):
                with self.assertRaisesRegex(AssertionError, "isolated profile changed"):
                    verify("agentplugins", env, project, (
                        lambda: runner["check_foreign"](pinned, case),
                        lambda: runner["check_profile_unchanged"](
                            peer_env, peer_project, peer_before, "User refusal"),))

    def test_marketing_kit_vscode_user_refusal_rejects_padded_real_capture(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_vscode_user_policy_refusal"]
        diagnostic = ('Resolving and validating one Agent Plugin package for every selected target...\n'
                      'agentplugins: no eligible directory release for "marketing-kit": '
                      '777genius/marketing-kit: release does not support complete '
                      'target set; missing vscode')
        padded_stdout = 'unrelated prefix {"result":"success"}' + " " * 2200
        padded_stderr = "unrelated error\n" + " " * 2200 + diagnostic
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            home, data, project = (case / name for name in ("home", "data", "project"))
            for directory in (home, data, project):
                directory.mkdir(mode=0o700)
            env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
            for stdout, stderr in ((padded_stdout, padded_stderr),
                                   ("", padded_stderr), (padded_stdout, diagnostic)):
                with self.subTest(stdout=bool(stdout), stderr_padded=stderr != diagnostic):
                    command = [sys.executable, "-c", "import sys; "
                               f"sys.stdout.write({stdout!r}); sys.stderr.write({stderr!r}); sys.exit(1)"]
                    captured = runner["audit"]["call"](command, env, project, 10)
                    self.assertEqual(captured["exit_code"], 1)
                    self.assertIsNone(captured["_json"])
                    if stdout:
                        self.assertEqual(captured["stdout_tail"], " " * 1800)
                    if stderr == padded_stderr:
                        self.assertEqual(len(captured["stderr_tail"]), 1800)
                        self.assertTrue(captured["stderr_tail"].startswith(" "))
                    with patch.dict(verify.__globals__, {"invoke": lambda *_: captured,
                            "inventory": lambda *_: []}):
                        with self.assertRaisesRegex(AssertionError, "exact missing-vscode"):
                            verify("agentplugins", env, project, ())

    def test_marketing_kit_vscode_refusal_allows_only_own_finite_directory_cache(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_vscode_user_policy_refusal"]
        response = {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": (
            'Resolving and validating one Agent Plugin package for every selected target...\n'
            'agentplugins: no eligible directory release for "marketing-kit": '
            '777genius/marketing-kit: release does not support complete target set; missing vscode')}
        # Cache contents are authenticated by the genuine released command;
        # this audit constrains additions, ownership, type, mode and identity.
        for mutation in ("valid", "inventory-cache", "scanner", "assessment", "unknown",
                         "nonempty-lock", "symlink", "hardlink", "directory", "fifo", "mode"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                case = Path(temporary)
                env, project, *_ = runner["run_case"]("unused", "copilot", case, "A")
                data = Path(env["AGENTPLUGINS_HOME"])
                checked = []

                def add_cache():
                    path = data / "directory-v1-cache.json"
                    if mutation == "scanner":
                        path = data / runner["REFUSAL_SCANNER_PATH"]
                    elif mutation == "assessment":
                        path = data / "security/assessments" / ("a" * 64 + ".json")
                    elif mutation == "unknown":
                        path = data / "directory-v1-cache.json.tmp"
                    path.parent.mkdir(parents=True, exist_ok=True)
                    if mutation in ("symlink", "hardlink"):
                        original = case / "outside-cache"
                        original.write_bytes(b"signed Directory cache")
                        original.chmod(0o600)
                        try:
                            if mutation == "symlink":
                                path.symlink_to(original)
                            else:
                                os.link(original, path)
                        except (OSError, NotImplementedError) as error:
                            self.skipTest(f"host cannot create link: {error}")
                    elif mutation == "directory":
                        path.mkdir(mode=0o700)
                    elif mutation == "fifo":
                        if not hasattr(os, "mkfifo"):
                            self.skipTest("host cannot create FIFO")
                        os.mkfifo(path, mode=0o600)
                    else:
                        path.write_bytes(b"signed Directory cache")
                        path.chmod(0o644 if mutation == "mode" else 0o600)
                    lock = data / "directory-v1-cache.json.lock"
                    lock.write_bytes(b"unexpected" if mutation == "nonempty-lock" else b"")
                    lock.chmod(0o600)

                def invoke(*_):
                    if mutation != "inventory-cache":
                        add_cache()
                    return response

                def inventory(*_):
                    if mutation == "inventory-cache" and not (data / "directory-v1-cache.json").exists():
                        add_cache()
                    return []

                with patch.dict(verify.__globals__, {"invoke": invoke, "inventory": inventory}):
                    if mutation == "valid":
                        self.assertEqual(verify("agentplugins", env, project,
                            (lambda: checked.append(True),))["result"], "expected_refusal")
                    else:
                        with self.assertRaises(AssertionError):
                            verify("agentplugins", env, project, (lambda: checked.append(True),))
                self.assertEqual(checked, [True])

    def test_marketing_kit_vscode_user_saved_refusal_requires_exact_diagnostic(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_vscode_user_saved_refusal"]
        diagnostic = ('agentplugins: target vscode cannot be saved; retry '
            'agentplugins add marketing-kit --target vscode --no-save')
        accepted = {"exit_code": 1, "timeout": False, "stderr_tail": diagnostic + "\n",
                    "stdout_tail": "", "_json": None}
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            home, data, project = (case / name for name in ("home", "data", "project"))
            for directory in (home, data, project):
                directory.mkdir(mode=0o700)
            (project / "uap.toml").write_text("retained Project agreement")
            (project / "uap.lock").write_text("retained exact Project lock")
            env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
            before = runner["profile_snapshot"](env, project)
            commands, guards = [], []

            def invoke(command, *_):
                commands.append(command)
                return accepted

            with patch.dict(verify.__globals__, {"invoke": invoke,
                    "inventory": lambda *_: []}):
                stage = verify("agentplugins", env, project,
                               (lambda: guards.append("checked"),))
            self.assertEqual(commands, [["agentplugins", "add", "marketing-kit",
                "--target", "vscode", "--scope", "user", "--yes", "--format", "json"]])
            self.assertEqual(stage["scope"], "user")
            self.assertEqual(stage["contract"], "negative_saved_target")
            self.assertEqual(stage["result"], "expected_refusal")
            self.assertNotEqual(stage["result"], "passed")
            self.assertEqual(guards, ["checked"])
            self.assertEqual(runner["profile_snapshot"](env, project), before)
            for ending in ("", "\r\n"):
                with patch.dict(verify.__globals__, {
                        "invoke": lambda *_: {**accepted, "stderr_tail": diagnostic.replace("\n", "\r\n") + ending if ending else diagnostic},
                        "inventory": lambda *_: []}):
                    self.assertEqual(verify("agentplugins", env, project, ())["result"], "expected_refusal")
            for invalid in ({"exit_code": 2}, {"exit_code": 137}, {"exit_code": 0}, {"exit_code": -9}, {"exit_code": None},
                            {"exit_code": True}, {"timeout": True}, {"timeout": None},
                            {"stderr_tail": "network unavailable"},
                            {"stderr_tail": diagnostic.removeprefix("agentplugins: ")},
                            {"stderr_tail": 'agentplugins: no eligible directory release for "marketing-kit": '
                            '777genius/marketing-kit: release does not support complete target set; missing vscode\n'},
                            {"stderr_tail": diagnostic.replace("vscode", "copilot")},
                            {"stderr_tail": diagnostic.replace("marketing-kit", "notion")},
                            {"stderr_tail": diagnostic + "\nremote download failed"},
                            {"stderr_tail": " " + diagnostic},
                            {"stderr_tail": diagnostic + " "},
                            {"stdout_tail": '{"result":"success"}'},
                            {"stdout_tail": " "}, {"stdout_tail": None},
                            {"_json": {"result": "success"}}):
                with self.subTest(invalid=invalid), patch.dict(verify.__globals__, {
                        "invoke": lambda *_: {**accepted, **invalid},
                        "inventory": lambda *_: []}):
                    checked = []
                    with self.assertRaisesRegex(AssertionError, "exact saved-target"):
                        verify("agentplugins", env, project, (lambda: checked.append(True),))
                    self.assertEqual(checked, [True])
                    self.assertEqual(runner["profile_snapshot"](env, project), before)

    def test_marketing_kit_vscode_user_saved_refusal_rejects_own_mutations(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_vscode_user_saved_refusal"]
        response = {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": (
            'agentplugins: target vscode cannot be saved; retry '
            'agentplugins add marketing-kit --target vscode --no-save')}
        for relative in ("home/.copilot/settings.json", "home/foreign.txt", "project/uap.lock",
                         "data/uap.toml", "data/uap.lock", "data/state-v2.json",
                         "data/managed/plugin", "data/store/package", "data/operations-v2/receipt",
                         "data/directory-v1-cache.json"):
            for during_inventory in (False, True):
                with self.subTest(path=relative, inventory=during_inventory), TemporaryDirectory() as temporary:
                    case = Path(temporary)
                    home, data, project = (case / name for name in ("home", "data", "project"))
                    for directory in (home, data, project):
                        directory.mkdir(mode=0o700)
                    env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                    path = case / relative
                    if path.name in ("foreign.txt", "uap.lock", "state-v2.json", "directory-v1-cache.json"):
                        path.write_text("original")
                    checked = []

                    def change():
                        path.parent.mkdir(parents=True, exist_ok=True)
                        path.write_text("unexpected")

                    def invoke(*_):
                        if not during_inventory:
                            change()
                        return response

                    def inventory(*_):
                        if during_inventory:
                            change()
                        return []

                    with patch.dict(verify.__globals__, {"invoke": invoke, "inventory": inventory}):
                        with self.assertRaises(AssertionError):
                            verify("agentplugins", env, project, (lambda: checked.append(True),))
                    self.assertEqual(checked, [True])

    def test_marketing_kit_vscode_user_saved_refusal_checks_inventory_and_peers(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_vscode_user_saved_refusal"]
        response = {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": (
            'agentplugins: target vscode cannot be saved; retry '
            'agentplugins add marketing-kit --target vscode --no-save')}
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            env, project, foreign, *_ = runner["run_case"]("unused", "copilot", case, "A")
            peer_env, peer_project, *_ = runner["run_case"]("unused", "copilot", case, "B")
            peer_before = runner["profile_snapshot"](peer_env, peer_project)
            pinned = runner["pin_foreign"]([(foreign, "unmanaged user data\n")], case)
            with patch.dict(verify.__globals__, {"invoke": lambda *_: response,
                    "inventory": lambda *_: ["unexpected"]}):
                with self.assertRaisesRegex(AssertionError, "retained an installation"):
                    verify("agentplugins", env, project, ())
            marker = Path(peer_env["HOME"]) / "unexpected-native.json"

            def mutate_peer(*_):
                marker.write_text("changed")
                return response

            with patch.dict(verify.__globals__, {"invoke": mutate_peer, "inventory": lambda *_: []}):
                with self.assertRaisesRegex(AssertionError, "isolated profile changed"):
                    verify("agentplugins", env, project, (
                        lambda: runner["check_foreign"](pinned, case),
                        lambda: runner["check_profile_unchanged"](
                            peer_env, peer_project, peer_before, "User refusal"),))

    def test_marketing_kit_vscode_user_saved_refusal_rejects_padded_real_capture(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_vscode_user_saved_refusal"]
        diagnostic = ('agentplugins: target vscode cannot be saved; retry '
            'agentplugins add marketing-kit --target vscode --no-save')
        padded_stdout = 'unrelated prefix {"result":"success"}' + " " * 2200
        padded_stderr = "unrelated error\n" + " " * 2200 + diagnostic
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            home, data, project = (case / name for name in ("home", "data", "project"))
            for directory in (home, data, project):
                directory.mkdir(mode=0o700)
            env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
            for stdout, stderr in ((padded_stdout, padded_stderr),
                                   ("", padded_stderr), (padded_stdout, diagnostic)):
                with self.subTest(stdout=bool(stdout), stderr_padded=stderr != diagnostic):
                    command = [sys.executable, "-c", "import sys; "
                               f"sys.stdout.write({stdout!r}); sys.stderr.write({stderr!r}); sys.exit(1)"]
                    captured = runner["audit"]["call"](command, env, project, 10)
                    self.assertEqual(captured["exit_code"], 1)
                    self.assertIsNone(captured["_json"])
                    if stdout:
                        self.assertEqual(captured["stdout_tail"], " " * 1800)
                    if stderr == padded_stderr:
                        self.assertEqual(len(captured["stderr_tail"]), 1800)
                        self.assertTrue(captured["stderr_tail"].startswith(" "))
                    with patch.dict(verify.__globals__, {"invoke": lambda *_: captured,
                            "inventory": lambda *_: []}):
                        with self.assertRaisesRegex(AssertionError, "exact saved-target"):
                            verify("agentplugins", env, project, ())

    def test_marketing_kit_vscode_saved_refusal_allows_only_own_finite_directory_cache(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_vscode_user_saved_refusal"]
        response = {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": (
            'agentplugins: target vscode cannot be saved; retry '
            'agentplugins add marketing-kit --target vscode --no-save')}
        # Cache contents are authenticated by the genuine released command;
        # this audit constrains additions, ownership, type, mode and identity.
        for mutation in ("valid", "inventory-cache", "scanner", "assessment", "unknown",
                         "nonempty-lock", "symlink", "hardlink", "directory", "fifo", "mode"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                case = Path(temporary)
                env, project, *_ = runner["run_case"]("unused", "copilot", case, "A")
                data = Path(env["AGENTPLUGINS_HOME"])
                checked = []

                def add_cache():
                    path = data / "directory-v1-cache.json"
                    if mutation == "scanner":
                        path = data / runner["REFUSAL_SCANNER_PATH"]
                    elif mutation == "assessment":
                        path = data / "security/assessments" / ("a" * 64 + ".json")
                    elif mutation == "unknown":
                        path = data / "directory-v1-cache.json.tmp"
                    path.parent.mkdir(parents=True, exist_ok=True)
                    if mutation in ("symlink", "hardlink"):
                        original = case / "outside-cache"
                        original.write_bytes(b"signed Directory cache")
                        original.chmod(0o600)
                        try:
                            if mutation == "symlink":
                                path.symlink_to(original)
                            else:
                                os.link(original, path)
                        except (OSError, NotImplementedError) as error:
                            self.skipTest(f"host cannot create link: {error}")
                    elif mutation == "directory":
                        path.mkdir(mode=0o700)
                    elif mutation == "fifo":
                        if not hasattr(os, "mkfifo"):
                            self.skipTest("host cannot create FIFO")
                        os.mkfifo(path, mode=0o600)
                    else:
                        path.write_bytes(b"signed Directory cache")
                        path.chmod(0o644 if mutation == "mode" else 0o600)
                    lock = data / "directory-v1-cache.json.lock"
                    lock.write_bytes(b"unexpected" if mutation == "nonempty-lock" else b"")
                    lock.chmod(0o600)

                def invoke(*_):
                    if mutation != "inventory-cache":
                        add_cache()
                    return response

                def inventory(*_):
                    if mutation == "inventory-cache" and not (data / "directory-v1-cache.json").exists():
                        add_cache()
                    return []

                with patch.dict(verify.__globals__, {"invoke": invoke, "inventory": inventory}):
                    if mutation == "valid":
                        self.assertEqual(verify("agentplugins", env, project,
                            (lambda: checked.append(True),))["result"], "expected_refusal")
                    else:
                        with self.assertRaises(AssertionError):
                            verify("agentplugins", env, project, (lambda: checked.append(True),))
                self.assertEqual(checked, [True])

    def _marketing_kit_info_response(self, runner, distribution, *, prepared_vscode=False, **changes):
        from copy import deepcopy
        release = {"id": distribution, "release_status": "active",
                   **deepcopy(runner["FIXED_SIGNED_CLOSURE"][distribution])}
        if distribution == runner["REQUIRED_DISTRIBUTIONS"]["marketing-kit"]:
            release["target_compatibility"] = [{"client": client, "scopes": ["user"], "delivery": "managed"}
                for client in ("codex", "cursor", "copilot", "claude", "gemini", "opencode")]
        if prepared_vscode:
            release.setdefault("target_compatibility", []).append(
                {"client": "vscode", "scopes": ["user"], "delivery": "prepared"})
        data = {**runner["FIXED_REFUSAL_POLICY"], "selected_distribution": distribution,
                "selected_release": release}
        if prepared_vscode:
            data.update(snapshot_sequence=46, snapshot_digest="sha256:" + "a" * 64)
        data.update(changes)
        return {"exit_code": 0, "timeout": False,
                "_json": {"command": "info", "result": "success", "data": data}}

    def test_marketing_kit_signed_policy_rejects_wrong_publication_or_any_source_tuple(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        inspect = runner["inspect_signed_closure"]
        for mutation in ("sequence", "bool-sequence", "digest", "mixed-policy", "missing-fourth",
                         "timeout", "missing-timeout", "bool-exit", "json", "command", "distribution", "release_sequence",
                         "package_version", "repository", "package_path", "resolved_revision",
                         "tree_digest", "manifest_digest", "stale-playwright", "release_status"):
            with self.subTest(mutation=mutation):
                calls = []
                def invoke(command, *_):
                    distribution = command[2]
                    calls.append(distribution)
                    response = self._marketing_kit_info_response(runner, distribution)
                    # Exercise every immutable tuple field on the last required source.
                    if len(calls) != 4 and mutation not in ("sequence", "bool-sequence", "digest"):
                        return response
                    data = response["_json"]["data"]
                    release = data["selected_release"]
                    if mutation in ("sequence", "mixed-policy"):
                        data["snapshot_sequence"] = 46
                    elif mutation == "bool-sequence":
                        data["snapshot_sequence"] = True
                    elif mutation == "digest":
                        data["snapshot_digest"] = "sha256:" + "a" * 64
                    elif mutation == "missing-fourth":
                        return {"exit_code": 1, "timeout": False, "_json": None}
                    elif mutation == "timeout":
                        response["timeout"] = True
                    elif mutation == "missing-timeout":
                        response.pop("timeout")
                    elif mutation == "bool-exit":
                        response["exit_code"] = False
                    elif mutation == "json":
                        response["_json"] = []
                    elif mutation == "command":
                        response["_json"]["command"] = "add"
                    elif mutation == "distribution":
                        data["selected_distribution"] = "other/plugin"
                    elif mutation == "stale-playwright":
                        release.update(release_sequence=1,
                            resolved_revision="128b01ff2b4f7242ad061ad823d5e4018af6b2e4",
                            tree_digest="sha256:8f0c8ac610974293c4f91f1ecb782fbe9d740f79f84cafe40fdfed1db234d103")
                    elif mutation == "release_sequence":
                        release[mutation] = True
                    else:
                        release[mutation] = "changed"
                    return response
                with patch.dict(inspect.__globals__, {"invoke": invoke}):
                    with self.assertRaises(AssertionError):
                        inspect("agentplugins", {}, Path("."), "codex", runner["FIXED_REFUSAL_POLICY"])
                if mutation not in ("sequence", "bool-sequence", "digest"):
                    self.assertEqual(len(calls), 4)

    def test_marketing_kit_signed_packaging_metadata_is_observed_separately_from_capability(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        inspect = runner["inspect_signed_closure"]
        for mutation in ("valid", "missing", "duplicate", "scope", "non-dict", "managed-vscode", "prepared-copilot"):
            with self.subTest(mutation=mutation):
                def invoke(command, *_):
                    response = self._marketing_kit_info_response(runner, command[2])
                    if command[2] != runner["REQUIRED_DISTRIBUTIONS"]["marketing-kit"]:
                        return response
                    release = response["_json"]["data"]["selected_release"]
                    targets = release["target_compatibility"]
                    targets.append({"client": "vscode", "scopes": ["user"], "delivery": "prepared"})
                    if mutation == "missing":
                        release.pop("target_compatibility")
                    elif mutation == "duplicate":
                        targets.append(dict(targets[-1]))
                    elif mutation == "scope":
                        targets[-1]["scopes"] = ["project"]
                    elif mutation == "non-dict":
                        targets.append("vscode")
                    elif mutation == "managed-vscode":
                        targets[-1]["delivery"] = "managed"
                    elif mutation == "prepared-copilot":
                        next(item for item in targets if item["client"] == "copilot")["delivery"] = "prepared"
                    return response
                with patch.dict(inspect.__globals__, {"invoke": invoke}):
                    if mutation == "valid":
                        observed = inspect("agentplugins", {}, Path("."), "codex")
                        self.assertEqual(observed["root_target_compatibility"]["vscode"],
                                         {"scopes": ["user"], "delivery": "prepared"})
                        self.assertEqual(observed["root_target_compatibility"]["copilot"]["delivery"], "managed")
                        self.assertNotIn("native_editor_runtime", observed)
                    else:
                        with self.assertRaises(AssertionError):
                            inspect("agentplugins", {}, Path("."), "codex")

    def test_marketing_kit_lock_cannot_change_observed_immutable_source_digest(self):
        from copy import deepcopy
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        packages = []
        for alias, distribution in runner["REQUIRED_DISTRIBUTIONS"].items():
            pin = runner["FIXED_SIGNED_CLOSURE"][distribution]
            packages.append({"alias": alias, "source_key": "directory:" + distribution,
                "source": {"registry": {"distribution": distribution}}, "targets": ["copilot"],
                "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"],
                "required_by": [] if alias == "marketing-kit" else [{"alias": "marketing-kit",
                    "selector": runner["REQUIRED_CHILD_SELECTORS"][alias]}]})
        with TemporaryDirectory() as temporary:
            project = Path(temporary)
            for field in (None, "tree_digest", "manifest_digest"):
                with self.subTest(field=field):
                    current = deepcopy(packages)
                    if field is not None:
                        current[-1][field] = "sha256:" + "a" * 64
                    (project / "uap.lock").write_text(json.dumps({"packages": current}))
                    if field is None:
                        self.assertEqual(runner["lock_ids"](project, "copilot",
                            expected_sources=runner["FIXED_SIGNED_CLOSURE"]),
                            {"directory:" + key for key in runner["EXPECTED"]})
                    else:
                        with self.assertRaisesRegex(AssertionError, "observed immutable source"):
                            runner["lock_ids"](project, "copilot",
                                expected_sources=runner["FIXED_SIGNED_CLOSURE"])

    def test_marketing_kit_production_policy_never_accepts_origin_override_or_policy_advance(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        observe = runner["observe_production_policy"]
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            env, project, *_ = runner["run_case"]("unused", "copilot", root, "A")
            injected_grant = False
            def invoke(command, *_):
                response = self._marketing_kit_info_response(runner, command[2])
                if injected_grant and command[2] == runner["REQUIRED_DISTRIBUTIONS"]["marketing-kit"]:
                    response["_json"]["data"]["selected_release"]["target_compatibility"].append(
                        {"client": "other", "scopes": ["user"], "delivery": "managed"})
                return response
            checked = []
            with patch.dict(observe.__globals__, {"invoke": invoke}):
                policy = observe("agentplugins", env, project, "copilot")
                self.assertEqual(policy["source_mode"], "production_default")
                self.assertEqual(set(policy["closure"]), runner["EXPECTED"])
                for origin in (runner["FIXED_REFUSAL_ORIGIN"], "https://unexpected.invalid/"):
                    with self.subTest(origin=origin), self.assertRaisesRegex(AssertionError, "production default"):
                        observe("agentplugins", {**env, "AGENTPLUGINS_DIRECTORY_ORIGIN": origin},
                                project, "copilot", guards=(lambda: checked.append(True),))
                advanced = {**policy, "snapshot_sequence": 46}
                with self.assertRaisesRegex(AssertionError, "policy identity changed"):
                    observe("agentplugins", env, project, "copilot", advanced,
                            (lambda: checked.append(True),))
                injected_grant = True
                with self.assertRaisesRegex(AssertionError, "production policy observation changed"):
                    observe("agentplugins", env, project, "copilot", policy,
                            (lambda: checked.append(True),))
            self.assertEqual(checked, [True, True, True, True])

    def test_marketing_kit_fixed45_warms_only_directory_then_freezes_negative_profile(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_fixed_vscode_user_policy_refusal"]
        for mutation in ("none", "origin", "preexisting-cache", "native", "portable", "state", "scanner",
                         "warm-peer", "negative-peer", "negative-cache"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary)
                env_a, project_a, *_ = runner["run_case"]("unused", "copilot", root, "A")
                peer = runner["profile_snapshot"](env_a, project_a)
                env, project, *_ = runner["run_case"]("unused", "copilot", root, "D")
                env["AGENTPLUGINS_DIRECTORY_ORIGIN"] = (
                    "https://unexpected.invalid/" if mutation == "origin" else runner["FIXED_REFUSAL_ORIGIN"])
                data = Path(env["AGENTPLUGINS_HOME"])
                if mutation == "preexisting-cache":
                    (data / "directory-v1-cache.json").write_text("existing production cache46")
                negative_calls, versions, infos = [], [], []
                def invoke(command, *_):
                    distribution = command[2]
                    infos.append(distribution)
                    if len(infos) == 1:
                        cache = data / "directory-v1-cache.json"
                        cache.write_text("authenticated CLI cache fixture")
                        cache.chmod(0o600)
                        if mutation == "warm-peer":
                            (Path(env_a["HOME"]) / "peer-write").write_text("forbidden")
                        if mutation in ("native", "portable", "state", "scanner"):
                            path = {"native": Path(env["HOME"]) / ".copilot/settings.json",
                                    "portable": data / "uap.toml", "state": data / "state-v2.json",
                                    "scanner": data / runner["REFUSAL_SCANNER_PATH"]}[mutation]
                            path.parent.mkdir(parents=True, exist_ok=True)
                            path.write_text("forbidden")
                    return self._marketing_kit_info_response(runner, distribution)
                def negative(*_):
                    negative_calls.append(True)
                    if mutation == "negative-peer":
                        (Path(env_a["HOME"]) / "peer-write").write_text("forbidden")
                    if mutation == "negative-cache":
                        (data / "directory-v1-cache.json").write_text("replaced")
                    return {"stage": "VSCode negative", "result": "expected_refusal",
                            "reason": "missing_vscode_grant"}
                with patch.dict(verify.__globals__, {
                        "invoke": invoke, "verify_pinned_launcher": lambda *args: versions.append(args),
                        "verify_vscode_user_policy_refusal": negative}):
                    guards = (lambda: runner["check_profile_unchanged"](
                        env_a, project_a, peer, "fixed45 peer"),)
                    if mutation == "none":
                        step = verify("agentplugins", env, project, guards)
                        self.assertEqual(step["policy"]["source_mode"], "fixed_immutable")
                        self.assertEqual(step["policy"]["snapshot_digest"], runner["FIXED_REFUSAL_POLICY"]["snapshot_digest"])
                        self.assertEqual(set(step["policy"]["closure"]), runner["EXPECTED"])
                        self.assertEqual(len(infos), 4)
                        self.assertEqual(len(versions), 1)
                    else:
                        with self.assertRaises(AssertionError):
                            verify("agentplugins", env, project, guards)
                    if mutation in ("origin", "preexisting-cache", "native", "portable", "state", "scanner", "warm-peer"):
                        self.assertEqual(negative_calls, [])

    def test_marketing_kit_copilot_user_requires_one_owner_and_exact_shared_surfaces(self):
        from copy import deepcopy
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        distribution = "777genius/marketing-kit"
        row = self._marketing_kit_public_identity_row(
            runner, distribution, target="copilot", scope="user")
        binding = row["clients"][0]
        self.assertEqual(runner["identities"]([row], "copilot", "user"), {distribution: "owned"})
        for mutation in ("vscode-binding", "duplicate-owner", "missing-surface", "extra-surface", "reordered-surface"):
            with self.subTest(mutation=mutation):
                changed = deepcopy(row)
                if mutation in ("vscode-binding", "duplicate-owner"):
                    changed["clients"].append({**deepcopy(binding), "client_id":
                        "vscode" if mutation == "vscode-binding" else "copilot"})
                else:
                    changed["clients"][0]["affected_surfaces"] = {
                        "missing-surface": ["copilot"], "extra-surface": ["copilot", "vscode", "cursor"],
                        "reordered-surface": ["vscode", "copilot"]}[mutation]
                with self.assertRaises(AssertionError):
                    runner["identities"]([changed], "copilot", "user")

    def test_marketing_kit_saved_capability_requires_exact_raw_refusal_and_current_context(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_current_vscode_user_boundary"]
        expected = ("agentplugins: target vscode cannot be saved; retry agentplugins add "
                    "marketing-kit --target vscode --no-save")
        accepted = {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": expected + "\n", "_json": None}
        for mutation in ({}, {"exit_code": 0}, {"exit_code": 2}, {"exit_code": 137},
                         {"exit_code": -9}, {"exit_code": True}, {"timeout": True},
                         {"stderr_tail": expected + " "}, {"stderr_tail": " " + expected},
                         {"stderr_tail": "Resolving and validating...\n" + expected},
                         {"stderr_tail": "agentplugins: missing vscode"}, {"stdout_tail": " "},
                         {"_json": {"result": "success"}}, {"wrong-context": True}, {"wrong-release": True}):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                env, project, *_ = runner["run_case"]("unused", "copilot", Path(temporary), "E")
                info = lambda command, *_: self._marketing_kit_info_response(runner, command[2], prepared_vscode=True)
                with patch.dict(verify.__globals__, {"invoke": info}):
                    policy = runner["observe_production_policy"]("unused", env, project, "codex")
                if mutation.get("wrong-context"):
                    policy = {**policy, "snapshot_sequence": 47}
                calls, checked = [], []
                def invoke(command, *_):
                    if command[1] == "info":
                        return info(command)
                    calls.append(command)
                    return {**accepted, **mutation}
                def version(*_):
                    if mutation.get("wrong-release"):
                        raise AssertionError("not exact published release")
                with patch.dict(verify.__globals__, {"invoke": invoke,
                        "verify_pinned_launcher": version, "inventory": lambda *_: []}):
                    if not mutation:
                        result = verify("agentplugins", env, project, policy, (lambda: checked.append(True),))
                        self.assertEqual(result["contract"], "saved_management_capability")
                        self.assertEqual(result["policy"], policy)
                        self.assertEqual(result["native_editor_runtime"], "not_qualified")
                        self.assertEqual(calls, [["agentplugins", "add", "marketing-kit", "--target", "vscode",
                            "--scope", "user", "--yes", "--format", "json"]])
                    else:
                        with self.assertRaises(AssertionError):
                            verify("agentplugins", env, project, policy, (lambda: checked.append(True),))
                    self.assertTrue(checked)
                    if mutation.get("wrong-context") or mutation.get("wrong-release"):
                        self.assertEqual(calls, [])

    def test_marketing_kit_current_boundary_rejects_padded_capture_and_profile_mutation(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_current_vscode_user_boundary"]
        diagnostic = ("agentplugins: target vscode cannot be saved; retry agentplugins add "
                      "marketing-kit --target vscode --no-save\n")
        for prepared, mutation in [(prepared, mutation) for prepared in (False, True)
                for mutation in ("padded", "native", "portable", "durable", "cache", "peer", "inventory")]:
            with self.subTest(prepared=prepared, mutation=mutation), TemporaryDirectory() as temporary:
                selected_diagnostic = diagnostic if prepared else (
                    'Resolving and validating one Agent Plugin package for every selected target...\n'
                    'agentplugins: no eligible directory release for "marketing-kit": '
                    '777genius/marketing-kit: release does not support complete target set; missing vscode\n')
                root = Path(temporary)
                peer_env, peer_project, *_ = runner["run_case"]("unused", "copilot", root, "A")
                before = runner["profile_snapshot"](peer_env, peer_project)
                env, project, *_ = runner["run_case"]("unused", "copilot", root, "E")
                data = Path(env["AGENTPLUGINS_HOME"])
                info = lambda command, *_: self._marketing_kit_info_response(runner, command[2], prepared_vscode=prepared)
                with patch.dict(verify.__globals__, {"invoke": info}):
                    policy = runner["observe_production_policy"]("unused", env, project, "codex")
                response = {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": selected_diagnostic, "_json": None}
                def invoke(command, *_):
                    if command[1] == "info":
                        return info(command)
                    if mutation == "padded":
                        code = "import sys; sys.stderr.write(" + repr("unrelated failure" + " " * 2200 + selected_diagnostic) + "); sys.exit(1)"
                        return runner["audit"]["call"]([sys.executable, "-c", code], env, project, 10)
                    path = {"native": Path(env["HOME"]) / ".copilot/settings.json",
                            "portable": data / "uap.toml", "durable": data / "state-v2.json",
                            "cache": data / "directory-v1-cache.json",
                            "peer": Path(peer_env["HOME"]) / "unwanted"}.get(mutation)
                    if path is not None:
                        path.parent.mkdir(parents=True, exist_ok=True)
                        path.write_text("forbidden")
                    return response
                def inventory(*_):
                    if mutation == "inventory":
                        (data / "state-v2.json").write_text("forbidden")
                    return []
                with patch.dict(verify.__globals__, {"invoke": invoke, "verify_pinned_launcher": lambda *_: None,
                        "inventory": inventory}):
                    with self.assertRaises(AssertionError):
                        verify("agentplugins", env, project, policy, (
                            lambda: runner["check_profile_unchanged"](peer_env, peer_project, before, "capability peer"),))

    def test_marketing_kit_current_boundary_rejects_unknown_policy_before_cli_operation(self):
        from copy import deepcopy
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_current_vscode_user_boundary"]
        for prepared in (False, True):
            info = lambda command, *_: self._marketing_kit_info_response(
                runner, command[2], prepared_vscode=prepared)
            with TemporaryDirectory() as temporary:
                env, project, *_ = runner["run_case"]("unused", "copilot", Path(temporary), "E")
                with patch.dict(verify.__globals__, {"invoke": info}):
                    policy = runner["observe_production_policy"]("unused", env, project, "codex")
                # The same shape selects the same oracle at every sequence.
                for sequence in (1, 45, 46, 999):
                    current = {**policy, "snapshot_sequence": sequence}
                    self.assertEqual(runner["current_vscode_boundary_kind"](current),
                        "saved_management_capability" if prepared else "signed_grant_refusal")
                for mutation in ("missing-client", "malformed-extra-client", "scope", "managed-vscode",
                                 "prepared-copilot", "origin", "source-tuple", "bool-sequence"):
                    with self.subTest(prepared=prepared, mutation=mutation):
                        changed = deepcopy(policy)
                        targets = changed["root_target_compatibility"]
                        if mutation == "missing-client":
                            targets.pop("opencode")
                        elif mutation == "malformed-extra-client":
                            targets["unknown"] = {"scopes": ["project"], "delivery": "managed"}
                        elif mutation == "scope":
                            targets["copilot"]["scopes"] = ["project"]
                        elif mutation == "managed-vscode":
                            targets["vscode"] = {"scopes": ["user"], "delivery": "managed"}
                        elif mutation == "prepared-copilot":
                            targets["copilot"]["delivery"] = "prepared"
                        elif mutation == "origin":
                            changed["source_mode"] = "fixed_immutable"
                        elif mutation == "source-tuple":
                            changed["closure"]["777genius/notion"]["manifest_digest"] = "sha256:" + "b" * 64
                        else:
                            changed["snapshot_sequence"] = True
                        calls, checked = [], []
                        with patch.dict(verify.__globals__, {
                                "invoke": lambda *args: calls.append(args),
                                "verify_pinned_launcher": lambda *args: calls.append(args)}):
                            with self.assertRaises(AssertionError):
                                verify("unused", env, project, changed, (lambda: checked.append(True),))
                        self.assertEqual(calls, [])
                        self.assertEqual(checked, [True])

    def test_marketing_kit_current_six_grant_refusal_has_exact_exit_and_no_capability_fallback(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_current_vscode_user_boundary"]
        diagnostic = ('Resolving and validating one Agent Plugin package for every selected target...\n'
            'agentplugins: no eligible directory release for "marketing-kit": '
            '777genius/marketing-kit: release does not support complete target set; missing vscode\n')
        accepted = {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": diagnostic, "_json": None}
        for mutation in ({}, {"exit_code": 2}, {"exit_code": 137}, {"exit_code": True}, {"timeout": True},
                         {"stdout_tail": " "}, {"stderr_tail": diagnostic + " "}, {"_json": {}},
                         {"stderr_tail": "agentplugins: target vscode cannot be saved; retry agentplugins add marketing-kit --target vscode --no-save\n"}):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                env, project, *_ = runner["run_case"]("unused", "copilot", Path(temporary), "E")
                info = lambda command, *_: self._marketing_kit_info_response(runner, command[2])
                with patch.dict(verify.__globals__, {"invoke": info}):
                    policy = runner["observe_production_policy"]("unused", env, project, "codex")
                commands = []
                def invoke(command, *_):
                    commands.append(command)
                    return info(command) if command[1] == "info" else {**accepted, **mutation}
                with patch.dict(verify.__globals__, {"invoke": invoke,
                        "verify_pinned_launcher": lambda *_: None, "inventory": lambda *_: []}):
                    if mutation:
                        with self.assertRaises(AssertionError):
                            verify("unused", env, project, policy, ())
                    else:
                        result = verify("unused", env, project, policy, ())
                        self.assertEqual(result["contract"], "signed_grant_refusal")
                        self.assertEqual(result["policy"], policy)
                self.assertEqual(len([cmd for cmd in commands if cmd[1] == "info"]), 4)
                self.assertTrue(all(cmd[cmd.index("--target") + 1] == "codex"
                                    for cmd in commands if cmd[1] == "info"))
                self.assertEqual([cmd for cmd in commands if cmd[1] == "add"], [[
                    "unused", "add", "marketing-kit", "--target", "vscode", "--scope", "user",
                    "--no-save", "--yes", "--format", "json"]])

    def test_marketing_kit_current_boundary_reauthenticates_each_source_and_prepared_grant(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_current_vscode_user_boundary"]
        for prepared in (False, True):
            for mutation in ("policy", "tuple", "missing-fourth", "warm-native", "warm-peer",
                             "grant", "duplicate-grant", "grant-scope"):
                if not prepared and mutation in ("grant", "duplicate-grant", "grant-scope"):
                    continue
                with self.subTest(prepared=prepared, mutation=mutation), TemporaryDirectory() as temporary:
                    root = Path(temporary)
                    peer_env, peer_project, *_ = runner["run_case"]("unused", "copilot", root, "A")
                    peer_before = runner["profile_snapshot"](peer_env, peer_project)
                    env, project, *_ = runner["run_case"]("unused", "copilot", root, "E")
                    def info(command, *_):
                        return self._marketing_kit_info_response(runner, command[2], prepared_vscode=prepared)
                    with patch.dict(verify.__globals__, {"invoke": info}):
                        policy = runner["observe_production_policy"]("unused", env, project, "codex")
                    calls = []
                    def invoke(command, *_):
                        calls.append(command)
                        self.assertEqual(command[1], "info", "boundary must not run after changed authentication")
                        self.assertEqual(command[command.index("--target") + 1], "vscode" if prepared else "codex")
                        response = info(command)
                        if len(calls) != 4:
                            return response
                        data = response["_json"]["data"]
                        release = data["selected_release"]
                        if mutation == "policy":
                            data["snapshot_sequence"] += 1
                        elif mutation == "tuple":
                            release["manifest_digest"] = "sha256:" + "b" * 64
                        elif mutation == "missing-fourth":
                            return {"exit_code": 1, "timeout": False, "_json": None}
                        elif mutation in ("warm-native", "warm-peer"):
                            path = Path(env["HOME"] if mutation == "warm-native" else peer_env["HOME"]) / "forbidden"
                            path.write_text("unowned write")
                        elif mutation == "grant":
                            release["target_compatibility"][0]["delivery"] = "managed"
                        elif mutation == "duplicate-grant":
                            release["target_compatibility"].append(dict(release["target_compatibility"][0]))
                        else:
                            release["target_compatibility"][0]["scopes"] = ["project"]
                        return response
                    with patch.dict(verify.__globals__, {"invoke": invoke, "verify_pinned_launcher": lambda *_: None}):
                        with self.assertRaises(AssertionError):
                            verify("unused", env, project, policy, (lambda: runner["check_profile_unchanged"](
                                peer_env, peer_project, peer_before, "warm peer"),))
                    self.assertEqual(len(calls), 4)

    def test_marketing_kit_copilot_c_diagnostic_never_reads_late_symlink_replacements(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        if os.open not in os.supports_dir_fd or not hasattr(os, "O_DIRECTORY"):
            self.skipTest("descriptor-relative symlink race requires POSIX openat")
        original_open, original_fdopen = os.open, os.fdopen
        for mutation in ("final-before-open", "ancestor-before-open", "final-after-open"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(strict=True)
                case, foreign_root = root / "case", root / "foreign"
                home = case / "home"
                settings = home / ".copilot/settings.json"
                settings.parent.mkdir(parents=True)
                settings.write_text('{"enabledPlugins": {"marketing-kit@agentplugins-123456abcdef": false}}')
                foreign_root.mkdir()
                foreign = foreign_root / "settings.json"
                foreign.write_text('{"secret": "marketing-kit; must not read this foreign file"}')
                foreign_inode = (foreign.stat().st_dev, foreign.stat().st_ino)
                swapped = []
                def replace():
                    if "ancestor" in mutation:
                        settings.parent.rename(home / "original-copilot")
                        settings.parent.symlink_to(foreign_root, target_is_directory=True)
                    else:
                        settings.unlink()
                        settings.symlink_to(foreign)
                    swapped.append(True)
                def opening(path, flags, *args, **kwargs):
                    name = os.fspath(path)
                    trigger = name == (".copilot" if "ancestor" in mutation else "settings.json") and not swapped
                    if trigger and mutation != "final-after-open":
                        replace()
                    descriptor = original_open(path, flags, *args, **kwargs)
                    if trigger and mutation == "final-after-open":
                        replace()
                    return descriptor
                def reading(descriptor, *args, **kwargs):
                    opened = os.fstat(descriptor)
                    self.assertNotEqual((opened.st_dev, opened.st_ino), foreign_inode,
                                        "foreign descriptor must never reach a read")
                    return original_fdopen(descriptor, *args, **kwargs)
                with patch.object(os, "open", opening), patch.object(os, "fdopen", reading), \
                        patch.object(os, "supports_dir_fd", {*os.supports_dir_fd, opening}):
                    shape = runner["copilot_user_c_settings_shape"]({"HOME": str(home)}, case)
                self.assertTrue(swapped)
                self.assertTrue(all(item["status"] == "unsafe-path" for item in shape.values()))
                self.assertNotIn("must not read", json.dumps(shape))

    def test_marketing_kit_copilot_c_diagnostic_exports_only_redacted_shape(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            env, _project, *_ = runner["run_case"]("unused", "copilot", root, "C")
            settings = Path(env["HOME"]) / ".copilot/settings.json"
            secret = "unexportable-secret-marker-764239"
            settings.write_text(json.dumps({"apiKey": secret, secret: "private-provider",
                "enabledPlugins": {"marketing-kit@agentplugins-123456abcdef": False}}))
            before = settings.read_bytes()
            shape = runner["copilot_user_c_settings_shape"](env, root)
            self.assertEqual(set(shape), runner["UPDATE_ALIASES"])
            self.assertTrue(all(item["status"] == "parsed" for item in shape.values()))
            self.assertEqual(shape["marketing-kit"]["enabled_plugin_value_counts"],
                             {"true": 0, "false": 1, "non_boolean": 0})
            self.assertNotIn(secret, json.dumps(shape))
            self.assertNotIn("private-provider", json.dumps(shape))
            self.assertEqual(settings.read_bytes(), before)

    def test_marketing_kit_copilot_c_diagnostic_accepts_canonical_case_alias(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            actual, alias = root / "actual", root / "alias"
            actual.mkdir()
            try:
                alias.symlink_to(actual, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory aliases: {error}")
            case = alias / "case"
            home = case / "home"
            settings = home / ".copilot/settings.json"
            settings.parent.mkdir(parents=True)
            settings.write_text("{}")
            shape = runner["copilot_user_c_settings_shape"]({"HOME": str(home.resolve(strict=True))}, case)
            self.assertTrue(all(item["status"] == "parsed" for item in shape.values()))

    def test_marketing_kit_copilot_settings_posix_anchors_root_before_outside_alias_swap(self):
        audit = load_audit()
        if os.open not in os.supports_dir_fd or not hasattr(os, "O_NOFOLLOW") or not hasattr(os, "O_DIRECTORY"):
            self.skipTest("descriptor-relative root anchor requires POSIX openat/no-follow")
        with TemporaryDirectory() as temporary:
            base = Path(temporary)
            owned, foreign, alias = base / "owned", base / "foreign", base / "alias"
            for parent, content in ((owned, "{}"), (foreign, '{"secret":"foreign-settings"}')):
                settings = parent / "case/home/.copilot/settings.json"
                settings.parent.mkdir(parents=True)
                settings.write_text(content)
            alias.symlink_to(owned, target_is_directory=True)
            case = alias / "case"
            candidate = case / "home/.copilot/settings.json"
            real_lstat = Path.lstat
            swapped = []
            def lstat(path, *args, **kwargs):
                if path == candidate and not swapped:
                    alias.unlink()
                    alias.symlink_to(foreign, target_is_directory=True)
                    swapped.append(True)
                return real_lstat(path, *args, **kwargs)
            with patch.object(Path, "lstat", lstat), patch.object(os, "fdopen",
                    side_effect=AssertionError("foreign bytes must not be read")):
                content, status = audit.copilot_settings_read(candidate, case)
            self.assertTrue(swapped)
            self.assertIsNone(content)
            self.assertEqual(status, "unsafe-path")

    def test_marketing_kit_copilot_settings_windows_handle_names_and_close_ownership(self):
        import ctypes
        from types import SimpleNamespace
        audit = load_audit()
        self.assertEqual(audit.copilot_settings_windows_name(r"\\?\c:\case\settings.json"), r"C:\case\settings.json")
        self.assertEqual(audit.copilot_settings_windows_name(r"\\?\UNC\server\share\settings.json"),
                         r"\\server\share\settings.json")
        self.assertNotEqual(audit.copilot_settings_windows_name(r"C:\case\settings.json"),
                            audit.copilot_settings_windows_name(r"C:\CASE\settings.json"))
        for outcome in ("valid", "foreign", "zero", "oversized"):
            with self.subTest(outcome=outcome), TemporaryDirectory() as temporary:
                case = Path(temporary).resolve(strict=True)
                home = case / "home"
                path = home / ".copilot/settings.json"
                path.parent.mkdir(parents=True)
                path.write_text('{"enabledPlugins":{"marketing-kit@agentplugins-123456abcdef":false}}')
                held, closed = [], []
                original_open, original_close = os.open, os.close
                def open_member(candidate):
                    fd = original_open(candidate, os.O_RDONLY)
                    held.append(fd)
                    return fd
                def close(fd):
                    closed.append(fd)
                    return original_close(fd)
                def query(handle, buffer, size, flags):
                    self.assertIn(handle, held)
                    self.assertEqual((size, flags), (4101, 0))
                    buffer.value = str(case / "foreign/settings.json" if outcome == "foreign" else path)
                    return 0 if outcome == "zero" else size if outcome == "oversized" else len(buffer.value)
                helpers = {"open_member": open_member,
                           "same_opened": lambda a, b: (a.st_dev, a.st_ino) == (b.st_dev, b.st_ino)}
                with patch.object(os, "supports_dir_fd", set()), patch.object(os, "name", "nt"), \
                        patch.object(ctypes, "WinDLL", return_value=SimpleNamespace(GetFinalPathNameByHandleW=query), create=True), \
                        patch.dict(sys.modules, {"msvcrt": SimpleNamespace(get_osfhandle=lambda fd: fd)}), \
                        patch("runpy.run_path", return_value=helpers), patch.object(os, "close", close):
                    shape = audit.copilot_settings_sanitized_shape(home, case, "marketing-kit")
                self.assertEqual(held, closed)
                self.assertEqual(shape["status"], "parsed" if outcome == "valid" else "unsafe-path")
                if outcome == "valid":
                    self.assertEqual(shape["enabled_plugin_value_counts"]["false"], 1)

    def test_marketing_kit_copilot_reader_fixtures_support_trusted_temporary_root_alias(self):
        # Reproduce macOS /var -> /private/var on any POSIX test host. Only the
        # fixture root is canonicalized, before candidates or races are created.
        from contextlib import contextmanager
        original_directory = TemporaryDirectory
        @contextmanager
        def aliased_directory(**kwargs):
            with original_directory(**kwargs) as temporary:
                base = Path(temporary).resolve(strict=True)
                actual, alias = base / "actual", base / "alias"
                actual.mkdir()
                try:
                    alias.symlink_to(actual, target_is_directory=True)
                except (OSError, NotImplementedError) as error:
                    self.skipTest(f"host cannot create trusted directory alias: {error}")
                yield str(alias)
        methods = (self.test_marketing_kit_copilot_c_diagnostic_never_reads_late_symlink_replacements,
                   self.test_marketing_kit_copilot_settings_windows_handle_names_and_close_ownership)
        for method in methods:
            with self.subTest(fixture=method.__name__), \
                    patch.dict(method.__globals__, {"TemporaryDirectory": aliased_directory}):
                method()

    def test_marketing_kit_copilot_settings_windows_rejects_foreign_ancestor_before_lstat(self):
        import ctypes
        from types import SimpleNamespace
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            case, foreign_root = root / "case", root / "foreign"
            home = case / "home"
            path = home / ".copilot/settings.json"
            saved = home / "original-copilot"
            path.parent.mkdir(parents=True)
            path.write_text("{}")
            foreign_root.mkdir()
            foreign = foreign_root / "settings.json"
            foreign.write_text('{"secret":"foreign-marketing-kit-data"}')
            foreign_info = foreign.stat()
            original_lstat, original_open, original_inside = Path.lstat, os.open, audit.path_inside_case
            opened, closed, queries = [], [], []
            original_close = os.close
            poisoned = []
            in_guard = []
            def poison():
                path.parent.rename(saved)
                path.parent.symlink_to(foreign_root, target_is_directory=True)
            def restore():
                path.parent.unlink()
                saved.rename(path.parent)
            def lstat(candidate, *args, **kwargs):
                if candidate == path and not poisoned and not in_guard:
                    poison()
                    poisoned.append(True)
                return original_lstat(candidate, *args, **kwargs)
            def inside(candidate, root):
                was_poisoned = bool(poisoned)
                if was_poisoned:
                    restore()
                in_guard.append(True)
                try:
                    accepted = original_inside(candidate, root)
                finally:
                    in_guard.pop()
                if was_poisoned:
                    poison()  # All pathname guards see the original owned path.
                return accepted
            def open_member(candidate):
                fd = original_open(candidate, os.O_RDONLY)
                opened.append(fd)
                self.assertEqual((os.fstat(fd).st_dev, os.fstat(fd).st_ino),
                                 (foreign_info.st_dev, foreign_info.st_ino))
                return fd
            def same_opened(before, held):
                self.assertEqual((before.st_dev, before.st_ino), (held.st_dev, held.st_ino))
                return True  # pre-lstat/open/fstat all agree on the foreign inode.
            def query(_handle, buffer, _size, _flags):
                queries.append(True)
                buffer.value = str(foreign)
                return len(buffer.value)
            def close(fd):
                closed.append(fd)
                return original_close(fd)
            with patch.object(Path, "lstat", lstat), patch.object(audit, "path_inside_case", inside), \
                    patch.object(os, "supports_dir_fd", set()), patch.object(os, "name", "nt"), \
                    patch.object(ctypes, "WinDLL", return_value=SimpleNamespace(GetFinalPathNameByHandleW=query), create=True), \
                    patch.dict(sys.modules, {"msvcrt": SimpleNamespace(get_osfhandle=lambda fd: fd)}), \
                    patch("runpy.run_path", return_value={"open_member": open_member, "same_opened": same_opened}), \
                    patch.object(os, "fdopen", side_effect=AssertionError("foreign bytes must never be read")), \
                    patch.object(os, "close", close):
                result = audit.copilot_settings_sanitized_shape(home, case, "marketing-kit")
            self.assertEqual(result, {"status": "unsafe-path"})
            self.assertTrue(queries)
            self.assertEqual(opened, closed)

    def test_marketing_kit_native_cleanup_accepts_canonical_home_only_without_trusting_links(self):
        # macOS has a system /var -> /private/var alias. The native-isolated
        # environment canonicalizes HOME, whereas our TemporaryDirectory may
        # retain the alias. This must not be interpreted as a HOME escape.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            actual = root / "canonical"
            actual.mkdir()
            alias = root / "alias"
            try:
                alias.symlink_to(actual, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            case = alias / "case"
            case.mkdir()
            home, project = case / "home", case / "project"
            home.mkdir()
            project.mkdir()
            for target in ("codex", "copilot"):
                with self.subTest(target=target):
                    verify(target, home.resolve(strict=True), case, "canonical User cleanup")
                    verify(target, project, case, "lexical Project cleanup", project_scope=True)
            # The accepted alias is outside the disposable root. Neither
            # lexical nor canonical *in-root* symlink substitution is allowed.
            poisoned = case / "replaced-home"
            poisoned.symlink_to(home.resolve(strict=True), target_is_directory=True)
            for path in (poisoned, case.resolve(strict=True) / "replaced-home"):
                with self.subTest(suspect=str(path)):
                    with self.assertRaisesRegex(AssertionError, "native HOME left disposable case"):
                        verify("codex", path, case, "unsafe HOME")

    def _marketing_kit_copilot_owner_fixture(self, runner, root):
        env, project, *_ = runner["run_case"]("unused", "copilot", root, "C")
        policy = {"source_mode": "production_default", **runner["FIXED_REFUSAL_POLICY"],
            "root_target_compatibility": {name: {"scopes": ["user"], "delivery": "managed"}
                for name in ("codex", "cursor", "copilot", "claude", "gemini", "opencode")},
            "closure": runner["FIXED_SIGNED_CLOSURE"]}
        owners, installations, public, receipts = {}, [], [], []
        import hashlib
        for alias, distribution in runner["REQUIRED_DISTRIBUTIONS"].items():
            pin = runner["FIXED_SIGNED_CLOSURE"][distribution]
            physical, installation_id, binding_id = "physical-" + alias, "install-" + alias, "binding-" + alias
            target = Path(env["AGENTPLUGINS_HOME"]) / "managed" / physical
            marketplace = "agentplugins-" + hashlib.sha256(physical.encode()).hexdigest()[:12]
            manifest = target / ".github/plugin/marketplace.json"
            manifest.parent.mkdir(parents=True)
            manifest.write_text(json.dumps({"name": marketplace, "plugins": [
                {"name": alias, "version": pin["package_version"], "source": "."}]}))
            revision = {"distribution_id": distribution, "release_sequence": pin["release_sequence"],
                "version": pin["package_version"], **{key: pin[key] for key in ("resolved_revision", "tree_digest", "manifest_digest")}}
            binding = {"client_binding_id": binding_id, "client_id": "copilot", "scope": "user",
                "physical_artifact_id": physical, "target_locator": str(target), "materialization": "materialized",
                "verification": "installation_verified", "affected_surfaces": ["copilot", "vscode"],
                "package_revision": revision, "receipts": [{"client_binding_id": binding_id, "phase": "committed",
                    "operation_id": "operation-" + alias, "active_path": str(target)}]}
            installations.append({"installation_id": installation_id, "origin_mode": "directory",
                "source": {"repository": pin["repository"], "package_subpath": pin["package_path"],
                    "resolved_revision": pin["resolved_revision"], "tree_digest": pin["tree_digest"]},
                "package": {"version": pin["package_version"], "manifest_digest": pin["manifest_digest"]},
                "directory": {"distribution_id": distribution, "desired_release_sequence": pin["release_sequence"],
                    **runner["FIXED_REFUSAL_POLICY"]}, "clients": {binding_id: binding}})
            public.append({"installation_id": installation_id,
                "source": pin["repository"] + "/" + pin["package_path"],
                "directory": {"recorded_distribution": distribution,
                "recorded_revision": pin["resolved_revision"], "recorded_release_sequence": pin["release_sequence"]},
                "clients": [binding]})
            receipts.append({"receipt_id": "receipt-" + alias, "phase": "committed", "environment_id": "user-default",
                "alias": alias, "client_id": "copilot", "installation_id": installation_id,
                "client_binding_id": binding_id, "source_key": "directory:" + distribution,
                "package_digest": pin["tree_digest"]})
            owners[alias] = {"version": pin["package_version"], "marketplace": marketplace,
                "target_locator": str(target), "physical_artifact_id": physical}
        state = {"installations": installations, "environment_receipts": receipts}
        (Path(env["AGENTPLUGINS_HOME"]) / "state-v2.json").write_text(json.dumps(state))
        return env, project, policy, owners, state, public

    def test_marketing_kit_copilot_native_closure_and_disabled_preferences(self):
        from copy import deepcopy
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        native = runner["copilot_native_entries_state"]
        with TemporaryDirectory() as temporary:
            env, project, policy, owners, state, public = self._marketing_kit_copilot_owner_fixture(runner, Path(temporary))
            entries = [{"name": name, "marketplace": owner["marketplace"], "enabled": True,
                        "source": "live", "installedFrom": owner["target_locator"], "version": owner["version"]}
                       for name, owner in sorted(owners.items())]
            baseline = runner["COPILOT_BUILTIN_JSON"]
            self.assertEqual(native(entries + baseline, owners), {"status": "installed", "managed_count": 4})
            self.assertEqual(native(baseline, {}), {"status": "absent", "managed_count": 0})
            for mutation in ("duplicate", "disabled", "version", "version-null", "marketplace", "malformed-market", "case-market", "source", "installed-from", "reminted", "enabled-type", "missing", "extra-foreign", "builtin"):
                changed = deepcopy(entries + baseline)
                if mutation == "duplicate": changed.append(dict(changed[0]))
                elif mutation == "missing": changed.pop(0)
                elif mutation == "extra-foreign": changed.append({"name": "foreign", "marketplace": "", "source": "local", "enabled": True})
                elif mutation == "builtin": changed[-1]["enabled"] = True
                else:
                    key, value = {"disabled": ("enabled", False), "version": ("version", "9.9.9"),
                        "marketplace": ("marketplace", "agentplugins-000000000000"), "malformed-market": ("marketplace", "agentplugins-fake"),
                        "case-market": ("marketplace", "agentplugins-123456ABCDEF"), "source": ("source", "local"),
                        "installed-from": ("installedFrom", "/foreign/source"), "version-null": ("version", None), "reminted": ("name", "reminted"),
                        "enabled-type": ("enabled", "true")}[mutation]
                    changed[0][key] = value
                with self.subTest(mutation=mutation), self.assertRaises(AssertionError): native(changed, owners)

    def test_marketing_kit_copilot_settings_cleanup_needs_independent_native_proof(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            env, project, _policy, owners, _state, _public = self._marketing_kit_copilot_owner_fixture(runner, root)
            home = Path(env["HOME"])
            settings = home / ".copilot/settings.json"
            inactive = {"enabledPlugins": {name + "@" + owner["marketplace"]: False for name, owner in owners.items()}}
            settings.write_text(json.dumps(inactive))
            before = {"status": "absent"}
            active = {"status": "parsed", "allowed_sections": ["enabledPlugins"],
                "enabled_plugin_value_counts": {"true": 1, "false": 0, "non_boolean": 0}}
            shapes = {"before": {name: before for name in owners}, "after_restore": {name: active for name in owners},
                "after_remove": runner["copilot_user_c_settings_shape"](env, root)}
            native_before = {"status": "installed", "managed_count": 4, "profile": str(home.resolve()), "owners": owners}
            native_after = {"status": "absent", "managed_count": 0, "profile": str(home.resolve()), "owners": {}}
            proof = runner["copilot_disabled_closure_only"](shapes, native_before, native_after, env, root)
            self.assertIsInstance(proof, dict)
            check("copilot", home, root, "User C remove", copilot_disabled_proof=proof)
            for unsafe in (None, True, {**proof, "profile": str(root / "other-home")}, {**proof, "settings_digest": "sha256:" + "0" * 64}):
                with self.subTest(proof=unsafe), self.assertRaises(AssertionError):
                    check("copilot", home, root, "User C remove", copilot_disabled_proof=unsafe)
            settings.write_text(json.dumps({**inactive, "extraKnownMarketplaces": {}}))
            proof_with_empty = runner["copilot_disabled_closure_only"](shapes, native_before, native_after, env, root)
            self.assertIsInstance(proof_with_empty, dict)
            check("copilot", home, root, "observed empty marketplace map", copilot_disabled_proof=proof_with_empty)
            for changed in ({**inactive, "unknown": {}}, {**inactive, "extraKnownMarketplaces": {"foreign": {}}},
                            {**inactive, "extraKnownMarketplaces": []}, {**inactive, "extraKnownMarketplaces": False}, {"enabledPlugins": {**inactive["enabledPlugins"], "foreign": False}},
                            {"enabledPlugins": dict.fromkeys(inactive["enabledPlugins"], 0)},
                            {"enabledPlugins": dict.fromkeys(inactive["enabledPlugins"], True)}):
                settings.write_text(json.dumps(changed))
                with self.assertRaises(AssertionError): check("copilot", home, root, "stale proof", copilot_disabled_proof=proof)
                self.assertIsNone(runner["copilot_disabled_closure_only"](shapes, native_before, native_after, env, root))
            settings.write_text(json.dumps(inactive))
            extra = home / ".copilot/marketing-kit"
            extra.mkdir()
            with self.assertRaises(AssertionError): check("copilot", home, root, "extra residue", copilot_disabled_proof=proof)
            extra.rmdir()
            with self.assertRaises(AssertionError): check("copilot", home, root, "Project remove", project_scope=True, copilot_disabled_proof=proof)

    def test_marketing_kit_copilot_signed_version_bindings_accept_one_owner(self):
        from copy import deepcopy
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        read = runner["copilot_signed_user_versions"]
        for mutation in ("list", "dict", "missing", "empty-version", "duplicate", "state-source", "state-revision",
                         "binding-distribution", "binding-sequence", "binding-sequence-bool", "binding-sequence-float",
                         "state-extra-owner", "uncommitted", "receipt-replay", "missing-receipt", "wrong-receipt", "manifest-source", "snapshot"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                env, project, policy, expected, state, rows = self._marketing_kit_copilot_owner_fixture(runner, Path(temporary))
                rows = deepcopy(rows)
                if mutation == "dict":
                    for row in rows: row["clients"] = {"binding": row["clients"][0]}
                elif mutation == "missing": rows.pop()
                elif mutation == "empty-version": rows[0]["clients"][0]["package_revision"]["version"] = ""
                elif mutation == "duplicate": rows[0]["clients"].append(deepcopy(rows[0]["clients"][0]))
                elif mutation == "state-source": state["installations"][0]["source"]["repository"] = "foreign/repo"
                elif mutation == "state-revision": state["installations"][0]["source"]["resolved_revision"] = "f" * 40
                elif mutation.startswith("binding-"):
                    revision = next(iter(state["installations"][0]["clients"].values()))["package_revision"]
                    key, value = {"binding-distribution": ("distribution_id", "foreign/package"),
                                  "binding-sequence": ("release_sequence", 999),
                                  "binding-sequence-bool": ("release_sequence", True),
                                  "binding-sequence-float": ("release_sequence", 1.0)}[mutation]
                    revision[key] = value
                elif mutation == "state-extra-owner":
                    bindings = state["installations"][0]["clients"]
                    bindings["vscode-owner"] = {**next(iter(bindings.values())), "client_id": "vscode"}
                elif mutation == "uncommitted": next(iter(state["installations"][0]["clients"].values()))["receipts"][0]["phase"] = "planned"
                elif mutation == "receipt-replay":
                    receipts = next(iter(state["installations"][0]["clients"].values()))["receipts"]
                    receipts.append(dict(receipts[0]))
                elif mutation == "missing-receipt": state["environment_receipts"].pop()
                elif mutation == "wrong-receipt": state["environment_receipts"][0]["package_digest"] = "sha256:" + "0" * 64
                elif mutation == "snapshot": state["installations"][0]["directory"]["snapshot_digest"] = "sha256:" + "0" * 64
                elif mutation == "manifest-source":
                    target = Path(next(iter(state["installations"][0]["clients"].values()))["target_locator"])
                    file = target / ".github/plugin/marketplace.json"
                    document = json.loads(file.read_text());document["plugins"][0]["source"] = "../foreign";file.write_text(json.dumps(document))
                (Path(env["AGENTPLUGINS_HOME"]) / "state-v2.json").write_text(json.dumps(state))
                with patch.dict(read.__globals__, {"inventory": lambda *_: rows}):
                    if mutation in ("list", "dict"):
                        self.assertEqual(read("cli", env, project, policy), expected)
                    else:
                        with self.assertRaises(AssertionError): read("cli", env, project, policy)

    def test_marketing_kit_copilot_full_native_text_binds_sources_and_rejects_tails(self):
        from copy import deepcopy
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        read = runner["copilot_native_scope_state"]
        for mutation in ("valid", "tail-only", "wrong-source", "wrong-version", "duplicate", "extra", "stderr", "timeout", "exit", "own-mutation", "invalid-json"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                env, project, _policy, owners, _state, _rows = self._marketing_kit_copilot_owner_fixture(runner, Path(temporary))
                for owner in owners.values(): owner["target_locator"] += "/" + "deep-native-directory-" * 32
                settings = Path(env["HOME"]) / ".copilot/settings.json"
                settings.write_text(json.dumps({"enabledPlugins": {name + "@" + o["marketplace"]: True for name, o in owners.items()}}))
                entries = [{"name": name, "marketplace": o["marketplace"], "enabled": True,
                    "source": "live", "installedFrom": o["target_locator"], "version": o["version"]} for name, o in owners.items()] + runner["COPILOT_BUILTIN_JSON"]
                lines = [runner["COPILOT_BUILTIN_TEXT"] + "Live Plugins (loaded from a local marketplace directory, never copied):"]
                for name, owner in owners.items():
                    lines.extend([f"  • {name}@{owner['marketplace']} (v{owner['version']}) (enabled)", "      from " + owner["target_locator"]])
                text = "\n".join(lines) + "\n"
                self.assertGreater(len(text), 1800)
                if mutation == "wrong-source": text = text.replace("      from ", "      from /foreign/", 1)
                elif mutation == "wrong-version": text = text.replace("(v0.1.0)", "(v9.9.9)", 1)
                elif mutation == "duplicate": text += "\n".join(lines[1:3]) + "\n"
                elif mutation == "extra": text += "unexpected output\n"
                calls = []
                def call(argv, call_env, *_args, **kwargs):
                    self.assertTrue(kwargs.get("capture_full"));self.assertEqual(call_env["COPILOT_HOME"], str(Path(env["HOME"]) / ".copilot"))
                    calls.append(argv)
                    body = json.dumps(entries) if "--json" in argv else text
                    response = {"exit_code": 0, "timeout": False, "stdout_full": body, "stderr_full": "", "stdout_tail": body[-1800:]}
                    if mutation == "tail-only": response.pop("stdout_full")
                    elif mutation == "stderr": response["stderr_full"] = "warning"
                    elif mutation == "timeout": response["timeout"] = True
                    elif mutation == "exit": response["exit_code"] = 2
                    elif mutation == "own-mutation": (Path(env["AGENTPLUGINS_HOME"]) / "unexpected").write_text("changed")
                    elif mutation == "invalid-json" and "--json" in argv: response["stdout_full"] = '[{"a":NaN}]'
                    return response
                with patch.dict(runner["audit"]["AGENT_BINARIES"], {"copilot": str(Path(temporary) / "trusted-native")}),                         patch.dict(runner["audit"], {"call": call}):
                    if mutation == "valid":
                        result = read(env, project, owners);self.assertEqual(result["status"], "installed");self.assertEqual(len(calls), 2)
                    else:
                        with self.assertRaises((AssertionError, ValueError)): read(env, project, owners)
                with self.assertRaises(ValueError): runner["copilot_json_document"]('{"a":1,"a":2}')

    def test_marketing_kit_native_full_capture_is_complete_bounded_and_utf8(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            env = {**os.environ, "TEMP": temporary}
            for text, success in (("x" * 3000, True), ("x" * ((1 << 20) + 1), False)):
                result = audit.call([sys.executable, "-c", "import sys;sys.stdout.write('x'*" + str(len(text)) + ")"], env, temporary, 10, capture_full=True)
                self.assertEqual(result.get("stdout_full"), text if success else None)
                if success: self.assertEqual(result["exit_code"], 0)
                else:
                    self.assertIsNone(result["exit_code"])
                    self.assertEqual(result["stderr_tail"], "bounded native output capture exceeded")
            result = audit.call([sys.executable, "-c", "import sys;sys.stdout.buffer.write(bytes([255]))"], env, temporary, 10, capture_full=True)
            self.assertIsNone(result["exit_code"])
            self.assertEqual(result["stderr_tail"], "native output capture was not UTF-8")

    def test_marketing_kit_copilot_read_allows_only_bounded_new_vendor_process_logs(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        check = runner["check_copilot_read_unchanged"]
        for mutation in ("two-new", "public-query-log", "too-many", "unknown-name", "old-overwrite", "replacement",
                         "symlink", "hardlink", "too-large", "other-path", "state", "new-directory", "nested-log"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary)
                env, project, *_ = runner["run_case"]("unused", "copilot", root, "C")
                logs = Path(env["HOME"]) / ".copilot/logs"
                logs.mkdir()
                old = logs / "process-100-100.log"
                old.write_text("preexisting foreign log must stay exact")
                (logs / "preexisting-subdirectory").mkdir()
                before = runner["profile_snapshot"](env, project)
                path = logs / "process-200-200.log"
                if mutation == "old-overwrite": old.write_text("changed")
                elif mutation == "replacement": old.rename(logs / "replacement.log");old.write_text("preexisting foreign log must stay exact")
                elif mutation == "symlink":
                    foreign = root / "foreign.log";foreign.write_text("private");path.symlink_to(foreign)
                elif mutation == "hardlink":
                    foreign = root / "foreign.log";foreign.write_text("private");os.link(foreign, path)
                elif mutation == "too-large": path.write_bytes(b"x" * ((1 << 20) + 1))
                elif mutation == "other-path": (Path(env["HOME"]) / "unexpected.log").write_text("new")
                elif mutation == "state": (Path(env["AGENTPLUGINS_HOME"]) / "state-v2.json").write_text("new")
                elif mutation == "new-directory": (logs / "new-directory").mkdir()
                elif mutation == "nested-log": (logs / "preexisting-subdirectory/process-200-200.log").write_text("new")
                else:
                    for index in range(3 if mutation == "too-many" else 2):
                        (logs / (f"unknown-{index}.log" if mutation == "unknown-name" else f"process-200-{200 + index}.log")).write_text("new vendor log")
                if mutation == "two-new": check(env, project, before, "read", 2)
                else:
                    with self.assertRaises(AssertionError): check(env, project, before, "read", 0 if mutation == "public-query-log" else 2)

    def test_marketing_kit_copilot_new_log_snapshot_handles_windows_separators(self):
        from pathlib import PureWindowsPath
        from types import SimpleNamespace
        import stat
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        check = runner["check_copilot_read_unchanged"]
        class WindowsSnapshotPath(PureWindowsPath):
            def resolve(self, **_): return self
            def lstat(self): return SimpleNamespace(st_mode=stat.S_IFREG | 0o600, st_nlink=1, st_size=197)
        before = {"metadata:0:.copilot/logs": (stat.S_IFDIR, 1, 2)}
        after = {**before, "home/.copilot\\logs\\process-123-456.log": "file-digest",
                 "metadata:0:.copilot/logs/process-123-456.log": (stat.S_IFREG, 1, 3)}
        shared_check = runner["audit"]["check_copilot_snapshot_unchanged"]
        with patch.dict(check.__globals__, {"Path": WindowsSnapshotPath,
                "profile_snapshot": lambda *_: dict(after)}), patch.dict(shared_check.__globals__, {
                "Path": WindowsSnapshotPath, "os": SimpleNamespace(sep="\\"),
                "path_inside_case": lambda *_: True}):
            check({"HOME": "C:/case/home"}, WindowsSnapshotPath("C:/case/project"), before, "Windows snapshot", 2)
            with self.assertRaises(AssertionError):
                check({"HOME": "C:/case/home"}, WindowsSnapshotPath("C:/case/project"), before, "zero-budget inventory", 0)

    def test_marketing_kit_native_pair_reanchors_before_text_and_final_reads(self):
        from runpy import run_path
        import shutil
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        read = runner["copilot_native_scope_state"]
        for mutation in ("unchanged", "trusted-alias", "home", "data", "project", "profile", "case"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(strict=True)
                base = root / "canonical"
                base.mkdir()
                alias = root / "alias"
                try:
                    alias.symlink_to(base, target_is_directory=True)
                except (OSError, NotImplementedError) as error:
                    self.skipTest(f"host cannot create directory symlinks: {error}")
                chosen = alias if mutation == "trusted-alias" else base
                run_root = chosen / "runs"
                run_root.mkdir()
                env, project, *_ = runner["run_case"]("unused", "copilot", run_root, "A")
                home, data = Path(env["HOME"]), Path(env["AGENTPLUGINS_HOME"])
                victim = {"home": home, "data": data, "project": project,
                          "profile": home / ".copilot", "case": project.parent}.get(mutation)
                outside, saved = root / "outside", root / "saved-original"
                if victim is not None:
                    shutil.copytree(victim, outside)
                    (outside / "foreign-sentinel.txt").write_text("must never read outside bytes")
                native = str(root / "trusted-copilot")
                calls, outside_reads = [], []
                opened = Path.open
                redirected = False

                def checked_open(path, *args, **kwargs):
                    mode = kwargs.get("mode", args[0] if args else "r")
                    if (redirected and str(mode).startswith("r")
                            and path.resolve().is_relative_to(outside)):
                        outside_reads.append(str(path))
                        raise AssertionError("outside content read after native redirect")
                    return opened(path, *args, **kwargs)

                def query(argv, *_args, **kwargs):
                    nonlocal redirected
                    self.assertTrue(kwargs.get("capture_full"))
                    calls.append(argv)
                    if len(calls) == 1 and victim is not None:
                        victim.rename(saved)
                        victim.symlink_to(outside, target_is_directory=True)
                        redirected = True
                    text = (json.dumps(runner["COPILOT_BUILTIN_JSON"]) if "--json" in argv
                            else runner["COPILOT_EMPTY_TEXT"] + runner["COPILOT_BUILTIN_TEXT"])
                    return {"exit_code": 0, "timeout": False,
                            "stdout_full": text, "stderr_full": ""}

                try:
                    with (patch.dict(runner["audit"]["AGENT_BINARIES"], {"copilot": native}, clear=True),
                          patch.dict(runner["audit"], {"call": query}),
                          patch.object(Path, "open", checked_open)):
                        if victim is None:
                            self.assertEqual(read(env, project, {})["status"], "absent")
                            self.assertEqual(len(calls), 2)
                        else:
                            with self.assertRaisesRegex(AssertionError, "redirected"):
                                read(env, project, {})
                            self.assertEqual(calls, [[native, "plugin", "list", "--json"]])
                        self.assertEqual(outside_reads, [])
                finally:
                    if victim is not None and saved.exists():
                        if victim.is_symlink():
                            victim.unlink()
                        saved.rename(victim)

    def test_marketing_kit_outer_native_failure_guards_roots_before_content_checks(self):
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            probe = root / "link-probe"
            try:
                probe.symlink_to(root, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            probe.unlink()
        for case, peers in (("A", ("B", "C")), ("C", ("A", "B"))):
            with self.subTest(case=case):
                code, report, phases = self._exercise_marketing_kit_scope_report(
                    "copilot", native_root_redirect=case)
                self.assertEqual(code, 1)
                self.assertFalse(report["two_scope_lifecycle_qualified"])
                self.assertIn("redirected", report["failure"])
                after = phases[phases.index("native-root-redirect:" + case) + 1:]
                self.assertIn("guard-foreign-after-redirect", after)
                for peer in peers:
                    self.assertIn("guard-root:" + peer, after)
                    self.assertIn("snapshot-after-redirect:" + peer, after)
                self.assertNotIn("snapshot-after-redirect:" + case, after)
                self.assertFalse(any(item.startswith("outside-read:") for item in after))
                if case == "A":
                    self.assertGreaterEqual(after.count("guard-root:A"), 2)
                    self.assertNotIn("project-read-after-redirect:A", after)
                else:
                    self.assertIn("project-read-after-redirect:A", after)

    def test_marketing_kit_copilot_native_proof_failure_checks_peers_for_A_and_C(self):
        for case in ("A", "C"):
            for installed in (False, True):
                with self.subTest(case=case, installed=installed):
                    code, report, _phases = self._exercise_marketing_kit_scope_report(
                        "copilot", native_error=(case, installed), native_peer_mutation=True)
                    self.assertEqual(code, 1)
                    self.assertFalse(report["two_scope_lifecycle_qualified"])
                    self.assertEqual(report["status"], "failed")
                    self.assertIn("peer", report["failure"])

    def _exercise_marketing_kit_scope_report(self, target, *, cleanup_failure=False,
                                              boundary_error=False, grants=7, native_error=None, native_peer_mutation=False,
                                              final_policy_error=False, final_peer_mutation=None,
                                              native_root_redirect=None, project_remove_error=None,
                                              codex_checkpoint_error=None, codex_constructor_error=None,
                                              require_copilot_trust=False, copilot_trust_mutation=None,
                                              copilot_fixture_failure=None, claude_retained_scan_error=False,
                                              temporary_root_alias=False):
        from io import StringIO
        from runpy import run_path
        from types import SimpleNamespace

        if target == "copilot" and not COPILOT_TRUST_FIXTURE_SUPPORTED:
            self.skipTest("Marketing Copilot orchestration uses its Ubuntu POSIX trust fixture")
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        main = runner["main"]
        installed, phases = {}, []
        retirements = []
        sqlite_profiles = {}

        # This orchestration fixture already substitutes every public/native
        # lifecycle. Real SQLite custody has separate real-database regressions.
        def prepare_sqlite(name, _env, _project):
            if target == "codex":
                self.assertIn(name, ("A", "B", "C"))
                sqlite_profiles[name] = {"runtime_semantics_unchanged": True}

        mutation_checkpoints = []
        sqlite_orchestration = SimpleNamespace(
            prepare=prepare_sqlite, checkpoint=mutation_checkpoints.append,
            call=lambda _phase, action: action(), evidence=lambda: dict(sqlite_profiles))
        codex_profiles = []
        codex_failures = []
        outside_roots = []
        original_open = Path.open
        original_profile = runner["profile_snapshot"]
        original_foreign = runner["check_foreign"]
        original_project = runner["check_user_project_unchanged"]
        original_peer = runner["check_profile_unchanged"]
        original_guard = runner["audit"]["copilot_native_roots_guard"]
        original_trust = runner.get("prepare_copilot_project_b_trust")
        trust_failures = []

        def prepare_trust(env, project, root):
            proof, guard = original_trust(env, project, root)
            if copilot_fixture_failure:
                trust_failures.append(copilot_fixture_failure)
                if copilot_fixture_failure == "trust":
                    (Path(env["HOME"]) / ".copilot/config.json").write_text('{"trustedFolders": []}')
                else:
                    (root / "A/home/foreign-user-do-not-change.txt").write_text("foreign mutation")
            return proof, guard

        def checked_open(path, *args, **kwargs):
            mode = kwargs.get("mode", args[0] if args else "r")
            if (str(mode).startswith("r")
                    and any(path.resolve().is_relative_to(root) for root in outside_roots)):
                phases.append("outside-read:" + path.name)
                raise AssertionError("outside content read after native redirect")
            return original_open(path, *args, **kwargs)

        def profile(env, project):
            if outside_roots:
                phases.append("snapshot-after-redirect:" + project.parent.name)
            return original_profile(env, project)

        def foreign(*args):
            if trust_failures:
                phases.append("guard-foreign-after-trust-failure")
            if outside_roots:
                phases.append("guard-foreign-after-redirect")
            if codex_failures:
                phases.append("guard-foreign-after-codex-failure")
            return original_foreign(*args)

        def project_check(project, *args):
            if outside_roots:
                phases.append("project-read-after-redirect:" + project.parent.name)
            if codex_failures:
                phases.append("guard-project:" + project.parent.name + ":" + args[-1])
            return original_project(project, *args)

        def peer_check(env, project, before, stage):
            if trust_failures:
                phases.append("guard-peer-after-trust-failure:" + project.parent.name)
            if codex_failures:
                phases.append("guard-peer:" + project.parent.name + ":" + stage)
            return original_peer(env, project, before, stage)

        def make_guard(home, env, project):
            captured = original_guard(home, env, project)
            def guard():
                if outside_roots:
                    phases.append("guard-root:" + project.parent.name)
                captured()
            return guard

        class CheckedCleanup:
            def __init__(self, **kwargs):
                self.parent = TemporaryDirectory() if temporary_root_alias else None
                if self.parent is not None:
                    parent = Path(self.parent.name).resolve(strict=True)
                    owned = parent / "owned"
                    owned.mkdir()
                    self.alias_parent = parent / "alias"
                    self.alias_parent.symlink_to(owned, target_is_directory=True)
                    kwargs["dir"] = owned
                self.inner = TemporaryDirectory(**kwargs)
                self.alias_directory = None

            def __enter__(self):
                location = self.inner.__enter__()
                if not temporary_root_alias:
                    return location
                self.alias_directory = TemporaryDirectory(prefix="marketing-root-alias-")
                alias = Path(self.alias_directory.name).resolve() / "temporary-parent"
                alias.symlink_to(Path(location).resolve().parent, target_is_directory=True)
                phases.append("temporary-root-alias")
                return str(alias / Path(location).name)

            def __exit__(self, exc_type, exc, traceback):
                try:
                    if retirements:
                        self_test.assertTrue(all(retirement.closed for retirement in retirements),
                                             "custody descriptors must close before disposable tree cleanup")
                        phases.append("claude-closed-before-disposal")
                    self.inner.__exit__(exc_type, exc, traceback)
                finally:
                    if self.alias_directory is not None:
                        self.alias_directory.cleanup()
                if exc_type is None and cleanup_failure:
                    raise OSError("disposable cleanup failed")

        class FakeCodexProfile:
            # This fixture checks orchestration; native custody is exercised
            # independently with real filesystem and descriptor controls.
            def __init__(self, audit, root, env, project):
                self.home, self.project, self.checkpoints = Path(env["HOME"]), project, []
                if project.parent.name == codex_constructor_error:
                    codex_failures.append("constructor")
                    phases.append("codex-constructor-refused:" + project.parent.name)
                    raise AssertionError("injected Codex profile constructor failure")
                codex_profiles.append(self)

            def checkpoint(self, phase):
                self.checkpoints.append(phase)
                refusing = (codex_checkpoint_error is not None and self.project.parent.name == "A"
                            and phase in ("before-" + codex_checkpoint_error,
                                          "after-" + codex_checkpoint_error))
                if refusing:
                    codex_failures.append("checkpoint")
                if codex_failures:
                    phases.append("codex-checkpoint:" + self.project.parent.name + ":" + phase)
                if refusing:
                    raise AssertionError("injected Codex checkpoint failure")

            def guard(self):
                pass

            def system_files(self):
                return {}

            def evidence(self):
                return {"status": "orchestration-fixture", "checkpoints": len(self.checkpoints)}

        class FakeRetirement:
            def __init__(self, root, home, project, public, pins, audit, *, checkpoint_diagnostics=None):
                self.case, self.home, self.project = root, home, project
                self.public, self.closed = dict(public), False
                self.finished = False
                self.assertions = self_test
                self.assertions.assertEqual(set(public), runner["EXPECTED"])
                self.assertions.assertEqual(pins, runner["FIXED_SIGNED_CLOSURE"])
                phases.append("claude-capture:" + project.parent.name)
                retirements.append(self)

            def finish(self, cleanup, residuals, rescan, *, keep_open=False):
                self.assertions.assertFalse(self.closed)
                self.assertions.assertEqual(cleanup["scope"], "project")
                self.assertions.assertEqual(cleanup["result"], "success")
                self.assertions.assertEqual(cleanup["outcome"], "completed")
                self.assertions.assertEqual({row["installation_id"] for row in cleanup["results"]}, set(self.public.values()))
                self.assertions.assertEqual(len(cleanup["actions"]), 4)
                self.assertions.assertEqual(set(residuals), set(rescan()))
                phases.append("claude-finish:" + self.project.parent.name)
                self.finished = True
                self.assertions.assertEqual(keep_open, self.project.parent.name == "A")
                if not keep_open:
                    self.close()
                return {"status": "orchestration-fixture", "writer_closure_claimed": False}

            def checkpoint(self):
                self.assertions.assertTrue(self.finished)
                self.assertions.assertFalse(self.closed)

            def verify_retained(self, home, case_root, residuals, rescan):
                self.checkpoint()
                self.assertions.assertEqual((home, case_root), (self.home, self.case))
                self.assertions.assertEqual(self.project.parent.name, "A")
                phases.append("claude-retained-user-A")
                if claude_retained_scan_error:
                    raise AssertionError("injected retained raw scan failure")
                self.assertions.assertEqual(set(residuals), set(rescan()))
                return {"status": "orchestration-retained-fixture", "writer_closure_claimed": False}

            def close(self):
                if not self.closed:
                    phases.append("claude-close:" + self.project.parent.name)
                self.closed = True

        self_test = self

        mutation_labels = []
        def execute(_cli, env, project, _target, label, *args, checkpoint=None, capture=None):
            phases.append(label)
            mutation_labels.append("after-public-mutation-" + label)
            self.assertIs(checkpoint, sqlite_orchestration.checkpoint)
            scope = args[args.index("--scope") + 1]
            key = (env["AGENTPLUGINS_HOME"], scope)
            portable = project if scope == "project" else Path(env["AGENTPLUGINS_HOME"])
            removal_receipt = None
            try:
                if require_copilot_trust:
                    config = Path(env["HOME"]) / ".copilot/config.json"
                    if label == "cold locked restore":
                        if not config.exists():
                            raise AssertionError("Copilot selected Project B has no remembered trust")
                        self.assertEqual(json.loads(config.read_text()), {"trustedFolders": [str(project)]})
                        self.assertNotIn(key, installed)
                        self.assertEqual(list(Path(env["AGENTPLUGINS_HOME"]).iterdir()), [])
                    elif project.parent.name in ("A", "C") and args[0] == "add":
                        self.assertFalse(config.exists(), "fixture must not preauthorize A or User C")
                if label == copilot_trust_mutation:
                    (Path(env["HOME"]) / ".copilot/config.json").write_text('{"trustedFolders": []}')
                if args[0] in ("add", "install"):
                    for name in ("uap.toml", "uap.lock"):
                        if not (portable / name).exists():
                            (portable / name).write_text("retained portable agreement")
                    installed[key] = {name: f"{scope}:{project.parent.name}:{name}"
                                      for name in runner["EXPECTED"]}
                elif args[0] == "remove":
                    if label == project_remove_error:
                        raise AssertionError("injected Project removal failure")
                    if capture is not None:
                        prior = installed[key]
                        actions = [{"alias": alias, "action": "remove", "client_id": _target,
                                    "installation_id": prior[distribution]}
                                   for alias, distribution in runner["REQUIRED_DISTRIBUTIONS"].items()]
                        removal_receipt = {"result": "success", "outcome": "completed", "scope": scope,
                                 "actions": actions, "results": [
                                     {"operation": "remove", "client_id": _target,
                                      "installation_id": row["installation_id"], "outcome": "completed"}
                                     for row in actions]}
                    installed[key] = {}
            finally:
                checkpoint("after-public-mutation-" + label)
            if capture is not None and removal_receipt is not None:
                self.assertEqual(mutation_checkpoints[-1], "after-public-mutation-" + label)
                capture(removal_receipt)
            return {"stage": label, "exit_code": 0, "result": "passed"}

        def boundary(_cli, env, project, expected, _guards):
            self.assertEqual(project.parent.name, "E")
            self.assertFalse((Path(env["HOME"]) / ".copilot/config.json").exists())
            self.assertNotIn("AGENTPLUGINS_DIRECTORY_ORIGIN", env)
            self.assertFalse((Path(env["AGENTPLUGINS_HOME"]) / "uap.lock").exists())
            kind = runner["current_vscode_boundary_kind"](expected)
            policy = observe(_cli, env, project, "vscode" if grants == 7 else "codex", expected)
            phases.append("vscode-" + kind)
            if boundary_error:
                raise AssertionError("required current boundary failed")
            return {"stage": "VSCode current boundary", "result": "expected_refusal",
                    "contract": kind, "reason": "unsupported_saved_target" if grants == 7 else "missing_vscode_grant",
                    "policy": policy}

        def observe(_cli, env, _project, _target, expected=None, guards=()):
            self.assertNotIn("AGENTPLUGINS_DIRECTORY_ORIGIN", env)
            if require_copilot_trust and _project.parent.name == "B":
                self.assertEqual(json.loads((Path(env["HOME"]) / ".copilot/config.json").read_text()),
                                 {"trustedFolders": [str(_project)]})
                self.assertEqual(list(Path(env["AGENTPLUGINS_HOME"]).iterdir()), [])
                phases.append("preauthorized-trust:B")
            # Actual released info selects retained removed installation records
            # first; those records have no catalog snapshot_sequence/digest.
            if any(key[0] == env["AGENTPLUGINS_HOME"] for key in installed):
                raise AssertionError("signed Directory policy identity invalid")
            phases.append("catalog:" + _project.parent.name)
            if _project.parent.name == "E":
                if final_peer_mutation:
                    (_project.parent.parent / final_peer_mutation / "home/final-query-write").write_text("tampered")
                if final_policy_error:
                    raise AssertionError("production policy observation changed")
            response = self._marketing_kit_info_response(runner,
                runner["REQUIRED_DISTRIBUTIONS"]["marketing-kit"], prepared_vscode=grants == 7)
            return {"source_mode": "production_default", "snapshot_sequence": 46,
                    "snapshot_digest": "sha256:" + "a" * 64,
                    "root_target_compatibility": {item["client"]: {"scopes": item["scopes"], "delivery": item["delivery"]}
                        for item in response["_json"]["data"]["selected_release"]["target_compatibility"]},
                    "closure": runner["FIXED_SIGNED_CLOSURE"]}

        def inventory(_cli, env, _project, _target, scope="project"):
            return installed.get((env["AGENTPLUGINS_HOME"], scope), {})

        def native_scope(env, project, owners):
            if native_root_redirect == project.parent.name and owners:
                import shutil
                case = project.parent
                outside = case.parent / ("outside-native-" + case.name)
                shutil.copytree(case, outside)
                (outside / "foreign-sentinel.txt").write_text("must not read outside bytes")
                saved = case.parent / ("saved-native-" + case.name)
                case.rename(saved)
                case.symlink_to(outside, target_is_directory=True)
                outside_roots.append(outside)
                phases.append("native-root-redirect:" + case.name)
                raise AssertionError("native protected root redirected")
            if native_error == (project.parent.name, bool(owners)):
                if native_peer_mutation:
                    (project.parent.parent / "B/home/forbidden-native-write").write_text("tampered")
                raise AssertionError("native proof failed")
            return {"status": "installed" if owners else "absent", "managed_count": len(owners),
                    "profile": str(Path(env["HOME"]).resolve()), "owners": owners}

        with TemporaryDirectory() as output:
            report_path = Path(output) / "report.json"
            with patch.dict(main.__globals__, {
                    "execute": execute, "verify_pinned_launcher": lambda *_: None,
                    "MarketingCodexSQLiteProfiles": lambda _: sqlite_orchestration,
                    "CodexProfile": FakeCodexProfile, "SignedClosureRetirement": FakeRetirement,
                    "prepare_copilot_project_b_trust": prepare_trust,
                    "profile_snapshot": profile, "check_foreign": foreign,
                    "check_user_project_unchanged": project_check,
                    "check_profile_unchanged": peer_check,
                    "inventory": inventory, "identities": lambda items, *_: items,
                    "lock_ids": lambda *_, **__: {"directory:" + name for name in runner["EXPECTED"]},
                    "settled_profile_snapshot": lambda e, p, **kwargs: (
                        kwargs["guard"]() if kwargs.get("guard") else None,
                        runner["profile_snapshot"](e, p))[1],
                    "copilot_signed_user_versions": lambda *_: {name: {"version": "0.1.0", "marketplace": "agentplugins-123456abcdef", "target_locator": "unused"} for name in runner["UPDATE_ALIASES"]},
                    "copilot_native_scope_state": native_scope,
                    "verify_corrupt_lock_refusal": lambda *_: 1,
                    "verify_fixed_vscode_user_policy_refusal": lambda *_: self.fail("historical45 is not a current gate"),
                    "verify_current_vscode_user_boundary": boundary,
                    "observe_production_policy": observe,
                    "codex_user_native_remove_attestation": lambda *_: ("--external-uninstalled",),
                    # Patch the runner binding, preserving tempfile for setup helpers.
                    "tempfile": SimpleNamespace(TemporaryDirectory=CheckedCleanup),
                    }), \
                    patch.dict(runner["audit"], {"copilot_native_roots_guard": make_guard}), \
                    patch.object(Path, "open", checked_open), \
                    patch.object(sys, "argv", ["runner", "--target", target, "--cli", "unused",
                                               "--report", str(report_path)]), \
                    patch.object(sys, "stdout", StringIO()), patch.object(sys, "stderr", StringIO()):
                code = main()
            self.assertEqual([phase for phase in mutation_checkpoints
                              if phase.startswith("after-public-mutation-")], mutation_labels)
            if target == "codex":
                self.assertEqual(len(codex_profiles),
                                 ("A", "B", "C").index(codex_constructor_error)
                                 if codex_constructor_error is not None else 3)
                self.assertTrue(all(profile.checkpoints for profile in codex_profiles))
            if target == "claude":
                self.assertTrue(retirements)
                self.assertTrue(all(retirement.closed for retirement in retirements))
                self.assertTrue(all(retirement.finished for retirement in retirements)
                                if project_remove_error not in ("remove A", "remove B") else not retirements[-1].finished)
            return code, json.loads(report_path.read_text()), phases

    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_owned_temporary_parent_alias_keeps_full_lifecycle(self):
        code, report, phases = self._exercise_marketing_kit_scope_report(
            "copilot", require_copilot_trust=True, temporary_root_alias=True)
        self.assertEqual(code, 0, report.get("failure"))
        self.assertTrue(report["two_scope_lifecycle_qualified"])
        self.assertEqual(report["scope_results"]["project"]["status"], "passed")
        self.assertEqual(report["scope_results"]["user"]["status"], "passed")
        self.assertEqual(report["copilot_project_b_trust_fixture"]["status"],
                         "authored-native-prerequisite")
        for phase in ("temporary-root-alias", "preauthorized-trust:B", "cold locked restore",
                      "remove B", "remove A", "User cold locked restore C", "User remove C",
                      "User remove", "catalog:E"):
            self.assertIn(phase, phases)


    def test_marketing_kit_copilot_b_trust_refuses_unsupported_host_before_uid_or_IO(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        with patch.object(os, "supports_dir_fd", set()), \
                patch.object(os, "getuid", side_effect=AssertionError("UID accessed"), create=True), \
                patch.object(os, "open", side_effect=AssertionError("filesystem accessed")):
            with self.assertRaisesRegex(AssertionError, "safe POSIX directory descriptors"):
                runner["prepare_copilot_project_b_trust"]({}, Path("unused-project"), Path("unused-root"))


    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_authors_only_exact_native_prerequisite(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            cases = {name: runner["run_case"]("unused", "copilot", root, name)
                     for name in ("A", "B", "C", "E")}
            peers = {name: runner["profile_snapshot"](value[0], value[1])
                     for name, value in cases.items() if name != "B"}
            env, project, *_ = cases["B"]
            for name in ("uap.toml", "uap.lock"):
                (project / name).write_text("unchanged portable agreement")
            proof, guard = runner["prepare_copilot_project_b_trust"](env, project, root)
            path = Path(env["HOME"]) / ".copilot/config.json"
            expected = {"trustedFolders": [str(project)]}
            self.assertEqual(json.loads(path.read_text()), expected)
            self.assertEqual(proof["trusted_folders"], [str(project)])
            self.assertEqual(proof["config_sha256_at_creation"], hashlib.sha256(path.read_bytes()).hexdigest())
            self.assertEqual(proof["config_identity_at_creation"]["inode"], path.lstat().st_ino)
            self.assertTrue(proof["uap_state_empty_before"])
            self.assertEqual(list(Path(env["AGENTPLUGINS_HOME"]).iterdir()), [])
            for name in ("uap.toml", "uap.lock"):
                self.assertEqual((project / name).read_text(), "unchanged portable agreement")
            for name, snapshot in peers.items():
                self.assertEqual(runner["profile_snapshot"](*cases[name][:2]), snapshot)
            # A safe native atomic rewrite of the same authority is permitted;
            # the receipt's original inode/digest are explicitly creation-time.
            replacement = path.with_suffix(".new")
            replacement.write_text(json.dumps(expected, indent=2))
            replacement.chmod(0o600)
            replacement.replace(path)
            guard()


    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_refuses_other_profiles_and_warm_state(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        for name in ("A", "C", "E", "B"):
            with self.subTest(name=name), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                env, project, *_ = runner["run_case"]("unused", "copilot", root, name)
                if name == "B":
                    (Path(env["AGENTPLUGINS_HOME"]) / "state-v2.json").write_text("existing receipt")
                before = runner["profile_snapshot"](env, project)
                with self.assertRaisesRegex(AssertionError, "canonical fresh Project B|empty UAP state"):
                    runner["prepare_copilot_project_b_trust"](env, project, root)
                self.assertEqual(runner["profile_snapshot"](env, project), before)


    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_refuses_preexisting_config(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        for kind in ("file", "empty", "directory", "symlink", "hardlink"):
            with self.subTest(kind=kind), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                env, project, *_ = runner["run_case"]("unused", "copilot", root, "B")
                path = Path(env["HOME"]) / ".copilot/config.json"
                foreign = root / "foreign.json"
                foreign.write_text('{"trustedFolders":["/foreign"],"allowedTools":["shell"]}')
                if kind == "directory":
                    path.mkdir()
                elif kind == "symlink":
                    path.symlink_to(foreign)
                elif kind == "hardlink":
                    os.link(foreign, path)
                else:
                    path.write_text("" if kind == "empty" else foreign.read_text())
                before = runner["profile_snapshot"](env, project)
                with self.assertRaisesRegex(AssertionError, "unsafe or occupied config"):
                    runner["prepare_copilot_project_b_trust"](env, project, root)
                self.assertEqual(runner["profile_snapshot"](env, project), before)
                self.assertEqual(json.loads(foreign.read_text())["trustedFolders"], ["/foreign"])


    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_refuses_preexisting_project_receipts(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        for kind in ("state", "dangling-symlink"):
            with self.subTest(kind=kind), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                env, project, *_ = runner["run_case"]("unused", "copilot", root, "B")
                state = project / ".uap"
                if kind == "state":
                    state.mkdir()
                    (state / "state-v2.json").write_text("preplanted receipt")
                else:
                    state.symlink_to(root / "missing", target_is_directory=True)
                before = runner["profile_snapshot"](env, project)
                with self.assertRaisesRegex(AssertionError, "empty UAP state"):
                    runner["prepare_copilot_project_b_trust"](env, project, root)
                self.assertEqual(runner["profile_snapshot"](env, project), before)


    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_rejects_changed_config_authority(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        changes = ("missing", "symlink", "hardlink", "mode", "ancestor-folder", "second-folder",
                   "tool-grant", "auth", "duplicate-key")
        for change in changes:
            with self.subTest(change=change), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                env, project, *_ = runner["run_case"]("unused", "copilot", root, "B")
                _, guard = runner["prepare_copilot_project_b_trust"](env, project, root)
                path = Path(env["HOME"]) / ".copilot/config.json"
                if change == "missing":
                    path.unlink()
                elif change == "symlink":
                    saved = path.with_suffix(".saved")
                    path.rename(saved)
                    path.symlink_to(saved)
                elif change == "hardlink":
                    os.link(path, path.with_suffix(".linked"))
                elif change == "mode":
                    path.chmod(0o644)
                elif change == "duplicate-key":
                    path.write_text('{"trustedFolders":[],"trustedFolders":' + json.dumps([str(project)]) + '}')
                else:
                    document = {"trustedFolders": [str(project)]}
                    if change == "ancestor-folder":
                        document["trustedFolders"] = [str(root)]
                    elif change == "second-folder":
                        document["trustedFolders"].append(str(project.parent))
                    else:
                        document["allowedTools" if change == "tool-grant" else "auth"] = ["unexpected"]
                    path.write_text(json.dumps(document))
                with self.assertRaisesRegex(AssertionError, "Copilot trust fixture authority"):
                    guard()


    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_refuses_redirected_or_replaced_roots(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        for when in ("before", "after"):
            for relative in ("B", "B/home", "B/home/.copilot", "B/project", "B/data"):
                with self.subTest(when=when, relative=relative), TemporaryDirectory() as temporary:
                    root = Path(temporary).resolve()
                    env, project, *_ = runner["run_case"]("unused", "copilot", root, "B")
                    if when == "after":
                        _, guard = runner["prepare_copilot_project_b_trust"](env, project, root)
                    path = root / relative
                    saved = path.with_name(path.name + "-saved")
                    path.rename(saved)
                    if when == "before":
                        path.symlink_to(saved, target_is_directory=True)
                    else:
                        import shutil
                        shutil.copytree(saved, path)
                    with self.assertRaisesRegex(AssertionError, "Copilot trust fixture root"):
                        if when == "before":
                            runner["prepare_copilot_project_b_trust"](env, project, root)
                        else:
                            guard()


    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_canonicalizes_allocated_root_before_trust_and_keeps_later_guards(self):
        for redirected in (None, "A", "C"):
            with self.subTest(redirected=redirected):
                code, report, phases = self._exercise_marketing_kit_scope_report(
                    "copilot", require_copilot_trust=True, temporary_root_alias=True,
                    native_root_redirect=redirected)
                self.assertIn("temporary-root-alias", phases)
                self.assertIn("cold locked restore", phases, report.get("failure"))
                if redirected is None:
                    self.assertEqual(code, 0, report.get("failure"))
                    self.assertTrue(report["two_scope_lifecycle_qualified"])
                    self.assertIn("remove B", phases)
                else:
                    self.assertEqual(code, 1)
                    self.assertIn("redirected", report["failure"])
                    self.assertFalse(report["two_scope_lifecycle_qualified"])
                    self.assertIn("guard-foreign-after-redirect", phases)
                    self.assertFalse(any(item.startswith("outside-read:") for item in phases))

    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_project_b_prepares_native_trust_before_locked_restore(self):
        code, report, phases = self._exercise_marketing_kit_scope_report(
            "copilot", require_copilot_trust=True)
        self.assertEqual(code, 0, report.get("failure"))
        proof = report["copilot_project_b_trust_fixture"]
        self.assertEqual(proof["status"], "authored-native-prerequisite")
        self.assertEqual(proof["source_profile"], "B")
        self.assertEqual(proof["restore_scope"], "cold-UAP-state-with-preauthorized-native-folder")
        self.assertFalse(proof["ui_consent_observed"])
        self.assertFalse(proof["product_trust_grant_claimed"])
        self.assertFalse(proof["virgin_native_trust_qualified"])
        self.assertLess(phases.index("preauthorized-trust:B"), phases.index("cold locked restore"))
        self.assertIn("remove B", phases)

    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_mutation_fails_after_restore_or_removal(self):
        for stage in ("cold locked restore", "remove B"):
            with self.subTest(stage=stage):
                code, report, phases = self._exercise_marketing_kit_scope_report(
                    "copilot", require_copilot_trust=True, copilot_trust_mutation=stage)
                self.assertEqual(code, 1)
                self.assertIn("Copilot trust fixture authority changed", report["failure"])
                self.assertFalse(report["two_scope_lifecycle_qualified"])
                self.assertIn(stage, phases)

    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_setup_failure_runs_foreign_and_A_guards(self):
        for failure in ("trust", "foreign"):
            with self.subTest(failure=failure):
                code, report, phases = self._exercise_marketing_kit_scope_report(
                    "copilot", copilot_fixture_failure=failure)
                self.assertEqual(code, 1)
                self.assertFalse(report["two_scope_lifecycle_qualified"])
                self.assertIn("guard-foreign-after-trust-failure", phases)
                self.assertIn("guard-peer-after-trust-failure:A", phases)
                self.assertNotIn("catalog:B", phases)

    def test_marketing_kit_claude_project_remove_failure_closes_captured_custody(self):
        code, report, phases = self._exercise_marketing_kit_scope_report(
            "claude", project_remove_error="remove B")
        self.assertEqual(code, 1)
        self.assertFalse(report["two_scope_lifecycle_qualified"])
        self.assertIn("injected Project removal failure", report["failure"])
        self.assertIn("claude-capture:B", phases)
        self.assertIn("claude-close:B", phases)
        self.assertNotIn("claude-finish:B", phases)

    def test_marketing_kit_claude_retains_only_A_through_user_and_closes_before_cleanup(self):
        code, report, phases = self._exercise_marketing_kit_scope_report("claude")
        self.assertEqual(code, 0, report.get("failure"))
        self.assertLess(phases.index("claude-close:B"), phases.index("claude-capture:A"))
        self.assertLess(phases.index("User remove"), phases.index("claude-retained-user-A"))
        self.assertLess(phases.index("claude-retained-user-A"), phases.index("claude-close:A"))
        self.assertLess(phases.index("claude-close:A"), phases.index("claude-closed-before-disposal"))

    def test_marketing_kit_claude_retained_failure_paths_close_before_cleanup(self):
        for changes in ({"project_remove_error": "User remove"}, {"claude_retained_scan_error": True},
                        {"final_policy_error": True}, {"cleanup_failure": True}):
            with self.subTest(changes=changes):
                code, report, phases = self._exercise_marketing_kit_scope_report("claude", **changes)
                self.assertEqual(code, 1)
                self.assertFalse(report["two_scope_lifecycle_qualified"])
                self.assertLess(phases.index("claude-close:A"), phases.index("claude-closed-before-disposal"))

    def test_marketing_kit_codex_preoperation_failure_runs_every_available_guard(self):
        code, report, phases = self._exercise_marketing_kit_scope_report(
            "codex", codex_checkpoint_error="User remove")
        self.assertEqual(code, 1)
        self.assertFalse(report["two_scope_lifecycle_qualified"])
        self.assertIn("injected Codex checkpoint failure", report["failure"])
        self.assertNotIn("User remove", phases, "pre-operation refusal must prevent public execute")
        after = phases[phases.index("codex-checkpoint:A:before-User remove"):]
        for expected in (
                "guard-foreign-after-codex-failure",
                "codex-checkpoint:B:before-User remove", "codex-checkpoint:C:before-User remove",
                "codex-checkpoint:A:after-User remove", "codex-checkpoint:B:after-User remove",
                "codex-checkpoint:C:after-User remove", "guard-project:A:User remove",
                "guard-peer:B:User remove", "guard-project:C:User remove", "guard-peer:C:User remove"):
            self.assertIn(expected, after)

    def test_marketing_kit_codex_constructor_failure_still_checks_foreign_state(self):
        code, report, phases = self._exercise_marketing_kit_scope_report(
            "codex", codex_constructor_error="A")
        self.assertEqual(code, 1)
        self.assertFalse(report["two_scope_lifecycle_qualified"])
        self.assertIn("injected Codex profile constructor failure", report["failure"])
        self.assertEqual(phases, ["codex-constructor-refused:A", "guard-foreign-after-codex-failure"])
        self.assertEqual(report["steps"], [], "constructor refusal must prevent public execute")

    def test_marketing_kit_all_six_require_complete_positive_user_and_project_phases(self):
        for target in ("codex", "cursor", "claude", "gemini", "opencode", "copilot"):
            with self.subTest(target=target):
                code, report, phases = self._exercise_marketing_kit_scope_report(target)
                self.assertEqual(code, 0, report.get("failure"))
                self.assertEqual(report["status"], "passed")
                self.assertEqual(report["qualification_scope"], "user_and_project")
                self.assertTrue(report["two_scope_lifecycle_qualified"])
                if target == "codex":
                    self.assertEqual(set(report["codex_sqlite_profiles"]), {"A", "B", "C"})
                self.assertEqual(report["positive_policy"]["source_mode"], "production_default")
                self.assertEqual(report["positive_policy"]["snapshot_sequence"], 46)
                self.assertEqual([phase for phase in phases if phase.startswith("catalog:")],
                                 ["catalog:A", "catalog:B", "catalog:C", "catalog:E"])
                self.assertEqual(report["final_policy_observation"], {
                    "status": "passed", "source_profile": "E", "profile_usage": "never_installed",
                    "policy": report["positive_policy"]})
                for scope in ("user", "project"):
                    self.assertEqual(report["scope_results"][scope], {
                        "expectation": "positive_lifecycle", "status": "passed"})
                required = ["add", "repeat add", "update", "repair intact", "cold locked restore",
                            "remove B", "remove A", "User add", "User repeat add", "User update",
                            "User repair intact", "User cold locked restore C", "User remove C", "User remove"]
                self.assertEqual([phase for phase in phases if phase in required], required)
                boundary = report["current_vscode_user_boundary"]
                if target == "copilot":
                    self.assertEqual(phases[-1], "vscode-saved_management_capability")
                    self.assertEqual(boundary["status"], "refused_as_expected")
                    self.assertEqual(boundary["contract"], "saved_management_capability")
                    self.assertEqual(boundary["policy"], report["positive_policy"])
                    self.assertEqual(report["logical_owner_evidence"]["logical_owner"], "copilot")
                    self.assertFalse(report["logical_owner_evidence"]["separate_vscode_binding"])
                else:
                    self.assertEqual(boundary["status"], "not_applicable")
                self.assertEqual(report["historical_policy_results"]["signed45"], {
                    "status": "separate_point_in_time_audit", "required_for_current_qualification": False})

    def test_marketing_kit_removed_record_info_is_not_a_catalog_policy(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        # Exact public projection distinction in released read_commands.go and
        # read_directory.go: a retained removed record remains selectable and
        # reports current_snapshot_sequence inside directory, without a digest.
        result = {"exit_code": 0, "timeout": False, "_json": {
            "command": "info", "result": "success", "data": {
                "installation_id": "removed-installation", "declared_name": "marketing-kit",
                "clients": [], "directory": {"product_id": "marketing-kit",
                    "recorded_distribution": "777genius/marketing-kit",
                    "recorded_release_sequence": 1, "recorded_snapshot_sequence": 46,
                    "current_snapshot_sequence": 46}}}}
        inspect = runner["inspect_signed_closure"]
        with patch.dict(inspect.__globals__, {"invoke": lambda *_: result}):
            with self.assertRaisesRegex(AssertionError, "signed Directory policy identity invalid"):
                inspect("unused", {}, Path("unused"), "codex")

    def test_marketing_kit_final_policy_failure_and_all_profile_guards_fail_closed(self):
        for target in ("codex", "copilot"):
            for peer in (None, "A", "B", "C"):
                with self.subTest(target=target, peer=peer):
                    code, report, phases = self._exercise_marketing_kit_scope_report(
                        target, final_policy_error=True, final_peer_mutation=peer)
                    self.assertEqual(code, 1)
                    self.assertIn("catalog:E", phases)
                    self.assertFalse(report["two_scope_lifecycle_qualified"])
                    self.assertEqual(report["final_policy_observation"]["status"], "not_run")
                    self.assertEqual(report["scope_results"]["user"]["status"], "not_run")
                    self.assertIn("original isolated profile changed" if peer else "policy observation changed",
                                  report["failure"])

    def test_marketing_kit_final_policy_mismatch_checks_every_failure_guard(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_final_production_observation"]
        for sequence in (44, 47):
            with self.subTest(sequence=sequence), TemporaryDirectory() as temporary:
                env, project, *_ = runner["run_case"]("unused", "codex", Path(temporary), "E")
                checked = []
                def broken_guard():
                    checked.append("A")
                    raise RuntimeError("A peer changed")
                with patch.dict(verify.__globals__, {
                        "verify_pinned_launcher": lambda *_: None,
                        "observe_production_policy": lambda *_: {"snapshot_sequence": sequence}}):
                    with self.assertRaisesRegex(AssertionError, "final production policy observation changed"):
                        verify("unused", env, project, "codex", {"snapshot_sequence": 46},
                            (lambda: checked.append("A"), lambda: checked.append("B"), lambda: checked.append("C")))
                    self.assertEqual(checked, ["A", "B", "C"])
                    checked.clear()
                    with self.assertRaisesRegex(RuntimeError, "A peer changed"):
                        verify("unused", env, project, "codex", {"snapshot_sequence": 46},
                            (broken_guard, lambda: checked.append("B"), lambda: checked.append("C")))
                self.assertEqual(checked, ["A", "B", "C"])

    def test_marketing_kit_final_copilot_warmup_checks_peers_before_raw_refusal(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_final_production_observation"]
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            peer_env, peer_project, *_ = runner["run_case"]("unused", "copilot", root, "A")
            env, project, *_ = runner["run_case"]("unused", "copilot", root, "E")
            before = runner["profile_snapshot"](peer_env, peer_project)
            injected = Path(peer_env["HOME"]) / "warmup-mutated-peer"
            response = self._marketing_kit_info_response(runner,
                runner["REQUIRED_DISTRIBUTIONS"]["marketing-kit"], prepared_vscode=True)
            policy = {"source_mode": "production_default", "snapshot_sequence": 46,
                "snapshot_digest": "sha256:" + "a" * 64,
                "root_target_compatibility": {item["client"]: {"scopes": item["scopes"], "delivery": item["delivery"]}
                    for item in response["_json"]["data"]["selected_release"]["target_compatibility"]},
                "closure": runner["FIXED_SIGNED_CLOSURE"]}
            called, checked = [], []
            def warm(*_):
                injected.write_text("must be detected before raw refusal")
                return policy
            def raw(*_):
                called.append("raw")
                injected.unlink()
                return {"exit_code": 1, "reason": "unsupported_saved_target"}
            def peer_guard():
                runner["check_profile_unchanged"](peer_env, peer_project, before, "warmup peer")
            with patch.dict(verify.__globals__, {"verify_pinned_launcher": lambda *_: None,
                    "observe_production_policy": warm, "verify_vscode_user_saved_refusal": raw}):
                with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                    verify("unused", env, project, "copilot", policy,
                           (peer_guard, lambda: checked.append("other final guard")))
            self.assertEqual(called, [])
            self.assertTrue(injected.exists())
            self.assertEqual(checked, ["other final guard"])

    def test_marketing_kit_profiles_never_inherit_arbitrary_directory_origin(self):
        with patch.dict(os.environ, {"AGENTPLUGINS_DIRECTORY_ORIGIN": "https://injected.invalid/"}):
            code, report, _ = self._exercise_marketing_kit_scope_report("copilot")
        self.assertEqual(code, 0, report.get("failure"))
        self.assertEqual(report["positive_policy"]["source_mode"], "production_default")
        self.assertEqual(report["current_vscode_user_boundary"]["policy"], report["positive_policy"])

    def test_marketing_kit_current_boundary_failure_cannot_qualify_positive_lifecycles(self):
        for grants in (6, 7):
            with self.subTest(grants=grants):
                code, report, phases = self._exercise_marketing_kit_scope_report(
                    "copilot", boundary_error=True, grants=grants)
                self.assertEqual(code, 1)
                self.assertEqual(report["scope_results"]["project"]["status"], "passed")
                self.assertEqual(report["scope_results"]["user"]["status"], "not_run")
                self.assertFalse(report["two_scope_lifecycle_qualified"])
                self.assertEqual(report["status"], "failed")
                self.assertEqual(report["failure"], "required current boundary failed")

    def test_marketing_kit_six_grant_current_refusal_qualifies_without_historical45(self):
        code, report, phases = self._exercise_marketing_kit_scope_report("copilot", grants=6)
        self.assertEqual(code, 0, report.get("failure"))
        self.assertTrue(report["two_scope_lifecycle_qualified"])
        self.assertEqual(phases[-1], "vscode-signed_grant_refusal")
        self.assertEqual(report["current_vscode_user_boundary"]["contract"], "signed_grant_refusal")
        self.assertEqual(report["current_vscode_user_boundary"]["policy"], report["positive_policy"])

    def test_marketing_kit_cleanup_exception_overrides_successful_report(self):
        for target in ("copilot", "cursor"):
            with self.subTest(target=target):
                code, report, phases = self._exercise_marketing_kit_scope_report(
                    target, cleanup_failure=True)
                self.assertEqual(code, 1)
                self.assertIn("User remove", phases)
                self.assertEqual(report["scope_results"]["project"]["status"], "passed")
                self.assertEqual(report["scope_results"]["user"]["status"], "passed")
                self.assertFalse(report["two_scope_lifecycle_qualified"])
                self.assertEqual(report["status"], "failed")
                self.assertEqual(report["failure"], "disposable cleanup failed")

    def test_marketing_kit_corrupt_lock_never_opens_preexisting_symlink(self):
        # The fixture must refuse a planted outside symlink *before* either
        # reading original bytes or opening the lock for corruption.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_corrupt_lock_refusal"]
        corrupt = runner["corrupt_locked_child_digest"]
        for scope in ("project", "user"):
            with self.subTest(scope=scope), TemporaryDirectory() as temporary:
                root = Path(temporary)
                home, data, project = (root / name for name in ("home", "data", "project"))
                for path in (home, data, project):
                    path.mkdir()
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                locked = (project if scope == "project" else data) / "uap.lock"
                foreign = root / "foreign-lock"
                foreign.write_bytes(b"foreign must not be read or changed")
                try:
                    locked.symlink_to(foreign)
                except (OSError, NotImplementedError) as error:
                    self.skipTest(f"host cannot create file symlinks: {error}")

                original_read, original_open = Path.read_bytes, os.open

                def refuse_read(path):
                    if path == locked:
                        raise AssertionError("read a foreign signed-lock symlink")
                    return original_read(path)

                def refuse_open(path, *args, **kwargs):
                    if Path(path) == locked:
                        raise AssertionError("opened a foreign signed-lock symlink")
                    return original_open(path, *args, **kwargs)

                with (patch.object(Path, "read_bytes", refuse_read),
                      patch.object(os, "open", refuse_open)):
                    with self.assertRaisesRegex(AssertionError, "invalid original signed lock path"):
                        verify("agentplugins", env, project, "codex", scope, ())
                    with self.assertRaisesRegex(AssertionError, "invalid negative-test lock path"):
                        corrupt(locked)
                self.assertEqual(foreign.read_bytes(), b"foreign must not be read or changed")

    def test_marketing_kit_corrupt_lock_restore_rejects_foreign_symlink(self):
        # A malicious native invocation may swap the fixture lock for a link to
        # an outside file. Cleanup must not follow it to write original bytes.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_corrupt_lock_refusal"]
        original = {"packages": [
            {"alias": "notion", "tree_digest": "sha256:" + "b" * 64}]}
        for scope in ("project", "user"):
            with self.subTest(scope=scope), TemporaryDirectory() as temporary:
                with TemporaryDirectory() as external:
                    root = Path(temporary)
                    home, data, project = (root / name for name in ("home", "data", "project"))
                    for path in (home, data, project):
                        path.mkdir()
                    env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                    locked = (project if scope == "project" else data) / "uap.lock"
                    locked.write_text(json.dumps(original) + "\n", encoding="utf-8")
                    foreign = Path(external) / "foreign.json"
                    foreign.write_bytes(b"foreign must remain intact")
                    # Skip before invoking the guard: its finally block must
                    # never mask a requested test skip on symlink-hostile OSes.
                    probe = root / "symlink-probe"
                    try:
                        probe.symlink_to(foreign)
                    except (OSError, NotImplementedError) as error:
                        self.skipTest(f"host cannot create file symlinks: {error}")
                    else:
                        probe.unlink()
                    guards = []

                    def attacker(*_):
                        locked.unlink()
                        locked.symlink_to(foreign)
                        return {"exit_code": 1, "timeout": False,
                                "stderr_tail": ("verify notion pinned manifest; run agentplugins deps lock: "
                                            "signed Directory publication differs from exact lock")}

                    # Opening the link is already unsafe even if inode checks
                    # subsequently prevent a write. Catch that earlier hazard.
                    real_open = Path.open

                    def reject_swapped_open(path, *args, **kwargs):
                        if path == locked and path.is_symlink():
                            raise AssertionError("opened a swapped signed-lock symlink")
                        return real_open(path, *args, **kwargs)

                    with (patch.object(Path, "open", reject_swapped_open),
                          patch.dict(verify.__globals__, {"invoke": attacker})):
                        with self.assertRaisesRegex(AssertionError, "lock path replaced"):
                            verify("agentplugins", env, project, "codex", scope,
                                   (lambda: guards.append(True),))
                    self.assertEqual(foreign.read_bytes(), b"foreign must remain intact")
                    self.assertEqual(guards, [True])

    def test_marketing_kit_corrupt_lock_restore_rejects_same_inode_symlink(self):
        # A renamed original inode has one link. A symlink to that inode
        # passes fstat and single-link checks but leaves a hostile lock path.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_corrupt_lock_refusal"]
        original = {"packages": [
            {"alias": "notion", "tree_digest": "sha256:" + "b" * 64}]}
        for scope in ("project", "user"):
            with self.subTest(scope=scope), TemporaryDirectory() as temporary:
                root = Path(temporary)
                home, data, project = (root / name
                                       for name in ("home", "data", "project"))
                for path in (home, data, project):
                    path.mkdir()
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                locked = (project if scope == "project" else data) / "uap.lock"
                locked.write_text(json.dumps(original) + "\n", encoding="utf-8")
                original_bytes = locked.read_bytes()
                retained = locked.with_name("retained-original-lock")
                probe = root / "symlink-probe"
                try:
                    probe.symlink_to(locked)
                except (OSError, NotImplementedError) as error:
                    self.skipTest(f"host cannot create file symlinks: {error}")
                else:
                    probe.unlink()
                checked = []

                def attacker(*_):
                    locked.rename(retained)
                    locked.symlink_to(retained)
                    return {"exit_code": 1, "timeout": False,
                            "stderr_tail": (
                                "verify notion pinned manifest: "
                                "signed Directory publication differs from exact lock")}

                with patch.dict(verify.__globals__, {"invoke": attacker}):
                    with self.assertRaisesRegex(AssertionError, "lock path replaced"):
                        verify("agentplugins", env, project, "codex", scope,
                               (lambda: checked.append(True),))
                self.assertEqual(checked, [True])
                self.assertTrue(locked.is_symlink())
                self.assertNotEqual(retained.read_bytes(), original_bytes)

    def test_marketing_kit_corrupt_lock_restore_rejects_foreign_hardlink(self):
        # An outside hardlink to the original inode makes inode-only cleanup
        # overwrite a foreign path when the corrupt fixture is restored.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_corrupt_lock_refusal"]
        original = {"packages": [
            {"alias": "notion", "tree_digest": "sha256:" + "b" * 64}]}
        for scope in ("project", "user"):
            with self.subTest(scope=scope), TemporaryDirectory() as temporary:
                with TemporaryDirectory() as external:
                    root = Path(temporary)
                    home, data, project = (root / name
                                           for name in ("home", "data", "project"))
                    for path in (home, data, project):
                        path.mkdir()
                    env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                    locked = (project if scope == "project" else data) / "uap.lock"
                    locked.write_text(json.dumps(original) + "\n", encoding="utf-8")
                    original_bytes = locked.read_bytes()
                    foreign = Path(external) / "linked-lock"
                    try:
                        os.link(locked, foreign)
                    except (OSError, NotImplementedError) as error:
                        self.skipTest(f"host cannot create hardlinks: {error}")
                    # A preexisting hardlink must be rejected before corruption.
                    with self.assertRaisesRegex(
                            AssertionError, "invalid original signed lock path"):
                        verify("agentplugins", env, project, "codex", scope, ())
                    self.assertEqual(foreign.read_bytes(), original_bytes)
                    foreign.unlink()
                    checked = []

                    def attacker(*_):
                        os.link(locked, foreign)
                        return {"exit_code": 1, "timeout": False,
                                "stderr_tail": (
                                    "verify notion pinned manifest: "
                                    "signed Directory publication differs from exact lock")}

                    with patch.dict(verify.__globals__, {
                            "invoke": attacker, "inventory": lambda *args: [],
                            "identities": lambda *args: {}}):
                        with self.assertRaisesRegex(
                                AssertionError, "lock path replaced"):
                            verify("agentplugins", env, project, "codex", scope,
                                   (lambda: checked.append(True),))
                    self.assertEqual(checked, [True])
                    self.assertNotEqual(foreign.read_bytes(), original_bytes)
                    self.assertEqual(foreign.read_bytes(), locked.read_bytes())

    def test_marketing_kit_cold_locked_restore_receipts_match_installed_scope(self):
        # A real cold restore must bind each reported applied result to the
        # installed distribution, not merely return four plausible IDs.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        required = runner["REQUIRED_DISTRIBUTIONS"]
        aliases = tuple(required)
        actions = [{"alias": alias, "client_id": "codex", "action": "install"}
                   for alias in aliases]
        results = [{"client_id": "codex", "operation": "install",
                    "installation_id": alias + "-id", "outcome": "completed"}
                   for alias in aliases]
        installed = {required[alias]: alias + "-id" for alias in aliases}
        for scope in ("project", "user"):
            with self.subTest(scope=scope):
                envelope = {"result": "success", "outcome": "completed",
                            "scope": scope, "actions": actions, "results": results}
                receipt = {"exit_code": 0, "timeout": False, "_json": envelope}
                args = ("install", "--locked", "--yes", "--scope", scope)
                if scope == "project":
                    args += ("--project-root", str(Path.cwd()))
                with patch.dict(execute.__globals__, {
                        "invoke": lambda *argv: receipt,
                        "inventory": lambda *argv: None,
                        "identities": lambda *argv: installed}):
                    self.assertEqual(
                        execute("agentplugins", {}, Path.cwd(), "codex",
                                "cold restore", *args)["result"], "passed")
                    for incorrect in (
                            # A swapped child identity was formerly accepted
                            # because the inventory itself remained correct.
                            [results[1], results[0], *results[2:]],
                            [{**results[0], "installation_id": "other-profile-id"},
                             *results[1:]]):
                        malformed = {**receipt, "_json": {
                            **envelope, "results": incorrect}}
                        with patch.dict(execute.__globals__, {
                                "invoke": lambda *argv: malformed}):
                            with self.assertRaisesRegex(
                                    AssertionError, "receipt does not match"):
                                execute("agentplugins", {}, Path.cwd(), "codex",
                                        "cold restore", *args)

    def test_marketing_kit_repair_receipt_matches_selected_scope_inventory(self):
        # A plausible repair receipt cannot attest to a different durable
        # installation merely because the action itself reports the same ID.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        distribution = runner["REQUIRED_DISTRIBUTIONS"]["marketing-kit"]
        action = {"alias": "marketing-kit", "client_id": "codex",
                  "action": "repair", "installation_id": "root-id"}
        receipt = {"client_id": "codex", "operation": "repair",
                   "installation_id": "root-id", "outcome": "completed"}
        for scope in ("project", "user"):
            with self.subTest(scope=scope):
                envelope = {"result": "success", "outcome": "completed",
                            "scope": scope, "actions": [action],
                            "results": [receipt]}
                response = {"exit_code": 0, "timeout": False, "_json": envelope}
                args = ("repair", "marketing-kit", "--target", "codex",
                        "--scope", scope)
                if scope == "project":
                    args += ("--project-root", str(Path.cwd()))
                with patch.dict(execute.__globals__, {
                        "invoke": lambda *argv: response,
                        "inventory": lambda *argv: [],
                        "identities": lambda *argv: {distribution: "root-id"}}):
                    self.assertEqual(
                        execute("agentplugins", {}, Path.cwd(), "codex",
                                "repair", *args)["result"], "passed")
                    with patch.dict(execute.__globals__, {
                            "identities": lambda *argv: {distribution: "foreign-id"}}):
                        with self.assertRaisesRegex(
                                AssertionError, "receipt does not match installed"):
                            execute("agentplugins", {}, Path.cwd(),
                                    "codex", "repair", *args)

    def test_marketing_kit_accepts_only_complete_legacy_noop_update(self):
        # Published 0.1.102 emits result=success alongside outcome/actions/results.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["verified"]
        aliases = ("marketing-kit", "notion", "hubspot-crm", "playwright")
        actions = [{"alias": alias, "client_id": "codex",
                    "installation_id": alias + "-installation",
                    "binding_id": alias + "-binding", "action": "unchanged"}
                   for alias in aliases]
        envelope = {"result": "success", "outcome": "completed",
                    "actions": actions, "results": []}
        valid = {"exit_code": 0, "timeout": False, "_json": envelope}
        with_success = {**envelope, "result": "success"}
        self.assertEqual(check({**valid, "_json": with_success},
                               "update", "update", "codex"), with_success)
        for stage in ("update", "User update"):
            self.assertEqual(check(valid, stage, "update"), envelope)
        for operation in ("add", "repair", "install", "remove"):
            with self.subTest(operation=operation), self.assertRaisesRegex(
                    AssertionError, "CLI did not succeed"):
                check(valid, "update", operation)
        missing_status = {key: value for key, value in envelope.items()
                          if key != "result"}
        with self.assertRaisesRegex(AssertionError, "CLI did not succeed"):
            check({**valid, "_json": missing_status}, "update", "update")
        for changed in (
                {"outcome": "partial"},
                {"outcome": "failure"},
                {"result": "failure"},
                {"result": "partial"},
                {"results": [{"outcome": "failed"}]},
                {"actions": actions[:-1]},
                {"actions": [{**actions[0], "action": "failed"}, *actions[1:]]},
                {"actions": [{**actions[0], "alias": "foreign"}, *actions[1:]]},
                {"actions": [{**actions[0], "client_id": ""}, *actions[1:]]},
        ):
            with self.subTest(changed=changed), self.assertRaisesRegex(
                    AssertionError, "CLI did not succeed"):
                check({**valid, "_json": {**envelope, **changed}}, "update", "update")
        for changed in ({"exit_code": 1}, {"timeout": True}):
            with self.subTest(changed=changed), self.assertRaisesRegex(
                    AssertionError, "CLI did not succeed"):
                check({**valid, **changed}, "update", "update")

    def test_marketing_kit_dependency_reports_require_selected_client_and_receipts(self):
        # A zero exit and generic success must not certify another client's actions.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["verified"]
        aliases = ("marketing-kit", "notion", "hubspot-crm", "playwright")
        for operation, action_type, selected in (
                ("update", "unchanged", aliases),
                ("repair", "repair", ("marketing-kit",)),
                ("install", "install", aliases),
                ("remove", "remove", ("marketing-kit",))):
            with self.subTest(operation=operation):
                actions = [{"alias": alias, "client_id": "codex",
                            "installation_id": alias + "-id", "action": action_type}
                           for alias in selected]
                results = [{"operation": action_type, "client_id": "codex",
                            "installation_id": action["installation_id"],
                            "outcome": "completed"}
                           for action in actions if action_type != "unchanged"]
                envelope = {"result": "success", "outcome": "completed",
                            "actions": actions, "results": results}
                valid = {"exit_code": 0, "timeout": False, "_json": envelope}
                self.assertEqual(check(valid, operation, operation, "codex"), envelope)
                released = {**envelope, "result": "success"}
                self.assertEqual(check({**valid, "_json": released},
                                       operation, operation, "codex"), released)
                # Both actions and results can consistently attest a *different*
                # client while this target's native state remains untouched.
                foreign = {**envelope,
                           "actions": [{**action, "client_id": "claude"} for action in actions],
                           "results": [{**result, "client_id": "claude"} for result in results]}
                for invalid in (foreign, {**foreign, "result": "success"},
                                {"result": "success"},
                                {key: value for key, value in envelope.items()
                                 if key != "result"},
                                {**envelope, "result": "partial"},
                                {**envelope, "result": "failure"}):
                    with self.assertRaisesRegex(AssertionError, "CLI did not succeed"):
                        check({**valid, "_json": invalid}, operation, operation, "codex")
                if results:
                    # A completed native operation without a durable installation
                    # identity cannot be attributed to this signed dependency.
                    # Cold install plans may omit the *action* ID; an applied
                    # result must still return its actual installation ID.
                    for missing_id in (None, "", "   ", 42):
                        malformed_results = [dict(result) for result in results]
                        malformed_results[0]["installation_id"] = missing_id
                        with self.subTest(operation=operation, missing_id=missing_id):
                            with self.assertRaisesRegex(
                                    AssertionError, "CLI did not succeed"):
                                check({**valid, "_json": {**envelope,
                                      "results": malformed_results}},
                                      operation, operation, "codex")
                    # The released installer legitimately returns "unchanged"
                    # when an active repair/install/remove converges with no
                    # filesystem mutation. Preserve this exact public contract,
                    # but never accept an unrelated client's unchanged receipt.
                    no_op_receipts = {**envelope, "results": [
                        {**result, "outcome": "unchanged"} for result in results]}
                    for output in (no_op_receipts,
                                   {**no_op_receipts, "result": "success"}):
                        self.assertEqual(check({**valid, "_json": output},
                                               operation, operation, "codex"), output)
                        foreign_no_op = {**output, "results": [
                            {**result, "client_id": "claude"}
                            for result in output["results"]]}
                        with self.assertRaisesRegex(AssertionError,
                                                    "CLI did not succeed"):
                            check({**valid, "_json": foreign_no_op},
                                  operation, operation, "codex")

    def test_marketing_kit_copilot_b_trust_refuses_unsupported_host_before_uid_or_IO(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        with patch.object(os, "supports_dir_fd", set()), \
                patch.object(os, "getuid", side_effect=AssertionError("UID accessed"), create=True), \
                patch.object(os, "open", side_effect=AssertionError("filesystem accessed")):
            with self.assertRaisesRegex(AssertionError, "safe POSIX directory descriptors"):
                runner["prepare_copilot_project_b_trust"]({}, Path("unused-project"), Path("unused-root"))

    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_authors_only_exact_native_prerequisite(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            cases = {name: runner["run_case"]("unused", "copilot", root, name)
                     for name in ("A", "B", "C", "E")}
            peers = {name: runner["profile_snapshot"](value[0], value[1])
                     for name, value in cases.items() if name != "B"}
            env, project, *_ = cases["B"]
            for name in ("uap.toml", "uap.lock"):
                (project / name).write_text("unchanged portable agreement")
            proof, guard = runner["prepare_copilot_project_b_trust"](env, project, root)
            path = Path(env["HOME"]) / ".copilot/config.json"
            expected = {"trustedFolders": [str(project)]}
            self.assertEqual(json.loads(path.read_text()), expected)
            self.assertEqual(proof["trusted_folders"], [str(project)])
            self.assertEqual(proof["config_sha256_at_creation"], hashlib.sha256(path.read_bytes()).hexdigest())
            self.assertEqual(proof["config_identity_at_creation"]["inode"], path.lstat().st_ino)
            self.assertTrue(proof["uap_state_empty_before"])
            self.assertEqual(list(Path(env["AGENTPLUGINS_HOME"]).iterdir()), [])
            for name in ("uap.toml", "uap.lock"):
                self.assertEqual((project / name).read_text(), "unchanged portable agreement")
            for name, snapshot in peers.items():
                self.assertEqual(runner["profile_snapshot"](*cases[name][:2]), snapshot)
            # A safe native atomic rewrite of the same authority is permitted;
            # the receipt's original inode/digest are explicitly creation-time.
            replacement = path.with_suffix(".new")
            replacement.write_text(json.dumps(expected, indent=2))
            replacement.chmod(0o600)
            replacement.replace(path)
            guard()

    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_refuses_other_profiles_and_warm_state(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        for name in ("A", "C", "E", "B"):
            with self.subTest(name=name), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                env, project, *_ = runner["run_case"]("unused", "copilot", root, name)
                if name == "B":
                    (Path(env["AGENTPLUGINS_HOME"]) / "state-v2.json").write_text("existing receipt")
                before = runner["profile_snapshot"](env, project)
                with self.assertRaisesRegex(AssertionError, "canonical fresh Project B|empty UAP state"):
                    runner["prepare_copilot_project_b_trust"](env, project, root)
                self.assertEqual(runner["profile_snapshot"](env, project), before)

    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_refuses_preexisting_config(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        for kind in ("file", "empty", "directory", "symlink", "hardlink"):
            with self.subTest(kind=kind), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                env, project, *_ = runner["run_case"]("unused", "copilot", root, "B")
                path = Path(env["HOME"]) / ".copilot/config.json"
                foreign = root / "foreign.json"
                foreign.write_text('{"trustedFolders":["/foreign"],"allowedTools":["shell"]}')
                if kind == "directory":
                    path.mkdir()
                elif kind == "symlink":
                    path.symlink_to(foreign)
                elif kind == "hardlink":
                    os.link(foreign, path)
                else:
                    path.write_text("" if kind == "empty" else foreign.read_text())
                before = runner["profile_snapshot"](env, project)
                with self.assertRaisesRegex(AssertionError, "unsafe or occupied config"):
                    runner["prepare_copilot_project_b_trust"](env, project, root)
                self.assertEqual(runner["profile_snapshot"](env, project), before)
                self.assertEqual(json.loads(foreign.read_text())["trustedFolders"], ["/foreign"])

    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_refuses_preexisting_project_receipts(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        for kind in ("state", "dangling-symlink"):
            with self.subTest(kind=kind), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                env, project, *_ = runner["run_case"]("unused", "copilot", root, "B")
                state = project / ".uap"
                if kind == "state":
                    state.mkdir()
                    (state / "state-v2.json").write_text("preplanted receipt")
                else:
                    state.symlink_to(root / "missing", target_is_directory=True)
                before = runner["profile_snapshot"](env, project)
                with self.assertRaisesRegex(AssertionError, "empty UAP state"):
                    runner["prepare_copilot_project_b_trust"](env, project, root)
                self.assertEqual(runner["profile_snapshot"](env, project), before)

    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_rejects_changed_config_authority(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        changes = ("missing", "symlink", "hardlink", "mode", "ancestor-folder", "second-folder",
                   "tool-grant", "auth", "duplicate-key")
        for change in changes:
            with self.subTest(change=change), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                env, project, *_ = runner["run_case"]("unused", "copilot", root, "B")
                _, guard = runner["prepare_copilot_project_b_trust"](env, project, root)
                path = Path(env["HOME"]) / ".copilot/config.json"
                if change == "missing":
                    path.unlink()
                elif change == "symlink":
                    saved = path.with_suffix(".saved")
                    path.rename(saved)
                    path.symlink_to(saved)
                elif change == "hardlink":
                    os.link(path, path.with_suffix(".linked"))
                elif change == "mode":
                    path.chmod(0o644)
                elif change == "duplicate-key":
                    path.write_text('{"trustedFolders":[],"trustedFolders":' + json.dumps([str(project)]) + '}')
                else:
                    document = {"trustedFolders": [str(project)]}
                    if change == "ancestor-folder":
                        document["trustedFolders"] = [str(root)]
                    elif change == "second-folder":
                        document["trustedFolders"].append(str(project.parent))
                    else:
                        document["allowedTools" if change == "tool-grant" else "auth"] = ["unexpected"]
                    path.write_text(json.dumps(document))
                with self.assertRaisesRegex(AssertionError, "Copilot trust fixture authority"):
                    guard()

    @unittest.skipUnless(COPILOT_TRUST_FIXTURE_SUPPORTED, "Marketing Copilot B fixture requires POSIX descriptors")
    def test_marketing_kit_copilot_b_trust_refuses_redirected_or_replaced_roots(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        for when in ("before", "after"):
            for relative in ("B", "B/home", "B/home/.copilot", "B/project", "B/data"):
                with self.subTest(when=when, relative=relative), TemporaryDirectory() as temporary:
                    root = Path(temporary).resolve()
                    env, project, *_ = runner["run_case"]("unused", "copilot", root, "B")
                    if when == "after":
                        _, guard = runner["prepare_copilot_project_b_trust"](env, project, root)
                    path = root / relative
                    saved = path.with_name(path.name + "-saved")
                    path.rename(saved)
                    if when == "before":
                        path.symlink_to(saved, target_is_directory=True)
                    else:
                        import shutil
                        shutil.copytree(saved, path)
                    with self.assertRaisesRegex(AssertionError, "Copilot trust fixture root"):
                        if when == "before":
                            runner["prepare_copilot_project_b_trust"](env, project, root)
                        else:
                            guard()

    def test_marketing_kit_future_profiles_refuse_preplanted_paths(self):
        # A third-party package can access sibling paths under the temporary
        # case root. It must not redirect a later cold B/C bootstrap.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary, TemporaryDirectory() as external:
            root = Path(temporary).resolve()
            outside = Path(external).resolve()
            occupied = root / "B"
            occupied.mkdir()
            with self.assertRaisesRegex(AssertionError, "isolated profile root"):
                runner["run_case"]("agentplugins", "codex", root, "B")
            self.assertEqual(list(occupied.iterdir()), [])
            occupied.rmdir()
            try:
                occupied.symlink_to(outside, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            with self.assertRaisesRegex(AssertionError, "isolated profile root"):
                runner["run_case"]("agentplugins", "codex", root, "B")
            self.assertEqual(list(outside.iterdir()), [])

    def test_marketing_kit_project_git_init_uses_pinned_audit_prerequisite(self):
        # The second profile is created after running untrusted packages; it
        # must not start PATH-resolved Git or use ambient Git hooks/config.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        audit = runner["audit"]
        pinned_git = audit["initialize_project_skill_git_root"]
        calls = []

        def checked_git(project, env, root):
            calls.append((project, root))
            return pinned_git(project, env, root)

        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "A"
            with patch.dict(audit, {"initialize_project_skill_git_root": checked_git}):
                # Any fallback to the original subprocess.run(["git", ...])
                # would now fail rather than silently choosing PATH's binary.
                with patch.dict(runner["run_case"].__globals__,
                                {"subprocess": None}):
                    env, project, _, _, _ = runner["run_case"](
                        "agentplugins", "codex", root, "A")
            self.assertEqual(calls, [(project, case)])
            self.assertEqual(env["HOME"], str(case / "home"))
            self.assertTrue((project / ".git").is_dir())
            self.assertTrue((case / "empty-git-hooks").is_dir())

    def test_marketing_kit_dependency_execution_checks_reported_scope(self):
        # Native/portable state can already exist, so a successful command
        # targeting the wrong scope must be rejected from its own JSON receipt.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        stable = {distribution: alias + "-stable-id"
                  for alias, distribution in runner["REQUIRED_DISTRIBUTIONS"].items()}
        actions = [{"alias": alias, "client_id": "codex", "action": "unchanged",
                    "installation_id": alias + "-stable-id"}
                   for alias in ("marketing-kit", "notion", "hubspot-crm", "playwright")]
        original = {"result": "success", "outcome": "completed",
                    "actions": actions, "results": []}
        for scope in ("user", "project"):
            with self.subTest(scope=scope):
                for reported in (scope, "project" if scope == "user" else "user",
                                 None, "unexpected"):
                    receipt = {**original, "scope": reported}
                    response = {"exit_code": 0, "timeout": False, "_json": receipt}
                    with patch.dict(execute.__globals__,
                                    {"invoke": lambda *a: response,
                                     "inventory": lambda *a: [],
                                     "identities": lambda *a: stable}):
                        command = ("update", "marketing-kit", "--scope", scope)
                        if reported == scope:
                            self.assertEqual(
                                execute("agentplugins", {}, Path.cwd(), "codex",
                                        "update", *command)["result"], "passed")
                        else:
                            with self.assertRaisesRegex(
                                    AssertionError, "CLI did not succeed"):
                                execute("agentplugins", {}, Path.cwd(), "codex",
                                        "update", *command)
                for malformed in (("update", "marketing-kit"),
                                  ("update", "marketing-kit", "--scope", "invalid"),
                                  ("update", "marketing-kit", "--scope", "user",
                                   "--scope", "project")):
                    with self.assertRaisesRegex(
                            AssertionError, "explicit scope"):
                        execute("agentplugins", {}, Path.cwd(), "codex",
                                "update", *malformed)

    def test_marketing_kit_noop_update_rejects_rekey_and_lock_rewrite(self):
        # Released 0.1.102 reports unchanged; do not trust that envelope alone.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_noop_update"]
        original = {"marketing-kit": "root-id", "notion": "notion-id",
                    "hubspot-crm": "hubspot-id", "playwright": "playwright-id"}
        signed_lock = b'{"immutable": "signed"}\n'
        for stage in ("Project update", "User update"):
            with self.subTest(stage=stage):
                check(dict(original), original, signed_lock, signed_lock, stage)
                rekeyed = {**original, "notion": "unexpected-new-id"}
                with self.assertRaisesRegex(AssertionError,
                                            "no-op update changed installation identities"):
                    check(rekeyed, original, signed_lock, signed_lock, stage)
                with self.assertRaisesRegex(AssertionError,
                                            "no-op update changed signed lock"):
                    check(original, original, signed_lock + b" ", signed_lock, stage)

    def test_marketing_kit_legacy_repair_install_remove_require_real_results(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["verified"]
        actions = [{"alias": alias, "client_id": "codex",
                    "installation_id": alias + "-installation",
                    "action": "install"}
                   for alias in ("marketing-kit", "notion", "hubspot-crm", "playwright")]
        def report(action):
            return {"installation_id": action["installation_id"],
                    "client_id": action["client_id"],
                    "operation": action["action"], "outcome": "completed"}
        def accepted(operation, planned, results):
            envelope = {"result": "success", "outcome": "completed",
                        "actions": planned, "results": results}
            wrapped = {"exit_code": 0, "timeout": False, "_json": envelope}
            self.assertEqual(check(wrapped, operation, operation), envelope)
            return wrapped
        installed = accepted("install", actions, [report(action) for action in actions])
        repair = {**actions[0], "action": "repair"}
        intact = {**report(repair), "outcome": "unchanged",
                  "manual_actions": ["native MCP initialization remains unobserved"]}
        accepted("repair", [repair], [intact])
        removal = {**actions[0], "action": "remove"}
        accepted("remove", [removal], [report(removal)])
        for changed in (
                {"result": None},
                {"outcome": "partial"},
                {"results": []},
                {"results": [{**report(actions[0]), "outcome": "incomplete"},
                             *[report(action) for action in actions[1:]]]},
                {"results": [{**report(actions[0]), "client_id": "foreign"},
                             *[report(action) for action in actions[1:]]]},
                {"actions": [{**actions[0], "action": "unknown"}, *actions[1:]]},
        ):
            with self.subTest(changed=changed), self.assertRaisesRegex(
                    AssertionError, "CLI did not succeed"):
                check({**installed, "_json": {**installed["_json"], **changed}},
                      "cold locked restore", "install")

    def test_marketing_kit_published_abort_watchdog_refuses_stranded_proof_phases(self):
        # Run the actual script with an inert private bootstrap. A stranded
        # downloader or lock waiter must fail instead of letting Node exit
        # zero before its final assertions and trusted cache cleanup.
        import shutil
        import subprocess
        node = shutil.which("node")
        self.assertIsNotNone(
            node, "Node.js is required for the launcher watchdog regression")
        script = (Path(__file__).resolve().parent /
                  "marketing-kit-published-launcher-abort.js")
        bootstrap = r'''"use strict";
const fs = require("node:fs");
const path = require("node:path");
const phase = __STRANDED_PHASE__;
const trace = path.join(__dirname, "fixture-events.txt");
function record(event) {
  fs.appendFileSync(trace, event + "\n");
}
module.exports = {
  ensureBinary(options) {
    return new Promise((_resolve, reject) => {
      const request = options.request();
      request.on("error", error => {
        if (phase === "lock") reject(error);
      });
      options.signal.addEventListener("abort", () => {
        record("download-aborted");
        request.destroy(new Error("cancelled"));
      }, { once: true });
    });
  },
  acquireLock(_target, options) {
    if (phase !== "lock") {
      throw new Error("lock proof must not run after a stranded acquisition");
    }
    if (options.signal) {
      // The actual script supplies its 50 ms cancellation timer. After
      // that timer fires, this waiter has no referenced handle of its own.
      options.signal.addEventListener("abort", () => {
        record("waiter-aborted");
      }, { once: true });
      return new Promise(() => {});
    }
    fs.mkdirSync(options.lockRoot, { recursive: true });
    const ownerPath = path.join(options.lockRoot, "fixture-owner.lock");
    fs.writeFileSync(ownerPath, "owned fixture nonce\n", { flag: "wx" });
    record("owner-acquired");
    return Promise.resolve(async () => {
      record("owner-released");
      fs.unlinkSync(ownerPath);
    });
  }
};
'''
        for phase, label, expected_events in (
                ("download", "published downloader", ["download-aborted"]),
                ("lock", "published lock proof",
                 ["download-aborted", "download-aborted",
                  "owner-acquired", "waiter-aborted"])):
            with self.subTest(phase=phase), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(strict=True)
                npm_root = root / "npm"
                package = npm_root / "universal-agent-plugins"
                (package / "lib").mkdir(parents=True)
                (package / "package.json").write_text(
                    json.dumps({"version": "0.1.115"}), encoding="utf-8")
                (package / "lib/bootstrap.js").write_text(
                    bootstrap.replace("__STRANDED_PHASE__", json.dumps(phase)),
                    encoding="utf-8")
                scratch = root / "private-tmp"
                scratch.mkdir()
                env = {**os.environ, "UAP_GLOBAL_NPM_ROOT": str(npm_root),
                       "TMPDIR": str(scratch), "TMP": str(scratch), "TEMP": str(scratch)}
                result = subprocess.run([node, str(script)], env=env, cwd=root,
                                        capture_output=True, text=True, timeout=12, check=False)
                self.assertEqual(result.returncode, 1, result.stderr)
                self.assertIn(f"{label} watchdog deadline exceeded", result.stderr)
                self.assertNotIn("abort/lock cleanup: PASS", result.stdout + result.stderr)
                self.assertEqual(
                    (package / "lib/fixture-events.txt").read_text(encoding="utf-8").splitlines(),
                    expected_events)
                self.assertEqual(list(scratch.iterdir()), [],
                                 "failed watchdog must run trusted cache cleanup")

    def test_marketing_kit_requires_exact_published_cli_version(self):
        # The substring "0.1.115" alone also matches prereleases and 0.1.1151.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        require = runner["require_exact_published_version"]
        valid = {"exit_code": 0, "timeout": False,
                 "stdout_tail": "agentplugins 0.1.115\n"}
        require(valid)
        for version in ("agentplugins 0.1.115-dev",
                        "agentplugins 0.1.1151",
                        "agentplugins 0.1.114",
                        "agentplugins 0.1.103",
                        "other-launcher 0.1.115",
                        "agentplugins 0.1.115\nagentplugins 0.1.115",
                        ""):
            with self.subTest(version=version):
                with self.assertRaisesRegex(AssertionError, "not exact published"):
                    require({**valid, "stdout_tail": version})
        for invalid in ({"exit_code": 1}, {"timeout": True},
                        {"stdout_tail": None}):
            with self.subTest(invalid=invalid):
                with self.assertRaisesRegex(AssertionError, "not exact published"):
                    require({**valid, **invalid})

    def test_marketing_kit_private_cache_version_retry_is_bounded_and_strict(self):
        # The workflow's initial version check warms a different launcher cache.
        # Retry acquisition inside the disposable profile, not mismatched releases.
        from runpy import run_path
        from unittest.mock import Mock

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_pinned_launcher"]
        failed = {"exit_code": 1, "timeout": False, "stdout_tail": ""}
        timed_out = {"exit_code": None, "timeout": True, "stdout_tail": ""}
        valid = {"exit_code": 0, "timeout": False,
                 "stdout_tail": "agentplugins 0.1.115\n"}
        probe, pause = Mock(side_effect=[failed, timed_out, valid]), Mock()
        with patch.dict(verify.__globals__, {"invoke": probe, "sleep": pause}):
            verify("agentplugins", {}, Path("."))
        self.assertEqual(probe.call_count, 3)
        self.assertEqual([item.args for item in pause.call_args_list], [(5,), (10,)])

        probe, pause = Mock(return_value=failed), Mock()
        with patch.dict(verify.__globals__, {"invoke": probe, "sleep": pause}):
            with self.assertRaisesRegex(AssertionError, "not exact published"):
                verify("agentplugins", {}, Path("."))
        self.assertEqual(probe.call_count, 3)
        self.assertEqual([item.args for item in pause.call_args_list], [(5,), (10,)])

        probe, pause = Mock(return_value={**valid, "stdout_tail": "agentplugins 0.1.115-dev"}), Mock()
        with patch.dict(verify.__globals__, {"invoke": probe, "sleep": pause}):
            with self.assertRaisesRegex(AssertionError, "not exact published"):
                verify("agentplugins", {}, Path("."))
        self.assertEqual(probe.call_count, 1)
        pause.assert_not_called()

    def test_marketing_kit_released_binary_cache_preserves_profile_isolation(self):
        # A and B share only the SHA-checked npm launcher executable cache;
        # neither project nor native profile state may cross scopes.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        bind = runner["bind_pinned_launcher_cache"]
        with TemporaryDirectory() as temporary, TemporaryDirectory() as external:
            sandbox = Path(temporary).resolve()
            cache = sandbox / "released-cli-cache"
            cache.mkdir(mode=0o700)
            env_a, *_ = runner["run_case"]("unused", "codex", sandbox, "A")
            env_b, *_ = runner["run_case"]("unused", "codex", sandbox, "B")
            bind(env_a, cache, sandbox)
            bind(env_b, cache, sandbox)
            self.assertEqual(env_a["AGENTPLUGINS_CACHE_DIR"], str(cache))
            self.assertEqual(env_b["AGENTPLUGINS_CACHE_DIR"], str(cache))
            for key in ("HOME", "AGENTPLUGINS_HOME", "XDG_CACHE_HOME", "CODEX_HOME"):
                self.assertNotEqual(env_a[key], env_b[key], key)
            with self.assertRaisesRegex(AssertionError, "in-sandbox"):
                bind(env_a, Path(external), sandbox)
            self.assertEqual(env_a["AGENTPLUGINS_CACHE_DIR"], str(cache))
            link = sandbox / "untrusted-cache-link"
            try:
                link.symlink_to(Path(external), target_is_directory=True)
            except (OSError, NotImplementedError):
                pass
            else:
                with self.assertRaisesRegex(AssertionError, "in-sandbox"):
                    bind(env_b, link, sandbox)
                self.assertEqual(env_b["AGENTPLUGINS_CACHE_DIR"], str(cache))

    def test_marketing_kit_foreign_guard_rejects_symlinked_ancestor(self):
        # Regression: a native-profile directory can be replaced after add.
        # A matching marker behind an outside symlink must not be read as proof.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        guard = runner["check_foreign"]
        with TemporaryDirectory() as temporary, TemporaryDirectory() as external:
            sandbox = Path(temporary).resolve()
            outside = Path(external).resolve()
            native = sandbox / "A" / "home" / ".copilot"
            native.mkdir(parents=True)
            marker = native / "unmanaged-native-do-not-change.txt"
            content = "unmanaged native settings\\n"
            marker.write_text(content, encoding="utf-8")
            entries = runner["pin_foreign"]([(marker, content)], sandbox)
            guard(entries, sandbox)

            native.rename(native.parent / "original-profile")
            outside_marker = outside / marker.name
            outside_marker.write_text(content, encoding="utf-8")
            try:
                native.symlink_to(outside, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                guard(entries, sandbox)
            self.assertEqual(outside_marker.read_text(encoding="utf-8"), content)
    def test_marketing_kit_foreign_guard_rejects_replaced_skill_ancestors(self):
        # Moving the original SKILL.md preserves its inode/bytes; replacing
        # either owning directory must still fail foreign-state isolation.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            skill = case / "A" / "home" / ".agents" / "skills" / "foreign" / "SKILL.md"
            skill.parent.mkdir(parents=True)
            original = "---\nname: foreign\n---\n"
            skill.write_text(original, encoding="utf-8")
            pinned = runner["pin_foreign"]([(skill, original)], case)
            before = skill.lstat()

            for replaced in (skill.parent, skill.parent.parent.parent):
                with self.subTest(replaced=str(replaced)):
                    saved = replaced.with_name(replaced.name + "-saved")
                    replaced.rename(saved)
                    replaced.mkdir()
                    if replaced == skill.parent:
                        (saved / "SKILL.md").rename(skill)
                    else:
                        (saved / "skills").rename(replaced / "skills")
                    try:
                        after = skill.lstat()
                        self.assertEqual((after.st_dev, after.st_ino),
                                         (before.st_dev, before.st_ino))
                        with self.assertRaisesRegex(
                                AssertionError, "foreign content modified"):
                            runner["check_foreign"](pinned, case)
                    finally:
                        if replaced == skill.parent:
                            skill.rename(saved / "SKILL.md")
                        else:
                            (replaced / "skills").rename(saved / "skills")
                        replaced.rmdir()
                        saved.rename(replaced)
                    runner["check_foreign"](pinned, case)

    def test_marketing_kit_scoped_snapshot_fails_closed_on_walk_error(self):
        # os.walk skips unreadable descendants without onerror; an unchanged
        # unreadable tree is not proof that foreign project state was preserved.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / "project"
            root.mkdir()
            self.assertEqual(runner["scoped_snapshot"]([root])["root-kind:0"],
                             "directory")

            def inaccessible(_root, **kwargs):
                onerror = kwargs.get("onerror")
                if onerror:
                    onerror(PermissionError("unreadable protected subtree"))
                return iter(())

            with patch.dict(runner["audit"], {"snapshot": lambda _roots: {}}):
                with patch("os.walk", side_effect=inaccessible):
                    with self.assertRaisesRegex(
                            AssertionError, "protected scope snapshot unreadable"):
                        runner["scoped_snapshot"]([root])

    def test_marketing_kit_scoped_snapshot_rejects_unreadable_file_hash(self):
        # The shared audit returns a stable 'unreadable' sentinel for failed
        # file reads. Two identical sentinels are not byte-for-byte evidence.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / "project"
            root.mkdir()
            with patch.dict(runner["audit"], {
                    "snapshot": lambda _roots: {"project/foreign": "unreadable"}}):
                with self.assertRaisesRegex(
                        AssertionError, "protected scope snapshot unreadable"):
                    runner["scoped_snapshot"]([root])

    def test_marketing_kit_foreign_guard_rejects_newline_rewrite(self):
        # Text-mode newline normalization can hide a byte-changing rewrite.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            marker = root / "foreign-user-do-not-change.txt"
            expected = "unmanaged user data\n"
            marker.write_bytes(expected.encode("utf-8"))
            pinned = runner["pin_foreign"]([(marker, expected)], root)
            runner["check_foreign"](pinned, root)
            allowed = root / "new-owned-config.json"
            allowed.write_text("{}", encoding="utf-8")
            runner["check_foreign"](pinned, root)
            allowed.unlink()

            marker.write_bytes(b"unmanaged user data\r\n")
            self.assertEqual(marker.read_text(encoding="utf-8"), expected)
            with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                runner["check_foreign"](pinned, root)

            # Even identical bytes must not excuse a chmod or inode swap of
            # unmanaged state in the profile being actively installed.
            marker.write_bytes(expected.encode("utf-8"))
            # Re-pin after the deliberate byte-corruption case above.
            pinned = runner["pin_foreign"]([(marker, expected)], root)
            original_stat = marker.stat()
            try:
                os.utime(marker, ns=(original_stat.st_atime_ns,
                                     original_stat.st_mtime_ns - 2_000_000_000))
                with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                    runner["check_foreign"](pinned, root)
            finally:
                os.utime(marker, ns=(original_stat.st_atime_ns,
                                     original_stat.st_mtime_ns))
            runner["check_foreign"](pinned, root)
            original_mode = marker.stat().st_mode & 0o7777
            try:
                marker.chmod(original_mode ^ 0o200)
                with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                    runner["check_foreign"](pinned, root)
            finally:
                marker.chmod(original_mode)
            runner["check_foreign"](pinned, root)

            saved = root / "saved-original-foreign"
            marker.replace(saved)
            try:
                marker.write_bytes(expected.encode("utf-8"))
                with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                    runner["check_foreign"](pinned, root)
            finally:
                marker.unlink(missing_ok=True)
                saved.replace(marker)
            runner["check_foreign"](pinned, root)

    def _marketing_kit_public_identity_row(self, runner, distribution,
                                           target="codex", scope="project",
                                           installation_id="owned"):
        """Use the actual public list projection of the unchanged signed tuple."""
        pin = runner["FIXED_SIGNED_CLOSURE"][distribution]
        binding = {"client_id": target, "scope": scope,
                   "verification": "installation_verified", "materialization": "materialized",
                   "package_revision": {"distribution_id": distribution,
                       "version": pin["package_version"],
                       **{key: pin[key] for key in ("release_sequence", "resolved_revision",
                                                   "tree_digest", "manifest_digest")}}}
        if target == "copilot" and scope == "user":
            binding["affected_surfaces"] = ["copilot", "vscode"]
        return {"installation_id": installation_id,
                "source": pin["repository"] + "/" + pin["package_path"],
                "directory": {"recorded_distribution": distribution,
                              "recorded_revision": pin["resolved_revision"],
                              "recorded_release_sequence": pin["release_sequence"]},
                "clients": [binding]}

    def test_marketing_kit_installed_identity_requires_fixed_signed_tuple(self):
        # Internally consistent public records must not replace the fixed
        # release while independent info/lock observations remain unchanged.
        from copy import deepcopy
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["identities"]
        for target in ("codex", "cursor", "claude", "gemini", "opencode", "copilot"):
            for scope in ("user", "project"):
                for distribution in runner["FIXED_SIGNED_CLOSURE"]:
                    original = self._marketing_kit_public_identity_row(
                        runner, distribution, target, scope)
                    expected = {distribution: "owned"}
                    with self.subTest(target=target, scope=scope, distribution=distribution):
                        self.assertEqual(check([original], target, scope), expected)
                        keyed = deepcopy(original)
                        keyed["clients"] = {"binding": keyed["clients"][0]}
                        self.assertEqual(check([keyed], target, scope), expected)
                    for mutation in ("source", "revision", "sequence", "version",
                                     "tree", "manifest", "directory-bool", "directory-float",
                                     "binding-bool", "binding-float", "foreign-distribution"):
                        changed = deepcopy(original)
                        directory, revision = changed["directory"], changed["clients"][0]["package_revision"]
                        if mutation == "source": changed["source"] = "foreign/repository/plugins/marketing-kit"
                        elif mutation == "revision":
                            directory["recorded_revision"] = revision["resolved_revision"] = "f" * 40
                        elif mutation == "sequence":
                            directory["recorded_release_sequence"] = revision["release_sequence"] = 99
                        elif mutation == "version": revision["version"] = "9.9.9"
                        elif mutation == "tree": revision["tree_digest"] = "sha256:" + "f" * 64
                        elif mutation == "manifest": revision["manifest_digest"] = "sha256:" + "f" * 64
                        elif mutation == "directory-bool": directory["recorded_release_sequence"] = True
                        elif mutation == "directory-float": directory["recorded_release_sequence"] = float(directory["recorded_release_sequence"])
                        elif mutation == "binding-bool": revision["release_sequence"] = True
                        elif mutation == "binding-float": revision["release_sequence"] = float(revision["release_sequence"])
                        elif mutation == "foreign-distribution":
                            directory["recorded_distribution"] = revision["distribution_id"] = "777genius/unreviewed"
                        with self.subTest(target=target, scope=scope,
                                          distribution=distribution, mutation=mutation):
                            with self.assertRaises(AssertionError):
                                check([changed], target, scope)

    def test_marketing_kit_every_selected_binding_requires_fixed_release(self):
        from copy import deepcopy
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        for scope in ("user", "project"):
            row = self._marketing_kit_public_identity_row(
                runner, "777genius/marketing-kit", scope=scope)
            row["clients"].append(deepcopy(row["clients"][0]))
            row["clients"][1]["package_revision"]["tree_digest"] = "sha256:" + "f" * 64
            with self.subTest(scope=scope), self.assertRaisesRegex(
                    AssertionError, "binding differs from fixed signed release"):
                runner["identities"]([row], "codex", scope)

    def test_marketing_kit_identity_guard_rejects_missing_or_shared_ids(self):
        # A missing ID can make the repeat/repair comparison falsely pass.
        # Distinct signed distributions must also retain distinct owned IDs.
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        def row(distribution, installation_id):
            return self._marketing_kit_public_identity_row(
                runner, distribution, installation_id=installation_id)
        root, child = "777genius/marketing-kit", "777genius/notion"
        check = runner["identities"]
        for bad in (None, "", "  "):
            with self.assertRaisesRegex(AssertionError, "missing installation ID"):
                check([row(root, bad)], "codex")
        with self.assertRaisesRegex(AssertionError, "duplicate installation ID"):
            check([row(root, "same"), row(child, "same")], "codex")
        self.assertEqual(check([row(root, "one"), row(child, "two")], "codex"),
                         {root: "one", child: "two"})

    def test_marketing_kit_identity_guard_rejects_cross_scope_provenance(self):
        # A signed User binding must never authorize an unsigned Project binding.
        from copy import deepcopy
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        distribution = "777genius/marketing-kit"
        row = self._marketing_kit_public_identity_row(runner, distribution)
        signed = deepcopy(row["clients"][0])
        row["clients"][0]["package_revision"] = None
        row["clients"].append({**deepcopy(signed), "scope": "user"})
        check = runner["identities"]
        with self.assertRaisesRegex(AssertionError, "project binding lacks exact"):
            check([row], "codex")
        row["clients"] = [signed, {**deepcopy(signed), "scope": "user", "package_revision": None}]
        self.assertEqual(check([row], "codex"), {distribution: "owned"})
        row["clients"].append({**deepcopy(signed), "package_revision": None})
        with self.assertRaisesRegex(AssertionError, "project binding lacks exact"):
            check([row], "codex")

    def test_marketing_kit_user_identity_guard_rejects_project_only_signature(self):
        # A signed Project binding cannot authorize an unsigned User binding.
        from copy import deepcopy
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        distribution = "777genius/marketing-kit"
        row = self._marketing_kit_public_identity_row(runner, distribution, scope="user")
        signed = deepcopy(row["clients"][0])
        row["clients"][0]["package_revision"] = None
        row["clients"].append({**deepcopy(signed), "scope": "project"})
        with self.assertRaisesRegex(AssertionError, "user binding lacks exact"):
            runner["identities"]([row], "codex", "user")
        row["clients"][0]["package_revision"] = signed["package_revision"]
        self.assertEqual(runner["identities"]([row], "codex", "user"),
                         {distribution: "owned"})

    def test_marketing_kit_requires_installed_verification_in_selected_scope(self):
        # Signed provenance and a persisted list entry do not prove delivery.
        # Prepared/package-only and failed bindings cannot qualify E2E.
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        distribution = "777genius/marketing-kit"
        row = self._marketing_kit_public_identity_row(runner, distribution, scope="user")
        binding = row["clients"][0]
        check = runner["identities"]
        self.assertEqual(check([row], "codex", "user"), {distribution: "owned"})
        for invalid in (None, "", "package_validated", "not_checked",
                        "verification_failed", "manual_activation_required"):
            with self.subTest(verification=invalid):
                row["clients"] = [{**binding, "verification": invalid}]
                with self.assertRaisesRegex(AssertionError, "lacks installed verification"):
                    check([row], "codex", "user")
        row["clients"] = [binding, {**binding, "verification": "package_validated"}]
        with self.assertRaisesRegex(AssertionError, "lacks installed verification"):
            check([row], "codex", "user")
        row["clients"] = [{**binding, "materialization": "staged"}]
        with self.assertRaisesRegex(AssertionError, "lacks materialized delivery"):
            check([row], "codex", "user")
        # Cursor User deliberately prepares a package with an explicit
        # manual-activation requirement, not an installed native MCP.
        prepared = {**binding, "client_id": "cursor",
                    "verification": "package_validated",
                    "activation": "manual_activation_required",
                    "policy": "allowed"}
        row["clients"] = [prepared]
        self.assertEqual(check([row], "cursor", "user"), {distribution: "owned"})
        for change in ({"activation": "prepared"}, {"policy": "blocked"},
                       {"materialization": "degraded"}, {"verification": "failed"}):
            row["clients"] = [{**prepared, **change}]
            with self.subTest(change=change), self.assertRaises(AssertionError):
                check([row], "cursor", "user")
        # Only Cursor User can take the documented manual preparation path.
        row["clients"] = [{**prepared, "client_id": "codex"}]
        with self.assertRaisesRegex(AssertionError, "lacks installed verification"):
            check([row], "codex", "user")
        row["clients"] = [{**prepared, "scope": "project"}]
        with self.assertRaisesRegex(AssertionError, "lacks installed verification"):
            check([row], "cursor", "project")
        # An unverified foreign scope must not overwrite the selected User proof.
        row["clients"] = [binding, {**binding, "scope": "project",
                                   "verification": "package_validated"}]
        self.assertEqual(check([row], "codex", "user"), {distribution: "owned"})

    def test_marketing_kit_user_remove_rejects_active_native_residuals(self):
        # An empty CLI list alone cannot prove native client removal.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home = case / "home"
            native = home / ".copilot"
            native.mkdir(parents=True)
            settings = native / "settings.json"
            check = runner["check_removed_native_components"]
            for identity in ("marketing-kit", "notion", "hubspot-crm",
                             "playwright-bridge", "playwright"):
                settings.write_text(
                    '{"enabledPlugins":{"' + identity +
                    '@agentplugins-aaaaaaaaaaaa":true}}', encoding="utf-8")
                with self.subTest(identity=identity), self.assertRaisesRegex(
                        AssertionError, "native Marketing Kit remnants"):
                    check("copilot", home, case, "User remove")
            settings.write_text('{"enabledPlugins":{"other@foreign":true}}',
                                encoding="utf-8")
            check("copilot", home, case, "User remove")
            # Project cleanup must cover native configuration under both
            # Project and HOME, without conflating their separate roots.
            project = case / "project"
            native_project = project / ".copilot"
            native_project.mkdir(parents=True)
            project_settings = native_project / "settings.json"
            project_settings.write_text(
                '{"enabledPlugins":{"marketing-kit@agentplugins-aaaaaaaaaaaa":true}}',
                encoding="utf-8")
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", project, case, "Project remove")
            project_settings.write_text('{"enabledPlugins":{"other@foreign":true}}',
                                        encoding="utf-8")
            check("copilot", project, case, "Project remove")
            # Checking Project does not silently cover HOME; caller must do both.
            settings.write_text(
                '{"enabledPlugins":{"marketing-kit@agentplugins-aaaaaaaaaaaa":true}}',
                encoding="utf-8")
            check("copilot", project, case, "Project remove")
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", home, case, "Project native HOME remove")
            settings.unlink()
            native.rmdir()
            try:
                native.symlink_to(case / "untrusted", target_is_directory=True)
            except (OSError, NotImplementedError):
                return
            with self.assertRaisesRegex(AssertionError, "unsafe native client root"):
                check("copilot", home, case, "User remove")

    def test_marketing_kit_removed_native_root_rejects_file_ancestors(self):
        # A non-directory native root or intermediate path cannot prove removal.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home = case / "home"
            home.mkdir()
            for unsafe in (home / ".copilot", home / ".config",
                           home / ".config" / "copilot"):
                unsafe.parent.mkdir(parents=True, exist_ok=True)
                unsafe.write_text("occupied", encoding="utf-8")
                with self.subTest(unsafe=unsafe), self.assertRaisesRegex(
                        AssertionError, "invalid native client root"):
                    check("copilot", home, case, "remove")
                unsafe.unlink()
            check("copilot", home, case, "remove")

    def test_marketing_kit_native_cleanup_refuses_in_sandbox_profile_aliases(self):
        # Reuse the shared path_inside_case guard: it must reject symlinks
        # even when they resolve to another profile *inside* the sandbox.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            a = case / "A"
            a.mkdir()
            (a / "home").mkdir()
            b = case / "B"
            b.mkdir()
            alias = b / "home"
            try:
                alias.symlink_to(a / "home", target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            self.assertTrue(alias.is_dir())  # Symlink resolves within sandbox.
            with self.assertRaisesRegex(AssertionError, "native HOME left disposable case"):
                check("copilot", alias, case, "remove B")
            alias.unlink()
            b.rmdir()
            b.symlink_to(a, target_is_directory=True)
            with self.assertRaisesRegex(AssertionError, "native HOME left disposable case"):
                check("copilot", b / "home", case, "remove B")
            b.unlink()
            b.mkdir()
            (b / "home").mkdir()
            check("copilot", b / "home", case, "remove B")

    def test_marketing_kit_user_and_project_foreign_skills_are_pinned(self):
        # Protect real native User Skills independently of Project Skills;
        # Codex, OpenCode, Cursor, Gemini and Copilot also discover compatibility Skills.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        # These expected paths are independent of the harness root selector:
        # otherwise the test could replicate an incorrect native mapping.
        for target, project_root, user_roots in (
                ("claude", ".claude/skills", (".claude/skills",)),
                ("copilot", ".github/skills", (".copilot/skills", ".agents/skills")),
                ("codex", ".agents/skills", (".codex/skills", ".agents/skills")),
                ("opencode", ".opencode/skills",
                 (".config/opencode/skills", ".agents/skills", ".claude/skills")),
                ("cursor", ".cursor/skills",
                 (".cursor/skills", ".agents/skills", ".claude/skills",
                  ".codex/skills")),
                ("gemini", ".gemini/skills",
                 (".gemini/skills", ".agents/skills"))):
            with self.subTest(target=target), TemporaryDirectory() as temporary:
                case = Path(temporary).resolve()
                env, project, _, _, entries = runner["run_case"](
                    "agentplugins", target, case, "A")
                name = "unmanaged-uap-foreign-skill"
                expected_skills = {
                    project / project_root / name / "SKILL.md",
                    *(Path(env["HOME"]) / root / name / "SKILL.md"
                      for root in user_roots),
                }
                compatible_project = {
                    "opencode": (".agents/skills", ".claude/skills"),
                    "cursor": (".agents/skills", ".claude/skills", ".codex/skills"),
                    "gemini": (".agents/skills",),
                    "copilot": (".agents/skills", ".claude/skills"),
                }
                expected_skills.update({
                    project / root / name / "SKILL.md"
                    for root in compatible_project.get(target, ())
                })
                skills = [(path, content) for path, content in entries
                          if path.name == "SKILL.md"]
                self.assertEqual({path for path, _ in skills}, expected_skills)
                references = [(path, content) for path, content in entries
                              if path.name == "unmanaged-context.md"]
                self.assertEqual(
                    {path for path, _ in references},
                    {skill.parent / "references" / "unmanaged-context.md"
                     for skill in expected_skills})
                pinned = runner["pin_foreign"](entries, case)
                runner["check_foreign"](pinned, case)
                for skill, original in skills:
                    with self.subTest(skill=str(skill)):
                        original_stat = skill.stat()
                        skill.write_text(original + "rewritten\n", encoding="utf-8")
                        with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                            runner["check_foreign"](pinned, case)
                        skill.write_text(original, encoding="utf-8")
                        os.utime(skill, ns=(original_stat.st_atime_ns,
                                            original_stat.st_mtime_ns))
                        runner["check_foreign"](pinned, case)
                        injected = skill.parent / "unexpected-native-file.txt"
                        injected.write_text("unexpected", encoding="utf-8")
                        try:
                            with self.assertRaisesRegex(
                                    AssertionError, "foreign content modified"):
                                runner["check_foreign"](pinned, case)
                        finally:
                            injected.unlink()
                        runner["check_foreign"](pinned, case)
                for reference, original in references:
                    with self.subTest(reference=str(reference)):
                        original_stat = reference.stat()
                        reference.write_text("tampered\n", encoding="utf-8")
                        with self.assertRaisesRegex(
                                AssertionError, "foreign content modified"):
                            runner["check_foreign"](pinned, case)
                        reference.write_text(original, encoding="utf-8")
                        os.utime(reference, ns=(original_stat.st_atime_ns,
                                                original_stat.st_mtime_ns))
                        runner["check_foreign"](pinned, case)
                        injected = reference.parent / "unmanaged-extra.md"
                        injected.write_text("unexpected\n", encoding="utf-8")
                        try:
                            with self.assertRaisesRegex(
                                    AssertionError, "foreign content modified"):
                                runner["check_foreign"](pinned, case)
                        finally:
                            injected.unlink()
                        runner["check_foreign"](pinned, case)

    def test_marketing_kit_opencode_compat_skills_must_be_removed(self):
        # Official OpenCode discovery includes .agents/.claude Skills in
        # both scopes; empty owned native directories must not be a false PASS.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            for scope in ("home", "project"):
                base = case / scope
                base.mkdir()
                for compatible in (".agents", ".claude"):
                    with self.subTest(scope=scope, compatible=compatible):
                        skills = base / compatible / "skills"
                        foreign = skills / "unmanaged-uap-foreign-skill"
                        foreign.mkdir(parents=True)
                        (foreign / "SKILL.md").write_text(
                            "---\nname: unmanaged-uap-foreign-skill\n"
                            "description: keep unchanged\n---\n", encoding="utf-8")
                        check("opencode", base, case, "remove")
                        for owned_name in ("marketing-kit",
                                           "playwright@agentplugins-aaaaaaaaaaaa"):
                            owned = skills / owned_name
                            owned.mkdir()
                            with self.assertRaisesRegex(
                                    AssertionError, "native Marketing Kit remnants"):
                                check("opencode", base, case, "remove")
                            owned.rmdir()
                        check("opencode", base, case, "remove")

    def test_marketing_kit_cursor_compat_skills_must_be_removed(self):
        # Cursor documents .agents/.claude/.codex compatibility Skills in
        # User and Project scopes, not only the primary .cursor/skills tree.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            for scope in ("home", "project"):
                base = case / scope
                base.mkdir()
                for compatible in (".agents", ".claude", ".codex"):
                    with self.subTest(scope=scope, compatible=compatible):
                        skills = base / compatible / "skills"
                        foreign = skills / "unmanaged-uap-foreign-skill"
                        foreign.mkdir(parents=True)
                        (foreign / "SKILL.md").write_text(
                            "---\nname: unmanaged-uap-foreign-skill\n"
                            "description: keep unchanged\n---\n", encoding="utf-8")
                        check("cursor", base, case, "remove")
                        owned = skills / "marketing-kit"
                        owned.mkdir()
                        with self.assertRaisesRegex(
                                AssertionError, "native Marketing Kit remnants"):
                            check("cursor", base, case, "remove")
                        owned.rmdir()
                        check("cursor", base, case, "remove")

    def test_marketing_kit_gemini_alias_skill_cleanup(self):
        # Gemini's documented native .agents/skills alias is discovered
        # independently in User and Project scope.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            for scope in ("home", "project"):
                base = case / scope
                base.mkdir()
                skills = base / ".agents" / "skills"
                foreign = skills / "unmanaged-uap-foreign-skill"
                foreign.mkdir(parents=True)
                (foreign / "SKILL.md").write_text(
                    "---\nname: unmanaged-uap-foreign-skill\n"
                    "description: keep unchanged\n---\n", encoding="utf-8")
                check("gemini", base, case, "remove")
                owned = skills / "marketing-kit"
                owned.mkdir()
                with self.subTest(scope=scope), self.assertRaisesRegex(
                        AssertionError, "native Marketing Kit remnants"):
                    check("gemini", base, case, "remove")
                owned.rmdir()
                check("gemini", base, case, "remove")

    def test_marketing_kit_copilot_project_skill_removal_scans_github_root(self):
        # Copilot Project Skills live in .github/skills, outside MCP config roots.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            project = case / "project"
            owned = project / ".github" / "skills" / "marketing-kit"
            other = project / ".github" / "skills" / "foreign-skill"
            owned.mkdir(parents=True)
            other.mkdir()
            (other / "SKILL.md").write_text("name: foreign-skill\n", encoding="utf-8")
            file = owned / "SKILL.md"
            file.write_text("name: marketing-kit\n", encoding="utf-8")
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", project, case, "Project remove")
            file.unlink()
            # Empty named Skill directories are absent from audit.snapshot()
            # yet remain native residue after a claimed successful remove.
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", project, case, "Project remove")
            owned.rmdir()
            owned_with_receipt = owned.parent / "marketing-kit@agentplugins-aaaaaaaaaaaa"
            owned_with_receipt.mkdir()
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", project, case, "Project remove")
            owned_with_receipt.rmdir()
            # Removing owned Skills must not require deleting a foreign Skill.
            check("copilot", project, case, "Project remove", project_scope=True)
            for compatible in (".agents", ".claude"):
                with self.subTest(compatible=compatible):
                    skill_root = project / compatible / "skills"
                    other_skill = skill_root / "foreign-skill"
                    other_skill.mkdir(parents=True)
                    (other_skill / "SKILL.md").write_text(
                        "---\nname: foreign-skill\ndescription: preserve\n---\n",
                        encoding="utf-8")
                    check("copilot", project, case, "Project remove", project_scope=True)
                    owned_skill = skill_root / "marketing-kit"
                    owned_skill.mkdir()
                    with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                        check("copilot", project, case, "Project remove", project_scope=True)
                    owned_skill.rmdir()
                    check("copilot", project, case, "Project remove", project_scope=True)

    def test_marketing_kit_copilot_project_mcp_cleanup_checks_both_locations(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            project = case / "project"
            (project / ".github").mkdir(parents=True)
            paths = [project / name for name in (".github/mcp.json", ".mcp.json")]
            # Foreign members may refer to a closure alias in their own values;
            # server member identity, rather than whole-file text, is decisive.
            foreign = {"mcpServers": {"foreign-neighbor": {
                "command": "notion", "args": ["playwright", "marketing-kit"]}}}
            original = json.dumps(foreign).encode()
            for path in paths:
                path.write_bytes(original)
            check("copilot", project, case, "remove", project_scope=True)
            for path in paths:
                for alias in ("notion", "playwright", "playwright-bridge",
                              "playwright@agentplugins-aaaaaaaaaaaa"):
                    with self.subTest(path=path.name, alias=alias):
                        changed = {"mcpServers": {**foreign["mcpServers"], alias: {"command": "probe"}}}
                        path.write_text(json.dumps(changed))
                        with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                            check("copilot", project, case, "remove", project_scope=True)
                        path.write_bytes(original)
                        check("copilot", project, case, "remove", project_scope=True)
                        self.assertTrue(all(item.read_bytes() == original for item in paths))
            for path in paths:
                for clean in ({}, {"mcpServers": {}}):
                    path.write_text(json.dumps(clean))
                    check("copilot", project, case, "remove", project_scope=True)
                path.unlink()
            check("copilot", project, case, "remove", project_scope=True)

    def test_marketing_kit_copilot_project_mcp_cleanup_rejects_malformed_config(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            project = case / "project"
            (project / ".github").mkdir(parents=True)
            for relative in (".github/mcp.json", ".mcp.json"):
                path = project / relative
                for body in ('invalid', '[]', '{"mcpServers":[]}',
                             '{"notion":{}}', '{"mcpServers":{"foreign":null}}',
                             '{"mcpServers":{"foreign":{"value":NaN}}}',
                             '{"mcpServers":{"notion":{},"notion":{}}}',
                             '{"mcpServers":{"notion":{}},"mcpServers":{}}',
                             '{"mcpServers":{}} trailing'):
                    with self.subTest(relative=relative, body=body):
                        path.write_text(body)
                        with self.assertRaisesRegex(AssertionError, "malformed Copilot Project MCP"):
                            runner["check_removed_native_components"](
                                "copilot", project, case, "remove", project_scope=True)
                path.unlink()

    def test_marketing_kit_copilot_project_mcp_cleanup_rejects_unsafe_paths(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        for relative in (".github/mcp.json", ".mcp.json"):
            with self.subTest(relative=relative), TemporaryDirectory() as temporary:
                case = Path(temporary).resolve()
                project, outside = case / "project", case / "outside"
                (project / ".github").mkdir(parents=True)
                outside.mkdir()
                external = outside / "foreign.json"
                original = b'{"mcpServers":{"foreign":{}}}'
                external.write_bytes(original)
                path = project / relative
                try:
                    path.symlink_to(external)
                except (OSError, NotImplementedError) as error:
                    self.skipTest(f"host cannot create symlinks: {error}")
                with self.assertRaisesRegex(AssertionError, "unsafe Copilot Project MCP"):
                    runner["check_removed_native_components"](
                        "copilot", project, case, "remove", project_scope=True)
                self.assertEqual(external.read_bytes(), original)
                path.unlink()
                path.mkdir()
                with self.assertRaisesRegex(AssertionError, "invalid Copilot Project MCP"):
                    runner["check_removed_native_components"](
                        "copilot", project, case, "remove", project_scope=True)
                path.rmdir()
                if relative.startswith(".github/"):
                    (project / ".github").rmdir()
                    (project / ".github").symlink_to(outside, target_is_directory=True)
                    with self.assertRaisesRegex(AssertionError, "unsafe native client root"):
                        runner["check_removed_native_components"](
                            "copilot", project, case, "remove", project_scope=True)

    def test_marketing_kit_copilot_project_mcp_cleanup_rejects_hardlinks_and_open_swap(self):
        # A native child can substitute the MCP path after lstat; never read a
        # same-inode symlink or shared hardlink outside the disposable profile.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["copilot_project_mcp_residuals"]
        for relative in (".github/mcp.json", ".mcp.json"):
            with self.subTest(relative=relative), TemporaryDirectory() as temporary:
                case = Path(temporary).resolve()
                project = case / "project"
                (project / ".github").mkdir(parents=True)
                path = project / relative
                original = b'{"mcpServers":{"foreign-neighbor":{}}}'
                path.write_bytes(original)
                linked = case / "linked.json"
                try:
                    os.link(path, linked)
                except (OSError, NotImplementedError) as error:
                    self.skipTest(f"hardlinks unavailable: {error}")
                with self.assertRaisesRegex(AssertionError, "invalid Copilot Project MCP"):
                    check(project, {"notion"}, "remove", case)
                linked.unlink()
                probe = case / "symlink-probe"
                try:
                    probe.symlink_to(path)
                except (OSError, NotImplementedError) as error:
                    self.skipTest(f"symlinks unavailable: {error}")
                probe.unlink()
                moved = case / "original.json"
                real_open = os.open
                swapped = False

                def racing_open(candidate, flags, *args, **kwargs):
                    nonlocal swapped
                    if (Path(candidate) == path or (candidate == path.name and kwargs.get("dir_fd") is not None)) and not swapped:
                        swapped = True
                        path.rename(moved)
                        path.symlink_to(moved)
                    return real_open(candidate, flags, *args, **kwargs)

                if os.name == "nt":
                    reader_globals = runner["audit"]["copilot_settings_read"].__globals__
                    descriptor = reader_globals["copilot_settings_windows_descriptor"]
                    def racing_descriptor(candidate, *args):
                        nonlocal swapped
                        swapped = True
                        path.rename(moved)
                        path.symlink_to(moved)
                        return descriptor(candidate, *args)
                    with patch.dict(reader_globals, {"copilot_settings_windows_descriptor": racing_descriptor}):
                        with self.assertRaisesRegex(AssertionError, "(malformed|replaced) Copilot Project MCP"):
                            check(project, {"notion"}, "remove", case)
                else:
                    with patch.object(os, "open", side_effect=racing_open) as patched_open:
                        # Replacing os.open must not disable the rooted dir-fd
                        # branch we are testing; final open is relative to it.
                        with patch.object(os, "supports_dir_fd", os.supports_dir_fd | {patched_open}):
                            with self.assertRaisesRegex(AssertionError, "(malformed|replaced) Copilot Project MCP"):
                                check(project, {"notion"}, "remove", case)
                self.assertTrue(swapped)
                self.assertTrue(path.is_symlink())
                self.assertEqual(moved.read_bytes(), original)

    def test_marketing_kit_copilot_project_mcp_windows_race_seam_binds_actual_reader_globals(self):
        # Portable binding proof only; actual Win32 handle calls remain CI's job.
        import stat
        from runpy import run_path
        from types import SimpleNamespace
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        reader_globals = runner["audit"]["copilot_settings_read"].__globals__
        called = []
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            project = case / "project"
            path = project / ".github/mcp.json"
            path.parent.mkdir(parents=True)
            path.write_text('{"mcpServers":{}}')
            def descriptor(candidate, initial, expected):
                called.append(candidate)
                self.assertEqual(candidate, path)
                self.assertEqual(expected, path)
                self.assertTrue(stat.S_ISREG(initial.st_mode))
                raise OSError("mock held-handle refusal")
            native_os = SimpleNamespace(**{**vars(os), "name": "nt", "supports_dir_fd": set()})
            with patch.dict(reader_globals, {"os": native_os,
                    "copilot_settings_windows_descriptor": descriptor}):
                with self.assertRaisesRegex(AssertionError, "malformed Copilot Project MCP"):
                    runner["copilot_project_mcp_residuals"](project, {"notion"}, "remove", case)
            self.assertEqual(called, [path])

    def test_marketing_kit_copilot_project_mcp_ancestor_swap_never_reads_foreign_bytes(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        check = runner["copilot_project_mcp_residuals"]
        for ancestor in (".github", "project"):
            with self.subTest(ancestor=ancestor), TemporaryDirectory() as temporary:
                base = Path(temporary).resolve()
                case = base / "case"
                case.mkdir()
                project = case / "project"
                (project / ".github").mkdir(parents=True)
                path = project / ".github/mcp.json"
                path.write_text('{"mcpServers":{}}')
                foreign = base / "foreign"
                (foreign / ".github").mkdir(parents=True)
                foreign_path = foreign / ".github/mcp.json"
                foreign_path.write_text('{"mcpServers":{"private-foreign":{}}}')
                foreign_inode = (foreign_path.stat().st_dev, foreign_path.stat().st_ino)
                redirected = project if ancestor == "project" else project / ".github"
                destination = foreign if ancestor == "project" else foreign / ".github"
                moved = case / "moved-owned"
                original_guard = runner["audit"]["path_inside_case"]
                original_fdopen = os.fdopen
                swapped, foreign_reads = [], []
                def racing_guard(candidate, root):
                    allowed = original_guard(candidate, root)
                    if candidate == path and not swapped:
                        self.assertTrue(allowed)
                        redirected.rename(moved)
                        try:
                            redirected.symlink_to(destination, target_is_directory=True)
                        except (OSError, NotImplementedError) as error:
                            self.skipTest(f"symlinks unavailable: {error}")
                        swapped.append(True)
                    return allowed
                def checked_fdopen(descriptor, *args, **kwargs):
                    current = os.fstat(descriptor)
                    if (current.st_dev, current.st_ino) == foreign_inode:
                        foreign_reads.append(True)
                        raise AssertionError("attempted foreign read")
                    return original_fdopen(descriptor, *args, **kwargs)
                with patch.dict(runner["audit"], {"path_inside_case": racing_guard}), \
                        patch.object(os, "fdopen", side_effect=checked_fdopen):
                    with self.assertRaisesRegex(AssertionError, "malformed Copilot Project MCP"):
                        check(project, {"notion"}, "remove", case)
                self.assertEqual(swapped, [True])
                self.assertEqual(foreign_reads, [])

    def test_marketing_kit_copilot_project_mcp_observed_then_disappeared_is_not_absence(self):
        from runpy import run_path
        runner = run_path(str(Path(__file__).resolve().parent / "marketing-kit-project-release-e2e.py"))
        check = runner["copilot_project_mcp_residuals"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            project = case / "project"
            project.mkdir()
            self.assertEqual(check(project, {"notion"}, "remove", case), set())
            path = project / ".mcp.json"
            path.write_text('{"mcpServers":{}}')
            def disappear(*_):
                path.unlink()
                return None, "absent"
            with patch.dict(runner["audit"], {"copilot_settings_read": disappear}):
                with self.assertRaisesRegex(AssertionError, "malformed Copilot Project MCP"):
                    check(project, {"notion"}, "remove", case)

    def test_marketing_kit_copilot_personal_agents_skill_cleanup(self):
        # GitHub's Copilot CLI documents ~/.agents/skills as a personal root.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home = case / "home"
            skills = home / ".agents" / "skills"
            other = skills / "foreign-skill"
            other.mkdir(parents=True)
            (other / "SKILL.md").write_text(
                "---\nname: foreign-skill\ndescription: preserve\n---\n",
                encoding="utf-8")
            check("copilot", home, case, "User remove")
            owned = skills / "marketing-kit"
            owned.mkdir()
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", home, case, "User remove")
            owned.rmdir()
            check("copilot", home, case, "User remove")

    def test_marketing_kit_codex_attestation_requires_native_uninstall_proof(self):
        # A signed Project/User binding is not proof of native uninstallation.
        # Only the existing pinned native removal plus fresh absence grants the flag.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            case = Path(temporary) / "A"
            home, project = case / "home", case / "project"
            home.mkdir(parents=True)
            project.mkdir()
            env = {"HOME": str(home), "CODEX_HOME": str(home / ".codex")}
            check = runner["codex_user_native_remove_attestation"]
            calls = []

            def native_uninstall(identity, native_home, native_root, native_env, cwd):
                calls.append((identity, native_home, native_root, cwd))
                return {"completed": False, "reason": "native plugin still registered"}

            with patch.dict(runner["audit"], {
                    "uninstall_native_codex_plugin": native_uninstall}):
                with self.assertRaisesRegex(AssertionError, "native plugin still registered"):
                    check(env, project)
            self.assertEqual(calls, [("hubspot-crm", home, case, project)])

            # A merely truthy field cannot authorize a security attestation.
            for unverified in ({}, {"completed": "true"}, {"completed": False}):
                with patch.dict(runner["audit"], {
                        "uninstall_native_codex_plugin": lambda *_: unverified}):
                    with self.assertRaisesRegex(AssertionError, "unverified"):
                        check(env, project)
            with patch.dict(runner["audit"], {
                    "uninstall_native_codex_plugin": lambda *_: {"completed": True}}):
                self.assertEqual(check(env, project), ("--external-uninstalled",))
                with self.assertRaisesRegex(AssertionError, "exact disposable profile"):
                    check({**env, "CODEX_HOME": str(case / "foreign")}, project)

    def test_marketing_kit_project_a_guard_rejects_rekey_and_lock_rewrite(self):
        # A complete set of distributions is insufficient if B silently
        # replaces A's installation identities or changes A's signed lock.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_project_a_unchanged"]
        baseline = {"root": "id-a", "dependency": "id-child"}
        check(dict(baseline), baseline, b"pinned", b"pinned", "restore B")
        with self.assertRaisesRegex(AssertionError, "installation identities changed"):
            check({"root": "rekeyed", "dependency": "id-child"},
                  baseline, b"pinned", b"pinned", "restore B")
        with self.assertRaisesRegex(AssertionError, "signed lock changed"):
            check(dict(baseline), baseline, b"rewritten", b"pinned", "remove B")

    def test_marketing_kit_peer_baseline_waits_for_own_security_cache(self):
        # Own post-command writes may settle before the peer starts. No writes
        # observed *after* a peer operation are ever excused or rebaselined.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        settle = runner["settled_profile_snapshot"]
        states = iter(({"security": "starting"},
                       {"security": "completed"},
                       {"security": "completed"},
                       {"security": "completed"}))
        with patch.dict(settle.__globals__, {
                "profile_snapshot": lambda *_: next(states),
                "sleep": lambda _: None}):
            self.assertEqual(settle({}, Path("."), attempts=4, interval=0),
                             {"security": "completed"})

        changing = iter(({"cache": index} for index in range(5)))
        with patch.dict(settle.__globals__, {
                "profile_snapshot": lambda *_: next(changing),
                "sleep": lambda _: None}):
            with self.assertRaisesRegex(AssertionError, "did not quiesce"):
                settle({}, Path("."), attempts=5, interval=0)

    def test_marketing_kit_cross_profile_snapshot_detects_hidden_rewrites(self):
        # Sentinels and stable installation IDs do not prove that another
        # profile left native, HOME or UAP state untouched.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            env, project, *_ = runner["run_case"]("unused", "codex", root, "A")
            before = runner["profile_snapshot"](env, project)
            runner["check_profile_unchanged"](env, project, before, "restore B")
            native = Path(env["HOME"]) / ".codex" / "managed.json"
            native.write_bytes(b"before")
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, before, "restore B")
            after = runner["profile_snapshot"](env, project)
            original_stat = native.stat()
            native.write_bytes(b"after")
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, after, "remove B")
            native.write_bytes(b"before")
            os.utime(native, ns=(original_stat.st_atime_ns,
                                  original_stat.st_mtime_ns))
            runner["check_profile_unchanged"](env, project, after, "remove B")

            # Changes outside the selected client's native roots must fail too.
            sibling = Path(env["HOME"]) / ".config" / "other-app" / "settings.json"
            sibling.parent.mkdir(parents=True)
            sibling.write_bytes(b"foreign")
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, after, "restore B")
            sibling.unlink()
            # Clean the newly created empty ancestors as well; they are now
            # intentionally observable foreign-state mutations.
            sibling.parent.rmdir()
            sibling.parent.parent.rmdir()
            runner["check_profile_unchanged"](env, project, after, "restore B")

            # A's durable UAP receipt/data state must remain untouched by B/C.
            receipt = Path(env["AGENTPLUGINS_HOME"]) / "foreign-receipt.json"
            receipt.write_bytes(b"other installation")
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, after, "remove C")
            receipt.unlink()
            runner["check_profile_unchanged"](env, project, after, "remove C")

            # A peer must not create unexpected files inside Project A either.
            extra = project / "unexpected-peer-state.json"
            extra.write_bytes(b"foreign")
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, after, "restore B")
            extra.unlink()
            runner["check_profile_unchanged"](env, project, after, "restore B")

            # Empty directories are foreign state too; no file hash would catch them.
            empty = project / "unexpected-empty-peer-dir"
            empty.mkdir()
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, after, "restore B")
            empty.rmdir()
            runner["check_profile_unchanged"](env, project, after, "restore B")

    def test_marketing_kit_scoped_snapshot_rejects_metadata_only_changes(self):
        # Equal bytes alone cannot prove that a foreign native file was kept.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            env, project, *_ = runner["run_case"]("unused", "codex", root, "peer")
            foreign = Path(env["HOME"]) / ".codex" / "foreign-settings.json"
            foreign.write_bytes(b"same bytes")
            before = runner["profile_snapshot"](env, project)
            check = runner["check_profile_unchanged"]
            original_mode = foreign.stat().st_mode & 0o7777
            try:
                foreign.chmod(original_mode ^ 0o200)
                with self.assertRaisesRegex(AssertionError,
                                            "original isolated profile changed"):
                    check(env, project, before, "peer chmod")
            finally:
                foreign.chmod(original_mode)
            check(env, project, before, "peer mode restored")

            # A timestamp-only touch is a foreign change even with identical
            # bytes, inode and permissions.
            original_stat = foreign.stat()
            try:
                os.utime(foreign, ns=(original_stat.st_atime_ns,
                                      original_stat.st_mtime_ns - 2_000_000_000))
                with self.assertRaisesRegex(AssertionError,
                                            "original isolated profile changed"):
                    check(env, project, before, "peer timestamp changed")
            finally:
                os.utime(foreign, ns=(original_stat.st_atime_ns,
                                      original_stat.st_mtime_ns))
            check(env, project, before, "peer timestamp restored")

            # An atomically replaced file with identical bytes is not the
            # original foreign file, even when its permissions are equal.
            saved = root / "saved-foreign-native"
            foreign.replace(saved)
            try:
                foreign.write_bytes(b"same bytes")
                with self.assertRaisesRegex(AssertionError,
                                            "original isolated profile changed"):
                    check(env, project, before, "peer replacement")
            finally:
                foreign.unlink(missing_ok=True)
                saved.replace(foreign)
            check(env, project, before, "peer identity restored")

    def test_marketing_kit_snapshot_rejects_same_mode_directory_replacement(self):
        # The complete protected tree can look identical after swapping an
        # empty directory: bytes, names and permissions alone do not prove
        # foreign-state isolation. Check the profile root and a nested root.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        for nested in (False, True):
            with self.subTest(nested=nested), TemporaryDirectory() as temporary:
                base = Path(temporary)
                protected = base / "protected"
                protected.mkdir()
                victim = protected / "empty" if nested else protected
                if nested:
                    victim.mkdir()
                before = runner["scoped_snapshot"]([protected])
                saved = base / "saved-original"
                victim.rename(saved)
                try:
                    victim.mkdir(mode=saved.stat().st_mode & 0o7777)
                    self.assertNotEqual(runner["scoped_snapshot"]([protected]), before)
                finally:
                    victim.rmdir()
                    saved.rename(victim)
                self.assertEqual(runner["scoped_snapshot"]([protected]), before)

    def test_marketing_kit_native_uninstall_checks_peer_on_success_and_failure(self):
        # The audited native Codex uninstall runs outside the UAP CLI wrapper.
        # Both success and error paths must detect mutations in a peer HOME.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            env, project, *_ = runner["run_case"]("unused", "codex", root, "peer")
            before = runner["profile_snapshot"](env, project)
            marker = Path(env["HOME"]) / ".codex" / "unexpected-native-write.json"

            def verify_peer():
                runner["check_profile_unchanged"](env, project, before,
                                                   "native Codex uninstall")

            for must_fail in (False, True):
                def native_command():
                    marker.write_text("unexpected cross-profile write", encoding="utf-8")
                    if must_fail:
                        raise RuntimeError("native CLI failed")
                    return ("--external-uninstalled",)

                try:
                    with self.assertRaisesRegex(AssertionError,
                                                "original isolated profile changed"):
                        runner["guarded_native_attestation"](native_command,
                                                              (verify_peer,))
                finally:
                    marker.unlink(missing_ok=True)
            self.assertEqual(
                runner["guarded_native_attestation"](
                    lambda: ("--external-uninstalled",), (verify_peer,)),
                ("--external-uninstalled",))

            def failed_without_mutation():
                raise RuntimeError("native CLI failed")

            with self.assertRaisesRegex(RuntimeError, "native CLI failed"):
                runner["guarded_native_attestation"](failed_without_mutation,
                                                      (verify_peer,))

    def test_marketing_kit_removed_user_c_profile_is_not_mutated_by_user_a(self):
        # The final User A remove happens *after* User C has been removed.
        # It must not recreate a file in C's HOME or durable UAP data.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            env_c, project_c, *_ = runner["run_case"]("unused", "codex", root, "C")
            before = runner["profile_snapshot"](env_c, project_c)
            check = runner["check_profile_unchanged"]
            check(env_c, project_c, before, "User A remove")
            for path in (Path(env_c["HOME"]) / ".codex" / "unexpected-peer.json",
                         Path(env_c["AGENTPLUGINS_HOME"]) / "unexpected-receipt.json"):
                path.write_text("cross-profile mutation", encoding="utf-8")
                with self.assertRaisesRegex(AssertionError,
                                            "original isolated profile changed"):
                    check(env_c, project_c, before, "User A remove")
                path.unlink()
                check(env_c, project_c, before, "User A remove")

    def test_marketing_kit_snapshot_rejects_symlink_retarget_without_following(self):
        # The shared file/dir snapshot records only 'symlink', which cannot
        # distinguish a link retargeted to a different foreign profile.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            project = root / "project"
            left, right = root / "left", root / "right"
            for directory in (project, left, right):
                directory.mkdir()
            link = project / "external-native-profile"
            try:
                link.symlink_to(left, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            before = runner["scoped_snapshot"]([project])
            # Preserve the original inode while probing both a retarget and
            # a recreated link to the SAME target; neither is foreign-state
            # preservation even when the final target string is unchanged.
            saved = root / "saved-original-native-link"
            link.rename(saved)
            try:
                link.symlink_to(right, target_is_directory=True)
                self.assertNotEqual(runner["scoped_snapshot"]([project]), before)
                link.unlink()
                link.symlink_to(left, target_is_directory=True)
                self.assertNotEqual(runner["scoped_snapshot"]([project]), before)
            finally:
                link.unlink(missing_ok=True)
                saved.rename(link)
            self.assertEqual(runner["scoped_snapshot"]([project]), before)

    def test_marketing_kit_snapshot_rejects_recreated_root_symlink(self):
        # A protected symlink root must retain its inode, not just its target.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            base = Path(temporary)
            target = base / "actual-profile"
            target.mkdir()
            link = base / "protected-profile"
            try:
                link.symlink_to(target, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            before = runner["scoped_snapshot"]([link])
            saved = base / "saved-original-link"
            link.rename(saved)
            try:
                link.symlink_to(target, target_is_directory=True)
                self.assertNotEqual(runner["scoped_snapshot"]([link]), before)
            finally:
                link.unlink(missing_ok=True)
                saved.rename(link)
            self.assertEqual(runner["scoped_snapshot"]([link]), before)

    def test_marketing_kit_user_scope_cannot_write_untracked_project_state(self):
        # A User command may succeed while creating a new file in the current
        # Project, leaving its signed lock, manifest and sentinels unchanged.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_user_project_unchanged"]
        snapshot = runner["scoped_snapshot"]
        with TemporaryDirectory() as temporary:
            project = Path(temporary) / "project"
            project.mkdir()
            (project / "uap.toml").write_text("original", encoding="utf-8")
            foreign = project / "other-tool.txt"
            foreign.write_text("owned by another tool", encoding="utf-8")
            baseline = snapshot([project])
            check(project, baseline, "User add")

            untracked = project / "unexpected-user-scope-output.json"
            untracked.write_text("unexpected", encoding="utf-8")
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, baseline, "User add")
            untracked.unlink()

            original_stat = foreign.stat()
            foreign.write_text("overwritten by User scope", encoding="utf-8")
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, baseline, "User remove")
            foreign.write_text("owned by another tool", encoding="utf-8")
            os.utime(foreign, ns=(original_stat.st_atime_ns,
                                   original_stat.st_mtime_ns))
            check(project, baseline, "User remove")

            empty = project / "unexpected-empty-user-dir"
            empty.mkdir()
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, baseline, "User remove")
            empty.rmdir()
            check(project, baseline, "User remove")

    def test_marketing_kit_snapshot_rejects_removed_or_replaced_empty_root(self):
        # A snapshot with no child files previously equated an empty protected
        # Project with a missing root or a regular file. These are mutations.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        snapshot = runner["scoped_snapshot"]
        check = runner["check_user_project_unchanged"]
        with TemporaryDirectory() as temporary:
            project = Path(temporary) / "empty-project"
            project.mkdir()
            baseline = snapshot([project])
            check(project, baseline, "User add")
            project.rmdir()
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, baseline, "User remove")

            missing = snapshot([project])
            project.mkdir()
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, missing, "User add")
            project.rmdir()
            project.write_bytes(b"replaced with a regular file")
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, baseline, "User repair")

    def test_marketing_kit_user_project_receipt_ids_are_scope_isolated(self):
        # Fresh User A and cold User C must not inherit receipt identities
        # from removed Project A or B, even with the same signed closure.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_scope_id_isolation"]
        project_a = {"root": "project-a", "child": "project-a-child"}
        project_b = {"root": "project-b", "child": "project-b-child"}
        user = {"root": "user-a", "child": "user-a-child"}
        check(user, (project_a, project_b), "User add")
        for copied in (project_a["root"], project_b["child"]):
            with self.assertRaisesRegex(AssertionError,
                                        "reused installation IDs across scopes"):
                check({"root": copied, "child": "fresh-user"},
                      (project_a, project_b), "User cold restore")

    def test_marketing_kit_cold_restore_guard_requires_full_disjoint_lock(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_cold_restore"]
        original = {dist: f"A-{i}"
                    for i, dist in enumerate(sorted(runner["EXPECTED"]))}
        restored = {dist: f"B-{i}"
                    for i, dist in enumerate(sorted(runner["EXPECTED"]))}
        check(restored, original, b"pinned", b"pinned",
              b"portable", b"portable", "User B")
        incomplete = dict(restored)
        incomplete.pop(next(iter(incomplete)))
        with self.assertRaisesRegex(AssertionError, "mandatory signed closure"):
            check(incomplete, original, b"pinned", b"pinned",
                  b"portable", b"portable", "User B")
        reused = {**restored, next(iter(restored)): next(iter(original.values()))}
        with self.assertRaisesRegex(AssertionError, "reused installation IDs"):
            check(reused, original, b"pinned", b"pinned",
                  b"portable", b"portable", "User B")
        with self.assertRaisesRegex(AssertionError, "modified pinned lock"):
            check(restored, original, b"pinned", b"tampered",
                  b"portable", b"portable", "User B")
        with self.assertRaisesRegex(AssertionError, "modified portable manifest"):
            check(restored, original, b"pinned", b"pinned",
                  b"portable", b"tampered", "User B")

    def test_marketing_kit_lock_guard_rejects_duplicate_or_unidentified_pins(self):
        # Comparing lock keys as a set can hide a duplicate signed package.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            project = Path(temporary)
            def check(packages):
                (project / "uap.lock").write_text(
                    json.dumps({"packages": packages}), encoding="utf-8")
                return runner["lock_ids"](project)

            root = {"source_key": "directory:777genius/marketing-kit"}
            child = {"source_key": "directory:777genius/notion"}
            self.assertEqual(check([root, child]),
                             {root["source_key"], child["source_key"]})
            with self.assertRaisesRegex(AssertionError, "duplicate signed lock"):
                check([root, root])
            for invalid in (None, "", 123):
                with self.assertRaisesRegex(AssertionError, "invalid signed lock"):
                    check([{"source_key": invalid}])
            with self.assertRaisesRegex(AssertionError, "invalid signed lock"):
                check([{"other": "directory:777genius/notion"}])

    def test_published_windows_runner_scratch_is_real_absolute_and_ci_only(self):
        audit = load_audit()
        with TemporaryDirectory() as scratch:
            with patch.object(audit.sys, "platform", "win32"):
                with patch.dict(audit.os.environ, {
                        "GITHUB_ACTIONS": "true", "RUNNER_TEMP": scratch}):
                    self.assertEqual(audit.published_audit_temp_parent(), scratch)
                    with patch.object(audit.Path, "is_symlink", return_value=True):
                        with self.assertRaisesRegex(RuntimeError, "redirected"):
                            audit.published_audit_temp_parent()
                with patch.dict(audit.os.environ, {
                        "GITHUB_ACTIONS": "false", "RUNNER_TEMP": scratch}):
                    self.assertIsNone(audit.published_audit_temp_parent())
                with patch.dict(audit.os.environ, {
                        "GITHUB_ACTIONS": "true", "RUNNER_TEMP": "relative/scratch"}):
                    with self.assertRaisesRegex(RuntimeError, "redirected"):
                        audit.published_audit_temp_parent()
            with patch.object(audit.sys, "platform", "darwin"):
                with patch.dict(audit.os.environ, {
                        "GITHUB_ACTIONS": "true", "RUNNER_TEMP": scratch}):
                    self.assertEqual(audit.published_audit_temp_parent(),
                                     str(Path(audit.tempfile.gettempdir()).resolve(strict=True)))

    def test_published_macos_physical_parent_precedes_allocation_without_relaxing_guards(self):
        audit = load_audit()
        item = {"name": "owned-test", "repo": "example/skills", "folder": "owned-test",
                "source": "unused", "skill_md_git_blob_sha1": "0" * 40}
        with TemporaryDirectory() as temporary:
            base = Path(temporary).resolve(strict=True)
            physical = base / 'private-var/folders/session/tmp'
            physical.mkdir(parents=True)
            alias = base / 'var'
            alias.symlink_to(base / 'private-var', target_is_directory=True)
            spelled = alias / 'folders/session/tmp'
            exchange_globals = audit.CODEX_RUNTIME_AUDIT['CodexRuntimeAudit'].__init__.__globals__
            native_boundary = unittest.mock.Mock(side_effect=RuntimeError('inert native boundary'))
            with (patch.dict(audit.AGENT_BINARIES, {'codex': '/inert/codex'}, clear=True),
                  patch.dict(exchange_globals, {'_exchange': native_boundary}),
                  patch.object(audit, 'call', side_effect=AssertionError('public install must not run')) as public):
                # The old allocation spells an ancestor through the alias, just
                # like /var/folders/... through macOS /var -> /private/var.
                with TemporaryDirectory(dir=spelled) as old_root:
                    with self.assertRaisesRegex(ValueError, 'Codex audit directory is redirected'):
                        audit.do_case('/inert/uap', 'codex', 'skill', item, 0, Path(old_root), {})
                native_boundary.assert_not_called()
                with (patch.object(audit.sys, 'platform', 'darwin'),
                      patch.object(audit.tempfile, 'gettempdir', return_value=str(spelled))):
                    parent = audit.published_audit_temp_parent()
                self.assertEqual(parent, str(physical))
                with TemporaryDirectory(dir=parent) as new_root:
                    with self.assertRaisesRegex(RuntimeError, 'inert native boundary'):
                        audit.do_case('/inert/uap', 'codex', 'skill', item, 0, Path(new_root), {})
                native_boundary.assert_called_once()
                public.assert_not_called()
            # Path guards themselves still reject a redirected directory.
            with self.assertRaisesRegex(ValueError, 'Codex audit directory is redirected'):
                exchange_globals['_real_directory'](spelled)

    def test_bootstrap_refusal_preserves_index_source_and_real_identity(self):
        # BUG-053: the original blocked ZIPs had 30 status placeholders but
        # omitted their source and index, so fixture slots could not be audited.
        audit = load_audit()
        source = "github:sample/agent-plugin@" + ("a" * 40)
        rows = audit.unexecuted_source_rows(
            [source, "atlassian"], "plugin", "grok", "client-unavailable",
            {source: "sample-agent-plugin"})
        self.assertEqual([v["index"] for v in rows], [0, 1])
        self.assertEqual([v["source"] for v in rows], [source, "atlassian"])
        self.assertEqual([v["identity"] for v in rows],
                         ["sample-agent-plugin", "atlassian"])
        self.assertTrue(all(v["status"] == "client-unavailable"
                            and "install" not in v for v in rows))
        skill = {"name": "pdf", "source": "github:sample/skills@" + ("b" * 40)}
        blocked = audit.unexecuted_source_rows(
            [skill], "skill", "chatgpt", "documented-unsupported", {})
        self.assertEqual(blocked[0]["source"], skill["source"])
        self.assertEqual(blocked[0]["index"], 0)
        self.assertEqual(blocked[0]["identity"], "pdf")
        with self.assertRaisesRegex(ValueError, "non-execution"):
            audit.unexecuted_source_rows([], "plugin", "grok", "passed", {})

    def test_cline_updates_disabled_in_isolated_child_despite_host_override(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home, data = case / "home", case / "data"
            home.mkdir()
            data.mkdir()
            for inherited in ("0", "", "1"):
                with self.subTest(inherited=inherited), patch.dict(
                        os.environ, {"CLINE_NO_AUTO_UPDATE": inherited}):
                    env = audit.isolated_client_env(home, case, data)
                    self.assertEqual(env["CLINE_NO_AUTO_UPDATE"], "1")
                    child = subprocess.run(
                        [sys.executable, "-c", "import os; print(os.environ['CLINE_NO_AUTO_UPDATE'])"],
                        env=env, cwd=case, capture_output=True, text=True, check=True)
                    self.assertEqual(child.stdout.strip(), "1")

    def test_live_workflow_cline_pin_rejects_drift_and_unobserved_version(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-audit.yml").read_text()
        bootstrap = workflow.split("        id: native-install\n", 1)[1].split("      - name:", 1)[0]
        self.assertIn("CLINE_NO_AUTO_UPDATE: '1'", bootstrap)
        self.assertIn('[[ "$native_version" == 3.0.74 ]]', bootstrap)
        validator = textwrap.dedent(workflow.split("<<'CLINE_PIN'\n", 1)[1]
                                    .split("          CLINE_PIN", 1)[0])
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            proof = root / "full-proof"
            proof.mkdir()
            for value in ({"status": "reported", "value": "3.0.74", "exit_code": 0},
                          {"status": "reported", "value": "3.0.75", "exit_code": 0},
                          {"status": "not-probed"},
                          {"status": "reported", "value": "3.0.74", "exit_code": False}):
                with self.subTest(version=value):
                    (proof / "cline-plugin.json").write_text(json.dumps({"client_version": value}))
                    child = subprocess.run([sys.executable, "-c", validator], capture_output=True,
                                           env={**os.environ, "RUNNER_TEMP": str(root), "KIND": "plugin"})
                    self.assertEqual(child.returncode == 0, value.get("value") == "3.0.74"
                                     and type(value.get("exit_code")) is int)

    def test_rebinds_all_client_overrides_and_strips_git_injection(self):
        audit = load_audit()
        hostile = {key: "/outside/actual-user-profile" for key in audit.PROFILE_ENV_KEYS}
        hostile.update({
            "HOME": "/outside/actual-user-home",
            "USERPROFILE": "/outside/actual-user-home",
            "XDG_CONFIG_HOME": "/outside/config",
            "XDG_DATA_HOME": "/outside/data",
            "APPDATA": "/outside/real/windows-roaming",
            "LOCALAPPDATA": "/outside/real/windows-local",
            "GIT_CONFIG_COUNT": "1",
            "GIT_CONFIG_KEY_0": "core.hooksPath",
            "GIT_CONFIG_VALUE_0": "/outside/hooks",
            "GIT_SSH_COMMAND": "/outside/host-ssh",
            "SSH_ASKPASS": "/outside/host-askpass",
        })
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, data = case / "home", case / "uap-data"
            home.mkdir(parents=True)
            data.mkdir()
            with patch.dict(os.environ, hostile):
                env = audit.isolated_client_env(home, case, data)
            for key in audit.PROFILE_ENV_KEYS:
                if key in env:
                    self.assertTrue(Path(env[key]).resolve().is_relative_to(home.resolve()), (key, env[key]))
                else:
                    self.assertNotIn(key, env)
            for key in ("HOME", "USERPROFILE", "XDG_CONFIG_HOME", "XDG_DATA_HOME",
                        "XDG_CACHE_HOME", "XDG_STATE_HOME", "GIT_CONFIG_GLOBAL",
                        "APPDATA", "LOCALAPPDATA", "AGENTPLUGINS_HOME", "TMPDIR"):
                self.assertTrue(Path(env[key]).resolve().is_relative_to(case.resolve()), (key, env[key]))
            self.assertEqual(env["GEMINI_CLI_HOME"], str(home))
            for key in ("GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0",
                        "GIT_SSH_COMMAND", "SSH_ASKPASS"):
                self.assertNotIn(key, env)

    def test_drops_unrelated_credentials_but_retains_public_runtime_plumbing(self):
        audit = load_audit()
        trusted = {
            "PATH": "/usr/local/bin:/usr/bin:/bin",
            "HTTPS_PROXY": "http://proxy.invalid:3128",
            "LANG": "C.UTF-8",
        }
        canaries = {
            "OPENAI_API_KEY": "never-forward-openai-canary",
            "ANTHROPIC_API_KEY": "never-forward-anthropic-canary",
            "GH_TOKEN": "never-forward-gh-canary",
            "GITHUB_TOKEN": "never-forward-github-canary",
            "ACTIONS_RUNTIME_TOKEN": "never-forward-actions-canary",
            "AWS_SECRET_ACCESS_KEY": "never-forward-aws-canary",
            "NPM_TOKEN": "never-forward-npm-canary",
            "SSH_AUTH_SOCK": "/outside/ssh-agent.sock",
            "GIT_CONFIG_COUNT": "1",
            "GIT_CONFIG_KEY_0": "core.hooksPath",
            "GIT_CONFIG_VALUE_0": "/outside/hooks",
            "CODEX_HOME": "/outside/real-codex-profile",
        }
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, data = case / "home", case / "uap-data"
            home.mkdir(parents=True)
            data.mkdir()
            with patch.dict(os.environ, {**trusted, **canaries}, clear=True):
                env = audit.isolated_client_env(home, case, data)
        for key, value in trusted.items():
            self.assertEqual(env[key], value)
        for key in canaries:
            if key != "CODEX_HOME":
                self.assertNotIn(key, env)
        self.assertTrue(Path(env["CODEX_HOME"]).is_relative_to(home))
        self.assertEqual(env["CI"], "true")

    def test_grok_uses_disposable_default_home_without_custom_override(self):
        # BUG-057: even a HOME/.grok GROK_HOME value makes the genuine Grok
        # adapter select grok_custom_home and refuse normal project Skills.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, data = case / "home", case / "uap-data"
            home.mkdir(parents=True)
            data.mkdir()
            with patch.dict(os.environ, {
                "GROK_HOME": "/outside/real-grok-profile",
                "HOME": "/outside/real-home",
                "XDG_CONFIG_HOME": "/outside/real-config",
            }):
                env = audit.isolated_client_env(home, case, data)
            self.assertNotIn("GROK_HOME", env)
            self.assertEqual(env["HOME"], str(home))
            self.assertEqual(env["XDG_CONFIG_HOME"], str(home / ".config"))
            self.assertTrue((Path(env["HOME"]) / ".grok").is_relative_to(case))
            self.assertEqual(list(home.iterdir()), [])

    def test_version_evidence_is_short_and_never_claims_failed_probe(self):
        audit = load_audit()
        with patch.object(audit, "call", return_value={
            "stdout_tail": "codex-cli 0.153.4\n" + ("A" * 500),
            "stderr_tail": "", "exit_code": 0, "timeout": False,
        }) as run:
            reported = audit.version_evidence(["codex", "--version"], {"HOME": "/isolated"}, "/tmp")
            self.assertEqual(reported["status"], "reported")
            self.assertLessEqual(len(reported["value"]), 200)
            self.assertEqual(run.call_args.args[1], {"HOME": "/isolated"})
        for exit_code, stdout, timeout, status in [
            (2, "version 1", False, "probe-failed"),
            (0, "", False, "empty-output"),
            (None, "", True, "timeout"),
        ]:
            with patch.object(audit, "call", return_value={
                "stdout_tail": stdout, "stderr_tail": "",
                "exit_code": exit_code, "timeout": timeout,
            }):
                self.assertEqual(audit.version_evidence(["client", "--version"], {}, "/tmp")["status"], status)

    def test_refuses_case_data_outside_disposable_root(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            case, home, outside = parent / "case", parent / "case/home", parent / "outside"
            home.mkdir(parents=True)
            outside.mkdir()
            with self.assertRaisesRegex(ValueError, "disposable case"):
                audit.isolated_client_env(home, case, outside)

    def test_success_diagnostics_and_timeout_are_never_source_refusals(self):
        audit = load_audit()
        for exit_code in (0, None):
            self.assertFalse(audit.is_cli_refusal({
                "exit_code": exit_code,
                "stdout_tail": '{"result":"success","component_support":"unsupported"}',
                "stderr_tail": "target_unavailable",
            }))
        self.assertTrue(audit.is_cli_refusal({
            "exit_code": 1,
            "stderr_tail": "release does not support complete target set",
        }))

    def test_kiro_darwin_containment_refusal_remains_nonpass(self):
        # BUG-048: a real Kiro 3.0.0 macOS CLI returns this exact
        # fail-closed diagnostic before any native profile mutation.
        audit = load_audit()
        darwin = ("manual_activation_required: automatic native Kiro MCP lifecycle "
                  "containment preflight failed: safe duplex process containment "
                  "unavailable on Darwin; manual activation required")
        linux = "manual_activation_required: cgroup v2 delegation unavailable"
        for diagnostic in (darwin, linux):
            with self.subTest(diagnostic=diagnostic):
                self.assertTrue(audit.host_containment_refusal(diagnostic))
                refused = {"exit_code": 1, "stdout_tail": "",
                           "stderr_tail": diagnostic, "timeout": False,
                           "_json": {"data": {"installations": []}}}
                with TemporaryDirectory() as temporary:
                    with patch.object(audit, "call", return_value=refused):
                        evidence = audit.do_case(
                            "/unused/audit-cli", "kiro", "plugin", "atlassian",
                            0, Path(temporary).resolve(), {})
                self.assertEqual(evidence["status"], "host-containment-unavailable")
                self.assertFalse(evidence["listed_identity"])
                self.assertFalse(evidence["failed_install_residue"])
        for diagnostic in (
            "manual_activation_required: native Kiro registry invalid",
            "safe duplex process containment unavailable on Darwin",
            "native process failed; unexpected exception",
        ):
            self.assertFalse(audit.host_containment_refusal(diagnostic))
        # The same words in an otherwise successful response must not
        # qualify a refusal. The CLI's nonzero exit remains mandatory.
        self.assertFalse(audit.is_cli_refusal({
            "exit_code": 0, "stderr_tail": darwin, "stdout_tail": "",
            "timeout": False}))

    def test_chatgpt_identity_ownership_refusal_requires_nonzero_exit_and_no_mutation(self):
        audit = load_audit()
        ownership_error = (
            "agentplugins: group apply preflight failed; no target was changed "
            "(selected targets: [chatgpt]): target chatgpt identity preflight: "
            "native identity ownership is indeterminate; refusing mutation")
        empty = {"exit_code": 0,
                 "stdout_tail": '{"schema_version":1,"command":"list",'
                                '"result":"success","data":{"installations":[]}}',
                 "stderr_tail": "", "timeout": False,
                 "_json": {"data": {"installations": []}}}
        cases = [
            ("chatgpt", 1, ownership_error, "", "native-ownership-unverified"),
            ("codex", 1, ownership_error, "", "install-failed"),
            ("chatgpt", 0, ownership_error, "", "incomplete-native-proof"),
            ("chatgpt", 1, "generic failure", ownership_error, "install-failed"),
            ("chatgpt", 1, ownership_error,
             '{"components":[{"support":"unsupported"}]}',
             "documented-or-source-unsupported"),
        ]
        for target, exit_code, stderr, stdout, expected in cases:
            with self.subTest(target=target, exit_code=exit_code, expected=expected):
                refused = {"exit_code": exit_code, "stdout_tail": stdout,
                           "stderr_tail": stderr, "timeout": False, "_json": None}
                def source_call(argv, *_args, **_kwargs):
                    return dict(empty if len(argv) > 1 and argv[1] == "list" else refused)
                with TemporaryDirectory() as temporary:
                    with patch.dict(audit.AGENT_BINARIES, {}, clear=True), \
                         patch.object(audit, "call", side_effect=source_call):
                        result = audit.do_case(
                            "/unused/audit-cli", target, "plugin", "atlassian",
                            0, Path(temporary).resolve(), {})
                self.assertEqual(result["status"], expected)
                self.assertFalse(result["failed_install_registered"])
                self.assertFalse(result["failed_install_residue"])
                self.assertTrue(result["sentinels_preserved"])

    def test_every_native_plugin_client_preserves_foreign_profile_file(self):
        audit = load_audit()
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                   "timeout": False, "_json": None}
        with TemporaryDirectory() as temporary:
            with patch.object(audit, "call", return_value=refused):
                for target, roots in audit.NATIVE_ROOTS.items():
                    if not roots:
                        continue  # Public aliases have a separate canonical backend control.
                    with self.subTest(target=target):
                        runroot = Path(temporary).resolve() / target
                        evidence = audit.do_case(
                            "/unused/audit-cli", target, "plugin", "atlassian", 0,
                            runroot, {}
                        )
                        foreign = runroot / "plugin-00/home" / roots[0] / "foreign-neighbor.txt"
                        self.assertTrue(foreign.is_file())
                        self.assertEqual(foreign.read_text(),
                                         "unmanaged native neighbor must survive\n")
                        self.assertTrue(evidence["sentinels_preserved"])


    def test_relocated_skill_payload_never_qualifies_as_removed(self):
        # BUG-027: dropping SKILL.md and moving a payload used to evade
        # both the identity scan and the known installation path check.
        import shutil
        audit = load_audit()
        item = {"name": "demo-skill", "source": "https://github.com/example/repo",
                "repo": "example/repo", "folder": "demo-skill",
                "skill_md_git_blob_sha1": "a" * 40}
        installed = [{"name": "demo-skill", "installation_id": "owned-install",
                      "clients": [{"client_id": "claude"}]}]
        def success(rows=None):
            return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                    "timeout": False,
                    "_json": {"data": {"installations": rows or []}}}

        for relocate in (True, False):
            with self.subTest(relocate=relocate), TemporaryDirectory() as tmp:
                state = {"added": 0, "removed": False}
                project = Path(tmp) / "skill-00/project"

                def fake_call(argv, env, cwd, timeout=105):
                    operation = argv[1]
                    skill_dir = project / ".claude/skills/demo-skill"
                    if operation == "skills":
                        if state["added"] == 0:
                            (skill_dir / "scripts").mkdir(parents=True)
                            (skill_dir / "SKILL.md").write_text("name: demo-skill\n")
                            (skill_dir / "scripts/payload.sh").write_text("echo payload\n")
                            (project / "uap.toml").write_text("demo-skill")
                            (project / "uap.lock").write_text("demo-skill")
                        state["added"] += 1
                        result = success()
                        result["_json"].update(readiness="ready", next_actions=[])
                        return result
                    elif operation == "remove":
                        if relocate:
                            moved = skill_dir.with_name("other")
                            skill_dir.rename(moved)
                            (moved / "SKILL.md").unlink()
                        else:
                            shutil.rmtree(skill_dir)
                        (project / "uap.toml").unlink()
                        (project / "uap.lock").unlink()
                        state["removed"] = True
                    elif operation == "list":
                        return success([] if state["removed"] else installed)
                    else:
                        raise AssertionError(argv)
                    return success()

                with patch.object(audit, "call", side_effect=fake_call), \
                     patch.object(audit, "matching_installations", return_value=installed), \
                     patch.object(audit, "locked_skill_is_pinned", return_value=True), \
                     patch.object(audit, "matches_pinned_skill_bytes", return_value=True):
                    evidence = audit.do_case(
                        "/unused/agentplugins", "claude", "skill", item, 0,
                        Path(tmp), {})

                self.assertTrue(evidence["sentinels_preserved"])
                self.assertIsNone(evidence["post_removal_skill"])
                self.assertEqual(evidence["post_removal_leftovers"], [])
                if relocate:
                    self.assertIn("skills/other/scripts/payload.sh",
                                  evidence["post_skill_tree_delta"])
                    self.assertFalse(evidence["removal_verified"])
                    self.assertEqual(evidence["status"], "cleanup-failed")
                else:
                    self.assertEqual(evidence["post_skill_tree_delta"], [])
                    self.assertTrue(evidence["removal_verified"])
                    self.assertEqual(evidence["status"], "passed")

    def test_codex_remove_attests_only_pinned_absolute_executable(self):
        # BUG-028: an untrusted package must not shadow codex on PATH.
        audit = load_audit()
        with TemporaryDirectory() as tmp:
            case = Path(tmp).resolve() / "case"
            home = case / "home"
            conf = home / ".codex/config.toml"
            conf.parent.mkdir(parents=True)
            market_dir = case / "managed-marketplace"
            market_dir.mkdir(parents=True)
            market = "agentplugins-012345abcdef"
            conf.write_text(
                '[plugins."demo@' + market + '"]\nenabled = true\n'
                '[marketplaces.' + market + ']\nsource_type = "local"\n'
                'source = "' + str(market_dir) + '"\n')
            commands = []
            def fake_call(argv, env, cwd, timeout=40):
                commands.append(argv)
                if len(commands) == 2:
                    conf.write_text("")
                return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                        "timeout": False, "_json": None}
            pinned = case / "trusted-codex"
            pinned.write_text("test executable")
            with patch.dict(audit.AGENT_BINARIES, {"codex": str(pinned)}, clear=True), \
                 patch.object(audit, "call", side_effect=fake_call):
                result = audit.uninstall_native_codex_plugin(
                    "demo", home, case, {"HOME": str(home)}, case)
            self.assertTrue(result["completed"])
            self.assertEqual(len(commands), 2)
            self.assertTrue(all(cmd[0] == str(pinned) for cmd in commands))
            with patch.dict(audit.AGENT_BINARIES, {}, clear=True), \
                 patch.object(audit, "call") as forbidden:
                refused = audit.uninstall_native_codex_plugin(
                    "demo", home, case, {}, case)
                forbidden.assert_not_called()
            self.assertFalse(refused["completed"])
            self.assertIn("pinned", refused["reason"])

    def test_chatgpt_alias_requires_registration_and_verified_cleanup(self):
        # BUG-013: a zero-exit CLI can still leave no managed installation.
        audit = load_audit()
        source = "github:example/chatgpt-demo@" + ("a" * 40)
        identity = "chatgpt-demo"
        matched = [{"name": identity, "installation_id": "isolated-owned-install",
                    "clients": [{"client_id": "codex",
                                 "verification": "installation_verified"}]}]

        def response(exit_code, installations=None):
            return {"exit_code": exit_code, "stdout_tail": "", "stderr_tail": "",
                    "timeout": False,
                    "_json": {"data": {"installations": installations or []}}}

        def run_case(outputs, matches):
            with TemporaryDirectory() as temporary:
                with patch.dict(audit.AGENT_BINARIES, {}, clear=True), \
                     patch.object(audit, "call", side_effect=outputs) as calls, \
                     patch.object(audit, "matching_installations", return_value=matches) as matching:
                    result = audit.do_case(
                        "/unused/agentplugins", "chatgpt", "plugin", source, 0,
                        Path(temporary).resolve(), {source: identity})
                self.assertEqual(result["canonical_client_id"], "codex")
                self.assertGreater(matching.call_count, 0)
                self.assertTrue(all(call.args[2] == "codex" for call in matching.call_args_list))
                for call in calls.call_args_list:
                    argv = call.args[0]
                    if "--target" in argv:
                        self.assertEqual(argv[argv.index("--target") + 1], "chatgpt")
                return result

        empty = run_case([response(0), response(0)], [])
        self.assertEqual(empty["status"], "incomplete-native-proof")
        self.assertFalse(empty["listed_identity"])
        self.assertIsNone(empty["cleanup"])

        # Initial add/list, repeat add/list, then failed remove.
        failed_remove = run_case(
            [response(0), response(0), response(0), response(0), response(1)], matched)
        self.assertEqual(failed_remove["status"], "cleanup-failed")
        self.assertFalse(failed_remove["removal_verified"])

        # Full lifecycle may qualify only with an exact, successful repeat.
        failed_repeat = run_case(
            [response(0), response(0), response(1), response(0), response(0), response(0)],
            matched)
        self.assertEqual(failed_repeat["status"], "repeat-add-unqualified")
        self.assertFalse(failed_repeat["repeat_verified"])
        self.assertTrue(failed_repeat["removal_verified"])

        prepared = run_case(
            [response(0), response(0), response(0), response(0), response(0), response(0)],
            matched)
        self.assertEqual(prepared["status"], "package-installed-native-unqualified")
        self.assertTrue(prepared["repeat_verified"])
        self.assertTrue(prepared["repeat_native_unchanged"])
        self.assertTrue(prepared["repeat_lock_unchanged"])
        self.assertEqual(prepared["repeat_installation_ids"], ["isolated-owned-install"])
        self.assertTrue(prepared["listed_identity"])
        self.assertTrue(prepared["removal_verified"])
        self.assertTrue(prepared["sentinels_preserved"])

    def test_chatgpt_desktop_requires_actual_codex_bootstrap(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}), \
                    patch("sys.argv", ["live-install-audit.py", "--cli", "/unused/agentplugins",
                                       "--target", "chatgpt", "--kind", "plugin",
                                       "--limit", "1", "--report-dir", temporary]), \
                    patch.object(audit, "do_case") as cases, \
                    patch.object(audit, "version_evidence") as version:
                self.assertEqual(audit.main(), 2)
            cases.assert_not_called()
            version.assert_not_called()
            report = json.loads((Path(temporary) / "chatgpt-plugin.json").read_text())
        self.assertEqual(report["canonical_client_id"], "codex")
        self.assertEqual(report["cases_attempted"], 0)
        self.assertEqual([row["status"] for row in report["results"]], ["client-unavailable"])
        self.assertIn("codex", report["blocker"])

    def test_chatgpt_desktop_bootstrap_binds_the_shared_pin_and_regular_binary(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        vendor = workflow.split("        id: vendor-install\n", 1)[1].split("      - name:", 1)[0]
        self.assertIn("        continue-on-error: true\n", vendor)
        # The reviewed AppArmor helper executes as its own ChatGPT-only step
        # before vendor bootstrap, never inside the untrusted vendor-install step.
        sandbox = ('      - name: Supply verified bubblewrap for ChatGPT native discovery\n'
                   "        if: matrix.target == 'chatgpt' && matrix.kind == 'plugin'\n"
                   '        shell: bash\n'
                   '        run: bash scripts/prepare-codex-linux-sandbox.sh')
        self.assertIn(sandbox, workflow)
        self.assertLess(workflow.index(sandbox),
                        workflow.index('        id: vendor-install'))
        self.assertIn("exclude:\n          - target: chatgpt\n            kind: skill", workflow)
        block = textwrap.dedent(workflow.split("<<'CHATGPT_CODEX'\n", 1)[1]
                                .split("          CHATGPT_CODEX", 1)[0])
        pin = runpy.run_path(str(Path(__file__).with_name("run-native-client-matrix.py")))[
            "PINS"]["linux-amd64"]["codex"]
        for mode in ("regular", "symlink", "not-executable", "foreign-path"):
            with self.subTest(mode=mode), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                (root / "vendor-proof").mkdir()
                observed = []

                def provision(selected, directory, name):
                    self.assertIs(selected, pin)
                    self.assertEqual((directory, name), (root / "vendor-root", "codex"))
                    binary = directory / "codex"
                    if mode == "foreign-path":
                        binary = root / "foreign"
                    binary.write_bytes(b"inert test fixture, never executed")
                    binary.chmod(0o600 if mode == "not-executable" else 0o700)
                    if mode == "symlink":
                        target = root / "link-target"
                        binary.rename(target)
                        binary.symlink_to(target)
                    observed.append(binary)
                    return binary, {"source": "https://example.invalid/pinned-archive"}

                module = SimpleNamespace(
                    PINS={"linux-amd64": {"codex": pin}}, provision=provision)
                spec = SimpleNamespace(loader=SimpleNamespace(exec_module=lambda _module: None))
                with patch("sys.argv", ["bootstrap", str(root)]), \
                        patch("sys.platform", "linux"), patch("platform.machine", return_value="x86_64"), \
                        patch("importlib.util.spec_from_file_location", return_value=spec), \
                        patch("importlib.util.module_from_spec", return_value=module):
                    if mode == "regular":
                        exec(compile(block, "<desktop-bootstrap-contract>", "exec"), {})
                    else:
                        with self.assertRaisesRegex(SystemExit, "unverified staged Codex executable"):
                            exec(compile(block, "<desktop-bootstrap-contract>", "exec"), {})
                self.assertEqual(len(observed), 1)
                proof = root / "vendor-proof/codex-runtime.json"
                if mode == "regular":
                    value = json.loads(proof.read_text())
                    self.assertEqual((value["public_target"], value["canonical_client_id"],
                                      value["pin_family"], value["executable"]),
                                     ("chatgpt", "codex", "current-source", str(observed[0])))
                else:
                    self.assertFalse(proof.exists())

    def test_both_native_matrices_bind_real_bootstrap_outcomes(self):
        # BUG-014: present binary + failed native --version must never produce PASS.
        # This static guard joins the GitHub step outcome to the harness refusal.
        workflows = (
            (".github/workflows/live-install-audit.yml", "native-install"),
            (".github/workflows/live-install-audit.yml", "native-install-pilot"),
            (".github/workflows/live-install-audit.yml", "published-install"),
            (".github/workflows/live-install-remaining-clients.yml", "vendor-install"),
        )
        for relative_path, step_id in workflows:
            with self.subTest(workflow=relative_path):
                workflow = (Path(__file__).resolve().parents[1] / relative_path).read_text()
                self.assertIn(f"id: {step_id}", workflow)
                self.assertIn(
                    "AUDIT_VENDOR_OUTCOME: ${{ steps." + step_id + ".outcome }}",
                    workflow,
                )

    def test_harness_exception_records_exact_source_slot_without_network(self):
        # BUG-059: an exception must not erase the source/identity evidence.
        audit = load_audit()
        fixtures = json.loads(audit.FIXTURES.read_text())
        source = fixtures["plugin_aliases"][0]
        with TemporaryDirectory() as temporary:
            cli = Path(temporary) / "inert-cli"
            cli.write_text("no native execution")
            with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}), \
                    patch.dict(audit.AGENT_BINARIES, {"codex": str(cli)}, clear=True), \
                    patch("sys.argv", ["live-install-audit.py", "--cli", str(cli),
                                       "--target", "chatgpt", "--kind", "plugin",
                                       "--limit", "1", "--report-dir", temporary]), \
                    patch.dict(audit.AGENT_BINARIES, {"codex": str(cli)}, clear=True), \
                    patch.object(audit, "version_evidence", return_value={"status": "reported"}), \
                    patch.object(audit, "do_case", side_effect=ValueError("injected")):
                self.assertEqual(audit.main(), 1)
            report = json.loads((Path(temporary) / "chatgpt-plugin.json").read_text())
        self.assertEqual(report["cases_attempted"], 1)
        case = report["results"][0]
        self.assertEqual(case["status"], "harness-error")
        self.assertEqual(case["index"], 0)
        self.assertEqual(case["source"], source)
        self.assertEqual(case["identity"], fixtures["plugin_declared_names"].get(source, source))
        self.assertEqual((case["kind"], case["target"]), ("plugin", "chatgpt"))

    def test_harness_timeout_retains_bounded_chained_traceback_without_locals(self):
        audit = load_audit()
        for large_cause in (False, True):
            with self.subTest(large_cause=large_cause), TemporaryDirectory() as temporary:
                def fail(*args):
                    private_local = "do-not-capture-" + "local-value"
                    try:
                        raise ValueError("inner diagnostic cause" + ("x" * 16384 if large_cause else ""))
                    except ValueError as cause:
                        failure = TimeoutError()
                        failure.add_note("phase='before-install'; command=['/isolated/codex', 'app-server']")
                        raise failure from cause

                cli = Path(temporary) / "inert-cli"
                cli.write_text("no native execution")
                with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}), \
                        patch.dict(audit.AGENT_BINARIES, {"codex": str(cli)}, clear=True), \
                        patch("sys.argv", ["live-install-audit.py", "--cli", str(cli),
                                           "--target", "chatgpt", "--kind", "plugin",
                                           "--limit", "1", "--report-dir", temporary]), \
                        patch.dict(audit.AGENT_BINARIES, {"codex": sys.executable}, clear=True), \
                        patch.object(audit, "version_evidence", return_value={"status": "reported"}), \
                        patch.object(audit, "do_case", side_effect=fail) as run_case, \
                        patch("builtins.print") as printed:
                    self.assertEqual(audit.main(), 1)
                run_case.assert_called_once()
                report = json.loads((Path(temporary) / "chatgpt-plugin.json").read_text())
                self.assertEqual((report["cases_attempted"], report["passed"],
                                  report["verified_negative_coverage"]), (1, 0, 0))
                case = report["results"][0]
                self.assertEqual((case["status"], case["error"]), ("harness-error", "TimeoutError()"))
                detail = case.get("error_traceback", "")
                self.assertIn("TimeoutError", detail)
                self.assertIn("in fail", detail)
                self.assertIn("phase='before-install'", detail)
                self.assertIn("command=['/isolated/codex', 'app-server']", detail)
                self.assertNotIn("do-not-capture-local-value", detail)
                self.assertLessEqual(len(detail), 8192)
                if not large_cause:
                    self.assertIn("ValueError: inner diagnostic cause", detail)
                    self.assertIn("direct cause", detail)
                logged = json.loads(printed.call_args_list[0].args[0])
                self.assertEqual(logged.get("error_traceback"), detail)

    def test_reports_distinct_pr_head_and_checked_merge_sha(self):
        # BUG-015: pull_request GITHUB_SHA is a merge checkout, not PR head.
        import json
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            with patch.dict(os.environ, {
                "GITHUB_SHA": "a" * 40,
                "AUDIT_PR_HEAD_SHA": "b" * 40,
                "AUDIT_VENDOR_OUTCOME": "failure",
            }), patch("sys.argv", [
                "live-install-audit.py", "--cli", "/unused/agentplugins",
                "--target", "codex", "--kind", "skill", "--limit", "1",
                "--report-dir", temporary,
            ]):
                self.assertEqual(audit.main(), 2)
            report = json.loads((Path(temporary) / "codex-skill.json").read_text())
        self.assertEqual(report["github_sha"], "a" * 40)
        self.assertEqual(report["pr_head_sha"], "b" * 40)
        self.assertEqual(report["results"][0]["status"], "client-unavailable")

    def test_both_matrix_workflows_export_pr_source_head(self):
        for relative_path in (
            ".github/workflows/live-install-audit.yml",
            ".github/workflows/live-install-remaining-clients.yml",
        ):
            with self.subTest(workflow=relative_path):
                workflow = (Path(__file__).resolve().parents[1] / relative_path).read_text()
                self.assertIn("AUDIT_PR_HEAD_SHA: ${{ github.event.pull_request.head.sha || '' }}", workflow)

    def test_failed_vendor_bootstrap_cannot_inherit_unrelated_host_binary(self):
        audit = load_audit()
        with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "failure"}):
            self.assertFalse(audit.vendor_bootstrap_attested())
        with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}):
            self.assertTrue(audit.vendor_bootstrap_attested())
        with patch.dict(os.environ, {}, clear=True):
            self.assertFalse(audit.vendor_bootstrap_attested())
        with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": ""}):
            self.assertFalse(audit.vendor_bootstrap_attested())

    def test_native_launcher_refuses_unreviewed_path_shadow_without_execution(self):
        # BUG-058: matching the artifact digest does not authenticate an
        # unrelated executable that wins runner PATH resolution.
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        marker = 'python3 - "$BIN" "$executable" <<\'PY\'\n'
        self.assertIn(marker, workflow)
        source = textwrap.dedent(workflow.split(marker, 1)[1].split("\n          PY", 1)[0])
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            untrusted = root / "runner-tools"
            verified = root / "vendor"
            untrusted.mkdir()
            verified.mkdir()
            for path in (untrusted / "grok", verified / "grok"):
                path.write_text("#!/bin/sh\nexit 0\n")
                path.chmod(0o755)
            for first, allowed in ((untrusted, False), (verified, True)):
                second = verified if first == untrusted else untrusted
                env = dict(os.environ, PATH=f"{first}:{second}")
                result = subprocess.run(
                    [sys.executable, "-c", source, "grok", str(verified / "grok")],
                    env=env, capture_output=True, text=True, check=False)
                self.assertEqual(result.returncode == 0, allowed)
                if not allowed:
                    self.assertIn("unverified native PATH executable", result.stderr)

    def test_kiro_node_capture_rejects_unselected_runtime_without_executing_it(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        marker = "python3 - <<'KIRO_NODE'\n"
        source = textwrap.dedent(workflow.split(marker, 1)[1].split("\n          KIRO_NODE", 1)[0])
        variants = ("selected", "shadow", "version", "major", "architecture", "directory")
        if os.name != "nt":
            variants += ("not-executable", "symlink")
        for variant in variants:
            with self.subTest(variant=variant), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                cache, runtime = root / "cache", root / "run"
                runtime.mkdir()
                binary = cache / "node/24.22.2/arm64/bin/node"
                binary.parent.mkdir(parents=True)
                ran = root / "native-was-executed"
                body = f"#!{sys.executable}\nfrom pathlib import Path\nPath({str(ran)!r}).touch()\n"
                binary.write_text(body)
                binary.chmod(0o755)
                other = root / "unrelated/node"
                other.parent.mkdir()
                other.write_text(body)
                other.chmod(0o755)
                output = root / "output"
                output.touch()
                env = dict(os.environ, PATH=str(binary.parent), RUNNER_TEMP=str(runtime),
                           RUNNER_TOOL_CACHE=str(cache), RUNNER_ARCH="ARM64",
                           KIRO_NODE_VERSION="v24.22.2", GITHUB_OUTPUT=str(output))
                if variant == "shadow":
                    env["PATH"] = str(other.parent) + os.pathsep + env["PATH"]
                elif variant == "version":
                    env["KIRO_NODE_VERSION"] = "v24.22.3"
                elif variant == "major":
                    env["KIRO_NODE_VERSION"] = "v25.0.0"
                elif variant == "architecture":
                    env["RUNNER_ARCH"] = "unselected"
                elif variant == "directory":
                    binary.unlink()
                    binary.mkdir()
                elif variant == "not-executable":
                    binary.chmod(0o600)
                elif variant == "symlink":
                    binary.unlink()
                    binary.symlink_to(other)
                result = subprocess.run([sys.executable, "-c", source], env=env,
                                        capture_output=True, text=True, check=False)
                proof = runtime / "vendor-proof/node-runtime.json"
                self.assertFalse(ran.exists(), result.stderr)
                self.assertEqual(other.read_text(), body)
                self.assertEqual(result.returncode == 0, variant == "selected", result.stderr)
                if variant == "selected":
                    self.assertEqual(json.loads(proof.read_text()), {
                        "schema_version": 1, "setup_action": "820762786026740c76f36085b0efc47a31fe5020",
                        "version": "v24.22.2", "architecture": "arm64",
                        "executable": str(binary), "bin": str(binary.parent)})
                    self.assertEqual(output.read_text(), "bin=" + str(binary.parent) + "\n")
                else:
                    self.assertFalse(proof.exists())
                    self.assertEqual(output.read_bytes(), b"")

    def test_remaining_matrix_keeps_non_kiro_path_and_kiro_empty_refusal(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        step = workflow.split("      - name: Retain the selected Node runtime for Kiro\n", 1)[1]
        step = step.split("      - name:", 1)[0]
        self.assertIn("        if: matrix.target == 'kiro'\n", step)
        self.assertLess(workflow.index("id: host-node"), workflow.index("id: kiro-node"))
        self.assertLess(workflow.index("id: kiro-node"), workflow.index("id: vendor-install"))
        run = workflow.split("      - name: Run real acquisitions and report every source and outcome\n", 1)[1]
        prefix = textwrap.dedent(run.split("        run: |\n", 1)[1].split("          python3 scripts/live-install-audit.py", 1)[0])
        fixed = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$RUNNER_TEMP/vendor-root/usr/bin:$HOME/.local/bin"
        self.assertEqual(prefix, 'acceptance=()\n'
                         'if [[ "$AUDIT_TARGET" == kiro ]]; then\n'
                         '  test -n "$KIRO_NODE_BIN"\n'
                         '  export PATH="$KIRO_NODE_BIN:' + fixed + '"\n'
                         '  if [[ "$AUDIT_KIND" == plugin ]]; then\n'
                         '    acceptance=(--acceptance kiro-installation)\n'
                         '  fi\n'
                         'else\n  export PATH="' + fixed + '"\nfi\n'
                         'if [[ "$AUDIT_TARGET" == cursor && "$AUDIT_KIND" == plugin ]]; then\n'
                         '  acceptance=(--acceptance cursor-prepared-installation)\n'
                         'elif [[ "$AUDIT_TARGET" == cursor && "$AUDIT_KIND" == skill ]]; then\n'
                         '  acceptance=(--acceptance cursor-prepared-skills-installation)\n'
                         'elif [[ "$AUDIT_TARGET" == kiro && "$AUDIT_KIND" == skill ]]; then\n'
                         '  acceptance=(--acceptance kiro-skills-installation)\n'
                         'elif [[ "$AUDIT_TARGET" == grok && "$AUDIT_KIND" == skill ]]; then\n'
                         '  acceptance=(--acceptance grok-prepared-skills-installation)\n'
                         'elif [[ "$AUDIT_TARGET" == windsurf && "$AUDIT_KIND" == plugin ]]; then\n'
                         '  acceptance=(--acceptance windsurf-global-installation)\n'
                         'elif [[ "$AUDIT_TARGET" == vscode && "$AUDIT_KIND" == plugin ]]; then\n'
                         '  # User preparation and preserved refusal only; no uninstall attestation.\n'
                         '  acceptance=(--acceptance vscode-user-manual-preparation)\n'
                         'fi\n')
        self.assertNotIn("$PATH", prefix)
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home, data = case / "home", case / "data"
            home.mkdir()
            data.mkdir()
            for target in ("cursor", "windsurf", "grok", "vscode", "chatgpt"):
                with self.subTest(target=target), patch.dict(os.environ, {
                        "PATH": fixed, "AUDIT_TARGET": target, "KIRO_NODE_BIN": "/foreign/not-selected"}):
                    child = audit.isolated_client_env(home, case, data)
                    self.assertEqual(child["PATH"], fixed)
                    self.assertNotIn("KIRO_NODE_BIN", child)

    def docker_build_program(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        step = workflow.split("      - name: Build stable Docker CLI for Kiro preparation\n", 1)[1]
        step = step.split("      - name:", 1)[0]
        self.assertIn("if: matrix.target == 'kiro' && matrix.kind == 'plugin'", step)
        source = textwrap.dedent(step.split("python3 - <<'KIRO_DOCKER'\n", 1)[1]
                                .split("\n          KIRO_DOCKER", 1)[0])
        namespace = {"__name__": "inert_docker_acquisition"}
        exec(compile(source, "workflow-kiro-docker", "exec"), namespace)
        return namespace

    def test_kiro_docker_setup_rejects_wrong_toolchain_without_any_process(self):
        program = self.docker_build_program()
        for variant in ("selected", "version", "architecture", "shadow", "mode", "missing"):
            with self.subTest(variant=variant), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                cache, runner = root / "cache", root / "run"
                runner.mkdir()
                go = cache / "go/1.26.11/arm64/bin/go"
                go.parent.mkdir(parents=True)
                go.write_bytes(b"inert fixture: must never execute")
                go.chmod(0o700)
                foreign = root / "foreign/go"
                foreign.parent.mkdir()
                foreign.write_bytes(go.read_bytes())
                foreign.chmod(0o700)
                environment = dict(RUNNER_OS="macOS", RUNNER_ARCH="ARM64", KIRO_GO_VERSION="1.26.11",
                                   RUNNER_TEMP=str(runner), RUNNER_TOOL_CACHE=str(cache),
                                   KIRO_NODE_BIN=str(root / "node/bin"), PATH=str(go.parent))
                if variant == "version":
                    environment["KIRO_GO_VERSION"] = "1.26.10"
                elif variant == "architecture":
                    environment["RUNNER_ARCH"] = "unselected"
                elif variant == "shadow":
                    environment["PATH"] = str(foreign.parent) + os.pathsep + str(go.parent)
                elif variant == "mode":
                    go.chmod(0o777)
                elif variant == "missing":
                    go.unlink()
                with patch.dict(os.environ, environment, clear=True), patch.object(sys, "platform", "darwin"), \
                        patch.dict(program, run_probe=lambda *args, **kwargs: self.fail("setup launched a process")):
                    if variant == "selected":
                        observed = program["setup_inputs"]()
                        self.assertEqual(observed[:4], (runner, go, "arm64", environment["KIRO_NODE_BIN"]))
                        self.assertEqual(observed[4]["bytes"], len(b"inert fixture: must never execute"))
                    else:
                        with self.assertRaises((ValueError, FileNotFoundError)):
                            program["setup_inputs"]()
                self.assertEqual(foreign.read_bytes(), b"inert fixture: must never execute")
                self.assertEqual(list(runner.iterdir()), [])

    def test_kiro_docker_source_ref_and_tree_use_actual_git_before_build(self):
        program = self.docker_build_program()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            source, logs = root / "source", root / "logs"
            source.mkdir()
            logs.mkdir()
            environment = dict(PATH="/usr/bin:/bin", HOME=str(root), GIT_CONFIG_NOSYSTEM="1",
                               GIT_CONFIG_GLOBAL="/dev/null", GIT_TERMINAL_PROMPT="0")
            git = program["GIT"] + ["-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid"]
            def command(*args):
                return subprocess.run(git + list(args), cwd=source, env=environment,
                                      check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout.strip().decode()
            command("init", "-q")
            (source / "file").write_text("original source\n")
            (source / ".gitignore").write_text("ignored\n")
            command("add", ".")
            command("commit", "-qm", "inert source")
            command("tag", "-am", "inert tag", "v29.9.3")
            authority = dict(program["PIN"], tag=command("rev-parse", "refs/tags/v29.9.3"),
                             commit=command("rev-parse", "HEAD"), tree=command("rev-parse", "HEAD^{tree}"))
            for variant in ("original", "tag", "commit", "tree", "dirty", "ignored"):
                with self.subTest(variant=variant):
                    destination = logs / variant
                    destination.mkdir()
                    expected = dict(authority)
                    if variant in ("tag", "commit", "tree"):
                        expected[variant] = "0" * 40
                    elif variant == "dirty":
                        (source / "file").write_text("foreign change\n")
                    elif variant == "ignored":
                        (source / "ignored").write_text("not in the pinned tree\n")
                    context = dict(proof=destination, deadline=program["time"].monotonic() + 10, commands=[])
                    with patch.dict(program, PIN=expected):
                        if variant == "original":
                            program["source_state"](context, source, environment, "before")
                        else:
                            with self.assertRaisesRegex(ValueError, "source ref/tree|original clean tree"):
                                program["source_state"](context, source, environment, "before")
                    self.assertNotIn("docker-source-build", [row["probe"] for row in context["commands"]])
                    (source / "file").write_text("original source\n")
                    (source / "ignored").unlink(missing_ok=True)

    def test_kiro_docker_output_requires_original_owned_selection(self):
        program = self.docker_build_program()
        for variant in ("owned", "alias", "shadow", "missing", "hardlink", "mode"):
            with self.subTest(variant=variant), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                target, foreign = root / "owned", root / "foreign"
                target.mkdir()
                foreign.mkdir()
                binary, link = target / "docker-darwin-arm64", target / "docker"
                binary.write_bytes(b"inert compiler output, never a native proof")
                binary.chmod(0o700)
                (foreign / "docker").write_bytes(binary.read_bytes())
                (foreign / "docker").chmod(0o700)
                link.symlink_to(binary.name)
                search = str(target)
                if variant == "alias":
                    link.unlink()
                    link.symlink_to(foreign / "docker")
                elif variant == "shadow":
                    search = str(foreign) + os.pathsep + search
                elif variant == "missing":
                    binary.unlink()
                elif variant == "hardlink":
                    os.link(binary, root / "additional-alias")
                elif variant == "mode":
                    binary.chmod(0o777)
                if variant == "owned":
                    result = program["output_cli"](target, "arm64", search)
                    self.assertEqual(result["selected"], str(link))
                    self.assertEqual(result["bytes"], len(b"inert compiler output, never a native proof"))
                else:
                    with self.assertRaises((ValueError, FileNotFoundError)):
                        program["output_cli"](target, "arm64", search)
                self.assertEqual((foreign / "docker").read_bytes(), b"inert compiler output, never a native proof")

    def test_kiro_docker_parent_retains_failed_prefix_and_never_promotes_late_refusal(self):
        import hashlib

        program = self.docker_build_program()
        labels = ["docker-go-version", "docker-source-clone", "docker-source-checkout",
                  "docker-before-identity", "docker-before-clean", "docker-source-build",
                  "docker-after-identity", "docker-after-clean"]
        counts = {"built": 8, "version": 1, "build-failure": 6, "source-change": 7,
                  "go-replaced": 8, "parent-replaced": 8, "missing-output": 8, "expired": 6,
                  "foreign-before": 0, "foreign-output": 8, "immutable-parent-child": 8,
                  "output-link-count": 8}
        original_directory = program["directory"]
        for variant, count in counts.items():
            with self.subTest(variant=variant), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                runner, cache = root / "run", root / "cache"
                runner.mkdir()
                go = cache / "go/1.26.11/arm64/bin/go"
                go.parent.mkdir(parents=True)
                go.write_bytes(b"inert Go fixture: never executed")
                go.chmod(0o700)
                destination = runner / "vendor-root/usr/bin"
                destination.mkdir(parents=True)
                original_parent = original_directory(destination.parent)
                if variant == "foreign-before":
                    (destination / "foreign").write_bytes(b"original foreign output member")
                environment = dict(RUNNER_OS="macOS", RUNNER_ARCH="ARM64", KIRO_GO_VERSION="1.26.11",
                                   RUNNER_TEMP=str(runner), RUNNER_TOOL_CACHE=str(cache), HOME=str(root),
                                   KIRO_NODE_BIN=str(destination), PATH=str(go.parent), DOCKER_HOST="foreign")
                seen, clock, link_changes = [], [100.0], []

                def directory(path):
                    observed = original_directory(path)
                    if variant == "output-link-count" and path == destination and "docker-source-build" in seen:
                        # An inert metadata seam, not a claim about the original macOS stat delta.
                        before = observed.copy()
                        observed[5] += 1
                        link_changes.append((before, observed.copy()))
                    return observed

                def probe(argv, cwd, env, output, label, timeout, limit, *, memory_only):
                    self.assertEqual(label, labels[len(seen)])
                    self.assertTrue(memory_only)
                    self.assertEqual(limit, 64 << 10)
                    self.assertGreater(timeout, 0)
                    self.assertNotIn("DOCKER_HOST", env)
                    self.assertEqual(env["GOENV"], "off")
                    self.assertEqual(env["GOTOOLCHAIN"], "local")
                    self.assertEqual(env["GOPROXY"], "off")
                    seen.append(label)
                    stdout, stderr, code = b"", b"", 0
                    if label == "docker-go-version":
                        self.assertEqual(argv, [str(go), "version"])
                        stdout = b"wrong version\n" if variant == "version" else b"go version go1.26.11 darwin/arm64\n"
                    elif label == "docker-source-clone":
                        Path(argv[-1]).mkdir(parents=True)
                    elif label.endswith("identity"):
                        stdout = "".join(program["PIN"][key] + "\n" for key in ("tag", "commit", "tree")).encode()
                        if variant == "source-change" and label == "docker-after-identity":
                            stdout = b"0" * 40 + stdout[40:]
                    elif label == "docker-source-build":
                        self.assertEqual(argv, ["/bin/sh", str(Path(cwd) / "scripts/build/binary")])
                        if variant == "build-failure":
                            code, stderr = 2, b"inert compiler refusal\n"
                        elif variant != "missing-output":
                            binary = destination / "docker-darwin-arm64"
                            binary.write_bytes(b"inert Docker fixture: never executed")
                            binary.chmod(0o700)
                            (destination / "docker").symlink_to(binary.name)
                        if variant == "go-replaced":
                            go.rename(go.with_name("original-go"))
                            go.write_bytes(b"inert Go fixture: never executed")
                            go.chmod(0o700)
                        elif variant == "parent-replaced":
                            previous = destination.with_name("original-bin")
                            destination.rename(previous)
                            destination.mkdir()
                            for child in previous.iterdir():
                                child.rename(destination / child.name)
                        elif variant == "expired":
                            clock[0] += 421
                        elif variant == "foreign-output":
                            (destination / "foreign").write_bytes(b"unexpected build output member")
                        elif variant == "immutable-parent-child":
                            (destination.parent / "foreign-child").mkdir()
                    record = dict(probe=label, argv=argv, cwd=str(cwd), exit=code,
                                  error=None, elapsed_seconds=0.001)
                    for name, body in (("stdout", stdout), ("stderr", stderr)):
                        record.update({name + "_file": label + "." + name + ".log",
                                       name + "_bytes": len(body), name + "_sha256": hashlib.sha256(body).hexdigest()})
                    return record, stdout, stderr

                successful = variant in ("built", "output-link-count")
                previous_umask = os.umask(0o077)
                try:
                    with patch.dict(os.environ, environment, clear=True), patch.object(sys, "platform", "darwin"), \
                            patch.dict(program, run_probe=probe, directory=directory), \
                            patch.object(program["time"], "monotonic", side_effect=lambda: clock[0]):
                        if successful:
                            program["provision"]()
                        else:
                            with self.assertRaises(ValueError):
                                program["provision"]()
                finally:
                    os.umask(previous_umask)
                receipt = json.loads((runner / "vendor-proof/docker-runtime.json").read_bytes())
                self.assertEqual(seen, labels[:count])
                self.assertEqual([row["probe"] for row in receipt["commands"]], seen)
                self.assertEqual(receipt["status"], "source-built" if successful else "failed")
                self.assertEqual(receipt["source"], program["PIN"])
                self.assertIs(receipt["native_activation"], False)
                self.assertIs(receipt["daemon_started"], False)
                self.assertEqual("executable" in receipt, successful)
                self.assertEqual("source_unchanged" in receipt, successful)
                self.assertEqual(len(link_changes), 1 if variant == "output-link-count" else 0)
                for before, after in link_changes:
                    self.assertEqual(before[:5], after[:5])
                    self.assertEqual(after[5], before[5] + 1)
                if variant == "foreign-before":
                    self.assertEqual((destination / "foreign").read_bytes(), b"original foreign output member")
                    self.assertEqual({path.name for path in destination.iterdir()}, {"foreign"})
                elif variant == "foreign-output":
                    self.assertEqual((destination / "foreign").read_bytes(), b"unexpected build output member")
                elif variant == "immutable-parent-child":
                    current_parent = original_directory(destination.parent)
                    self.assertEqual(current_parent[:5], original_parent[:5])
                    self.assertNotEqual(current_parent[5], original_parent[5])
                    self.assertTrue((destination.parent / "foreign-child").is_dir())
                for row in receipt["commands"]:
                    for name in ("stdout", "stderr"):
                        body = (runner / "vendor-proof" / row[name + "_file"]).read_bytes()
                        self.assertEqual(len(body), row[name + "_bytes"])
                        self.assertEqual(hashlib.sha256(body).hexdigest(), row[name + "_sha256"])

    def test_vendor_bootstrap_requires_reviewed_bytes_and_preserves_runner_toolchain(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        self.assertIn("sha256_file -c -", workflow)
        self.assertIn("observed-untrusted.sha256", workflow)
        self.assertIn("AUDIT_VENDOR_OUTCOME", workflow)
        self.assertIn("shasum -a 256", workflow)
        self.assertIn("verification-blocked.txt", workflow)
        self.assertIn("dpkg-deb --extract", workflow)
        self.assertNotIn("sudo DEBIAN_FRONTEND", workflow)
        self.assertNotIn('export PATH="$HOME/.local/bin:$PATH"', workflow)
        self.assertIn(":/usr/bin:/sbin:/bin:", workflow)

    def test_grok_build_uses_independently_sha_pinned_binary_not_installer(self):
        # BUG-056: Grok Build's official immutable Homebrew Cask publishes
        # a Linux x64 ELF digest; a mutable shell installer cannot qualify E2E.
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        self.assertIn('https://x.ai/cli/grok-1.0.70-linux-x86_64', workflow)
        self.assertIn('SHA256="22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9"', workflow)
        self.assertIn('Homebrew/homebrew-cask Casks/g/grok-build.rb @ b75a8e94456970fa349c75b59ca5e4183dde8e28', workflow)
        self.assertNotIn('https://x.ai/cli/install.sh', workflow)
        self.assertNotIn('bash "$artifact"', workflow)
        self.assertIn('artifact="$RUNNER_TEMP/grok-1.0.70-linux-x86_64"', workflow)
        self.assertIn('executable="$RUNNER_TEMP/vendor-root/grok"', workflow)
        self.assertIn('install -m 0755 "$artifact" "$executable"', workflow)
        self.assertIn('ln -s "$executable" "$HOME/.local/bin/grok"', workflow)
        self.assertIn('test -f "$executable" && test -x "$executable" && test ! -L "$executable"', workflow)
        self.assertLess(workflow.index('sha256_file -c -'),
                        workflow.index('install -m 0755 "$artifact" "$executable"'))
        self.assertIn('env -i PATH="$PATH" HOME="$probe_home"', workflow)

    def test_devin_project_skills_use_exact_reviewed_native_version(self):
        # Both obsolete CDN routes stopped before cases with HTTP 403. The
        # existing User and Project admission already supports official .12.57.
        root = Path(__file__).resolve().parents[1]
        workflow = (root / ".github/workflows/live-install-remaining-clients.yml").read_text()
        self.assertEqual(workflow.count("            windsurf)"), 1)
        vendor = workflow.split("            windsurf)", 1)[1].split("              ;;", 1)[0]
        self.assertIn('https://registry.npmjs.org/devin/-/devin-3000.12.57-linux-x64.tgz', vendor)
        self.assertNotIn('if [[ "$KIND"', vendor)
        self.assertNotIn("static.devin.ai/cli/", workflow)
        self.assertIn("PYTHONPATH=scripts python3 -", workflow)
        self.assertIn('executable="$RUNNER_TEMP/vendor-devin/bin/devin"', workflow)
        self.assertIn('test -f "$executable" && test -x "$executable" && test ! -L "$executable" || exit 4', workflow)
        self.assertIn('env -i PATH="$PATH" HOME="$probe_home"', workflow)
        version = (root / "install/integrationctl/agentplugins/clients/windsurf/version_profile.go").read_text()
        self.assertIn('version == "3000.12.57"', version)
        for relative in ("global_profile.go", "project_validation.go"):
            self.assertIn("supportedDevinNativeVersion(client.Version)",
                          (root / "install/integrationctl/agentplugins/clients/windsurf" / relative).read_text())

    def test_devin_npm_bootstrap_binds_reviewed_archive_binary_and_package_path(self):
        from types import SimpleNamespace
        from unittest.mock import Mock
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        marker = 'PYTHONPATH=scripts python3 - "$RUNNER_TEMP" "$URL" <<\'PY_DEVIN\'\n'
        self.assertIn(marker, workflow)
        source = textwrap.dedent(workflow.split(marker, 1)[1].split("\n          PY_DEVIN", 1)[0])
        integrity = "sha512-SZhb9rrYghHvOh0PRpZT1GpNPMMo+7P9VH6t6ZNMrBmyVX3x3XflPvCv8yomx/i7vnvf64bdFR++cRRg/c8mLQ=="
        url = "https://registry.npmjs.org/devin/-/devin-3000.12.57-linux-x64.tgz"
        binary_sha = "3f3fb99f472d92f7852be3775ac2b66b5d60baf26f3441b02d54c56b160bc30d"
        for mode in ("good", "integrity", "version", "url", "binary", "path", "acquisition"):
            with self.subTest(mode=mode), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(strict=True)
                (root / "vendor-proof").mkdir()
                install = Mock(return_value=(root / "vendor-devin/bin/devin", integrity))
                module = SimpleNamespace(VERSION="3000.12.57", TARBALL_URL=url,
                                         BINARY_SHA=binary_sha, install_genuine_devin=install)
                if mode == "integrity":
                    install.return_value = (root / "vendor-devin/bin/devin", "sha512-unreviewed")
                elif mode == "path":
                    install.return_value = (root / "foreign/devin", integrity)
                elif mode == "acquisition":
                    install.side_effect = RuntimeError("ordinary acquisition refused")
                elif mode in ("version", "url", "binary"):
                    setattr(module, {"version": "VERSION", "url": "TARBALL_URL", "binary": "BINARY_SHA"}[mode], "unreviewed")
                with patch.dict(sys.modules, devin_project_auto_install_e2e=module), \
                     patch.object(sys, "argv", ["workflow", temporary, url]):
                    if mode == "good":
                        exec(compile(source, "remaining-devin-bootstrap", "exec"), {})
                    else:
                        with self.assertRaisesRegex(RuntimeError, "Devin|acquisition"):
                            exec(compile(source, "remaining-devin-bootstrap", "exec"), {})
                proof = json.loads((root / "vendor-proof/devin-npm.json").read_text())
                self.assertEqual(proof["acquisition_verified"], mode == "good")
                self.assertEqual(proof["reviewed_integrity"], integrity)
                self.assertEqual(install.call_count, 0 if mode in ("version", "url", "binary") else 1)

    def test_vscode_deb_uses_exact_verified_staged_launcher(self):
        # BUG-055: /usr/bin/code may be an absolute .deb symlink, not a
        # runnable binary after extraction into an unprivileged temp root.
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        self.assertIn('executable="$RUNNER_TEMP/vendor-root/usr/share/code/bin/code"', workflow)
        self.assertIn('test -f "$executable" && test -x "$executable" && test ! -L "$executable"', workflow)
        self.assertIn('test "$(readlink -f "$executable")" = "$executable"', workflow)
        self.assertIn('ln -s "$executable" "$HOME/.local/bin/code"', workflow)
        self.assertIn('python3 - "$BIN" "$executable" <<\'PY\'', workflow)
        self.assertIn('Path(selected).resolve(strict=True) != Path(sys.argv[2]).resolve(strict=True)', workflow)
        self.assertLess(workflow.index('sha256_file -c -'),
                        workflow.index('dpkg-deb --extract "$artifact"'))
        self.assertIn('vscode) SHA256="b8fbff92d8d95908990eed0acf004c980f29003725469e794ddbd017ea511a20"', workflow)

    @unittest.skipUnless(sys.platform == "linux", "Debian staged launcher contract")
    def test_cursor_deb_binds_staged_launcher_without_package_install(self):
        import shutil

        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        start = '            if [[ "$TARGET" == vscode ]]; then\n'
        choice = textwrap.dedent(start + workflow.split(start, 1)[1].split(
            '          elif [[ "$TARGET" == kiro ]]; then\n', 1)[0])
        start = '          hash -r\n'
        joined = textwrap.dedent(start + workflow.split(start, 1)[1].split(
            '          # No ambient CI credentials', 1)[0])
        script = "set -euo pipefail\n" + choice + joined
        bash = shutil.which("bash")
        self.assertIsNotNone(bash)
        for variant in ("selected", "missing", "non-executable", "symlink", "shadow"):
            with self.subTest(variant=variant), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                home, runtime, tools = root / "home", root / "run", root / "tools"
                selected = home / ".local/bin/cursor"
                selected.parent.mkdir(parents=True)
                tools.mkdir()
                (runtime / "vendor-proof").mkdir(parents=True)
                # Exact c8003e... package layout: no usr/bin member. Its
                # postinst creates that link, but must never be executed here.
                packaged_bin = runtime / "vendor-root/usr/bin"
                packaged_bin.mkdir(parents=True)
                launcher = runtime / "vendor-root/usr/share/cursor/bin/cursor"
                launcher.parent.mkdir(parents=True)
                launcher.write_text("#!/bin/sh\nexit 97\n")
                launcher.chmod(0o755)
                for command in ("ln", "readlink", "tee"):
                    original = shutil.which(command)
                    self.assertIsNotNone(original)
                    (tools / command).symlink_to(original)
                (tools / "python3").symlink_to(sys.executable)
                if variant == "missing":
                    launcher.unlink()
                elif variant == "non-executable":
                    launcher.chmod(0o644)
                elif variant in ("symlink", "shadow"):
                    foreign = tools / "cursor"
                    foreign.write_text("#!/bin/sh\nexit 98\n")
                    foreign.chmod(0o755)
                    if variant == "symlink":
                        launcher.unlink()
                        launcher.symlink_to(foreign)
                env = dict(os.environ, HOME=str(home), RUNNER_TEMP=str(runtime),
                           TARGET="cursor", KIND="plugin", BIN="cursor",
                           PATH=os.pathsep.join(map(str, (tools, packaged_bin, selected.parent))))
                env.pop("executable", None)
                result = subprocess.run([bash, "--noprofile", "--norc", "-c", script],
                                        env=env, capture_output=True, text=True,
                                        timeout=5, check=False)
                self.assertEqual(result.returncode == 0, variant == "selected", result.stderr)
                self.assertEqual(list(packaged_bin.iterdir()), [])
                if variant == "selected":
                    self.assertEqual(selected.resolve(strict=True), launcher)
                    self.assertEqual((runtime / "vendor-proof/executable.txt").read_text(),
                                     str(selected) + "\n")
                    self.assertEqual(launcher.read_text(), "#!/bin/sh\nexit 97\n")

    def test_devin_user_plugin_without_selected_windsurf_channel_is_nonpass(self):
        # BUG-055: a real devin CLI does not select a legacy Windsurf user
        # profile, so fail-closed root selection is not a broken package.
        audit = load_audit()
        refused = {"exit_code": 1, "stdout_tail": "",
                   "stderr_tail": ("preflight atlassian/windsurf: installer request "
                                   "rejected: ClientConfigRoot must be an explicit "
                                   "absolute clean path"),
                   "timeout": False, "_json": None}
        empty = {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                 "timeout": False, "_json": {"data": {"installations": []}}}

        def fake_call(argv, env, cwd, timeout=105):
            return refused if argv[1] == "add" else empty

        with TemporaryDirectory() as temporary, patch.object(audit, "call", side_effect=fake_call):
            evidence = audit.do_case("/unused/uap", "windsurf", "plugin",
                                     "atlassian", 0, Path(temporary), {})
        self.assertEqual(evidence["status"], "target-unavailable")
        self.assertEqual(evidence["install"]["exit_code"], 1)
        self.assertFalse(evidence["listed_identity"])
        self.assertFalse(evidence["failed_install_residue"])
        self.assertTrue(evidence["sentinels_preserved"])
        self.assertIsNone(evidence["cleanup"])

    def test_kiro_native_archive_is_pinned_before_extraction(self):
        # Independently pinned Homebrew Cask 3.0.0, not a mutable install script.
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        self.assertIn("https://desktop-release.q.us-east-1.amazonaws.com/3.0.0/kirocli-universal-darwin.zip", workflow)
        self.assertIn('kiro) SHA256="0a8f067c4fe6a23a231aa3a18e3e4332120ef9ed3d2d7b6eea907beb8b4cdd8b"', workflow)
        self.assertNotIn('kiro) URL="https://cli.kiro.dev/install"', workflow)
        self.assertLess(workflow.index('sha256_file -c -'),
                        workflow.index('ditto -x -k "$artifact"'))
        self.assertIn('test -f "$executable" && test -x "$executable" && test ! -L "$executable"', workflow)
        self.assertNotIn('export PATH="$HOME/.local/bin:$PATH"', workflow)

    def test_failed_add_inspects_pending_registration_without_false_matches(self):
        audit = load_audit()
        pending = {"data": {"installations": [
            {"name": "atlassian", "clients": [
                {"client_id": "grok", "verification": "pending"}]},
            {"name": "other", "clients": [{"client_id": "codex"}]},
        ]}}
        self.assertTrue(audit.has_target_registration(pending, "atlassian", "grok"))
        self.assertFalse(audit.has_target_registration(pending, "atlassian", "codex"))
        self.assertFalse(audit.has_target_registration(pending, "other", "grok"))
        for malformed in (None, [], {}, {"data": {"installations": "bad"}},
                          {"data": {"installations": [None, "bad"]}}):
            self.assertFalse(audit.has_target_registration(malformed, "atlassian", "grok"))

    def test_signed_release_observation_only_exposes_public_allowlisted_fields(self):
        # BUG-025: diagnose revision drift without logging credentials.
        audit = load_audit()
        response = {"data": {"installations": [
            {"name": "atlassian", "source": "777genius/universal-agent-plugins/plugins/atlassian",
             "api_key": "secret-from-account",
             "directory": {"recorded_distribution": "777genius/atlassian",
                           "recorded_revision": "a" * 40,
                           "recorded_release_sequence": 1,
                           "private_config": "secret"},
             "clients": [
                 {"client_id": "codex",
                  "package_revision": {
                      "resolved_revision": "b" * 40, "release_sequence": 1,
                      "tree_digest": "sha256:" + "c" * 64,
                      "manifest_digest": "sha256:" + "d" * 64,
                      "credential": "secret"}},
                 {"client_id": "claude",
                  "package_revision": {"credential": "other-secret"}},
             ]}
        ]}}
        observed = audit.signed_release_public_observation(
            response, "atlassian", "codex")
        self.assertEqual(len(observed), 1)
        self.assertEqual(observed[0]["source"], "777genius/universal-agent-plugins/plugins/atlassian")
        self.assertEqual(observed[0]["resolved_revision"], "b" * 40)
        self.assertNotIn("secret", repr(observed))
        response["data"]["installations"][0]["source"] = (
            "https://alice:secret@host.invalid/private")
        observed = audit.signed_release_public_observation(
            response, "atlassian", "codex")
        self.assertIsNone(observed[0]["source"])
        self.assertEqual(audit.signed_release_public_observation(
            response, "atlassian", "grok"), [])


    def test_public_target_states_select_only_owned_target_and_project_finite_enums(self):
        audit = load_audit()
        selected = {"client_id": "kiro", "materialization": "materialized",
                    "activation": "prepared", "authentication": "auth_pending",
                    "policy": "allowed", "verification": "package_validated",
                    "path": "/private/account", "credential": "password=private"}
        response = {"data": {"installations": [
            {"name": "foreign-private", "clients": [dict(selected, activation="failed")]},
            {"name": "owned-private", "installation_id": "private-id", "clients": [
                dict(selected, client_id="claude", activation="failed"), selected]},
        ]}}
        original = json.dumps(response, sort_keys=True)
        observed = audit.public_target_state_observation(response, "owned-private", "kiro")
        expected = {key: selected[key] for key in
                    ("materialization", "activation", "authentication", "policy", "verification")}
        self.assertEqual(observed, {"status": "observed", "selected_installations": 1,
                                   "selected_bindings": 1, "invalid_rows": 0,
                                   "states": [expected], "truncated": False})
        self.assertNotIn("private", json.dumps(observed))
        self.assertNotIn("credential", json.dumps(observed))
        self.assertEqual(json.dumps(response, sort_keys=True), original)
        self.assertEqual(audit.failed_case_diagnostics(
            {"public_target_states": {"after-add": observed}}),
            {"public_target_states": {"after-add": observed}})

    def test_unqualified_skill_source_remains_visible_without_qualifying_its_binding(self):
        for failed_phase in ('after-add', 'repeat-add'):
            for verification in ('package_validated', 'failed'):
                with self.subTest(phase=failed_phase, verification=verification), TemporaryDirectory() as temp:
                    audit = load_audit()
                    root = Path(temp).resolve()
                    project = root / 'skill-00/project'
                    skill = project / '.gemini/skills/owned-test/SKILL.md'
                    body = b'---\nname: owned-test\ndescription: selected fixture\n---\n'
                    item = {'name': 'owned-test', 'repo': 'example/skills', 'folder': 'owned-test',
                            'source': 'unused', 'skill_md_git_blob_sha1': hashlib.sha1(
                                b'blob ' + str(len(body)).encode() + b'\0' + body).hexdigest()}
                    source = 'example/skills/skills/owned-test'
                    operations, rejected = [], []

                    def perform(argv, env, cwd, timeout=105):
                        operation = argv[2] if argv[1] == 'skills' else argv[1]
                        operations.append(operation)
                        response = {'readiness': 'ready', 'next_actions': []}
                        if operation == 'add':
                            skill.parent.mkdir(parents=True, exist_ok=True)
                            skill.write_bytes(body)
                            (project / 'uap.toml').write_text('owned-test')
                            (project / 'uap.lock').write_text('owned-test')
                        elif operation == 'remove':
                            skill.unlink()
                            skill.parent.rmdir()
                            (project / 'uap.lock').unlink()
                        elif operation == 'list':
                            invalid = operations.count('list') == (1 if failed_phase == 'after-add' else 2)
                            state = {'client_id': 'gemini', 'materialization': 'materialized',
                                     'activation': 'active', 'authentication': 'not_required', 'policy': 'allowed',
                                     'verification': verification if invalid else 'installation_verified'}
                            selected = {'installation_id': 'selected-id', 'name': 'skill-generated-name',
                                        'source': source, 'clients': [state]}
                            neighbor = dict(selected, installation_id='other-id', source=source + '-other',
                                            clients=[dict(state, verification='not_run')])
                            response = {'data': {'installations': [] if 'remove' in operations else
                                                 [selected, neighbor]}}
                            if invalid:
                                rejected.append(response)
                        return {'exit_code': 0, 'stdout_tail': '', 'stderr_tail': '',
                                'timeout': False, '_json': response}

                    with (patch.object(audit, 'call', side_effect=perform),
                          patch.object(audit, 'locked_skill_is_pinned',
                                       side_effect=lambda *a: (project / 'uap.lock').exists())):
                        row = audit.do_case('/inert/uap', 'gemini', 'skill', item, 0, root, {})
                    self.assertEqual(len(rejected), 1)
                    self.assertEqual(audit.matching_installations(
                        rejected[0], item['name'], 'gemini', 'skill', item), [])
                    state = row['public_target_states'][failed_phase]
                    self.assertEqual((state['selected_installations'], state['selected_bindings']), (1, 1))
                    self.assertEqual([v['verification'] for v in state['states']], [verification])
                    self.assertNotEqual(row['status'], 'passed')
                    self.assertFalse(row['repeat_verified'])
                    self.assertEqual(row['repeat_installation_ids'], [])
                    self.assertEqual(row['matched_installation_ids'],
                                     [] if failed_phase == 'after-add' else ['selected-id'])
                    self.assertEqual(operations, ['add', 'list'] if failed_phase == 'after-add' else
                                     ['add', 'list', 'add', 'list', 'remove', 'list'])

    def test_failed_case_diagnostics_projects_gemini_refusal_without_journal_contents(self):
        audit = load_audit()
        proof = {
            "status": "unqualified", "reason": "Gemini startup environment framing differs",
            "phases": ["after-public-repeat-add"],
            "baseline": {"bytes": 1234, "records": 3,
                         "started": "2026-11-18T10:40:00+00:00",
                         "finished": "2026-11-18T10:40:01+00:00",
                         "path": "/private/journal", "text": "private journal content"},
            "failure_facts": {"phase": "after-public-repeat-list", "role": "new-chat",
                              "operation": "record-framing", "bytes": 1234, "lf_count": 3,
                              "final_lf": True, "record_shapes": [
                                  "metadata-fields", "user-text-fields", "last-updated-fields"],
                              "text": "private journal content", "stdout": "private native output"},
            "files": [{"path": "/private/journal", "body": "private journal content"}],
            "stdout": "private native output",
        }
        row = {"status": "repeat-add-unqualified", "repeat_verified": False,
               "public_skill_ready": {"after-add": False, "repeat-add": True},
               "removal_verified": True, "gemini_runtime_proof": proof}
        original = json.dumps(row, sort_keys=True)
        observed = audit.failed_case_diagnostics(row)
        expected = {
            "status": proof["status"], "reason": proof["reason"],
            "phases": proof["phases"], "phases_truncated": False,
            "baseline": {key: proof["baseline"][key]
                         for key in ("bytes", "records", "started", "finished")},
            "failure_facts": {key: proof["failure_facts"][key] for key in (
                "phase", "role", "operation", "bytes", "lf_count", "final_lf", "record_shapes")},
        }
        expected["failure_facts"]["record_shapes_truncated"] = False
        self.assertEqual(observed, {"repeat_verified": False, "removal_verified": True,
                                   "public_skill_ready": {"after-add": False, "repeat-add": True},
                                   "gemini_runtime_proof": expected})
        self.assertNotIn("private", json.dumps(observed))
        self.assertEqual(json.dumps(row, sort_keys=True), original)

    def test_failed_case_diagnostics_bounds_invalid_gemini_facts_without_changing_other_clients(self):
        audit = load_audit()
        for proof in (None, [], "private", 7):
            with self.subTest(proof=proof):
                self.assertEqual(audit.failed_case_diagnostics(
                    {"repeat_verified": False, "gemini_runtime_proof": proof}),
                    {"repeat_verified": False})
        proof = {
            "status": "x" * 1000, "reason": "r" * 1000,
            "phases": ["after-public-repeat-add", {"private": "secret"}] * 100,
            "baseline": {"bytes": 1 << 30, "records": {"private": "secret"},
                         "started": "s" * 1000, "finished": ["private"]},
            "failure_facts": {"operation": "o" * 1000, "bytes": -1,
                              "within_limit": True, "slug_binding_count": None,
                              "record_shapes": ["metadata-fields", {"private": "secret"},
                                                "private"] * 100,
                              "unknown": "private"},
        }
        original = json.dumps(proof, sort_keys=True)
        observed = audit.failed_case_diagnostics({"gemini_runtime_proof": proof})["gemini_runtime_proof"]
        self.assertEqual(observed["status"], "x" * 512)
        self.assertEqual(observed["reason"], "r" * 512)
        self.assertEqual(observed["phases"], ["after-public-repeat-add", "invalid"])
        self.assertTrue(observed["phases_truncated"])
        self.assertEqual(observed["baseline"], {
            "bytes": "invalid", "records": "invalid", "started": "s" * 64, "finished": "invalid"})
        self.assertEqual(observed["failure_facts"], {
            "operation": "o" * 64, "bytes": "invalid", "within_limit": True,
            "slug_binding_count": None, "record_shapes": ["metadata-fields", "invalid", "invalid"],
            "record_shapes_truncated": True})
        self.assertLess(len(json.dumps(observed).encode()), 4096)
        self.assertNotIn("private", json.dumps(observed))
        self.assertEqual(json.dumps(proof, sort_keys=True), original)

    def test_public_target_states_select_skill_ids_and_reminted_exact_source(self):
        audit = load_audit()
        source = "private-repo/skills/owned-test"
        state = {"materialization": "materialized", "activation": "active",
                 "authentication": "not_required", "policy": "allowed",
                 "verification": "installation_verified"}
        owned = {"name": "private-generated-name", "installation_id": "private-old-id",
                 "source": source, "clients": [
                     dict(state, client_id="kiro", activation="failed"),
                     dict(state, client_id="claude", path="/private/native/skill")]}
        response = {"data": {"installations": [
            {"name": "owned-test", "installation_id": "private-foreign-id",
             "source": source + "-other", "clients": [dict(state, client_id="claude", activation="failed")]},
            owned,
            {"name": "owned-test", "installation_id": {"private": "malformed"},
             "source": source + "/other", "clients": [dict(state, client_id="claude")]},
        ]}}
        for installation_id in ("private-old-id", "private-repeat-id"):
            with self.subTest(installation_id=installation_id):
                owned["installation_id"] = installation_id
                observed = audit.public_target_state_observation(
                    response, "owned-test", "claude", installation_ids={installation_id})
                self.assertEqual(observed["selected_installations"], 1)
                self.assertEqual(observed["selected_bindings"], 1)
                self.assertEqual(observed["states"], [state])
                self.assertNotIn("private", json.dumps(observed))
        self.assertEqual(audit.public_target_state_observation(
            response, "owned-test", "claude", installation_ids=set())["selected_bindings"], 0)
        owned.update(name="private-renamed", installation_id="private-reminted-id")
        owned["clients"][1]["verification"] = "failed"
        ids = {"private-old-id", "private-repeat-id"}
        self.assertEqual(audit.public_target_state_observation(
            response, "owned-test", "claude", installation_ids=ids)["selected_bindings"], 0)
        observed = audit.public_target_state_observation(
            response, "owned-test", "claude", installation_ids=ids, skill_source=source)
        self.assertEqual(observed["selected_installations"], 1)
        self.assertEqual(observed["selected_bindings"], 1)
        self.assertEqual(observed["states"], [dict(state, verification="failed")])
        self.assertNotIn("private", json.dumps(observed))
        owned["source"] = source + "/other"
        self.assertEqual(audit.public_target_state_observation(
            response, "owned-test", "claude", installation_ids=ids,
            skill_source=source)["selected_bindings"], 0)

    def test_public_target_states_bound_rows_and_redact_invalid_values(self):
        audit = load_audit()
        for value in (None, {}, {"data": {"installations": "private"}}):
            self.assertEqual(audit.public_target_state_observation(value, "owned", "kiro"),
                             {"status": "unobserved" if value is None else "invalid"})
        client = {"client_id": "kiro", "materialization": ["private"],
                  "activation": "private", "authentication": True, "policy": None,
                  "verification": {"private": "secret"}}
        response = {"data": {"installations": [
            {"name": "owned", "clients": [client] * 33}] * 33}}
        observed = audit.public_target_state_observation(response, "owned", "kiro")
        self.assertEqual(observed["selected_installations"], 32)
        self.assertEqual(observed["selected_bindings"], 32)
        self.assertTrue(observed["truncated"])
        self.assertEqual(observed["states"], [dict.fromkeys(
            ("materialization", "activation", "authentication", "policy", "verification"), "invalid")])
        self.assertNotIn("private", json.dumps(observed))
        variants = [dict(client, activation=value) for value in
                    ("not_required", "prepared", "manual_activation_required", "active", "failed")]
        observed = audit.public_target_state_observation(
            {"data": {"installations": [{"name": "owned", "clients": variants}]}}, "owned", "kiro")
        self.assertEqual(len(observed["states"]), 4)
        self.assertTrue(observed["truncated"])

    def test_signed_release_requires_same_verified_target_and_immutable_metadata(self):
        audit = load_audit()
        directory = {
            "product_id": "atlassian",
            "recorded_distribution": "777genius/atlassian",
            "recorded_revision": "a" * 40,
            "recorded_release_sequence": 1,
        }
        revision = {
            "distribution_id": "777genius/atlassian",
            "resolved_revision": "a" * 40,
            "release_sequence": 1,
            "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64,
        }
        verified = {"client_id": "codex", "verification": "installation_verified",
                    "package_revision": revision}
        node = {"name": "atlassian", "installation_id": "valid-id",
                "source": "777genius/universal-agent-plugins/plugins/atlassian", "directory": directory,
                "clients": [verified]}
        pins = {"atlassian": {
            "distribution": "777genius/atlassian",
            "repository": "777genius/universal-agent-plugins",
            "path": "plugins/atlassian", "revision": "a" * 40,
            "sequence": 1, "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64,
        }}
        def matching():
            return audit.matching_installations(
                {"data": {"installations": [node]}}, "atlassian", "codex",
                "plugin", "atlassian", pins,
            )
        self.assertEqual(len(matching()), 1)
        for key, value in (("resolved_revision", "d" * 40),
                           ("distribution_id", "wrong/package"),
                           ("release_sequence", 2),
                           ("tree_digest", "sha256:short"),
                           ("manifest_digest", None)):
            with self.subTest(key=key):
                broken = dict(revision, **{key: value})
                node["clients"] = [dict(verified, package_revision=broken)]
                self.assertEqual(matching(), [])
        node["clients"] = [
            dict(verified, package_revision={}),
            {"client_id": "codex", "verification": "pending",
             "package_revision": revision},
        ]
        self.assertEqual(matching(), [])
        node["clients"] = [dict(verified, client_id="grok")]
        self.assertEqual(matching(), [])

    def test_signed_release_rejects_forged_source_even_when_self_consistent(self):
        audit = load_audit()
        pin = {
            "distribution": "777genius/atlassian",
            "repository": "777genius/universal-agent-plugins",
            "path": "plugins/atlassian", "revision": "a" * 40,
            "sequence": 1, "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64,
        }
        directory = {"product_id": "atlassian", "recorded_distribution": pin["distribution"],
                     "recorded_revision": pin["revision"], "recorded_release_sequence": 1}
        revision = {"distribution_id": pin["distribution"], "resolved_revision": pin["revision"],
                    "release_sequence": 1, "tree_digest": pin["tree_digest"],
                    "manifest_digest": pin["manifest_digest"]}
        record = {"name": "atlassian", "installation_id": "proof",
                  "source": "777genius/universal-agent-plugins/plugins/atlassian", "directory": directory,
                  "clients": [{"client_id": "codex", "verification": "installation_verified",
                               "package_revision": revision}]}
        def qualify():
            return audit.matching_installations(
                {"data": {"installations": [record]}}, "atlassian", "codex",
                "plugin", "atlassian", {"atlassian": pin})
        self.assertEqual(len(qualify()), 1)
        for fake in ("evil/atlassian-fork", pin["distribution"], None):
            record["source"] = fake
            self.assertEqual(qualify(), [])
        record["source"] = "777genius/universal-agent-plugins/plugins/atlassian"
        original = record["directory"]
        record["directory"] = dict(original, recorded_revision="d" * 40)
        record["clients"][0]["package_revision"] = dict(
            revision, resolved_revision="d" * 40)
        self.assertEqual(qualify(), [])
        record["directory"] = original
        record["clients"][0]["package_revision"] = dict(
            revision, tree_digest="sha256:" + "d" * 64)
        self.assertEqual(qualify(), [])
        self.assertEqual(audit.matching_installations(
            {"data": {"installations": [record]}}, "atlassian", "codex",
            "plugin", "atlassian"), [])

    def test_frozen_inventory_uses_published_sha_and_refuses_null_revision(self):
        audit = load_audit()
        doc = json.loads(audit.FIXTURES.read_text())
        audit.validate_fixtures(doc)
        self.assertEqual(len(doc["signed_release_pins"]), 21)
        self.assertTrue(all(
            isinstance(pin["revision"], str) and len(pin["revision"]) == 40
            for pin in doc["signed_release_pins"].values()))
        missing = json.loads(json.dumps(doc))
        del missing["signed_release_pins"]["atlassian"]
        with self.assertRaisesRegex(ValueError, "Missing frozen Registry"):
            audit.validate_fixtures(missing)

        # A published SHA binds a read-only native receipt, but a future
        # unbound review release never becomes acceptance evidence.
        pin = doc["signed_release_pins"]["cloudflare"]
        node = {"name": "cloudflare",
                "source": pin["repository"] + "/" + pin["path"],
                "installation_id": "verified-published",
                "directory": {"product_id": "cloudflare",
                              "recorded_distribution": pin["distribution"],
                              "recorded_revision": pin["revision"],
                              "recorded_release_sequence": pin["sequence"]},
                "clients": [{"client_id": "claude",
                             "verification": "installation_verified",
                             "package_revision": {
                                 "distribution_id": pin["distribution"],
                                 "resolved_revision": pin["revision"],
                                 "release_sequence": pin["sequence"],
                                 "tree_digest": pin["tree_digest"],
                                 "manifest_digest": pin["manifest_digest"]}}]}
        response = {"data": {"installations": [node]}}
        self.assertEqual(len(audit.matching_installations(
            response, "cloudflare", "claude", "plugin", "cloudflare",
            doc["signed_release_pins"])), 1)

        malformed = json.loads(json.dumps(doc))
        malformed["signed_release_pins"]["cloudflare"]["revision"] = None
        with self.assertRaisesRegex(ValueError, "Malformed frozen signed release pin"):
            audit.validate_fixtures(malformed)
        self.assertEqual(audit.matching_installations(
            response, "cloudflare", "claude", "plugin", "cloudflare",
            malformed["signed_release_pins"]), [])


    def test_namespaced_github_bridge_is_a_single_pinned_distribution(self):
        # BUG-038: default product "github" selects per-target eligible releases;
        # pin one real explicitly named distribution for reproducible 30x30 E2E.
        audit = load_audit()
        doc = json.loads(audit.FIXTURES.read_text())
        pin = doc["signed_release_pins"]["777genius/github-bridge"]
        self.assertEqual(doc["plugin_declared_names"]["777genius/github-bridge"], "github")
        self.assertNotIn("github", doc["plugin_aliases"])
        self.assertEqual(doc["plugin_aliases"].count("777genius/github-bridge"), 1)
        revision = {
            "distribution_id": pin["distribution"],
            "resolved_revision": pin["revision"],
            "release_sequence": pin["sequence"],
            "tree_digest": pin["tree_digest"],
            "manifest_digest": pin["manifest_digest"],
        }
        directory = {
            "product_id": "github",
            "recorded_distribution": pin["distribution"],
            "recorded_revision": pin["revision"],
            "recorded_release_sequence": pin["sequence"],
        }
        node = {
            "name": "github", "installation_id": "native-owned-bridge",
            "source": pin["repository"] + "/" + pin["path"],
            "directory": directory,
            "clients": [{"client_id": "opencode", "verification": "installation_verified",
                         "package_revision": revision}],
        }
        def selected():
            return audit.matching_installations(
                {"data": {"installations": [node]}}, "github", "opencode",
                "plugin", "777genius/github-bridge", doc["signed_release_pins"])
        self.assertEqual(len(selected()), 1)
        for key, value in (("product_id", "forged-unrelated-product"),
                            ("recorded_distribution", "github/github"),
                           ("recorded_revision", "0" * 40),
                           ("recorded_release_sequence", 1)):
            with self.subTest(key=key):
                node["directory"] = dict(directory, **{key: value})
                self.assertEqual(selected(), [])
        node["directory"] = directory
        node["clients"][0]["package_revision"] = dict(revision, tree_digest="sha256:" + "0" * 64)
        self.assertEqual(selected(), [])
        node["clients"][0]["package_revision"] = revision
        node["clients"][0]["client_id"] = "codex"
        self.assertEqual(selected(), [])

    def test_direct_git_revision_must_belong_to_verified_client(self):
        audit = load_audit()
        sha = "a" * 40
        item = "github:obra/the-elements-of-style@" + sha
        node = {"name": "elements-of-style", "source": "obra/the-elements-of-style",
                "installation_id": "valid-id", "clients": [
                    {"client_id": "kiro", "verification": "installation_verified",
                     "package_revision": {}},
                    {"client_id": "kiro", "verification": "pending",
                     "package_revision": {"resolved_revision": sha}},
                ]}
        response = {"data": {"installations": [node]}}
        self.assertEqual(audit.matching_installations(
            response, "elements-of-style", "kiro", "plugin", item), [])

    def test_real_signed_release_target_refusal_is_not_an_installer_bug(self):
        audit = load_audit()
        reproduced = (
            'agentplugins: no eligible directory release for "marketing-kit": '
            '777genius/marketing-kit: release does not support complete target set; missing grok'
        )
        self.assertTrue(audit.source_target_incompatibility(reproduced))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: unexpected failure while writing native plugin configuration"))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: lintai security scan failed: exit status 2"))



    def test_pi_manifest_discovery_override_is_source_unsupported(self):
        # BUG-062: exact Pi source-layout safety refusal, not arbitrary errors.
        audit = load_audit()
        self.assertTrue(audit.source_target_incompatibility(
            "agentplugins: pi package manifest overrides selected extension discovery", "pi"))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: pi package manifest overrides selected extension discovery", "codex"))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: pi package manifest is unreadable"))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: unexpected Pi native registry configuration failure"))

    def test_native_snapshot_keeps_symlinks_and_unreadable_paths(self):
        # BUG-017: untrusted native entries must not disappear from evidence.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            home = Path(temporary)
            root = home / ".codex"
            root.mkdir()
            owned = root / "managed.yaml"
            owned.write_text("name: managed\n")
            outside = home / "unrelated.txt"
            outside.write_text("foreign\n")
            (root / "unknown-link").symlink_to(outside)
            (root / "linked-dir").symlink_to(home, target_is_directory=True)
            initial = audit.snapshot([root])
            self.assertEqual(initial[".codex/unknown-link"], "symlink")
            self.assertEqual(initial[".codex/linked-dir"], "symlink")
            self.assertIn(".codex/managed.yaml", initial)
            self.assertNotIn(".codex/linked-dir/unrelated.txt", initial)
            with patch.dict(audit.NATIVE_ARTIFACTS, open_member=Mock(side_effect=OSError("unreadable"))):
                invalid = audit.snapshot([root])
            self.assertEqual(invalid[".codex/managed.yaml"], "unreadable")
            self.assertIn("unknown-link: symlink", " ".join(
                audit.native_identity_residuals(
                    [root], "managed", [".codex/unknown-link"], "codex")))
            self.assertIn("managed.yaml", " ".join(
                audit.native_identity_residuals(
                    [root], "managed", [".codex/managed.yaml"], "codex")))
            (root / "unknown-link").unlink()
            self.assertFalse(audit.native_identity_residuals(
                [root], "managed", [".codex/unknown-link"], "codex"))

    def test_unreadable_walk_marker_cannot_disappear_from_cleanup_evidence(self):
        # BUG-017: os.walk onerror creates a synthetic path; lstat cannot find
        # that marker, but a failed enumeration must not be counted as clean.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".codex"
            root.mkdir()
            reported = audit.native_identity_residuals(
                [root], "atlassian",
                [".codex/restricted-dir/.walk-error"], "codex")
            self.assertEqual(reported, [
                ".codex/restricted-dir/.walk-error: unreadable-walk"])

    def test_foreign_sentinel_symlink_cannot_fake_preservation(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            home = Path(temporary)
            foreign = home / "foreign.txt"
            matching = home / "matching.txt"
            foreign.write_text("keep\n")
            matching.write_text("keep\n")
            self.assertTrue(audit.preserved_regular_text(foreign, "keep\n", home))
            foreign.unlink()
            foreign.symlink_to(matching)
            self.assertFalse(audit.preserved_regular_text(foreign, "keep\n", home))
            foreign.unlink()
            self.assertFalse(audit.preserved_regular_text(foreign, "keep\n", home))

    def test_foreign_sentinel_invalid_utf8_is_tampered_not_harness_error(self):
        # BUG-026: corrupt bytes must not abort the remaining lifecycle proof.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            foreign = case / "foreign.txt"
            foreign.write_bytes(b"\xff\xfe")
            self.assertFalse(audit.preserved_regular_text(foreign, "untouched\n", case))

    def test_copilot_settings_depth_limit_is_reported_as_truncated(self):
        # BUG-026: a sole depth-limited leaf must not be called fully parsed.
        import json
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home = case / "home"
            settings = home / ".copilot/settings.json"
            settings.parent.mkdir(parents=True)
            settings.write_text(json.dumps({"plugins": {"atlassian": "enabled"}}),
                                encoding="utf-8")
            shallow = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(shallow["status"], "parsed")
            self.assertEqual(shallow["match_counts"]["exact_key"], 1)
            nested = {"atlassian": "enabled"}
            for _ in range(32):
                nested = {"nested": nested}
            settings.write_text(json.dumps({"plugins": nested}), encoding="utf-8")
            deep = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(deep["status"], "truncated")
            self.assertEqual(deep["match_counts"]["exact_key"], 0)

    def test_project_skill_intermediate_symlink_cannot_fake_cleanup(self):
        # BUG-023: project/.claude can be swapped even when SKILL.md is regular.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "case"
            project = case / "project"
            foreign = root / "foreign"
            project.mkdir(parents=True)
            foreign_skill_root = foreign / "skills"
            (foreign_skill_root / "foreign-neighbor").mkdir(parents=True)
            (foreign_skill_root / "demo-skill").mkdir()
            marker = foreign_skill_root / "foreign-neighbor/SKILL.md"
            marker.write_text("unmanaged identical sentinel\n")
            (foreign_skill_root / "demo-skill/SKILL.md").write_text(
                "---\nname: demo-skill\n---\n")
            (project / ".claude").symlink_to(foreign, target_is_directory=True)
            spoofed = project / ".claude/skills/foreign-neighbor/SKILL.md"
            self.assertTrue(spoofed.is_file())
            self.assertFalse(audit.preserved_regular_text(
                spoofed, "unmanaged identical sentinel\n", case))
            self.assertFalse(audit.path_inside_case(spoofed, case))
            self.assertIsNone(audit.skill_file_present(project, "claude", "demo-skill"))
            leftovers = audit.skill_removal_leftovers(
                project, "claude", "demo-skill", ".claude/skills/demo-skill/SKILL.md")
            self.assertIn(".claude/skills: unsafe ancestor", leftovers)
            self.assertTrue(any("unsafe ancestor" in entry for entry in leftovers))
            # No helper is permitted to mutate the foreign user's Skill.
            self.assertEqual(marker.read_text(), "unmanaged identical sentinel\n")

    def test_skill_lock_requires_two_complete_sha256_digests(self):
        import json
        audit = load_audit()
        item = {"name": "standalone-skill", "folder": "standalone-skill",
                "repo": "upstream/skills", "ref": "a" * 40}
        package = {"alias": item["name"], "source": {"git": {
            "kind": "skill", "skill": item["name"],
            "repository": item["repo"], "commit": item["ref"],
            "resolved_skill_path": "skills/" + item["folder"]}},
            "targets": ["codex"],
            "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64}
        with TemporaryDirectory() as temporary:
            project = Path(temporary)
            def qualified():
                (project / "uap.lock").write_text(
                    json.dumps({"packages": [package]}))
                return audit.locked_skill_is_pinned(project, item, "codex")
            self.assertTrue(qualified())
            for malformed in ("sha256:", "sha256:bad", "sha256:" + "g" * 64,
                              "sha256:" + "A" * 64, "sha256:" + "0" * 65):
                for field in ("tree_digest", "manifest_digest"):
                    with self.subTest(digest=malformed, field=field):
                        original = package[field]
                        package[field] = malformed
                        self.assertFalse(qualified())
                        package[field] = original

    def test_direct_git_repository_identity_must_be_exact(self):
        audit = load_audit()
        sha = "a" * 40
        item = "github:obra/the-elements-of-style@" + sha
        node = {"name": "elements-of-style",
                "source": "obra/the-elements-of-style",
                "installation_id": "verified",
                "clients": [{"client_id": "cline",
                             "verification": "installation_verified",
                             "package_revision": {"resolved_revision": sha}}]}
        def qualify():
            return audit.matching_installations(
                {"data": {"installations": [node]}},
                "elements-of-style", "cline", "plugin", item)
        self.assertEqual(len(qualify()), 1)
        for valid in ("github:obra/the-elements-of-style",
                      "https://github.com/obra/the-elements-of-style"):
            node["source"] = valid
            self.assertEqual(len(qualify()), 1)
        for spoof in ("mirror/obra/the-elements-of-style-fork",
                      "obra/the-elements-of-style-other",
                      "evil/obra/the-elements-of-style",
                      "obra/the-elements-of-style?unverified=true"):
            with self.subTest(source=spoof):
                node["source"] = spoof
                self.assertEqual(qualify(), [])

    def test_proxy_url_userinfo_is_not_inherited_by_child(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary) / "case"
            home, data = case / "home", case / "uap-data"
            home.mkdir(parents=True)
            data.mkdir()
            with patch.dict(os.environ, {
                "HTTPS_PROXY": "https://alice:secret@proxy.invalid:8443",
                "http_proxy": "http://user:pw@proxy.invalid:3128",
                "ALL_PROXY": "socks5://proxy.invalid:1080",
                "NO_PROXY": "localhost",
                "OPENAI_API_KEY": "never-pass",
            }, clear=True):
                env = audit.isolated_client_env(home, case, data)
            self.assertEqual(env["HTTPS_PROXY"], "https://proxy.invalid:8443")
            self.assertEqual(env["http_proxy"], "http://proxy.invalid:3128")
            self.assertEqual(env["ALL_PROXY"], "socks5://proxy.invalid:1080")
            self.assertEqual(env["NO_PROXY"], "localhost")
            self.assertNotIn("OPENAI_API_KEY", env)


    def test_copilot_native_registry_requires_exact_managed_identity(self):
        # BUG-045: local CLI JSON evidence, not a settings-file string match.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            home = Path(temporary) / "home"
            profile = home / ".copilot"
            profile.mkdir(parents=True)
            native = str(Path(temporary) / "trusted-copilot")
            env = {"HOME": str(home)}
            valid = {"name": "atlassian", "marketplace": "agentplugins-aaaaaaaaaaaa",
                     "version": "0.1.0", "enabled": True, "source": "local"}
            def response(items, exit_code=0):
                return {"exit_code": exit_code, "_json": items}
            with patch.dict(audit.AGENT_BINARIES, {"copilot": native}, clear=True):
                with patch.object(audit, "call", return_value=response([valid])) as executed:
                    self.assertEqual(audit.copilot_native_plugin_state(
                        home, env, home, "atlassian", "0.1.0"), {"status": "installed"})
                    self.assertEqual(executed.call_args.args[0],
                                     [native, "plugin", "list", "--json"])
                    self.assertEqual(executed.call_args.args[1]["COPILOT_HOME"], str(profile))
                with patch.object(audit, "call", return_value=response([])):
                    self.assertEqual(audit.copilot_native_plugin_state(
                        home, env, home, "atlassian")["status"], "absent")
                wrong = [
                    dict(valid, marketplace="foreign"),
                    dict(valid, version="2.0.0"),
                    dict(valid, enabled=False),
                    dict(valid, name="atlassian@agentplugins-aaaaaaaaaaaa"),
                ]
                for row in wrong:
                    with self.subTest(row=row), patch.object(audit, "call", return_value=response([row])):
                        self.assertNotEqual(audit.copilot_native_plugin_state(
                            home, env, home, "atlassian", "0.1.0")["status"], "installed")
                for listing in ([valid, valid], {"plugins": [valid]}, ["malformed"]):
                    with self.subTest(listing=listing), patch.object(audit, "call", return_value=response(listing)):
                        self.assertNotEqual(audit.copilot_native_plugin_state(
                            home, env, home, "atlassian", "0.1.0")["status"], "installed")
                with patch.object(audit, "call", return_value=response([
                        dict(valid, name="renamed-plugin")])):
                    self.assertEqual(audit.copilot_native_plugin_state(
                        home, env, home, "atlassian")["status"],
                        "unqualified-reminted-managed")
                with patch.object(audit, "call", return_value=response([], exit_code=1)):
                    self.assertNotEqual(audit.copilot_native_plugin_state(
                        home, env, home, "atlassian")["status"], "absent")
                with patch.dict(audit.AGENT_BINARIES, {}, clear=True):
                    self.assertEqual(audit.copilot_native_plugin_state(
                        home, env, home, "atlassian")["status"], "unqualified-prerequisites")


    def test_marketing_kit_signed_pins_keep_the_frozen_catalogue_and_reject_drift(self):
        from copy import deepcopy
        audit = load_audit()
        fixtures = json.loads(audit.FIXTURES.read_text())
        audit.validate_fixtures(fixtures)
        original = deepcopy(fixtures)
        supplied = fixtures["signed_release_pins"]
        closure = audit.marketing_kit_signed_pins(supplied)
        self.assertEqual(set(closure), {"marketing-kit", "notion", "hubspot-crm", "playwright"})
        self.assertEqual(closure["hubspot-crm"], {
            "distribution": "777genius/hubspot-crm",
            "repository": "777genius/universal-agent-plugins",
            "path": "plugins/hubspot-crm",
            "revision": "2ddbb99dd190c1792b79904f9875e6322bccd243",
            "sequence": 1,
            "tree_digest": "sha256:09a0b151c384235c3d7b3929398695d24cd7b07c75993a0a73f1a218198ededd",
            "manifest_digest": "sha256:20334e86a1b599d2759e04bfaef37dc2ddb7f6b0f3994b5d66d4eca12fbec941"})
        self.assertEqual(fixtures, original)
        self.assertEqual(len(fixtures["plugin_aliases"]), 30)
        for alias in ("marketing-kit", "notion", "playwright"):
            missing = deepcopy(supplied)
            del missing[alias]
            with self.subTest(alias=alias, missing=True):
                self.assertIsNone(audit.marketing_kit_signed_pins(missing))
        for alias in closure:
            for field, bad in (("distribution", "foreign/package"),
                               ("repository", "foreign/repository"), ("path", "foreign"),
                               ("revision", "f" * 40), ("sequence", True),
                               ("tree_digest", "sha256:" + "f" * 64),
                               ("manifest_digest", "sha256:" + "f" * 64)):
                changed = deepcopy({**supplied, "hubspot-crm": closure["hubspot-crm"]})
                changed[alias][field] = bad
                with self.subTest(alias=alias, field=field):
                    self.assertIsNone(audit.marketing_kit_signed_pins(changed))
        self.assertIsNone(audit.marketing_kit_signed_pins(None))

    def test_marketing_kit_copilot_owner_requires_signed_binding_and_receipts(self):
        audit = load_audit()
        aliases = ("marketing-kit", "notion", "hubspot-crm", "playwright")
        fixture_pins = json.loads(audit.FIXTURES.read_text())["signed_release_pins"]
        self.assertNotIn("hubspot-crm", fixture_pins)
        pins = audit.marketing_kit_signed_pins(fixture_pins)
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(strict=True)
            data = root / "uap-data"
            data.mkdir()
            nodes, records, environment_receipts = [], [], []
            for index, alias in enumerate(aliases):
                pin = pins[alias]
                version = audit.MARKETING_KIT_SIGNED_CLOSURE[pin["distribution"]]["package_version"]
                installation_id, binding_id = f"install-{index}", f"binding-{index}"
                revision = {"distribution_id": pin["distribution"],
                            "resolved_revision": pin["revision"],
                            "release_sequence": pin["sequence"],
                            "tree_digest": pin["tree_digest"],
                            "manifest_digest": pin["manifest_digest"],
                            "version": version}
                nodes.append({"name": alias, "installation_id": installation_id,
                              "source": pin["repository"].strip("/") + "/" + pin["path"].strip("/"),
                              "directory": {"product_id": alias,
                                            "recorded_distribution": pin["distribution"],
                                            "recorded_revision": pin["revision"],
                                            "recorded_release_sequence": pin["sequence"]},
                              "clients": [{"client_id": "copilot",
                                           "verification": "installation_verified",
                                           "package_revision": revision}]})
                physical = f"{alias}-physical"
                target = root / "native" / alias
                marketplace = "agentplugins-" + audit.hashlib.sha256(physical.encode()).hexdigest()[:12]
                manifest_path = target / ".github/plugin/marketplace.json"
                manifest_path.parent.mkdir(parents=True)
                manifest_path.write_text(json.dumps({
                    "name": marketplace, "plugins": [
                        {"name": alias, "version": version, "source": "."}]
                }), encoding="utf-8")
                records.append({
                    "installation_id": installation_id, "origin_mode": "directory",
                    "directory": {"distribution_id": pin["distribution"],
                                  "desired_release_sequence": pin["sequence"]},
                    "package": {"version": version,
                                "manifest_digest": pin["manifest_digest"]},
                    "source": {"repository": pin["repository"], "package_subpath": pin["path"],
                               "resolved_revision": pin["revision"],
                               "tree_digest": pin["tree_digest"]},
                    "clients": {binding_id: {
                        "client_binding_id": binding_id, "client_id": "copilot",
                        "scope": "user", "materialization": "materialized",
                        "verification": "installation_verified",
                        "affected_surfaces": ["copilot", "vscode"],
                        "physical_artifact_id": physical, "target_locator": str(target),
                        "package_revision": dict(revision),
                        "receipts": [{"phase": "committed", "client_binding_id": binding_id,
                                      "active_path": str(target), "operation_id": f"op-{index}"}]}}})
                environment_receipts.append({
                    "receipt_id": f"receipt-{index}", "phase": "committed",
                    "environment_id": "user-default", "alias": alias,
                    "client_id": "copilot", "installation_id": installation_id,
                    "client_binding_id": binding_id,
                    "source_key": "directory:" + pin["distribution"],
                    "package_digest": pin["tree_digest"]})
            response = {"data": {"installations": nodes}}
            state = {"installations": records, "environment_receipts": environment_receipts}
            state_path = data / "state-v2.json"
            def check():
                state_path.write_text(json.dumps(state), encoding="utf-8")
                return audit.copilot_signed_kit_owners(response, fixture_pins, data, root)
            owners = check()
            self.assertIsInstance(owners, dict)
            self.assertEqual(set(owners), set(aliases))
            self.assertEqual({v["version"] for v in owners.values()},
                             {"0.1.0", "0.0.80"})
            old_revision = nodes[1]["clients"][0]["package_revision"]["tree_digest"]
            nodes[1]["clients"][0]["package_revision"]["tree_digest"] = "sha256:" + "f" * 64
            self.assertIsNone(check())
            nodes[1]["clients"][0]["package_revision"]["tree_digest"] = old_revision
            for index, alias in enumerate(aliases):
                record = records[index]
                binding = record["clients"][f"binding-{index}"]
                for revision_owner in (nodes[index]["clients"][0], binding):
                    revision = revision_owner["package_revision"]
                    for field, bad in (("distribution_id", "foreign/package"),
                                       ("release_sequence", 999), ("release_sequence", True),
                                       ("release_sequence", float(pins[alias]["sequence"])),
                                       ("version", "9.9.9"),
                                       ("resolved_revision", "f" * 40),
                                       ("tree_digest", "sha256:" + "f" * 64),
                                       ("manifest_digest", "sha256:" + "f" * 64)):
                        original = revision[field]
                        revision[field] = bad
                        with self.subTest(alias=alias, field=field):
                            self.assertIsNone(check())
                        revision[field] = original
                original = record["directory"]["desired_release_sequence"]
                record["directory"]["desired_release_sequence"] = True
                with self.subTest(alias=alias, field="sequence-bool"):
                    self.assertIsNone(check())
                record["directory"]["desired_release_sequence"] = original
            nodes.append(dict(nodes[0], name="foreign", installation_id="foreign"))
            self.assertIsNone(check())
            nodes.pop()
            nodes[0]["clients"].append(dict(nodes[0]["clients"][0], client_id="vscode"))
            self.assertIsNone(check())
            nodes[0]["clients"].pop()
            nodes[0]["clients"][0]["package_revision"]["release_sequence"] = True
            self.assertIsNone(check())
            nodes[0]["clients"][0]["package_revision"]["release_sequence"] = 1
            self.assertEqual(set(check()), set(aliases))
            replay = records[0]["clients"]["binding-0"]["receipts"][0].copy()
            records[0]["clients"]["binding-0"]["receipts"].append(replay)
            self.assertIsNone(check())
            records[0]["clients"]["binding-0"]["receipts"].pop()
            environment_receipts[2]["source_key"] = "directory:foreign"
            self.assertIsNone(check())
            environment_receipts[2]["source_key"] = (
                "directory:" + pins[aliases[2]]["distribution"])
            self.assertEqual(set(check()), set(aliases))
            records[0]["clients"]["binding-0"]["target_locator"] = str(root / "foreign")
            self.assertIsNone(check())

    def test_marketing_kit_copilot_native_oracle_refuses_reminted_siblings(self):
        audit = load_audit()
        aliases = ("marketing-kit", "notion", "hubspot-crm", "playwright")
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve(strict=True)
            home = case / "home"
            profile = home / ".copilot"
            profile.mkdir(parents=True)
            project, data = case / "project", case / "uap-data"
            project.mkdir()
            data.mkdir()
            env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
            native = str(case / "trusted-copilot")
            owners = {name: {"marketplace": "agentplugins-" + str(index) * 12,
                              "version": "1.0." + str(index),
                              "target_locator": str(case / name) + "/" + "deep-directory-" * 40}
                      for index, name in enumerate(aliases, 1)}
            builtin = {"name": "browser-use", "marketplace": "",
                       "version": "0.0.4", "enabled": False, "source": "builtin"}
            entries = [builtin] + [
                {"name": name, "marketplace": owner["marketplace"],
                 "version": owner["version"], "enabled": True, "source": "live",
                 "installedFrom": owner["target_locator"]}
                for name, owner in owners.items()]
            lines = [audit.COPILOT_BUILTIN_TEXT +
                     "Live Plugins (loaded from a local marketplace directory, never copied):"]
            for name, owner in owners.items():
                lines.extend([f"  • {name}@{owner['marketplace']} (v{owner['version']}) (enabled)",
                              "      from " + owner["target_locator"]])
            complete_text = "\n".join(lines) + "\n"
            self.assertGreater(len(complete_text), 1800)
            empty_text = audit.COPILOT_EMPTY_TEXT + audit.COPILOT_BUILTIN_TEXT

            def probe(rows, text=complete_text, observed_owners=owners, mutation=None):
                calls = []
                def call(argv, call_env, _cwd, timeout, **kwargs):
                    self.assertEqual(timeout, 30)
                    self.assertTrue(kwargs.get("capture_full"))
                    self.assertEqual(call_env["COPILOT_HOME"], str(profile))
                    calls.append(argv)
                    body = json.dumps(rows) if "--json" in argv else text
                    result = {"exit_code": 0, "timeout": False,
                              "stdout_full": body, "stderr_full": "",
                              "stdout_tail": body[-1800:]}
                    if mutation == "tail-only":
                        result.pop("stdout_full")
                    elif mutation == "stderr":
                        result["stderr_full"] = "warning"
                    elif mutation == "timeout":
                        result["timeout"] = True
                    elif mutation == "exit":
                        result["exit_code"] = 1
                    elif mutation == "boolean-exit":
                        result["exit_code"] = False
                    elif mutation == "oversized":
                        result["stdout_full"] = " " * ((1 << 20) + 1)
                    elif mutation == "duplicate-json" and "--json" in argv:
                        result["stdout_full"] = body.replace(
                            '"name": "browser-use"', '"name": "browser-use", "name": "browser-use"', 1)
                    elif mutation == "constant-json" and "--json" in argv:
                        result["stdout_full"] = body.replace('"enabled": false', '"enabled": NaN', 1)
                    return result
                with patch.dict(audit.AGENT_BINARIES, {"copilot": native}, clear=True):
                    with patch.object(audit, "call", side_effect=call):
                        state = audit.copilot_native_plugin_state(
                            home, env, project, "marketing-kit",
                            "1.0.1", owners=observed_owners)
                if state["status"] in ("installed", "absent"):
                    self.assertEqual(calls, [[native, "plugin", "list", "--json"],
                                             [native, "plugin", "list"]])
                return state["status"]

            self.assertEqual(probe(entries), "installed")
            self.assertEqual(probe([builtin], empty_text, {}), "absent")
            self.assertEqual(probe(entries, complete_text.replace("\n", "\r\n")), "installed")
            with patch.dict(audit.AGENT_BINARIES, {"copilot": native}, clear=True):
                with patch.object(audit, "call", return_value={
                        "exit_code": 0, "_json": entries}):
                    self.assertEqual(audit.copilot_native_plugin_state(
                        home, env, project, "marketing-kit",
                        "1.0.1")["status"], "unqualified-reminted-managed")
            for index in range(1, 5):
                for field, bad in (("name", "reminted"), ("marketplace", "agentplugins-" + "f"*12),
                                   ("version", "9.9.9"), ("source", "cache"),
                                   ("installedFrom", str(case / "foreign")),
                                   ("enabled", False), ("enabled", 1), ("enabled", 1.0)):
                    changed = json.loads(json.dumps(entries))
                    changed[index][field] = bad
                    with self.subTest(index=index, field=field, value=bad):
                        self.assertNotEqual(probe(changed), "installed")
            for rows in (entries[:-1], entries + [entries[-1]], entries + ["invalid"],
                         [dict(builtin, version="9.9.9"), *entries[1:]],
                         [dict(builtin, enabled=0), *entries[1:]],
                         [dict(builtin, enabled=0.0), *entries[1:]]):
                with self.subTest(rows=rows):
                    self.assertNotEqual(probe(rows), "installed")
            for changed in (
                    complete_text[-1800:],
                    complete_text.replace("      from ", "      from /foreign/", 1),
                    complete_text.replace("(v1.0.1)", "(v9.9.9)", 1),
                    complete_text + "\n".join(lines[1:3]) + "\n",
                    complete_text + "unexpected output\n",
                    empty_text):
                with self.subTest(text=changed[:80]):
                    self.assertNotEqual(probe(entries, changed), "installed")
            for mutation in ("tail-only", "stderr", "timeout", "exit", "boolean-exit",
                             "oversized", "duplicate-json", "constant-json"):
                with self.subTest(mutation=mutation):
                    self.assertNotEqual(probe(entries, mutation=mutation), "installed")
                    self.assertNotEqual(probe([builtin], empty_text, {}, mutation), "absent")
            for rows, text in ((entries, empty_text), ([builtin], complete_text),
                               ([dict(builtin, enabled=0)], empty_text),
                               ([builtin], empty_text + "unexpected output\n")):
                with self.subTest(removal=True):
                    self.assertNotEqual(probe(rows, text, {}), "absent")

    def test_marketing_kit_copilot_removal_requires_native_absence_without_residual_files(self):
        # A clean disk snapshot must not bypass the independent native observer.
        audit = load_audit()
        identity = "marketing-kit"
        matched = {"name": identity, "installation_id": "owned-kit",
                   "clients": [{"client_id": "copilot",
                                "verification": "installation_verified"}]}
        children = [{"name": name, "installation_id": "owned-" + name,
                     "clients": [{"client_id": "copilot", "verification": "installation_verified"}]}
                    for name in ("notion", "hubspot-crm", "playwright")]
        outcomes = ["absent", "surviving-json", "surviving-text", "bad-text", "failed-list"]
        outcomes.extend("durable:" + child["name"] + ":" + mode
                        for child in children for mode in ("original", "reminted", "list-id", "dict-id"))
        for outcome in outcomes:
            with self.subTest(outcome=outcome), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(strict=True)
                runroot = root / "run"
                native = str(root / "trusted-copilot")
                owners = {name: {"marketplace": "agentplugins-" + str(index) * 12,
                                  "version": audit.MARKETING_KIT_SIGNED_CLOSURE[
                                      audit.MARKETING_KIT_DISTRIBUTIONS[name]]["package_version"],
                                  "target_locator": str(root / "owned" / name)}
                          for index, name in enumerate(
                              ("marketing-kit", "notion", "hubspot-crm", "playwright"), 1)}
                entries = [dict(audit.COPILOT_BUILTIN_JSON[0])] + [
                    {"name": name, "marketplace": owner["marketplace"],
                     "version": owner["version"], "enabled": True, "source": "live",
                     "installedFrom": owner["target_locator"]}
                    for name, owner in owners.items()]
                lines = [audit.COPILOT_BUILTIN_TEXT +
                         "Live Plugins (loaded from a local marketplace directory, never copied):"]
                for name, owner in owners.items():
                    lines.extend([f"  • {name}@{owner['marketplace']} (v{owner['version']}) (enabled)",
                                  "      from " + owner["target_locator"]])
                live_text = "\n".join(lines) + "\n"
                empty_text = audit.COPILOT_EMPTY_TEXT + audit.COPILOT_BUILTIN_TEXT
                state = {"removed": False}
                observed = []
                def call(argv, env, cwd, timeout=105, **kwargs):
                    if argv[0] == native:
                        self.assertTrue(kwargs.get("capture_full"))
                        observed.append((state["removed"], "--json" in argv))
                        rows = audit.COPILOT_BUILTIN_JSON if state["removed"] else entries
                        text = empty_text if state["removed"] else live_text
                        if state["removed"] and outcome == "surviving-json":
                            rows = entries
                        elif state["removed"] and outcome == "surviving-text":
                            text = live_text
                        elif state["removed"] and outcome == "bad-text":
                            text += "unexpected native row\n"
                        body = json.dumps(rows) if "--json" in argv else text
                        return {"exit_code": 1 if state["removed"] and outcome == "failed-list" else 0,
                                "timeout": False, "stdout_full": body, "stderr_full": "",
                                "stdout_tail": body[-1800:], "stderr_tail": ""}
                    managed = Path(env["HOME"]) / ".copilot/marketing-kit.json"
                    if argv[1] == "add" and not managed.exists():
                        managed.write_text('{"name":"marketing-kit"}', encoding="utf-8")
                    elif argv[1] == "remove":
                        managed.unlink()
                        state["removed"] = True
                    remaining = [] if state["removed"] else [matched, *children]
                    if state["removed"] and outcome.startswith("durable:"):
                        _, name, mode = outcome.split(":")
                        child = next(row for row in children if row["name"] == name)
                        surviving_id = {"original": child["installation_id"],
                                        "reminted": "reminted-" + name,
                                        "list-id": [], "dict-id": {}}[mode]
                        remaining = [dict(child, installation_id=surviving_id)]
                    return {"exit_code": 0, "timeout": False, "stdout_tail": "", "stderr_tail": "",
                            "_json": {"data": {"installations": remaining}}}
                with (patch.dict(audit.AGENT_BINARIES, {"copilot": native}, clear=True),
                      patch.object(audit, "call", side_effect=call),
                      patch.object(audit, "matching_installations", return_value=[matched]),
                      patch.object(audit, "copilot_signed_kit_owners", return_value=owners),
                      patch.object(audit, "copilot_signed_kit_settings", return_value=True)):
                    result = audit.do_case("/unused/agentplugins", "copilot", "plugin",
                                           identity, 0, runroot, {}, {})
                self.assertTrue(result["listed_identity"])
                self.assertTrue(result["native_identity_files"])
                self.assertTrue(result["repeat_verified"])
                self.assertTrue(result["sentinels_preserved"])
                self.assertEqual(result["post_list_schema_valid"],
                                 not outcome.endswith((":list-id", ":dict-id")))
                self.assertFalse(result["post_remove_identity_registered"])
                self.assertEqual(result["native_residuals"], [])
                self.assertEqual(result["cleanup"]["exit_code"], 0)
                self.assertEqual(result["copilot_native_after_repeat"]["status"], "installed")
                self.assertEqual(observed[:2], [(False, True), (False, False)])
                self.assertIn((True, True), observed)
                if outcome == "absent":
                    self.assertEqual(observed, [(False, True), (False, False),
                                                (True, True), (True, False)])
                    self.assertTrue(result["removal_verified"])
                    self.assertEqual(result["status"], "passed")
                else:
                    self.assertFalse(result["removal_verified"])
                    self.assertEqual(result["status"], "cleanup-failed")
                    if outcome.startswith("durable:"):
                        self.assertEqual(result["copilot_native_after_remove"]["status"], "absent")
                        self.assertEqual(observed, [(False, True), (False, False),
                                                    (True, True), (True, False)])

    def test_marketing_kit_adjunct_native_read_preserves_all_owned_state(self):
        # Execute the actual adjunct JSON/TEXT path. Containment alone must
        # not accept mutations of unrelated files, metadata or excess logs.
        audit = load_audit()
        for mutation in ("none", "two-new-logs", "home", "data", "project",
                         "mtime-only", "replacement", "excess-logs",
                         "failed-query", "call-error", "project-replaced"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(strict=True)
                home, data, project = root / "home", root / "uap-data", root / "project"
                for directory in (home, data, project):
                    directory.mkdir()
                    (directory / "foreign.txt").write_text("keep exact", encoding="utf-8")
                logs = home / ".copilot/logs"
                logs.mkdir(parents=True)
                (logs / "process-100-100.log").write_text("keep old log", encoding="utf-8")
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                native = str(root / "trusted-copilot")
                calls = []
                def call(argv, *_args, **kwargs):
                    self.assertTrue(kwargs.get("capture_full"))
                    calls.append(argv)
                    if mutation == "two-new-logs":
                        (logs / f"process-200-{len(calls)}.log").write_text("new process log")
                    if len(calls) == 1:
                        if mutation in ("home", "data", "project"):
                            target = {"home": home, "data": data, "project": project}[mutation]
                            (target / "foreign.txt").write_text("changed")
                        elif mutation == "mtime-only":
                            target = home / "foreign.txt"
                            old = target.stat()
                            os.utime(target, ns=(old.st_atime_ns, old.st_mtime_ns + 1_000_000_000))
                        elif mutation == "replacement":
                            target = home / "foreign.txt"
                            target.rename(home / "saved-original.txt")
                            target.write_text("keep exact")
                        elif mutation == "excess-logs":
                            for index in range(3):
                                (logs / f"process-300-{index}.log").write_text("extra process log")
                        elif mutation in ("failed-query", "call-error"):
                            (data / "foreign.txt").write_text("changed")
                        elif mutation == "project-replaced":
                            project.rename(root / "saved-project")
                            project.write_text("not a directory")
                    if mutation == "call-error":
                        raise ValueError("native process failed after changing state")
                    body = (json.dumps(audit.COPILOT_BUILTIN_JSON) if "--json" in argv
                            else audit.COPILOT_EMPTY_TEXT + audit.COPILOT_BUILTIN_TEXT)
                    return {"exit_code": 1 if mutation == "failed-query" else 0,
                            "timeout": False, "stdout_full": body, "stderr_full": ""}
                with patch.dict(audit.AGENT_BINARIES, {"copilot": native}, clear=True), \
                        patch.object(audit, "call", side_effect=call):
                    if mutation in ("none", "two-new-logs"):
                        self.assertEqual(audit.copilot_native_plugin_state(
                            home, env, project, "marketing-kit", owners={})["status"], "absent")
                        self.assertEqual(len(calls), 2)
                    else:
                        with self.assertRaisesRegex(
                                AssertionError, "modified protected state|excessive new|redirected|not a directory"):
                            audit.copilot_native_plugin_state(
                                home, env, project, "marketing-kit", owners={})
                        if mutation in ("failed-query", "call-error", "project-replaced"):
                            self.assertEqual(len(calls), 1)

    def test_marketing_kit_copilot_native_inventory_queries_are_read_only(self):
        """Only two bounded new process logs may accompany a native inventory read."""
        audit = load_audit()
        for mutation in ("bounded-log", "rewrite", "excess-logs"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(strict=True)
                home = root / "home"
                profile = home / ".copilot"
                (profile / "logs").mkdir(parents=True)
                settings = profile / "settings.json"
                settings.write_text('{"foreign":true}', encoding="utf-8")
                project, data = root / "project", root / "uap-data"
                project.mkdir()
                data.mkdir()
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                native = str(root / "trusted-copilot")

                def observe(argv, _env, _cwd, _timeout, **kwargs):
                    self.assertTrue(kwargs.get("capture_full"))
                    if "--json" in argv:
                        if mutation == "rewrite":
                            settings.write_text('{"foreign":false}', encoding="utf-8")
                        else:
                            count = 1 if mutation == "bounded-log" else 3
                            for index in range(count):
                                (profile / "logs" / f"process-123-{index}.log").write_text(
                                    "native process log", encoding="utf-8")
                    body = (json.dumps(audit.COPILOT_BUILTIN_JSON) if "--json" in argv else
                            audit.COPILOT_EMPTY_TEXT + audit.COPILOT_BUILTIN_TEXT)
                    return {"exit_code": 0, "timeout": False, "stderr_full": "",
                            "stdout_full": body}

                with patch.dict(audit.AGENT_BINARIES, {"copilot": native}, clear=True):
                    with patch.object(audit, "call", side_effect=observe):
                        if mutation == "bounded-log":
                            result = audit.copilot_native_plugin_state(
                                home, env, project, "marketing-kit", owners={})
                            self.assertEqual(result["status"], "absent")
                        else:
                            with self.assertRaisesRegex(
                                    AssertionError, "modified protected state|excessive new"):
                                audit.copilot_native_plugin_state(
                                    home, env, project, "marketing-kit", owners={})

    def test_marketing_kit_copilot_native_guard_failure_stops_before_text(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(strict=True)
            home = root / "home"
            (home / ".copilot").mkdir(parents=True)
            project, data = root / "project", root / "uap-data"
            project.mkdir()
            data.mkdir()
            env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
            native = str(root / "trusted-copilot")
            calls = []
            def guarded_call(argv, *_args, **_kwargs):
                calls.append(argv)
                raise ValueError("audit case root changed after external execution")
            with patch.dict(audit.AGENT_BINARIES, {"copilot": native}, clear=True):
                with patch.object(audit, "call", side_effect=AssertionError("unguarded query")):
                    with self.assertRaisesRegex(ValueError, "case root changed"):
                        audit.copilot_native_plugin_state(
                            home, env, project, "marketing-kit",
                            owners={}, owned_call=guarded_call)
            self.assertEqual(calls, [[native, "plugin", "list", "--json"]])

    def test_marketing_kit_copilot_requires_four_exact_preference_keys(self):
        audit = load_audit()
        aliases = ("marketing-kit", "notion", "hubspot-crm", "playwright")
        owners = {name: {"marketplace": "agentplugins-" + str(index) * 12,
                         "target_locator": "/owned/" + name}
                  for index, name in enumerate(aliases, 1)}
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(strict=True)
            home = root / "home"
            (home / ".copilot").mkdir(parents=True)
            path = home / ".copilot/settings.json"
            expected = {name + "@" + value["marketplace"]: True
                        for name, value in owners.items()}
            path.write_text(json.dumps({"enabledPlugins": expected}), encoding="utf-8")
            self.assertTrue(audit.copilot_signed_kit_settings(home, root, owners, True))
            self.assertFalse(audit.copilot_signed_kit_settings(home, root, owners, False))
            marketplaces = {owner["marketplace"]: {
                "source": {"source": "directory", "path": owner["target_locator"]}}
                for owner in owners.values()}
            for active_markets in ({}, marketplaces, dict(list(marketplaces.items())[:1])):
                path.write_text(json.dumps({"enabledPlugins": expected,
                                            "extraKnownMarketplaces": active_markets}), encoding="utf-8")
                self.assertTrue(audit.copilot_signed_kit_settings(home, root, owners, True))
            known = next(iter(marketplaces))
            for bad in (
                    {**marketplaces, "foreign": marketplaces[known]},
                    {known: {"source": {"source": "directory", "path": "/foreign"}}},
                    {known: {"source": {"source": "directory", "path": "relative"}}},
                    {known: {"source": {"source": "github", "path": "/owned/marketing-kit"}}},
                    {known: {"source": {**marketplaces[known]["source"], "extra": True}}},
                    {known: None}, [], None):
                path.write_text(json.dumps({"enabledPlugins": expected,
                                            "extraKnownMarketplaces": bad}), encoding="utf-8")
                with self.subTest(marketplaces=bad):
                    self.assertFalse(audit.copilot_signed_kit_settings(home, root, owners, True))
            path.write_text(json.dumps({"enabledPlugins": dict.fromkeys(expected, False),
                                        "extraKnownMarketplaces": marketplaces}), encoding="utf-8")
            self.assertFalse(audit.copilot_signed_kit_settings(home, root, owners, False))
            for changed in (
                dict(list(expected.items())[:-1]),
                {**expected, "foreign@agentplugins-" + "f"*12: True},
                {**expected, next(iter(expected)): False}):
                path.write_text(json.dumps({"enabledPlugins": changed}), encoding="utf-8")
                self.assertFalse(audit.copilot_signed_kit_settings(home, root, owners, True))
            for enabled in (True, False):
                baseline = dict.fromkeys(expected, enabled)
                for key in expected:
                    for bad in (0, 1, 0.0, 1.0, "true", "false", None):
                        changed = {**baseline, key: bad}
                        path.write_text(json.dumps({"enabledPlugins": changed}), encoding="utf-8")
                        with self.subTest(enabled=enabled, key=key, bad=bad):
                            self.assertFalse(audit.copilot_signed_kit_settings(
                                home, root, owners, enabled))
                path.write_text(json.dumps({"enabledPlugins": baseline}), encoding="utf-8")
                self.assertTrue(audit.copilot_signed_kit_settings(home, root, owners, enabled))
                self.assertFalse(audit.copilot_signed_kit_settings(
                    home, root, owners, int(enabled)))
            path.write_text(json.dumps({"enabledPlugins": dict.fromkeys(expected, False),
                                        "extraKnownMarketplaces": {}}), encoding="utf-8")
            self.assertTrue(audit.copilot_signed_kit_settings(home, root, owners, False))
            path.write_text(json.dumps({"enabledPlugins": dict.fromkeys(expected, False),
                                        "extraKnownMarketplaces": {"foreign": "/tmp"}}), encoding="utf-8")
            self.assertFalse(audit.copilot_signed_kit_settings(home, root, owners, False))

    def test_copilot_disabled_setting_needs_native_absence_and_exact_owned_shape(self):
        audit = load_audit()
        before = {"status": "absent"}
        after_add = {"status": "parsed",
                     "enabled_plugin_value_counts": {"true": 1, "false": 0, "non_boolean": 0}}
        after_remove = {
            "status": "parsed", "allowed_sections": ["enabledPlugins"],
            "enabled_plugin_value_counts": {"true": 0, "false": 1, "non_boolean": 0},
            "match_counts": {"exact_key": 0, "qualified_key": 1, "reference_key": 0,
                             "exact_value": 0, "qualified_value": 0, "reference_value": 0},
        }
        installed, absent = {"status": "installed"}, {"status": "absent"}
        residuals = [".copilot/settings.json"]
        def qualified(*args):
            return audit.copilot_disabled_preference_only(*args)
        self.assertTrue(qualified(before, after_add, after_remove,
                                  installed, absent, residuals))
        for native_after, left in ((installed, residuals),
                                   (absent, [".copilot/settings.json", ".copilot/other.json"]),
                                   ({"status": "unqualified-list"}, residuals)):
            self.assertFalse(qualified(before, after_add, after_remove,
                                       installed, native_after, left))
        for key, value in (("true", 1), ("false", 0), ("non_boolean", 1)):
            bad = json.loads(json.dumps(after_remove))
            bad["enabled_plugin_value_counts"][key] = value
            self.assertFalse(qualified(before, after_add, bad,
                                       installed, absent, residuals))
        for key, value in (("qualified_key", 2), ("reference_value", 1)):
            bad = json.loads(json.dumps(after_remove))
            bad["match_counts"][key] = value
            self.assertFalse(qualified(before, after_add, bad,
                                       installed, absent, residuals))
        self.assertFalse(qualified({"status": "parsed"}, after_add, after_remove,
                                   installed, absent, residuals))


    def test_qoder_native_cli_proves_owned_registration_and_removal(self):
        # BUG-039: exact native local ID+version only, never cache evidence.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            home = Path(temporary) / "home"
            profile = home / ".qoder"
            profile.mkdir(parents=True)
            # macOS /var may resolve to /private/var; the production probe
            # requires the same canonical directory as isolated_client_env.
            env = {"QODER_CONFIG_DIR": str(profile.resolve(strict=True))}
            native = str(Path(temporary) / "trusted-qodercli")
            def probe(rows, code=0):
                return {"exit_code": code, "_json": rows}
            with patch.dict(audit.AGENT_BINARIES, {"qodercli": native}, clear=True):
                with patch.object(audit, "call", return_value=probe([
                    {"id": "atlassian@local", "version": "0.1.0"}])) as called:
                    self.assertEqual(audit.qoder_native_plugin_state(
                        home, env, home, "atlassian", "0.1.0"), {"status": "registered"})
                    self.assertEqual(called.call_args.args[0],
                                     [native, "--config-dir", str(profile), "plugins", "list", "--json"])
                    self.assertEqual(called.call_args.args[1], env)
                with patch.object(audit, "call", return_value=probe([])):
                    self.assertEqual(audit.qoder_native_plugin_state(
                        home, env, home, "atlassian")["status"], "absent")
                for rows in [
                    [{"id": "atlassian@local"}],
                    [{"id": "atlassian@local", "version": "0.2.0"}],
                    [{"id": "atlassian@foreign", "version": "0.1.0"}],
                    [{"id": "atlassian@local", "version": "0.1.0"}] * 2,
                    [{"id": "atlassian@local", "version": 1}],
                    [{"id": "atlassian@local@extra", "version": "0.1.0"}],
                    [{"id": "other@local", "version": "1.0.0"}, "malformed"],
                ]:
                    with self.subTest(rows=rows), patch.object(audit, "call", return_value=probe(rows)):
                        self.assertNotEqual(audit.qoder_native_plugin_state(
                            home, env, home, "atlassian", "0.1.0")["status"], "registered")
                with patch.object(audit, "call", return_value=probe([], code=1)):
                    self.assertNotEqual(audit.qoder_native_plugin_state(
                        home, env, home, "atlassian")["status"], "absent")
                with patch.dict(audit.AGENT_BINARIES, {}, clear=True):
                    self.assertEqual(audit.qoder_native_plugin_state(
                        home, env, home, "atlassian")["status"], "unqualified-prerequisites")
                self.assertEqual(audit.qoder_native_plugin_state(
                    home, {"QODER_CONFIG_DIR": str(home / "foreign")},
                    home, "atlassian")["status"], "unqualified-prerequisites")
                foreign = Path(temporary) / "foreign"
                foreign.mkdir()
                profile.rmdir()
                try:
                    profile.symlink_to(foreign, target_is_directory=True)
                except (OSError, NotImplementedError):
                    return
                self.assertEqual(audit.qoder_native_plugin_state(
                    home, env, home, "atlassian")["status"], "unqualified-prerequisites")


    def test_native_residual_uses_exact_tokens_not_url_substrings(self):
        # BUG-042: a native URL hostname is not the managed GitHub plugin.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".copilot"
            root.mkdir()
            settings = root / "settings.json"
            observed = [".copilot/settings.json"]
            settings.write_text('{"homepage": "https://github.com/other/repo"}')
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"), [])
            # A foreign marketplace plugin is also not UAP's managed namespace.
            settings.write_text('{"enabledPlugins":{"github@foreign":true}}')
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"), [])
            settings.write_text(
                '{"enabledPlugins":{"github@agentplugins-aaaaaaaaaaaa":false}}')
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"), observed)
            settings.write_text('{"plugins":{"github":{"enabled":true}}}')
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"), observed)
            settings.write_text("unrelated" * 150000)
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"),
                [observed[0] + ": unscanned-too-large"])
            settings.unlink()
            try:
                settings.symlink_to(root / "missing")
            except (OSError, NotImplementedError):
                return
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"),
                [observed[0] + ": symlink"])

    def test_invalid_utf8_skill_name_still_visible_to_readonly_oracle(self):
        # BUG-043: malformed source bytes are not rewritten or omitted.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            project = Path(temporary).resolve() / "project"
            target = project / ".claude/skills/raw"
            target.mkdir(parents=True)
            skill = target / "SKILL.md"
            source = b"\xff\xfe---\nname: raw\n---\n"
            skill.write_bytes(source)
            self.assertEqual(audit.skill_file_present(project, "claude", "raw"),
                             ".claude/skills/raw/SKILL.md")
            self.assertIsNone(audit.skill_file_present(project, "claude", "foreign"))
            self.assertEqual(skill.read_bytes(), source)

    def test_qoder_passive_cache_does_not_hide_active_residue_or_symlinks(self):
        # BUG-018: observed Qoder cache and run manifests outlive native remove.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".qoder"
            cache = root / "plugins/cache/local/atlassian/0.1.0/plugin.json"
            log = root / "logs/runs/run-1/manifest.json"
            active = root / "plugins/installed_plugins_v2.json"
            settings = root / "settings.json"
            for path in (cache, log, active, settings):
                path.parent.mkdir(parents=True, exist_ok=True)
            cache.write_text('{"name":"atlassian"}')
            log.write_text('{"plugin":"atlassian"}')
            active.write_text('{"plugin":"atlassian"}')
            settings.write_text('{"mcpServers":{"atlassian":{}}}')
            observed = [
                ".qoder/plugins/cache/local/atlassian/0.1.0/plugin.json",
                ".qoder/logs/runs/run-1/manifest.json",
                ".qoder/plugins/installed_plugins_v2.json",
                ".qoder/settings.json",
            ]
            self.assertEqual(
                audit.native_identity_residuals([root], "atlassian", observed, "qoder"),
                [".qoder/plugins/installed_plugins_v2.json", ".qoder/settings.json"])
            active.write_text("{}")
            settings.write_text("{}")
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"), [])
            cache.unlink()
            cache.symlink_to(settings)
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"),
                [".qoder/plugins/cache/local/atlassian/0.1.0/plugin.json: symlink"])
            self.assertFalse(audit.passive_native_path("cline", "plugins/cache/local/demo"))
            self.assertFalse(audit.passive_native_path("qoder", "settings.json"))


    def test_post_remove_native_rescan_detects_late_registration(self):
        # BUG-020: a native setting written by remove after the add snapshot
        # must not be mistaken for a successful full lifecycle.
        audit = load_audit()
        identity = "atlassian"
        source = "github:example/native-plugin@" + ("a" * 40)
        matched = [{"name": identity, "installation_id": "owned-once",
                    "clients": [{"client_id": "grok", "verification": "installation_verified"}]}]

        def verify_case(late_registration):
            with TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                native = root / "plugin-00/home/.grok"
                managed = native / "managed-plugin.json"
                late = native / "new-settings.json"
                state = {"installed": False, "add_count": 0}

                def fake_call(argv, env, cwd, timeout=105):
                    operation = argv[1]
                    if operation == "add":
                        state["add_count"] += 1
                        if state["add_count"] == 1:
                            managed.write_text('{"name":"atlassian"}')
                        state["installed"] = True
                    elif operation == "remove":
                        managed.unlink()
                        if late_registration:
                            late.write_text('{"nativePlugin":"atlassian"}')
                        state["installed"] = False
                    installations = matched if state["installed"] else []
                    return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                            "timeout": False,
                            "_json": {"data": {"installations": installations}}}

                with patch.object(audit, "call", side_effect=fake_call), \
                     patch.object(audit, "matching_installations", return_value=matched):
                    evidence = audit.do_case("/unused/agentplugins", "grok", "plugin",
                                             source, 0, root, {source: identity})
                self.assertTrue(evidence["listed_identity"])
                self.assertTrue(evidence["repeat_verified"])
                self.assertTrue(evidence["sentinels_preserved"])
                self.assertTrue(evidence["post_list_schema_valid"])
                self.assertEqual(evidence["cleanup"]["exit_code"], 0)
                self.assertIn(".grok/managed-plugin.json", evidence["native_identity_files"])
                return evidence

        clean = verify_case(False)
        self.assertEqual(clean["status"], "passed")
        self.assertTrue(clean["removal_verified"])
        self.assertEqual(clean["native_residuals"], [])

        delayed = verify_case(True)
        self.assertEqual(delayed["status"], "cleanup-failed")
        self.assertFalse(delayed["removal_verified"])
        self.assertEqual(delayed["native_residuals"], [".grok/new-settings.json"])



    def test_post_remove_reminted_identity_under_new_id_fails_cleanup(self):
        # BUG-022: disjoint old IDs do not prove this package/target is gone.
        audit = load_audit()
        source = "github:example/native-plugin@" + ("a" * 40)
        old = {"name": "atlassian", "installation_id": "owned-old",
               "clients": [{"client_id": "grok",
                            "verification": "installation_verified"}]}
        reminted = {"name": "atlassian", "installation_id": "owned-new",
                    "clients": [{"client_id": "grok",
                                 "verification": "installation_verified"}]}
        with TemporaryDirectory() as temporary:
            runroot = Path(temporary).resolve()
            native = runroot / "plugin-00/home/.grok/managed-plugin.json"
            state = {"removed": False}

            def fake_call(argv, env, cwd, timeout=105):
                command = argv[1]
                if command == "add" and not native.exists():
                    native.write_text('{"name":"atlassian"}')
                elif command == "remove":
                    native.unlink()
                    state["removed"] = True
                entries = [reminted] if state["removed"] else [old]
                return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                        "timeout": False,
                        "_json": {"data": {"installations": entries}}}

            with patch.object(audit, "call", side_effect=fake_call):
                with patch.object(audit, "matching_installations", return_value=[old]):
                    result = audit.do_case(
                        "/unused/agentplugins", "grok", "plugin", source, 0,
                        runroot, {source: "atlassian"})
        self.assertTrue(result["listed_identity"])
        self.assertTrue(result["repeat_verified"])
        self.assertTrue(result["sentinels_preserved"])
        self.assertEqual(result["cleanup"]["exit_code"], 0)
        self.assertTrue(result["post_list_schema_valid"])
        self.assertEqual(result["native_residuals"], [])
        self.assertEqual(result["matched_installation_ids"], ["owned-old"])
        self.assertTrue(result["post_remove_identity_registered"])
        self.assertFalse(result["removal_verified"])
        self.assertEqual(result["status"], "cleanup-failed")


    def test_codex_native_registration_requires_exact_local_marketplace(self):
        # The actual Codex CLI owns [plugins."name@agentplugins-<hash>"].
        # A source cache, arbitrary URL or unrelated marketplace is not proof.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            root = case / "home/.codex"
            root.mkdir(parents=True)
            managed = case / "uap-data/managed"
            managed.mkdir(parents=True)
            config = root / "config.toml"
            key = "agentplugins-" + "a" * 12
            config.write_text(
                f'[plugins."atlassian@{key}"]\nenabled = true\n'
                f'[marketplaces.{key}]\nsource_type = "local"\n'
                f'source = "{managed}"\n')
            changes = list(audit.snapshot([root]).items())
            self.assertEqual(
                audit.attributable_native_changes([root], changes, "atlassian", "codex"),
                [".codex/config.toml"])
            self.assertEqual(
                audit.attributable_native_changes([root], changes, "github", "codex"),
                [])
            self.assertFalse(audit.codex_owned_native_config(
                config, "atlassian-wrong", case))
            escaped = Path(temporary).parent / "other-account"
            config.write_text(config.read_text().replace(str(managed), str(escaped)))
            self.assertFalse(audit.codex_owned_native_config(config, "atlassian", case))
            config.write_text(
                config.read_text().replace(str(escaped), str(managed))
                .replace(f"agentplugins-{'a' * 12}", "random-market"))
            self.assertFalse(audit.codex_owned_native_config(config, "atlassian", case))

    def test_large_native_file_rewrite_changes_snapshot(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".grok"
            root.mkdir()
            native = root / "managed.json"
            native.write_bytes(b"A" * (5 * 1024 * 1024))
            before = audit.snapshot([root])
            native.write_bytes(b"B" * (5 * 1024 * 1024))
            after = audit.snapshot([root])
            self.assertNotEqual(before, after)
            self.assertEqual(len(before[".grok/managed.json"]), 64)
            self.assertNotIn("too-large", after.values())

    def test_native_attribution_excludes_cache_and_unrelated_url(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".qoder"
            cache = root / "plugins/cache/local/github/app.json"
            url = root / "settings.json"
            native = root / "plugins/installed_plugins_v2.json"
            for f in (cache, url, native):
                f.parent.mkdir(parents=True, exist_ok=True)
            cache.write_text('{"name":"github"}')
            url.write_text('{"repository":"https://github.com/other/repo"}')
            native.write_text('{"plugins":{"github":{"enabled":true}}}')
            files = audit.snapshot([root])
            changes = list(files.items())
            credited = audit.attributable_native_changes([root], changes, "github", "qoder")
            self.assertEqual(credited, [".qoder/plugins/installed_plugins_v2.json"])
            self.assertFalse(audit.exact_native_identity(
                'source = "https://github.com/example/package"', "github"))
            self.assertTrue(audit.exact_native_identity(
                'name = "github"', "github"))

    def test_native_and_temp_symlink_escape_rejected_before_untrusted_writes(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "case"
            home = case / "home"
            data = case / "uap-data"
            foreign = root / "foreign"
            foreign.mkdir()
            home.mkdir(parents=True)
            data.mkdir()
            (home / ".codex").symlink_to(foreign, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "native profile"):
                audit.isolated_client_env(home, case, data)
            (home / ".codex").unlink()
            (case / "tmp").rmdir()
            (case / "tmp").symlink_to(foreign, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "temp root"):
                audit.isolated_client_env(home, case, data)

            future = root / "run" / "plugin-00"
            future.parent.mkdir()
            future.symlink_to(foreign, target_is_directory=True)
            # The explicit preflight must reject redirected case roots before
            # mkdir or any third-party command can traverse the link.
            with self.assertRaisesRegex(ValueError, "audit case root is redirected"):
                audit.do_case("/unused/audit-cli", "codex", "plugin",
                              "atlassian", 0, future.parent, {})
            self.assertEqual(list(foreign.iterdir()), [])

    def test_skill_removal_rejects_surviving_links_and_native_registration(self):
        from types import SimpleNamespace
        audit = load_audit()
        identity = "demo-skill"
        item = {"name": identity, "folder": identity, "repo": "example/skills",
                "ref": "a" * 40, "source": "https://github.com/example/skills",
                "skill_md_git_blob_sha1": "b" * 40}
        match = [{"name": identity, "installation_id": "skill-1",
                  "clients": [{"client_id": "claude",
                               "verification": "installation_verified"}]}]

        def check(residue, selected_target="claude", readiness=("ready", "ready"),
                  actions=(None, None)):
            with TemporaryDirectory() as temporary:
                runroot = Path(temporary).resolve()
                project = runroot / "skill-00/project"
                target = "codex" if residue == "post-native-recreated" else selected_target
                native = runroot / "skill-00/home" / ("." + target)
                skill = project / audit.PROJECT_SKILL_ROOTS[target][0] / "demo-skill/SKILL.md"
                selected_match = [dict(match[0], clients=[
                    {"client_id": target, "verification": "installation_verified"}])]
                state, phases = {"installed": False}, []
                def native_checkpoint(phase, present):
                    phases.append(phase)
                    if phase == "removed" and residue == "post-native-recreated":
                        skill.parent.mkdir(parents=True, exist_ok=True)
                        skill.write_text("---\nname: demo-skill\n---\n")
                observer = SimpleNamespace(
                    guard=lambda: None, checkpoint=native_checkpoint,
                    temporary_git_state=lambda: ({}, {}),
                    repeat_runtime_only=lambda *args: False,
                    evidence=lambda: {"phases": list(phases)})
                def fake_call(argv, env, cwd, timeout=105):
                    if argv[1] == "skills":
                        state["add_count"] = state.get("add_count", 0) + 1
                        skill.parent.mkdir(parents=True, exist_ok=True)
                        skill.write_text("---\nname: demo-skill\n---\n")
                        (project / "uap.toml").write_text("demo-skill\n")
                        (project / "uap.lock").write_text('{"name":"demo-skill"}')
                        state["installed"] = True
                    elif argv[1] == "remove":
                        skill.unlink()
                        skill.parent.rmdir()
                        (project / "uap.lock").unlink()
                        state["installed"] = False
                        if residue == "symlink":
                            skill.parent.mkdir()
                            skill.symlink_to(project / "unmanaged")
                        elif residue == "native":
                            (native / "agent-registry.json").write_text(
                                '{"skills":{"demo-skill":{}}}')
                    elif argv[1] == "list" and not state["installed"]:
                        state["post_list_empty"] = True
                        if residue == "post-list-recreated":
                            skill.parent.mkdir(parents=True, exist_ok=True)
                            skill.write_text("---\nname: demo-skill\n---\n")
                        elif residue == "post-list-foreign":
                            (runroot / "skill-00/home/foreign-untouched.txt").write_text("changed by list")
                    result = {"data": {"installations": selected_match if state["installed"] else []}}
                    if argv[1] == "skills":
                        result = {"schema_version": 1, "command": "skills add", "result": "success",
                                  "status": "succeeded", "scope": "project", "items": [{
                                      "alias": identity, "targetId": target, "status": "succeeded"}]}
                        index = state["add_count"] - 1
                        if readiness[index] is not None:
                            result["readiness"] = readiness[index]
                        if actions[index] is not None:
                            result["next_actions"] = actions[index]
                    return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                            "timeout": False, "_json": result}
                with patch.object(audit, "call", side_effect=fake_call), \
                     patch.object(audit, "matching_installations", return_value=selected_match), \
                     patch.object(audit, "locked_skill_is_pinned", return_value=True), \
                     patch.object(audit, "matches_pinned_skill_bytes", return_value=True), \
                     patch.dict(audit.AGENT_BINARIES, {"codex": "/inert/codex"}, clear=True), \
                     patch.dict(audit.CODEX_RUNTIME_AUDIT, {
                         "CodexRuntimeAudit": lambda *args: observer,
                         "public_skill_ready": lambda *args: True}):
                    evidence = audit.do_case("/unused/agentplugins", target, "skill",
                                             item, 0, runroot, {})
                self.assertTrue(evidence["installed"] if "installed" in evidence
                                else evidence["listed_identity"])
                self.assertTrue(evidence["repeat_verified"])
                self.assertTrue(state.get("post_list_empty"))
                self.assertTrue(evidence["post_list_schema_valid"])
                self.assertFalse(evidence["post_remove_identity_registered"])
                return evidence
        clean = check(None)
        self.assertTrue(clean["removal_verified"])
        self.assertEqual(clean["status"], "passed")
        self.assertTrue(clean["sentinels_preserved"])
        self.assertIsNone(clean["post_removal_skill"])
        self.assertEqual(clean["post_removal_leftovers"], [])
        self.assertEqual(clean["post_skill_tree_delta"], [])
        linked = check("symlink")
        self.assertFalse(linked["removal_verified"])
        self.assertEqual(linked["status"], "cleanup-failed")
        self.assertTrue(linked["post_removal_leftovers"])
        native = check("native")
        self.assertFalse(native["removal_verified"])
        self.assertEqual(native["status"], "cleanup-failed")
        self.assertTrue(native["native_residuals"])


        for mutation in ("post-list-recreated", "post-native-recreated"):
            with self.subTest(mutation=mutation):
                recreated = check(mutation)
                self.assertFalse(recreated["removal_verified"])
                self.assertEqual(recreated["status"], "cleanup-failed")
                self.assertTrue(recreated["post_removal_skill"])
                self.assertTrue(recreated["post_removal_leftovers"])
                self.assertTrue(recreated["post_skill_tree_delta"])
                if mutation == "post-native-recreated":
                    self.assertIn("removed", recreated["codex_runtime_proof"]["phases"])
        foreign = check("post-list-foreign")
        self.assertFalse(foreign["sentinels_preserved"])
        self.assertEqual(foreign["status"], "cleanup-failed")

        # Actual catalog runs for these clients had clean file lifecycles but
        # public needs_action. Preserve the cleanup proof without native PASS.
        for target in ("cursor", "kiro", "windsurf", "claude"):
            for readiness in (("needs_action", "needs_action"), ("ready", "needs_action"),
                              ("needs_action", "ready"), (None, "ready"), ("ready", None),
                              (True, "ready"), ("unknown", "ready")):
                with self.subTest(target=target, readiness=readiness):
                    pending = check(None, target, readiness)
                    self.assertEqual(pending["status"], "incomplete-native-proof")
                    self.assertTrue(pending["removal_verified"])
                    self.assertTrue(pending["repeat_verified"])
                    self.assertTrue(pending["sentinels_preserved"])
                    self.assertEqual(pending["public_skill_ready"], {
                        "after-add": readiness[0] == "ready", "repeat-add": readiness[1] == "ready"})
            ready = check(None, target)
            self.assertEqual(ready["status"], "passed")
            for actions in ((["finish setup"], None), (None, ["finish setup"]), ({"pending": True}, None)):
                with self.subTest(target=target, actions=actions):
                    pending = check(None, target, actions=actions)
                    self.assertEqual(pending["status"], "incomplete-native-proof")
                    self.assertTrue(pending["removal_verified"])
        pending_with_residue = check("symlink", readiness=("needs_action", "needs_action"))
        self.assertEqual(pending_with_residue["status"], "cleanup-failed")
        self.assertFalse(pending_with_residue["removal_verified"])


    def test_copilot_settings_probe_redacts_values_and_keeps_refusals(self):
        # BUG-019: distinguish a qualified native key from incidental text,
        # never include native settings contents in retained E2E artifacts.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "plugin-00"
            home = case / "home"
            settings = home / ".copilot/settings.json"
            settings.parent.mkdir(parents=True)
            self.assertEqual(
                audit.copilot_settings_sanitized_shape(home, case, "atlassian"),
                {"status": "absent"})
            settings.write_text(
                '{"plugins":{"atlassian@agentplugins-aaaaaaaaaaaa":true},'
                '"unmanaged":{"secret":"confidential-password-123",'
                '"url":"https://example.invalid/atlassian"}}')
            proof = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(proof["status"], "parsed")
            self.assertEqual(proof["allowed_sections"], ["plugins"])
            self.assertEqual(proof["match_counts"]["qualified_key"], 1)
            self.assertEqual(proof["match_counts"]["reference_value"], 1)
            self.assertEqual(proof["match_counts"]["exact_key"], 0)
            self.assertNotIn("confidential-password-123", str(proof))
            self.assertNotIn("example.invalid", str(proof))
            self.assertNotIn("unmanaged", str(proof))
            settings.write_text('{"plugins":{}}')
            clean = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(clean["status"], "parsed")
            self.assertFalse(any(clean["match_counts"].values()))

    def test_copilot_enabled_plugin_values_are_exact_and_never_exported(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home = case / "home"
            settings = home / ".copilot/settings.json"
            settings.parent.mkdir(parents=True)
            settings.write_text(json.dumps({"enabledPlugins": {
                "atlassian@agentplugins-aaaaaaaaaaaa": True,
                "atlassian@agentplugins-bbbbbbbbbbbb": False,
                "atlassian@agentplugins-cccccccccccc": "private-auth-token",
                "unrelated@agentplugins-dddddddddddd": True,
                "atlassian@agentplugins-not-a-hex": "private-user-value",
            }, "unmanaged": {"API_KEY": "private-secret-value"}}))
            proof = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(proof["status"], "parsed")
            self.assertEqual(proof["enabled_plugin_value_counts"],
                             {"true": 1, "false": 1, "non_boolean": 1})
            self.assertEqual(proof["match_counts"]["qualified_key"], 3)
            self.assertFalse(any(secret in str(proof) for secret in (
                "private-auth-token", "private-secret-value",
                "private-user-value", "unrelated@agentplugins")))
            settings.write_text('{"enabledPlugins":{"other@agentplugins-aaaaaaaaaaaa":true}}')
            clean = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(clean["enabled_plugin_value_counts"],
                             {"true": 0, "false": 0, "non_boolean": 0})

    def test_copilot_settings_probe_rejects_unsafe_paths_and_bad_json(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case, outside = root / "case", root / "outside"
            settings = case / "home/.copilot/settings.json"
            settings.parent.mkdir(parents=True)
            outside.write_text('{"plugins":{"atlassian":true}}')
            settings.symlink_to(outside)
            self.assertEqual(audit.copilot_settings_sanitized_shape(
                case / "home", case, "atlassian")["status"], "unsafe-path")
            settings.unlink()
            settings.write_text('{"invalid-json"')
            self.assertEqual(audit.copilot_settings_sanitized_shape(
                case / "home", case, "atlassian")["status"],
                "unreadable-or-invalid-json")
            settings.write_bytes(b"X" * (1024 * 1024 + 1))
            self.assertEqual(audit.copilot_settings_sanitized_shape(
                case / "home", case, "atlassian")["status"],
                "not-regular-or-too-large")



    def test_pi_foreign_skill_uses_separate_preserved_project(self):
        audit = load_audit()
        item = {"name": "frontend-design", "source": "unused-fixture",
                "repo": "example/skills", "folder": "frontend-design",
                "skill_md_git_blob_sha1": "0" * 40}
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                   "timeout": False, "_json": {"data": {"installations": []}}}
        for mutation in (None, "bytes", "extra-file"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                runroot = Path(temporary).resolve()
                case = runroot / "skill-00"
                neighbor = case / "project-b/.pi/skills/foreign-neighbor/SKILL.md"
                def source_refusal(*args, **kwargs):
                    if mutation == "bytes":
                        neighbor.write_text("changed")
                    elif mutation == "extra-file":
                        (neighbor.parent / "unowned.txt").write_text("unexpected")
                    return dict(refused)
                with patch.object(audit, "call", side_effect=source_refusal):
                    if mutation:
                        with self.assertRaisesRegex(ValueError, "Pi foreign project changed"):
                            audit.do_case("/unused/agentplugins", "pi", "skill", item, 0, runroot, {})
                    else:
                        row = audit.do_case("/unused/agentplugins", "pi", "skill", item, 0, runroot, {})
                        self.assertEqual(neighbor.read_text(),
                                         "---\nname: foreign-neighbor\ndescription: unrelated\n---\n")
                        self.assertFalse((case / "project/.pi/skills/foreign-neighbor").exists())
                        self.assertTrue(row["sentinels_preserved"])
                        self.assertEqual(row["status"], "install-failed")

    def test_pi_foreign_project_redirect_is_refused_before_fixture_write(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            runroot, outside = root / "run", root / "outside"
            case = runroot / "skill-00"
            case.mkdir(parents=True)
            outside.mkdir()
            try:
                (case / "project-b").symlink_to(outside, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"symlinks unavailable: {error}")
            with patch.object(audit, "call") as forbidden:
                with self.assertRaisesRegex(ValueError, "foreign Skill neighbor escapes"):
                    audit.do_case("/unused/agentplugins", "pi", "skill", {}, 0, runroot, {})
                forbidden.assert_not_called()
            self.assertEqual(list(outside.iterdir()), [])

    def test_pi_foreign_project_mkdir_swap_refuses_before_fixture_write(self):
        audit = load_audit()
        item = {"name": "frontend-design", "source": "unused-fixture",
                "repo": "example/skills", "folder": "frontend-design",
                "skill_md_git_blob_sha1": "0" * 40}
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                   "timeout": False, "_json": {"data": {"installations": []}}}
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            runroot, outside = root / "run", root / "outside"
            case = runroot / "skill-00"
            project_b = case / "project-b"
            foreign = project_b / ".pi/skills/foreign-neighbor/SKILL.md"
            outside_member = outside / ".pi/skills/foreign-neighbor/SKILL.md"
            outside_member.parent.mkdir(parents=True)
            before = audit.scoped_snapshot((outside,))
            original_mkdir, swapped = Path.mkdir, []
            def redirected_mkdir(path, *args, **kwargs):
                result = original_mkdir(path, *args, **kwargs)
                if path == foreign.parent and not swapped:
                    project_b.rename(case / "original-project-b")
                    try:
                        project_b.symlink_to(outside, target_is_directory=True)
                    except (OSError, NotImplementedError) as error:
                        self.skipTest(f"symlinks unavailable: {error}")
                    swapped.append(True)
                return result
            with (patch.object(Path, "mkdir", redirected_mkdir),
                  patch.object(audit, "call", return_value=refused) as forbidden):
                with self.assertRaisesRegex(ValueError, "Pi foreign Skill neighbor.*before fixture write"):
                    audit.do_case("/unused/agentplugins", "pi", "skill", item, 0, runroot, {})
                forbidden.assert_not_called()
            self.assertEqual(swapped, [True])
            self.assertEqual(audit.scoped_snapshot((outside,)), before)
            self.assertFalse(outside_member.exists())

    def test_pi_existing_foreign_skill_refuses_without_overwriting_bytes(self):
        audit = load_audit()
        item = {"name": "frontend-design", "source": "unused-fixture",
                "repo": "example/skills", "folder": "frontend-design",
                "skill_md_git_blob_sha1": "0" * 40}
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                   "timeout": False, "_json": {"data": {"installations": []}}}
        for linked in (False, True):
            with self.subTest(hardlink=linked), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                runroot, outside = root / "run", root / "outside"
                project_b = runroot / "skill-00/project-b"
                foreign = project_b / ".pi/skills/foreign-neighbor/SKILL.md"
                foreign.parent.mkdir(parents=True)
                outside.mkdir()
                original = outside / "foreign.txt"
                original.write_bytes(b"original foreign bytes\n")
                if linked:
                    try:
                        os.link(original, foreign)
                    except (OSError, NotImplementedError) as error:
                        self.skipTest(f"hardlinks unavailable: {error}")
                else:
                    foreign.write_bytes(original.read_bytes())
                before = audit.scoped_snapshot((project_b, outside))
                with patch.object(audit, "call", return_value=refused) as forbidden:
                    with self.assertRaises(FileExistsError):
                        audit.do_case("/unused/agentplugins", "pi", "skill", item, 0, runroot, {})
                    forbidden.assert_not_called()
                self.assertEqual(audit.scoped_snapshot((project_b, outside)), before)
                self.assertEqual(foreign.read_bytes(), b"original foreign bytes\n")
                self.assertEqual(original.read_bytes(), b"original foreign bytes\n")

    def test_pi_foreign_project_must_be_ordinary_before_baseline(self):
        audit = load_audit()
        item = {"name": "frontend-design", "source": "unused-fixture",
                "repo": "example/skills", "folder": "frontend-design",
                "skill_md_git_blob_sha1": "0" * 40}
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                   "timeout": False, "_json": {"data": {"installations": []}}}
        for replacement in ("symlink", "file", "absent"):
            with self.subTest(replacement=replacement), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                runroot, outside = root / "run", root / "outside"
                case = runroot / "skill-00"
                project_b = case / "project-b"
                foreign = project_b / ".pi/skills/foreign-neighbor/SKILL.md"
                outside.mkdir()
                before = audit.scoped_snapshot((outside,))
                original_open, swapped = Path.open, []
                @contextmanager
                def redirected_open(path, *args, **kwargs):
                    with original_open(path, *args, **kwargs) as stream:
                        yield stream
                    mode = args[0] if args else kwargs.get("mode", "r")
                    if path == foreign and mode in ("w", "x") and not swapped:
                        project_b.rename(case / "original-project-b")
                        if replacement == "symlink":
                            try:
                                project_b.symlink_to(outside, target_is_directory=True)
                            except (OSError, NotImplementedError) as error:
                                self.skipTest(f"symlinks unavailable: {error}")
                        elif replacement == "file":
                            project_b.write_bytes(b"replacement")
                        swapped.append(True)
                with (patch.object(Path, "open", redirected_open),
                      patch.object(audit, "call", return_value=refused) as forbidden):
                    with self.assertRaisesRegex(ValueError, "Pi foreign project.*ordinary contained directory"):
                        audit.do_case("/unused/agentplugins", "pi", "skill", item, 0, runroot, {})
                    forbidden.assert_not_called()
                self.assertEqual(swapped, [True])
                self.assertEqual(audit.scoped_snapshot((outside,)), before)

    def test_qoder_foreign_skill_is_never_placed_in_uap_managed_bundle(self):
        audit = load_audit()
        item = {"name": "frontend-design", "source": "unused-fixture",
                "repo": "example/skills", "folder": "frontend-design",
                "skill_md_git_blob_sha1": "0" * 40}
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                   "timeout": False, "_json": {"data": {"installations": []}}}
        with TemporaryDirectory() as temporary:
            runroot = Path(temporary).resolve()
            with patch.object(audit, "call", return_value=refused):
                row = audit.do_case("/unused/agentplugins", "qoder", "skill",
                                    item, 0, runroot, {})
            project = runroot / "skill-00/project"
            foreign = project / ".qoder/skills/foreign-neighbor/SKILL.md"
            managed_foreign = project / ".uap/managed/clients/qoder/foreign-neighbor"
            self.assertTrue(foreign.is_file())
            self.assertEqual(foreign.read_text(encoding="utf-8"),
                             "---\nname: foreign-neighbor\ndescription: unrelated\n---\n")
            self.assertFalse(managed_foreign.exists())
            self.assertTrue(row["sentinels_preserved"])

    def test_grok_foreign_skill_is_not_an_invalid_native_plugin(self):
        # BUG-060: Grok rejects a plugins/ child lacking plugin.json before
        # reviewing any selected source. The unrelated Skill belongs in skills/.
        audit = load_audit()
        item = {"name": "frontend-design", "repo": "example/skills",
                "folder": "frontend-design", "source": "unused",
                "skill_md_git_blob_sha1": "0" * 40}
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "refused",
                   "timeout": False, "_json": {"data": {"installations": []}}}
        # The Grok project prerequisite uses a real isolated git init.
        # Mock source installs, not the pinned local Git executable itself.
        real_call = audit.call
        def source_refusal(argv, env, cwd, timeout=105):
            if argv[0] == audit.GIT_EXECUTABLE:
                return real_call(argv, env, cwd, timeout)
            return dict(refused)
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            with patch.object(audit, "call", side_effect=source_refusal):
                evidence = audit.do_case("/unused/agentplugins", "grok", "skill",
                                         item, 0, root, {})
            project = root / "skill-00/project"
            self.assertTrue((project / ".git").is_dir())
            foreign = project / ".grok/skills/foreign-neighbor/SKILL.md"
            self.assertTrue(foreign.is_file())
            self.assertEqual(foreign.read_text(encoding="utf-8"),
                             "---\nname: foreign-neighbor\ndescription: unrelated\n---\n")
            self.assertFalse((project / ".grok/plugins/foreign-neighbor").exists())
            self.assertTrue(evidence["sentinels_preserved"])
            self.assertEqual(evidence["status"], "install-failed")

    def test_vscode_user_plugins_use_only_documented_no_save_route(self):
        # BUG-061: no new VS Code product capability or saved dependency.
        audit = load_audit()
        source = "github:example/native-plugin@" + ("a" * 40)
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "refused",
                   "timeout": False, "_json": {"data": {"installations": []}}}
        with TemporaryDirectory() as temporary:
            for target in ("vscode", "codex"):
                with self.subTest(target=target):
                    root = Path(temporary).resolve() / target
                    with patch.object(audit, "call",
                                      side_effect=lambda *a, **kw: dict(refused)) as call:
                        evidence = audit.do_case("/unused/agentplugins", target,
                                                 "plugin", source, 0, root,
                                                 {source: "native-plugin"})
                    first = call.call_args_list[0].args[0]
                    self.assertEqual(first[1:3], ["add", source])
                    self.assertEqual(first.count("--no-save"), int(target == "vscode"))
                    self.assertEqual(first[first.index("--scope") + 1], "user")
                    self.assertTrue(evidence["sentinels_preserved"])
                    self.assertEqual(evidence["status"], "install-failed")

    def test_vscode_manual_external_removal_stays_nonpass(self):
        # BUG-064: prepared, non-activated VS Code is NOT proven removed.
        audit = load_audit()
        selected = "github:example/native-demo@" + ("a" * 40)
        matched = [{"name": "native-demo", "installation_id": "same-install",
                    "clients": [{"client_id": "vscode", "scope": "user",
                                 "materialization": "materialized",
                                 "verification": "package_validated",
                                 "activation": "manual_activation_required"}]}]
        def response(exit_code=0, stderr="", installations=None):
            return {"exit_code": exit_code, "stdout_tail": "", "stderr_tail": stderr,
                    "timeout": False,
                    "_json": {"data": {"installations": matched if installations is None
                                       else installations}}}
        manual = response(1, "agentplugins: removal requires a manual client step for [vscode]; "
                          "verify native absence, then rerun --external-uninstalled")
        with TemporaryDirectory() as temporary:
            with (patch.object(audit, "call", side_effect=[
                    response(), response(), response(), response(), manual]),
                  patch.object(audit, "matching_installations", return_value=matched)):
                proof = audit.do_case(
                    "/unused/agentplugins", "vscode", "plugin", selected, 0,
                    Path(temporary).resolve(), {selected: "native-demo"})
        self.assertEqual(proof["status"], "external-native-uninstall-required")
        self.assertTrue(proof["prepared_binding"])
        self.assertTrue(proof["repeat_verified"])
        self.assertFalse(proof["removal_verified"])
        self.assertEqual(proof["cleanup"]["exit_code"], 1)
        self.assertTrue(proof["sentinels_preserved"])

    def test_kimi_skill_uses_real_isolated_git_worktree(self):
        audit = load_audit()
        self.assertTrue(audit.GIT_EXECUTABLE and Path(audit.GIT_EXECUTABLE).is_file())
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, project, data = case / "home", case / "project", case / "uap-data"
            for directory in (home, project, data):
                directory.mkdir(parents=True)
            env = audit.isolated_client_env(home, case, data)
            audit.initialize_project_skill_git_root(project, env, case)
            self.assertTrue((project / ".git").is_dir())
            self.assertFalse((project / ".git").is_symlink())
            # Real Git must recognize this disposable root; an empty marker is not enough.
            checked = audit.call([audit.GIT_EXECUTABLE, "-C", str(project),
                                  "rev-parse", "--show-toplevel"], env, project, 30)
            self.assertEqual(checked["exit_code"], 0)
            self.assertEqual(Path(checked["stdout_tail"].strip()).resolve(), project)
            with self.assertRaisesRegex(RuntimeError, "prerequisite unavailable"):
                audit.initialize_project_skill_git_root(project, env, case / "outside")

    def test_project_git_init_rejects_existing_symlink_metadata(self):
        # BUG-065: git init follows .git symlinks and can write outside the case
        # before a post-execution containment check detects the redirect.
        audit = load_audit()
        with TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            case = root / "case"
            project, home, data = case / "project", case / "home", case / "uap-data"
            for directory in (project, home, data):
                directory.mkdir(parents=True, exist_ok=True)
            outside = root / "outside"
            outside.mkdir()
            marker = project / ".git"
            try:
                marker.symlink_to(outside, target_is_directory=True)
            except OSError as err:
                self.skipTest(f"symlinks unavailable: {err}")
            env = audit.isolated_client_env(home, case, data)
            with patch.object(audit, "call") as forbidden:
                with self.assertRaisesRegex(RuntimeError, "metadata must be initially absent"):
                    audit.initialize_project_skill_git_root(project, env, case)
                forbidden.assert_not_called()
            self.assertEqual(list(outside.iterdir()), [])
            marker.unlink()
            marker.mkdir()
            with patch.object(audit, "call") as forbidden:
                with self.assertRaisesRegex(RuntimeError, "metadata must be initially absent"):
                    audit.initialize_project_skill_git_root(project, env, case)
                forbidden.assert_not_called()

    def test_isolated_git_prerequisite_only_for_kimi_grok_and_copilot_skills(self):
        audit = load_audit()
        failed = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                  "timeout": False, "_json": {"data": {"installations": []}}}
        fixture = {"name": "frontend-design", "source": "unused",
                   "repo": "example/skills", "folder": "frontend-design",
                   "skill_md_git_blob_sha1": "0" * 40}
        with (TemporaryDirectory() as temporary,
              patch.object(audit, "call", return_value=failed),
              patch.object(audit, "initialize_project_skill_git_root") as initialize):
            root = Path(temporary).resolve()
            for target, kind, item in (
                    ("kimi", "plugin", "atlassian"),
                    ("grok", "plugin", "atlassian"),
                    ("copilot", "plugin", "atlassian"),
                    ("qoder", "skill", fixture)):
                audit.do_case("/unused/agentplugins", target, kind, item, 0,
                              root / f"{target}-{kind}", {})
            self.assertEqual(initialize.call_count, 0)
            for target in ("kimi", "grok", "copilot"):
                audit.do_case("/unused/agentplugins", target, "skill", fixture,
                              0, root / f"{target}-skill", {})
            self.assertEqual(initialize.call_count, 3)
            self.assertEqual(
                {call.args[0] for call in initialize.call_args_list},
                {root / "kimi-skill/skill-00/project",
                 root / "grok-skill/skill-00/project",
                 root / "copilot-skill/skill-00/project"})

    def test_grok_and_copilot_skills_get_real_git_root_before_install(self):
        # Both native clients discover their selected project through Git.
        audit = load_audit()
        self.assertTrue(audit.GIT_EXECUTABLE and Path(audit.GIT_EXECUTABLE).is_file())
        item = {"name": "frontend-design", "source": "unused",
                "repo": "example/skills", "folder": "frontend-design",
                "skill_md_git_blob_sha1": "0" * 40}
        failed = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                  "timeout": False, "_json": {"data": {"installations": []}}}
        original_call = audit.call
        def fake_call(argv, env, cwd, timeout=105):
            if argv[0] == audit.GIT_EXECUTABLE:
                return original_call(argv, env, cwd, timeout)
            self.assertTrue((cwd / ".git").is_dir())
            return dict(failed)
        for target in ("grok", "copilot"):
            with self.subTest(target=target), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                with patch.object(audit, "call", side_effect=fake_call):
                    result = audit.do_case("/unused/agentplugins", target, "skill",
                                           item, 0, root, {})
                project = root / "skill-00/project"
                self.assertEqual(result["status"], "install-failed")  # mocked add is not PASS
                self.assertTrue(result["sentinels_preserved"])
                self.assertTrue((project / ".git").is_dir())
                self.assertFalse((project / ".git").is_symlink())
                checked = subprocess.run([audit.GIT_EXECUTABLE, "-C", str(project),
                                          "rev-parse", "--show-toplevel"],
                                         capture_output=True, text=True, check=False)
                self.assertEqual(checked.returncode, 0)
                self.assertEqual(Path(checked.stdout.strip()).resolve(), project)
                skill_root = ".grok/skills" if target == "grok" else ".github/skills"
                self.assertTrue((project / skill_root / "foreign-neighbor/SKILL.md").is_file())
                self.assertFalse((project / ".grok/plugins/foreign-neighbor").exists())

    def test_qoder_latest_log_link_only_qualifies_closed_run_diagnostics(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve() / ".qoder"
            run = root / "logs" / "runs" / "session-1"
            run.mkdir(parents=True)
            (run / "qodercli.log").write_text("diagnostic only")
            link = root / "logs/latest"
            observed = [".qoder/logs/latest"]
            link.symlink_to("runs/session-1", target_is_directory=True)
            self.assertTrue(audit.qoder_passive_latest_log_link(root, link))
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"), [])
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "codex"),
                [".qoder/logs/latest: symlink"])
            link.unlink()
            outside = root.parent / "outside"
            outside.mkdir()
            link.symlink_to("../../outside", target_is_directory=True)
            self.assertFalse(audit.qoder_passive_latest_log_link(root, link))
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"),
                [".qoder/logs/latest: symlink"])
            link.unlink()
            link.symlink_to("runs/missing", target_is_directory=True)
            self.assertFalse(audit.qoder_passive_latest_log_link(root, link))
            link.unlink()
            (root / "logs/runs/chain").symlink_to(run, target_is_directory=True)
            link.symlink_to("runs/chain", target_is_directory=True)
            self.assertFalse(audit.qoder_passive_latest_log_link(root, link))
            link.unlink()
            other = root / "logs/other"
            other.symlink_to("runs/session-1", target_is_directory=True)
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", [".qoder/logs/other"], "qoder"),
                [".qoder/logs/other: symlink"])

    def test_post_source_redirected_ancestor_is_rejected_before_reading_native_state(self):
        # BUG-044: a plugin can replace a parent path AFTER the initial
        # containment check; the descendant native root itself is not a symlink.
        import shutil
        audit = load_audit()
        for target, poisoned in (("codex", "home"), ("opencode", "native-ancestor"),
                                 ("codex", "project")):
            with self.subTest(target=target, poisoned=poisoned), TemporaryDirectory() as temp:
                base = Path(temp).resolve()
                runroot = base / "runs"
                foreign = base / "foreign"
                foreign.mkdir()
                calls = 0

                def malicious_install(argv, env, cwd, timeout=105):
                    nonlocal calls
                    calls += 1
                    self.assertEqual(calls, 1)  # no subsequent read-only list
                    case = runroot / "plugin-00"
                    path = (case / "home/.config" if poisoned == "native-ancestor"
                            else case / poisoned)
                    self.assertTrue(path.is_dir())
                    shutil.rmtree(path)
                    path.symlink_to(foreign, target_is_directory=True)
                    return {"exit_code": 0, "_json": None, "stdout_tail": "",
                            "stderr_tail": "", "timeout": False}

                with patch.object(audit, "call", side_effect=malicious_install), \
                     self.assertRaisesRegex(ValueError, "audit case"):
                    audit.do_case("/unused/agentplugins", target, "plugin",
                                  "atlassian", 0, runroot, {})
                self.assertEqual(calls, 1)
                self.assertEqual(list(foreign.iterdir()), [])


    def test_case_root_symlink_never_redirects_prewrite_fixtures(self):
        # BUG-041: an old case could redirect the *case* trust anchor itself.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            outer = Path(temporary).resolve()
            foreign = outer / "foreign"
            foreign.mkdir()
            for poisoned in ("case-root", "run-root", "post-mkdir-swap"):
                with self.subTest(poisoned=poisoned):
                    root = outer / poisoned
                    case = root / "plugin-00"
                    if poisoned == "run-root":
                        root.symlink_to(foreign, target_is_directory=True)
                    else:
                        root.mkdir()
                        if poisoned == "case-root":
                            case.symlink_to(foreign, target_is_directory=True)
                    original_mkdir = Path.mkdir
                    modified = False

                    def post_mkdir(path, *args, **kwargs):
                        nonlocal modified
                        result = original_mkdir(path, *args, **kwargs)
                        if poisoned == "post-mkdir-swap" and path == case and not modified:
                            modified = True
                            case.rmdir()
                            case.symlink_to(foreign, target_is_directory=True)
                        return result

                    with (patch.object(Path, "mkdir", post_mkdir),
                          patch.object(audit, "call") as call,
                          self.assertRaisesRegex(ValueError, "audit (case|run) root")):
                        audit.do_case("/unused/agentplugins", "codex", "plugin",
                                      "atlassian", 0, root, {})
                    call.assert_not_called()
                    self.assertEqual(list(foreign.iterdir()), [])
                    if poisoned != "run-root":
                        # Leave the root available for the next disposable case.
                        case.unlink()
                        root.rmdir()
                    else:
                        root.unlink()

    def test_case_creation_never_writes_through_planted_child_symlinks(self):
        audit = load_audit()
        original_mkdir = Path.mkdir
        with TemporaryDirectory() as temporary:
            outer = Path(temporary).resolve()
            foreign = outer / "foreign"
            foreign.mkdir()
            for poisoned in ("home", "local-parent"):
                with self.subTest(poisoned=poisoned):
                    runroot = outer / poisoned
                    case = runroot / "plugin-00"

                    injected = set()

                    def injected_mkdir(path, *args, **kwargs):
                        result = original_mkdir(path, *args, **kwargs)
                        # Path.mkdir(parents=True) may retry the same path.
                        # Plant a symlink exactly once, before the next guard.
                        if not injected and poisoned == "home" and path == case:
                            injected.add(poisoned)
                            (case / "home").symlink_to(foreign, target_is_directory=True)
                        elif not injected and poisoned == "local-parent" and path == case / "home":
                            injected.add(poisoned)
                            (case / "home/.local").symlink_to(
                                foreign, target_is_directory=True)
                        return result

                    with (patch.object(Path, "mkdir", injected_mkdir),
                          self.assertRaisesRegex(ValueError, "nested audit directory")):
                        audit.do_case("/unused/agentplugins", "codex", "plugin",
                                      "atlassian", 0, runroot, {})
                    self.assertEqual(injected, {poisoned})
                    self.assertEqual(list(foreign.iterdir()), [])

    def test_gemini_repeat_phase_diagnostics_keep_generated_logs_unqualified(self):
        audit = load_audit()
        item = {"name": "owned-test", "repo": "example/skills", "folder": "owned-test",
                "source": "unused", "skill_md_git_blob_sha1": "0" * 40}
        matched = [{"installation_id": "owned-id", "name": "owned-test",
                    "clients": [{"client_id": "gemini"}]}]
        operations = []
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            project = root / 'skill-00/project'
            skill = project / '.gemini/skills/owned-test/SKILL.md'
            logs = root / 'skill-00/home/.gemini/tmp/project/chats'
            def fake_call(argv, env, cwd, timeout=105):
                operation = argv[2] if argv[1] == 'skills' else argv[1]
                operations.append(operation)
                if operation == 'add':
                    skill.parent.mkdir(parents=True, exist_ok=True)
                    skill.write_text('---\nname: owned-test\n---\n')
                    (project / 'uap.toml').write_text('owned-test')
                    (project / 'uap.lock').write_text('owned-test')
                    if operations.count('add') == 2:
                        logs.mkdir(parents=True)
                        (logs / 'session-add-skills-l.jsonl').write_text('{}\n')
                elif operation == 'list' and operations.count('list') == 2:
                    (logs / 'session-list-skills-l.jsonl').write_text('{}\n')
                elif operation == 'remove':
                    skill.unlink()
                    skill.parent.rmdir()
                    (project / 'uap.lock').unlink()
                return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "", "timeout": False,
                        "_json": {"data": {"installations": [] if 'remove' in operations else matched}}}
            with (patch.object(audit, 'call', side_effect=fake_call),
                  patch.object(audit, 'matching_installations', return_value=matched),
                  patch.object(audit, 'locked_skill_is_pinned',
                               side_effect=lambda *args: (project / 'uap.lock').exists()),
                  patch.object(audit, 'matches_pinned_skill_bytes', return_value=True)):
                row = audit.do_case('/unused/agentplugins', 'gemini', 'skill', item, 0, root, {})
        self.assertEqual(operations, ['add', 'list', 'add', 'list', 'remove', 'list'])
        phases = row['repeat_native_phases']
        self.assertEqual([phase['phase'] for phase in phases],
                         ['after-public-repeat-add', 'after-public-repeat-list'])
        self.assertEqual([phase['total'] for phase in phases], [1, 2])
        self.assertEqual([entry['path'].split('/')[-1] for entry in phases[0]['entries']],
                         ['session-add-skills-l.jsonl'])
        self.assertEqual({entry['path'].split('/')[-1] for entry in phases[1]['entries']},
                         {'session-add-skills-l.jsonl', 'session-list-skills-l.jsonl'})
        self.assertFalse(row['repeat_verified'])
        self.assertFalse(row['repeat_native_runtime_only'])
        self.assertEqual(row['status'], 'repeat-add-unqualified')
        self.assertTrue(row['removal_verified'])
        self.assertTrue(row['sentinels_preserved'])

    def test_failed_first_list_cannot_attest_plugin_lifecycle(self):
        # BUG-029: matching JSON from a failed list is not an attestation.
        audit = load_audit()
        rows = [{"installation_id": "owned-id", "name": "atlassian",
                 "clients": [{"client_id": "claude"}]}]
        operations = []

        def fake_call(argv, env, cwd, timeout=105):
            operation = argv[1]
            operations.append(operation)
            first_list_failed = operation == "list" and operations.count("list") == 1
            return {"exit_code": 1 if first_list_failed else 0, "stdout_tail": "",
                    "stderr_tail": "", "timeout": False,
                    "_json": {"data": {"installations": rows}}}

        with TemporaryDirectory() as tmp, \
             patch.object(audit, "call", side_effect=fake_call), \
             patch.object(audit, "matching_installations", return_value=rows):
            result = audit.do_case("/unused/agentplugins", "claude", "plugin",
                                   "atlassian", 0, Path(tmp), {})
        self.assertEqual(result["install"]["exit_code"], 0)
        self.assertEqual(result["list"]["exit_code"], 1)
        self.assertTrue(result["listed_identity"])
        self.assertIsNone(result["repeat_add"])
        self.assertIsNone(result["cleanup"])
        self.assertEqual(operations, ["add", "list"])
        self.assertEqual(result["status"], "incomplete-native-proof")
        self.assertNotIn("native_residual_diagnostics", result)


    def test_grok_prepared_skill_is_source_bound_and_never_native_pass(self):
        # A real prepared Grok Skill is not a native installation attestation.
        audit = load_audit()
        item = {"name": "frontend-design", "repo": "sample/skills",
                "folder": "frontend-design", "ref": "a" * 40}
        name = "skill-" + "b" * 56
        bundle = "skill-" + "b" * 45 + "-" + "c" * 12
        uid = "c99742fc-59bf-4225-b862-0c21e5fcc992"
        revision = {"resolved_revision": item["ref"],
                    "tree_digest": "sha256:" + "d" * 64,
                    "manifest_digest": "sha256:" + "e" * 64}
        entry = {"installation_id": uid, "name": name,
                 "source": "sample/skills/skills/frontend-design",
                 "clients": [{"client_id": "grok", "scope": "project",
                              "materialization": "materialized", "verification": "package_validated",
                              "activation": "prepared", "policy": "allowed",
                              "package_revision": revision}]}
        response = {"data": {"installations": [entry]}}
        with TemporaryDirectory() as tmp:
            project = Path(tmp) / "project"
            relative = f".grok/plugins/{bundle}/skills/{item['name']}/SKILL.md"
            skill = project / relative
            skill.parent.mkdir(parents=True)
            skill.write_text("---\nname: frontend-design\n---\n")
            (project / "uap.lock").write_text(json.dumps({"packages": [{
                "alias": item["name"], "tree_digest": revision["tree_digest"],
                "manifest_digest": revision["manifest_digest"]}]}))
            first = {"status": "succeeded", "items": [{
                "alias": item["name"], "targetId": "grok", "status": "succeeded",
                "candidateId": "sha256:" + "f" * 64,
                "destinationPath": str(project / ".grok/plugins" / bundle)}]}
            output = {"stdout_tail": json.dumps(first)}
            matched = audit.grok_prepared_skill_installations(response, item, relative, project, output)
            self.assertEqual([node["installation_id"] for node in matched], [uid])
            repeat = json.loads(json.dumps(first))
            repeat["items"][0]["status"] = "unchanged"
            repeat_output = {"stdout_tail": json.dumps(repeat)}
            self.assertEqual(len(audit.grok_prepared_skill_installations(
                response, item, relative, project, repeat_output, repeated=True)), 1)
            self.assertEqual(audit.grok_prepared_skill_installations(
                response, item, relative, project, output, repeated=True), [])
            for field in ("source", "name", "installation_id"):
                forged = json.loads(json.dumps(response))
                forged["data"]["installations"][0][field] = "forged"
                self.assertEqual(audit.grok_prepared_skill_installations(
                    forged, item, relative, project, output), [])
            for field in ("activation", "verification", "scope", "policy"):
                forged = json.loads(json.dumps(response))
                forged["data"]["installations"][0]["clients"][0][field] = "forged"
                self.assertEqual(audit.grok_prepared_skill_installations(
                    forged, item, relative, project, output), [])
            forged = json.loads(json.dumps(response))
            forged["data"]["installations"][0]["clients"][0]["package_revision"]["tree_digest"] = "sha256:" + "0" * 64
            self.assertEqual(audit.grok_prepared_skill_installations(
                forged, item, relative, project, output), [])
            self.assertFalse(audit.grok_prepared_source_survives(
                {"data": {"installations": []}}, item, name, uid))
            self.assertTrue(audit.grok_prepared_source_survives(
                {"data": {"installations": [{**entry, "name": "reminted", "installation_id": "new"}]}},
                item, name, uid))
            original_bytes = skill.read_bytes()
            foreign = project / "outside.md"
            foreign.write_bytes(original_bytes)
            skill.unlink()
            skill.symlink_to(foreign)
            self.assertEqual(audit.grok_prepared_skill_installations(
                response, item, relative, project, output), [])
            skill.unlink()
            skill.write_bytes(original_bytes)
            self.assertEqual(len(audit.grok_prepared_skill_installations(
                response, item, relative, project, output)), 1)


    def test_codex_marketplace_containment_follows_physical_directory_identity(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "case"
            source = case / "home/.codex/plugins/marketplaces/owned"
            foreign = root / "case-replacement/owned"
            source.mkdir(parents=True)
            foreign.mkdir(parents=True)
            self.assertTrue(audit.codex_marketplace_source_in_case(source, case))
            self.assertFalse(audit.codex_marketplace_source_in_case(foreign, case))
            self.assertFalse(audit.codex_marketplace_source_in_case(
                case / "home/.codex/plugins/missing", case))
            self.assertFalse(audit.codex_marketplace_source_in_case(Path("relative"), case))
            # Reach main's actual creation seam with an owned lexical parent.
            (root / "spelling").mkdir()
            alias_parent = root / "spelling" / ".."
            cli = root / "inert-cli"
            cli.write_text("no native execution")
            observed = []

            def create_owned_root(*args, **kwargs):
                parent = Path(kwargs["dir"])
                self.assertEqual(parent, parent.resolve(strict=True),
                                 "main selected a noncanonical TEMP parent")
                return TemporaryDirectory(*args, **kwargs)

            def capture_case(*args):
                runroot = args[5]
                self.assertEqual(runroot, runroot.resolve(strict=True),
                                 "main handed a noncanonical root to the case")
                self.assertEqual(runroot.parent, root)
                before = runroot.lstat()
                held = audit.CODEX_RUNTIME_AUDIT["_real_directory"](runroot)
                self.assertTrue(os.path.samestat(before, held))
                lexical = alias_parent / runroot.name
                with self.assertRaisesRegex(ValueError, "audit directory is redirected"):
                    audit.CODEX_RUNTIME_AUDIT["_real_directory"](lexical)
                observed.append(runroot)
                return {"status": "passed"}

            with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}), \
                    patch.dict(audit.AGENT_BINARIES, {"codex": str(cli)}, clear=True), \
                    patch("sys.argv", ["live-install-audit.py", "--cli", str(cli),
                                       "--target", "chatgpt", "--kind", "plugin",
                                       "--limit", "1", "--report-dir", str(root)]), \
                    patch.object(audit, "published_audit_temp_parent", return_value=str(alias_parent)), \
                    patch.dict(audit.AGENT_BINARIES, {"codex": sys.executable}, clear=True), \
                    patch.object(audit, "version_evidence", return_value={"status": "reported"}), \
                    patch.object(audit, "do_case", side_effect=capture_case), \
                    patch.object(audit.tempfile, "TemporaryDirectory", side_effect=create_owned_root):
                self.assertEqual(audit.main(), 0)
            self.assertEqual(len(observed), 1)
            if os.name == "nt":
                # Same on-disk directory; Go may use the Windows extended prefix.
                extended = Path("\\\\?\\" + str(source))
                self.assertTrue(audit.codex_marketplace_source_in_case(extended, case))


    def test_genuine_project_git_init_uses_empty_hooks_inside_case(self):
        # Windows has no POSIX /dev/null; never use ambient hooks or config.
        audit = load_audit()
        if not audit.GIT_EXECUTABLE:
            self.skipTest("Git binary unavailable")
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, data, project = (case / "home", case / "uap-data", case / "project")
            for folder in (home, data, project):
                folder.mkdir(parents=True)
            env = audit.isolated_client_env(home, case, data)
            audit.initialize_project_skill_git_root(project, env, case)
            hooks = case / "empty-git-hooks"
            self.assertTrue(hooks.is_dir())
            self.assertFalse(hooks.is_symlink())
            self.assertTrue(hooks.resolve().is_relative_to(case.resolve()))
            self.assertTrue((project / ".git").is_dir())



class CrossPlatformPublishedProofRegressions(unittest.TestCase):
    """Keep raw published client proofs honest on ARM and Windows runners."""

    def test_windows_qoder_cache_and_log_paths_are_passive_but_settings_are_not(self):
        audit = load_audit()
        for relative in ("plugins/cache/local/atlassian/0.1.0/plugin.json",
                         "logs/runs/session-1/manifest.json"):
            self.assertTrue(audit.passive_native_path("qoder", relative), relative)
        # Only Windows treats backslashes as path separators. A POSIX
        # literal-backslash filename is not a passive managed cache.
        for relative in (r"plugins\cache\local\atlassian\0.1.0\plugin.json",
                         r"logs\runs\session-1\manifest.json"):
            self.assertEqual(audit.passive_native_path("qoder", relative),
                             os.name == "nt", relative)
        self.assertEqual(
            audit.passive_native_path("codex", r"plugins\cache\example.json"),
            os.name == "nt")
        for relative in ("settings.json", r"plugins\installed_plugins_v2.json",
                         "plugins/installed_plugins_v2.json"):
            self.assertFalse(audit.passive_native_path("qoder", relative), relative)

    def test_posix_literal_backslashes_cannot_hide_active_qoder_state(self):
        if os.name == "nt":
            self.skipTest("Windows cannot create literal backslash filenames")
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".qoder"
            root.mkdir()
            disguised = root / r"plugins\cache\managed.json"
            disguised.write_text('{"plugins":{"atlassian":{"enabled":true}}}',
                                 encoding="utf-8")
            observed = f"{root.name}/{disguised.name}"
            self.assertFalse(audit.passive_native_path("qoder", disguised.name))
            self.assertEqual(
                audit.native_identity_residuals(
                    [root], "atlassian", [observed], "qoder"),
                [observed])

    def test_qoder_profile_requires_same_canonical_disposable_root(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            home = root / "home"
            profile = home / ".qoder"
            profile.mkdir(parents=True)
            trusted_binary = str(root / "trusted-qodercli")
            env = {"QODER_CONFIG_DIR": str(profile.resolve(strict=True))}
            rows = [{"id": "atlassian@local", "version": "0.1.0"}]
            with (patch.dict(audit.AGENT_BINARIES, {"qodercli": trusted_binary}, clear=True),
                  patch.object(audit, "call", return_value={"exit_code": 0, "_json": rows}) as probe):
                self.assertEqual(audit.qoder_native_plugin_state(
                    home, env, root, "atlassian", "0.1.0"), {"status": "registered"})
                self.assertEqual(probe.call_args.args[0][2], str(profile))
                self.assertEqual(audit.qoder_native_plugin_state(
                    home, {"QODER_CONFIG_DIR": str(root / "other")},
                    root, "atlassian", "0.1.0")["status"], "unqualified-prerequisites")

    def prepared_signed_package(self, target="kiro"):
        pin = {"distribution": "777genius/atlassian", "repository": "777genius/universal-agent-plugins",
               "path": "plugins/atlassian", "revision": "a" * 40, "sequence": 1,
               "tree_digest": "sha256:" + "b" * 64, "manifest_digest": "sha256:" + "c" * 64}
        revision = {"distribution_id": pin["distribution"], "resolved_revision": pin["revision"],
                    "release_sequence": 1, "tree_digest": pin["tree_digest"],
                    "manifest_digest": pin["manifest_digest"]}
        binding = {"client_id": target, "scope": "user", "install_intent": "prepare",
                   "materialization": "materialized",
                   "activation": "prepared", "verification": "package_validated",
                   "policy": "allowed", "authentication": "not_checked", "package_revision": revision}
        row = {"name": "atlassian", "installation_id": "same-prepared-installation",
               "source": pin["repository"] + "/" + pin["path"], "clients": [binding],
               "directory": {"product_id": "atlassian", "recorded_distribution": pin["distribution"],
                             "recorded_revision": pin["revision"], "recorded_release_sequence": 1}}
        return row, {"atlassian": pin}

    def test_signed_prepared_package_requires_allowed_user_delivery_and_frozen_receipt(self):
        audit = load_audit()
        for target in ("kiro", "cursor"):
            with self.subTest(target=target):
                row, pins = self.prepared_signed_package(target)
                def match():
                    return audit.matching_installations(
                        {"data": {"installations": [row]}}, "atlassian", target,
                        "plugin", "atlassian", pins)
                self.assertEqual(match(), [row])
                binding = row["clients"][0]
                for key, bad in (("scope", "project"), ("materialization", "pending"),
                                 ("activation", "failed"), ("verification", "pending"),
                                 ("policy", "blocked"), ("client_id", "other")):
                    with self.subTest(key=key):
                        row["clients"] = [dict(binding, **{key: bad})]
                        self.assertEqual(match(), [])
                row["clients"] = [dict(binding, package_revision={})]
                self.assertEqual(match(), [])
                row["clients"] = [binding]
                pins["atlassian"]["manifest_digest"] = "sha256:" + "d" * 64
                self.assertEqual(match(), [])
                pins.clear()
                self.assertEqual(match(), [])

    def test_signed_prepared_package_runs_repeat_remove_without_native_pass(self):
        audit = load_audit()
        for mode in ("clean", "repeat-drift", "remove-residue", "foreign-change"):
            with self.subTest(mode=mode), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                matched, pins = self.prepared_signed_package()
                config = root / "plugin-00/home/.kiro/settings/mcp.json"
                state = {"installed": False, "adds": 0}
                operations = []
                def fake_call(argv, _env, cwd, timeout=105):
                    operation = argv[1]
                    operations.append(operation)
                    if operation == "add":
                        state["adds"] += 1
                        config.parent.mkdir(parents=True, exist_ok=True)
                        config.write_text('{"mcpServers":{"atlassian":{"command":"inert"}}}')
                        state["installed"] = True
                        if state["adds"] == 2 and mode == "repeat-drift":
                            config.write_text('{"mcpServers":{"atlassian":{"command":"changed"}}}')
                    elif operation == "remove":
                        if mode != "remove-residue":
                            config.unlink()
                        state["installed"] = False
                        if mode == "foreign-change":
                            (cwd / "foreign-untouched.txt").write_text("changed")
                    data = ({"actions": [{"component_support": [
                        {"name": "atlassian", "support": "native"}]}]} if operation == "add" else
                            {"installations": [matched] if state["installed"] else []})
                    return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "", "timeout": False,
                            "_json": {"data": data}}
                with patch.object(audit, "call", side_effect=fake_call):
                    row = audit.do_case("/unused/agentplugins", "kiro", "plugin",
                                        "atlassian", 0, root, {}, pins)
                self.assertEqual(operations, ["add", "list", "add", "list", "remove", "list"])
                self.assertTrue(row["prepared_binding"])
                self.assertNotEqual(row["status"], "passed")
                self.assertFalse(audit.expected_negative_coverage(row))
                if mode == "clean":
                    self.assertEqual(row["status"], "incomplete-native-proof")
                    self.assertTrue(row["kiro_prepared_lifecycle_verified"])
                    self.assertTrue(row["repeat_verified"])
                    self.assertTrue(row["removal_verified"])
                    self.assertTrue(row["sentinels_preserved"])
                    self.assertEqual(row["repeat_installation_ids"], [matched["installation_id"]])
                    self.assertEqual(row["public_target_states"]["after-add"]["states"][0]["activation"], "prepared")
                else:
                    self.assertNotEqual(row["status"], "prepared-lifecycle-passed")
                    self.assertFalse(row["kiro_prepared_lifecycle_verified"])
                    failed_gate = {"repeat-drift": "repeat_verified",
                                   "remove-residue": "removal_verified",
                                   "foreign-change": "sentinels_preserved"}[mode]
                    self.assertFalse(row[failed_gate])

    def test_prepared_only_opencode_config_is_not_native_pass(self):
        # Real published 0.1.102 can write opencode.json while all components
        # are reported as prepared, never native. Disk attribution is not activation.
        audit = load_audit()
        matched = [{"name": "atlassian", "installation_id": "frozen",
                    "clients": [{"client_id": "opencode", "verification": "installation_verified"}]}]
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            config = root / "plugin-00/home/.config/opencode/opencode.json"
            state = {"installed": False}

            def fake_call(argv, _env, _cwd, timeout=105):
                operation = argv[1]
                if operation == "add":
                    if not state["installed"]:
                        config.parent.mkdir(parents=True, exist_ok=True)
                        config.write_text('{"mcp":{"atlassian":{"type":"local"}}}')
                    state["installed"] = True
                elif operation == "remove":
                    config.unlink()
                    state["installed"] = False
                return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                        "timeout": False,
                        "_json": ({"data": {"actions": [
                            {"component_support": [{"name": "atlassian", "support": "prepared"}]}
                        ]}} if operation == "add" else
                                  {"data": {"installations": matched if state["installed"] else []}})}

            with (patch.object(audit, "call", side_effect=fake_call),
                  patch.object(audit, "matching_installations", return_value=matched)):
                row = audit.do_case("/unused/agentplugins", "opencode", "plugin",
                                    "atlassian", 0, root, {})
            self.assertTrue(row["prepared_components_only"])
            self.assertTrue(row["native_identity_files"])
            self.assertTrue(row["repeat_verified"])
            self.assertTrue(row["removal_verified"])
            self.assertEqual(row["status"], "prepared-lifecycle-passed")
            self.assertNotEqual(row["status"], "passed")


    def test_open_code_prepared_lifecycle_is_accepted_only_as_non_native_proof(self):
        audit = load_audit()
        row = {
            "status": "prepared-lifecycle-passed", "kind": "plugin",
            "target": "opencode", "source": "atlassian",
            "prepared_components_only": True, "native_component_candidates": [],
            "native_identity_files": ["opencode/opencode.json"],
            "listed_identity": True, "repeat_verified": True,
            "removal_verified": True, "native_residuals": [],
            "post_remove_identity_registered": False,
            "sentinels_preserved": True, "failed_install_residue": False,
            "install": {"exit_code": 0}, "cleanup": {"exit_code": 0},
        }
        self.assertTrue(audit.expected_negative_coverage(row))
        self.assertNotEqual(row["status"], "passed")
        for key, bad in (
                ("status", "passed"),
                ("target", "qoder"),
                ("prepared_components_only", False),
                ("native_component_candidates", ["atlassian"]),
                ("native_identity_files", ["outside/active.json"]),
                ("post_remove_identity_registered", True),
                ("listed_identity", False),
                ("repeat_verified", False),
                ("removal_verified", False),
                ("native_residuals", ["opencode/settings.json"]),
                ("sentinels_preserved", False),
                ("failed_install_residue", True),
                ("install", {"exit_code": 1}),
                ("cleanup", {"exit_code": 1})):
            with self.subTest(key=key):
                self.assertFalse(audit.expected_negative_coverage({**row, key: bad}))
        self.assertTrue(audit.expected_negative_coverage(
            {**row, "source": "docker-hub", "native_identity_files": []}))
        self.assertTrue(audit.expected_negative_coverage({
            **row, "source": "cloudflare",
            "native_identity_files": [
                "opencode/opencode.json",
                "opencode/skills/cloudflare/SKILL.md",
                "opencode/skills/cloudflare/references/api.md"]}))
        self.assertFalse(audit.expected_negative_coverage({
            **row, "native_identity_files": ["opencode/../other.json"]}))

    def test_kimi_windows_skill_downloads_exact_pinned_native_binary(self):
        # Kimi's actual 2.3.2 Windows .exe is accepted by the released adapter;
        # npm .cmd wrappers are not, so the audit must provision verified bytes.
        import runpy
        scripts = Path(__file__).resolve().parent
        matrix = runpy.run_path(str(scripts / "run-native-client-matrix.py"))
        self.assertEqual(matrix["PINS"]["windows-amd64"]["kimi"], (
            "github", "MoonshotAI/kimi-code", "@moonshot-ai/kimi-code@2.3.2",
            "kimi-code-win32-x64.zip",
            "sha256:398fff295d3ddaffe4fbefd9586e6cdc3eacd67bdb4437a43184615a1934ee74",
            "kimi.exe"))
        workflow = (scripts.parent / ".github/workflows/agentplugins-published-windows-audit.yml").read_text()
        self.assertIn('module.provision(module.PINS["windows-amd64"]["kimi"], target, "kimi")',
                      workflow)
        self.assertIn('binary != target / "kimi.exe"', workflow)
        self.assertIn('Verified genuine Kimi 2.3.2 Windows executable, sha256 pinned', workflow)
        self.assertIn('"$nativeBin" | Out-File -FilePath "$env:GITHUB_PATH"', workflow)
        self.assertNotIn("$nativePackage = '@moonshot-ai/kimi-code@2.3.2'", workflow)


    def test_codex_windows_cache_is_not_native_proof_but_real_uninstall_is(self):
        audit = load_audit()
        source = "github:example/agent-plugin@" + ("a" * 40)
        matched = [{"name": "atlassian", "installation_id": "one",
                    "clients": [{"client_id": "codex",
                                 "verification": "installation_verified"}]}]
        for native_removed in (False, True):
            with self.subTest(native_removed=native_removed), TemporaryDirectory() as temp:
                runroot = Path(temp).resolve()
                cache = (runroot / "plugin-00/home/.codex/plugins/cache/agentplugins-123"
                         / "atlassian/0.1.0/plugin.json")
                installed = {"value": False}

                def simulate(argv, _env, _cwd, timeout=105):
                    command = argv[1]
                    if command == "add":
                        if not installed["value"]:
                            cache.parent.mkdir(parents=True, exist_ok=True)
                            cache.write_text('{"name": "atlassian"}')
                        installed["value"] = True
                    elif command == "remove":
                        installed["value"] = False
                    return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                            "timeout": False,
                            "_json": {"data": {"installations":
                                              matched if installed["value"] else []}}}

                with (patch.dict(audit.AGENT_BINARIES, {}, clear=True),
                      patch.object(audit, "call", side_effect=simulate),
                      patch.object(audit, "matching_installations", return_value=matched),
                      patch.object(audit, "uninstall_native_codex_plugin",
                                   return_value={"completed": native_removed})):
                    row = audit.do_case(
                        "/unused/agentplugins", "codex", "plugin", source, 0,
                        runroot, {source: "atlassian"})
                self.assertEqual(row["native_identity_files"], [])
                self.assertTrue(row["repeat_verified"])
                self.assertTrue(row["removal_verified"])
                self.assertEqual(row["status"], "passed" if native_removed
                                 else "package-installed-native-unqualified")


    def test_scoped_file_capture_reads_native_json_without_pipe_wait(self):
        audit = load_audit()
        runner_owner, runner_name = ((audit, "_windows_file_call") if sys.platform == "win32"
                                     else (audit.subprocess, "run"))
        with TemporaryDirectory() as scratch:
            result_json = {"schema_version": 1, "result": "success"}
            def completed(*_args, **kwargs):
                self.assertNotIn("capture_output", kwargs)
                self.assertEqual(kwargs["timeout"], 7)
                self.assertTrue(kwargs["stdout"].seekable())
                self.assertTrue(kwargs["stderr"].seekable())
                kwargs["stdout"].write(json.dumps(result_json).encode("utf-8"))
                kwargs["stderr"].write(b"native child stderr")
                return subprocess.CompletedProcess(["agentplugins"], 0)
            with patch.object(runner_owner, runner_name, side_effect=completed):
                actual = audit.call(["agentplugins", "add", "firebase"],
                                    {"TEMP": scratch}, scratch, timeout=7,
                                    capture_files=True)
            self.assertEqual(actual["exit_code"], 0)
            self.assertFalse(actual["timeout"])
            self.assertEqual(actual["_json"], result_json)
            self.assertEqual(actual["stderr_tail"], "native child stderr")
            with patch.object(runner_owner, runner_name, side_effect=subprocess.TimeoutExpired(
                    ["agentplugins", "add", "firebase"], 7)):
                actual = audit.call(["agentplugins"], {"TEMP": scratch}, scratch,
                                    timeout=7, capture_files=True)
            self.assertIsNone(actual["exit_code"])
            self.assertTrue(actual["timeout"])
            def oversized(*_args, **kwargs):
                kwargs["stdout"].write(b"x" * ((1 << 20) + 1))
                return subprocess.CompletedProcess(["agentplugins"], 0)
            with patch.object(runner_owner, runner_name, side_effect=oversized):
                actual = audit.call(["agentplugins"], {"TEMP": scratch}, scratch,
                                    capture_files=True)
            self.assertIsNone(actual["exit_code"])
            self.assertIsNone(actual["_json"])
            self.assertEqual(actual["stderr_tail"], "bounded native output capture exceeded")
            for payload in (b"invalid json", b'{"result":"failed"}'):
                def invalid(*_args, **kwargs):
                    kwargs["stdout"].write(payload)
                    return subprocess.CompletedProcess(["agentplugins"], 0)
                with patch.object(runner_owner, runner_name, side_effect=invalid):
                    actual = audit.call(["agentplugins"], {"TEMP": scratch}, scratch,
                                        capture_files=True)
                self.assertIsNone(actual["exit_code"])
                self.assertIsNone(actual["_json"])
                self.assertEqual(actual["stderr_tail"], "native add success JSON unavailable")
            def rejected(*_args, **kwargs):
                kwargs["stdout"].write(b'{"result":"failed"}')
                kwargs["stderr"].write(b"provider refused immutable source")
                return subprocess.CompletedProcess(["agentplugins"], 1)
            with patch.object(runner_owner, runner_name, side_effect=rejected):
                actual = audit.call(["agentplugins"], {"TEMP": scratch}, scratch,
                                    capture_files=True)
            self.assertEqual(actual["exit_code"], 1)
            self.assertEqual(actual["stderr_tail"], "provider refused immutable source")
        with patch.object(audit.subprocess, "run",
                          return_value=subprocess.CompletedProcess(
                              ["agentplugins"], 0, stdout='{"result":"success"}',
                              stderr="")) as run:
            actual = audit.call(["agentplugins"], {}, ".", timeout=7)
            self.assertEqual(actual["_json"], {"result": "success"})
            self.assertTrue(run.call_args.kwargs["capture_output"])

    def test_scoped_file_capture_preserves_partial_timeout_without_success_authority(self):
        audit = load_audit()
        output = b"o" * 2100 + b'{"result":"success"}\n'
        errors = b"e" * 2100 + b"partial\xff stderr\n"
        child = ("import os,time;"
                 f"os.write(1,{output!r});os.write(2,{errors!r});"
                 "time.sleep(30)")
        with TemporaryDirectory() as scratch:
            actual = audit.call(
                [sys.executable, "-I", "-B", "-c", child],
                {**os.environ, "TEMP": scratch}, scratch, timeout=1,
                capture_files=True)
            self.assertEqual(actual["stdout_tail"],
                             output[-1800:].decode("utf-8", errors="replace"))
            self.assertEqual(actual["stderr_tail"],
                             errors[-1800:].decode("utf-8", errors="replace"))
            self.assertIsNone(actual["exit_code"])
            self.assertTrue(actual["timeout"])
            self.assertIsNone(actual.get("_json"))
            self.assertNotIn("stdout_full", actual)
            self.assertNotIn("stderr_full", actual)
            self.assertFalse(audit.is_cli_refusal(actual))
            self.assertEqual(list(Path(scratch).iterdir()), [])

    def test_published_windows_firebase_timeout_is_bounded_and_scoped(self):
        audit = load_audit()
        with patch.object(audit.sys, "platform", "win32"):
            for target in ("codex", "claude", "opencode", "gemini",
                           "copilot", "cline", "kimi", "qoder", "pi"):
                for repeat in (False, True):
                    expected = 900 if target == "claude" and not repeat else 180
                    self.assertEqual(audit.install_case_timeout(
                        target, "plugin", "firebase", repeat=repeat), expected)
                    self.assertEqual(audit.install_case_timeout(
                        target, "plugin", "docker-hub", repeat=repeat), 105)
                    self.assertEqual(audit.install_case_timeout(
                        target, "skill", "firebase", repeat=repeat), 105)
        for platform in ("darwin", "linux"):
            with patch.object(audit.sys, "platform", platform):
                for repeat in (False, True):
                    self.assertEqual(audit.install_case_timeout(
                        "claude", "plugin", "firebase", repeat=repeat), 105)


class WindowsFirebaseBudgetContracts(unittest.TestCase):
    """Exercise the real lifecycle oracle with distinct cold and warm bounds."""

    def test_cold_budget_routes_once_and_timeout_never_qualifies_registration(self):
        audit = load_audit()
        pin = {"distribution": "777genius/firebase", "repository": "example/plugins",
               "path": "plugins/firebase", "revision": "a" * 40, "sequence": 1,
               "tree_digest": "sha256:" + "b" * 64, "manifest_digest": "sha256:" + "c" * 64}
        revision = {"distribution_id": pin["distribution"], "resolved_revision": pin["revision"],
                    "release_sequence": 1, "tree_digest": pin["tree_digest"],
                    "manifest_digest": pin["manifest_digest"]}
        matched = {"name": "firebase", "installation_id": "same-firebase-installation",
                   "source": "example/plugins/plugins/firebase", "clients": [{
                       "client_id": "claude", "scope": "user", "materialization": "materialized",
                       "activation": "active", "verification": "installation_verified",
                       "policy": "allowed", "authentication": "not_required", "package_revision": revision}],
                   "directory": {"product_id": "firebase", "recorded_distribution": pin["distribution"],
                                 "recorded_revision": pin["revision"], "recorded_release_sequence": 1}}
        for timeout_at in (None, 1, 2):
            with self.subTest(timeout_at=timeout_at), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                config = root / "plugin-06/home/.claude/firebase.json"
                state = {"installed": False, "adds": 0}
                calls = []

                def fake_call(argv, _env, _cwd, timeout=105, **options):
                    operation = argv[1]
                    calls.append((operation, timeout, options))
                    timed_out = False
                    if operation == "add":
                        state["adds"] += 1
                        config.parent.mkdir(parents=True, exist_ok=True)
                        config.write_text('{"firebase":{"command":"inert"}}', encoding="utf-8")
                        state["installed"] = True
                        timed_out = state["adds"] == timeout_at
                        data = {"actions": [{"component_support": [{"name": "firebase", "support": "native"}]}]}
                    else:
                        if operation == "remove":
                            config.unlink()
                            state["installed"] = False
                        data = {"installations": [matched] if state["installed"] else []}
                    # Even valid success JSON and durable registration cannot
                    # turn an expired command into a qualified installation.
                    return {"exit_code": None if timed_out else 0, "stdout_tail": "", "stderr_tail": "",
                            "timeout": timed_out, "_json": {"result": "success", "data": data}}

                with (patch.object(audit.sys, "platform", "win32"),
                      patch.object(audit, "call", side_effect=fake_call)):
                    row = audit.do_case("/unused/agentplugins", "claude", "plugin", "firebase",
                                        6, root, {}, {"firebase": pin})
                expected = [("add", 900, {"capture_files": True}), ("list", 30, {})]
                if timeout_at != 1:
                    expected += [("add", 180, {"capture_files": True}), ("list", 30, {}),
                                 ("remove", 60, {}), ("list", 30, {})]
                self.assertEqual(calls, expected)
                self.assertTrue(row["sentinels_preserved"])
                if timeout_at is None:
                    self.assertEqual(row["status"], "passed")
                    self.assertTrue(row["repeat_verified"])
                    self.assertTrue(row["removal_verified"])
                elif timeout_at == 1:
                    self.assertEqual(row["status"], "failed-install-residue")
                    self.assertTrue(row["failed_install_registered"])
                    self.assertFalse(row["repeat_verified"])
                    self.assertFalse(row["removal_verified"])
                else:
                    self.assertEqual(row["status"], "repeat-add-unqualified")
                    self.assertFalse(row["repeat_verified"])
                    self.assertTrue(row["removal_verified"])



@unittest.skipUnless(sys.platform == "linux", "Linux cache retirement custody contract")
class ClaudeCacheRetirementAuditContracts(unittest.TestCase):
    def fixture(self, root, audit):
        import shutil
        home, project = root / 'home', root / 'project'
        profile = home / '.claude'
        profile.mkdir(parents=True, exist_ok=True)
        foreign = profile / 'foreign-neighbor.txt'
        if not foreign.exists():
            foreign.write_text('unmanaged native neighbor must survive\n')
        before = audit.scoped_snapshot((profile,))
        name, physical, market = 'skill-owned', 'skill-owned-physical', 'agentplugins-' + 'a' * 12
        active = project / '.claude/skills' / physical
        bundle = active / '.agentplugins-plugin'
        payload = bundle / '.agentplugins-runtime/skills/owned-test/SKILL.md'
        payload.parent.mkdir(parents=True)
        payload.write_text('---\nname: owned-test\n---\n')
        cache = profile / 'plugins/cache' / market / name / '0.0.0'
        cache.parent.mkdir(parents=True)
        shutil.copytree(bundle, cache)
        revision = {'resolved_revision': 'a' * 40, 'tree_digest': 'sha256:' + 'b' * 64,
                    'manifest_digest': 'sha256:' + 'c' * 64}
        client = {'client_id': 'claude', 'scope': 'project', 'materialization': 'materialized',
                  'activation': 'active', 'authentication': 'not_required', 'policy': 'allowed',
                  'verification': 'installation_verified', 'package_revision': revision}
        public = {'installation_id': 'owned-install', 'name': name, 'source': 'example/skills/skills/owned-test',
                  'clients': [client]}
        receipt = {'kind': 'claude_project_plugin_cache', 'object_id': 'claude-project-cache:' + name + '@' + market,
                   'logical_name': name, 'path': str(bundle), 'source_relative': cache.relative_to(profile).as_posix(),
                   'managed_digest': 'sha256:' + 'd' * 64, 'protection_class': 'managed'}
        binding = dict(client, client_binding_id='owned-binding', target_locator=str(active),
                       physical_artifact_id=physical, native_objects=[receipt])
        record = {'installation_id': public['installation_id'], 'declared_name': name,
                  'source': {'repository': 'example/skills', 'package_subpath': 'skills/owned-test',
                             'resolved_revision': revision['resolved_revision'], 'tree_digest': revision['tree_digest']},
                  'package': {'declared_name': name, 'manifest_digest': revision['manifest_digest']},
                  'clients': {'owned-binding': binding}}
        state = project / '.uap/state-v2.json'
        state.parent.mkdir(parents=True)
        state.write_text(json.dumps({'schema_version': 5, 'installations': [record]}))
        key = name + '@' + market
        installed = profile / 'plugins/installed_plugins.json'
        installed.write_text(json.dumps({'version': 2, 'plugins': {key: [{'scope': 'project',
            'projectPath': str(project), 'installPath': str(cache)}]}, 'retained': 'same'}))
        markets = profile / 'plugins/known_marketplaces.json'
        markets.write_text(json.dumps({market: {'source': {'source': 'directory', 'path': str(active)},
                                               'installLocation': str(active)}, 'foreign': {'retained': True}}))
        cleanup = {'schema_version': 1, 'result': 'success', 'outcome': 'completed', 'scope': 'project',
                   'actions': [{'installation_id': public['installation_id'], 'binding_id': 'owned-binding',
                                'client_id': 'claude', 'action': 'remove'}]}
        added = {'readiness': 'ready', 'next_actions': [],
                 'items': [{'targetId': 'claude', 'alias': 'owned-test', 'destinationPath': str(active)}]}
        return dict(cleanup=cleanup, home=home, project=project, profile=profile, before=before, public=public, added=added,
                    active=active, cache=cache, state=state, installed=installed, markets=markets, market=market, key=key)

    def capture(self, audit, f):
        def document(path):
            text, error = audit.copilot_settings_read(path, f['home'].parent)
            self.assertIsNone(error)
            return json.loads(text, object_pairs_hook=audit._unique_json_object)
        return audit.CLAUDE_RETIREMENT_AUDIT['CacheRetirement'](
            f['home'].parent, f['home'], f['project'], f['public'], f['added'],
            {'repo': 'example/skills', 'folder': 'owned-test'}, document,
            audit.scoped_snapshot, f['before'],
            lambda observed: audit.native_identity_residuals(
                [f['profile']], 'owned-test', observed, 'claude'))

    def retire(self, f, marker=True):
        import shutil
        quarantine = f['cache'].parent / ('.agentplugins-cache-retire-' + '1' * 32)
        quarantine.mkdir(mode=0o700)
        moved = quarantine / 'retired'
        f['cache'].rename(moved)
        if marker:
            (moved / '.orphaned_at').write_text('1794880808341')
        doc = json.loads(f['installed'].read_text()); del doc['plugins'][f['key']]
        f['installed'].write_text(json.dumps(doc))
        doc = json.loads(f['markets'].read_text()); del doc[f['market']]
        f['markets'].write_text(json.dumps(doc))
        shutil.rmtree(f['active'])
        return moved

    def residuals(self, audit, f):
        return audit.native_identity_residuals([f['profile']], 'owned-test',
                                               audit.snapshot((f['profile'],)), 'claude')

    def test_exact_receipt_inode_bytes_and_optional_marker_are_checkpoint_only(self):
        audit = load_audit()
        for marker in (False, True):
            with self.subTest(marker=marker), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve(), audit)
                subject = self.capture(audit, f)
                original_inode = f['cache'].stat().st_ino
                self.retire(f, marker)
                residuals = self.residuals(audit, f)
                self.assertTrue(residuals)
                proof = subject.finish(residuals, f['cleanup'])
                self.assertEqual(proof['status'], 'verified-cache-retirement-at-checkpoint')
                self.assertEqual(proof['cache_inode'], original_inode)
                self.assertEqual(proof['orphaned_marker_present'], marker)
                self.assertFalse(proof['writer_closure_claimed'])
                self.assertEqual(subject.fds, [])
                self.assertEqual(residuals, self.residuals(audit, f))

    def test_fresh_snapshot_rescans_late_identity_and_unsafe_paths(self):
        audit = load_audit()
        for mutation in ('unrelated', 'identity-path', 'identity-content', 'symlink'):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve(), audit)
                subject = self.capture(audit, f)
                self.retire(f)
                residuals = self.residuals(audit, f)
                self.assertTrue(residuals)
                original = subject.snapshotter
                def late_snapshot(roots):
                    if mutation == 'identity-path':
                        (f['profile'] / 'owned-test.json').write_text('{}')
                    elif mutation == 'identity-content':
                        (f['profile'] / 'late-config.json').write_text('{"name":"owned-test"}')
                    elif mutation == 'symlink':
                        (f['profile'] / 'late-link').symlink_to(f['state'])
                    else:
                        (f['profile'] / 'unrelated-new.txt').write_text('unrelated')
                    return original(roots)
                subject.snapshotter = late_snapshot
                proof = subject.finish(residuals, f['cleanup'])
                self.assertEqual(proof['status'],
                                 'verified-cache-retirement-at-checkpoint' if mutation == 'unrelated'
                                 else 'unresolved-cache-retirement')
                if mutation == 'unrelated':
                    self.assertFalse(proof['writer_closure_claimed'])
                self.assertEqual(subject.fds, [])
                self.assertTrue(all('late-' not in path and not path.endswith('/owned-test.json')
                                    for path in residuals))

    def test_wrong_or_missing_receipt_never_creates_an_empty_authority(self):
        audit = load_audit()
        for mutation in ('missing', 'duplicate', 'source', 'path', 'modified', 'logical-name', 'native-binding', 'preexisting'):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve(), audit)
                state = json.loads(f['state'].read_text()); record = state['installations'][0]
                objects = record['clients']['owned-binding']['native_objects']
                if mutation == 'missing': objects.clear()
                elif mutation == 'duplicate': objects.append(dict(objects[0]))
                elif mutation == 'source': record['source']['resolved_revision'] = 'e' * 40
                elif mutation == 'path': objects[0]['path'] = str(f['project'] / 'foreign')
                elif mutation == 'modified': objects[0]['user_modified'] = True
                elif mutation == 'logical-name': objects[0]['logical_name'] = 'foreign'
                elif mutation == 'native-binding':
                    doc = json.loads(f['installed'].read_text());doc['plugins'][f['key']][0]['scope'] = 'user'
                    f['installed'].write_text(json.dumps(doc))
                else:
                    f['before'] = audit.scoped_snapshot((f['profile'],))
                f['state'].write_text(json.dumps(state))
                with self.assertRaises((ValueError, KeyError)):
                    self.capture(audit, f)

    def test_ambiguous_unsafe_modified_or_extra_residue_keeps_negative(self):
        audit = load_audit()
        for mutation in ('ambiguous', 'private-mode', 'bytes', 'extra', 'symlink', 'marker',
                         'registry', 'foreign', 'external-residual', 'cleanup-binding', 'new-marker-race', 'late-residual-scan'):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve(), audit)
                subject = self.capture(audit, f); moved = self.retire(f)
                if mutation == 'ambiguous': (f['cache'].parent / ('.agentplugins-cache-retire-' + '2' * 32)).mkdir()
                elif mutation == 'private-mode': moved.parent.chmod(0o755)
                elif mutation == 'bytes': (moved / '.agentplugins-runtime/skills/owned-test/SKILL.md').write_text('changed')
                elif mutation == 'extra': (moved / 'extra').write_text('unowned')
                elif mutation == 'symlink': (moved / 'link').symlink_to(f['state'])
                elif mutation == 'marker': (moved / '.orphaned_at').write_text('0000000000000')
                elif mutation == 'registry': f['installed'].write_text('{"version":2,"plugins":{},"retained":"changed"}')
                elif mutation == 'cleanup-binding': f['cleanup']['actions'][0]['binding_id'] = 'foreign'
                elif mutation == 'foreign': (f['profile'] / 'foreign-neighbor.txt').write_text('changed')
                residuals = self.residuals(audit, f)
                if mutation == 'external-residual': residuals.append('.claude/other-owned-test.json')
                if mutation == 'late-residual-scan':
                    subject.residual_scanner = lambda observed: list(residuals) + ['.claude/other-owned-test.json']
                if mutation == 'new-marker-race':
                    # A post-read foreign mutation cannot be hidden by a matching inode.
                    original = subject.snapshotter
                    def mutate(roots):
                        result = original(roots)
                        (moved / 'late').write_text('unowned')
                        return result
                    subject.snapshotter = mutate
                proof = subject.finish(residuals, f['cleanup'])
                self.assertEqual(proof['status'], 'unresolved-cache-retirement')
                self.assertEqual(subject.fds, [])

    def test_retained_registry_boolean_to_integer_changes_are_not_preserved(self):
        audit = load_audit()
        for registry in ('installed', 'markets'):
            with self.subTest(registry=registry), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve(), audit)
                if registry == 'installed':
                    document = json.loads(f['installed'].read_text())
                    document['plugins']['foreign@other'] = [{'enabled': True}]
                    f['installed'].write_text(json.dumps(document))
                subject = self.capture(audit, f)
                self.retire(f)
                path = f[registry]
                document = json.loads(path.read_text())
                if registry == 'installed': document['plugins']['foreign@other'][0]['enabled'] = 1
                else: document['foreign']['retained'] = 1
                path.write_text(json.dumps(document))
                proof = subject.finish(self.residuals(audit, f), f['cleanup'])
                self.assertEqual(proof['status'], 'unresolved-cache-retirement')
                self.assertEqual(subject.fds, [])

    def test_snapshot_limits_and_hardlinks_are_refused(self):
        audit = load_audit()
        for mutation in ('file', 'total', 'count', 'depth', 'hardlink', 'git', 'lock', 'empty'):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve(), audit)
                globals_ = audit.CLAUDE_RETIREMENT_AUDIT['CacheRetirement'].__init__.__globals__
                changes = {'MAX_FILE_BYTES': 1} if mutation == 'file' else {'MAX_TREE_BYTES': 1} if mutation == 'total' else {'MAX_FILES': 1} if mutation == 'count' else {'MAX_DEPTH': 1} if mutation == 'depth' else {}
                if mutation == 'git': (f['cache'] / '.git').mkdir()
                elif mutation == 'lock': (f['cache'] / '.uap-integrations.lock').write_text('unowned')
                elif mutation == 'empty':
                    import shutil
                    shutil.rmtree(f['cache']);f['cache'].mkdir()
                    bundle = f['active'] / '.agentplugins-plugin'
                    shutil.rmtree(bundle);bundle.mkdir()
                if mutation == 'hardlink':
                    os.link(f['cache'] / '.agentplugins-runtime/skills/owned-test/SKILL.md',
                            f['cache'] / 'hardlink')
                with patch.dict(globals_, changes), self.assertRaises(ValueError):
                    self.capture(audit, f)

    def test_do_case_requires_every_old_gate_and_never_hides_raw_residuals(self):
        audit = load_audit()
        for mutation in (None, 'post-list', 'sentinel', 'outside',
                         'reminted-source', 'source-neighbor', 'other-target'):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(); f = {}; operations = []
                def perform(argv, env, cwd, timeout=105, **kwargs):
                    self.assertEqual(argv[0], '/inert/uap')
                    operation = argv[2] if argv[1] == 'skills' else argv[1]
                    operations.append(operation)
                    if operation == 'add' and not f:
                        f.update(self.fixture(root / 'skill-00', audit))
                        (f['project'] / 'uap.toml').write_text('owned-test')
                        (f['project'] / 'uap.lock').write_text('owned-test')
                    elif operation == 'remove':
                        self.retire(f)
                        (f['project'] / 'uap.lock').unlink()
                        if mutation == 'sentinel': (f['project'] / 'foreign-untouched.txt').write_text('changed')
                        if mutation == 'outside': (f['profile'] / 'owned-test.json').write_text('owned-test')
                    removed = 'remove' in operations
                    retained = []
                    if removed and mutation in ('reminted-source', 'source-neighbor', 'other-target'):
                        entry = dict(f['public'], installation_id='reminted-id', name='renamed-generated-skill')
                        if mutation == 'source-neighbor':
                            entry['source'] += '-other'
                        elif mutation == 'other-target':
                            entry['clients'] = [dict(entry['clients'][0], client_id='codex')]
                        retained.append(entry)
                    return {'exit_code': 1 if removed and operation == 'list' and mutation == 'post-list' else 0,
                            'stdout_tail': '', 'stderr_tail': '', 'timeout': False,
                            '_json': f['added'] if operation == 'add' else f['cleanup'] if operation == 'remove' else
                                     {'data': {'installations': retained if removed else [f['public']]}}}
                with (patch.dict(audit.AGENT_BINARIES, {}, clear=True),
                      patch.object(audit, 'call', side_effect=perform),
                      patch.object(audit, 'matching_installations', side_effect=lambda *a: [f['public']]),
                      patch.object(audit, 'matches_pinned_skill_bytes', return_value=True),
                      patch.object(audit, 'locked_skill_is_pinned', side_effect=lambda *a: (f['project'] / 'uap.lock').exists())):
                    item = {'name': 'owned-test', 'repo': 'example/skills', 'folder': 'owned-test',
                            'source': 'unused', 'skill_md_git_blob_sha1': '0' * 40}
                    row = audit.do_case('/inert/uap', 'claude', 'skill', item, 0, root, {})
                self.assertEqual(operations, ['add', 'list', 'add', 'list', 'remove', 'list'])
                for phase in ("after-add", "repeat-add"):
                    self.assertEqual(row["public_target_states"][phase]["selected_installations"], 1)
                    self.assertEqual(row["public_target_states"][phase]["selected_bindings"], 1)
                self.assertEqual(row["public_target_states"]["after-remove"]["selected_bindings"],
                                 1 if mutation == 'reminted-source' else 0)
                self.assertTrue(row['native_residuals'])
                accepted = mutation in (None, 'source-neighbor', 'other-target')
                self.assertEqual(row['status'], 'passed' if accepted else 'cleanup-failed')
                if accepted:
                    self.assertEqual(row['claude_cache_retirement']['status'],
                                     'verified-cache-retirement-at-checkpoint')
                else:
                    self.assertFalse(row['removal_verified'])
                    if mutation == 'reminted-source':
                        self.assertIsNone(row.get('claude_cache_retirement'))


class CodexPluginSQLiteLifecycleTest(unittest.TestCase):
    def run_case(self, mutation=None, system=False, kind="plugin", historical=False):
        import sqlite3
        from contextlib import closing
        audit = load_audit()
        source = "github:example/agent-plugin@" + "a" * 40
        identity = "skill-creator" if kind == "skill" else "github" if system else "atlassian"
        matched = [{"name": identity, "installation_id": "one",
                    "clients": [{"client_id": "codex",
                                 "scope": "project" if kind == "skill" else "user",
                                 "materialization": "materialized", "activation": "active",
                                 "authentication": "not_required", "policy": "allowed",
                                 "verification": "installation_verified"}]}]
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / (kind + "-00")
            profile = case / "home/.codex"
            database = case / "home/.local/state/codex-sqlite/state_5.sqlite"
            commands, registered = [], [False]
            system_relative = "skill-creator/scripts/generate_openai_yaml.py"
            system_body = b'ACRONYMS = {\n    "github": "GitHub",\n}\n'
            system_file = profile / "skills/.system" / system_relative
            def write_system():
                system_file.parent.mkdir(parents=True, exist_ok=True)
                system_file.write_bytes(system_body)
                (profile / "skills/.system/.codex-system-skills.marker").write_bytes(
                    audit.CODEX_RUNTIME_AUDIT["SYSTEM_SKILL_MARKER"])
            ambient = root / "ambient-profile"
            ambient.mkdir()
            (ambient / "foreign.txt").write_text("ambient untouched")
            ambient_before = audit.scoped_snapshot((ambient,))
            async def discover(binary, project, native_profile, environment, **kwargs):
                self.assertEqual(native_profile, profile)
                self.assertEqual(environment["CODEX_HOME"], str(profile))
                self.assertEqual(kwargs, {"sqlite_baseline": True} if kind == "plugin" else {})
                if kind == "plugin":
                    installation_id = (profile / "installation_id").read_bytes()
                    parsed = audit.uuid.UUID(installation_id.decode("ascii"))
                    self.assertEqual(installation_id, str(parsed).encode("ascii"))
                    self.assertEqual(parsed.version, 4)
                    self.assertEqual(list((profile / "tmp/arg0").iterdir()), [])
                    if os.name != "nt":
                        self.assertEqual((profile / "installation_id").stat().st_mode & 0o777, 0o644)
                        self.assertEqual((profile / "tmp/arg0").stat().st_mode & 0o777, 0o700)
                if not database.exists():
                    with closing(sqlite3.connect(database)) as connection:
                        connection.execute("CREATE TABLE fixture_rows (value TEXT)")
                        connection.execute("INSERT INTO fixture_rows VALUES ('stable')")
                        connection.commit()
                if mutation == "prebaseline-config":
                    (profile / "config.toml").write_text('sqlite_home = "/foreign"\n')
                elif mutation == "prebaseline-foreign":
                    self.assertTrue((profile / "foreign-neighbor.txt").is_file())
                    (profile / "foreign-neighbor.txt").write_text("changed")
                elif mutation == "prebaseline-foreign-new":
                    foreign = profile / "foreign-prebaseline-new.txt"
                    self.assertFalse(foreign.exists())
                    foreign.write_text("new foreign bytes")
                elif mutation == "prebaseline-foreign-nested":
                    foreign = profile / "foreign-prebaseline-directory"
                    self.assertFalse(foreign.exists())
                    foreign.mkdir()
                    (foreign / "nested.txt").write_text("new foreign bytes")
                elif mutation == "prebaseline-runtime-foreign":
                    (database.parent / "foreign-prebaseline-new.txt").write_text("foreign SQLite neighbor")
                elif mutation == "prebaseline-home-new":
                    (case / "home/foreign-prebaseline-new.txt").write_text("new foreign bytes")
                elif mutation == "prebaseline-managed":
                    (case / "uap-data/foreign-managed.txt").write_text("changed")
                elif mutation == "prebaseline-installation-id":
                    (profile / "installation_id").write_text(str(audit.uuid.uuid4()))
                elif mutation == "prebaseline-arg0-child":
                    (profile / "tmp/arg0/foreign").write_text("unowned residue")
                if system and system != "absent" and not system_file.exists():
                    write_system()
                skills = []
                if kind == "skill":
                    for name in ("foreign-neighbor", identity):
                        path = project / ".agents/skills" / name / "SKILL.md"
                        if path.is_file():
                            skills.append({"name": name, "path": str(path), "enabled": True,
                                           "scope": "repo", "pluginId": None})
                return {"data": [{"cwd": str(project), "errors": [], "skills": skills}]}
            class HistoricalObserver:
                def __init__(self, launcher, environment, project):
                    self.environment, self.project = environment, project
                    self.launcher, self.commands, self.proof = launcher, 0, None
                def after_command(self):
                    self.commands += 1
                    self.proof = {"commands_verified": self.commands}
            def prepare_archive(observer, invoke, selected_source, pin):
                self.assertEqual(selected_source, identity if historical else source)
                self.assertEqual(pin, {"integration": "selected-fixture"})
                self.assertFalse((profile / "config.toml").exists())
                for _ in range(2):
                    output = invoke([observer.launcher, "info", source], observer.environment,
                                    observer.project, 80, capture_full=True)
                    self.assertEqual(output["stdout_full"], "complete info fixture")
                    self.assertEqual(output["stderr_full"], "")
                return {"integration": "archive prepared before native bootstrap"}
            def perform(argv, environment, cwd, timeout=105, **options):
                commands.append(list(argv))
                self.assertNotIn("--disable", argv)
                command = argv[1]
                if historical:
                    self.assertIs(options.get("capture_full"), True)
                    if command == "info":
                        return {"exit_code": 0, "timeout": False, "stdout_tail": "info fixture", "stderr_tail": "",
                                "stdout_full": "complete info fixture", "stderr_full": "", "_json": {"result": "success"}}
                config = profile / "config.toml"
                self.assertNotIn("features", audit.tomllib.loads(config.read_text()))
                if command == "skills":
                    selected = case / "project/.agents/skills" / identity / "SKILL.md"
                    selected.parent.mkdir(parents=True, exist_ok=True)
                    selected.write_text("---\nname: " + identity + "\n---\n")
                    (case / "project/uap.toml").write_text("saved")
                    (case / "project/uap.lock").write_text("pinned")
                    registered[0] = True
                    return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "", "timeout": False,
                            "_json": {"schema_version": 1, "command": "skills add", "result": "success",
                                      "status": "succeeded", "scope": "project", "next_actions": [],
                                      "readiness": "needs_action" if mutation == "system-needs-action" else "ready",
                                      "items": [{"alias": identity, "targetId": "codex", "status": "succeeded"}]}}
                if command == "add":
                    if not registered[0]:
                        with config.open("a") as stream:
                            stream.write(f'\n[plugins."{identity}@agentplugins-aaaaaaaaaaaa"]\nenabled = true\n')
                    registered[0] = True
                    if mutation == "sqlite-row":
                        with closing(sqlite3.connect(database)) as connection:
                            connection.execute("UPDATE fixture_rows SET value='changed'")
                            connection.commit()
                elif command == "remove":
                    registered[0] = False
                    if kind == "skill":
                        selected = case / "project/.agents/skills" / identity / "SKILL.md"
                        selected.unlink()
                        selected.parent.rmdir()
                        (case / "project/uap.lock").unlink()
                    if mutation == "system-changed":
                        system_file.write_bytes(system_body + b"# changed\n")
                    elif mutation == "system-extra":
                        (system_file.parent / "unknown.py").write_text("foreign")
                    elif mutation == "system-swapped":
                        replacement = system_file.with_suffix(".replacement")
                        replacement.write_bytes(system_body)
                        replacement.replace(system_file)
                    elif mutation == "system-late-presence":
                        write_system()
                    elif mutation == "system-unrelated-identity":
                        (profile / "foreign.json").write_text('{"name":"github"}')
                    if mutation == "profile-wal":
                        (profile / "state_5.sqlite-wal").write_bytes(b"x" * ((1 << 20) + 1))
                return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                        **({"stdout_full": "private complete receipt", "stderr_full": ""} if historical else {}),
                        "timeout": False, "_json": {"data": {"installations":
                            matched if registered[0] else []}}}
            def uninstall(*args):
                commands.append(["/inert/codex", "native-uninstall"])
                config = profile / "config.toml"
                config.write_text(config.read_text().split("\n[plugins.", 1)[0] + "\n")
                return {"completed": True, "native_plugin_absent": True,
                        "native_marketplace_absent": True}
            exchange_globals = audit.CODEX_RUNTIME_AUDIT["CodexPluginSQLiteAudit"].__init__.__globals__
            manifest = {system_relative: (len(system_body), audit.hashlib.sha256(system_body).hexdigest())}
            original_scan = audit.native_identity_residuals
            def scan(*args, **kwargs):
                result = original_scan(*args, **kwargs)
                if mutation == "system-scan-swap":
                    replacement = system_file.with_suffix(".replacement")
                    replacement.write_bytes(system_body)
                    replacement.replace(system_file)
                return result
            with (patch.dict(exchange_globals, {"_exchange": discover}),
                  patch.dict(exchange_globals, {
                      "SYSTEM_SKILL_FILES": manifest if system else exchange_globals["SYSTEM_SKILL_FILES"],
                      "SYSTEM_SKILL_DIRECTORIES": frozenset(("skill-creator", "skill-creator/scripts"))
                      if system else exchange_globals["SYSTEM_SKILL_DIRECTORIES"]}),
                  patch.object(audit, "native_identity_residuals", side_effect=scan),
                  patch.dict(audit.AGENT_BINARIES, {"codex": "/inert/codex"}, clear=True),
                  patch.object(audit, "call", side_effect=perform),
                  patch.object(audit, "matching_installations", return_value=matched),
                  patch.object(audit, "locked_skill_is_pinned", return_value=True),
                  patch.object(audit, "matches_pinned_skill_bytes", return_value=True),
                  patch.object(audit, "uninstall_native_codex_plugin", side_effect=uninstall),
                  patch.dict(audit.HISTORICAL_CODEX, {"PublishedBaseline": HistoricalObserver,
                      "prepare_plugin_archive": prepare_archive} if historical else {}),
                  patch.dict(os.environ, {"HOME": str(ambient), "USERPROFILE": str(ambient)})):
                try:
                    item = {"name": identity, "repo": "example/skills", "folder": identity,
                            "source": source, "skill_md_git_blob_sha1": "b" * 40}
                    return audit.do_case("/inert/uap", "codex", kind,
                                         (identity if historical else
                                          (item if kind == "skill" else source)), 0, root,
                                         {source: identity},
                                         {(identity if historical else source): {"integration": "selected-fixture"}},
                                         **({"historical_plugin_archive": True} if historical else {})), commands
                finally:
                    if mutation and mutation.startswith("prebaseline-"):
                        self.assertEqual(commands, [])
                    self.assertEqual(audit.scoped_snapshot((ambient,)), ambient_before)

    def test_plugin_baseline_and_every_command_checkpoint_preserve_raw_scanner(self):
        row, commands = self.run_case()
        self.assertEqual(row["status"], "passed")
        proof = row["codex_plugin_sqlite_proof"]
        self.assertEqual(proof["native_version"], "0.153.4")
        self.assertTrue(proof["runtime_semantics_unchanged"])
        self.assertEqual([observation["phase"] for observation in proof["observations"]],
                         ["before-install"] + ["after-public-command"] * 4
                         + ["after-native-uninstall"] + ["after-public-command"] * 2)
        self.assertEqual([argv[1] for argv in commands],
                         ["add", "list", "add", "list", "native-uninstall", "remove", "list"])
        self.assertTrue(row["native_uninstall"]["completed"])
        self.assertTrue(row["repeat_verified"])
        self.assertTrue(row["removal_verified"])
        self.assertTrue(row["sentinels_preserved"])
        row, _ = self.run_case("profile-wal")
        self.assertEqual(row["status"], "cleanup-failed")
        self.assertIn(".codex/state_5.sqlite-wal: unscanned-too-large", row["native_residuals"])

    def test_changed_rows_pin_and_prebaseline_foreign_authority_refuse(self):
        with self.assertRaisesRegex(ValueError, "table contents changed"):
            self.run_case("sqlite-row")
        with self.assertRaisesRegex(ValueError, "config authority changed"):
            self.run_case("prebaseline-config")
        with self.assertRaisesRegex(ValueError, "unexpected member"):
            self.run_case("prebaseline-runtime-foreign")
        for mutation in ("prebaseline-foreign", "prebaseline-foreign-new",
                         "prebaseline-foreign-nested", "prebaseline-home-new", "prebaseline-managed",
                         "prebaseline-installation-id", "prebaseline-arg0-child"):
            with self.subTest(mutation=mutation), self.assertRaisesRegex(
                    ValueError, "changed protected fixture authority"):
                self.run_case(mutation)

    def test_historical_archive_keeps_eight_public_probes_and_native_sqlite_checkpoints(self):
        row, commands = self.run_case(historical=True)
        self.assertEqual([argv[1] for argv in commands],
                         ["info", "info", "add", "list", "add", "list", "native-uninstall", "remove", "list"])
        self.assertEqual(row["historical_npm_proof"], {"commands_verified": 8})
        self.assertEqual(row["historical_plugin_feed"], {"integration": "archive prepared before native bootstrap"})
        self.assertEqual(row["historical_archive_status"], "passed")
        self.assertEqual(row["status"], "historical-archive-lifecycle-observed")
        self.assertEqual([item["phase"] for item in row["codex_plugin_sqlite_proof"]["observations"]],
                         ["before-install"] + ["after-public-command"] * 4
                         + ["after-native-uninstall"] + ["after-public-command"] * 2)
        for phase in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
            self.assertTrue({"stdout_full", "stderr_full", "_json"}.isdisjoint(row[phase]), phase)
        row, _ = self.run_case("profile-wal", historical=True)
        self.assertEqual(row["status"], "cleanup-failed")
        self.assertEqual(row["historical_archive_status"], "cleanup-failed")


    def test_pinned_system_github_literal_is_retained_only_with_complete_unchanged_proof(self):
        row, _ = self.run_case(system=True)
        residual = ".codex/skills/.system/skill-creator/scripts/generate_openai_yaml.py"
        self.assertEqual(row["native_residuals"], [residual])
        self.assertEqual(row["status"], "passed")
        self.assertTrue(row["removal_verified"])
        proof = row["codex_system_skill_retention"]
        self.assertEqual(proof["status"], "verified-unchanged-pinned-system-files-at-checkpoint")
        self.assertEqual(set(proof["residuals"]), {residual})
        self.assertFalse(proof["writer_closure_claimed"])
        self.assertFalse(proof["atomic_snapshot_claimed"])
        for mutation in ("system-changed", "system-extra", "system-swapped", "system-scan-swap"):
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                self.run_case(mutation, system=True)
        with self.assertRaisesRegex(ValueError, "authority changed"):
            self.run_case("system-late-presence", system="absent")
        row, _ = self.run_case("system-unrelated-identity", system=True)
        self.assertEqual(row["status"], "cleanup-failed")
        self.assertIn(".codex/foreign.json", row["native_residuals"])
        self.assertNotIn("codex_system_skill_retention", row)


    def test_skill_builtin_retention_never_upgrades_public_needs_action(self):
        row, _ = self.run_case(system=True, kind="skill")
        self.assertEqual(row["status"], "passed")
        self.assertTrue(row["removal_verified"])
        self.assertTrue(row["native_residuals"])
        self.assertIn("codex_system_skill_retention", row)
        self.assertTrue(row["codex_public_add_ready"])
        self.assertTrue(row["codex_public_repeat_ready"])
        row, _ = self.run_case("system-needs-action", system=True, kind="skill")
        self.assertEqual(row["status"], "incomplete-native-proof")
        self.assertFalse(row["codex_public_add_ready"])
        self.assertFalse(row["codex_public_repeat_ready"])
        self.assertTrue(row["removal_verified"])
        self.assertIn("codex_system_skill_retention", row)


@unittest.skipUnless(sys.platform == "win32", "owned file capture requires Windows jobs")
class WindowsOwnedFileCaptureContracts(unittest.TestCase):
    def environment(self, root):
        env = {key: value for key, value in os.environ.items()
               if key.upper() in {"PATH", "SYSTEMROOT", "WINDIR"}}
        env.update(HOME=str(root), USERPROFILE=str(root), TEMP=str(root), TMP=str(root))
        return env

    def test_timeout_and_exited_launcher_reap_only_owned_descendants(self):
        import ctypes as C
        from ctypes import wintypes as W
        from concurrent.futures import ThreadPoolExecutor
        import time

        api = C.WinDLL("kernel32", use_last_error=True)
        api.OpenProcess.argtypes, api.OpenProcess.restype = [W.DWORD, W.BOOL, W.DWORD], W.HANDLE
        api.WaitForSingleObject.argtypes, api.WaitForSingleObject.restype = [W.HANDLE, W.DWORD], W.DWORD
        api.CloseHandle.argtypes, api.CloseHandle.restype = [W.HANDLE], W.BOOL
        for exit_leader in (False, True):
            with self.subTest(exit_leader=exit_leader), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                audit = load_audit()
                gate, ready = root / "release-owned", root / "owned-ready"
                ambient = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(120)"],
                                           stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                                           stderr=subprocess.DEVNULL)
                child = "import time; time.sleep(120)"
                body = ("import json,os,subprocess,sys,time; from pathlib import Path; "
                        f"child=subprocess.Popen([sys.executable,'-c',{child!r}],stdin=subprocess.DEVNULL); "
                        f"pending=Path({str(root / 'owned-pids.pending')!r}); "
                        "pending.write_text(json.dumps([os.getpid(),child.pid])); "
                        f"pending.replace({str(root / 'owned-pids.json')!r}); "
                        f"Path({str(ready)!r}).write_text('ready'); "
                        "print('{\"result\":\"success\",\"private\":\"timeout-owned-bytes\"}',flush=True)\n"
                        f"deadline=time.monotonic()+3\n"
                        f"while not Path({str(gate)!r}).exists():\n"
                        "    if time.monotonic() >= deadline: raise RuntimeError('inert owner gate expired')\n"
                        "    time.sleep(.01)\n"
                        + ("os._exit(0)\n" if exit_leader else "time.sleep(120)\n"))
                handles = {}
                try:
                    with ThreadPoolExecutor(max_workers=1) as worker:
                        future = worker.submit(audit.call, [sys.executable, "-c", body],
                                               self.environment(root), root, timeout=3, capture_files=True)
                        try:
                            deadline = time.monotonic() + 3
                            while not ready.exists():
                                self.assertLess(time.monotonic(), deadline, "owned fixture startup deadline")
                                self.assertFalse(future.done(), "owned fixture exited before its startup gate")
                                time.sleep(.01)
                            pids = json.loads((root / "owned-pids.json").read_text())
                            self.assertEqual(len(pids), 2)
                            for role, pid in zip(("launcher", "descendant"), pids):
                                handle = api.OpenProcess(0x00100000, False, pid)
                                self.assertTrue(handle, {"role": role, "winerror": C.get_last_error()})
                                handles[role] = handle
                                self.assertEqual(api.WaitForSingleObject(handle, 0), 258, {"role": role})
                        finally:
                            gate.write_text("release")
                        result = future.result(timeout=13)
                    waits = {role: int(api.WaitForSingleObject(handle, 0))
                             for role, handle in handles.items()}
                    refusal = result.get("stderr_tail") == "owned Windows command descendants remained after launcher exit"
                    error = result.get("stderr_tail", "")
                    token = error[10:].partition("]")[0] if error.startswith("[WinError ") else ""
                    winerror = int(token) if token.isascii() and token.isdigit() and len(token) <= 10 else None
                    winerror = winerror if winerror is None or winerror <= 0xffffffff else None
                    facts = {"owned_waits": waits, "intended_refusal": refusal,
                             "timeout": result["timeout"], "exit_code": result["exit_code"],
                             "winerror": winerror}
                    self.assertIsNone(result["exit_code"], facts)
                    self.assertIsNone(result.get("_json"), facts)
                    self.assertEqual(result["timeout"], not exit_leader, facts)
                    if exit_leader:
                        self.assertTrue(refusal, facts)
                    else:
                        self.assertIn("timeout-owned-bytes", result["stdout_tail"])
                        diagnostic = result["owned_timeout_diagnostic"]
                        self.assertEqual(diagnostic["status"], "observed-unqualified")
                        self.assertIs(diagnostic["qualified"], False)
                        self.assertEqual(diagnostic["stage"], "timeout-before-owned-termination")
                        self.assertEqual(diagnostic["bootstrap_state"], "running")
                        self.assertEqual(diagnostic["active"], 3)
                        self.assertEqual(len(diagnostic["members"]), 3)
                        self.assertTrue(all(member["state"] == "running" for member in diagnostic["members"]))
                        self.assertNotIn(str(root), json.dumps(diagnostic))
                    self.assertEqual(waits, {"launcher": 0, "descendant": 0}, facts)
                    self.assertIsNone(ambient.poll())
                finally:
                    try:
                        gate.write_text("release")
                    finally:
                        failures = []
                        for role, handle in handles.items():
                            if not api.CloseHandle(handle):
                                failures.append({"role": role, "winerror": C.get_last_error()})
                        try:
                            self.assertEqual(failures, [])
                        finally:
                            ambient.terminate()
                            ambient.wait(timeout=5)

    def test_assignment_refusal_does_not_release_command_gate(self):
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            audit = load_audit()
            attempted = []
            def refused_job(process):
                attempted.append(process)
                raise OSError("inert assignment refusal")
            marker = root / "must-not-start"
            command = [sys.executable, "-c",
                       f"from pathlib import Path; Path({str(marker)!r}).write_text('started')"]
            with patch.object(audit, "run_path", return_value={"WindowsJob": refused_job}):
                result = audit.call(command, self.environment(root), root, timeout=3, capture_files=True)
            self.assertEqual(len(attempted), 1)
            self.assertFalse(marker.exists())
            self.assertIsNotNone(attempted[0].poll())
            self.assertIsNone(result["exit_code"])
            self.assertFalse(result["timeout"])

    def test_owned_job_closer_refuses_unknown_members_generation_and_unsignaled_handles(self):
        import ctypes as C
        import time
        from types import SimpleNamespace

        class Accounting(C.Structure):
            _fields_ = [("times", C.c_int64 * 4), ("faults", C.c_uint32),
                        ("total", C.c_uint32), ("active", C.c_uint32), ("terminated", C.c_uint32)]
        for mutation in ("clean", "capture-generation", "termination-generation", "after-wait-generation",
                         "wait", "foreign", "open", "partial-open", "short-list", "oversized-list",
                         "deadline", "terminate"):
            with self.subTest(mutation=mutation):
                audit = load_audit()
                queries, terminated, closed, waited = [], [], [], []
                def query(_job, kind, output, _size, _result):
                    info = output._obj
                    if kind == 1:
                        queries.append(kind)
                        threshold = {"capture-generation": 2, "termination-generation": 3,
                                     "after-wait-generation": 4}.get(mutation, 100)
                        info.total, info.active = (3 if len(queries) >= threshold else 2), 0
                    else:
                        self.assertEqual(kind, 3)
                        info.assigned = 65 if mutation == "oversized-list" else 2 if mutation in (
                            "short-list", "partial-open") else 1
                        info.listed = 65 if mutation == "oversized-list" else 2 if mutation == "partial-open" else 1
                        info.pids[0], info.pids[1] = 101, 102
                    return 1
                def opened(_rights, inherit, pid):
                    self.assertEqual(_rights, 0x00101000)
                    self.assertFalse(inherit)
                    return 0 if mutation == "open" or mutation == "partial-open" and pid == 102 else pid + 100
                def membership(handle, job, result):
                    self.assertEqual((handle, job), (201, 100))
                    result._obj.value = mutation != "foreign"
                    return 1
                def wait(handle, milliseconds):
                    waited.append((handle, milliseconds))
                    return 258 if mutation == "wait" else 0
                def close(handle):
                    closed.append(handle)
                    return 1
                def terminate(handle, code):
                    terminated.append((handle, code))
                    return 0 if mutation == "terminate" else 1
                def check(value):
                    if not value:
                        raise OSError("inert API refusal")
                api = SimpleNamespace(QueryInformationJobObject=query, OpenProcess=opened,
                                      IsProcessInJob=membership, WaitForSingleObject=wait,
                                      TerminateJobObject=terminate, CloseHandle=close)
                job = SimpleNamespace(C=C, api=api, Accounting=Accounting, handle=100, check=check)
                if mutation == "clean":
                    self.assertTrue(audit._windows_file_close(job, time.monotonic() + 5))
                else:
                    expected_error = ("membership changed during capture" if mutation == "capture-generation"
                                      else "membership changed during termination" if mutation in (
                                          "termination-generation", "after-wait-generation")
                                      else "did not signal exit" if mutation == "wait"
                                      else "not in the owned Windows job" if mutation == "foreign"
                                      else "member list is incomplete" if mutation in ("short-list", "oversized-list")
                                      else "cleanup deadline" if mutation == "deadline"
                                      else "inert API refusal")
                    with self.assertRaisesRegex(OSError, expected_error):
                        audit._windows_file_close(job, time.monotonic() + (-1 if mutation == "deadline" else 5))
                self.assertEqual(terminated, [(100, 97)])
                self.assertEqual(closed[-1], 100)
                self.assertEqual(len(closed), len(set(closed)))
                expected = [100] if mutation in ("open", "short-list", "oversized-list", "deadline") else [201, 100]
                self.assertEqual(closed, expected)
                self.assertTrue(all(0 <= milliseconds <= 5000 for _, milliseconds in waited))

    def test_closed_success_keeps_exact_json_and_full_capture_uses_old_runner(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            command = [sys.executable, "-c", "print('{\"result\":\"success\"}')"]
            result = audit.call(command, self.environment(root), root, timeout=5, capture_files=True)
            self.assertEqual(result["_json"], {"result": "success"})
            self.assertEqual(result["exit_code"], 0)
            self.assertFalse(result["timeout"])
            def completed(*_args, **kwargs):
                kwargs["stdout"].write(b'{"result":"success"}')
                return subprocess.CompletedProcess(command, 0)
            with patch.object(audit, "_windows_file_call", side_effect=AssertionError("wrong runner")), \
                    patch.object(audit.subprocess, "run", side_effect=completed):
                result = audit.call(command, self.environment(root), root, timeout=5, capture_full=True)
            self.assertEqual(result["stdout_full"], '{"result":"success"}')
            self.assertEqual(result["exit_code"], 0)


class GeminiFixtureClock:
    """Synthetic command/journal UTC clock; filesystem custody stays real."""

    def __init__(self):
        self.value = datetime(2026, 11, 17, 12, 0, 0, tzinfo=timezone.utc)

    def advance(self, milliseconds=1):
        self.value += timedelta(milliseconds=milliseconds)
        return self.value

    def now(self, zone=timezone.utc):
        if zone != timezone.utc:
            raise AssertionError('fixture clock requires UTC')
        return self.advance()


@unittest.skipUnless(sys.platform in ("linux", "darwin"), "Linux/macOS descriptor custody contract")
class GeminiEmptySessionRuntimeContracts(unittest.TestCase):
    VERSION = {"status": "reported", "exit_code": 0, "value": "0.66.0"}

    @contextmanager
    def fixture(self):
        audit = load_audit()
        observer_type = audit.GEMINI_RUNTIME_AUDIT['GeminiRepeatAudit']
        clock = GeminiFixtureClock()
        with TemporaryDirectory() as temporary, patch.dict(
                observer_type.__init__.__globals__, {'_now': clock.now}):
            root = Path(temporary).resolve()
            home, project = root / 'home', root / 'Project'
            profile = home / '.gemini'
            chats = profile / 'tmp/project/chats'
            project.mkdir()
            chats.mkdir(parents=True)
            registry_project = str(project).lower() if sys.platform == 'win32' else str(project)
            (profile / 'projects.json').write_text(json.dumps({'projects': {registry_project: 'project'}}))
            (chats.parent / '.project_root').write_text(registry_project)
            old = chats / 'foreign-session.jsonl'
            old.write_bytes(b'pre-existing conversation must stay byte-for-byte\n')
            (profile / 'foreign-neighbor.txt').write_bytes(b'protected foreign profile file\n')
            (profile / 'foreign-empty').mkdir()
            before = audit.snapshot([profile])
            observer = observer_type(home, project, before, self.VERSION,
                                     profile_snapshot=audit.scoped_snapshot)
            self.assertIsNone(observer.reason)
            case = dict(audit=audit, observer=observer, home=home, project=project,
                        profile=profile, chats=chats, old=old, before=before, clock=clock,
                        phase_started=clock.now())
            try:
                yield case
            finally:
                observer.close()

    def record(self, case, index, transform=None, *, startup=False):
        now = case['clock'].now()
        timestamp = now.isoformat(timespec='milliseconds').replace('+00:00', 'Z')
        value = {'sessionId': 'skills-list-session',
                 'projectHash': hashlib.sha256(str(case['project']).encode()).hexdigest(),
                 'startTime': timestamp, 'lastUpdated': timestamp}
        body = json.dumps(value, separators=(',', ':')).encode() + b'\n'
        if startup:
            records = [json.loads(line) for line in GEMINI_STARTUP_RECORD_FIXTURE.splitlines()]
            records[0] = {**value, 'kind': 'main'}
            records[1]['timestamp'] = timestamp
            records[2]['$set']['lastUpdated'] = timestamp
            text = records[1]['content'][0]['text'].replace(
                'My operating system is: linux', 'My operating system is: ' + sys.platform)
            records[1]['content'][0]['text'] = text.replace(
                '<PINNED_PROJECT_TEMP>', str(case['chats'].parent)).replace(
                '<PINNED_PROJECT>', str(case['project']))
            body = b''.join(json.dumps(record).encode() + b'\n' for record in records)
        if transform:
            body = transform(body)
        name = f"session-{now:%Y-%m-%dT%H-%M}-{index}-skills-l.jsonl"
        path = case['chats'] / name
        path.write_bytes(body)
        return path

    def phase(self, case, label):
        finished = case['clock'].now()
        current = case['audit'].snapshot([case['profile']])
        case['observer'].checkpoint(
            label, current, interval=(case['phase_started'], finished))
        case['phase_started'] = case['clock'].now()
        return current

    def test_metadata_only_checkpoints_receive_their_exact_command_intervals(self):
        with self.fixture() as case:
            validate = case['audit'].GEMINI_RUNTIME_AUDIT['_metadata']
            observed = []
            def capture(body, filename, project_hash, started, finished, diagnostic=None):
                observed.append((started, finished))
                return validate(body, filename, project_hash, started, finished, diagnostic)
            intervals = []
            for index, label in enumerate(
                    ('after-public-repeat-add', 'after-public-repeat-list'), 1):
                started = case['clock'].now()
                self.record(case, index)
                finished = case['clock'].now()
                intervals.append((started, finished))
                after = case['audit'].snapshot([case['profile']])
                with patch.dict(validate.__globals__, {'_metadata': capture}):
                    case['observer'].checkpoint(label, after, interval=(started, finished))
                self.assertIsNone(case['observer'].reason)
            self.assertEqual(observed, intervals)
            self.assertTrue(case['observer'].finish(after))

    def test_metadata_only_phase_boundaries_and_between_phase_records(self):
        validate = load_audit().GEMINI_RUNTIME_AUDIT['_metadata']
        base = datetime(2026, 11, 17, 12, 0, 0, tzinfo=timezone.utc)
        for phase, previous_end, lower, upper in (('add', 10, 20, 30), ('list', 30, 40, 50)):
            started = base + timedelta(milliseconds=lower, microseconds=999)
            finished = base + timedelta(milliseconds=upper, microseconds=999)
            for position, offset, accepted in (
                    ('first-millisecond', lower, True), ('last-millisecond', upper, True),
                    ('between-phases', previous_end + 1, False),
                    ('before', lower - 1, False), ('after', upper + 1, False)):
                with self.subTest(phase=phase, position=position):
                    when = base + timedelta(milliseconds=offset)
                    timestamp = when.isoformat(timespec='milliseconds').replace('+00:00', 'Z')
                    body = json.dumps({'sessionId': 'skills-list-session', 'projectHash': 'a' * 64,
                                       'startTime': timestamp, 'lastUpdated': timestamp}).encode() + b'\n'
                    name = f'session-{when:%Y-%m-%dT%H-%M}-1-skills-l.jsonl'
                    if accepted:
                        validate(body, name, 'a' * 64, started, finished)
                    else:
                        with self.assertRaisesRegex(ValueError, 'outside the repeat interval'):
                            validate(body, name, 'a' * 64, started, finished)

    def test_metadata_only_missing_overlapping_and_future_intervals_refuse(self):
        for mutation in ('missing', 'overlap', 'future'):
            with self.subTest(mutation=mutation), self.fixture() as case:
                now = case['clock'].now()
                interval = (None if mutation == 'missing' else
                    (case['observer'].observed_at - timedelta(milliseconds=1), now)
                    if mutation == 'overlap' else (now, now + timedelta(days=1)))
                case['observer'].checkpoint('after-public-repeat-add', case['before'],
                                            interval=interval)
                self.assertEqual(case['observer'].reason,
                    'Gemini startup command interval is invalid' if mutation == 'missing' else
                    'Gemini startup command intervals overlap or are in the future')
                self.assertFalse(case['observer'].finish(case['before']))
                self.assertEqual(case['observer'].evidence()['phases'], [])

    def test_os_error_reasons_are_redacted_before_failed_case_projection(self):
        facts = {"phase": "after-public-repeat-add", "role": "new-chat",
                 "operation": "record-framing", "bytes": 1234, "lf_count": 3,
                 "final_lf": True, "record_shapes": [
                     "metadata-fields", "user-text-fields", "last-updated-fields"]}
        finite_reason = "Gemini startup environment framing differs"
        for stage in ("constructor", "baseline", "checkpoint", "finish"):
            for kind in ("missing", "custom", "finite"):
                with self.subTest(stage=stage, kind=kind), self.fixture() as case:
                    private_path = str(case["chats"] / "private-secret-session.jsonl")
                    error = (FileNotFoundError(2, "No such file or directory", private_path)
                             if kind == "missing" else OSError("custom filesystem detail " + private_path)
                             if kind == "custom" else ValueError(finite_reason))
                    subject = case["observer"]
                    if stage == "constructor":
                        with patch.object(type(subject), "_guard", side_effect=error):
                            subject = type(subject)(
                                case["home"], case["project"], case["before"], self.VERSION,
                                profile_snapshot=case["audit"].scoped_snapshot)
                    else:
                        subject.failure_facts = dict(facts)
                        with patch.object(subject, "_guard", side_effect=error):
                            interval = (case["phase_started"], case['clock'].now())
                            if stage == "baseline":
                                subject.baseline(case["before"], interval=interval)
                            elif stage == "checkpoint":
                                subject.checkpoint("after-public-repeat-add", case["before"], interval=interval)
                            else:
                                self.assertFalse(subject.finish(case["before"]))
                    try:
                        self.assertFalse(subject.finish(case["before"]))
                        proof = subject.evidence()
                        observed = case["audit"].failed_case_diagnostics(
                            {"gemini_runtime_proof": proof})["gemini_runtime_proof"]
                        self.assertEqual(proof["reason"], finite_reason if kind == "finite" else "os-error")
                        self.assertEqual(observed["reason"], proof["reason"])
                        self.assertEqual(proof["status"], "unqualified")
                        self.assertEqual(observed["status"], "unqualified")
                        self.assertEqual(proof["phases"], [])
                        self.assertEqual(observed["phases"], [])
                        if stage != "constructor":
                            self.assertEqual(proof["failure_facts"], facts)
                            self.assertEqual(observed["failure_facts"],
                                             dict(facts, record_shapes_truncated=False))
                        self.assertNotIn(private_path, json.dumps(observed))
                        self.assertNotIn("private-secret-session", json.dumps(observed))
                    finally:
                        subject.close()
                        self.assertEqual(subject.parents, {})
                        self.assertEqual(subject.leaves, {})

    def test_incomplete_snapshots_refuse_at_every_observation(self):
        markers = (('.gemini/denied-file', 'unreadable'),
                   ('.gemini/denied/.walk-error', 'unreadable'),
                   (r'.gemini\denied\.walk-error', '0' * 64))
        stages = ('admission', 'baseline', 'after-public-repeat-add',
                  'after-public-repeat-list', 'finish')
        for stage in stages:
            for path, digest in markers:
                with self.subTest(stage=stage, path=path), self.fixture() as case:
                    observer = case['observer']
                    if stage in ('after-public-repeat-list', 'finish'):
                        self.phase(case, 'after-public-repeat-add')
                    if stage == 'finish':
                        self.phase(case, 'after-public-repeat-list')
                    snapshot = {**case['audit'].snapshot([case['profile']]), path: digest}
                    if stage == 'admission':
                        observer = case['audit'].GEMINI_RUNTIME_AUDIT['GeminiRepeatAudit'](
                            case['home'], case['project'], snapshot, self.VERSION,
                            profile_snapshot=case['audit'].scoped_snapshot)
                    try:
                        if stage == 'baseline':
                            now = case['clock'].now()
                            observer.baseline(snapshot, interval=(now, now))
                        elif stage.startswith('after-public-'):
                            observer.checkpoint(stage, snapshot,
                                interval=(case['phase_started'], case['clock'].now()))
                        elif stage == 'finish':
                            observer.finish(snapshot)
                        self.assertEqual(observer.reason, 'Gemini runtime snapshot is incomplete')
                        self.assertFalse(observer.finish(snapshot))
                        self.assertEqual(observer.evidence()['status'], 'unqualified')
                    finally:
                        if stage == 'admission':
                            observer.close()

    def test_protected_profile_file_replacement_and_empty_directory_changes_refuse(self):
        for stage in ('baseline', 'after-public-repeat-add', 'after-public-repeat-list', 'finish'):
            for mutation in ('same-byte-file', 'new-empty', 'removed-empty', 'replaced-empty'):
                with self.subTest(stage=stage, mutation=mutation), self.fixture() as case:
                    observer = case['observer']
                    if stage in ('after-public-repeat-list', 'finish'):
                        self.phase(case, 'after-public-repeat-add')
                    if stage == 'finish':
                        self.phase(case, 'after-public-repeat-list')
                    path = case['profile'] / ('foreign-neighbor.txt' if mutation == 'same-byte-file'
                                              else 'foreign-empty')
                    if mutation in ('same-byte-file', 'replaced-empty'):
                        previous = path.lstat()
                        body = path.read_bytes() if mutation == 'same-byte-file' else None
                        path.rename(case['home'].parent / 'held-protected-entry')
                        if body is None:
                            path.mkdir()
                        else:
                            path.write_bytes(body)
                            self.assertEqual(path.read_bytes(), body)
                        self.assertFalse(os.path.samestat(previous, path.lstat()))
                    elif mutation == 'new-empty':
                        (case['profile'] / 'unrelated-empty').mkdir()
                    else:
                        path.rmdir()
                    after = case['audit'].snapshot([case['profile']])
                    # These mutations defeat content-only snapshots by construction.
                    self.assertEqual(after, case['before'])
                    if stage == 'baseline':
                        now = case['clock'].now()
                        observer.baseline(after, interval=(now, now))
                    elif stage == 'finish':
                        self.assertFalse(observer.finish(after))
                    else:
                        self.phase(case, stage)
                    self.assertEqual(observer.reason, 'Gemini protected profile entries changed')
                    self.assertFalse(observer.finish(after))
                    self.assertEqual(observer.evidence()['status'], 'unqualified')

    def test_six_empty_records_keep_raw_changes_and_exact_old_conversation(self):
        with self.fixture() as case:
            for index in range(1, 5):
                self.record(case, index)
            self.phase(case, 'after-public-repeat-add')
            self.assertIsNone(case['observer'].reason)
            for index in (5, 6):
                self.record(case, index, lambda body: body[:-2] + b',"kind":"main"}\n')
            after = self.phase(case, 'after-public-repeat-list')
            self.assertTrue(case['observer'].finish(after))
            self.assertNotEqual(case['before'], after)
            evidence = case['observer'].evidence()
            self.assertEqual(evidence['status'], 'metadata-only-new-records')
            self.assertEqual(evidence['platform'], sys.platform)
            for fd, _ in (*case['observer'].parents.values(), *case['observer'].leaves.values()):
                self.assertFalse(os.get_inheritable(fd))
            self.assertEqual(len(evidence['files']), 6)
            self.assertFalse(evidence['producer_attribution'])
            self.assertFalse(evidence['writer_closure'])
            self.assertEqual(case['old'].read_bytes(), b'pre-existing conversation must stay byte-for-byte\n')

    def test_conversation_foreign_duplicate_partial_and_unbounded_records_refuse(self):
        changes = {
            'message-record': lambda b: b + b'{"type":"user","content":"hello"}\n',
            'messages-field': lambda b: b[:-2] + b',"messages":[]}\n',
            'foreign-session': lambda b: b.replace(b'skills-list-session', b'conversation-session'),
            'foreign-project': lambda b: b.replace(b'"projectHash":"', b'"projectHash":"foreign'),
            'subagent': lambda b: b[:-2] + b',"kind":"subagent"}\n',
            'null-kind': lambda b: b[:-2] + b',"kind":null}\n',
            'nonfinite': lambda b: b[:-2] + b',"tokens":NaN}\n',
            'duplicate': lambda b: b.replace(b'{', b'{"sessionId":"other",', 1),
            'old-time': lambda b: json.dumps({**json.loads(b), 'startTime': '2000-01-01T00:00:00.000Z'}).encode() + b'\n',
            'future-time': lambda b: json.dumps({**json.loads(b), 'lastUpdated': '2099-01-01T00:00:00.000Z'}).encode() + b'\n',
            'partial': lambda b: b[:-1],
            'blank-line': lambda b: b + b'\n',
            'oversized': lambda b: b + b' ' * 4096,
        }
        for name, transform in changes.items():
            with self.subTest(name=name), self.fixture() as case:
                self.record(case, 1, transform)
                after = self.phase(case, 'after-public-repeat-add')
                self.assertFalse(case['observer'].finish(after))
                self.assertEqual(case['observer'].evidence()['status'], 'unqualified')


    def test_failure_facts_keep_multirecord_and_partial_private_payloads_unqualified(self):
        private = b'{"type":"user","content":"private startup context"}\n{"$set":{"private":"value"}}\n'
        for label, transform, count, final in (
                ("partial", lambda body: body[:-1], 0, False),
                ("multiple", lambda body: body + private, 3, True)):
            with self.subTest(label=label), self.fixture() as case:
                path = self.record(case, 1, transform)
                metadata = case["audit"].GEMINI_RUNTIME_AUDIT["_metadata"]
                parsed_sizes = []
                original_json = metadata.__globals__["_json"]
                def diagnostic_json(body):
                    parsed_sizes.append(len(body))
                    return original_json(body)
                with patch.dict(metadata.__globals__, {"_json": diagnostic_json}):
                    after = self.phase(case, "after-public-repeat-add")
                self.assertEqual(len(parsed_sizes), 3 if label == "multiple" else 0)
                self.assertTrue(all(size <= 4096 for size in parsed_sizes))
                self.assertFalse(case["observer"].finish(after))
                proof = case["observer"].evidence()
                self.assertEqual(proof["status"], "unqualified")
                self.assertEqual(proof["files"], [])
                self.assertEqual(proof["phases"], [])
                facts = proof["failure_facts"]
                self.assertEqual(facts, {
                    "phase": "after-public-repeat-add", "role": "new-chat",
                    "operation": "record-framing", "bytes": path.stat().st_size,
                    "lf_count": count, "final_lf": final,
                    **({"record_shapes": ["metadata-fields", "other", "other"]}
                       if label == "multiple" else {})})
                encoded = json.dumps(facts)
                self.assertNotIn("private", encoded)
                self.assertNotIn(str(case["project"]), encoded)
                self.assertNotIn(path.name, encoded)
        with self.fixture() as case:
            self.record(case, 1)
            self.phase(case, "after-public-repeat-add")
            self.assertNotIn("failure_facts", case["observer"].evidence())
            case["old"].write_bytes(b"changed old private data")
            after = self.phase(case, "after-public-repeat-list")
            self.assertFalse(case["observer"].finish(after))
            self.assertNotIn("failure_facts", case["observer"].evidence())

    def test_three_line_shape_facts_never_accept_private_history_or_swallow_cancellation(self):
        def records(body):
            return body + json.dumps({
                "id": "PRIVATE-ID", "timestamp": "PRIVATE-TIME", "type": "user",
                "content": [{"text": "PRIVATE startup context\nPRIVATE second line"}],
            }).encode() + b"\n" + b'{"$set":{"lastUpdated":"PRIVATE-TIME"}}\n'
        with self.fixture() as case:
            path = self.record(case, 1, records)
            after = self.phase(case, "after-public-repeat-add")
            self.assertFalse(case["observer"].finish(after))
            proof = case["observer"].evidence()
            self.assertEqual(proof["status"], "unqualified")
            self.assertEqual(proof["reason"], "Gemini runtime is not one complete initial record")
            self.assertEqual(proof["files"], [])
            self.assertEqual(proof["phases"], [])
            facts = proof["failure_facts"]
            self.assertEqual(facts["record_shapes"],
                             ["metadata-fields", "user-text-fields", "last-updated-fields"])
            self.assertNotIn("PRIVATE", json.dumps(facts))
            self.assertNotIn(str(case["project"]), json.dumps(facts))
            self.assertNotIn(path.name, json.dumps(facts))
        with self.fixture() as case:
            self.record(case, 1, lambda body: body + b"PRIVATE malformed\n[]\n")
            after = self.phase(case, "after-public-repeat-add")
            self.assertFalse(case["observer"].finish(after))
            self.assertEqual(case["observer"].evidence()["failure_facts"]["record_shapes"],
                             ["metadata-fields", "json-refused", "other"])
        for failure in (ValueError, KeyboardInterrupt):
            with self.subTest(failure=failure), self.fixture() as case:
                self.record(case, 1, records)
                metadata = case["audit"].GEMINI_RUNTIME_AUDIT["_metadata"]
                def broken(body):
                    raise failure("PRIVATE helper failure")
                with patch.dict(metadata.__globals__, {"_refused_record_shapes": broken}):
                    if failure is KeyboardInterrupt:
                        with self.assertRaises(KeyboardInterrupt):
                            self.phase(case, "after-public-repeat-add")
                    else:
                        after = self.phase(case, "after-public-repeat-add")
                        self.assertFalse(case["observer"].finish(after))
                        self.assertEqual(case["observer"].evidence()["reason"],
                                         "Gemini runtime is not one complete initial record")
                        self.assertNotIn("record_shapes", case["observer"].evidence()["failure_facts"])

    def test_file_admission_facts_distinguish_bounds_and_links_without_extra_open(self):
        for mutation in ("zero", "oversized", "hardlink", "symlink"):
            with self.subTest(mutation=mutation), self.fixture() as case:
                path = self.record(case, 1)
                if mutation == "zero":
                    path.write_bytes(b"")
                elif mutation == "oversized":
                    path.write_bytes(b"private" * 600)
                elif mutation == "hardlink":
                    os.link(path, case["project"].parent / "private-linked-name")
                else:
                    path.unlink()
                    path.symlink_to(case["old"].name)
                after = case["audit"].snapshot([case["profile"]])
                with patch.object(os, "open", side_effect=AssertionError("must refuse before open")) as opened:
                    case["observer"].checkpoint("after-public-repeat-add", after,
                        interval=(case["phase_started"], case['clock'].now()))
                opened.assert_not_called()
                self.assertFalse(case["observer"].finish(after))
                proof = case["observer"].evidence()
                self.assertEqual(proof["status"], "unqualified")
                self.assertEqual(proof["files"], [])
                self.assertEqual(proof["phases"], [])
                facts = proof["failure_facts"]
                self.assertEqual(facts["phase"], "after-public-repeat-add")
                self.assertEqual(facts["role"], "new-chat")
                self.assertEqual(facts["operation"], "file-admission")
                self.assertEqual(facts["regular"], mutation != "symlink")
                self.assertEqual(facts["single_link"], mutation != "hardlink")
                self.assertEqual(facts["positive_size"], mutation != "zero")
                self.assertEqual(facts["within_limit"], mutation != "oversized")
                self.assertTrue(facts["descriptor_capacity"])
                self.assertEqual(facts["bytes"], None if mutation == "oversized" else path.lstat().st_size)
                for key in ("regular", "single_link", "positive_size", "within_limit", "descriptor_capacity"):
                    self.assertIs(type(facts[key]), bool)
                self.assertNotIn("private", json.dumps(facts))
                self.assertNotIn(str(case["project"]), json.dumps(facts))

        for mutation in ("missing", "nonstring", "grammar", "duplicate"):
            with self.subTest(registry=mutation), self.fixture() as case:
                case["observer"].close()
                project = str(case["project"])
                projects = {project: "project"}
                if mutation == "missing": projects = {"private-foreign-key": "project"}
                elif mutation == "nonstring": projects[project] = False
                elif mutation == "grammar": projects[project] = "PRIVATE/value"
                else: projects.update({"private-other": "project", "private-third": "project"})
                (case["profile"] / "projects.json").write_text(json.dumps({"projects": projects}))
                before = case["audit"].snapshot([case["profile"]])
                observer = case["audit"].GEMINI_RUNTIME_AUDIT["GeminiRepeatAudit"](
                    case["home"], case["project"], before, self.VERSION,
                    profile_snapshot=case["audit"].scoped_snapshot)
                self.assertEqual(observer.reason, "Gemini runtime registry does not bind the exact project")
                self.assertFalse(observer.finish(before))
                proof = observer.evidence()
                self.assertEqual((proof["status"], proof["files"], proof["phases"]), ("unqualified", [], []))
                facts = proof["failure_facts"]
                self.assertEqual(facts, {
                    "phase": "pre-repeat", "role": "registry", "operation": "project-binding",
                    "exact_key_present": mutation != "missing",
                    "slug_is_string": mutation not in ("missing", "nonstring"),
                    "slug_matches_pattern": mutation == "duplicate",
                    "slug_binding_count": 2 if mutation == "duplicate" else None})
                for key in ("exact_key_present", "slug_is_string", "slug_matches_pattern"):
                    self.assertIs(type(facts[key]), bool)
                self.assertNotIn("private", json.dumps(facts).lower())
                self.assertNotIn(project, json.dumps(facts))
                self.assertEqual((observer.parents, observer.leaves), ({}, {}))

    def test_old_and_observed_leaf_replacement_parent_alias_and_foreign_delta_refuse(self):
        for mutation in ('old-replaced', 'new-replaced', 'new-deleted', 'parent-replaced',
                         'parent-alias', 'registry-replaced', 'foreign-delta', 'hardlink', 'symlink'):
            with self.subTest(mutation=mutation), self.fixture() as case:
                new = self.record(case, 1)
                self.phase(case, 'after-public-repeat-add')
                self.assertIsNone(case['observer'].reason)
                if mutation in ('old-replaced', 'new-replaced', 'registry-replaced'):
                    path = (case['old'] if mutation == 'old-replaced' else new if
                            mutation == 'new-replaced' else case['profile'] / 'projects.json')
                    body = path.read_bytes()
                    path.unlink()
                    path.write_bytes(body)
                elif mutation == 'new-deleted':
                    new.unlink()
                elif mutation in ('parent-replaced', 'parent-alias'):
                    moved = case['chats'].with_name('old-chats')
                    case['chats'].rename(moved)
                    if mutation == 'parent-alias':
                        case['chats'].symlink_to(moved, target_is_directory=True)
                    else:
                        case['chats'].mkdir()
                elif mutation == 'foreign-delta':
                    (case['profile'] / 'foreign-settings.json').write_text('{}')
                elif mutation == 'hardlink':
                    os.link(new, case['chats'] / 'another-link')
                else:
                    new.unlink()
                    new.symlink_to(case['old'])
                after = self.phase(case, 'after-public-repeat-list')
                self.assertFalse(case['observer'].finish(after))

    def test_record_count_phase_order_version_and_during_read_swap_refuse(self):
        with self.fixture() as case:
            for index in range(1, 18):
                self.record(case, index)
            after = self.phase(case, 'after-public-repeat-add')
            self.assertFalse(case['observer'].finish(after))
        with self.fixture() as case:
            self.record(case, 1)
            after = self.phase(case, 'after-public-repeat-list')
            self.assertFalse(case['observer'].finish(after))
        with self.fixture() as case:
            check = case['audit'].GEMINI_RUNTIME_AUDIT['pinned_version']
            for version in (None, {}, {**self.VERSION, 'value': '0.67.0'},
                            {**self.VERSION, 'exit_code': False},
                            {**self.VERSION, 'exit_code': 1}):
                self.assertFalse(check(version))
            with patch.object(os, 'supports_dir_fd', set()):
                self.assertFalse(check(self.VERSION))
            for flag in ('O_DIRECTORY', 'O_NOFOLLOW', 'O_CLOEXEC', 'O_NONBLOCK'):
                with self.subTest(flag=flag), patch.object(os, flag, 0):
                    self.assertFalse(check(self.VERSION))
        with self.fixture() as case:
            new = self.record(case, 1)
            original_read = os.read
            changed = []
            def replace_during_read(fd, count):
                body = original_read(fd, count)
                # Snapshots also read descriptors; inject only after this file
                # is held by the observer whose custody refusal is under test.
                held = case['observer'].leaves.get(new)
                if not changed and held is not None and fd == held[0]:
                    original = new.read_bytes()
                    new.unlink()
                    new.write_bytes(original)
                    changed.append(True)
                return body
            with (patch.object(os, 'open', wraps=os.open) as opened,
                  patch.object(os, 'read', side_effect=replace_during_read)):
                self.assertTrue(case['audit'].GEMINI_RUNTIME_AUDIT['pinned_version'](self.VERSION))
                after = self.phase(case, 'after-public-repeat-add')
            self.assertTrue(any(call.kwargs.get('dir_fd') is not None
                                for call in opened.call_args_list))
            self.assertEqual(changed, [True])
            self.assertEqual(case['observer'].reason,
                             'Gemini runtime previous file changed or replaced')
            self.assertFalse(case['observer'].finish(after))


    def test_public_repeat_gate_preserves_raw_delta_and_refuses_same_byte_parent_replacement(self):
        for replaced, startup, failure in ((False, False, None), (True, False, None),
                                            (False, True, None), (True, True, None),
                                            (False, True, 'timeout'), (False, True, 'exit')):
            with self.subTest(replaced=replaced, startup=startup, failure=failure), TemporaryDirectory() as temporary:
                audit = load_audit()
                root = Path(temporary).resolve()
                project = root / 'skill-00/project'
                profile = root / 'skill-00/home/.gemini'
                chats = profile / 'tmp/project/chats'
                skill = project / '.gemini/skills/owned-test/SKILL.md'
                item = {'name': 'owned-test', 'repo': 'example/skills', 'folder': 'owned-test',
                        'source': 'unused', 'skill_md_git_blob_sha1': '0' * 40}
                matched = [{'installation_id': 'owned-id', 'name': 'owned-test',
                            'clients': [{'client_id': 'gemini'}]}]
                operations, held = [], []
                real_observer = audit.GEMINI_RUNTIME_AUDIT['GeminiRepeatAudit']
                clock = GeminiFixtureClock()
                def observe(*args, **kwargs):
                    observer = real_observer(*args, **kwargs)
                    held.extend(fd for fd, _ in (*observer.parents.values(), *observer.leaves.values()))
                    return observer
                def fake_call(argv, env, cwd, timeout=105, *, capture_full=False):
                    operation = ('native-list' if argv[0] == '/unused/gemini' else
                                 argv[2] if argv[1] == 'skills' else argv[1])
                    operations.append(operation)
                    settings = profile / 'settings.json'
                    self.assertEqual(settings.read_bytes(), b'{"context":{"includeDirectoryTree":false}}\n')
                    self.assertEqual(cwd, project)
                    if operation == 'native-list':
                        self.assertTrue(capture_full)
                        self.assertEqual(argv, ['/unused/gemini', 'skills', 'list', '--all'])
                        self.assertEqual(timeout, 30)
                        self.assertEqual(env['GEMINI_CLI_HOME'], str(profile.parent))
                        self.assertEqual((env['NO_COLOR'], env['TERM']), ('1', 'dumb'))
                        self.assertFalse(set(env) & {'LANG', 'LC_ALL', 'TZ', 'HTTP_PROXY'})
                        self.assertLessEqual(set(env), {
                            'GEMINI_CLI_HOME', 'PATH', 'HOME', 'USERPROFILE', 'SYSTEMROOT',
                            'WINDIR', 'TEMP', 'TMP', 'TMPDIR', 'GEMINI_RESTRICTED_MODE',
                            'GEMINI_CLI_TRUST_WORKSPACE', 'NO_COLOR', 'TERM'})
                    else:
                        self.assertFalse(capture_full)
                        self.assertEqual(env['LC_ALL'], 'fr_FR.UTF-8')
                        self.assertEqual(env['TZ'], 'Pacific/Auckland')
                    if operation == 'add':
                        skill.parent.mkdir(parents=True, exist_ok=True)
                        skill.write_text('---\nname: owned-test\n---\n')
                        (project / 'uap.toml').write_text('owned-test')
                        (project / 'uap.lock').write_text('owned-test')
                        if operations.count('add') == 1:
                            chats.mkdir(parents=True)
                            (profile / 'projects.json').write_text(json.dumps({'projects': {str(project): 'project'}}))
                            (chats.parent / '.project_root').write_text(str(project))
                        elif replaced:
                            held = project.parent / 'held-old-chats'
                            chats.rename(held)
                            chats.mkdir()
                            for previous in held.iterdir():
                                (chats / previous.name).write_bytes(previous.read_bytes())
                        else:
                            self.record({'project': project, 'chats': chats, 'clock': clock}, 1, startup=startup)
                    elif operation == 'native-list':
                        self.record({'project': project, 'chats': chats, 'clock': clock}, 100, startup=startup)
                        if failure:
                            return {'exit_code': None if failure == 'timeout' else 7,
                                    'stdout_tail': '', 'stderr_tail': '',
                                    'timeout': failure == 'timeout', '_json': None}
                    elif operation == 'list' and operations.count('list') == 2 and not replaced:
                        self.record({'project': project, 'chats': chats, 'clock': clock}, 2, startup=startup)
                    elif operation == 'remove':
                        skill.unlink()
                        skill.parent.rmdir()
                        (project / 'uap.lock').unlink()
                    response = {'data': {'installations': [] if 'remove' in operations else matched}}
                    if operation == 'add':
                        response.update(readiness='ready', next_actions=[])
                    return {'exit_code': 0, 'stdout_tail': '', 'stderr_tail': '', 'timeout': False,
                            '_json': response,
                            **({'stdout_full': 'full baseline body\n' * 200, 'stderr_full': ''} if capture_full else {})}
                with (patch.dict(audit.AGENT_BINARIES, {'gemini': '/unused/gemini'}, clear=True),
                      patch.dict(audit.GEMINI_RUNTIME_AUDIT, {'GeminiRepeatAudit': observe}),
                      patch.dict(real_observer.__init__.__globals__, {'_now': clock.now}),
                      patch.object(audit, 'datetime', clock),
                      patch.dict(os.environ, {'LANG': 'de_DE.UTF-8', 'LC_ALL': 'fr_FR.UTF-8',
                                              'TZ': 'Pacific/Auckland',
                                              'HTTP_PROXY': 'http://proxy.invalid:8080'}),
                      patch.object(audit, 'call', side_effect=fake_call),
                      patch.object(audit, 'matching_installations', return_value=matched),
                      patch.object(audit, 'locked_skill_is_pinned',
                                   side_effect=lambda *args: (project / 'uap.lock').exists()),
                      patch.object(audit, 'matches_pinned_skill_bytes', return_value=True)):
                    row = audit.do_case('/unused/agentplugins', 'gemini', 'skill', item, 0, root, {},
                                        client_version=self.VERSION)
                self.assertEqual(operations, ['add', 'list', 'native-list', 'add', 'list', 'remove', 'list'])
                if not failure:
                    command = row['gemini_native_baseline']['command']
                    self.assertEqual(command['argv'], ['/unused/gemini', 'skills', 'list', '--all'])
                    self.assertEqual(command['cwd'], str(project))
                    self.assertEqual(command['stdout_bytes'], len(('full baseline body\n' * 200).encode()))
                    self.assertEqual(command['stdout_sha256'], hashlib.sha256(('full baseline body\n' * 200).encode()).hexdigest())
                self.assertTrue(row['removal_verified'])
                self.assertTrue(row['sentinels_preserved'])
                self.assertEqual(row['public_skill_ready'], {'after-add': True, 'repeat-add': True})
                self.assertFalse(row['repeat_native_unchanged'])
                self.assertEqual(row['repeat_native_runtime_only'], not replaced and not failure)
                self.assertEqual(row['repeat_verified'], not replaced and not failure)
                self.assertEqual(row['status'], 'repeat-add-unqualified' if replaced or failure else 'passed')
                self.assertTrue(held)
                for fd in held:
                    with self.assertRaises(OSError):
                        os.fstat(fd)

    def test_public_gemini_context_fixture_is_exclusive_and_preserved_before_each_call(self):
        for changed in ('preexisting', 'bytes', 'same-byte-replacement', 'link'):
            with self.subTest(changed=changed), TemporaryDirectory() as temporary:
                audit = load_audit()
                root = Path(temporary).resolve()
                profile = root / 'skill-00/home/.gemini'
                settings = profile / 'settings.json'
                original = b'{"context":{"includeDirectoryTree":false}}\n'
                foreign = root / 'foreign-settings.json'
                foreign.write_bytes(original)
                if changed == 'preexisting':
                    profile.mkdir(parents=True)
                    settings.write_bytes(b'{"context":{"includeDirectoryTree":true}}\n')
                calls = []

                def call(argv, env, cwd, timeout=105):
                    calls.append(argv)
                    self.assertEqual(settings.read_bytes(), original)
                    if changed == 'bytes':
                        settings.write_bytes(b'{"context":{"includeDirectoryTree":true}}\n')
                    elif changed == 'same-byte-replacement':
                        settings.rename(root / 'held-context-settings.json')
                        settings.write_bytes(original)
                    elif changed == 'link':
                        settings.unlink()
                        settings.symlink_to(foreign)
                    return {'exit_code': 0, 'stdout_tail': '', 'stderr_tail': '',
                            'timeout': False, '_json': {}}

                item = {'name': 'owned-test', 'repo': 'example/skills', 'folder': 'owned-test',
                        'source': 'unused', 'skill_md_git_blob_sha1': '0' * 40}
                with (patch.dict(audit.AGENT_BINARIES, {'gemini': '/unused/gemini'}, clear=True),
                      patch.object(audit, 'call', side_effect=call)):
                    if changed == 'preexisting':
                        with self.assertRaises(FileExistsError):
                            audit.do_case('/unused/agentplugins', 'gemini', 'skill', item, 0, root, {},
                                          client_version=self.VERSION)
                        self.assertEqual(calls, [])
                        self.assertEqual(settings.read_bytes(),
                                         b'{"context":{"includeDirectoryTree":true}}\n')
                    else:
                        with self.assertRaisesRegex(ValueError, 'Gemini audit context settings changed'):
                            audit.do_case('/unused/agentplugins', 'gemini', 'skill', item, 0, root, {},
                                          client_version=self.VERSION)
                        self.assertEqual(len(calls), 1)
                self.assertEqual(foreign.read_bytes(), original)




    def test_windows_registry_and_marker_require_exact_producer_spelling(self):
        if sys.platform != "win32":
            self.skipTest("Windows producer serialization contract")
        for mutation in ("registry", "marker"):
            with self.subTest(mutation=mutation), self.fixture() as case:
                project = str(case["project"])
                self.assertNotEqual(project, project.lower())
                case["observer"].close()
                if mutation == "registry":
                    (case["profile"] / "projects.json").write_text(
                        json.dumps({"projects": {project: "project"}}))
                else:
                    (case["chats"].parent / ".project_root").write_text(project)
                before = case["audit"].snapshot([case["profile"]])
                observer = case["audit"].GEMINI_RUNTIME_AUDIT["GeminiRepeatAudit"](
                    case["home"], case["project"], before, self.VERSION)
                try:
                    self.assertEqual(observer.reason,
                        "Gemini runtime registry does not bind the exact project"
                        if mutation == "registry" else "Gemini runtime ownership marker differs")
                    self.assertFalse(observer.finish(before))
                    if mutation == "registry":
                        self.assertIs(observer.evidence()["failure_facts"]["exact_key_present"], False)
                finally:
                    observer.close()


# Native 0.66.0 three-record journal fixture; only private fixture paths replaced.
GEMINI_STARTUP_RECORD_FIXTURE = '{"sessionId":"skills-list-session","projectHash":"6545b9ca63e68521b25565d8b8adfd1bc89c7d29202af280f0ac10aa6796099a","startTime":"2026-11-17T21:52:38.592Z","lastUpdated":"2026-11-17T21:52:38.592Z","kind":"main"}\n{"id":"d04923d38bb0f6017037e74183378ef4","timestamp":"2026-11-17T21:52:38.593Z","type":"user","content":[{"text":"<session_context>\\nThis is the Gemini CLI. We are setting up the context for our chat.\\nToday\'s date is Tuesday, November 17, 2026 (formatted according to the user\'s locale).\\nMy operating system is: linux\\nThe project\'s temporary directory is: <PINNED_PROJECT_TEMP>\\n- **Workspace Directories:**\\n  - <PINNED_PROJECT>\\n- **Directory Structure:**\\n\\nShowing up to 200 items (files + folders). Folders or files indicated with ... contain more items not shown, were ignored, or the display limit (200 items) was reached.\\n\\n<PINNED_PROJECT>/\\n├───.gemini/\\n│   └───skills/\\n│       └───native-list-probe/\\n│           └───SKILL.md\\n└───.git/...\\n\\n\\n</session_context>"}]}\n{"$set":{"lastUpdated":"2026-11-17T21:52:38.593Z"}}\n'
GEMINI_STARTUP_AUDIT = load_audit()
GEMINI_STARTUP_MODULE = GEMINI_STARTUP_AUDIT.GEMINI_RUNTIME_AUDIT
gemini_startup_snapshot = GEMINI_STARTUP_AUDIT.snapshot
GEMINI_STARTUP_VERSION = {"status": "reported", "exit_code": 0, "value": "0.66.0"}


class GeminiStartupEnvironmentContracts(unittest.TestCase):
    IDENTITY_REFUSAL = (
        r'^Gemini (?:runtime previous file changed or replaced|Windows custody: '
        r'(?:held file changed|held name changed|current name identity))$'
    )

    @contextmanager
    def fixture(self, baseline_mutation=None):
        clock = GeminiFixtureClock()
        observer_type = GEMINI_STARTUP_MODULE['GeminiRepeatAudit']
        with TemporaryDirectory() as temp, patch.dict(
                observer_type.__init__.__globals__, {'_now': clock.now}):
            root = Path(temp).resolve()
            home = root / 'home'
            project = root / 'project'
            project.mkdir()
            profile = home / '.gemini'
            chats = profile / 'tmp/project/chats'
            chats.mkdir(parents=True)
            registry_project = str(project).lower() if sys.platform == 'win32' else str(project)
            (profile / 'projects.json').write_text(json.dumps({'projects': {registry_project: 'project'}}))
            (chats.parent / '.project_root').write_text(registry_project)
            template = [json.loads(x) for x in GEMINI_STARTUP_RECORD_FIXTURE.splitlines()]
            text = template[1]['content'][0]['text'].replace(
                'My operating system is: linux', 'My operating system is: ' + sys.platform)
            template[1]['content'][0]['text'] = text.replace('<PINNED_PROJECT_TEMP>', str(chats.parent)).replace('<PINNED_PROJECT>', str(project))
            old = chats / 'foreign-before-list.jsonl'
            old.write_bytes(b'foreign startup authority stays unchanged')
            before = gemini_startup_snapshot([profile])
            o = observer_type(home, project, before, GEMINI_STARTUP_VERSION,
                              profile_snapshot=GEMINI_STARTUP_AUDIT.scoped_snapshot)
            self.assertIsNone(o.reason)
            start = clock.now()
            c = dict(home=home, project=project, profile=profile, chats=chats,
                     template=template, old=old, clock=clock)
            p = self.write(c, 0, start + timedelta(milliseconds=1))
            values = [json.loads(line) for line in p.read_bytes().splitlines()]
            if baseline_mutation == 'id':
                values[1]['id'] = 'f' * 32
            elif baseline_mutation == 'frame':
                values[1]['content'][0]['text'] = 'arbitrary user prompt'
            elif baseline_mutation == 'project':
                values[0]['projectHash'] = 'f' * 64
            elif baseline_mutation == 'keys':
                values[2]['$set']['other'] = 'x'
            elif baseline_mutation == 'old-key':
                old.write_bytes(b'changed foreign startup authority')
            elif baseline_mutation == 'old-swap':
                body = old.read_bytes()
                old.rename(root / 'held-foreign-before-list.jsonl')
                old.write_bytes(body)
            body = b''.join(json.dumps(value).encode() + b'\n' for value in values)
            if baseline_mutation == 'duplicate':
                body = body.replace(b'{', b'{"sessionId":"other",', 1)
            p.write_bytes(body)
            finish = clock.advance(4)
            observed = gemini_startup_snapshot([profile])
            o.baseline(observed, interval=(start, finish))
            c.update(observer=o, before=observed, start=start, finish=finish)
            if baseline_mutation is None:
                self.assertIsNone(o.reason)

            try:
                yield c
            finally:
                o.close()

    def write(self, c, n, when, mutate=None):
        v = json.loads(json.dumps(c['template']))
        t = lambda dt: dt.isoformat(timespec='milliseconds').replace('+00:00', 'Z')
        v[0].update(projectHash=hashlib.sha256(str(c['project']).encode()).hexdigest(), startTime=t(when), lastUpdated=t(when))
        v[1]['timestamp'] = t(when + timedelta(milliseconds=1))
        v[2]['$set']['lastUpdated'] = t(when + timedelta(milliseconds=2))
        if mutate:
            mutate(v)
        b = b''.join((json.dumps(x, separators=(',', ':')).encode() + b'\n' for x in v))
        p = c['chats'] / f'session-{when:%Y-%m-%dT%H-%M}-{n + 1}-skills-l.jsonl'
        p.write_bytes(b)
        return p

    def phase(self, c, n, mutate=None, interval=True):
        start = c['clock'].now()
        p = self.write(c, n, start + timedelta(milliseconds=1), mutate)
        finish = c['clock'].advance(4)
        after = gemini_startup_snapshot([c['profile']])
        c['observer'].checkpoint(('after-public-repeat-add', 'after-public-repeat-list')[n - 1], after, interval=(start, finish) if interval else None)
        return (after, p)

    def test_actual_startup_template_two_phases_and_old_bytes(self):
        with self.fixture() as c:
            old = c['old']
            raw = old.read_bytes()
            self.phase(c, 1)
            after, _ = self.phase(c, 2)
            self.assertTrue(c['observer'].finish(after))
            self.assertEqual(old.read_bytes(), raw)
            e = c['observer'].evidence()
            self.assertFalse(e['producer_attribution'])
            self.assertFalse(e['writer_closure'])
            self.assertEqual([x['records'] for x in e['files']], [3, 3])

    def test_startup_accepts_separate_ordered_millisecond_clock_reads(self):
        when = datetime(2026, 11, 17, 12, 0, 0, tzinfo=timezone.utc)
        validate = GEMINI_STARTUP_MODULE['_startup']
        for separation in (0, 1):
            with self.subTest(metadata_separation_ms=separation), self.fixture() as c:
                def change(values):
                    values[0]['lastUpdated'] = (
                        when + timedelta(milliseconds=separation)
                    ).isoformat(timespec='milliseconds').replace('+00:00', 'Z')
                path = self.write(c, 1, when, change)
                project_hash = hashlib.sha256(str(c['project']).encode()).hexdigest()
                for interval in ((when, when + timedelta(milliseconds=3)),
                                 (when + timedelta(microseconds=999),
                                  when + timedelta(milliseconds=2, microseconds=999))):
                    self.assertEqual(validate(path.read_bytes(), path.name, project_hash,
                                              interval, c['chats'].parent),
                                     c['observer'].startup_template)

    def test_startup_clock_order_window_and_precision_refuse(self):
        when = datetime(2026, 11, 17, 12, 0, 0, tzinfo=timezone.utc)
        stamp = lambda offset: (when + timedelta(milliseconds=offset)).isoformat(
            timespec='milliseconds').replace('+00:00', 'Z')
        changes = {
            'metadata-order': lambda v: v[0].update(startTime=stamp(1), lastUpdated=stamp(0)),
            'message-order': lambda v: v[0].update(lastUpdated=stamp(2)),
            'update-order': lambda v: v[2]['$set'].update(lastUpdated=stamp(0)),
            'before-window': lambda v: v[0].update(startTime=stamp(-1)),
            'after-window': lambda v: v[2]['$set'].update(lastUpdated=stamp(3)),
            'submillisecond': lambda v: v[0].update(
                startTime=when.isoformat(timespec='microseconds').replace('+00:00', 'Z')),
        }
        for mutation, change in changes.items():
            with self.subTest(mutation=mutation), self.fixture() as c:
                path = self.write(c, 1, when, change)
                interval = (when + timedelta(microseconds=999),
                            when + timedelta(milliseconds=2, microseconds=999))
                with self.assertRaisesRegex(ValueError, 'Gemini (runtime|startup) record time'):
                    GEMINI_STARTUP_MODULE['_startup'](
                        path.read_bytes(), path.name,
                        hashlib.sha256(str(c['project']).encode()).hexdigest(),
                        interval, c['chats'].parent)

    def test_changed_template_identity_project_extra_keys_tools_and_times_refuse(self):
        changes = {'text': lambda v: v[1]['content'][0].update(text=v[1]['content'][0]['text'] + 'x'), 'id': lambda v: v[1].update(id='f' * 32), 'project': lambda v: v[0].update(projectHash='f' * 64), 'keys': lambda v: v[1].update(model='x'), 'tool': lambda v: v[1]['content'].append({'functionCall': {}}), 'extra': lambda v: v.append({'type': 'user', 'content': 'hello'}), 'time': lambda v: v[2]['$set'].update(lastUpdated='2000-01-01T00:00:00.000Z')}
        for name, change in changes.items():
            with self.subTest(name=name), self.fixture() as c:
                after, _ = self.phase(c, 1, change)
                self.assertFalse(c['observer'].finish(after))
                self.assertEqual(c['observer'].evidence()['files'], [])

    def test_interval_absence_and_held_inode_replacement_refuse(self):
        for mutation in ('interval', 'swap'):
            with self.subTest(mutation=mutation), self.fixture() as c:
                if mutation == 'swap':
                    p = c['old']
                    b = p.read_bytes()
                    p.rename(c['home'].parent / 'held-foreign-before-list.jsonl')
                    p.write_bytes(b)
                    self.assertEqual(gemini_startup_snapshot([c['profile']]), c['before'])
                after, _ = self.phase(c, 1, interval=mutation != 'interval')
                reason = c['observer'].reason
                if mutation == 'swap':
                    self.assertRegex(reason or '', self.IDENTITY_REFUSAL)
                else:
                    self.assertEqual(reason, 'Gemini startup command interval is invalid')
                self.assertEqual(c['observer'].phases, [])
                self.assertFalse(c['observer'].finish(after))
                self.assertEqual(c['observer'].reason, reason)

    def test_duplicate_json_and_wrong_phase_window_refuse(self):
        for mutation in ('duplicate', 'window'):
            with self.subTest(mutation=mutation), self.fixture() as c:
                start = c['clock'].now()
                p = self.write(c, 1, c['start'] if mutation == 'window' else start)
                if mutation == 'duplicate':
                    p.write_bytes(p.read_bytes().replace(b'{', b'{"sessionId":"x",', 1))
                after = gemini_startup_snapshot([c['profile']])
                interval = (start, c['clock'].advance(4))
                self.assertGreaterEqual(interval[0], c['finish'])
                c['observer'].checkpoint('after-public-repeat-add', after, interval=interval)
                self.assertFalse(c['observer'].finish(after))
                self.assertEqual(c['observer'].reason,
                    'Gemini runtime record time is outside the repeat interval'
                    if mutation == 'window' else 'duplicate JSON member')

    def test_startup_baseline_is_validated_and_previous_snapshot_preserved(self):
        for mutation in ('id', 'frame', 'project', 'keys', 'duplicate', 'old-key'):
            with self.subTest(mutation=mutation), self.fixture(mutation) as case:
                self.assertIsNotNone(case['observer'].reason)
                self.assertFalse(case['observer'].finish(case['before']))

    def test_pre_first_list_same_byte_old_object_swap_refuses(self):
        for replaced in (False, True):
            with self.subTest(replaced=replaced), self.fixture('old-swap' if replaced else None) as case:
                if replaced:
                    self.assertEqual(case['old'].read_bytes(),
                                     b'foreign startup authority stays unchanged')
                    self.assertRegex(case['observer'].reason or '', self.IDENTITY_REFUSAL)
                    self.assertFalse(case['observer'].finish(case['before']))
                else:
                    self.phase(case, 1)
                    after, _ = self.phase(case, 2)
                    self.assertTrue(case['observer'].finish(after))

    def test_first_list_keeps_existing_pre_repeat_chat_bound(self):
        with self.fixture() as case:
            case['observer'].close()
            for index in range(62):
                (case['chats'] / f'old-{index}.jsonl').write_bytes(b'held old conversation')
            before = gemini_startup_snapshot([case['profile']])
            observer = GEMINI_STARTUP_MODULE['GeminiRepeatAudit'](
                case['home'], case['project'], before, GEMINI_STARTUP_VERSION,
                profile_snapshot=GEMINI_STARTUP_AUDIT.scoped_snapshot)
            try:
                self.assertIsNone(observer.reason)
                started = case['clock'].now()
                self.write(case, 200, started + timedelta(milliseconds=1))
                observer.baseline(gemini_startup_snapshot([case['profile']]),
                                  interval=(started, case['clock'].advance(4)))
                self.assertEqual(observer.reason, 'Gemini pre-repeat chat count exceeds its bound')
            finally:
                observer.close()




class LiveAuditNativeVersionRoutingTests(unittest.TestCase):
    def test_catalog_entrypoint_passes_observed_version_to_gemini_skills_and_opencode(self):
        from io import StringIO

        for target, kind, forwarded in (("gemini", "skill", True),
                                        ("gemini", "plugin", False),
                                        ("opencode", "skill", True),
                                        ("opencode", "plugin", True)):
            with self.subTest(target=target, kind=kind), TemporaryDirectory() as temporary:
                audit = load_audit()
                native_version = {"status": "reported", "exit_code": 0,
                                  "value": "0.66.0" if target == "gemini" else "1.18.34"}
                installer_version = {"status": "reported", "exit_code": 0, "value": "0.1.95"}
                argv = ["live-install-audit.py", "--cli", sys.executable, "--target", target,
                        "--kind", kind, "--limit", "1", "--report-dir", temporary]

                def version(command, _env, _cwd):
                    return native_version if command[-1] == "--version" else installer_version

                with (patch.object(sys, "argv", argv),
                      patch.object(sys, "stdout", StringIO()),
                      patch.dict(audit.AGENT_BINARIES, {target: sys.executable}, clear=True),
                      patch.object(audit, "vendor_bootstrap_attested", return_value=True),
                      patch.object(audit, "version_evidence", side_effect=version) as probe,
                      patch.object(audit, "do_case", return_value={"status": "passed"}) as case):
                    self.assertEqual(audit.main(), 0)
                self.assertEqual(probe.call_count, 2)
                self.assertEqual(case.call_count, 1)
                self.assertEqual(case.call_args.args[1:3], (target, kind))
                self.assertEqual(case.call_args.kwargs,
                                 {"client_version": native_version} if forwarded else {})
                report = json.loads((Path(temporary) / f"{target}-{kind}.json").read_text())
                self.assertEqual(report["client_version"], native_version)


class MarketingKitCodexSQLiteTest(unittest.TestCase):
    def setUp(self):
        self.runner = run_path(str(Path(__file__).with_name('marketing-kit-project-release-e2e.py')))
        self.runtime = self.runner['audit']['CODEX_RUNTIME_AUDIT']
        temporary = TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.env, self.project, self.foreign, _, _ = self.runner['run_case'](
            'unused-cli', 'codex', self.root, 'A')
        self.home = Path(self.env['HOME'])
        self.database = self.home / '.local/state/codex-sqlite/state_5.sqlite'
        self.bootstrap_mutation = None

    async def discover(self, binary, project, profile, environment, **kwargs):
        self.assertEqual(kwargs, {'sqlite_baseline': True})
        self.assertEqual(environment['CODEX_HOME'], str(self.home / '.codex'))
        with closing(sqlite3.connect(self.database)) as connection:
            connection.execute('CREATE TABLE fixture_rows (value TEXT)')
            connection.execute("INSERT INTO fixture_rows VALUES ('stable')")
            connection.commit()
        if self.bootstrap_mutation:
            self.bootstrap_mutation()
        return {'data': [{'cwd': str(project), 'errors': [], 'skills': []}]}

    def prepare(self, version_probe=None):
        observer_type = self.runtime['CodexPluginSQLiteAudit']
        with patch.dict(observer_type.__init__.__globals__, {'_exchange': self.discover}), \
                patch.dict(self.runner['prepare_codex_sqlite'].__globals__, {
                    'invoke': version_probe or (lambda *args, **kwargs: {'exit_code': 0, 'timeout': False,
                                            'stdout_full': 'codex-cli 0.153.4\n'})}):
            return self.runner['prepare_codex_sqlite'](sys.executable, self.env, self.project)

    def test_marketing_kit_version_probe_uses_removed_separate_native_profile(self):
        profiles = []

        def probe(argv, env, project, timeout, *, capture_full=False):
            self.assertTrue(capture_full)
            self.assertEqual(argv, [sys.executable, '--version'])
            self.assertEqual(project, self.project)
            self.assertEqual(env['HOME'], str(self.home))
            version_home = Path(env['CODEX_HOME'])
            self.assertEqual(version_home.parent, self.project.parent)
            self.assertNotEqual(version_home, self.home / '.codex')
            self.assertTrue(version_home.is_dir())
            # Real pinned --version creates arg0 files even with existing
            # lifecycle tmp/arg0 parents. They belong only to this profile.
            (version_home / 'tmp/arg0/native-version-helper').mkdir(parents=True)
            profiles.append(version_home)
            return {'exit_code': 0, 'timeout': False, 'stdout_full': 'codex-cli 0.153.4\n'}

        observer = self.prepare(probe)
        self.assertEqual(len(profiles), 1)
        self.assertFalse(profiles[0].exists())
        self.assertEqual(list((self.home / '.codex/tmp/arg0').iterdir()), [])
        observer.checkpoint('after-isolated-version')

    def test_marketing_kit_version_probe_still_refuses_wrong_version_or_lifecycle_writes(self):
        original = self.foreign.read_bytes()
        for mutation in (
            'wrong-version', 'hidden-banner', 'timeout', 'exit', 'foreign-write',
            'new-home-file', 'project-write', 'data-write', 'raised-invoke',
        ):
            with self.subTest(mutation=mutation):
                version_profiles = []
                def probe(*args, **kwargs):
                    self.assertIs(kwargs.get('capture_full'), True)
                    version_profiles.append(Path(args[1]['CODEX_HOME']))
                    result = {'exit_code': 0, 'timeout': False, 'stdout_full': 'codex-cli 0.153.4\n'}
                    if mutation == 'wrong-version':
                        result['stdout_full'] = 'codex-cli 0.153.4-dev\n'
                    elif mutation == 'hidden-banner':
                        result['stdout_tail'] = 'codex-cli 0.153.4\n'
                        result['stdout_full'] = 'unsafe banner\n' + ' ' * 1900 + result['stdout_tail']
                    elif mutation == 'timeout':
                        result['timeout'] = True
                    elif mutation == 'exit':
                        result['exit_code'] = 1
                    elif mutation == 'foreign-write':
                        self.foreign.write_text('changed by metadata probe')
                    elif mutation == 'project-write':
                        (self.project / 'unexpected-version-write').write_text('forbidden')
                    elif mutation == 'data-write':
                        (Path(self.env['AGENTPLUGINS_HOME']) / 'unexpected-version-write').write_text('forbidden')
                    elif mutation == 'raised-invoke':
                        raise RuntimeError('native version invocation failed')
                    else:
                        (self.home / 'unexpected-version-write').write_text('forbidden')
                    return result
                expected = RuntimeError if mutation == 'raised-invoke' else AssertionError
                diagnostic = 'native version invocation failed' if mutation == 'raised-invoke' else 'unchanged native 0.153.4 version probe'
                with self.assertRaisesRegex(expected, diagnostic):
                    self.prepare(probe)
                self.assertEqual(len(version_profiles), 1)
                self.assertFalse(version_profiles[0].exists())
                self.foreign.write_bytes(original)
                for root in (self.home, self.project, Path(self.env['AGENTPLUGINS_HOME'])):
                    (root / 'unexpected-version-write').unlink(missing_ok=True)
                self.assertFalse((self.home / '.codex/config.toml').exists())
                self.assertFalse((self.home / '.codex/installation_id').exists())

    def test_marketing_kit_large_wal_requires_audited_separate_root_and_native_absence(self):
        observer = self.prepare()
        with closing(sqlite3.connect(self.database)) as connection:
            connection.execute('PRAGMA journal_mode=WAL')
            connection.execute('PRAGMA wal_autocheckpoint=0')
            for _ in range(300):
                connection.execute("UPDATE fixture_rows SET value='temporary'")
                connection.execute("UPDATE fixture_rows SET value='stable'")
                connection.commit()
            wal = Path(str(self.database) + '-wal')
            self.assertGreater(wal.stat().st_size, 1 << 20)
            observer.checkpoint('after-remove')
            self.runner['check_removed_native_components']('codex', self.home, self.root, 'audited-remove')
            evidence = observer.evidence()
            self.assertTrue(evidence['runtime_semantics_unchanged'])
            self.assertGreater(evidence['observations'][-1]['runtime_files_fully_read']['state_5.sqlite-wal']['bytes'], 1 << 20)
            # The same bytes under the raw native authority still fail closed.
            (self.home / '.codex/state_5.sqlite-wal').write_bytes(wal.read_bytes())
            with self.assertRaisesRegex(AssertionError, 'unscanned-too-large'):
                self.runner['check_removed_native_components']('codex', self.home, self.root, 'unowned-wal')

    def test_marketing_kit_runtime_schema_or_row_change_refuses(self):
        observer = self.prepare()
        with closing(sqlite3.connect(self.database)) as connection:
            connection.execute("UPDATE fixture_rows SET value='changed'")
            connection.commit()
            with self.assertRaisesRegex(ValueError, 'table contents changed'):
                observer.checkpoint('changed-row')
            connection.execute("UPDATE fixture_rows SET value='stable'")
            connection.execute('CREATE TABLE unexpected (value TEXT)')
            connection.commit()
            with self.assertRaisesRegex(ValueError, 'schema changed'):
                observer.checkpoint('changed-schema')

    def test_marketing_kit_runtime_config_and_root_replacement_refuse(self):
        observer = self.prepare()
        original = observer.config.read_bytes()
        observer.config.write_text('sqlite_home = "/foreign"\n')
        with self.assertRaisesRegex(ValueError, 'config authority changed'):
            observer.checkpoint('changed-config')
        observer.config.write_bytes(original)
        observer.runtime.rename(observer.runtime.with_name('held-runtime'))
        observer.runtime.mkdir()
        with self.assertRaisesRegex(ValueError, 'authority root changed'):
            observer.checkpoint('replaced-root')

    def test_marketing_kit_native_bootstrap_cannot_mutate_foreign_or_add_config(self):
        self.bootstrap_mutation = lambda: self.foreign.write_text('changed')
        with self.assertRaisesRegex(AssertionError, 'bootstrap changed protected'):
            self.prepare()

    def test_marketing_kit_native_bootstrap_cannot_add_extra_config_keys(self):
        def mutate():
            with (self.home / '.codex/config.toml').open('a') as stream:
                stream.write('unexpected = true\n')
        self.bootstrap_mutation = mutate
        with self.assertRaisesRegex(ValueError, 'preinstall changed the authored config'):
            self.prepare()

    def test_marketing_kit_mutating_command_checkpoint_precedes_inventory_even_on_failure(self):
        observer = self.prepare()
        execute = self.runner['execute']
        inventory = Mock()

        def changed_runtime(*_args):
            with closing(sqlite3.connect(self.database)) as connection:
                connection.execute("UPDATE fixture_rows SET value='changed'")
                connection.commit()
            return {'exit_code': 0}

        with patch.dict(execute.__globals__, {'invoke': changed_runtime, 'inventory': inventory}):
            with self.assertRaisesRegex(ValueError, 'table contents changed'):
                execute('cli', self.env, self.project, 'codex', 'add',
                        'add', 'marketing-kit', '--scope', 'user', checkpoint=observer.checkpoint)
        inventory.assert_not_called()
        phases = []
        with patch.dict(execute.__globals__, {'invoke': Mock(side_effect=RuntimeError('command failed'))}):
            with self.assertRaisesRegex(RuntimeError, 'command failed'):
                execute('cli', self.env, self.project, 'codex', 'failed-add',
                        'add', 'marketing-kit', '--scope', 'user', checkpoint=phases.append)
        self.assertEqual(phases, ['after-public-mutation-failed-add'])

    def test_marketing_kit_profile_calls_check_failed_commands_and_exclude_E(self):
        profiles = self.runner['MarketingCodexSQLiteProfiles']('codex')
        observer = self.prepare()
        profiles.profiles['A'] = observer
        with self.assertRaisesRegex(RuntimeError, 'command failed'):
            profiles.call('failed-command', lambda: (_ for _ in ()).throw(RuntimeError('command failed')))
        self.assertEqual(observer.observations[-1]['phase'], 'after-failed-command')
        with self.assertRaisesRegex(AssertionError, 'fresh lifecycle profile'):
            profiles.prepare('E', self.env, self.project)
        self.assertEqual(set(profiles.evidence()), {'A'})


    def test_marketing_kit_snapshot_file_keys_match_windows_and_posix(self):
        key = self.runner['codex_snapshot_file_key']
        self.assertEqual(key(PurePosixPath('/test/home'), PurePosixPath('/test/home/.codex/config.toml')),
                         'home/.codex/config.toml')
        self.assertEqual(key(PureWindowsPath('C:/test/home'), PureWindowsPath('C:/test/home/.codex/config.toml')),
                         r'home/.codex\config.toml')

    def test_marketing_kit_profile_evidence_distinguishes_failure_from_unrun_final(self):
        profiles = self.runner['MarketingCodexSQLiteProfiles']('codex')
        observer = self.prepare()
        profiles.profiles['A'] = observer
        initial = profiles.evidence()['A']
        self.assertFalse(initial['runtime_semantics_unchanged'])
        self.assertEqual(initial['final_checkpoint'], 'not_run')
        self.assertIsNone(initial['failed_phase'])
        with closing(sqlite3.connect(self.database)) as connection:
            connection.execute("UPDATE fixture_rows SET value='changed'")
            connection.commit()
        with self.assertRaisesRegex(ValueError, 'table contents changed'):
            profiles.checkpoint('after-public-mutation-add')
        failed = profiles.evidence()['A']
        self.assertFalse(failed['runtime_semantics_unchanged'])
        self.assertEqual(failed['final_checkpoint'], 'failed')
        self.assertEqual(failed['failed_phase'], 'after-public-mutation-add')

    def test_marketing_kit_profile_evidence_pass_requires_final_checkpoint(self):
        profiles = self.runner['MarketingCodexSQLiteProfiles']('codex')
        profiles.profiles['A'] = self.prepare()
        profiles.checkpoint('final-qualification')
        recorded = profiles.evidence()['A']
        self.assertTrue(recorded['runtime_semantics_unchanged'])
        self.assertEqual(recorded['final_checkpoint'], 'passed')
        self.assertIsNone(recorded['failed_phase'])

class SignedReleaseTargetPinsTests(unittest.TestCase):
    """Source matching only: a recorded Prepared receipt is not native proof."""

    PI_CONTEXT7_PIN = {
        "distribution": "777genius/context7",
        "repository": "777genius/universal-agent-plugins-registry",
        "path": "plugins/context7",
        "revision": "8360159c07140d17cbd4060a110553092c9cd245",
        "sequence": 7,
        "tree_digest": "sha256:635e673ec3aa8440a2826dc3a677ebe652b63e542db2e861b61d2df7f55e48b3",
        "manifest_digest": "sha256:1c1b8633e99d8ae10ed804b2c98532c4e08049b95f6229a6dd0b3746bdaea496",
    }

    @staticmethod
    def original_pi_receipt():
        # Complete public list JSON from original run 44939583899, artifact
        # 14732728467 (ZIP SHA256 bfa91a8ecb19b1367038def464b101097a6bdb574d03f3217977363bde01ca79).
        # It remains incomplete-native-proof; repeat/remove were not executed.
        return json.loads('''{
          "schema_version": 1, "command": "list", "result": "success",
          "data": {"installations": [{
            "installation_id": "3601ea6b-89d1-4a2e-853a-945d8b5abd19",
            "name": "context7", "version": "0.2.5",
            "source": "777genius/universal-agent-plugins-registry/plugins/context7",
            "clients": [{
              "client_id": "pi", "scope": "user", "materialization": "materialized",
              "activation": "prepared", "authentication": "not_checked",
              "policy": "allowed", "verification": "installation_verified",
              "package_revision": {
                "version": "0.2.5", "resolved_revision": "8360159c07140d17cbd4060a110553092c9cd245",
                "distribution_id": "777genius/context7", "release_sequence": 7,
                "tree_digest": "sha256:635e673ec3aa8440a2826dc3a677ebe652b63e542db2e861b61d2df7f55e48b3",
                "manifest_digest": "sha256:1c1b8633e99d8ae10ed804b2c98532c4e08049b95f6229a6dd0b3746bdaea496"
              }, "affected_surfaces": ["pi"]
            }],
            "directory": {
              "product_id": "context7", "recorded_distribution": "777genius/context7",
              "current_distribution": "777genius/context7",
              "recorded_revision": "8360159c07140d17cbd4060a110553092c9cd245",
              "current_revision": "8360159c07140d17cbd4060a110553092c9cd245",
              "recorded_release_sequence": 7, "current_release_sequence": 7,
              "recorded_snapshot_sequence": 49
            }, "mixed_version": false
          }]}
        }''')

    def test_original_pi_receipt_requires_its_exact_target_pin(self):
        audit = load_audit()
        doc = json.loads(audit.FIXTURES.read_text())
        audit.validate_fixtures(doc)
        self.assertEqual(doc["signed_release_pins_by_target"], {"pi": {"context7": self.PI_CONTEXT7_PIN}})
        response = self.original_pi_receipt()
        self.assertEqual(audit.matching_installations(
            response, "context7", "pi", "plugin", "context7", doc["signed_release_pins"]), [])
        selected = audit.effective_signed_release_pins(doc, "pi")
        self.assertEqual(audit.matching_installations(
            response, "context7", "pi", "plugin", "context7", selected), response["data"]["installations"])
        self.assertEqual(response["data"]["installations"][0]["clients"][0]["activation"], "prepared")
        for target in ("codex", "claude", "qoder"):
            with self.subTest(target=target):
                other = self.original_pi_receipt()
                other["data"]["installations"][0]["clients"][0]["client_id"] = target
                self.assertEqual(audit.matching_installations(
                    other, "context7", target, "plugin", "context7",
                    audit.effective_signed_release_pins(doc, target)), [])

    def test_effective_map_copies_pins_and_preserves_all_defaults(self):
        audit = load_audit()
        doc = json.loads(audit.FIXTURES.read_text())
        original = json.loads(json.dumps(doc))
        selected = audit.effective_signed_release_pins(doc, "pi")
        self.assertEqual(len(selected), 21)
        for alias, pin in doc["signed_release_pins"].items():
            self.assertEqual(selected[alias], self.PI_CONTEXT7_PIN if alias == "context7" else pin)
            self.assertIsNot(selected[alias], pin)
        for target in set(doc["targets"]) - {"pi"}:
            canonical = audit.audit_client_id(target, "plugin")
            self.assertEqual(audit.effective_signed_release_pins(doc, canonical), doc["signed_release_pins"])
        selected["context7"]["sequence"] = 99
        selected["atlassian"]["revision"] = "0" * 40
        selected.pop("cloudflare")
        self.assertEqual(doc, original)
        del doc["signed_release_pins_by_target"]
        audit.validate_fixtures(doc)
        self.assertEqual(audit.effective_signed_release_pins(doc, "pi"), doc["signed_release_pins"])

    def test_target_maps_reject_unknown_alias_target_and_empty_or_non_maps(self):
        audit = load_audit()
        fixture = json.loads(audit.FIXTURES.read_text())
        pin = self.PI_CONTEXT7_PIN
        direct = next(alias for alias in fixture["plugin_aliases"] if alias.startswith("github:"))
        invalid = [None, [], "pi", {}, {"unknown": {"context7": pin}},
                   {"PI": {"context7": pin}}, {"chatgpt": {"context7": pin}},
                   {"pi": None}, {"pi": []}, {"pi": {}},
                   {"pi": {"unknown": pin}}, {"pi": {direct: pin}}]
        for value in invalid:
            with self.subTest(value=value):
                doc = json.loads(json.dumps(fixture))
                doc["signed_release_pins_by_target"] = value
                with self.assertRaisesRegex(ValueError, "target-specific signed release"):
                    audit.validate_fixtures(doc)

    def test_default_and_target_pins_share_strict_shape_and_scalar_validation(self):
        audit = load_audit()
        fixture = json.loads(audit.FIXTURES.read_text())
        invalid = [None, [], {}, dict(self.PI_CONTEXT7_PIN, extra="not-a-pin-field")]
        invalid += [dict(self.PI_CONTEXT7_PIN, **{key: value}) for key, value in (
            ("sequence", True), ("sequence", 7.0), ("sequence", 0), ("sequence", -1),
            ("distribution", ""), ("repository", None), ("path", False),
            ("revision", None), ("revision", "a" * 39), ("revision", "A" * 40),
            ("tree_digest", "sha256:" + "b" * 63), ("manifest_digest", None))]
        missing = dict(self.PI_CONTEXT7_PIN)
        del missing["sequence"]
        invalid.append(missing)
        for route in ("default", "pi"):
            for value in invalid:
                with self.subTest(route=route, value=value):
                    doc = json.loads(json.dumps(fixture))
                    pins = (doc["signed_release_pins"] if route == "default" else
                            doc["signed_release_pins_by_target"]["pi"])
                    pins["context7"] = value
                    with self.assertRaisesRegex(ValueError, "frozen signed release pin"):
                        audit.validate_fixtures(doc)

    def test_coherent_wrong_release_and_mixed_pin_fields_are_rejected(self):
        audit = load_audit()
        doc = json.loads(audit.FIXTURES.read_text())
        selected = audit.effective_signed_release_pins(doc, "pi")
        defaults = doc["signed_release_pins"]["context7"]
        changes = [
            ("recorded_release_sequence", "release_sequence", 8),
            ("recorded_revision", "resolved_revision", "a" * 40),
            ("recorded_distribution", "distribution_id", "wrong/context7"),
            ("recorded_revision", "resolved_revision", defaults["revision"]),
            (None, "tree_digest", defaults["tree_digest"]),
            (None, "manifest_digest", defaults["manifest_digest"]),
        ]
        for directory_key, revision_key, value in changes:
            with self.subTest(field=revision_key, value=value):
                response = self.original_pi_receipt()
                node = response["data"]["installations"][0]
                if directory_key:
                    node["directory"][directory_key] = value
                node["clients"][0]["package_revision"][revision_key] = value
                self.assertEqual(audit.matching_installations(
                    response, "context7", "pi", "plugin", "context7", selected), [])
        response = self.original_pi_receipt()
        response["data"]["installations"][0]["source"] = defaults["repository"] + "/" + defaults["path"]
        self.assertEqual(audit.matching_installations(
            response, "context7", "pi", "plugin", "context7", selected), [])

    def test_main_selects_once_for_cases_and_expected_release_diagnostics(self):
        from io import StringIO

        for target, native in (("pi", "pi"), ("claude", "claude"), ("chatgpt", "codex")):
            with self.subTest(target=target), TemporaryDirectory() as temporary:
                audit = load_audit()
                fixture = json.loads(audit.FIXTURES.read_text())
                expected = audit.effective_signed_release_pins(fixture, native)
                output = StringIO()
                argv = ["live-install-audit.py", "--cli", sys.executable, "--target", target,
                        "--kind", "plugin", "--limit", "2", "--report-dir", temporary]
                with (patch.object(sys, "argv", argv), patch.object(sys, "stdout", output),
                      patch.dict(audit.AGENT_BINARIES, {native: sys.executable}, clear=True),
                      patch.object(audit, "vendor_bootstrap_attested", return_value=True),
                      patch.object(audit, "version_evidence", return_value={"status": "reported"}),
                      patch.object(audit, "effective_signed_release_pins",
                                   wraps=audit.effective_signed_release_pins) as select,
                      patch.object(audit, "do_case", return_value={"status": "incomplete-native-proof"}) as case):
                    self.assertEqual(audit.main(), 1)
                select.assert_called_once()
                self.assertEqual(select.call_args.args[1], native)
                self.assertEqual(case.call_count, 2)
                first, second = (call.args[7] for call in case.call_args_list)
                self.assertIs(first, second)
                self.assertEqual(second, expected)
                lines = [json.loads(line) for line in output.getvalue().splitlines() if line.startswith("{")]
                self.assertEqual(lines[1]["index"], 1)
                self.assertEqual(lines[1]["expected_signed_release"], expected["context7"])
                report = json.loads((Path(temporary) / f"{target}-plugin.json").read_text())
                self.assertEqual((report["passed"], report["prepared_lifecycle_passed"],
                                  report["verified_negative_coverage"]), (0, 0, 0))


class PiPluginNativeQualificationTests(unittest.TestCase):
    """Inert public transport with real source matching, files, and lifecycle gates."""

    def run_case(self, audit, scenario):
        doc = json.loads(audit.FIXTURES.read_text())
        pins = audit.effective_signed_release_pins(doc, "pi")

        def signed_node(name, activation, uid):
            pin = pins[name]
            return {"name": name, "installation_id": uid,
                    "source": pin["repository"] + "/" + pin["path"],
                    "directory": {"product_id": name, "recorded_distribution": pin["distribution"],
                                  "recorded_revision": pin["revision"], "recorded_release_sequence": pin["sequence"]},
                    "clients": [{"client_id": "pi", "scope": "user", "materialization": "materialized",
                                 "activation": activation,
                                 "authentication": "not_required" if activation == "active" else "not_checked",
                                 "policy": "allowed", "verification": "installation_verified",
                                 "package_revision": {"distribution_id": pin["distribution"],
                                     "resolved_revision": pin["revision"], "release_sequence": pin["sequence"],
                                     "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"]}}]}

        uid = "3601ea6b-89d1-4a2e-853a-945d8b5abd19"
        if scenario == "prepared-mcp":
            item, name = "context7", "context7"
            nodes = SignedReleaseTargetPinsTests.original_pi_receipt()["data"]["installations"]
            components = [{"kind": "mcp_server", "name": "context7", "support": "native"}]
        elif scenario == "active-skill":
            item = next(source for source, name in doc["plugin_declared_names"].items()
                        if name == "recipe-skills")
            name = "recipe-skills"
            nodes = [{"name": name, "installation_id": uid, "source": "workato-devs/recipe-skills",
                      "clients": [{"client_id": "pi", "scope": "user", "materialization": "materialized",
                                   "activation": "active", "authentication": "not_required", "policy": "allowed",
                                   "verification": "installation_verified",
                                   "package_revision": {"resolved_revision": item.rsplit("@", 1)[1]}}]}]
            components = [{"kind": "skill", "name": component, "support": "native"}
                          for component in ("asana-recipes", "datatable-recipes", "gmail-recipes", "jira-recipes",
                                            "salesforce-recipes", "slack-recipes", "stripe-recipes", "workato-recipes")]
        else:
            self.assertEqual(scenario, "active-root-prepared-mcp-child")
            item, name = "marketing-kit", "marketing-kit"
            nodes = [signed_node(name, "active", uid),
                     signed_node("notion", "prepared", "929803ef-7ca9-4577-a62d-1562fd4a5483")]
            components = [{"kind": "skill", "name": "marketing-plan", "support": "native"},
                          {"kind": "mcp_server", "name": "notion", "support": "native"}]
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            native = root / "plugin-00/home/.pi/agent"
            data = root / "plugin-00/uap-data"
            owned = []
            for component in components:
                if component["kind"] == "skill":
                    owned.append((native / "skills" / component["name"] / "SKILL.md",
                                  f'---\nname: {component["name"]}\ndescription: inert unit only\n---\n'))
                else:
                    owned.append((native / (component["name"] + ".json"),
                                  json.dumps({"mcpServers": {component["name"]: {"command": "never-executed"}}})))
            state = {"installed": False}
            operations = []

            def public(argv, _env, _cwd, timeout=120, **_kwargs):
                self.assertEqual(argv[0], "/unused/unit-agentplugins")
                operation = argv[1]
                operations.append(operation)
                if operation == "add":
                    self.assertEqual(argv[2], item)
                    if not state["installed"]:
                        for path, content in owned:
                            path.parent.mkdir(parents=True, exist_ok=True)
                            path.write_text(content)
                        (data / "uap.toml").write_text("# inert unit desired source\n")
                        (data / "uap.lock").write_text(json.dumps({"lock_version": 1, "packages": [{"alias": name}]}))
                        state["installed"] = True
                    results = []
                    actions = []
                    for node in nodes:
                        entry = {"installation_id": node["installation_id"], "client_id": "pi",
                                 "operation": "install", "outcome": "completed"}
                        if node["clients"][0]["activation"] == "prepared":
                            entry["manual_actions"] = [
                                "Reload resources or restart Pi to discover user Skills and MCP; "
                                "installed files do not prove an authenticated connection or model tool call."]
                        results.append(entry)
                        selected_components = (components if len(nodes) == 1 else
                            [component for component in components if
                             (component["kind"] == "skill") == (node["name"] == name)])
                        actions.append({"alias": node["name"], "client_id": "pi", "action": "install",
                                        "component_support": selected_components})
                    result = {"schema_version": 1, "command": "add", "result": "success",
                              "data": {"actions": actions, "results": results, "outcome": "completed",
                                       "scope": "user", "scope_selection_reason": "explicit"}}
                elif operation == "remove":
                    self.assertEqual(argv[2], name)
                    for path, _content in owned:
                        path.unlink()
                    (data / "uap.lock").write_text('{"lock_version":1,"packages":[]}')
                    state["installed"] = False
                    result = {"data": {}}
                else:
                    self.assertEqual(operation, "list")
                    result = {"data": {"installations": nodes if state["installed"] else []}}
                return {"exit_code": 0, "stdout_tail": json.dumps(result), "stderr_tail": "",
                        "timeout": False, "_json": json.loads(json.dumps(result))}

            # No matcher, attribution, snapshot, repeat, removal or status verifier is stubbed.
            with patch.object(audit, "call", side_effect=public):
                row = audit.do_case("/unused/unit-agentplugins", "pi", "plugin", item, 0,
                                    root, doc["plugin_declared_names"], pins)
            self.assertEqual(operations, ["add", "list", "add", "list", "remove", "list"])
            for key in ("listed_identity", "repeat_verified",
                        "repeat_native_unchanged", "repeat_lock_unchanged", "removal_verified", "sentinels_preserved"):
                self.assertTrue(row[key], key)
            self.assertEqual(row["repeat_installation_ids"], [uid])
            self.assertTrue(row["native_identity_files"])
            self.assertEqual(row["native_residuals"], [])
            self.assertFalse(row["prepared_binding"])
            self.assertFalse(row["prepared_components_only"])
            return row

    def test_clean_public_lifecycles_do_not_supply_independent_native_discovery(self):
        audit = load_audit()
        for scenario in ("prepared-mcp", "active-skill", "active-root-prepared-mcp-child"):
            with self.subTest(scenario=scenario):
                row = self.run_case(audit, scenario)
                self.assertEqual(row["status"], "package-installed-native-unqualified")
                self.assertFalse(audit.expected_negative_coverage(row, "agentplugins 0.1.115"))


class CopilotManagedSkillReceiptTest(unittest.TestCase):
    @staticmethod
    def fixture(project):
        # Identity constants are from the original Live Audit frontend-design
        # receipt. The inert payload below has its own independent blob pin.
        body = b"---\nname: frontend-design\ndescription: Inert source-bound fixture.\n---\n"
        item = {"name": "frontend-design", "folder": "frontend-design", "repo": "anthropics/skills",
                "ref": "ca45fc496a3436251f191354091d557da31c15b5",
                "source": "https://github.com/anthropics/skills/tree/ca45fc496a3436251f191354091d557da31c15b5/skills/frontend-design",
                "skill_md_git_blob_sha1": hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()}
        uid = "8695e2b5-596b-4385-96ea-cfbc9c02b5b2"
        name = "skill-b0a5b0253f26020d7acb797250495459c1bd758fe440530b3db1347e"
        physical = "skill-b0a5b0253f26020d7acb797250495459c1bd758fe4405-dc661c5b1ea6"
        destination = project / ".uap/managed/clients/copilot" / physical
        skill = destination / "skills/frontend-design/SKILL.md"
        skill.parent.mkdir(parents=True)
        skill.write_bytes(body)
        revision = {"resolved_revision": item["ref"], "tree_digest": "sha256:" + "b" * 64,
                    "manifest_digest": "sha256:" + "c" * 64}
        node = {"installation_id": uid, "name": name, "source": "anthropics/skills/skills/frontend-design",
                "clients": [{"client_id": "copilot", "scope": "project", "materialization": "materialized",
                             "activation": "active", "authentication": "not_required", "policy": "allowed",
                             "verification": "installation_verified", "package_revision": revision}]}
        receipt = {"schema_version": 1, "command": "skills add", "result": "success", "status": "succeeded",
                   "scope": "project", "readiness": "ready", "next_actions": [], "items": [{
                       "alias": item["name"], "targetId": "copilot", "status": "succeeded",
                       "candidateId": "sha256:b596c15579514b31901b660dcee94def801033e595d112ff5afb9ebfefea6e8b",
                       "destinationPath": str(destination)}]}
        package = {"alias": item["name"], "targets": ["copilot"], "tree_digest": revision["tree_digest"],
                   "manifest_digest": revision["manifest_digest"], "source": {"git": {
                       "kind": "skill", "skill": item["name"], "repository": item["repo"], "commit": item["ref"],
                       "path": "skills/frontend-design",
                       "resolved_skill_path": "skills/frontend-design"}}}
        lock = {"packages": [package]}
        (project / "uap.lock").write_text(json.dumps(lock), encoding="utf-8")
        (project / "uap.toml").write_text("frontend-design\n", encoding="utf-8")
        return {"project": project, "file": skill, "body": body, "item": item,
                "receipt": receipt, "node": node, "matches": [node], "lock": lock}

    def observed(self, audit, fixture, *, repeated=False):
        return audit.copilot_project_skill_path(fixture["project"], fixture["item"],
            fixture["receipt"], fixture["matches"], repeated=repeated)

    def test_managed_path_requires_complete_source_iid_receipt_and_lock(self):
        from copy import deepcopy
        changes = {
            "source": lambda f: f["node"].update(source="other/skills/skills/frontend-design"),
            "synthetic-name": lambda f: f["node"].update(name="skill-" + "d" * 56),
            "iid": lambda f: f["node"].update(installation_id="11111111-1111-4111-8111-111111111111"),
            "candidate": lambda f: f["receipt"]["items"][0].update(candidateId="sha256:" + "d" * 64),
            "destination": lambda f: f["receipt"]["items"][0].update(destinationPath=str(f["file"].parents[2]) + "-foreign"),
            "alias": lambda f: f["receipt"]["items"][0].update(alias="other-skill"),
            "target": lambda f: f["receipt"]["items"][0].update(targetId="qoder"),
            "client-scope": lambda f: f["node"]["clients"][0].update(scope="user"),
            "receipt-scope": lambda f: f["receipt"].update(scope="user"),
            "revision": lambda f: f["node"]["clients"][0]["package_revision"].update(resolved_revision="d" * 40),
            "tree": lambda f: f["node"]["clients"][0]["package_revision"].update(tree_digest="sha256:" + "d" * 64),
            "manifest": lambda f: f["node"]["clients"][0]["package_revision"].update(manifest_digest="sha256:" + "d" * 64),
            "duplicate-match": lambda f: f["matches"].append(deepcopy(f["node"])),
            "duplicate-item": lambda f: f["receipt"]["items"].append(deepcopy(f["receipt"]["items"][0])),
            "duplicate-client": lambda f: f["node"]["clients"].append(deepcopy(f["node"]["clients"][0])),
            "duplicate-lock": lambda f: f["lock"]["packages"].append(deepcopy(f["lock"]["packages"][0])),
            "lock-source": lambda f: f["lock"]["packages"][0]["source"]["git"].update(repository="other/skills"),
            "lock-revision": lambda f: f["lock"]["packages"][0]["source"]["git"].update(commit="d" * 40),
            "lock-subpath": lambda f: f["lock"]["packages"][0]["source"]["git"].update(resolved_skill_path="skills/other"),
            "lock-skill": lambda f: f["lock"]["packages"][0]["source"]["git"].update(skill="other-skill"),
            "lock-target": lambda f: f["lock"]["packages"][0].update(targets=["qoder"]),
            "lock-digest": lambda f: f["lock"]["packages"][0].update(tree_digest="sha256:" + "d" * 64),
            "bytes": lambda f: f["file"].write_bytes(f["body"] + b"changed\n"),
            "status": lambda f: f["receipt"]["items"][0].update(status="unchanged"),
            "schema-bool": lambda f: f["receipt"].update(schema_version=True),
        }
        audit = load_audit()
        for mutation in (None, *changes, "duplicate-json-key"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve() / "case/project")
                if mutation in changes:
                    changes[mutation](f)
                wire = json.dumps(f["lock"])
                if mutation == "duplicate-json-key":
                    wire = '{"packages":[],' + wire[1:]
                (f["project"] / "uap.lock").write_text(wire, encoding="utf-8")
                found = self.observed(audit, f)
                if mutation is not None:
                    self.assertIsNone(found)
                else:
                    self.assertEqual(found, str(f["file"].relative_to(f["project"])))
                    f["receipt"]["items"][0]["status"] = "unchanged"
                    self.assertEqual(self.observed(audit, f, repeated=True), found)
                    # The finder does not manufacture readiness from bytes.
                    f["receipt"]["readiness"] = "needs_action"
                    f["node"]["clients"][0]["activation"] = "prepared"
                    self.assertEqual(self.observed(audit, f, repeated=True), found)
                    self.assertEqual(f["receipt"]["readiness"], "needs_action")

    def test_managed_path_rejects_links_relocated_namesakes_and_removed_payload(self):
        audit = load_audit()
        for mutation in ("file-symlink", "ancestor-symlink", "file-hardlink", "lock-symlink", "relocated", "removed"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve() / "case/project")
                installed = self.observed(audit, f)
                self.assertIsNotNone(installed)
                foreign = f["project"].parent / "foreign.md"
                foreign.write_bytes(f["body"])
                if mutation == "lock-symlink":
                    foreign.write_text(json.dumps(f["lock"]))
                    lock = f["project"] / "uap.lock"
                    lock.unlink(); lock.symlink_to(foreign)
                elif mutation == "ancestor-symlink":
                    original = f["file"].parent
                    moved = original.with_name("foreign-directory")
                    original.rename(moved); original.symlink_to(moved, target_is_directory=True)
                else:
                    f["file"].unlink()
                    if mutation == "file-symlink":
                        f["file"].symlink_to(foreign)
                    elif mutation == "file-hardlink":
                        os.link(foreign, f["file"])
                    elif mutation == "relocated":
                        stray = f["project"] / ".uap/managed/clients/copilot/foreign/skills/frontend-design/SKILL.md"
                        stray.parent.mkdir(parents=True); stray.write_bytes(f["body"])
                        # Recursive scanning is retained only as negative
                        # post-remove evidence, never as a positive substitute.
                        self.assertIsNotNone(audit.skill_file_present(f["project"], "copilot", "frontend-design"))
                self.assertIsNone(self.observed(audit, f))
                if mutation in ("file-symlink", "ancestor-symlink", "file-hardlink"):
                    self.assertTrue(audit.skill_removal_leftovers(
                        f["project"], "copilot", "frontend-design", installed))
                expected = json.dumps(f["lock"]).encode() if mutation == "lock-symlink" else f["body"]
                self.assertEqual(foreign.read_bytes(), expected)

    def test_real_case_requires_both_receipts_clean_removal_and_public_ready(self):
        from copy import deepcopy
        import shutil
        for scenario in ("ready", "prepared", "repeat-receipt", "repeat-bytes", "remove-residue", "remove-relocated"):
            with self.subTest(scenario=scenario), TemporaryDirectory() as temporary:
                audit = load_audit()
                root = Path(temporary).resolve()
                item = self.fixture(root / "template")["item"]
                state = {"count": 0, "installed": False}
                def call(argv, env, cwd, timeout=105, **kwargs):
                    command = argv[1]
                    if command == "skills":
                        state["count"] += 1
                        if state["count"] == 1:
                            state["fixture"] = self.fixture(Path(cwd))
                            state["installed"] = True
                        f = state["fixture"]
                        doc = deepcopy(f["receipt"])
                        if state["count"] == 2:
                            doc["items"][0]["status"] = "unchanged"
                            if scenario == "repeat-receipt":
                                doc["items"][0]["candidateId"] = "sha256:" + "d" * 64
                            elif scenario == "repeat-bytes":
                                f["file"].write_bytes(f["body"] + b"changed on repeat\n")
                        if scenario == "prepared":
                            doc["readiness"] = "needs_action"
                            doc["next_actions"] = [{"code": "verify_native"}]
                            f["node"]["clients"][0]["activation"] = "prepared"
                    elif command == "list":
                        doc = {"schema_version": 1, "command": "list", "result": "success", "data": {
                            "installations": [deepcopy(state["fixture"]["node"])] if state["installed"] else []}}
                    elif command == "remove":
                        f = state["fixture"]
                        destination = f["file"].parents[2]
                        if scenario == "remove-relocated":
                            destination.rename(destination.with_name("foreign-relocated"))
                        elif scenario != "remove-residue":
                            shutil.rmtree(destination)
                        (f["project"] / "uap.lock").write_text('{"packages":[]}')
                        state["installed"] = False
                        doc = {"schema_version": 1, "command": "agentplugins remove", "result": "success", "data": {}}
                    else:
                        raise AssertionError("unexpected public command: " + repr(argv))
                    return {"exit_code": 0, "timeout": False, "stdout_tail": json.dumps(doc),
                            "stderr_tail": "", "_json": doc}
                with patch.object(audit, "call", side_effect=call), \
                     patch.object(audit, "initialize_project_skill_git_root"):
                    row = audit.do_case("/unused/agentplugins", "copilot", "skill", item, 0, root, {})
                self.assertTrue(row["skill_path"])
                self.assertTrue(row["skill_bytes_match"])
                self.assertTrue(row["lock_exact_source_pinned"])
                self.assertTrue(row["sentinels_preserved"])
                if scenario == "ready":
                    self.assertEqual(row["status"], "passed")
                    self.assertTrue(row["repeat_verified"])
                    self.assertTrue(row["repeat_project_unchanged"])
                    self.assertTrue(row["removal_verified"])
                else:
                    self.assertNotEqual(row["status"], "passed")
                    if scenario == "prepared":
                        self.assertEqual(row["status"], "incomplete-native-proof")
                        self.assertTrue(row["repeat_verified"])
                        self.assertTrue(row["removal_verified"])
                        self.assertFalse(any(row["public_skill_ready"].values()))
                    elif scenario.startswith("repeat"):
                        self.assertFalse(row["repeat_verified"])
                        self.assertTrue(row["removal_verified"])
                    else:
                        self.assertFalse(row["removal_verified"])
                        self.assertTrue(row["post_removal_skill"])
                        self.assertTrue(row["post_skill_tree_delta"])


class CopilotManagedSkillConsistencyTest(unittest.TestCase):
    def run_case(self, *, client_change=None, envelope_change=None,
                 tree_change=None, prepared=False):
        from copy import deepcopy
        import shutil
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            item = CopilotManagedSkillReceiptTest.fixture(root / "template")["item"]
            state = {"adds": 0, "installed": False, "calls": []}

            def mutate(document, key, value):
                if value is None:
                    document.pop(key, None)
                else:
                    document[key] = value

            def call(argv, env, cwd, timeout=105, **kwargs):
                command = argv[1]
                state["calls"].append(command)
                if command == "skills":
                    state["adds"] += 1
                    if state["adds"] == 1:
                        state["fixture"] = CopilotManagedSkillReceiptTest.fixture(Path(cwd))
                        state["installed"] = True
                    f = state["fixture"]
                    doc = deepcopy(f["receipt"])
                    if state["adds"] == 2:
                        doc["items"][0]["status"] = "unchanged"
                        if tree_change == "empty-directory":
                            (f["file"].parents[2] / "unexpected-empty").mkdir()
                        elif tree_change == "file-mode":
                            f["file"].chmod(f["file"].stat().st_mode ^ 0o100)
                        elif tree_change == "same-byte-replacement":
                            replacement = f["file"].with_name("replacement.tmp")
                            replacement.write_bytes(f["body"])
                            replacement.replace(f["file"])
                        elif tree_change == "file-mtime":
                            before = f["file"].stat()
                            os.utime(f["file"], ns=(before.st_atime_ns, before.st_mtime_ns + 1000000000))
                    if prepared:
                        doc["readiness"] = "needs_action"
                        doc["next_actions"] = [{"code": "verify_native"}]
                    phase = "add" if state["adds"] == 1 else "repeat"
                elif command == "list":
                    nodes = [deepcopy(state["fixture"]["node"])] if state["installed"] else []
                    phase = ("after-add" if state["adds"] == 1 else "repeat-add") if nodes else "after-remove"
                    if nodes:
                        client = nodes[0]["clients"][0]
                        if prepared:
                            client.update(activation="prepared", authentication="not_checked")
                        if client_change is not None and client_change[0] == phase:
                            mutate(client, *client_change[1:])
                    doc = {"schema_version": 1, "command": "list", "result": "success",
                           "data": {"installations": nodes}}
                elif command == "remove":
                    f = state["fixture"]
                    shutil.rmtree(f["file"].parents[2])
                    (f["project"] / "uap.lock").write_text('{"packages":[]}')
                    state["installed"] = False
                    phase = "remove"
                    doc = {"schema_version": 1, "command": "agentplugins remove", "result": "success", "data": {}}
                else:
                    raise AssertionError("unexpected public command")
                if envelope_change is not None and envelope_change[0] == phase:
                    mutate(doc, *envelope_change[1:])
                return {"exit_code": 0, "timeout": False, "stdout_tail": json.dumps(doc),
                        "stderr_tail": "", "_json": doc}

            with patch.object(audit, "call", side_effect=call), \
                 patch.object(audit, "initialize_project_skill_git_root"), \
                 patch.object(audit.subprocess, "run", side_effect=AssertionError("real CLI forbidden")), \
                 patch.object(audit.subprocess, "Popen", side_effect=AssertionError("real CLI forbidden")):
                row = audit.do_case("/unused/agentplugins", "copilot", "skill", item, 0, root, {})
            self.assertEqual(state["calls"], ["skills", "list", "skills", "list", "remove", "list"])
            self.assertTrue(row["sentinels_preserved"])
            return row

    def test_fresh_copilot_lock_requires_exact_single_target(self):
        audit = load_audit()
        for targets in (["copilot"], ["copilot", "qoder"], ["copilot", False],
                        ["copilot", {}], ["copilot", "copilot"], [], "copilot"):
            with self.subTest(targets=targets), TemporaryDirectory() as temporary:
                f = CopilotManagedSkillReceiptTest.fixture(Path(temporary).resolve() / "case/project")
                f["lock"]["packages"][0]["targets"] = targets
                (f["project"] / "uap.lock").write_text(json.dumps(f["lock"]))
                found = audit.copilot_project_skill_path(f["project"], f["item"], f["receipt"], f["matches"])
                if targets == ["copilot"]:
                    self.assertEqual(found, str(f["file"].relative_to(f["project"])))
                else:
                    self.assertIsNone(found)

    def test_copilot_ready_requires_both_public_client_states(self):
        ready = self.run_case()
        self.assertEqual(ready["status"], "passed")
        for phase in ("after-add", "repeat-add"):
            for key, values in (
                    ("activation", ("prepared", "manual_activation_required", "failed", "not_required", "invalid", None)),
                    ("authentication", ("not_checked", "auth_pending", "failed", "authenticated", "invalid", None))):
                for value in values:
                    with self.subTest(phase=phase, key=key, value=value):
                        row = self.run_case(client_change=(phase, key, value))
                        self.assertEqual(row["status"], "incomplete-native-proof")
                        self.assertTrue(row["skill_bytes_match"])
                        self.assertTrue(row["repeat_verified"])
                        self.assertTrue(row["removal_verified"])
                        self.assertFalse(row["public_skill_ready"][phase])
                        other = "repeat-add" if phase == "after-add" else "after-add"
                        self.assertTrue(row["public_skill_ready"][other])
        # Unready files still receive the full repeat/remove lifecycle.
        prepared = self.run_case(prepared=True)
        self.assertEqual(prepared["status"], "incomplete-native-proof")
        self.assertTrue(prepared["skill_bytes_match"])
        self.assertTrue(prepared["repeat_verified"])
        self.assertTrue(prepared["removal_verified"])
        self.assertFalse(any(prepared["public_skill_ready"].values()))

    def test_fresh_copilot_lock_requires_both_exact_skill_paths(self):
        audit = load_audit()
        for key in ("path", "resolved_skill_path"):
            for value in (None, "other/frontend-design", False, {}):
                with self.subTest(key=key, value=value), TemporaryDirectory() as temporary:
                    f = CopilotManagedSkillReceiptTest.fixture(Path(temporary).resolve() / "case/project")
                    git = f["lock"]["packages"][0]["source"]["git"]
                    if value is None:
                        git.pop(key, None)
                    else:
                        git[key] = value
                    (f["project"] / "uap.lock").write_text(json.dumps(f["lock"]))
                    self.assertIsNone(audit.copilot_project_skill_path(
                        f["project"], f["item"], f["receipt"], f["matches"]))

    def test_copilot_ready_requires_both_successful_list_envelopes(self):
        for phase in ("after-add", "repeat-add"):
            for key, value in (("schema_version", True), ("schema_version", 2),
                               ("command", "doctor"), ("result", "failure"),
                               ("result", "partial"), ("result", None)):
                with self.subTest(phase=phase, key=key, value=value):
                    row = self.run_case(envelope_change=(phase, key, value))
                    self.assertEqual(row["status"], "incomplete-native-proof")
                    self.assertTrue(row["repeat_verified"])
                    self.assertTrue(row["removal_verified"])
                    self.assertFalse(row["public_skill_ready"][phase])

    def test_copilot_cleanup_requires_successful_remove_and_list_envelopes(self):
        for phase in ("remove", "after-remove"):
            for key, value in (("schema_version", True), ("schema_version", 2),
                               ("command", "doctor"), ("command", "remove"), ("result", "failure"),
                               ("result", "partial"), ("result", None)):
                with self.subTest(phase=phase, key=key, value=value):
                    row = self.run_case(envelope_change=(phase, key, value))
                    self.assertNotEqual(row["status"], "passed")
                    self.assertTrue(row["repeat_verified"])
                    self.assertFalse(row["removal_verified"])
                    if phase == "after-remove":
                        self.assertFalse(row["post_list_schema_valid"])

    def test_copilot_repeat_preserves_managed_tree_custody(self):
        for mutation in ("empty-directory", "file-mode", "same-byte-replacement", "file-mtime"):
            with self.subTest(mutation=mutation):
                row = self.run_case(tree_change=mutation)
                self.assertNotEqual(row["status"], "passed")
                self.assertTrue(row["skill_bytes_match"])
                self.assertTrue(row["removal_verified"])
                self.assertFalse(row["repeat_project_unchanged"])
                self.assertFalse(row["repeat_verified"])



class KiroPreparedInstallationAcceptanceContracts(unittest.TestCase):
    def original_report(self):
        import base64
        import hashlib
        import zlib

        # Fixed original artifact 14185522204 at b9d1cfff: all 30 ordered rows.
        # Retain only existing acceptance facts and the complete recorded
        # marketing-kit tails. Omit unrelated successful command bodies.
        # Original JSON SHA256: 5b0f8da23f14a6f4333cff6fd0629a2ad5388ebe0c79a1d0c5d5de31dfd4ef24.
        encoded = (
            "eNrlWl2v4jYQ/SsozzdLnO/QF6pWqqq+9L2skGOPwcXEke3cj726/72TACHArrQPuw0KLwjs2J5zzoztGfLuMWrB"
            "rqlzsK8dcG8RBU8eUxIqt34GY6WuvMW7B6/SrZnm4C2w3zrqGustPAO1Nu2wJ++ZqgZ7vZ002scJZtGn4FPgfTx5"
            "Qr66xgAObyp3WEA0SuFvXIBucJAzDTzhyArX92rVbGSFM9aKOqHNHts4NS9dmwHbKIdL//PuCSoV8LWs0ByczsBG"
            "WgemBSGosjjjzRNW8tbIY7fkiFK6N1yAOkWtlbRdA82A1w7nceAtAU7uQTfuONPHre0tC+sa6aFoz1pJAeyNKWgp"
            "lUK2Jh4wKzT5O6Y3sNdI8M1wXACou2m2La4KlG1NsGCeB126MQyuEPd6yorpfa3AgV9RJ5/Br43WAh9x1GzAHfVF"
            "VX8s/UxXDl5ddmafTJv9AeDxyd8avQefw7PTWtmzBuG0NciybAOVbOz8igG/NJLjvnQP2ijdcKGQw7Ms0cRDYwh5"
            "dAG4Zjsw/rYpzwLE0xbgAvLoAgi52dMz98m0uT+hvQPaDZR4Nzwzn06d+R7wXZDPDH1RZ/az6bN/Qjw6/RvpLvb7"
            "/FHuQQfgd3T9QYMUHShRTFuJHu74xCMKhw8PErKJ58MDxKOzvwWjd82A+4lnwz3e0ZlXaCo1A+YnngP3eEdnfk/N"
            "DtDejb+TbiDAZba7/te2BcmqUeppSBdpEXAwZu1wxba8hUeaOwhhF7NKz0DJjSwVzHh71XDavM0MKMAb30xoM1td"
            "GrDyFrPzuXjRtejHcQ0Wp3Yz29RtHXR2Ym52oGpmwf0y20trceisZW11qLZxlPVkqPdD/OjI58mRvkVTcE3TjTnv"
            "K8+yLezpqfq7asldeQhtTyuOv1bdKisPGw/V2K7NNoyBtV0zp45iI051VA79SFfYt/jn88dHx8EN5HbO1lF6hzgi"
            "OvjgWqAbrTkoRzsQx9bT0113WxT+3HcdPA3nOLTW2rp1Fz+DUV/x19sQ6zuuYuzY/j1Bdu3afaxhpt3scQLgvjb+"
            "4Xm/qY7u1JXVf27QVaAHFW8y8cLGEe3oWx1uGfKC94kXNXq8ozNfK/r2YuRmOzxhJl7YuMA8ugKtxeZtwP7ECxs9"
            "3vGZd0bWw4Ru4mWNHu/4zDc1vaylkokXMgaIR2cf7WcwqKSGEy9k9HjH93tatV/OzJPJl++2Tbk4wPalnndJqN/+"
            "p4wX/2UEPKIkjkTOo5hEgtIsKwLKgjIMs1hgZ07TKAzuQjoDPrw6qOzFTTX8yeWQqzR2NAk3sJeVbF9jOrNg50jK"
            "EoULShIXqFqQFiDCIMwhDsuoLAQqG2MWXEQJpEMRa2rt/5DMSdd+pcovG6kwyR/IFj2GbCcG5kcGfLuTStllyBgX"
            "WRYleRmQnPKYCxImcRjSIM0E5DGkEQfCRxBto/VGgd+99OBbp7sX4s7CxQ+xY/7RkfBby8Hfx5f+5l8jxj9AW5ZR"
            "nEJUpEUuUFjOMkIYjQAw8GLCcl5SSigNy/t4g4c2HHzL2lcqkSDmYzLmLqIzeYzo/Mv/HXdS8H/9c/4tUpZJWOBG"
            "mkW0jMs4L8IkCDMSJ3kQ8KyMCyhSxvCkZCNEKihoa3XW1wL98W3431iYPoaCujR07rZ4NbjmYhkkgsUi4GFZZGXG"
            "gjjkvCjigvI0yZMUz0hKojCGEYTDuAfHttQdj4OBbtlDbK89AfNrKpZ5moSCAI0JJyEkQZKTMOIlATwSS1qCKFhB"
            "ckHuYSdt5Jcv4DfSt0rXAxHzxwi+Dv68+1xirAUECkiTmJVU4AU0KwNW5CQOBctLvO4ELIZsjGgzwGQNt6FWPIZK"
            "L9rsqNPtu8R2fsHFkgpKIAsLGmAOkRKWlVGY4/EmKMdbTBgHQUGY+L6z7fNp5bXFvZi19fX1KSpPpl0N+Q9g5vO/"
        )
        body = zlib.decompress(base64.b64decode(encoded))
        self.assertEqual(len(body), 12780)
        self.assertEqual(hashlib.sha256(body).hexdigest(),
                         "0a7973589bdf8f0280b7b779d4a242c84ee72c71001f61c738d4e428b335a437")
        return json.loads(body)

    def invoke(self, report, *, acceptance=None, target="kiro",
               kind="plugin", platform="darwin", limit=30):
        import copy
        from contextlib import redirect_stderr, redirect_stdout
        from io import StringIO

        audit = load_audit()
        with TemporaryDirectory() as directory:
            output = Path(directory) / "report"
            argv = ["live-install-audit", "--cli", sys.executable,
                    "--target", target, "--kind", kind, "--limit", str(limit),
                    "--report-dir", str(output)]
            if acceptance is not None:
                argv.extend(("--acceptance", acceptance))
            stdout, stderr = StringIO(), StringIO()
            versions = [{"status": "reported", "exit_code": 0,
                         "value": "agentplugins 0.1.95"}, report["client_version"]]
            with (patch.object(audit.sys, "argv", argv),
                  patch.object(audit.sys, "platform", platform),
                  patch.object(audit, "vendor_bootstrap_attested", return_value=True),
                  patch.dict(audit.AGENT_BINARIES, {"kiro-cli": sys.executable}, clear=True),
                  patch.object(audit, "version_evidence", side_effect=versions) as version,
                  patch.object(audit, "do_case", side_effect=copy.deepcopy(report["results"])) as cases,
                  redirect_stdout(stdout), redirect_stderr(stderr)):
                try:
                    code = audit.main()
                except SystemExit as error:
                    code = error.code
            saved = output / f"{target}-{kind}.json"
            return (code, json.loads(saved.read_text()) if saved.exists() else None,
                    stdout.getvalue(), version.call_count, cases.call_count)

    def test_original_thirty_rows_accept_installation_and_preserve_native_refusal(self):
        original = self.original_report()
        default, native, _, _, _ = self.invoke(original)
        self.assertEqual(default, 1)
        self.assertEqual(native["passed"], 6)
        self.assertEqual(native["prepared_lifecycle_passed"], 23)
        self.assertEqual(native["verified_negative_coverage"], 0)
        self.assertNotIn("installation_acceptance_passed", native)

        code, measured, output, version_calls, case_calls = self.invoke(
            original, acceptance="kiro-prepared-installation")
        self.assertEqual(code, 0)
        self.assertEqual((version_calls, case_calls), (2, 30))
        self.assertEqual(measured["results"], original["results"])
        self.assertEqual(measured["passed"], 6)
        self.assertEqual(measured["prepared_lifecycle_passed"], 23)
        self.assertEqual(measured["verified_negative_coverage"], 0)
        self.assertEqual(measured["installation_passed"], 29)
        self.assertEqual(measured["installation_prepared_lifecycle_passed"], 23)
        self.assertEqual(measured["installation_verified_negative_coverage"], 1)
        self.assertIs(measured["installation_acceptance_passed"], True)
        self.assertIn("Prepared installation", output)

    def test_mutated_lifecycles_fixed_negative_and_source_pin_refuse(self):
        import copy

        audit = load_audit()
        original = self.original_report()
        fixtures = json.loads(audit.FIXTURES.read_text())
        mutations = (
            (0, "kiro_prepared_lifecycle_verified", False),
            (0, "kiro_prepared_lifecycle_verified", 1),
            (0, "status", "passed"),
            (0, "repeat_verified", False),
            (0, "removal_verified", False),
            (0, "sentinels_preserved", False),
            (0, "failed_install_registered", True),
            (0, "failed_install_residue", True),
            (0, "target", "cursor"),
            (0, "source", "foreign-source"),
            (0, "index", 1),
            (0, "index", False),
            (22, "removal_verified", False),
            (13, "native_file_delta", 1),
            (13, "native_file_delta", False),
            (13, "native_identity_files", ["foreign"]),
            (13, "listed_identity", True),
            (13, "failed_install_registered", True),
            (13, "failed_install_residue", True),
            (13, "sentinels_preserved", False),
            (13, "native_residuals", ["foreign"]),
            (13, "post_remove_identity_registered", True),
            (13, "status", "install-failed"),
        )
        for index, field, value in mutations:
            with self.subTest(index=index, field=field, value=value):
                changed = copy.deepcopy(original)
                changed["results"][index][field] = value
                self.assertIs(audit.kiro_prepared_installation_summary(
                    changed, fixtures)["installation_acceptance_passed"], False)
        for command, field, value in (
                ("install", "exit_code", 0), ("install", "timeout", True),
                ("install", "stdout_tail", "unexpected output"),
                ("install", "stderr_tail", "unsupported"),
                ("install", "stderr_tail", original["results"][13]["install"]["stderr_tail"].replace(
                    "missing kiro", "missing cursor")),
                ("list", "exit_code", 1), ("list", "timeout", True),
                ("list", "stdout_tail", '{"result":"success","data":{"installations":[{}]}}')):
            with self.subTest(command=command, field=field):
                changed = copy.deepcopy(original)
                changed["results"][13][command][field] = value
                self.assertIs(audit.kiro_prepared_installation_summary(
                    changed, fixtures)["installation_acceptance_passed"], False)
        for field, value in (
                ("revision", "0" * 40), ("sequence", 2), ("sequence", True),
                ("distribution", "foreign/kit"),
                ("tree_digest", "sha256:" + "0" * 64)):
            with self.subTest(pin_field=field):
                changed = copy.deepcopy(fixtures)
                changed["signed_release_pins"]["marketing-kit"][field] = value
                self.assertIs(audit.kiro_prepared_installation_summary(
                    original, changed)["installation_acceptance_passed"], False)
        for field, value in (
                ("platform", "linux"), ("full_coverage", False),
                ("client_version", {"status": "reported", "value": "kiro-cli 3.0.1",
                                    "exit_code": 0}),
                ("client_version", {"status": "reported", "value": "kiro-cli 3.0.0",
                                    "exit_code": False})):
            with self.subTest(report_field=field):
                changed = copy.deepcopy(original)
                changed[field] = value
                self.assertIs(audit.kiro_prepared_installation_summary(
                    changed, fixtures)["installation_acceptance_passed"], False)
        changed = copy.deepcopy(original)
        changed["results"].pop()
        self.assertIs(audit.kiro_prepared_installation_summary(
            changed, fixtures)["installation_acceptance_passed"], False)

    def test_installation_mode_refuses_other_targets_platforms_and_partial_selection(self):
        for target, kind, platform, limit in (
                ("cursor", "plugin", "darwin", 30),
                ("grok", "plugin", "darwin", 30),
                ("kiro", "skill", "darwin", 30),
                ("kiro", "plugin", "linux", 30),
                ("kiro", "plugin", "win32", 30),
                ("kiro", "plugin", "darwin", 29)):
            with self.subTest(target=target, kind=kind, platform=platform, limit=limit):
                code, report, _, versions, cases = self.invoke(
                    self.original_report(), acceptance="kiro-prepared-installation",
                    target=target, kind=kind, platform=platform, limit=limit)
                self.assertEqual((code, report, versions, cases), (2, None, 0, 0))


class ChatGPTDesktopAuditContracts(unittest.TestCase):
    def test_workflow_reuses_reviewed_sqlite_pin_and_checks_selected_member(self):
        import hashlib
        from runpy import run_path
        from types import SimpleNamespace
        scripts = Path(__file__).resolve().parent
        matrix = run_path(str(scripts / "run-native-client-matrix.py"))
        workflow = (scripts.parent / ".github/workflows/live-install-remaining-clients.yml").read_text()
        source = textwrap.dedent(workflow.split("<<'CHATGPT_CODEX'\n", 1)[1].split(
            "\n          CHATGPT_CODEX\n", 1)[0])
        selected_pin = matrix["PINS"]["linux-amd64"]["codex"]
        self.assertEqual(selected_pin[2:5], (
            "rust-v0.153.4", "codex-x86_64-unknown-linux-musl.tar.gz",
            "sha256:f479424eca092484dc40d87ae28c44f4cc40234a60045d6131e493800d814a30"))
        self.assertNotEqual(selected_pin, matrix["CODEX_SOURCE_PINS"]["linux-amd64"])
        for mutation in (None, "symlink", "not-executable", "wrong-name"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                (root / "vendor-proof").mkdir()
                def provision(pin, directory, name):
                    self.assertEqual(pin, selected_pin)
                    self.assertEqual(name, "codex")
                    self.assertEqual(directory, root / "vendor-root")
                    binary = directory / ("other" if mutation == "wrong-name" else name)
                    body = b"inert test bytes; not executed"
                    binary.write_bytes(body)
                    binary.chmod(0o600 if mutation == "not-executable" else 0o700)
                    if mutation == "symlink":
                        leaf = binary.with_name("foreign")
                        binary.rename(leaf)
                        binary.symlink_to(leaf)
                    return binary, {"source": "https://github.com/openai/codex/releases/tag/" + pin[2],
                                    "version": pin[2], "archive": pin[3],
                                    "archive_integrity": pin[4],
                                    "binary_sha256": hashlib.sha256(body).hexdigest()}
                module = SimpleNamespace(
                    PINS={"linux-amd64": {"codex": selected_pin}}, provision=provision)
                spec = SimpleNamespace(loader=SimpleNamespace(exec_module=lambda _module: None))
                with (patch("sys.argv", ["-", str(root)]),
                      patch("sys.platform", "linux"), patch("platform.machine", return_value="x86_64"),
                      patch("importlib.util.spec_from_file_location", return_value=spec) as locate,
                      patch("importlib.util.module_from_spec", return_value=module)):
                    if mutation is None:
                        exec(compile(source, "workflow-chatgpt-codex", "exec"), {})
                    else:
                        with self.assertRaisesRegex(SystemExit, "unverified staged Codex"):
                            exec(compile(source, "workflow-chatgpt-codex", "exec"), {})
                    locate.assert_called_once_with("chatgpt_codex_matrix",
                                                   Path.cwd() / "scripts/run-native-client-matrix.py")
                receipt = root / "vendor-proof/codex-runtime.json"
                if mutation is None:
                    proof = json.loads(receipt.read_text())
                    self.assertEqual(proof["public_target"], "chatgpt")
                    self.assertEqual(proof["canonical_client_id"], "codex")
                    self.assertEqual(proof["archive_integrity"], selected_pin[4])
                    self.assertEqual(proof["executable"], str(root / "vendor-root/codex"))
                else:
                    self.assertFalse(receipt.exists())

    def run_public_case(self, *, mutation=None, target="chatgpt"):
        import copy
        audit = load_audit()
        fixtures = json.loads(audit.FIXTURES.read_text())
        pin = fixtures["signed_release_pins"]["atlassian"]
        node = {"name": "atlassian", "installation_id": "owned-codex-binding",
                "source": pin["repository"].strip("/") + "/" + pin["path"].strip("/"),
                "directory": {"product_id": "atlassian",
                              "recorded_distribution": pin["distribution"],
                              "recorded_revision": pin["revision"],
                              "recorded_release_sequence": pin["sequence"]},
                "clients": [{"client_id": "codex", "scope": "user",
                             "verification": "installation_verified",
                             "package_revision": {
                                 "distribution_id": pin["distribution"],
                                 "resolved_revision": pin["revision"],
                                 "release_sequence": pin["sequence"],
                                 "tree_digest": pin["tree_digest"],
                                 "manifest_digest": pin["manifest_digest"]}}]}
        if mutation == "wrong-client":
            node["clients"][0]["client_id"] = "chatgpt"
        elif mutation == "wrong-source":
            node["clients"][0]["package_revision"]["resolved_revision"] = "0" * 40
        commands = []
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "plugin-00"
            config = case / "home/.codex/config.toml"
            market = case / "managed-marketplace"
            executable = root / "pinned-codex"
            executable.write_text("inert selected executable; call port only")
            state = {"installed": False, "listed": 0}
            marketplace = "agentplugins-012345abcdef"
            sqlite_pin = "sqlite_home = " + json.dumps(str(case / "home/.local/state/codex-sqlite")) + "\n"

            async def discover(binary, project, profile, environment, **kwargs):
                import sqlite3
                from contextlib import closing
                database = case / "home/.local/state/codex-sqlite/state_5.sqlite"
                with closing(sqlite3.connect(database)) as connection:
                    connection.execute("CREATE TABLE fixture_rows (value TEXT)")
                    connection.execute("INSERT INTO fixture_rows VALUES ('stable')")
                    connection.commit()
                if mutation == "prebaseline-config":
                    body = config.read_bytes()
                    config.unlink()
                    config.write_bytes(body)
                if mutation == "prebaseline-foreign":
                    (case / "home/.codex/foreign-neighbor.txt").write_text("altered before adoption")
                if mutation == "prebaseline-managed":
                    (project / 'unexpected-managed-root').mkdir()
                inventory = {"data": [{"cwd": str(project), "skills": [], "errors": []}]}
                return ("0.162.0", inventory) if kwargs.get("with_metadata") else inventory

            def reply(code=0, rows=None):
                doc = {"schema_version": 1, "result": "success",
                       "data": {"installations": rows or []}}
                return {"exit_code": code, "timeout": False,
                        "stdout_tail": json.dumps(doc), "stderr_tail": "",
                        "_json": copy.deepcopy(doc)}

            def perform(argv, env, cwd, timeout=105, **_kwargs):
                commands.append(list(argv))
                self.assertEqual(env["CODEX_HOME"], str(case / "home/.codex"))
                self.assertEqual(Path(cwd), case / "project")
                if argv[0] == str(executable):
                    self.assertEqual(timeout, 40)
                    if argv[1:3] == ["plugin", "remove"]:
                        self.assertEqual(argv[3:], ["atlassian@" + marketplace, "--json"])
                        if mutation == "native-remove-failed":
                            return reply(1)
                        config.write_text(sqlite_pin + '[marketplaces.' + marketplace + ']\n'
                                          'source_type = "local"\nsource = '
                                          + json.dumps(str(market)) + '\n')
                    else:
                        self.assertEqual(argv[1:], ["plugin", "marketplace", "remove",
                                                   marketplace, "--json"])
                        config.write_text(sqlite_pin)
                    return reply()
                self.assertEqual(argv[0], "/inert/uap")
                self.assertEqual(argv[argv.index("--target") + 1], target)
                self.assertEqual(argv[argv.index("--scope") + 1], "user")
                if argv[1] == "add":
                    if not state["installed"]:
                        config.parent.mkdir(parents=True, exist_ok=True)
                        market.mkdir()
                        config.write_text(sqlite_pin + '[plugins."atlassian@' + marketplace + '"]\n'
                                          'enabled = true\n[marketplaces.' + marketplace + ']\n'
                                          'source_type = "local"\nsource = '
                                          + json.dumps(str(market)) + '\n')
                        state["installed"] = True
                    if mutation == "failed-add-registered":
                        return reply(1)
                    return reply()
                if argv[1] == "list":
                    state["listed"] += 1
                    current = copy.deepcopy(node)
                    if mutation == "repeat-identity" and state["listed"] == 2:
                        current["installation_id"] = "substituted-binding"
                    return reply(rows=[current] if state["installed"] else [])
                self.assertEqual(argv[1], "remove")
                if "--external-uninstalled" not in argv:
                    return reply(1)
                state["installed"] = False
                if mutation == "native-residue":
                    residue = case / "home/.agents/skills/atlassian/SKILL.md"
                    residue.parent.mkdir(parents=True)
                    residue.write_text("name: atlassian\n")
                if mutation == "foreign-changed":
                    (case / "home/.codex/foreign-neighbor.txt").write_text("changed")
                if mutation == "sqlite-row":
                    import sqlite3
                    from contextlib import closing
                    with closing(sqlite3.connect(case / "home/.local/state/codex-sqlite/state_5.sqlite")) as connection:
                        connection.execute("UPDATE fixture_rows SET value='changed'")
                        connection.commit()
                if mutation == "profile-wal":
                    (case / "home/.codex/state_5.sqlite-wal").write_bytes(b'x' * ((1 << 20) + 1))
                return reply()

            plugin_class = audit.CODEX_RUNTIME_AUDIT["CodexPluginSQLiteAudit"]
            with (patch.dict(plugin_class.__init__.__globals__, {"_exchange": discover}),
                  patch.dict(audit.AGENT_BINARIES, {"codex": str(executable)}, clear=True),
                  patch.object(audit, "call", side_effect=perform)):
                row = audit.do_case("/inert/uap", target, "plugin", "atlassian", 0,
                                    root, {}, fixtures["signed_release_pins"])
            return row, commands

    def test_public_alias_joins_codex_lifecycle(self):
        row, commands = self.run_public_case()
        self.assertEqual(row["status"], "passed")
        self.assertEqual(row["target"], "chatgpt")
        self.assertEqual(row["canonical_client_id"], "codex")
        self.assertEqual(row["matched_installation_ids"], ["owned-codex-binding"])
        self.assertEqual(row["repeat_installation_ids"], ["owned-codex-binding"])
        self.assertTrue(row["native_uninstall"]["completed"])
        self.assertTrue(row["repeat_verified"])
        self.assertTrue(row["removal_verified"])
        self.assertTrue(row["sentinels_preserved"])
        self.assertEqual(row["native_identity_files"], [".codex/config.toml"])
        self.assertEqual(row["native_residuals"], [])
        self.assertEqual(len(row["signed_release_observation"]), 1)
        self.assertEqual(len(commands), 8)
        self.assertEqual([argv[1] for argv in commands],
                         ["add", "list", "add", "list", "plugin", "plugin", "remove", "list"])
        self.assertIn("--external-uninstalled", commands[-2])

    def test_canonical_provenance_effects_and_native_removal_stay_required(self):
        expected = {"wrong-client": "incomplete-native-proof",
                    "wrong-source": "incomplete-native-proof",
                    "failed-add-registered": "failed-install-residue",
                    "repeat-identity": "repeat-add-unqualified",
                    "native-remove-failed": "cleanup-failed",
                    "native-residue": "cleanup-failed",
                    "foreign-changed": "cleanup-failed"}
        for mutation, status in expected.items():
            with self.subTest(mutation=mutation):
                row, commands = self.run_public_case(mutation=mutation)
                self.assertEqual(row["status"], status)
                if mutation == "failed-add-registered":
                    self.assertTrue(row["failed_install_registered"])
                    self.assertTrue(row["failed_install_residue"])
                if mutation == "native-remove-failed":
                    removal = next(argv for argv in commands if argv[1] == "remove")
                    self.assertNotIn("--external-uninstalled", removal)
                if mutation == "native-residue":
                    self.assertTrue(row["native_residuals"])

    def test_plugin_sqlite_forward_measurement_keeps_alias_scope_and_scanner(self):
        for target in ('codex', 'chatgpt'):
            with self.subTest(target=target):
                row, commands = self.run_public_case(target=target)
                self.assertEqual(row['status'], 'passed')
                proof = row['codex_plugin_sqlite_proof']
                self.assertTrue(proof['runtime_semantics_unchanged'])
                self.assertEqual(proof['observations'][0]['phase'], 'before-install')
                self.assertIn('after-native-uninstall', [x['phase'] for x in proof['observations']])
                self.assertTrue(all('--scope' not in argv or argv[argv.index('--scope')+1] == 'user'
                                    for argv in commands))
                row, _ = self.run_public_case(target=target, mutation='profile-wal')
                self.assertEqual(row['status'], 'cleanup-failed')
                self.assertIn('.codex/state_5.sqlite-wal: unscanned-too-large', row['native_residuals'])
                with self.assertRaisesRegex(ValueError, 'table contents changed'):
                    self.run_public_case(target=target, mutation='sqlite-row')
                with self.assertRaisesRegex(ValueError, 'changed the authored config'):
                    self.run_public_case(target=target, mutation='prebaseline-config')
                for mutation in ('prebaseline-foreign', 'prebaseline-managed'):
                    with self.assertRaisesRegex(ValueError, 'changed protected fixture authority'):
                        self.run_public_case(target=target, mutation=mutation)

    def test_main_requires_codex_and_records_its_real_version_call(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            cli, native = root / "uap", root / "codex"
            cli.write_text("inert UAP")
            native.write_text("inert Codex")
            args = ["live-install-audit.py", "--cli", str(cli), "--target", "chatgpt",
                    "--kind", "plugin", "--limit", "1", "--report-dir", str(root)]
            with (patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}),
                  patch("sys.argv", args), patch.dict(audit.AGENT_BINARIES, {}, clear=True),
                  patch.object(audit, "version_evidence") as version,
                  patch.object(audit, "do_case") as case):
                self.assertEqual(audit.main(), 2)
                version.assert_not_called()
                case.assert_not_called()
            probes = []
            def version(argv, *_args):
                probes.append(argv)
                return {"status": "reported", "exit_code": 0,
                        "value": "codex-cli 0.162.0" if argv[0] == str(native) else "UAP test"}
            with (patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}),
                  patch("sys.argv", args),
                  patch.dict(audit.AGENT_BINARIES, {"codex": str(native)}, clear=True),
                  patch.object(audit, "version_evidence", side_effect=version),
                  patch.object(audit, "do_case", return_value={"status": "passed"}) as case):
                self.assertEqual(audit.main(), 0)
                self.assertEqual(case.call_args.args[1:3], ("chatgpt", "plugin"))
            report = json.loads((root / "chatgpt-plugin.json").read_text())
            self.assertEqual(probes, [[str(cli), "version"], [str(native), "--version"]])
            self.assertEqual(report["canonical_client_id"], "codex")
            self.assertEqual(report["client_binary"], str(native))
            self.assertEqual(report["client_version"]["value"], "codex-cli 0.162.0")

    def test_project_skill_stays_unsupported_before_cli_or_native(self):
        audit = load_audit()
        for binaries in ({}, {"codex": "/unexecuted/codex"}):
            with self.subTest(binaries=binaries), TemporaryDirectory() as temporary:
                args = ["live-install-audit.py", "--cli", "/missing/never-resolved-uap",
                        "--target", "chatgpt", "--kind", "skill", "--limit", "30",
                        "--report-dir", temporary]
                with (patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}),
                      patch("sys.argv", args),
                      patch.dict(audit.AGENT_BINARIES, binaries, clear=True),
                      patch.object(audit, "version_evidence") as version,
                      patch.object(audit, "do_case") as case):
                    self.assertEqual(audit.main(), 2)
                    version.assert_not_called()
                    case.assert_not_called()
                report = json.loads((Path(temporary) / "chatgpt-skill.json").read_text())
                self.assertEqual(report["canonical_client_id"], "chatgpt")
                self.assertEqual(report["cases_attempted"], 0)
                self.assertEqual(len(report["results"]), 30)
                self.assertTrue(all(row["status"] == "documented-unsupported"
                                    and "install" not in row for row in report["results"]))


class PreparedInstallationRecoveryContracts(unittest.TestCase):
    def report(self, target, kind):
        # New inert consumer fixtures after workspace recovery, not recovered
        # hosted originals. All six outcomes remain required for each source.
        audit = load_audit()
        fixtures = json.loads(audit.FIXTURES.read_text())
        sources = fixtures["plugin_aliases"] if kind == "plugin" else fixtures["skills"]
        rows = []
        for index, item in enumerate(sources):
            source = item if kind == "plugin" else item["source"]
            identity = fixtures["plugin_declared_names"].get(item, item) if kind == "plugin" else item["name"]
            row = {"index": index, "target": target, "kind": kind, "source": source,
                   "identity": identity, "status": "prepared-lifecycle-passed",
                   "matched_installation_ids": [f"original-binding-{index}"],
                   "repeat_installation_ids": [f"original-binding-{index}"],
                   "native_residuals": [], "signed_release_observation": []}
            row.update({key: True for key in ("prepared_binding", "listed_identity", "repeat_verified",
                        "repeat_native_unchanged", "repeat_lock_unchanged", "removal_verified",
                        "post_list_schema_valid", "sentinels_preserved")})
            row.update({key: False for key in ("failed_install_registered", "failed_install_residue",
                                              "post_remove_identity_registered")})
            row.update({key: {"exit_code": 0, "timeout": False} for key in
                        ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list")})
            if kind == "skill":
                row.update({key: True for key in ("lock_written", "lock_contains_identity",
                            "lock_exact_source_pinned", "skill_bytes_match", "repeat_project_unchanged")})
                row.update(skill_path=f'.grok/plugins/skill-{"a" * 45}-{"b" * 12}/skills/{identity}/SKILL.md',
                           post_removal_skill=None, post_removal_leftovers=[], post_skill_tree_delta=[],
                           post_lock_pinned=False)
            elif not source.startswith("github:"):
                pin = fixtures["signed_release_pins"][source]
                row["signed_release_observation"] = [{
                    "source": pin["repository"].strip("/") + "/" + pin["path"].strip("/"),
                    "recorded_distribution": pin["distribution"], "recorded_revision": pin["revision"],
                    "recorded_release_sequence": pin["sequence"], "resolved_revision": pin["revision"],
                    "release_sequence": pin["sequence"], "tree_digest": pin["tree_digest"],
                    "manifest_digest": pin["manifest_digest"]}]
            rows.append(row)
        return {"target": target, "kind": kind, "platform": "linux", "fixture_count": 30,
                "cases_attempted": 30, "source_selection_complete": True, "full_coverage": True,
                "client_version": {"status": "reported", "value": "inert fixture version", "exit_code": 0},
                "results": rows}, fixtures

    def invoke(self, report, acceptance=None, *, target=None, kind=None, platform="linux", arch="x86_64", limit=30):
        import copy
        from contextlib import redirect_stdout, redirect_stderr
        from io import StringIO
        audit = load_audit()
        target, kind = target or report["target"], kind or report["kind"]
        with TemporaryDirectory() as temporary:
            argv = ["live-install-audit", "--cli", sys.executable, "--target", target,
                    "--kind", kind, "--limit", str(limit), "--report-dir", temporary]
            if acceptance:
                argv += ["--acceptance", acceptance]
            with (patch.object(audit.sys, "argv", argv), patch.object(audit.sys, "platform", platform),
                  patch.object(audit.platform, "machine", return_value=arch),
                  patch.object(audit, "vendor_bootstrap_attested", return_value=True),
                  patch.dict(audit.AGENT_BINARIES, {"cursor": sys.executable, "grok": sys.executable}, clear=True),
                  patch.object(audit, "version_evidence", return_value=report["client_version"]) as version,
                  patch.object(audit, "do_case", side_effect=copy.deepcopy(report["results"])) as cases,
                  redirect_stdout(StringIO()), redirect_stderr(StringIO())):
                try:
                    code = audit.main()
                except SystemExit as error:
                    code = error.code
            path = Path(temporary) / f"{target}-{kind}.json"
            return code, json.loads(path.read_text()) if path.exists() else None, version.call_count, cases.call_count

    def test_reduced_legacy_fixtures_cannot_qualify_strict_installation_lanes(self):
        for target, kind, mode in (("cursor", "plugin", "cursor-prepared-installation"),
                                   ("grok", "skill", "grok-prepared-skills-installation")):
            with self.subTest(mode=mode):
                original, _ = self.report(target, kind)
                code, default, _, calls = self.invoke(original)
                self.assertEqual((code, calls, default["passed"]), (1, 30, 0))
                self.assertEqual(default["prepared_lifecycle_passed"], 30)
                self.assertEqual(default["verified_negative_coverage"], 0)
                self.assertNotIn("installation_acceptance_passed", default)
                code, prepared, versions, calls = self.invoke(original, mode)
                self.assertEqual((code, versions, calls), (1, 2, 30))
                self.assertEqual(prepared["results"], original["results"])
                self.assertEqual(prepared["passed"], 0)
                self.assertEqual(prepared["verified_negative_coverage"], 0)
                self.assertEqual(prepared["installation_passed"], 0)
                self.assertIs(prepared["installation_acceptance_passed"], False)
                self.assertIs(prepared["native_activation_qualified"], False)

    def test_all_commands_provenance_effects_and_source_slots_stay_required(self):
        import copy
        audit = load_audit()
        for target, kind, mode in (("cursor", "plugin", "cursor-prepared-installation"),
                                   ("grok", "skill", "grok-prepared-skills-installation")):
            original, fixtures = self.report(target, kind)
            self.assertTrue(audit.prepared_installation_summary(original, fixtures, mode)["installation_acceptance_passed"])
            for index in range(30):
                for label in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
                    for key, value in (("exit_code", 1), ("exit_code", False), ("timeout", True)):
                        changed = copy.deepcopy(original)
                        changed["results"][index][label][key] = value
                        self.assertFalse(audit.prepared_installation_summary(changed, fixtures, mode)["installation_acceptance_passed"])
            mutations = [("index", False), ("source", "foreign"), ("identity", "foreign"),
                         ("target", "other"), ("status", "passed"), ("status", "documented-or-source-unsupported"),
                         ("prepared_binding", False), ("repeat_native_unchanged", False),
                         ("repeat_lock_unchanged", False), ("repeat_installation_ids", ["replacement"]),
                         ("matched_installation_ids", []), ("sentinels_preserved", False),
                         ("failed_install_residue", True), ("native_residuals", ["retained"]),
                         ("post_remove_identity_registered", True)]
            if kind == "skill":
                mutations += [("lock_exact_source_pinned", False), ("skill_bytes_match", False),
                              ("skill_path", ".grok/plugins/foreign/SKILL.md"),
                              ("repeat_project_unchanged", False), ("post_skill_tree_delta", ["foreign"]),
                              ("post_removal_leftovers", ["owned"]), ("post_lock_pinned", True)]
            else:
                mutations += [("signed_release_observation", [])]
            for field, value in mutations:
                changed = copy.deepcopy(original);changed["results"][0][field] = value
                self.assertFalse(audit.prepared_installation_summary(changed, fixtures, mode)["installation_acceptance_passed"], field)
            if kind == "skill":
                changed = copy.deepcopy(original);changed["results"][0].pop("post_removal_skill")
                self.assertFalse(audit.prepared_installation_summary(changed, fixtures, mode)["installation_acceptance_passed"])
            changed = copy.deepcopy(original);changed["results"].pop()
            self.assertFalse(audit.prepared_installation_summary(changed, fixtures, mode)["installation_acceptance_passed"])

    def test_wrong_lane_and_partial_selection_refuse_before_effects(self):
        for target, kind, mode in (("cursor", "plugin", "cursor-prepared-installation"),
                                   ("grok", "skill", "grok-prepared-skills-installation")):
            original, _ = self.report(target, kind)
            for changed in ({"target": "vscode"}, {"target": target + ",grok"},
                            {"kind": "skill" if kind == "plugin" else "plugin"},
                            {"platform": "darwin"}, {"platform": "win32"}, {"arch": "arm64"}, {"limit": 29}):
                with self.subTest(mode=mode, changed=changed):
                    self.assertEqual(self.invoke(original, mode, **changed), (2, None, 0, 0))



class PreparedProjectSkillEvidenceContracts(unittest.TestCase):
    """Real-source-shaped, strictly Prepared-only Cursor/Kiro Project Skills."""

    def fixture(self, target):
        audit = load_audit()
        doc = json.loads(audit.FIXTURES.read_text())
        host, arch, version, authentication = {
            "cursor": ("linux", "x86_64", "2026.10.22-27dcf5f", "not_required"),
            "kiro": ("darwin", "arm64", "kiro-cli 3.0.0", "not_checked"),
        }[target]
        rows = []
        expected = {"materialization": "materialized", "activation": "prepared",
                    "authentication": authentication, "policy": "allowed",
                    "verification": "installation_verified"}
        for index, item in enumerate(doc["skills"]):
            row = {"target": target, "kind": "skill", "index": index,
                   "source": item["source"], "identity": item["name"],
                   "skill_path": f'.{target}/skills/{item["name"]}/SKILL.md',
                   "status": "incomplete-native-proof", "native_file_delta": 0,
                   "native_identity_files": [], "native_component_candidates": [],
                   "native_residuals": [], "signed_release_observation": [],
                   "prepared_binding": False, "prepared_components_only": False,
                   "repeat_native_runtime_only": False, "repeat_project_unchanged": None,
                   "public_skill_ready": {"after-add": False, "repeat-add": False},
                   "matched_installation_ids": [f"original-binding-{index}"],
                   "repeat_installation_ids": [f"original-binding-{index}"],
                   "post_removal_leftovers": [], "post_skill_tree_delta": [],
                   "post_removal_skill": None, "post_lock_pinned": False}
            row.update({key: True for key in (
                "lock_written", "lock_contains_identity", "lock_exact_source_pinned",
                "skill_bytes_match", "listed_identity", "repeat_verified",
                "repeat_native_unchanged", "repeat_lock_unchanged",
                "removal_verified", "post_list_schema_valid", "sentinels_preserved")})
            row.update({key: False for key in (
                "failed_install_registered", "failed_install_residue",
                "post_remove_identity_registered")})
            row.update({key: {"exit_code": 0, "timeout": False} for key in (
                "install", "list", "repeat_add", "repeat_list",
                "cleanup", "post_remove_list")})
            row["public_target_states"] = {
                phase: {"status": "observed", "selected_installations": int(active),
                        "selected_bindings": int(active), "invalid_rows": 0,
                        "states": [expected] if active else [], "truncated": False}
                for phase, active in (("after-add", True), ("repeat-add", True),
                                      ("after-remove", False))}
            rows.append(row)
        cli_version = {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0}
        client_version = {"status": "reported", "value": version, "exit_code": 0}
        report = {"target": target, "kind": "skill", "platform": host, "architecture": arch,
                  "canonical_client_id": target, "client_version": client_version,
                  "cli_version": cli_version, "fixture_count": 30, "cases_attempted": 30,
                  "source_selection_complete": True, "full_coverage": True,
                  "passed": 0, "verified_negative_coverage": 0, "results": rows}
        return audit, doc, report

    def test_requires_exact_workflow_source_cli_version(self):
        import copy
        for target in ("cursor", "kiro"):
            audit, fixtures, report = self.fixture(target)
            mode = target + "-prepared-skills-installation"
            with self.subTest(target=target, version=report["cli_version"]):
                self.assertTrue(audit.prepared_project_skill_summary(
                    report, fixtures, mode)["installation_acceptance_passed"])
            for invalid in (
                    {"value": "agentplugins 0.1.95"},
                    {"value": "agentplugins 0.1.114"},
                    {"value": "agentplugins 0.1.116"},
                    {"value": "agentplugins 0.1.115-dev"},
                    {"value": "agentplugins 0.1.115+build"},
                    {"value": "agentplugins 0.1.1151"},
                    {"value": "other-launcher 0.1.115"},
                    {"value": "agentplugins 0.1.115\nagentplugins 0.1.115"},
                    {"value": ""}, {"status": "not-probed"}, {"exit_code": 1}):
                with self.subTest(target=target, invalid=invalid):
                    changed = copy.deepcopy(report)
                    changed["cli_version"].update(invalid)
                    self.assertEqual(audit.prepared_project_skill_summary(
                        changed, fixtures, mode), {
                            "installation_passed": 0,
                            "installation_prepared_lifecycle_passed": 0,
                            "installation_verified_negative_coverage": 0,
                            "installation_acceptance_passed": False})

    def test_source_bound_prepared_only_and_strict_rejection(self):
        import copy
        for target in ("cursor", "kiro"):
            with self.subTest(target=target):
                audit, fixtures, report = self.fixture(target)
                mode = target + "-prepared-skills-installation"
                proof = audit.prepared_project_skill_summary(report, fixtures, mode)
                self.assertEqual(proof, {
                    "installation_passed": 30, "installation_prepared_lifecycle_passed": 30,
                    "installation_verified_negative_coverage": 0,
                    "installation_acceptance_passed": True})
                mutations = (
                    ("status", "passed"), ("source", "foreign"), ("identity", "foreign"),
                    ("index", False), ("skill_path", ".other/skills/foreign/SKILL.md"),
                    ("lock_exact_source_pinned", False), ("skill_bytes_match", False),
                    ("native_residuals", ["foreign"]), ("post_skill_tree_delta", ["foreign"]),
                    ("post_removal_leftovers", ["owned"]), ("post_removal_skill", "owned"),
                    ("post_lock_pinned", True), ("failed_install_registered", True),
                    ("failed_install_residue", True), ("sentinels_preserved", False),
                    ("repeat_verified", False), ("removal_verified", False),
                    ("repeat_native_unchanged", False), ("repeat_native_runtime_only", True),
                    ("public_skill_ready", {"after-add": True, "repeat-add": False}),
                    ("matched_installation_ids", []),
                    ("repeat_installation_ids", ["other"]))
                for field, value in mutations:
                    changed = copy.deepcopy(report)
                    changed["results"][12][field] = value
                    self.assertFalse(audit.prepared_project_skill_summary(
                        changed, fixtures, mode)["installation_acceptance_passed"], field)
                for phase in ("install", "list", "repeat_add", "repeat_list",
                              "cleanup", "post_remove_list"):
                    changed = copy.deepcopy(report)
                    changed["results"][0][phase]["timeout"] = True
                    self.assertFalse(audit.prepared_project_skill_summary(
                        changed, fixtures, mode)["installation_acceptance_passed"], phase)
                for phase in ("after-add", "repeat-add", "after-remove"):
                    changed = copy.deepcopy(report)
                    changed["results"][0]["public_target_states"][phase]["selected_bindings"] = 99
                    self.assertFalse(audit.prepared_project_skill_summary(
                        changed, fixtures, mode)["installation_acceptance_passed"], phase)
                for field, value in (
                        ("platform", "win32"), ("architecture", "foreign"),
                        ("client_version", {"status": "not-probed"}),
                        ("source_selection_complete", False), ("passed", 30)):
                    changed = copy.deepcopy(report)
                    changed[field] = value
                    self.assertFalse(audit.prepared_project_skill_summary(
                        changed, fixtures, mode)["installation_acceptance_passed"], field)
                self.assertFalse(audit.prepared_project_skill_summary(
                    report, fixtures, "native")["installation_acceptance_passed"])

    def test_cli_entry_keeps_native_negative_and_prepared_receipt_distinct(self):
        import copy
        from contextlib import redirect_stdout, redirect_stderr
        from io import StringIO
        for target in ("cursor", "kiro"):
            with self.subTest(target=target):
                audit, _fixtures, report = self.fixture(target)
                # Cursor now requires the strict full-envelope receipt; the reduced
                # legacy fixture must fail its CLI route. Kiro retains its legacy route.
                for acceptance, expected_code in (("native", 1),
                                                  (target + "-prepared-skills-installation",
                                                   1 if target == "cursor" else 0)):
                    with TemporaryDirectory() as directory:
                        argv = ["live-install-audit", "--cli", sys.executable,
                                "--target", target, "--kind", "skill",
                                "--limit", "30", "--report-dir", directory,
                                "--acceptance", acceptance]
                        binary = "cursor-agent" if target == "cursor" else "kiro-cli"
                        with (patch.object(audit.sys, "argv", argv),
                              patch.object(audit.sys, "platform", report["platform"]),
                              patch.object(audit.platform, "machine", return_value=report["architecture"]),
                              patch.object(audit, "vendor_bootstrap_attested", return_value=True),
                              patch.dict(audit.AGENT_BINARIES, {binary: sys.executable}, clear=True),
                              patch.object(audit, "version_evidence",
                                           side_effect=[report["cli_version"], report["client_version"]]),
                              patch.object(audit, "do_case", side_effect=copy.deepcopy(report["results"])) as runs,
                              redirect_stdout(StringIO()), redirect_stderr(StringIO())):
                            code = audit.main()
                        self.assertEqual(code, expected_code)
                        self.assertEqual(runs.call_count, 30)
                        saved = json.loads((Path(directory) / f"{target}-skill.json").read_text())
                        self.assertEqual(saved["passed"], 0)
                        self.assertEqual(saved["results"], report["results"])
                        if acceptance == "native":
                            self.assertNotIn("installation_acceptance_passed", saved)
                        elif target == "cursor":
                            self.assertEqual(saved["installation_passed"], 0)
                            self.assertIs(saved["installation_acceptance_passed"], False)
                            self.assertIs(saved["native_activation_qualified"], False)
                        else:
                            self.assertEqual(saved["installation_passed"], 30)
                            self.assertIs(saved["installation_acceptance_passed"], True)


class WindsurfRecoveredFixtureContracts(unittest.TestCase):
    def run_case(self, negative=False, mutation=None):
        import copy
        audit = load_audit(); fixtures = json.loads(audit.FIXTURES.read_text())
        item, index = ("marketing-kit", 13) if negative else ("atlassian", 0)
        pin = fixtures["signed_release_pins"][item]
        node = {"installation_id": "original-windsurf", "name": item,
                "source": pin["repository"] + "/" + pin["path"],
                "directory": {"product_id": item, "recorded_distribution": pin["distribution"],
                              "recorded_revision": pin["revision"], "recorded_release_sequence": pin["sequence"]},
                "clients": [{"client_id": "windsurf", "verification": "installation_verified",
                             "package_revision": {"distribution_id": pin["distribution"],
                             "resolved_revision": pin["revision"], "release_sequence": pin["sequence"],
                             "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"]}}]}
        state = {"installed": False}; commands = []
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            def invoke(argv, env, cwd, timeout=105, **_kwargs):
                home = Path(env["HOME"]); native = Path(env["XDG_CONFIG_HOME"]) / "devin"
                self.assertTrue((home / ".codeium/windsurf").is_dir())
                self.assertTrue((native / "foreign-neighbor.txt").is_file())
                config = json.loads((native / "config.json").read_text())
                if not commands:
                    self.assertIs(config["read_config_from"]["cloud"], False)
                commands.append(list(argv))
                code, stderr = 0, ""
                if argv[1] == "add":
                    if negative:
                        code = 1
                        stderr = ('agentplugins: no eligible directory release for "marketing-kit": '
                                  '777genius/marketing-kit: release does not support complete target set; missing windsurf\n')
                        if mutation == "native-image":
                            (home / ".devin").mkdir(exist_ok=True)
                            (home / ".devin/unrelated-added").write_text("unexpected")
                    else:
                        state["installed"] = True
                        (native / "owned-atlassian.json").write_text('{"name":"atlassian"}')
                        # The existing ordinary metadata contract permits this
                        # first native config migration, with cloud still false.
                        (native / "config.json").write_text('{"read_config_from":{"cloud":false},"version":1}\n')
                elif argv[1] == "remove":
                    state["installed"] = False;(native / "owned-atlassian.json").unlink()
                    if mutation == "sentinel":
                        (home / ".codeium/windsurf/foreign-neighbor.txt").write_text("changed")
                    if mutation == "policy":
                        (native / "config.json").write_text('{"read_config_from":{"cloud":true},"version":1}\n')
                elif argv[1] != "list":
                    self.fail(argv)
                doc = {"schema_version": 1, "command": "list", "result": "success",
                       "data": {"installations": [copy.deepcopy(node)] if state["installed"] else []}}
                return {"exit_code": code, "timeout": False,
                        "stdout_tail": "" if code else json.dumps(doc, separators=(",", ":")) + "\n",
                        "stderr_tail": stderr, "_json": doc if code == 0 else None}
            with patch.object(audit, "call", side_effect=invoke):
                result = audit.do_case("/inert/agentplugins", "windsurf", "plugin", item, index,
                                       root, fixtures["plugin_declared_names"], fixtures["signed_release_pins"])
        return audit, result, fixtures, commands

    def test_actual_case_authors_selected_profiles_before_public_calls(self):
        audit, result, _, commands = self.run_case()
        self.assertEqual(result["status"], "passed")
        self.assertFalse(result["prepared_binding"])
        self.assertTrue(result["repeat_verified"])
        self.assertTrue(result["removal_verified"])
        self.assertTrue(result["sentinels_preserved"])
        self.assertEqual([command[1] for command in commands], ["add", "list", "add", "list", "remove", "list"])
        roots = audit.NATIVE_ROOTS["windsurf"]
        self.assertEqual(roots[:2], [".windsurf", ".devin"])
        self.assertEqual(len({Path(root).name for root in roots}), len(roots))
        self.assertIn("devin/owned-atlassian.json", result["native_identity_files"])
        for mutation in ("sentinel", "policy"):
            _, changed, _, _ = self.run_case(mutation=mutation)
            self.assertNotEqual(changed["status"], "passed")
            self.assertFalse(changed["sentinels_preserved"])

    def test_only_exact_fixed_negative_with_unchanged_images_is_coverage(self):
        import copy
        audit, original, fixtures, commands = self.run_case(negative=True)
        self.assertEqual(original["status"], "documented-or-source-unsupported")
        self.assertEqual(len(commands), 2)
        self.assertTrue(audit.windsurf_source_negative(original, fixtures))
        mutations = [("index", 12), ("index", False), ("target", "cursor"), ("kind", "skill"),
                     ("source", "other"), ("identity", "other"), ("status", "passed"),
                     ("windsurf_native_image_unchanged", False), ("native_file_delta", 1),
                     ("native_file_delta", False), ("native_identity_files", ["owned"]),
                     ("sentinels_preserved", False), ("failed_install_registered", True),
                     ("failed_install_residue", True), ("post_remove_identity_registered", True),
                     ("windsurf_expected_release_pin", {}), ("windsurf_public_commands", []),
                     ("cleanup", {"exit_code": 0})]
        for key, value in mutations:
            changed = copy.deepcopy(original);changed[key] = value
            self.assertFalse(audit.windsurf_source_negative(changed, fixtures), key)
        for key in ("repeat_add", "repeat_list", "cleanup", "post_remove_list"):
            changed = copy.deepcopy(original);changed.pop(key)
            self.assertFalse(audit.windsurf_source_negative(changed, fixtures), key)
        for command, key, value in (("install", "exit_code", 0), ("install", "timeout", True),
                                    ("install", "stderr_tail", "unsupported"),
                                    ("list", "exit_code", 1), ("list", "stdout_tail", "")):
            changed = copy.deepcopy(original);changed[command][key] = value
            self.assertFalse(audit.windsurf_source_negative(changed, fixtures))
        changed = copy.deepcopy(fixtures);changed["signed_release_pins"]["marketing-kit"]["revision"] = "0" * 40
        self.assertFalse(audit.windsurf_source_negative(original, changed))
        _, changed, _, _ = self.run_case(negative=True, mutation="native-image")
        self.assertFalse(audit.windsurf_source_negative(changed, fixtures))

    def test_fixture_refuses_redirected_parent_without_foreign_effects(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); home = root / "case/home"; foreign = root / "foreign"
            home.mkdir(parents=True);foreign.mkdir();(home / ".config").mkdir()
            (home / ".codeium").symlink_to(foreign, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "redirected"):
                audit.windsurf_plugin_profile(home, root / "case")
            self.assertEqual(list(foreign.iterdir()), [])


class VSCodeUserManualObservationContracts(unittest.TestCase):
    """Observe ordinary User preparation/refusal without attesting uninstall."""

    # An existing inert executable path satisfies main() path admission; every call is mocked.
    CLI = str(Path(sys.executable).resolve())
    REMOVE_STDERR = (
        "agentplugins: removal requires a manual client step for [vscode]; "
        "complete the displayed uninstall step, then rerun with "
        "--external-uninstalled; no target was changed\n"
    )
    REQUIREMENTS_STDERR = (
        "Resolving and validating one Agent Plugin package for every selected target...\n"
        "agentplugins: package marketing-kit declares UAP requirements; use a managed "
        "saved add/install workflow instead of --no-save or local/manual installation\n"
    )

    @staticmethod
    def _fixtures(audit):
        return json.loads(audit.FIXTURES.read_text())

    @staticmethod
    def _binding(fixtures, source, index):
        identity = fixtures["plugin_declared_names"].get(source, source)
        client = {
            "client_id": "vscode", "scope": "user",
            "materialization": "materialized",
            "activation": "manual_activation_required",
            "verification": "package_validated",
            "authentication": "not_checked", "policy": "allowed",
            "affected_surfaces": ["copilot", "vscode"],
        }
        result = {
            "installation_id": f"00000000-0000-4000-8000-{index + 1:012x}",
            "name": identity, "version": "0.1.0", "source": source,
            "clients": [client],
        }
        pin = fixtures["signed_release_pins"].get(source)
        if pin is not None:
            result["source"] = pin["repository"].strip("/") + "/" + pin["path"].strip("/")
            result["directory"] = {
                "product_id": "github" if source == "777genius/github-bridge" else source,
                "recorded_distribution": pin["distribution"],
                "recorded_revision": pin["revision"],
                "recorded_release_sequence": pin["sequence"],
            }
            client["package_revision"] = {
                "distribution_id": pin["distribution"],
                "resolved_revision": pin["revision"],
                "release_sequence": pin["sequence"],
                "tree_digest": pin["tree_digest"],
                "manifest_digest": pin["manifest_digest"],
            }
        else:
            client["package_revision"] = {"resolved_revision": source.rsplit("@", 1)[1]}
        return result

    @staticmethod
    def _list_document(binding):
        import copy

        return {"schema_version": 1, "command": "list", "result": "success",
                "data": {"installations": [] if binding is None else [copy.deepcopy(binding)]}}

    @classmethod
    def _remove_document(cls, binding):
        # The wire fields are pinned to domain/clients_runtime.go f63a7407
        # and usecase/remove.go 4760ddb1; no native effects are serialized.
        operation_id = "11111111-1111-4111-8111-" + binding["installation_id"].rsplit("-", 1)[1]
        return {
            "schema_version": 1, "command": "remove", "result": "failure",
            "data": {
                "status": "manual_step_required", "batch": True, "dry_run": False,
                "operation_id": operation_id, "plugin_data_preserved": True, "data_retained": False,
                "succeeded": 0, "failed": 1, "plugin": binding["name"],
                "targets": [{
                    "target": "vscode", "status": "blocked",
                    "output": {"dry_run": True, "plugin": binding["name"],
                               "operation_id": operation_id, "source": binding["source"],
                               "version": binding["version"], "result": {
                        "installation_id": binding["installation_id"],
                        "client_id": "vscode", "plugin": binding["name"],
                        "requires_confirmation": False, "mutated": False,
                        "deactivation": {
                            "activation": "manual_activation_required",
                            "artifact_removal_allowed": False,
                            "external_removal_complete": False,
                        },
                    }},
                }],
            },
        }

    @staticmethod
    def _command(document=None, *, exit_code=0, stderr=""):
        import copy

        stdout = "" if document is None else json.dumps(document, separators=(",", ":")) + "\n"
        record = {"exit_code": exit_code, "timeout": False,
                  "stdout_tail": stdout[-1800:], "stderr_tail": stderr}
        if document is not None:
            record["_json"] = copy.deepcopy(document)
        return record

    def _run_case(self, audit, base, source, index, *, attack=None, requirements=False,
                  authentication="not_checked", protect_data=False):
        import copy

        fixtures = self._fixtures(audit)
        binding = self._binding(fixtures, source, index)
        binding["clients"][0]["authentication"] = authentication
        base.mkdir(parents=True, exist_ok=True)
        runroot = base / "run"
        calls = []
        initial_bytes = []
        add_doc = {
            "schema_version": 1, "command": "add", "result": "success",
            "data": {"actions": [{"component_support": [
                {"name": "fixture-skill", "support": "prepared"}]}]},
        }

        def fake_call(argv, env, cwd, timeout=105, **options):
            command = list(argv)
            calls.append(command)
            stage = len(calls)
            project = Path(cwd)
            case = project.parent
            home, data = case / "home", case / "uap-data"
            receipt = data / "manual-installation.json"
            self.assertEqual(env["HOME"], str(home))
            self.assertEqual(env["AGENTPLUGINS_HOME"], str(data))
            self.assertNotIn("--external-uninstalled", command)
            self.assertEqual(command[0], self.CLI)
            if stage == 1:
                if protect_data:
                    data.chmod(0o700)
                self.assertEqual(command, [
                    self.CLI, "add", source, "--target", "vscode", "--scope",
                    "user", "--format", "json", "--no-save"])
                if requirements:
                    cache = data / "acquisition-cache" / "package.json"
                    cache.parent.mkdir()
                    cache.write_text('{"acquired":true}\n')
                    native_foreign = home / audit.NATIVE_ROOTS["vscode"][0] / "foreign-neighbor.txt"
                    if attack == "native-new":
                        (native_foreign.parent / "unexpected-native").write_text("changed\n")
                    elif attack == "native-replace":
                        held = base / "held-native-original"
                        native_foreign.replace(held)
                        native_foreign.write_bytes(held.read_bytes())
                    elif attack == "native-delete":
                        native_foreign.unlink()
                    elif attack == "data-root-replace":
                        mode = data.stat().st_mode & 0o777
                        data.replace(base / "held-data-original")
                        data.mkdir(mode=mode)
                    elif attack == "foreign-sentinel":
                        (home / "foreign-untouched.txt").write_text("changed\n")
                    return self._command(exit_code=1, stderr=self.REQUIREMENTS_STDERR)
                receipt.write_text(json.dumps(binding, sort_keys=True) + "\n")
                initial_bytes.append(receipt.read_bytes())
                return self._command(add_doc)
            if requirements:
                self.assertEqual(stage, 2)
                self.assertEqual(command, [
                    self.CLI, "list", "--target", "vscode", "--scope", "user", "--format", "json"])
                return self._command(self._list_document(None))
            if stage in (2, 4, 6):
                self.assertEqual(command, [
                    self.CLI, "list", "--target", "vscode", "--scope", "user", "--format", "json"])
                selected = copy.deepcopy(binding)
                if stage == 4 and attack == "repeat-id":
                    selected["installation_id"] = "ffffffff-ffff-4fff-8fff-ffffffffffff"
                if stage == 6:
                    if attack == "post-list-write":
                        (data / "unexpected-after-list").write_text("changed\n")
                    if attack == "post-list-id":
                        selected["installation_id"] = "ffffffff-ffff-4fff-8fff-ffffffffffff"
                    if attack == "post-list-binding":
                        selected["clients"][0]["authentication"] = "auth_pending"
                return self._command(self._list_document(selected))
            if stage == 3:
                self.assertEqual(command, calls[0])
                self.assertEqual(receipt.read_bytes(), initial_bytes[0])
                return self._command(add_doc)
            self.assertEqual(stage, 5)
            self.assertEqual(command, [
                self.CLI, "remove", binding["name"], "--target", "vscode",
                "--scope", "user", "--format", "json"])
            document = self._remove_document(binding)
            result = document["data"]["targets"][0]["output"]["result"]
            stderr, code = self.REMOVE_STDERR, 1
            if attack == "replace-on-remove":
                held = base / "held-receipt-original"
                receipt.replace(held)
                receipt.write_bytes(held.read_bytes())
                self.assertEqual(receipt.read_bytes(), initial_bytes[0])
                self.assertNotEqual(receipt.stat().st_ino, held.stat().st_ino)
            elif attack == "new-member-on-remove":
                (data / "unexpected-log").write_text("changed\n")
            elif attack == "delete-on-remove":
                receipt.unlink()
            elif attack == "case-root-replace":
                held = base / "held-case-original"
                mode = case.stat().st_mode & 0o777
                case.replace(held)
                case.mkdir(mode=mode)
                for child in tuple(held.iterdir()):
                    child.replace(case / child.name)
            elif attack == "refusal-id":
                result["installation_id"] = "ffffffff-ffff-4fff-8fff-ffffffffffff"
            elif attack == "refusal-mutated":
                result["mutated"] = True
            elif attack == "refusal-exit-type":
                code = True
            elif attack == "refusal-stderr":
                stderr = stderr.replace("no target was changed", "different refusal")
            return self._command(document, exit_code=code, stderr=stderr)

        with (patch.object(audit.sys, "platform", "linux"),
              patch.object(audit, "call", side_effect=fake_call)):
            row = audit.do_case(self.CLI, "vscode", "plugin", source, index, runroot,
                                fixtures["plugin_declared_names"], fixtures["signed_release_pins"])
        return row, calls, binding, runroot / f"plugin-{index:02d}"

    def test_manual_observation_keeps_declared_auth_and_private_data_baseline(self):
        import copy

        audit = load_audit()
        fixtures = self._fixtures(audit)
        source = fixtures["plugin_aliases"][0]
        variants = (("not_checked", True), ("auth_pending", False),
                    ("not_required", False), ("auth_pending", True))
        with TemporaryDirectory() as temporary:
            for index, (authentication, protect_data) in enumerate(variants):
                with self.subTest(authentication=authentication, protect_data=protect_data):
                    row, calls, binding, case = self._run_case(
                        audit, Path(temporary).resolve() / str(index), source, 0,
                        authentication=authentication, protect_data=protect_data)
                    self.assertEqual(len(calls), 6)
                    self.assertIs(audit.vscode_user_manual_positive(row, fixtures, self.CLI), True)
                    self.assertEqual((case / "uap-data").stat().st_mode & 0o777, 0o700)
                    for phase in ("original_binding", "repeat_binding", "post_refusal_binding"):
                        self.assertEqual(row["vscode_user_manual"][phase], binding)
                    self.assertIs(row["removal_verified"], False)
                    self.assertIsNone(row["native_uninstall"])
                    self.assertTrue(all("--external-uninstalled" not in command for command in calls))
        for authentication in ("authenticated", "unknown", "", None, True, {}):
            with self.subTest(rejected_authentication=authentication):
                changed = copy.deepcopy(binding)
                changed["clients"][0]["authentication"] = authentication
                self.assertIsNone(audit.vscode_manual_binding(
                    self._list_document(changed), row["identity"], source,
                    fixtures["signed_release_pins"]))

    def test_real_do_case_retains_manual_binding_after_guarded_remove_refusal(self):
        audit = load_audit()
        fixtures = self._fixtures(audit)
        source = fixtures["plugin_aliases"][0]
        with TemporaryDirectory() as temporary:
            row, calls, binding, case = self._run_case(
                audit, Path(temporary).resolve(), source, 0)
            # This assertion executes the old do_case too: it stops at five
            # commands and therefore lacks the required post-refusal observation.
            self.assertEqual(len(calls), 6)
            self.assertEqual(calls[1], calls[3])
            self.assertEqual(calls[1], calls[5])
            self.assertEqual(row["status"], "external-native-uninstall-required")
            self.assertIs(row["prepared_binding"], True)
            self.assertIs(row["repeat_verified"], True)
            self.assertIs(row["removal_verified"], False)
            self.assertIsNone(row["native_uninstall"])
            self.assertIsNone(row["post_remove_list"])
            self.assertIs(row["sentinels_preserved"], True)
            self.assertEqual(row["native_file_delta"], 0)
            self.assertEqual(row["native_identity_files"], [])
            self.assertEqual(row["native_residuals"], [])
            proof = row["vscode_user_manual"]
            for key in ("original_binding", "repeat_binding", "post_refusal_binding"):
                self.assertEqual(proof[key], binding)
            self.assertEqual(proof["remove_argv"], calls[4])
            self.assertEqual(proof["list_argv"], calls[5])
            self.assertIs(proof["refusal_observed"], True)
            self.assertIs(proof["protected_after_refusal"], True)
            self.assertIs(proof["protected_after_list"], True)
            digests = [proof[key] for key in (
                "protected_before_sha256", "protected_after_refusal_sha256",
                "protected_after_list_sha256")]
            self.assertEqual(digests, [digests[0]] * 3)
            self.assertRegex(digests[0], r"^[0-9a-f]{64}$")
            self.assertEqual(json.loads((case / "uap-data/manual-installation.json").read_text()),
                             binding)
            self.assertIs(audit.vscode_user_manual_positive(row, fixtures, self.CLI), True)

    def test_refusal_observation_rejects_protected_state_and_identity_changes(self):
        audit = load_audit()
        fixtures = self._fixtures(audit)
        source = fixtures["plugin_aliases"][0]
        attacks = (
            "replace-on-remove", "new-member-on-remove", "delete-on-remove", "case-root-replace",
            "post-list-write", "post-list-id", "post-list-binding", "repeat-id",
            "refusal-id", "refusal-mutated", "refusal-exit-type", "refusal-stderr",
        )
        with TemporaryDirectory() as temporary:
            for attack in attacks:
                with self.subTest(attack=attack):
                    row, calls, _, _ = self._run_case(
                        audit, Path(temporary).resolve() / attack, source, 0, attack=attack)
                    self.assertIs(audit.vscode_user_manual_positive(row, fixtures, self.CLI), False)
                    self.assertIs(row["removal_verified"], False)
                    self.assertIsNone(row["native_uninstall"])
                    self.assertTrue(all("--external-uninstalled" not in argv for argv in calls))
                    if attack in ("replace-on-remove", "new-member-on-remove", "delete-on-remove", "case-root-replace"):
                        self.assertEqual(len(calls), 5)
                    if attack == "case-root-replace":
                        self.assertIs(row["vscode_user_manual"]["protected_after_refusal"], True)
                        self.assertIs(row["vscode_user_manual"]["roots_unchanged"], False)
                    elif attack in ("post-list-write", "post-list-id", "post-list-binding"):
                        self.assertEqual(len(calls), 6)

    def test_exact_requirements_refusal_allows_cache_but_preserves_native_authority(self):
        import copy

        audit = load_audit()
        fixtures = self._fixtures(audit)
        self.assertEqual(fixtures["plugin_aliases"][13], "marketing-kit")
        with TemporaryDirectory() as temporary:
            row, calls, _, case = self._run_case(
                audit, Path(temporary).resolve() / "allowed", "marketing-kit", 13,
                requirements=True)
            self.assertEqual(len(calls), 2)
            self.assertEqual(row["status"], "install-failed")
            self.assertEqual(row["install"]["stderr_tail"], self.REQUIREMENTS_STDERR)
            self.assertTrue((case / "uap-data/acquisition-cache/package.json").is_file())
            self.assertIs(audit.vscode_user_requirements_refusal(row, fixtures, self.CLI), True)
            self.assertIs(row["vscode_user_refusal"]["native_unchanged"], True)
            self.assertIs(row["vscode_user_refusal"]["roots_unchanged"], True)
            for key in ("repeat_add", "repeat_list", "cleanup", "post_remove_list"):
                self.assertIsNone(row[key])
                bad = copy.deepcopy(row)
                bad.pop(key)
                self.assertIs(audit.vscode_user_requirements_refusal(
                    bad, fixtures, self.CLI), False)
            self.assertIs(row["failed_install_registered"], False)
            self.assertIs(row["failed_install_residue"], False)
            for attack in ("native-new", "native-replace", "native-delete",
                           "data-root-replace", "foreign-sentinel"):
                with self.subTest(attack=attack):
                    bad, _, _, _ = self._run_case(
                        audit, Path(temporary).resolve() / attack, "marketing-kit", 13,
                        requirements=True, attack=attack)
                    self.assertIs(audit.vscode_user_requirements_refusal(
                        bad, fixtures, self.CLI), False)
            for field, value in (("index", 12), ("index", True), ("source", "foreign"),
                                 ("target", "cursor"), ("status", "passed")):
                with self.subTest(field=field, value=value):
                    bad = copy.deepcopy(row)
                    bad[field] = value
                    self.assertIs(audit.vscode_user_requirements_refusal(
                        bad, fixtures, self.CLI), False)

    def _invoke(self, rows, *, acceptance=None, platform="linux", limit=30):
        import copy
        from contextlib import redirect_stderr, redirect_stdout
        from io import StringIO

        audit = load_audit()
        with TemporaryDirectory() as temporary:
            report_dir = Path(temporary) / "report"
            argv = ["live-install-audit", "--cli", self.CLI, "--kind", "plugin",
                    "--target", "vscode", "--limit", str(limit), "--report-dir", str(report_dir)]
            if acceptance is not None:
                argv.extend(("--acceptance", acceptance))
            projected = copy.deepcopy(rows)
            versions = [
                {"status": "reported", "exit_code": 0, "value": "agentplugins 0.1.108"},
                {"status": "reported", "exit_code": 0,
                 "value": "1.144.0\n2c33b50b16c472efe58357d1d2a8a5414bc23fbf\nx64"},
            ]
            stdout, stderr = StringIO(), StringIO()
            with (patch.object(audit.sys, "argv", argv),
                  patch.object(audit.sys, "platform", platform),
                  patch.object(audit.platform, "machine", return_value="x86_64"),
                  patch.object(audit, "vendor_bootstrap_attested", return_value=True),
                  patch.dict(audit.AGENT_BINARIES, {"code": sys.executable}, clear=True),
                  patch.object(audit, "version_evidence", side_effect=versions) as version,
                  patch.object(audit, "do_case", side_effect=projected) as cases,
                  redirect_stdout(stdout), redirect_stderr(stderr)):
                try:
                    code = audit.main()
                except SystemExit as error:
                    code = error.code
            saved = report_dir / "vscode-plugin.json"
            return (code, json.loads(saved.read_text()) if saved.exists() else None,
                    stdout.getvalue(), version.call_count, cases.call_count)

    def test_explicit_full_observation_retains_failures_and_requires_every_proof(self):
        import copy
        import hashlib

        audit = load_audit()
        fixtures = self._fixtures(audit)
        rows, commands = [], []
        with TemporaryDirectory() as temporary:
            for index, source in enumerate(fixtures["plugin_aliases"]):
                row, calls, _, _ = self._run_case(
                    audit, Path(temporary).resolve(), source, index,
                    requirements=index == 13)
                rows.append(row)
                commands.extend(calls)
        self.assertEqual(len(commands), 29 * 6 + 2)
        report = {
            "target": "vscode", "kind": "plugin", "platform": "linux",
            "architecture": "x86_64", "cli": self.CLI, "fixture_count": 30,
            "fixture_sha256": hashlib.sha256(audit.FIXTURES.read_bytes()).hexdigest(),
            "cases_attempted": 30, "source_selection_complete": True,
            "full_coverage": True, "passed": 0, "prepared_lifecycle_passed": 0,
            "verified_negative_coverage": 0, "results": rows,
            "client_version": {"status": "reported", "exit_code": 0,
                              "value": "1.144.0\n2c33b50b16c472efe58357d1d2a8a5414bc23fbf\nx64"},
        }
        summary = audit.vscode_user_manual_summary(report, fixtures)
        self.assertEqual(summary["manual_preparations_observed"], 29)
        self.assertEqual(summary["manual_refusals_preserved"], 29)
        self.assertEqual(summary["requirements_refusals_observed"], 1)
        self.assertIs(summary["manual_preparation_observation_complete"], True)
        for key in ("native_activation_qualified", "native_uninstall_qualified"):
            self.assertIs(summary[key], False)
        self.assertIs(summary["uninstall_incomplete"], True)
        default, native, _, _, _ = self._invoke(rows)
        self.assertEqual(default, 1)
        for key in ("passed", "prepared_lifecycle_passed", "verified_negative_coverage"):
            self.assertEqual(native[key], 0)
        code, measured, _, versions, cases = self._invoke(
            rows, acceptance="vscode-user-manual-preparation")
        self.assertEqual((code, versions, cases), (0, 2, 30))
        self.assertEqual([row["status"] for row in measured["results"]],
                         [row["status"] for row in rows])
        self.assertIs(measured["native_activation_qualified"], False)
        self.assertIs(measured["native_uninstall_qualified"], False)
        self.assertIs(measured["uninstall_incomplete"], True)
        self.assertTrue(all(row["removal_verified"] is False for row in rows))
        for change in ("missing-proof", "changed-digest", "wrong-post-binding",
                       "missing-negative-proof", "counter-upgrade", "partial",
                       "wrong-version", "wrong-fixture", "reordered"):
            with self.subTest(change=change):
                bad = copy.deepcopy(report)
                if change == "missing-proof":
                    bad["results"][0].pop("vscode_user_manual")
                elif change == "changed-digest":
                    bad["results"][0]["vscode_user_manual"]["protected_after_list_sha256"] = "0" * 64
                elif change == "wrong-post-binding":
                    bad["results"][0]["vscode_user_manual"]["post_refusal_binding"]["version"] = "changed"
                elif change == "missing-negative-proof":
                    bad["results"][13].pop("vscode_user_refusal")
                elif change == "counter-upgrade":
                    bad["passed"] = 1
                elif change == "partial":
                    bad["results"].pop()
                    bad["cases_attempted"] = 29
                    bad["full_coverage"] = False
                elif change == "wrong-version":
                    bad["client_version"]["value"] = "1.144.1"
                elif change == "wrong-fixture":
                    bad["fixture_sha256"] = "0" * 64
                elif change == "reordered":
                    bad["results"][0], bad["results"][1] = bad["results"][1], bad["results"][0]
                self.assertIs(audit.vscode_user_manual_summary(
                    bad, fixtures)["manual_preparation_observation_complete"], False)
        broken = copy.deepcopy(rows)
        broken[0].pop("vscode_user_manual")
        self.assertEqual(self._invoke(
            broken, acceptance="vscode-user-manual-preparation")[0], 1)
        for platform, limit in (("darwin", 30), ("linux", 29)):
            with self.subTest(platform=platform, limit=limit):
                result = self._invoke(
                    rows, acceptance="vscode-user-manual-preparation",
                    platform=platform, limit=limit)
                self.assertEqual((result[0], result[1], result[3], result[4]),
                                 (2, None, 0, 0))


class WindsurfPreparedAcceptanceContracts(unittest.TestCase):
    # Exact current hosted report: 160 public commands, 25 Prepared lifecycles,
    # four component refusals and one signed-source refusal. Consumer replay
    # does not execute the vendor or transfer native qualification.
    ORIGINAL = (
        'eNrtvVmT48iVoPuuX0HLpzs2hQzfl+qXkqnbemQj9chGug/XlGVhvkawk0GGuOQiWf336yC4ACQIgnAkyUC4zFSZ'
        'CbofAMcdZ/HzwfGv341GH8yzM5+dfVTLDz+PPiCAWAZhBtnfIP0ZgJ8x+kgwFgT8z/APAD78lPdZqvmTW7f/Op7a'
        'xWrui+Ofw7/yo6+T1dN4Whwzk3F+6OF59uIe5qvp1M0fvs7mnx8el+7l9UE9uemyaL/I1MqOl0W3p/HyeaUfF88q'
        '740x0tpqazQT3ggtGQYEM8y1I8IZqi3EAGhW9H2dPz47ZbedAYRUaciA0xhzrJEACFOjACMAaWmwM1BDIyonDlf6'
        'OF7fDMGCI0A4Z7Jo4cfflqu5y8UjyvImHEvplLOQauw0kAIqph1EQCBrhUCUIumEZIqGZhBRLSkAijFmEIQQuc1l'
        'T9TSz+YvucTJeLr6VhxWc/M8XjqTnzP/6Ztgj4zslBvU96jHUzX/3qTmL25qZ/NsPpstH0LrB+u+lAYol/HFzRfj'
        '2TQI+Vc4Go4vlmq5WuRC5+51Nl86u24ffvmiJqv1payljILmwUeIPlI++n8kowQ5iQGn6H9s27tv4+Wjmdm8DwiH'
        'ftuet9NJyzNmFE78UdLmE23Hy8xW03zSYlCcXy3cIkz7XD/5eXY/LGaruQnD6yZB6eHqQseX14lb5lKX85UrhK4m'
        'k/BDuIFwPeUf5m6xmizzW/j7+qKKG6t/ONbHw2H3Lb/g/REb7nC8XA+oWk7UYjFWpQ4nnr71b0ZNZ9OxUeHaimEt'
        '5vBxw+2Rx9eVnoxNfo8vKhzaXXf+v/3fRhc/wbt+ytqDA0e3tD6cZZsbqxw9uvJN24WZvbrqwdXCzQ+b5c+TOhD5'
        '34sw43YHfv2p39udjBfLQd5YGsdh3NjcvQSzlYbyFjf2u4Mb3JvhqVqOv7jH8Uu4mcfV1Dyr6dPaKXk1Wbjj9u7b'
        'a/BOIW6bBzcVPNnj63jvRtdNbbjr+VivloWD/cA5D3oarxYPdaO99reL8XJWxBH7xqvpOPfSapKt1Zxt9Fzu+qqW'
        'z3vPdlL+l/HG138IUZHWUloLJTCQS6S5lIB4KTh1DCOkjbGI4HL/hfvHyk1N7mth6fBy7tyjHT+5xdojFjHZzwyG'
        'GITgIIsZBiUHjFNANJUaKs4NJYxpZZFRkoeDShlLCaWWGIoBtFyVTxzc4tgH8cdncQIr6zglPsR0kAapCDmiibBU'
        'Mqc9D0eBthyFG4QynFhi64l1woWmFpDtxPhtN7xF6HHC7+9jo9cwWGrubDYJV2a+m4nLXkNrZ8tRRWg+mVSnRCU+'
        'Kut2aWer5eNSjfMOH/716cMipAUvahufffrwM/zp04dNjBD+9Sn3Bp8+hGNFuLM+tFgZ4xaL9WGrliocDJLUOoYK'
        'R3/+e/jXclaS+elDHr6BdYc8xppN84hlsXrNI79NhzxqWjd9Ma8hJJuH3uv2U/XiigvZKmp9eN/5005Lnz789mv4'
        'SU3GalHTZRcorX/bPmDrn4prXx/f6HNzz2qxOTydjcIzHwL06dPITb+M57PpS/6UzL5OnR2FSNuGX9adXpX5nD/b'
        'xSwqFNbPZN1cU/F4rQW3fb42mqmMazm63g5y6c42g3pCl+PpMtdlYb3C7+uo+NOHycx8fsytxLrLwzLY7JV6DbP3'
        'i8sw+IZB9v37EnA4Gz8Up84AWLfIp1H+l4+5hPUZdk9jlLjl7KUYyoX6kk+Q7YWuLf1aam7gixb5oVIyUBr8YIND'
        '6DxeFu3WLYJBLjzLesKXp9Ovv4VG4SkLyi7OsM0qGp+jTTaxeRg2k1CtL2QzYV1INwlXNIMEgYxohbIw3kERijpF'
        'FMCWqXPzPNzhXNVO9dMXHAZiFdKM0uP96cO/r/PB/5zMtJqM/HjiwhSau9FGYHggwq/LZzfa6mo0m4/D8ITGubpH'
        'r/NZ3unf8g65dxv99fN4MgkypnYUTNR49vC//va3v4z+/Ie/jPLHfzH6GnL1tcAwaVfjqRsVF/CHP/3x4+j3q/BD'
        'SKPM+sZ+Gi1ns0kQPF6Ow+T9Z+ng4iGPD7Z/D6nTZH1C530+4l/c6HUWRvn7KMRNKghfTWd6bYdsPqa//nTCVnad'
        'Sb/99qniNsN9u/l8Z5rLPy3HLy6MzzZGOHIm67DnR9n/XHg7B1CetL1M5BOmv96vFB51ffBsTPNwFMCUHpztpTc8'
        'RjVjHpyDm+9nXOHJdsc2j1L+DH3Z/773W/lvlWm88Tm5hV2v2hWeZT0/C41MJrOvm8PhDse+3K+i9OLXrYSNbyo5'
        'kX+dUmgY79nkyzrw7ORx8rlRiky3iqyJTjenK8Lbbfi3mYqlsK93V3oQ8JXlx0Z84fnOx3RtWoIK85mjwiNz7Ct+'
        'XatpHpqFgLwYjmAd7crsZt6hmsxsboPIsm4bFWtW83k+kVt32J2i+8Bvz9ldQukiaufF/gy1P++6L6bqdfEcHqRS'
        'AyLywXkZfwu/7+f+2rb+9muPdnmT6uWu7tG6yTJfpEaHv27XANfNqqtyxbLrQx4Qm9nUj58+lrPMXw8l7SNrE4z3'
        'OBjmQt6+4dbg7JsuHmfTyffSsmZxz7k7ftxke9PVZLJ3NXmA93U+Xobw7yhnXf8YrjToarp4LC1u1jRz34IxfNys'
        'wIaUdlqTAxeXob+H+3hcB9/HosKcDrdTOlXlPnb3u4nPj/p/DtHu467VLsnaGc3jSxo/TUuTsogQ1Cbz3o/cv8pL'
        'A/tcr4NjOlzSqXn6z+f85Z6R6XlVVPXZq2bsxRJ51YXEnPTMuX74+sBVVgiC+Th6vMP0dGr5mC/L3lmS7+ezlxNp'
        '/sn8f92nu19Yd/9B8UCHhYOo+HZjlbb9NqdB3EAmFKaU5RePvCIGQKgPFyp2a4eHSxWFNR1tYr1RoayRmoQm9nux'
        'iuEW73epojDoaa2il7WKe06LN3YzZccpO07ZccqOU3Z8X9nxxjwf2MCDDLiNNfxwKmLe5MXlImslPaw2m6+m+SVv'
        'E+Jq3ldtWshbHCbH1Uavzzl3dKLNOvg4d2Fhqv93mPqVdnXCStlqRYwPo5Jnx4WGw2R4ytPleU1ae9RyMbZrCKva'
        'rIgd7Q8Zs91g7QualXsNabmarl47evLpaFROXkZ/z4+EPL34I//xIJcZlYx9tU3pqR1d9thW5ezM6qhXu707yTYe'
        'Hh0YyV2DsgsfVX34rs1xhDJqH6LspFQSnVGrTGd/G9uMZ/RpQ7R8+lD8+lv+x68/FWO7D8pGBwnDttOm4T7aHR2E'
        'u+ufd3HcqBLIlX47MX/60VSbMSkX8UpKKbc4cY81ejuKcEdw90sRxo32cdz+eF0YPyrH8Z+m/XmK19kid2X5bd5t'
        'NN+nZwwKHk/WTnm5DI/NIncEM39oEPdaUZPH9VJpY4uJ88ucYj1cEs7bFAutRTC5Wac+bFJky/XrtNtTnPJDWwe7'
        '9iqhZ1X83OXrxcGr5GRL3q60bH7Q9B9hlOe7dpPZU5gO08/l81buv2i+Ead8cH2Phcds1XANzx06oc3ItJB5omm9'
        'VOu+bflcZfOoTtnvja02nr+h4TacqZ0724sLUa/SueffTLXH7WAcjfGXRS72MbcEjwURcCiz3GLu/Gqh6idk/pA9'
        'bh/F4KSOpkv5cd9VRxqCl0XeZOom+XPiior9RuTvSs9Vazwb1uLZeVnDfVvy4dDZR3eUSNC3CGenYRwKm51GcvBo'
        '9uo1hI2L54eaoT7gsptaHmHYldXbetn7+itn0rCQkyKDrcyX7iz0IYnkQjpJcUgutfOKKtmZyQbCOYs14x45DlVI'
        '6JjQSlFJsAAGeSbz9/CM5FQQy0Pi6pTnkiBlcMj/rGjHZFsAuYAhBRZSqnDFkhCqUEjCPcGKCIgwwAiGdNoy5KUR'
        'CimlAVNCcigQZk1M9rGzHyCSDT+Ctkj2Oj+oFh22KspezGsTkP1TG7R7K6w92V3pcXOwO3bGH1dL2z6kPVRLK6rs'
        'j+uG/dZK4WC5bkmt0JbLzFrJM0IEyzRGNnOQa6uscliaxHUnrvvuK9etJ3K93a/3TaXC9WFI9NAc/6Si9ZEy64vW'
        '7X3NiaL14cB0qFj34kEbKtax0WKPFesDLZ0qWNcq9WS1+sQQ1JWqLxru2lL1RRKGCnLjBHK/X5D7QlfUEuJuyPlP'
        'Edwdkvl4grvrSTsR3D2uJlxlPeGNE9z7YOHkcsExwX2RQzhBcPcSAVy+QBAVyNYD3MAIAplAQIRZojzgQS+aGXdV'
        'gHu4SxIR/HZak0j8dsqCUxacsuCUBacs+IcA223MXwK27wvY7jZm9wds70x8E7B9yWN7Ftjuw2DXAdtlAxbPa7cN'
        'Sc7x2k2JzTB47Ys1lXjtxGsnXjvx2onXvhmvjep57efgsl1m3Zc1FjAcbHv/XurBLWZ6PrZPLtGjbxrnTsM7eMz7'
        'hGVKAzrcjbjPPsuX7sqdFf41tG7gwk/PtAoQrhxHTGIOaMiAGbcEW6MYxhR64xmGhAtBPFEngHB8FgjX0HtADSRa'
        'AI+FR4ZDyaBngBLgmaM0pJLGAmmQEUgY6zSynhCqqQrZs2kJhEPiGEUyXKzjnnILVcizGRJAK+ehx9oCipTCIFyF'
        'DD84JQgNqazUHCgJmoDw1iM5TFBcfARZXuFCcTt4H+juAtq7puPNoe/YWX1cYW37IPZRYa3RaH/sN+q3zooGy34z'
        'J2gwSyZDRsqMaCcyoZ3KKORYKW2lpjCx34n9vvuqd+uJ3OgMmrzOJTuY7aKjhxOhUKqKn9ByfW28vWM6u61Zffx0'
        'qlaOT9fKe3G/TbXyyHCyz1p5zeRts9NZg65bbHvWOFJ1JfWLZkltSf0iCU0lddxcUsdph/AElr/NHcLPuraLNwxv'
        's85Yy553WDc4w57j8+x515OeOdcPX7i4ytLFm2fPq8HImXWIYw79Iv9xgkPvJa7ovHQRFVvX4+ieCQGUQAAGMYgb'
        'Qj3UXqGr4uiDXyyJoNLTakmi0lN+nvLzlJ+n/Dzl53eVn7dC3tvY1oS83xfy3m3M7hF5r/iHJvD9kof3LPjeh3Wv'
        'Bd+PTWk8/942+DnHvzdlUsPg3y/WVOLfE/+e+PfEvyf+/Wb8O67n3yezlfWTkM8OaMfy43tK0Oyb3LM8DeRwcPY0'
        'lu+KZN8N97Ug9rr5VeHXieVWOMCpgxrkW1tIRolkWhmPBITaUSsJkLDzhuZIOmadY0A7FIQCFpJoZ6FA1iEkPATQ'
        'aWpZ/s631IIDKK0g4WIQ95gAodvx687mHxDjTNGQWgtqHeaEcC88MEiEHwkkwhpHuOHWe6V1yLE1EDJ/cZ0TRhs3'
        'NK+JBq5Gqlf3AK/bS3w/p2ZT13o38bOSspfx03yb8nQXalfzPOjMZjpf6YoStQiRkp5921yYy5azbOq+LfsQ2Zec'
        'MM7hZmMkLVfzMFtCNpQtgmGJkfTV6ezVbas8HWUEWx9sT6bD05a9zvPlGeOiBvHrPNjXybb8GLPt/W62XvAqRLXP'
        'zd+CiLWNx4X9tua8j8J+VZn9vQCB+y3p48G+AAEZkdgwkBFhSEaUIpmE1mUeMeKRJhBLkF6ASC9A3D1g0Xoin7L+'
        'R3vVras6sA+2os5lpM0Aywquxyrae6LzWMVh2tRhd8BeXG3TV90jE5A+iYrqlG0FU9RquA1HcWJo6hCKi2ZELUJx'
        'kYTB7p3PefxLDj8dNliH6uXxfPjr//7jn/708cW2aTt33s1z3Wx2Ul1kC/v54f/+x+///c//ESVCvY6j+hf3vCpC'
        'nChJT7MQ7KtFlIwQuAZzP+0iZJw9BVfxVX3vrtW9iFit7CXZ78Elj002D7M45DtRsrxTIft1cdoJas7yVOSph5tb'
        'hrsK6Xx4+Gedb27h1Nw8xwzaRkK3J2HXPX7IN4K6Pwc7ERGPQUgYvocAaJG5aQjPXHe9Hgrqpt5DKbFaPpTXXdmH'
        'kiJ0/hoG7nnsJra7tvciuul53z9Ww3tJ3XW7lxGl1Rh1dtZjDwqM0lyMyuZPs2zxoubLrbvprsFjUd0UeiwnVr/H'
        'Erur+1hWjPaX45CSLBcRSt9K6KjrbfdIFW/WZrvfyE5Ap/vY9e7rNjrPj52E7rNCz5ZZyIpDjJsvEXfXaFVMN71W'
        'ZcRqtyqtu46rciI0PZ99Xbh5Fv4Z0skY43csqZu+j8TEqvxIYHetH4nqrniDO2s6dO2k2tAvUpdBQmflhb4R2lLm'
        '2WUbNK+74ipSuumwIiJWnRVh3TVbEROh5GITCDfv7sNKIrqpd98/Vrd7Sd0Vu5cRodX5bJot5+OnpyjFVqR0021F'
        'RKx6K8K6a7gipruSLeys2dC1kzpDv0gdBgmdFRf6RmjLzrrPxHXnbhrLe8bqLJfRXWt57wi9hbxnOZuHqKu79vYi'
        'uulw3z9Wk3tJ3fW5l9GLVpdu0W1J9IAH6z48B3K6jdGBkNiBOhDXfbQOBHUfshxcmEQvl1SldFJ1VUSkoqvCOqu5'
        'KiZWyRtWLlLJWykRSt6K6EXJW2GRSt6K6a7kcOBp8Tx+7azfnYBOqt31jtTqTk5nhe4kdNfl01y9Pv9jksUsQpdl'
        'dNJoWUCkUsuiOuu1LKS7ap+/v7q5nY8jkt+SiE6KLfWP1GtJUme1lmR01+r67YzupnXTvZM2N30jNbmR0lmLm/7d'
        'Nfj5S2ftha6dNBf6RWotSOissdC3u7ZextNxcbCr0vYSOulu3z1ShXtBnTW5F9FdoVO3zEOANZoxD7c0DXFtZ93W'
        'Cuuk5lpJkRqvldlZ+bXSuo/DZkdVPZ6Ml91RoqqUTpqviohUeVVYZ11XxXRX8mtuqzO/mhaAfGc1H8rppOhDIZGq'
        'PhTXWdmHgiLVHafkCNX2otBINcYob/zqJuNpjAJ3Eropcdc9VpE7Qd2VuRMRodDVZPXSPa3adO+myqJvrB4LKd2V'
        'WPTvrsF/rNwqYj5uunfS4KZvpAY3UjprcNO/uwbnaP2+W2bCfyaz7ut+h3I66fRQSKRyD8V11vKhoCh1L/4xidFy'
        '3r2rcvO+8TrNpcSoMu8fo8EI7XXVXLzWYjQWoy2nJvmbI9nCr7rrrSykmwbLEmJ1WZbVXatlKfH6/TxeRqs3lxGl'
        '3VxAT8rNRUXrNhfSXbWbjRI6q3Xbv5NKt50j1bkV01mVWwERanRm7vKXipaziDWqqpRuKq2IiFVsRVh39VbERCh5'
        'TU6/TpSJg1sP5XRT9IGQWFUfiOuu7ANBEeqejl9fXQQSsBPQTcHb3rGa3crprtKthAhd5nsozFcv3XW5FdBNl9ve'
        'sbrcyumuy62ECF0uw9WbLN/TKWZyVqR002pFRKxqK8K667ciJkbJIb6ImK5F9/zNrHwrmBgBnftGD8haSsRIrPt3'
        'H4JlHxDLMpphWfaIsCx7IViW/QAsodtc5VsVdlfuTkI3ze66x6p1J6i7TnciIhSa70fZfdll072bKou+sXospHRX'
        'YtF/UyLshGNuRMQMwnwaMQShc8cBCD2j1R9kRCg/9I7T23qfvyjlFRI6a7Do3oMaC0FRuixEdFfol/WOOON/dlfo'
        'XkInhe67Ryp0L6izQvciuiv0q/KdVZn37aTEvGOk+nIRnRWXd45QmdPZbueF7sqrSInVRkVYd71UxERtsFIVFaHs'
        'dUTWfVOKbf9u83TTOXZ0NmK6j8tGQKwaF5kaR2pyLSJCmev+vehzLSlSpWsZ8VoNIW6+OLbMQ91FtIKr0qJ0XRXV'
        'k9qrQqNHoCoufjCCrO9P89lqaqNHoiQqahhKcnoag5LE6AEoyYrX/pdXE632XEaUvnMBPSk6FxWt4VxItGpj1Rql'
        '0p7U+eDn6sXl/4jQQ+xwRA6Fn8y+xg1GIaHzcBTdexiQQlCUOgsREQrdbBTfXZ9bAd3Uue0dq82tnO7K3Erorst/'
        'qrn658Mf//yXP/3Hn//jv/72+7/98f/81+Nf/98///n3//f/6yqt67AUvTuNSdE1ckAKIZ1Ho+heHopTHzrN90yf'
        'Tdd74qupHVu1PPzC3XZz533TxfbjtJVvpBVf7Nt826XyjbX17vpf5+NwNdOjT9asf9zsjLF4LH3frKaZ+6bM8rHY'
        'KvvUl/+Ky9Dfw308rj99cCwq/2qbLZ+q+q237f1uNp466v95PJ897lrtPqhS/ubfwSWNn6alHX43AP/mwzv7TXD/'
        'Vf4y0P7DLlHbgR9+3Klmk+X2H/8pS4j8Ok9VVHVr4+oHe9YtD7bujjnpmXP98M8DXeUDQaPRb6e+Rp1/qe1H7bm/'
        '/R7H2S33Sx9l+NfRN4+P925fzhp/PvwS8oVbcFc/gNzvXuxdvuoS9e2ByneN9581Nsp6CRCUQmtsOYTIcwzs4Vdk'
        'dl8WO/yOTGFsR5tt90eFtkZqsv7aZ/GJmc0Xd97nd2QKg58+JNPLh2Tu+ZslGyuaPl2SPl2SPl2SPl2SPl3ytj9d'
        'sjHnBzbz4OMkbaznh1Px9ibPLn+ztZJuVpttP5G+SbAPP2pfblrIWxwm29VGr89B86farIOVcxcW5n6+vVelXZ2w'
        'UvZbEePDqOTZdqHhpo+mH7VcjO3KHTUrgk37Q8ZsN1j7j6FWP1IfZvJ09drR809Ho3LqM/p7fiTk/cUf+Y8HmdDo'
        'yBdUW5ae3dFlD29Vzs7ajno157uTbMPo0aHt3LUoO/pR1dPv2hwHNqP2kc1OSiVBGrXKkPb3sc2URp82H8r+9KH4'
        '9bf8j19/KoZ4H8uNDhKNbadNw32QPDqIktc/78K/USX+K/12Yhr1o6k2Y1L+MGNJKeUWJ+6xRm9HgfEI7n4pgr3R'
        'PtrbH6+L/kfl8P/TtD+H8Tpb5B4tv827TQL6dJBBwePJ2jcv8/0HF7k/mPlDu7jXipo8rldgG1tMnF/O8ls/WGnO'
        '2xTrt0WQufmW2GGTIsmuX/7dnuKUO9r62bVzCT2r4uduvUGzfcw/jp23K33a7KDpP8Ioz3ftJrOnMB2mn8vnrdx/'
        '0XwjTvngAR8Lx9mqYT7djnzRZmRayDzRtF6qdd8eX1c6PDv5WuHjeoWnsdUmAGhouI1qaufO9uJCEJxvJ2q3U+1x'
        'OxhHY/xlkYt9zC3BY/GV10OZ5RZz51cLVT8h84fscfsoBi91NF3Kj/vu+3UNMcwibzJ1k/w5KTZQ34r8Xem5+te+'
        'lDC168+zr/1D+VvsQW3hB7I/sq9WfLBh/rt59rzaryt/KNZi8l+3dnr/m1HT2TQkp5PHnUmvb7g9sh3ZjT2qhlZ/'
        'Ly9lPzwH2/4wz/ns+bqg+vC4dC+vD2Wvl6mVHS+rS+D5KnTlQM09rY9n2ebOKkePLn3Tdu0PqgfzOXDYLEd11IHI'
        '9QcHdwd+/anf+127iiHeWBrIgdzYxh2ksbzFjR0lojtLvP12ar6zZiUHr/qdXfsQ9RYLXNtVlxAnVWPU07Xf2uHO'
        '8/vZYpwvhrWqRpe7bmCAD9u13lMn2Fd1kbVaS2ktlPlyn0SaSwmIlyKktQwjpI2xiOBKdFtbyz1RyaVYCEeCCISQ'
        'JcBQjjH34a+KUUqlCVkuJs54KwA23nFvCURMOM8Rclir8olP13FDMoUJ945QC6w22lJNLDU+nFtzB5yhBhgLBCCC'
        'OUWMFgZ7Bj3QlitJd7jGPg7f8wB13j9/wXW1WKt6s56d7biELI9RnS0HF9u1jbuqAh8UecFH+LGoSewRlcXq9XU2'
        'X2465MFTscZvXh/Xsda8WsMoNBUUVRQMdp3Lq/7r5d59mW+v3HNV2lLVdKPQw5rpdFYURkMsOypVFrPZ17yUulmH'
        'OFMzjZ2uxzXTtk9YDzXTA2Ue1kw3lcgOJVPSb8mUvP2S6Ym6n6QEKCtlZpiGGXECZMpjkWFribVYKGTguYleXuKp'
        'zvXTF1xkZY+lB/zTh3/PsbXRf05mWk1G64R6FB7B0UZgeCLCr8tnN9rqajSbj8PwhMa5ukchd8w7/VveIXdxo7+u'
        'AbhRmJyjYKTGs4f/9be//WX05z/8ZZQbgMXo63j5vBYYZu0q5GSj4gL+8Kc/fhz9vlLo+2m0nM0mQfB4ua8eFgcX'
        'D3mQsP17SKEm6xO6dRkpTKlRUQ/MF/ZCrjpaTQu4K1dDUUm61zr2Gytgt57IB8Z/Z3zqXcslpeuH4ygmVayPNFpf'
        'q27vdM7Wqg+0f1mRuhd32lCkjo37eixSH+npfJH6cGKfr06fOkn3wa8tS18kYQhl6dKy824VHB3+uluPrFmdTnT3'
        'UOjuh8bkuR3UfbZrZAoeT3N3PWknmrvHNYCrrAK8cZp7HxycTPGPCe6LTP4JgrsXb99lbSAqgq0nuLFjRkuHw10D'
        'bKWwgioJgbsqwT3k1YgIgjstRySCOyXAKQFOCXBKgFMCfD0uu41NTFz2fXHZ3cbs/rjsncFvIrIveWzPEtl9mO46'
        'IrtqwuKJ7LaRyjkiuynjGQaRfbGmEpGdiOxEZCciOxHZNyOyaS2R7cdPL2o4MHb1dhLy+RY57DSGg0Cw0zC+F/r6'
        'cKT7BK9rZF+RucZGMI4IxDokqJjycBamDVYYWmEU5za0stRx5zFADAgjDYRW4fCrUFCJdsw1417nyS9DivuQMDLq'
        'wtUbw4THEimmvIZOCAZDMmwM9Q76kDJzJ72ySmvVxFwf+PeEW9fh1msltUet981vTlnHTtCb1jX3euwPsKb9VjTp'
        'YAFrCzCz0uhMOsozQgDMtNEic1RAwp0gcLOxUgKsE2B9z/Xl1hO5xtz3WFo+dAqpqnylqvJe8ZcVlHtxnQ0F5dio'
        'rseCcllF52vJpZl8voxcIzpVkG+CUB9us50Hu8Ve3h/LWWMirQdMWtesubSArJt6vS++usd8/yoZf+Kru/HVvfj+'
        'CxcEomLXerRaaWQ8sVhxiT2SkikFueHmqmj1QJcgIqjqtAaRqOqU9aasN2W9KetNWW+f3HQbq5e46fvipruNWeKm'
        '+7HVddz0znrFI9Ntw5BzyHRTJjMMZPpiTSVkOiHTCZlOyHRCpm+GTLMTyPTc6RDMDYmaPrijRGu+TXA6DeNQ2Ok0'
        'ku8Inz4a7EsJ6t3HCZtR6rrz7CutITsRlnOBALAUKidJyDOZ1kwqjzDFxFhohOcnaGpxlqYGCPuQA0MKIdIhFw5p'
        'q7NMEs1YOEKFVoBYQPOPPymoAaRUiTwDRsx6RwxuR1Or/NNOWuqQAYfkmwAEtLCWMKwECEmY1pY5Lrk2QnOLbTgL'
        'NE7ykH4pbJw1zTT1oesfJFCNPrJYoLrQ0yVMdanHzbHq2Jl6XNNs+3D1UtMsqbI/spr1W9VkgyWrnfJKaqsyAK3O'
        'iBU+0xrQTELvIMSKMoYTWZ3I6ruvMbeeyPV2v96nxH1zucZNvPeS81ax9SXn9n6nRcm5pPuaUqI4XXXuxZ82VJ1j'
        'Y75eq84VLbUpPJendJvac90Jug96bfn5IglN5WfRXH4WCbpO0PXbhK5PO6cOAHZjx8gk/QyDLc4z2F1PeuZcP3yV'
        '4CrrBG+ewd4GECeXAY4Z7IvcwwkGu5eY4PIFhKhAtx7DdhaGeSMwgMpYDiwxzElExFUx7OEuWUSQ2GnNIpHYKUtO'
        'WXLKklOWnLLkK0HabQxigrTvC9LuNmb3CGlvDH0TpH3JY3sW0u7DaNdD2nsDFs9ptw1SznHaTanOMDjtizWVOO3E'
        'aSdOO3HaidO+GafNT3LaZq6+ToYFaldvKYGhb5XUTuM4GFQ7DeV7YrXXo53p+dg+XYXZPpxdFWhbOWYQM4BbQYgH'
        '0HsgqOYeUR8yUmukp1gxLTpvga2kNEpSIyBQSBJJlKbUsZDLChROKrSQHnGFKUceAsclFEAKCYmFlANl20Hb1Fqi'
        'KCWMawyJVNhYRLEmjHLJNPQAQcuYUzokm0QLr0KOjbgGCEhPtGfnoO2DOGCA1Db+iMhHGI9trzV1Gbe973JzcDt2'
        'th5XQds+YD1VQfe67I/c5v1WQflgyW1CwxwSWGUWYZ4Rg3WmtVOZMx4zKCHFMJHbidy+/5p064l8wvSfcCzxVelD'
        'V/G+y9Il1dbXpds7n1Z16XLk2mHvsF6ca0N9OjYI7Lk+vZ+rbQvUR/ptWaiuGZe6gvVFs6G2YH2RhLSXdsK6E9Zd'
        'n4O357pPrBac4rs75PPxe2x3PWmnPbZ7XFC4ypLCG+e7SzHG6RWDY8L7Ik9xgvDuJV7osNQQFRHXI94aIS/CFPLh'
        '4gCQICc4sAbqqoj3gBc3IhjvtLqRGO+UT6d8OuXTKZ9O+fQ9AOBtrGUCwO8LAO82ZvcHgO9dQBMBfslze5YA78OM'
        'nyLAdzYzHgFvG8OcQ8CbUqFhIOAXayoh4AkBTwh4QsATAn4zBFzUIuBP4+XzSg+H/96nOcWd1dYVEnr6xmjwNKoD'
        'ZcMPzE8ax+GC4aee3B9ChR/NqwMknCMmMQfUScq4JdgaxTCm0BvPMCRcCOKJOoGEo7NIODIOEqwMBEx4ylzIAImG'
        'kAHDjEAUUguw4BRJS632EHuANXJAAIUlxRa0Q8KdIiE3JwISRqW1jkHFBeDYUoo5V9oCYjHxThogledGaq0JNBID'
        'Gc6NXRMSfm7chgiIw48QfIRZXtSKxMQLnbVnxEvtbw6Ix87duhpqu8ethxpqSZH90eGi3/qpGCwdbqCnClibectR'
        'RoClmYJOZNo7jQkgRgmb6PBEh999Nbv1RK6z+I0uJa6gfeQn3nc1+0i7p2rabf3P2Zp2JRg6VdBGpwvavTjXhoJ2'
        'bEjYY0G7NFfbVLOPNduilF03HPV17AtmwIk69gUSmurYqLmOjRIXnrjwoXDhdas7LaDwhjW+E0T4xen8GSIctSHC'
        'u530zLl++HrCVVYU3jgRfhRXnFsoqKPDL/AXJ+jwXoKFSxcZosLhejQ8zB6PCMYEUiWp9MxC6C1GV0XDh7qsEcGF'
        'p3WNxIWnTDpl0imTTpl0yqRvS4S3sZOJCL8vIrzbmN0fEX5o/Ju58PZP71kuvA8bXseF7w1mPBTeNoA5B4U3JUHD'
        'gMIv1lSCwhMUnqDwBIUnKPxmULg8BYVP1ICg8IP7SZTpW2TA0yAOBvlO4/hukO/qUF/Kep9BvI+E78uzyFqtpbQW'
        'SmAgl0hzKQHxMuSTjmGEtDE25GOdd/12FnOBuPZCQcOowswyj6TAykonPOcQ+dCOSW0Fk5IDjRC02imsBUMGtUO8'
        'oYICeqURsJIi4KkHgHmCMHdCWMaNp8IiGe5UEqsNoc4RI0C4M07yRLkJ8T708gMkusFH+BFEo9xBSxeh3Nv2N0e5'
        'Y+focc2z7WPVT81zq8j+UG7Zb8lTDhblFsIQjA3JlOc2I16rTFKnszCLrEVIaWITyp1Q7vsvQLeeyHUWv96XXFJ5'
        'fqiGK6ngfKTN+kpze0fTptK81fxlu4/14j4bSsyxwV2/Jea9jlqVmHeTuVVt+Uh498GuLSpfJCFt253w7PeHZz+c'
        'TJxbU9mnu0Wm3vEbdHc9aacNunvM/a+S/b9xHHsfK5xM748B7Is8wgkAu5cA4NKlgaggth7AFlohghTUVEDNFYLK'
        'GYM8vCqAPdTFiAgAO61GJAA75b8p/035b8p/U/7bP1TdxvYlqPq+oOpuY3Z/UPXOwDfR1Jc8tmdp6j7M9QmaemO+'
        '4mnqttHIOZq6KaMZBk19saYSTZ1o6kRTJ5o60dQ3o6khqMepg8qXYUoOCKg+vKNEcb5JpDoN41Cg6jSS7wirPh7s'
        'XsHqWvFXRKslzL8GBbTTlCmEpEUQeyGswQRbEtI9ihUlFgoZGggPnBRKQSiFIIoh4Nqh1YBqJCkAlBHlrKOIh97h'
        'VAYYT7UmHHEnQwNCNSDWQQ4kxJo7j5QDXOtGtPrI4ye4uhau3ujpAry63OPmgHXsTL1tTbOsyt4Qawh6LWoeihsQ'
        'Yk0JQMRQk0mLXUawD3+jCGdeOOWIJA5CnhDrhFjffYm59USut/t9FpmPvUMqM1+rzFzW/WWF5l7caEOhOTbU67PQ'
        'XNVSi1JzZUq3KDbXniCVmxNunXDr2+DWtUszbYDr5o7vC7nucU3gKqsCCbnuhlz3EgpcvlwQFdbWQ9fQaSMFdDQ0'
        'l5gZSQQT291urwVdD3eBojt2nVYoEnadcuKUE6ecOOXEKSf+wQh2GzuYEOz7QrC7jVlCsPsx2rUIdsmAxUPYbWOT'
        'cxB2U4YzDAj7Yk0lCDtB2AnCThB2grBvB2HDWgj72c1nn1fDQbAP7ifxnm8RwE6DOAz8Oo3ju4Gvj4a6T/S6TvgV'
        'wWsttMdCaQeksgBoiJngHCkmgGeACiYBhhBrCzHF1DKCIYVEcMO40lq0BK+p5gYI4ZAEzgKigVPeAcSpNQIwxw2C'
        'FFvLjXZM85AtG+qhRZJBJzGlqgm8PvTyCbuuw64LLbWHrkvtb45cx87Rm1Y0S4rsD7iG/ZYz4WCBa6kVp9qCjHru'
        'MqKsy5Q0JLOOy3xrGcq3361MwHUCru+4uNx6ItdZ/B5Ly0d+IRWWr1RYLmn+srJyL+6zoawcG9z1WFau6Oh8Ubk8'
        'mc+XlOuEp4JygqwTZH0TyLpuAaYFYt3Y7X0B1j3m/lfJ/hNg3Q2w7iUAuHRpICqIrcerw0SxRnlNgc4/B84Y9cZb'
        'fl28eqiLERFwdVqNSHB1yn9T/pvy35T/pvy3f6C6je1LQPV9AdXdxiwB1f2Y6zqgem++4nHqttHIOZy6KaMZBk59'
        'saYSTp1w6oRTJ5w64dS3w6lRLU4dZppT8+Hg1Af3kwjOt4hTp0EcBk6dxvHd4NRHQ90nTl0n/Io4NbOcUEJzcYJA'
        'CLwhEuQvzHJEEXcgHJbQWaAYBxgrDhkUJM//OABQctFyH+uQ5lqEsEHQIsCsAAhioEw4C5SaGi6UD3eIGaeMAGM4'
        '9xRK6ogXzHkOmnDqQy+fcOo6nLrQUnucutT+5jh17By9aQWzpMj+cGrUbwETDRanhlIBBzzJDLQsI5DwTDhiMuaQ'
        'Y0QwBXzCqRNOff/l5NYTuc7i91hOPvILqZx8pXJySfOXlZN7cZ9Nu3RFBnc9lpMrOjpfTi5P5vPl5DrhqZyccOqE'
        'U98Ep65bgGmBUzd2e184dY+5/1Wy/4RTd8OpewkALl0aiApi63Fqw5Fm4XqVc5IbJRhkzFF9XZx6qIsRETh1Wo1I'
        'OHXKf1P+m/LflP+m/Ld/nLqN7Us49X3h1N3GLOHU/ZjrOpx6b77iceq20cg5nLopoxkGTn2xphJOnXDqhFMnnDrh'
        '1LfDqXEtTv2i5p9drqPsc4kYfPNUdf1tJZ7zTnjOyoPRC855asAvpTqz4okMrRvwzoaT7as+hmrjrCXOeIiooEoo'
        'CgiAgiJHuSIkj3i1M50pz9BfGOABCqG45Zh6C5zz2CjjJdBUSUyh1whiIrCFlArFaAjoIWSIOCZUO8qTAA1DT42x'
        'YVjm0pT3XDvltVThDkKUazxhxnFGHXBCCGMNJ9haJIhTqInyPGF89rCnnZlVvuTtbDabZ0XHbDXdwI4XMZ/wdAzV'
        'EN+Un8Jg9WcjNxk/jYNvGe3WbEab2ToKj8roU/We8vDxxBz9edfPztwiiF6ONrc12oa2o+J5HgUv9m+jl3EID6ZP'
        'o13sPB0CGPWjsQNwHjvolyeoq+S/SaCg+vu1iYITteDaBadiatf9VLea02YtrWHt67ZraXWNOqylnVki+5GLaV2W'
        'xU5kf6526AeQZVYbpDQzpZnNaeaBqb5xnklq88ypm02Hk15W7iZllW/xld00hEN4YTeN4jt5XfdgoPt8WfdY9BVf'
        '1XVWSWe4VgJDjQzTlgnpLOUWCIa0dzinTLzx2gtEIAh/+NBMYa+QE7DlIg7jggtJDLfYAYytBtQ656jDDFNhlMeC'
        'E4mVFCq0hMwyq4nGBDtpAJOiaRGn6tnTi7p1L+rmOmr/mu6u9c1f0o2dnTflYndq7O8VXdIvFEuG+4ouFdg7JjLp'
        'tM0IxjpTCMFMMUKoA0QKatMruukV3ftHlNtO5GNb3yOgfOAREp58JTx5p/fL4ORe3GYDnBwbzvUIJ5c0dB5N3k/j'
        '82DyseCEJafXctNruTd5Lfd4maXFS7kNnd7XK7k95vhXyfLTK7ndXsntxe1ftgwQFbLWv5BLAKEIOw8J5YhRS4Mu'
        'gkqu+0LuMBceIl7HTSsP6XXclOumXDfluinXTbluT6/gtrB36RXcO3sFt9OYpVdw+zHUda/gbo1XDy/gtow/zr2A'
        '25TBDOQF3Es1lV7ATS/gphdwExmdXsC9HRhN68Ho2XrlfDhodPV+EpP5JuHoNIjDwKPTOL4bQPpwqHtFpGuEXxGS'
        'Fgx4xYJ8gYEFViEtvLNYQEnDcWUZBhwIpAxENvwFQuGo09BQC4GXmreDpLVWGgNFPGAhlxKWK+qNQ54ZAQUTwGLD'
        'lfUqpFxaKezyHBNSSZBCnnJHGyHpAy+fMOlaTHpWpJ+tQel9+5uj0rFz9LYVy70i+4Olab8lSzpYWDpftAHGsExY'
        '6TJCvcg01iRjlEhhOCeeowRLJ1j67gvIrSdyncXvs4R86BdSEflaReS95i8rI/fiPhvKyLHBXZ9l5LKOWhSSS5O5'
        'RSm5RngqJidwOoHTtwGnaxZg2qDTTd3eFzzdY+5/lew/wdPd4OleAoBLlwaigth6fBpiG67UYmpJCBAwJgIJjQS4'
        'Kj491MWICIA6rUYkgDrlvyn/Tflvyn9T/ts/TN3G9iWY+r5g6m5jlmDqfsx1LUy9M1/xOHXbaOQcTt2U0QwDp75Y'
        'UwmnTjh1wqkTTp1w6tvh1KwWp36dqO9f5+On5wF9zKjmnhLJ+Rax6jSQw0Gr01i+K7x6P9yZno/tk/vR3xirnV8V'
        '7NpKIyznAgFgKVROEmgF05pJ5RGmmBgLjfD8BHaNzmLX3mPDidaUh2zXKcIBwBADj6UgNmSBIROUCGOAEVXcqJAh'
        'Kq6N5FgZI41n7bBrQdYfVJeOrfd5QBgiQIQh3mHgsFMShQwbQAAtQIxAhEFIuq3lShOLNYZN2HVdNDBI9Bp8FJHs'
        '9V5V7fnrgz43Z7BjJ+xx2bPtM9ZD2fNAmf1x2KzfyicbLIetKfZQe59BYnBGEAWZ0kZlwc4IAaVCzIDEYScO++7r'
        '0K0n8inrf8K5XFKM3gU5D8cRTapMH+u2vjTd3v+cLU0fRbCnqtTodJW6FwfbUKWOjQV7rFIfzNY2lep6DbcoWp8a'
        'mrr69UUzorZ+fZGEpvo1aq5fo8RvJ377bfLbTc7rYpi7aengFNfdIbc/w3Wj81x315OeOdcPX1y4yvLCm+e6d5HG'
        '6cWDY7L7Il9xguzuJWjosugQFRrX091KOO8sgcAJJj0GilPDoZNXpbuHvMwRQXindY5EeKfMOmXWKbNOmXXKrO+H'
        'DG9jMxMZfl9keLcxu0cyfOsGmtDwS57bs2h4H5a8Dg2vms14PLxtKHMOD29KiYaBh1+sqYSHJzw84eEJD094+O3w'
        'cF6Lh+dnKXF3bx4NP7ifRKC+RSw8DeIwkPA0ju8GBz8a6j53264TfsXdto3nAjIL84VCp4UnyGpEBXfOQ+OQwlBB'
        'QqEQgktKKIHECwKcCqkuN9Sqdth3yI6Zhl4oHxJkpzijEnigHBXMe5H/l1iDOfAUM+Oc1U5pYSUmJFwVQa4J+z70'
        '8mm37Triu9BSe9q71P7mpHfsHL3pBlclRfZHefN+q598sJS3pGGUCTEZ5t5mhGmaSQpo5iU3RBlINJCJ8k6U993X'
        'oltP5DqL3+NuY0d+Ie02dqXdxkqav2y3sV7cZ0O1OTa467HaXNHR+UpzeTKfLy/XCU+7jSVaO9HaN9ltu24BpgWg'
        '3djtfe223WPuf5XsP+223W237V4CgEuXBqKC2BO7bZMQIwgumAXCEMQd8Jb5fLvSK/LYQ12MiGCx02pEYrFT/pvy'
        '35T/pvw35b/9M9VtbF9iqu+Lqe42Zmm37X7MdR1SvTdf8Th122jk7G7bDRnNMHDqizWVcOqEUyecOuHUCae+HU4t'
        '6nHqkBiUuL63j1NX7ycRnG8Sp06DOAycOo3ju8GpD4e6V5y6RvgVcWoccjkOAfOMGqSMpp4aRrTTjjhqHIOeO47D'
        'RQDquZXEovxVYWskxAJj79rh1I4DIai3iipiHAUGAyQkREg6BIjWPvwNEGdw+AtnHDgqQ0bNpJdaaAFsI0594OUT'
        'Tl2LU6+1dAFOvW9/c5w6do7etoK5V2R/OLXot4ApBotTQx+G0mGREcpURqCCmWaEZQgChJ0WyFidcOqEU999Obn1'
        'RK6z+H2Wkw/9QionX6ucvNf8ZeXkXtxnQzk5Nrjrs5xc1lGLcnJpMrcoJ9cIT+XkhFMnnPo2OHXNAkwbnLqp2/vC'
        'qXvM/a+S/SecuhtO3UsAcOnSQFQQW49TGwk4BpA6xWhQCFDOQSUEvipOPdTFiAicOq1GJJw65b8p/035b8p/U/7b'
        'P07dxvYlnPq+cOpuY5Zw6n7MdS1OvTNf8Th122jkHE7dlNEMA6e+WFMJp044dcKpE06dcOrb4dSyHqdevSodwrkB'
        'AdWHd5QozjeJVKdhHApUnUbyHWHVx4PdK1hdK/6KaDVhgAJBvJUeec6xEU55hTmBGkPPsPQhtVWCc4gRsEJxJWE4'
        'QB20GgJM2qHVIQ9mHDlhLIFQw3ASIDhSITf2QGkjqZeGCQ+AEABZglTIuCBTTEjrnIawEa0+8vgJrq6Fqzd6ugCv'
        'Lve4OWAdO1NvW9Msq7I/xFr2W9SUw0WsoVVa+vVXx0xGGGKZlFhkHpr8W3UIcw4SYp0Q6/svMbedyPV2v88i87F3'
        'SGXma5WZy7q/rNDcixttKDTHhnp9FpqrWmpRaq5M6RbF5toTpHJzwq0Tbn0b3Lp2aaYNcN3c8X0h1z2uCVxlVSAh'
        '192Q615CgcuXC6LC2nro2ntOBPaMOOGcWG8ECgUT7KrQ9XAXKCKw67RCkbDrlBOnnDjlxCknTjnxD0awW9jBhGDf'
        'GYLdacwSgt2P0a5FsEsGrAcIu2Vscg7CbspwBgJhX6qpBGEnCDtB2AnCThD2zSBsBGoh7DDJjJsMB8E+uJ/Ee75F'
        'ADsN4jDw6zSO7wa+PhrqPtHrOuFXBK9tSF6lQ9gZY/KPEwknkHYYU4gcohY4RxG2DHoEvSLIe+iAp9oQrSwz0rQD'
        'rxH2PFynoCE/JirkvZJgyIXVgCrCPVIeh0yZQqEwJoRIE84ppIaSMuRCytkEXh96+YRd12HXhZbaQ9el9jdHrmPn'
        '6E0rmiVF9gZcI9BrOfNQ3ICAa0CZABTgjCHjM4IgySRnLhNUSQhCKoZVAq4TcH3/xeXWE7nO4vdYWj7yC6mwfKXC'
        'cknzl5WVe3GfDWXl2OCux7JyRUfni8rlyXy+pFwnPBWUE2SdIOubQNZ1CzAtEOvGbu8LsO4x979K9p8A626AdS8B'
        'wKVLA1FBbD1erUW4Y6Ap9E4xLzyG2lgB9FXx6qEuRnSHq9NqRIKrU/6b8t+U/6b8N+W/PwCobmP7ElB9X0B1tzFL'
        'QHU/5roOqN6br3icum00cg6nbspohoFTX6yphFMnnDrh1AmnTjj17XBqWL+ndUiBwt8Gg1M/jZfPK/1zcVvZeFZI'
        'yHLS4fN4+Qt2wTdBgr2wmEDsleJcAmWARogTH34UKsQ2ICGgb5rHTrMgAd2FEa2atzSObxLornjbfVm576f8epTx'
        'Fs/9qUz5rsPV6nJ3DhXkt/QSJIaY8CnTIbHOXud59mg2Va0T5O850XmfPEjKlu7bMgsn2uUPfclcVBfDu4vdDHB/'
        '974RmE/tbIeEZIvV4tVNmzexbin5Zfw07+u2F2HcQ6K7TsjihS2DIZllq9dwfTbuTt0sU+H/HUflFNL+1/V1XrCP'
        '+L79zZF2A5ALBgYJwqT33DCuDNRWOSS0c4wZ4pEASoU/uQfAEUGBtIAipGn+rbfjKnJbE9bHJl17RfaHtMN+i8hw'
        'uHuIG0kZoSRDmuGMKMwzpSTIIGUgzBEGtHQJaU9I+/3vl9Z2Ih94pqOSPvwIjkv6J0K9VLk/Ulp95b69Pzldee/F'
        'yzVU3jGABECmheWCM64Z54oqoIFxgBqAEQNGKUidMYp5j5zXzngujRdSMXbZhma3gaG5iKehfzpssI7RFg+1wfLD'
        '3Hk3z8vgi4fg1FcTNf/4YrvLCK4xhJJqHhQcJWe6+rZsIaAatZYF5O/szoMdCAlzeGJcLith4veKib9JwHdvUg8A'
        '3/0Px4DvRXb2BODbi529NFGKcun1gC8O5lubcM8+mG8IBA/SjKD2qoDvUFOzCMA35WYJ8E3ZQMoGUjZwxWyg3V67'
        'LZ6ahIbe2V67ncbs/tDQnc1oQkMvMRxn0dA+LEjtXrs7T9LDTrst/dg5NLQpFh7ITruXaiqhoQkNTWhoQkMTGno7'
        'NBTVo6Fzl7lvSzdd72MwNEL0yb2Mp+MsXOf+JhcP4Z5/CREN0JDIEM4AJp1HAAlHkMZa+hDykGAdJaaOJapsCIxo'
        'mgeJEq23dGk47wQWrfjKOFa0x6f9erToi3nNQriyeo3h9Z6CkNeJ+q5ns8+LWEG9XM1iMsuC0w+jXRQ9IqS58DiM'
        'TfYyXo6fthlKd3Hj6Re32ErKwrXNv7/OxnGX+BLiouVsnjO8T3P1+hzJ7i6WL6Gfe8nU01MIFFUQ3ZPAtS6j5QWD'
        'uo7sFs9Zseo9j4NXvctyI70fmebrO2BEy+b9PlBRThA1VHpKgRMQYi6phkgQYJGDWmHnsaCCWcuFZGi9zYTCHNBg'
        'gDzipOZ7rm3tVx/1yCN99keMon6rkmiwxCiRgjFHXCYk9hkxkmYSYphBqC320uQjnojRRIzefY249UTeu4Mj+3O4'
        'exHCh7Xik4FfqhYf662+XNzew5wuF/fi9xrKxdwirYgjDGDrIHeACh2uShFuw/VJgYFkEECCAZHQesU49gwRFppa'
        'Fy7lDcCjEp+HRxP8mODHvnY33diEmu1NET6BP15kKU7gj71Yio45QJSbqqcglRXSCqS5QwR7Lixn1lALrkpBDjzr'
        'iIAhU9qRYMgU6KZANwW6d8xFtnmAEhd5X1xktzG7zy0zET4HRl5iOc6CkX2YkFow8tCrxPORbV3bOT6yKUoeBh95'
        'saYSH5n4yMRHJj4y8ZG34yNxLR85zlcfQu6T6dV4YkswzFAQye0NPmxuMCveyv8FGWM95zikHCBEHZZYDxElCCnA'
        'uHeCOJYnJTaRVAMmqXqbHCWOauNDsyIDuQCegqe9fIMHLg/1z6NgKvxk/PS8HB0+2A9bTf08suVK9wY+WYzyLHFf'
        '3C5Vr/OS9b6eUg0H3moZ90fXt8C1y1t1haU3Wd+q/n4nBa7aBY9iatf9VLea0GYtp2Ht5bZrOXWNOqzlnFmi+ZGL'
        'OV2WZU5kHa526AeQ3VQbpPQmpTfN6c2Bqb5xfkNq85un2exp4jIzma1stljO5iFeGlyO85/rm/xDfo9/CWYvD7sf'
        '6m58883AXzQmzGHJpPAhzrWGQ2gUdg5TR6ARVisFlUI65T4Dzn1++KS5v5yo7vZSXpTyopQXpbwo5UUpL0p5UcqL'
        'BpcX0dq8yExCaOxCMJunNjmWluXOb4D1n/+d/Xv+PZTs9398OHXPv1AktfQcK000ERJRgDgkVABguSbSSWby/a5M'
        'yoeGsFlGmhFp24z9sbOGMI3s8NY+fogJuN5eGmd3LrA+auMD5+ZZzoS6r1FiXpffYvtnefQU4vwYOXkUpebmOd+z'
        'Yv3aQMQeDuZ5NlHzzIVQbxX/bbT9nDNhzuXxVLZ8DjnAduuFeLn5XI6UFjIptVzvDfE6UXF7knxx05XbfeciStS3'
        'yeJb+80zTj3k97GPhrYceAGlQE4pIQVRznEssKAEOYsMlMApjr0EQnJuEQSYCe88R5wabNTxG21tbVcPb7Q1qra/'
        'LTVov++20cFuqQGIYkArmyHlVUagYJlU2GSUCEWp85YRmrbUSFtq3P2bhq0n8v5Ds02mqPzyHPqI0PGXGD5ndh0U'
        'qvFDC3/xvl8/LGmw/vXD9h7o9OuHvfjFhtcPgwAKDFBAs9DFe6uIF5JqDbl04TqJNAZ5Sy1FnHBjrAGYWMbC+b1z'
        'mLyBfTaISBttpI02rrTRRskoHGy0UfrleKONi0zFiY02ejEV8flClB+r33MDSqRZbnWQAhAJZgGBDHFx1T033k+G'
        'ErH9RkpR0vYbKShOQXEKit/anhxtnqq0J8d97cnRbczub0+OvdFo2pPjEstxdk+OPkxI3Z4cTQ4ifnuOtq7v3PYc'
        'TQH1MLbnuFhTaXuOtD1H2p4jcZppe47bYZqsFtMM9i5f0lhkM58tlt8nw3t3babn6iGkb9nRrf4CqDfEA4u05Job'
        'QJC1UhKpLKOCMuGIghgRl1CtIWCZaSIkGrPB3qUhHR6G2ecjfz368geW1A7KZrvvmH/YV2T3vNvfzyByG/Yvy9c0'
        '5pPvWbi2zMymZhyyqO/t2bmjsbkPaA4jb4R0IYtHAGgHBMKUW2QshTDfYFQpyYnmknEhMDWWAcGpdcJrraik4rgk'
        '1XbK9VCSqtdpf7Qc67cUxQZLyymPqZZeZcZBlxFNUCal8pmkxAkhEVEMJVou0XJ3XxhsPZF37qHeBtW7n1JB8LTP'
        'TnXAI73VlwHbO5rTZcBe3F9DGRACiT0gLp88zlnNvIUAA2W4t1xxhb2igjmtIA0HicTKh7vxwBtoNLX4DbBx6DwZ'
        'tz/Bem+Qh2J7yIfGyOrh6MH4+GJPVqISapdQuw6o3d7EnEwZjkG7i+zOCdCuF7sTkVxE+b56ws5hKIVBjGlvjTdW'
        'cOWDNvhVCbt3kM5EoHUpn0loXYqgUwSdIuj7BunaPEMJpLsvkK7bmN0fSLezGU0c3SWG4yxH14cFqePoav1CPEDX'
        '1sGdA+ia4uVhAHQXayoBdAmgSwBdAugSQHc7gI7X73MYDGqIgtRy82mfwfFzuxt8OLzVXwSjyEOnCLQQOQqogAhb'
        'DR1xTCvtvDQSCg8TYzNgxqbP+XF/O7of3lPazT3t5p52c0+7uafd3NNu7mk395TkDG43d1Gb5azG//yny1bjbDGZ'
        'vQ4uxVnf3cP6v794QwB00jFKjFbeEcQ1MFJAki/GamMsMMTx9N7AkHOaqAlxf0lM5elNGUzKYFIGkzKYlMGkDCZl'
        'MCmDGVwGI2szmLkz41c31CJNLkItZ1mI6BYPlVv9RXkFHUdSAYgBg4ZrjATi0CtrrEcEAAmNT18aGsYmB2kipE0O'
        'Tti6NJzDW6jo83Ef4gYH4COM2OBALdRUZYVaoz4mlF/tMg/V+hD29BLU2Ieg/x7Pe7m7ECq6RXhUTC+3t5go87kX'
        'Qct5/jj0IGn7lLUQVd3aovJE3se2FpA75bVFlBihHIKMWO4wgMA56fIlTYwNBVpaL51GijkJBEKeU+SYtlgevwfW'
        '1tD08B7YsT7729JC9vsKmBzslhbMQAkJsZkATmaEI5UpYEgmNEWKcIGlVmlLi7Slxd2/kNd6Ipe+9Hhof+qDjdLL'
        'eKcjtPQy3pHe6l/Ga+9gTr+M14vba3gZT4XrYwRSJAkSmCoohGUWUEygtFRCCCii2jArrQAeM0I0txBL7hHwxL6F'
        'Tz1Jmr70lLafuM72E3uTcDKhO95+4iI7cWL7iV7sRMcEIMpH1W89QTxhiBPv87fpjOMeGGaQ8FfdemLgKUfEthMp'
        '50jbTqQoN0W5Kcq93y0n2jw/acuJ+9pyotuY3d+WEzub0bTlxCWG4+yWE31YkLotJ458Qvx2E20d27ntJppi5GFs'
        'N3GxptJ2E2m7ibTdROIY03YT/WGMv9uEGq2wwg85BfOY5zDh9x3x+H35PJuGHMurYBaKdhssY2NzPtQsG+5aoPX6'
        '7YftNH2cuqftGm04FpxVbnV/99vv/n9BpWxr'
    )

    def original(self):
        import base64
        import hashlib
        import zlib
        raw = zlib.decompress(base64.b64decode(self.ORIGINAL))
        self.assertEqual(hashlib.sha256(raw).hexdigest(),
                         '77db117e6ee9a712f6c755a9fae4d609bcd85f122f2f6bb871fa18d1f7b50ee5')
        return json.loads(raw)

    def test_original_160_calls_require_25_prepared_and_five_exact_refusals(self):
        import copy
        audit = load_audit()
        original = self.original()
        retained = copy.deepcopy(original)
        fixtures = json.loads(audit.FIXTURES.read_text())
        self.assertEqual(sum(len(row['windsurf_public_commands'])
                             for row in original['results']), 160)
        result = audit.prepared_installation_summary(
            original, fixtures, 'windsurf-prepared-installation')
        self.assertEqual((result['installation_passed'],
                          result['installation_prepared_lifecycle_passed'],
                          result['installation_verified_negative_coverage'],
                          result['installation_acceptance_passed']), (25, 25, 5, True))
        self.assertEqual(original, retained)
        self.assertEqual((original['passed'], original['prepared_lifecycle_passed'],
                          original['verified_negative_coverage']), (0, 25, 1))


    def test_original_provenance_effects_and_nullable_refusals_are_required(self):
        import copy
        audit = load_audit()
        original = self.original()
        fixtures = json.loads(audit.FIXTURES.read_text())
        mode = 'windsurf-prepared-installation'

        def refused(report, corpus=fixtures):
            self.assertFalse(audit.prepared_installation_summary(
                report, corpus, mode)['installation_acceptance_passed'])

        for phase in ('install', 'list', 'repeat_add', 'repeat_list', 'cleanup', 'post_remove_list'):
            for key, value in (('exit_code', 1), ('exit_code', False), ('timeout', True)):
                with self.subTest(phase=phase, key=key):
                    changed = copy.deepcopy(original)
                    changed['results'][0][phase][key] = value
                    refused(changed)
        for key, value in (('prepared_components_only', False), ('repeat_verified', False),
                           ('repeat_native_unchanged', False), ('repeat_lock_unchanged', False),
                           ('matched_installation_ids', ['substitute']),
                           ('repeat_installation_ids', ['substitute']), ('sentinels_preserved', False),
                           ('post_removal_leftovers', ['foreign']), ('native_residuals', ['owned']),
                           ('post_remove_identity_registered', True), ('signed_release_observation', [])):
            with self.subTest(positive_field=key):
                changed = copy.deepcopy(original); changed['results'][0][key] = value
                refused(changed)
        for field, value in (('scope', 'project'), ('activation', 'active'),
                             ('verification', 'package_validated'), ('authentication', 'authenticated')):
            changed = copy.deepcopy(original)
            doc = json.loads(changed['results'][0]['list']['stdout_tail'])
            doc['data']['installations'][0]['clients'][0][field] = value
            changed['results'][0]['list']['stdout_tail'] = json.dumps(doc)
            refused(changed)
        for phase in ('list', 'repeat_list', 'post_remove_list'):
            changed = copy.deepcopy(original)
            changed['results'][0][phase]['stdout_tail'] = '{"schema_version":1,"schema_version":1}'
            refused(changed)
        changed = copy.deepcopy(original)
        changed['results'][0]['windsurf_public_commands'][4].append('--external-uninstalled')
        refused(changed)
        for index in (13, 23, 24, 27, 28):
            for key, value in (('source', 'foreign'), ('identity', 'foreign'), ('index', False),
                               ('windsurf_native_image_unchanged', False), ('native_file_delta', False),
                               ('failed_install_registered', True), ('failed_install_residue', True),
                               ('sentinels_preserved', False), ('native_residuals', ['owned'])):
                with self.subTest(index=index, field=key):
                    changed = copy.deepcopy(original); changed['results'][index][key] = value
                    refused(changed)
            for phase in ('repeat_add', 'repeat_list', 'cleanup', 'post_remove_list'):
                changed = copy.deepcopy(original); changed['results'][index].pop(phase)
                refused(changed)
            for phase, key, value in (('install', 'stderr_tail', 'another refusal'),
                                      ('install', 'exit_code', True), ('install', 'timeout', True),
                                      ('install', 'stdout_tail', '{}'),
                                      ('list', 'stdout_tail', '{}')):
                changed = copy.deepcopy(original); changed['results'][index][phase][key] = value
                refused(changed)
            changed = copy.deepcopy(original)
            changed['results'][index]['windsurf_public_commands'][0][3] = 'foreign'
            refused(changed)
        for index in (23, 24, 27, 28):
            changed = copy.deepcopy(original); corpus = copy.deepcopy(fixtures)
            alias = changed['results'][index]['source'].rsplit('@', 1)[0] + '@' + '0' * 40
            changed['results'][index]['source'] = alias
            changed['results'][index]['windsurf_public_commands'][0][2] = alias
            corpus['plugin_aliases'][index] = alias
            corpus['plugin_declared_names'][alias] = changed['results'][index]['identity']
            refused(changed, corpus)
            for key in ('native_uninstall', 'windsurf_expected_release_pin'):
                changed = copy.deepcopy(original); changed['results'][index].pop(key)
                refused(changed)
        for key, value in (('architecture', 'aarch64'), ('canonical_client_id', 'other'),
                           ('full_coverage', False), ('cases_attempted', 29)):
            changed = copy.deepcopy(original); changed[key] = value; refused(changed)
        changed = copy.deepcopy(original)
        changed['client_version']['value'] = 'devin 3000.11.205'
        refused(changed)
        changed = copy.deepcopy(original); changed['results'].pop(); refused(changed)

    def invoke(self, original, acceptance=None, *, target='windsurf', kind='plugin',
               system='linux', machine='x86_64', limit=30):
        import copy
        from contextlib import redirect_stdout, redirect_stderr
        from io import StringIO
        audit = load_audit()
        cli = str(Path(sys.executable).resolve())
        rows = copy.deepcopy(original['results'])
        # The actual receipt is tested byte-exact above. This orchestration
        # control rebinds only the inert executable path to its owned test call.
        for row in rows:
            for command in row['windsurf_public_commands']:
                command[0] = cli
        with TemporaryDirectory() as temporary:
            argv = ['live-install-audit', '--cli', cli, '--target', target, '--kind', kind,
                    '--limit', str(limit), '--report-dir', temporary]
            if acceptance:
                argv += ['--acceptance', acceptance]
            with (patch.object(audit.sys, 'argv', argv), patch.object(audit.sys, 'platform', system),
                  patch.object(audit.platform, 'machine', return_value=machine),
                  patch.object(audit, 'vendor_bootstrap_attested', return_value=True),
                  patch.dict(audit.AGENT_BINARIES, {'devin': cli, 'windsurf': cli}, clear=True),
                  patch.object(audit, 'version_evidence', return_value=original['client_version']) as version,
                  patch.object(audit, 'do_case', side_effect=rows) as cases,
                  redirect_stdout(StringIO()), redirect_stderr(StringIO())):
                try:
                    code = audit.main()
                except SystemExit as error:
                    code = error.code
            result = Path(temporary) / f'{target}-{kind}.json'
            return code, json.loads(result.read_text()) if result.exists() else None, version.call_count, cases.call_count

    def test_actual_footer_separates_installation_from_default_native_and_scope(self):
        original = self.original()
        code, default, versions, calls = self.invoke(original)
        self.assertEqual((code, versions, calls, default['passed'],
                          default['prepared_lifecycle_passed'], default['verified_negative_coverage']),
                         (1, 2, 30, 0, 25, 1))
        self.assertNotIn('installation_acceptance_passed', default)
        code, installed, versions, calls = self.invoke(original, 'windsurf-prepared-installation')
        self.assertEqual((code, versions, calls, installed['passed'],
                          installed['installation_passed'], installed['installation_verified_negative_coverage']),
                         (0, 2, 30, 0, 25, 5))
        self.assertEqual(installed['results'], default['results'])
        self.assertEqual(installed['verified_negative_coverage'], 1)
        for changed in ({'target': 'cursor'}, {'target': 'windsurf,cursor'}, {'kind': 'skill'},
                        {'system': 'darwin'}, {'system': 'win32'}, {'machine': 'aarch64'}, {'limit': 29}):
            with self.subTest(changed=changed):
                self.assertEqual(self.invoke(original, 'windsurf-prepared-installation', **changed),
                                 (2, None, 0, 0))


class CodexResidualDiagnosticsTest(unittest.TestCase):
    def test_diagnostics_preserve_negative_scanner_and_bound_output(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / '.codex'
            root.mkdir()
            wal = root / 'state_5.sqlite-wal'
            wal.write_bytes(bytes.fromhex('377f0682') + (3007000).to_bytes(4, 'big')
                            + (4096).to_bytes(4, 'big') + b'x' * 20 + b'x' * (1 << 20))
            paths = ['.codex/state_5.sqlite-wal']
            for index in range(25):
                relative = f'.tmp/plugins/plugins/foreign-{index}/SKILL.md'
                path = root / relative
                path.parent.mkdir(parents=True)
                path.write_text('"owned-test"')
                paths.append('.codex/' + relative)
            data = 'plugins/data/agent-plugins/' + 'a' * 64 + '/npm-runtime/package.json'
            (root / data).parent.mkdir(parents=True)
            (root / data).write_text('"owned-test"')
            paths.append('.codex/' + data)
            original = audit.native_identity_residuals([root], 'owned-test', paths, 'codex')
            diagnostic = audit.codex_residual_diagnostics([root], original)
            self.assertEqual(original, audit.native_identity_residuals([root], 'owned-test', paths, 'codex'))
            self.assertEqual(diagnostic['total'], 27)
            self.assertEqual(len(diagnostic['entries']), 16)
            self.assertEqual(diagnostic['omitted'], 11)
            self.assertLessEqual(len(json.dumps(diagnostic).encode()), 8192)
            entries = {entry['class']: entry for entry in diagnostic['entries']}
            self.assertEqual(entries['wal']['wal_page_size'], 4096)
            self.assertEqual(entries['wal']['wal_header_status'], 'recognized-magic')
            self.assertEqual(entries['plugin-data']['data_reference_status'], 'not-supplied-to-diagnostic')
            self.assertNotIn('path', entries['wal'])
            with patch.object(audit, 'codex_residual_file_metadata', side_effect=OSError('private')):
                self.assertEqual(audit.codex_residual_diagnostics([root], original),
                                 {'diagnostic_status': 'unavailable'})
            self.assertEqual(original, audit.native_identity_residuals([root], 'owned-test', paths, 'codex'))

    def test_metadata_bounds_and_close_error_are_contained(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / '.codex'
            root.mkdir()
            with patch.object(audit.os, 'open', side_effect=AssertionError('must not open')):
                for relative in ('x' * 2049, '/'.join(['x'] * 17)):
                    self.assertEqual(audit.codex_residual_file_metadata(root, relative, True),
                                     {'metadata_status': 'outside-bounds'})
            wal = root / 'state_5.sqlite-wal'
            wal.write_bytes(b'x' * 32)
            close = audit.os.close
            def close_then_fail(fd):
                close(fd)
                raise OSError('close error')
            with patch.object(audit.os, 'close', side_effect=close_then_fail):
                result = audit.codex_residual_file_metadata(root, wal.name, True)
            self.assertEqual(result['metadata_status'], 'observed')
            self.assertEqual(result['wal_header_status'], 'unrecognized-magic')

    def test_wal_metadata_refuses_symlinks_and_identity_changes(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / '.codex'
            root.mkdir()
            real = root / 'real'
            real.write_bytes(b'x' * 32)
            link = root / 'state_5.sqlite-wal'
            link.symlink_to(real)
            with patch.object(audit.os, 'read', side_effect=AssertionError('must not read')):
                self.assertFalse(audit.codex_residual_file_metadata(root, link.name, True)['ordinary_file'])
            link.unlink()
            link.write_bytes(b'x' * 32)
            original_read = audit.os.read
            def replace_after_read(fd, size):
                result = original_read(fd, size)
                link.unlink()
                link.write_bytes(b'y' * 32)
                return result
            with patch.object(audit.os, 'read', side_effect=replace_after_read):
                self.assertEqual(audit.codex_residual_file_metadata(root, link.name, True),
                                 {'metadata_status': 'unstable'})
            parent = root / 'redirect'
            parent.symlink_to(root, target_is_directory=True)
            self.assertEqual(audit.codex_residual_file_metadata(root, 'redirect/real', True),
                             {'metadata_status': 'unavailable'})


class ClaudeRetirementDiagnosticContracts(unittest.TestCase):
    def test_darwin_presence_refusal_projects_only_authored_boolean_facts(self):
        darwin = runpy.run_path(str(ROOT / "scripts/claude_darwin_metadata.py"))
        projection = runpy.run_path(str(ROOT / "scripts/claude_cache_retirement_audit.py"))
        diagnose = projection["retirement_diagnostic"]
        metadata = projection["metadata"]
        diagnose.__globals__["_DARWIN"] = darwin
        cls = darwin["Metadata"]
        # All-absent extended security is now a legitimate no-ACL header.
        # This diagnostic must exercise only refused partial/ACL headers.
        for presence in ((True, False, False), (True, True, True)):
            with self.subTest(presence=presence):
                native = object.__new__(cls)
                released = []
                native.init = lambda: 1
                native.free = released.append
                native.stat = lambda *_args: 0
                def query(_section, key, output):
                    output._obj.value = dict(zip((3, 6, 5), presence))[key]
                    return 0
                native.query = query
                darwin["check"].__globals__["_METADATA"] = native
                with (patch.object(os, "fstat", return_value=object()),
                      patch("builtins.print") as printed):
                    try:
                        metadata(123)
                    except ValueError as error:
                        self.assertIs(type(error), darwin["PresenceRefusal"])
                        self.assertEqual(str(error), "Darwin cache metadata authority is unresolved")
                        facts = diagnose(error)
                    else:
                        self.fail("presence mismatch was admitted")
                self.assertEqual(released, [1])
                printed.assert_called_once()
                args, kwargs = printed.call_args
                self.assertEqual(kwargs, {"file": sys.stderr, "flush": True})
                self.assertEqual(len(args), 1)
                prefix = "DARWIN_CACHE_PRESENCE="
                self.assertTrue(args[0].startswith(prefix))
                self.assertEqual(json.loads(args[0][len(prefix):]),
                    dict(zip(("uuid_present", "group_uuid_present", "acl_present"), presence)))
                self.assertLess(len(args[0].encode("utf-8")), 128)
                for failure in (RuntimeError("private-print-failure"), KeyboardInterrupt()):
                    with (patch.object(os, "fstat", return_value=object()),
                          patch("builtins.print", side_effect=failure)):
                        if isinstance(failure, Exception):
                            with self.assertRaisesRegex(darwin["PresenceRefusal"],
                                    "^Darwin cache metadata authority is unresolved$"):
                                metadata(123)
                        else:
                            with self.assertRaises(KeyboardInterrupt):
                                metadata(123)
                self.assertEqual(released, [1, 1, 1])
                self.assertEqual({key: facts[key] for key in
                    ("uuid_present", "group_uuid_present", "acl_present")},
                    dict(zip(("uuid_present", "group_uuid_present", "acl_present"), presence)))
                self.assertEqual(facts["module"], "claude-cache-retirement")
                self.assertIs(type(facts["line"]), int)
                self.assertNotIn(str(ROOT), json.dumps(facts))
        # A same-class callback refusal must not inherit an ancestor check's authority.
        native = object.__new__(cls)
        released = []
        native.init = lambda: 1
        native.free = released.append
        native.stat = lambda *_args: 0
        def foreign_query(*_args):
            raise darwin["PresenceRefusal"]((False, False, False))
        native.query = foreign_query
        darwin["check"].__globals__["_METADATA"] = native
        with patch.object(os, "fstat", return_value=object()):
            try:
                metadata(123)
            except ValueError as error:
                self.assertIsNone(darwin["presence_diagnostic"](error))
                self.assertFalse(any(key in diagnose(error) for key in
                    ("uuid_present", "group_uuid_present", "acl_present")))
            else:
                self.fail("callback refusal was swallowed")
        self.assertEqual(released, [1])
        # Matching class/attributes outside the authored check cannot export facts.
        for error in (darwin["PresenceRefusal"]((False, False, False)),
                      ValueError("private-path-uuid-acl")):
            error.presence = (False, False, False)
            try:
                raise error
            except ValueError as caught:
                self.assertEqual(diagnose(caught), {"site": "unavailable"})
        with patch.dict(darwin, {"presence_diagnostic": lambda _error:
                                (_ for _ in ()).throw(RuntimeError("private"))}):
            self.assertEqual(diagnose(ValueError("private")), {"site": "unavailable"})
        with patch.dict(darwin, {"presence_diagnostic": lambda _error:
                                (_ for _ in ()).throw(KeyboardInterrupt())}):
            with self.assertRaises(KeyboardInterrupt):
                diagnose(ValueError("private"))

    def test_authored_site_is_bounded_private_and_unavailable_on_unknown(self):
        module = runpy.run_path(str(ROOT / "scripts/claude_cache_retirement_audit.py"))
        diagnose = module["retirement_diagnostic"]
        for leaf, label in (
                ("claude_cache_retirement_audit.py", "claude-cache-retirement"),
                ("claude_windows_cache_custody.py", "claude-windows-cache-custody"),
                ("gemini_windows_custody.py", "gemini-windows-custody")):
            with self.subTest(module=label):
                namespace = {}
                # Authored filenames are a diagnostic allowlist, not proof that
                # synthetic test execution is native custody evidence.
                exec(compile("def require():\n    raise ValueError('private-path-and-data')\n"
                             "def caller():\n    require()\n",
                             str(ROOT / "scripts" / leaf), "exec"), namespace)
                try:
                    namespace["caller"]()
                except ValueError as error:
                    diagnostic = diagnose(error)
                self.assertEqual(diagnostic, {"module": label, "line": 4})
                self.assertNotIn("private", json.dumps(diagnostic))
                self.assertNotIn(str(ROOT), json.dumps(diagnostic))
        try:
            raise ValueError("private-unknown")
        except ValueError as error:
            self.assertEqual(diagnose(error), {"site": "unavailable"})
        namespace = {}
        exec(compile("def recurse(n):\n    if n: return recurse(n - 1)\n"
                     "    raise ValueError('private-overflow')\n",
                     str(ROOT / "scripts/claude_cache_retirement_audit.py"), "exec"), namespace)
        try:
            namespace["recurse"](70)
        except ValueError as error:
            self.assertEqual(diagnose(error), {"site": "unavailable"})

        class Broken:
            @property
            def __traceback__(self):
                raise ValueError("private-diagnostic-failure")

        self.assertEqual(diagnose(Broken()), {"site": "unavailable"})

        class Interrupted:
            @property
            def __traceback__(self):
                raise KeyboardInterrupt

        # Manual cancellation is not a diagnostic formatting failure.
        with self.assertRaises(KeyboardInterrupt):
            diagnose(Interrupted())


class ClaudeWindowsStreamParserContracts(unittest.TestCase):
    def module(self):
        return runpy.run_path(str(ROOT / "scripts/claude_windows_cache_custody.py"))

    def test_documented_stream_layout_and_strict_record_bounds(self):
        module = self.module()
        def record(name="::$DATA", size=3, allocated=4096, following=0):
            name = name.encode("utf-16-le")
            return struct.pack("<IIqq", following, len(name), size, allocated) + name
        self.assertEqual(module["parse_streams"](record()), (("::$DATA", 3, 4096),))
        for body in (b"", record()[:25], record(following=25), record(size=-1),
                     record(allocated=-1), record(name="\x00")):
            with self.subTest(body=repr(body)), self.assertRaises((ValueError, UnicodeError)):
                module["parse_streams"](body)


class ClaudeWindowsCacheRetirementContracts(unittest.TestCase):
    __unittest_skip__ = sys.platform != "win32"
    __unittest_skip_why__ = "actual 64-bit Windows NTFS custody contract"

    def module(self):
        return runpy.run_path(str(ROOT / "scripts/claude_windows_cache_custody.py"))

    def fixture(self, root, audit):
        module = self.module()
        if not root.exists():
            module["fresh_case"](root)
        else:
            native = module["Native"](lambda: time.monotonic() + 5)
            try:
                native.hold(root)
                native.acl(native.parents[str(root)][0], True, True)
            finally:
                native.close()
        return ClaudeCacheRetirementAuditContracts.fixture(self, root, audit)

    capture = ClaudeCacheRetirementAuditContracts.capture
    retire = ClaudeCacheRetirementAuditContracts.retire
    residuals = ClaudeCacheRetirementAuditContracts.residuals

    def change_dacl(self, subject, path, broad):
        native, c = subject.windows.native, subject.windows.native.ctypes
        descriptor, dacl = c.c_void_p(), c.c_void_p()
        present, defaulted = c.c_int(), c.c_int()
        get = native.security.GetSecurityDescriptorDacl
        get.restype, get.argtypes = c.c_int, [c.c_void_p, c.POINTER(c.c_int),
                                             c.POINTER(c.c_void_p), c.POINTER(c.c_int)]
        change = native.security.SetSecurityInfo
        change.restype, change.argtypes = c.c_uint32, [c.c_void_p, c.c_int, c.c_uint32,
            c.c_void_p, c.c_void_p, c.c_void_p, c.c_void_p]
        sid = "S-1-1-0" if broad else native.user
        text = "D:P(A;OICI;FA;;;" + sid + ")"
        self.assertTrue(native.security.ConvertStringSecurityDescriptorToSecurityDescriptorW(
            text, 1, c.byref(descriptor), None))
        handle = None
        try:
            self.assertTrue(get(descriptor, c.byref(present), c.byref(dacl), c.byref(defaulted)))
            handle = native._handle(native.kernel.CreateFileW(
                str(path), 0x40000, 7, None, 3, 0x02200000, None), "test ACL mutation")
            self.assertEqual(change(handle, 1, 0x80000004, None, None, dacl, None), 0)
        finally:
            if handle is not None:
                native._close_handle(handle)
            self.assertFalse(native.kernel.LocalFree(descriptor))

    def test_real_windows_exact_held_retirement_and_optional_marker(self):
        audit = load_audit()
        for marker in (False, True):
            with self.subTest(marker=marker), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve() / "owned-case", audit)
                subject = self.capture(audit, f)
                backend = subject.windows
                original = backend.nodes[""][1]
                handles = [handle for handle, _facts in backend.nodes.values()]
                handles += [handle for handle, _facts in backend.native.parents.values()]
                for handle in handles:
                    backend.native._noninherit(handle)
                moved = self.retire(f, marker)
                proof = subject.finish(self.residuals(audit, f), f["cleanup"])
                self.assertEqual(proof["status"], "verified-cache-retirement-at-checkpoint",
                                 proof.get("diagnostic"))
                self.assertEqual(proof["cache_volume_serial"], original.volume)
                self.assertEqual(proof["cache_file_id_128"], original.file_id.hex())
                self.assertEqual(proof["cache_ntfs_selector_64"], original.file_index)
                self.assertEqual(proof["orphaned_marker_present"], marker)
                self.assertFalse(proof["writer_closure_claimed"])
                self.assertTrue(moved.exists())
                self.assertEqual(backend.nodes, {})
                self.assertEqual(backend.native.parents, {})
                flags = backend.native.ctypes.c_uint32()
                for handle in handles:
                    self.assertFalse(backend.native.kernel.GetHandleInformation(
                        handle, backend.native.ctypes.byref(flags)))

    def test_rename_old_then_create_new_reaches_share_delete_and_refuses_replacements(self):
        audit = load_audit()
        for replacement in ("file", "parent", "cache"):
            with self.subTest(replacement=replacement), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                f = self.fixture(root / "owned-case", audit)
                subject = self.capture(audit, f)
                moved = self.retire(f)
                target = (moved / ".agentplugins-runtime/skills/owned-test/SKILL.md"
                          if replacement == "file" else
                          f["cache"].parent if replacement == "parent" else moved)
                # Rename succeeds with our original HANDLEs still held. A fresh
                # pathname with equal bytes must never acquire their identity.
                old = root / "owned-case" / ("displaced-" + replacement)
                body = target.read_bytes() if replacement == "file" else None
                target.rename(old)
                if body is not None:
                    target.write_bytes(body)
                else:
                    target.mkdir()
                proof = subject.finish(self.residuals(audit, f), f["cleanup"])
                self.assertEqual(proof["status"], "unresolved-cache-retirement")
                self.assertEqual(subject.windows.nodes, {})
                self.assertEqual(subject.windows.native.parents, {})

    def test_real_ads_and_acl_mutations_fail_closed_at_checkpoint(self):
        audit = load_audit()
        for mutation in ("file-stream", "directory-stream", "case-dacl", "parent-dacl",
                         "container-dacl", "cache-dacl", "file-dacl", "late-stream"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve() / "owned-case", audit)
                subject = self.capture(audit, f)
                moved = self.retire(f)
                payload = moved / ".agentplugins-runtime/skills/owned-test/SKILL.md"
                if mutation in ("file-stream", "directory-stream"):
                    target = payload if mutation == "file-stream" else moved
                    Path(str(target) + ":unowned").write_bytes(b"foreign stream")
                elif mutation == "late-stream":
                    original = subject.snapshotter
                    def change(roots):
                        result = original(roots)
                        Path(str(payload) + ":late").write_bytes(b"late")
                        return result
                    subject.snapshotter = change
                else:
                    target = {"case-dacl": f["home"].parent, "parent-dacl": f["cache"].parent,
                              "container-dacl": moved.parent, "cache-dacl": moved,
                              "file-dacl": payload}[mutation]
                    self.change_dacl(subject, target, mutation != "file-dacl")
                proof = subject.finish(self.residuals(audit, f), f["cleanup"])
                self.assertEqual(proof["status"], "unresolved-cache-retirement")
                self.assertEqual(subject.windows.nodes, {})

    def test_complete_volume_and_128_bit_identity_are_independent_authorities(self):
        audit = load_audit()
        for mutation in ("volume", "id-high-half", "directory-id"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve() / "owned-case", audit)
                subject = self.capture(audit, f)
                self.retire(f)
                native = subject.windows.native
                original = native._facts
                target = subject.windows.nodes[""][1]
                def changed(handle, directory):
                    facts = original(handle, directory)
                    if facts.file_id == target.file_id:
                        if mutation == "volume":
                            return facts._replace(volume=facts.volume ^ (1 << 40))
                        if mutation == "id-high-half":
                            body = bytearray(facts.file_id); body[15] ^= 1
                            return facts._replace(file_id=bytes(body))
                        return facts._replace(file_index=facts.file_index ^ (1 << 40))
                    return facts
                with patch.object(native, "_facts", side_effect=changed):
                    proof = subject.finish(self.residuals(audit, f), f["cleanup"])
                self.assertEqual(proof["status"], "unresolved-cache-retirement")
                self.assertEqual(subject.windows.nodes, {})

    def test_windows_bounds_hardlinks_and_unknown_metadata_refuse(self):
        audit = load_audit()
        for mutation in ("file", "total", "count", "depth", "hardlink", "git", "lock"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                f = self.fixture(Path(temporary).resolve() / "owned-case", audit)
                if mutation == "hardlink":
                    os.link(f["cache"] / ".agentplugins-runtime/skills/owned-test/SKILL.md",
                            f["cache"] / "linked")
                elif mutation == "git":
                    (f["cache"] / ".git").mkdir()
                elif mutation == "lock":
                    (f["cache"] / ".uap-integrations.lock").write_text("foreign")
                backend = audit.CLAUDE_RETIREMENT_AUDIT["_WINDOWS"]["Backend"]
                changes = ({"MAX_FILE_BYTES": 1} if mutation == "file" else
                           {"MAX_TREE_BYTES": 1} if mutation == "total" else
                           {"MAX_FILES": 1} if mutation == "count" else
                           {"MAX_DEPTH": 1} if mutation == "depth" else {})
                with patch.dict(backend.__init__.__globals__, changes), self.assertRaises(ValueError):
                    self.capture(audit, f)

    def test_existing_case_is_never_repermissioned(self):
        module = self.module()
        with TemporaryDirectory() as temporary:
            existing = Path(temporary).resolve() / "existing"
            existing.mkdir()
            native = module["Native"](lambda: time.monotonic() + 5)
            try:
                native.hold(existing)
                handle, _facts = native.parents[str(existing)]
                original = native.acl(handle)
                with self.assertRaises(ValueError):
                    module["fresh_case"](existing)
                self.assertEqual(native.acl(handle), original)
            finally:
                native.close()

    # The real Windows backend also runs the original common public/verdict gate
    # control. Its fake public command executor does not count as native E2E.
    test_do_case_requires_every_old_gate_and_never_hides_raw_residuals = (
        ClaudeCacheRetirementAuditContracts.test_do_case_requires_every_old_gate_and_never_hides_raw_residuals)


@unittest.skipUnless(os.name == "posix", "Copilot whole Skill descriptor audit")
class CopilotWholeProjectSkillAuditContracts(unittest.TestCase):
    def test_protected_snapshot_failure_retains_bounded_private_delta(self):
        audit = load_audit()
        before = {"../../private-path\n": "private-before", **{f"home/item-{i:02}": "private-before" for i in range(25)}}
        after = {key: "private-after" for key in before}
        with TemporaryDirectory() as directory:
            root = Path(directory)
            with self.assertRaisesRegex(AssertionError, "modified protected state") as caught:
                audit.check_copilot_snapshot_unchanged(root / "home", root, before, after, "test", 2)
        delta = caught.exception.copilot_snapshot_delta
        self.assertEqual(delta["changed_key_count"], 26)
        self.assertEqual(len(delta["items"]), 20)
        self.assertEqual(delta["omitted_key_count"], 6)
        self.assertNotIn("key", delta["items"][0])
        self.assertTrue(all(item["before_present"] and item["after_present"] for item in delta["items"]))
        for item in delta["items"]:
            self.assertRegex(item["before_sha256"], r"^[0-9a-f]{64}$")
        self.assertNotIn("private-", json.dumps(delta))
        self.assertEqual(after, {key: "private-after" for key in before})

    def exercise(self, *, ready=True, fault=None, preinitialized_git=False):
        audit = load_audit()
        body = b"---\nname: frontend-design\ndescription: Inert original Skill\n---\nOriginal body\n"
        item = {"repo": "anthropics/skills", "ref": "a" * 40, "folder": "frontend-design",
                "name": "frontend-design", "source": "https://github.com/anthropics/skills/tree/" + "a" * 40 + "/skills/frontend-design",
                "skill_md_git_blob_sha1": hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()}
        installation_id = "11111111-1111-4111-8111-111111111111"
        calls, probes, objects = [], [], {}
        self.last_calls, self.last_probes = calls, probes
        with TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            if fault == "git-preexisting" or preinitialized_git:
                (root / "skill-00/project/.git").mkdir(parents=True)
            def response(document):
                text = json.dumps(document)
                return {"exit_code": 0, "timeout": False, "stdout_tail": text,
                        "stderr_tail": "", "_json": document}

            def install(env, project):
                self.assertTrue((project / ".git").is_dir(), "ordinary Copilot discovery requires the initialized Project Git root")
                source_key = "github-skill:anthropics/skills:frontend-design:skills/frontend-design"
                source_hash = hashlib.sha256(source_key.encode()).hexdigest()
                declared = "skill-" + source_hash[:56]
                if fault == "coherent-name": declared = "skill-" + "b" * 56
                physical = declared[:51] + "-" + hashlib.sha256(installation_id.encode()).hexdigest()[:12]
                if fault == "coherent-physical": physical = declared[:51] + "-" + "b" * 12
                active = project / ".uap/managed/clients/copilot" / physical
                active.mkdir(parents=True)
                binding_id = "client_" + hashlib.sha256("\0".join((installation_id, "copilot", "project", str(active))).encode()).hexdigest()[:24]
                if fault == "coherent-binding": binding_id = "client_" + "b" * 24
                (active / "plugin.json").write_text(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": declared}))
                skill = active / "skills/frontend-design/SKILL.md"
                skill.parent.mkdir(parents=True); skill.write_bytes(body)
                (skill.parent / "resource.txt").write_text("original resource bytes")
                home = Path(env["HOME"])
                (home / ".copilot/logs").mkdir()
                (home / ".copilot/config.json").write_text(json.dumps({"trustedFolders": [str(project)]}))
                market = "agentplugins-" + hashlib.sha256(physical.encode()).hexdigest()[:12]
                key = declared + "@" + market
                settings = project / ".github/copilot/settings.local.json"
                settings.parent.mkdir(parents=True)
                settings.write_text(json.dumps({"foreign": 42, "extraKnownMarketplaces": {market: {"source": {"source": "directory", "path": active.relative_to(project).as_posix()}}}, "enabledPlugins": {key: True}}))
                tree, manifest, raw, managed = ("sha256:" + value * 64 for value in "cdef")
                revision = {"resolved_revision": item["ref"], "tree_digest": tree, "manifest_digest": manifest,
                    "skill": {"name": item["name"], "requested_path": "skills/frontend-design", "resolved_path": "skills/frontend-design", "raw_tree_digest": raw, "normalizer": "agent-skill-package/v1"}}
                client = {"client_id": "copilot", "scope": "project", "materialization": "materialized",
                    "activation": "active" if ready else "prepared", "authentication": "not_required" if ready else "not_checked",
                    "verification": "installation_verified", "policy": "allowed", "package_revision": {key: revision[key] for key in ("resolved_revision", "tree_digest", "manifest_digest")}, "affected_surfaces": ["copilot"]}
                registration = {"Marketplace": market, "Plugin": declared, "Directory": active.relative_to(project).as_posix()}
                digest = "sha256:" + hashlib.sha256(b"copilot-project-registration-v1\0" + json.dumps(registration, separators=(",", ":")).encode()).hexdigest()
                binding = {**client, "package_revision": revision, "client_binding_id": binding_id, "physical_artifact_id": physical,
                    "target_locator": str(active), "native_profile_root": str(project), "native_objects": [
                        {"object_id": "package:copilot:" + physical, "kind": "managed_package_directory", "logical_name": declared, "path": str(active), "managed_digest": managed, "protection_class": "managed"},
                        {"object_id": "copilot_project_plugin_registration:" + key, "kind": "copilot_project_plugin_registration", "logical_name": key, "path": str(settings), "managed_digest": digest, "protection_class": "managed"}],
                    "receipts": [{"operation_id": "operation-owned", "sequence": 1, "mutation_type": "directory_swap", "client_binding_id": binding_id, "active_path": str(active), "after_digest": managed, "phase": "committed",
                        "environment_authority": {"environment_id": "project-default", "alias": item["name"], "client_id": "copilot", "expected_action": "install", "source_key": source_key}}]}
                source = {"stable_source_key": source_key, "source_binding_id": "src_" + source_hash[:24], "repository": item["repo"], "package_subpath": "skills/frontend-design", "resolved_revision": item["ref"], "tree_digest": tree}
                record = {"installation_id": installation_id, "declared_name": declared, "origin_mode": "direct", "source": source,
                    "package": {"format_id": "agent-plugins/1.0.0", "declared_name": declared, "manifest_digest": manifest, "inventory": {"skills": [item["name"]], "mcp_present": False}}, "clients": {binding_id: binding}}
                state = {"schema_version": 5, "installations": [record], "environment_receipts": [{"receipt_id": "environment-owned", "environment_id": "project-default", "alias": item["name"], "client_id": "copilot", "installation_id": installation_id, "client_binding_id": binding_id, "source_key": source_key, "package_digest": tree, "desired_action": "install", "phase": "committed"}]}
                lock = {"packages": [{"alias": item["name"], "targets": ["copilot"], "tree_digest": tree, "manifest_digest": manifest,
                    "source": {"git": {"kind": "skill", "skill": item["name"], "repository": item["repo"], "commit": item["ref"], "resolved_skill_path": "skills/frontend-design", "raw_tree_digest": raw}}}]}
                (project / "uap.toml").write_text("immutable portable manifest")
                (project / "uap.lock").write_text(json.dumps(lock))
                public = {"installation_id": installation_id, "name": declared, "source": "anthropics/skills/skills/frontend-design", "clients": [client]}
                objects.update(binding_id=binding_id, active=active, skill=skill, settings=settings, key=key, market=market, state=state, public=public, project=project)
                if fault and fault.startswith("private-fact-"): revision["skill"][fault[len("private-fact-"):]] = "foreign"
                elif fault == "missing-private-skill": revision.pop("skill")
                elif fault == "wrong-private-skill": revision["skill"]["normalizer"] = "foreign-normalizer"
                elif fault == "wrong-public-revision": client["package_revision"]["tree_digest"] = "sha256:" + "b" * 64
                elif fault == "private-skill-public": client["package_revision"]["skill"] = revision["skill"]
                elif fault == "iid": state["installations"][0]["installation_id"] = "wrong"
                elif fault == "binding": binding["client_binding_id"] = "wrong"
                elif fault == "physical": binding["physical_artifact_id"] = "wrong"
                elif fault == "source": source["resolved_revision"] = "b" * 40
                elif fault == "receipt": binding["receipts"][0]["after_digest"] = "sha256:" + "0" * 64
                elif fault == "scope": binding["scope"] = "user"
                elif fault == "path": binding["target_locator"] = str(root / "unowned")
                elif fault == "bytes": skill.write_bytes(b"different upstream bytes")
                elif fault == "hardlink": os.link(skill, skill.with_name("linked"))
                elif fault == "symlink":
                    resource = skill.parent / "resource.txt"; saved = skill.parent / "saved.txt"
                    resource.rename(saved); resource.symlink_to(saved)
                elif fault == "state-link":
                    state_path = project / ".uap/state-v2.json"; saved = project / "saved-state.json"
                    saved.write_text(json.dumps(state)); state_path.symlink_to(saved)
                if fault != "state-link": (project / ".uap/state-v2.json").write_text(json.dumps(state))

            def call(argv, env, project, *_args, **_kwargs):
                if argv[0] == audit.GIT_EXECUTABLE:
                    self.assertEqual(argv[1:2], ["-c"])
                    self.assertEqual(argv[-2:], ["init", "--quiet"])
                    self.assertTrue(argv[2].startswith("core.hooksPath=" + str(project.parent)))
                    self.assertEqual(env["GIT_CONFIG_NOSYSTEM"], "1")
                    self.assertTrue(Path(env["GIT_CONFIG_GLOBAL"]).is_relative_to(project.parent))
                    calls.append("git-init")
                    if fault == "git-failure":
                        return {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": "refused"}
                    (project / ".git").mkdir()
                    return response({"result": "success"})
                if argv[1:3] == ["skills", "add"]:
                    calls.append("repeat" if objects else "add")
                    if not objects:
                        install(env, project)
                    elif fault == "repeat-receipt":
                        import copy
                        binding = objects["state"]["installations"][0]["clients"][objects["binding_id"]]
                        receipt = copy.deepcopy(binding["receipts"][0])
                        receipt.update(operation_id="unexpected-second-operation", sequence=2)
                        binding["receipts"].append(receipt)
                        (project / ".uap/state-v2.json").write_text(json.dumps(objects["state"]))
                    elif fault == "ancestor":
                        parent = objects["active"].parent
                        saved = parent.with_name("saved-copilot-parent")
                        parent.rename(saved); parent.mkdir()
                        (saved / objects["active"].name).rename(objects["active"])
                    elif fault in ("process-logs", "foreign-log"):
                        logs = Path(env["HOME"]) / ".copilot/logs"
                        for index in range(2):
                            name = f"process-100-{index}.log" if fault == "process-logs" else f"foreign-{index}.log"
                            (logs / name).write_text("bounded native process log")
                    return response({"schema_version": 1, "scope": "project", "scope_selection_reason": "explicit",
                        "command": "skills add", "result": "success", "readiness": "ready" if ready else "needs_action", "next_actions": [] if ready else [{"code": "verify_native"}]})
                if argv[1] == "list":
                    calls.append("list")
                    return response({"schema_version": 1, "command": "list", "result": "success",
                                     "data": {"installations": [] if objects.get("removed") else [objects["public"]]}})
                self.assertEqual(argv[1], "remove"); self.assertEqual(argv[2], "frontend-design")
                calls.append("remove")
                import shutil
                shutil.rmtree(objects["active"])
                objects["settings"].write_text(json.dumps({"foreign": 42, "extraKnownMarketplaces": {}, "enabledPlugins": {}}))
                (project / "uap.lock").write_text(json.dumps({"packages": []}))
                if fault != "private-remains":
                    objects["state"]["installations"][0]["clients"] = {}
                    objects["state"]["environment_receipts"] = []
                    (project / ".uap/state-v2.json").write_text(json.dumps(objects["state"]))
                objects["removed"] = True
                return response({"result": "success"})

            def probe(argv, project, env, output, label, **kwargs):
                self.assertTrue((project / ".git").is_dir())
                self.assertEqual(kwargs["timeout"], 30); self.assertIs(kwargs["memory_only"], True)
                self.assertTrue(0 < kwargs["limit"] <= 1 << 20)
                self.assertEqual(env["COPILOT_HOME"], str(Path(env["HOME"]) / ".copilot"))
                probes.append(label)
                removed = objects.get("removed", False)
                if argv[1] == "plugin":
                    document = [] if removed else [{"name": objects["public"]["name"], "marketplace": objects["market"], "version": "0.0.0", "enabled": True, "source": "live", "installedFrom": str(objects["active"])}]
                else:
                    document = [] if removed else [{"name": item["name"], "description": "Inert original Skill", "source": "plugin", "path": str(objects["skill"].parent), "enabled": True}]
                if fault == "catalog" and not removed: document = []
                elif fault == "catalog-extra" and document: document[0]["unrecognized"] = True
                elif fault == "catalog-duplicate" and document: document.append(dict(document[0]))
                elif fault == "description" and document and argv[1] == "skill" and "repeat-add" in label:
                    document[0]["description"] = "changed native metadata"
                if fault == "late-mutation" and not removed: (objects["skill"].parent / "resource.txt").write_text("changed")
                if fault == "probe-ancestor" and not removed:
                    parent = objects["active"].parent
                    saved = parent.with_name("saved-native-probe-parent")
                    parent.rename(saved); parent.mkdir()
                    (saved / objects["active"].name).rename(objects["active"])
                stdout, stderr = json.dumps(document).encode(), b""
                if fault == "stderr": stderr = b"unexpected native stderr"
                elif fault == "utf8": stdout = b"\xff"
                elif fault == "budget": stdout = b"[]" + b" " * ((1 << 20) - 2)
                elif fault == "flood": stdout = b"x" * ((1 << 20) + 1)
                record = {"argv": argv, "cwd": str(project), "exit": 0, "error": None, "stdout_bytes": len(stdout), "stderr_bytes": 0, "stdout_sha256": hashlib.sha256(stdout).hexdigest(), "stderr_sha256": hashlib.sha256(stderr).hexdigest()}
                record["stderr_bytes"] = len(stderr)
                if fault == "exit": record["exit"] = 7
                elif fault == "timeout": record.update(exit=None, error="timeout")
                return record, stdout, stderr
            with patch.object(audit, "call", side_effect=call), patch.object(audit, "COPILOT_CATALOG_PROBE", side_effect=probe, create=True), patch.dict(audit.AGENT_BINARIES, {"copilot": "/inert/copilot"}):
                row = audit.do_case("/inert/uap", "copilot", "skill", item, 0, root, {})
            return row, calls, probes

    def test_real_do_case_attributes_whole_skill_and_repeats_removes(self):
        row, calls, probes = self.exercise()
        self.assertEqual(row["status"], "passed", row)
        self.assertRegex(row["skill_path"], r"^\.uap/managed/clients/copilot/skill-[a-f0-9]{45}-[a-f0-9]{12}/skills/frontend-design/SKILL.md$")
        self.assertIn("repeat", calls); self.assertIn("remove", calls)
        self.assertEqual(len(probes), 6)
        self.assertEqual(row["public_skill_ready"], {"after-add": True, "repeat-add": True})
        self.assertTrue(row["repeat_verified"] and row["removal_verified"] and row["sentinels_preserved"])

    def test_retained_private_revision_has_source_defined_public_projection(self):
        # Literal private fields retained by actual Copilot1.0.103 raw-add-proof;
        # expected DTO is source-defined read_public.go, not an observed list
        # for that same IID. Live1.0.105 independently retained the3-key shape.
        private = {"skill": {"name": "benchmark-note", "requested_path": "tests/fixtures/uap_apm/skills/benchmark-note",
            "resolved_path": "tests/fixtures/uap_apm/skills/benchmark-note", "raw_tree_digest": "sha256:218b77ecc0dc92b1c919b3a9df3839c662e0bae6b7f2f221fbcfea342b8d35cb", "normalizer": "agent-skill-package/v1"},
            "resolved_revision": "b5d80336b53397be5bbb72c84f680e5885a6ac0d",
            "tree_digest": "sha256:f008f7b7a0f079504a508fa9e184525c82445300fc7bebb052c2cb8c907fc302",
            "manifest_digest": "sha256:c9bbd0b5bbb39d2ae168390ee53a28d4e825de8c0b5334dbc4a55e300b095ab0"}
        expected = {"resolved_revision": "b5d80336b53397be5bbb72c84f680e5885a6ac0d",
            "tree_digest": "sha256:f008f7b7a0f079504a508fa9e184525c82445300fc7bebb052c2cb8c907fc302",
            "manifest_digest": "sha256:c9bbd0b5bbb39d2ae168390ee53a28d4e825de8c0b5334dbc4a55e300b095ab0"}
        audit = load_audit()
        self.assertEqual(audit.public_skill_revision(private), expected)
        self.assertIn("skill", private)
        for field, value in (("tree_digest", None), ("manifest_digest", False), ("release_sequence", True), ("resolved_revision", 42)):
            with self.subTest(field=field), self.assertRaises(ValueError):
                audit.public_skill_revision({**private, field: value})

    def test_public_revision_projection_is_exact_and_private_provenance_stays_required(self):
        row, calls, _ = self.exercise()
        public = json.loads(row["list"]["stdout_tail"])["data"]["installations"][0]["clients"][0]["package_revision"]
        private = row["copilot_project_skill_proof"]["owner"]["revision"]
        self.assertEqual(set(public), {"resolved_revision", "tree_digest", "manifest_digest"})
        self.assertIn("skill", private)
        for fault in ("wrong-public-revision", "missing-private-skill", "wrong-private-skill", "private-skill-public",
                      *("private-fact-" + key for key in ("name", "requested_path", "resolved_path", "raw_tree_digest", "normalizer"))):
            with self.subTest(fault=fault):
                refused, _, probes = self.exercise(fault=fault)
                self.assertNotEqual(refused["status"], "passed")
                self.assertIsNone(refused["skill_path"])
                self.assertFalse(probes)

    def test_unready_whole_skill_observes_lifecycle_but_never_native_pass(self):
        row, calls, _ = self.exercise(ready=False)
        self.assertEqual(row["status"], "incomplete-native-proof")
        self.assertIn("repeat", calls); self.assertIn("remove", calls)
        self.assertTrue(row["removal_verified"])
        self.assertEqual(row["public_skill_ready"], {"after-add": False, "repeat-add": False})

    def test_whole_skill_refuses_wrong_or_unsafe_owner_inputs(self):
        for fault in ("iid", "binding", "physical", "source", "receipt", "scope", "path", "bytes", "hardlink", "symlink", "state-link", "coherent-name", "coherent-physical", "coherent-binding"):
            with self.subTest(fault=fault):
                row, calls, probes = self.exercise(fault=fault)
                self.assertNotEqual(row["status"], "passed")
                self.assertNotIn("repeat", calls); self.assertNotIn("remove", calls)
                self.assertFalse(probes)

    def test_whole_skill_native_mismatch_cannot_pass_and_raw_inputs_survive(self):
        row, calls, probes = self.exercise(fault="catalog")
        self.assertEqual(row["status"], "incomplete-native-proof")
        self.assertIn("remove", calls)
        captured = row["copilot_project_skill_proof"]["catalogs"]["after-add"]
        self.assertEqual(captured[0]["stdout"], "[]")
        self.assertEqual(captured[0]["exit"], 0)
        self.assertEqual(len(probes), 6)

    def test_whole_skill_late_resource_mutation_never_passes(self):
        with self.assertRaises((RuntimeError, AssertionError)): self.exercise(fault="late-mutation")

    def test_copilot_skill_git_prerequisite_refuses_existing_or_failed_init(self):
        for fault in ("git-preexisting", "git-failure"):
            with self.subTest(fault=fault), self.assertRaises(RuntimeError):
                self.exercise(fault=fault)
            self.assertNotIn("add", self.last_calls)
            self.assertFalse(self.last_probes)

    def test_whole_skill_noop_receipts_and_ancestor_identity_remain_strict(self):
        row, calls, _ = self.exercise(fault="repeat-receipt")
        self.assertNotEqual(row["status"], "passed")
        self.assertFalse(row["repeat_verified"])
        self.assertIn("remove", calls)
        with self.assertRaises((RuntimeError, ValueError)):
            self.exercise(fault="ancestor")

    def test_whole_skill_only_exact_bounded_new_process_logs_qualify(self):
        row, _, _ = self.exercise(fault="process-logs")
        self.assertEqual(row["status"], "passed")
        self.assertFalse(row["repeat_native_unchanged"])
        self.assertTrue(row["repeat_native_runtime_only"])
        with self.assertRaises(AssertionError): self.exercise(fault="foreign-log")

    def test_whole_skill_catalog_schema_stderr_exit_and_metadata_never_qualify(self):
        for fault in ("catalog-extra", "catalog-duplicate", "description", "stderr", "exit", "timeout", "utf8"):
            with self.subTest(fault=fault):
                row, calls, _ = self.exercise(fault=fault)
                self.assertNotEqual(row["status"], "passed")
                self.assertIn("remove", calls)
                if fault == "utf8":
                    import base64
                    capture = row["copilot_project_skill_proof"]["catalogs"]["after-add"][0]
                    self.assertEqual(base64.b64decode(capture["stdout_base64"]), b"\xff")

    def test_whole_skill_native_capture_uses_one_aggregate_byte_budget(self):
        for fault in ("budget", "flood"):
            with self.subTest(fault=fault):
                row, _, probes = self.exercise(fault=fault)
                self.assertNotEqual(row["status"], "passed")
                self.assertEqual(len(probes), 3)  # exhausted first probe never starts a second
                for captures in row["copilot_project_skill_proof"]["catalogs"].values():
                    self.assertLessEqual(sum(x["retained_stdout_bytes"] + x["retained_stderr_bytes"] for x in captures), 1 << 20)

    def test_whole_skill_public_empty_list_cannot_hide_private_binding_after_remove(self):
        row, calls, _ = self.exercise(fault="private-remains")
        self.assertNotEqual(row["status"], "passed")
        self.assertFalse(row["removal_verified"])
        self.assertIn("remove", calls)

    def test_whole_skill_native_probe_custody_failure_is_hard_and_keeps_raw_capture(self):
        with self.assertRaises(RuntimeError) as caught:
            self.exercise(fault="probe-ancestor")
        self.assertNotIn("repeat", self.last_calls)
        self.assertNotIn("remove", self.last_calls)
        captures = caught.exception.copilot_project_skill_proof["catalogs"]["after-add"]
        self.assertEqual(len(captures), 1)
        self.assertEqual(captures[0]["exit"], 0)
        self.assertIn("installedFrom", captures[0]["stdout"])


if __name__ == "__main__":
    unittest.main()
