"""Evidence reporting must retain failures rather than silently qualify a run."""

from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import package_manager_skill_collections as collections


class CollectionEvidenceTests(unittest.TestCase):
    def test_failure_is_retained_and_native_qualification_is_false(self):
        class Run:
            report = {}
            saves = 0

            def save(self):
                self.saves += 1

        run = Run()
        with patch.object(collections, "_collections", side_effect=RuntimeError("missing evidence")):
            with self.assertRaisesRegex(RuntimeError, "missing evidence"):
                collections.run_collections(run, None, None, None)
        self.assertEqual(run.report["binary_skill_collections"]["status"], "fail")
        self.assertFalse(run.report["binary_skill_collections"]["native_runtime_qualified"])
        self.assertEqual(run.saves, 2)

    def test_tree_snapshot_detects_membership_and_byte_changes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "SKILL.md").write_bytes(b"before")
            before = collections.tree_bytes(root)
            (root / "SKILL.md").write_bytes(b"after")
            self.assertNotEqual(collections.tree_bytes(root), before)
            (root / "SKILL.md").write_bytes(b"before")
            (root / "another").write_bytes(b"new")
            self.assertNotEqual(collections.tree_bytes(root), before)


if __name__ == "__main__":
    unittest.main()
