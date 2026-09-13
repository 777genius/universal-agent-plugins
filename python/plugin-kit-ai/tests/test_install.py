"""Cold-cache wrapper regressions, without network or user cache access."""
import hashlib
import io
import os
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

from plugin_kit_ai import install


class ReleaseTags(unittest.TestCase):
    def test_numeric_and_product_tags(self):
        for raw, tag, version in [
            ('2.0.2', 'plugin-kit-ai-v2.0.2', '2.0.2'),
            ('plugin-kit-ai-v2.0.2', 'plugin-kit-ai-v2.0.2', '2.0.2'),
            ('1.2.4', 'v1.2.4', '1.2.4'),
            ('v1.2.4', 'v1.2.4', '1.2.4'),
            ('plugin-kit-ai-v1.2.4', 'plugin-kit-ai-v1.2.4', '1.2.4'),
        ]:
            with self.subTest(raw=raw):
                self.assertEqual(install.normalize_tag(raw), tag)
                self.assertEqual(install.version_from_tag(tag), version)

    def test_reject_wrong_product_and_malformed_tags(self):
        for tag in ['agentplugins-v0.1.62', 'v2.0.2', '2.00.2', '2.0',
                    'plugin-kit-ai-plugin-kit-ai-v2.0.2', '2.0.2-rc.1',
                    '2.0.2+build', '../2.0.2', 'vlatest', '2.0.2\n',
                    ' plugin-kit-ai-v2.0.2']:
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                install.normalize_tag(tag)

    def test_packaged_version_and_latest_product_validation(self):
        with patch.dict(os.environ, {}, clear=True), patch.object(install, '__version__', '2.0.2'):
            self.assertEqual(install.resolve_requested_tag(), 'plugin-kit-ai-v2.0.2')
        with patch.object(install, 'fetch_text', return_value='{"tag_name":"agentplugins-v0.1.62"}'):
            with self.assertRaises(ValueError):
                install.latest_tag(install.DEFAULT_API_BASE, install.DEFAULT_REPOSITORY)

    def test_cold_cache_and_checksum(self):
        platform = install.detect_platform()
        stream = io.BytesIO()
        binary = b'fixture executable bytes'
        with tarfile.open(fileobj=stream, mode='w:gz') as archive:
            member = tarfile.TarInfo(platform.binary_name)
            member.size = len(binary)
            archive.addfile(member, io.BytesIO(binary))
        body = stream.getvalue()
        for raw in ['2.0.2', 'plugin-kit-ai-v2.0.2', '1.2.4', 'v1.2.4']:
            for bad_checksum in [False, True]:
                with self.subTest(raw=raw, bad_checksum=bad_checksum), tempfile.TemporaryDirectory() as root:
                    tag = install.normalize_tag(raw)
                    version = install.version_from_tag(tag)
                    asset = install.asset_name_for_version(version, platform)
                    base = f'https://github.com/{install.DEFAULT_REPOSITORY}/releases/download/{tag}'
                    checksum = '0' * 64 if bad_checksum else hashlib.sha256(body).hexdigest()
                    def fetch(url, accept_json=False):
                        self.assertIn(url, [f'{base}/checksums.txt', f'{base}/{asset}'])
                        return f'{checksum}  {asset}\n'.encode() if url.endswith('/checksums.txt') else body
                    with patch.dict(os.environ, {'PLUGIN_KIT_AI_VERSION': raw,
                                                 'PLUGIN_KIT_AI_CACHE_DIR': root}, clear=True), \
                            patch.object(install, 'fetch_bytes', side_effect=fetch) as downloaded:
                        if bad_checksum:
                            with self.assertRaisesRegex(RuntimeError, 'checksum mismatch'):
                                install.ensure_installed(quiet=True)
                            self.assertFalse((Path(root) / tag / platform.binary_name).exists())
                        else:
                            result = install.ensure_installed(quiet=True)
                            self.assertEqual(result['version'], version)
                            self.assertEqual(Path(result['installed_binary']).read_bytes(), binary)
                            self.assertEqual(downloaded.call_count, 2)
                            install.ensure_installed(quiet=True)
                            self.assertEqual(downloaded.call_count, 2)


if __name__ == '__main__':
    unittest.main()
