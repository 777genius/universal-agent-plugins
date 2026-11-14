"""Inert trusted project scanner handoffs; no vendor, acquisition or UID spoofing."""
import copy
import contextlib
import os
import hashlib
import json
import re
import stat
import importlib.util
import io
from pathlib import Path
import tempfile
import time
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

    def native_capture(self, output, held):
        summary = dict(source_commit=self.commit, source_tree=self.tree, collection_complete=False,
                       qualification={key: False for key in collector.QUALIFICATIONS})
        native = output / 'native'
        collector.toolchain.write_new(native / 'vscode-project-research.json', json.dumps(summary).encode())
        collector.toolchain.write_new(native / 'trusted-host-policy.json', json.dumps(HostPolicyMeasurementTests.receipt()).encode())
        collector.publish_native_capture(native, native.lstat(), summary, held)
        return summary

    def test_readout_refusal_preserves_both_admitted_originals_and_staged_copies(self):
        with tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
            output, held = self.setup_capture(parent, stack)
            self.frontend(output, held)
            summary = self.native_capture(output, held)
            receipt = json.loads(held.receipt(summary))
            originals = {label: (output / label / 'upload/research.zip').read_bytes() for label in ('frontend', 'native')}
            readout = []
            with mock.patch.object(collector, 'closed_native_readout', side_effect=ValueError('PRIVATE_ERROR')), mock.patch('builtins.print') as printed:
                paths = collector.stage_handoff(receipt, output, Path(parent), self.commit, self.tree, readout=readout)
                printed.assert_not_called()
            self.assertEqual(readout, [{'status': 'refused', 'failures': []}])
            self.assertNotIn('PRIVATE', json.dumps(readout))
            self.assertEqual(set(paths), {'frontend', 'native'})
            for label in paths:
                self.assertEqual(Path(paths[label]).read_bytes(), originals[label])
                self.assertEqual((output / label / 'upload/research.zip').read_bytes(), originals[label])
                self.assertEqual(hashlib.sha256(originals[label]).hexdigest(), receipt['archives'][label]['sha256'])

    def test_late_readout_uses_original_deadline_without_revoking_admitted_copies(self):
        with tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
            output, held = self.setup_capture(parent, stack)
            self.frontend(output, held)
            summary = self.native_capture(output, held)
            receipt = json.loads(held.receipt(summary))
            originals = {label: (output / label / 'upload/research.zip').read_bytes() for label in ('frontend', 'native')}
            readout, clock = [], [time.monotonic()]
            original_readout = collector.closed_native_readout
            def late(body, deadline):
                self.assertEqual(deadline, clock[0] + 5)
                clock[0] = deadline + 1
                return original_readout(body, deadline)
            with (mock.patch.object(collector.time, 'monotonic', side_effect=lambda: clock[0]),
                  mock.patch.object(collector, 'closed_native_readout', side_effect=late)):
                paths = collector.stage_handoff(receipt, output, Path(parent), self.commit, self.tree, readout=readout)
            self.assertEqual(readout, [{'status': 'refused', 'failures': []}])
            self.assertEqual(set(paths), {'frontend', 'native'})
            for label in paths:
                self.assertEqual(Path(paths[label]).read_bytes(), originals[label])
                self.assertEqual((output / label / 'upload/research.zip').read_bytes(), originals[label])

    def test_staging_refusal_never_commits_partial_readout_or_prints(self):
        for kind in ('root', 'zip', 'hash', 'staging'):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
                output, held = self.setup_capture(parent, stack)
                self.frontend(output, held)
                summary = self.native_capture(output, held)
                receipt = json.loads(held.receipt(summary))
                if kind == 'root':
                    output.rename(Path(parent) / 'original')
                    output.mkdir(mode=0o700)
                    (output / 'frontend').mkdir(mode=0o700); (output / 'native').mkdir(mode=0o700)
                elif kind == 'zip':
                    archive = output / 'native/upload/research.zip'
                    archive.write_bytes(archive.read_bytes() + b'changed')
                elif kind == 'hash':
                    receipt['archives']['native']['sha256'] = '0' * 64
                original_open = collector.os.open
                def open_staging(path, *args, **kwargs):
                    if kind == 'staging' and path == 'native.zip':
                        raise OSError('PRIVATE_STAGING_ERROR')
                    return original_open(path, *args, **kwargs)
                readout = []
                with mock.patch.object(collector.os, 'open', side_effect=open_staging), mock.patch('builtins.print') as printed:
                    with self.assertRaises((ValueError, OSError)):
                        collector.stage_handoff(receipt, output, Path(parent), self.commit, self.tree, readout=readout)
                        print('must only occur after successful return')
                    printed.assert_not_called()
                self.assertEqual(readout, [])
                self.assertFalse(list(Path(parent).glob('vscode-admitted-*')))

    def test_successful_staging_without_failure_rows_and_missing_native_are_explicit(self):
        for with_native in (False, True):
            with self.subTest(with_native=with_native), tempfile.TemporaryDirectory() as parent, contextlib.ExitStack() as stack:
                output, held = self.setup_capture(parent, stack)
                summary = self.frontend(output, held)
                if with_native: summary = self.native_capture(output, held)
                receipt = json.loads(held.receipt(summary)); readout = []
                paths = collector.stage_handoff(receipt, output, Path(parent), self.commit, self.tree, readout=readout)
                self.assertEqual(readout, [{'status': 'observed' if with_native else 'unavailable', 'failures': []}])
                self.assertEqual(set(paths), {'frontend', 'native'} if with_native else {'frontend'})

    def test_artifact_root_refusal_still_attempts_independent_preparation_cleanup(self):
        import shutil
        for cleanup_failure in (False, True):
            with self.subTest(cleanup_failure=cleanup_failure), tempfile.TemporaryDirectory() as temporary, contextlib.ExitStack() as stack:
                output, handoff = self.setup_capture(temporary, stack)
                preparation = mock.Mock()
                def fail_source():
                    output.rename(output.with_name('original-output'))
                    output.mkdir(mode=0o700)
                    (output / 'foreign').write_bytes(b'retained')
                    raise ValueError('preparation failed')
                preparation.source.side_effect = fail_source
                cleaned = []
                def cleanup(root, original, native, *, memory_only):
                    self.assertTrue(memory_only)
                    self.assertEqual((root.lstat().st_dev, root.lstat().st_ino), (original.st_dev, original.st_ino))
                    cleaned.append(root)
                    shutil.rmtree(root)
                    if cleanup_failure:
                        raise OSError('inert cleanup diagnostic failure')
                    return {'complete': True, '_streams': {'stdout': b'owned cleanup', 'stderr': b''}}
                with (mock.patch.dict(os.environ, EXPECTED_COMMIT=self.commit, EXPECTED_TREE=self.tree),
                      mock.patch.object(collector, 'Preparation', return_value=preparation),
                      mock.patch.object(collector.legacy, 'cleanup_frontend', side_effect=cleanup) as cleanup_call,
                      mock.patch.object(collector, 'write_owned') as write,
                      mock.patch.object(collector, 'publish_frontend_capture') as frontend,
                      mock.patch.object(collector, 'publish_native_capture') as native,
                      mock.patch.object(handoff, 'receipt') as receipt):
                    with self.assertRaisesRegex(ValueError, 'original artifact directory changed'):
                        collector.run_capture(output, collector.time.monotonic_ns(), handoff)
                cleanup_call.assert_called_once()
                self.assertEqual(len(cleaned), 1)
                self.assertFalse(cleaned[0].exists())
                self.assertEqual((output / 'foreign').read_bytes(), b'retained')
                write.assert_not_called(); frontend.assert_not_called(); native.assert_not_called(); receipt.assert_not_called()

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


