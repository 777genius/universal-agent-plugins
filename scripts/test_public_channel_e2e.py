import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('channels', Path(__file__).with_name('public-channel-e2e.py'))
channels = importlib.util.module_from_spec(spec)
spec.loader.exec_module(channels)


class Contract(unittest.TestCase):
    def test_identity_rejects_wrong_release(self):
        good = {'data': {'revision': channels.REVISION, 'product_version': '2.0.1'}}
        channels.identity(good, '2.0.1')
        with self.assertRaises(ValueError):
            channels.identity(good, '2.0.0')
        good['data']['revision'] = 'a' * 40
        with self.assertRaises(ValueError):
            channels.identity(good, '2.0.1')

    def test_exact_release_inputs(self):
        argv = ['github', '--agentplugins-version', '0.1.62',
                '--plugin-kit-ai-version', '2.0.2', '--agentplugins-tag',
                'agentplugins-v0.1.62', '--plugin-kit-ai-tag', 'plugin-kit-ai-v2.0.2',
                '--revision', 'b' * 40]
        args = channels.arguments(argv)
        self.assertEqual(args.agentplugins_version, '0.1.62')
        self.assertEqual(args.plugin_kit_ai_tag, 'plugin-kit-ai-v2.0.2')
        channels.identity({'data': {'revision': args.revision,
                                    'product_version': '2.0.2'}}, '2.0.2', args.revision)
        import contextlib
        import io
        for position, bad in ((2, 'latest'), (6, '../tag'), (6, 'v0.1.62'),
                              (8, 'agentplugins-v0.1.62'), (10, 'short')):
            invalid = list(argv)
            invalid[position] = bad
            with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
                channels.arguments(invalid)

    def test_retry_is_bounded_and_preserves_failure(self):
        with patch.object(channels.time, 'sleep') as sleep:
            with patch.object(channels, 'download', side_effect=ValueError('unpublished')) as operation:
                with self.assertRaisesRegex(ValueError, 'unpublished'):
                    channels.retry(operation)
                self.assertEqual(operation.call_count, 4)
                self.assertEqual(sleep.call_count, 3)

    def test_dispatch_contract(self):
        workflow = (Path(__file__).resolve().parents[1] / '.github/workflows/milestone-a-public-channels.yml').read_text()
        self.assertIn('workflow_dispatch:', workflow)
        self.assertNotIn('pull_request:', workflow)
        self.assertNotIn('push:', workflow)
        self.assertIn('fail-fast: false', workflow)
        self.assertIn('channel: [npm, pypi, github, brew]', workflow)
        self.assertIn('if: always()', workflow)
        self.assertEqual(channels.COMMANDS, ('validate', 'inspect', 'compat', 'test'))

    def test_temporary_root_is_canonicalized(self):
        source = Path(__file__).with_name('public-channel-e2e.py').read_text()
        self.assertIn('root = Path(temporary.name).resolve()', source)

    def test_homebrew_trust_is_exact_and_precedes_formula_evaluation(self):
        source = Path(__file__).with_name('public-channel-e2e.py').read_text()
        trust = "run([brew, 'trust', '--tap', '777genius/plugin-kit-ai'])"
        tap = "run([brew, 'tap', '777genius/plugin-kit-ai'])"
        info = "run([brew, 'info', '--json=v2', formula])"
        self.assertEqual(source.count(trust), 1)
        self.assertLess(source.index(trust), source.index(tap))
        self.assertLess(source.index(tap), source.index(info))

