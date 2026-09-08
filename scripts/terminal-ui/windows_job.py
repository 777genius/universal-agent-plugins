#!/usr/bin/env python3
"""ConPTY-resident owner: inherited-console CLI, mode proof, then line reuse."""
import ctypes
from ctypes import wintypes as W
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
from contextlib import ExitStack


class CursorInfo(ctypes.Structure):
    _fields_ = [('size', W.DWORD), ('visible', W.BOOL)]


class ConsoleOwner:
    def __init__(self, stack):
        import msvcrt
        self.k = ctypes.WinDLL('kernel32', use_last_error=True)
        signatures = {
            'GetStdHandle': ([W.DWORD], W.HANDLE),
            'SetStdHandle': ([W.DWORD, W.HANDLE], W.BOOL),
            'GetConsoleProcessList': ([ctypes.POINTER(W.DWORD), W.DWORD], W.DWORD),
            'CreateFileW': ([W.LPCWSTR, W.DWORD, W.DWORD, W.LPVOID, W.DWORD, W.DWORD, W.HANDLE], W.HANDLE),
            'CloseHandle': ([W.HANDLE], W.BOOL),
            'GetConsoleMode': ([W.HANDLE, ctypes.POINTER(W.DWORD)], W.BOOL),
            'GetConsoleCursorInfo': ([W.HANDLE, ctypes.POINTER(CursorInfo)], W.BOOL),
        }
        for name, (args, result) in signatures.items():
            function = getattr(self.k, name)
            function.argtypes, function.restype = args, result
        # Reject an ordinary inherited user/runner console before opening any
        # console device. The harness creates this owner alone in a new ConPTY.
        pids = (W.DWORD * 64)()
        count = self.k.GetConsoleProcessList(pids, len(pids))
        self.ok(count, 'GetConsoleProcessList')
        self.require_private_console(list(pids[:min(count, len(pids))]), count, os.getpid())
        self.initial = [self.k.GetStdHandle(i & 0xffffffff) for i in (-10, -11, -12)]
        self.initial_modes = []
        for handle in self.initial:
            mode = W.DWORD()
            valid = self.k.GetConsoleMode(handle, ctypes.byref(mode))
            self.initial_modes.append(dict(mode=mode.value) if valid else dict(winerror=ctypes.get_last_error()))
        self.streams = []
        # ConPTY association and the process standard-handle table are distinct.
        # A launcher/CRT can leave the latter pointing at stale runner handles.
        # Open ONLY this already-attached private console; never Alloc/AttachConsole.
        # Transfer each HANDLE exactly once to a CRT fd, then to its Python stream.
        for identifier, device, mode, flags in (
                (-10, 'CONIN$', 'r', os.O_RDONLY),
                (-11, 'CONOUT$', 'w', os.O_WRONLY),
                (-12, 'CONOUT$', 'w', os.O_WRONLY)):
            handle = self.k.CreateFileW(device, 0xc0000000, 3, None, 3, 0, None)
            if handle == ctypes.c_void_p(-1).value:
                self.ok(False, 'CreateFileW(' + device + ')')
            try:
                fd = msvcrt.open_osfhandle(handle, flags | os.O_BINARY)
            except BaseException:
                self.k.CloseHandle(handle)
                raise
            try:
                stream = os.fdopen(fd, mode, encoding='utf-8', buffering=1)
            except BaseException:
                os.close(fd)
                raise
            stack.enter_context(stream)
            self.streams.append(stream)
            self.ok(self.k.SetStdHandle(identifier & 0xffffffff, handle), 'SetStdHandle')
        self.stdin, self.stdout, self.stderr = self.streams

    @staticmethod
    def require_private_console(pids, count, pid):
        if count != 1 or pids != [pid]:
            raise AssertionError('owner must be alone in its new ConPTY: ' + repr(pids))

    @staticmethod
    def ok(value, operation):
        if not value:
            error = ctypes.WinError(ctypes.get_last_error())
            raise OSError(error.winerror, operation + ': ' + str(error))

    def snapshot(self):
        handles, modes = [], []
        for identifier in (-10, -11, -12):
            handle = self.k.GetStdHandle(identifier & 0xffffffff)
            value = W.DWORD()
            self.ok(self.k.GetConsoleMode(handle, ctypes.byref(value)),
                    'GetConsoleMode(std=' + str(identifier) + ', handle=' + str(handle) + ')')
            handles.append(handle)
            modes.append(value.value)
        cursor = CursorInfo()
        self.ok(self.k.GetConsoleCursorInfo(handles[1], ctypes.byref(cursor)), 'GetConsoleCursorInfo')
        return dict(handles=handles, modes=modes, cursor=dict(size=cursor.size, visible=bool(cursor.visible)))


def main():
    config = json.loads(Path(sys.argv[1]).read_text(encoding='utf-8'))
    state = {'phase': 'bootstrap'}

    def save():
        path = Path(config['status'])
        temporary = path.with_suffix('.pending')
        temporary.write_text(json.dumps(state), encoding='utf-8')
        temporary.replace(path)

    try:
        with ExitStack() as stack:
            console = ConsoleOwner(stack)
            # A callable handler survives Ctrl+C without the inheritable ignore
            # flag installed by SIG_IGN, which would disable the CLI's Ctrl+C.
            signal.signal(signal.SIGINT, lambda *_: None)
            before = console.snapshot()
            if not before['cursor']['visible']:
                raise AssertionError('owner cursor hidden before CLI launch')
            state.update(phase='owner-ready', initial_std_handles=console.initial,
                         initial_std_modes=console.initial_modes, owner_before=before)
            save()
            print('OWNER_READY_' + config['nonce'], file=console.stdout, flush=True)
            # Explicit streams make CPython pass valid inherited console handles
            # even if its original CRT standard streams referenced runner pipes.
            child = subprocess.Popen(config['argv'], cwd=config['cwd'], stdin=console.stdin,
                                     stdout=console.stdout, stderr=console.stderr)
            state.update(phase='running', pid=child.pid, before=before['modes'])
            save()
            code = child.wait()
            after = console.snapshot()
            state.update(phase='probe', exit=code, after=after['modes'], owner_after=after,
                         cursor_before=before['cursor']['visible'], cursor_after=after['cursor']['visible'])
            save()
            print('RESTORE_READY_' + config['nonce'], file=console.stdout, flush=True)
            line = console.stdin.readline()
            probe = console.snapshot()
            state.update(phase='done', line_read=line.rstrip('\r\n') == 'line_' + config['nonce'],
                         probe_modes=probe['modes'], owner_probe=probe)
            print(('RESTORE_OK_' if state['line_read'] else 'RESTORE_FAILED_') + config['nonce'],
                  file=console.stdout, flush=True)
            save()
    except BaseException as exc:
        state.update(error=repr(exc))
        save()
        raise


if __name__ == '__main__': main()
