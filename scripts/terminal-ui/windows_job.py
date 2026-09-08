#!/usr/bin/env python3
"""ConPTY-resident owner: inherited-console CLI, mode proof, then line reuse."""
import ctypes
from ctypes import wintypes as W
import json
from pathlib import Path
import signal
import subprocess
import sys


def main():
    config = json.loads(Path(sys.argv[1]).read_text(encoding='utf-8'))
    kernel = ctypes.WinDLL('kernel32', use_last_error=True)
    kernel.GetStdHandle.argtypes = [W.DWORD]
    kernel.GetStdHandle.restype = W.HANDLE
    kernel.GetConsoleMode.argtypes = [W.HANDLE, ctypes.POINTER(W.DWORD)]
    kernel.GetConsoleMode.restype = W.BOOL

    class CursorInfo(ctypes.Structure):
        _fields_ = [('size', W.DWORD), ('visible', W.BOOL)]

    kernel.GetConsoleCursorInfo.argtypes = [W.HANDLE, ctypes.POINTER(CursorInfo)]
    kernel.GetConsoleCursorInfo.restype = W.BOOL

    def cursor():
        value = CursorInfo()
        if not kernel.GetConsoleCursorInfo(kernel.GetStdHandle((-11) & 0xffffffff), ctypes.byref(value)):
            raise ctypes.WinError(ctypes.get_last_error())
        return bool(value.visible)

    def modes():
        result = []
        for identifier in (-10, -11, -12):
            value = W.DWORD()
            if not kernel.GetConsoleMode(kernel.GetStdHandle(identifier & 0xffffffff), ctypes.byref(value)):
                raise ctypes.WinError(ctypes.get_last_error())
            result.append(value.value)
        return result

    def save(value):
        path = Path(config['status'])
        temporary = path.with_suffix('.pending')
        temporary.write_text(json.dumps(value), encoding='utf-8')
        temporary.replace(path)

    # The terminal owner must survive a console Ctrl+C delivered to the CLI.
    signal.signal(signal.SIGINT, lambda *_: None)
    before = modes()
    cursor_before = cursor()
    child = subprocess.Popen(config['argv'], cwd=config['cwd'])
    save({'phase': 'running', 'pid': child.pid, 'before': before})
    code = child.wait()
    after = modes()
    result = dict(phase='probe', pid=child.pid, exit=code, before=before, after=after,
                  cursor_before=cursor_before, cursor_after=cursor())
    save(result)
    nonce = config['nonce']
    print('RESTORE_READY_' + nonce, flush=True)
    line = sys.stdin.readline()
    result.update(phase='done', line_read=line.rstrip('\r\n') == 'line_' + nonce,
                  probe_modes=modes())
    print('RESTORE_OK_' + nonce if result['line_read'] else 'RESTORE_FAILED_' + nonce, flush=True)
    save(result)


if __name__ == '__main__': main()
