"""Inert parser checks for the delegated private-parent prerequisite."""
from pathlib import Path
import runpy
import unittest


parse = runpy.run_path(str(Path(__file__).parent / "e2e/grok-enable-readonly.py"))["private_parent_mounts"]
ROW = b"20 1 8:2 /private-root /private-point rw,nosuid - ext4 private-source rw\n"


class PrivateParentMountTests(unittest.TestCase):
    def test_private_and_unbindable_report_only_finite_state(self):
        for body in (ROW, ROW.replace(b" - ", b" unbindable - ")):
            result = parse(body)
            self.assertFalse(result["qualification"])
            self.assertTrue(result["private"])
            self.assertEqual(result["mount_count"], 1)
            self.assertEqual(result["unbindable_count"], int(b"unbindable" in body))
            self.assertNotIn("private-root", str(result))
            self.assertNotIn("private-source", str(result))

    def test_propagation_and_unknown_tokens_refuse(self):
        for token in (b"shared:3", b"master:4", b"propagate_from:5", b"future-tag", b"unbindable unbindable"):
            with self.subTest(token=token):
                with self.assertRaisesRegex(ValueError, "propagation is not private"):
                    parse(ROW.replace(b" - ", b" " + token + b" - "))

    def test_empty_truncated_overbound_and_malformed_refuse_without_paths(self):
        for body in (b"", ROW[:-1], b"x" * 262145, ROW + b"\n", ROW.replace(b"20 1", b"20  1"),
                     ROW.replace(b"20 1", b"020 1"), ROW.replace(b"8:2", b"8:2:3"),
                     ROW.replace(b" - ", b" missing "), ROW.replace(b"ext4 private-source", b"ext4 "),
                     ROW + ROW, ROW.replace(b"20 1", b"0 1"), ROW.replace(b"/private-root", b"relative")):
            with self.subTest(size=len(body)):
                with self.assertRaises(ValueError) as failure:
                    parse(body)
                self.assertNotIn("private-root", str(failure.exception))
                self.assertNotIn("private-source", str(failure.exception))


if __name__ == "__main__":
    unittest.main()
