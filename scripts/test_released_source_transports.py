"""Offline frozen .116 transport/data contracts, never native qualification."""
import copy
import importlib.util
import json
import os
from pathlib import Path, PureWindowsPath
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location("transport_audit", Path(__file__).with_name("released-source-audit.py"))
audit = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(audit)
closure = audit.module("transport_closure", "released_source_closure.py")


class OwnedDataTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name).resolve()
        self.case = {"client": "pi", "scope": "project"}
        self.iid = "00000000-0000-4000-8000-000000000001"
        self.physical = "playwright-" + audit.hashlib.sha256(self.iid.encode()).hexdigest()[:12]
        self.path = self.root / "project/.uap/plugin-data" / self.physical
        self.path.mkdir(parents=True, mode=0o700)
        self.path.chmod(0o700)
        rid = "data_" + audit.hashlib.sha256("\0".join((self.iid, self.physical, "project")).encode()).hexdigest()[:24]
        self.core = {"data_receipt_id": rid, "physical_backend_id": self.physical, "scope": "project",
                     "locator": str(self.path), "state": "owned",
                     "ownership_digest": audit.digest(("agentplugins-plugin-data-v1\0" + rid + "\0" + str(self.path)).encode())}
        self.marker = self.path / ".agentplugins-data-owner.json"
        self.write_marker(self.core)
        self.owner = {"installation": {"installation_id": self.iid, "data_receipts": {rid: copy.deepcopy(self.core)}},
                      "binding": {"physical_artifact_id": self.physical, "data_receipt_id": rid}}

    def write_marker(self, value):
        self.marker.write_text(json.dumps(value), encoding="utf-8")
        self.marker.chmod(0o600)

    def check(self, **kwargs):
        return closure.checked_owned_data(audit, self.case, self.root, "playwright", self.owner, **kwargs)

    def test_exact_core_receipt_and_held_private_marker_are_accepted(self):
        self.assertEqual(self.check(), {"path": self.path, "receipt": self.core})

    def test_each_state_and_marker_core_field_is_independently_bound(self):
        for field in self.core:
            for location in ("state", "marker"):
                with self.subTest(field=field, location=location):
                    altered = {**self.core, field: "foreign"}
                    self.owner["installation"]["data_receipts"][self.core["data_receipt_id"]] = altered if location == "state" else copy.deepcopy(self.core)
                    self.write_marker(altered if location == "marker" else self.core)
                    with self.assertRaises(ValueError):
                        self.check()

    def test_extra_receipt_and_unknown_marker_field_are_refused(self):
        self.owner["installation"]["data_receipts"]["foreign"] = self.core
        with self.assertRaisesRegex(ValueError, "inventory"):
            self.check()
        del self.owner["installation"]["data_receipts"]["foreign"]
        self.write_marker({**self.core, "extra": "unowned"})
        with self.assertRaisesRegex(ValueError, "marker differs"):
            self.check()

    def test_binding_and_derived_physical_identity_are_required(self):
        self.owner["binding"]["data_receipt_id"] = "foreign"
        with self.assertRaisesRegex(ValueError, "binding"):
            self.check()
        del self.owner["binding"]["data_receipt_id"]
        with self.assertRaisesRegex(ValueError, "binding"):
            self.check()
        self.assertEqual(self.check(bound=False)["receipt"], self.core)
        self.owner["binding"]["physical_artifact_id"] = "foreign"
        with self.assertRaisesRegex(ValueError, "physical"):
            self.check(bound=False)

    def test_optional_timestamps_are_not_native_path_authority(self):
        self.write_marker({**self.core, "created_at": "2026-11-25T00:00:00Z"})
        self.assertEqual(self.check()["path"], self.path)
        self.write_marker({**self.core, "created_at": {"path": str(self.path)}})
        with self.assertRaises(ValueError):
            self.check()

    def test_duplicate_marker_key_is_refused(self):
        body = json.dumps(self.core)
        self.marker.write_text(body.replace('"state": "owned"', '"state": "owned", "state": "owned"'), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "duplicate"):
            self.check()

    @unittest.skipIf(os.name == "nt", "POSIX private mode fixture")
    def test_nonprivate_directory_or_marker_and_hardlink_are_refused(self):
        self.path.chmod(0o755)
        with self.assertRaisesRegex(ValueError, "directory"):
            self.check()
        self.path.chmod(0o700)
        self.marker.chmod(0o644)
        with self.assertRaisesRegex(ValueError, "marker"):
            self.check()
        self.marker.chmod(0o600)
        os.link(self.marker, self.root / "foreign-hardlink")
        with self.assertRaisesRegex(ValueError, "marker"):
            self.check()

    def test_same_bytes_marker_replacement_during_held_read_is_refused(self):
        original = closure.held_read
        def replace(*args, **kwargs):
            body = original(*args, **kwargs)
            replacement = self.path / "replacement"
            replacement.write_bytes(body)
            replacement.chmod(0o600)
            replacement.replace(self.marker)
            return body
        with mock.patch.object(closure, "held_read", side_effect=replace):
            with self.assertRaisesRegex(ValueError, "marker changed"):
                self.check()

    def test_foreign_scope_path_cannot_supply_data_authority(self):
        self.case["scope"] = "user"
        with self.assertRaises(ValueError):
            self.check()


