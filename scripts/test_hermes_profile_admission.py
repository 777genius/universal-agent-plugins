"""Inert admission contracts only: no vendor imports, execution or hosted spoofing."""
import json
import os
from pathlib import Path
import runpy
import tempfile
import unittest
from unittest import mock
from types import SimpleNamespace

CASE = runpy.run_path(str(Path(__file__).parent / "e2e/hermes-profile-admission.py"), run_name="hermes_profile_contract")


class ProfileAdmissionContracts(unittest.TestCase):
    def test_census_is_content_free_disjoint_and_does_not_follow_links(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            for name in ("user/selected-profile/cache", "user/selected-profile/tools", "user/selected-profile/installs", "project-a", "project-b"):
                (root / name).mkdir(parents=True, exist_ok=True)
                (root / name / "item").write_bytes(b"private")
            os.link(root / "project-a/item", root / "project-a/second")
            if os.name != "nt": (root / "project-b/link").symlink_to(root / "project-a", target_is_directory=True)
            expected = CASE["directory_identity"](root)
            with mock.patch.object(os, "readlink", side_effect=AssertionError("link target read")), mock.patch.object(os, "fdopen", side_effect=AssertionError("content read")), mock.patch.dict(CASE["metadata_census"].__globals__, {"ordinary_bytes": mock.Mock(side_effect=AssertionError("content read"))}):
                value = CASE["metadata_census"](root, expected, time.monotonic() + 5)
            self.assertTrue(value["complete"])
            CASE["validate_census_member"]("bootstrap-census.json", json.dumps(value).encode())
            self.assertEqual(value["logical_regular_bytes"], 42)
            self.assertEqual(value["groups"]["projectA"]["logical_regular_bytes"], 14)
            expected_links = max(os.lstat(root / name).st_nlink for name in ("project-a", "project-a/item", "project-a/second"))
            self.assertEqual(value["groups"]["projectA"]["max_links"], expected_links)
            self.assertEqual(value["groups"]["projectA"]["regular_hardlinked_entries"], 2)
            self.assertEqual(value["entries"], sum(row["entries"] for row in value["groups"].values()))
            self.assertTrue(all(row["entries"] == sum(row["kinds"].values()) for row in value["groups"].values()))
            self.assertNotIn("private", json.dumps(value))

    def test_census_root_replacement_budget_and_saturation_are_incomplete(self):
        import stat
        import time
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root = parent / "root"; root.mkdir()
            original = CASE["directory_identity"](root)
            (root / "file").write_bytes(b"unused")
            partial = CASE["metadata_census"](root, original, time.monotonic() - 1)
            self.assertEqual((partial["status"], partial["reason"], partial["entries"]), ("partial", "time-limit", 0))
            with mock.patch.object(CASE["time"], "monotonic", side_effect=[0, 10]):
                partial = CASE["metadata_census"](root, original, 5)
            self.assertEqual((partial["status"], partial["entries"]), ("partial", 1))
            actual_stat, actual_lstat = os.stat, Path.lstat
            def huge(path, *args, **kwargs):
                # POSIX traverses by dir_fd; Windows' path-based inventory calls Path.lstat.
                if (path == "file" and kwargs.get("dir_fd") is not None) or (
                    os.name == "nt" and isinstance(path, Path) and path == root / "file"
                ):
                    return SimpleNamespace(st_mode=stat.S_IFREG | 0o600, st_nlink=1 << 54, st_size=1 << 54)
                return actual_lstat(path, *args, **kwargs) if os.name == "nt" else actual_stat(path, *args, **kwargs)
            patch = (mock.patch.object(Path, "lstat", autospec=True, side_effect=huge) if os.name == "nt"
                     else mock.patch.object(os, "stat", side_effect=huge))
            with patch:
                value = CASE["metadata_census"](root, original, time.monotonic() + 5)
            self.assertTrue(value["overflow"]); self.assertFalse(value["complete"])
            self.assertEqual(value["logical_regular_bytes"], (1 << 53) - 1)
            root.rename(parent / "original"); root.mkdir()
            with mock.patch.object(os, "scandir", side_effect=AssertionError("foreign root enumerated")):
                refused = CASE["metadata_census"](root, original, time.monotonic() + 5)
            self.assertEqual(refused["status"], "refused")

    def test_census_member_depth_and_exhaustion_are_distinct(self):
        import time
        self.assertEqual(CASE["CENSUS_MEMBERS"], 131072)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            for index in range(8): (root / str(index)).touch()
            expected = CASE["directory_identity"](root)
            with mock.patch.dict(CASE["metadata_census"].__globals__, {"CENSUS_MEMBERS": 8}):
                value = CASE["metadata_census"](root, expected, time.monotonic() + 5)
                self.assertEqual((value["reason"], value["entries"], value["complete"]), ("member-limit", 8, False))
                CASE["validate_census_member"]("bootstrap-census.json", json.dumps(value).encode())
                value["reason"] = "time-limit"
                CASE["validate_census_member"]("bootstrap-census.json", json.dumps(value).encode())
                value["reason"], value["entries"] = "member-limit", 7
                group = value["groups"]["outer-other"]
                group["entries"] -= 1; group["kinds"]["regular"] -= 1
                with self.assertRaisesRegex(RuntimeError, "census member limit join"):
                    CASE["validate_census_member"]("bootstrap-census.json", json.dumps(value).encode())
            complete = CASE["metadata_census"](root, expected, time.monotonic() + 5)
            self.assertEqual((complete["complete"], complete["entries"]), (True, 9))
            with mock.patch.object(CASE["time"], "monotonic", side_effect=[0] * 9 + [10]):
                expired = CASE["metadata_census"](root, expected, 5)
            self.assertEqual((expired["reason"], expired["entries"], expired["complete"]), ("time-limit", 9, False))
            nested = root
            for _ in range(33): nested = nested / "d"; nested.mkdir()
            deep = CASE["metadata_census"](root, expected, time.monotonic() + 5)
            self.assertEqual((deep["reason"], deep["complete"]), ("depth-limit", False))
            CASE["validate_census_member"]("bootstrap-census.json", json.dumps(deep).encode())

    def test_census_request_requires_settlement_and_closed_identity_before_walk(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root, evidence = parent / "root", parent / "evidence"
            root.mkdir(); evidence.mkdir()
            expected = CASE["directory_identity"](root)
            with mock.patch.dict(CASE["maybe_census"].__globals__, {"metadata_census": mock.Mock(side_effect=AssertionError("fallback walk"))}):
                CASE["maybe_census"](evidence, root, expected, "c" * 64, True, time.monotonic() + 5, CASE["directory_identity"](evidence))
                self.assertEqual(list(evidence.iterdir()), [])
                CASE["publish_census_request"](evidence, root, "c" * 64, CASE["directory_identity"](evidence))
                CASE["maybe_census"](evidence, root, expected, "c" * 64, False, time.monotonic() + 5, CASE["directory_identity"](evidence))
                value = json.loads((evidence / "bootstrap-census.json").read_text())
                self.assertFalse(value["complete"])
            self.assertTrue({"bootstrap-census.json", "bootstrap-census-request.json"} <= CASE["MEMBERS"])
            self.assertLess((evidence / "bootstrap-census.json").stat().st_size, 16 << 10)
            (evidence / "bootstrap-census.json").unlink()
            with mock.patch.dict(CASE["publish_census_request"].__globals__, {"write_census_member": mock.Mock(side_effect=OSError("cannot publish"))}):
                CASE["publish_census_request"](evidence, root, "c" * 64, CASE["directory_identity"](evidence))
            (evidence / "bootstrap-census-request.json").write_text('{"schema_version":1,"schema_version":1}')
            with mock.patch.dict(CASE["maybe_census"].__globals__, {"metadata_census": mock.Mock(side_effect=AssertionError("invalid request walked"))}):
                CASE["maybe_census"](evidence, root, expected, "c" * 64, True, time.monotonic() + 5, CASE["directory_identity"](evidence))
            self.assertEqual(json.loads((evidence / "bootstrap-census.json").read_text())["reason"], "request")
            with self.assertRaises(RuntimeError): CASE["stage_evidence"](evidence)
            self.assertFalse((evidence / "upload").exists())
            (evidence / "bootstrap-census-request.json").unlink()
            CASE["publish_census_request"](evidence, root, "c" * 64, CASE["directory_identity"](evidence))
            CASE["stage_evidence"](evidence)
            self.assertEqual({path.name for path in (evidence / "upload").iterdir()}, {"bootstrap-census.json", "bootstrap-census-request.json"})

    def test_census_evidence_replacement_never_writes_foreign_parent(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root, evidence = parent / "root", parent / "evidence"
            root.mkdir(); evidence.mkdir()
            expected = CASE["directory_identity"](evidence)
            evidence.rename(parent / "original-evidence"); evidence.mkdir()
            sentinel = evidence / "foreign"
            sentinel.write_bytes(b"retained")
            with self.assertRaisesRegex(RuntimeError, "original census evidence changed"):
                CASE["maybe_census"](evidence, root, CASE["directory_identity"](root), "c" * 64, True, time.monotonic() + 5, expected)
            self.assertEqual({path.name for path in evidence.iterdir()}, {"foreign"})
            self.assertEqual(sentinel.read_bytes(), b"retained")

    def test_census_publication_refuses_oversize_unknown_and_boolean_documents(self):
        for name in ("bootstrap-census-request.json", "bootstrap-census.json"):
            for body in (b" " * ((16 << 10) + 1), b'{"private":"must not publish"}', b'{"schema_version":true}'):
                with self.subTest(name=name, body=body[:30]), tempfile.TemporaryDirectory() as temporary:
                    evidence = Path(temporary).resolve()
                    (evidence / name).write_bytes(body)
                    with self.assertRaises((RuntimeError, ValueError)):
                        CASE["stage_evidence"](evidence)
                    self.assertFalse((evidence / "upload").exists())

    def test_inventory_refusal_identifies_metadata_without_reading_member(self):
        for mutation in ("oversize", "hardlink", "symlink"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                member = root / "runtime-member"
                member.write_bytes(b"private contents must never appear in a refusal")
                if mutation == "oversize":
                    with member.open("r+b") as stream:
                        stream.truncate(CASE["LIMIT"] + 1)
                elif mutation == "hardlink":
                    os.link(member, root / "alias")
                else:
                    member.unlink()
                    member.symlink_to(root / "absent")
                with mock.patch.dict(CASE["inventory"].__globals__, {"ordinary_bytes": mock.Mock()}) as patched:
                    with self.assertRaisesRegex(RuntimeError, "inventory member refused:") as caught:
                        CASE["inventory"](root)
                    patched["ordinary_bytes"].assert_not_called()
                message = str(caught.exception)
                details = json.loads(message.split(": ", 1)[1])
                self.assertIn(details["member"], ("runtime-member", "alias"))
                self.assertEqual(set(details), {"member", "member_sha256", "mode", "links", "bytes"})
                self.assertLess(len(message), 800)
                self.assertNotIn(str(root), message)
                self.assertNotIn("private contents", message)

    def test_inventory_refusal_bounds_and_escapes_relative_member(self):
        info = SimpleNamespace(st_mode=0o100600, st_nlink=2, st_size=17)
        for relative in ("line\nname", "x" * 513):
            with self.assertRaises(RuntimeError) as caught:
                CASE["inventory_member"](relative, info)
            message = str(caught.exception)
            self.assertNotIn("\n", message)
            self.assertLess(len(message), 800)
            value = json.loads(message.split(": ", 1)[1])
            self.assertEqual(value["member"], None if len(relative) > 512 else relative)
            self.assertEqual(len(value["member_sha256"]), 64)

    def test_inventory_detects_bytes_modes_directories_and_aliases(self):
        for change in ("bytes", "mode", "directory", "symlink", "hardlink"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                owned = root / "foreign"
                owned.write_bytes(b"unchanged")
                before = CASE["inventory"](root)
                if change == "bytes":
                    owned.write_bytes(b"changed!!")
                elif change == "mode":
                    owned.chmod(0o400)
                elif change == "directory":
                    (root / "new-empty").mkdir()
                elif change == "symlink":
                    (root / "alias").symlink_to(owned)
                else:
                    os.link(owned, root / "alias")
                if change in ("symlink", "hardlink"):
                    with self.assertRaises(RuntimeError):
                        CASE["inventory"](root)
                else:
                    self.assertTrue(CASE["changes"](before, CASE["inventory"](root)))

    def test_inventory_refuses_directory_swap_before_or_during_enumeration(self):
        for boundary in ("before-open", "during-scan"):
            with self.subTest(boundary=boundary), tempfile.TemporaryDirectory() as temporary:
                parent = Path(temporary).resolve()
                root, foreign = parent / "root", parent / "foreign"
                root.mkdir(); foreign.mkdir()
                child = root / "child"
                child.mkdir()
                (foreign / "foreign-directory").mkdir()
                sentinel = foreign / "sentinel"
                sentinel.write_bytes(b"foreign contents")
                member = CASE["inventory_member"]
                scan = os.scandir
                def replace():
                    child.rename(root / "original-child")
                    child.symlink_to(foreign, target_is_directory=True)
                def checked(relative, info):
                    member(relative, info)
                    if boundary == "before-open" and relative == "child": replace()
                def scanning(operand):
                    selected = (Path(operand) == child if not isinstance(operand, int)
                                else os.fstat(operand).st_ino == child.lstat().st_ino)
                    entries = scan(operand)
                    if boundary == "during-scan" and selected:
                        try: replace()
                        except BaseException:
                            entries.close()
                            raise
                    return entries
                with mock.patch.dict(CASE["inventory"].__globals__, {"inventory_member": checked}), mock.patch.object(os, "scandir", side_effect=scanning):
                    # Windows rejects a replaced directory through its typed identity guard.
                    with self.assertRaises((RuntimeError, OSError, ValueError)):
                        CASE["inventory"](root)
                self.assertEqual(sentinel.read_bytes(), b"foreign contents")

    @unittest.skipIf(os.name == "nt", "Windows held directory denies rename instead")
    def test_posix_ordinary_parent_replacement_never_reads_foreign_same_name(self):
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root, foreign = parent / "root", parent / "foreign"
            root.mkdir(); foreign.mkdir()
            child = root / "child"
            child.mkdir()
            (child / "same-name").write_bytes(b"owned")
            secret = foreign / "same-name"
            secret.write_bytes(b"foreign private contents")
            secret_id = (secret.stat().st_dev, secret.stat().st_ino)
            scan, fdopen = os.scandir, os.fdopen
            replaced, read_foreign = [], []
            def scanning(operand):
                entries = scan(operand)
                if isinstance(operand, int) and os.fstat(operand).st_ino == child.lstat().st_ino:
                    child.rename(root / "original-child")
                    foreign.rename(child)
                    replaced.append(True)
                return entries
            def guarded_open(descriptor, *args, **kwargs):
                info = os.fstat(descriptor)
                if (info.st_dev, info.st_ino) == secret_id:
                    read_foreign.append(True)
                    os.close(descriptor)
                    raise AssertionError("foreign bytes would be read")
                return fdopen(descriptor, *args, **kwargs)
            with mock.patch.object(os, "scandir", side_effect=scanning), mock.patch.object(os, "fdopen", side_effect=guarded_open):
                with self.assertRaisesRegex(RuntimeError, "inventory directory changed"):
                    CASE["inventory"](root)
            self.assertEqual(replaced, [True])
            self.assertEqual(read_foreign, [])
            self.assertEqual((child / "same-name").read_bytes(), b"foreign private contents")

    def test_initial_root_replacement_never_reads_foreign_inode(self):
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root, foreign = parent / "root", parent / "foreign"
            root.mkdir(); foreign.mkdir()
            secret = foreign / "same-name"
            secret.write_bytes(b"foreign private contents")
            secret_id = (secret.stat().st_dev, secret.stat().st_ino)
            original, fdopen = CASE["directory_identity"], os.fdopen
            def swap(path):
                identity = original(path)
                root.rename(parent / "original-root")
                foreign.rename(root)
                return identity
            def guarded_open(descriptor, *args, **kwargs):
                info = os.fstat(descriptor)
                if (info.st_dev, info.st_ino) == secret_id:
                    os.close(descriptor)
                    raise AssertionError("foreign root bytes would be read")
                return fdopen(descriptor, *args, **kwargs)
            with mock.patch.dict(CASE["inventory"].__globals__, {"directory_identity": swap}), mock.patch.object(os, "fdopen", side_effect=guarded_open):
                with self.assertRaisesRegex(RuntimeError, "inventory original root changed before open"):
                    CASE["inventory"](root)
            self.assertEqual((root / "same-name").read_bytes(), b"foreign private contents")

    def test_windows_directory_handle_uses_no_reparse_and_no_delete_sharing(self):
        import ctypes
        import sys
        create, close = mock.Mock(return_value=123), mock.Mock(return_value=1)
        transfer = mock.Mock(return_value=17)
        kernel = SimpleNamespace(CreateFileW=create, CloseHandle=close)
        with mock.patch.object(CASE["artifacts"], "WINDOWS", True), mock.patch.object(ctypes, "WinDLL", return_value=kernel, create=True), mock.patch.dict(sys.modules, {"msvcrt": SimpleNamespace(open_osfhandle=transfer)}), mock.patch.object(os, "O_BINARY", 0x8000, create=True):
            self.assertEqual(CASE["open_inventory_directory"](Path("inert-directory")), 17)
            self.assertEqual(create.call_args.args, ("inert-directory", 0x80000000, 3, None, 3, 0x02200000, None))
            self.assertIs(create.restype, ctypes.c_void_p)
            transfer.assert_called_once_with(123, os.O_RDONLY | 0x8000)
            close.assert_not_called()
            transfer.side_effect = OSError("transfer failed")
            with self.assertRaises(OSError): CASE["open_inventory_directory"](Path("inert-directory"))
            close.assert_called_once_with(123)

    def test_windows_held_directory_requires_positive_non_reparse_identity(self):
        import stat
        base = dict(st_dev=4, st_ino=5, st_mode=stat.S_IFDIR | 0o755, st_nlink=1,
                    st_size=0, st_mtime_ns=100, st_birthtime_ns=90,
                    st_file_attributes=16, st_reparse_tag=0)
        with mock.patch.object(CASE["artifacts"], "WINDOWS", True):
            original = SimpleNamespace(**base)
            self.assertTrue(CASE["same_inventory_directory"](original, SimpleNamespace(**dict(base, st_mode=stat.S_IFDIR | 0o700))))
            # Verified on the real Windows runner: path lstat and held fstat
            # can disagree on ordinary-directory attribute flags.
            self.assertTrue(CASE["same_inventory_directory"](
                original, SimpleNamespace(**dict(base, st_file_attributes=32))))
            self.assertFalse(CASE["same_inventory_directory"](original, SimpleNamespace(**dict(base, st_ino=6))))
            for change in ({"st_file_attributes": 16 | 0x400}, {"st_reparse_tag": 1}, {"st_birthtime_ns": None}):
                with self.assertRaises(ValueError): CASE["same_inventory_directory"](original, SimpleNamespace(**dict(base, **change)))

    def test_windows_directory_mismatch_reports_only_field_names(self):
        import stat
        base = dict(st_dev=4, st_ino=5, st_mode=stat.S_IFDIR | 0o755,
                    st_nlink=1, st_size=0, st_mtime_ns=100, st_birthtime_ns=90,
                    st_file_attributes=16, st_reparse_tag=0)
        original = SimpleNamespace(**base)
        with mock.patch.object(CASE["artifacts"], "WINDOWS", True):
            CASE["check_inventory_directory"](
                original, SimpleNamespace(**dict(base, st_mode=stat.S_IFDIR | 0o700)),
                "inventory directory changed at open")
            CASE["check_inventory_directory"](
                original, SimpleNamespace(**dict(base, st_file_attributes=32)),
                "inventory directory changed at open")
            for field, value, label in (
                ("st_dev", 6, "device"), ("st_ino", 6, "inode"),
                ("st_nlink", 2, "links"), ("st_size", 123456789, "bytes"),
                ("st_mtime_ns", 101, "mtime"), ("st_birthtime_ns", 91, "birth"),
            ):
                with self.subTest(field=field), self.assertRaises(RuntimeError) as caught:
                    CASE["check_inventory_directory"](
                        original, SimpleNamespace(**dict(base, **{field: value})),
                        "inventory directory changed at open")
                message = str(caught.exception)
                self.assertIn("Windows metadata: " + label, message)
                self.assertNotIn("123456789", message)
                self.assertNotIn("\\", message)
            for change in ({"st_file_attributes": 16 | 0x400},
                           {"st_reparse_tag": 1}, {"st_birthtime_ns": None}):
                with self.assertRaises(ValueError):
                    CASE["check_inventory_directory"](
                        original, SimpleNamespace(**dict(base, **change)),
                        "inventory directory changed at open")
            with self.assertRaises(RuntimeError):
                CASE["check_inventory_directory"](
                    original, SimpleNamespace(**dict(base, st_mode=stat.S_IFREG | 0o755)),
                    "inventory directory changed at open")

    def test_inventory_does_not_eagerly_enumerate_past_membership_bound(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            for index in range(2048):
                (root / f"member-{index:04d}").mkdir()
            original_scandir, observed = os.scandir, [0]

            class CountedScan:
                def __init__(self, stream):
                    self.stream = stream

                def __enter__(self):
                    self.stream.__enter__()
                    return self

                def __exit__(self, *args):
                    return self.stream.__exit__(*args)

                def __iter__(self):
                    return self

                def __next__(self):
                    item = next(self.stream)
                    observed[0] += 1
                    if observed[0] > 2048:
                        raise RuntimeError("scanned beyond membership bound")
                    return item

            with mock.patch.object(os, "scandir", side_effect=lambda path: CountedScan(original_scandir(path))):
                with self.assertRaisesRegex(RuntimeError, "inventory membership bound"):
                    CASE["inventory"](root)
            self.assertEqual(observed[0], 2048)

    def test_protected_original_user_cannot_be_rebaselined(self):
        before = {"auth.json": {"sha256": "old"}, "history": {"sha256": "history"}}
        CASE["protected_unchanged"](before, dict(before), before)
        for name in before:
            changed = {**before, name: {"sha256": "native-new"}}
            with self.assertRaises(RuntimeError):
                CASE["protected_unchanged"](before, changed, before)

    def test_guard_rejects_live_gateway_multiplexer_service_and_external_memory(self):
        clean = {"gateway_running": False, "default_gateway_pid": None, "served_profiles": None,
                 "service_manager": "none", "memory_provider": None}
        CASE["validate_guards"](clean)
        for key, value in (("gateway_running", True), ("default_gateway_pid", 123),
                           ("served_profiles", []), ("service_manager", "s6"), ("memory_provider", "external")):
            with self.subTest(key=key), self.assertRaises(RuntimeError):
                CASE["validate_guards"]({**clean, key: value})
        with self.assertRaises(RuntimeError):
            CASE["validate_guards"]({**clean, "unexpected": False})

    def test_protocol_requires_selected_endpoint_and_zero_sibling_events(self):
        before = {"a": [], "b": []}
        valid = {"a": ["initialize", "notifications/initialized", "tools/list"], "b": []}
        CASE["protocol_join"](before, valid, "a", 1)
        CASE["protocol_join"](valid, valid, None, 0)
        for after in ({"a": ["initialize"], "b": []},
                      {"a": valid["a"], "b": ["initialize"]},
                      {"a": valid["a"] + ["tools/call"], "b": []},
                      {"a": ["tools/list", "initialize"], "b": []}):
            with self.assertRaises(RuntimeError):
                CASE["protocol_join"](before, after, "a", 1)
        with self.assertRaises(RuntimeError):
            CASE["protocol_join"](valid, before, "a", 1)

    def test_custom_root_must_not_be_original_native_home_or_named_profile(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            user = root / "user"
            native = user / ".hermes"
            a, b = root / "project-a" / "runtime", root / "project-b" / "runtime"
            for path in (native, a, b):
                path.mkdir(parents=True)
            CASE["validate_roots"](user, native, [a, b])
            invalid = [native, native / "child", user, root / "profiles" / "named"]
            for path in invalid:
                path.mkdir(parents=True, exist_ok=True)
                with self.subTest(path=path), self.assertRaises(RuntimeError):
                    CASE["validate_roots"](user, native, [path, b])
            with self.assertRaises(RuntimeError):
                CASE["validate_roots"](user, native, [a, a])

    def test_no_user_model_auth_or_history_copy_is_tolerated(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            config = root / "config.yaml"
            config.write_text("mcp_servers: {}\n")
            CASE["no_user_copies"](root, ["USER_MODEL_nonce", "USER_AUTH_nonce", "USER_HISTORY_nonce"])
            for marker in ("USER_MODEL_nonce", "USER_AUTH_nonce", "USER_HISTORY_nonce"):
                config.write_text(marker)
                with self.subTest(marker=marker), self.assertRaises(RuntimeError):
                    CASE["no_user_copies"](root, [marker])

    def test_census_readout_uses_closed_staged_bytes_without_private_context(self):
        import time
        stage = CASE["stage_evidence"]
        authority = stage.__globals__
        with tempfile.TemporaryDirectory() as temporary:
            native, evidence = Path(temporary).resolve() / "native", Path(temporary).resolve() / "evidence"
            native.mkdir(); evidence.mkdir()
            (native / "PRIVATE-NAME").write_bytes(b"PRIVATE-CONTENT")
            identity = CASE["directory_identity"](native)
            census = CASE["metadata_census"](native, identity, time.monotonic() + 5)
            nonce = "f" * 64
            (evidence / "bootstrap-census.json").write_text(json.dumps(census))
            (evidence / "bootstrap-census-request.json").write_text(json.dumps({
                "schema_version": 1, "reason": "initial-inventory-refused", "nonce": nonce,
                "root_identity": list(identity)}))
            (evidence / "profile-admission.json").write_text('{"admission_qualified":false}')
            lines, reads, actual = [], [], CASE["ordinary_bytes"]
            def read(path, *args, **kwargs):
                self.assertFalse((evidence / "upload").exists(), "readout reread after commit")
                reads.append(path.name)
                return actual(path, *args, **kwargs)
            def emit(line, *, flush):
                self.assertTrue(flush)
                self.assertTrue(authority["EVIDENCE_CLOSED"])
                self.assertTrue((evidence / "upload").is_dir())
                lines.append(line)
            with mock.patch.dict(authority, {"ordinary_bytes": read, "print": emit, "EVIDENCE_CLOSED": False}):
                stage(evidence)
                self.assertTrue(authority["EVIDENCE_CLOSED"])
            self.assertEqual(len(lines), 1)
            self.assertEqual(len(reads), 5)
            report = json.loads(lines[0].removeprefix("HERMES_BOOTSTRAP_CENSUS "))
            self.assertEqual(report["kind"], "hermes-bootstrap-census-readout")
            self.assertTrue(report["present"])
            self.assertTrue(report["measurement_only"])
            self.assertFalse(report["admission_qualified"])
            self.assertEqual(report["census"], census)
            for private in (nonce, str(native), str(evidence), "PRIVATE-NAME", "PRIVATE-CONTENT"):
                self.assertNotIn(private, lines[0])

    def test_census_readout_missing_invalid_commit_and_stdout_failures(self):
        stage = CASE["stage_evidence"]
        authority = stage.__globals__
        for mode in ("missing", "invalid", "rename", "stdout-oserror", "stdout-valueerror"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as temporary:
                evidence = Path(temporary).resolve()
                proof = b'{"admission_qualified":false,"error":"original native failure"}'
                (evidence / "profile-admission.json").write_bytes(proof)
                if mode != "missing":
                    census = {"schema_version": 1, "measurement_only": True, "admission_qualified": False,
                              "complete": False, "status": "refused", "reason": "request"}
                    if mode == "invalid": census["nonce"] = "private-not-a-census-field"
                    (evidence / "bootstrap-census.json").write_text(json.dumps(census))
                lines = []
                def emit(line, *, flush):
                    self.assertTrue(authority["EVIDENCE_CLOSED"])
                    self.assertTrue((evidence / "upload").is_dir())
                    lines.append(line)
                    if mode == "stdout-oserror": raise OSError("inert stdout closed")
                    if mode == "stdout-valueerror": raise ValueError("inert stream closed")
                with mock.patch.dict(authority, {"print": emit, "EVIDENCE_CLOSED": False}):
                    if mode == "invalid":
                        with self.assertRaisesRegex(RuntimeError, "closed census schema"):
                            stage(evidence)
                    elif mode == "rename":
                        with mock.patch.object(Path, "rename", side_effect=OSError("inert commit refusal")):
                            with self.assertRaisesRegex(OSError, "inert commit refusal"):
                                stage(evidence)
                    else:
                        stage(evidence)
                    if mode in ("invalid", "rename"):
                        self.assertFalse(authority["EVIDENCE_CLOSED"])
                        self.assertFalse((evidence / "upload").exists())
                        self.assertFalse(list(evidence.glob(".upload-pending-*")))
                        self.assertEqual(lines, [])
                    else:
                        self.assertTrue(authority["EVIDENCE_CLOSED"])
                        self.assertEqual((evidence / "upload/profile-admission.json").read_bytes(), proof)
                        self.assertEqual(len(lines), 1)
                        if mode == "missing":
                            self.assertEqual(json.loads(lines[0].removeprefix("HERMES_BOOTSTRAP_CENSUS ")), {
                                "schema_version": 1, "kind": "hermes-bootstrap-census-readout",
                                "measurement_only": True, "admission_qualified": False, "present": False})
                self.assertEqual((evidence / "profile-admission.json").read_bytes(), proof)

    def test_closed_upload_rejects_unknown_members_and_never_stages_private_tree(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            (root / "auth.json").write_text('{"inert":"must never upload"}')
            with self.assertRaises(RuntimeError):
                CASE["stage_evidence"](root)
            self.assertFalse((root / "upload").exists())
            (root / "auth.json").unlink()
            (root / "profile-admission.json").write_text('{"admission_qualified":false}')
            CASE["stage_evidence"](root)
            self.assertEqual([p.name for p in (root / "upload").iterdir()], ["profile-admission.json"])
            self.assertFalse(json.loads((root / "upload/profile-admission.json").read_text())["admission_qualified"])

    def test_late_second_member_failure_never_publishes_partial_upload(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            (root / "lifecycle.stdout").write_bytes(b"first")
            (root / "lifecycle.stderr").write_bytes(b"second")
            actual, calls = CASE["ordinary_bytes"], []
            def fail_second(path, expected=None):
                calls.append(path.name)
                if len(calls) == 2:
                    raise RuntimeError("late source read failure")
                return actual(path, expected)
            with mock.patch.dict(CASE["stage_evidence"].__globals__, {"ordinary_bytes": fail_second}):
                with self.assertRaisesRegex(RuntimeError, "late source read"):
                    CASE["stage_evidence"](root)
            self.assertEqual(len(calls), 2)
            self.assertFalse((root / "upload").exists())
            self.assertFalse(list(root.glob(".upload-pending-*")))

    def test_reader_rejects_wrong_opened_identity_before_reading(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            wanted, foreign = root / "wanted", root / "foreign"
            wanted.write_bytes(b"wanted")
            foreign.write_bytes(b"foreign")
            descriptor = CASE["artifacts"].open_member(foreign)
            with mock.patch.object(CASE["artifacts"], "open_member", return_value=descriptor):
                with mock.patch.object(os, "fdopen") as read:
                    with self.assertRaisesRegex(RuntimeError, "changed at open"):
                        CASE["ordinary_bytes"](wanted)
                    read.assert_not_called()
            with mock.patch.object(CASE["artifacts"], "open_member") as opened:
                with self.assertRaisesRegex(RuntimeError, "before open"):
                    CASE["ordinary_bytes"](wanted, foreign.stat())
                opened.assert_not_called()
            wanted.write_bytes(b"x" * ((1 << 20) + 1))
            with self.assertRaisesRegex(RuntimeError, "bounded ordinary"):
                CASE["ordinary_bytes"](wanted)

    @unittest.skipUnless(hasattr(os, "mkfifo") and hasattr(os, "O_NONBLOCK"), "POSIX FIFO replacement contract")
    def test_fifo_swap_is_nonblocking_and_refused_before_any_read(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            wanted, fifo = root / "wanted", root / "fifo"
            wanted.write_bytes(b"regular before open")
            os.mkfifo(fifo)
            original_open = os.open
            def swapped_open(path, flags):
                self.assertTrue(flags & os.O_NONBLOCK)
                return original_open(fifo, flags)
            with mock.patch.object(os, "open", side_effect=swapped_open):
                with mock.patch.object(os, "fdopen") as read:
                    with self.assertRaisesRegex(RuntimeError, "changed at open"):
                        CASE["ordinary_bytes"](wanted)
                    read.assert_not_called()

    def test_windows_path_and_fd_domains_keep_separate_complete_stability(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary).resolve() / "ordinary"
            path.write_bytes(b"portable bytes")
            real_lstat, real_fstat, real_open = Path.lstat, os.fstat, os.open
            def win_info(info, held=False, **changes):
                names = ("st_dev", "st_ino", "st_mode", "st_nlink", "st_uid", "st_gid", "st_size", "st_mtime_ns", "st_ctime_ns")
                values = {name: getattr(info, name) for name in names}
                values.update(st_birthtime_ns=1234567, st_file_attributes=0, st_reparse_tag=0)
                if held:
                    values["st_ctime_ns"] += 100000
                    # Windows path and opened-handle APIs can synthesize
                    # execute permission bits differently on the same file.
                    values["st_mode"] ^= 0o111
                values.update(changes)
                return SimpleNamespace(**values)
            for mutation in (None, "path-ctime", "held-ctime", "path-mode", "held-mode",
                             "file-type", "inode", "birth", "reparse"):
                counts = {"path": 0, "held": 0}
                def path_stat(p):
                    counts["path"] += 1
                    value = win_info(real_lstat(p))
                    if mutation == "path-ctime" and counts["path"] > 1:
                        value.st_ctime_ns += 1
                    if mutation == "path-mode" and counts["path"] > 1:
                        value.st_mode ^= 0o200
                    return value
                def held_stat(fd):
                    counts["held"] += 1
                    value = win_info(real_fstat(fd), held=True)
                    if mutation == "held-ctime" and counts["held"] > 1:
                        value.st_ctime_ns += 1
                    if mutation == "held-mode" and counts["held"] > 1:
                        value.st_mode ^= 0o200
                    if mutation == "file-type":
                        value.st_mode = (value.st_mode & 0o7777) | 0o040000
                    if mutation == "inode": value.st_ino += 1
                    if mutation == "birth": value.st_birthtime_ns += 1
                    if mutation == "reparse": value.st_reparse_tag = 1
                    return value
                with self.subTest(mutation=mutation), mock.patch.object(CASE["artifacts"], "WINDOWS", True):
                    # Metadata-domain simulation only. No native Windows success is claimed.
                    with mock.patch.object(Path, "lstat", autospec=True, side_effect=path_stat), mock.patch.object(os, "fstat", side_effect=held_stat):
                        with mock.patch.object(CASE["artifacts"], "open_member", side_effect=lambda p: real_open(p, os.O_RDONLY)):
                            if mutation is None:
                                self.assertEqual(CASE["ordinary_bytes"](path), b"portable bytes")
                            else:
                                with self.assertRaises((RuntimeError, ValueError)):
                                    CASE["ordinary_bytes"](path)

    def test_file_read_access_time_does_not_change_stable_identity(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary).resolve() / "ordinary"
            path.write_bytes(b"read exactly")
            current = path.stat()
            os.utime(path, ns=(1, current.st_mtime_ns))
            before = path.stat()
            self.assertEqual(CASE["ordinary_bytes"](path), b"read exactly")
            self.assertEqual(CASE["stable_file"](before), CASE["stable_file"](path.stat()))

    def test_original_custom_directory_replacement_or_mode_change_refuses(self):
        for mutation in ("replacement", "mode"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                parent = Path(temporary).resolve()
                root = parent / "selected"
                root.mkdir(mode=0o700)
                held = {"a": CASE["directory_identity"](root)}
                if mutation == "replacement":
                    root.rename(parent / "original")
                    root.mkdir(mode=0o700)
                else:
                    root.chmod(0o500)
                with self.assertRaises(RuntimeError):
                    CASE["verify_root_identities"]({"a": root}, held)

    def test_exact_config_boundary_rejects_extra_server_wrong_url_and_other_settings(self):
        # These are mapping/byte contracts. The hosted child alone imports the
        # pinned Hermes YAML parser; no substitute parser is admitted here.
        boundary = CASE["config_document_boundary"]
        url = "http://127.0.0.1:12345/mcp"
        valid = {"mcp_servers": {"owned": {"url": url}}}
        body = json.dumps(valid).encode()
        digest = boundary(body, valid, "owned", url)
        boundary(body, valid, "owned", url, authored_sha=digest)
        with self.assertRaisesRegex(RuntimeError, "config bytes changed"):
            boundary(body + b"\n", valid, "owned", url, authored_sha=digest)
        for value in ({"mcp_servers": {"owned": {"url": url + "-changed"}}},
                      {"mcp_servers": {"owned": {"url": url}, "foreign": {"url": url}}},
                      {**valid, "model": "copied"}, {**valid, "auth": {}}, None):
            with self.assertRaises(RuntimeError):
                boundary(json.dumps(value).encode(), value, "owned", url)
        for value in ({}, {"mcp_servers": {}}):
            body = json.dumps(value).encode()
            boundary(body, value, "owned", url, removed=True)
            with self.assertRaises(RuntimeError):
                boundary(body, value, "owned", url, authored_sha=digest)

    def test_safe_failure_envelope_keeps_every_qualification_false(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = CASE["safe_failure_evidence"](Path(temporary).resolve())
            proof = json.loads((root / "upload/profile-admission.json").read_text())
            self.assertEqual(proof["status"], "failed")
            for key in ("admission_qualified", "production_project_mcp", "vendor_authentication", "real_model_inference", "tools_call"):
                self.assertIs(proof[key], False)


if __name__ == "__main__":
    unittest.main()
