"""Inert trusted project scanner handoffs; no vendor, acquisition or UID spoofing."""
import copy
import json
import stat
import importlib.util
from pathlib import Path
import tempfile
import unittest
import zipfile
from unittest import mock

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("vscode_project_test", ROOT / "research-vscode-project.py")
collector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(collector)


class TrustedProjectScannerTests(unittest.TestCase):
    def test_runner_requires_matching_real_image_and_release(self):
        ubuntu24 = 'ID=ubuntu\nVERSION_ID="24.04"\n'
        collector.validate_host_image("ubuntu24", ubuntu24)
        for image, release in (("ubuntu22", ubuntu24), ("ubuntu24", 'ID=ubuntu\nVERSION_ID="22.04"\n'),
                               ("ubuntu24", 'ID=debian\nVERSION_ID="24.04"\n'), (None, ubuntu24)):
            with self.subTest(image=image, release=release):
                with self.assertRaises(ValueError):
                    collector.validate_host_image(image, release)

    def test_actual_acquisition_uses_current_gnu_source_pin_and_unique_member(self):
        member = "lintai-v0.1.3-x86_64-unknown-linux-gnu/lintai"
        receipt = {"members": [{"kind": "file", "name": member}]}
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            binary = root / "scanner" / member
            contents, budget = object(), object()
            with mock.patch.object(collector.shared, "acquire", return_value=(receipt, contents)) as acquire:
                with mock.patch.object(collector.shared, "materialize", return_value=(binary, {binary: "a" * 64})) as materialize:
                    self.assertEqual(collector.acquire_scanner(root, budget), (binary, "a" * 64, receipt))
            actual_spec, actual_budget = acquire.call_args.args
            self.assertIs(actual_budget, budget)
            self.assertEqual(actual_spec["url"], "https://github.com/777genius/lintai/releases/download/v0.1.3/lintai-v0.1.3-x86_64-unknown-linux-gnu.tar.gz")
            self.assertEqual(actual_spec["integrity"], "sha256:2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da")
            self.assertEqual(actual_spec["entrypoint"], member)
            materialize.assert_called_once_with(root / "scanner", actual_spec, receipt, contents, selected_only=True)

    def test_wrong_scanner_pin_refuses_before_acquisition(self):
        selected = collector.publisher.native.PINS["linux-amd64"]["lintai"]
        changed = (*selected[:4], "sha256:" + "a" * 64, selected[5])
        with mock.patch.dict(collector.publisher.native.PINS["linux-amd64"], {"lintai": changed}):
            with mock.patch.object(collector.shared, "acquire") as acquire:
                with self.assertRaisesRegex(ValueError, "reviewed scanner archive differs"):
                    collector.acquire_scanner(Path("/inert"), object())
                acquire.assert_not_called()

    def test_ambiguous_or_missing_scanner_member_refuses_before_materialization(self):
        for members in ([], [{"kind": "file", "name": "one/lintai"}, {"kind": "file", "name": "two/lintai"}]):
            with self.subTest(members=members):
                with mock.patch.object(collector.shared, "acquire", return_value=({"members": members}, object())):
                    with mock.patch.object(collector.shared, "materialize") as materialize:
                        with self.assertRaisesRegex(ValueError, "absent or ambiguous"):
                            collector.acquire_scanner(Path("/inert"), object())
                        materialize.assert_not_called()


