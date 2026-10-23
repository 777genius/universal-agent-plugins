#!/usr/bin/env python3
"""Pinned Qoder help/empty-list research, only on disposable hosted Linux.

This is not a project installation or runtime qualification. No plugin install,
uninstall, trust, authentication, model or MCP process is requested.
"""
import argparse
import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import resource
import signal
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request

SPEC = importlib.util.spec_from_file_location(
    "qoder_research_native_matrix", Path(__file__).with_name("run-native-client-matrix.py"))
native = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(native)

VERSION = "1.1.78"
URL = "https://qoder-ide.oss-accelerate.aliyuncs.com/qodercli/releases/1.1.78/qodercli-linux-x64.tar.gz"
ARCHIVE_PIN = "sha256:5e3213d2d4256c86c5ad2c8bf73fd392f92ba80d0d38f822806542805a48a8ea"
BINARY_SHA256 = "0da0976e13fe00bc28f725d6fe7c2f0fe9fa9a8c37eeb2ee4e80ad4c0fc58953"
MAX_ARCHIVE = 96 << 20
MAX_BINARY = 256 << 20
MAX_EXPANDED = MAX_BINARY + (1 << 20)
MAX_MEMBERS = 16
OUTPUT_LIMIT = 1 << 20
PROBES = (
    ("version", ("--version",)),
    ("plugins-help", ("plugins", "--help")),
    ("plugins-list-help", ("plugins", "list", "--help")),
    ("plugins-install-help", ("plugins", "install", "--help")),
    ("plugins-uninstall-help", ("plugins", "uninstall", "--help")),
    ("plugins-empty-list", ("plugins", "list", "--json")),
)


def bounded_read(stream, limit):
    chunks, size = [], 0
    while True:
        chunk = stream.read(min(64 << 10, limit + 1 - size))
        if not chunk:
            return b"".join(chunks)
        chunks.append(chunk)
        size += len(chunk)
        if size > limit:
            raise ValueError("input exceeds configured byte limit")


def archive_binary(body):
    """Parse bounded bytes; archive names are never output filesystem paths."""
    with gzip.GzipFile(fileobj=io.BytesIO(body)) as compressed:
        expanded = bounded_read(compressed, MAX_EXPANDED)
    binary, seen = None, set()
    with tarfile.open(fileobj=io.BytesIO(expanded), mode="r:") as archive:
        for count, member in enumerate(archive, 1):
            if count > MAX_MEMBERS:
                raise ValueError("too many archive members")
            name = member.name
            if name.startswith("./"):
                name = name[2:]
            parts = PurePosixPath(name).parts
            if (not name or name.startswith("/") or "\\" in name or "\0" in name
                    or any(part in ("..", ".") or ":" in part for part in parts)
                    or (name != "." and name.rstrip("/") != "/".join(parts))
                    or name.casefold() in seen):
                raise ValueError("unsafe or duplicate archive member")
            seen.add(name.casefold())
            if member.isdir() and member.size == 0:
                continue
            if (member.type not in (tarfile.REGTYPE, tarfile.AREGTYPE)
                    or PurePosixPath(name).name != "qodercli" or binary is not None
                    or not 0 < member.size <= MAX_BINARY):
                raise ValueError("expected one bounded regular qodercli binary")
            with archive.extractfile(member) as source:
                binary = bounded_read(source, MAX_BINARY)
            if len(binary) != member.size:
                raise ValueError("truncated binary member")
    if binary is None:
        raise ValueError("missing qodercli binary")
    return binary


def acquire_binary(directory):
    with urllib.request.urlopen(URL, timeout=90) as response:
        body = bounded_read(response, MAX_ARCHIVE)
    native.verify_digest(body, ARCHIVE_PIN)
    binary = archive_binary(body)
    actual = hashlib.sha256(binary).hexdigest()
    if actual != BINARY_SHA256:
        raise ValueError("binary integrity mismatch")
    path = directory / "qoder"
    path.write_bytes(binary)
    path.chmod(0o700)
    return path, {"url": URL, "version": VERSION, "archive_integrity": ARCHIVE_PIN,
                  "archive_bytes": len(body), "binary_sha256": actual,
                  "binary_bytes": len(binary), "installer_executed": False}


def runtime_environment(home, temp):
    env = native.profile_environment(home, "linux-amd64")
    env.update({"PATH": "/usr/bin:/bin", "TMPDIR": str(temp), "LANG": "C.UTF-8"})
    return env


