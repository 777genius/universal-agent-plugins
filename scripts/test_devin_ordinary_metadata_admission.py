"""Inert ordinary-profile contracts; no vendor process or hosted spoofing."""
from contextlib import contextmanager
import copy
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import devin_ordinary_metadata_admission as case
import test_devin_structured_inventory_admission as existing

base = case.base
MIGRATED = b'{"read_config_from":{"cloud":false},"version":1}\n'
# Match the measured byte count/projection; original raw migrated bytes were
# intentionally not retained by the closed known-file diagnostic.
MIGRATED_57_BYTES = b'{\n"read_config_from": {\n"cloud": false\n},\n"version": 1\n}\n'


class OrdinaryMetadataContracts(unittest.TestCase):
    @contextmanager
    def fixture(self, replace_before=False):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            helper = existing.KnownFileDiagnosticContracts()
            user, _ = helper.fixture(root); user.chmod(0o644)
            held = helper.hold(root)
            ctx = SimpleNamespace(root=root, summary={}, original_output_identity=base.root_identity(root),
                                  budget=SimpleNamespace(check=lambda: None), known_file_diagnostics=held)
            try:
                if replace_before:
                    replacement = user.with_suffix('.replacement'); replacement.write_bytes(case.ORIGINAL); replacement.chmod(0o644)
                    replacement.replace(user)
                effects = case.OrdinaryEffects(ctx, base.metadata_inventory(root, ctx.budget))
                yield ctx, effects, user
            finally:
                held.close()

    def advance_help(self, ctx, effects):
        current = base.metadata_inventory(ctx.root, ctx.budget)
        for _ in base.HELP: effects.verify_command(current, current)
        return current

    def test_atomic_first_migration_retains_baseline_and_both_fingerprints(self):
        with self.assertRaisesRegex(ValueError, 'original User authority differs'), self.fixture(replace_before=True):
            pass
        with self.fixture() as (ctx, effects, user):
            baseline, original = copy.deepcopy(effects.baseline), copy.deepcopy(effects.before)
            before = self.advance_help(ctx, effects)
            replacement = user.with_suffix('.replacement'); replacement.write_bytes(MIGRATED_57_BYTES); replacement.chmod(0o600)
            replacement.replace(user)
            current = base.metadata_inventory(ctx.root, ctx.budget)
            effects.verify_command(before, current)
            self.assertEqual(len(MIGRATED_57_BYTES), 57)
            self.assertEqual(effects.after['identity'][2], base.stat.S_IFREG | 0o600)
            self.assertEqual(effects.after['sha256'], case.hashlib.sha256(MIGRATED_57_BYTES).hexdigest())
            self.assertNotEqual(original['identity'], effects.after['identity'])
            self.assertNotEqual(original['sha256'], effects.after['sha256'])
            for _ in base.METADATA_PROBES[effects.step:]: effects.verify_command(current, current)
            self.assertEqual((effects.baseline, effects.report['before']), (baseline, original))
            self.assertEqual(ctx.summary['ordinary_user_config_migration']['after'], effects.after)
            with self.assertRaises(ValueError): effects.verify_command(current, current)

    def test_migration_rejects_wrong_policy_version_shape_and_size(self):
        bodies = (case.ORIGINAL, MIGRATED.replace(b'1}', b'true}'), MIGRATED.replace(b'1}', b'1.0}'),
                  MIGRATED.replace(b'false', b'0'), MIGRATED.replace(b'false', b'true'),
                  MIGRATED.replace(b'1}', b'1,"permissions":{}}'), MIGRATED.replace(b'1}', b'1,"version":1}'), b' ' * 257)
        for body in bodies:
            with self.subTest(body=body[:60]), self.fixture() as (ctx, effects, user):
                before = self.advance_help(ctx, effects); user.write_bytes(body); user.chmod(0o600)
                with self.assertRaises(ValueError): effects.verify_command(before, base.metadata_inventory(ctx.root, ctx.budget))
                self.assertIsNone(effects.report['after'])

    def test_first_wrong_mode_and_later_mode_regression_refuse_before_bytes(self):
        for mode in (0o644, 0o640, 0o400, 0o666):
            with self.subTest(first_mode=mode), self.fixture() as (ctx, effects, user):
                before = self.advance_help(ctx, effects)
                user.write_bytes(MIGRATED_57_BYTES); user.chmod(mode)
                current = base.metadata_inventory(ctx.root, ctx.budget)
                with patch.object(base.os, 'read', side_effect=AssertionError('wrong-mode bytes read')):
                    with self.assertRaisesRegex(ValueError, 'pre-read authority'):
                        effects.verify_command(before, current)
                self.assertIsNone(effects.after)
        with self.fixture() as (ctx, effects, user):
            original = copy.deepcopy(effects.baseline)
            before = self.advance_help(ctx, effects)
            user.write_bytes(MIGRATED_57_BYTES); user.chmod(0o600)
            current = base.metadata_inventory(ctx.root, ctx.budget)
            effects.verify_command(before, current)
            admitted = copy.deepcopy(effects.after)
            user.chmod(0o644)
            with patch.object(base.os, 'read', side_effect=AssertionError('late-mode bytes read')):
                with self.assertRaisesRegex(ValueError, 'pre-read authority'):
                    effects.verify_command(current, base.metadata_inventory(ctx.root, ctx.budget))
            self.assertEqual((effects.after, effects.baseline), (admitted, original))

    def test_early_late_and_same_bytes_replacement_never_renew_baseline(self):
        for change in ('help', 'before-mcp', 'late-bytes', 'late-identity'):
            with self.subTest(change=change), self.fixture() as (ctx, effects, user):
                original = copy.deepcopy(effects.baseline)
                if change != 'help': before = self.advance_help(ctx, effects)
                user.write_bytes(MIGRATED)
                if change.startswith('late'):
                    user.chmod(0o600)
                    effects.verify_command(before, base.metadata_inventory(ctx.root, ctx.budget))
                    replacement = user.with_suffix('.replacement')
                    replacement.write_bytes(MIGRATED if change == 'late-identity' else case.ORIGINAL); replacement.chmod(0o600)
                    replacement.replace(user)
                with self.assertRaises(ValueError): effects.verify(base.metadata_inventory(ctx.root, ctx.budget))
                self.assertEqual(effects.baseline, original)

    def test_original_alias_mode_and_parent_replacement_refuse(self):
        for change in ('hardlink', 'symlink', 'mode', 'parent'):
            with self.subTest(change=change), self.fixture() as (ctx, effects, user):
                before = self.advance_help(ctx, effects); user.write_bytes(MIGRATED); user.chmod(0o600)
                if change == 'hardlink': os.link(user, ctx.root / 'alias')
                elif change == 'symlink': user.unlink(); user.symlink_to(ctx.root / 'missing')
                elif change == 'mode': user.chmod(0o644)
                else:
                    parent = user.parent; parent.rename(ctx.root / 'replaced-parent'); parent.mkdir(); user.write_bytes(MIGRATED); user.chmod(0o600)
                with patch.object(base.os, 'read', side_effect=AssertionError('unadmitted bytes read')):
                    with self.assertRaises((ValueError, OSError)): effects.verify_command(before, base.metadata_inventory(ctx.root, ctx.budget))

    def test_ordinary_completion_refuses_private_argv_wrong_scope_and_recipe(self):
        rows = [(label, args, 'a') for label, args in base.HELP]
        rows += [(kind + '-' + label, [kind, 'list', '--json'], scope)
                 for label, scope in (('a', 'a'), ('b', 'b'), ('user', 'user'), ('a-repeat', 'a')) for kind in ('mcp', 'skills')]
        ctx = SimpleNamespace(argv=['/owned/client/bin/devin'], projects={name: Path('/owned') / name for name in ('a', 'b', 'user')})
        for change in (None, 'argv', 'cwd', 'recipe', 'route', 'missing'):
            ctx.summary = dict(metadata_recipe='ordinary', metadata_route=case.OrdinaryEffects.metadata_route,
                               commands=[dict(probe=label, argv=ctx.argv + args, cwd=str(ctx.projects[scope])) for label, args, scope in rows])
            if change == 'argv': ctx.summary['commands'][5]['argv'][1:1] = ['--config', '/owned/private/metadata-config.json']
            elif change == 'cwd': ctx.summary['commands'][5]['cwd'] = '/foreign'
            elif change in ('recipe', 'route'): ctx.summary['metadata_' + change] = 'private'
            elif change == 'missing': ctx.summary['commands'].pop()
            if change is None: case.require_ordinary_commands(ctx)
            else:
                with self.subTest(change=change), self.assertRaises(ValueError): case.require_ordinary_commands(ctx)

    def test_real_collect_sequence_keeps_private_route_and_refuses_nonzero_ordinary_migration(self):
        for ordinary in (False, True):
            with self.subTest(ordinary=ordinary), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                ctx = base.shared.Context(root, {}, SimpleNamespace(check=lambda: 10))
                ctx.original_output_identity = base.root_identity(root)
                for scope in ('a', 'b'):
                    exclude = ctx.projects[scope] / '.git/info/exclude'; exclude.parent.mkdir(parents=True); exclude.write_bytes(b'# original\n')
                calls = []
                def probe(label, arguments, project):
                    calls.append((label, arguments, project))
                    if label == 'mcp-a':
                        path = root / (case.OrdinaryEffects.config_path if ordinary else base.METADATA_CONFIG)
                        path.write_bytes(MIGRATED)
                        if ordinary: path.chmod(0o600)
                        return {'exit': 1}, b'[]', b'inert native refusal'
                    return {'exit': 0}, b'devin 3000.12.57 (96542e930752)\n' if label == 'version' else b'--json', b''
                ctx.probe = probe
                probes = {name: SimpleNamespace(url='http://127.0.0.1:1234/mcp', accepted_connections=[], methods=lambda: [],
                                               call_records=lambda: ([], 0)) for name in ('a', 'b', 'user', 'local', 'legacy')}
                try:
                    with self.assertRaisesRegex(ValueError, 'native metadata command refused'):
                        (case.collect if ordinary else base.collect)(ctx, probes)
                    prefix = [] if ordinary else ['--config', str(root / base.METADATA_CONFIG)]
                    self.assertEqual([args for _, args, _ in calls], [prefix + args for _, args in base.HELP] + [prefix + ['mcp', 'list', '--json']])
                    self.assertEqual((root / base.METADATA_CONFIG).exists(), not ordinary)
                    self.assertFalse(ctx.summary.get('metadata_capture_complete', False))
                    self.assertEqual(ctx.summary['metadata_route'], (case.OrdinaryEffects if ordinary else base.BootstrapEffects).metadata_route)
                    if ordinary: self.assertIsNotNone(ctx.summary['ordinary_user_config_migration']['after'])
                    else: self.assertEqual((root / case.OrdinaryEffects.config_path).read_bytes(), case.ORIGINAL)
                finally:
                    ctx.known_file_diagnostics.close()


