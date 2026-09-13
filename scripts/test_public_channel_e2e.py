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
        self.assertEqual(channels.COMMANDS, ('validate', 'inspect', 'pack', 'compat'))

class DisposableJourney(unittest.TestCase):
    def test_both_entrypoints_continue_after_pack_failure_and_cleanup(self):
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
            status = 0
            data = {'revision': channels.REVISION}
            if 'version' in args:
                data['product_version'] = '0.1.61' if 'author' in args else '2.0.1'
            if 'init' in args:
                project = Path(args[args.index('init') + 1])
                project.mkdir()
                (project / 'plugin.json').write_text('{}')
            if 'pack' in args:
                status = 1
            return subprocess.CompletedProcess(args, status, json.dumps({'data': data}), '')

        output = io.StringIO()
        with patch.object(channels.subprocess, 'run', side_effect=execute), \
                contextlib.redirect_stdout(output):
            with self.assertRaisesRegex(ValueError, 'pack'):
                channels.main('npm')
        self.assertEqual(sum('compat' in args for args in calls), 2)
        self.assertEqual(sum('pack' in args for args in calls), 2)
        self.assertTrue(all(not root.exists() for root in roots))
        evidence = json.loads(output.getvalue())
        self.assertEqual(evidence['status'], 'failed')
        self.assertEqual(len(evidence['results']), len(calls))


if __name__ == '__main__':
    unittest.main()
