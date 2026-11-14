"""Inert contracts only: no Grok acquisition/execution or forged hosted guards."""
import importlib.util
import io
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location("grok_research", Path(__file__).with_name("research-grok-native.py"))
research = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(research)


class GrokResearchTests(unittest.TestCase):
    def test_total_deadline_interrupts_blocked_work_and_restores_signal_state(self):
        old_handler = signal.getsignal(signal.SIGALRM)
        with self.assertRaises(research.CollectorDeadline):
            with research.Budget(.03):
                time.sleep(2)
        self.assertEqual(signal.getsignal(signal.SIGALRM), old_handler)
        self.assertEqual(signal.getitimer(signal.ITIMER_REAL)[0], 0)
        class Expired:
            def check(self):
                raise research.CollectorDeadline("inert expired budget")
        source = io.BytesIO(b"must not be read")
        with self.assertRaises(research.CollectorDeadline):
            research.BudgetReader(source, Expired()).read(1)
        self.assertEqual(source.tell(), 0)

    def test_inventory_records_alias_and_special_metadata_without_following(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            inside, outside = root / "inside", root / "outside"
            inside.mkdir(); outside.mkdir()
            (outside / "secret").write_text("outside must not be read")
            (inside / "file").write_text("inert bytes")
            (inside / "alias").symlink_to(outside, target_is_directory=True)
            os.mkfifo(inside / "fifo")
            measured = research.inventory({"project-a": inside}, research.Budget())
            self.assertEqual(measured["project-a/alias"]["kind"], "symlink")
            self.assertEqual(measured["project-a/alias"]["target"], str(outside))
            self.assertEqual(measured["project-a/fifo"]["kind"], "special")
            self.assertNotIn("project-a/alias/secret", measured)
            self.assertEqual(measured["project-a/file"]["kind"], "file")
            os.link(inside / "file", inside / "hard-alias")
            with self.assertRaisesRegex(ValueError, "alias/size"):
                research.inventory({"project-a": inside}, research.Budget())

    def test_inventory_limits_and_baseline_changes_fail_without_partial_proof(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "one").write_bytes(b"12")
            (root / "two").write_bytes(b"34")
            before = research.inventory({"home": root}, research.Budget())
            for key, value in [("MAX_FILE", 1), ("MAX_TOTAL", 3), ("MAX_ENTRIES", 1), ("MAX_METADATA", 2)]:
                with self.subTest(limit=key), patch.object(research, key, value), self.assertRaises(ValueError):
                    research.inventory({"home": root}, research.Budget())
            (root / "nested").mkdir()
            with patch.object(research, "MAX_DEPTH", 0), self.assertRaises(ValueError):
                research.inventory({"home": root}, research.Budget())
            (root / "one").write_bytes(b"changed")
            after = research.inventory({"home": root}, research.Budget())
            with self.assertRaisesRegex(ValueError, "baseline"):
                research.require_baseline(after, before)

    def test_fixture_contains_only_unexecutable_manifest_and_skill_text(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            files = research.fixture(root, "uap-inert-marker", "1.0.0")
            self.assertEqual(len(files), 2)
            for path in files:
                self.assertEqual(path.stat().st_mode & 0o777, 0o644)
            manifest = json.loads(next(path for path in files if path.name == "plugin.json").read_text())
            self.assertEqual(set(manifest), {"name", "version", "description", "skills"})
            with self.assertRaises(FileExistsError):
                research.fixture(root, "another-marker", "2.0.0")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            outside, inside = root / "outside", root / "inside"
            outside.mkdir(); inside.mkdir()
            (inside / ".grok").symlink_to(outside, target_is_directory=True)
            with self.assertRaises(ValueError):
                research.fixture(inside, "marker", "1.0.0")
            self.assertEqual(list(outside.iterdir()), [])

    def test_acquisition_rejects_limits_and_wrong_pin_before_gzip_or_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for body, cap in [(b"wrong official bytes", 100), (b"12345", 4)]:
                with patch.object(research.urllib.request, "urlopen", return_value=io.BytesIO(body)), \
                        patch.object(research, "MAX_ARCHIVE", cap), patch.object(research.gzip, "GzipFile") as parse:
                    with self.assertRaises(ValueError):
                        research.acquire_binary(root, research.Budget())
                    parse.assert_not_called()
                    self.assertEqual(list(root.iterdir()), [])

    def test_validation_does_not_upgrade_failed_or_ambiguous_json(self):
        good = {"error": None, "exit": 0}
        self.assertTrue(research.validate_probe(good, b"grok 1.0.67 (inert fixture)", b"", "version"))
        self.assertFalse(research.validate_probe(good, b"grok 1.0.670", b"", "version"))
        self.assertFalse(research.validate_probe(good, b"grok 1.0.67-alpha", b"", "version"))
        self.assertTrue(research.validate_probe(good, b"[]", b"", "json"))
        for body in [b"", b"null", b"true", b"[] trailing", b'{"x":1,"x":2}', b'{"x":NaN}', b"\xff"]:
            self.assertFalse(research.validate_probe(good, body, b"", "json"))
        self.assertFalse(research.validate_probe(good, b"[]", b"error", "json"))
        self.assertFalse(research.validate_probe({"error": "deadline", "exit": 0}, b"[]", b"", "json"))
        self.assertFalse(research.validate_probe({"error": None, "exit": 1}, b"[]", b"", "json"))

    def test_all_stages_use_fixed_read_only_commands_and_selected_fresh_cwd(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for name in ("home", "tmp", "project-a", "project-b", "output"):
                (root / name).mkdir()
            with patch.dict(os.environ, {"GROK_API_KEY": "inert secret", "GH_TOKEN": "inert secret", "HTTP_PROXY": "foreign"}):
                env = research.runtime_environment(root / "home", root / "tmp")
            self.assertNotIn("GROK_API_KEY", env)
            self.assertNotIn("GH_TOKEN", env)
            self.assertNotIn("HTTP_PROXY", env)
            calls = []
            def inert_probe(argv, cwd, child_env, output, label, **kwargs):
                calls.append((argv, cwd, child_env, label))
                record = {"probe": label, "exit": 0, "error": None}
                # Deliberately inert test observations, never native proof.
                body = b"grok 1.0.67" if label == "version" else b'{"inert fixture":true}'
                return record, body, b""
            summary = {"probes": [], "fixture_preparation": [], "inventories": []}
            with patch.object(research, "run_probe", side_effect=inert_probe):
                research.collect(Path("/never-executed-grok"), root, env, root / "output", summary, research.Budget())
            self.assertEqual([entry["probe"] for entry in summary["probes"]], research.EXPECTED_PROBES)
            self.assertEqual(len(summary["probes"]), 19)
            self.assertEqual(len(summary["fixture_preparation"]), 2)
            self.assertEqual([entry["stage"] for entry in summary["inventories"]], list(research.STAGES))
            allowed = {tuple(args) for _, args in (*research.HELP_PROBES, *research.JSON_PROBES)}
            for argv, cwd, child_env, label in calls:
                self.assertEqual(child_env, env)
                self.assertIn(cwd, [root / "project-a", root / "project-b"])
                if label.startswith("git-init"):
                    self.assertEqual(argv, ["/usr/bin/git", "init", "--quiet", "--template="])
                else:
                    self.assertEqual(argv[0], "/never-executed-grok")
                    self.assertIn(tuple(argv[1:]), allowed)
            for owner, marker, version in [("home", "uap-research-user", "0.9.0"), ("project-a", "uap-research-a", "1.0.0"),
                                           ("project-b", "uap-research-b", "2.0.0")]:
                plugin = root / owner / ".grok/plugins" / research.PLUGIN_NAME
                self.assertEqual(json.loads((plugin / ".grok-plugin/plugin.json").read_text())["version"], version)
                self.assertTrue((plugin / "skills" / marker / "SKILL.md").is_file())

    def test_real_guard_refuses_without_any_output_or_native_admission(self):
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