class HostPolicyMeasurementTests(unittest.TestCase):
    @staticmethod
    def receipt():
        identity = {"device": 1, "inode": 2, "mode": stat.S_IFREG | 0o644, "uid": 0, "gid": 0,
                    "links": 1, "size": 1, "mtime_ns": 1, "ctime_ns": 1}
        absent = {"state": "absent", "sha256": "", "identity": None}
        snap = {"files": {key: dict(absent) for key in ("code", "local_code", "abi", "global", "alias")},
                "directories": {key: {"identity": {**identity, "mode": stat.S_IFDIR | 0o755}, "mount_id": 1}
                                for key in ("root", "usr", "usr_share")},
                "entries": {"disable_code": "absent", "force_complain_code": "absent"},
                "sysctls": {"apparmor_userns": 1, "apparmor_unconfined": 1, "userns_clone": 1, "max_user_namespaces": 1024},
                "grammar": "known-ubuntu-vscode-userns", "known_profile_match": True,
                "loaded_vscode_mode": "unconfined", "apparmor_enabled": True, "code_absent": True}
        snap["files"]["code"] = {"state": "present", "sha256": collector.HOST_POLICY_REFERENCE, "identity": identity}
        return {"schema_version": 1, "status": "observed", "reason": "none",
                "reference_profile_sha256": collector.HOST_POLICY_REFERENCE, "before": snap, "after": copy.deepcopy(snap),
                "native_qualified": False, "loaded_equals_disk_qualified": False}

    def test_measurement_observation_never_qualifies_native_or_loaded_bytes(self):
        value = self.receipt()
        admitted = collector.validate_host_policy(json.dumps(value).encode())
        self.assertIs(admitted["native_qualified"], False)
        self.assertIs(admitted["loaded_equals_disk_qualified"], False)
        self.assertIn("trusted-host-policy.json", collector.native_names())
        # A measured optional local override is represented without exposing it.
        for snap in (value["before"], value["after"]):
            snap["files"]["local_code"] = {**snap["files"]["code"], "sha256": "b" * 64}
            snap["files"]["code"]["sha256"] = "c" * 64
            snap["known_profile_match"] = False
        collector.validate_host_policy(json.dumps(value).encode())

    def test_closed_measurement_refusal_is_valid_failure_evidence(self):
        value = self.receipt()
        value.update(status="refused", reason="measurement-changed")
        value["after"]["code_absent"] = False
        self.assertEqual(collector.validate_host_policy(json.dumps(value).encode())["status"], "refused")

    def test_unknown_private_fields_and_invalid_facts_are_refused(self):
        for kind in ("extra", "file-extra", "identity-extra", "qualified", "loaded-equality", "boolean", "range",
                     "mode", "digest", "hash-claim", "drift", "incomplete", "sysctl-float", "entry-text", "mode-text"):
            value = self.receipt()
            snap = value["after"]
            if kind == "extra": value["raw_profile"] = "private"
            elif kind == "file-extra": snap["files"]["code"]["raw"] = "private"
            elif kind == "identity-extra": snap["files"]["code"]["identity"]["path"] = "private"
            elif kind == "qualified": value["native_qualified"] = True
            elif kind == "loaded-equality": value["loaded_equals_disk_qualified"] = True
            elif kind == "boolean": snap["known_profile_match"] = 1
            elif kind == "range": snap["sysctls"]["userns_clone"] = 2
            elif kind == "mode": snap["files"]["code"]["identity"]["mode"] = stat.S_IFREG | 0o666
            elif kind == "digest": snap["files"]["code"]["sha256"] = "private"
            elif kind == "hash-claim": snap["files"]["code"]["sha256"] = "b" * 64
            elif kind == "drift": snap["directories"]["usr_share"]["mount_id"] = 2
            elif kind == "incomplete": snap["sysctls"]["userns_clone"] = None
            elif kind == "sysctl-float": snap["sysctls"]["max_user_namespaces"] = 1.0
            elif kind == "entry-text": snap["entries"]["disable_code"] = "private-target"
            elif kind == "mode-text": snap["loaded_vscode_mode"] = "private-other-profile"
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                collector.validate_host_policy(json.dumps(value).encode())
        for body in (b"x" * ((16 << 10) + 1), b'{"schema_version":1,"schema_version":1}', b"{", b'{"x":NaN}'):
            with self.assertRaises(ValueError):
                collector.validate_host_policy(body)


