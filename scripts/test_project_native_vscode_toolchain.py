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
    def test_npm_configs_are_distinct_empty_private_files_and_reject_changes(self):
        for mutation in (None, "contents", "replacement", "mode", "symlink", "hardlink"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                home = Path(temporary)
                environment, identities = compiler.npm_configs(home)
                paths = [Path(environment["NPM_CONFIG_" + scope + "CONFIG"]) for scope in ("USER", "GLOBAL")]
                self.assertNotEqual(paths[0], paths[1])
                self.assertNotEqual(paths[0].stat().st_ino, paths[1].stat().st_ino)
                for path in paths:
                    self.assertEqual(path.read_bytes(), b"")
                    self.assertEqual(path.stat().st_mode & 0o777, 0o600)
                path = paths[0]
                if mutation == "contents":
                    path.write_bytes(b"registry=https://unreviewed.invalid\n")
                elif mutation == "replacement":
                    path.rename(home / "original-config")
                    compiler.write_new(path, b"")
                elif mutation == "mode":
                    path.chmod(0o644)
                elif mutation == "symlink":
                    path.unlink()
                    path.symlink_to(paths[1])
                elif mutation == "hardlink":
                    (home / "linked-config").hardlink_to(path)
                if mutation:
                    with self.assertRaises(ValueError):
                        compiler.verify_npm_configs(identities)
                else:
                    compiler.verify_npm_configs(identities)
                    with self.assertRaises(FileExistsError):
                        compiler.npm_configs(home)

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
        native = lock["packages"]["node_modules/@typescript/typescript-linux-x64"]
        self.assertEqual((native["version"], native["os"], native["cpu"]), ("7.0.2", ["linux"], ["x64"]))
        self.assertNotIn("node_modules/typescript", lock["packages"])

    def test_compiler_identity_rejects_changed_binary_and_library_alias(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            body = b"inert compiler bytes"
            compiler.write_new(root / "lib/tsc", body, executable=True)
            compiler.write_new(root / "lib/lib.es2023.d.ts", b"inert library")
            with patch.object(compiler, "COMPILER_BYTES", len(body)), patch.object(compiler, "COMPILER_SHA256", hashlib.sha256(body).hexdigest()):
                executable, identities = compiler.compiler_identities(root, Budget())
                self.assertEqual(executable, root / "lib/tsc")
                self.assertEqual(len(identities), 2)
                (root / "lib/tsc").write_bytes(b"changed compiler!!!")
                with self.assertRaisesRegex(ValueError, "identity"):
                    compiler.compiler_identities(root, Budget())
                (root / "lib/tsc").write_bytes(body)
                (root / "lib/alias").symlink_to(root / "lib/lib.es2023.d.ts")
                with self.assertRaisesRegex(ValueError, "alias"):
                    compiler.compiler_identities(root, Budget())

    def test_declaration_requires_both_exact_size_and_git_blob_identity(self):
        body = b"inert typed declaration\n"
        blob = hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()
        with patch.dict(compiler.DECLARATIONS, {"inert.d.ts": (len(body), blob)}):
            receipt = compiler.check_declaration("inert.d.ts", body)
            self.assertEqual(receipt["git_blob"], blob)
            for wrong in (body + b" ", body.replace(b"typed", b"other")):
                with self.assertRaisesRegex(ValueError, "pinned source"):
                    compiler.check_declaration("inert.d.ts", wrong)

    def test_staged_input_and_type_library_drift_are_detected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for name in ("build/observer/runner.ts", "declarations/vscode.d.ts", "node_modules/@types/node/index.d.ts"):
                compiler.write_new(root / name, b"inert type input\n")
            identities = compiler.build_tree_identities(root, Budget())
            compiler.shared.verify_files(identities, Budget())
            for path in identities:
                original = path.read_bytes()
                path.write_bytes(b"changed type input\n")
                with self.assertRaisesRegex(ValueError, "changed"):
                    compiler.shared.verify_files(identities, Budget())
                path.write_bytes(original)

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
