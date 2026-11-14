#!/usr/bin/env python3
"""Read one immutable failed Grok capture; never admit or execute a native client."""
import argparse
import base64
import hashlib
import io
import json
from pathlib import Path, PurePosixPath
import re
import runpy
import stat
import struct
import zipfile
import zlib

check_expansion = runpy.run_path(str(Path(__file__).with_name('read-authoring-evidence-zip.py')),
                               run_name='fixed_original_zip_checks')['check_expansion']
CASE = 'grok-5ff-session'
ARTIFACT = 14100597648
ARCHIVE_SIZE = 11865
ARCHIVE_HASH = '8c1d6a6a7dba38cc58882ffc8019c35fea4bdaf41808c644aff704dee409ff15'
RUN = '43559930011'
SOURCE = '5ff83587db83f613ebd8c784d2032f24e46b6bdb'
TREE = 'e667fdd088aae50259332f772c7eb23098889083'
FRAME_SIZE = 2493
FRAME_HASH = 'f9457d63ac0bb5d432a98790a84c119a3354b2ef53c181fd50600a6da5bfe533'
MODE = 'session-only-v1'
LABELS = ('version', 'help', 'agent-help', 'stdio-help', 'acp')
ARGUMENTS = (['--version'], ['--help'], ['agent', '--help'],
             ['agent', 'stdio', '--help'], ['agent', '--no-leader', 'stdio'])
MEMBERS = {'admission.json', 'acquisition.json', 'a.ledger.json', 'a.rpc.json'} | {
    'a-' + label + '.merged.log' for label in LABELS}
FLAGS = ('inventory_admitted', 'native_enablement', 'workspace_trust', 'native_mcp_call',
         'provider_authentication', 'real_model_inference', 'production_project_activation')
PIN = '11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc'
BINARY = '22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9'

def require(ok):
    if not ok:
        raise ValueError('fixed original refused')

def digest(body):
    return hashlib.sha256(body).hexdigest()

def closed(value, keys):
    require(type(value) is dict and set(value) == set(keys.split()))
    return value

def unique(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result)
        result[key] = value
    return result

def document(body, maximum=1 << 20):
    require(type(body) is bytes and len(body) <= maximum)
    return json.loads(body.decode('utf-8'), object_pairs_hook=unique,
                      parse_constant=lambda _: require(False))

def unpack(body, allowed):
    require(type(body) is bytes and 0 < len(body) <= 1 << 20)
    # No comment/ZIP64; allocation authority is checked before ZipFile constructs entries.
    require(len(body) >= 22 and body[-22:-18] == b"PK\x05\x06")
    disk, start_disk, on_disk, count, central_size, central_offset, comment = struct.unpack("<4H2IH", body[-18:])
    require(disk == start_disk == comment == 0 and on_disk == count and 0 < count <= len(allowed)
            and count != 65535 and central_size <= 16384 and central_offset + central_size == len(body) - 22)
    result, total, intervals = {}, 0, []
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        require(len(archive.infolist()) == count and not archive.comment)
        for info in archive.infolist():
            require(info.filename in allowed and info.filename not in result and not info.is_dir()
                    and not info.flag_bits & 1 and info.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)
                    and len(info.extra) <= 4096 and info.file_size <= 1 << 20 and info.compress_size <= len(body))
            offset = 0
            while offset < len(info.extra):
                require(offset + 4 <= len(info.extra))
                kind, size = struct.unpack_from("<HH", info.extra, offset)
                offset += 4
                require(kind != 1 and offset + size <= len(info.extra))
                offset += size
            mode = info.external_attr >> 16
            require(not stat.S_ISLNK(mode) and (not stat.S_IFMT(mode) or stat.S_ISREG(mode)))
            offset = info.header_offset
            require(0 <= offset and offset + 30 <= central_offset and body[offset:offset + 4] == b"PK\x03\x04")
            flags, method = struct.unpack_from("<HH", body, offset + 6)
            crc, compressed, expanded = struct.unpack_from("<III", body, offset + 14)
            require((flags, method) == (info.flag_bits, info.compress_type)
                    and ((crc, compressed, expanded) == (info.CRC, info.compress_size, info.file_size)
                         or flags & 8 and (crc, compressed, expanded) == (0, 0, 0)))
            name_size, extra_size = struct.unpack_from("<HH", body, offset + 26)
            require(extra_size == 0 and body[offset + 30:offset + 30 + name_size] == info.filename.encode("utf-8"))
            start = offset + 30 + name_size
            end = start + info.compress_size
            require(end <= central_offset)
            check_expansion(body, start, info)
            if flags & 8:
                if body[end:end + 4] == b"PK\x07\x08":
                    end += 4
                require(end + 12 <= central_offset and struct.unpack_from("<III", body, end)
                        == (info.CRC, info.compress_size, info.file_size))
                end += 12
            intervals.append((offset, end))
            total += info.file_size
            require(total <= 16 << 20)
            with archive.open(info) as source:
                data = source.read((1 << 20) + 1)
                require(len(data) == info.file_size and not source.read(1))
            result[info.filename] = data  # ZipExtFile verifies CRC on full bounded read.
    intervals.sort()
    require(intervals[0][0] == 0 and intervals[-1][1] == central_offset
            and all(a[1] == b[0] for a, b in zip(intervals, intervals[1:])))
    require(set(result) == set(allowed))
    return result

