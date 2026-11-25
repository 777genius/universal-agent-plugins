"""Exercise the actual release-workflow digest check before any staging."""
import hashlib
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest


class MacOSStagingBindingTests(unittest.TestCase):
    def test_digest_mismatch_missing_or_invalid_refuses_staging(self):
        workflow = Path(__file__).resolve().parents[1] / ".github/workflows/agentplugins-release.yml"
        body = workflow.read_text()
        start = body.index('          for signed in "darwin_amd64:')
        end = body.index('          node "${GITHUB_WORKSPACE}/npm/agentplugins/scripts/release-assets.js"', start)
        check = textwrap.dedent(body[start:end])
        with tempfile.TemporaryDirectory(prefix="TEST-macos-stage-") as tmp:
            root = Path(tmp)
            env = dict(os.environ, TAG="agentplugins-v1.2.3")
            for target in ("amd64", "arm64"):
                data = target.encode()
                (root / f"agentplugins_1.2.3_darwin_{target}").write_bytes(data)
                env[f"SIGNED_DARWIN_{target.upper()}_SHA256"] = hashlib.sha256(data).hexdigest()
            def run(values):
                return subprocess.run(["bash", "-e", "-c", check], cwd=root, env=values, capture_output=True)
            self.assertEqual(run(env).returncode, 0)
            for bad in ("", "x" * 64, "0" * 64):
                self.assertNotEqual(run(dict(env, SIGNED_DARWIN_ARM64_SHA256=bad)).returncode, 0)
            (root / "agentplugins_1.2.3_darwin_amd64").unlink()
            self.assertNotEqual(run(env).returncode, 0)


if __name__ == "__main__":
    unittest.main()
