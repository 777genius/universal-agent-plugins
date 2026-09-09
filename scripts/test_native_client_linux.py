"""Linux hosted-runner safety tests; no client execution or network access."""
from contextlib import ExitStack
import importlib.util
import json
from pathlib import Path
import sys
from tempfile import TemporaryDirectory
from types import SimpleNamespace
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
    @staticmethod
    def fake_provision(pin, directory, name):
        path = directory / name
        path.write_bytes(('frozen-' + name).encode())
        path.chmod(0o600)
        return path, dict(source='fixture', version=pin[2], archive=pin[3],
                          archive_integrity=pin[4], binary_sha256=matrix.draft.digest(path))

    def prepare_fixture(self, temp, state='public', client='codex'):
        root = Path(temp) / 'prepared'
        args = SimpleNamespace(prepared_input=root, prepare_only=True, release_state=state,
                               client=client, target='linux-amd64',
                               release_tag='agentplugins-v1.2.3', release_commit='a'*40,
                               release_repo=matrix.draft.REPOSITORY)

        def public(*unused):
            path = root / 'installer'
            path.write_bytes(b'installer')
            return path, {'version': '1.2.3'}

        def draft(*unused):
            tarball = root / 'package.tgz'
            tarball.write_bytes(b'package')
            assets = root / 'assets'
            assets.mkdir()
            (assets / 'installer').write_bytes(b'installer')
            (root / 'initial-draft.json').write_text('{}')
            return tarball, assets, {'version': '1.2.3'}, {'version': '1.2.3'}

        with patch.object(matrix, 'provision', side_effect=self.fake_provision) as tools, \
             patch.object(matrix, 'provision_release', side_effect=public) as release, \
             patch.object(matrix.draft, 'acquire', side_effect=draft) as acquire:
            matrix.prepared_release(Path(temp), args)
            self.assertEqual([call.args for call in tools.call_args_list],
                             [(matrix.PINS[args.target][name], root / 'prepared-tools', name)
                              for name in (client, 'lintai', 'rg')])
            self.assertEqual(release.call_count, int(state == 'public'))
            self.assertEqual(acquire.call_count, int(state == 'draft'))
        args.prepare_only = False
        matrix.os.environ['PREPARED_SHA256'] = matrix.draft.digest(root / 'prepared.json')
        return args

    def test_preparation_returns_before_runtime_setup(self):
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
                                       client='codex', release_tag='agentplugins-v1.2.3', release_commit='a'*40, release_repo=matrix.draft.REPOSITORY)
                def acquire(*unused):
                    binary = root/'installer'; binary.write_bytes(b'original')
                    return binary, {'version':'1.2.3'}
                with patch.object(matrix, 'provision_release', side_effect=acquire), \
                     patch.object(matrix, 'provision', side_effect=self.fake_provision):
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

    def forbid_acquisition(self, stack):
        for owner, name in ((matrix, 'provision'), (matrix, 'provision_release'),
                            (matrix.draft, 'acquire'), (matrix.urllib.request, 'urlopen')):
            stack.enter_context(patch.object(owner, name, side_effect=AssertionError('acquisition during consumption')))

    def test_prepared_public_and_draft_tools_round_trip(self):
        for state in ('public', 'draft'):
            for client in matrix.PATTERNS:
                with self.subTest(state=state, client=client), TemporaryDirectory() as temp, \
                     patch.dict(matrix.os.environ, {}, clear=True), ExitStack() as stack:
                    args = self.prepare_fixture(temp, state, client)
                    self.forbid_acquisition(stack)
                    release = matrix.prepared_release(Path(temp), args)
                    self.assertEqual(len(release), 4 if state == 'draft' else 2)
                    self.assertEqual(release[0].read_bytes(), b'package' if state == 'draft' else b'installer')
                    self.assertEqual(release[-1], {'version': '1.2.3'})
                    if state == 'draft':
                        self.assertEqual((release[1] / 'installer').read_bytes(), b'installer')
                        self.assertEqual(release[2], {'version': '1.2.3'})
                    runtime = Path(temp) / 'bin'
                    runtime.mkdir()
                    tools = matrix.prepared_tools(args, runtime)
                    self.assertEqual(set(tools), {client, 'lintai', 'rg'})
                    for name, (path, evidence) in tools.items():
                        self.assertEqual(path.parent, runtime)
                        self.assertEqual(path.read_bytes(), ('frozen-' + name).encode())
                        self.assertEqual(evidence['binary_sha256'], matrix.draft.digest(path))
                        self.assertEqual(evidence['archive_integrity'], matrix.PINS[args.target][name][4])
                        if matrix.os.name != 'nt':
                            self.assertEqual(path.stat().st_mode & 0o777, 0o700)

    def test_prepared_tools_reject_mixed_or_damaged_inputs(self):
        mutations = ('client', 'target', 'pin', 'metadata', 'missing-tools',
                     'missing-client', 'missing-lintai', 'missing-rg',
                     'tamper-client', 'tamper-lintai', 'tamper-rg')
        for state in ('public', 'draft'):
            for mutation in mutations:
                with self.subTest(state=state, mutation=mutation), TemporaryDirectory() as temp, \
                     patch.dict(matrix.os.environ, {}, clear=True), ExitStack() as stack:
                    args = self.prepare_fixture(temp, state)
                    root = args.prepared_input
                    manifest = root / 'prepared.json'
                    record = json.loads(manifest.read_bytes())
                    if mutation == 'client':
                        args.client = 'claude'
                    elif mutation == 'target':
                        args.target = 'linux-arm64'
                    elif mutation == 'pin':
                        pin = list(matrix.PINS[args.target]['rg'])
                        pin[4] = 'sha256:' + '0'*64
                        stack.enter_context(patch.dict(matrix.PINS[args.target], rg=tuple(pin)))
                    elif mutation == 'metadata':
                        record['tools']['rg']['evidence']['version'] = 'changed'
                        manifest.write_text(json.dumps(record))
                    elif mutation == 'missing-tools':
                        del record['tools']
                        manifest.write_text(json.dumps(record))
                        matrix.os.environ['PREPARED_SHA256'] = matrix.draft.digest(manifest)
                    else:
                        action, name = mutation.split('-', 1)
                        name = args.client if name == 'client' else name
                        path = root / record['tools'][name]['path']
                        if action == 'missing':
                            path.unlink()
                        else:
                            path.write_bytes(b'changed')
                    runtime = Path(temp) / 'bin'
                    runtime.mkdir()
                    self.forbid_acquisition(stack)
                    with self.assertRaises(ValueError):
                        matrix.prepared_release(Path(temp), args)
                    with self.assertRaises(ValueError):
                        matrix.prepared_tools(args, runtime)
                    self.assertEqual(list(runtime.iterdir()), [])

    def test_standalone_tools_keep_acquisition(self):
        with TemporaryDirectory() as temp, patch.object(matrix, 'provision', side_effect=self.fake_provision) as acquire:
            args = SimpleNamespace(prepared_input=None, client='claude', target='linux-amd64')
            tools = matrix.prepared_tools(args, Path(temp))
            self.assertEqual(set(tools), {'claude', 'lintai', 'rg'})
            self.assertEqual(acquire.call_count, 3)

    def test_prepared_main_reaches_build_with_local_tools(self):
        class BuildReached(Exception):
            pass

        for state in ('public', 'draft'):
            with self.subTest(state=state), TemporaryDirectory() as temp, \
                 patch.dict(matrix.os.environ, {}, clear=True), ExitStack() as stack:
                args = self.prepare_fixture(temp, state)
                matrix.os.environ.update(RUNNER_TEMP=temp, EXPECTED_COMMIT='a'*40)
                argv = ['runner', '--client', args.client, '--target', args.target,
                        '--output', str(Path(temp) / 'output'), '--release-state', state,
                        '--release-tag', args.release_tag, '--release-commit', args.release_commit,
                        '--prepared-input', str(args.prepared_input)]
                if state == 'draft':
                    argv += ['--producer-source', temp]
                    for field in matrix.draft.FIELDS:
                        argv += ['--' + field.replace('_', '-'), 'a'*64 if 'digest' in field else '1']
                self.forbid_acquisition(stack)
                stack.enter_context(patch.object(sys, 'argv', argv))
                stack.enter_context(patch.object(matrix, 'require_hosted'))
                stack.enter_context(patch.object(matrix.draft, 'helper_hashes', return_value={}))
                stack.enter_context(patch.object(matrix.shutil, 'which', return_value=sys.executable))
                stack.enter_context(patch.object(matrix.subprocess, 'check_output',
                                                side_effect=['a'*40, 'b'*40, b'']))
                stack.enter_context(patch.object(matrix, 'profile_environment', return_value={}))
                stack.enter_context(patch.object(matrix.os, 'name', 'posix'))

                def build(command, **kwargs):
                    self.assertEqual(command[:2], ['go', 'build'])
                    binary_dir = Path(command[command.index('-o') + 1]).parent
                    for name in (args.client, 'lintai', 'rg'):
                        self.assertEqual((binary_dir / name).read_bytes(), ('frozen-' + name).encode())
                    raise BuildReached()

                run = stack.enter_context(patch.object(matrix.subprocess, 'run', side_effect=build))
                with self.assertRaises(BuildReached):
                    matrix.main()
                run.assert_called_once()


if __name__ == "__main__":
    unittest.main()
