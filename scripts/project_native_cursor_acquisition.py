"""Exact Cursor publisher transport; acquired payloads are never executed here."""
import os
from pathlib import Path
import re
import stat
import tempfile

import native_client_project_artifacts as artifacts
from native_client_research import bounded_read, run_probe

URL = "https://downloads.cursor.com/lab/2026.10.22-27dcf5f/linux/x64/agent-cli-package.tar.gz"
TRANSPORT = "cursor-publisher-curl"


def download(spec, budget):
    maximum = spec["max_bytes"]
    if (spec.get("transport") != TRANSPORT or spec["url"] != URL
            or spec["format"] != "tar.gz" or type(maximum) is not int
            or not 1 <= maximum <= 256 << 20):
        raise ValueError("unregistered Cursor publisher transport")
    with tempfile.TemporaryDirectory(prefix="uap-cursor-acquire-") as temporary:
        root = Path(temporary).resolve()
        root.chmod(0o700)
        env = {"PATH": "/usr/bin:/bin", "HOME": str(root), "LANG": "C", "LC_ALL": "C"}
        version, stdout, _ = run_probe(["/usr/bin/curl", "-q", "--version"], root, env, root,
                                       "cursor-curl-version", timeout=min(5, budget.check()),
                                       limit=16384, memory_only=True)
        match = re.match(rb"curl (\d+)\.(\d+)\.(\d+) ", stdout)
        if (version["error"] is not None or version["exit"] != 0 or match is None
                or tuple(int(value) for value in match.groups()) < (8, 4, 0)):
            raise ValueError("Cursor publisher transport requires curl 8.4 or newer")
        target = root / "archive.tar.gz"
        with target.open("xb"):
            target.chmod(0o600)
        before = target.lstat()
        timeout = min(90, budget.check())
        argv = ["/usr/bin/curl", "-q", "-fSL", "--silent", "--proto", "=https",
                "--proto-redir", "=https", "--max-redirs", "0", "--connect-timeout",
                str(min(10, timeout)), "--max-time", str(timeout), "--max-filesize", str(maximum),
                "--output", str(target), "--write-out", "%{http_code}\n%{url_effective}\n", URL]
        record, stdout, _ = run_probe(argv, root, env, root, "cursor-curl-download",
                                      timeout=timeout, limit=16384, memory_only=True)
        if record["error"] is not None or record["exit"] != 0 or stdout != b"200\n" + URL.encode() + b"\n":
            code = stdout.split(b"\n", 1)[0]
            status = code.decode("ascii") if re.fullmatch(rb"[0-9]{3}", code) else "unavailable"
            raise ValueError(f"Cursor publisher acquisition failed: curl_exit={record['exit']}; "
                             f"http_status={status}; process_error={record['error'] is not None}")
        after = target.lstat()
        stable = ("st_dev", "st_ino", "st_mode", "st_nlink", "st_uid", "st_gid")
        if (not stat.S_ISREG(after.st_mode) or after.st_nlink != 1 or after.st_uid != os.getuid()
                or not 0 < after.st_size <= maximum or target.resolve() != target
                or any(getattr(before, field) != getattr(after, field) for field in stable)):
            raise ValueError("Cursor publisher archive identity refused")
        descriptor = artifacts.open_member(target)
        with os.fdopen(descriptor, "rb") as source:
            held = os.fstat(source.fileno())
            if not artifacts.same_opened(after, held):
                raise ValueError("Cursor publisher archive changed while opening")
            budget.check()
            body = bounded_read(source, maximum)
            budget.check()
            if (len(body) != after.st_size or not artifacts.same_opened(held, os.fstat(source.fileno()))
                    or not artifacts.same_opened(after, target.lstat())):
                raise ValueError("Cursor publisher archive changed while reading")
        return body, URL