def identities(value, nonce):
    require(type(value['schema_version']) is int and value['schema_version'] == 1
            and value['source_commit'] == SOURCE and value['source_tree'] == TREE
            and value['scope'] == 'a' and value['nonce'] == nonce)

def acquisition(value):
    closed(value, 'url resolved_origin bytes sha256 integrity pin_verified members installer_executed')
    require(value['url'] == 'https://x.ai/cli/grok-1.0.70-linux-x86_64.gz'
            and type(value['bytes']) is int and value['bytes'] == 76760282
            and value['sha256'] == PIN and value['integrity'] == 'sha256:' + PIN
            and value['pin_verified'] is True and value['installer_executed'] is False)
    require(type(value['resolved_origin']) is str and len(value['resolved_origin']) <= 256
            and re.fullmatch(r'https://[A-Za-z0-9.-]+', value['resolved_origin']))
    require(type(value['members']) is list and len(value['members']) == 1)
    row = closed(value['members'][0], 'name kind bytes executable sha256')
    require(row['name'] == 'grok' and row['kind'] == 'file' and row['executable'] is True
            and type(row['bytes']) is int and 0 < row['bytes'] <= 256 << 20
            and row['sha256'] == BINARY)

def isolation(value):
    closed(value, 'uid gid euid egid groups network_namespace parent_network_namespace interfaces '
                  'capabilities_zero no_new_privileges loopback_up')
    require(all(type(value[k]) is int and 0 < value[k] < 1 << 64 for k in
                ('uid', 'gid', 'euid', 'egid', 'network_namespace', 'parent_network_namespace'))
            and value['uid'] == value['euid'] and value['gid'] == value['egid']
            and value['groups'] == [] and value['interfaces'] == ['lo']
            and value['capabilities_zero'] is True and value['loopback_up'] is True
            and type(value['no_new_privileges']) is int and value['no_new_privileges'] == 1
            and value['network_namespace'] != value['parent_network_namespace'])

