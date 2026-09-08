"""Parser/renderer regression tests, not native CLI qualification."""
import unittest
import tempfile
import subprocess
import os
from harness import Fixture
from harness import Screen
from selection_matrix import CASES, SETS, choices, plan_ids, selected_frame, prepare_codex_registry
from types import SimpleNamespace


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
