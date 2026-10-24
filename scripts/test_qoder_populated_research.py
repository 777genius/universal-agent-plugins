"""Inert Qoder scope collector contracts; no vendor execution or guard spoofing."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import mock_open, patch

SPEC = importlib.util.spec_from_file_location("qoder_scope_research", Path(__file__).with_name("research-qoder-populated.py"))
research = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(research)


class CollectorTests(unittest.TestCase):
    def request(self, root, output):
        for name in ("home", "tmp", "binary", *research.LOCATIONS):
            (root / name).mkdir(parents=True)
        sources = research.fixtures(root)
        return {"root": str(root), "output": str(output), "binary": str(root / "binary/qoder"),
                "sources": {key: str(value) for key, value in sources.items()},
                "fixture_pins": research.fixture_pins(sources, research.grok.Budget()),
                "source_baseline": research.inventory({"source-" + key: value for key, value in sources.items()}, research.grok.Budget()),
                "native_environment": research.phase1.runtime_environment(root / "home", root / "tmp"),
                "parent_namespace": "net:[old]", "uid": 1001, "gid": 1001}

    def fake_probe(self, calls, fail=None):
        def run(argv, cwd, env, output, label, **kwargs):
            calls.append((list(argv), Path(cwd), dict(env), label))
            record = {"probe": label, "exit": 0, "error": None}
            if label == fail:
                record["exit"] = 1
                return record, b"", b"inert injected failure"
            if label == "version":
                body = b"Qoder 1.1.78\n"
            elif label.endswith("-help"):
                body = b"Usage\n"
            elif label.endswith("-install"):
                body = b'{"unknownNativeFields":"preserved without interpretation"}'
            else:
                body = b'[]' if label.startswith("empty-") else b'[{"unknownNativeFields":true}]'
            return record, body, b""
        return run

    def test_finite_stages_exact_argv_and_stop_after_first_failed_mutation(self):
        for fail in (None, "user-installed-install"):
            with self.subTest(fail=fail), tempfile.TemporaryDirectory() as temporary:
                base = Path(temporary); root = base / "root"; output = base / "output"; output.mkdir()
                request = self.request(root, output); summary = research.summary_record(); calls = []
                with patch.object(research, "isolation_facts", return_value={"verified": True}), \
                        patch.object(research, "check_pins"), patch.object(research, "run_probe", side_effect=self.fake_probe(calls, fail)):
                    if fail:
                        with self.assertRaises(ValueError):
                            research.collect(request, summary, research.grok.Budget())
                        self.assertFalse(summary["success"])
                        self.assertFalse(summary["probes"][-1]["accepted"])
                    else:
                        research.collect(request, summary, research.grok.Budget())
                        self.assertTrue(summary["success"])
                        self.assertEqual(len(calls), 19)
                        installs = [(argv[1:], cwd.name) for argv, cwd, _, label in calls if label.endswith("-install")]
                        self.assertEqual(installs, [(["plugins", "install", request["sources"][key], "--scope", scope, "--json"], location)
                                                  for key, scope, location in [("user", "user", "user-cwd"), ("A", "project", "project-A"), ("B", "project", "project-B")]])
                        self.assertEqual(len([x for x in calls if x[0][1:] == ["plugins", "list", "--json"]]), 12)
                    self.assertTrue(all(summary[key] is False for key in research.FALSE_CLAIMS))
                    self.assertTrue(all(env == request["native_environment"] for _, _, env, _ in calls))
                    self.assertFalse(any(any(word in argv for word in ["uninstall", "enable", "disable", "trust", "login", "run"]) for argv, _, _, _ in calls if "--help" not in argv))

    def test_text_only_sources_and_request_rejects_foreign_authority(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); output = base / "output"; output.mkdir(); request = self.request(base / "root", output)
            research.validate_request(request)
            paths = [p for p in (base / "root/sources").rglob("*") if p.is_file()]
            self.assertEqual(len(paths), 6)
            self.assertEqual({p.suffix for p in paths}, {".json", ".md"})
            for change in [lambda r: r["native_environment"].update({"GITHUB_TOKEN": "inert"}),
                           lambda r: r["native_environment"].update({"HOME": "/foreign"}),
                           lambda r: r["sources"].update({"A": "/foreign"}),
                           lambda r: r.update({"binary": "/foreign/qoder"}),
                           lambda r: r.update({"output": r["root"]}),
                           lambda r: r["fixture_pins"].clear()]:
                altered = copy.deepcopy(request); change(altered)
                with self.assertRaises(ValueError): research.validate_request(altered)

    def test_strict_json_and_dirty_unknown_output_refusal(self):
        for body in [b'{"a":1,"a":2}', b'[] trailing', b'NaN', b'\xff']:
            with self.assertRaises((ValueError, UnicodeError)): research.strict_json(body)
        record = {"error": None, "exit": 0}
        for body, kind, stage in [(b'{}', "list", "empty"), (b'[{}]', "list", "empty"),
                                  (b'[]', "list", "user-installed"), (b'null', "install", "A-installed")]:
            with self.assertRaises(ValueError): research.validate_output(record, body, b"", kind, stage)
        for dirty in [{"error": "limit", "exit": 0}, {"error": None, "exit": 1}]:
            with self.assertRaises(ValueError): research.validate_output(dirty, b'[]', b"", "list", "empty")
        with self.assertRaises(ValueError): research.validate_output(record, b'[]', b"warning", "list", "empty")
        self.assertEqual(research.validate_output(record, b'[{"future":true}]', b"", "list", "A-installed"), [{"future": True}])

    def test_inventory_refuses_alias_special_bounds_and_preserves_partial_metadata(self):
        for kind in ["link", "fifo", "large", "hardlink"]:
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary); (root / "a").write_bytes(b"inert")
                if kind == "link": (root / "z").symlink_to(root / "a")
                if kind == "fifo": os.mkfifo(root / "z")
                if kind == "large": (root / "z").write_bytes(b"x" * 33)
                if kind == "hardlink": os.link(root / "a", root / "z")
                start = time.monotonic()
                with patch.object(research, "MAX_FILE", 32), self.assertRaises(research.InventoryError) as raised:
                    research.inventory({"root": root}, research.grok.Budget())
                self.assertTrue(raised.exception.partial)
                self.assertLess(time.monotonic() - start, 1)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); (root / "a").write_bytes(b"x")
            for attr in ["MAX_ENTRIES", "MAX_TOTAL"]:
                with patch.object(research, attr, 0), self.assertRaises(research.InventoryError):
                    research.inventory({"root": root}, research.grok.Budget())
            (root / "d").mkdir(); (root / "d/e").mkdir()
            with patch.object(research, "MAX_DEPTH", 0), self.assertRaises(research.InventoryError):
                research.inventory({"root": root}, research.grok.Budget())

    def test_inventory_preopen_symlink_swap_never_reads_foreign_file(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); watched = root / "a"; watched.write_bytes(b"owned")
            real_open = os.open
            def race(path, flags, *args, **kwargs):
                if path == "a":
                    watched.unlink(); watched.symlink_to("/foreign-inert")
                return real_open(path, flags, *args, **kwargs)
            with patch.object(research.os, "open", side_effect=race), self.assertRaises(research.InventoryError):
                research.inventory({"root": root}, research.grok.Budget())

    def test_isolation_verifier_checks_namespace_user_groups_caps_and_nnp(self):
        good = "Groups:\t\nNoNewPrivs:\t1\n" + "\n".join(key + ":\t0000" for key in ["CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb"])
        for failure in [None, "namespace", "interfaces", "uid", "groups", "caps", "nnp"]:
            with self.subTest(failure=failure), patch("builtins.open", mock_open(read_data=good.replace("CapEff:\t0000", "CapEff:\t0001") if failure == "caps" else good.replace("NoNewPrivs:\t1", "NoNewPrivs:\t0") if failure == "nnp" else good)), \
                    patch.object(research.os, "getuid", return_value=0 if failure == "uid" else 1001), \
                    patch.object(research.os, "getgid", return_value=1001), \
                    patch.object(research.os, "geteuid", return_value=1001), \
                    patch.object(research.os, "getegid", return_value=1001), \
                    patch.object(research.os, "getgroups", return_value=[1001] if failure == "groups" else []), \
                    patch.object(research.os, "readlink", return_value="net:[old]" if failure == "namespace" else "net:[new]"), \
                    patch.object(research.socket, "if_nameindex", return_value=[(1, "lo"), (2, "eth0")] if failure == "interfaces" else [(1, "lo")]):
                if failure:
                    with self.assertRaises(ValueError): research.isolation_facts("net:[old]", 1001, 1001)
                else:
                    self.assertEqual(research.isolation_facts("net:[old]", 1001, 1001)["interfaces"], ["lo"])

    def test_fixture_binary_tamper_and_deadline_refuse_without_vendor_execution(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); output = base / "output"; output.mkdir(); request = self.request(base / "root", output)
            binary = Path(request["binary"]); binary.write_bytes(b"inert")
            import hashlib
            with patch.object(research, "BINARY_BYTES", 5), patch.object(research.phase1, "BINARY_SHA256", hashlib.sha256(b"inert").hexdigest()):
                research.check_pins(request, research.grok.Budget())
                added = Path(request["sources"]["A"]) / "extra-hook.json"
                added.write_text("inert addition")
                with self.assertRaisesRegex(ValueError, "complete inert source"):
                    research.check_pins(request, research.grok.Budget())
                added.unlink()
                source = Path(next(iter(request["fixture_pins"]))); source.write_text("tampered")
                with self.assertRaises(ValueError): research.check_pins(request, research.grok.Budget())
            with self.assertRaises(ValueError): research.check_pins(request, research.grok.Budget())
            budget = research.grok.Budget(.001)
            with patch.object(research.grok.time, "monotonic", return_value=budget.deadline + 1), self.assertRaises(research.grok.CollectorDeadline):
                research.inventory({"root": base / "root"}, budget)

    def test_real_hosted_guard_refuses_before_any_effect(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "must-not-exist"
            result = subprocess.run([sys.executable, str(Path(research.__file__)), "--output", str(output)],
                                    env={"PATH": os.environ.get("PATH", "/usr/bin:/bin"), "PYTHONDONTWRITEBYTECODE": "1"}, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 2)
            self.assertIn(b"disposable GitHub-hosted", result.stderr)
            self.assertFalse(output.exists())

    def test_wrapper_carries_only_actual_hosted_fields_and_drops_authority(self):
        request = {"uid": 1001, "gid": 1002}
        with patch.dict(os.environ, {"GITHUB_TOKEN": "inert", "HTTP_PROXY": "inert"}):
            command = research.wrapper_command(Path("/inert/request"), request)
        self.assertIn("--net", command); self.assertIn("--clear-groups", command)
        self.assertIn("--no-new-privs", command); self.assertIn("--bounding-set=-all", command)
        self.assertFalse(any("GITHUB_TOKEN=" in arg or "HTTP_PROXY=" in arg for arg in command))
        self.assertNotIn("--scope", command)


if __name__ == "__main__":
    unittest.main()