class DisposableJourney(unittest.TestCase):
    def test_success_and_failure_cleanup(self):
        for fail in (False, True):
            with self.subTest(fail=fail):
                self.check_journey(fail)

    def test_cleanup_failure_is_recorded_before_failed_evidence(self):
        self.check_journey(False, cleanup_failure=True)
        self.check_journey(True, cleanup_failure=True)

    def test_early_product_failures_still_check_second_product(self):
        for stage in ('install', 'version', 'init'):
            with self.subTest(stage=stage):
                self.check_journey(False, early_failure=stage)

    def test_timeout_retains_partial_output_and_continues(self):
        for partial in ((b'partial stdout', b'partial stderr'),
                        ('partial stdout', 'partial stderr'), (None, None)):
            with self.subTest(partial=partial):
                self.check_journey(False, timeout_output=partial)

    def check_journey(self, fail, cleanup_failure=False, early_failure=None,
                      timeout_output=None):
        import contextlib
        import io
        import json
        import subprocess
        calls = []
        roots = set()

        def execute(args, **kwargs):
            calls.append(args)
            root = Path(kwargs['cwd'])
            roots.add(root)
            env = kwargs['env']
            for key in ('HOME', 'USERPROFILE', 'XDG_CACHE_HOME', 'npm_config_cache'):
                self.assertTrue(Path(env[key]).is_relative_to(root))
            self.assertNotEqual(env['npm_config_userconfig'], env['npm_config_globalconfig'])
            self.assertTrue(Path(env['npm_config_userconfig']).is_file())
            self.assertNotIn('GITHUB_TOKEN', env)
            first_product = 'author' in args or any('universal-agent-plugins@' in arg for arg in args)
            if first_product and early_failure is not None and early_failure in args:
                raise subprocess.CalledProcessError(1, args, 'early failure', '')
            if first_product and 'validate' in args and timeout_output is not None:
                raise subprocess.TimeoutExpired(args, 240, output=timeout_output[0],
                                                stderr=timeout_output[1])
            status = 0
            data = {'revision': channels.REVISION}
            if 'version' in args:
                data['product_version'] = '0.1.61' if 'author' in args else '2.0.1'
            if 'init' in args:
                project = Path(args[args.index('init') + 1])
                project.mkdir()
                (project / 'plugin.json').write_text('{}')
            if fail and 'validate' in args:
                status = 1
            return subprocess.CompletedProcess(args, status, json.dumps({'data': data}), '')

        output = io.StringIO()
        cleanup = channels.tempfile.TemporaryDirectory.cleanup
        cleanup_calls = []

        def clean(temporary):
            # Evidence must not be emitted until cleanup has completed.
            self.assertEqual(output.getvalue(), '')
            cleanup_calls.append(temporary.name)
            cleanup(temporary)
            if cleanup_failure:
                raise OSError('cleanup denied')

        failed = fail or cleanup_failure or early_failure or timeout_output is not None
        with patch.object(channels.subprocess, 'run', side_effect=execute), \
                patch.object(channels.tempfile.TemporaryDirectory, 'cleanup', clean), \
                patch.object(channels.time, 'sleep'), \
                contextlib.redirect_stdout(output):
            if failed:
                with self.assertRaises(ValueError):
                    channels.main('npm')
            else:
                channels.main('npm')
        self.assertEqual(len(cleanup_calls), 1)
        self.assertEqual(sum('compat' in args for args in calls), 1 if early_failure else 2)
        self.assertEqual(sum('test' in args for args in calls), 1 if early_failure else 2)
        self.assertTrue(all(not root.exists() for root in roots))
        evidence = json.loads(output.getvalue())
        self.assertEqual(evidence['status'], 'failed' if failed else 'passed')
        self.assertEqual(evidence['cleanup']['status'], 'failed' if cleanup_failure else 'passed')
        if cleanup_failure:
            self.assertIn('cleanup denied', evidence['cleanup']['error'])
            self.assertIn('cleanup denied', evidence['error'])
        if fail:
            self.assertIn('agentplugins validate', evidence['error'])
            self.assertIn('plugin-kit-ai validate', evidence['error'])
        if early_failure:
            self.assertIn('agentplugins:', evidence['error'])
            self.assertTrue(any('test' in args and 'author' not in args for args in calls))
        else:
            self.assertEqual(len(evidence['results']), len(calls))
        if timeout_output is not None:
            timed_out = [result for result in evidence['results'] if result['status'] == 'timeout']
            self.assertEqual(len(timed_out), 1)
            result = timed_out[0]
            self.assertEqual(result['argv'], next(args for args in calls if 'validate' in args))
            self.assertEqual(result['timeout'], 240)
            self.assertEqual(result['stdout'], 'partial stdout' if timeout_output[0] else None)
            self.assertEqual(result['stderr'], 'partial stderr' if timeout_output[1] else None)
            self.assertIn('agentplugins validate', evidence['error'])


if __name__ == '__main__':
    unittest.main()
