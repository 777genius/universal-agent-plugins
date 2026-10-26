"""Inert cache handoff contracts; no installer, npm, historical or vendor runs."""
import contextlib
import hashlib
import shlex
import io
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import harness as h
import historical_state_flow as historical
import npm_packed_flow as npm_flow
import plugin_matrix
import provision_scanner
import resilience_matrix
import selection_matrix
import ui_regressions
import windows_conpty


class BeforeExecution(Exception):
    pass


class CacheFlowTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.binary = self.root / 'inert-binary'
        self.binary.write_bytes(b'inert supplied bytes; never executable')
        self.digest = hashlib.sha256(self.binary.read_bytes()).hexdigest()
        self.system = patch.object(h.platform, 'system', return_value='Linux')
        self.machine = patch.object(h.platform, 'machine', return_value='x86_64')
        self.system.start(); self.machine.start()
        self.addCleanup(self.system.stop); self.addCleanup(self.machine.stop)
        metadata = patch.object(h.platform, 'platform', return_value='inert platform metadata')
        metadata.start(); self.addCleanup(metadata.stop)

    def quiet(self):
        stack = contextlib.ExitStack()
        stack.enter_context(contextlib.redirect_stdout(io.StringIO()))
        stack.enter_context(contextlib.redirect_stderr(io.StringIO()))
        return stack

    def test_manual_entrypoints_forward_declared_cache_before_child_execution(self):
        for lane in (h, selection_matrix, plugin_matrix, ui_regressions, resilience_matrix):
            with self.subTest(lane=lane.__name__):
                artifacts = self.root / lane.__name__
                args = [lane.__name__, '--binary', str(self.binary), '--artifacts', str(artifacts),
                        '--case', lane.CASES[0], '--scanner-cache-platform', 'linux-amd64']
                if lane is selection_matrix:
                    args += ['--binary-sha256', self.digest]
                with patch.object(sys, 'argv', args), self.quiet(), \
                     patch.object(lane, 'Fixture', side_effect=BeforeExecution) as fixture, \
                     patch.object(lane.subprocess, 'run') as child, \
                     patch.object(lane.subprocess, 'check_output', return_value='1'*40) as metadata:
                    extra = patch.object(selection_matrix, 'source_digest', return_value='source')
                    with extra:
                        try:
                            lane.main()
                        except BeforeExecution:
                            pass
                    fixture.assert_called_once()
                    self.assertEqual(fixture.call_args.kwargs['scanner_cache_platform'], 'linux-amd64')
                    child.assert_not_called()

    def test_plugin_report_repro_preserves_resolved_cache_route(self):
        for declared in (None, 'linux-amd64'):
            with self.subTest(declared=declared):
                artifacts = self.root / ('plugin-default' if declared is None else 'plugin-legacy')
                argv = ['plugin', '--binary', str(self.binary), '--artifacts', str(artifacts),
                        '--case', plugin_matrix.CASES[0]]
                if declared:
                    argv += ['--scanner-cache-platform', declared]
                with patch.object(sys, 'argv', argv), self.quiet(), \
                     patch.object(plugin_matrix, 'run_case') as case, \
                     patch.object(plugin_matrix.subprocess, 'run') as child, \
                     patch.object(plugin_matrix.subprocess, 'check_output', return_value='1'*40):
                    self.assertEqual(plugin_matrix.main(), 0)
                    child.assert_not_called()
                report = json.loads((artifacts / 'report.json').read_text())
                expected = declared or 'linux-amd64-musl'
                self.assertEqual(report['scanner_cache_platform'], expected)
                self.assertEqual(case.call_args.kwargs['scanner_cache_platform'], expected)
                command = shlex.split(report['cases'][0]['repro'])
                self.assertEqual(command[command.index('--scanner-cache-platform') + 1], expected)
                self.assertEqual(command[command.index('--binary') + 1], str(self.binary))

    def test_selection_source_build_refuses_legacy_route_before_build_or_fixture(self):
        argv = ['selection', '--build-go', '/not-executed/go', '--artifacts', str(self.root/'source'),
                '--scanner-cache-platform', 'linux-amd64']
        with patch.object(sys, 'argv', argv), patch.object(selection_matrix.subprocess, 'run') as child, \
             patch.object(selection_matrix, 'source_digest') as source, \
             patch.object(selection_matrix, 'Fixture') as fixture:
            with self.assertRaisesRegex(AssertionError, 'current-source build'):
                selection_matrix.main()
            child.assert_not_called(); fixture.assert_not_called(); source.assert_not_called()
        self.assertFalse((self.root/'source').exists())

    def test_resilience_nested_fixture_and_windows_controller_forward_key(self):
        matrix = resilience_matrix.Matrix(self.binary, 1, scanner_cache_platform='linux-amd64')
        outer = SimpleNamespace(root=self.root)
        with patch.object(resilience_matrix, 'Fixture', side_effect=BeforeExecution) as fixture:
            with self.assertRaises(BeforeExecution):
                matrix.case_per_client_lifecycle(outer, self.root/'nested')
            self.assertEqual(fixture.call_args.kwargs['scanner_cache_platform'], 'linux-amd64')
        args = SimpleNamespace(artifacts=self.root, scanner_path=None, scanner_cache_platform='linux-amd64')
        with patch.object(windows_conpty, 'Fixture', side_effect=BeforeExecution) as fixture, \
             patch.object(windows_conpty, 'ConPTY') as terminal:
            with self.assertRaises(BeforeExecution):
                windows_conpty.run_case('cache-forwarding', args)
            self.assertEqual(fixture.call_args.kwargs['scanner_cache_platform'], 'linux-amd64')
            terminal.assert_not_called()

    def test_all_manual_parsers_reject_wrong_host_before_fixture_or_child(self):
        for lane in (h, selection_matrix, plugin_matrix, ui_regressions, resilience_matrix):
            argv = [lane.__name__, '--binary', str(self.binary), '--artifacts', str(self.root/('bad-'+lane.__name__)),
                    '--scanner-cache-platform', 'windows-amd64']
            if lane is selection_matrix:
                argv += ['--binary-sha256', self.digest]
            with self.subTest(lane=lane.__name__), patch.object(sys, 'argv', argv), \
                 patch.object(lane, 'Fixture') as fixture, patch.object(lane.subprocess, 'run') as child:
                with self.assertRaises(AssertionError):
                    lane.main()
                fixture.assert_not_called(); child.assert_not_called()

    def test_historical_flow_retains_one_legacy_synthetic_cache_and_command_order(self):
        for tag, version, schema in historical.RELEASES:
            with self.subTest(tag=tag):
                root = self.root/tag; root.mkdir()
                old = root/'old-inert'; old.write_bytes(b'never executed')
                calls, fixtures = [], []
                def build(*args):
                    return old, '1'*40
                def fixture(*args, **kwargs):
                    value = h.Fixture(*args, **kwargs); fixtures.append(value); return value
                def run(argv, cwd, env, evidence, label, expected):
                    calls.append(label)
                    state = Path(env['AGENTPLUGINS_HOME'])/'state-v2.json'
                    if label == 'historical-add':
                        historical.write_json(state, {'schema_version':schema, 'installations':[{'id':'inert'}]})
                    elif label == 'migrate-apply':
                        original = state.read_bytes()
                        state.with_name(state.name+f'.schema{schema}.backup-agentplugins-inert').write_bytes(original)
                        historical.write_json(state, {'schema_version':5, 'installations':[{'id':'inert'}]})
                    elif label == 'current-remove':
                        historical.write_json(state, {'schema_version':5, 'installations':[]})
                    return subprocess.CompletedProcess(argv, expected, json.dumps({'schema_version':1}),
                                                       'migrate-state required' if expected else '')
                with patch.object(historical, 'build_release', side_effect=build), \
                     patch.object(historical, 'Fixture', side_effect=fixture), \
                     patch.object(historical, 'run', side_effect=run):
                    row = historical.release_flow(self.binary, '/not-executed/go', tag, version, schema, root)
                self.assertEqual(calls, ['historical-add','current-update-before-migrate','migrate-dry-run','migrate-apply','current-remove'])
                self.assertEqual(len(fixtures), 1)
                caches = list((fixtures[0].data/'security/lintai/0.1.3').rglob('lintai'))
                self.assertEqual([x.parent.name for x in caches], ['linux-amd64'])
                self.assertEqual(row['scanner_cache_platform'], 'linux-amd64')
                self.assertIn('SYNTHETIC', row['scanner'])
                self.assertIn('not established', row['scanner'])

    def test_npm_supplied_binary_cache_handoff_and_provenance_without_children(self):
        for declared in (None, 'linux-amd64'):
            artifacts = self.root/('npm-default' if declared is None else 'npm-legacy')
            node = self.root/'inert-node'; node.write_bytes(b'never executed')
            argv = ['npm-flow','--binary',str(self.binary),'--node',str(node),'--npm','/not-executed/npm',
                    '--artifacts',str(artifacts)]
            if declared: argv += ['--scanner-cache-platform',declared]
            calls = []
            def run(evidence, label, command, cwd, env):
                calls.append(label)
                output = {'schema_version':1,'result':'success','data':{}}
                if label == 'npm-pack':
                    packed = artifacts/'packed/inert.tgz'; packed.write_bytes(b'inert tarball')
                    return subprocess.CompletedProcess(command,0,json.dumps([{'filename':packed.name}]),'')
                if label == 'npm-install':
                    package = artifacts/'consumer/node_modules/universal-agent-plugins/bin'
                    package.mkdir(parents=True); (package/'agentplugins.js').write_text('// inert')
                    bins = artifacts/'consumer/node_modules/.bin'; bins.mkdir()
                    (bins/'agentplugins').write_text('inert')
                elif label == 'version':
                    platform, _, name = npm_flow.platform_contract()
                    cached = Path(env['AGENTPLUGINS_CACHE_DIR'])/npm_flow.VERSION/platform/name
                    cached.parent.mkdir(parents=True); shutil.copyfile(env['AGENTPLUGINS_INTERNAL_PROOF_BINARY'],cached)
                    return subprocess.CompletedProcess(command,0,'agentplugins inert\n','')
                elif label == 'add': output['data']={'targets':[{'target':'cursor'}]}
                elif label.startswith('doctor-'): output['data']={'installation_count':int(label=='doctor-installed')}
                return subprocess.CompletedProcess(command,0,json.dumps(output),'')
            with patch.object(sys,'argv',argv), self.quiet(), patch.object(npm_flow,'run',side_effect=run), \
                 patch.object(npm_flow.subprocess,'check_output',return_value='1'*40), \
                 patch.object(npm_flow.subprocess,'run') as child:
                self.assertEqual(npm_flow.main(),0)
                child.assert_not_called()
            report=json.loads((artifacts/'report.json').read_text())
            self.assertEqual(report['scanner_cache_platform'], declared or 'linux-amd64-musl')
            self.assertIn('not established', report['binary_source_equivalence'])
            self.assertIn('SYNTHETIC', report['scanner'])
            staged=json.loads((artifacts/'staged-package/assets.json').read_text())
            self.assertEqual(staged['client_evidence'],npm_flow.historical_evidence())
            self.assertEqual(calls,['npm-pack','npm-install','version','add','doctor-installed','remove','doctor-removed'])
        argv=['npm-flow','--binary',str(self.binary),'--node','/unused','--npm','/unused',
              '--artifacts',str(self.root/'npm-invalid'),'--scanner-cache-platform','windows-amd64']
        with patch.object(sys,'argv',argv), patch.object(npm_flow,'Fixture') as fixture, \
             patch.object(npm_flow,'run') as child, self.assertRaises(AssertionError):
            npm_flow.main()
        fixture.assert_not_called(); child.assert_not_called()

    def test_source_provisioner_requests_the_asset_named_by_the_workflow(self):
        workflow=(Path(__file__).resolve().parents[2]/'.github/workflows/terminal-ui.yml').read_text()
        archive=re.search(r'target: linux-amd64\s+archive: ([^\s]+)',workflow).group(1)
        argv=['provision','--target','linux-amd64','--output',str(self.root/'provision')]
        with patch.object(sys,'argv',argv), patch.object(provision_scanner.urllib.request,'urlopen',return_value=io.BytesIO(b'inert-invalid-archive')) as request:
            with self.assertRaises(ValueError):
                provision_scanner.main()
        url=request.call_args.args[0]
        self.assertTrue(url.endswith('/'+archive))
        self.assertEqual(h.SCANNER_ARCHIVES[archive][1],'linux-amd64-musl')
        self.assertFalse((self.root/'provision').exists())


if __name__ == '__main__':
    unittest.main()
