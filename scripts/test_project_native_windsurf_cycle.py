"""Inert complete-ZIP and precise Windsurf lifecycle refusal contracts."""
import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import project_native_runtime as runtime
import project_native_windsurf_cycle as cycle
from windsurf_cycle_contract_fixture import build, wire


class WindsurfCycleContracts(unittest.TestCase):
    def setUp(self):
        self.commands,self.proofs,self.probes,self.result=build()
        self.source={"source_commit":"a"*40,"source_tree":"b"*40}

    def bodies(self):
        return {"runtime-attempt.json":cycle.compact(self.result),**{name:b"".join(cycle.compact(row)+b"\n" for row in rows) for name,rows in zip(cycle.MEMBERS,(self.commands,self.proofs,self.probes))}}

    def admit(self):
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/"research.zip"
            with zipfile.ZipFile(path,"w",zipfile.ZIP_DEFLATED) as archive:
                for name,raw in self.bodies().items():archive.writestr(name,raw)
            with patch.object(cycle,"pins"):
                return runtime.admit_runtime(path,"windsurf",self.source)[1]

    def test_actual_four_member_zip_crosses_dispatch_and_all_proofs(self):
        self.assertEqual(self.admit(),self.result)
        self.assertEqual(len(self.commands),47)
        self.assertTrue(all(len(raw)<=1<<20 for raw in self.bodies().values()))

    def test_typed_a2_proof_schema_source_policy_phase_hash_refusals(self):
        index=next(i for i,row in enumerate(self.proofs) if row["stage"]=="update-a")
        original=copy.deepcopy(self.proofs)
        changes=(lambda row:row["cache"].update(scope="b"),lambda row:row["cache"].update(phase="existing"),lambda row:row["cache"].update(key="0"*64),lambda row:row["cache"].update(extra=True),lambda row:row["cache"]["assessment"].update(extra=True),lambda row:row["cache"]["assessment"].update(sha256="0"*64),lambda row:row.update(source_digest="sha256:"+"0"*64))
        for change in changes:
            with self.subTest(change=change):
                self.proofs=copy.deepcopy(original);change(self.proofs[index]["value"])
                with self.assertRaises(ValueError):self.admit()
        self.proofs=original

    def test_a2_parent_replacement_cannot_renew_accepted_global_authority(self):
        active=False
        root=self.result["component_layout"]["root"]+"/installer-state"
        for row in self.proofs:
            if row["stage"]=="update-a":active=True
            if active and row["stage"] in cycle.PUBLIC:
                value=row["value"]
                value["frames"][root]["security/assessments"]["Inode"]+=1
                value["cache"]["before_parent"]["Inode"]+=1
                value["cache"]["after_parent"]["Inode"]+=1
        with self.assertRaises(ValueError):self.admit()

    def test_repeat_remove_cannot_refresh_unchanged_lock(self):
        value=next(row["value"] for row in self.proofs if row["stage"]=="repeat-remove-a")
        command=next(row["value"] for row in self.commands if row["stage"]=="repeat-remove-a")
        path=self.result["component_layout"]["a"]+"/.uap"
        lock=value["frames"][path]["mutation.lock"]
        raw=json.dumps({"pid":4321,"acquired_at":command["started_at"]},separators=(",",":")).encode()+b"\n"
        second=cycle.stamp(command["started_at"])//10**9
        lock.update(Size=len(raw),SHA256=cycle.sha(raw),Modified=[second,200000000],Changed=[second,200000000])
        value["frames"][self.result["component_layout"]["a"]][".uap/mutation.lock"]=copy.deepcopy(lock)
        value["mutation_lock"]=wire(raw)
        with self.assertRaises(ValueError):self.admit()

    def test_native_skill_revision_and_removal_are_same_frame_facts(self):
        original=copy.deepcopy(self.proofs)
        paths=self.result["component_layout"]
        profile=paths["a"]+"/.devin"
        row=next(row["value"] for row in self.proofs if row["stage"]=="update-a")
        key="skills/"+cycle.NAME+"/SKILL.md"
        row["frames"][profile][key]["SHA256"]="0"*64
        row["frames"][paths["a"]][".devin/"+key]["SHA256"]="0"*64
        with self.assertRaises(ValueError):self.admit()
        self.proofs=original
        prior=next(row["value"] for row in self.proofs if row["stage"]=="missing-repair-a")
        for proof in self.proofs:
            if proof["stage"] in ("remove-a","repeat-remove-a"):
                value=proof["value"]
                for key in ("skills/"+cycle.NAME,"skills/"+cycle.NAME+"/SKILL.md"):
                    value["frames"][profile][key]=copy.deepcopy(prior["frames"][profile][key])
                    value["frames"][paths["a"]][".devin/"+key]=copy.deepcopy(prior["frames"][profile][key])
        with self.assertRaises(ValueError):self.admit()

    def test_old_fixture_fault_cannot_claim_current_phase(self):
        fault=next(row["value"] for row in self.proofs if row["stage"]=="missing-owned-a-skill")
        fault.update(started_at="2000-01-01T00:00:00Z",ended_at="2000-01-01T00:00:01Z")
        with self.assertRaises(ValueError):self.admit()

    def test_security_must_be_exact_data_field_not_nested_output(self):
        row=self.commands[9]["value"]
        public=cycle.load(cycle.stream(row["stdout"]))
        security=public["data"].pop("security")
        public["data"]["targets"][0]["output"]["security"]=security
        row["stdout"]=wire(cycle.compact(public))
        with self.assertRaises(ValueError):self.admit()

    def test_whole_time_span_nanosecond_duration_and_summary_refuse(self):
        original=copy.deepcopy(self.commands)
        self.commands[-1]["value"]["ended_at"]="2026-10-30T15:00:46.8Z"
        with self.assertRaises(ValueError):self.admit()
        self.commands=copy.deepcopy(original)
        self.commands[0]["value"]["ended_at"]="2026-10-30T14:00:00.800000001Z"
        self.commands[0]["value"]["elapsed_ms"]=801
        with self.assertRaises(ValueError):self.admit()
        self.commands=original;self.result["elapsed_ms"]=100
        with self.assertRaises(ValueError):self.admit()

    def test_marker_authority_and_old_cache_renewal_refuse(self):
        original=copy.deepcopy(self.proofs)
        self.proofs[0]["value"]["after"]["caller-owned.txt"]["Mode"]=0o100644
        with self.assertRaises(ValueError):self.admit()
        self.proofs=original
        row=next(row["value"] for row in self.proofs if row["stage"]=="update-a")
        root=self.result["component_layout"]["root"]+"/installer-state"
        old=next(name for name in row["frames"][root] if name.startswith("security/assessments/") and name not in row["cache"]["path"])
        row["frames"][root][old]["Inode"]+=1
        with self.assertRaises(ValueError):self.admit()

    def test_noop_and_whole_catalog_controls_refuse(self):
        original=copy.deepcopy(self.commands)
        row=next(row["value"] for row in self.commands if row["stage"]=="updated-a-skills")
        catalog=cycle.load(cycle.stream(row["stdout"]));owned=next(v for v in catalog if v["name"]==cycle.NAME);owned["description"]="a-one"
        row["stdout"]=wire(cycle.compact(catalog))
        with self.assertRaises(ValueError):self.admit()
        self.commands=original
        row=next(row["value"] for row in self.commands if row["stage"]=="repeat-add-a")
        value=cycle.load(cycle.stream(row["stdout"]));value["data"]["result"].pop("no_change");row["stdout"]=wire(cycle.compact(value))
        with self.assertRaises(ValueError):self.admit()

    def test_per_record_privacy_keeps_nested_stream_and_node_limits(self):
        aggregate=self.bodies()[cycle.MEMBERS[1]]
        self.assertTrue(cycle.private(aggregate))  # Whole-member generic node budget is finite.
        self.assertFalse(cycle.aggregate_private(aggregate))
        self.assertTrue(cycle.aggregate_private(b'{"value":"uap-windsurf-private-secret"}\n'))
        value=wire(b"uap-windsurf-private-encoded-secret")
        self.assertFalse(cycle.aggregate_private(cycle.compact({"stage":"x","value":value})+b"\n"))
        with self.assertRaises(ValueError):cycle.stream(value)
        self.proofs[1]["value"]["data_marker"]=value
        with self.assertRaises(ValueError):self.admit()
        row={"value":list(range(20000))}
        self.assertTrue(cycle.aggregate_private(cycle.compact(row)+b"\n"))

    def test_failed_prefix_stays_all_false_and_cross_mode_refuses(self):
        self.commands=self.commands[:9];self.proofs=[];self.probes=self.probes[:9]
        self.result.update(runtime_attempt_complete=False,cycle_observed_complete=False,test_failed=True,attempt_stage="add-user",cycle_command_stages=[r["stage"] for r in self.commands],cycle_proof_stages=[],qualification={flag:False for flag in cycle.FLAGS})
        self.assertEqual(self.admit(),self.result)
        self.result["qualification"]["project-lifecycle"]=True
        with self.assertRaises(ValueError):self.admit()


if __name__=="__main__":unittest.main()
