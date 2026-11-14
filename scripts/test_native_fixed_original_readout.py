"""Inert archive bytes only; these contracts do not execute Grok or qualify its protocol."""
import base64
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import struct
import sys
import tempfile
import unittest
from unittest import mock
import warnings
import zipfile
import zlib

spec = importlib.util.spec_from_file_location('reader', Path(__file__).with_name('native_fixed_original_readout.py'))
reader = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reader)

class FixedOriginalTests(unittest.TestCase):
    def packed(self, members, compression=zipfile.ZIP_DEFLATED):
        output = io.BytesIO()
        with zipfile.ZipFile(output, 'w', compression=compression) as archive:
            for name, body in members.items():
                archive.writestr(name, body)
        return output.getvalue()

    def fixture(self):
        nonce = 'a' * 32
        initial = {'jsonrpc': '2.0', 'id': 1, 'result': {'protocolVersion': 1}}
        notices = [{'jsonrpc': '2.0', 'method': '_x.ai/session/setup',
                    'params': {'method': 'session/new', 'phase': 'auth', 'sessionId': None}}
                   for _ in range(11)]
        selected = json.dumps({'jsonrpc': '2.0', 'method': 'session/update',
                               'params': {'inert': 'ONLY_HASHED_FRAME'}}).encode()
        wire = b'\n'.join([json.dumps(initial).encode(),
                           *(json.dumps(row).encode() for row in notices), selected]) + b'\n'
        rpc = dict(initialize=initial, plugins=None, notifications=notices, session=None)
        members = {'a.rpc.json': json.dumps(rpc).encode(), 'a-acp.merged.log': wire}
        commands = []
        for index, label in enumerate(reader.LABELS):
            name = 'a-' + label + '.merged.log'
            if index < 4:
                members[name] = b'grok 1.0.70 (f50904eaee6c)\n' if index == 0 else b'PRIVATE_HELP_TEXT\n'
            commands.append(dict(label=label, argv=['/tmp/uap-grok-acp-sealed-PRIVATE/grok', *reader.ARGUMENTS[index]],
                                 exit=0 if index < 4 else -9, error=None if index < 4 else 'protocol-refused',
                                 merged_file=name, merged_bytes=len(members[name]), merged_sha256=reader.digest(members[name])))
        identity = dict(schema_version=1, source_commit=reader.SOURCE, source_tree=reader.TREE, scope='a', nonce=nonce)
        isolation = dict(uid=1001, gid=1001, euid=1001, egid=1001, groups=[], network_namespace=42,
                         parent_network_namespace=43, interfaces=['lo'], capabilities_zero=True,
                         no_new_privileges=1, loopback_up=True)
        result = dict(identity, semantic_complete=False, reason='protocol-refused', isolation=isolation,
                      commands=commands, mode=reader.MODE,
                      provider=dict(requests=2, models_get=1, root_get=1, forbidden=0, settled=True))
        executables = {name: dict(path='/usr/bin/' + ('python3.12' if name == 'python3' else name),
                                 sha256='1' * 64) for name in ('unshare', 'setpriv', 'env', 'python3')}
        receipt = dict(identity, cleanup_complete=True, sealed_cleanup_complete=True,
                       system_executables=executables, result=result)
        summary = dict(schema_version=1, source_commit=reader.SOURCE, source_tree=reader.TREE, nonce=nonce,
                       run=reader.RUN, attempt='1', mode=reader.MODE, collection_complete=False,
                       cleanup_complete=True, measurement_complete=True, error='admission-refused',
                       qualification=dict.fromkeys(reader.FLAGS, False), scopes=[receipt])
        acquired = dict(url='https://x.ai/cli/grok-1.0.70-linux-x86_64.gz', resolved_origin='https://PRIVATE.invalid',
                        bytes=76760282, sha256=reader.PIN, integrity='sha256:' + reader.PIN,
                        pin_verified=True, installer_executed=False,
                        members=[dict(name='grok', kind='file', bytes=1024, executable=True, sha256=reader.BINARY)])
        ledger = dict(before={'private': [1]}, after={'private': [2]}, accepted=False,
                      protected_originals_unchanged=False, selected_config_capture={'PRIVATE_CONFIG': 'must not be projected'})
        members.update({'admission.json': json.dumps(summary).encode(),
                        'acquisition.json': json.dumps(acquired).encode(), 'a.ledger.json': json.dumps(ledger).encode()})
        return members, summary, selected

    def pins(self, body, frame):
        return mock.patch.multiple(reader, ARCHIVE_SIZE=len(body), ARCHIVE_HASH=reader.digest(body),
                                   FRAME_SIZE=len(frame), FRAME_HASH=reader.digest(frame))

    def archive(self, members):
        return self.packed({'research.zip': self.packed(members)})

    def set_summary(self, members, summary):
        members['admission.json'] = json.dumps(summary).encode()

    def refresh_wire(self, members, summary):
        row = summary['scopes'][0]['result']['commands'][-1]
        body = members['a-acp.merged.log']
        row.update(merged_bytes=len(body), merged_sha256=reader.digest(body))
        self.set_summary(members, summary)

    def test_complete_real_nested_zip_closed_projection_and_no_native_verdict(self):
        members, _, frame = self.fixture()
        body = self.archive(members)
        with self.pins(body, frame):
            result = reader.audit(body)
        self.assertEqual(base64.b64decode(result['frame_base64'], validate=True), frame)
        self.assertEqual(result['command_streams_joined'], 5)
        self.assertEqual(result['prefix_frames_joined'], 12)
        self.assertEqual(result['inner_members'], 9)
        self.assertTrue(result['crc_verified'])
        self.assertTrue(result['original_failed'])
        self.assertFalse(result['native_qualified'])
        self.assertFalse(result['original_semantic_complete'])
        self.assertTrue(all(value is False for value in result['qualification'].values()))
        encoded = reader.encode(result)
        self.assertLessEqual(len(encoded), 8192)
        self.assertNotIn(b'PRIVATE', encoded)

    def test_original_fixed_outer_identity_is_checked_before_zip_parser(self):
        with mock.patch.object(reader, 'unpack') as unpack:
            for raw in (b'', b'x' * reader.ARCHIVE_SIZE, b'x' * (reader.ARCHIVE_SIZE + 1)):
                with self.subTest(size=len(raw)), self.assertRaises(ValueError):
                    reader.audit(raw)
            unpack.assert_not_called()

    def test_unknown_missing_duplicate_member_and_local_crc_refuse(self):
        for members in ({'../research.zip': b'x'}, {'research.zip': b'x', 'extra': b'y'}, {}):
            with self.subTest(members=list(members)), self.assertRaises(ValueError):
                reader.unpack(self.packed(members), {'research.zip'})
        out = io.BytesIO()
        with warnings.catch_warnings():
            warnings.simplefilter('ignore', UserWarning)
            with zipfile.ZipFile(out, 'w') as archive:
                archive.writestr('research.zip', b'one')
                archive.writestr('research.zip', b'two')
        with self.assertRaises(ValueError):
            reader.unpack(out.getvalue(), {'research.zip'})
        original = self.packed({'research.zip': b'exact stored bytes'}, zipfile.ZIP_STORED)
        corrupt = bytearray(original)
        corrupt[original.index(b'exact stored bytes')] ^= 1
        with self.assertRaises((ValueError, zipfile.BadZipFile)):
            reader.unpack(bytes(corrupt), {'research.zip'})

    def test_eocd_bounds_apply_before_zipfile_and_archive_comment_refuses(self):
        raw = bytearray(self.packed({'research.zip': b'body'}))
        struct.pack_into('<H', raw, len(raw) - 12, 65535)
        with mock.patch.object(reader.zipfile, 'ZipFile') as parser:
            with self.assertRaises(ValueError):
                reader.unpack(bytes(raw), {'research.zip'})
            parser.assert_not_called()
        original = self.packed({'research.zip': b'body'})
        with self.assertRaises(ValueError):
            reader.unpack(original + b'comment', {'research.zip'})

    def test_member_type_encryption_and_expansion_bounds_refuse(self):
        output = io.BytesIO()
        info = zipfile.ZipInfo('research.zip')
        info.external_attr = (0o120777 << 16)
        with zipfile.ZipFile(output, 'w') as archive:
            archive.writestr(info, b'target')
        with self.assertRaises(ValueError):
            reader.unpack(output.getvalue(), {'research.zip'})
        original = bytearray(self.packed({'research.zip': b'body'}))
        central = original.index(b'PK\x01\x02')
        for offset in (6, central + 8):
            flags = struct.unpack_from('<H', original, offset)[0]
            struct.pack_into('<H', original, offset, flags | 1)
        with self.assertRaises(ValueError):
            reader.unpack(bytes(original), {'research.zip'})
        with self.assertRaises(ValueError):
            reader.unpack(self.packed({'research.zip': b'x' * ((1 << 20) + 1)}), {'research.zip'})

    def test_actual_deflate_prefix_forgery_and_trailing_bytes_refuse(self):
        data = b'actual complete expanded bytes'
        original = self.packed({'research.zip': data})
        self.assertEqual(reader.unpack(original, {'research.zip'})['research.zip'], data)
        central = original.index(b'PK\x01\x02')
        forged = bytearray(original)
        prefix = data[:3]
        for offset in (14, central + 16):
            struct.pack_into('<I', forged, offset, zlib.crc32(prefix))
        struct.pack_into('<I', forged, 22, len(prefix))
        struct.pack_into('<I', forged, central + 24, len(prefix))
        with self.assertRaises(ValueError):
            reader.unpack(bytes(forged), {'research.zip'})
        with zipfile.ZipFile(io.BytesIO(original)) as archive:
            info = archive.infolist()[0]
            start = 30 + len(info.filename)
            reader.check_expansion(original, start, info)
            info.compress_size += 1
            with self.assertRaises(ValueError):
                reader.check_expansion(original, start, info)

    def test_closed_source_cleanup_qualification_and_native_failure_types(self):
        mutations = [
            (('schema_version',), True), (('source_commit',), '0' * 40), (('source_tree',), '0' * 40),
            (('run',), 'other'), (('attempt',), 1), (('mode',), 'other'), (('nonce',), 'bad'),
            (('collection_complete',), True), (('cleanup_complete',), False),
            (('measurement_complete',), 1), (('error',), None), (('extra',), 'PRIVATE'),
            (('qualification', reader.FLAGS[0]), 0), (('qualification', reader.FLAGS[-1]), True),
            (('scopes', 0, 'source_commit'), '0' * 40), (('scopes', 0, 'nonce'), 'b' * 32),
            (('scopes', 0, 'cleanup_complete'), 1), (('scopes', 0, 'sealed_cleanup_complete'), False),
            (('scopes', 0, 'result', 'semantic_complete'), True), (('scopes', 0, 'result', 'reason'), 'complete'),
            (('scopes', 0, 'result', 'mode'), 'other'),
            (('scopes', 0, 'result', 'isolation', 'euid'), 0),
            (('scopes', 0, 'result', 'isolation', 'no_new_privileges'), True),
            (('scopes', 0, 'result', 'provider', 'settled'), False),
            (('scopes', 0, 'result', 'provider', 'forbidden'), True),
            (('scopes', 0, 'system_executables', 'env', 'path'), '/PRIVATE/env'),
        ]
        for path, value in mutations:
            with self.subTest(path=path):
                members, summary, frame = self.fixture()
                target = summary
                for part in path[:-1]:
                    target = target[part]
                target[path[-1]] = value
                self.set_summary(members, summary)
                body = self.archive(members)
                with self.pins(body, frame), self.assertRaises(ValueError):
                    reader.audit(body)

    def test_all_command_bytes_argv_exit_and_hashes_are_joined(self):
        mutations = [('merged_bytes', True), ('merged_sha256', '0' * 64),
                     ('merged_file', 'unknown'), ('label', 'other'), ('error', 'deadline'),
                     ('argv', ['/PRIVATE/grok', 'agent', '--no-leader', 'stdio'])]
        for key, value in mutations:
            with self.subTest(key=key):
                members, summary, frame = self.fixture()
                summary['scopes'][0]['result']['commands'][-1][key] = value
                self.set_summary(members, summary)
                body = self.archive(members)
                with self.pins(body, frame), self.assertRaises((ValueError, KeyError)):
                    reader.audit(body)
        members, summary, frame = self.fixture()
        summary['scopes'][0]['result']['commands'][0]['exit'] = True
        self.set_summary(members, summary)
        body = self.archive(members)
        with self.pins(body, frame), self.assertRaises(ValueError):
            reader.audit(body)

    def test_wire_prefix_exact_frame_position_and_stored_rpc_refuse_drift(self):
        for mode in ('tail', 'unterminated', 'frame-copy', 'rpc-copy', 'session-reply'):
            with self.subTest(mode=mode):
                members, summary, frame = self.fixture()
                if mode == 'tail':
                    members['a-acp.merged.log'] += b'{}\n'
                elif mode == 'unterminated':
                    members['a-acp.merged.log'] = members['a-acp.merged.log'][:-1]
                elif mode == 'frame-copy':
                    members['a-acp.merged.log'] = members['a-acp.merged.log'].replace(frame, b'{}')
                else:
                    rpc = json.loads(members['a.rpc.json'])
                    if mode == 'rpc-copy':
                        rpc['initialize']['result']['different'] = True
                    else:
                        rpc['session'] = {'private': True}
                    members['a.rpc.json'] = json.dumps(rpc).encode()
                self.refresh_wire(members, summary)
                body = self.archive(members)
                with self.pins(body, frame), self.assertRaises(ValueError):
                    reader.audit(body)

    def test_acquisition_and_ledger_guards_preserve_negative_readout(self):
        for name, key, value in (
            ('acquisition.json', 'pin_verified', 1),
            ('acquisition.json', 'installer_executed', True),
            ('acquisition.json', 'sha256', '0' * 64),
            ('a.ledger.json', 'accepted', True),
            ('a.ledger.json', 'protected_originals_unchanged', True),
        ):
            with self.subTest(name=name, key=key):
                members, _, frame = self.fixture()
                value_object = json.loads(members[name])
                value_object[key] = value
                members[name] = json.dumps(value_object).encode()
                body = self.archive(members)
                with self.pins(body, frame), self.assertRaises(ValueError):
                    reader.audit(body)

    def test_duplicate_json_nonfinite_wrong_types_and_output_bound_refuse(self):
        for body in (b'{"a":1,"a":2}', b'{"a":NaN}', b'{"a":Infinity}', b'\xff'):
            with self.subTest(body=body), self.assertRaises((ValueError, UnicodeError)):
                reader.document(body)
        with self.assertRaises(ValueError):
            reader.encode({'private': 'x' * 8192})
        with self.assertRaises(ValueError):
            reader.closed([], '')

    def test_cli_mismatch_or_oversized_projection_emits_only_closed_refusal(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / 'original.zip'
            path.write_bytes(b'not the original')
            for large in (False, True):
                output = io.StringIO()
                patches = contextlib.ExitStack()
                with patches:
                    patches.enter_context(mock.patch.object(sys, 'argv', ['reader', reader.CASE, str(path)]))
                    if large:
                        patches.enter_context(mock.patch.object(reader, 'audit', return_value={'frame_base64': 'PRIVATE' * 2048}))
                    patches.enter_context(contextlib.redirect_stdout(output))
                    self.assertEqual(reader.main(), 1)
                result = json.loads(output.getvalue())
                self.assertEqual(result, dict(kind='fixed-original-readout', case=reader.CASE,
                                              refused=True, native_qualified=False))
                self.assertLessEqual(len(output.getvalue().encode()), 8192)
                self.assertNotIn('PRIVATE', output.getvalue())
                self.assertNotIn('frame_base64', result)

if __name__ == '__main__':
    unittest.main()
