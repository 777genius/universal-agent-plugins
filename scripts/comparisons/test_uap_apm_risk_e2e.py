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

    def test_graph_lock_does_not_accept_sha_in_unrelated_field(self):
        with tempfile.TemporaryDirectory() as tmp:
            project=Path(tmp)
            v=risk.CHILD_V2
            (project/"uap.lock").write_text(json.dumps({"packages":[{
                "source":{"git":{"path":risk.CHILD,"commit":risk.CHILD_V1}},
                "comment":v
            }]}))
            self.assertFalse(risk.child_lock(project,"uap",v))
            (project/"uap.lock").write_text(json.dumps({"packages":[{
                "source":{"git":{"path":risk.CHILD,"commit":v}}
            }]}))
            self.assertTrue(risk.child_lock(project,"uap",v))
            (project/"uap.lock").write_text(json.dumps({"packages":[{
                "source":{"git":{"path":"different/plugin","commit":v}}
            }]}))
            self.assertFalse(risk.child_lock(project,"uap",v))

    def test_profile_observer_does_not_admit_wrong_body_or_symlinks(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            skill=root/"skills"/"risk"
            skill.mkdir(parents=True)
            (skill/"SKILL.md").write_bytes(b"body")
            self.assertTrue(risk.profile_scan(root,b"body"))
            self.assertFalse(risk.profile_scan(root,b"different"))
            (skill/"SKILL.md").unlink()
            (skill/"SKILL.md").symlink_to(root/"other")
            (root/"other").write_bytes(b"body")
            self.assertFalse(risk.profile_scan(root,b"body"))

    def test_failure_negative_control_is_not_mislabeled_crash_e2e(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            outcome={"exit_code":1,"timed_out":False,"cleanup_error":None}
            with patch.object(risk.baseline,"run_process",return_value=outcome):
                row=risk.case_interrupted("uap","/bin/fake",root,"a"*40)
            self.assertEqual(row["status"],"BLOCKED")
            self.assertFalse(row["deterministic_post_write_barrier"])
            self.assertEqual(row["crash_recovery"],"NOT_RUN")

    def test_risk_file_does_not_embed_live_credentials(self):
        self.assertNotIn("sk-",str(risk.GROUPS))
        self.assertNotEqual(risk.PARENT_V1,risk.PARENT_V2)
        self.assertNotEqual(risk.CHILD_V1,risk.CHILD_V2)


if __name__=="__main__":
    unittest.main()