class OrdinaryPreReadContracts(unittest.TestCase):
    fixture = OrdinaryMetadataContracts.fixture
    advance_help = OrdinaryMetadataContracts.advance_help

    def test_original_and_first_migrated_authority_refuse_before_any_read(self):
        for change in ('leaf', 'mtime', 'oversize', 'owner', 'mode'):
            with self.subTest(change=change), self.fixture() as (ctx, effects, user):
                before = self.advance_help(ctx, effects)
                if change in ('leaf', 'mtime'):
                    user.write_bytes(MIGRATED); user.chmod(0o600)
                    effects.verify_command(before, base.metadata_inventory(ctx.root, ctx.budget))
                    if change == 'leaf':
                        replacement = user.with_suffix('.replacement')
                        replacement.write_bytes(MIGRATED); replacement.chmod(0o600); replacement.replace(user)
                    else:
                        info = user.stat()
                        os.utime(user, ns=(info.st_atime_ns, info.st_mtime_ns - 60_000_000_000))
                elif change == 'oversize':
                    user.write_bytes(b' ' * 257); effects.migrating = True
                elif change == 'mode':
                    user.chmod(0o600)
                current = base.metadata_inventory(ctx.root, ctx.budget)
                if change == 'owner':
                    row = list(current[effects.config_path]); row[4] += 1
                    current[effects.config_path] = tuple(row)
                with patch.object(base.os, 'read', side_effect=AssertionError('unadmitted config bytes read')):
                    with self.assertRaises(ValueError): effects.verify(current)

    def test_real_ordinary_collect_retains_outer_delta_before_refused_content_snapshot(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            ctx = base.shared.Context(root, {}, SimpleNamespace(check=lambda: 10))
            ctx.original_output_identity = base.root_identity(root)
            for scope in ('a', 'b'):
                exclude = ctx.projects[scope] / '.git/info/exclude'
                exclude.parent.mkdir(parents=True); exclude.write_bytes(b'# original\n')
            mutated, snapshots = [], []
            def probe(label, arguments, project):
                if label == 'mcp-a':
                    (root / case.OrdinaryEffects.config_path).chmod(0o600); mutated.append(True)
                return {'exit': 0}, b'devin 3000.12.57 (96542e930752)\n' if label == 'version' else b'--json', b''
            ctx.probe = probe
            probes = {name: SimpleNamespace(url='http://127.0.0.1:1234/mcp', accepted_connections=[],
                      methods=lambda: [], call_records=lambda: ([], 0)) for name in ('a', 'b', 'user', 'local', 'legacy')}
            original_snapshot = base.snapshot
            def snapshot(context):
                if mutated: raise AssertionError('unadmitted content snapshot')
                snapshots.append(True); return original_snapshot(context)
            try:
                with patch.object(base, 'snapshot', side_effect=snapshot):
                    with self.assertRaisesRegex(ValueError, 'migration changed policy or version'): case.collect(ctx, probes)
                self.assertTrue(snapshots)
                effect = ctx.summary['command_effects'][-1]
                self.assertEqual(effect['label'], 'mcp-a')
                self.assertEqual(effect['changes'], {})
                self.assertIn(case.OrdinaryEffects.config_path, effect['outer_changes'])
                self.assertFalse(ctx.summary['admission_qualified'])
            finally:
                ctx.known_file_diagnostics.close()
