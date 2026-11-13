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
