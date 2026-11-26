"""Real Pi namespace/receipt proof with inert public/native process seams."""
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


@unittest.skipUnless(sys.platform == "linux", "Pi Project lane is Linux only")
class PiProjectSkillContracts(unittest.TestCase):
    def exercise(self, fault=None, *, mode="foreign-prepared", enabled=True, slot=0, version=None):
        audit = load_audit()
        body = b"---\nname: frontend-design\ndescription: Original fixture\n---\nOriginal bytes\n"
        item = {"repo": "anthropics/skills", "ref": "a" * 40, "folder": "frontend-design", "name": "frontend-design",
                "source": "https://github.com/anthropics/skills/tree/" + "a" * 40 + "/skills/frontend-design",
                "skill_md_git_blob_sha1": hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()}
        iid = f"11111111-1111-4111-8111-{slot + 1:012d}"
        calls, owned = [], {}
        self.probes = []
        self.calls, self.owned = calls, owned
        def response(document):
            return {"exit_code": 0, "timeout": False, "stdout_tail": json.dumps(document), "stderr_tail": "", "_json": copy.deepcopy(document)}
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            self.last_root = root
            def install(project):
                if mode == "clean-native":
                    self.assertFalse((project / ".pi/skills/foreign-neighbor/SKILL.md").exists(), "clean cohort already contains unmanaged Project Skill")
                source_key = "github-skill:anthropics/skills:frontend-design:skills/frontend-design"
                source_hash = hashlib.sha256(source_key.encode()).hexdigest()
                declared = "skill-" + source_hash[:56]
                physical = declared[:51] + "-" + hashlib.sha256(iid.encode()).hexdigest()[:12]
                if fault == "coherent-physical": physical = declared[:51] + "-" + "b" * 12
                active = project / ".pi/extensions" / physical
                source = active / "skills/frontend-design"
                source.mkdir(parents=True)
                (source / "SKILL.md").write_bytes(body)
                (source / "binary.bin").write_bytes(b"\xff\x00\x81original")
                (source / "a").mkdir(); (source / "a/nested.txt").write_bytes(b"nested")
                (source / "a.txt").write_bytes(b"sibling")
                native = project / ".pi/skills/frontend-design"
                shutil.copytree(source, native)
                # Independent literal Go WalkDir framing, including directory a
                # before sibling a.txt. This is not the production proof function.
                digest = "sha256:" + hashlib.sha256(b"f\0SKILL.md\0-\0" + body + b"\0"
                    + b"d\0a\0f\0a/nested.txt\0-\0nested\0f\0a.txt\0-\0sibling\0"
                    + b"f\0binary.bin\0-\0\xff\x00\x81original\0").hexdigest()
                (active / "plugin.json").write_text(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": declared}))
                tree, manifest, raw, managed = ("sha256:" + value * 64 for value in "cdef")
                revision = {"resolved_revision": item["ref"], "tree_digest": tree, "manifest_digest": manifest,
                    "skill": {"name": item["name"], "requested_path": "skills/frontend-design", "resolved_path": "skills/frontend-design", "raw_tree_digest": raw, "normalizer": "agent-skill-package/v1"}}
                public_revision = {key: revision[key] for key in ("resolved_revision", "tree_digest", "manifest_digest")}
                public_client = {"client_id": "pi", "scope": "project", "materialization": "materialized", "activation": "prepared",
                    "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "package_revision": public_revision, "affected_surfaces": ["pi"]}
                if mode == "clean-native":
                    public_client.update(activation="active", authentication="not_required")
                bid = "client_" + hashlib.sha256("\0".join((iid, "pi", "project", str(active))).encode()).hexdigest()[:24]
                if fault == "coherent-binding": bid = "client_" + "b" * 24
                binding = {**public_client, "client_binding_id": bid, "physical_artifact_id": physical, "target_locator": str(active),
                    "native_profile_root": str(project / ".pi"), "package_revision": revision, "native_objects": [
                        {"object_id": "package:pi:" + physical, "kind": "managed_package_directory", "logical_name": declared, "path": str(active), "managed_digest": managed, "protection_class": "managed"},
                        {"object_id": "pi_project_skill_directory:frontend-design", "kind": "pi_project_skill_directory", "logical_name": "frontend-design", "path": str(native), "source_relative": "skills/frontend-design", "managed_digest": digest, "protection_class": "managed"}],
                    "receipts": [{"phase": "committed", "operation_id": "operation-original", "sequence": 1, "mutation_type": "directory_swap", "client_binding_id": bid, "active_path": str(active), "after_digest": managed,
                        "environment_authority": {"environment_id": "project-default", "client_id": "pi", "expected_action": "install", "alias": item["name"], "source_key": source_key}}]}
                source_identity = {"stable_source_key": source_key, "source_binding_id": "src_" + source_hash[:24], "repository": item["repo"], "package_subpath": "skills/frontend-design", "resolved_revision": item["ref"], "tree_digest": tree}
                record = {"installation_id": iid, "declared_name": declared, "origin_mode": "direct", "source": source_identity,
                    "package": {"declared_name": declared, "format_id": "agent-plugins/1.0.0", "manifest_digest": manifest, "inventory": {"skills": [item["name"]], "mcp_present": False}}, "clients": {bid: binding}}
                state = {"schema_version": 5, "installations": [record], "environment_receipts": [{"receipt_id": "environment-original", "phase": "committed", "environment_id": "project-default", "alias": item["name"], "client_id": "pi", "installation_id": iid, "client_binding_id": bid, "source_key": source_key, "package_digest": tree, "desired_action": "install"}]}
                lock = {"packages": [{"alias": item["name"], "targets": ["pi"], "tree_digest": tree, "manifest_digest": manifest, "source": {"git": {"kind": "skill", "skill": item["name"], "repository": item["repo"], "commit": item["ref"], "resolved_skill_path": "skills/frontend-design", "raw_tree_digest": raw}}}]}
                public = {"installation_id": iid, "name": declared, "source": "anthropics/skills/skills/frontend-design", "clients": [public_client]}
                (project / ".uap").mkdir()
                profile = project.parent / "home/.pi/agent"
                hook = profile / "extensions/agentplugins-project-trust"
                hook.mkdir(parents=True, mode=0o700)
                (profile / "extensions").chmod(0o700); profile.chmod(0o700)
                hook_source = (Path(__file__).resolve().parents[1] / "install/integrationctl/agentplugins/clients/pi/project_trust_hook.go").read_text()
                marker = '// Generated by Universal Agent Plugins: Pi project trust hook'
                hook_body = marker + hook_source.split('const piTrustHookSource = piTrustHookMarker + `', 1)[1].split('`', 1)[0]
                (hook / "index.ts").write_text(hook_body); (hook / "index.ts").chmod(0o600)
                def trust_digest(path):
                    import os
                    digest = hashlib.sha256()
                    def walk(root, relative=""):
                        for child in sorted(root.iterdir(), key=lambda p: p.name.encode()):
                            rel = relative + child.name
                            if child.is_dir():
                                digest.update(b"d\0" + rel.encode() + b"\0"); walk(child, rel + "/")
                            else:
                                digest.update(b"f\0" + rel.encode() + b"\0" + hashlib.sha256(child.read_bytes()).hexdigest().encode() + b"\0")
                    walk(path)
                    return "sha256:" + digest.hexdigest()
                trust = {"schema_version": 1, "projects": {str(project): {physical: {
                    "extensions": {physical: trust_digest(active)}, "skills": {item["name"]: trust_digest(native)}, "mcp_servers": {}}}}}
                (hook / "projects.json").write_text(json.dumps(trust)); (hook / "projects.json").chmod(0o600)
                for name in ("index.ts.agentplugins.lock", "projects.json.agentplugins.lock"):
                    (hook / name).write_bytes(b""); (hook / name).chmod(0o600)
                if mode == "clean-native":
                    for name in ("auth.json", "models-store.json"):
                        (profile / name).write_bytes(b"{}"); (profile / name).chmod(0o600)
                    if fault == "nonempty-native-store": (profile / "auth.json").write_text('{"provider":"unexpected"}')
                if fault is None and enabled:
                    # Execute only the exact emitted inert trust hook through a
                    # fake extension API, never Pi or its startup/model services.
                    import subprocess
                    module = hook / "index.mjs"
                    module.write_text(hook_body)
                    runner = "import {pathToFileURL} from 'node:url'; const [file,cwd]=process.argv.slice(1); const handlers=[]; (await import(pathToFileURL(file).href)).default({on:(event,fn)=>{if(event==='project_trust')handlers.push(fn)}}); if(handlers.length!==1)throw Error('handler'); console.log(JSON.stringify(await handlers[0]({cwd})));"
                    try:
                        result = subprocess.run([shutil.which("node"), "--input-type=module", "-e", runner, str(module), str(project)], env={"HOME":str(project.parent/"home"), "PI_CODING_AGENT_DIR":str(profile)}, capture_output=True, timeout=5, check=True)
                        self.assertEqual(result.stderr, b"")
                        self.hook_decision = json.loads(result.stdout)
                    finally:
                        module.unlink()
                if fault == "missing-trust": (hook / "projects.json").unlink()
                elif fault == "wrong-trust":
                    trust["projects"][str(project)][physical]["skills"][item["name"]] = "sha256:" + "b" * 64
                    (hook / "projects.json").write_text(json.dumps(trust))
                elif fault == "wrong-hook": (hook / "index.ts").write_text(marker + " spoofed")
                elif fault == "saved-trust": (profile / "trust.json").write_text('{}')
                elif fault == "default-override": (profile / "settings.json").write_text('{"defaultProjectTrust":"always"}')
                elif fault == "wrong-public-revision": public_revision["skill"] = copy.deepcopy(revision["skill"])
                elif fault == "clean-prepared":
                    public_client.update(activation="prepared", authentication="not_checked")
                    binding.update(activation="prepared", authentication="not_checked")
                owned.update(hook=hook, profile=profile,
active=active, native=native, state=state, public=public, binding=binding, removed=False, project=project)
                if fault == "missing-native": shutil.rmtree(native)
                elif fault == "wrong-receipt": binding["native_objects"][1]["path"] = str(project / ".pi/skills/foreign-neighbor")
                elif fault == "wrong-source-relative": binding["native_objects"][1]["source_relative"] = ".agentplugins-pi-native/skills/frontend-design"
                elif fault == "wrong-scope": binding["scope"] = public_client["scope"] = "user"
                elif fault == "wrong-tuple": binding["authentication"] = public_client["authentication"] = "not_required"
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
                        elif fault == "repeat-extra": (project / ".pi/extra-user-residue").write_text("foreign")
                        elif fault == "repeat-receipt":
                            owned["binding"]["receipts"][0]["operation_id"] = "reminted"
                            (project / ".uap/state-v2.json").write_text(json.dumps(owned["state"]))
                        elif fault == "repeat-ancestor":
                            parent = owned["active"].parent; saved = parent.with_name("saved")
                            parent.rename(saved); parent.mkdir(); (saved / owned["active"].name).rename(owned["active"])
                    if fault == "foreign-removed":
                        foreign = project / ".pi/skills/foreign-neighbor/SKILL.md"
                        if foreign.exists(): foreign.unlink()
                    return response({"schema_version": 1, "command": "skills add", "result": "success", "status": "succeeded", "scope": "project", "scope_selection_reason": "explicit", "readiness": "ready" if mode == "clean-native" and fault != "clean-prepared" else "needs_action", "next_actions": [] if mode == "clean-native" and fault != "clean-prepared" else [{"code": "verify_native", "actor": "native_client", "message": "Start a fresh selected client session and verify that the installed Skill is discovered."}], "repairInstructions": [], "items": [{"alias": item["name"], "targetId": "pi", "destinationPath": str(owned["active"]), "status": "succeeded" if calls.count("repeat") == 0 else "unchanged"}]})
                if argv[1:3] == ["skills", "list"]:
                    calls.append("source-metadata")
                    self.assertEqual(argv, ["/inert/agentplugins", "skills", "list", item["source"], "--path", "skills/frontend-design", "--scope", "project", "--project-root", str(project), "--format", "json"])
                    self.assertTrue(_kwargs["capture_full"])
                    data = {"scope": "project", "scope_selection_reason": "explicit", "source": item["repo"], "revision": item["ref"], "source_path": "skills/frontend-design", "complete": False,
                        "candidates": [{"name": "frontend-design", "description": "Original fixture", "path": "skills/frontend-design", "status": "available"}],
                        "diagnostics": [{"code": "exact_skill_selection", "severity": "warning", "message": "Catalog scan is incomplete; explicitly scan a source folder to discover nested Skills"}]}
                    if fault == "metadata-complete": data["complete"] = True
                    elif fault == "metadata-other-incomplete": data["diagnostics"][0]["code"] = "deadline"
                    elif fault == "metadata-wrong-source": data["source"] = "foreign/repo"
                    elif fault == "metadata-wrong-ref": data["revision"] = "b" * 40
                    elif fault == "metadata-wrong-path": data["candidates"][0]["path"] = "foreign"
                    elif fault == "metadata-wrong-name": data["candidates"][0]["name"] = "foreign"
                    elif fault == "metadata-unavailable": data["candidates"][0]["status"] = "unavailable"
                    elif fault == "metadata-extra": data["candidates"].append(copy.deepcopy(data["candidates"][0]))
                    elif fault == "metadata-malformed-mutates-foreign":
                        data["revision"] = "b" * 40
                        (project / "foreign-untouched.txt").write_text("foreign mutation")
                    elif fault == "metadata-native-mutation": (owned["native"] / "binary.bin").write_bytes(b"mutation")
                    output = response({"schema_version": 1, "command": "skills list", "result": "success", "data": data})
                    output.update(stdout_full=output["stdout_tail"], stderr_full="")
                    if fault == "metadata-missing-full": output.pop("stdout_full")
                    elif fault == "metadata-timeout": output["timeout"] = True
                    elif fault == "metadata-exit": output["exit_code"] = 1
                    return output
                if argv[1] == "list":
                    calls.append("post-list" if owned.get("removed") else "list")
                    return response({"schema_version": 1, "command": "list", "result": "success", "data": {"installations": [] if owned.get("removed") else [owned["public"]]}})
                if argv[1] == "remove":
                    calls.append("remove")
                    if fault != "remove-trust":
                        (owned["hook"] / "index.ts").unlink(); (owned["hook"] / "projects.json").unlink()
                    shutil.rmtree(owned["active"])
                    if fault != "remove-native": shutil.rmtree(owned["native"])
                    if fault == "remove-extra": (project / ".pi/extra-user-residue").write_text("new residue")
                    if fault != "remove-receipt":
                        owned["state"]["installations"][0]["clients"] = {}; owned["state"]["environment_receipts"] = []
                    (project / ".uap/state-v2.json").write_text(json.dumps(owned["state"]))
                    (project / "uap.lock").write_text('{"packages": []}')
                    owned["removed"] = True
                    return response({"schema_version": 1, "command": "agentplugins remove", "result": "success", "outcome": "completed", "scope": "project", "scope_selection_reason": "explicit", "actions": [{"from_revision": item["ref"], "from_digest": "sha256:" + "c" * 64, "alias": item["name"], "client_id": "pi", "installation_id": iid, "binding_id": owned["binding"]["client_binding_id"], "action": "remove"}], "results": [{"installation_id": iid, "client_id": "pi", "operation": "remove", "outcome": "completed"}]})
                raise AssertionError(argv)
            def probe(argv, project, env, output, label, **kwargs):
                self.probes.append(label)
                self.assertEqual(argv, ["/inert/pi", "--mode", "rpc", "--no-session", "--offline"])
                self.assertEqual(env["PI_CODING_AGENT_DIR"], str(root / "skill-00/home/.pi/agent"))
                self.assertEqual(env["PI_OFFLINE"], "1")
                self.assertFalse(any("TOKEN" in key or "API_KEY" in key for key in env))
                self.assertTrue(kwargs["memory_only"])
                payload = kwargs["input_bytes"]
                request = json.loads(payload)
                self.assertEqual(set(request), {"id", "type"}); self.assertEqual(request["type"], "get_commands")
                commands = [] if owned["removed"] or fault == "native-missing" else [{"name": "skill:frontend-design", "description": "Original fixture", "source": "skill", "sourceInfo": {"path": str(owned["native"] / "SKILL.md"), "source": "auto", "scope": "project", "origin": "top-level", "baseDir": str(project / ".pi")}}]
                if fault == "native-wrong-description" and commands: commands[0]["description"] = "Wrong but stable description"
                if fault == "native-wrong-path" and commands: commands[0]["sourceInfo"]["path"] = str(project / ".pi/skills/foreign-neighbor/SKILL.md")
                response = {"id": request["id"], "type": "response", "command": "get_commands", "success": True, "data": {"commands": commands}}
                if fault == "native-wrong-id": response["id"] = "wrong"
                stdout = (json.dumps(response) + "\n").encode()
                if fault == "native-extra-line": stdout += b'{}\n'
                kwargs["input_complete"](stdout.splitlines(keepends=True)[0])
                stderr = b"" if fault != "native-stderr" else b"warning"
                record = {"argv": argv, "cwd": str(project), "error": "probe deadline exceeded" if fault == "native-timeout" else None, "exit": 0, "stdin_written": fault != "native-input-unwritten", "stdin_response_confirmed": True}
                for stream, body in (("stdin", payload), ("stdout", stdout), ("stderr", stderr)):
                    record[stream + "_bytes"] = len(body); record[stream + "_sha256"] = hashlib.sha256(body).hexdigest()
                return record, stdout, stderr
            with patch.object(audit, "call", side_effect=call), patch.dict(audit.AGENT_BINARIES, pi="/inert/pi"), patch.object(audit, "COPILOT_CATALOG_PROBE", side_effect=probe):
                row = audit.do_case("/inert/agentplugins", "pi", "skill", item, 0, root, {},
                                     **({"pi_skills": mode} if enabled else {}), client_version=version if version is not None else {"status": "reported", "value": "1.5.0", "exit_code": 0})
            if enabled:
                self.assertFalse((root / "skill-00" / "project-b").exists(),
                                 "explicit Pi cohort created the generic foreign project")
            self.audit, self.item = audit, item
            return row

    def test_foreign_project_is_strict_prepared_only_with_owned_trust_record(self):
        row = self.exercise()
        self.assertEqual(row["status"], "prepared-lifecycle-passed")
        self.assertEqual(row["public_skill_ready"], {"after-add": False, "repeat-add": False})
        proof = row["pi_project_skill_proof"]
        self.assertIn("skills/foreign-neighbor/SKILL.md", proof["before"]["native"]["snapshot"])
        self.assertFalse(proof["native_ready"])
        self.assertEqual(proof["native_catalogs"], {})
        self.assertIn(str(self.owned["project"]), proof["trust"]["record"]["projects"])
        self.assertEqual(set(proof["checkpoints"]), {"after-add", "repeat-add", "after-remove"})
        self.assertEqual(self.probes, [])
        self.assertEqual(self.hook_decision, {"trusted": "undecided"})
        self.assertEqual(self.calls, ["add", "list", "repeat", "list", "remove", "post-list"])

    def test_clean_separate_project_requires_actual_ready_and_fresh_native_catalogs(self):
        row = self.exercise(mode="clean-native")
        self.assertEqual(row["status"], "passed")
        self.assertEqual(self.hook_decision, {"trusted": "yes"})
        self.assertEqual(self.calls, ["add", "list", "source-metadata", "repeat", "list", "remove", "post-list"])
        self.assertEqual(row["public_skill_ready"], {"after-add": True, "repeat-add": True})
        proof = row["pi_project_skill_proof"]
        self.assertTrue(proof["native_ready"])
        self.assertEqual(self.probes, ["pi-after-add", "pi-repeat-add", "pi-after-remove"])
        before = proof["before"]["native"]["snapshot"]
        self.assertNotIn("skills/foreign-neighbor/SKILL.md", before)
        self.assertIn("foreign-neighbor.txt", proof["before"]["home"]["snapshot"])
        self.assertFalse(proof["vendor_authentication"] or proof["model_calls"] or proof["writer_closure"])

    def test_default_native_still_refuses_foreign_prepared(self):
        row = self.exercise(enabled=False)
        self.assertEqual(row["status"], "incomplete-native-proof")
        self.assertIsNone(row["pi_project_skill_proof"])
        self.assertTrue(row["repeat_verified"] and row["removal_verified"])

    def test_wrong_native_version_refuses_before_public_or_native_commands(self):
        for version in ({"status": "reported", "value": "1.5.0-rc.1", "exit_code": 0},
                        {"status": "reported", "value": "1.5.1", "exit_code": 0},
                        {"status": "reported", "value": "1.5.0", "exit_code": False},
                        {"status": "not-probed", "value": "1.5.0", "exit_code": 0}):
            with self.subTest(version=version), self.assertRaises(ValueError):
                self.exercise(version=version)
            self.assertEqual(self.calls, [])
            self.assertEqual(self.probes, [])

    def test_wrong_or_unsafe_initial_authority_refuses(self):
        for mode in ("foreign-prepared", "clean-native"):
            for fault in ("missing-native", "wrong-receipt", "wrong-scope", "wrong-pin", "wrong-digest", "wrong-source-relative", "coherent-physical", "coherent-binding", "symlink", "hardlink", "missing-trust", "wrong-trust", "wrong-hook", "saved-trust", "default-override", "wrong-public-revision", "nonempty-native-store"):
                if fault == "nonempty-native-store" and mode != "clean-native": continue
                with self.subTest(mode=mode, fault=fault), self.assertRaises((ValueError, OSError, KeyError)):
                    self.exercise(fault, mode=mode)
                self.assertNotIn("repeat", self.calls)

    def test_clean_missing_native_or_ready_refuses_before_repeat(self):
        for fault in ("clean-prepared", "native-wrong-description", "native-missing", "native-wrong-path", "native-wrong-id", "native-extra-line", "native-stderr", "native-timeout", "native-input-unwritten"):
            with self.subTest(fault=fault), self.assertRaises((ValueError, OSError, KeyError)):
                self.exercise(fault, mode="clean-native")
            self.assertNotIn("repeat", self.calls)

    def test_canonical_source_metadata_requires_exact_selected_document_and_custody(self):
        for fault in ("metadata-complete", "metadata-other-incomplete", "metadata-wrong-source", "metadata-wrong-ref", "metadata-wrong-path", "metadata-wrong-name", "metadata-unavailable", "metadata-extra", "metadata-missing-full", "metadata-timeout", "metadata-exit", "metadata-native-mutation"):
            with self.subTest(fault=fault), self.assertRaises((ValueError, OSError, KeyError)):
                self.exercise(fault, mode="clean-native")
            self.assertEqual(self.probes, [])
            self.assertNotIn("repeat", self.calls)

    def test_metadata_parser_failure_still_executes_foreign_custody_guard(self):
        with self.assertRaises(ValueError) as caught:
            self.exercise("metadata-malformed-mutates-foreign", mode="clean-native")
        proof = caught.exception.pi_project_skill_proof
        metadata = proof["source_metadata"]
        self.assertEqual((metadata["refusal"], metadata["custody_refusal"]), ("ValueError", "ValueError"))
        self.assertFalse(metadata["owned_resources_unchanged"])
        self.assertIn("stdout_base64", metadata)
        self.assertIn("post-command custody also refused", " ".join(caught.exception.__notes__))
        self.assertFalse(proof["native_ready"])
        self.assertEqual(self.probes, [])

    def test_failed_native_observation_never_preclaims_ready(self):
        with self.assertRaises(ValueError) as caught:
            self.exercise("native-missing", mode="clean-native")
        proof = caught.exception.pi_project_skill_proof
        self.assertFalse(proof["native_ready"])
        self.assertNotIn("selected_present", proof["native_catalogs"]["after-add"])
        self.assertTrue(proof["source_metadata"]["owned_resources_unchanged"])

    def test_full_aggregation_requires_all_independent_proofs_and_original_stream_hashes(self):
        # Aggregation mechanics use 30 fresh inert lifecycle instances. Canonical
        # 30-source selection is independently checked by the main-routing test.
        for mode, acceptance in (("foreign-prepared", "pi-foreign-prepared-skills-installation"), ("clean-native", "pi-clean-native-skills")):
            rows = []
            for index in range(30):
                row = self.exercise(mode=mode, slot=index)
                row["index"] = index
                rows.append(row)
            report = {"target": "pi", "kind": "skill", "platform": "linux", "architecture": "x86_64", "canonical_client_id": "pi", "pi_skill_cohort": mode, "cli": "/inert/agentplugins",
                "client_version": {"status": "reported", "value": "1.5.0", "exit_code": 0}, "cli_version": {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0},
                "fixture_count": 30, "cases_attempted": 30, "passed": 30 if mode == "clean-native" else 0, "prepared_lifecycle_passed": 0 if mode == "clean-native" else 30,
                "verified_negative_coverage": 0, "full_coverage": True, "source_selection_complete": True, "results": rows}
            fixtures = {"skills": [self.item] * 30}
            summary = self.audit.pi_project_skill_audit()["summary"]
            measured = summary(report, fixtures, acceptance)
            self.assertEqual(measured, {"pi_cohort_verified": 30, "pi_cohort_acceptance_passed": True})
            mutations = [lambda r: r.update(cases_attempted=29),
                         lambda r: r["results"][4].update(pi_project_skill_proof=None),
                         lambda r: r["results"][4].update(repeat_verified=False),
                         lambda r: r["results"][4]["pi_project_skill_proof"].update(native_ready=mode != "clean-native"),
                         lambda r: r["results"][4].update(public_skill_ready={"after-add": False, "repeat-add": False} if mode == "clean-native" else {"after-add": True, "repeat-add": True})]
            # These are valid full proofs; change only JSON scalar types whose
            # Python equality would otherwise alias booleans and integers.
            for phase in ("after-add", "repeat-add"):
                for wrong in (int(mode == "clean-native"), float(mode == "clean-native")):
                    mutations.append(lambda r, phase=phase, wrong=wrong:
                        r["results"][4]["public_skill_ready"].update({phase: wrong}))
            for phase in ("after-add", "repeat-add", "after-remove"):
                for key in ("invalid_rows", "selected_installations", "selected_bindings"):
                    value = report["results"][4]["public_target_states"][phase][key]
                    for wrong in (bool(value), float(value)):
                        mutations.append(lambda r, phase=phase, key=key, wrong=wrong:
                            r["results"][4]["public_target_states"][phase].update({key: wrong}))
            for key in ("client_version", "cli_version"):
                for wrong in (False, 0.0):
                    mutations.append(lambda r, key=key, wrong=wrong: r[key].update(exit_code=wrong))
            if mode == "clean-native":
                for phase in ("after-add", "repeat-add", "after-remove"):
                    for stream in ("stdin", "stdout", "stderr"):
                        key = stream + "_bytes"
                        value = report["results"][4]["pi_project_skill_proof"]["native_catalogs"][phase][key]
                        for wrong in ([float(value), False] if value == 0 else [float(value)]):
                            mutations.append(lambda r, phase=phase, key=key, wrong=wrong:
                                r["results"][4]["pi_project_skill_proof"]["native_catalogs"][phase].update({key: wrong}))
                mutations += [lambda r: r["results"][4]["pi_project_skill_proof"]["source_metadata"].update(stdout_sha256="0"*64),
                              lambda r: r["results"][4]["pi_project_skill_proof"]["source_metadata"].update(owned_resources_unchanged=False),
                              lambda r: r["results"][4]["pi_project_skill_proof"]["native_catalogs"]["after-add"].update(stdout_sha256="0"*64),
                              lambda r: r["results"][4]["pi_project_skill_proof"]["native_catalogs"]["after-remove"].update(selected_present=True)]
            for mutate in mutations:
                changed = copy.deepcopy(report); mutate(changed)
                self.assertFalse(summary(changed, fixtures, acceptance)["pi_cohort_acceptance_passed"])

    def test_original_foreign_cannot_be_removed_to_gain_readiness(self):
        with self.assertRaises((ValueError, OSError)):
            self.exercise("foreign-removed")

    def test_repeat_original_receipts_and_complete_namespaces_are_stable(self):
        for mode in ("foreign-prepared", "clean-native"):
            for fault in ("repeat-native", "repeat-extra", "repeat-receipt", "repeat-ancestor"):
                with self.subTest(mode=mode, fault=fault), self.assertRaises((ValueError, OSError)):
                    self.exercise(fault, mode=mode)
                self.assertNotIn("remove", self.calls)

    def test_removal_requires_all_receipts_native_and_owned_hook_cleanup(self):
        for mode in ("foreign-prepared", "clean-native"):
            for fault in ("remove-native", "remove-extra", "remove-receipt", "remove-trust"):
                with self.subTest(mode=mode, fault=fault), self.assertRaises((ValueError, OSError)):
                    self.exercise(fault, mode=mode)

    def test_main_only_routes_complete_exact_pi_lane_and_missing_proof_fails(self):
        from contextlib import redirect_stdout, redirect_stderr
        from io import StringIO
        for acceptance, mode in (("pi-foreign-prepared-skills-installation", "foreign-prepared"), ("pi-clean-native-skills", "clean-native")):
            audit = load_audit()
            fixtures = json.loads(audit.FIXTURES.read_text())
            def row(*args, **kwargs):
                self.assertEqual(kwargs["pi_skills"], mode)
                self.assertEqual(kwargs["client_version"], {"status": "reported", "value": "1.5.0", "exit_code": 0})
                self.assertEqual(args[1:3], ("pi", "skill"))
                item, index = args[3:5]
                self.assertEqual(item, fixtures["skills"][index])
                return {"index": index, "target": "pi", "kind": "skill", "identity": item["name"], "source": item["source"], "status": "passed" if mode == "clean-native" else "prepared-lifecycle-passed"}
            with TemporaryDirectory() as temporary:
                argv = ["audit", "--cli", sys.executable, "--target", "pi", "--kind", "skill", "--limit", "30", "--acceptance", acceptance, "--report-dir", temporary]
                with patch.object(audit.sys, "argv", argv), patch.object(audit.platform, "machine", return_value="x86_64"), patch.dict(audit.AGENT_BINARIES, pi=sys.executable), patch.object(audit, "vendor_bootstrap_attested", return_value=True), patch.object(audit, "version_evidence", side_effect=lambda argv, *_: {"status": "reported", "value": "1.5.0" if argv[-1] == "--version" else "agentplugins 0.1.115", "exit_code": 0}), patch.object(audit, "do_case", side_effect=row) as cases, redirect_stdout(StringIO()), redirect_stderr(StringIO()):
                    self.assertEqual(audit.main(), 1)
                self.assertEqual(cases.call_count, 30)
                report = json.loads((Path(temporary) / "pi-skill.json").read_text())
                self.assertEqual(report["pi_skill_cohort"], mode)
                self.assertFalse(report["pi_cohort_acceptance_passed"])
                self.assertEqual(report["pi_cohort_verified"], 0)

    def test_default_and_explicit_native_routes_do_not_load_or_forward_pi_cohort(self):
        from contextlib import redirect_stdout, redirect_stderr
        from io import StringIO
        for target, kind in (("pi", "skill"), ("pi", "plugin"), ("copilot", "skill")):
            for acceptance in (None, "native"):
                audit = load_audit()
                with TemporaryDirectory() as temporary:
                    argv = ["audit", "--cli", sys.executable, "--target", target,
                            "--kind", kind, "--limit", "1", "--report-dir", temporary]
                    if acceptance is not None:
                        argv += ["--acceptance", acceptance]
                    def row(*args, **kwargs):
                        self.assertNotIn("pi_skills", kwargs)
                        item, index = args[3:5]
                        return {"target": target, "kind": kind, "index": index,
                                "identity": item["name"] if kind == "skill" else item,
                                "source": item["source"] if kind == "skill" else item,
                                "status": "incomplete-native-proof"}
                    with patch.object(audit.sys, "argv", argv), patch.dict(audit.AGENT_BINARIES, {target: sys.executable}), patch.object(audit, "vendor_bootstrap_attested", return_value=True), patch.object(audit, "version_evidence", return_value={"status": "reported", "value": "1.5.0", "exit_code": 0}), patch.object(audit, "do_case", side_effect=row) as cases, patch.object(audit, "pi_project_skill_audit", side_effect=AssertionError("inactive Pi observer loaded")) as loader, redirect_stdout(StringIO()), redirect_stderr(StringIO()):
                        self.assertEqual(audit.main(), 1)
                    cases.assert_called_once()
                    loader.assert_not_called()
                    report = json.loads((Path(temporary) / (target + "-" + kind + ".json")).read_text())
                    self.assertNotIn("pi_skill_cohort", report)
                    self.assertNotIn("pi_cohort_acceptance_passed", report)
                    self.assertEqual(report["passed"], 0)

    def test_explicit_cohort_bootstrap_absence_retains_unexecuted_nonpass_evidence(self):
        from contextlib import redirect_stdout, redirect_stderr
        from io import StringIO
        for acceptance, mode in (("pi-foreign-prepared-skills-installation", "foreign-prepared"),
                                 ("pi-clean-native-skills", "clean-native")):
            audit = load_audit()
            with TemporaryDirectory() as temporary:
                argv = ["audit", "--cli", sys.executable, "--target", "pi", "--kind", "skill",
                        "--limit", "30", "--acceptance", acceptance, "--report-dir", temporary]
                with patch.object(audit.sys, "argv", argv), patch.object(audit.platform, "machine", return_value="x86_64"), patch.object(audit, "vendor_bootstrap_attested", return_value=False), patch.object(audit, "do_case") as cases, patch.object(audit, "version_evidence") as probes, patch.object(audit, "pi_project_skill_audit", side_effect=AssertionError("unexecuted observer loaded")) as loader, redirect_stdout(StringIO()), redirect_stderr(StringIO()):
                    self.assertEqual(audit.main(), 2)
                cases.assert_not_called()
                probes.assert_not_called()
                loader.assert_not_called()
                report = json.loads((Path(temporary) / "pi-skill.json").read_text())
                self.assertEqual(report["pi_skill_cohort"], mode)
                self.assertEqual(report["cases_attempted"], 0)
                self.assertFalse(report["full_coverage"])
                self.assertEqual(len(report["results"]), 30)
                self.assertTrue(all(row["status"] == "client-unavailable" for row in report["results"]))
                self.assertEqual(report.get("passed", 0), 0)
                self.assertEqual(report.get("verified_negative_coverage", 0), 0)
                self.assertNotIn("pi_cohort_acceptance_passed", report)

    def test_main_refuses_partial_other_scope_host_and_client_before_any_case(self):
        from contextlib import redirect_stdout, redirect_stderr
        from io import StringIO
        for target, kind, limit, platform, architecture in (("pi", "skill", 29, "linux", "x86_64"), ("pi", "plugin", 30, "linux", "x86_64"), ("qoder", "skill", 30, "linux", "x86_64"), ("pi", "skill", 30, "darwin", "x86_64"), ("pi", "skill", 30, "linux", "aarch64")):
            for acceptance in ("pi-foreign-prepared-skills-installation", "pi-clean-native-skills"):
                audit = load_audit()
                argv = ["audit", "--cli", sys.executable, "--target", target, "--kind", kind, "--limit", str(limit), "--acceptance", acceptance, "--report-dir", "/must-not-create"]
                with patch.object(audit.sys, "argv", argv), patch.object(audit.sys, "platform", platform), patch.object(audit.platform, "machine", return_value=architecture), patch.object(audit, "do_case") as cases, redirect_stdout(StringIO()), redirect_stderr(StringIO()), self.assertRaises(SystemExit) as error:
                    audit.main()
                self.assertEqual(error.exception.code, 2)
                cases.assert_not_called()



if __name__ == "__main__": unittest.main()
