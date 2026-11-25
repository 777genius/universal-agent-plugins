"""Runtime fault injection for native peer initialization custody."""
import runpy
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch

RUNNER = runpy.run_path(str(Path(__file__).with_name('marketing-kit-project-release-e2e.py')))


class PeerInitializationCustody(unittest.TestCase):
    def test_native_initialization_c_cannot_rebaseline_new_a_home_file(self):
        for raises in (False, True):
            with self.subTest(initializer_raises=raises), TemporaryDirectory() as directory:
                root = Path(directory)
                home, data, project = (root / name for name in ('home', 'data', 'project'))
                for path in (home, data, project):
                    path.mkdir()
                env = {'HOME': str(home), 'AGENTPLUGINS_HOME': str(data)}
                guard = RUNNER['unchanged_profile']
                # Keep real snapshots/refusals; only suppress settling delays.
                with patch.dict(guard.__wrapped__.__globals__, {'sleep': lambda _: None}):
                    with self.assertRaisesRegex(AssertionError, 'User C initialization: original isolated profile changed'):
                        with guard(env, project, 'User C initialization'):
                            (home / 'unexpected-from-c').write_text('native peer write')
                            if raises:
                                raise RuntimeError('native initialization failed')

    def test_unchanged_initialization_preserves_original_exception(self):
        with TemporaryDirectory() as directory:
            root = Path(directory)
            env = {'HOME': str(root), 'AGENTPLUGINS_HOME': str(root)}
            guard = RUNNER['unchanged_profile']
            with patch.dict(guard.__wrapped__.__globals__, {'sleep': lambda _: None}):
                with self.assertRaisesRegex(RuntimeError, 'native initialization failed'):
                    with guard(env, root, 'User C initialization'):
                        raise RuntimeError('native initialization failed')


if __name__ == '__main__':
    unittest.main()
