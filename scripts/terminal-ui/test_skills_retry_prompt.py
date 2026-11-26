"""Retry readiness follows the complete visible prompt across PTY encodings."""
import re
import unittest

from harness import clean
from skills_matrix import RETRY


class RetryPromptTests(unittest.TestCase):
    def test_complete_prompt_accepts_literal_or_conpty_cursor_encoded_spacing(self):
        # ConPTY can represent the trailing blank as cursor-forward rather than
        # a space byte. This is the exact shape of the Windows partial captures.
        prompt = b'Retry OpenAI Codex? [Y/n]'
        for suffix in (b' ', b'\x1b[1C', b''):
            with self.subTest(suffix=suffix):
                output = b'Needs attention\r\n' + prompt + suffix
                found = re.search(RETRY, clean(output))
                self.assertIsNotNone(found)
                self.assertEqual(found.group(), prompt.decode())

    def test_incomplete_or_different_prompt_never_authorizes_an_answer(self):
        for output in (
                b'Retry OpenAI Codex? ',
                b'Retry OpenAI Codex? [Y/',
                b'Retry OpenAI Codex? [Y/n\x1b[1C',
                b'Retry OpenAI Codex? [Y/nx]',
                b'Retry OpenAI Codex? [y/N]',
                b'Install these Skills? [Y/n]',
                b'Retrying OpenAI Codex? [Y/n]',
                b'Retry OpenAI\r\nCodex? [Y/n]'):
            with self.subTest(output=output):
                self.assertIsNone(re.search(RETRY, clean(output)))


if __name__ == '__main__':
    unittest.main()
