#!/usr/bin/env python3
"""Fixed inert tracer mechanics only; never Grok or process-family admission."""
import argparse
from contextlib import ExitStack
import errno
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import signal
import stat
import subprocess
import sys
import time

TOOLS = ('/usr/bin/strace', '/usr/bin/bwrap', '/usr/bin/python3', '/usr/bin/true')
LIMITS = {'stdout': 4096, 'stderr': 4096, 'trace': 65536}
RECEIPT_LIMIT = 65536
CLAIM = 'Inert tracer mechanics only; no production view, C0/C1, native, process-family or no-startup qualification.'
CHILD_CODE = """import errno, os, sys
pid = os.fork()
if pid == 0:
    os.execve('/usr/bin/true', ['/usr/bin/true'], {'PATH': '/usr/bin:/bin'})
got, status = os.waitpid(pid, 0)
if got != pid or not os.WIFEXITED(status) or os.WEXITSTATUS(status) != 0:
    raise SystemExit(72)
target, readonly = sys.argv[1], sys.argv[2] == 'readonly'
try:
    fd = os.open(target, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
except OSError as error:
    if not readonly or error.errno != errno.EROFS:
        raise
    os.write(1, b'STEP0_READONLY_EROFS\\n')
else:
    os.close(fd)
    os.unlink(target)
    if readonly:
        raise SystemExit(73)
    os.write(1, b'STEP0_DIRECT_DONE\\n')
"""


def require(value, reason):
    if not value:
        raise RuntimeError(reason)


def remaining(deadline):
    value = deadline - time.monotonic()
    require(value > 0, 'original-deadline')
    return value


def identity(info):
    return [info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_uid,
            info.st_gid, info.st_size, info.st_mtime_ns, info.st_ctime_ns]


def hold_tools(stack, deadline):
    held = {}
    for name in TOOLS:
        remaining(deadline)
        path = Path(name)
        lexical = identity(path.lstat())
        link = os.readlink(path) if stat.S_ISLNK(lexical[2]) else None
        resolved = path.resolve(strict=True)
        descriptor = os.open(resolved, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW | os.O_CLOEXEC)
        stack.callback(os.close, descriptor)
        original = identity(os.fstat(descriptor))
        require(stat.S_ISREG(original[2]) and original[2] & 0o6022 == 0
                and original[2] & 0o111 and original[3] == 1 and 0 < original[6] <= 16 << 20,
                'tool-metadata')
        held[name] = dict(path=name, resolved=str(resolved), lexical=lexical, link=link,
                          original=original, descriptor=descriptor)
    return held


def tool_facts(held, deadline):
    facts = {}
    for name, item in held.items():
        remaining(deadline)
        path, resolved, descriptor = Path(name), Path(item['resolved']), item['descriptor']
        require(identity(path.lstat()) == item['lexical'] and path.resolve(strict=True) == resolved
                and (os.readlink(path) if item['link'] is not None else None) == item['link']
                and identity(os.fstat(descriptor)) == identity(resolved.lstat()) == item['original'],
                'tool-original-changed')
        try:
            os.getxattr(descriptor, 'security.capability')
        except OSError as error:
            require(error.errno == errno.ENODATA, 'tool-capability-unproved')
        else:
            raise RuntimeError('tool-capability-present')
        os.lseek(descriptor, 0, os.SEEK_SET)
        hasher, count = hashlib.sha256(), 0
        while True:
            remaining(deadline)
            chunk = os.read(descriptor, min(65536, item['original'][6] - count + 1))
            if not chunk:
                break
            count += len(chunk)
            require(count <= item['original'][6], 'tool-size-changed')
            hasher.update(chunk)
        require(count == item['original'][6] and identity(os.fstat(descriptor)) == item['original']
                == identity(resolved.lstat()) and identity(path.lstat()) == item['lexical']
                and path.resolve(strict=True) == resolved, 'tool-final-changed')
        remaining(deadline)
        facts[name] = {key: item[key] for key in ('path', 'resolved', 'lexical', 'link', 'original')}
        facts[name].update(bytes=count, sha256=hasher.hexdigest(), capabilities='absent')
    return facts


def fixture_snapshot(root):
    rows = {}
    for path in (root, root / 'original.txt'):
        info = path.lstat()
        require(stat.S_ISDIR(info.st_mode) if path == root else stat.S_ISREG(info.st_mode), 'fixture-kind')
        digest = None
        if path != root:
            descriptor = os.open(path, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW | os.O_CLOEXEC)
            try:
                body = os.read(descriptor, 21)
                require(body == b'inert original input\n' and identity(os.fstat(descriptor)) == identity(info)
                        == identity(path.lstat()), 'fixture-original')
                digest = hashlib.sha256(body).hexdigest()
            finally:
                os.close(descriptor)
        rows[path.name] = dict(identity=identity(info), sha256=digest)
    require(set(os.listdir(root)) == {'original.txt'}, 'fixture-members')
    return rows


