#!/usr/bin/env python3
"""Pinned untrusted-workbench metadata research; genuine hosted Linux only.

No installation, chat, server start, trust grant, sandbox fallback or project
adapter qualification. The privileged fixed helper owns strict native trees.
"""
import argparse
import encodings.ascii
import encodings.cp437
import encodings.utf_8
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import selectors
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time

from native_client_research import run_probe


def adjacent(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


publisher = adjacent("vscode_workspace_publisher", "research-vscode-native.py")
PINNED_FILES = {
    "code": (223685880, "cf8508cc7ddde30cfccc70caeda877a9a99119953c4476dcb30a3eb1a2c415e9"),
    "resources/app/out/cli.js": (275169, "bd393ad6f0d28ba0aa7cd79cc685617f5187096f092009e315a4f098349f7976"),
    "resources/app/product.json": (68251, "a141a1de8ed44415c0c527b1882362e6abc25e674be052d67c09da71a27fbd7e"),
}
MAX_CONTROL = 64 << 10
ROOT_TIMEOUT = 460
ROOT_KILL = 470
SOURCE_RE = re.compile(r"[0-9a-f]{40}\Z")
GUARD_KEYS = ("GITHUB_ACTIONS", "RUNNER_ENVIRONMENT", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED", "EXPECTED_COMMIT")


def guard():
    publisher.native.require_hosted("linux-amd64")
    if (os.getuid() <= 0 or os.getgid() <= 0 or os.getuid() != os.geteuid()
            or os.getgid() != os.getegid() or not SOURCE_RE.fullmatch(os.environ.get("EXPECTED_COMMIT", ""))):
        raise ValueError("exact-source original nonroot hosted identity required")


def clean_environment(root):
    return {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8", "HOME": str(root)}


FRONTEND_CLEANUP_CODE = r'''
import os,stat,sys
path,device,inode,owner=sys.argv[1:]
device,inode,owner=int(device),int(inode),int(owner)
parent,name=os.path.split(path)
if parent!="/tmp" or not name.startswith("vscode-workspace-input-") or not name or owner<=0 or os.getuid()!=owner or os.geteuid()!=owner:
    raise SystemExit("frontend cleanup authority refused")
def same(a,b):
    return (a.st_dev,a.st_ino,a.st_uid,stat.S_IFMT(a.st_mode))==(b.st_dev,b.st_ino,b.st_uid,stat.S_IFMT(b.st_mode))
def owned_directory(info):
    if not stat.S_ISDIR(info.st_mode) or info.st_uid!=owner or info.st_dev!=device:
        raise RuntimeError("frontend cleanup directory authority differs")
parent_fd=os.open(parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
root=os.open(name,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=parent_fd)
identity=os.fstat(root)
if (identity.st_dev,identity.st_ino,identity.st_uid)!=(device,inode,owner) or not same(identity,os.stat(name,dir_fd=parent_fd,follow_symlinks=False)):
    raise SystemExit("frontend cleanup root changed")
def clear(fd,expected):
    current=os.fstat(fd)
    owned_directory(current)
    if not same(current,expected):raise RuntimeError("frontend cleanup opened directory changed")
    os.fchmod(fd,(current.st_mode&0o777)|0o700)
    for entry in os.listdir(fd):
        item=os.stat(entry,dir_fd=fd,follow_symlinks=False)
        if item.st_uid!=owner or item.st_dev!=device:raise RuntimeError("frontend cleanup leaf authority differs")
        if stat.S_ISDIR(item.st_mode):
            child=os.open(entry,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)
            try:
                opened=os.fstat(child)
                if not same(item,opened) or not same(opened,os.stat(entry,dir_fd=fd,follow_symlinks=False)):
                    raise RuntimeError("frontend cleanup directory replaced")
                clear(child,opened)
                if not same(opened,os.stat(entry,dir_fd=fd,follow_symlinks=False)):
                    raise RuntimeError("frontend cleanup directory replaced before removal")
            finally:os.close(child)
            os.rmdir(entry,dir_fd=fd)
        else:
            if not (stat.S_ISREG(item.st_mode) or stat.S_ISLNK(item.st_mode)):
                raise RuntimeError("frontend cleanup unsupported leaf")
            if not same(item,os.stat(entry,dir_fd=fd,follow_symlinks=False)):
                raise RuntimeError("frontend cleanup leaf replaced")
            os.unlink(entry,dir_fd=fd)
clear(root,identity)
if not same(identity,os.stat(name,dir_fd=parent_fd,follow_symlinks=False)):
    raise SystemExit("frontend cleanup root replaced before removal")
os.close(root)
os.rmdir(name,dir_fd=parent_fd)
os.close(parent_fd)
'''


def cleanup_frontend(root, identity, output):
    record, _, _ = run_probe(["/usr/bin/python3", "-I", "-c", FRONTEND_CLEANUP_CODE,
                              str(root), str(identity.st_dev), str(identity.st_ino), str(identity.st_uid)],
                             Path("/"), {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8"}, output, "cleanup", timeout=5)
    return {"complete": record["exit"] == 0 and record["error"] is None, "probe": record}


def digest(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        while body := stream.read(64 << 10):
            h.update(body)
    return h.hexdigest()


def source_checkout(repo, output):
    record, stdout, stderr = run_probe(["/usr/bin/git", "rev-parse", "HEAD"], repo,
                                       clean_environment(output), output, "source-checkout", timeout=5, limit=MAX_CONTROL)
    if record["error"] is not None or record["exit"] != 0 or stderr or stdout.decode("ascii", "strict").strip() != os.environ["EXPECTED_COMMIT"]:
        raise ValueError("actual checkout does not equal exact expected source")
    return record


def verify_publisher(root, budget):
    found = publisher.required_files(root, budget)
    for name, (size, expected) in PINNED_FILES.items():
        if found[name]["bytes"] != size or found[name]["sha256"] != expected:
            raise ValueError("previously measured exact publisher identity differs")
    return found


def build_helper(repo, root, output, budget):
    go = shutil.which("go")
    if not go:
        raise ValueError("pinned workflow Go toolchain missing")
    home = root / "build-home"
    home.mkdir()
    env = clean_environment(home)
    env.update({"PATH": str(Path(go).parent) + ":/usr/bin:/bin", "GOTOOLCHAIN": "local", "GOMAXPROCS": "2", "GOMEMLIMIT": "512MiB"})
    version, stdout, stderr = run_probe([go, "version"], repo, env, output, "go-version", timeout=5, limit=MAX_CONTROL)
    if version["error"] is not None or version["exit"] != 0 or stdout != b"go version go1.26.10 linux/amd64\n" or stderr:
        raise ValueError("exact pinned Go version differs")
    executable = root / "controller"
    argv = [go, "build", "-trimpath", "-p=1", "-ldflags", "-X main.buildCommit=" + os.environ["EXPECTED_COMMIT"],
            "-o", str(executable), "./install/integrationctl/research/vscodeworkspace"]
    record, _, _ = run_probe(argv, repo, env, output, "helper-build", timeout=min(60, budget.check()), limit=1 << 20)
    if record["error"] is not None or record["exit"] != 0:
        raise ValueError("exact-source helper build failed or exceeded its bound")
    info = executable.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size > 64 << 20:
        raise ValueError("helper is not an ordinary bounded executable")
    return executable, {"version_probe": version, "build": record, "bytes": info.st_size, "sha256": digest(executable)}


def signal_controller(process, sig, errors):
    try:
        os.killpg(process.pid, sig)
    except ProcessLookupError:
        pass
    except PermissionError:
        errors.append("controller signal permission refused; cleanup not proved")


def capture_controller(argv, repo, output, origin_ns):
    env = {key: os.environ[key] for key in GUARD_KEYS}
    env.update({"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8"})
    record = {"argv": argv, "cwd": str(repo), "error": None, "controller_reaped": False}
    errors, buffers = [], {"stdout": bytearray(), "stderr": bytearray()}
    process, stopping = None, False
    try:
        process = subprocess.Popen(argv, cwd=repo, env=env, stdin=subprocess.PIPE,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        with selectors.DefaultSelector() as selector:
            for name, pipe in (("stdout", process.stdout), ("stderr", process.stderr)):
                os.set_blocking(pipe.fileno(), False)
                selector.register(pipe, selectors.EVENT_READ, name)
            while selector.get_map() or process.poll() is None:
                elapsed = (time.monotonic_ns() - origin_ns) / 1e9
                if elapsed >= ROOT_TIMEOUT and not stopping:
                    errors.append("controller exceeded reserved return deadline")
                    process.stdin.close()
                    signal_controller(process, signal.SIGTERM, errors)
                    stopping = True
                if elapsed >= ROOT_KILL:
                    signal_controller(process, signal.SIGKILL, errors)
                    errors.append("controller terminal watchdog reached; cleanup unproved")
                    break
                for key, _ in selector.select(0.05):
                    allowance = MAX_CONTROL - len(buffers[key.data])
                    chunk = os.read(key.fd, min(64 << 10, allowance + 1))
                    if not chunk:
                        selector.unregister(key.fileobj)
                        key.fileobj.close()
                    else:
                        buffers[key.data].extend(chunk[:allowance])
                        if len(chunk) > allowance:
                            errors.append("controller stream limit reached")
                            if not stopping:
                                process.stdin.close()
                                signal_controller(process, signal.SIGTERM, errors)
                                stopping = True
            try:
                process.wait(timeout=0.1)
                record["controller_reaped"] = True
            except subprocess.TimeoutExpired:
                errors.append("controller did not reap within reserved boundary")
    except (OSError, subprocess.SubprocessError) as exc:
        errors.append(type(exc).__name__ + ": " + str(exc)[:512])
    finally:
        if process is not None:
            for pipe in (process.stdin, process.stdout, process.stderr):
                if pipe is not None:
                    pipe.close()
    record["exit"] = None if process is None else process.returncode
    for stream, body in buffers.items():
        path = output / ("controller." + stream + ".log")
        path.write_bytes(body)
        record.update({stream + "_file": path.name, stream + "_bytes": len(body), stream + "_sha256": hashlib.sha256(body).hexdigest()})
    if errors or not record["controller_reaped"] or record["exit"] != 0:
        record["error"] = "; ".join(errors or ["fixed controller returned failure"])[:2048]
    return record


def collect(root, output, summary, origin_ns):
    repo = Path(__file__).resolve().parent.parent
    summary["checkout"] = source_checkout(repo, output)
    with publisher.bounds.Budget(seconds=max(1, 420 - (time.monotonic_ns() - origin_ns) / 1e9)) as budget:
        if shutil.disk_usage(root).free < publisher.MIN_FREE:
            raise ValueError("at least four GiB free required")
        archive = publisher.acquire_archive(root, budget)
        materialized = root / "publisher"
        materialized.mkdir()
        summary["archive"] = {"url": publisher.URL, "bytes": publisher.ARCHIVE_BYTES, "sha256": publisher.ARCHIVE_SHA256}
        summary["materialization"] = publisher.materialize_archive(archive, materialized, budget)
        summary["publisher_files"] = verify_publisher(materialized, budget)
        helper, summary["helper"] = build_helper(repo, root, output, budget)
        request = {"uid": os.getuid(), "gid": os.getgid(), "euid": os.geteuid(), "egid": os.getegid(), "start_monotonic_ns": origin_ns,
                   "source_commit": os.environ["EXPECTED_COMMIT"], "observer_root": str(repo / "scripts" / "vscode-workspace-observer"),
                   "output_root": str(output), "helper_sha256": summary["helper"]["sha256"]}
        (root / "request.json").write_text(json.dumps(request), encoding="utf-8")
    argv = ["/usr/bin/sudo", "-n", "--preserve-env=" + ",".join(GUARD_KEYS), str(helper), "--input-root", str(root)]
    summary["controller"] = capture_controller(argv, repo, output, origin_ns)
    if summary["controller"]["error"] is not None:
        raise ValueError("fixed root controller failed; retain bounded evidence")
    with (output / "vscode-workspace-controller.json").open("rb") as stream:
        body = publisher.bounded_read(stream, 1 << 20)
    result = json.loads(body, object_pairs_hook=publisher.bounds.unique_object, parse_constant=publisher.bounds.invalid_constant)
    if (result.get("source_commit") != os.environ["EXPECTED_COMMIT"] or result.get("collection_complete") is not True
            or result.get("native_project_qualified") is not False or result.get("native_mcp_initialization_qualified") is not False
            or result.get("native_runtime_qualified") is not False or result.get("cleanup", {}).get("complete") is not True):
        raise ValueError("controller did not prove complete bounded collection/cleanup")
    summary["collection_complete"] = True


def allowed_artifact_names():
    names = {"vscode-workspace-research.json", "vscode-workspace-controller.json", "system-executables.json", "xauth.json"}
    for label in ("source-checkout", "go-version", "helper-build", "controller", "cleanup", "xauth", "root-cleanup"):
        names.update({label + ".stdout.log", label + ".stderr.log"})
    for label in ("Xvfb", "A", "B", "A-restart", "A-after-removal"):
        names.add(label + ".merged.log")
        if label != "Xvfb":
            names.add(label + ".observation.json")
    return names


class RefuseNewImports:
    """The post-native child may execute only its already preloaded closure."""
    def find_spec(self, fullname, path=None, target=None):
        raise ImportError("post-native import refused: " + fullname)


def publish_from_memory(output, summary, work_seconds=4.0, reap_seconds=1.0):
    pid = os.fork()
    if pid == 0:
        try:
            sys.meta_path = [RefuseNewImports()]
            accepted = publisher.publish_artifacts(output, summary, allowed_names=allowed_artifact_names())
            os._exit(0 if accepted else 10)
        except BaseException:
            os._exit(2)
    work_deadline = time.monotonic() + work_seconds
    while time.monotonic() < work_deadline:
        child, status = os.waitpid(pid, os.WNOHANG)
        if child:
            return {"complete": os.WIFEXITED(status) and os.WEXITSTATUS(status) == 0, "child_reaped": True,
                    "exit": os.waitstatus_to_exitcode(status), "archive_failure_allowed": os.WIFEXITED(status) and os.WEXITSTATUS(status) == 10,
                    "error": None if status == 0 else "artifact admission refused"}
        time.sleep(0.01)
    os.kill(pid, signal.SIGKILL)
    reap_deadline = time.monotonic() + reap_seconds
    while time.monotonic() < reap_deadline:
        child, status = os.waitpid(pid, os.WNOHANG)
        if child:
            return {"complete": False, "child_reaped": True, "exit": os.waitstatus_to_exitcode(status), "error": "artifact admission deadline"}
        time.sleep(0.01)
    return {"complete": False, "child_reaped": False, "error": "artifact admission cleanup unproved"}



def quarantine_output(output, expected):
    """Withhold the original upload namespace; never recreate its parent."""
    if output.parent.resolve() != output.parent:
        raise ValueError("quarantine parent is aliased")
    parent = os.open(output.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        current = os.stat(output.name, dir_fd=parent, follow_symlinks=False)
        if (not stat.S_ISDIR(current.st_mode) or current.st_uid != os.getuid()
                or not os.path.samestat(current, expected)):
            raise ValueError("quarantine output authority changed")
        quarantine = Path(tempfile.mkdtemp(prefix=".vscode-refused-", dir=output.parent))
        held = os.open(quarantine, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            os.rename(output.name, "evidence", src_dir_fd=parent, dst_dir_fd=held)
            moved = os.stat("evidence", dir_fd=held, follow_symlinks=False)
            if not os.path.samestat(moved, expected):
                raise ValueError("quarantine output identity changed")
        finally:
            os.close(held)
    finally:
        os.close(parent)
    return quarantine / "evidence"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        guard()
    except (RuntimeError, ValueError) as exc:
        print(str(exc)[:512], file=sys.stderr)
        return 2
    output = args.output.absolute()
    origin_ns, root = time.monotonic_ns(), None
    output.mkdir(parents=True, exist_ok=False)
    output_identity = output.lstat()
    summary = {"evidence_kind": "vscode-untrusted-workspace-metadata-research", "source_commit": os.environ["EXPECTED_COMMIT"],
               "version": publisher.VERSION, "publisher_commit": publisher.COMMIT, "collection_complete": False,
               "native_project_qualified": False, "native_mcp_initialization_qualified": False, "native_runtime_qualified": False}
    try:
        root = Path(tempfile.mkdtemp(prefix="vscode-workspace-input-", dir="/tmp"))
        identity = root.lstat()
        collect(root, output, summary, origin_ns)
    except Exception as exc:
        summary["collection_complete"] = False
        summary["error"] = type(exc).__name__ + ": " + str(exc)[:2048]
    finally:
        if root is not None:
            try:
                summary["frontend_cleanup"] = cleanup_frontend(root, identity, output)
            except Exception as exc:
                summary["frontend_cleanup"] = {"complete": False, "error": str(exc)[:512]}
            if not summary["frontend_cleanup"]["complete"]:
                summary["collection_complete"] = False
        summary["elapsed_seconds"] = (time.monotonic_ns() - origin_ns) / 1e9
        (output / "vscode-workspace-research.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
        admission = publish_from_memory(output, summary)
        if not admission["complete"]:
            summary["collection_complete"] = False
            if not (admission.get("child_reaped") and admission.get("archive_failure_allowed")):
                quarantine_output(output, output_identity)

    return 0 if summary["collection_complete"] else 2


if __name__ == "__main__":
    raise SystemExit(main())

