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
        complete={"exit_code":0,"timed_out":False,"cleanup_error":None,
                  "capture_complete":True}
        self.assertEqual(risk.status(True,complete,False,"wrong bytes")[0],"FAIL")
        self.assertEqual(risk.status(True,complete,True,"unproven lock",
                                     provenance=False)[0],"FAIL")
        self.assertEqual(risk.status(True,complete,True,"foreign changed",
                                     protected=False)[0],"FAIL")
        self.assertEqual(risk.status(True,{"exit_code":0},True,"unobserved pipe")[0],"BLOCKED")

    def test_safe_refusal_vs_verified_loss(self):
        refusal={"exit_code":1,"timed_out":False,"cleanup_error":None,
                 "capture_complete":True}
        self.assertTrue(risk.observed_command(refusal))
        self.assertFalse(risk.complete_command(refusal))
        self.assertEqual(risk.status(True,refusal,True,"refusal")[0],"BLOCKED")
        self.assertEqual(risk.status(True,refusal,False,"bytes deleted")[0],"FAIL")
        incomplete=dict(refusal,capture_complete=False)
        self.assertFalse(risk.observed_command(incomplete))
        self.assertEqual(risk.status(True,incomplete,False,"unknown state")[0],"BLOCKED")
        timed_out=dict(refusal,exit_code=0,timed_out=True)
        self.assertEqual(risk.status(True,timed_out,True,"late success")[0],"BLOCKED")
        self.assertEqual(risk.status(False,refusal,True,"unsupported")[0],"BLOCKED")

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
                "commit":risk.PARENT_V1,"requested":{"kind":"commit","value":risk.PARENT_V1}}}}
            child={"alias":"risk-child","source":{"git":{
                "repository":risk.REPO,"path":risk.CHILD,
                "commit":risk.CHILD_V1,"requested":{"kind":"commit","value":risk.CHILD_V1}}},
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

    def test_graph_lock_symlinks_never_certify_source_identity(self):
        with tempfile.TemporaryDirectory() as tmp:
            parent=Path(tmp)
            project=parent/"project"
            project.mkdir()
            p={"alias":"parent","source":{"git":{
                "repository":risk.REPO,"path":risk.PARENT,
                "commit":risk.PARENT_V1,"requested":{
                    "kind":"commit","value":risk.PARENT_V1}}}}
            c={"alias":"child","source":{"git":{
                "repository":risk.REPO,"path":risk.CHILD,
                "commit":risk.CHILD_V1,"requested":{
                    "kind":"commit","value":risk.CHILD_V1}}},
                "required_by":[{"alias":"parent","selector":{
                    "kind":"commit","value":risk.CHILD_V1}}]}
            outside=parent/"real.lock"
            outside.write_text(json.dumps({"packages":[p,c]}))
            (project/"uap.lock").symlink_to(outside)
            self.assertFalse(risk.graph_lock_matches(
                project,"uap",risk.PARENT_V1,risk.CHILD_V1))
            (project/"uap.lock").unlink()
            (project/"uap.lock").write_bytes(outside.read_bytes())
            self.assertTrue(risk.graph_lock_matches(
                project,"uap",risk.PARENT_V1,risk.CHILD_V1))
            apm={"dependencies":[
                {"repo_url":risk.REPO,"virtual_path":risk.PARENT,
                 "resolved_commit":risk.PARENT_V1},
                {"repo_url":risk.REPO,"virtual_path":risk.CHILD,
                 "resolved_commit":risk.CHILD_V1,"resolved_by":risk.REPO}]}
            outside.write_text(json.dumps(apm))
            (project/"apm.lock.yaml").symlink_to(outside)
            self.assertFalse(risk.graph_lock_matches(
                project,"apm",risk.PARENT_V1,risk.CHILD_V1))

    def test_graph_initial_install_never_hides_foreign_mode_change(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            def mutate_first(*_args, **_kwargs):
                foreign=root/"project/.agents/skills/foreign-neighbor/SKILL.md"
                foreign.chmod(0o400)
                return {"exit_code":0,"timed_out":False,"cleanup_error":None,
                        "capture_complete":True}
            with patch.object(risk.baseline,"run_process",side_effect=mutate_first):
                row=risk.case_graph("apm","/bin/no-op",root)
            self.assertEqual(row["status"],"FAIL")
            self.assertEqual(row["stage"],"initial_foreign_modified")

    def test_graph_pin_refresh_never_hides_foreign_inode_replacement(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            operations=[]
            def mutate_refresh(*_args, **_kwargs):
                operations.append(True)
                project=root/"project"
                if len(operations)==1:
                    for name in ("risk-child","risk-parent"):
                        p=project/f".agents/skills/{name}/SKILL.md"
                        p.parent.mkdir(parents=True,exist_ok=True)
                        p.write_bytes(b"v1")
                if len(operations)==2:
                    foreign=project/".agents/skills/foreign-neighbor/SKILL.md"
                    contents=foreign.read_bytes()
                    # Build the replacement while the original inode
                    # remains allocated, then atomically swap identical bytes.
                    # Unlink-and-recreate alone can reuse the same inode.
                    replacement=foreign.with_name("replacement-for-inode-test")
                    replacement.write_bytes(contents)
                    replacement.replace(foreign)
                return {"exit_code":0,"timed_out":False,"cleanup_error":None,
                        "capture_complete":True}
            with patch.object(risk.baseline,"run_process",side_effect=mutate_refresh), \
                 patch.object(risk,"fixture",side_effect=lambda commit,_: (
                     b"v1" if commit in (risk.CHILD_V1,risk.PARENT_V1) else b"v2")), \
                 patch.object(risk,"graph_lock_matches",return_value=True):
                row=risk.case_graph("uap","/bin/no-op",root)
            self.assertEqual(row["status"],"FAIL")
            self.assertEqual(row["stage"],"pin_refresh_foreign_modified")
            self.assertEqual(len(operations),2)

    def test_UAP_graph_rejects_stale_requested_pins(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = Path(tmp)
            def locked(commit, requested, path, alias, required_by=None):
                node = {"alias": alias, "source": {"git": {
                    "repository": risk.REPO, "path": path, "commit": commit,
                    "requested": {"kind": "commit", "value": requested},
                }}}
                if required_by is not None:
                    node["required_by"] = required_by
                return node
            parent = locked(risk.PARENT_V2, risk.PARENT_V2, risk.PARENT, "parent")
            child = locked(risk.CHILD_V2, risk.CHILD_V2, risk.CHILD, "child",
                           [{"alias": "parent", "selector": {
                               "kind": "commit", "value": risk.CHILD_V2}}])
            path = project / "uap.lock"
            data = {"packages": [parent, child]}
            path.write_text(json.dumps(data))
            self.assertTrue(risk.graph_lock_matches(
                project, "uap", risk.PARENT_V2, risk.CHILD_V2))
            # An inconsistent lock with correct source coordinates but stale
            # consumer intent must not receive a false full-graph PASS.
            child["source"]["git"]["requested"]["value"] = risk.CHILD_V1
            path.write_text(json.dumps(data))
            self.assertFalse(risk.graph_lock_matches(
                project, "uap", risk.PARENT_V2, risk.CHILD_V2))
            child["source"]["git"]["requested"]["value"] = risk.CHILD_V2
            parent["source"]["git"]["requested"]["value"] = risk.PARENT_V1
            path.write_text(json.dumps(data))
            self.assertFalse(risk.graph_lock_matches(
                project, "uap", risk.PARENT_V2, risk.CHILD_V2))
            parent["source"]["git"].pop("requested")
            path.write_text(json.dumps(data))
            self.assertFalse(risk.graph_lock_matches(
                project, "uap", risk.PARENT_V2, risk.CHILD_V2))

    def test_UAP_graph_rejects_reverse_edges_and_same_alias(self):
        with tempfile.TemporaryDirectory() as tmp:
            project=Path(tmp)
            parent={"alias":"parent","source":{"git":{
                "repository":risk.REPO,"path":risk.PARENT,
                "commit":risk.PARENT_V1,"requested":{"kind":"commit","value":risk.PARENT_V1}}}}
            child={"alias":"child","source":{"git":{
                "repository":risk.REPO,"path":risk.CHILD,
                "commit":risk.CHILD_V1,"requested":{"kind":"commit","value":risk.CHILD_V1}}},
                "required_by":[{"alias":"parent","selector":{
                    "kind":"commit","value":risk.CHILD_V1}}]}
            data={"packages":[parent,child]}
            def write():
                (project/"uap.lock").write_text(json.dumps(data))
            write()
            self.assertTrue(risk.graph_lock_matches(
                project,"uap",risk.PARENT_V1,risk.CHILD_V1))
            parent["required_by"]=[{"alias":"child","selector":{
                "kind":"commit","value":risk.PARENT_V1}}]
            write()
            self.assertFalse(risk.graph_lock_matches(
                project,"uap",risk.PARENT_V1,risk.CHILD_V1))
            del parent["required_by"]
            child["alias"]="parent"
            write()
            self.assertFalse(risk.graph_lock_matches(
                project,"uap",risk.PARENT_V1,risk.CHILD_V1))

    def test_incomplete_prerequisite_zero_exit_rejected_before_install(self):
        success={"exit_code":0,"timed_out":False,"cleanup_error":None,
                 "capture_complete":True}
        for bad in ({"capture_complete":False},{"cleanup_error":"lost pipe"},
                    {"timed_out":True},{"exit_code":1}):
            command=dict(success,**bad)
            self.assertFalse(risk.complete_command(command))
            self.assertEqual(risk.classify_removed_result(
                command,"absent",True)[0],"BLOCKED")
            self.assertEqual(risk.status(True,command,True,
                                         "child not verified")[0],"BLOCKED")
        self.assertTrue(risk.complete_command(success))

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
            same_bytes=root/"skills"/"unexpected.toml"
            before=risk.profile_snapshot(root)
            same_bytes.chmod(0o400)
            self.assertNotEqual(before,risk.profile_snapshot(root))
            same_bytes.chmod(0o600)
            before=risk.profile_snapshot(root)
            identical_content=same_bytes.read_bytes()
            same_bytes.unlink()
            same_bytes.write_bytes(identical_content)
            self.assertNotEqual(before,risk.profile_snapshot(root))

    def test_cold_clone_file_identity_includes_metadata(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            file=root/"committed.md"
            file.write_bytes(b"committed exact bytes")
            before=risk.file_identity(file,root)
            self.assertIsNotNone(before)
            file.chmod(0o400)
            self.assertEqual(file.read_bytes(),b"committed exact bytes")
            self.assertNotEqual(before,risk.file_identity(file,root))

    def test_failure_negative_control_is_not_mislabeled_crash_e2e(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            outcome={"exit_code":1,"timed_out":False,"cleanup_error":None}
            with patch.object(risk.baseline,"run_process",return_value=outcome):
                row=risk.case_interrupted("uap","/bin/fake",root,"a"*40)
            self.assertEqual(row["status"],"BLOCKED")
            self.assertFalse(row["deterministic_post_write_barrier"])
            self.assertEqual(row["crash_recovery"],"NOT_RUN")

    def test_unobserved_preflight_never_claims_negative_gate_passed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            incomplete={"exit_code":1,"timed_out":False,"cleanup_error":None,
                        "capture_complete":False}
            with patch.object(risk.baseline,"run_process",return_value=incomplete):
                row=risk.case_interrupted("uap","/bin/fake",root,"a"*40)
            self.assertEqual(row["status"],"BLOCKED")
            self.assertIn("incomplete",row["reason"])
            complete=dict(incomplete,capture_complete=True,
                          output_tail="error: invalid uap.lock syntax")
            # Each negative-control run owns a fresh fixture. Reusing the
            # same root would collide with the prior foreign-neighbor dir.
            with patch.object(risk.baseline,"run_process",return_value=complete):
                row=risk.case_interrupted("uap","/bin/fake",root/"complete","a"*40)
            self.assertEqual(row["status"],"BLOCKED")
            self.assertIn("verified preflight refusal",row["reason"])
            self.assertFalse(row["deterministic_post_write_barrier"])

    def test_residual_removed_skill_directory_is_not_absent(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            directory=root/".agents/skills/benchmark-secondary"
            directory.mkdir(parents=True)
            (directory/"references.txt").write_bytes(b"orphaned reference")
            self.assertEqual(risk.skill_presence(directory,root),"present")
            self.assertEqual(risk.skill_presence(directory/"SKILL.md",root),"absent")
            self.assertEqual(risk.classify_removed_result(
                {"exit_code":0,"timed_out":False,"cleanup_error":None,
                 "capture_complete":True},
                "present",True)[0],"FAIL")

    def test_post_pull_never_passes_with_stale_visible_skill(self):
        succeeded = {"exit_code": 0, "timed_out": False, "cleanup_error": None,
                     "capture_complete": True}
        failed = {"exit_code": 1, "timed_out": False, "cleanup_error": None,
                  "capture_complete": True}
        timeout = {"exit_code": 0, "timed_out": True, "cleanup_error": None,
                   "capture_complete": True}
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
        ok={"exit_code":0,"timed_out":False,"cleanup_error":None,
            "capture_complete":True}
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

    def test_canary_common_text_encodings_are_detected_without_exposing_value(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            value="risk-"+("c"*48)
            file=root/"encoded-trace"
            for data in (risk.base64.b64encode(value.encode()),
                         value.encode().hex().encode(), value.upper().encode()):
                file.write_bytes(data)
                observed=risk.scan_synthetic_canary((root,),value)
                self.assertTrue(observed["canary_found"])
                self.assertTrue(observed["complete"])
                self.assertNotIn(value,json.dumps(observed))

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

    def test_corrupt_lock_control_rejects_signals_and_unrelated_failure(self):
        valid={"exit_code":1, "timed_out":False, "cleanup_error":None,
               "capture_complete":True, "output_tail":"Error: invalid uap.lock syntax"}
        self.assertTrue(risk.lock_validation_refused(valid,"uap.lock"))
        self.assertTrue(risk.lock_validation_refused(
            dict(valid,output_tail="apm.lock.yaml: parse error near line 1"),
            "apm.lock.yaml"))
        for other in (
            dict(valid,exit_code=0),
            dict(valid,exit_code=-11),
            dict(valid,exit_code=-9),
            dict(valid,cleanup_error="pipe left open"),
            dict(valid,timed_out=True),
            dict(valid,capture_complete=False),
            dict(valid,output_tail="error: unknown flag in source package"),
            dict(valid,output_tail="warning: invalid uap.lock syntax\nerror: target unavailable"),
            dict(valid,output_tail="[WARN] malformed uap.lock\nerror: no detected Codex"),
            dict(valid,output_tail="uap.lock parsed successfully"),
        ):
            self.assertFalse(risk.lock_validation_refused(other,"uap.lock"))

    def test_corrupt_lock_refusal_keeps_allowlisted_basename_not_private_path(self):
        observed={"exit_code":1,"timed_out":False,"cleanup_error":None,
                  "capture_complete":True,
                  "output_tail":"error: failed to parse /tmp/private/uap.lock: unexpected token"}
        self.assertTrue(risk.lock_validation_refused(observed,"uap.lock"))
        self.assertTrue(risk.lock_validation_refused(
            dict(observed,output_tail=r"C:\private\apm.lock.yaml: syntax error"),
            "apm.lock.yaml"))
        self.assertFalse(risk.lock_validation_refused(
            dict(observed,output_tail="failed opening /tmp/corrupt/uap.lock: file missing"),
            "uap.lock"))

    def test_tool_prefixed_and_structured_warnings_cannot_prove_lock_refusal(self):
        valid={"exit_code":1,"timed_out":False,"cleanup_error":None,
               "capture_complete":True,"output_tail":"error: invalid uap.lock syntax"}
        self.assertTrue(risk.lock_validation_refused(valid,"uap.lock"))
        for message in (
            "agentplugins: WARNING malformed uap.lock\nerror: target unavailable",
            '{"level":"warning","message":"invalid uap.lock"}\nerror: target unavailable',
            "2026-11-01 INFO cannot parse uap.lock\nerror: unexpected provider state",
        ):
            self.assertFalse(risk.lock_validation_refused(
                {**valid,"output_tail":message},"uap.lock"))

    def test_survivor_directory_entry_sets_reject_added_files(self):
        with tempfile.TemporaryDirectory() as tmp:
            project=Path(tmp)
            skill=project/"surviving"
            ref=skill/"references"
            ref.mkdir(parents=True)
            (skill/"SKILL.md").write_bytes(b"original")
            (ref/"fixture.txt").write_bytes(b"reference")
            initial=risk.directory_identity(skill,project,with_entries=True)
            reference_initial=risk.directory_identity(ref,project,with_entries=True)
            self.assertIsNotNone(initial)
            self.assertIsNotNone(reference_initial)
            (skill/"foreign-extra.md").write_bytes(b"not expected")
            self.assertNotEqual(initial,risk.directory_identity(skill,project,with_entries=True))
            (skill/"foreign-extra.md").unlink()
            (ref/"untracked.txt").write_bytes(b"unexpected nested file")
            self.assertNotEqual(reference_initial,risk.directory_identity(ref,project,with_entries=True))

    def test_cold_claude_requires_actual_native_skill_package_kind(self):
        import yaml
        commit = risk.PARENT_V2
        def portable(kind):
            lock = {"dependencies": [{
                "repo_url": risk.REPO, "virtual_path": risk.SKILL,
                "resolved_commit": commit, "package_type": kind,
            }]}
            return {"apm.lock.yaml": yaml.safe_dump(lock).encode()}
        self.assertTrue(risk.apm_claude_skill_kind(portable("claude_skill"), commit))
        self.assertTrue(risk.apm_claude_skill_kind(portable("skill_bundle"), commit))
        for kind in ("apm_package", "marketplace_plugin", "", None):
            self.assertFalse(risk.apm_claude_skill_kind(portable(kind), commit))
        self.assertFalse(risk.apm_claude_skill_kind(portable("claude_skill"), risk.PARENT_V1))
        self.assertFalse(risk.apm_claude_skill_kind({}, commit))

    def test_Claude_version_requires_exact_numeric_token(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for version in ("12.1.2634", "2.1.2634", "12.1.263",
                            "2.1.2630", "not-the-native-client"):
                row = risk.case_profiles(
                    "apm", "/bin/unavailable", root, "a" * 40,
                    {"SKILL.md": b"fixture", "references/fixture.txt": b"ref"},
                    claude_available=True, claude_version=version,
                    claude_native_sha256="26d020351e8112f4006790f3cfce43b4c9df0c1bb1d0e542364d64151b81d5ba",
                )
                self.assertEqual(row["status"], "BLOCKED")
                self.assertIn("pinned real Claude", row["reason"])

    def test_CLAUDE_identity_cannot_be_attested_just_by_caller_values(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            with patch.object(risk.shutil,"which",return_value=None):
                row=risk.case_profiles(
                    "apm","/bin/unused",root,"a"*40,
                    {"SKILL.md":b"x","references/fixture.txt":b"y"},
                    claude_available=True,claude_version="2.1.263 (Claude Code)",
                    claude_native_sha256="26d020351e8112f4006790f3cfce43b4c9df0c1bb1d0e542364d64151b81d5ba",
                )
            self.assertEqual(row["status"],"BLOCKED")
            self.assertIn("actual selected Claude",row["reason"])

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
            toml.write_text('[mcp_servers.riskcanary]\ncommand = "node"\n'
                            'args = ["-e", "process.exit(0)"]\n'
                            'env_vars = ["UAP_APM_SECRET_CANARY"]\n'
                            '[mcp_servers.riskcanary.env]\nOTHER_TOKEN = "embedded"\n')
            self.assertEqual(risk.codex_mcp_contract(project),"environment_forwarding_unverified")
            toml.write_text('[mcp_servers.riskcanary]\ncommand = "node"\n'
                            'args = ["-e", "process.exit(0)"]\n'
                            'env_vars = ["UAP_APM_SECRET_CANARY"]\n'
                            'env = ["unexpected"]\n')
            self.assertEqual(risk.codex_mcp_contract(project),"environment_forwarding_unverified")
            toml.write_text('[mcp_servers.riskcanary]\ncommand = "node"\n'
                            'args = ["-e", "process.exit(0)"]\n'
                            'env_vars = ["UAP_APM_SECRET_CANARY", "UNRELATED_ENV"]\n')
            self.assertEqual(risk.codex_mcp_contract(project),"environment_forwarding_unverified")
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
            false_substring=dict(complete,output_tail="wrapper "+complete["output_tail"]+" and other text")
            with patch.object(risk.baseline,"run_process",return_value=false_substring):
                identity=risk.observe_cli_identity("uap",executable,root/"probe")
            self.assertFalse(identity["probe_verified"])
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

    def test_removed_unknown_is_not_reported_as_confirmed_orphan(self):
        for observed, expected in (("present", True), ("absent", False),
                                   ("unknown", None)):
            self.assertEqual(
                {"present": True, "absent": False}.get(observed), expected)
            completed = {"exit_code": 0, "timed_out": False,
                         "cleanup_error": None, "capture_complete": True}
            if observed == "unknown":
                self.assertEqual(
                    risk.classify_removed_result(completed, observed, True)[0],
                    "BLOCKED")

    def test_risk_file_does_not_embed_live_credentials(self):
        self.assertNotIn("sk-",str(risk.GROUPS))
        self.assertNotEqual(risk.PARENT_V1,risk.PARENT_V2)
        self.assertNotEqual(risk.CHILD_V1,risk.CHILD_V2)



    def test_lock_control_rejects_fixture_path_words_without_real_diagnostic(self):
        control={"exit_code":1,"timed_out":False,"cleanup_error":None,
                 "capture_complete":True,
                 "output_tail":"failed opening /tmp/corrupt-lock-clone/uap.lock: file missing"}
        self.assertFalse(risk.lock_validation_refused(control,"uap.lock"))
        control["output_tail"]="uap.lock: invalid syntax"
        self.assertTrue(risk.lock_validation_refused(control,"uap.lock"))

    def test_project_snapshot_detects_unaccounted_user_files_but_ignores_manager_store(self):
        with tempfile.TemporaryDirectory() as temp:
            project=Path(temp)
            first=risk.project_file_snapshot(project)
            (project/".uap").mkdir()
            (project/".uap"/"state.json").write_text('{"owned": true}')
            second=risk.project_file_snapshot(project)
            self.assertTrue(first["complete"])
            self.assertEqual(first,second)
            (project/"AGENTS.md").write_text("unexpected generated instructions")
            third=risk.project_file_snapshot(project)
            self.assertNotEqual(second["sha256"],third["sha256"])

    def test_project_snapshot_detects_directory_mode_and_inode_change(self):
        with tempfile.TemporaryDirectory() as temp:
            project=Path(temp)
            folder=project/".agents"/"skills"
            folder.mkdir(parents=True)
            before=risk.project_file_snapshot(project)
            self.assertTrue(before["complete"])
            folder.chmod(0o700)
            changed=risk.project_file_snapshot(project)
            self.assertNotEqual(before["sha256"],changed["sha256"])
            # Same directory name and mode, but replaced inode, must be seen.
            replacement=project/".agents"/"replacement"
            replacement.mkdir()
            # Match the original directory's permissions so the comparison
            # requires inode evidence, not a coincidental mode difference.
            replacement.chmod(folder.stat().st_mode & 0o777)
            folder.rmdir()
            replacement.rename(folder)
            swapped=risk.project_file_snapshot(project)
            self.assertTrue(changed["complete"])
            self.assertTrue(swapped["complete"])
            self.assertNotEqual(changed["sha256"],swapped["sha256"])

    def test_graph_foreign_directory_mode_mutation_fails_before_admission(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            def mutate_first(*_args,**_kwargs):
                directory=root/"project/.agents/skills/foreign-neighbor"
                old=directory.stat().st_mode & 0o777
                directory.chmod(0o700 if old != 0o700 else 0o750)
                return {"exit_code":0,"timed_out":False,"cleanup_error":None,
                        "capture_complete":True}
            with patch.object(risk.baseline,"run_process",side_effect=mutate_first):
                row=risk.case_graph("apm","/bin/no-op",root)
            self.assertEqual(row["status"],"FAIL")
            self.assertEqual(row["stage"],"initial_foreign_modified")

    def test_graph_bounded_second_observation_detects_late_writer(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            calls=[]
            def run_graph(*_args,**_kwargs):
                calls.append(True)
                project=root/"project"
                version=b"v1" if len(calls)==1 else b"v2"
                for name in ("risk-child","risk-parent"):
                    p=project/f".agents/skills/{name}/SKILL.md"
                    p.parent.mkdir(parents=True,exist_ok=True)
                    p.write_bytes(version)
                return {"exit_code":0,"timed_out":False,"cleanup_error":None,
                        "capture_complete":True}
            def delayed_change(_seconds):
                path=root/"project/.agents/skills/foreign-neighbor/SKILL.md"
                path.chmod(0o400)
            with patch.object(risk.baseline,"run_process",side_effect=run_graph), \
                 patch.object(risk,"fixture",side_effect=lambda commit,_: (
                     b"v1" if commit in (risk.CHILD_V1,risk.PARENT_V1) else b"v2")), \
                 patch.object(risk,"graph_lock_matches",return_value=True), \
                 patch.object(risk.time,"sleep",side_effect=delayed_change):
                row=risk.case_graph("apm","/bin/no-op",root)
            self.assertEqual(row["status"],"FAIL")
            self.assertFalse(row["foreign_unchanged"])
            self.assertEqual(row["detached_writer_containment"],"NOT_RUN")

    def test_audit_crash_is_unknown_not_proven_clean(self):
        complete={"exit_code":-11,"timed_out":False,"cleanup_error":None,
                  "capture_complete":True,"output_tail":"Segmentation fault"}
        self.assertIsNone(risk.audit_problem_observed(complete))
        self.assertFalse(risk.audit_problem_observed(dict(complete,exit_code=0)))
        self.assertTrue(risk.audit_problem_observed(
            dict(complete,exit_code=1,output_tail="Drift detected: orphaned")))

    def test_directory_identity_catches_permissions_without_filename_changes(self):
        with tempfile.TemporaryDirectory() as temp:
            project=Path(temp)
            user_dir=project/"foreign"
            user_dir.mkdir()
            before=risk.directory_identity(user_dir,project)
            user_dir.chmod(0o700 if (user_dir.stat().st_mode & 0o777)!=0o700 else 0o750)
            self.assertNotEqual(before,risk.directory_identity(user_dir,project))
            self.assertIsNone(risk.directory_identity(project/"missing",project))

    def test_project_snapshot_detects_project_root_permission_mutation(self):
        with tempfile.TemporaryDirectory() as tmp:
            project=Path(tmp)/"project"
            project.mkdir()
            before=risk.project_file_snapshot(project)
            self.assertTrue(before["complete"])
            permissions=project.stat().st_mode & 0o777
            project.chmod(0o700 if permissions != 0o700 else 0o750)
            after=risk.project_file_snapshot(project)
            self.assertTrue(after["complete"])
            self.assertNotEqual(before["sha256"],after["sha256"])

    def test_preflight_detects_original_lock_and_foreign_metadata_changes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            def mutate(*_args,**_kwargs):
                path=root/"project/.agents/skills/foreign-neighbor/SKILL.md"
                path.chmod(0o400)
                return {"exit_code":1,"timed_out":False,"cleanup_error":None,
                        "capture_complete":True,"output_tail":"invalid uap.lock syntax"}
            with patch.object(risk.baseline,"run_process",side_effect=mutate):
                result=risk.case_interrupted("uap","/bin/no-op",root,"a"*40)
            self.assertEqual(result["status"],"FAIL")
            self.assertFalse(result["preflight_identity_untouched"])

    def test_mcp_secret_scenario_rejects_foreign_metadata_change(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            def mutate(*_args,**_kwargs):
                path=root/"project/.agents/skills/foreign-neighbor/SKILL.md"
                path.chmod(0o400)
                return {"exit_code":1,"timed_out":False,"cleanup_error":None,
                        "capture_complete":True,"output_tail":"safely refused"}
            with patch.object(risk.baseline,"run_process",side_effect=mutate):
                result=risk.case_secret("uap","/bin/no-op",root)
            self.assertEqual(result["status"],"FAIL")
            self.assertFalse(result["foreign_unchanged"])

    def test_inert_mcp_config_does_not_prove_no_delayed_secret_writes(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp)
            def successful_install(*_args,**_kwargs):
                config=root/"project"/".codex"/"config.toml"
                config.parent.mkdir(parents=True,exist_ok=True)
                config.write_text(
                    '[mcp_servers.riskcanary]\ncommand = "node"\n'
                    'args = ["-e", "process.exit(0)"]\n'
                    'env_vars = ["UAP_APM_SECRET_CANARY"]\n')
                return {"exit_code":0,"timed_out":False,"cleanup_error":None,
                        "capture_complete":True,"output_tail":""}
            with patch.object(risk.baseline,"run_process",side_effect=successful_install):
                entry=risk.case_secret("apm","/bin/fake",root)
            self.assertEqual(entry["status"],"BLOCKED")
            self.assertEqual(entry["native_mcp_call"],"NOT_RUN")
            self.assertEqual(entry["filesystem_scan"]["other_transformations"],"NOT_RUN")
            self.assertFalse(entry["filesystem_scan"]["canary_found"])

    def test_audit_reporting_excludes_unrelated_failures_and_signals(self):
        report={"exit_code":1,"timed_out":False,"cleanup_error":None,
                "capture_complete":True,"output_tail":"Drift detected: one file"}
        self.assertTrue(risk.audit_problem_observed(report))
        for bad in (-11,-9,0):
            self.assertFalse(risk.audit_problem_observed({**report,"exit_code":bad}))
        self.assertFalse(risk.audit_problem_observed({**report,"output_tail":"unknown option"}))
        self.assertIsNone(risk.audit_problem_observed({**report,"capture_complete":False}))

if __name__=="__main__":
    unittest.main()
