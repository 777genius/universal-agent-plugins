"""Parser/renderer regression tests, not native CLI qualification."""
import unittest
from harness import Screen
from selection_matrix import CASES, SETS, choices, plan_ids, selected_frame
from types import SimpleNamespace


class EvidenceTests(unittest.TestCase):
    def test_matrix_covers_each_nonempty_set_both_decisions(self):
        self.assertEqual(set(SETS.values()), {('codex',), ('cursor',), ('codex', 'cursor')})
        for name in SETS:
            for decision in ('default-no', 'yes'):
                self.assertIn(name + '-' + decision, CASES)
        self.assertIn('neither', CASES)

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
