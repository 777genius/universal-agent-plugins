#!/usr/bin/env python3
"""Run one actual Management SDK journey against an owned pinned HTTPS fixture."""
import argparse
import gzip
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io
import json
import os
from pathlib import Path, PurePosixPath
import shlex
import ssl
import subprocess
import sys
import tarfile
import tempfile
import threading
import zipfile

from native_client_research import run_probe

CLIENTS = ('opencode', 'claude', 'codex', 'cline', 'gemini', 'pi', 'copilot', 'grok', 'qoder', 'kimi', 'kiro', 'windsurf')
POLICY = ('kimi', 'kiro', 'windsurf')
GEMINI_ARCHIVE = 'bb5e7e879608bd9b3fe261930d3886fa56aacbf471da1655f2978eb35b705a73'
GEMINI_ENTRY = '41847af2158ced805c4a3e098a05b9a2c26fb1a49b12583009b2eaf6a4059a59'


def digest(body):
    return hashlib.sha256(body).hexdigest()


def identity(path):
    return {'path': str(path), 'sha256': digest(path.read_bytes())}


def fixture_archive(fixture):
    files, raw = {}, io.BytesIO()
    with tarfile.open(fileobj=raw, mode='w', format=tarfile.USTAR_FORMAT) as archive:
        for path in sorted(fixture.rglob('*')):
            if path.is_symlink():
                raise ValueError('fixture symlinks are not admitted')
            if path.is_dir():
                continue
            if not path.is_file():
                raise ValueError('fixture must contain only regular files')
            name, body = path.relative_to(fixture).as_posix(), path.read_bytes()
            files[name] = digest(body)
            info = tarfile.TarInfo(name)
            info.size, info.mode, info.mtime = len(body), 0o644, 0
            archive.addfile(info, io.BytesIO(body))
    if not files:
        raise ValueError('empty fixture')
    compressed = io.BytesIO()
    with gzip.GzipFile(fileobj=compressed, mode='wb', filename='', mtime=0) as stream:
        stream.write(raw.getvalue())
    return compressed.getvalue(), files


