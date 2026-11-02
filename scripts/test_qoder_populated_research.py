"""Inert Qoder scope collector contracts; no vendor execution or guard spoofing."""
import copy
import hashlib
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
from types import SimpleNamespace

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

    def test_exact_native_log_alias_is_only_bounded_literal_metadata(self):
        for target_kind in ("file", "directory", "fifo", "dangling", "self", "nonutf8"):
            with self.subTest(target_kind=target_kind), tempfile.TemporaryDirectory() as temporary:
                base = Path(temporary); home = base / "home"; logs = home / ".qoder/logs"; logs.mkdir(parents=True)
                foreign = base / "foreign"
                if target_kind == "file": foreign.write_text("foreign data must never be read")
                if target_kind == "directory": foreign.mkdir()
                if target_kind == "fifo": os.mkfifo(foreign)
                target = os.fsencode(foreign) if target_kind not in ("self", "nonutf8") else b"latest" if target_kind == "self" else b"\xff-literal"
                alias = logs / "latest"; os.symlink(target, os.fsencode(alias))
                real_open, real_stat = os.open, os.stat
                def no_target_open(path, *args, **kwargs):
                    self.assertNotIn(path, (str(foreign), os.fsencode(foreign), target))
                    return real_open(path, *args, **kwargs)
                def no_target_stat(path, *args, **kwargs):
                    self.assertNotIn(path, (str(foreign), os.fsencode(foreign), target))
                    return real_stat(path, *args, **kwargs)
                with patch.object(research.os, "open", side_effect=no_target_open), patch.object(research.os, "stat", side_effect=no_target_stat):
                    result = research.inventory({"home": home}, research.grok.Budget(), native_log_alias=True)
                record = result["home/.qoder/logs/latest"]
                self.assertEqual(record["kind"], "symlink_metadata_only")
                self.assertFalse(record["target_followed"])
                self.assertEqual(record["target_bytes"], len(target))
                self.assertEqual(bytes.fromhex(record["target_hex"]), target)
                self.assertEqual(record["target_sha256"], hashlib.sha256(target).hexdigest())
                with self.assertRaises(research.InventoryError):
                    research.inventory({"home": home}, research.grok.Budget())
                for label in ("source-user", "source-A", "source-B", "another-home"):
                    with self.assertRaises(research.InventoryError):
                        research.inventory({label: home}, research.grok.Budget(), native_log_alias=True)
                with patch.object(research, "MAX_TOTAL", len(target) - 1), self.assertRaises(research.InventoryError):
                    research.inventory({"home": home}, research.grok.Budget(), native_log_alias=True)

    def test_native_log_alias_admission_refuses_before_read_and_rechecks_bytes_identity_budget(self):
        for failure in ("oversize", "negative", "hardlink", "length", "replacement", "deadline"):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temporary:
                base = Path(temporary); home = base / "home"; logs = home / ".qoder/logs"; logs.mkdir(parents=True)
                alias = logs / "latest"; alias.symlink_to("inert-target")
                if failure == "hardlink": os.link(alias, base / "outside-alias", follow_symlinks=False)
                budget = research.grok.Budget()
                real_stat, real_readlink = os.stat, os.readlink
                calls = []
                def metadata(path, *args, **kwargs):
                    info = real_stat(path, *args, **kwargs)
                    if path == "latest" and failure in ("oversize", "negative"):
                        fields = {name: getattr(info, name) for name in ("st_dev", "st_ino", "st_mode", "st_nlink", "st_size", "st_mtime_ns", "st_ctime_ns")}
                        fields["st_size"] = research.MAX_METADATA + 1 if failure == "oversize" else -1
                        return SimpleNamespace(**fields)
                    return info
                def link(path, *args, **kwargs):
                    calls.append(path)
                    self.assertIsInstance(path, bytes)
                    if failure in ("oversize", "negative", "hardlink"):
                        self.fail("inadmissible metadata reached readlink")
                    target = real_readlink(path, *args, **kwargs)
                    if failure == "length": return target + b"x"
                    if failure == "replacement": alias.unlink(); alias.symlink_to("replacement")
                    if failure == "deadline": budget.deadline = time.monotonic() - 1
                    return target
                with patch.object(research.os, "stat", side_effect=metadata), patch.object(research.os, "readlink", side_effect=link), self.assertRaises(research.InventoryError) as raised:
                    research.inventory({"home": home}, budget, native_log_alias=True)
                self.assertEqual(raised.exception.partial["home/.qoder/logs/latest"]["kind"], "refused")
                self.assertEqual(len(calls), 0 if failure in ("oversize", "negative", "hardlink") else 1)

    def test_native_log_alias_optin_never_admits_an_intermediate_or_other_alias(self):
        for intermediate in (False, True):
            with self.subTest(intermediate=intermediate), tempfile.TemporaryDirectory() as temporary:
                base = Path(temporary); home = base / "home"; (home / ".qoder").mkdir(parents=True)
                foreign = base / "foreign"; foreign.mkdir()
                if intermediate: (home / ".qoder/logs").symlink_to(foreign, target_is_directory=True)
                else:
                    (home / ".qoder/logs").mkdir()
                    (home / ".qoder/logs/not-latest").symlink_to("inert")
                with patch.object(research.os, "readlink", side_effect=AssertionError("unexpected alias readlink")), self.assertRaises(research.InventoryError):
                    research.inventory({"home": home}, research.grok.Budget(), native_log_alias=True)

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

    def test_cleanup_is_anchored_bounded_and_preserves_primary_deadline_failure(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); output = base / "raw"; output.mkdir()
            root = base / "root"; root.mkdir(); foreign = base / "foreign"; foreign.write_text("preserve")
            (root / "link").symlink_to(foreign)
            self.assertTrue(research.cleanup(root, root.lstat(), output)["complete"])
            self.assertFalse(root.exists()); self.assertEqual(foreign.read_text(), "preserve")
            root.mkdir(); held = root.lstat(); root.rename(base / "original"); root.mkdir()
            self.assertFalse(research.cleanup(root, held, output)["complete"])
            self.assertTrue(root.is_dir()); self.assertTrue((base / "original").is_dir())
            root.rmdir()
            summary = research.summary_record()
            with patch.object(research, "verify_checkout"), \
                    patch.object(research.os, "getuid", return_value=1001), \
                    patch.object(research.os, "getgid", return_value=1001), \
                    patch.object(research.os, "geteuid", return_value=1001), \
                    patch.object(research.os, "getegid", return_value=1001), \
                    patch.object(research.phase1.tempfile, "mkdtemp", return_value=str(base / "experiment")), \
                    patch.object(research, "acquire", side_effect=research.grok.CollectorDeadline("inert deadline")), \
                    patch.object(research, "cleanup", return_value={"complete": False, "error": "inert cleanup timeout"}) as clean:
                # mkdtemp is replaced only with an inert precreated test root.
                (base / "experiment").mkdir()
                research.run_collection(output, summary)
            self.assertFalse(summary["success"])
            self.assertIn("inert deadline", summary["error"])
            self.assertFalse(summary["cleanup"]["complete"])
            self.assertEqual(clean.call_count, 1)
            self.assertEqual(json.loads((output / "qoder-populated-research.json").read_text())["error"], summary["error"])
            calls = []
            def timed(argv, cwd, env, output, label, **kwargs):
                calls.append(kwargs)
                return {"exit": -9, "error": "inert deadline"}, b"", b""
            with patch.object(research, "run_probe", side_effect=timed):
                self.assertFalse(research.cleanup(base / "experiment", (base / "experiment").lstat(), output)["complete"])
            self.assertEqual(calls, [{"timeout": 5}])

    def test_upload_boundary_never_exposes_rejected_oversize_or_nonregular_tree(self):
        for refused in [None, "overflow", "link", "hardlink", "directory"]:
            with self.subTest(refused=refused), tempfile.TemporaryDirectory() as temporary:
                base = Path(temporary); raw = base / "raw"; raw.mkdir(); upload = base / "upload"
                (raw / "safe.log").write_text("inert")
                if refused == "overflow": (raw / "large.log").write_bytes(b"x" * 5000)
                if refused == "link": (raw / "z").symlink_to("/foreign")
                if refused == "hardlink": os.link(raw / "safe.log", raw / "z")
                if refused == "directory": (raw / "z").mkdir()
                summary = research.summary_record(); summary["success"] = True
                with patch.object(research, "MAX_ARTIFACT", 4096):
                    result = research.stage_artifacts(raw, upload, summary)
                if refused:
                    self.assertFalse(result["complete"]); self.assertFalse(summary["success"])
                    self.assertEqual([p.name for p in upload.iterdir()], ["qoder-populated-research.json"])
                    fallback = json.loads((upload / "qoder-populated-research.json").read_text())
                    self.assertFalse(fallback["success"])
                    self.assertTrue(all(fallback[key] is False for key in research.FALSE_CLAIMS))
                else:
                    self.assertTrue(result["complete"])
                    self.assertEqual((upload / "safe.log").read_text(), "inert")
                self.assertLess(sum(p.stat().st_size for p in upload.iterdir()), 4096)
        workflow = Path(research.__file__).parent.parent / ".github/workflows/qoder-populated-research.yml"
        self.assertIn("path: ${{ runner.temp }}/qoder-populated-upload/", workflow.read_text())
        self.assertNotIn("path: ${{ runner.temp }}/qoder-populated-research/", workflow.read_text())

    def test_source_head_is_observed_and_mismatch_stops_before_acquisition(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            for observed in ["a" * 40, "b" * 40]:
                summary = research.summary_record()
                with patch.dict(os.environ, {"EXPECTED_COMMIT": "a" * 40}), \
                        patch.object(research, "run_probe", return_value=({"exit": 0, "error": None}, (observed + "\n").encode(), b"")) as probe:
                    if observed == "a" * 40:
                        research.verify_checkout(output, summary, research.grok.Budget())
                        self.assertEqual(summary["source_commit"], observed)
                    else:
                        with self.assertRaisesRegex(ValueError, "actual git HEAD"):
                            research.verify_checkout(output, summary, research.grok.Budget())
                    self.assertEqual(probe.call_args[0][0], ["/usr/bin/git", "rev-parse", "HEAD"])

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
