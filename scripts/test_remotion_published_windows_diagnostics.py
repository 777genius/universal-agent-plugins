"""Bounded refusal diagnostics preserve private paths and original failures."""
from contextlib import redirect_stdout
import hashlib
import io
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

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



if __name__ == "__main__":
    unittest.main()
