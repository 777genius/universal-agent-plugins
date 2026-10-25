#!/usr/bin/env python3
"""UNREVIEWED recovery of the pinned Cline read-only current-source gate.

No native execution is authorized by this draft. Public source operations use
only disposable file authority; Cline may only print version/config metadata.
"""
import argparse
import codecs
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import signal
import stat
import sys
import tempfile
import time
import zipfile

from native_client_research import run_probe
import cline_project_envelope  # Preloaded before native, never imported afterward.


def adjacent(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


cline = adjacent("cline_project_acquisition", "research-cline-native.py")
artifacts = adjacent("cline_project_envelope_io", "native_client_project_artifacts.py")
SOURCE = Path(__file__).resolve().parent.parent
STREAM = 128 << 10
SUMMARY = "cline-project-frontend.json"
NATIVE_SUMMARY = cline_project_envelope.NATIVE
LABELS = cline_project_envelope.FRONT_LABELS


def clean_environment(root, go):
    home, temporary = root / "frontend-home", root / "tmp"
    for path in (home, temporary, root / "gomodcache", root / "gocache"):
        path.mkdir(mode=0o700, exist_ok=False)
    return {"HOME": str(home), "USERPROFILE": str(home), "TMPDIR": str(temporary),
            "PATH": str(go.parent) + ":/usr/bin:/bin", "LANG": "C.UTF-8", "GOTOOLCHAIN": "local",
            "GOMODCACHE": str(root / "gomodcache"), "GOCACHE": str(root / "gocache"),
            "GOPROXY": "https://proxy.golang.org", "GOSUMDB": "sum.golang.org", "GOMAXPROCS": "2",
            "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": str(home / "git-config"),
            "GIT_TERMINAL_PROMPT": "0", "CLINE_NO_AUTO_UPDATE": "1"}


def capture(label, argv, cwd, env, raw, summary, budget, seconds):
    if label not in LABELS or any(row["label"] == label for row in summary["commands"]):
        raise ValueError("closed frontend command table refused")
    record, stdout, stderr = run_probe([str(part) for part in argv], cwd, env, raw, label,
                                      timeout=min(seconds, budget.check()), limit=STREAM)
    record["label"], record["class"] = label, "frontend"
    summary["commands"].append(record)
    if record["error"] or record["exit"] != 0:
        raise ValueError(label + " failed; exact bounded streams retained")
    return stdout, stderr


def checkout(source, env, raw, summary, budget):
    values = {}
    for label, arguments, key in (
            ("frontend-commit", ("rev-parse", "HEAD"), "commit"),
            ("frontend-tree", ("rev-parse", "HEAD^{tree}"), "tree"),
            ("frontend-clean", ("status", "--porcelain", "--untracked-files=normal"), "clean")):
        stdout, stderr = capture(label, ("/usr/bin/git", *arguments), source, env, raw, summary, budget, 10)
        if stderr:
            raise ValueError("checkout query emitted unexpected diagnostics")
        values[key] = stdout.decode("ascii", errors="strict").strip()
    expected = os.environ.get("EXPECTED_COMMIT", "")
    if (not re.fullmatch(r"[0-9a-f]{40}", expected) or values["commit"] != expected
            or not re.fullmatch(r"[0-9a-f]{40}", values["tree"]) or values["clean"]):
        raise ValueError("exact clean workflow checkout required")
    summary["source_commit"], summary["source_tree"] = values["commit"], values["tree"]


def read_ordinary(path, limit):
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or not 0 <= before.st_size <= limit:
        raise ValueError("bounded ordinary evidence/executable required")
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        held = os.fstat(descriptor)
        if artifacts.identity(before) != artifacts.identity(held):
            raise ValueError("held identity differs")
        chunks, left = [], before.st_size + 1
        while left:
            chunk = os.read(descriptor, min(left, 65536))
            if not chunk:
                break
            chunks.append(chunk)
            left -= len(chunk)
        body = b"".join(chunks)
        if len(body) != before.st_size or artifacts.identity(os.fstat(descriptor)) != artifacts.identity(held):
            raise ValueError("bytes/held identity changed")
    finally:
        os.close(descriptor)
    if artifacts.identity(path.lstat()) != artifacts.identity(before):
        raise ValueError("path identity changed")
    return body


def pinned_file(path, limit):
    body = read_ordinary(path, limit)
    if not body:
        raise ValueError("empty executable refused")
    return {"path": str(path), "bytes": len(body), "sha256": hashlib.sha256(body).hexdigest()}


def gate_environment(base, root, binary, raw, source_summary, seconds):
    env = dict(base)
    env.update({key: os.environ[key] for key in ("GITHUB_ACTIONS", "RUNNER_ENVIRONMENT", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED")})
    env.update({"EXPECTED_COMMIT": source_summary["source_commit"],
                "AGENTPLUGINS_INSTALLER_TREE": source_summary["source_tree"],
                "AGENTPLUGINS_CLINE_PROJECT_NATIVE_E2E": "1", "AGENTPLUGINS_CLINE_BIN": str(binary),
                "AGENTPLUGINS_CLINE_FIXTURE_BASE": str(root / "fixtures"),
                "AGENTPLUGINS_CLINE_EVIDENCE": str(raw), "AGENTPLUGINS_CLINE_WORK_SECONDS": str(seconds)})
    return env


def build_and_observe(go, root, env, raw, summary, budget):
    stdout, stderr = capture("frontend-go-version", (go, "version"), SOURCE, env, raw, summary, budget, 10)
    if stdout != b"go version go1.26.10 linux/amd64\n" or stderr:
        raise ValueError("exact Go1.26.10 Linux amd64 toolchain required")
    binary, summary["acquisition"] = cline.acquire_binary(root / "binary", budget)
    test_binary = root / "bin" / "cline-project.test"
    capture("frontend-build", (go, "test", "-p=1", "-c", "-trimpath", "-o", test_binary, "./cmd/agentplugins"),
            SOURCE / "cli", env, raw, summary, budget, 180)
    summary["fixture_executable"] = pinned_file(test_binary, 256 << 20)
    work = min(360, int(budget.check()) - 20)
    if work < 30:
        raise TimeoutError("insufficient original budget for fixture and cleanup")
    selected = gate_environment(env, root, binary, raw, summary, work)
    capture("frontend-gate", (test_binary, "-test.run=^TestClineCurrentSourceNativeProjectConfigLifecycle$",
                              "-test.count=1", "-test.v", "-test.timeout=" + str(work + 10) + "s"),
            root, selected, raw, summary, budget, work + 12)
    result = cline_project_envelope.document(read_ordinary(raw / NATIVE_SUMMARY, 1 << 20))
    if (result.get("source_commit") != summary["source_commit"] or result.get("source_tree") != summary["source_tree"]
            or result.get("collection_complete") is not True
            or result.get("read_only_unique_marker_discovery_qualified") is not True):
        raise ValueError("actual current-source gate did not complete")
    for key in cline_project_envelope.NATIVE_FALSE:
        if result.get(key) is not False:
            raise ValueError("unobserved qualification must remain false")
    summary["collection_complete"] = True
    summary["read_only_unique_marker_discovery_qualified"] = True


def bounded_cleanup(root, expected):
    started, count = time.monotonic(), [0]
    parent = os.open(root.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    descriptor = None
    try:
        descriptor = os.open(root.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
        if not os.path.samestat(os.fstat(descriptor), expected):
            raise ValueError("scratch authority changed")
        def clear(held, depth):
            if depth > 32 or time.monotonic() - started > 5:
                raise TimeoutError("scratch cleanup bound")
            os.fchmod(held, os.fstat(held).st_mode | 0o700)
            with os.scandir(held) as entries:
                for entry in entries:
                    count[0] += 1
                    if count[0] > 20000 or time.monotonic() - started > 5:
                        raise TimeoutError("scratch cleanup entry/deadline bound")
                    before = os.stat(entry.name, dir_fd=held, follow_symlinks=False)
                    if before.st_uid != expected.st_uid:
                        raise ValueError("foreign scratch owner")
                    if stat.S_ISDIR(before.st_mode):
                        child = os.open(entry.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=held)
                        try:
                            if not os.path.samestat(before, os.fstat(child)):
                                raise ValueError("scratch descendant replaced")
                            clear(child, depth + 1)
                        finally:
                            os.close(child)
                        os.rmdir(entry.name, dir_fd=held)
                    else:
                        os.unlink(entry.name, dir_fd=held)
        clear(descriptor, 0)
        if not os.path.samestat(expected, os.stat(root.name, dir_fd=parent, follow_symlinks=False)):
            raise ValueError("scratch root replaced")
        os.rmdir(root.name, dir_fd=parent)
    finally:
        if descriptor is not None:
            os.close(descriptor)
        os.close(parent)


def deny_late_imports(event, _args):
    if event == "import":
        raise RuntimeError("post-native publisher import refused")


def publish(raw, destination, work_seconds=4.0, reap_seconds=1.0):
    pid = os.fork()
    if pid == 0:
        try:
            os.chdir("/")
            sys.addaudithook(deny_late_imports)
            admitted = artifacts.write_archive(raw, destination, client="cline")
            os._exit(0 if admitted else 2)
        except BaseException:
            os._exit(3)
    work_deadline = time.monotonic() + work_seconds
    while time.monotonic() < work_deadline:
        waited, status = os.waitpid(pid, os.WNOHANG)
        if waited:
            return {"complete": os.WIFEXITED(status) and os.WEXITSTATUS(status) == 0, "child_reaped": True,
                    "exit": os.waitstatus_to_exitcode(status), "error": None if status == 0 else "artifact admission refused"}
        time.sleep(0.01)
    try:
        os.kill(pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    reap_deadline = time.monotonic() + reap_seconds
    while time.monotonic() < reap_deadline:
        waited, status = os.waitpid(pid, os.WNOHANG)
        if waited:
            return {"complete": False, "child_reaped": True, "exit": os.waitstatus_to_exitcode(status), "error": "artifact admission deadline"}
        time.sleep(0.01)
    return {"complete": False, "child_reaped": False, "exit": None, "error": "artifact admission cleanup unproved"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    try:
        cline.native.require_hosted("linux-amd64")
        if os.getuid() <= 0 or os.getuid() != os.geteuid() or os.getgid() <= 0 or os.getgid() != os.getegid():
            raise RuntimeError("original nonroot identity required")
    except RuntimeError as error:
        print(str(error)[:512], file=sys.stderr)
        return 2
    output = args.output.absolute()
    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    raw, upload = output / "raw", output / "upload"
    raw.mkdir(mode=0o700)
    raw_identity = raw.lstat()
    upload.mkdir(mode=0o700)
    upload_identity = upload.lstat()
    summary = {"schema_version": 1, "client": "cline", "version": "3.0.73", "commands": [],
               "collection_complete": False, "read_only_unique_marker_discovery_qualified": False,
               "python_runtime": {"version": sys.version[:512], "implementation": sys.implementation.name[:64], "executable": sys.executable[:1024]},
               "isolation": "fresh task authority and bounded direct children; no OS sandbox or unrelated network claim"}
    summary.update({key: False for key in cline_project_envelope.NATIVE_FALSE})
    root = Path(tempfile.mkdtemp(prefix="uap-cline-project-gate-", dir=os.environ["RUNNER_TEMP"])).resolve()
    expected = root.lstat()
    for name in ("utf-8", "cp437"):
        codecs.lookup(name)
    try:
        with cline.grok.Budget(460) as budget:
            selected_go = shutil.which("go")
            if not selected_go:
                raise ValueError("Go executable unavailable")
            go = Path(selected_go).resolve(strict=True)
            for name in ("binary", "bin", "fixtures"):
                (root / name).mkdir(mode=0o700)
            env = clean_environment(root, go)
            checkout(SOURCE, env, raw, summary, budget)
            build_and_observe(go, root, env, raw, summary, budget)
    except BaseException as error:
        summary["error"] = type(error).__name__ + ": " + str(error)[:512]
        summary["collection_complete"], summary["read_only_unique_marker_discovery_qualified"] = False, False
    try:
        bounded_cleanup(root, expected)
        summary["cleanup_complete"] = True
    except BaseException as error:
        summary.update(cleanup_complete=False, cleanup_error=type(error).__name__ + ": " + str(error)[:512],
                       collection_complete=False, read_only_unique_marker_discovery_qualified=False)
    if raw.resolve() != raw or not os.path.samestat(raw.lstat(), raw_identity):
        raise ValueError("raw evidence authority changed")
    descriptor = os.open(raw / SUMMARY, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, "w", encoding="utf-8") as destination:
        destination.write(json.dumps(summary, indent=2) + "\n")
    if upload.resolve() != upload or not os.path.samestat(upload.lstat(), upload_identity):
        raise ValueError("upload directory authority changed")
    admitted = publish(raw, upload / "research.zip")
    if not admitted["child_reaped"]:
        if upload.resolve() != upload or not os.path.samestat(upload.lstat(), upload_identity):
            raise ValueError("unproved publisher upload authority changed")
        quarantine = Path(tempfile.mkdtemp(prefix="upload-unproved-", dir=output))
        os.rename(upload, quarantine / "withheld")
    if not admitted["complete"]:
        print(json.dumps({"artifact_admission": admitted, "collection_complete": False}), file=sys.stderr)
    return 0 if admitted["complete"] and summary["collection_complete"] else 1


if __name__ == "__main__":
    sys.exit(main())