def streams(result, members):
    rows = result['commands']
    require(type(rows) is list and len(rows) == len(LABELS))
    native, total = None, 0
    for index, row in enumerate(rows):
        closed(row, 'label argv exit error merged_bytes merged_sha256 merged_file')
        require(row['label'] == LABELS[index] and row['merged_file'] == 'a-' + LABELS[index] + '.merged.log')
        argv = row['argv']
        require(type(argv) is list and len(argv) == len(ARGUMENTS[index]) + 1
                and all(type(part) is str for part in argv) and argv[1:] == ARGUMENTS[index]
                and len(argv[0]) <= 512 and re.fullmatch(r'/tmp/uap-grok-acp-sealed-[^/]+/grok', argv[0])
                and all(32 <= ord(char) <= 126 for char in argv[0])
                and str(PurePosixPath(argv[0])) == argv[0])
        if native is None:
            native = argv[0]
        require(argv[0] == native)
        raw = members[row['merged_file']]
        require(type(row['merged_bytes']) is int and 0 <= row['merged_bytes'] <= 1 << 20
                and row['merged_bytes'] == len(raw) and row['merged_sha256'] == digest(raw))
        total += len(raw)
        if index < 4:
            require(type(row['exit']) is int and row['exit'] == 0 and row['error'] is None)
        else:
            require((row['exit'] is None or type(row['exit']) is int and -255 <= row['exit'] <= 255)
                    and row['error'] == 'protocol-refused')
    require(total <= 1 << 20 and members['a-version.merged.log'] in
            (b'grok 1.0.70 (f50904eaee6c)\n', b'grok 1.0.70 (f50904eaee6c)\r\n'))
    return len(rows)

def wire_prefix(members):
    rpc = closed(document(members['a.rpc.json']), 'initialize plugins notifications session')
    require(rpc['plugins'] is None and rpc['session'] is None
            and type(rpc['notifications']) is list and len(rpc['notifications']) == 11)
    initial = closed(rpc['initialize'], 'jsonrpc id result')
    require(initial['jsonrpc'] == '2.0' and type(initial['id']) is int and initial['id'] == 1
            and type(initial['result']) is dict)
    wire = members['a-acp.merged.log']
    require(wire.endswith(b'\n'))
    lines = wire.splitlines()
    require(len(lines) == 13 and all(0 < len(line) <= 64 << 10 for line in lines))
    last = lines[12]
    require(len(last) == FRAME_SIZE and digest(last) == FRAME_HASH and type(document(last, 64 << 10)) is dict)
    # This joins the stored prefix, not a new acceptance of the unrecognized frame.
    seen_initial, notices = False, []
    for line in lines[:12]:
        frame = document(line, 64 << 10)
        if frame == initial:
            require(not seen_initial)
            seen_initial = True
        else:
            closed(frame, 'jsonrpc method params')
            require(frame['jsonrpc'] == '2.0' and frame['method'] in
                    ('_x.ai/mcp/servers_updated', '_x.ai/session/setup', '_x.ai/mcp_initialized')
                    and type(frame['params']) is dict)
            notices.append(frame)
    require(seen_initial and notices == rpc['notifications'])
    return last

