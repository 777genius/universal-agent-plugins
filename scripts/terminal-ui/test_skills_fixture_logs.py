"""Native JSONL records preserve Unicode arguments and refuse malformed evidence."""
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest

from skills_fixture import SkillsFixture


class NativeCallLogTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.log = self.root / 'native-calls.jsonl'
        self.fixture = SimpleNamespace(root=self.root)

    def calls(self):
        return SkillsFixture.calls(self.fixture)

    def test_lf_records_preserve_literal_next_line_and_unicode_arguments(self):
        calls = [
            ['--version'],
            ['plugin', 'add', str(self.root / 'guide\u0085é Ω' / '技能'), '--json'],
            ['plugin', 'remove', 'документы@источник', 'emoji-🌍'],
        ]
        records = [json.dumps(row, ensure_ascii=False).encode('utf-8') for row in calls]
        self.assertIn('\u0085'.encode('utf-8'), records[1])
        for delimiter in (b'\n', b'\r\n'):
            for terminated in (False, True):
                with self.subTest(delimiter=delimiter, terminated=terminated):
                    payload = delimiter.join(records) + (delimiter if terminated else b'')
                    self.log.write_bytes(payload)
                    self.assertEqual(self.calls(), calls)

    def test_absent_or_empty_log_has_no_calls(self):
        self.assertEqual(self.calls(), [])
        self.log.write_bytes(b'')
        self.assertEqual(self.calls(), [])

    def test_malformed_record_is_not_ignored_after_valid_call(self):
        for malformed in (b'not-json', b'["plugin",', b'', b'{"args": invalid}'):
            with self.subTest(malformed=malformed):
                self.log.write_bytes(b'["--version"]\n' + malformed + b'\n')
                with self.assertRaises(json.JSONDecodeError):
                    self.calls()

    def test_bare_carriage_return_cannot_separate_jsonl_records(self):
        self.log.write_bytes(b'["--version"]\r["plugin", "list"]\n')
        with self.assertRaises(json.JSONDecodeError):
            self.calls()

    def test_invalid_utf8_cannot_be_replaced_or_ignored(self):
        self.log.write_bytes(b'["plugin", "add", "\xff"]\n')
        with self.assertRaises(UnicodeDecodeError):
            self.calls()


if __name__ == '__main__':
    unittest.main()
