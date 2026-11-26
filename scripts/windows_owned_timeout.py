"""Bounded failure-only facts from the existing owned Windows job."""

import ntpath
import sys
import time


def snapshot(job, bootstrap, command_executable, started, cleanup_deadline):
    """Observe before termination; never authorize success or open ambient PIDs."""
    refused = {"status": "refused", "qualified": False}
    handles, observed, close_failed = [], None, False
    C, api = job.C, job.api
    deadline = min(cleanup_deadline, time.monotonic() + .25)

    def bounded():
        if time.monotonic() >= deadline:
            raise OSError("diagnostic deadline")

    def accounting():
        bounded()
        result = job.Accounting()
        job.check(api.QueryInformationJobObject(
            job.handle, 1, C.byref(result), C.sizeof(result), None))
        if result.active > result.total:
            raise OSError("invalid accounting")
        return int(result.total), int(result.active)

    class Members(C.Structure):
        _fields_ = [("assigned", C.c_uint32), ("listed", C.c_uint32),
                    ("pids", C.c_size_t * 64)]

    def members():
        bounded()
        result = Members()
        job.check(api.QueryInformationJobObject(
            job.handle, 3, C.byref(result), C.sizeof(result), None))
        if result.assigned != result.listed or result.listed > 64:
            raise OSError("incomplete members")
        pids = list(result.pids[:result.listed])
        if len(set(pids)) != len(pids) or any(not 0 < pid <= 0xffffffff for pid in pids):
            raise OSError("invalid members")
        return set(pids)

    def state(handle):
        bounded()
        belongs = C.c_int()
        job.check(api.IsProcessInJob(handle, job.handle, C.byref(belongs)))
        if not belongs.value:
            raise OSError("foreign process")
        wait = api.WaitForSingleObject(handle, 0)
        if wait not in (0, 258):
            raise OSError("unavailable exit state")
        return "exited" if wait == 0 else "running"

    try:
        for name, args, result in (
                ("OpenProcess", [C.c_uint32, C.c_int, C.c_uint32], C.c_void_p),
                ("IsProcessInJob", [C.c_void_p, C.c_void_p, C.POINTER(C.c_int)], C.c_int),
                ("WaitForSingleObject", [C.c_void_p, C.c_uint32], C.c_uint32),
                ("QueryFullProcessImageNameW", [C.c_void_p, C.c_uint32, C.c_wchar_p,
                                               C.POINTER(C.c_uint32)], C.c_int),
                ("CloseHandle", [C.c_void_p], C.c_int)):
            getattr(api, name).argtypes, getattr(api, name).restype = args, result
        generation = accounting()
        original_members = members()
        records, states = [], []
        for pid in sorted(original_members):
            bounded()
            handle = api.OpenProcess(0x00101000, False, pid)
            job.check(handle)
            handles.append(handle)
            initial_state = state(handle)
            size = C.c_uint32(32768)
            image = C.create_unicode_buffer(size.value)
            job.check(api.QueryFullProcessImageNameW(handle, 0, image, C.byref(size)))
            if not 0 < size.value < len(image):
                raise OSError("invalid image length")
            # Full paths stay private. Unknown executables have one fixed category.
            path = ntpath.normcase(ntpath.normpath(image.value))
            category = ("command" if path == ntpath.normcase(ntpath.normpath(command_executable))
                        else "python" if path == ntpath.normcase(ntpath.normpath(sys.executable))
                        else {"claude.exe": "claude", "node.exe": "node", "cmd.exe": "cmd",
                              "conhost.exe": "console"}.get(ntpath.basename(path), "other"))
            records.append({"image": category, "state": initial_state})
            states.append(initial_state)
        if accounting() != generation or members() != original_members:
            raise OSError("changed membership")
        if [state(handle) for handle in handles] != states:
            raise OSError("changed exit state")
        if accounting() != generation or members() != original_members:
            raise OSError("changed membership")
        if generation[1] != states.count("running"):
            raise OSError("inconsistent active count")
        bootstrap_state = "running" if bootstrap.poll() is None else "exited"
        bounded()
        elapsed_ms = int((time.monotonic() - started) * 1000)
        if not 0 <= elapsed_ms <= 3600000:
            raise OSError("invalid elapsed time")
        observed = {"status": "observed-unqualified", "qualified": False,
                    "stage": "timeout-before-owned-termination", "elapsed_ms": elapsed_ms,
                    "bootstrap_state": bootstrap_state, "total_started": generation[0],
                    "active": generation[1],
                    "members": sorted(records, key=lambda row: (row["image"], row["state"]))}
    except Exception:
        observed = None
    finally:
        for handle in handles:
            try:
                if not api.CloseHandle(handle):
                    close_failed = True
            except Exception:
                close_failed = True
    return observed if observed is not None and not close_failed else refused
