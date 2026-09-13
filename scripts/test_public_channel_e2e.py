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


if __name__ == '__main__':
    unittest.main()
