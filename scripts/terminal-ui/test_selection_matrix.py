"""Parser/renderer regression tests, not native CLI qualification."""
import unittest
import copy
import json
import shutil
import tempfile
import subprocess
import os
from harness import Fixture
from harness import Screen
from selection_matrix import CASES, SETS, choices, plan_ids, selected_frame, prepare_codex_registry, installed
from types import SimpleNamespace


class InstallationTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.fixture = Fixture(self.tmp.name)
        native = self.fixture.package / '.codex-plugin/plugin.json'
        native.parent.mkdir()
        native.write_text('{\n  "description": "Synthetic terminal fixture",\n'
                          '  "name": "pty-synthetic",\n  "version": "1.0.0"\n}\n')

    def materialize(self, selected):
        fixture = self.fixture
        self.targets = {}
        bindings = {}
        for client in selected:
            root = (fixture.data / 'managed/clients/codex' if client == 'codex'
                    else fixture.home / '.cursor')
            target = root / 'pty-synthetic'
            shutil.copytree(fixture.package, target)
            self.targets[client] = target
            binding_id = client + '-binding'
            bindings[binding_id] = {
                'client_id': client, 'client_binding_id': binding_id,
                'materialization': 'materialized',
                'activation': 'active' if client == 'codex' else 'unknown',
                'authentication': 'unknown', 'target_locator': str(target),
                'receipts': [{'phase': 'committed', 'client_binding_id': binding_id,
                              'active_path': str(target)}],
            }
        self.body = {'installations': [{'clients': bindings}]}
        self.save_state()
        (fixture.data / 'synthetic-codex-registry.json').write_text(
            json.dumps({'installed': [{'enabled': True}]}))
        installed(fixture, selected)

    def save_state(self):
        (self.fixture.data / 'state-v2.json').write_text(json.dumps(self.body))

    def test_each_selected_set_accepts_valid_packages(self):
        # Separate fixture per selection avoids residue from earlier selections.
        for selected in SETS.values():
            with self.subTest(selected=selected):
                with tempfile.TemporaryDirectory() as root:
                    original = self.fixture
                    try:
                        self.fixture = Fixture(root)
                        shutil.copytree(original.package / '.codex-plugin',
                                        self.fixture.package / '.codex-plugin')
                        self.materialize(selected)
                    finally:
                        self.fixture = original

    def test_both_rejects_missing_wrong_and_changed_cursor_manifest(self):
        self.materialize(SETS['both'])
        manifest = self.targets['cursor'] / 'plugin.json'
        source = manifest.read_bytes()
        for corruption, message in (
                (None, 'native package manifest missing'),
                (b'{"name": "wrong"}', 'wrong package materialized'),
                (source + b'\n', 'package manifest bytes differ from source')):
            with self.subTest(corruption=corruption):
                if corruption is None:
                    manifest.unlink()
                else:
                    manifest.write_bytes(corruption)
                with self.assertRaisesRegex(AssertionError, message):
                    installed(self.fixture, SETS['both'])
                manifest.write_bytes(source)

    def test_both_rejects_changed_codex_native_bytes(self):
        self.materialize(SETS['both'])
        native = self.targets['codex'] / '.codex-plugin/plugin.json'
        native.write_bytes(native.read_bytes() + b'\n')
        with self.assertRaisesRegex(AssertionError, 'Codex native manifest bytes'):
            installed(self.fixture, SETS['both'])

    def test_both_rejects_extra_installation(self):
        self.materialize(SETS['both'])
        self.body['installations'].append(copy.deepcopy(self.body['installations'][0]))
        self.save_state()
        with self.assertRaisesRegex(AssertionError, 'exactly one fresh installation'):
            installed(self.fixture, SETS['both'])

    def test_both_rejects_extra_binding(self):
        self.materialize(SETS['both'])
        bindings = self.body['installations'][0]['clients']
        bindings['extra'] = copy.deepcopy(bindings['cursor-binding'])
        self.save_state()
        with self.assertRaisesRegex(AssertionError, 'binding identities differ'):
            installed(self.fixture, SETS['both'])


class EvidenceTests(unittest.TestCase):
    def test_matrix_covers_each_nonempty_set_both_decisions(self):
        self.assertEqual(set(SETS.values()), {('codex',), ('cursor',), ('codex', 'cursor')})
        for name in SETS:
            for decision in ('default-no', 'yes'):
                self.assertIn(name + '-' + decision, CASES)
        self.assertIn('neither', CASES)

    @unittest.skipUnless(os.name == 'posix', 'shell client fixture is Unix only')
    def test_registry_protocol_is_strict_and_read_only(self):
        with tempfile.TemporaryDirectory() as root:
            fixture = Fixture(root)
            prepare_codex_registry(fixture)
            for argv, code, output in [(['plugin', 'list', '--json'], 0, '{"installed": []}'),
                                       (['plugin', 'install', 'anything'], 97, ''),
                                       (['plugin', 'list', '--json', 'extra'], 97, '')]:
                result = subprocess.run([str(fixture.bin / 'codex'), *argv],
                                        env=fixture.env, capture_output=True, text=True, timeout=3)
                self.assertEqual(result.returncode, code)
                self.assertEqual(result.stdout.strip(), output)
                fixture.unchanged()

    def test_eof_covers_both_prompts_and_partial_confirmation(self):
        for name in ('selection-eof', 'confirmation-eof', 'confirmation-partial-eof'):
            self.assertIn(name, CASES)

    def test_plan_ignores_labels_but_retains_wrong_and_duplicate_ids(self):
        self.assertEqual(plan_ids('OpenAI Codex (codex)\nTarget: cursor\nTarget: codex\n'),
                         ['cursor', 'codex'])
        self.assertEqual(plan_ids('Target: cursor\nTarget: cursor\n'), ['cursor', 'cursor'])

    def test_display_order_and_selection_are_independent_assertions(self):
        screen = Screen()
        screen.feed(b'[ ] OpenAI Codex (codex)\r\n')
        screen.feed('[•] Cursor (cursor)'.encode())
        session = SimpleNamespace(screen=screen)
        selected_frame(session, ('cursor',))
        with self.assertRaises(AssertionError): selected_frame(session, ('codex',))
        screen = Screen()
        screen.feed('[•] Cursor (cursor)\r\n[ ] OpenAI Codex (codex)'.encode())
        with self.assertRaises(AssertionError):
            selected_frame(SimpleNamespace(screen=screen), ('cursor',))

    def test_reverse_index_redraw_preserves_identity(self):
        raw = '[•] Codex (codex)\r\n[•] Cursor (cursor)\r\x1bM[ ]'.encode()
        screen = Screen()
        screen.feed(raw.replace(b'\x1bM', b'\x1b[1A'))
        self.assertEqual(choices(screen.snapshot()), [(' ', 'codex'), ('•', 'cursor')])


if __name__ == '__main__':
    unittest.main()
