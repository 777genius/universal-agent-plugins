"""Fixture validation plus opt-in actual PTY integration (never mocked CLI)."""
import json
import http.client
from urllib.parse import urlsplit
import os
from pathlib import Path
import tempfile
import unittest

import plugin_matrix as lane


class FixtureValidation(unittest.TestCase):
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
        self.assertEqual(len(lane.CASES), len(set(lane.CASES)))
        self.assertIn('skill:install', lane.CASES)
        for kind in ('stdio-missing', 'mixed', 'collision', 'http-auth', 'malformed'):
            self.assertIn(kind + ':reject', lane.CASES)

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
