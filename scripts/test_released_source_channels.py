"""Offline channel/capture boundaries; real Windows custody runs separately."""
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location("source_channel_audit", Path(__file__).with_name("released-source-audit.py"))
audit = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(audit)


class ChannelFailureTests(unittest.TestCase):
    def test_npm_acquisition_failure_preserves_binary_cases_but_fails_receipt(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            native = root / "hermes"
            native.write_bytes(b"inert target detection fixture")
            args = SimpleNamespace(client="hermes", target="linux-amd64", native_bin=native,
                                   report_dir=root / "receipt")
            events = []

            def provision(*args, **kwargs):
                events.append(("npm", "acquire"))
                raise ValueError("public npm unavailable")

            def selected(snapshot, distribution, sequence, client, scope, refusal):
                return {"name": distribution.split("/")[-1], "client": client,
                        "scope": scope, "expected": "materialized-lifecycle"}

            def run(command, case, case_root, executable, **kwargs):
                events.append((case["channel"], case["name"]))
                self.assertEqual(command, root / "verified-binary")
                self.assertIsNone(kwargs["command_env"])
                return {**case, "status": "materialized-lifecycle-passed"}

            npm = SimpleNamespace(provision=provision)
            with mock.patch.object(audit.argparse.ArgumentParser, "parse_args", return_value=args), \
                 mock.patch.object(audit.native, "require_hosted"), \
                 mock.patch.object(audit.native, "provision_release", return_value=(root / "verified-binary", {"verified": True})), \
                 mock.patch.object(audit, "authenticated_snapshot", return_value={}), \
                 mock.patch.object(audit, "provision_native", return_value=(native, {"verified": True})), \
                 mock.patch.object(audit, "prepare_policy_context", return_value={"frozen": "verified"}), \
                 mock.patch.object(audit, "module", return_value=npm), \
                 mock.patch.object(audit, "source_case", side_effect=selected), \
                 mock.patch.object(audit, "run_case", side_effect=run), \
                 mock.patch("sys.stdout", new_callable=io.StringIO), \
                 mock.patch.dict(os.environ, {"RUNNER_TEMP": str(root)}):
                self.assertEqual(audit.main(), 1)
            receipt = json.loads((args.report_dir / "receipt.json").read_text(encoding="utf-8"))
            self.assertEqual(events, [("binary", "notion"), ("binary", "marketing-kit"), ("npm", "acquire")])
            self.assertEqual(receipt["status"], "failed")
            self.assertEqual(receipt["reason"], "public npm unavailable")
            self.assertEqual(len(receipt["cases"]), 2)
            self.assertTrue(all(case["channel"] == "binary" and case["status"] != "failed"
                                for case in receipt["cases"]))
            self.assertNotIn("npm", receipt)

    def test_complete_output_never_bypasses_windows_owner_refusal(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            for capture_files in (True, False):
                with self.subTest(capture_files=capture_files), \
                     mock.patch.object(audit.live.sys, "platform", "win32"), \
                     mock.patch.object(audit.live, "_windows_file_call", side_effect=OSError("owned descendants remained")) as owned, \
                     mock.patch.object(audit.live.subprocess, "run", side_effect=AssertionError("unowned fallback")) as fallback:
                    result = audit.live.call(["inert-command"], {"TEMP": str(root)}, root,
                                             capture_files=capture_files, capture_full=True)
                owned.assert_called_once()
                fallback.assert_not_called()
                self.assertIsNone(result["exit_code"])
                self.assertIsNone(result.get("_json"))
                self.assertIn("owned descendants remained", result["stderr_tail"])


if __name__ == "__main__":
    unittest.main()
