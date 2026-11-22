"""Inert wiring regressions; these do not execute or qualify native Grok."""
from contextlib import ExitStack
import hashlib
from io import StringIO
import json
import os
from pathlib import Path
import platform
import shutil
import sys
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import Mock, patch

from test_live_install_audit_env import load_audit


@unittest.skipUnless(sys.platform == "linux" and platform.machine() == "x86_64",
                     "owner-pretrusted Grok scenario is pinned to Linux x86_64")
class GrokProfileWiringTests(unittest.TestCase):
    def lifecycle(self, *, pretrusted=True, ready=True, native_verified=True,
                  mutate_repeat=False, remint=False, duplicate_public=False, nonfinite_public=None,
                  real_verifier=False):
        audit = load_audit()
        body = b"---\nname: source-skill\ndescription: unit fixture\n---\nInert bytes.\n"
        item = {"repo": "example/skills", "folder": "source-skill", "name": "source-skill",
                "ref": "a" * 40, "source": "https://github.com/example/skills/tree/" + "a" * 40 + "/skills/source-skill",
                "skill_md_git_blob_sha1": hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()}
        uid = "c99742fc-59bf-4225-b862-0c21e5fcc992"
        name = "skill-" + hashlib.sha256(b"github-skill:example/skills:source-skill:skills/source-skill").hexdigest()[:56]
        physical = name[:51] + "-" + hashlib.sha256(uid.encode()).hexdigest()[:12]
        revision = {"resolved_revision": item["ref"], "tree_digest": "sha256:" + "b" * 64,
                    "manifest_digest": "sha256:" + "c" * 64}
        node = {"installation_id": uid, "name": name,
                "source": "example/skills/skills/source-skill", "clients": [{
                    "client_id": "grok", "scope": "project", "materialization": "materialized",
                    "activation": "active" if ready else "prepared", "authentication": "not_required" if ready else "not_checked",
                    "verification": "installation_verified" if ready else "package_validated",
                    "policy": "allowed", "package_revision": revision}]}
        state, events = {"installed": False, "adds": 0}, []
        runtime = Mock()
        runtime.finish.return_value = native_verified
        runtime.evidence.return_value = {"unit_only": True}
        if real_verifier:
            import grok_project_skill_audit as verifier_module

        def factory(*_args):
            nonlocal runtime
            events.append("fixture")
            if real_verifier:
                runtime = verifier_module.GrokPretrustedSkillAudit(*_args)
            return runtime

        def probe(*args, **kwargs):
            self.assertEqual(args[2].get("GROK_DISABLE_AUTOUPDATER"), "1")
            self.assertTrue(kwargs["memory_only"])
            self.assertEqual(kwargs["limit"], 1 << 20)
            self.assertEqual(kwargs["timeout"], 30)
            events.append(args[4])
            if real_verifier:
                document = {"grokVersion": "1.0.70", "cwd": str(project), "projectRoot": str(project),
                            "projectTrusted": True, "plugins": [], "mcpServers": [],
                            "skills": [{"name": "foreign-neighbor", "description": "unrelated", "userInvocable": True,
                                        "source": {"type": "project", "path": str(project / ".grok/skills/foreign-neighbor/SKILL.md")}}]}
                if state["installed"]:
                    document["plugins"].append({"name": name, "scope": "project", "path": str(bundle), "enabled": True,
                                                "provides": {"skills": 1, "agents": 0, "hooks": False, "mcpServers": 0}})
                    document["skills"].append({"name": item["name"], "description": "unit fixture", "userInvocable": True,
                                               "source": {"type": "plugin", "plugin_name": name, "path": str(skill)}})
                stdout, stderr, label = json.dumps(document).encode(), b"", args[4]
                record = {"probe": label, "argv": args[0], "cwd": str(args[1]), "error": None, "exit": 0,
                          "elapsed_seconds": 0.01, "stdout_file": label + ".stdout.log", "stderr_file": label + ".stderr.log",
                          "stdout_bytes": len(stdout), "stderr_bytes": 0,
                          "stdout_sha256": hashlib.sha256(stdout).hexdigest(), "stderr_sha256": hashlib.sha256(stderr).hexdigest()}
                return record, stdout, stderr
            return {"unit_only": True}, b"{}", b""

        with TemporaryDirectory() as tmp, ExitStack() as patches:
            root = Path(tmp).resolve()
            project = root / "skill-00/project"
            bundle = project / ".grok/plugins" / physical
            skill = bundle / "skills/source-skill/SKILL.md"
            native_binary = sys.executable
            if real_verifier:
                inert_bytes = b"Inert integration unit fixture; never execute this file.\n"
                executable = root / "grok-inert-unit"
                executable.write_bytes(inert_bytes)
                executable.chmod(0o700)
                native_binary = str(executable)
                patches.enter_context(patch.object(verifier_module, "BINARY_SHA256", hashlib.sha256(inert_bytes).hexdigest()))

            def call(argv, env, cwd, timeout=105, **kwargs):
                if pretrusted:
                    self.assertEqual(env.get("GROK_DISABLE_AUTOUPDATER"), "1")
                if real_verifier and argv[0] == audit.GIT_EXECUTABLE:
                    # Keep the production Git prerequisite wrapper, substituting only its child transport.
                    self.assertEqual(argv[-2:], ["init", "--quiet"])
                    self.assertTrue((root / "skill-00/home/.grok/config.toml").is_file())
                    self.assertTrue((root / "skill-00/home/.grok/trusted_folders.toml").is_file())
                    (project / ".git").mkdir()
                    events.append("git")
                    value = {}
                elif argv[1:3] == ["skills", "add"]:
                    state["adds"] += 1
                    if state["adds"] == 1:
                        skill.parent.mkdir(parents=True)
                        skill.write_bytes(body)
                        native_manifest = bundle / ".grok-plugin/plugin.json"
                        native_manifest.parent.mkdir()
                        native_manifest.write_text(json.dumps({"name": name, "version": "0.0.0", "skills": "./skills/"}))
                        (project / "uap.toml").write_text("# owned unit lock\n")
                        (project / "uap.lock").write_text(json.dumps({"lock_version": 1,
                            "manifest_digest": "sha256:" + "e" * 64, "packages": [{
                            "alias": item["name"], "targets": ["grok"],
                            "source_key": "github-skill:example/skills:source-skill:skills/source-skill",
                            "digest_algorithm": "agentplugins-tree-sha256-v1",
                            "source": {"git": {"kind": "skill", "skill": item["name"],
                                "repository": item["repo"], "commit": item["ref"],
                                "path": "skills/source-skill", "resolved_skill_path": "skills/source-skill",
                                "normalizer": "agent-skill-package/v1",
                                "requested": {"kind": "commit", "value": item["ref"]}}},
                            "tree_digest": revision["tree_digest"], "manifest_digest": revision["manifest_digest"]}]}))
                    elif mutate_repeat:
                        (bundle / "unexpected.txt").write_text("repeat mutation")
                    state["installed"] = True
                    value = {"schema_version": 1, "command": "skills add", "result": "success", "scope": "project",
                             "status": "succeeded", "readiness": "ready" if ready else "needs_action",
                             "next_actions": [] if ready else ["trust"], "items": [{
                                 "alias": item["name"], "targetId": "grok",
                                 "status": "succeeded" if state["adds"] == 1 else "unchanged",
                                 "candidateId": "sha256:" + "d" * 64, "destinationPath": str(bundle)}]}
                elif argv[1] == "remove":
                    self.assertEqual(argv[2], uid, "generated-name Skills must remove by verified UUID")
                    shutil.rmtree(bundle)
                    (project / "uap.toml").write_text("schema_version = 1\n[dependencies]\n[targets]\n")
                    empty_digest = "sha256:" + hashlib.sha256(
                        b'{"schema_version":1,"dependencies":null,"targets":null}').hexdigest()
                    candidate = project / "empty-lock-unit-candidate"
                    candidate.write_text(json.dumps({"lock_version": 1,
                        "manifest_digest": empty_digest, "packages": []}))
                    os.replace(candidate, project / "uap.lock")
                    state["installed"] = False
                    value = {"result": "success"}
                elif argv[1] == "list":
                    entries = [node] if state["installed"] else []
                    if not state["installed"] and remint:
                        entries = [{**node, "name": "renamed", "installation_id": "reminted"}]
                    value = {"data": {"installations": entries}}
                else:
                    raise AssertionError(argv)
                text = json.dumps(value)
                if duplicate_public and argv[1:3] == ["skills", "add"]:
                    text = '{"readiness":"needs_action",' + text[1:]
                if nonfinite_public and argv[1:3] == ["skills", "add"]:
                    text = '{"ignored":' + nonfinite_public + ',' + text[1:]
                return {"exit_code": 0, "timeout": False, "stdout_tail": text[-1800:],
                        "stderr_tail": "", "stdout_full": text, "stderr_full": "", "_json": value}

            patches.enter_context(patch.object(audit, "call", side_effect=call))
            if not real_verifier:
                patches.enter_context(patch.object(audit, "initialize_project_skill_git_root",
                                                   side_effect=lambda *_args: events.append("git")))
            patches.enter_context(patch.dict(audit.AGENT_BINARIES, {"grok": native_binary}))
            patches.enter_context(patch.dict(audit.GROK_PROJECT_AUDIT,
                                  {"GrokPretrustedSkillAudit": factory}))
            patches.enter_context(patch.dict(audit.NATIVE_RESEARCH, {"run_probe": probe}))
            result = audit.do_case("/unused/agentplugins", "grok", "skill", item, 0, root, {},
                client_version={"status": "reported", "value": "grok 1.0.70 (f50904eaee6c)", "exit_code": 0},
                grok_pretrusted=pretrusted)
        return result, events, runtime

    def test_real_verifier_survives_saved_remove_atomic_empty_agreement(self):
        """Integration unit only: real guards/verdict, inert child transports and binary pin."""
        result, events, runtime = self.lifecycle(real_verifier=True)
        self.assertEqual(result["status"], "passed")
        self.assertEqual(events, ["fixture", "git", "grok-pretrusted-before", "grok-pretrusted-add",
                                  "grok-pretrusted-repeat", "grok-pretrusted-remove"])
        proof = result["grok_pretrusted_native_proof"]
        self.assertEqual([row["phase"] for row in proof["observations"]], ["before", "add", "repeat", "remove"])
        self.assertTrue(all(row["verified"] for row in proof["observations"]))
        self.assertTrue(proof["native_discovery_verified"])
        self.assertFalse(proof["cold_activation_claimed"])
        self.assertFalse(proof["writer_closure_claimed"])
        self.assertEqual(proof["profile"]["initial"], proof["profile"]["last_verified"])
        self.assertEqual(len(proof["profile"]["initial"]), 6)
        self.assertNotEqual(proof["binding"]["lock"]["inode"], proof["removal_agreement"]["lock"]["inode"])
        self.assertEqual(proof["binding"]["lock"]["device"], proof["removal_agreement"]["lock"]["device"])
        import base64
        import tomllib
        locked = json.loads(base64.b64decode(proof["removal_agreement"]["lock"]["base64"]))
        self.assertEqual(locked, {"lock_version": 1, "packages": [], "manifest_digest": "sha256:" + hashlib.sha256(
            b'{"schema_version":1,"dependencies":null,"targets":null}').hexdigest()})
        manifest = tomllib.loads(base64.b64decode(proof["removal_agreement"]["manifest"]["base64"]).decode())
        self.assertEqual(manifest, {"schema_version": 1, "dependencies": {}, "targets": {}})
        self.assertTrue(result["repeat_project_unchanged"])
        self.assertTrue(result["removal_verified"])
        self.assertTrue(runtime.closed)

    def test_explicit_profile_precedes_children_and_requires_all_native_phases(self):
        result, events, runtime = self.lifecycle()
        self.assertEqual(result["status"], "passed")
        self.assertEqual(events, ["fixture", "git", "grok-pretrusted-before", "grok-pretrusted-add",
                                  "grok-pretrusted-repeat", "grok-pretrusted-remove"])
        self.assertEqual([call.args[0] for call in runtime.observe.call_args_list],
                         ["before", "add", "repeat", "remove"])
        self.assertEqual(runtime.bind_installation.call_count, 2)
        runtime.begin_remove.assert_called_once()
        runtime.close.assert_called_once()
        self.assertTrue(result["repeat_project_unchanged"])
        self.assertEqual(result["project_profile"], "owner-pretrusted")

    def test_default_cold_preparation_stays_non_native_without_profile_authoring(self):
        result, events, runtime = self.lifecycle(pretrusted=False, ready=False)
        self.assertEqual(result["status"], "prepared-lifecycle-passed")
        self.assertEqual(events, ["git"])
        runtime.finish.assert_not_called()
        self.assertNotIn("grok_pretrusted_native_proof", result)

    def test_native_ready_and_cleanup_are_all_required(self):
        for change in ({"ready": False}, {"native_verified": False}, {"native_verified": None},
                       {"native_verified": 1}, {"mutate_repeat": True}, {"remint": True},
                       {"duplicate_public": True}, {"nonfinite_public": "NaN"},
                       {"nonfinite_public": "Infinity"}, {"nonfinite_public": "1e999"}):
            with self.subTest(change=change):
                result, _, _ = self.lifecycle(**change)
                self.assertNotEqual(result["status"], "passed")

    def test_entrypoint_profile_is_explicit_separate_and_cannot_accept_prepared(self):
        for profile in ("cold", "owner-pretrusted"):
            with self.subTest(profile=profile), TemporaryDirectory() as tmp:
                audit = load_audit()
                native = {"status": "reported", "exit_code": 0, "value": "grok 1.0.70 (f50904eaee6c)"}
                argv = ["live-install-audit.py", "--cli", sys.executable, "--target", "grok",
                        "--kind", "skill", "--limit", "1", "--report-dir", tmp,
                        "--grok-project-profile", profile]
                with (patch.object(sys, "argv", argv), patch.object(sys, "stdout", StringIO()),
                      patch.dict(audit.AGENT_BINARIES, {"grok": sys.executable}, clear=True),
                      patch.object(audit, "vendor_bootstrap_attested", return_value=True),
                      patch.object(audit, "version_evidence", return_value=native),
                      patch.object(audit, "do_case", return_value={"status": "prepared-lifecycle-passed"}) as case):
                    self.assertEqual(audit.main(), 1)
                expected = {"client_version": native, "grok_pretrusted": True} if profile == "owner-pretrusted" else {}
                self.assertEqual(case.call_args.kwargs, expected)
                suffix = "-owner-pretrusted" if profile == "owner-pretrusted" else ""
                report = json.loads((Path(tmp) / f"grok-skill{suffix}.json").read_text())
                self.assertEqual(report["passed"], 0)
                self.assertEqual(report["prepared_lifecycle_passed"], 1)
                self.assertEqual(report.get("project_profile"), profile if suffix else None)

    def test_profile_refuses_other_targets_or_scopes_before_execution(self):
        audit = load_audit()
        for target, kind in (("codex", "skill"), ("grok", "plugin")):
            with self.subTest(target=target, kind=kind), TemporaryDirectory() as tmp:
                argv = ["live-install-audit.py", "--cli", sys.executable, "--target", target,
                        "--kind", kind, "--report-dir", tmp, "--grok-project-profile", "owner-pretrusted"]
                with patch.object(sys, "argv", argv), patch.object(sys, "stderr", StringIO()), \
                     patch.object(audit, "do_case") as case:
                    with self.assertRaises(SystemExit) as raised:
                        audit.main()
                self.assertEqual(raised.exception.code, 2)
                case.assert_not_called()

    def test_workflow_keeps_cold_matrix_and_adds_distinct_pretrusted_cell(self):
        import itertools
        import yaml
        path = Path(__file__).resolve().parents[1] / ".github/workflows/live-install-remaining-clients.yml"
        workflow = yaml.safe_load(path.read_text())
        job = workflow["jobs"]["remaining-client-install"]
        matrix = job["strategy"]["matrix"]
        axes = ["target", "kind", "project_profile"]
        original = [dict(zip(axes, values)) for values in itertools.product(*(matrix[key] for key in axes))]
        original = [row for row in original if not any(all(row.get(k) == v for k, v in excluded.items())
                    for excluded in matrix["exclude"])]
        extra = matrix["include"]
        self.assertEqual(len(original), 9)
        self.assertTrue(all(row["project_profile"] == "cold" for row in original))
        self.assertEqual(extra, [{"target": "grok", "kind": "skill", "project_profile": "owner-pretrusted"}])
        self.assertNotIn(extra[0], original)
        self.assertIn("owner-pretrusted Project", job["name"])
        upload = next(step for step in job["steps"] if "upload-artifact@" in step.get("uses", ""))
        self.assertIn("-owner-pretrusted", upload["with"]["name"])


if __name__ == "__main__":
    unittest.main()
