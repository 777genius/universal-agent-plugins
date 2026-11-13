"""Inert trusted project scanner handoffs; no vendor, acquisition or UID spoofing."""
import copy
import contextlib
import os
import hashlib
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
        snap["directories"]["usr_share"]["identity"]["mode"] = stat.S_IFDIR | 0o777
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


class PublisherViewAdmissionTests(unittest.TestCase):
    @staticmethod
    def receipt():
        policy = HostPolicyMeasurementTests.receipt()
        return {"scope": "private-publisher-view-v1", "mount_namespace": 2,
                "original_mount_namespace": 1, "original_mount_id": 1,
                "original_share_identity": copy.deepcopy(policy["before"]["directories"]["usr_share"]["identity"]),
                "acquired_mounts": [{"kind": "lower", "mount_id": 2, "parent_mount_id": 1},
                                    {"kind": "overlay", "mount_id": 3, "parent_mount_id": 1},
                                    {"kind": "publisher", "mount_id": 4, "parent_mount_id": 3}],
                "released": True, "remaining_mounts": 0, "mount_identity_uncertain": False,
                "label_required": "vscode (unconfined)", "sudo_parent_pid": 2,
                "sudo_parent_executable_sha256": "a" * 64}

    def test_success_requires_released_private_chain_and_independent_host_join(self):
        collector.validate_publisher_view(self.receipt(), True, HostPolicyMeasurementTests.receipt())
        for kind in ("unreleased", "remaining", "uncertain", "namespace", "mount-chain", "duplicate-mount",
                     "boolean-id", "parent-pid", "parent-sha", "foreign-identity", "missing-mount", "unknown"):
            value = self.receipt()
            if kind == "unreleased": value["released"] = False
            elif kind == "remaining": value["remaining_mounts"] = 1
            elif kind == "uncertain": value["mount_identity_uncertain"] = True
            elif kind == "namespace": value["mount_namespace"] = value["original_mount_namespace"]
            elif kind == "mount-chain": value["acquired_mounts"][2]["parent_mount_id"] = 1
            elif kind == "duplicate-mount": value["acquired_mounts"][2]["mount_id"] = 2
            elif kind == "boolean-id": value["original_mount_id"] = True
            elif kind == "parent-pid": value["sudo_parent_pid"] = 0
            elif kind == "parent-sha": value["sudo_parent_executable_sha256"] = ""
            elif kind == "foreign-identity": value["original_share_identity"]["inode"] += 1
            elif kind == "missing-mount": value["acquired_mounts"].pop()
            else: value["unadmitted"] = "private"
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                collector.validate_publisher_view(value, True, HostPolicyMeasurementTests.receipt())
        for complete in (None, 1, "true"):
            with self.subTest(complete=complete), self.assertRaises(ValueError):
                collector.validate_publisher_view(self.receipt(), complete)

    def test_early_or_partial_refusal_never_qualifies_success(self):
        collector.validate_publisher_view(None, False)
        partial = self.receipt()
        partial.update(released=False, acquired_mounts=None, original_mount_id=0,
                       sudo_parent_pid=0, sudo_parent_executable_sha256="")
        partial["original_share_identity"] = {key: 0 for key in partial["original_share_identity"]}
        collector.validate_publisher_view(partial, False)
        for value in (None, partial):
            with self.subTest(value=value), self.assertRaises(ValueError):
                collector.validate_publisher_view(value, True)

    def test_controller_decoder_refuses_ambiguous_unbounded_or_nonfinite_input(self):
        for body in (b'{"publisher_view":null,"publisher_view":null}', b'{"elapsed":1e999}',
                     b'{"elapsed":NaN}', b'[]', b'x' * ((1 << 20) + 1)):
            with self.subTest(body=body[:80]), self.assertRaises(ValueError):
                collector.controller_record(body)


