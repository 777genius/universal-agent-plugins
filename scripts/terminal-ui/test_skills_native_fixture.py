"""Execute the compiled responder's registration/refusal/retry protocol."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

from skills_fixture import build_native_fixture


class NativeMarketplaceProtocolTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        compiler = shutil.which('go')
        if compiler is None:
            raise RuntimeError('real Go compiler required for native fixture controls')
        temporary = tempfile.TemporaryDirectory(prefix='skills-responder-build-')
        cls.addClassCleanup(temporary.cleanup)
        suffix = '.exe' if os.name == 'nt' else ''
        cls.compiled = build_native_fixture(compiler, Path(temporary.name) / ('native' + suffix))

    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix='skills-native-é-')
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        binary_dir = self.root / 'bin'
        binary_dir.mkdir()
        self.native = binary_dir / ('codex.exe' if os.name == 'nt' else 'codex')
        shutil.copy2(self.compiled, self.native)
        self.native.chmod(0o700)
        self.package = self.root / 'installer' / 'managed' / 'clients' / 'codex' / 'owned-package'
        marketplace = self.package / '.agents' / 'plugins' / 'marketplace.json'
        marketplace.parent.mkdir(parents=True)
        marketplace.write_text(json.dumps({'name': 'fixture-market'}), encoding='utf-8')
        (self.package / 'plugin.json').write_text(
            json.dumps({'name': 'fixture-plugin', 'version': '1.0.0'}), encoding='utf-8')
        skill = self.package / 'skills' / 'guide' / 'SKILL.md'
        skill.parent.mkdir(parents=True)
        skill.write_bytes(b'---\nname: guide\ndescription: inert fixture\n---\n')
        self.registry = self.root / 'native-registry.json'
        self.cache = self.root / 'home' / '.codex' / 'plugins' / 'cache'
        self.env = dict(os.environ, CODEX_HOME=str(self.root / 'home' / '.codex'))

    def command(self, args, expected=0):
        result = subprocess.run([str(self.native), *args], cwd=self.root, env=self.env,
                                stdin=subprocess.DEVNULL, capture_output=True, timeout=5)
        self.assertEqual(result.returncode, expected,
                         result.stderr.decode('utf-8', 'replace'))
        if expected == 0:
            self.assertEqual(json.loads(result.stdout), {})
        else:
            self.assertEqual(result.stdout, b'')
        return result

    def add_marketplace(self):
        self.command(['plugin', 'marketplace', 'add', str(self.package), '--json'])

    def calls(self):
        with (self.root / 'native-calls.jsonl').open(encoding='utf-8', newline='\n') as stream:
            return [json.loads(line) for line in stream]

    def test_initial_missing_marketplace_and_plugin_add_refuse_without_registry_or_cache(self):
        result = self.command(['plugin', 'marketplace', 'update', 'fixture-market', '--json'], 97)
        self.assertIn(b'synthetic marketplace is not registered', result.stderr)
        self.assertFalse(self.registry.exists())
        self.command(['plugin', 'add', 'fixture-plugin@fixture-market', '--json'], 97)
        self.assertFalse(self.registry.exists())
        self.assertFalse(self.cache.exists())

    def test_registered_update_preserves_identity_and_foreign_update_does_not_mutate_it(self):
        self.add_marketplace()
        self.command(['plugin', 'add', 'fixture-plugin@fixture-market', '--json'])
        before = self.registry.read_bytes()
        cached_skill = self.cache / 'fixture-market' / 'fixture-plugin' / '1.0.0' / 'skills' / 'guide' / 'SKILL.md'
        original_skill = cached_skill.read_bytes()
        self.command(['plugin', 'marketplace', 'update', 'fixture-market', '--json'])
        self.assertEqual(self.registry.read_bytes(), before)
        self.assertEqual(cached_skill.read_bytes(), original_skill)
        for args in (['plugin', 'marketplace', 'update', 'foreign-market', '--json'],
                     ['plugin', 'marketplace', 'update', 'fixture-market', '--unexpected']):
            with self.subTest(args=args):
                self.command(args, 97)
                self.assertEqual(self.registry.read_bytes(), before)
                self.assertEqual(cached_skill.read_bytes(), original_skill)

    def test_compensated_registration_refuses_update_then_allows_explicit_fallback_add(self):
        self.add_marketplace()
        (self.root / 'fail-add').write_text('once', encoding='utf-8')
        self.command(['plugin', 'add', 'fixture-plugin@fixture-market', '--json'], 7)
        self.assertFalse((self.root / 'fail-add').exists())
        self.command(['plugin', 'marketplace', 'remove', 'fixture-market', '--json'])
        compensated = self.registry.read_bytes()
        self.assertEqual(json.loads(compensated), {'path': str(self.package)})
        self.command(['plugin', 'marketplace', 'update', 'fixture-market', '--json'], 97)
        self.assertEqual(self.registry.read_bytes(), compensated)
        # A plugin cannot bypass the absent marketplace after compensation.
        self.command(['plugin', 'add', 'fixture-plugin@fixture-market', '--json'], 97)
        self.assertEqual(self.registry.read_bytes(), compensated)
        self.assertFalse(self.cache.exists())
        self.add_marketplace()
        self.command(['plugin', 'add', 'fixture-plugin@fixture-market', '--json'])
        installed = json.loads(self.registry.read_text(encoding='utf-8'))
        self.assertEqual(installed['spec'], 'fixture-plugin@fixture-market')
        self.assertEqual(installed['path'], str(self.package))
        self.assertTrue(installed['installed'][0]['installed'])
        self.assertTrue(installed['installed'][0]['enabled'])
        self.assertTrue(self.cache.is_dir())
        self.assertEqual([args[:3] for args in self.calls()], [
            ['plugin', 'marketplace', 'add'], ['plugin', 'add', 'fixture-plugin@fixture-market'],
            ['plugin', 'marketplace', 'remove'], ['plugin', 'marketplace', 'update'],
            ['plugin', 'add', 'fixture-plugin@fixture-market'], ['plugin', 'marketplace', 'add'],
            ['plugin', 'add', 'fixture-plugin@fixture-market']])


if __name__ == '__main__':
    unittest.main()