class ClosedNativeReadoutTests(unittest.TestCase):
    def failure(self):
        return {'type': 'driver-failure', 'stage': 'installed-a', 'nonce': 'PRIVATE_NONCE',
                'phase': 'mcp-state', 'code': 'closed-stage-contract-refused',
                'visibleNativeUI': [{'private': 'PRIVATE_UI'}],
                'diagnostics': {'reason': 'mcp-start-deadline', 'private': 'PRIVATE_ENDPOINT',
                    'mcp': {'startSelected': True, 'trustAccepted': False, 'lastState': 'Stopped',
                            'stopped': 17, 'starting': 0, 'running': 0, 'other': 0,
                            'launchNotification': True, 'outputVisible': False, 'launchError': 'other', 'postStartInput': 'unobserved',
                            'awaitBoundary': 'state-row', 'elapsedMs': 15000}}}

    def archive(self, members):
        output = io.BytesIO()
        with zipfile.ZipFile(output, 'w', compression=zipfile.ZIP_STORED) as archive:
            for name, body in members:
                archive.writestr(name, body if isinstance(body, bytes) else json.dumps(body).encode())
        return output.getvalue()

    def test_readout_vocabularies_match_actual_go_failure_admission(self):
        source = (ROOT.parent / 'install/integrationctl/agentplugins/research/vscodeworkspace/trusted_failure_linux.go').read_text()
        reasons = re.search(r'const failureReasons = "([^"]+)"', source)
        phases = re.search(r'failureChoice\(outer\["phase"\], "([^"]+)"\)', source)
        self.assertIsNotNone(reasons); self.assertIsNotNone(phases)
        self.assertEqual(collector.READOUT_REASONS, frozenset(reasons.group(1).split()))
        self.assertEqual(collector.READOUT_PHASES, frozenset(phases.group(1).split()))
        boundaries = re.search(r'const failureAwaitBoundaries = "([^"]+)"', source)
        self.assertIsNotNone(boundaries)
        self.assertEqual(collector.READOUT_AWAIT_BOUNDARIES, frozenset(boundaries.group(1).split()))
        driver = (ROOT / 'vscode-project-runtime-observer/driver.ts').read_text()
        boundaries = re.search(r"const MCP_AWAIT_BOUNDARIES = '([^']+)'", driver)
        self.assertIsNotNone(boundaries)
        self.assertEqual(collector.READOUT_AWAIT_BOUNDARIES, frozenset(boundaries.group(1).split()))

    def test_await_boundary_and_elapsed_require_closed_types(self):
        for boundary in collector.READOUT_AWAIT_BOUNDARIES:
            for elapsed in (0, 15000, 15001):
                value = self.failure()
                value['diagnostics']['mcp'].update(awaitBoundary=boundary, elapsedMs=elapsed)
                rows = collector.closed_native_readout(self.archive([('installed-a.observation.json', value)]), time.monotonic() + 5)
                self.assertEqual(rows[0]['mcp']['awaitBoundary'], boundary)
                self.assertEqual(rows[0]['mcp']['elapsedMs'], elapsed)
        for key, invalid in {'awaitBoundary': (None, True, 1, 'PRIVATE_BOUNDARY', 'input-count PRIVATE'),
                             'elapsedMs': (None, True, '1', -1, 1.5, 1.0, 15002)}.items():
            for changed in invalid:
                with self.subTest(key=key, value=changed):
                    value = self.failure(); value['diagnostics']['mcp'][key] = changed
                    with self.assertRaises(ValueError):
                        collector.closed_native_readout(self.archive([('installed-a.observation.json', value)]), time.monotonic() + 5)
            value = self.failure(); del value['diagnostics']['mcp'][key]
            with self.assertRaises(ValueError):
                collector.closed_native_readout(self.archive([('installed-a.observation.json', value)]), time.monotonic() + 5)

    def test_actual_projection_exposes_only_fixed_state_facts(self):
        failure = self.failure()
        body = self.archive([('empty-a.observation.json', {'type': 'driver-result', 'private': 'PRIVATE_RESULT'}),
                             ('installed-a.observation.json', failure)])
        rows = collector.closed_native_readout(body, time.monotonic() + 5)
        self.assertEqual(rows, [{'stage': 'installed-a', 'phase': 'mcp-state',
                                'reason': 'mcp-start-deadline', 'mcp': failure['diagnostics']['mcp']}])
        self.assertNotIn('PRIVATE', json.dumps(rows))

    def test_unknown_context_fields_and_numeric_boolean_facts_refuse(self):
        for kind in ('stage', 'phase', 'reason', 'unknown-type', 'extra-outer', 'extra-mcp',
                     'state', 'boolean', 'numeric-boolean', 'count', 'launch-boolean', 'output-boolean', 'launch-enum', 'input-enum', 'input-type', 'missing-input', 'missing-count'):
            with self.subTest(kind=kind):
                value = self.failure(); mcp = value['diagnostics']['mcp']
                if kind in ('stage', 'phase'): value[kind] = 'PRIVATE_TEXT'
                elif kind == 'reason': value['diagnostics']['reason'] = 'PRIVATE_ERROR'
                elif kind == 'unknown-type': value['type'] = 'unknown'
                elif kind == 'extra-outer': value['private'] = 'PRIVATE'
                elif kind == 'extra-mcp': mcp['private'] = 'PRIVATE'
                elif kind == 'state': mcp['lastState'] = 'PRIVATE_STATE'
                elif kind == 'boolean': mcp['trustAccepted'] = 'true'
                elif kind == 'numeric-boolean': mcp['stopped'] = True
                elif kind == 'count': mcp['stopped'] = 256
                elif kind == 'launch-boolean': mcp['launchNotification'] = 1
                elif kind == 'output-boolean': mcp['outputVisible'] = 'PRIVATE'
                elif kind == 'launch-enum': mcp['launchError'] = 'PRIVATE'
                elif kind == 'input-enum': mcp['postStartInput'] = 'PRIVATE_INPUT'
                elif kind == 'input-type': mcp['postStartInput'] = True
                elif kind == 'missing-input': del mcp['postStartInput']
                else: del mcp['stopped']
                with self.assertRaises(ValueError):
                    collector.closed_native_readout(self.archive([('installed-a.observation.json', value)]), time.monotonic() + 5)

    def test_duplicate_nonfinite_oversize_foreign_members_and_crc_refuse(self):
        for name, body in [('installed-a.observation.json', b'{"type":1,"type":2}'),
                           ('installed-a.observation.json', b'{"private":NaN}'),
                           ('installed-a.observation.json', b'x' * ((1 << 20) + 1)),
                           ('../foreign.json', b'{}')]:
            with self.assertRaises(ValueError):
                collector.closed_native_readout(self.archive([(name, body)]), time.monotonic() + 5)
        with self.assertWarns(UserWarning):
            duplicate = self.archive([('installed-a.observation.json', self.failure())] * 2)
        with self.assertRaises(ValueError): collector.closed_native_readout(duplicate, time.monotonic() + 5)
        encoded = json.dumps(self.failure()).encode()
        damaged = bytearray(self.archive([('installed-a.observation.json', encoded)]))
        damaged[damaged.index(encoded)] ^= 1
        with self.assertRaises(zipfile.BadZipFile):
            collector.closed_native_readout(bytes(damaged), time.monotonic() + 5)
        with self.assertRaises(ValueError):
            collector.closed_native_readout(self.archive([]), time.monotonic() - 1)

    def test_zip_metadata_allocation_is_bounded_before_zipfile(self):
        original = self.archive([('installed-a.observation.json', self.failure())])
        for kind in ('count', 'central-size', 'trailing'):
            with self.subTest(kind=kind):
                changed = bytearray(original)
                if kind == 'count':
                    collector.struct.pack_into('<H', changed, len(changed) - 12, len(collector.native_names()) + 1)
                elif kind == 'central-size':
                    collector.struct.pack_into('<L', changed, len(changed) - 10, 16385)
                else:
                    changed.extend(b'PRIVATE_TRAILING')
                with mock.patch.object(collector.zipfile, 'ZipFile') as archive:
                    with self.assertRaises(ValueError):
                        collector.closed_native_readout(bytes(changed), time.monotonic() + 5)
                    archive.assert_not_called()


    def test_actual_archive_preserves_only_closed_input_and_native_error_enums(self):
        for state in ('unobserved', 'none', 'action-picker', 'trust-picker', 'other', 'multiple'):
            for error in ('native-start', 'command-not-found'):
                value = self.failure()
                value['diagnostics']['mcp'].update(postStartInput=state, launchError=error)
                rows = collector.closed_native_readout(self.archive([('installed-a.observation.json', value)]), time.monotonic() + 5)
                self.assertEqual(rows[0]['mcp']['postStartInput'], state)
                self.assertEqual(rows[0]['mcp']['launchError'], error)
                self.assertNotIn('PRIVATE', json.dumps(rows))


    def test_native_log_is_separate_closed_settled_controller_projection(self):
        facts = dict(status='observed', transport_start=True, last_state='Starting', stopped=0, starting=1, running=0, error=0)
        controller = {'source_commit': 'a' * 40, 'source_tree': 'b' * 40, 'mode': 'trusted-project-v1',
                      'evidence_kind': 'vscode-pinned-trusted-project-gateway-lifecycle',
                      'observations': [{'stage': 'installed-a', 'nonce': 'PRIVATE_NONCE', 'native_log': facts, 'error': 'PRIVATE'}]}
        body = self.archive([('installed-a.observation.json', self.failure()), ('vscode-workspace-controller.json', controller)])
        rows = collector.closed_native_readout(body, time.monotonic() + 5, 'a' * 40, 'b' * 40)
        self.assertEqual(rows[0]['native_log'], facts)
        self.assertEqual(rows[0]['mcp']['lastState'], 'Stopped')
        self.assertNotIn('PRIVATE', json.dumps(rows))
        for key, value in [('status', 'PRIVATE'), ('last_state', 'PRIVATE'), ('transport_start', 1), ('starting', 256), ('path', 'PRIVATE')]:
            with self.subTest(key=key):
                changed = dict(facts); changed[key] = value
                controller['observations'][0]['native_log'] = changed
                with self.assertRaises(ValueError):
                    collector.closed_native_readout(self.archive([('installed-a.observation.json', self.failure()), ('vscode-workspace-controller.json', controller)]), time.monotonic() + 5, 'a' * 40, 'b' * 40)

        for field in ('source_commit', 'source_tree', 'mode', 'evidence_kind', 'nonce'):
            changed = copy.deepcopy(controller)
            changed['observations'][0]['native_log'] = dict(facts)
            if field == 'nonce': changed['observations'][0][field] = 'FOREIGN_NONCE'
            else: changed[field] = 'FOREIGN_SOURCE'
            with self.subTest(field=field), self.assertRaises(ValueError):
                collector.closed_native_readout(self.archive([('installed-a.observation.json', self.failure()), ('vscode-workspace-controller.json', changed)]), time.monotonic() + 5, 'a' * 40, 'b' * 40)


if __name__ == "__main__":
    unittest.main()
