"""Bounded refusal diagnostics preserve private paths and original failures."""
from contextlib import redirect_stdout
import errno
import hashlib
import io
import importlib.util
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

from test_live_install_audit_env import load_audit

MODULE_PATH = Path(__file__).with_name("remotion_published_windows_long_temp_e2e.py")
SPEC = importlib.util.spec_from_file_location("remotion_diagnostics", MODULE_PATH)
AUDIT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(AUDIT)


class RemotionFailureDiagnosticsTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.repo = Path(self.temporary.name)
        (self.repo / "scripts").mkdir()
        self.root = self.repo / "private-profile-token-SECRET"

    def raised(self, filename, function, bindings, expression):
        namespace = dict(bindings)
        source = "def " + function + "():\n" + expression + "\n"
        exec(compile(source, str(self.repo / "scripts" / filename), "exec"), namespace)
        try:
            namespace[function]()
        except Exception as error:
            return error
        self.fail("fixture did not raise")

    def test_snapshot_reports_held_unreadable_key_hash_and_fixed_root_role(self):
        protected = self.root / "skill-24/home"
        key = "home/.codex/private-SECRET"
        error = self.raised("live-install-audit.py", "scoped_snapshot",
                            {"held": {key: "unreadable"}, "protected": protected},
                            "    state = held\n    roots = (protected,)\n"
                            "    raise AssertionError('private exception SECRET')")
        facts = AUDIT.sanitized_failure_diagnostics(error, self.repo, self.root)
        guard = facts["guards"][0]
        self.assertEqual(guard["category"], "protected-snapshot-unreadable")
        self.assertEqual(guard["unreadable_count"], 1)
        self.assertEqual(guard["entries"][0]["key_sha256"], hashlib.sha256(key.encode()).hexdigest())
        self.assertEqual(guard["entries"][0]["root_candidates"][0]["role"], "home")
        self.assertEqual(facts["source_chain"][-1]["function"], "scoped_snapshot")
        self.assertNotIn("SECRET", json.dumps(facts))
        self.assertNotIn(str(self.root), json.dumps(facts))
        self.assertIsInstance(error, AssertionError)

    def test_snapshot_caps_records_without_reading_objects_or_exporting_values(self):
        class Poison:
            def __str__(self):
                raise AssertionError("must not stringify")
            __repr__ = __str__
        state = {f"home/private-{i}": "unreadable" for i in range(20)}
        state["home/secret-value"] = Poison()
        state[Poison()] = "unreadable"
        facts = AUDIT.snapshot_failure_facts(
            {"state": state, "roots": (self.root / "skill-24/home",)}, self.root)
        self.assertEqual(facts["unreadable_count"], 20)
        self.assertEqual(len(facts["entries"]), 16)
        self.assertEqual(facts["entries_omitted"], 4)
        self.assertNotIn("private-", json.dumps(facts))

    def test_directory_uses_held_metadata_without_filesystem_reprobe(self):
        class NoProbePath(type(Path())):
            def resolve(self, *args, **kwargs):
                raise AssertionError("must not resolve refused directory")
            def lstat(self, *args, **kwargs):
                raise AssertionError("must not restat refused directory")
            def is_symlink(self):
                raise AssertionError("must not reprobe refused directory")
        path = NoProbePath(self.root / "skill-24/home/.local/state/codex-sqlite")
        info = self.repo.stat()
        facts = AUDIT.directory_failure_facts({"path": path, "info": info}, self.root)
        self.assertEqual(facts["role"], "sqlite-runtime")
        self.assertTrue(facts["held_directory"])
        self.assertFalse(facts["held_symlink"])
        self.assertEqual(facts["canonical_relation"], "not-retained-by-guard")
        self.assertNotIn("SECRET", json.dumps(facts))

    def test_unallowlisted_frame_cannot_export_guard_locals(self):
        error = self.raised("unrelated.py", "scoped_snapshot",
                            {"held": {"SECRET": "unreadable"}},
                            "    state = held\n    roots = ()\n    raise ValueError('SECRET')")
        facts = AUDIT.sanitized_failure_diagnostics(error, self.repo, self.root)
        self.assertEqual(facts["guards"], [])
        self.assertNotIn("SECRET", json.dumps(facts))

    def test_outside_path_and_malformed_context_fail_closed(self):
        self.assertEqual(AUDIT.diagnostic_path(self.repo / "SECRET", self.root),
                         {"role": "outside-disposable-root"})
        self.assertEqual(AUDIT.snapshot_failure_facts({}, self.root),
                         {"category": "snapshot-context-unavailable"})
        self.assertEqual(AUDIT.directory_failure_facts({}, self.root),
                         {"category": "directory-context-unavailable"})

    def test_overlong_keys_are_counted_without_encoding_or_export(self):
        key = "home/" + "SECRET" * 1000
        facts = AUDIT.snapshot_failure_facts(
            {"state": {key: "unreadable"}, "roots": (self.root / "skill-24/home",)}, self.root)
        self.assertEqual(facts["entries"], [])
        self.assertEqual(facts["overlong_keys"], 1)
        self.assertEqual(facts["entries_omitted"], 1)
        self.assertNotIn("SECRET", json.dumps(facts))

    def test_duplicate_root_names_remain_candidates_not_authority(self):
        roots = (self.root / "skill-24/home", self.root / "skill-24/project/home")
        facts = AUDIT.snapshot_failure_facts(
            {"state": {"home/entry": "unreadable"}, "roots": roots}, self.root)
        candidates = facts["entries"][0]["root_candidates"]
        self.assertEqual([row["root_index"] for row in candidates], [0, 1])
        self.assertEqual([row["role"] for row in candidates], ["home", "other-disposable-entry"])


    def capture_emission(self, facts=None, failure=None, cohort="runner-temp"):
        original = ValueError("PRIVATE-original-refusal")
        original.add_note("PRIVATE-original-note")
        stream = io.StringIO()
        arguments = ({"side_effect": failure} if failure is not None else {"return_value": facts})
        with mock.patch.object(AUDIT, "sanitized_failure_diagnostics", **arguments):
            with redirect_stdout(stream):
                returned = AUDIT.emit_failure_diagnostics(original, self.repo, self.root, cohort)
        self.assertEqual(original.args, ("PRIVATE-original-refusal",))
        self.assertEqual(original.__notes__, ["PRIVATE-original-note"])
        self.assertIsNone(original.__traceback__)
        output = stream.getvalue()
        self.assertNotIn("PRIVATE", output)
        self.assertEqual(output.count("\n"), 1)
        prefix, encoded = output.rstrip("\n").split(": ", 1)
        self.assertLessEqual(len(encoded.encode("utf-8")), 64 << 10)
        self.assertEqual(json.loads(encoded), returned)
        return returned, prefix, output

    def test_emission_returns_and_prints_identical_sanitized_facts(self):
        facts = {"source_chain": [], "guards": [{"category": "directory-guard-refused",
                                                "held_directory": True}]}
        returned, prefix, _ = self.capture_emission(facts)
        self.assertIs(returned, facts)
        self.assertEqual(prefix, "Remotion failure diagnostics [runner-temp]")
        _, prefix, _ = self.capture_emission(facts, cohort="PRIVATE-cohort")
        self.assertEqual(prefix, "Remotion failure diagnostics [unqualified]")

    def test_emission_sanitizer_failure_has_fixed_fallback(self):
        returned, _, _ = self.capture_emission(failure=RuntimeError("PRIVATE-sanitizer"))
        self.assertEqual(returned, {"category": "diagnostic-unavailable"})

    def test_emission_unserializable_facts_have_fixed_fallback(self):
        returned, _, _ = self.capture_emission({"value": object()})
        self.assertEqual(returned, {"category": "diagnostic-unavailable"})

    def test_emission_encoded_bound_has_fixed_fallback(self):
        returned, _, _ = self.capture_emission({"value": "x" * (64 << 10)})
        self.assertEqual(returned, {"category": "diagnostic-unavailable"})
        returned, _, _ = self.capture_emission({"value": "\u2603" * 12000})
        self.assertEqual(returned, {"category": "diagnostic-unavailable"})

    def test_emission_stdout_failure_preserves_original_refusal(self):
        original = ValueError("PRIVATE-original-refusal")
        facts = {"guards": []}
        with mock.patch.object(AUDIT, "sanitized_failure_diagnostics", return_value=facts):
            with mock.patch.object(AUDIT, "print", side_effect=OSError("PRIVATE-stdout"), create=True):
                returned = AUDIT.emit_failure_diagnostics(original, self.repo, self.root, "runner-temp")
        self.assertIs(returned, facts)
        self.assertEqual(original.args, ("PRIVATE-original-refusal",))
        self.assertIsNone(original.__traceback__)




    def test_snapshot_key_area_handles_mixed_windows_keys_without_exporting_names(self):
        for relative, expected in (
                (".codex/skills/.system/private-file", "codex-system-skills"),
                (".local/state/codex-sqlite/private-file", "sqlite-runtime"),
                (".codex/private-file", "codex-profile"),
                (".cache/private-file", "other-home")):
            for key in ("home/" + relative, "home/" + relative.replace("/", "\\")):
                with self.subTest(key=key):
                    self.assertEqual(AUDIT.snapshot_key_area(key), expected)
                    facts = AUDIT.snapshot_failure_facts(
                        {"state": {key: "unreadable"}, "roots": (self.root / "skill-24/home",)},
                        self.root)
                    self.assertEqual(facts["entries"][0]["area"], expected)
                    self.assertEqual(facts["entries"][0]["key_sha256"], hashlib.sha256(key.encode()).hexdigest())
                    self.assertNotIn("private-file", json.dumps(facts))
        self.assertEqual(AUDIT.snapshot_key_area("project/private-file"), "other-protected-root")
        for key in (None, object(), "x" * 4097):
            self.assertEqual(AUDIT.snapshot_key_area(key), "unqualified")


