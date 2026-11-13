"""Inert tests for the fixed historical Hermes archive reader."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import struct
import unittest
from unittest import mock
import zlib
import zipfile

spec = importlib.util.spec_from_file_location("reader", Path(__file__).with_name("hermes_original_archive_readout.py"))
reader = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reader)

class OriginalArchiveTests(unittest.TestCase):
    def fixture(self):
        case = reader.CASES["mac-b79"]
        files = {"lifecycle.stderr": b'Traceback (most recent call last):\n  File "/PRIVATE/hermes-profile-admission.py", line 123, in ordinary_bytes\nRuntimeError: inventory member refused: PRIVATE\n'}
        proof = dict(source={"commit": case[4], "tree": case[5], "dirty": ""},
                     hermes_source={"commit": reader.PIN, "tree": reader.PIN_TREE, "dirty": ""},
                     run_id=case[3], run_attempt="1", runner_os=case[6], status="failed",
                     cleanup="completed", runtime_distribution="bundled", error="RuntimeError: native profile observation failed")
        proof["source_after"], proof["hermes_source_after"] = proof["source"], proof["hermes_source"]
        proof.update({key: False for key in reader.FLAGS})
        proof["streams"] = {name: {"bytes": len(body), "sha256": hashlib.sha256(body).hexdigest()} for name, body in files.items()}
        files["profile-admission.json"] = json.dumps(proof).encode()
        return case, files, proof

    def packed(self, files):
        out = io.BytesIO()
        with zipfile.ZipFile(out, "w") as archive:
            for name, body in files.items():
                archive.writestr(name, body)
        return out.getvalue()

    def test_real_zip_crc_source_stream_join_and_private_projection(self):
        case, files, _ = self.fixture()
        body = self.packed(files)
        found = reader.unpack(body, len(body), hashlib.sha256(body).hexdigest())
        value = reader.project(found, case)
        self.assertEqual(value["traceback_frames"], [{"file": "hermes-profile-admission.py", "line": 123}])
        self.assertEqual(value["stream_guards"], ["inventory-member-refused"])
        self.assertNotIn("PRIVATE", json.dumps(value))
        self.assertFalse(value["native_qualified"])

    def test_wrong_digest_unknown_members_duplicate_and_crc_refuse(self):
        _, files, _ = self.fixture()
        body = self.packed(files)
        with self.assertRaises(ValueError):
            reader.unpack(body, len(body), "0" * 64)
        for bad in ({**files, "../private": b"x"}, {**files, "unknown": b"x"}):
            raw = self.packed(bad)
            with self.assertRaises(ValueError):
                reader.unpack(raw, len(raw), hashlib.sha256(raw).hexdigest())
        duplicate = io.BytesIO()
        with zipfile.ZipFile(duplicate, "w") as archive:
            archive.writestr("lifecycle.stdout", b"a")
            archive.writestr("lifecycle.stdout", b"b")
        raw_duplicate = duplicate.getvalue()
        with self.assertRaises(ValueError):
            reader.unpack(raw_duplicate, len(raw_duplicate), hashlib.sha256(raw_duplicate).hexdigest())
        raw = bytearray(body)
        offset = body.index(files["lifecycle.stderr"])
        raw[offset] ^= 1
        with self.assertRaises(zipfile.BadZipFile):
            reader.unpack(bytes(raw), len(raw), hashlib.sha256(raw).hexdigest())

    def test_eocd_bounds_precede_zipfile_and_comment_refuses(self):
        body = bytearray(self.packed({"lifecycle.stdout": b""}))
        struct.pack_into("<H", body, len(body) - 12, 65535)
        with self.assertRaises(ValueError):
            reader.unpack(bytes(body), len(body), hashlib.sha256(body).hexdigest())
        body = self.packed({"lifecycle.stdout": b""}) + b"private"
        with self.assertRaises(ValueError):
            reader.unpack(body, len(body), hashlib.sha256(body).hexdigest())

    def test_proof_source_flags_and_stream_membership_refuse(self):
        case, files, proof = self.fixture()
        for mutate in (lambda p: p.update(source_after={}), lambda p: p.update(capture_complete=True),
                       lambda p: p.update(streams={}), lambda p: p.update(run_id="0")):
            value = json.loads(json.dumps(proof))
            mutate(value)
            with self.assertRaises(ValueError):
                reader.project({**files, "profile-admission.json": json.dumps(value).encode()}, case)

    def test_unique_json_and_alias_closed_types(self):
        with self.assertRaises(ValueError):
            reader.document(b'{"x":1,"x":2}')
        member = "tools/cache/file"
        value = dict(schema_version=1, measurement_only=True, member=member,
                     member_sha256=hashlib.sha256(member.encode()).hexdigest(),
                     hash_encoding="utf-8-surrogatepass", device=1, inode=2, links=2,
                     captured_aliases=1, uid=0, gid=0, integer_saturated=False)
        projected = reader.alias(value)
        self.assertEqual(projected["member"], member)
        for bad in ({**value, "links": True}, {**value, "extra": "private"}, {**value, "member": "../private"}):
            with self.assertRaises(ValueError):
                reader.alias(bad)


    def test_actual_deflate_declared_prefix_and_trailing_data_refuse(self):
        data = b"actual complete expanded bytes"
        out = io.BytesIO()
        with zipfile.ZipFile(out, "w", compression=zipfile.ZIP_DEFLATED) as archive:
            archive.writestr("lifecycle.stdout", data)
        original = out.getvalue()
        self.assertEqual(reader.unpack(original, len(original), hashlib.sha256(original).hexdigest())["lifecycle.stdout"], data)
        central = original.index(b"PK\x01\x02")
        forged = bytearray(original)
        prefix = data[:3]
        for offset in (14, central + 16):
            struct.pack_into("<I", forged, offset, zlib.crc32(prefix))
        struct.pack_into("<I", forged, 22, len(prefix))
        struct.pack_into("<I", forged, central + 24, len(prefix))
        with self.assertRaises(ValueError):
            reader.unpack(bytes(forged), len(forged), hashlib.sha256(forged).hexdigest())
        with zipfile.ZipFile(io.BytesIO(original)) as archive:
            info = archive.infolist()[0]
            start = 30 + len(info.filename)
            reader.check_expansion(original, start, info)
            info.compress_size += 1
            with self.assertRaises(ValueError):
                reader.check_expansion(original, start, info)

    def test_runtime_second_failure_retained_without_raw_error(self):
        case, files, proof = self.fixture()
        proof.update(runtime_before={"private": 1}, runtime_after={"private": 2}, runtime_error="PRIVATE")
        value = reader.project({**files, "profile-admission.json": json.dumps(proof).encode()}, case)
        self.assertTrue(value["runtime_present"])
        self.assertFalse(value["runtime_stable"])
        self.assertTrue(value["runtime_error_present"])
        self.assertNotIn("PRIVATE", json.dumps(value))

    def test_cli_refusal_nonzero_and_fixed_output(self):
        from contextlib import redirect_stdout
        output = io.StringIO()
        with mock.patch("sys.argv", ["reader", "mac-b79", "missing"]), mock.patch.object(Path, "open", side_effect=OSError("PRIVATE")), redirect_stdout(output):
            self.assertEqual(reader.main(), 1)
        value = json.loads(output.getvalue())
        self.assertTrue(value["refused"])
        self.assertNotIn("PRIVATE", output.getvalue())

if __name__ == "__main__":
    unittest.main()