def gemini_identity(launcher, archive_path):
    lines = launcher.read_text().splitlines()
    if len(lines) != 2 or lines[0] != '#!/bin/sh' or not lines[1].endswith(' "$@"'):
        raise ValueError('expected the explicit two-line Gemini Node launcher')
    parts = shlex.split(lines[1][:-5])
    if len(parts) != 3 or parts[0] != 'exec':
        raise ValueError('unexpected Gemini launcher command')
    node, entry = (Path(part).resolve(strict=True) for part in parts[1:])
    if digest(entry.read_bytes()) != GEMINI_ENTRY or digest(archive_path.read_bytes()) != GEMINI_ARCHIVE:
        raise ValueError('Gemini 0.65.0 bundle integrity mismatch')
    files = {}
    with zipfile.ZipFile(archive_path) as archive:
        for item in archive.infolist():
            name = PurePosixPath(item.filename)
            if name.is_absolute() or '..' in name.parts or '\\' in item.filename or (item.external_attr >> 16) & 0o170000 == 0o120000:
                raise ValueError('unsafe Gemini bundle entry')
            if item.is_dir():
                continue
            path = entry.parent.joinpath(*name.parts)
            if path.is_symlink() or not path.resolve(strict=True).is_relative_to(entry.parent):
                raise ValueError('Gemini bundle escapes its extracted root')
            body = archive.read(item)
            if path.read_bytes() != body or item.filename in files:
                raise ValueError('Gemini extracted bytes differ from the official archive')
            files[item.filename] = digest(body)
    actual = {path.relative_to(entry.parent).as_posix() for path in entry.parent.rglob('*') if path.is_file()}
    if actual != set(files):
        raise ValueError('Gemini bundle contains extra or missing files')
    return {'native_bundle': {**identity(entry), 'archive_sha256': GEMINI_ARCHIVE, 'file_count': len(files),
                              'tree_sha256': digest(json.dumps(files, sort_keys=True, separators=(',', ':')).encode())},
            'native_node': {**identity(node), 'version': subprocess.check_output([str(node), '--version'], text=True, timeout=10).strip()}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--client', choices=CLIENTS, required=True)
    parser.add_argument('--executable', type=Path, required=True)
    parser.add_argument('--client-executable', type=Path)
    parser.add_argument('--client-archive', type=Path, help='verified official Gemini 0.65.0 bundle ZIP')
    parser.add_argument('--node', default='node')
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if args.client != 'windsurf' and args.client_executable is None:
        parser.error('--client-executable is required for this client')
    if args.client == 'gemini' and args.client_archive is None:
        parser.error('--client-archive is required for Gemini bundle provenance')
    repo = Path(__file__).resolve().parent.parent
    engine = args.executable.resolve(strict=True)
    native = args.client_executable.resolve(strict=True) if args.client_executable else None
    output = args.output.absolute()
    output.mkdir(parents=True, exist_ok=False)
    fixture_name = 'agentplugins_pi_sdk_fixture' if args.client == 'pi' else 'agentplugins_sdk_mcp_fixture' if args.client == 'windsurf' else 'agentplugins_commands_fixture'
    body, files = fixture_archive(repo / 'repotests' / 'testdata' / fixture_name)
    source = {'source_commit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=repo, text=True).strip(),
              'source_dirty': bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=repo)),
              'client': args.client, 'fixture': fixture_name, 'archive_sha256': digest(body), 'fixture_files': files,
              'https_requests': 0, 'engine': identity(engine), 'native_launcher': identity(native) if native else None,
              'qualification': 'sdk-lifecycle-only' if args.client in POLICY else 'sdk-native-lifecycle'}
    if args.client == 'gemini':
        source.update(gemini_identity(native, args.client_archive.resolve(strict=True)))
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            if self.path != '/fixture.tar.gz':
                self.send_error(404)
                return
            source['https_requests'] += 1
            self.send_response(200)
            self.send_header('Content-Type', 'application/gzip')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        def log_message(self, *_args):
            pass
    server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    server.daemon_threads = True
    receipt, record = {}, {}
    with tempfile.TemporaryDirectory(prefix='uap-sdk-https-') as temporary:
        cert, key = Path(temporary) / 'cert.pem', Path(temporary) / 'key.pem'
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', str(key),
                        '-out', str(cert), '-days', '1', '-subj', '/CN=127.0.0.1', '-addext', 'subjectAltName=IP:127.0.0.1'],
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, timeout=20)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(cert, key)
        server.socket = context.wrap_socket(server.socket, server_side=True)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        url = f'https://127.0.0.1:{server.server_port}/fixture.tar.gz#sha256={source["archive_sha256"]}'
        example = 'real-engine-policy-smoke.ts' if args.client in POLICY else 'real-pi-engine-smoke.ts' if args.client == 'pi' else 'real-engine-smoke.ts'
        argv = [args.node, str(repo / 'sdks/typescript/examples' / example), '--client', args.client,
                '--executable', str(engine), '--source', url, '--output', str(output / 'sdk-native-evidence.json')]
        if native:
            argv += ['--client-executable', str(native)]
        allowed = {'PATH', 'LANG', 'LC_ALL', 'SYSTEMROOT', 'WINDIR', 'HTTP_PROXY', 'HTTPS_PROXY', 'NO_PROXY', 'SSL_CERT_DIR'}
        env, seen = {}, set()
        for name, value in os.environ.items():
            upper = name.upper()
            if upper in allowed and upper not in seen:
                env[name] = value
                seen.add(upper)
        env['SSL_CERT_FILE'] = str(cert)
        try:
            record, _, _ = run_probe(argv, repo / 'sdks/typescript', env, output, 'sdk-native', timeout=600)
        finally:
            server.shutdown()
            thread.join(timeout=5)
            server.server_close()
        (output / 'sdk-native-process.json').write_text(json.dumps(record, indent=2) + '\n')
        (output / 'source-identity.json').write_text(json.dumps(source, indent=2) + '\n')
        receipt_path = output / 'sdk-native-evidence.json'
        if receipt_path.exists():
            receipt = json.loads(receipt_path.read_text())
        passed = (record.get('exit') == 0 and not record.get('error') and receipt.get('status') == 'passed'
                  and receipt.get('stage') == 'complete' and not receipt.get('error') and receipt.get('client') == args.client
                  and receipt.get('source') == url and receipt.get('qualification') == source['qualification']
                  and receipt.get('engineSHA256') == source['engine']['sha256'] == digest(engine.read_bytes())
                  and source['https_requests'] > 0)
        if native:
            passed = passed and receipt.get('nativeSHA256') == source['native_launcher']['sha256'] == digest(native.read_bytes())
        if args.client in POLICY:
            passed = passed and receipt.get('nativeLoaded') is False
        print(json.dumps({'client': args.client, 'passed': bool(passed), 'exit': record.get('exit'),
                          'stage': receipt.get('stage'), 'error': record.get('error') or receipt.get('error'),
                          'elapsed_seconds': record.get('elapsed_seconds'), 'output': str(output)}))
        return 0 if passed else 1


if __name__ == '__main__':
    sys.exit(main())
