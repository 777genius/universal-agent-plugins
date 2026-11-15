"""Inert driver verdict tests; no native processes, network or model calls."""
import contextlib
import gzip
import hashlib
import io
import json
from pathlib import Path
import runpy
import shutil
import stat
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch


class NativeDriverVerdict(unittest.TestCase):
    def run_driver(self, fail_cleanup=False, mutation=None):
        module = runpy.run_path(str(Path(__file__).with_name("e2e") / "grok-enable-readonly.py"))
        scope = module["main"].__globals__
        body = b"inert fixture: never execute"
        archive = gzip.compress(body, mtime=0)
        archive_digest = hashlib.sha256(archive).hexdigest()
        binary_digest = hashlib.sha256(body).hexdigest()

        def inert_run(argv, **kwargs):
            if argv[1:] == ["--version"]:
                stdout = b"grok 1.0.70 (f50904eaee6c)\n"
            elif argv == ["/usr/bin/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"]:
                stdout = b""
            elif argv[1:] == ["mcp", "disable", "uap_inert_lock"]:
                (Path(kwargs["env"]["HOME"]) / ".grok/.config-init.lock").write_bytes(b"")
                stdout = b""
            elif argv[1] == "-test.run=^TestEnablementReadonlyNativeInventory$":
                home, project = Path(kwargs["env"]["HOME"]), Path(kwargs["cwd"])
                writer_lock = home / ".grok/config.toml.agentplugins.lock"
                self.assertTrue(stat.S_ISREG(writer_lock.lstat().st_mode))
                self.assertEqual(stat.S_IMODE(writer_lock.stat().st_mode), 0o600)
                self.assertEqual(writer_lock.read_bytes(), b"")
                if mutation == "added":
                    (home / ".grok/unexpected").write_bytes(b"unexpected")
                elif mutation == "removed":
                    (project / ".grok/plugins/uap-inert-readonly/.grok-plugin/plugin.json").unlink()
                elif mutation == "changed":
                    writer_lock.write_bytes(b"unexpected")
                stdout = b"GROK_READONLY_NATIVE_INVENTORY_PASS\n"
            elif argv[1] == "-test.run=^TestProjectGrokNativeSavedEnablementLifecycle$":
                stdout = b"GROK_PUBLIC_NATIVE_SAVED_LIFECYCLE_PASS\n"
            else:
                raise AssertionError("unexpected inert command: " + repr(argv))
            return subprocess.CompletedProcess(argv, 0, stdout, b"")

        output = io.StringIO()
        cleanup_error = OSError("inert fixture cleanup failed") if fail_cleanup else None
        with tempfile.TemporaryDirectory(prefix="grok-driver-verdict-test-") as temporary:
            directory = Path(temporary)
            with patch.dict(scope, ARCHIVE=archive_digest, BINARY=binary_digest,
                            producer_context=lambda: dict(qualification=False)), \
                    patch("urllib.request.urlopen", return_value=io.BytesIO(archive)) as download, \
                    patch("subprocess.run", side_effect=inert_run) as native, \
                    patch("shutil.rmtree", wraps=shutil.rmtree, side_effect=cleanup_error) as cleanup, \
                    patch("argparse.ArgumentParser.parse_args", return_value=SimpleNamespace(
                        test=directory / "readonly-test", public_test=directory / "public-test",
                        output=directory)), contextlib.redirect_stdout(output):
                status = module["main"]()
            download.assert_called_once_with(module["URL"], timeout=60)
            cleanup.assert_called_once()
            self.assertEqual(bool(list(directory.glob("grok-readonly-*"))), fail_cleanup)
            evidence = json.loads((directory / "evidence.json").read_text())
            self.assertEqual(native.call_count, len(evidence["commands"]))
        printed = json.loads(output.getvalue())
        self.assertEqual(printed["passed"], evidence["passed"])
        self.assertEqual(printed.get("error"), evidence.get("error"))
        labels = [entry["label"] for entry in evidence["commands"]]
        required = ["version", "git", "native-lock", "readonly-production-inventory"]
        self.assertIn(labels, [required, required + ["public-native-saved-lifecycle"]])
        self.assertTrue(all(entry["exit"] == 0 for entry in evidence["commands"]))
        if len(labels) == 5:
            self.assertTrue(evidence["public_native_saved_lifecycle_passed"])
        else:
            self.assertNotIn("public_native_saved_lifecycle_passed", evidence)
        if mutation is None:
            if fail_cleanup:
                for field in ("fixture_unchanged", "archive_sha256", "binary_sha256"):
                    self.assertNotIn(field, evidence)
                    self.assertNotIn(field, printed)
            else:
                self.assertTrue(evidence["fixture_unchanged"])
                self.assertEqual(evidence["archive_sha256"], archive_digest)
                self.assertEqual(evidence["binary_sha256"], binary_digest)
            self.assertNotIn("fixture_delta", evidence)
        else:
            self.assertEqual(labels, required)
            self.assertEqual(printed["fixture_delta"], evidence["fixture_delta"])
        self.assertFalse(evidence["model_invoked"])
        self.assertFalse(evidence["authentication_used"])
        return status, evidence

    def test_cleanup_failure_keeps_native_evidence_and_refuses_success(self):
        status, evidence = self.run_driver(fail_cleanup=True)
        self.assertEqual(evidence["error"], "OSError: inert fixture cleanup failed")
        self.assertEqual(status, 1)
        self.assertFalse(evidence["passed"])

    def test_seeded_writer_lock_preserves_fixture_and_cleanup_allows_success(self):
        status, evidence = self.run_driver(fail_cleanup=False)
        self.assertEqual(status, 0)
        self.assertTrue(evidence["passed"])
        self.assertNotIn("error", evidence)

    def test_unexpected_fixture_changes_still_refuse_with_difference(self):
        members = dict(added=("home", ".grok/unexpected"),
                       removed=("project", ".grok/plugins/uap-inert-readonly/.grok-plugin/plugin.json"),
                       changed=("home", ".grok/config.toml.agentplugins.lock"))
        for mutation, (role, member) in members.items():
            with self.subTest(mutation=mutation):
                status, evidence = self.run_driver(mutation=mutation)
                self.assertEqual(status, 1)
                self.assertFalse(evidence["passed"])
                self.assertEqual(evidence["error"],
                                 "ValueError: readonly native inventory changed the original fixture")
                difference = evidence["fixture_delta"]
                self.assertEqual(difference["count"], 1)
                self.assertFalse(difference["truncated"])
                change, = difference["changes"]
                self.assertEqual(change, dict(role=role,
                    relative_sha256=hashlib.sha256(member.encode("utf-8")).hexdigest(),
                    known_uap_config_lock=mutation == "changed", effect=mutation,
                    inode_changed=False, mode_changed=False, content_changed=mutation == "changed"))

    def test_fixture_delta_is_bounded_and_does_not_expose_names_or_metadata(self):
        module = runpy.run_path(str(Path(__file__).with_name("e2e") / "grok-enable-readonly.py"))
        old = dict(mode=stat.S_IFREG | 0o600, inode=987654321012345, digest="a" * 64)
        new = dict(mode=stat.S_IFREG | 0o644, inode=987654321054321, digest="b" * 64)
        home, project = "/fixture/home", "/fixture/project"
        lock = ".grok/config.toml.agentplugins.lock"
        before = {home: {"unchanged": old, lock: old}, project: {}}
        after = {home: dict(before[home]), project: {}}
        after[home][lock] = new
        names = ["private-\n\x1b[31m" + "x" * 300 + str(index) for index in range(40)]
        after[home].update({name: new for name in names})
        difference = module["fixture_delta"](before, after, home, project)
        self.assertEqual(difference["count"], 41)
        self.assertTrue(difference["truncated"])
        self.assertEqual(len(difference["changes"]), 16)
        encoded = json.dumps(difference, ensure_ascii=True)
        for private in (home, project, lock, old["digest"], new["digest"],
                        str(old["inode"]), str(new["inode"]), *names):
            self.assertNotIn(json.dumps(private, ensure_ascii=True)[1:-1], encoded)
        self.assertEqual(difference["changes"][0], dict(role="home",
            relative_sha256=hashlib.sha256(lock.encode("utf-8")).hexdigest(),
            known_uap_config_lock=True, effect="changed",
            inode_changed=True, mode_changed=True, content_changed=True))
        for change in difference["changes"]:
            self.assertEqual(set(change), {"role", "relative_sha256", "known_uap_config_lock", "effect",
                                           "inode_changed", "mode_changed", "content_changed"})
            self.assertEqual(change["role"], "home")
            self.assertRegex(change["relative_sha256"], r"^[a-f0-9]{64}$")


if __name__ == "__main__":
    unittest.main()
