"""Existing Windows Job custody shared by native proof drivers."""
import time


def require(value, message):
    if not value:
        raise RuntimeError(message)


class WindowsJob:
    """Owned job with the same kill-on-close/accounting contract as windows_conpty.py."""
    def __init__(self, process):
        import ctypes as C
        from ctypes import wintypes as W
        class Basic(C.Structure):
            _fields_ = [("process_time", C.c_int64), ("job_time", C.c_int64), ("flags", W.DWORD),
                        ("min_working", C.c_size_t), ("max_working", C.c_size_t), ("process_limit", W.DWORD),
                        ("affinity", C.c_size_t), ("priority", W.DWORD), ("scheduling", W.DWORD)]
        class Limits(C.Structure):
            _fields_ = [("basic", Basic), ("io", C.c_uint64 * 6), ("process_memory", C.c_size_t),
                        ("job_memory", C.c_size_t), ("peak_process", C.c_size_t), ("peak_job", C.c_size_t)]
        class Accounting(C.Structure):
            _fields_ = [("times", C.c_int64 * 4), ("faults", W.DWORD), ("total", W.DWORD),
                        ("active", W.DWORD), ("terminated", W.DWORD)]
        self.C, self.Accounting = C, Accounting
        self.api = C.WinDLL("kernel32", use_last_error=True)
        signatures = {"CreateJobObjectW": ([W.LPVOID, W.LPCWSTR], W.HANDLE),
            "SetInformationJobObject": ([W.HANDLE, C.c_int, W.LPVOID, W.DWORD], W.BOOL),
            "AssignProcessToJobObject": ([W.HANDLE, W.HANDLE], W.BOOL),
            "QueryInformationJobObject": ([W.HANDLE, C.c_int, W.LPVOID, W.DWORD, W.LPVOID], W.BOOL),
            "TerminateJobObject": ([W.HANDLE, W.UINT], W.BOOL), "CloseHandle": ([W.HANDLE], W.BOOL)}
        for name, (args, result) in signatures.items():
            getattr(self.api, name).argtypes, getattr(self.api, name).restype = args, result
        self.handle = self.api.CreateJobObjectW(None, None)
        self.check(self.handle)
        try:
            limits = Limits()
            limits.basic.flags = 0x2000
            self.check(self.api.SetInformationJobObject(self.handle, 9, C.byref(limits), C.sizeof(limits)))
            self.check(self.api.AssignProcessToJobObject(self.handle, int(process._handle)))
        except BaseException:
            self.api.CloseHandle(self.handle)
            raise

    def check(self, result):
        if not result:
            raise self.C.WinError(self.C.get_last_error())

    def close(self):
        try:
            self.check(self.api.TerminateJobObject(self.handle, 97))
            deadline = time.monotonic() + 5
            while True:
                counts = self.Accounting()
                self.check(self.api.QueryInformationJobObject(self.handle, 1, self.C.byref(counts), self.C.sizeof(counts), None))
                if counts.active == 0:
                    return
                require(time.monotonic() < deadline, "owned Windows job still has active processes")
                time.sleep(.05)
        finally:
            self.check(self.api.CloseHandle(self.handle))
