#!/usr/bin/env python3
"""Private RPC worker: own one controlling terminal until all probes finish.

The driver owns the master; this session leader owns the slave. CLI and probes
run in foreground process groups in this session, as jobs do under a shell.
No terminal modes are reset here. Control/status never travels over the PTY.
"""
import fcntl
import json
import os
import signal
import socket
import subprocess
import sys
import termios


def main():
    slave, control = map(int, sys.argv[1:])
    os.setsid()
    fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
    signal.signal(signal.SIGTTOU, signal.SIG_IGN)
    channel = socket.socket(fileno=control).makefile('rwb', buffering=0)
    children = {}

    def foreground():
        os.setpgid(0, 0)
        os.tcsetpgrp(slave, os.getpgrp())
        signal.signal(signal.SIGTTOU, signal.SIG_DFL)

    def cleanup():
        for child in children.values():
            try: os.killpg(child.pid, signal.SIGKILL)
            except ProcessLookupError: pass
            child.wait(timeout=2)

    try:
        for line in channel:
            request = json.loads(line)
            opened = []
            try:
                op = request['op']
                if op == 'start':
                    streams = dict(stdin=slave, stdout=slave, stderr=slave)
                    for name, path in request.get('streams', {}).items():
                        f = open(path, 'rb' if name == 'stdin' else 'wb', buffering=0)
                        opened.append(f); streams[name] = f
                    child = subprocess.Popen(request['argv'], cwd=request['cwd'],
                                             env=request['env'], preexec_fn=foreground, **streams)
                    children[child.pid] = child
                    result = child.pid
                elif op == 'poll': result = children[request['pid']].poll()
                elif op == 'kill':
                    try: os.killpg(request['pid'], signal.SIGKILL)
                    except ProcessLookupError: pass
                    result = children[request['pid']].wait(timeout=2)
                elif op == 'close':
                    cleanup(); result = True
                else: raise ValueError('unknown owner operation')
                channel.write((json.dumps({'result': result}) + '\n').encode())
                if op == 'close': break
            except Exception as exc:
                channel.write((json.dumps({'error': repr(exc)}) + '\n').encode())
            finally:
                for f in opened: f.close()
    finally:
        cleanup()
        channel.close()
        os.close(slave)


if __name__ == '__main__': main()