def project(members):
    summary = closed(document(members['admission.json']),
                     'schema_version collection_complete cleanup_complete qualification scopes '
                     'source_commit source_tree nonce run attempt error mode measurement_complete')
    require(type(summary['schema_version']) is int and summary['schema_version'] == 1
            and summary['source_commit'] == SOURCE and summary['source_tree'] == TREE
            and summary['run'] == RUN and summary['attempt'] == '1' and summary['mode'] == MODE
            and summary['collection_complete'] is False and summary['cleanup_complete'] is True
            and summary['measurement_complete'] is True and summary['error'] == 'admission-refused')
    flags = closed(summary['qualification'], ' '.join(FLAGS))
    require(all(flags[key] is False for key in FLAGS))
    nonce = summary['nonce']
    require(type(nonce) is str and re.fullmatch('[0-9a-f]{32}', nonce)
            and type(summary['scopes']) is list and len(summary['scopes']) == 1)
    receipt = closed(summary['scopes'][0], 'schema_version source_commit source_tree scope nonce '
                     'cleanup_complete sealed_cleanup_complete system_executables result')
    identities(receipt, nonce)
    require(receipt['cleanup_complete'] is True and receipt['sealed_cleanup_complete'] is True)
    executables = closed(receipt['system_executables'], 'unshare setpriv env python3')
    for name, row in executables.items():
        closed(row, 'path sha256')
        require(row['path'] == '/usr/bin/' + ('python3.12' if name == 'python3' else name)
                and type(row['sha256']) is str and re.fullmatch('[0-9a-f]{64}', row['sha256']))
    result = closed(receipt['result'], 'schema_version source_commit source_tree scope nonce '
                    'semantic_complete reason isolation commands mode provider')
    identities(result, nonce)
    require(result['mode'] == MODE and result['semantic_complete'] is False
            and result['reason'] == 'protocol-refused')
    isolation(result['isolation'])
    provider = closed(result['provider'], 'requests models_get root_get forbidden settled')
    require(provider['settled'] is True
            and all(type(provider[key]) is int and 0 <= provider[key] <= 13
                    for key in ('requests', 'models_get', 'root_get', 'forbidden'))
            and provider['requests'] == provider['models_get'] + provider['root_get'] + provider['forbidden']
            and provider['forbidden'] == 0)
    acquisition(document(members['acquisition.json']))
    joined = streams(result, members)
    frame = wire_prefix(members)
    ledger = closed(document(members['a.ledger.json']),
                    'before after accepted protected_originals_unchanged selected_config_capture')
    require(ledger['accepted'] is False and type(ledger['protected_originals_unchanged']) is bool
            and type(ledger['before']) is dict and type(ledger['after']) is dict
            and 0 < len(ledger['before']) <= 4096 and 0 < len(ledger['after']) <= 4096
            and ledger['protected_originals_unchanged'] is
                all(ledger['after'].get(name) == item for name, item in ledger['before'].items()))
    # Do not project private config captures, absolute paths, native stderr, or arbitrary diagnostics.
    return dict(kind='fixed-original-readout', case=CASE, artifact_id=ARTIFACT, run=RUN, source=SOURCE, tree=TREE,
                original_failed=True, native_qualified=False, qualification=dict.fromkeys(FLAGS, False),
                cleanup_complete=True, original_semantic_complete=False, original_reason='protocol-refused',
                command_streams_joined=joined, prefix_frames_joined=12, session_reply=False, inventory_reply=False,
                provider_requests=provider['requests'], provider_forbidden=0,
                frame_index=13, frame_bytes=len(frame), frame_sha256=digest(frame),
                frame_base64=base64.b64encode(frame).decode('ascii'))

def audit(body):
    require(type(body) is bytes and len(body) == ARCHIVE_SIZE and digest(body) == ARCHIVE_HASH)
    outer = unpack(body, {'research.zip'})
    inner = outer['research.zip']
    members = unpack(inner, MEMBERS)
    result = project(members)
    result.update(archive_bytes=len(body), archive_sha256=digest(body), inner_bytes=len(inner),
                  inner_sha256=digest(inner), outer_members=1, inner_members=len(members), crc_verified=True)
    return result

def encode(result):
    body = json.dumps(result, sort_keys=True, separators=(',', ':'), ensure_ascii=True, allow_nan=False).encode('ascii') + b'\n'
    require(len(body) <= 8192)
    return body

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('case', choices=(CASE,))
    parser.add_argument('archive', type=Path)
    args = parser.parse_args()
    try:
        with args.archive.open('rb') as source:
            body = source.read(ARCHIVE_SIZE + 1)
        output = encode(audit(body))
    except (ValueError, OSError, UnicodeError, KeyError, TypeError, RecursionError,
            zipfile.BadZipFile, RuntimeError, struct.error, zlib.error):
        output = encode(dict(kind='fixed-original-readout', case=CASE, refused=True, native_qualified=False))
        status = 1
    else:
        status = 0
    print(output.decode('ascii'), end='')
    return status

if __name__ == '__main__':
    raise SystemExit(main())
