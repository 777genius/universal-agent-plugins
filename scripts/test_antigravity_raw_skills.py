"""Inert assertions for the existing native fixture; no client or Git execution."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


SPEC = importlib.util.spec_from_file_location(
    "antigravity_native_cross", Path(__file__).parent / "e2e/antigravity-native-cross.py")
NATIVE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(NATIVE)


class AntigravityRawSkillsAssertions(unittest.TestCase):
    def test_native_inventory_keeps_original_mixed_and_selected_raw_components(self):
        path = Path("/owned/plugins/physical")
        row = {"name": path.name, "path": str(path), "version": "1.0.0", "enabled": True,
               "skills": ["native-smoke"], "mcpServers": ["native-http", "native-stdio"]}
        manifest = {"version": "1.0.0"}
        with patch.object(NATIVE, "run", side_effect=lambda *args, **kwargs: json.dumps([row])):
            NATIVE.checked_plugin("never-executed", {}, path.parent, path, manifest)
            row["skills"] = ["frontend-design"]
            row.pop("mcpServers")
            NATIVE.checked_plugin("never-executed", {}, path.parent, path, manifest,
                                  skills=("frontend-design",), servers=())
            row.pop("version")
            NATIVE.checked_plugin("never-executed", {}, path.parent, path, {"name": path.name},
                                  skills=("frontend-design",), servers=())
            row["version"] = "2.0.0"
            with self.assertRaises(AssertionError):
                NATIVE.checked_plugin("never-executed", {}, path.parent, path, manifest,
                                      skills=("frontend-design",), servers=())
            row["version"] = "1.0.0"
            with self.assertRaises(AssertionError):
                NATIVE.checked_plugin("never-executed", {}, path.parent, path, manifest)
            row["mcpServers"] = ["unselected"]
            with self.assertRaises(AssertionError):
                NATIVE.checked_plugin("never-executed", {}, path.parent, path, manifest,
                                      skills=("frontend-design",), servers=())

    def test_prepared_raw_success_does_not_become_ready_or_change_target(self):
        for scope in ("user", "project"):
            for repeated in (False, True):
                result = {"schema_version": 1, "command": "skills add", "result": "success",
                          "status": "succeeded", "scope": scope, "readiness": "needs_action",
                          "next_actions": [{"kind": "verify_native", "target": "antigravity"}],
                          "items": [{"alias": "frontend-design", "targetId": "antigravity",
                                     "status": "unchanged" if repeated else "succeeded"}]}
                with patch.object(NATIVE.subprocess, "run", side_effect=AssertionError("native process")):
                    NATIVE.checked_raw_skill_result(json.dumps(result), scope, repeated=repeated)
                    for field, value in (("readiness", "ready"), ("scope", "other"),
                                         ("next_actions", []), ("schema_version", True)):
                        wrong = copy.deepcopy(result)
                        wrong[field] = value
                        with self.subTest(scope=scope, repeated=repeated, field=field), self.assertRaises(AssertionError):
                            NATIVE.checked_raw_skill_result(json.dumps(wrong), scope, repeated=repeated)
                    wrong = copy.deepcopy(result)
                    wrong["items"][0]["targetId"] = "foreign"
                    with self.assertRaises(AssertionError):
                        NATIVE.checked_raw_skill_result(json.dumps(wrong), scope, repeated=repeated)

    def test_native_bundle_requires_receipt_and_exact_upstream_bytes(self):
        for scope in ("user", "project"):
            with self.subTest(scope=scope), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                state_root = root / ".uap" if scope == "project" else root / "state"
                state_root.mkdir()
                native_root = root / "native"
                bundle = native_root / "plugins/skill-physical"
                skill = bundle / "skills/frontend-design/SKILL.md"
                skill.parent.mkdir(parents=True)
                body = b"---\nname: frontend-design\ndescription: inert\n---\n"
                skill.write_bytes(body)
                item = {"name": "frontend-design", "repo": "anthropics/skills", "ref": "a" * 40,
                        "folder": "frontend-design", "skill_md_git_blob_sha1": hashlib.sha1(
                            b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()}
                binding = {"client_id": "antigravity", "scope": scope,
                           "physical_artifact_id": bundle.name, "target_locator": str(bundle),
                           "materialization": "materialized", "activation": "prepared",
                           "verification": "installation_verified", "policy": "allowed",
                           "native_objects": [{"kind": "managed_package_directory", "path": str(bundle),
                                               "managed_digest": "sha256:" + "b" * 64,
                                               "protection_class": "managed"}]}
                state = {"installations": [{"source": {"resolved_revision": item["ref"]},
                                            "clients": {"selected": binding}}]}
                state_file = state_root / "state-v2.json"
                state_file.write_text(json.dumps(state), encoding="utf-8")
                lock = {"packages": [{"alias": item["name"], "targets": ["antigravity"],
                                      "source": {"git": {"kind": "skill", "skill": item["name"],
                                                         "repository": item["repo"], "commit": item["ref"],
                                                         "resolved_skill_path": "skills/frontend-design"}}}]}
                intent_root = state_root.parent if scope == "project" else state_root
                lock_file = intent_root / "uap.lock"
                lock_file.write_text(json.dumps(lock), encoding="utf-8")
                NATIVE.raw_skill_binding(state_root, native_root, scope, item)
                binding["native_objects"][0]["path"] = str(native_root / "plugins/foreign")
                state_file.write_text(json.dumps(state), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    NATIVE.raw_skill_binding(state_root, native_root, scope, item)
                binding["native_objects"][0]["path"] = str(bundle)
                state_file.write_text(json.dumps(state), encoding="utf-8")
                skill.write_bytes(body + b"foreign replacement\n")
                with self.assertRaises(AssertionError):
                    NATIVE.raw_skill_binding(state_root, native_root, scope, item)
                skill.write_bytes(body)
                lock["packages"][0]["source"]["git"]["commit"] = "c" * 40
                lock_file.write_text(json.dumps(lock), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    NATIVE.raw_skill_binding(state_root, native_root, scope, item)


if __name__ == "__main__":
    unittest.main()
