"""Inert dedicated-key isolation and evidence canaries; no vendor execution."""
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
import zipfile

import project_native_kiro_auth as auth
import project_native_research as research
import project_native_runtime as runtime


class KiroAuthContracts(unittest.TestCase):
    def test_snapshot_is_optional_bounded_and_does_not_read_for_other_clients(self):
        class NoRead(dict):
            def get(self, *_args):
                raise AssertionError("another client read the dedicated key")
        self.assertEqual(auth.snapshot("copilot", NoRead()), "")
        self.assertEqual(auth.snapshot("kiro", {}), "")
        self.assertEqual(auth.snapshot("kiro", {"KIRO_API_KEY": ""}), "")
        key = "ksk_inert_private_key_12345"
        environment = {"KIRO_API_KEY": key, "HOME": "/original"}
        self.assertEqual(auth.snapshot("kiro", environment), key)
        self.assertEqual(environment, {"KIRO_API_KEY": key, "HOME": "/original"})
        for invalid in (None, 123, "short", "private key", "private\nkey", "private\0key",
                        "private\x7fkey", "privateékey", "a" * 4096):
            with self.subTest(kind=type(invalid).__name__), self.assertRaisesRegex(ValueError, "^Kiro CI key refused by private capture bounds$"):
                auth.snapshot("kiro", {"KIRO_API_KEY": invalid})

    def test_registered_plain_json_and_base64_forms_redact_before_publication(self):
        key = 'ksk_inert_private_"key_12345'
        with tempfile.TemporaryDirectory() as temporary:
            ctx = research.Context(Path(temporary), {"commands": []}, None)
            auth.register(ctx, key)
            self.assertNotIn("KIRO_API_KEY", ctx.env)
            self.assertEqual(ctx.kiro_api_key, key)
            forms = auth.private_forms(key)
            self.assertEqual(tuple(ctx.secrets), forms)
            for index, form in enumerate(forms):
                body = b"before " + form + b" after"
                digest = hashlib.sha256(body).hexdigest()
                summary = {"commands": [{"probe": "private", "stdout_file": "private.stdout.log",
                           "stderr_file": "private.stderr.log", "stdout_bytes": len(body),
                           "stdout_sha256": digest, "stderr_bytes": 0,
                           "stderr_sha256": hashlib.sha256(b"").hexdigest()}], "error": form.decode()}
                bodies = research.evidence(summary, {"private.stdout.log": body, "private.stderr.log": b""}, ctx.secrets)
                self.assertEqual(bodies["private.stdout.log"], b"before [redacted] after", index)
                self.assertFalse(any(private in value for private in forms for value in bodies.values()))

    def test_inner_native_zip_rejects_private_forms_before_admission(self):
        key = 'ksk_inert_private_"key_12345'
        source = {"source_commit": "a" * 40, "source_tree": "b" * 40}
        result = {"client": "kiro", "version": "2.34.0", "runtime_attempt_complete": False,
                  "test_failed": True, "activation": "prepared", "vendor_authentication_qualified": False,
                  "real_model_qualified": False, "qualification": {name: False for name in research.QUALIFICATION_CASES},
                  "source_identity": {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                                      "installer_source_state": "committed", "installer_patch_sha256": []}}
        with tempfile.TemporaryDirectory() as temporary:
            for index, form in enumerate((None, *auth.private_forms(key))):
                path = Path(temporary) / (str(index) + ".zip")
                with zipfile.ZipFile(path, "w") as archive:
                    archive.writestr("runtime-attempt.json", json.dumps(result))
                    if form is not None:
                        archive.writestr("probe-events.jsonl", b"before " + form + b" after")
                if form is None:
                    _, admitted = runtime.admit_runtime(path, "kiro", source, key)
                    self.assertTrue(admitted["test_failed"])
                    self.assertFalse(any(admitted["qualification"].values()))
                else:
                    with self.assertRaisesRegex(ValueError, "^private Kiro credential refused from native evidence$"):
                        runtime.admit_runtime(path, "kiro", source, key)


if __name__ == "__main__":
    unittest.main()
