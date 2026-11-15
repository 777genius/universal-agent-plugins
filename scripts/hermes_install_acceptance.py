"""Small native-install acceptance helpers using existing process/file authorities."""
import base64
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile

import native_client_project_artifacts as files


def require(value, message):
    if not value:
        raise RuntimeError(message)


def ordinary(path, maximum=1 << 20):
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and not path.is_symlink()
            and not getattr(before, 'st_file_attributes', 0) & 0x400 and before.st_size <= maximum,
            'bounded ordinary fixture file required: ' + str(path))
    descriptor = files.open_member(path)
    try:
        held = os.fstat(descriptor)
        require((before.st_dev, before.st_ino, before.st_size) == (held.st_dev, held.st_ino, held.st_size),
                'fixture file changed during open')
        with os.fdopen(descriptor, 'rb', closefd=False) as stream:
            body = stream.read(maximum + 1)
        require(len(body) <= maximum and files.identity(before) == files.identity(path.lstat())
                and files.identity(held) == files.identity(os.fstat(descriptor)), 'fixture file changed during read')
        return body
    finally:
        os.close(descriptor)


def snapshot(root):
    result, total = {}, 0
    pending = [root]
    while pending:
        path = pending.pop()
        require(len(result) < 4096, 'fixture member bound exceeded')
        info = path.lstat()
        require(not path.is_symlink() and not getattr(info, 'st_file_attributes', 0) & 0x400,
                'fixture redirect refused: ' + str(path))
        relative = str(path.relative_to(root))
        if stat.S_ISDIR(info.st_mode):
            result[relative] = [info.st_dev, info.st_ino, info.st_mode]
            with os.scandir(path) as children:
                for entry in children:
                    require(len(pending) < 4096, 'fixture directory bound exceeded')
                    pending.append(Path(entry.path))
            require(files.identity(info) == files.identity(path.lstat()), 'fixture directory changed during enumeration')
        else:
            body = ordinary(path, 8 << 20)
            total += len(body)
            require(total <= 64 << 20, 'fixture byte bound exceeded')
            result[relative] = [*files.identity(info), hashlib.sha256(body).hexdigest()]
    return result


def supervised(argv, cwd, env, timeout=60, limit=1 << 20, *, completed_builder_cleanup=False):
    """The Go owner alone starts/reaps the native process and its descendants.

    An outer watchdog failure never grants cleanup or successful evidence.
    The caller retains every disposable fixture on that unknown outcome.
    """
    request = {'argv': [str(x) for x in argv], 'directory': str(cwd),
               'timeout_seconds': int(timeout), 'limit': limit}
    evidence = Path(tempfile.mkdtemp(prefix='hermes-project-supervision-', dir=os.environ['RUNNER_TEMP']))
    (evidence / 'request.json').write_text(json.dumps(request) + '\n')
    try:
        owner = subprocess.run([os.environ['HERMES_INSTALL_RUNNER']], input=json.dumps(request).encode(),
                               env=env, capture_output=True, timeout=timeout + 30)
    except subprocess.TimeoutExpired as error:
        (evidence / 'owner.stdout').write_bytes(error.stdout or b'')
        (evidence / 'owner.stderr').write_bytes(error.stderr or b'')
        (evidence / 'failure.json').write_text(json.dumps({'outer_timeout': True, 'cleanup_complete': False}) + '\n')
        raise
    (evidence / 'owner.stdout').write_bytes(owner.stdout)
    (evidence / 'owner.stderr').write_bytes(owner.stderr)
    packet = json.loads(owner.stdout)
    result = packet['result']
    stdout, stderr = (base64.b64decode(result.get(key) or '', validate=True) for key in ('Stdout', 'Stderr'))
    (evidence / 'native.stdout').write_bytes(stdout)
    (evidence / 'native.stderr').write_bytes(stderr)
    require(len(stdout) <= limit and len(stderr) <= limit, 'supervised stream bound exceeded')
    record = {'argv': request['argv'], 'cwd': str(cwd), 'exit_code': result['ExitCode'],
              'cleanup_complete': result['CleanupComplete'], 'no_child_started': result['NoChildStarted'],
              'error': packet.get('error'), 'streams': {
                  name: {'bytes': len(body), 'sha256': hashlib.sha256(body).hexdigest()}
                  for name, body in (('stdout', stdout), ('stderr', stderr))}}
    (evidence / 'record.json').write_text(json.dumps(record, indent=2) + '\n')
    strict_success = (owner.returncode == 0 and result['ExitCode'] == 0 and result['CleanupComplete']
                      and not result['NoChildStarted'] and not packet.get('error'))
    builder_continuation = (completed_builder_cleanup and reviewed_windows_builder(request['argv'])
                            and owner.returncode == 1 and result['ExitCode'] == 0
                            and result['CleanupComplete'] and not result['NoChildStarted']
                            and bool(packet.get('error')) and packet.get('forced_cleanup_only') is True)
    if builder_continuation:
        record['builder_preparation_continued_after_forced_cleanup'] = True
        (evidence / 'record.json').write_text(json.dumps(record, indent=2) + '\n')
    require(strict_success or builder_continuation,
            'native command failed or lacks process settlement: ' + json.dumps({
                'record': record,
                'output_tails': {name: body[-4096:].decode('utf-8', errors='backslashreplace')
                                 for name, body in (('stdout', stdout), ('stderr', stderr))}}, ensure_ascii=True))
    return record, stdout, stderr


