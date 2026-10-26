"""Inert pin and complete-closure checks; no compiler or vendor execution."""
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import project_native_vscode_toolchain as compiler
from project_native_research import Budget


class CompilerTests(unittest.TestCase):
    def test_minimal_lock_has_only_four_exact_integrity_pinned_packages(self):
        manifest, lock = compiler.package_lock()
        self.assertIs(manifest["private"], True)
        self.assertEqual(lock["lockfileVersion"], 3)
        self.assertEqual(set(lock["packages"]), {"", *("node_modules/" + name for name in compiler.PACKAGES)})
        self.assertNotIn("scripts", manifest)
        for name, (version, integrity) in compiler.PACKAGES.items():
            row = lock["packages"]["node_modules/" + name]
            self.assertEqual((row["version"], row["integrity"]), (version, integrity))
            self.assertTrue(row["resolved"].startswith("https://registry.npmjs.org/" + name + "/-/"))
        self.assertEqual(lock["packages"]["node_modules/@types/node"]["dependencies"], {"undici-types": "~6.21.0"})

    def test_declaration_requires_both_exact_size_and_git_blob_identity(self):
        body = b"inert typed declaration\n"
        blob = hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()
        with patch.dict(compiler.DECLARATIONS, {"inert.d.ts": (len(body), blob)}):
            receipt = compiler.check_declaration("inert.d.ts", body)
            self.assertEqual(receipt["git_blob"], blob)
            for wrong in (body + b" ", body.replace(b"typed", b"other")):
                with self.assertRaisesRegex(ValueError, "pinned source"):
                    compiler.check_declaration("inert.d.ts", wrong)

    def test_ordinary_inputs_reject_leaf_and_parent_aliases_and_hardlinks(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "source"
            source.mkdir()
            actual = source / "input"
            actual.write_bytes(b"inert")
            self.assertEqual(compiler.ordinary_bytes(actual, 8), b"inert")
            leaf = source / "alias"
            leaf.symlink_to(actual)
            parent = root / "alias-parent"
            parent.symlink_to(source, target_is_directory=True)
            for path in (leaf, parent / "input"):
                with self.assertRaisesRegex(ValueError, "alias"):
                    compiler.ordinary_bytes(path, 8)
            linked = source / "hardlink"
            linked.hardlink_to(actual)
            with self.assertRaisesRegex(ValueError, "single-link"):
                compiler.ordinary_bytes(actual, 8)

    def fixture(self, root):
        source = root / "playwright"
        compiler.write_new(source / "package.json", json.dumps({"name": "playwright-core", "version": "1.64.1"}).encode())
        compiler.write_new(source / "lib/inert.js", b"module.exports={};\n")
        return source

    def test_runtime_closure_preserves_all_regular_files_but_rejects_aliases(self):
        for bad in (None, "link", "nested-dependency"):
            with self.subTest(bad=bad), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                source = self.fixture(root)
                if bad == "link":
                    (source / "lib/alias.js").symlink_to(source / "lib/inert.js")
                elif bad:
                    compiler.write_new(source / "node_modules/unreviewed/package.json", b"{}")
                if bad:
                    with self.assertRaises(ValueError):
                        compiler.runtime_tree(source, root / "runtime", Budget())
                else:
                    count, size = compiler.runtime_tree(source, root / "runtime", Budget())
                    self.assertEqual(count, 2)
                    self.assertGreater(size, 0)
                    self.assertEqual((root / "runtime/lib/inert.js").read_bytes(), b"module.exports={};\n")
                    self.assertEqual((root / "runtime/lib/inert.js").stat().st_mode & 0o777, 0o600)

    def test_manifest_requires_complete_runtime_and_excludes_compiler_inputs(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for name in (*compiler.RUNTIME_FILES, "observer/package.json", "driver/package.json", "node", "driver/node_modules/playwright-core/package.json"):
                compiler.write_new(root / name, b"inert", executable=name == "node")
            manifest = compiler.runtime_manifest(root, Budget())
            self.assertEqual(len(manifest["files"]), 7)
            self.assertEqual([row["path"] for row in manifest["files"] if row["executable"]], ["node"])
            compiler.write_new(root / "typescript/bin/tsc", b"not runtime")
            with self.assertRaisesRegex(ValueError, "unregistered"):
                compiler.runtime_manifest(root, Budget())
            (root / "typescript/bin/tsc").unlink()
            (root / "observer/runner.js").unlink()
            with self.assertRaisesRegex(ValueError, "incomplete"):
                compiler.runtime_manifest(root, Budget())


if __name__ == "__main__":
    unittest.main()
