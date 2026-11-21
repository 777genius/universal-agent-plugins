"""Real-filesystem Qoder preparation proof; public/native commands are inert seams."""
import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import sys
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch

from test_live_install_audit_env import load_audit


@unittest.skipUnless(sys.platform == "linux", "Qoder Prepared lane is Linux only")
class QoderPreparedSkillContracts(unittest.TestCase):
    def exercise(self, fault=None, *, enabled=True):
        audit = load_audit()
        body = b"---\nname: frontend-design\ndescription: Original fixture\n---\nOriginal bytes\n"
        item = {"repo": "anthropics/skills", "ref": "a" * 40, "folder": "frontend-design", "name": "frontend-design",
                "source": "https://github.com/anthropics/skills/tree/" + "a" * 40 + "/skills/frontend-design",
                "skill_md_git_blob_sha1": hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()}
        iid = "11111111-1111-4111-8111-111111111111"
        calls, owned = [], {}
        self.calls, self.owned = calls, owned
        def response(document):
            return {"exit_code": 0, "timeout": False, "stdout_tail": json.dumps(document), "stderr_tail": "", "_json": copy.deepcopy(document)}
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            self.last_root = root
            def install(project):
                source_key = "github-skill:anthropics/skills:frontend-design:skills/frontend-design"
                source_hash = hashlib.sha256(source_key.encode()).hexdigest()
                declared = "skill-" + source_hash[:56]
                physical = declared[:51] + "-" + hashlib.sha256(iid.encode()).hexdigest()[:12]
                if fault == "coherent-physical": physical = declared[:51] + "-" + "b" * 12
                active = project / ".uap/managed/clients/qoder" / physical
                source = active / "skills/frontend-design"
                source.mkdir(parents=True)
                (source / "SKILL.md").write_bytes(body)
                (source / "binary.bin").write_bytes(b"\xff\x00\x81original")
                (source / "a").mkdir(); (source / "a/nested.txt").write_bytes(b"nested")
                (source / "a.txt").write_bytes(b"sibling")
                native = project / ".qoder/skills/frontend-design"
                shutil.copytree(source, native)
                # Independent literal Go WalkDir framing, including directory a
                # before sibling a.txt. This is not the production proof function.
                digest = "sha256:" + hashlib.sha256(b"f\0SKILL.md\0-\0" + body + b"\0"
                    + b"d\0a\0f\0a/nested.txt\0-\0nested\0f\0a.txt\0-\0sibling\0"
                    + b"f\0binary.bin\0-\0\xff\x00\x81original\0").hexdigest()
                view = active / ".agentplugins-qoder-native/skills/frontend-design"
                shutil.copytree(source, view)
                (active / "plugin.json").write_text(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": declared}))
                tree, manifest, raw, managed = ("sha256:" + value * 64 for value in "cdef")
                revision = {"resolved_revision": item["ref"], "tree_digest": tree, "manifest_digest": manifest,
                    "skill": {"name": item["name"], "requested_path": "skills/frontend-design", "resolved_path": "skills/frontend-design", "raw_tree_digest": raw, "normalizer": "agent-skill-package/v1"}}
                public_client = {"client_id": "qoder", "scope": "project", "materialization": "materialized", "activation": "prepared",
                    "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "package_revision": {key: revision[key] for key in ("resolved_revision", "tree_digest", "manifest_digest")}, "affected_surfaces": ["qoder"]}
                bid = "client_" + hashlib.sha256("\0".join((iid, "qoder", "project", str(active))).encode()).hexdigest()[:24]
                if fault == "coherent-binding": bid = "client_" + "b" * 24
                binding = {**public_client, "package_revision": revision, "client_binding_id": bid, "physical_artifact_id": physical, "target_locator": str(active),
                    "native_profile_root": str(active.parent), "native_objects": [
                        {"object_id": "package:qoder:" + physical, "kind": "managed_package_directory", "logical_name": declared, "path": str(active), "managed_digest": managed, "protection_class": "managed"},
                        {"object_id": "qoder_project_skill_directory:frontend-design", "kind": "qoder_project_skill_directory", "logical_name": "frontend-design", "path": str(native), "source_relative": "skills/frontend-design", "managed_digest": digest, "protection_class": "managed"}],
                    "receipts": [{"phase": "committed", "operation_id": "operation-original", "sequence": 1, "mutation_type": "directory_swap", "client_binding_id": bid, "active_path": str(active), "after_digest": managed,
                        "environment_authority": {"environment_id": "project-default", "client_id": "qoder", "expected_action": "install", "alias": item["name"], "source_key": source_key}}]}
                source_identity = {"stable_source_key": source_key, "source_binding_id": "src_" + source_hash[:24], "repository": item["repo"], "package_subpath": "skills/frontend-design", "resolved_revision": item["ref"], "tree_digest": tree}
                record = {"installation_id": iid, "declared_name": declared, "origin_mode": "direct", "source": source_identity,
                    "package": {"declared_name": declared, "format_id": "agent-plugins/1.0.0", "manifest_digest": manifest, "inventory": {"skills": [item["name"]], "mcp_present": False}}, "clients": {bid: binding}}
                state = {"schema_version": 5, "installations": [record], "environment_receipts": [{"receipt_id": "environment-original", "phase": "committed", "environment_id": "project-default", "alias": item["name"], "client_id": "qoder", "installation_id": iid, "client_binding_id": bid, "source_key": source_key, "package_digest": tree, "desired_action": "install"}]}
                lock = {"packages": [{"alias": item["name"], "targets": ["qoder"], "tree_digest": tree, "manifest_digest": manifest, "source": {"git": {"kind": "skill", "skill": item["name"], "repository": item["repo"], "commit": item["ref"], "resolved_skill_path": "skills/frontend-design", "raw_tree_digest": raw}}}]}
                public = {"installation_id": iid, "name": declared, "source": "anthropics/skills/skills/frontend-design", "clients": [public_client]}
                owned.update(active=active, native=native, state=state, public=public, binding=binding, removed=False, project=project)
                if fault == "missing-native": shutil.rmtree(native)
                elif fault == "wrong-receipt": binding["native_objects"][1]["path"] = str(project / ".qoder/skills/foreign-neighbor")
                elif fault == "wrong-source-relative": binding["native_objects"][1]["source_relative"] = ".agentplugins-qoder-native/skills/frontend-design"
                elif fault == "wrong-scope": binding["scope"] = public_client["scope"] = "user"
                elif fault == "wrong-tuple": binding["authentication"] = public_client["authentication"] = "not_required"
                elif fault == "wrong-public-revision": public_client["package_revision"]["tree_digest"] = "sha256:" + "b" * 64
                elif fault and fault.startswith("private-fact-"): revision["skill"][fault[len("private-fact-"):]] = "foreign"
                elif fault == "missing-private-skill": revision.pop("skill")
                elif fault == "wrong-private-skill": revision["skill"]["normalizer"] = "foreign-normalizer"
                elif fault == "wrong-pin": source_identity["resolved_revision"] = "b" * 40
                elif fault == "wrong-digest": binding["native_objects"][1]["managed_digest"] = "sha256:" + "b" * 64
                elif fault == "symlink": (native / "binary.bin").unlink(); (native / "binary.bin").symlink_to(source / "binary.bin")
                elif fault == "hardlink": (native / "binary.bin").unlink(); os.link(source / "binary.bin", native / "binary.bin")
                (project / ".uap/state-v2.json").write_text(json.dumps(state))
                (project / "uap.lock").write_text(json.dumps(lock)); (project / "uap.toml").write_text("fixture agreement\n")
            def call(argv, env, project, *_args, **_kwargs):
                if argv[1:3] == ["skills", "add"]:
                    calls.append("repeat" if owned else "add")
                    if not owned: install(project)
                    else:
                        if fault == "repeat-native": (owned["native"] / "binary.bin").write_bytes(b"mutated")
                        elif fault == "repeat-extra": (project / ".qoder/extra-user-residue").write_text("foreign")
                        elif fault == "repeat-receipt":
                            owned["binding"]["receipts"][0]["operation_id"] = "reminted"
                            (project / ".uap/state-v2.json").write_text(json.dumps(owned["state"]))
                        elif fault == "repeat-ancestor":
                            parent = owned["active"].parent; saved = parent.with_name("saved")
                            parent.rename(saved); parent.mkdir(); (saved / owned["active"].name).rename(owned["active"])
                    return response({"schema_version": 1, "command": "skills add", "result": "success", "status": "succeeded", "scope": "project", "scope_selection_reason": "explicit", "readiness": "needs_action", "next_actions": [{"code": "verify_native", "actor": "native_client", "message": "Start a fresh selected client session and verify that the installed Skill is discovered."}], "repairInstructions": [], "items": [{"alias": item["name"], "targetId": "qoder", "destinationPath": str(owned["active"]), "status": "succeeded" if calls.count("repeat") == 0 else "unchanged"}]})
                if argv[1] == "list":
                    calls.append("post-list" if owned.get("removed") else "list")
                    return response({"schema_version": 1, "command": "list", "result": "success", "data": {"installations": [] if owned.get("removed") else [owned["public"]]}})
                if argv[1] == "remove":
                    calls.append("remove")
                    shutil.rmtree(owned["active"])
                    if fault != "remove-native": shutil.rmtree(owned["native"])
                    if fault == "remove-extra": (project / ".qoder/extra-user-residue").write_text("new residue")
                    if fault != "remove-receipt":
                        owned["state"]["installations"][0]["clients"] = {}; owned["state"]["environment_receipts"] = []
                    (project / ".uap/state-v2.json").write_text(json.dumps(owned["state"]))
                    (project / "uap.lock").write_text('{"packages": []}')
                    owned["removed"] = True
                    return response({"schema_version": 1, "command": "agentplugins remove", "result": "success", "outcome": "completed", "scope": "project", "scope_selection_reason": "explicit", "actions": [{"from_revision": item["ref"], "from_digest": "sha256:" + "c" * 64, "alias": item["name"], "client_id": "qoder", "installation_id": iid, "binding_id": owned["binding"]["client_binding_id"], "action": "remove"}], "results": [{"installation_id": iid, "client_id": "qoder", "operation": "remove", "outcome": "completed"}]})
                raise AssertionError(argv)
            with patch.object(audit, "call", side_effect=call):
                row = audit.do_case("/inert/agentplugins", "qoder", "skill", item, 0, root, {},
                                     qoder_prepared=enabled)
            self.audit, self.item = audit, item
            return row

    def test_real_projection_lifecycle_is_prepared_only(self):
        row = self.exercise()
        self.assertEqual(row["status"], "prepared-lifecycle-passed")
        self.assertEqual(row["public_skill_ready"], {"after-add": False, "repeat-add": False})
        self.assertEqual(row["skill_path"], ".qoder/skills/frontend-design/SKILL.md")
        self.assertEqual(set(row["qoder_prepared_skill_proof"]["checkpoints"]), {"after-add", "repeat-add", "after-remove"})
        self.assertFalse(row["qoder_prepared_skill_proof"]["native_ready"])
        self.assertEqual(self.calls, ["add", "list", "repeat", "list", "remove", "post-list"])

    def test_default_native_gate_still_refuses_complete_prepared_lifecycle(self):
        row = self.exercise(enabled=False)
        self.assertEqual(row["status"], "incomplete-native-proof")
        self.assertIsNone(row["qoder_prepared_skill_proof"])
        self.assertTrue(row["repeat_verified"] and row["removal_verified"])

    def test_wrong_or_unsafe_initial_authority_is_rejected(self):
        for fault in ("missing-native", "wrong-receipt", "wrong-scope", "wrong-tuple", "wrong-pin", "wrong-digest", "wrong-public-revision", *("private-fact-" + key for key in ("name", "requested_path", "resolved_path", "raw_tree_digest", "normalizer")), "missing-private-skill", "wrong-private-skill", "wrong-source-relative", "coherent-physical", "coherent-binding", "symlink", "hardlink"):
            with self.subTest(fault=fault), self.assertRaises((ValueError, OSError, KeyError)):
                self.exercise(fault)
            self.assertNotIn("repeat", self.calls)

    def test_repeat_requires_original_complete_namespace_and_receipt(self):
        for fault in ("repeat-native", "repeat-extra", "repeat-receipt", "repeat-ancestor"):
            with self.subTest(fault=fault), self.assertRaises((ValueError, OSError)):
                self.exercise(fault)
            self.assertNotIn("remove", self.calls)

    def test_remove_requires_native_private_and_foreign_namespace_cleanup(self):
        for fault in ("remove-native", "remove-extra", "remove-receipt"):
            with self.subTest(fault=fault), self.assertRaises((ValueError, OSError)):
                self.exercise(fault)

    def test_unsafe_namespace_failure_retains_partial_original_proof(self):
        with self.assertRaises(ValueError) as caught:
            self.exercise("repeat-extra")
        proof = caught.exception.qoder_prepared_skill_proof
        self.assertEqual(proof["refusal"], {"stage": "checkpoint", "type": "ValueError"})
        self.assertEqual(set(proof["checkpoints"]), {"after-add"})
        self.assertIn("snapshot", proof["before"]["native"])
        self.assertNotIn("remove", self.calls)

    def test_summary_cannot_requalify_original_actual_report(self):
        row = self.exercise()
        audit = self.audit
        source = audit.QODER_PREPARED_AUDIT
        report = {"target": "qoder", "kind": "skill", "platform": "linux", "architecture": "x86_64", "canonical_client_id": "qoder", "client_version": {"status": "reported", "value": "1.1.81", "exit_code": 0}, "cli_version": {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0}, "fixture_count": 30, "cases_attempted": 30, "passed": 0, "prepared_lifecycle_passed": 30, "verified_negative_coverage": 0, "full_coverage": True, "source_selection_complete": True, "results": []}
        for i in range(30):
            sample = copy.deepcopy(row); sample["index"] = i; report["results"].append(sample)
        fixtures = {"skills": [self.item] * 30}
        result = source["summary"](report, fixtures, source["ACCEPTANCE"])
        self.assertTrue(result["installation_acceptance_passed"])
        self.assertEqual((report["passed"], result["installation_prepared_lifecycle_passed"]), (0, 30))
        for mutate in (lambda r: r.update(passed=1), lambda r: r.update(prepared_lifecycle_passed=29),
                       lambda r: r.update(client_version={"status": "reported", "value": "1.1.78", "exit_code": 0}),
                       lambda r: r["results"][3].update(qoder_prepared_skill_proof=None),
                       lambda r: r["results"][3].update(status="incomplete-native-proof"),
                       lambda r: r["results"][3].update(skill_path=".uap/import-only/SKILL.md"),
                       lambda r: r["results"][3].update(public_skill_ready={"after-add": True, "repeat-add": True})):
            bad = copy.deepcopy(report); mutate(bad)
            self.assertFalse(source["summary"](bad, fixtures, source["ACCEPTANCE"])["installation_acceptance_passed"])


if __name__ == "__main__":
    unittest.main()
