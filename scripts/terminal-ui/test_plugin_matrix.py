"""Fixture validation plus opt-in actual PTY integration (never mocked CLI)."""
import json
import http.client
from urllib.parse import urlsplit
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import plugin_matrix as lane


class FixtureValidation(unittest.TestCase):
    def test_platform_discovery_paths(self):
        for system, editor in (('Linux', 'config'), ('Darwin', 'Library/Application Support')):
            with self.subTest(system=system), tempfile.TemporaryDirectory() as tmp:
                fixture = lane.Fixture(tmp)
                before = fixture.mutations()
                paths = lane.ten_client_paths(fixture, system)
                self.assertEqual({str(p.relative_to(fixture.home)) for p in paths}, {
                    '.copilot', '.kiro', '.claude', '.gemini/.gemini',
                    editor + '/Code/User/globalStorage/saoudrizwan.claude-dev',
                    'config/opencode', '.codeium/windsurf'})
                self.assertEqual(fixture.mutations(), before)
                self.assertTrue(all(p.resolve().is_relative_to(fixture.home.resolve()) for p in paths))

    def test_native_seed_preserves_isolation_and_version_only_stubs(self):
        with tempfile.TemporaryDirectory() as tmp:
            fixture = lane.Fixture(tmp)
            env = dict(fixture.env)
            with patch.object(lane, 'ten_client_paths', wraps=lane.ten_client_paths) as paths:
                lane.seed_ten_clients(fixture)
                paths.assert_called_once_with(fixture, lane.platform.system())
            self.assertEqual(fixture.env, env)
            self.assertTrue(all(p.is_dir() for p in lane.ten_client_paths(fixture, lane.platform.system())))
            self.assertEqual({p.name for p in fixture.bin.iterdir()}, {
                'codex', 'cursor', 'copilot', 'code', 'kiro-cli', 'claude', 'gemini', 'opencode', 'windsurf'})
            for stub in fixture.bin.iterdir():
                self.assertEqual(stub.read_bytes(), (fixture.bin / 'cursor').read_bytes())
                self.assertTrue(os.access(stub, os.X_OK))
            fixture.unchanged()

    def test_discovery_rejects_unsupported_platform_and_external_roots(self):
        with tempfile.TemporaryDirectory() as tmp:
            fixture = lane.Fixture(tmp)
            with self.assertRaisesRegex(AssertionError, 'Linux and Darwin'):
                lane.ten_client_paths(fixture, 'Windows')
            fixture.env['XDG_CONFIG_HOME'] = str(fixture.root / 'outside-home')
            for system in ('Linux', 'Darwin'):
                with self.subTest(system=system), self.assertRaisesRegex(AssertionError, 'escapes'):
                    lane.ten_client_paths(fixture, system)

    def test_standard_fixture_contracts(self):
        for kind in lane.KINDS:
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as tmp:
                fixture = lane.Fixture(tmp)
                lane.package(fixture, kind, 'http://127.0.0.1:12345/mcp')
                if kind == 'malformed':
                    with self.assertRaises(json.JSONDecodeError):
                        json.loads((fixture.package / 'plugin.json').read_text())
                    continue
                manifest = json.loads((fixture.package / 'plugin.json').read_text())
                self.assertEqual(manifest['$schema'], lane.SCHEMA + 'plugin.schema.json')
                self.assertEqual(manifest['name'], 'pty-synthetic')
                if kind in ('skill', 'mixed', 'collision'):
                    body = (fixture.package / 'skills/guide/SKILL.md').read_text()
                    self.assertTrue(body.startswith('---\nname: guide\ndescription: '))
                    self.assertIn('reference.txt', body)
                    self.assertTrue((fixture.package / 'skills/guide/reference.txt').is_file())
                if kind in ('stdio-missing', 'mixed', 'http-auth'):
                    mcp = json.loads((fixture.package / 'mcp.json').read_text())
                    server = mcp['mcpServers']['fixture']
                    if kind == 'http-auth':
                        self.assertEqual(set(server), {'type', 'url'})
                    else:
                        self.assertEqual(server['command'], 'uap-fixture-missing-runtime')
                        self.assertFalse((fixture.bin / server['command']).exists())
                fixture.unchanged()

    def test_controlled_endpoint_requires_auth_without_credentials(self):
        with lane.local_endpoint(True) as (url, requests):
            parsed = urlsplit(url)
            connection = http.client.HTTPConnection(parsed.hostname, parsed.port, timeout=2)
            try:
                connection.request('POST', parsed.path, body=b'{}')
                response = connection.getresponse()
                self.assertEqual(response.status, 401)
                self.assertIn('Bearer', response.getheader('WWW-Authenticate'))
                response.read()
            finally:
                connection.close()
            self.assertEqual(requests, [{'method': 'POST', 'path': '/mcp'}])

    def test_case_contract(self):
        self.assertEqual(len(lane.CASES), 19)
        self.assertEqual(len(lane.CASES), len(set(lane.CASES)))
        self.assertIn('empty:all-ten', lane.CASES)
        self.assertIn('skill:install', lane.CASES)
        for kind in ('stdio-missing', 'collision', 'malformed'):
            self.assertIn(kind + ':reject', lane.CASES)
        self.assertIn('mixed:partial-plan', lane.CASES)
        self.assertIn('http-auth:auth-unknown', lane.CASES)
        self.assertNotIn('mixed:reject', lane.CASES)
        self.assertNotIn('http-auth:reject', lane.CASES)

    def test_auth_uncertainty_does_not_imply_runtime_verification(self):
        lane.check_auth_unknown({'authentication': 'not_checked', 'verification': 'package_validated'})
        for auth, verification in (('not_required', 'package_validated'), ('verified', 'package_validated'),
                                   ('not_checked', 'installed')):
            with self.subTest(auth=auth, verification=verification), self.assertRaises(AssertionError):
                lane.check_auth_unknown({'authentication': auth, 'verification': verification})

    def test_choice_parser_rejects_diagnostic_only_client(self):
        self.assertEqual(lane.choices(b'Warning: cursor (cursor)\n'), [])
        self.assertEqual(lane.choices('┃ > [•] OpenAI Codex (codex)\r\n┃ [•] Cursor (cursor)'.encode()), ['codex', 'cursor'])

    def test_external_endpoint_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(AssertionError, 'loopback'):
                lane.package(lane.Fixture(tmp), 'http-auth', 'https://external.example/mcp')


@unittest.skipUnless(os.environ.get('PLUGIN_MATRIX_BINARY'), 'set PLUGIN_MATRIX_BINARY for actual PTY')
class RealPTY(unittest.TestCase):
    def test_skill_keyboard_lifecycle(self):
        with tempfile.TemporaryDirectory() as tmp:
            lane.run_case('skill:install', Path(os.environ['PLUGIN_MATRIX_BINARY']).resolve(), Path(tmp) / 'evidence')


if __name__ == '__main__':
    unittest.main()
