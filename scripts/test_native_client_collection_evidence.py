"""Exercise the persisted local-Skill RPC boundary without running a client."""
import copy
import json
import ntpath
import posixpath
import unittest
from unittest.mock import patch

import native_client_collection_evidence as audit
import audit_enterprise_native_evidence as acquire


def collection_fixture(target):
    path = ntpath if target.startswith("windows-") else posixpath
    root = r"C:\fixtures\uap-native-evidence-example" if path is ntpath else "/fixtures/uap-native-evidence-example"
    directory = path.basename(root)
    collection = path.join(root, "local-collection")
    required = {directory + "/" + label + suffix
                for label in ("local-collection-list", *audit.STAGES[:2],
                              "local-collection-remove-benchmark-note",
                              "local-collection-remove-benchmark-secondary")
                for suffix in ("-installer.json", "-installer-stderr.log")}
    files = {}
    for stage in audit.STAGES:
        label = "local-collection-removed" if stage == audit.STAGES[2] else stage
        for side in ("A", "B"):
            cwd = path.join(root, "project" if side == "A" else "project-B")
            skills = [{"name": name, "enabled": True, "scope": "repo", "pluginId": None,
                       "path": path.join(cwd, ".agents", "skills", name, "SKILL.md")}
                      for name in audit.SKILLS] if side == "A" and stage != audit.STAGES[2] else []
            records = [
                {"direction": "send", "message": {"id": 1, "method": "initialize",
                 "params": {"clientInfo": {"name": "uap_raw_project_check", "version": "1"},
                            "capabilities": {"experimentalApi": True}}}},
                {"direction": "receive", "message": {"id": 1, "result": {}}},
                {"direction": "send", "message": {"id": 2, "method": "skills/list",
                 "params": {"cwds": [cwd], "forceReload": True}}},
                {"direction": "receive", "message": {"id": 2, "result": {
                 "data": [{"cwd": cwd, "errors": [], "skills": skills}]}}},
            ]
            stem = directory + "/" + label + ("-project-B" if side == "B" else "")
            files[stem + "-rpc.jsonl"] = b"\n".join(json.dumps(row).encode() for row in records)
            files[stem + "-stderr.log"] = b""
    return files, directory, collection, required


def rewrite_response(files, member, change):
    records = [json.loads(line) for line in files[member].splitlines()]
    change(records[-1]["message"]["result"]["data"][0])
    files[member] = b"\n".join(json.dumps(row).encode() for row in records)


class CodexCollectionEvidenceTests(unittest.TestCase):
    def test_explicit_null_local_plugin_id_qualifies_install_repeat_remove(self):
        for target in ("linux-amd64", "windows-amd64", "darwin-arm64"):
            with self.subTest(target=target):
                files, directory, collection, required = collection_fixture(target)
                receipt = audit.codex_collection(files, directory, collection, target, required)
                self.assertEqual(receipt["protocol_sessions"], 6)
                self.assertEqual(len(required), 22)

    def test_changed_selected_skill_or_project_is_refused(self):
        def change_skill(key, value):
            return lambda row: row["skills"][0].__setitem__(key, value)

        changes = {
            "foreign-plugin": change_skill("pluginId", "foreign@marketplace"),
            "empty-plugin": change_skill("pluginId", ""),
            "boolean-plugin": change_skill("pluginId", False),
            "missing-plugin": lambda row: row["skills"][0].pop("pluginId"),
            "disabled": change_skill("enabled", False),
            "wrong-scope": change_skill("scope", "user"),
            "foreign-path": change_skill("path", "/foreign/SKILL.md"),
            "duplicate": lambda row: row["skills"].append(copy.deepcopy(row["skills"][0])),
            "missing-skill": lambda row: row["skills"].pop(),
            "discovery-error": lambda row: row["errors"].append("unreadable skill"),
            "foreign-cwd": lambda row: row.__setitem__("cwd", "/foreign/project"),
        }
        for target in ("linux-amd64", "windows-amd64"):
            for name, change in changes.items():
                with self.subTest(target=target, change=name):
                    files, directory, collection, required = collection_fixture(target)
                    member = directory + "/local_collection_all_install-rpc.jsonl"
                    rewrite_response(files, member, change)
                    with self.assertRaises(ValueError):
                        audit.codex_collection(files, directory, collection, target, required)

    def test_selected_skill_must_be_absent_after_removal_and_in_project_b(self):
        for label in ("local-collection-removed", "local_collection_all_install-project-B"):
            with self.subTest(label=label):
                files, directory, collection, required = collection_fixture("linux-amd64")
                installed = audit.rpc_inventory(files[directory + "/local_collection_all_install-rpc.jsonl"])[1]
                rewrite_response(files, directory + "/" + label + "-rpc.jsonl",
                                 lambda row: row["skills"].extend(copy.deepcopy(installed["skills"])))
                with self.assertRaises(ValueError):
                    audit.codex_collection(files, directory, collection, "linux-amd64", required)


class AcquisitionIdentityTests(unittest.TestCase):
    def test_only_original_source_run_is_admitted(self):
        run = {"id": acquire.RUN, "head_sha": audit.COMMIT, "path": acquire.WORKFLOW,
               "event": "pull_request", "run_attempt": 1,
               "repository": {"full_name": acquire.REPO},
               "head_repository": {"full_name": acquire.REPO}}
        acquire.validate_run(run)
        for key, value in (("head_sha", "0" * 40), ("id", acquire.RUN + 1),
                           ("event", "workflow_dispatch"), ("run_attempt", 2),
                           ("head_repository", {"full_name": "foreign/repo"})):
            with self.subTest(key=key), self.assertRaises(ValueError):
                acquire.validate_run(dict(run, **{key: value}))

    def test_foreign_or_unhashed_archives_are_refused_before_download(self):
        artifact = {"id": 123, "name": "native-client-codex-linux-amd64", "expired": False,
                    "size_in_bytes": 100, "digest": "sha256:" + "a" * 64,
                    "workflow_run": {"id": acquire.RUN, "head_sha": audit.COMMIT}}
        self.assertEqual(acquire.validate_artifact(artifact, "codex", "linux-amd64"), "a" * 64)
        for key, value in (("expired", True), ("digest", None), ("size_in_bytes", True),
                           ("workflow_run", {"id": acquire.RUN, "head_sha": "0" * 40}),
                           ("name", "native-client-codex-windows-amd64")):
            with self.subTest(key=key), self.assertRaises(ValueError):
                acquire.validate_artifact(dict(artifact, **{key: value}), "codex", "linux-amd64")

    def test_incomplete_job_never_reaches_archive_audit(self):
        job = {"id": 123, "run_id": acquire.RUN, "head_sha": audit.COMMIT, "run_attempt": 1,
               "name": "codex runtime (linux-amd64)", "status": "completed",
               "conclusion": "cancelled", "steps": []}
        with self.assertRaises(ValueError):
            acquire.validate_job(job, "codex", "linux-amd64")

    def test_wrong_source_is_refused_before_network_or_evidence_creation(self):
        with patch.object(acquire, "source_texts", return_value={
            "scripts/run-native-client-matrix.py": "foreign source", acquire.WORKFLOW: "foreign source"
        }), patch.object(acquire, "command") as command, patch.object(acquire.Path, "mkdir") as mkdir:
            with self.assertRaises(ValueError):
                acquire.collect(acquire.Path("source"), acquire.Path("evidence"))
            command.assert_not_called()
            mkdir.assert_not_called()


if __name__ == "__main__":
    unittest.main()
