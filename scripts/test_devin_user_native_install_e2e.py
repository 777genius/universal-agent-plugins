"""Inert boundary contracts for the separate pinned Devin User cohort."""
import copy
import hashlib
import json
import os
import sys
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import devin_user_native_install_e2e as subject


class DevinUserNativeContracts(unittest.TestCase):
    def test_skill_requires_exact_global_metadata_not_name_or_project_copy(self):
        with tempfile.TemporaryDirectory() as temporary:
            selected = Path(temporary) / "config/devin/skills" / subject.NAME
            row = {"name": subject.NAME, "description": subject.DESCRIPTION, "provider": "Devin",
                   "base_dir": str(selected), "display_name": subject.NAME,
                   "triggers": ["user", "model"], "warnings": [], "errors": []}
            self.assertTrue(subject.skill_present(json.dumps([row]), selected))
            self.assertFalse(subject.skill_present("[]", selected))
            for changed in [dict(row, base_dir=str(Path(temporary) / ".devin/skills" / subject.NAME)),
                            dict(row, provider="Cascade"), dict(row, warnings=["unavailable"]),
                            {"name": subject.NAME}]:
                with self.subTest(changed=changed), self.assertRaises(RuntimeError):
                    subject.skill_present(json.dumps([changed]), selected)
            for body in [json.dumps([row, row]), "{}", "[null]"]:
                with self.subTest(body=body), self.assertRaises(RuntimeError):
                    subject.skill_present(body, selected)

    def test_repeat_requires_singleton_nonmutating_same_installation_shape(self):
        iid = "00000000-0000-4000-8000-000000000123"
        outcome = {"installation_id": iid}
        first = {"targets": [{"target": "windsurf", "status": "external_completed",
                              "output": {"result": outcome}}]}
        self.assertEqual(subject.add_identity(first), iid)
        repeated = {"dry_run": False, "result": {**outcome, "no_change": True,
                    "mutated": False, "requires_confirmation": False}}
        self.assertEqual(subject.add_identity(repeated, repeated=True), iid)
        for key, value in [("mutated", True), ("no_change", False), ("requires_confirmation", True)]:
            changed = copy.deepcopy(repeated)
            changed["result"][key] = value
            with self.subTest(key=key), self.assertRaises(RuntimeError):
                subject.add_identity(changed, repeated=True)
        for data in [first, dict(repeated, operation_id="unexpected"), dict(repeated, targets=[])]:
            with self.subTest(data=data), self.assertRaises(RuntimeError):
                subject.add_identity(data, repeated=True)
        ambiguous = copy.deepcopy(first)
        ambiguous["targets"] *= 2
        with self.assertRaises(RuntimeError):
            subject.add_identity(ambiguous)

    @staticmethod
    def binding_fixture(root):
        native = root / "config/devin"
        managed = root / "state/managed/clients/windsurf"
        active = managed / "owned-artifact"
        active.mkdir(parents=True)
        native.mkdir(parents=True)
        data = root / "state/plugin-data/owned-artifact"
        data.mkdir(parents=True)
        iid = "00000000-0000-4000-8000-000000000123"
        objects = [
            {"object_id": "package:windsurf:" + active.name, "kind": "managed_package_directory",
             "logical_name": subject.NAME, "path": str(active)},
            {"object_id": "devin_global_skill_directory:" + subject.NAME, "kind": "devin_global_skill_directory",
             "logical_name": subject.NAME, "path": str(native / "skills" / subject.NAME)},
            {"object_id": "devin_global_mcp_server:" + subject.STDIO, "kind": "devin_global_mcp_server",
             "logical_name": subject.STDIO, "path": str(native / "mcp_config.json")},
            {"object_id": "devin_global_mcp_server:" + subject.HTTP, "kind": "devin_global_mcp_server",
             "logical_name": subject.HTTP, "path": str(native / "mcp_config.json")}]
        for row in objects:
            row.update(protection_class="managed", managed_digest="sha256:" + "a" * 64)
        binding = {"client_binding_id": "binding", "client_id": "windsurf", "scope": "user",
                   "native_profile_root": str(native), "target_locator": str(active),
                   "physical_artifact_id": active.name, "native_objects": objects, "data_receipt_id": "data"}
        state = {"schema_version": 5, "installations": [
            {"installation_id": iid, "clients": {"binding": binding},
             "data_receipts": {"data": {"data_receipt_id": "data", "physical_backend_id": active.name,
                "scope": "user", "state": "owned", "locator": str(data),
                "ownership_digest": "sha256:" + "b" * 64}}}]}
        return state, iid, native, managed

    def test_selected_user_state_refuses_project_cascade_alias_and_wrong_ownership(self):
        with tempfile.TemporaryDirectory() as temporary:
            state, iid, native, managed = self.binding_fixture(Path(temporary).resolve())
            actual = subject.selected_binding(state, iid, native, managed)
            self.assertEqual(actual["scope"], "user")
            for field, value in [("scope", "project"), ("native_profile_root", str(native.parent / ".codeium")),
                                 ("target_locator", str(managed.parent)), ("physical_artifact_id", "other")]:
                changed = copy.deepcopy(state)
                changed["installations"][0]["clients"]["binding"][field] = value
                with self.subTest(field=field), self.assertRaises(RuntimeError):
                    subject.selected_binding(changed, iid, native, managed)
            for mutation in ["kind", "path", "id"]:
                changed = copy.deepcopy(state)
                objects = changed["installations"][0]["clients"]["binding"]["native_objects"]
                if mutation == "kind":
                    objects[1]["kind"] = "devin_project_skill_directory"
                elif mutation == "path":
                    objects[1]["path"] = str(native.parent / ".devin/skills" / subject.NAME)
                else:
                    objects[2]["object_id"] = objects[1]["object_id"]
                with self.subTest(mutation=mutation), self.assertRaises(RuntimeError):
                    subject.selected_binding(changed, iid, native, managed)
            duplicate = copy.deepcopy(state)
            duplicate["installations"] *= 2
            with self.assertRaises(RuntimeError):
                subject.selected_binding(duplicate, iid, native, managed)
            with self.assertRaises(RuntimeError):
                subject.selected_binding(state, iid, native, managed, required=False)
            state["installations"][0]["clients"] = {}
            self.assertIsNone(subject.selected_binding(state, iid, native, managed, required=False))

    def test_snapshot_observes_real_content_identity_and_read_does_not_self_invalidate(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            payload = root / "payload"
            payload.mkdir()
            file = payload / "SKILL.md"
            file.write_bytes(b"original")
            first = subject.snapshot([payload], root)
            self.assertEqual(subject.snapshot([payload], root), first)
            self.assertEqual(first[str(file)][-1], hashlib.sha256(b"original").hexdigest())
            file.write_bytes(b"changed")
            self.assertNotEqual(subject.snapshot([payload], root), first)
            file.unlink()
            file.mkdir()
            self.assertNotEqual(subject.snapshot([payload], root), first)

    def test_snapshot_refuses_links_aliases_special_objects_and_actual_bounds(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            file = root / "file"
            file.write_bytes(b"12345")
            link = root / "link"
            link.symlink_to(file)
            with self.assertRaises(RuntimeError):
                subject.snapshot([link], root)
            alias = root / "alias"
            os.link(file, alias)
            with self.assertRaises(RuntimeError):
                subject.snapshot([file], root)
            alias.unlink()
            with patch.object(subject, "MAX_FILE", 4), self.assertRaises(RuntimeError):
                subject.snapshot([file], root)
            with patch.object(subject, "MAX_TOTAL", 4), self.assertRaises(RuntimeError):
                subject.snapshot([file], root)
            limited = root / "limited"
            limited.mkdir()
            (limited / "child").write_bytes(b"bounded")
            with patch.object(subject, "MAX_ENTRIES", 1), self.assertRaises(RuntimeError):
                subject.snapshot([limited], root)
            fifo = root / "fifo"
            os.mkfifo(fifo)
            with self.assertRaises(RuntimeError):
                subject.snapshot([fifo], root)

    def test_native_executable_hash_refuses_aliases_and_wrong_file_type(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            binary = root / "devin"
            binary.write_bytes(b"owned executable")
            binary.chmod(0o700)
            self.assertEqual(subject.file_digest(binary), hashlib.sha256(binary.read_bytes()).hexdigest())
            before = subject.executable_fact(binary)
            saved = root / "original"
            binary.rename(saved)
            binary.write_bytes(b"owned executable")
            binary.chmod(0o700)
            self.assertEqual(subject.executable_fact(binary)["sha256"], before["sha256"])
            self.assertNotEqual(subject.executable_fact(binary), before)
            alias = root / "alias"
            os.link(binary, alias)
            with self.assertRaises(RuntimeError):
                subject.file_digest(binary)
            alias.unlink()
            alias.symlink_to(binary)
            with self.assertRaises(RuntimeError):
                subject.file_digest(alias)
            with self.assertRaises(RuntimeError):
                subject.file_digest(root)

    def test_cascade_absence_includes_dangling_links_and_empty_directories(self):
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary).resolve()
            subject.no_cascade(home)
            cascade = home / ".codeium"
            cascade.symlink_to(home / "absent")
            with self.assertRaises(RuntimeError):
                subject.no_cascade(home)
            cascade.unlink()
            cascade.mkdir()
            with self.assertRaises(RuntimeError):
                subject.no_cascade(home)


    def test_data_receipt_is_exact_owned_user_authority(self):
        with tempfile.TemporaryDirectory() as temporary:
            state, iid, native, managed = self.binding_fixture(Path(temporary).resolve())
            for key, value in [("scope", "project"), ("state", "retained"), ("physical_backend_id", "other"),
                               ("data_receipt_id", "other"), ("locator", str(native)), ("ownership_digest", "unknown")]:
                changed = copy.deepcopy(state)
                changed["installations"][0]["data_receipts"]["data"][key] = value
                with self.subTest(key=key), self.assertRaises(RuntimeError):
                    subject.selected_binding(changed, iid, native, managed)

    def test_snapshot_large_admission_is_only_exact_generated_executable(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            launcher = root / "launcher"
            launcher.write_bytes(b"owned launcher")
            launcher.chmod(0o700)
            digest = hashlib.sha256(launcher.read_bytes()).hexdigest()
            with patch.object(subject, "MAX_FILE", 4):
                with self.assertRaises(RuntimeError):
                    subject.snapshot([launcher], root)
                self.assertEqual(subject.snapshot([launcher], root, (launcher, digest))[str(launcher)][-1], digest)
                with self.assertRaises(RuntimeError):
                    subject.snapshot([launcher], root, (launcher, "0" * 64))
                other = root / "unexpected"
                other.write_bytes(b"unexpected large file")
                with self.assertRaises(RuntimeError):
                    subject.snapshot([other], root, (launcher, digest))

    def test_executable_swapped_to_fifo_is_opened_nonblocking_and_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            binary = root / "binary"
            binary.write_bytes(b"owned executable")
            binary.chmod(0o700)
            real_open = os.open
            def swapped(path, flags):
                self.assertTrue(flags & os.O_NONBLOCK)
                binary.unlink()
                os.mkfifo(binary)
                return real_open(path, flags)
            with patch.object(subject.os, "open", side_effect=swapped), self.assertRaises(RuntimeError):
                subject.file_digest(binary)

    def test_generated_projection_has_exact_wrapper_roots_inventory_metadata_and_source_launcher(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            state, iid, native, managed_root = self.binding_fixture(root)
            managed = managed_root / "owned-artifact"
            data = root / "state/plugin-data/owned-artifact"
            package = root / "package"
            service = root / "service.py"
            service.write_text("inert fixture")
            source_skill = package / "skills" / subject.NAME / "SKILL.md"
            source_skill.parent.mkdir(parents=True)
            source_skill.write_bytes(b"owned Skill bytes")
            (package / "plugin.json").write_text('{"name":"' + subject.NAME + '","version":"1.0.0"}')
            (package / "mcp.json").write_text('{"mcpServers":{}}')
            skill = managed / "skills" / subject.NAME / "SKILL.md"
            skill.parent.mkdir(parents=True)
            skill.write_bytes(source_skill.read_bytes())
            (managed / "plugin.json").write_bytes((package / "plugin.json").read_bytes())
            launcher = managed / subject.LAUNCHER
            launcher.parent.mkdir(parents=True)
            launcher.write_bytes(b"trusted source CLI bytes")
            launcher.chmod(0o700)
            digest = hashlib.sha256(launcher.read_bytes()).hexdigest()
            metadata = {"protocol": 1, "sha256": digest, "cliVersion": "0.1.0-development"}
            metadata_path = launcher.parent / "metadata.json"
            metadata_path.write_text(json.dumps(metadata))
            argv = ["--internal-stdio-v1", str(managed), str(data), str(managed), "plugin", "--",
                    sys.executable, str(service)]
            stdio = {"command": str(launcher), "args": argv,
                     "env": {"PLUGIN_ROOT": str(managed), "PLUGIN_DATA": str(data)}}
            projected = {"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
                         "mcpServers": {subject.STDIO: dict(stdio, type="stdio"),
                                        subject.HTTP: {"type": "streamable-http", "url": "http://127.0.0.1:12345/mcp"}}}
            projection_path = managed / "mcp.json"
            projection_path.write_text(json.dumps(projected))
            native_path = native / "mcp_config.json"
            native_path.write_text(json.dumps({"mcpServers": {subject.STDIO: stdio,
                subject.HTTP: {"url": "http://127.0.0.1:12345/mcp", "transport": "http"}}}))
            def verify():
                return subject.verify_projection(managed, package, native, data, service, 12345, digest, root)
            self.assertEqual(len(verify()), 5)
            metadata_path.write_text(json.dumps(dict(metadata, sha256="0" * 64)))
            with self.assertRaises(RuntimeError):
                verify()
            metadata_path.write_text(json.dumps(metadata))
            wrong = copy.deepcopy(projected)
            wrong["mcpServers"][subject.STDIO]["args"][3] = str(native)
            projection_path.write_text(json.dumps(wrong))
            with self.assertRaises(RuntimeError):
                verify()
            projection_path.write_text(json.dumps(projected))
            unexpected = managed / "unexpected"
            unexpected.write_bytes(b"unclassified generated bytes")
            with self.assertRaises(RuntimeError):
                verify()
            unexpected.unlink()
            launcher.write_bytes(b"changed source CLI")
            with self.assertRaises(RuntimeError):
                verify()

    def test_project_success_and_cleanup_failure_cannot_qualify_user(self):
        self.assertFalse(subject.qualified({"installation_discovery_e2e": True,
                         "raw_project_skills_native_ready": True, "cleanup_complete": True}))
        self.assertFalse(subject.qualified({"user_installation_discovery_e2e": True,
                                           "cleanup_complete": False}))
        self.assertTrue(subject.qualified({"user_installation_discovery_e2e": True,
                                          "cleanup_complete": True}))

    def test_existing_workflow_retains_both_independent_cohorts_and_future_path_triggers(self):
        workflow = (Path(__file__).resolve().parents[1] / ".github/workflows/devin-project-auto-install.yml").read_text()
        for path in ["scripts/devin_user_native_install_e2e.py", "scripts/test_devin_user_native_install_e2e.py"]:
            self.assertIn("- '" + path + "'", workflow)
        self.assertIn("python3 scripts/devin_project_auto_install_e2e.py", workflow)
        self.assertIn("python3 scripts/devin_user_native_install_e2e.py", workflow)
        self.assertIn("test_devin_project_auto_install_e2e.py", workflow)
        self.assertIn("test_devin_user_native_install_e2e.py", workflow)
        self.assertIn("devin-user-v57-e2e.json", workflow)
        self.assertIn("devin-project-v57-e2e.json", workflow)
        self.assertIn("ref: \u0024{{ github.event.pull_request.head.sha }}", workflow)


if __name__ == "__main__":
    unittest.main()
