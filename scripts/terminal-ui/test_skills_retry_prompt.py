"""Retry readiness follows the complete visible prompt across PTY encodings."""
import re
import unittest

from harness import clean
from skills_matrix import RETRY, RETRY_CONPTY_CAPTURE, WindowsSession, retry_prompt_visible


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
                self.assertTrue(retry_prompt_visible(output))

    def test_cursor_forward_can_encode_each_interior_blank(self):
        parts = [b'Retry', b'OpenAI', b'Codex?', b'[Y/n]']
        for encoded in ((0,), (1,), (2,), (0, 1, 2)):
            for motion in (b'\x1b[1C', b'\x1b[C', b'\x1b[0C', b'\x1b[2C'):
                with self.subTest(encoded=encoded, motion=motion):
                    output = parts[0]
                    for index, part in enumerate(parts[1:]):
                        output += (motion if index in encoded else b' ') + part
                    output = b'\x1b[2K\x1b[1DNeeds attention\r\n\x1b[33m' + output + b'\x1b[0m'
                    self.assertTrue(retry_prompt_visible(output))
                    self.assertEqual(self.windows_wait(output), len(output))

    @staticmethod
    def windows_wait(output, after=0):
        class Terminal:
            raw = bytearray(output)

            def wait(self, pattern, after=0):
                if pattern != RETRY_CONPTY_CAPTURE:
                    raise AssertionError('retry did not use the scoped capture marker')
                if not re.search(pattern, clean(self.raw[after:])):
                    raise AssertionError('incomplete prompt cannot be answered')
                return len(self.raw)

        session = WindowsSession.__new__(WindowsSession)
        session.terminal, session.timeout = Terminal(), 1
        return session.wait(RETRY, 'retry-ready', after=after)

    def test_missing_blanks_are_not_invented_by_the_windows_wait(self):
        for output in (b'RetryOpenAI Codex? [Y/n]', b'Retry OpenAI Codex?[Y/n]',
                       b'Retrying OpenAI Codex? [Y/n]',
                       b'Retry\x1b[1BOpenAI Codex? [Y/n]',
                       b'Retry OpenAI Codex?\x1b[1D[Y/n]',
                       b'Retry\x1b[1D OpenAI Codex? [Y/n]',
                       b'Retry \x1b[1DOpenAI Codex? [Y/n]',
                       b'Retry OpenAI Codex?\x1b[2K [Y/n]',
                       b'Retry OpenAI\b Codex? [Y/n]'):
            with self.subTest(output=output):
                self.assertFalse(retry_prompt_visible(output))
                with self.assertRaisesRegex(AssertionError, 'visible spacing'):
                    self.windows_wait(output)

    def test_retry_wait_respects_the_captured_byte_boundary(self):
        old = b'Retry OpenAI Codex? [Y/n]\r\n'
        partial = b'Retry\x1b[1COpenAI Codex?\x1b[1C[Y/'
        with self.assertRaisesRegex(AssertionError, 'incomplete prompt'):
            self.windows_wait(old + partial, after=len(old))
        complete = partial + b'n]'
        self.assertEqual(self.windows_wait(old + complete, after=len(old)),
                         len(old + complete))

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
                self.assertFalse(retry_prompt_visible(output))


if __name__ == '__main__':
    unittest.main()
