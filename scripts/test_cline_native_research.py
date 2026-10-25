"""Inert contracts only. Never acquires/executes Cline or forges hosted admission."""
import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location("cline_research", Path(__file__).with_name("research-cline-native.py"))
research = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(research)


def archive_bytes(entries):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w:gz") as archive:
        for name, kind, body in entries:
            member = tarfile.TarInfo(name)
            member.type = kind
            member.size = len(body) if kind in (tarfile.REGTYPE, tarfile.AREGTYPE) else 0
            archive.addfile(member, io.BytesIO(body))
    return output.getvalue()


class ClineResearchTests(unittest.TestCase):
    def test_only_exact_binary_is_read_and_assets_are_never_materialized(self):
        entries = [("package/package.json", tarfile.REGTYPE, b"{}"),
                   ("package/bin/cline", tarfile.REGTYPE, b"inert"),
                   ("package/extensions/not-executed.js", tarfile.REGTYPE, b"never execute")]
        with patch.object(research, "BINARY_BYTES", 5):
            self.assertEqual(research.archive_binary(archive_bytes(entries), research.grok.Budget()), b"inert")
            for unsafe in ("/package/bin/cline", "package/../bin/cline", "package/./bin/cline",
                           "package/bin/../cline", "package\\bin\\cline", "package/bin/cline\n"):
                with self.subTest(name=unsafe), self.assertRaises(ValueError):
                    research.archive_binary(archive_bytes([(unsafe, tarfile.REGTYPE, b"inert")]), research.grok.Budget())
            for kind in (tarfile.SYMTYPE, tarfile.LNKTYPE, tarfile.FIFOTYPE):
                with self.subTest(kind=kind), self.assertRaises(ValueError):
                    research.archive_binary(archive_bytes([("package/bin/cline", kind, b"")]), research.grok.Budget())
            for invalid in (entries + [("package/bin/cline", tarfile.REGTYPE, b"inert")],
                            entries + [("PACKAGE/bin/CLINE", tarfile.REGTYPE, b"inert")],
                            [("package/bin/cline", tarfile.REGTYPE, b"wrong size")], entries[:1]):
                with self.assertRaises(ValueError):
                    research.archive_binary(archive_bytes(invalid), research.grok.Budget())
        for key, limit in (("MAX_MEMBERS", 1), ("MAX_BINARY", 2), ("MAX_EXPANDED", 32)):
            with patch.object(research, key, limit), self.assertRaises(ValueError):
                research.archive_binary(archive_bytes(entries), research.grok.Budget())

    def test_archive_pin_and_actual_read_limit_precede_parse_or_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for body, cap in ((b"wrong pinned archive", 100), (b"12345", 4)):
                with patch.object(research.urllib.request, "urlopen", return_value=io.BytesIO(body)), \
                        patch.object(research, "MAX_ARCHIVE", cap), patch.object(research, "archive_binary") as parse:
                    with self.assertRaises(ValueError):
                        research.acquire_binary(root, research.grok.Budget())
                    parse.assert_not_called()
                    self.assertEqual(list(root.iterdir()), [])
            inert = gzip.compress(b"inert bytes; never executed")
            pin = "sha256:" + hashlib.sha256(inert).hexdigest()
            with patch.object(research.urllib.request, "urlopen", return_value=io.BytesIO(inert)), \
                    patch.object(research, "ARCHIVE_PIN", pin), \
                    patch.object(research, "archive_binary", return_value=b"untrusted inert binary"):
                with self.assertRaisesRegex(ValueError, "binary integrity"):
                    research.acquire_binary(root, research.grok.Budget())
                self.assertEqual(list(root.iterdir()), [])

    def test_explicit_environment_suppresses_updater_and_rejects_ambient_authority(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            home, temp = root / "home", root / "tmp"
            home.mkdir(); temp.mkdir()
            ambient = {"GH_TOKEN": "inert", "GITHUB_TOKEN": "inert", "HTTP_PROXY": "foreign",
                       "CLINE_DIR": "/foreign", "CLINE_DATA_DIR": "/foreign", "CLINE_API_KEY": "inert",
                       "CLINE_MCP_SETTINGS_PATH": "/foreign", "CLINE_NO_AUTO_UPDATE": "0"}
            with patch.dict(os.environ, ambient):
                env = research.runtime_environment(home, temp)
            self.assertEqual(env["CLINE_NO_AUTO_UPDATE"], "1")
            self.assertEqual(env["HOME"], str(home))
            self.assertEqual(env["PATH"], "/usr/bin:/bin")
            for key in set(ambient) - {"CLINE_NO_AUTO_UPDATE"}:
                self.assertNotIn(key, env)
            paths = {name: root / name / "mcp.json" for name in research.PROJECTS}
            for override in research.PROJECTS:
                selected = research.child_environment(env, paths, override)
                self.assertEqual(selected["CLINE_MCP_SETTINGS_PATH"], str(paths[override]))
                self.assertEqual(selected["CLINE_NO_AUTO_UPDATE"], "1")
            self.assertNotIn("CLINE_MCP_SETTINGS_PATH", research.child_environment(env, paths, None))
            for bad in ({}, {**env, "CLINE_NO_AUTO_UPDATE": "0"}, {**env, "CLINE_MCP_SETTINGS_PATH": "/foreign"}):
                with self.assertRaises(ValueError):
                    research.child_environment(bad, paths, None)
            with self.assertRaises(ValueError):
                research.child_environment(env, paths, "foreign")

    def test_fixtures_are_disabled_remote_config_and_plain_skill_text_only(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for name in ("home", *research.PROJECTS):
                (root / name).mkdir()
            paths = research.prepare_fixtures(root)
            files = [path for path in root.rglob("*") if path.is_file()]
            self.assertEqual(len(files), 6)
            for path in files:
                self.assertEqual(path.stat().st_mode & 0o111, 0)
                self.assertIn(path.suffix, (".md", ".json"))
            for owner, path in paths.items():
                entries = json.loads(path.read_text())["mcpServers"]
                self.assertEqual(len(entries), 1)
                entry = next(iter(entries.values()))
                self.assertIs(entry["disabled"], True)
                self.assertIn(entry["transport"]["type"], ("streamableHttp", "sse"))
                self.assertTrue(entry["transport"]["url"].startswith("http://127.0.0.1:9/"))
                self.assertNotIn("command", entry["transport"])
                self.assertNotIn("oauth", entry)
                self.assertTrue(path.is_relative_to(root / owner))
            with self.assertRaises(FileExistsError):
                research.prepare_fixtures(root)

    def test_finite_argv_cwd_and_process_override_sequence_are_explicit(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for name in ("home", "tmp", "project-a", "project-b", "output"):
                (root / name).mkdir()
            env = research.runtime_environment(root / "home", root / "tmp")
            calls = []
            def inert_probe(argv, cwd, child_env, output, label, **kwargs):
                calls.append((argv, cwd, child_env, label, kwargs))
                body = b"3.0.73" if label == "version" else b"[]" if "--json" in argv else b"inert help"
                return {"probe": label, "exit": 0, "error": None}, body, b""
            summary = {"probes": [], "fixture_preparation": [], "inventories": {}}
            with patch.object(research, "run_probe", side_effect=inert_probe):
                research.collect(Path("/never-executed-cline"), root, env, root / "output", summary, research.grok.Budget())
            self.assertEqual([row["probe"] for row in summary["probes"]], research.EXPECTED_PROBES)
            self.assertEqual(len(summary["probes"]), 12)
            self.assertEqual(len(summary["fixture_preparation"]), 2)
            self.assertTrue(all(row["accepted"] for row in summary["probes"]))
            expected_overrides = {name: override for name, _, override in research.MCP_PROBES}
            for argv, cwd, child_env, label, kwargs in calls:
                self.assertIn(cwd, (root / "project-a", root / "project-b"))
                self.assertLessEqual(kwargs["timeout"], 30)
                if label.startswith("git-init"):
                    self.assertEqual(argv, ["/usr/bin/git", "init", "--quiet", "--template="])
                    continue
                self.assertEqual(argv[:5], ["/never-executed-cline", "--auto-approve", "false", "--cwd", str(cwd)])
                self.assertEqual(child_env["CLINE_NO_AUTO_UPDATE"], "1")
                override = expected_overrides.get(label)
                if override is None:
                    self.assertNotIn("CLINE_MCP_SETTINGS_PATH", child_env)
                else:
                    self.assertEqual(child_env["CLINE_MCP_SETTINGS_PATH"], str(root / override / ".uap/clients/cline/mcp.json"))
            self.assertEqual(summary["inventories"]["before"], summary["inventories"]["after"])

    def test_each_read_only_probe_must_preserve_the_fixture_baseline(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for name in ("home", "tmp", "project-a", "project-b", "output"):
                (root / name).mkdir()
            env = research.runtime_environment(root / "home", root / "tmp")
            def changing_probe(_argv, _cwd, _env, _output, label, **_kwargs):
                if label == "version":
                    (root / "project-a/.uap/clients/cline/mcp.json").write_text("changed by inert probe")
                return {"probe": label, "exit": 0, "error": None}, b"3.0.73", b""
            with patch.object(research, "run_probe", side_effect=changing_probe), self.assertRaisesRegex(ValueError, "baseline"):
                research.collect(Path("/never-executed"), root, env, root / "output",
                                 {"probes": [], "fixture_preparation": [], "inventories": {}}, research.grok.Budget())

    def test_failed_or_ambiguous_output_is_not_collection_evidence(self):
        good = {"error": None, "exit": 0}
        self.assertTrue(research.validate_probe(good, b"Cline 3.0.73", b"", "version"))
        for body in (b"3.0.730", b"13.0.73", b"3.0.73-alpha", b"missing"):
            self.assertFalse(research.validate_probe(good, body, b"", "version"))
        self.assertTrue(research.validate_probe(good, b'[{"name":"inert"}]', b"", "json"))
        for body in (b"", b"{}", b"null", b"[1]", b"[] trailing", b'[{"x":1,"x":2}]', b'[{"x":NaN}]', b"\xff"):
            self.assertFalse(research.validate_probe(good, body, b"", "json"))
        self.assertFalse(research.validate_probe(good, b"[]", b"native error", "json"))
        self.assertFalse(research.validate_probe({"error": "deadline", "exit": 0}, b"[]", b"", "json"))
        self.assertFalse(research.validate_probe({"error": None, "exit": 1}, b"[]", b"", "json"))

    def test_real_unmodified_hosted_guard_has_zero_requested_output_effects(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "not-created" / "research"
            result = subprocess.run([sys.executable, str(Path(research.__file__).resolve()), "--output", str(output)],
                                    env={"PATH": "/usr/bin:/bin"}, stdin=subprocess.DEVNULL,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10, check=False)
            self.assertEqual(result.returncode, 2)
            self.assertIn(b"disposable GitHub-hosted", result.stderr)
            self.assertEqual(result.stdout, b"")
            self.assertFalse(output.parent.exists())


if __name__ == "__main__":
    unittest.main()
