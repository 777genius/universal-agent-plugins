"""Inert admission contracts only: no vendor imports, execution or hosted spoofing."""
import json
import os
from pathlib import Path
import runpy
import tempfile
import unittest
from unittest import mock

CASE = runpy.run_path(str(Path(__file__).parent / "e2e/hermes-profile-admission.py"), run_name="hermes_profile_contract")


class ProfileAdmissionContracts(unittest.TestCase):
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
            descriptor = os.open(foreign, os.O_RDONLY)
            with mock.patch.object(os, "open", return_value=descriptor):
                with mock.patch.object(os, "fdopen") as read:
                    with self.assertRaisesRegex(RuntimeError, "changed at open"):
                        CASE["ordinary_bytes"](wanted)
                    read.assert_not_called()
            with mock.patch.object(os, "open") as opened:
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
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            config = root / "config.yaml"
            url = "http://127.0.0.1:12345/mcp"
            valid = {"mcp_servers": {"owned": {"url": url}}}
            config.write_text(json.dumps(valid))
            digest = CASE["config_boundary"](root, "owned", url)
            CASE["config_boundary"](root, "owned", url, authored_sha=digest)
            for value in ({"mcp_servers": {"owned": {"url": url + "-changed"}}},
                          {"mcp_servers": {"owned": {"url": url}, "foreign": {"url": url}}},
                          {**valid, "model": "copied"}, {**valid, "auth": {}}, None):
                config.write_text(json.dumps(value))
                with self.assertRaises(RuntimeError):
                    CASE["config_boundary"](root, "owned", url)
            for value in ({}, {"mcp_servers": {}}):
                config.write_text(json.dumps(value))
                CASE["config_boundary"](root, "owned", url, removed=True)
                with self.assertRaises(RuntimeError):
                    CASE["config_boundary"](root, "owned", url, authored_sha=digest)

    def test_safe_failure_envelope_keeps_every_qualification_false(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = CASE["safe_failure_evidence"](Path(temporary).resolve())
            proof = json.loads((root / "upload/profile-admission.json").read_text())
            self.assertEqual(proof["status"], "failed")
            for key in ("admission_qualified", "production_project_mcp", "vendor_authentication", "real_model_inference", "tools_call"):
                self.assertIs(proof[key], False)


if __name__ == "__main__":
    unittest.main()
