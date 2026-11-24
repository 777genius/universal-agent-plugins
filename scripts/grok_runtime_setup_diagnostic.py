"""Retain a bounded existing setup log; this never admits a native result."""
import base64
from contextlib import ExitStack
import hashlib
import json
import os
import stat
import sys


MAX_CAPTURE_BYTES = 65536
MAX_RECORD_BYTES = 98304
OUTPUT_NAME = "grok-runtime-setup-diagnostic.json"
SOURCE_PARTS = ("grok-runtime-root", "debootstrap", "debootstrap.log")


def identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_nlink,
            info.st_uid, info.st_gid, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def capture(root_fd):
    record = {"schema_version": 1, "diagnostic_only": True, "qualification": False,
              "source": "/".join(SOURCE_PARTS), "available": False}
    try:
        with ExitStack() as stack:
            parent_fd = root_fd
            held = []
            for index, name in enumerate(SOURCE_PARTS):
                leaf = index == len(SOURCE_PARTS) - 1
                flags = os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW
                flags |= os.O_NONBLOCK if leaf else os.O_DIRECTORY
                fd = os.open(name, flags, dir_fd=parent_fd)
                stack.callback(os.close, fd)
                info = os.fstat(fd)
                if info.st_uid not in (0, os.getuid()) or info.st_mode & 0o022:
                    raise ValueError("source_owner_or_mode_refused")
                if leaf and (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1):
                    raise ValueError("source_not_single_regular_file")
                held.append((parent_fd, name, fd, identity(info)))
                parent_fd = fd
            size = info.st_size
            offset = max(0, size - MAX_CAPTURE_BYTES)
            body = os.pread(fd, min(size, MAX_CAPTURE_BYTES), offset)
            if len(body) != size - offset:
                raise ValueError("source_short_read")
            for parent, name, descriptor, original in held:
                if (identity(os.fstat(descriptor)) != original or
                        identity(os.stat(name, dir_fd=parent, follow_symlinks=False)) != original):
                    raise ValueError("source_changed")
            record.update(available=True, reason=None, original_bytes=size, offset=offset,
                          truncated=offset != 0, captured_bytes=len(body),
                          captured_sha256=hashlib.sha256(body).hexdigest(),
                          content_base64=base64.b64encode(body).decode("ascii"))
    except FileNotFoundError:
        record["reason"] = "original_file_missing"
    except ValueError as error:
        record["reason"] = str(error)
    except OSError:
        record["reason"] = "source_open_or_read_refused"
    return record


def main():
    if len(sys.argv) != 1:
        return 1
    temporary = os.environ.get("RUNNER_TEMP", "")
    if not temporary or not os.path.isabs(temporary) or os.path.realpath(temporary) != temporary:
        return 1
    try:
        with ExitStack() as stack:
            root_fd = os.open(temporary, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
            stack.callback(os.close, root_fd)
            root_info = os.fstat(root_fd)
            if root_info.st_uid != os.getuid() or root_info.st_mode & 0o022:
                return 1
            original = identity(root_info)
            record = capture(root_fd)
            if (identity(os.fstat(root_fd)) != original or
                    identity(os.stat(temporary, follow_symlinks=False)) != original):
                return 1
            body = (json.dumps(record, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
            if len(body) > MAX_RECORD_BYTES:
                return 1
            fd = os.open(OUTPUT_NAME, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC,
                         0o600, dir_fd=root_fd)
            with os.fdopen(fd, "wb") as output:
                output.write(body)
    except (OSError, ValueError):
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
