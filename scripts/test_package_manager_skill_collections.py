"""Evidence reporting must retain failures rather than silently qualify a run."""

import json
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

    def test_readonly_listing_rejects_empty_directory_and_symlink_effects(self):
        for effect in ("directory", "symlink"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                home = root / "home"
                home.mkdir()
                native = root / "native"
                native.mkdir()
                outside = root / "outside"
                outside.mkdir()
                if effect == "symlink":
                    probe = root / "probe"
                    try:
                        probe.symlink_to(outside, target_is_directory=True)
                    except OSError as error:
                        self.skipTest("symlink creation unavailable: " + str(error))
                    probe.unlink()
                evidence = root / "evidence"
                evidence.mkdir()

                class Run:
                    report = {}
                    directory = evidence
                    commands = []

                    def save(self):
                        pass

                    def command(self, name, *args, **kwargs):
                        self.commands.append(name)
                        changed = home / "unexpected"
                        if effect == "directory":
                            changed.mkdir()
                        else:
                            changed.symlink_to(outside, target_is_directory=True)
                        document = {"schema_version": 1, "result": "success", "data": {
                            "complete": True, "revision": "", "candidates": [
                                {"name": name} for name in collections.NAMES]}}
                        (evidence / (name + ".stdout")).write_text(json.dumps(document), encoding="utf-8")
                        return True

                run = Run()
                with self.assertRaisesRegex(RuntimeError, "read-only collection listing"):
                    collections.run_collections(run, "unused", root, {
                        "AGENTPLUGINS_HOME": str(home), "GEMINI_CLI_HOME": str(native)})
                self.assertEqual(run.commands, ["binary-collection-list"])
                self.assertEqual(run.report["binary_skill_collections"]["status"], "fail")

    def test_linked_directory_target_is_not_snapshotted(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "root"
            root.mkdir()
            outside = Path(temporary) / "outside"
            outside.mkdir()
            marker = outside / "marker"
            marker.write_bytes(b"before")
            try:
                (root / "link").symlink_to(outside, target_is_directory=True)
            except OSError as error:
                self.skipTest("symlink creation unavailable: " + str(error))
            before = collections.tree_bytes(root)
            marker.write_bytes(b"after")
            (outside / "new-empty-directory").mkdir()
            self.assertEqual(collections.tree_bytes(root), before)
            (root / "link").unlink()
            self.assertNotEqual(collections.tree_bytes(root), before)


if __name__ == "__main__":
    unittest.main()
