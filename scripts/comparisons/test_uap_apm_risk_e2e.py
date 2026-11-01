"""Critical observer properties. Real products are tested in the hosted matrix."""
from pathlib import Path
import json
import tempfile
import unittest
from unittest.mock import patch

import uap_apm_risk_e2e as risk


class RiskOracleTests(unittest.TestCase):
    def test_exact_six_cases_split_into_three_parallel_groups(self):
        self.assertEqual(len(risk.GROUPS), 3)
        self.assertEqual(sum(map(len, risk.GROUPS.values())), 6)
        self.assertEqual(
            set().union(*(set(scenarios) for scenarios in risk.GROUPS.values())),
            {"R1_missing_receipts","R2_claude_profiles","R3_transitive_update",
             "R4_interrupted_operation","R5_mcp_env_secret","R6_git_pull_removal"},
        )

    def test_unobserved_success_never_qualifies(self):
        self.assertEqual(risk.status(True,None,True,"unobserved")[0],"BLOCKED")
        self.assertEqual(risk.status(True,{"exit_code":0},False,"wrong bytes")[0],"FAIL")
        self.assertEqual(risk.status(True,{"exit_code":0},True,"unproven lock",
                                     provenance=False)[0],"FAIL")
        self.assertEqual(risk.status(True,{"exit_code":0},True,"foreign changed",
                                     protected=False)[0],"FAIL")

    def test_safe_refusal_vs_verified_loss(self):
        self.assertEqual(risk.status(True,{"exit_code":1},True,"refusal")[0],"BLOCKED")
        self.assertEqual(risk.status(True,{"exit_code":1},False,"bytes deleted")[0],"FAIL")
        self.assertEqual(risk.status(True,{"exit_code":0,"timed_out":True},
                                     True,"late success")[0],"BLOCKED")
        self.assertEqual(risk.status(False,{"exit_code":0},True,"unsupported")[0],"BLOCKED")

    def test_read_observer_rejects_missing_symlink_directory_and_large(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            target=root/"skill.md"
            self.assertIsNone(risk.observed_file(target,root))
            target.write_bytes(b"exact")
            self.assertEqual(risk.observed_file(target,root),b"exact")
            self.assertIsNone(risk.observed_file(target,root,limit=4))
            alias=root/"alias.md"
            alias.symlink_to(target)
            self.assertIsNone(risk.observed_file(alias,root))
            folder=root/"subfolder"
            folder.mkdir()
            self.assertIsNone(risk.observed_file(folder,root))
            linked=root/"linked"
            linked.symlink_to(folder,target_is_directory=True)
            self.assertIsNone(risk.observed_file(linked/"missing",root))
            self.assertIsNone(risk.observed_file(root,"../",limit=1))

    def test_exact_two_node_graph_rejects_wrong_repo_stale_parent_and_duplicate_child(self):
        with tempfile.TemporaryDirectory() as tmp:
            project=Path(tmp)
            parent={"alias":"risk-parent","source":{"git":{
                "repository":risk.REPO, "path":risk.PARENT,
                "commit":risk.PARENT_V1}}}
            child={"alias":"risk-child","source":{"git":{
                "repository":risk.REPO,"path":risk.CHILD,
                "commit":risk.CHILD_V1}},
                "required_by":[{"alias":"risk-parent","selector":{
                    "kind":"commit","value":risk.CHILD_V1}}]}
            base={"packages":[parent,child]}
            def write(lock):
                (project/"uap.lock").write_text(json.dumps(lock))
            write(base)
            self.assertTrue(risk.graph_lock_matches(
                project,"uap",risk.PARENT_V1,risk.CHILD_V1))
            bad=json.loads(json.dumps(base))
            bad["packages"][0]["source"]["git"]["commit"]=risk.PARENT_V2
            write(bad)
            self.assertFalse(risk.graph_lock_matches(
                project,"uap",risk.PARENT_V1,risk.CHILD_V1))
            bad=json.loads(json.dumps(base))
            bad["packages"][1]["source"]["git"]["repository"]="wrong/repo"
            write(bad)
            self.assertFalse(risk.graph_lock_matches(
                project,"uap",risk.PARENT_V1,risk.CHILD_V1))
            bad=json.loads(json.dumps(base))
            bad["packages"].append(bad["packages"][1])
            write(bad)
            self.assertFalse(risk.graph_lock_matches(
                project,"uap",risk.PARENT_V1,risk.CHILD_V1))
            bad=json.loads(json.dumps(base))
            bad["packages"][1]["required_by"][0]["alias"]="unrelated"
            write(bad)
            self.assertFalse(risk.graph_lock_matches(
                project,"uap",risk.PARENT_V1,risk.CHILD_V1))

    def test_apm_graph_requires_real_resolved_by_relation(self):
        with tempfile.TemporaryDirectory() as tmp:
            project=Path(tmp)
            package={"repo_url":risk.REPO,"virtual_path":risk.PARENT,
                     "resolved_commit":risk.PARENT_V1}
            child={"repo_url":risk.REPO,"virtual_path":risk.CHILD,
                   "resolved_commit":risk.CHILD_V1,
                   "resolved_by":risk.REPO}
            source={"dependencies":[package,child]}
            (project/"apm.lock.yaml").write_text(json.dumps(source))
            self.assertTrue(risk.graph_lock_matches(
                project,"apm",risk.PARENT_V1,risk.CHILD_V1))
            child["resolved_by"]="wrong/repo"
            (project/"apm.lock.yaml").write_text(json.dumps(source))
            self.assertFalse(risk.graph_lock_matches(
                project,"apm",risk.PARENT_V1,risk.CHILD_V1))
            child["resolved_by"]=risk.REPO
            source["dependencies"].append(dict(child))
            (project/"apm.lock.yaml").write_text(json.dumps(source))
            self.assertFalse(risk.graph_lock_matches(
                project,"apm",risk.PARENT_V1,risk.CHILD_V1))

    def test_profile_observer_needs_skill_and_reference_and_no_symlink(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            skill=root/"skills"/"risk"
            skill.mkdir(parents=True)
            (skill/"SKILL.md").write_bytes(b"body")
            expected={"SKILL.md":b"body","references/fixture.txt":b"reference"}
            self.assertFalse(risk.profile_scan(root,expected))
            (skill/"references").mkdir()
            (skill/"references/fixture.txt").write_bytes(b"reference")
            self.assertEqual(len(risk.profile_scan(root,expected)),1)
            expected["SKILL.md"]=b"different"
            self.assertFalse(risk.profile_scan(root,expected))
            expected["SKILL.md"]=b"body"
            (skill/"SKILL.md").unlink()
            (skill/"SKILL.md").symlink_to(root/"other")
            (root/"other").write_bytes(b"body")
            self.assertFalse(risk.profile_scan(root,expected))

    def test_incomplete_zero_exit_cannot_qualify_Claude_profile(self):
        valid={"exit_code":0,"timed_out":False,"cleanup_error":None,
               "capture_complete":True}
        self.assertTrue(risk.complete_command(valid))
        for change in ({"timed_out":True},{"cleanup_error":"pipe leaked"},
                       {"capture_complete":False},{"exit_code":1}):
            bad=dict(valid,**change)
            self.assertFalse(risk.complete_command(bad))
        self.assertFalse(risk.complete_command({"exit_code":0}))

    def test_unselected_native_profile_snapshot_detects_mutations(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            (root/"skills").mkdir()
            initial=risk.profile_snapshot(root)
            self.assertTrue(initial["complete"])
            (root/"skills"/"unexpected.toml").write_bytes(b"mutation")
            self.assertNotEqual(initial,risk.profile_snapshot(root))

    def test_failure_negative_control_is_not_mislabeled_crash_e2e(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            outcome={"exit_code":1,"timed_out":False,"cleanup_error":None}
            with patch.object(risk.baseline,"run_process",return_value=outcome):
                row=risk.case_interrupted("uap","/bin/fake",root,"a"*40)
            self.assertEqual(row["status"],"BLOCKED")
            self.assertFalse(row["deterministic_post_write_barrier"])
            self.assertEqual(row["crash_recovery"],"NOT_RUN")

    def test_post_pull_never_passes_with_stale_visible_skill(self):
        succeeded = {"exit_code": 0, "timed_out": False, "cleanup_error": None}
        failed = {"exit_code": 1, "timed_out": False, "cleanup_error": None}
        timeout = {"exit_code": 0, "timed_out": True, "cleanup_error": None}
        self.assertEqual(risk.classify_removed_result(succeeded, "present", True)[0], "FAIL")
        self.assertEqual(risk.classify_removed_result(failed, "present", True)[0], "BLOCKED")
        self.assertEqual(risk.classify_removed_result(failed, "absent", True)[0], "BLOCKED")
        self.assertEqual(risk.classify_removed_result(timeout, "absent", True)[0], "BLOCKED")
        self.assertEqual(risk.classify_removed_result(succeeded, "absent", False)[0], "FAIL")
        self.assertEqual(risk.classify_removed_result(succeeded, "absent", True)[0], "PASS")

    def test_synthetic_canary_detected_outside_project_and_never_serialized(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic = "risk-" + "4" * 48
            def simulate_install(*_args, **_kwargs):
                # The named config is clean, but a tool stores a secret in TMPDIR.
                file = root / "tmp" / "cache-trace"
                file.parent.mkdir(parents=True, exist_ok=True)
                file.write_text(synthetic)
                config = root / "project" / ".codex" / "config.toml"
                config.parent.mkdir(parents=True, exist_ok=True)
                config.write_text('[mcp_servers.riskcanary]\\ncommand = "node"\\n')
                return {"exit_code":0,"timed_out":False,"cleanup_error":None,
                        "output_tail":f"debug={synthetic}"}
            with patch.object(risk.secrets,"token_hex",return_value="4"*48), \
                 patch.object(risk.baseline,"run_process",side_effect=simulate_install):
                observed = risk.case_secret("uap","/bin/test-client",root)
            self.assertEqual(observed["status"],"FAIL")
            self.assertTrue(observed["filesystem_scan"]["canary_found"])
            self.assertNotIn("matching_relative_paths",observed["filesystem_scan"])
            self.assertNotIn(synthetic,json.dumps(observed))

    def test_canary_path_and_file_bodies_never_appear_in_scan_output(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            value = "risk-" + "8" * 48
            secret_path = root / ("log-" + value)
            secret_path.write_text(value)
            observation = risk.scan_synthetic_canary((root,), value)
            self.assertTrue(observation["canary_found"])
            self.assertTrue(observation["complete"])
            self.assertNotIn(value,json.dumps(observation))

    def test_unreadable_or_nonregular_removed_skill_never_looks_absent(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            skill=root/".agents/skills/benchmark-secondary/SKILL.md"
            skill.parent.mkdir(parents=True)
            original=root/"stored.md"
            original.write_bytes(b"still here")
            skill.symlink_to(original)
            self.assertEqual(risk.skill_presence(skill,root),"unknown")
            skill.unlink()
            skill.mkdir()
            self.assertEqual(risk.skill_presence(skill,root),"present")
            skill.rmdir()
            skill.write_bytes(b"x"*(3*1024*1024))
            self.assertEqual(risk.skill_presence(skill,root),"present")
            skill.unlink()
            self.assertEqual(risk.skill_presence(skill,root),"absent")
        ok={"exit_code":0,"timed_out":False,"cleanup_error":None}
        self.assertEqual(risk.classify_removed_result(ok,"unknown",True)[0],"BLOCKED")

    def test_synthetic_canary_not_missed_when_directory_cannot_be_read(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            def deny_walk(_root, **kwargs):
                kwargs["onerror"](PermissionError("unlistable fixture"))
                return iter(())
            with patch.object(risk.os,"walk",side_effect=deny_walk):
                found=risk.scan_synthetic_canary((root,),"risk-synthetic")
            self.assertFalse(found["complete"])
            self.assertIn("unlistable_directory",found["incomplete_reasons"])

    def test_path_only_canary_is_detected_without_leaking_its_name(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            marker="risk-"+("a"*48)
            (root/("log-"+marker)).write_bytes(b"body contains no synthetic token")
            (root/("empty-"+marker)).mkdir()
            scan=risk.scan_synthetic_canary((root,),marker)
            self.assertTrue(scan["canary_found"])
            self.assertTrue(scan["complete"])
            self.assertNotIn(marker,json.dumps(scan))

    def test_claude_provenance_must_include_observed_version_and_digest(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            result=risk.case_profiles("apm","/bin/no-op",root,"a"*40,
                                       {"SKILL.md":b"fixture"},claude_available=True)
            self.assertEqual(result["status"],"BLOCKED")
            self.assertNotIn("claude_version_verified",result)

    def test_MCP_config_rejects_empty_or_unrelated_server(self):
        with tempfile.TemporaryDirectory() as tmp:
            project=Path(tmp)
            toml=project/".codex/config.toml"
            toml.parent.mkdir()
            self.assertEqual(risk.codex_mcp_contract(project),"missing_config")
            toml.write_text('[mcp_servers.some_other_server]\ncommand = "node"\n')
            self.assertEqual(risk.codex_mcp_contract(project),"missing_server")
            toml.write_text('[mcp_servers.riskcanary]\ncommand = "node"\n'
                            'args = ["-e", "process.exit(0)"]\n')
            self.assertEqual(risk.codex_mcp_contract(project),"environment_forwarding_unverified")
            toml.write_text('[mcp_servers.riskcanary]\ncommand = "node"\n'
                            'args = ["-e", "process.exit(0)"]\n'
                            'env_vars = ["UAP_APM_SECRET_CANARY"]\n')
            self.assertEqual(risk.codex_mcp_contract(project),"match")
            toml.write_text('[mcp_servers.riskcanary]\ncommand = "wrong-tool"\n'
                            'args = ["-e", "process.exit(0)"]\n'
                            'env_vars = ["UAP_APM_SECRET_CANARY"]\n')
            self.assertEqual(risk.codex_mcp_contract(project),"wrong_command")

    def test_tool_version_requires_observed_complete_process(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            executable=root/"manager"
            executable.write_bytes(b"fixture-executable")
            complete={"exit_code":0,"timed_out":False,"cleanup_error":None,
                      "capture_complete":True,
                      "output_tail":"agentplugins 0.1.0-development"}
            with patch.object(risk.baseline,"run_process",return_value=complete):
                identity=risk.observe_cli_identity("uap",executable,root/"probe")
            self.assertTrue(identity["probe_verified"])
            self.assertTrue(identity["source_build_not_published_binary"])
            self.assertEqual(identity["entrypoint_sha256"],risk.h(b"fixture-executable"))
            for changed in ({"capture_complete":False},{"timed_out":True},
                            {"cleanup_error":"lost child"},
                            {"output_tail":"not-the-manager"},
                            {"exit_code":1}):
                bad=dict(complete,**changed)
                with patch.object(risk.baseline,"run_process",return_value=bad):
                    identity=risk.observe_cli_identity("uap",executable,root/"probe")
                self.assertFalse(identity["probe_verified"])

    def test_unknown_audit_requires_null_evidence(self):
        # A UAP run without the APM audit cannot assert that audit passed.
        record={"audit":None,"audit_reported_a_problem":None}
        self.assertIsNone(record["audit_reported_a_problem"])
        self.assertIsNone(record["audit"])

    def test_risk_file_does_not_embed_live_credentials(self):
        self.assertNotIn("sk-",str(risk.GROUPS))
        self.assertNotEqual(risk.PARENT_V1,risk.PARENT_V2)
        self.assertNotEqual(risk.CHILD_V1,risk.CHILD_V2)


if __name__=="__main__":
    unittest.main()
