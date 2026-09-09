"""Linux hosted-runner safety tests; no client execution or network access."""
import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location("native_linux_matrix", Path(__file__).with_name("run-native-client-matrix.py"))
matrix = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(matrix)


class LinuxHostedSafetyTests(unittest.TestCase):
    def test_native_amd64_host_mapping_and_opt_in(self):
        env = {"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"}
        for machine in ("x86_64", "amd64", "AMD64"):
            with self.subTest(machine=machine), patch.dict(matrix.os.environ, env, clear=True), patch.object(matrix.platform, "system", return_value="Linux"), patch.object(matrix.platform, "machine", return_value=machine):
                runtime = matrix.disposable_runtime_environment("linux-amd64")
                self.assertEqual(runtime["AGENTPLUGINS_NATIVE_DISPOSABLE_LINUX"], "1")
        for changes, system, machine in [({}, "Linux", "aarch64"), ({}, "Darwin", "x86_64"), ({"RUNNER_ENVIRONMENT": "self-hosted"}, "Linux", "x86_64"), ({"AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "0"}, "Linux", "x86_64")]:
            with self.subTest(changes=changes, system=system, machine=machine), patch.dict(matrix.os.environ, {**env, **changes}, clear=True), patch.object(matrix.platform, "system", return_value=system), patch.object(matrix.platform, "machine", return_value=machine):
                with self.assertRaises(RuntimeError):
                    matrix.disposable_runtime_environment("linux-amd64")

    def test_linux_versions_match_existing_lanes(self):
        for tool in ("codex", "claude", "opencode", "rg", "lintai"):
            self.assertEqual(matrix.PINS["linux-amd64"][tool][2], matrix.PINS["linux-arm64"][tool][2])

    def test_unfilled_pins_fail_before_any_download(self):
        for tool in matrix.PINS["linux-amd64"]:
            pin = list(matrix.PINS["linux-amd64"][tool])
            pin[4] = None  # Explicitly test the incomplete-pin failure boundary.
            with self.subTest(tool=tool), patch.object(matrix.subprocess, "run") as github, patch.object(matrix.urllib.request, "urlopen") as npm:
                with self.assertRaisesRegex(ValueError, "unfilled"):
                    matrix.provision(pin, Path("unused"), tool)
                github.assert_not_called()
                npm.assert_not_called()

    def test_dispatch_scope_selects_only_three_linux_jobs(self):
        import json
        import re
        workflow = (Path(__file__).resolve().parents[1] / ".github/workflows/agentplugins-released-native-clients.yml").read_text()
        expression = next(line for line in workflow.splitlines() if "platform: ${{" in line)
        selected, historical = [json.loads(value) for value in re.findall(r"'([\[].*?[\]])'", expression)]
        self.assertEqual(selected, [{"target": "linux-amd64", "runner": "ubuntu-24.04"}])
        self.assertEqual({p["target"] for p in historical}, {"linux-arm64", "darwin-arm64", "windows-amd64"})
        self.assertIn("default: historical-nine", workflow)
        self.assertIn("client: [codex, claude, opencode]", workflow)

    def test_native_arm64_host_accepts_kernel_machine_aliases(self):
        env = {"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"}
        for machine in ("aarch64", "arm64"):
            with self.subTest(machine=machine), patch.dict(matrix.os.environ, env, clear=True), patch.object(matrix.platform, "system", return_value="Linux"), patch.object(matrix.platform, "machine", return_value=machine):
                matrix.require_hosted("linux-arm64")

    def test_runtime_gets_linux_opt_in_only_after_host_validation(self):
        env = {"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1", "AGENTPLUGINS_NATIVE_DISPOSABLE_LINUX": "0", "ANTHROPIC_API_KEY": "must-not-copy"}
        with patch.dict(matrix.os.environ, env, clear=True), patch.object(matrix.platform, "system", return_value="Linux"), patch.object(matrix.platform, "machine", return_value="aarch64"):
            runtime = matrix.disposable_runtime_environment("linux-arm64")
            self.assertEqual(runtime["AGENTPLUGINS_NATIVE_DISPOSABLE_LINUX"], "1")
            self.assertNotIn("ANTHROPIC_API_KEY", runtime)
        with patch.dict(matrix.os.environ, env, clear=True), patch.object(matrix.platform, "system", return_value="Darwin"), patch.object(matrix.platform, "machine", return_value="arm64"):
            self.assertNotIn("AGENTPLUGINS_NATIVE_DISPOSABLE_LINUX", matrix.disposable_runtime_environment("darwin-arm64"))
        with patch.dict(matrix.os.environ, {"AGENTPLUGINS_NATIVE_DISPOSABLE_LINUX": "1"}, clear=True):
            with self.assertRaises(RuntimeError):
                matrix.disposable_runtime_environment("linux-arm64")

    def test_linux_target_rejects_wrong_architecture_and_non_disposable_host(self):
        baseline = {"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"}
        cases = [(baseline, "x86_64"), ({**baseline, "RUNNER_ENVIRONMENT": "self-hosted"}, "aarch64"), ({**baseline, "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "0"}, "aarch64"), ({}, "aarch64")]
        for env, machine in cases:
            with self.subTest(env=env, machine=machine), patch.dict(matrix.os.environ, env, clear=True), patch.object(matrix.platform, "system", return_value="Linux"), patch.object(matrix.platform, "machine", return_value=machine):
                with self.assertRaises(RuntimeError):
                    matrix.require_hosted("linux-arm64")



class PreparedInputTests(unittest.TestCase):
    def test_preparation_returns_before_client_provisioning(self):
        import sys
        from tempfile import TemporaryDirectory
        with TemporaryDirectory() as temp, patch.object(matrix, 'require_hosted'), \
             patch.object(matrix, 'prepared_release') as prepare, patch.object(matrix, 'provision') as provision, \
             patch.object(matrix.subprocess, 'run', side_effect=AssertionError('execution during preparation')), \
             patch.object(sys, 'argv', ['runner', '--client', 'codex', '--target', 'linux-amd64', '--output', temp,
                  '--release-tag', 'agentplugins-v1.2.3', '--release-commit', 'a'*40, '--prepare-only', '--prepared-input', temp]):
            matrix.main()
            prepare.assert_called_once()
            provision.assert_not_called()

    def test_prepared_runtime_rehashes_and_never_falls_back(self):
        import json
        import os
        from tempfile import TemporaryDirectory
        from types import SimpleNamespace
        for mutation in ('none','missing','tamper','manifest','credential','extra'):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temp, patch.dict(os.environ, {}, clear=True):
                root = Path(temp)/'prepared'
                args = SimpleNamespace(prepared_input=root, prepare_only=True, release_state='public', target='linux-amd64',
                                       release_tag='agentplugins-v1.2.3', release_commit='a'*40, release_repo=matrix.draft.REPOSITORY)
                def acquire(*unused):
                    binary = root/'installer'; binary.write_bytes(b'original')
                    return binary, {'version':'1.2.3'}
                with patch.object(matrix, 'provision_release', side_effect=acquire):
                    matrix.prepared_release(Path(temp), args)
                args.prepare_only = False
                os.environ['PREPARED_SHA256'] = matrix.draft.digest(root/'prepared.json')
                if mutation == 'missing': (root/'installer').unlink()
                elif mutation == 'tamper': (root/'installer').write_bytes(b'changed')
                elif mutation == 'manifest': (root/'prepared.json').write_text('{}')
                elif mutation == 'credential': os.environ['GH_TOKEN'] = 'test-sentinel'
                elif mutation == 'extra': (root/'extra').write_bytes(b'new')
                with patch.object(matrix, 'provision_release', side_effect=AssertionError('network fallback')):
                    if mutation == 'none':
                        self.assertEqual(matrix.prepared_release(Path(temp), args)[0].read_bytes(), b'original')
                    else:
                        with self.assertRaises(ValueError): matrix.prepared_release(Path(temp), args)


if __name__ == "__main__":
    unittest.main()
