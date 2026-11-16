"""Inert failure evidence contracts; no policy installation or native command."""
import contextlib
import io
import json
from pathlib import Path
import runpy
import tempfile
import unittest
from unittest.mock import patch


class PolicyFailureEvidence(unittest.TestCase):
    def test_failed_setup_preserves_original_record_and_bounded_streams(self):
        module = runpy.run_path(str(Path(__file__).with_name("grok_bwrap_policy_setup.py")))
        scope = module["main"].__globals__
        original = dict(error=None, exit=7, cleanup_complete=True, stdout_sha256="original-out-hash",
                        stderr_sha256="original-error-hash")

        def failed_probe(*args, **kwargs):
            return dict(original), b"known stdout" * 100, b"known bwrap refusal" * 100

        def failed_setup(*args):
            records = args[-1] if isinstance(args[-1], list) else []
            return module["command"](Path(args[0]), "known-failure", ["inert"], args[1], records)

        output = io.StringIO()
        with tempfile.TemporaryDirectory() as temporary:
            with patch.dict(scope, run_probe=failed_probe, setup=failed_setup), \
                    patch("sys.argv", ["setup", temporary]), \
                    patch.dict("os.environ", SUDO_UID="1001", SUDO_GID="1001"), \
                    patch("os.getuid", return_value=0), patch("os.geteuid", return_value=0), \
                    patch("os.getgid", return_value=0), patch("os.getegid", return_value=0), \
                    patch("tempfile.mkdtemp", return_value=temporary), contextlib.redirect_stdout(output):
                with self.assertRaisesRegex(ValueError, "setup command refused: known-failure"):
                    module["main"]()
        report = json.loads(output.getvalue())
        self.assertFalse(report["setup_completed"])
        self.assertFalse(report["qualification"])
        self.assertIn("known-failure", report["error"])
        record, = report["commands"]
        for key, value in original.items():
            self.assertEqual(record[key], value)
        self.assertEqual(len(record["stdout_preview"]), 512)
        self.assertEqual(len(record["stderr_preview"]), 512)
        self.assertTrue(record["stdout_preview_truncated"])
        self.assertTrue(record["stderr_preview_truncated"])
        self.assertTrue(record["stderr_preview"].startswith("known bwrap refusal"))


if __name__ == "__main__":
    unittest.main()