def child_limits(limit):
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    resource.setrlimit(resource.RLIMIT_FSIZE, (limit, limit))


def run_probe(argv, cwd, env, output, label, timeout=30, limit=OUTPUT_LIMIT):
    """File limits apply in the child while running; every process group is closed."""
    stdout_path, stderr_path = output / (label + ".stdout.log"), output / (label + ".stderr.log")
    record = {"probe": label, "argv": argv, "cwd": str(cwd), "error": None}
    started, process = time.monotonic(), None
    with stdout_path.open("wb") as stdout, stderr_path.open("wb") as stderr:
        try:
            process = subprocess.Popen(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                       stdout=stdout, stderr=stderr, start_new_session=True,
                                       preexec_fn=lambda: child_limits(limit))
            while True:
                if stdout_path.stat().st_size >= limit or stderr_path.stat().st_size >= limit:
                    record["error"] = "output limit reached"
                    break
                if time.monotonic() - started >= timeout:
                    record["error"] = "probe deadline exceeded"
                    break
                if process.poll() is not None:
                    break
                time.sleep(0.02)
        except (OSError, subprocess.SubprocessError) as exc:
            record["error"] = str(exc)[:512]
        finally:
            if process is not None:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.wait(timeout=5)
    stdout_body, stderr_body = stdout_path.read_bytes(), stderr_path.read_bytes()
    if len(stdout_body) >= limit or len(stderr_body) >= limit:
        record["error"] = "output limit reached"
    record.update({"exit": process.returncode if process is not None else None,
                   "elapsed_seconds": round(time.monotonic() - started, 3),
                   "stdout_file": stdout_path.name, "stderr_file": stderr_path.name,
                   "stdout_bytes": len(stdout_body), "stderr_bytes": len(stderr_body),
                   "stdout_sha256": hashlib.sha256(stdout_body).hexdigest(),
                   "stderr_sha256": hashlib.sha256(stderr_body).hexdigest()})
    return record, stdout_body, stderr_body


def validate_probe(record, stdout, stderr):
    if record["error"] or record["exit"] != 0:
        return False
    try:
        if record["probe"] == "version":
            return re.search(r"(?<![A-Za-z0-9_.+-])v?" + re.escape(VERSION)
                             + r"(?![A-Za-z0-9_.+-])", stdout.decode("utf-8")) is not None
        if record["probe"] == "plugins-empty-list":
            return not stderr.strip() and json.loads(stdout.decode("utf-8")) == []
        return bool(stdout.strip() or stderr.strip())
    except (UnicodeError, ValueError):
        return False


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        # Do not create any output/probe path or acquire/execute before admission.
        native.require_hosted("linux-amd64")
    except RuntimeError as exc:
        print(str(exc)[:512], file=sys.stderr)
        return 2
    output = args.output.absolute()
    output.mkdir(parents=True, exist_ok=False)
    summary = {"evidence_kind": "qoder-precode-version-help-empty-list", "version": VERSION,
               "target": "linux-amd64", "source_commit": os.environ.get("EXPECTED_COMMIT", ""),
               "expected_probes": [name for name, _ in PROBES], "probes": [], "success": False,
               "unobserved": ["nonempty project record schema", "project installation/ownership",
                              "skill execution", "MCP initialization", "model/account/trust",
                              "native session restart"], "runtime_network_isolation_claimed": False}
    try:
        with tempfile.TemporaryDirectory(prefix="qoder-native-research-") as temporary:
            root = Path(temporary)
            for name in ("binary", "home", "project", "tmp"):
                (root / name).mkdir()
            binary, summary["acquisition"] = acquire_binary(root / "binary")
            env = runtime_environment(root / "home", root / "tmp")
            summary["native_environment"] = env
            for label, arguments in PROBES:
                record, stdout, stderr = run_probe([str(binary), *arguments], root / "project", env, output, label)
                record["accepted"] = validate_probe(record, stdout, stderr)
                summary["probes"].append(record)
            summary["success"] = (len(summary["probes"]) == len(PROBES)
                                  and all(record["accepted"] for record in summary["probes"]))
    except Exception as exc:
        summary["error"] = f"{type(exc).__name__}: {str(exc)[:512]}"
    (output / "qoder-precode-research.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    return 0 if summary["success"] else 1


if __name__ == "__main__":
    sys.exit(main())