class PublishedTemporaryRootContracts(unittest.TestCase):
    def exercise(self, *, failure_at=0, replacement=False, reparse=False,
                 case=None, version="agentplugins 0.1.102\n", lexical_alias=False):
        temporary = tempfile.TemporaryDirectory(prefix="published-remotion-unit-")
        self.addCleanup(temporary.cleanup)
        base = Path(temporary.name).resolve(strict=True)
        runner = base / "runner"
        default = base / ("default-temp-" + "long-" * 24)
        runner.mkdir()
        default.mkdir()
        cli = base / "inert-agentplugins.cmd"
        cli.write_bytes(b"never executed")
        fixture_path = MODULE_PATH.parent.parent / "tests/live-install/fixtures.json"
        checkout_bytes = fixture_path.read_bytes()
        fixture_bytes = checkout_bytes.replace(b"\r\n", b"\n")
        self.assertEqual(hashlib.sha256(fixture_bytes).hexdigest(), AUDIT.FIXTURE_SHA256)
        self.assertIn(checkout_bytes, (fixture_bytes, fixture_bytes.replace(b"\n", b"\r\n")))
        fixtures = json.loads(fixture_bytes)
        selected = fixtures["skills"][24]
        self.assertEqual(selected["name"], "remotion-best-practices")
        self.assertIn(AUDIT.REMOTION_SHA, selected["source"])
        created, calls, admissions, options_seen = [], [], [], []
        marked = set()
        original_handoff = AUDIT.physical_temp_root
        original_authority = AUDIT.temp_root_authority
        original_lstat = Path.lstat
        original_mkdtemp = tempfile.mkdtemp

        def create(**options):
            options_seen.append(dict(options))
            path = Path(original_mkdtemp(prefix=options["prefix"], dir=options.get("dir", default)))
            created.append(path)
            if lexical_alias:
                spelling = path.parent / ("spelling-" + str(len(created)))
                spelling.mkdir()
                return str(spelling / ".." / path.name)
            return str(path)

        def observed_lstat(path, *args, **kwargs):
            info = original_lstat(path, *args, **kwargs)
            if path not in marked:
                return info
            return SimpleNamespace(st_mode=info.st_mode, st_dev=info.st_dev,
                                   st_ino=info.st_ino, st_uid=info.st_uid,
                                   st_gid=info.st_gid, st_file_attributes=0x400,
                                   st_reparse_tag=0)

        def handoff(path, authority):
            position = len(admissions) + 1
            if position == failure_at:
                if replacement:
                    path.rename(path.with_name(path.name + "-original"))
                    path.mkdir()
                    (path / "foreign.txt").write_bytes(b"foreign replacement")
                elif reparse:
                    marked.add(path)
                else:
                    raise ValueError("PRIVATE-handoff-refused")
            physical = original_handoff(path, authority)
            admissions.append("runner-temp" if position == 1 else "default-windows-temp")
            return physical

        def validate(document):
            self.assertEqual(document, fixtures)

        def native(*args, **kwargs):
            self.assertEqual(kwargs, {})
            self.assertEqual(len(admissions), 2, "native case preceded both TEMP admissions")
            self.assertEqual(args[0], str(cli))
            self.assertEqual((args[1], args[2], args[4]), ("codex", "skill", 24))
            self.assertEqual(args[3], selected)
            self.assertEqual(args[6], fixtures["plugin_declared_names"])
            self.assertEqual(args[7], fixtures["signed_release_pins"])
            self.assertEqual(args[5], args[5].resolve(strict=True))
            calls.append({"root": args[5], "admitted": len(admissions)})
            return dict(case if case is not None else
                        {"status": "passed", "codex_runtime_proof": {"inert": True}})

        native_module = SimpleNamespace(validate_fixtures=validate, do_case=native)
        loader = SimpleNamespace(exec_module=lambda module: None)
        imports = SimpleNamespace(util=SimpleNamespace(
            spec_from_file_location=lambda *args: SimpleNamespace(loader=loader),
            module_from_spec=lambda spec: native_module))

        def git(*args, **kwargs):
            return "inert-checkout" if kwargs.get("text") else fixture_bytes

        processes = SimpleNamespace(check_output=git,
            run=lambda *args, **kwargs: SimpleNamespace(returncode=0, stdout=version))
        remove = mock.Mock()
        fake_shutil = SimpleNamespace(which=lambda name: str(cli), rmtree=remove)
        environment = SimpleNamespace(name="nt", environ={"RUNNER_TEMP": str(runner)})
        stream = io.StringIO()
        failed = None
        with mock.patch.dict(AUDIT.main.__globals__, {
                "os": environment, "subprocess": processes, "importlib": imports,
                "tempfile": SimpleNamespace(mkdtemp=create), "shutil": fake_shutil,
                "physical_temp_root": handoff, "temp_root_authority": original_authority}), \
                mock.patch.object(Path, "lstat", observed_lstat), redirect_stdout(stream):
            try:
                AUDIT.main()
            except SystemExit as error:
                failed = error
        remove.assert_not_called()
        should_fail = bool(failure_at or version != "agentplugins 0.1.102\n" or
                           case is not None and
                           (case.get("status") != "passed" or not case.get("codex_runtime_proof")))
        self.assertEqual(failed is not None, should_fail, "published wrapper exit contract differs")
        if failed is not None:
            self.assertIsInstance(failed.code, str)
            self.assertTrue(failed.code)
        report_path = runner / "remotion-windows-published-0.1.102.json"
        report = json.loads(report_path.read_text()) if report_path.exists() else None
        exported = json.dumps(report)
        self.assertNotIn("PRIVATE", exported)
        self.assertNotIn(str(base), exported)
        self.assertNotIn("PRIVATE", stream.getvalue())
        self.assertNotIn(str(base), stream.getvalue())
        if len(options_seen) == 2:
            self.assertEqual(options_seen[0], {"prefix": "uap-remotion-short-", "dir": str(runner)})
            self.assertEqual(options_seen[1], {"prefix": "uap-live-30x30-"})
        return report, created, calls, admissions

    def test_published_admits_both_physical_roots_before_native_cases(self):
        report, created, calls, admissions = self.exercise(lexical_alias=True)
        self.assertEqual(admissions, ["runner-temp", "default-windows-temp"])
        self.assertEqual([row["admitted"] for row in calls], [2, 2])
        self.assertEqual([row["root"] for row in calls], created)
        self.assertEqual(report["published_version"], "agentplugins 0.1.102")
        self.assertEqual(report["fixture_sha256"], AUDIT.FIXTURE_SHA256)
        self.assertEqual(report["remotion_commit"], AUDIT.REMOTION_SHA)
        roots = report["temp_roots"]
        for key in ("created_root_chars", "root_chars"):
            self.assertGreaterEqual(roots["default-windows-temp"][key], 65)
            self.assertGreater(roots["default-windows-temp"][key], roots["runner-temp"][key])
        self.assertEqual({row["cleanup"] for row in roots.values()}, {"retained-for-evidence"})
        self.assertTrue(all(path.is_dir() for path in created))

    def test_published_first_and_second_handoff_fail_before_native(self):
        for position in (1, 2):
            with self.subTest(position=position):
                report, created, calls, admissions = self.exercise(failure_at=position)
                self.assertEqual(calls, [])
                self.assertEqual(len(created), position)
                self.assertEqual(len(admissions), position - 1)
                self.assertEqual(report["results"], {})
                self.assertEqual(report["setup_error"]["error_type"], "ValueError")
                self.assertEqual({row["cleanup"] for row in report["temp_roots"].values()},
                                 {"retained-for-evidence"})
                self.assertTrue(all(path.is_dir() for path in created))

    def test_published_reparse_and_replacement_keep_unverified_roots(self):
        for option in ("reparse", "replacement"):
            with self.subTest(option=option):
                report, created, calls, admissions = self.exercise(failure_at=2, **{option: True})
                self.assertEqual(calls, [])
                self.assertEqual(admissions, ["runner-temp"])
                self.assertEqual(report["results"], {})
                self.assertEqual(report["setup_error"]["error_type"], "ValueError")
                self.assertEqual(report["temp_roots"]["default-windows-temp"]["cleanup"],
                                 "retained-unverified")
                self.assertTrue(all(path.is_dir() for path in created))
                if option == "replacement":
                    self.assertEqual((created[1] / "foreign.txt").read_bytes(), b"foreign replacement")
                    self.assertTrue(created[1].with_name(created[1].name + "-original").is_dir())

    def test_published_native_proof_and_passed_status_are_both_required(self):
        for result in ({"status": "passed"},
                       {"status": "incomplete-native-proof", "codex_runtime_proof": {"inert": True}}):
            with self.subTest(result=result):
                report, _, calls, _ = self.exercise(case=result)
                self.assertEqual(len(calls), 2)
                self.assertTrue(all(row["case"] == result for row in report["results"].values()))
                if result["status"] == "passed":
                    self.assertTrue(all(row["error_type"] == "RuntimeError"
                                        for row in report["results"].values()))

    def test_published_wrong_version_refuses_before_creating_profiles(self):
        report, created, calls, admissions = self.exercise(version="agentplugins 0.1.102-dev\n")
        self.assertIsNone(report)
        self.assertEqual((created, calls, admissions), ([], [], []))



class NativeReadFailureFactsTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.home = self.root / "home"
        self.home.mkdir()
        self.path = self.home / "PRIVATE-file"
        self.path.write_bytes(b"PRIVATE-body")
        self.audit = load_audit()

    def digest(self, failure=None):
        trace, descriptors, facts = [], [], {}
        old_lstat, old_fstat, old_read, old_close = Path.lstat, os.fstat, os.read, os.close
        old_open_member = self.audit.NATIVE_ARTIFACTS["open_member"]
        old_same_opened = self.audit.NATIVE_ARTIFACTS["same_opened"]

        def lstat(path, *args, **kwargs):
            trace.append("lstat")
            number = trace.count("lstat")
            if ((failure == "initial-lstat" and number == 1)
                    or (failure == "final-lstat" and number == 2)):
                raise OSError(errno.EIO, "PRIVATE-lstat-error")
            if failure == "replacement" and number == 2:
                # Move the held original aside before recreating its name.
                # This also permits a real swap on Windows without replacing
                # an existing open destination. The held-fd check has finished.
                self.path.rename(self.home / "held-original")
                self.path.write_bytes(b"PRIVATE-body")
            return old_lstat(path, *args, **kwargs)

        def opened(path):
            trace.append("open")
            if failure == "open":
                error = OSError(errno.EACCES, "PRIVATE-open-error")
                error.winerror = 32
                raise error
            descriptor = old_open_member(path)
            descriptors.append(descriptor)
            return descriptor

        def fstat(descriptor):
            trace.append("fstat")
            number = trace.count("fstat")
            if ((failure == "opened-fstat" and number == 1)
                    or (failure == "final-fstat" and number == 2)):
                raise OSError(errno.EIO, "PRIVATE-fstat-error")
            info = old_fstat(descriptor)
            if failure == "held-identity" and number == 2:
                fields = {name: getattr(info, name) for name in (
                    "st_dev", "st_ino", "st_mode", "st_nlink", "st_size", "st_mtime_ns", "st_ctime_ns",
                    "st_birthtime_ns", "st_file_attributes", "st_reparse_tag") if hasattr(info, name)}
                fields["st_mtime_ns"] += 1
                return SimpleNamespace(**fields)
            return info

        def read(descriptor, count):
            trace.append("read")
            if failure == "read":
                raise OSError(errno.EIO, "PRIVATE-read-error")
            if failure == "read-size":
                return b""
            return old_read(descriptor, count)

        def close(descriptor):
            trace.append("close")
            old_close(descriptor)
            if failure == "close":
                raise OSError(errno.EIO, "PRIVATE-close-error")

        def same_opened(before, held):
            return False if failure == "opened-identity" else old_same_opened(before, held)

        with mock.patch.object(Path, "lstat", lstat), \
                mock.patch.dict(self.audit.NATIVE_ARTIFACTS, {"open_member": opened, "same_opened": same_opened}), \
                mock.patch.object(os, "fstat", fstat), \
                mock.patch.object(os, "read", read), mock.patch.object(os, "close", close):
            value = self.audit.native_file_digest(self.path, failure_facts=facts)
        for descriptor in descriptors:
            with self.assertRaises(OSError):
                old_fstat(descriptor)
        self.assertNotIn("PRIVATE", json.dumps(facts))
        self.assertNotIn(str(self.root), json.dumps(facts))
        return value, facts, trace

    def test_success_preserves_exact_existing_syscall_sequence(self):
        value, facts, trace = self.digest()
        self.assertEqual(value, hashlib.sha256(b"PRIVATE-body").hexdigest())
        self.assertEqual(facts, {})
        self.assertEqual(trace, ["lstat", "open", "fstat", "read", "read", "fstat", "lstat", "close"])

    def test_existing_failures_report_errno_and_preserve_read_stage_across_close(self):
        for failure, operation, expected_trace in (
                ("open", "open", ["lstat", "open"]),
                ("read", "read", ["lstat", "open", "fstat", "read", "close"]),
                ("close", "close", ["lstat", "open", "fstat", "read", "read", "fstat", "lstat", "close"])):
            with self.subTest(failure=failure):
                value, facts, trace = self.digest(failure)
                self.assertEqual(value, "unreadable")
                self.assertEqual(facts["reason"], "os-error")
                self.assertEqual(facts["operation"], operation)
                self.assertEqual(facts["errno"], errno.EACCES if failure == "open" else errno.EIO)
                self.assertEqual(facts["winerror"], 32 if failure == "open" else None)
                self.assertEqual(trace, expected_trace)

    def test_same_bytes_path_replacement_retains_original_refusal(self):
        value, facts, trace = self.digest("replacement")
        self.assertEqual(value, "unreadable")
        self.assertEqual(facts["operation"], "final-lstat")
        self.assertEqual(facts["reason"], "name-identity-changed", facts)
        held_original = self.home / "held-original"
        self.assertEqual(self.path.read_bytes(), b"PRIVATE-body")
        self.assertEqual(held_original.read_bytes(), b"PRIVATE-body")
        current, held = self.path.stat(), held_original.stat()
        self.assertNotEqual((current.st_dev, current.st_ino), (held.st_dev, held.st_ino))
        self.assertEqual(trace, ["lstat", "open", "fstat", "read", "read", "fstat", "lstat", "close"])

    def test_malformed_error_attributes_and_collector_cannot_change_refusal(self):
        class PrivateError:
            @property
            def errno(self):
                raise RuntimeError("PRIVATE-attribute")

            @property
            def winerror(self):
                return "PRIVATE-value"

            def __str__(self):
                raise AssertionError("must not stringify")

        facts = {}
        self.audit.native_read_failure(facts, "read", "os-error", PrivateError())
        self.assertEqual(facts, {"operation": "read", "reason": "os-error", "errno": None, "winerror": None})
        for collector in (None, object(), {"entries": object()}, {"count": "PRIVATE"}):
            self.audit.snapshot_read_failure(collector, 0, "home/PRIVATE", facts)
        with mock.patch.object(self.audit, "native_read_failure", side_effect=RuntimeError("PRIVATE-diagnostic")), \
                mock.patch.dict(self.audit.NATIVE_ARTIFACTS, {
                    "open_member": mock.Mock(side_effect=OSError(errno.EACCES, "PRIVATE"))}):
            self.assertEqual(self.audit.native_file_digest(self.path, failure_facts={}), "unreadable")
        with mock.patch.object(self.audit, "snapshot_read_failure", side_effect=RuntimeError("PRIVATE-diagnostic")), \
                mock.patch.dict(self.audit.NATIVE_ARTIFACTS, {
                    "open_member": mock.Mock(side_effect=OSError(errno.EACCES, "PRIVATE"))}):
            with self.assertRaisesRegex(AssertionError, "protected scope snapshot unreadable"):
                self.audit.scoped_snapshot([self.home])

    def test_snapshot_caps_hashed_causes_without_changing_original_state(self):
        for index in range(20):
            (self.home / ("PRIVATE-" + str(index))).write_bytes(b"body")

        def refused(_path, *, failure_facts=None):
            self.audit.native_read_failure(failure_facts, "open", "os-error",
                                           OSError(errno.EACCES, "PRIVATE"))
            return "unreadable"

        facts = {}
        with mock.patch.object(self.audit, "native_file_digest", side_effect=refused):
            ordinary = self.audit.snapshot([self.home])
            observed = self.audit.snapshot([self.home], failure_facts=facts)
        self.assertEqual(observed, ordinary)
        self.assertEqual(facts["count"], 21)
        self.assertEqual(len(facts["entries"]), 16)
        self.assertEqual({entry["root_index"] for entry in facts["entries"]}, {0})
        self.assertEqual({entry["reason"] for entry in facts["entries"]}, {"os-error"})
        self.assertNotIn("PRIVATE", json.dumps(facts))
        self.assertNotIn(str(self.root), json.dumps(facts))

    def test_explicit_snapshotter_still_receives_roots_only_and_refuses(self):
        custom = mock.Mock(return_value={"home/PRIVATE": "unreadable"})
        with self.assertRaisesRegex(AssertionError, "protected scope snapshot unreadable"):
            self.audit.scoped_snapshot([self.home], snapshotter=custom)
        custom.assert_called_once_with((self.home,))

    def test_wrapper_exports_only_retained_key_bound_enum_and_integer_causes(self):
        key = "home/PRIVATE"
        digest = hashlib.sha256(key.encode()).hexdigest()
        entry = {"key_sha256": digest, "root_index": 0, "operation": "open",
                 "reason": "os-error", "errno": errno.EACCES, "winerror": 32}
        local = {"state": {key: "unreadable"}, "roots": (self.home,),
                 "snapshot_failures": {"entries": [
                     entry,
                     {"key_sha256": "0" * 64, "root_index": 0, "operation": "PRIVATE", "reason": "PRIVATE"},
                     {**entry, "root_index": 1},
                 ]}}
        facts = AUDIT.snapshot_failure_facts(local, self.root)
        self.assertEqual(facts["read_failures"], [entry])
        self.assertNotIn("PRIVATE", json.dumps(facts))
        local["snapshot_failures"]["entries"] = [
            {**entry, "operation": "PRIVATE", "reason": "PRIVATE", "errno": True, "winerror": "PRIVATE"}]
        facts = AUDIT.snapshot_failure_facts(local, self.root)
        self.assertEqual(facts["read_failures"], [
            {**entry, "operation": "unavailable", "reason": "unavailable", "errno": None, "winerror": None}])
        self.assertNotIn("PRIVATE", json.dumps(facts))

    def test_existing_stat_failures_keep_original_short_circuit_and_close(self):
        for failure, trace in (
                ("initial-lstat", ["lstat"]),
                ("opened-fstat", ["lstat", "open", "fstat", "close"]),
                ("final-fstat", ["lstat", "open", "fstat", "read", "read", "fstat", "close"]),
                ("final-lstat", ["lstat", "open", "fstat", "read", "read", "fstat", "lstat", "close"])):
            with self.subTest(failure=failure):
                value, facts, observed = self.digest(failure)
                self.assertEqual(value, "unreadable")
                self.assertEqual(facts, {"operation": failure, "reason": "os-error",
                                         "errno": errno.EIO, "winerror": None})
                self.assertEqual(observed, trace)

    def test_existing_identity_and_read_size_refusals_are_distinguished(self):
        for failure, operation, reason, trace in (
                ("opened-identity", "opened-fstat", "opened-identity-changed",
                 ["lstat", "open", "fstat", "close"]),
                ("read-size", "read", "read-size-changed",
                 ["lstat", "open", "fstat", "read", "close"]),
                ("held-identity", "final-fstat", "held-identity-changed",
                 ["lstat", "open", "fstat", "read", "read", "fstat", "close"])):
            with self.subTest(failure=failure):
                value, facts, observed = self.digest(failure)
                self.assertEqual(value, "unreadable")
                self.assertEqual(facts, {"operation": operation, "reason": reason,
                                         "errno": None, "winerror": None})
                self.assertEqual(observed, trace)

    def test_existing_walk_error_reports_only_hashed_key_and_fixed_errno(self):
        calls = []

        def inaccessible(root, **kwargs):
            calls.append(root)
            kwargs["onerror"](PermissionError(errno.EACCES, "PRIVATE-walk-error",
                                              str(self.home / "PRIVATE")))
            return iter(())

        facts = {}
        with mock.patch.object(self.audit.os, "walk", side_effect=inaccessible):
            state = self.audit.snapshot([self.home], failure_facts=facts)
        key = "home/PRIVATE/.walk-error"
        self.assertEqual(state, {key: "unreadable"})
        self.assertEqual(calls, [self.home])
        self.assertEqual(facts, {"count": 1, "entries": [{"root_index": 0,
            "key_sha256": hashlib.sha256(key.encode()).hexdigest(), "operation": "walk",
            "reason": "walk-error", "errno": errno.EACCES, "winerror": None}]})
        self.assertNotIn("PRIVATE", json.dumps(facts))
        self.assertNotIn(str(self.root), json.dumps(facts))


if __name__ == "__main__":
    unittest.main()
