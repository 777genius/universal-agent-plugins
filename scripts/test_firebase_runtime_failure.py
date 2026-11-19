import hashlib
import json
import os
from pathlib import Path
from runpy import run_path
import shutil
import tempfile
import unittest
from unittest.mock import patch


class FirebaseRuntimeFailureTest(unittest.TestCase):
    def test_owned_stage_facts_and_all_or_refusal(self):
        collect = run_path(str(Path(__file__).with_name("firebase_runtime_failure.py")))["collect"]
        for mutation in (None, "link", "foreign", "large", "unknown", "extra-log",
                         "missing-owner", "owner-state", "owner-digest", "owner-id",
                         "parent-link", "parent-replacement", "large-log", "package", "lock", "source-shape",
                         "source-staging", "stage-data", "stage-root", "stage-manifest",
                         "stage-duplicate", "stage-link", "stage-name", "stage-owner"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                case = Path(temporary).resolve() / "plugin-06"
                backend = case / "uap-data/plugin-data/firebase-123456789abc"
                stage = backend / ("npm-runtime/.tmp-" + "a" * 64 + "-123456")
                stage.mkdir(parents=True)
                receipt = "data_" + "a" * 24
                owner = {"locator": str(backend), "scope": "user", "physical_backend_id": backend.name,
                         "data_receipt_id": receipt, "state": "owned", "created_at": "2026-11-17T22:00:00Z",
                         "updated_at": "2026-11-17T22:00:00Z", "ownership_digest": "sha256:" + hashlib.sha256(
                             ("agentplugins-plugin-data-v1\0" + receipt + "\0" + str(backend)).encode()).hexdigest()}
                if mutation == "missing-owner":
                    del owner["data_receipt_id"]
                elif mutation in ("owner-state", "owner-digest", "owner-id"):
                    owner[{"owner-state": "state", "owner-digest": "ownership_digest",
                           "owner-id": "data_receipt_id"}[mutation]] = "invalid"
                (backend / ".agentplugins-data-owner.json").write_text(json.dumps(owner))
                package = b'{"dependencies":{"firebase-tools":"14.0.0"}}'
                (stage / "package.json").write_bytes(package)
                lock = stage / "package-lock.json"
                lock.write_bytes(b'{"lockfileVersion":3}')
                active = case / "home/.claude/skills/firebase-123456789abc"
                staged_source = mutation == "source-staging" or str(mutation).startswith("stage-")
                projection = (case / ("home/.claude/.agentplugins-staging-" + "b" * 16)
                              if staged_source else active)
                source = projection / ".agentplugins-runtime/io.github.777genius.agentplugins/runtime"
                source.mkdir(parents=True)
                (source / "runtime.json").write_text(json.dumps({
                    "schema_version": 1, "package": "firebase-tools", "version": "14.0.0",
                    "package_lock_sha256": "sha256:" + hashlib.sha256(lock.read_bytes()).hexdigest(),
                    "omit_optional": True, "entrypoint": "node_modules/firebase-tools/lib/bin/firebase.js"}))
                (source / "package.json").write_bytes(package)
                (source / "package-lock.json").write_bytes(lock.read_bytes())
                if staged_source:
                    (projection / ".claude-plugin").mkdir()
                    (projection / ".claude-plugin/plugin.json").write_text(json.dumps({"name": "firebase"}))
                    (projection / ".mcp.json").write_text(json.dumps({"firebase": {"env": {
                        "PLUGIN_ROOT": str(active / ".agentplugins-runtime"), "PLUGIN_DATA": str(backend)}}}))
                logs = backend / "npm-cache/_logs"
                logs.mkdir(parents=True)
                (logs / "2026-11-17T22_00_00_000Z-debug-0.log").write_bytes(b"PRIVATE https://token@secret.invalid")
                foreign = Path(temporary).resolve() / "foreign"
                foreign.write_bytes(b"foreign untouched")
                if mutation == "link":
                    lock.unlink()
                    try:
                        lock.symlink_to(foreign)
                    except OSError as error:
                        self.skipTest(str(error))
                elif mutation == "foreign":
                    (backend / ".agentplugins-data-owner.json").write_text(json.dumps({
                        "locator": str(foreign), "scope": "user", "physical_backend_id": "backend"}))
                elif mutation == "large":
                    lock.write_bytes(b"x" * ((1 << 20) + 1))
                elif mutation == "unknown":
                    (stage.parent / "unowned").mkdir()
                elif mutation == "extra-log":
                    (logs / "foreign-secret").write_bytes(b"private")
                elif mutation == "large-log":
                    (logs / "2026-11-17T22_00_00_000Z-debug-0.log").write_bytes(b"x" * ((1 << 20) + 1))
                elif mutation == "package":
                    (stage / "package.json").write_bytes(b'{"dependencies":{"unrelated":"1.0.0"}}')
                elif mutation == "lock":
                    lock.write_bytes(b"different")
                elif mutation == "source-shape":
                    (source / "package.json").write_bytes(b"[]")
                elif mutation in ("stage-data", "stage-root"):
                    mcp = projection / ".mcp.json"
                    doc = json.loads(mcp.read_text())
                    key = "PLUGIN_DATA" if mutation == "stage-data" else "PLUGIN_ROOT"
                    doc["firebase"]["env"][key] = str(foreign)
                    mcp.write_text(json.dumps(doc))
                elif mutation == "stage-manifest":
                    (projection / ".claude-plugin/plugin.json").write_text('{"name":"foreign"}')
                elif mutation == "stage-owner":
                    (backend / ".agentplugins-data-owner.json").write_text('{}')
                elif mutation == "stage-duplicate":
                    shutil.copytree(projection, active)
                elif mutation == "stage-name":
                    projection.rename(projection.with_name(".agentplugins-staging-foreign"))
                elif mutation == "stage-link":
                    old = projection.with_name("held-original-projection")
                    projection.rename(old)
                    try:
                        projection.symlink_to(old, target_is_directory=True)
                    except OSError as error:
                        self.skipTest(str(error))
                elif mutation == "parent-link":
                    old = stage.with_name(stage.name + "-old")
                    stage.rename(old)
                    try:
                        stage.symlink_to(old, target_is_directory=True)
                    except OSError as error:
                        self.skipTest(str(error))
                files = collect.__globals__["_FILES"]
                real_open = files["open_member"]
                def replacing_open(path):
                    fd = real_open(path)
                    if mutation == "parent-replacement" and path == lock:
                        stage.rename(stage.with_name(stage.name + "-old"))
                        stage.mkdir()
                    return fd
                with patch.dict(files, {"open_member": replacing_open}):
                    result = collect(case)
                self.assertIs(result["qualified"], False)
                self.assertNotIn("PRIVATE", json.dumps(result))
                self.assertNotIn("secret.invalid", json.dumps(result))
                self.assertNotIn(str(case), json.dumps(result))
                self.assertEqual(foreign.read_bytes(), b"foreign untouched")
                if mutation in (None, "large-log", "source-staging"):
                    self.assertEqual(result["status"], "observed-unqualified")
                    self.assertEqual(result["facts"][0]["staging"][0]["package_sha256"],
                                     hashlib.sha256(package).hexdigest())
                    self.assertEqual(result["facts"][0]["staging"][0]["dependencies"],
                                     {"firebase-tools": "14.0.0"})
                    self.assertEqual(result["facts"][0]["npm_logs"]["count"], 1)
                    if mutation == "large-log":
                        self.assertEqual(result["facts"][0]["npm_logs"]["latest"], {
                            "present": True, "bytes": (1 << 20) + 1, "uninspected_large": True})
                    self.assertFalse(result["facts"][0]["staging"][0]["entrypoint"]["present"])
                    self.assertEqual(result["source"][0]["package"]["sha256"], hashlib.sha256(package).hexdigest())
                    self.assertEqual(result["source"][0]["location"], "staging" if staged_source else "active")
                else:
                    self.assertEqual(result["status"], "refused-unqualified")
                    self.assertEqual(result["facts"], [])
                    self.assertIn(result["refusal_phase"], {
                        "case-root", "native-projection", "staging-binding",
                        "source-envelope", "owned-data", "locked-npm-runtime",
                        "npm-cache", "held-custody"})
                    self.assertIn(result["refusal_class"], {
                        "missing", "deadline", "invalid-state"})
                    if mutation in ("stage-data", "stage-root", "stage-manifest"):
                        self.assertEqual(result["refusal_phase"], "staging-binding")
                        self.assertEqual(result["refusal_detail"], {
                            "stage-data": "staging-data-parent",
                            "stage-root": "staging-root",
                            "stage-manifest": "staging-manifest",
                        }[mutation])
                    elif result["refusal_phase"] == "staging-binding":
                        self.assertEqual(result["refusal_detail"], "unclassified")
                    if mutation in ("owner-state", "owner-digest", "owner-id"):
                        self.assertEqual(result["refusal_phase"], "owned-data")
                    elif mutation == "extra-log":
                        self.assertEqual(result["refusal_phase"], "npm-cache")


if __name__ == "__main__":
    unittest.main()