def payload_contract(payload, source, pin):
    manifest_bytes = ordinary(payload / 'manifest.json', 64 << 10)
    prepared_bytes = ordinary(payload.with_name(payload.name + '.prepared.json'), 1 << 20)
    manifest, prepared = json.loads(manifest_bytes), json.loads(prepared_bytes)
    lock = hashlib.sha256(ordinary(source / 'pm/lock.json', 8 << 20)).hexdigest()
    require(manifest['schema'] == prepared['schema'] == 1 and manifest['ref'] == prepared['revision'] == pin
            and prepared['out'] == str(payload) and prepared['source'] == str(source) and prepared['lock'] == lock,
            'official payload preparation differs')
    inputs, runtime = prepared['inputs'], manifest['runtime']
    require(manifest['target'] == inputs['target'] and inputs['ref'] == pin and inputs['placement'] == 'contained'
            and manifest['repo'] == inputs['repo'] == runtime['repoDir'] == 'hermes-agent'
            and manifest['venv'] == 'venv' and manifest['store'] == runtime['toolsDir'] == 'tools'
            and not (payload / 'hermes-agent/.git').exists(), 'official payload layout differs')
    for key, relative in {'code': runtime['repoDir'], 'python': runtime['storePython'],
                          'site_packages': runtime['sitePackages'], 'environment': 'venv', 'tools': 'tools'}.items():
        path = payload / relative
        require(isinstance(relative, str) and relative and not Path(relative).is_absolute()
                and '..' not in Path(relative).parts and inputs[key] == str(path)
                and path.resolve().is_relative_to(payload), 'official selected runtime escapes')
    launcher = payload / 'bin' / ('hermes.exe' if os.name == 'nt' else 'hermes')
    require(runtime['commands']['hermes'] == launcher.relative_to(payload).as_posix(), 'official launcher differs')
    return {'ref': pin, 'target': manifest['target'], 'lock_sha256': lock,
            'manifest_sha256': hashlib.sha256(manifest_bytes).hexdigest(),
            'prepared_sha256': hashlib.sha256(prepared_bytes).hexdigest(),
            'launcher_sha256': hashlib.sha256(ordinary(launcher, 2 << 20)).hexdigest()}



def reviewed_windows_builder(argv):
    # The observed Windows compiler helper survived the successfully completed
    # official build. This grants no exception to native/UAP command acceptance.
    if os.name != 'nt' or len(argv) != 14 or not os.path.isabs(argv[0]) or os.path.basename(argv[0]).lower() != 'python.exe':
        return False
    root = Path(os.environ['RUNNER_TEMP']).resolve()
    source = root / 'hermes-agent'
    return argv[1:] == ['-S', '-B', str(source / 'scripts/bundles/native_build.py'),
                       '--source', str(source), '--work', str(root / 'hermes-project-work'),
                       '--cache', str(root / 'hermes-project-cache'), '--out', str(root / 'hermes-project-payload'),
                       '--commit', '2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19']