class TrustedProjectArtifactTests(unittest.TestCase):
    def capture(self, root, complete, include_policy=True):
        native = Path(root) / "native"
        native.mkdir()
        summary = {"source_commit": "a" * 40, "collection_complete": complete,
                   "qualification": {name: complete for name in collector.QUALIFICATIONS}}
        collector.write_owned(native, native.lstat(), "vscode-project-research.json",
                              json.dumps(summary).encode())
        if include_policy:
            collector.write_owned(native, native.lstat(), "trusted-host-policy.json",
                                  json.dumps(HostPolicyMeasurementTests.receipt()).encode())
        return native, native.lstat(), summary

    def test_refused_success_capture_clears_flags_and_withholds_stale_summary(self):
        with tempfile.TemporaryDirectory() as root:
            native, identity, summary = self.capture(root, True)
            (native / "unexpected-native-cache").mkdir()
            collector.publish_native_capture(native, identity, summary)
            self.assertIs(summary["collection_complete"], False)
            self.assertTrue(all(value is False for value in summary["qualification"].values()))
            self.assertFalse(native.exists())
            self.assertFalse((native / "vscode-project-research.json").exists())
            self.assertFalse((native / "upload/research.zip").exists())

    def test_negative_capture_retains_only_admitted_failure_archive(self):
        with tempfile.TemporaryDirectory() as root:
            native, identity, summary = self.capture(root, False)
            (native / "unexpected-native-cache").mkdir()
            collector.publish_native_capture(native, identity, summary)
            self.assertIs(summary["collection_complete"], False)
            self.assertTrue(all(value is False for value in summary["qualification"].values()))
            self.assertIs(json.loads((native / "vscode-project-research.json").read_bytes())["collection_complete"], False)
            with zipfile.ZipFile(native / "upload/research.zip") as archive:
                self.assertEqual(archive.namelist(), ["collection-failure.json"])
                failure = json.loads(archive.read("collection-failure.json"))
                self.assertIs(failure["collection_complete"], False)
                self.assertIs(failure["native_project_qualified"], False)

    def test_admitted_capture_preserves_matching_success_record(self):
        with tempfile.TemporaryDirectory() as root:
            native, identity, summary = self.capture(root, True)
            collector.publish_native_capture(native, identity, summary)
            self.assertIs(summary["collection_complete"], True)
            with zipfile.ZipFile(native / "upload/research.zip") as archive:
                record = json.loads(archive.read("vscode-project-research.json"))
            self.assertEqual(record, summary)

    def test_publisher_error_after_zip_withholds_partial_success(self):
        with tempfile.TemporaryDirectory() as root:
            native, identity, summary = self.capture(root, True)
            def late_error(output, _summary, allowed_names=None):
                (output / "upload").mkdir()
                (output / "upload/research.zip").write_bytes(b"unadmitted success")
                raise RuntimeError("inert publication failure")
            with mock.patch.object(collector.legacy.publisher, "publish_artifacts", late_error):
                collector.publish_native_capture(native, identity, summary)
            self.assertIs(summary["collection_complete"], False)
            self.assertTrue(all(value is False for value in summary["qualification"].values()))
            self.assertFalse(native.exists())

    def test_parent_publication_exception_withholds_written_success(self):
        for failure in (OSError("inert fork failure"), KeyboardInterrupt()):
            with self.subTest(failure=type(failure).__name__), tempfile.TemporaryDirectory() as root:
                native, identity, summary = self.capture(root, True)
                with mock.patch.object(collector.legacy, "publish_from_memory", side_effect=failure):
                    with self.assertRaises(type(failure)):
                        collector.publish_native_capture(native, identity, summary)
                self.assertIs(summary["collection_complete"], False)
                self.assertTrue(all(value is False for value in summary["qualification"].values()))
                self.assertFalse(native.exists())

    def test_success_cannot_publish_missing_refused_or_malformed_host_policy(self):
        for kind in ("missing", "refused", "malformed"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as root:
                native, identity, summary = self.capture(root, True, include_policy=False)
                if kind != "missing":
                    value = HostPolicyMeasurementTests.receipt()
                    if kind == "refused":
                        value.update(status="refused", reason="measurement-changed")
                    else:
                        value["unexpected"] = "unadmitted raw text"
                    collector.write_owned(native, identity, "trusted-host-policy.json", json.dumps(value).encode())
                with self.assertRaises(ValueError):
                    collector.publish_native_capture(native, identity, summary)
                self.assertIs(summary["collection_complete"], False)
                self.assertTrue(all(value is False for value in summary["qualification"].values()))
                self.assertFalse(native.exists())

    def test_early_controller_failure_can_publish_without_host_measurement(self):
        with tempfile.TemporaryDirectory() as root:
            native, identity, summary = self.capture(root, False, include_policy=False)
            collector.publish_native_capture(native, identity, summary)
            with zipfile.ZipFile(native / "upload/research.zip") as archive:
                record = json.loads(archive.read("vscode-project-research.json"))
            self.assertIs(record["collection_complete"], False)
            self.assertTrue(all(value is False for value in record["qualification"].values()))


if __name__ == "__main__":
    unittest.main()
