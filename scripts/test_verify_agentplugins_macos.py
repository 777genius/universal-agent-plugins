import hashlib
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("verify-agentplugins-macos.sh")


class VerifySignedMacOSTests(unittest.TestCase):
    def test_only_exact_signed_artifacts_are_executed(self):
        with tempfile.TemporaryDirectory(prefix="TEST-macos-verify-") as tmp:
            root = Path(tmp)
            binaries = root / "binaries"
            binaries.mkdir()
            fakes = root / "fakes"
            fakes.mkdir()
            for name, body in {"uname": "echo arm64", "xattr": "exit 0", "spctl": "exit 0", "sleep": "exit 0"}.items():
                path = fakes / name
                path.write_text("#!/bin/sh\n" + body + "\n")
                path.chmod(0o755)
            marker = root / "executed"
            arm = binaries / "agentplugins_1.2.3_darwin_arm64"
            arm.write_text(f'#!/bin/sh\ntouch "{marker}"\necho "agentplugins 1.2.3"\n')
            intel = binaries / "agentplugins_1.2.3_darwin_amd64"
            intel.write_text("other architecture")
            env = dict(os.environ, PATH=f"{fakes}:{os.environ['PATH']}", RUNNER_TEMP=tmp,
                       SIGNED_DARWIN_ARM64_SHA256=hashlib.sha256(arm.read_bytes()).hexdigest(),
                       SIGNED_DARWIN_AMD64_SHA256=hashlib.sha256(intel.read_bytes()).hexdigest())
            cmd = ["bash", str(SCRIPT), "agentplugins-v1.2.3", str(binaries)]
            result = subprocess.run(cmd, env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue(marker.exists())
            marker.unlink()
            arm.write_text('#!/bin/sh\necho "agentplugins 1.2.3"\nexit 1\n')
            env["SIGNED_DARWIN_ARM64_SHA256"] = hashlib.sha256(arm.read_bytes()).hexdigest()
            self.assertNotEqual(subprocess.run(cmd, env=env, capture_output=True).returncode, 0,
                                "a failed binary must not pass by printing the expected version")
            attempts = root / "first-attempt"
            arm.write_text(f'#!/bin/sh\ntouch "{marker}"\nif [ ! -e "{attempts}" ]; then '
                           f'touch "{attempts}"; echo "agentplugins 1.2.3"; exit 1; fi\n'
                           'echo "agentplugins 1.2.3"\n')
            env["SIGNED_DARWIN_ARM64_SHA256"] = hashlib.sha256(arm.read_bytes()).hexdigest()
            result = subprocess.run(cmd, env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue(attempts.exists(), "a failed attempt must be retried")
            self.assertTrue(marker.exists(), "the retry fixture must mark execution")
            marker.unlink()
            intel.write_text("tampered")
            result = subprocess.run(cmd, env=env, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(marker.exists())
            env["SIGNED_DARWIN_AMD64_SHA256"] = ""
            self.assertNotEqual(subprocess.run(cmd, env=env, capture_output=True).returncode, 0)
            self.assertFalse(marker.exists())


if __name__ == "__main__":
    unittest.main()
