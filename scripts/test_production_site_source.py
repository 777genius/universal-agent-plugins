"""Production source selection and publisher ordering regression tests."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location('production_source', Path(__file__).with_name('production_site_source.py'))
source = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(source)
REPO = Path(__file__).resolve().parent.parent


class ProductionSourceTests(unittest.TestCase):
    def test_malformed_pins(self):
        valid = dict(schema_version=1, source_commit='a' * 40, source_tree='b' * 40)
        source.validate(valid)
        with self.assertRaises(ValueError):
            json.loads('{"schema_version":1,"schema_version":1}', object_pairs_hook=source.unique_object)
        for bad in [None, {}, {**valid, 'extra': 1}, {**valid, 'schema_version': True},
                    {**valid, 'source_commit': 'main'}, {**valid, 'source_commit': '0' * 40}, {**valid, 'source_tree': 'A' * 40}]:
            with self.assertRaises(ValueError):
                source.validate(bad)

    def test_actual_checkout_tree_refusal_and_provenance(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'repo'
            root.mkdir()
            def git(*args):
                return source.git(root, *args)
            git('init', '-q')
            git('config', 'user.name', 'Fixture')
            git('config', 'user.email', 'fixture@example.test')
            for name in ['website', 'scripts', 'landing/scripts', 'landing/public']:
                (root / name).mkdir(parents=True, exist_ok=True)
            (root / 'old-site').write_text('previously published')
            accepted_module = 'module fixture\n\ngo 1.25.0\n\ntoolchain go1.26.10\n'
            (root / 'go.mod').write_text(accepted_module)
            (root / 'landing/public/og-image.png').write_bytes(b'committed PNG')
            git('add', '.')
            git('commit', '-qm', 'accepted site')
            pin = dict(schema_version=1, source_commit=git('rev-parse', 'HEAD'), source_tree=git('rev-parse', 'HEAD^{tree}'))
            (root / 'website/production-source.json').write_text(json.dumps(pin))
            (root / 'landing/scripts/check-published-capabilities.mjs').write_text('control guard')
            (root / 'new-cli').write_text('unreleased')
            (root / 'go.mod').write_text(accepted_module.replace('go1.26.10', 'go1.26.11'))
            git('add', '.')
            git('commit', '-qm', 'control')
            control_commit = git('rev-parse', 'HEAD')
            with patch.dict(os.environ, GITHUB_SHA='0' * 40):
                with self.assertRaisesRegex(ValueError, 'workflow/control'):
                    source.select(root, Path(directory) / 'wrong-control')
            with patch.dict(os.environ, GITHUB_SHA="f" * 40):
                source.select(root, Path(directory) / 'control', expected_control=control_commit)
            self.assertEqual(git('rev-parse', 'HEAD'), pin['source_commit'])
            self.assertFalse((root / 'new-cli').exists())
            self.assertEqual((root / 'go.mod').read_text(), accepted_module)
            self.assertIn('toolchain go1.26.11', git('show', control_commit + ':go.mod'))
            self.assertEqual((Path(directory) / 'control/check-published-capabilities.mjs').read_text(), 'control guard')
            (root / '.pages-dist').mkdir()
            (root / '.pages-dist/MIRROR_METADATA.json').write_text('{"signed":"fresh"}')
            image = root / 'landing/public/og-image.png'
            image.write_bytes(b'generated PNG')
            (root / '.pages-dist/og-image.png').write_bytes(b'generated PNG')
            with patch.dict(os.environ, GITHUB_SHA=control_commit, GITHUB_RUN_ID='123', GITHUB_RUN_ATTEMPT='1'):
                source.seal(root, Path(directory) / 'control')
            receipt = json.loads((root / '.pages-dist/BUILD_PROVENANCE.json').read_text())
            self.assertEqual(receipt['source_commit'], pin['source_commit'])
            self.assertEqual(receipt['control_commit'], control_commit)
            self.assertEqual(receipt['workflow_commit'], control_commit)
            self.assertEqual(receipt['social_image_source_blob'], git('rev-parse', pin['source_commit'] + ':landing/public/og-image.png'))
            self.assertEqual(receipt['social_image_sha256'], source.hashlib.sha256(b'generated PNG').hexdigest())
            with self.assertRaises(ValueError):
                source.verify(root, pin)
            (root / 'old-site').write_text('unexpected edit')
            with self.assertRaises(ValueError):
                source.verify(root, pin, generated=True)
            git('restore', 'old-site')
            image.chmod(0o755)
            with self.assertRaises(ValueError):
                source.verify(root, pin, generated=True)
            image.chmod(0o644)
            image.unlink()
            with self.assertRaises(ValueError):
                source.verify(root, pin, generated=True)
            image.symlink_to(root / 'old-site')
            with self.assertRaises(ValueError):
                source.verify(root, pin, generated=True)
            image.unlink()
            git('restore', 'landing/public/og-image.png')
            with self.assertRaises(ValueError):
                source.verify(root, {**pin, 'source_tree': 'a' * 40})
            git('checkout', '-q', '--detach', control_commit)
            git('checkout', '-q', '--orphan', 'unrelated')
            git('rm', '-rf', '.')
            (root / 'website').mkdir(exist_ok=True)
            (root / 'website/production-source.json').write_text(json.dumps(pin))
            git('add', '.')
            git('commit', '-qm', 'unrelated')
            with patch.dict(os.environ, GITHUB_SHA=git('rev-parse', 'HEAD')):
                with self.assertRaises(subprocess.CalledProcessError):
                    source.select(root, Path(directory) / 'refused')
            self.assertFalse((Path(directory) / 'refused').exists())

    def test_both_publishers_select_before_tools_and_keep_signed_feed_route(self):
        for name in ['docs-pages.yml', 'registry-compatibility-mirror.yml']:
            text = (REPO / '.github/workflows' / name).read_text()
            self.assertLess(text.index('production_site_source.py select'), text.index('actions/setup-'))
            self.assertLess(text.index('production_site_source.py select'), text.index('pnpm/action-setup'))
            go_blocks = text.split('      - uses: actions/setup-go@')[1:]
            self.assertEqual(len(go_blocks), 2)
            network_go, docs_go = [block.split('\n      - ', 1)[0] for block in go_blocks]
            self.assertIn('          go-version: "1.26.11"\n', network_go)
            self.assertNotIn('go-version-file:', network_go)
            self.assertIn('          go-version-file: go.mod\n', docs_go)
            self.assertNotIn('go-version:', docs_go)
            # setup-go exports local after parsing; the next action must reset
            # that inherited value to honor the accepted toolchain directive.
            self.assertIn('        env:\n          GOTOOLCHAIN: auto\n', docs_go)
            self.assertNotIn('GOTOOLCHAIN: auto', network_go)
            feed = text.split('      - name: Verify and stage signed', 1)[1].split('\n      # Offline docs', 1)[0]
            self.assertIn('          GOTOOLCHAIN: local\n', feed)
            self.assertIn('test "$(go env GOVERSION)" = go1.26.11', feed)
            self.assertLess(text.index('go-version: "1.26.11"'), text.index('go run ./cmd/agentplugins-registry-mirror'))
            self.assertLess(text.index('cp -a "${RUNNER_TEMP}/uap-registry-mirror/." landing/public/'), text.index('go-version-file: go.mod'))
            self.assertLess(text.index('go-version-file: go.mod'), text.index('pnpm/action-setup'))
            self.assertLess(text.index('go-version-file: go.mod'), text.index('Install pinned docs generators'))
            if name == 'registry-compatibility-mirror.yml':
                self.assertLess(text.index('Resolve registry source'), text.index('Verify and stage signed feeds'))
            self.assertIn('go run ./cmd/agentplugins-registry-mirror', text)
            self.assertIn('--previous-marker "$previous"', text)
            self.assertIn('cp -a "${RUNNER_TEMP}/uap-registry-mirror/." landing/public/', text)
            self.assertIn('UAP_VERIFIED_MIRROR: ${{ runner.temp }}/uap-registry-mirror', text)
            self.assertLess(text.index('check-published-capabilities.mjs'), text.index('production_site_source.py" seal'))
            self.assertLess(text.index('production_site_source.py" seal'), text.index('actions/upload-pages-artifact'))

    def test_live_consumer_selects_pages_control_pin_before_dependencies(self):
        text = (REPO / '.github/workflows/skillspector-live-acceptance.yml').read_text()
        publication, live = text.split('  live:', 1)
        self.assertNotIn('production_site_source.py', publication)
        self.assertIn('ref: ${{ github.event.pull_request.head.sha }}', publication)
        self.assertIn("EXPECTED_CONTROL_SHA: ${{ github.event_name == 'workflow_run' && github.event.workflow_run.head_sha || github.sha }}", live)
        self.assertLess(live.index('production_site_source.py select'), live.index('pnpm/action-setup'))
        self.assertIn('echo "commit=$(git rev-parse HEAD)" >> "$GITHUB_OUTPUT"', live)
        self.assertEqual(live.count('UAP_LANDING_DEPLOYMENT_SHA: ${{ steps.production-source.outputs.commit }}'), 2)
        self.assertIn('fetch-depth: 0', live)
        self.assertIn('--expected-control "$EXPECTED_CONTROL_SHA"', live)
        self.assertNotIn('          GITHUB_SHA:', live)
        self.assertIn('production-site-control/receipt.json', live)


if __name__ == '__main__':
    unittest.main()
