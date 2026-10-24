"""Inert security/contract tests; never acquires or executes a native client."""
import gzip
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import time
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location("qoder_research", Path(__file__).with_name("research-qoder-native.py"))
research = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(research)


def archive_bytes(entries):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w:gz") as archive:
        for name, kind, body in entries:
            member = tarfile.TarInfo(name)
            member.type = kind
            member.size = len(body) if kind in (tarfile.REGTYPE, tarfile.AREGTYPE) else 0
            member.linkname = "/foreign" if kind == tarfile.SYMTYPE else ""
            archive.addfile(member, io.BytesIO(body))
    return output.getvalue()


class QoderResearchTests(unittest.TestCase):
    def test_exact_regular_binary_and_archive_admission(self):
        good = [(".", tarfile.DIRTYPE, b""), ("./qodercli", tarfile.REGTYPE, b"inert bytes")]
        self.assertEqual(research.archive_binary(archive_bytes(good)), b"inert bytes")
        for entries in [
            [("../qodercli", tarfile.REGTYPE, b"x")],
            [("/qodercli", tarfile.REGTYPE, b"x")],
            [("nested/../qodercli", tarfile.REGTYPE, b"x")],
            [("qodercli", tarfile.SYMTYPE, b"")],
            [("qodercli", tarfile.FIFOTYPE, b"")],
            [("qodercli", tarfile.REGTYPE, b"")],
            good + [("other/qodercli", tarfile.REGTYPE, b"x")],
            good + [("install.sh", tarfile.REGTYPE, b"never execute")],
            [(".", tarfile.DIRTYPE, b"")],
        ]:
            with self.subTest(entries=entries), self.assertRaises(ValueError):
                research.archive_binary(archive_bytes(entries))
        with patch.object(research, "MAX_MEMBERS", 1), self.assertRaises(ValueError):
            research.archive_binary(archive_bytes(good))
        with patch.object(research, "MAX_BINARY", 2), self.assertRaises(ValueError):
            research.archive_binary(archive_bytes(good))
        with patch.object(research, "MAX_EXPANDED", 32), self.assertRaises(ValueError):
            research.archive_binary(gzip.compress(b"x" * 100))

    def test_download_limit_and_vendor_pin_precede_parsing_or_materialization(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for body, limit in [(b"not the pinned archive", 1024), (b"x" * 11, 10)]:
                with patch.object(research.urllib.request, "urlopen", return_value=io.BytesIO(body)), \
                        patch.object(research, "MAX_ARCHIVE", limit), \
                        patch.object(research, "archive_binary") as parse:
                    with self.assertRaises(ValueError):
                        research.acquire_binary(root)
                    parse.assert_not_called()
                    self.assertEqual(list(root.iterdir()), [])

    def test_runtime_has_explicit_fresh_profile_and_no_ambient_authority(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            home, temp = root / "home", root / "tmp"
            home.mkdir()
            temp.mkdir()
            ambient = {"GH_TOKEN": "inert secret", "GITHUB_TOKEN": "inert secret", "HTTP_PROXY": "http://foreign.invalid",
                       "QODER_API_KEY": "inert secret", "CLAUDE_CONFIG_DIR": "/foreign", "PATH": "/foreign/bin"}
            with patch.dict(os.environ, ambient):
                env = research.runtime_environment(home, temp)
            self.assertEqual(env["HOME"], str(home))
            self.assertEqual(env["USERPROFILE"], str(home))
            self.assertEqual(env["PATH"], "/usr/bin:/bin")
            self.assertEqual(env["TMPDIR"], str(temp))
            for key in set(ambient) - {"PATH"}:
                self.assertNotIn(key, env)
            for key in ("XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME",
                        "NPM_CONFIG_CACHE", "NPM_CONFIG_USERCONFIG", "NPM_CONFIG_GLOBALCONFIG", "GIT_CONFIG_GLOBAL"):
                self.assertTrue(Path(env[key]).is_relative_to(home))
                self.assertTrue(Path(env[key]).exists())

    def test_probe_closes_stdin_and_bounds_live_stdout(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            argv = [sys.executable, "-c", "import sys; print(repr(sys.stdin.read()))"]
            record, stdout, stderr = research.run_probe(argv, root, {"PATH": "/usr/bin:/bin"}, root, "stdin")
            self.assertEqual(record["exit"], 0)
            self.assertIsNone(record["error"])
            self.assertEqual(stdout.strip(), b"''")
            self.assertEqual(stderr, b"")
            argv = [sys.executable, "-c", "import sys; sys.stdout.buffer.write(b'x'*4096); sys.stdout.flush()"]
            record, stdout, _ = research.run_probe(argv, root, {}, root, "limit", limit=128)
            self.assertEqual(record["error"], "output limit reached")
            self.assertLessEqual(len(stdout), 128)
            self.assertFalse(research.validate_probe(record, stdout, b""))

    def test_deadline_terminates_group_including_an_inert_descendant(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            ready, escaped = root / "child-ready", root / "child-survived"
            child = ("from pathlib import Path; import time; Path(" + repr(str(ready))
                     + ").write_text('ready'); time.sleep(.5); Path(" + repr(str(escaped)) + ").write_text('survived')")
            parent = ("import subprocess,time; from pathlib import Path; subprocess.Popen(["
                      + repr(sys.executable) + ",'-c'," + repr(child) + "]); "
                      + "time.sleep(10)")
            record, _, _ = research.run_probe([sys.executable, "-c", parent], root, {}, root, "timeout", timeout=.2)
            self.assertEqual(record["error"], "probe deadline exceeded")
            self.assertTrue(ready.exists(), "inert descendant must actually start before the deadline")
            time.sleep(.6)
            self.assertFalse(escaped.exists(), "descendant outlived killed probe group")

    def test_exact_version_empty_schema_and_complete_command_allowlist(self):
        def accepted(label, body, stderr=b"", code=0, error=None):
            return research.validate_probe({"probe": label, "exit": code, "error": error}, body, stderr)
        self.assertTrue(accepted("version", b"Qoder CLI v1.1.78\n"))
        for body in [b"1.1.780", b"11.1.78", b"1.1.78-alpha", b"missing", b"\xff"]:
            self.assertFalse(accepted("version", body))
        self.assertTrue(accepted("plugins-empty-list", b"[]\n"))
        for body in [b"null", b"{}", b'[{}]', b'[] trailing', b'\xff', b'']:
            self.assertFalse(accepted("plugins-empty-list", body))
        self.assertFalse(accepted("plugins-empty-list", b"[]", b"native error"))
        self.assertFalse(accepted("plugins-empty-list", b"[]", code=1))
        self.assertFalse(accepted("plugins-empty-list", b"[]", error="output limit reached"))
        self.assertEqual([args for _, args in research.PROBES], [
            ("--version",), ("plugins", "--help"), ("plugins", "list", "--help"),
            ("plugins", "install", "--help"), ("plugins", "uninstall", "--help"),
            ("plugins", "list", "--json")])

    def test_real_hosted_guard_refuses_before_any_requested_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "not-created" / "research"
            # Fresh ordinary child environment has no hosted opt-in. The real
            # imported guard is neither patched nor supplied forged flags.
            result = subprocess.run([sys.executable, str(Path(research.__file__).resolve()), "--output", str(output)],
                                    env={"PATH": "/usr/bin:/bin"}, stdin=subprocess.DEVNULL,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10, check=False)
            self.assertEqual(result.returncode, 2)
            self.assertIn(b"disposable GitHub-hosted", result.stderr)
            self.assertFalse(output.parent.exists())
            self.assertEqual(result.stdout, b"")


if __name__ == "__main__":
    unittest.main()
