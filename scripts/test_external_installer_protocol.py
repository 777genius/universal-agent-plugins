"""Read-only protocol tests; no native installer or client execution."""
import importlib.util
import json
from pathlib import Path
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location('protocol', Path(__file__).with_name('run-external-installer-protocol.py'))
protocol = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(protocol)


class ProtocolTests(unittest.TestCase):
    def test_document_fixture_is_exact_skills_only_source(self):
        document = (Path(__file__).resolve().parents[1] / 'docs/external-installer-testing.md').read_text()
        files = protocol.fixture(document)
        self.assertEqual(set(files), {'plugin.json', 'skills/uap-external-check/SKILL.md'})
        self.assertEqual(json.loads(files['plugin.json'])['version'], '1.0.0')
        self.assertIn(b'UAP_EXTERNAL_CHECK_OK', files['skills/uap-external-check/SKILL.md'])
        with self.assertRaises(ValueError): protocol.fixture(document + document)
        with self.assertRaises(ValueError): protocol.fixture(document.replace("UAP_VERSION='0.1.53'", "UAP_VERSION='latest'"))

    def test_native_identity_requires_one_exact_managed_enabled_plugin(self):
        path = Path('/disposable/home/managed/plugin')
        plugin = {'name': protocol.PLUGIN, 'pluginId': protocol.PLUGIN+'@agentplugins-123456abcdef',
                  'marketplaceName': 'agentplugins-123456abcdef', 'source': {'path':str(path)}, 'installed':True, 'enabled':True}
        self.assertEqual(protocol.native_identity({'installed':[plugin]},path), (plugin['pluginId'],plugin['marketplaceName']))
        for entries in ([], [plugin,plugin], [dict(plugin,enabled=False)], [dict(plugin,pluginId='foreign@market')], [dict(plugin,source={'path':'/foreign'})]):
            with self.assertRaises(ValueError): protocol.native_identity({'installed':entries},path)

    def test_state_locator_discovery_preserves_ambiguity(self):
        state={'installations':[{'clients':[{'target_locator':'/a'},{'target_locator':'/b'}]}]}
        self.assertEqual(protocol.target_paths(state),{'/a','/b'})

    def test_local_invocation_rejected_before_network_or_runtime(self):
        with patch.dict(protocol.os.environ, {}, clear=True), patch.object(protocol.subprocess,'run') as run:
            with self.assertRaises(RuntimeError): protocol.matrix.require_hosted('linux-arm64')
            run.assert_not_called()


if __name__ == '__main__':
    unittest.main()