class FrozenTransportTests(unittest.TestCase):
    def test_full_maps_preserve_frozen_env_cwd_and_windows_literal_suffix(self):
        # Frozen shared/stdio.go expands tokens by literal replacement, while
        # antigravity/project.go deletes reserved env keys but retains env:{}.
        for locator, data in ((Path("/case/native/playwright"), Path("/case/data/playwright")),
                              (PureWindowsPath(r"C:\case\native\playwright"), PureWindowsPath(r"C:\case\data\playwright"))):
            for client in ("pi", "hermes", "antigravity"):
                with self.subTest(locator=str(locator), client=client), \
                     mock.patch.object(audit, "in_case", side_effect=lambda path, root: path), \
                     mock.patch.object(closure, "checked_playwright_runtime") as runtime, \
                     mock.patch.object(closure, "checked_owned_data", return_value={"path": data}) as owned:
                    selected = {"client": client, "scope": "user"}
                    owner = {"locator": locator}
                    value = closure.checked_playwright_transport(audit, selected, Path("/case"), owner)
                    expected = {"command": "node", "args": [str(locator) + "/io.github.777genius.agentplugins/runtime/launcher.mjs"],
                                "env": {} if client == "antigravity" else {"PLUGIN_ROOT": str(locator), "PLUGIN_DATA": str(data)},
                                "cwd": str(locator)}
                    self.assertEqual(value, expected)
                    runtime.assert_called_once_with(audit, locator, Path("/case"))
                    owned.assert_called_once_with(audit, selected, Path("/case"), "playwright", owner)
                    for field in ("env", "cwd"):
                        mutated = dict(expected)
                        del mutated[field]
                        self.assertNotEqual(value, mutated)
                    if isinstance(locator, PureWindowsPath):
                        self.assertNotEqual(value["args"], [str(locator / "io.github.777genius.agentplugins/runtime/launcher.mjs")])

    def test_runtime_and_owned_data_refusals_prevent_transport_proof(self):
        for failing in ("checked_playwright_runtime", "checked_owned_data"):
            with self.subTest(failing=failing), mock.patch.object(audit, "in_case", side_effect=lambda path, root: path), \
                 mock.patch.object(closure, "checked_playwright_runtime"), \
                 mock.patch.object(closure, "checked_owned_data", return_value={"path": Path("/case/data")}), \
                 mock.patch.object(closure, failing, side_effect=ValueError("unowned")):
                with self.assertRaisesRegex(ValueError, "unowned"):
                    closure.checked_playwright_transport(audit, {"client": "pi", "scope": "project"}, Path("/case"), {"locator": Path("/case/native")})


if __name__ == "__main__":
    unittest.main()
