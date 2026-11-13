"""Inert real-file/consumer contracts; never vendor or model execution."""
import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import devin_global_lifecycle_admission as case


class GlobalLifecycleContracts(unittest.TestCase):
    def context(self, root):
        return SimpleNamespace(root=root, original_output_identity=case.base.root_identity(root),
                               budget=SimpleNamespace(check=lambda: None), argv=[str(root / "no-native")])

    def test_large_role_streaming_hash_does_not_widen_ordinary_limit(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            path = root / "large"; path.write_bytes(b"x" * ((1 << 20) + 1))
            path.chmod(0o700)
            inventory = case.base.metadata_inventory(root, ctx.budget)
            expected = hashlib.sha256(path.read_bytes()).hexdigest()
            with self.assertRaisesRegex(ValueError, "ordinary file authority"):
                case.regular(ctx, "large", inventory)
            self.assertEqual(case.regular(ctx, "large", inventory, large=(path.stat().st_size, expected)), expected)
            with self.assertRaisesRegex(ValueError, "large role differs"):
                case.regular(ctx, "large", inventory, large=(path.stat().st_size, "0" * 64))
            original = case.base.os.read
            sizes = []
            def read(fd, amount):
                sizes.append(amount); return original(fd, amount)
            with mock.patch.object(case.base.os, "read", side_effect=read):
                case.regular(ctx, "large", inventory, large=(path.stat().st_size, expected))
            self.assertLessEqual(max(sizes), 65536)

    def test_large_role_alias_owner_and_identity_refuse_before_read(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            path = root / "member"; path.write_bytes(b"owned")
            inventory = case.base.metadata_inventory(root, ctx.budget)
            for change in ("owner", "links", "inode"):
                value = copy.deepcopy(inventory)
                if change in ("owner", "links"):
                    row = list(value["member"]); row[4 if change == "owner" else 3] += 1; value["member"] = tuple(row)
                else:
                    other = root / "other"; other.write_bytes(b"foreign"); other.replace(path)
                with mock.patch.object(os, "read", side_effect=AssertionError("foreign bytes read")):
                    with self.assertRaises(ValueError): case.regular(ctx, "member", value, large=(5, case.sha(b"owned")))

    def test_public_consumer_uses_real_argv_and_aborts_failed_command_without_adoption(self):
        journey = object.__new__(case.Journey)
        journey.installer, journey.home = Path("/owned/agentplugins"), Path("/owned/home/uap-state")
        journey.cli_pin = (12, '0' * 64)
        journey.ctx = SimpleNamespace(env={"HOME": "/original", "XDG_CONFIG_HOME": "/original/config", "PATH": "/usr/bin"},
                                     argv=["/owned/client/bin/devin"], projects={"user": Path("/owned/outside")}, budget=object())
        journey.frame = mock.Mock(return_value=({}, {}))
        journey.state = mock.Mock(side_effect=AssertionError("failed command adopted state"))
        journey.ctx.command = mock.Mock(return_value=({"exit": 1}, b"{}", b"native refusal"))
        with mock.patch.object(case, 'installer_pin', return_value=journey.cli_pin):
            with self.assertRaisesRegex(ValueError, "public Global lifecycle refused"):
                journey.public("uap-add", ["add", "/owned/source", "--no-save"])
        args = journey.ctx.command.call_args.args
        self.assertEqual(args[1], ["/owned/agentplugins", "add", "/owned/source", "--no-save", "--yes",
                                   "--target", "windsurf", "--scope", "user", "--format", "json"])
        self.assertEqual(args[3]["HOME"], "/original")
        self.assertEqual(args[3]["XDG_CONFIG_HOME"], "/original/config")
        self.assertEqual(args[3]["PATH"], "/owned/client/bin:/usr/bin")
        journey.state.assert_not_called()

    def test_native_fence_rejects_user_foreign_and_prior_log_changes(self):
        journey = object.__new__(case.Journey)
        journey.effects = mock.Mock(); journey.effects.baseline = {}; journey.effects.config_path = "config"
        journey.log_births = {}
        for name in ("home/config/devin/mcp_config.json", "home/uap-state/state-v2.json",
                     "project-b/foreign", "home/data/devin/cli/logs/devin_20261113-180000_12.log"):
            with self.subTest(name=name), self.assertRaises(ValueError):
                journey.verify_native_delta({name: (1,)}, {name: (2,)}, {name: {"sha256": "before"}},
                                            {name: {"sha256": "after"}}, "mcp-installed-a")
        journey.effects.verify.assert_not_called()

    def test_exact_sequence_completes_only_research_with_false_qualifications(self):
        first = ("source-commit", "source-tree", "source-clean", "git-init-a", "git-init-b", "git-init-c") + case.base.METADATA_PROBES
        tail = ("uap-add", *case.NATIVE[:4], "uap-doctor-installed", "uap-repair", *case.NATIVE[4:8],
                "uap-doctor-repaired", "uap-remove", *case.NATIVE[8:], "source-final-clean")
        value = {"commands": [{"probe": name, "exit": 0, "error": None} for name in first + tail],
                 "global_lifecycle_capture_complete": True, "metadata_capture_complete": True,
                 "admission_qualified": False, "qualification": dict.fromkeys(case.base.shared.QUALIFICATION_CASES, False),
                 "native_global_steps": list(case.NATIVE), "public_global_steps": list(case.PUBLIC)}
        case.completion(value)
        for change in ("missing", "error", "qualification"):
            wrong = copy.deepcopy(value)
            if change == "missing": wrong["commands"].pop()
            elif change == "error": wrong["commands"][0]["error"] = "timeout"
            else: wrong["qualification"]["mcp-tools-call"] = True
            with self.assertRaises(ValueError): case.completion(wrong)

    def test_actual_owned_skill_quarantine_preserves_inode_bytes_and_refuses_existing_target(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            ctx.private, ctx.summary = root / "private", {}
            ctx.private.mkdir()
            profile = root / "home/config/devin"
            selected = profile / "skills" / case.SKILL
            selected.mkdir(parents=True)
            body = b"---\nname: owned\n---\nprivate source\n"
            (selected / "SKILL.md").write_bytes(body)
            original = case.base.root_identity(selected)
            case.quarantine_skill(ctx, profile, body)
            moved = ctx.private / "deliberately-quarantined-skill"
            self.assertEqual(case.base.root_identity(moved), original)
            self.assertEqual((moved / "SKILL.md").read_bytes(), body)
            selected.mkdir(); (selected / "SKILL.md").write_bytes(body)
            with self.assertRaisesRegex(ValueError, "quarantine preexists"):
                case.quarantine_skill(ctx, profile, body)
            self.assertEqual((moved / "SKILL.md").read_bytes(), body)

    def test_closed_uap_home_refuses_unknown_member_before_reading_it(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            journey = object.__new__(case.Journey)
            journey.ctx, journey.home = ctx, root / "home/uap-state"
            journey.assessment_path = None
            data = journey.home / "plugin-data/physical"
            data.mkdir(parents=True)
            journey.data_receipt = {"locator": str(data), "state": "owned"}
            (data / ".agentplugins-data-owner.json").write_text(json.dumps(journey.data_receipt))
            (journey.home / "state-v2.json").write_text('{"schema_version":5,"installations":[]}')
            (journey.home / "mutation.lock").write_text('{"pid":12,"acquired_at":"2026-11-13T18:00:00Z"}')
            journey.read = lambda path, inventory: case.regular(ctx, path.relative_to(root).as_posix(), inventory)
            inventory = case.base.metadata_inventory(root, ctx.budget)
            journey.closed_home(inventory, removed=True)
            unknown = journey.home / "unknown-private-token"
            unknown.write_bytes(b"foreign sentinel")
            with mock.patch.object(journey, "read", side_effect=AssertionError("unknown content read")):
                with self.assertRaisesRegex(ValueError, "unknown UAP state member"):
                    journey.closed_home(case.base.metadata_inventory(root, ctx.budget), removed=True)
            self.assertEqual(unknown.read_bytes(), b"foreign sentinel")

    def test_scan_cache_role_is_exact_subject_policy_and_typed_counts(self):
        value = {"schema_version": 1, "subject": {"tree_digest": "sha256:" + "a" * 64,
                 "manifest_digest": "sha256:" + "b" * 64}, "scanner": {"id": "lintai", "version": "0.1.3"},
                 "policy": case.POLICY, "counts": {"blocking": 0, "warnings": 0, "total": 0},
                 "outcome": "no_blocking_findings", "scanned_files": 4, "report_digest": "sha256:" + "c" * 64}
        path = case.security_role(value)
        self.assertRegex(path, r"^security/assessments/[0-9a-f]{64}\.json$")
        changed = copy.deepcopy(value); changed["subject"]["tree_digest"] = "sha256:" + "d" * 64
        self.assertNotEqual(case.security_role(changed), path)
        for field in ("schema_version", "blocking", "scanner", "extra"):
            wrong = copy.deepcopy(value)
            if field == "schema_version": wrong[field] = True
            elif field == "blocking": wrong["counts"][field] = False
            elif field == "scanner": wrong[field]["version"] = "0.1.2"
            else: wrong["extra"] = "unbound"
            with self.assertRaises(ValueError): case.security_role(wrong)

    def test_actual_native_consumer_checks_selected_user_source_before_next_command(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            ctx = case.base.shared.Context(root, {}, SimpleNamespace(check=lambda: 10))
            ctx.original_output_identity = case.base.root_identity(root)
            ctx.argv = [str(root / "no-vendor")]
            probes = {name: SimpleNamespace(url='http://127.0.0.1:1234/mcp', accepted_connections=[],
                       methods=lambda: [], call_records=lambda: ([], 0)) for name in ('a', 'b', 'user', 'local', 'legacy')}
            expected, _, _ = case.base.fixture(ctx, {name: probe.url for name, probe in probes.items()}, "n" * 64)
            profile = Path(expected['user']['root'])
            entries = dict(expected['user']['mcp'])
            entries.update({case.SERVERS[0]: {'command': 'python3', 'args': []},
                            case.SERVERS[1]: {'url': probes['user'].url}})
            (profile / 'mcp_config.json').write_text(json.dumps({'mcpServers': entries}))
            wanted = {name: (entry, profile / 'mcp_config.json') for name, entry in entries.items()}
            wanted.update({name: (entry, Path(expected['a']['root']) / 'mcp_config.json') for name, entry in expected['a']['mcp'].items()})
            wanted['same-name'] = ({'url': expected['local']['url'], 'disabled': True}, Path(expected['local']['path']))
            wanted['legacy-foreign'] = ({'url': expected['legacy_url'], 'disabled': True}, ctx.home / '.codeium/windsurf/mcp_config.json')
            rows = [dict(name=name, enabled=not entry.get('disabled', False), disabled_reason='disabled' if entry.get('disabled') else None,
                         transport='stdio' if 'command' in entry else 'http', command=entry.get('command'),
                         args=entry.get('args', []), url=entry.get('url'), disabled_tools=entry.get('disabledTools', []), source=str(source))
                    for name, (entry, source) in wanted.items()]
            for wrong in (False, True):
                journey = object.__new__(case.Journey)
                journey.ctx, journey.probes, journey.expected, journey.profile = ctx, probes, expected, profile
                journey.cli_pin, journey.helper, journey.nonce = (1, '0' * 64), None, "n" * 64
                journey.authorities = {profile.relative_to(root).as_posix(): case.base.root_identity(profile)}
                journey.log_births, journey.native_labels = {}, []
                journey.effects = SimpleNamespace(baseline=case.base.metadata_inventory(root, ctx.budget),
                                                   config_path='home/config/devin/config.json', verify=mock.Mock())
                emitted = copy.deepcopy(rows)
                if wrong:
                    next(row for row in emitted if row['name'] == case.SERVERS[0])['source'] = str(ctx.projects['b'] / '.devin/mcp_config.json')
                ctx.probe = mock.Mock(side_effect=[({'exit': 0}, json.dumps(emitted).encode(), b''), StopIteration('second command reached')])
                with self.assertRaises(ValueError if wrong else StopIteration):
                    journey.native('installed')
                self.assertEqual(ctx.probe.call_count, 1 if wrong else 2)
                self.assertEqual(journey.native_labels, [] if wrong else ['mcp-installed-a'])
                self.assertNotIn('global_lifecycle_capture_complete', ctx.summary)

    def test_global_recipe_reaches_host_guard_without_fixture_io(self):
        with mock.patch.object(case.base.shared.native, 'require_hosted', side_effect=RuntimeError('host gate')) as gate:
            with mock.patch.object(Path, 'mkdir', side_effect=AssertionError('fixture created')):
                with self.assertRaisesRegex(RuntimeError, 'host gate'):
                    case.base.run(Path('/unused'), collector=lambda *args: None, recipe='ordinary-global')
        gate.assert_called_once()

    def test_execute_forwards_removed_state_to_public_consumer(self):
        journey = object.__new__(case.Journey)
        journey.source, journey.identifier = Path('/source'), 'installed-id'
        journey.ctx = SimpleNamespace(summary={})
        journey.log_births, journey.state_history = {}, []
        journey.public_labels, journey.native_labels = [], []
        journey.source_bodies = {'skills/' + case.SKILL + '/SKILL.md': b'inert'}
        journey.profile = Path('/profile')
        calls = []
        def public(label, argv, **kwargs):
            calls.append((label, kwargs)); journey.public_labels.append(label)
        def native(phase):
            journey.native_labels.extend(kind + '-' + phase + '-' + scope
                                         for scope in ('a', 'b') for kind in ('mcp', 'skills'))
        journey.public, journey.native = public, native
        with mock.patch.object(case, 'quarantine_skill'):
            journey.execute()
        self.assertEqual(calls[-1], ('uap-remove', {'removed': True}))

    def test_native_replacement_refuses_before_any_post_command_body(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            path = root / 'user-config'; path.write_bytes(b'original')
            before = case.base.metadata_inventory(root, ctx.budget)
            journey = object.__new__(case.Journey)
            journey.ctx, journey.native_labels = ctx, []
            journey.frame = mock.Mock(return_value=(before, {}))
            def probe(*args, **kwargs):
                other = root / 'private'; other.write_bytes(b'foreign secret')
                other.replace(path)
                return {'exit': 0}, b'[]', b''
            ctx.probe = probe
            with mock.patch.object(os, 'read', side_effect=AssertionError('foreign body read')):
                with self.assertRaisesRegex(ValueError, 'nonauxiliary authority'):
                    journey.native('installed')
            journey.frame.assert_called_once_with()

    def test_original_uap_root_replacement_refuses_before_body_capture(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            state = root / "home/uap-state"; state.mkdir(parents=True)
            original = case.base.root_identity(state)
            state.rename(root / "moved-state")
            state.mkdir(); (state / "foreign").write_bytes(b"foreign private body")
            with mock.patch.object(os, 'read', side_effect=AssertionError('foreign bytes read')):
                with self.assertRaisesRegex(ValueError, 'original directory authority changed'):
                    case.capture(ctx, (1, '0' * 64), authorities={'home/uap-state': original})

    def test_successful_public_command_rejoins_original_roots_before_state_bytes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            state = root / 'home/uap-state'; state.mkdir(parents=True)
            (state / 'state-v2.json').write_text('{"schema_version":5,"installations":[]}')
            journey = object.__new__(case.Journey)
            journey.ctx, journey.home, journey.installer = ctx, state, root / 'agentplugins'
            journey.cli_pin = (1, '0' * 64)
            journey.authorities = {'home/uap-state': case.base.root_identity(state)}
            before = case.base.metadata_inventory(root, ctx.budget)
            journey.frame = mock.Mock(return_value=(before, {}))
            ctx.env, ctx.projects = {'PATH': '/usr/bin'}, {'user': root}
            def command(*args):
                state.rename(root / 'retained-original-state')
                state.mkdir()
                (state / 'state-v2.json').write_bytes(b'foreign private state')
                return {'exit': 0}, b'{"schema_version":1,"command":"doctor","result":"success","data":{}}', b''
            ctx.command = command
            with mock.patch.object(case, 'installer_pin', return_value=journey.cli_pin):
                with mock.patch.object(os, 'read', side_effect=AssertionError('foreign state bytes read')):
                    with self.assertRaisesRegex(ValueError, 'original directory authority changed'):
                        journey.public('uap-doctor-installed', ['doctor', 'owned-id'])

    def test_successful_public_source_leaf_swap_refuses_before_any_body(self):
        for label, operation in (("uap-repair", "repair"), ("uap-doctor-installed", "doctor")):
            with self.subTest(label=label), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(); ctx = self.context(root)
                state = root / "home/uap-state"; state.mkdir(parents=True)
                source = root / "component-source"; source.mkdir()
                member = source / "plugin.json"; member.write_bytes(b"original source")
                journey = object.__new__(case.Journey)
                journey.ctx, journey.home, journey.profile, journey.installer = ctx, state, root / "profile", root / "agentplugins"
                journey.cli_pin = (1, "0" * 64)
                journey.authorities = {path.relative_to(root).as_posix(): case.base.root_identity(path) for path in (state, source)}
                before = case.base.metadata_inventory(root, ctx.budget)
                journey.frame = mock.Mock(return_value=(before, {}))
                journey.state = lambda inventory, **kwargs: case.regular(ctx, "component-source/plugin.json", inventory)
                ctx.env, ctx.projects = {"PATH": "/usr/bin"}, {"user": root}
                def command(*args):
                    replacement = root / "replacement"; replacement.write_bytes(b"foreign private source")
                    replacement.replace(member)
                    return {"exit": 0}, json.dumps(dict(schema_version=1, command=operation, result="success", data={})).encode(), b""
                ctx.command = command
                with mock.patch.object(case, "installer_pin", return_value=journey.cli_pin):
                    with mock.patch.object(os, "read", side_effect=AssertionError("foreign source bytes read")) as read:
                        with self.assertRaisesRegex(ValueError, "public Global changed foreign file"):
                            journey.public(label, [operation, "owned-id"])
                        read.assert_not_called()



if __name__ == "__main__":
    unittest.main()
