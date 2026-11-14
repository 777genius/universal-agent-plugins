"""Inert closed Cline evidence checks, not native qualification."""
import copy
import hashlib
import json
from pathlib import Path
import sys
import unittest
sys.path.insert(0,str(Path(__file__).parent))
import cline_project_envelope as contract

def summary(flags):
    value={"schema_version":1,"client":"cline","version":"3.0.73","collection_complete":False,"commands":[],
           "source_commit":"a"*40,"source_tree":"b"*40}
    value.update({key:False for key in flags});return value

def command(label,kind,bodies,in_process=False):
    record={"label":label,"class":kind,"in_process":in_process,"exit":0,"error":"","overflow":False}
    for stream in ("stdout","stderr"):
        name=label+"."+stream+".log";body=b"inert" if stream=="stdout" else b"";bodies[name]=body
        record.update({stream+"_file":name,stream+"_bytes":len(body),stream+"_sha256":hashlib.sha256(body).hexdigest()})
    return record

def encoded(front,native,bodies):
    result=dict(bodies);result[contract.FRONTEND]=json.dumps(front).encode()
    if native is not None:result[contract.NATIVE]=json.dumps(native).encode()
    return result

class ClosedClineEnvelopeTests(unittest.TestCase):
    def fixture(self,complete=False):
        bodies={};front=summary(contract.FRONT_FALSE);native=summary(contract.NATIVE_FALSE)
        front["commands"]=[command(label,"frontend",bodies) for label in sorted(contract.FRONT_LABELS)]
        native["commands"]=[command(label,kind,bodies,kind=="public_uap") for label,kind in contract.NATIVE_LABELS.items()]
        native_names=set(bodies)-{row[stream+"_file"] for row in front["commands"] for stream in ("stdout","stderr")}
        for name in contract.EXTRAS:bodies[name]=b'{}'
        native["artifact_sha256"]={name:hashlib.sha256(bodies[name]).hexdigest() for name in native_names|set(contract.EXTRAS)}
        if complete:
            for value in (front,native):value.update(collection_complete=True,cleanup_complete=True,read_only_unique_marker_discovery_qualified=True)
            native["same_name_observation_complete"]=True
            native["stages"]={name:{"status":"blocked" if name=="same_name_precedence" else "passed"} for name in contract.STAGES}
        return front,native,bodies

    def test_complete_and_partial_fixed_bodies_are_admitted_without_rewriting(self):
        for complete in (False,True):
            front,native,bodies=self.fixture(complete);selected=encoded(front,native,bodies);before=copy.deepcopy(selected)
            contract.validate(selected);self.assertEqual(selected,before)
        contract.validate(encoded(summary(contract.FRONT_FALSE),None,{}))

    def test_unknown_profiles_labels_classes_and_process_claims_refuse(self):
        for kind in ("unknown-member","unknown-label","duplicate-label","wrong-class","wrong-boundary"):
            front,native,bodies=self.fixture()
            if kind=="unknown-member":bodies["native-cache.json"]=b'{}'
            elif kind=="unknown-label":native["commands"][0]["label"]="private-profile"
            elif kind=="duplicate-label":native["commands"].append(native["commands"][0])
            elif kind=="wrong-class":native["commands"][0]["class"]="native"
            else:native["commands"][0]["in_process"]=True
            with self.subTest(kind=kind),self.assertRaises(ValueError):contract.validate(encoded(front,native,bodies))

    def test_capture_hash_size_and_extra_authentication_refuse_drift(self):
        for kind in ("stream","extra","hash-membership","oversized","stream-name","stream-size-bool"):
            front,native,bodies=self.fixture();first=native["commands"][0];name=first["stdout_file"]
            if kind=="stream":bodies[name]+=b"changed"
            elif kind=="extra":bodies["prepared-A-config.json"]=b'{"changed":true}'
            elif kind=="hash-membership":native["artifact_sha256"]["foreign.json"]="0"*64
            elif kind=="oversized":bodies[name]=b"x"*((128<<10)+1)
            elif kind=="stream-name":first["stdout_file"]="profile.log"
            else:first["stdout_bytes"]=True
            with self.subTest(kind=kind),self.assertRaises(ValueError):contract.validate(encoded(front,native,bodies))

    def test_source_unobserved_flags_and_ambiguous_json_refuse(self):
        for kind in ("source","runtime","active","schema-bool","duplicate-json","nonfinite-json","missing-native"):
            front,native,bodies=self.fixture(True)
            if kind=="source":native["source_tree"]="c"*40
            elif kind=="runtime":native["native_runtime_qualified"]=True
            elif kind=="active":native["same_name_active_isolation_qualified"]=True
            elif kind=="schema-bool":front["schema_version"]=True
            selected=encoded(front,None if kind=="missing-native" else native,bodies)
            if kind=="duplicate-json":selected[contract.FRONTEND]=b'{"client":"cline","client":"cline"}'
            elif kind=="nonfinite-json":selected["prepared-A-config.json"]=b'{"number":NaN}'
            with self.subTest(kind=kind),self.assertRaises(ValueError):contract.validate(selected)

    def test_complete_requires_every_command_stage_source_file_and_cleanup(self):
        for kind in ("command","stage","extra","cleanup","empty","failed","true-exit"):
            front,native,bodies=self.fixture(True)
            if kind=="command":native["commands"].pop()
            elif kind=="stage":native["stages"]["same_name_precedence"]["status"]="passed"
            elif kind=="extra":bodies.pop("prepared-B-config.json")
            elif kind=="cleanup":front["cleanup_complete"]=False
            elif kind=="empty":native["read_only_unique_marker_discovery_qualified"]=False
            elif kind=="failed":front["commands"][0]["error"]="deadline"
            else:front["commands"][0]["exit"]=False
            with self.subTest(kind=kind),self.assertRaises((ValueError,KeyError)):contract.validate(encoded(front,native,bodies))