def signal_original_group(child, state):
    # The caller has not reaped this leader; its numeric identity is reserved.
    require(child.returncode is None and not state['wait_called'], 'signal-after-wait')
    if state['signalled']:
        return
    # WNOWAIT also refuses an unexpectedly reaped/non-child PID before signaling.
    state['unreaped_exit'] = observe_unreaped(child) or state['unreaped_exit']
    state['signalled'] = True
    try:
        os.killpg(child.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass


def observe_unreaped(child):
    observed = os.waitid(os.P_PID, child.pid, os.WEXITED | os.WNOHANG | os.WNOWAIT)
    require(observed is None or observed.si_pid == child.pid, 'original-wait-identity')
    return observed is not None


def drain(child, selector, streams, deadline, state):
    while selector.get_map() or not state['unreaped_exit']:
        if not state['unreaped_exit']:
            state['unreaped_exit'] = observe_unreaped(child)
        if state['dummy'] is not None and (streams['trace']['body'] or state['unreaped_exit']):
            os.close(state['dummy'])
            state['dummy'] = None
        now = time.monotonic()
        if state['error'] is None and now >= deadline - 1.0:
            state['error'] = 'original-deadline'
        if state['error'] is not None:
            signal_original_group(child, state)
        if now >= deadline:
            break
        for key, _ in selector.select(min(.02, deadline - now)):
            row = streams[key.data]
            allowance = LIMITS[key.data] - len(row['body'])
            try:
                chunk = os.read(key.fd, min(4096, allowance + 1))
            except BlockingIOError:
                continue
            if not chunk:
                row['eof'] = True
                selector.unregister(key.fd)
            else:
                row['body'].extend(chunk[:allowance])
                if len(chunk) > allowance and state['error'] is None:
                    state['error'] = key.data + '-limit'


def finish_original(child, deadline, state):
    try:
        if state['error'] is not None or not state['unreaped_exit']:
            signal_original_group(child, state)
    except (OSError, RuntimeError):
        state['error'] = state['error'] or 'original-signal-unproved'
    try:
        observe_unreaped(child)
    except (OSError, RuntimeError):
        state['error'] = state['error'] or 'original-wait-authority-lost'
        return False
    state['wait_called'] = True
    try:
        child.wait(timeout=max(0.0, deadline - time.monotonic()))
    except (OSError, subprocess.SubprocessError):
        state['error'] = state['error'] or 'original-wait-incomplete'
        return False
    return True


def close_trace_channel(fifo, descriptor, original, state):
    if state['dummy'] is not None:
        os.close(state['dummy'])
    if descriptor is None:
        return
    try:
        require(identity(os.fstat(descriptor))[:6] == original == identity(fifo.lstat())[:6], 'trace-channel-replaced')
        fifo.unlink()
    except (OSError, RuntimeError):
        state['error'] = state['error'] or 'trace-channel-replaced'
    finally:
        os.close(descriptor)


def collect(label, argv, root, fixture, env, deadline, *, trace=False):
    require(remaining(deadline) > 1.0, 'insufficient-original-budget')
    started, child = time.monotonic(), None
    fifo, trace_fd, trace_original = root / (label + '.fifo'), None, None
    state = dict(error=None, dummy=None, signalled=False, unreaped_exit=False, wait_called=False)
    streams, waited = {}, False
    with selectors.DefaultSelector() as selector:
        try:
            if trace:
                os.mkfifo(fifo, 0o600)
                trace_fd = os.open(fifo, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW | os.O_CLOEXEC)
                trace_original = identity(os.fstat(trace_fd))[:6]
                require(stat.S_ISFIFO(trace_original[2]) and trace_original == identity(fifo.lstat())[:6], 'trace-channel-original')
                state['dummy'] = os.open(fifo, os.O_WRONLY | os.O_NONBLOCK | os.O_NOFOLLOW | os.O_CLOEXEC)
                require(identity(os.fstat(state['dummy']))[:6] == trace_original, 'trace-writer-original')
                argv = ['/usr/bin/strace', '-f', '--kill-on-exit', '--decode-pids=pidns', '-e', 'trace=%process',
                        '-s', '4096', '-v', '-o', str(fifo), '--', *argv]
            child = subprocess.Popen(argv, cwd=fixture, env=env, stdin=subprocess.DEVNULL,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE, close_fds=True, start_new_session=True)
            descriptors = {'stdout': child.stdout.fileno(), 'stderr': child.stderr.fileno()}
            if trace:
                descriptors['trace'] = trace_fd
            for name, descriptor in descriptors.items():
                os.set_blocking(descriptor, False)
                selector.register(descriptor, selectors.EVENT_READ, name)
                streams[name] = dict(body=bytearray(), eof=False)
            drain(child, selector, streams, deadline, state)
        except (OSError, RuntimeError, subprocess.SubprocessError):
            state['error'] = state['error'] or 'capture-error'
        finally:
            if child is not None:
                waited = finish_original(child, deadline, state)
                child.stdout.close()
                child.stderr.close()
            close_trace_channel(fifo, trace_fd, trace_original, state)
    if not waited or not streams or not all(row['eof'] for row in streams.values()):
        state['error'] = state['error'] or 'capture-incomplete'
    return dict(label=label, argv=argv, cwd=str(fixture), environment=env, root_pid=child.pid if child else None,
                exit=child.returncode if child else None, wait_observed=waited, wait_called=state['wait_called'],
                unreaped_exit_observed=state['unreaped_exit'], group_signal_sent=state['signalled'],
                error=state['error'], elapsed_seconds=time.monotonic() - started, streams=streams)


def record_streams(row, root, *, version=False):
    for name, capture in row['streams'].items():
        body = bytes(capture.pop('body'))
        require(len(body) <= LIMITS[name], 'retained-stream-bound')
        capture.update(bytes=len(body), sha256=hashlib.sha256(body).hexdigest())
        if version:
            capture['text'] = body.decode('ascii', errors='backslashreplace')
        else:
            path = root / (row['label'] + '.' + name)
            with path.open('xb') as target:
                target.write(body)
            capture['file'] = path.name


def completed(row):
    return (row['exit'] == 0 and row['error'] is None and row['wait_observed']
            and all(stream['eof'] for stream in row['streams'].values()))

def readonly_failure_reason(row):
    # An exact original hosted bwrap denial is diagnostic, never a native pass.
    # Keep every other missing/partial/truncated result in the generic refusal.
    message = b'bwrap: loopback: Failed RTM_NEWADDR: Operation not permitted\\n'
    streams = row['streams']
    if (row['exit'] == 1 and row['error'] is None and row['wait_observed']
            and all(stream['eof'] for stream in streams.values())
            and bytes(streams.get('stdout', {}).get('body', b'')) == b''
            and bytes(streams.get('stderr', {}).get('body', b'')) == message):
        return 'readonly-bwrap-loopback-permission'
    return 'readonly-tracer-refused'


def readonly_arguments(fixture):
    argv = ['/usr/bin/bwrap', '--unshare-user', '--uid', str(os.getuid()), '--gid', str(os.getgid()),
            '--unshare-pid', '--unshare-net', '--unshare-ipc', '--unshare-uts', '--disable-userns',
            '--die-with-parent', '--new-session', '--cap-drop', 'ALL', '--tmpfs', '/', '--ro-bind', '/usr', '/usr']
    for path, target in (('/bin', 'usr/bin'), ('/lib', 'usr/lib'), ('/lib64', 'usr/lib64')):
        require(os.readlink(path) == target, 'runtime-alias-differs')
        argv.extend(('--symlink', target, path))
    argv.extend(('--ro-bind', str(fixture), str(fixture), '--proc', '/proc', '--remount-ro', '/proc', '--dev', '/dev'))
    for name in ('null', 'zero', 'full', 'random', 'urandom', 'tty'):
        argv.extend(('--remount-ro', '/dev/' + name))
    argv.extend(('--remount-ro', '/dev/pts', '--remount-ro', '/dev', '--remount-ro', '/', '--chdir', str(fixture), '--',
                 '/usr/bin/python3', '-I', '-B', '-c', CHILD_CODE, str(fixture / 'read-only-attempt'), 'readonly'))
    return argv


def controls(receipt, root, fixture, env, deadline):
    for label, binary in (('strace-version', TOOLS[0]), ('bwrap-version', TOOLS[1])):
        row = collect(label, [binary, '--version'], root, fixture, env, deadline)
        record_streams(row, root, version=True)
        receipt['versions'].append(row)
        require(completed(row), 'version-refused')
    direct = collect('direct', ['/usr/bin/python3', '-I', '-B', '-c', CHILD_CODE,
                                str(fixture / 'direct-owned'), 'direct'], root, fixture, env, deadline, trace=True)
    direct_body = bytes(direct['streams'].get('stdout', {}).get('body', b''))
    record_streams(direct, root)
    receipt['runs'].append(direct)
    require(completed(direct) and direct_body == b'STEP0_DIRECT_DONE\n'
            and direct['streams']['trace']['bytes'] > 0, 'direct-tracer-refused')
    before = fixture_snapshot(fixture)
    readonly = collect('readonly', readonly_arguments(fixture), root, fixture, env, deadline, trace=True)
    readonly_body = bytes(readonly['streams'].get('stdout', {}).get('body', b''))
    refusal_reason = readonly_failure_reason(readonly)
    record_streams(readonly, root)
    receipt['runs'].append(readonly)
    receipt['readonly_fixture_before'], receipt['readonly_fixture_after'] = before, fixture_snapshot(fixture)
    require(completed(readonly) and readonly_body == b'STEP0_READONLY_EROFS\n'
            and readonly['streams']['trace']['bytes'] > 0 and receipt['readonly_fixture_after'] == before,
            refusal_reason)


def final_tool_facts(receipt, held, deadline):
    if not held or 'tool_before' not in receipt:
        return
    try:
        receipt['tool_after'] = tool_facts(held, deadline)
        receipt['tool_bytes_and_metadata_unchanged'] = receipt['tool_before'] == receipt['tool_after']
        require(receipt['tool_bytes_and_metadata_unchanged'], 'tool-final-changed')
        remaining(deadline)
    except (OSError, RuntimeError):
        receipt['error'] = receipt.get('error', 'tool-final-unproved')
        receipt['outcome'] = 'refused'


def run(root, commit, tree):
    started = time.monotonic()
    deadline = started + 10.0
    require(sys.platform == 'linux' and hasattr(os, 'WNOWAIT') and os.geteuid() != 0, 'host-unavailable')
    require(re.fullmatch('[0-9a-f]{40}', commit) and re.fullmatch('[0-9a-f]{40}', tree), 'source-identity')
    require(root.is_absolute() and root.parent.resolve(strict=True) == root.parent and len(str(root).encode()) <= 1024,
            'output-parent')
    root.mkdir(mode=0o700)
    fixture = root / 'fixture'
    fixture.mkdir(mode=0o700)
    (fixture / 'original.txt').write_bytes(b'inert original input\n')
    (fixture / 'original.txt').chmod(0o600)
    env = {'HOME': str(fixture), 'PATH': '/usr/bin:/bin', 'LANG': 'C', 'LC_ALL': 'C'}
    receipt = dict(schema_version=1, claim=CLAIM, source_commit=commit, source_tree=tree, start_monotonic=started,
                   driver_sha256=hashlib.sha256(Path(__file__).read_bytes()).hexdigest(), child_source=CHILD_CODE,
                   child_source_sha256=hashlib.sha256(CHILD_CODE.encode()).hexdigest(), original_deadline_seconds=10.0,
                   versions=[], runs=[], outcome='refused', tool_bytes_and_metadata_unchanged=False,
                   complete_process_family_admitted=False, native_qualification=False)
    with ExitStack() as stack:
        held = None
        try:
            held = hold_tools(stack, deadline)
            receipt['tool_before'] = tool_facts(held, deadline)
            controls(receipt, root, fixture, env, deadline)
            remaining(deadline)
            receipt['outcome'] = 'inert-controls-completed'
        except RuntimeError as error:
            receipt['error'] = str(error)[:128]
        except (OSError, subprocess.SubprocessError):
            receipt['error'] = 'driver-io-refused'
        finally:
            final_tool_facts(receipt, held, deadline)
    receipt['finish_monotonic'] = time.monotonic()
    receipt['elapsed_seconds'] = receipt['finish_monotonic'] - started
    if receipt['finish_monotonic'] >= deadline:
        receipt['error'] = receipt.get('error', 'original-deadline')
        receipt['outcome'] = 'refused'
    body = (json.dumps(receipt, sort_keys=True, indent=2, ensure_ascii=True, allow_nan=False) + '\n').encode()
    require(len(body) <= RECEIPT_LIMIT, 'receipt-bound')
    with (root / 'receipt.json').open('xb') as output:
        output.write(body)
    print(json.dumps(dict(outcome=receipt['outcome'], error=receipt.get('error'),
                         bytes=len(body), sha256=hashlib.sha256(body).hexdigest(), native_qualification=False)))
    return 0 if receipt['outcome'] == 'inert-controls-completed' else 1


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--source-commit', required=True)
    parser.add_argument('--source-tree', required=True)
    arguments = parser.parse_args()
    raise SystemExit(run(arguments.output, arguments.source_commit, arguments.source_tree))