class TrustedProjectArtifactTests(unittest.TestCase):
    def capture(self, root, complete, include_policy=True, include_controller=True):
        native = Path(root) / "native"
        native.mkdir()
        summary = {"source_commit": "a" * 40, "collection_complete": complete,
                   "qualification": {name: complete for name in collector.QUALIFICATIONS}}
        collector.write_owned(native, native.lstat(), "vscode-project-research.json",
                              json.dumps(summary).encode())
        if include_policy:
            collector.write_owned(native, native.lstat(), "trusted-host-policy.json",
                                  json.dumps(HostPolicyMeasurementTests.receipt()).encode())
        if include_controller:
            collector.write_owned(native, native.lstat(), "vscode-workspace-controller.json",
                                  json.dumps({"publisher_view": PublisherViewAdmissionTests.receipt()}).encode())
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

    def test_missing_or_unjoined_publisher_view_withholds_success_archive(self):
        for kind in ("missing", "host-identity", "partial"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as root:
                native, identity, summary = self.capture(root, True, include_controller=False)
                if kind != "missing":
                    view = PublisherViewAdmissionTests.receipt()
                    if kind == "host-identity": view["original_share_identity"]["inode"] += 1
                    else: view.update(released=False, remaining_mounts=1)
                    collector.write_owned(native, identity, "vscode-workspace-controller.json",
                                          json.dumps({"publisher_view": view}).encode())
                with self.assertRaises(ValueError):
                    collector.publish_native_capture(native, identity, summary)
                self.assertIs(summary["collection_complete"], False)
                self.assertTrue(all(value is False for value in summary["qualification"].values()))
                self.assertFalse(native.exists())

    def test_early_controller_failure_can_publish_without_host_measurement(self):
        with tempfile.TemporaryDirectory() as root:
            native, identity, summary = self.capture(root, False, include_policy=False, include_controller=False)
            collector.publish_native_capture(native, identity, summary)
            with zipfile.ZipFile(native / "upload/research.zip") as archive:
                record = json.loads(archive.read("vscode-project-research.json"))
            self.assertIs(record["collection_complete"], False)
            self.assertTrue(all(value is False for value in record["qualification"].values()))


if __name__ == "__main__":
    unittest.main()


class ArtifactHandoffTests(unittest.TestCase):
    commit, tree = 'a' * 40, 'b' * 40

    def setup_capture(self, parent, stack):
        output = Path(parent) / 'output'
        output.mkdir(mode=0o700)
        for label in ('frontend', 'native'):
            (output / label).mkdir(mode=0o700)
        return output, collector.ArtifactHandoff(output, stack)

    def frontend(self, output, handoff, complete=True):
        summary = dict(source_commit=self.commit, source_tree=self.tree, collection_complete=complete)
        path = output / 'frontend' / 'vscode-project-frontend.json'
        collector.toolchain.write_new(path, json.dumps(summary).encode())
        collector.publish_frontend_capture(path.parent, path.parent.lstat(), summary, handoff)
        return summary

    def test_real_completed_zip_binding_and_private_staged_copy(self):
        with tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
            output, held = self.setup_capture(parent, stack)
            summary = self.frontend(output, held)
            receipt = json.loads(held.receipt(summary))
            paths = collector.stage_handoff(receipt, output, Path(parent), self.commit, self.tree)
            self.assertEqual(set(paths), {'frontend'})
            body = Path(paths['frontend']).read_bytes()
            self.assertEqual(hashlib.sha256(body).hexdigest(), receipt['archives']['frontend']['sha256'])
            self.assertNotIn(str(output), paths['frontend'])

    def test_replaced_outer_root_never_authorizes_or_touches_foreign_zip(self):
        with tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
            output, held = self.setup_capture(parent, stack)
            summary = self.frontend(output, held)
            receipt = json.loads(held.receipt(summary))
            output.rename(Path(parent) / 'original')
            output.mkdir(mode=0o700)
            for label in ('frontend', 'native'):
                (output / label).mkdir(mode=0o700)
            foreign = output / 'frontend' / 'foreign'
            foreign.write_bytes(b'preserved foreign data')
            with self.assertRaises(ValueError):
                held.receipt(summary)
            with self.assertRaises(ValueError):
                collector.stage_handoff(receipt, output, Path(parent), self.commit, self.tree)
            self.assertEqual(foreign.read_bytes(), b'preserved foreign data')

    def test_zip_changed_after_receipt_leaves_no_admitted_copy(self):
        with tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
            output, held = self.setup_capture(parent, stack)
            summary = self.frontend(output, held)
            receipt = json.loads(held.receipt(summary))
            archive = output / 'frontend/upload/research.zip'
            archive.write_bytes(archive.read_bytes() + b'changed')
            with self.assertRaises(ValueError):
                collector.stage_handoff(receipt, output, Path(parent), self.commit, self.tree)
            self.assertFalse(list(Path(parent).glob('vscode-admitted-*')))

    def test_late_exception_after_actual_zip_clears_binding_and_quarantines(self):
        for error in (OSError('inert late error'), KeyboardInterrupt()):
            with self.subTest(error=type(error).__name__), tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
                output, held = self.setup_capture(parent, stack)
                original = collector.legacy.publish_from_memory
                def late(*args, **kwargs):
                    original(*args, **kwargs)
                    raise error
                with mock.patch.object(collector.legacy, 'publish_from_memory', side_effect=late):
                    with self.assertRaises(type(error)):
                        self.frontend(output, held)
                self.assertIsNone(held.bindings['frontend'])
                self.assertFalse((output / 'frontend/upload/research.zip').exists())

    def test_unproved_or_numeric_cleanup_never_binds_even_if_quarantine_fails(self):
        for reaped, allowed in ((False, False), (False, True), (True, False), (1, True), (True, 1)):
            with self.subTest(flags=(reaped, allowed)), tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
                output, held = self.setup_capture(parent, stack)
                admission = dict(complete=False, child_reaped=reaped, archive_failure_allowed=allowed)
                with mock.patch.object(collector.legacy, 'publish_from_memory', return_value=admission):
                    with mock.patch.object(collector.legacy, 'quarantine_output', side_effect=OSError('inert quarantine failure')):
                        with self.assertRaises(OSError):
                            self.frontend(output, held, False)
                self.assertIsNone(held.bindings['frontend'])

    def test_real_negative_failure_zip_retains_only_admitted_failure(self):
        with tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
            output, held = self.setup_capture(parent, stack)
            (output / 'frontend/unexpected').write_bytes(b'inert')
            summary = self.frontend(output, held, False)
            receipt = json.loads(held.receipt(summary))
            self.assertIsNotNone(receipt['archives']['frontend'])
            with zipfile.ZipFile(output / 'frontend/upload/research.zip') as archive:
                self.assertEqual(archive.namelist(), ['collection-failure.json'])

    def test_closed_receipt_rejects_wrong_source_numeric_identity_and_extra(self):
        with tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
            output, held = self.setup_capture(parent, stack)
            summary = self.frontend(output, held)
            value = json.loads(held.receipt(summary))
            changes = [copy.deepcopy(value) for _ in range(3)]
            changes[0]['source_tree'] = self.commit
            changes[1]['output']['root']['inode'] = True
            changes[2]['extra'] = 'private'
            for changed in changes:
                with self.assertRaises(ValueError):
                    collector.validate_handoff(changed, self.commit, self.tree)

    def test_second_zip_refusal_discards_first_staged_copy(self):
        with tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
            output, held = self.setup_capture(parent, stack)
            self.frontend(output, held)
            summary = dict(source_commit=self.commit, source_tree=self.tree, collection_complete=False,
                           qualification={key: False for key in collector.QUALIFICATIONS})
            native = output / 'native'
            collector.toolchain.write_new(native / 'vscode-project-research.json', json.dumps(summary).encode())
            collector.toolchain.write_new(native / 'trusted-host-policy.json', json.dumps(HostPolicyMeasurementTests.receipt()).encode())
            collector.publish_native_capture(native, native.lstat(), summary, held)
            receipt = json.loads(held.receipt(summary))
            self.assertTrue(all(receipt['archives'].values()))
            archive = native / 'upload/research.zip'
            archive.write_bytes(archive.read_bytes() + b'changed second ZIP')
            with self.assertRaises(ValueError):
                collector.stage_handoff(receipt, output, Path(parent), self.commit, self.tree)
            self.assertFalse(list(Path(parent).glob('vscode-admitted-*')))

    def test_reparented_original_child_does_not_allow_foreign_root_quarantine(self):
        with tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
            output, held = self.setup_capture(parent, stack)
            original = Path(parent) / 'original'
            identity = (output / 'frontend').lstat()
            output.rename(original)
            output.mkdir(mode=0o700)
            (original / 'frontend').rename(output / 'frontend')
            (output / 'native').mkdir(mode=0o700)
            summary = dict(source_commit=self.commit, source_tree=self.tree, collection_complete=False)
            with mock.patch.object(collector.legacy, 'quarantine_output') as quarantine:
                with self.assertRaises(ValueError):
                    collector.publish_frontend_capture(output / 'frontend', identity, summary, held)
                quarantine.assert_not_called()
            self.assertTrue((output / 'frontend').is_dir())
            self.assertIsNone(held.bindings['frontend'])

    def test_same_bytes_zip_or_upload_replacement_during_copy_refuses_foreign(self):
        for kind in ('zip', 'upload'):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
                output, held = self.setup_capture(parent, stack)
                summary = self.frontend(output, held)
                receipt = json.loads(held.receipt(summary))
                original = collector.ArtifactHandoff.archive
                def replace_after_capture(authority, label, deadline):
                    result = original(authority, label, deadline)
                    upload = output / label / 'upload'
                    archive = upload / 'research.zip'
                    raw = archive.read_bytes()
                    if kind == 'zip':
                        archive.rename(upload / 'original.zip')
                    else:
                        upload.rename(upload.with_name('original-upload'))
                        upload.mkdir(mode=0o700)
                    archive.write_bytes(raw)
                    (upload / 'foreign-sentinel').write_bytes(b'preserved')
                    return result
                with mock.patch.object(collector.ArtifactHandoff, 'archive', replace_after_capture):
                    with self.assertRaises(ValueError):
                        collector.stage_handoff(receipt, output, Path(parent), self.commit, self.tree)
                self.assertEqual((output / 'frontend/upload/foreign-sentinel').read_bytes(), b'preserved')
                self.assertFalse(list(Path(parent).glob('vscode-admitted-*')))