if __name__=="__main__":unittest.main()

class ActualClineArchiveTests(unittest.TestCase):
    """Actual filesystem/ZIP admission, with synthetic FALSE native claims."""
    def fixture(self,root):
        import native_client_project_artifacts as artifacts
        front,native,bodies=ClosedClineEnvelopeTests().fixture(False)
        source=root/'raw';source.mkdir()
        for name,body in encoded(front,native,bodies).items():(source/name).write_bytes(body)
        upload=root/'upload';upload.mkdir()
        return artifacts,source,upload/'proof.zip'

    def test_real_cline_archive_contains_only_original_flat_bytes(self):
        import tempfile,zipfile
        with tempfile.TemporaryDirectory() as temporary:
            artifacts,source,destination=self.fixture(Path(temporary).resolve())
            self.assertTrue(artifacts.write_archive(source,destination,client='cline'))
            with zipfile.ZipFile(destination) as archive:
                self.assertEqual(set(archive.namelist()),{p.name for p in source.iterdir()})
                for member in archive.infolist():
                    self.assertEqual(member.compress_type,zipfile.ZIP_STORED)
                    self.assertEqual(archive.read(member),(source/member.filename).read_bytes())
            self.assertLess(destination.stat().st_size,32<<20)

    def test_unknown_member_yields_only_false_cline_failure_not_opencode(self):
        import tempfile,zipfile
        with tempfile.TemporaryDirectory() as temporary:
            artifacts,source,destination=self.fixture(Path(temporary).resolve())
            (source/'private-profile.json').write_bytes(b'{}')
            self.assertFalse(artifacts.write_archive(source,destination,client='cline'))
            with zipfile.ZipFile(destination) as archive:
                self.assertEqual(archive.namelist(),[contract.NATIVE])
                failure=json.loads(archive.read(contract.NATIVE))
                self.assertFalse(failure['collection_complete'])
                for key in contract.NATIVE_FALSE:self.assertIs(failure[key],False)

    def test_unknown_client_and_alias_upload_parent_fail_before_publication(self):
        import os,tempfile
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();artifacts,source,destination=self.fixture(root)
            with self.assertRaises(ValueError):artifacts.write_archive(source,destination,client='arbitrary')
            foreign=root/'foreign';foreign.mkdir();alias=root/'alias';alias.symlink_to(foreign,target_is_directory=True)
            with self.assertRaises(ValueError):artifacts.write_archive(source,alias/'proof.zip',client='cline')
            self.assertFalse((foreign/'proof.zip').exists())

    def test_parent_replacement_after_zip_assembly_refuses_final_publication(self):
        import os,tempfile
        from unittest.mock import patch
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();artifacts,source,destination=self.fixture(root)
            original=artifacts.write_zip
            def replace(pending,bodies):
                original(pending,bodies)
                os.rename(destination.parent,root/'old-upload')
                destination.parent.mkdir()
            with patch.object(artifacts,'write_zip',side_effect=replace):
                with self.assertRaises(ValueError):artifacts.write_archive(source,destination,client='cline')
            self.assertFalse(destination.exists())
