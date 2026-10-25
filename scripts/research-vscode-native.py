#!/usr/bin/env python3
"""Pinned VS Code help/version research; genuine disposable hosted Linux only.

This collector never opens an editor, extension host, workspace, model or MCP
session. Successful collection is not project installation qualification.
"""
import argparse
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.parse
import urllib.request
import zipfile

from native_client_research import bounded_read, run_probe


def adjacent(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


native = adjacent("vscode_research_matrix", "run-native-client-matrix.py")
bounds = adjacent("vscode_research_bounds", "research-grok-native.py")
VERSION = "1.143.1"
COMMIT = "e646868227c3e3a9e3f24277c8c7220be42c0be7"
URL = "https://vscode.download.prss.microsoft.com/dbazure/download/stable/" + COMMIT + "/code-stable-x64-1792691793.tar.gz"
ARCHIVE_BYTES = 350_844_741
ARCHIVE_SHA256 = "48f658052e54e020a1c1be7b37ac0f9ae39f07e8ccda805c63f053cfda72b2be"
PREFIX = "VSCode-linux-x64"
REQUIRED = ("code", "resources/app/out/cli.js", "resources/app/product.json")
MAX_EXPANDED, MAX_FILE, MAX_MEMBERS = 2 << 30, 512 << 20, 100_000
MAX_NAME, MAX_DEPTH, MAX_METADATA = 512, 32, 4096
MAX_ARTIFACT = 32 << 20
MIN_FREE = 4 << 30
PROBES = (("version", "--version"), ("help", "--help"))


class PublisherRedirects(urllib.request.HTTPRedirectHandler):
    def __init__(self):
        self.count = 0

    def redirect_request(self, request, response, code, message, headers, newurl):
        self.count += 1
        parsed = urllib.parse.urlsplit(newurl)
        if self.count > 3 or parsed.scheme != "https" or parsed.hostname != "vscode.download.prss.microsoft.com" or parsed.username or parsed.password or parsed.port not in (None, 443):
            raise ValueError("publisher redirect refused")
        return super().redirect_request(request, response, code, message, headers, newurl)


def acquire_archive(root, budget):
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), PublisherRedirects())
    path, digest, count = root / "publisher.tar.gz", hashlib.sha256(), 0
    with opener.open(URL, timeout=min(90, budget.check())) as response, path.open("xb") as output:
        while True:
            budget.check()
            chunk = response.read(min(64 << 10, ARCHIVE_BYTES + 1 - count))
            if not chunk:
                break
            count += len(chunk)
            if count > ARCHIVE_BYTES:
                raise ValueError("publisher archive exceeds exact byte bound")
            digest.update(chunk)
            output.write(chunk)
    if count != ARCHIVE_BYTES or digest.hexdigest() != ARCHIVE_SHA256:
        raise ValueError("publisher archive integrity/size mismatch")
    return path


class Expanded:
    def __init__(self, source, budget):
        self.source, self.budget, self.count = source, budget, 0

    def read(self, count):
        self.budget.check()
        body = self.source.read(min(count, MAX_EXPANDED + 1 - self.count))
        self.count += len(body)
        if self.count > MAX_EXPANDED:
            raise ValueError("expanded archive byte bound exceeded")
        return body

    def exact(self, count):
        chunks = []
        while count:
            part = self.read(min(count, 64 << 10))
            if not part:
                raise ValueError("truncated publisher archive")
            chunks.append(part)
            count -= len(part)
        return b"".join(chunks)


def path_parts(name, directory=False):
    if directory:
        name = name.removesuffix("/")
    if not name or len(name.encode("utf-8")) > MAX_NAME or "\\" in name or ":" in name or any(ord(c) < 32 or ord(c) == 127 for c in name):
        raise ValueError("unsafe archive member name")
    parts = name.split("/")
    if len(parts) > MAX_DEPTH or parts[0] != PREFIX or any(part in ("", ".", "..") for part in parts):
        raise ValueError("archive member escapes fixed publisher root")
    return parts


def pax_values(body):
    result, offset = {}, 0
    while offset < len(body):
        space = body.find(b" ", offset)
        if space < 0 or space - offset > 8 or not body[offset:space].isdigit():
            raise ValueError("invalid bounded PAX record")
        length = int(body[offset:space])
        end = offset + length
        if length <= space - offset + 2 or end > len(body) or body[end - 1:end] != b"\n":
            raise ValueError("invalid PAX record length")
        text = body[space + 1:end - 1].decode("utf-8", "strict")
        key, separator, value = text.partition("=")
        if not separator or not key or len(key.encode()) > MAX_NAME or len(value.encode()) > MAX_NAME or key in result or "sparse" in key.lower():
            raise ValueError("unsupported or oversized PAX metadata")
        result[key] = value
        offset = end
    return result


def directory_fd(root_fd, parts):
    current = os.dup(root_fd)
    try:
        for part in parts:
            try:
                os.mkdir(part, mode=0o700, dir_fd=current)
            except FileExistsError:
                pass
            child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=current)
            os.close(current)
            current = child
        return current
    except BaseException:
        os.close(current)
        raise


def write_member(root_fd, parts, stream, size):
    parent = directory_fd(root_fd, parts[:-1])
    try:
        descriptor = os.open(parts[-1], os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=parent)
        with os.fdopen(descriptor, "wb") as output:
            remaining = size
            while remaining:
                body = stream.exact(min(remaining, 64 << 10))
                output.write(body)
                remaining -= len(body)
    finally:
        os.close(parent)


def materialize_archive(path, destination, budget):
    """Inspect fixed-size headers before metadata allocation; materialize no links."""
    seen, spellings, global_pax, local_pax = {}, {}, {}, {}
    omitted, omitted_hash, count, regular_bytes = 0, hashlib.sha256(), 0, 0
    root_fd = os.open(destination, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        with path.open("rb") as compressed, gzip.GzipFile(fileobj=compressed) as source:
            stream = Expanded(source, budget)
            while True:
                block = stream.exact(512)
                if block == bytes(512):
                    if stream.exact(512) != bytes(512):
                        raise ValueError("invalid TAR ending")
                    while True:
                        tail = stream.read(64 << 10)
                        if not tail:
                            break
                        if any(tail):
                            raise ValueError("nonzero trailing expanded archive data")
                    if local_pax:
                        raise ValueError("dangling archive metadata")
                    break
                count += 1
                if count > MAX_MEMBERS:
                    raise ValueError("publisher member count exceeded")
                # frombuf parses only this fixed header; no archive extension hooks run.
                member = tarfile.TarInfo.frombuf(block, "utf-8", "strict")
                if len(member.name.encode("utf-8")) > MAX_NAME or len(member.linkname.encode("utf-8")) > MAX_NAME:
                    raise ValueError("archive header metadata exceeds bound")
                if member.type == tarfile.GNUTYPE_SPARSE:
                    raise ValueError("GNU sparse archive member refused")
                if member.type in (tarfile.XHDTYPE, tarfile.XGLTYPE, tarfile.GNUTYPE_LONGNAME, tarfile.GNUTYPE_LONGLINK):
                    if not 0 < member.size <= MAX_METADATA:
                        raise ValueError("archive extension metadata exceeds bound")
                    body = stream.exact(member.size)
                    padding = stream.exact((-member.size) % 512)
                    if any(padding):
                        raise ValueError("nonzero metadata padding")
                    if member.type in (tarfile.XHDTYPE, tarfile.XGLTYPE):
                        values = pax_values(body)
                        if member.type == tarfile.XGLTYPE:
                            global_pax.update(values)
                        else:
                            local_pax.update(values)
                    else:
                        if not body.endswith(b"\0") or b"\0" in body[:-1] or len(body) - 1 > MAX_NAME:
                            raise ValueError("invalid GNU long metadata")
                        local_pax["path" if member.type == tarfile.GNUTYPE_LONGNAME else "linkpath"] = body[:-1].decode("utf-8", "strict")
                    continue
                metadata = {**global_pax, **local_pax}
                local_pax = {}
                name = metadata.get("path", member.name)
                target = metadata.get("linkpath", member.linkname)
                size_text = metadata.get("size", str(member.size))
                if not re.fullmatch(r"[0-9]{1,12}", size_text):
                    raise ValueError("invalid bounded archive size")
                size = int(size_text)
                parts = path_parts(name, member.isdir())
                for index in range(1, len(parts) + 1):
                    spelling = "/".join(parts[:index])
                    folded = spelling.casefold()
                    if folded in spellings and spellings[folded] != spelling:
                        raise ValueError("case-colliding archive parent")
                    spellings[folded] = spelling
                key = "/".join(parts).casefold()
                if key in seen or any(seen.get("/".join(parts[:i]).casefold()) == "file" for i in range(1, len(parts))):
                    raise ValueError("duplicate/case-colliding or file-parent archive member")
                if member.type in (tarfile.REGTYPE, tarfile.AREGTYPE):
                    if size > MAX_FILE or any(existing.startswith(key + "/") for existing in seen):
                        raise ValueError("regular archive member bound/parent conflict")
                    regular_bytes += size
                    if regular_bytes > MAX_EXPANDED:
                        raise ValueError("publisher aggregate file bytes exceeded")
                    seen[key] = "file"
                    write_member(root_fd, parts, stream, size)
                elif member.isdir() and size == 0:
                    seen[key] = "directory"
                    descriptor = directory_fd(root_fd, parts)
                    os.close(descriptor)
                elif member.issym() and size == 0:
                    if len(target.encode("utf-8")) > MAX_NAME or any(ord(c) < 32 or ord(c) == 127 for c in target):
                        raise ValueError("invalid omitted link metadata")
                    seen[key] = "omitted-link"
                    omitted += 1
                    omitted_hash.update(json.dumps([name, target], separators=(",", ":")).encode())
                else:
                    raise ValueError("nonregular publisher archive member refused")
                if any(stream.exact((-size) % 512)):
                    raise ValueError("nonzero archive member padding")
        return {"members": count, "expanded_bytes": stream.count, "regular_bytes": regular_bytes,
                "omitted_symlinks": omitted, "omitted_symlink_metadata_sha256": omitted_hash.hexdigest()}
    finally:
        os.close(root_fd)


def required_files(root, budget=None):
    result = {}
    for name in REQUIRED:
        path = root / PREFIX / name
        current = root
        for part in (PREFIX, *name.split("/")):
            current = current / part
            info = current.lstat()
            if stat.S_ISLNK(info.st_mode):
                raise ValueError("required publisher path traverses a link")
        descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        with os.fdopen(descriptor, "rb") as source:
            before = os.fstat(source.fileno())
            if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or before.st_size > MAX_FILE:
                raise ValueError("required publisher file is not a bounded regular file")
            digest, size, first = hashlib.sha256(), 0, b""
            while True:
                if budget is not None:
                    budget.check()
                body = source.read(64 << 10)
                if not body:
                    break
                if not first:
                    first = body[:4]
                size += len(body)
                if size > MAX_FILE:
                    raise ValueError("required publisher file grew beyond bound")
                digest.update(body)
            after = os.fstat(source.fileno())
            if bounds.regular_identity(before) != bounds.regular_identity(after) or bounds.regular_identity(path.lstat()) != bounds.regular_identity(after) or size != before.st_size:
                raise ValueError("required publisher file changed while hashing")
        if name == "code" and first != b"\x7fELF":
            raise ValueError("publisher code member is not ELF")
        result[name] = {"sha256": digest.hexdigest(), "bytes": size, "identity": [before.st_dev, before.st_ino]}
    return result


ISOLATION_VERIFIER = r'''
import json,os,socket,sys
parent,uid,gid,output,env,argv=sys.argv[1:]
observed={"network_namespace":os.stat("/proc/self/ns/net").st_ino,"uid":os.getuid(),"gid":os.getgid(),"euid":os.geteuid(),"egid":os.getegid(),"groups":os.getgroups(),"interfaces":[name for _,name in socket.if_nameindex()]}
if observed["network_namespace"]==int(parent) or observed["uid"]!=int(uid) or observed["gid"]!=int(gid) or observed["uid"]==0 or observed["gid"]==0 or observed["euid"]!=int(uid) or observed["egid"]!=int(gid) or observed["groups"] or observed["interfaces"]!=["lo"]:
    raise SystemExit("isolated native process admission failed")
with open("/proc/self/status") as source:
    status=dict(line.rstrip().split(":",1) for line in source if ":" in line)
if any(int(status[key].strip(),16) for key in ("CapInh","CapPrm","CapEff","CapBnd","CapAmb")) or status["NoNewPrivs"].strip()!="1":
    raise SystemExit("native privilege drop failed")
observed["capabilities_zero"]=True
observed["no_new_privileges"]=True
with open(output,"x") as stream:json.dump(observed,stream,sort_keys=True)
os.execve(json.loads(argv)[0],json.loads(argv),json.loads(env))
'''

CLEANUP_CODE = r'''
import os,stat,sys
path,device,inode=sys.argv[1:]
parent,name=os.path.split(path)
pfd=os.open(parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
root=os.open(name,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=pfd)
identity=os.fstat(root)
if (identity.st_dev,identity.st_ino)!=(int(device),int(inode)):raise SystemExit("cleanup root identity changed")
def clear(fd):
    for name in os.listdir(fd):
        item=os.stat(name,dir_fd=fd,follow_symlinks=False)
        if stat.S_ISDIR(item.st_mode):
            child=os.open(name,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)
            try:
                if not os.path.samestat(item,os.fstat(child)):raise RuntimeError("cleanup directory changed")
                clear(child)
            finally:os.close(child)
            os.rmdir(name,dir_fd=fd)
        else:os.unlink(name,dir_fd=fd)
clear(root)
if not os.path.samestat(identity,os.stat(name,dir_fd=pfd,follow_symlinks=False)):raise SystemExit("cleanup root replaced")
os.close(root)
os.rmdir(name,dir_fd=pfd)
os.close(pfd)
'''


def cleanup(root, identity, output, *, isolated=False):
    interpreter = ["/usr/bin/python3", "-I"] if isolated else [sys.executable]
    cwd = Path("/") if isolated else output
    record, _, _ = run_probe(interpreter + ["-c", CLEANUP_CODE, str(root), str(identity.st_dev), str(identity.st_ino)],
                              cwd, {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8"}, output, "cleanup", timeout=5)
    return {"complete": record["exit"] == 0 and record["error"] is None, "probe": record}


def probe_environment(root):
    for name in ("home", "tmp", "user-data", "extensions", "cwd"):
        (root / name).mkdir()
    env = native.profile_environment(root / "home", "linux-amd64")
    env.update({"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8", "TMPDIR": str(root / "tmp"),
                "VSCODE_AGENT_FOLDER": str(root / "user-data"), "ELECTRON_RUN_AS_NODE": "1"})
    return env


def isolated_command(root, label, env, flag, parent_ns):
    if os.getuid() == 0 or os.getgid() == 0 or os.geteuid() != os.getuid() or os.getegid() != os.getgid():
        raise ValueError("hosted native research requires the original nonroot user")
    argv = [str(root / "publisher" / PREFIX / "code"), str(root / "publisher" / PREFIX / REQUIRED[1]), flag,
            "--user-data-dir", str(root / label / "user-data"), "--extensions-dir", str(root / label / "extensions")]
    facts = root / label / "isolation.json"
    command = ["/usr/bin/sudo", "-n", "/usr/bin/unshare", "--net", "/usr/bin/setpriv",
               "--reuid=" + str(os.getuid()), "--regid=" + str(os.getgid()), "--clear-groups",
               "--bounding-set=-all", "--inh-caps=-all", "--ambient-caps=-all", "--no-new-privs",
               "/usr/bin/env", "-i", "/usr/bin/python3", "-c", ISOLATION_VERIFIER,
               str(parent_ns), str(os.getuid()), str(os.getgid()), str(facts), json.dumps(env), json.dumps(argv)]
    return command, argv, facts


def validate_probe(record, stdout, stderr, label):
    if record["error"] is not None or record["exit"] != 0:
        return False
    try:
        text = stdout.decode("utf-8", "strict")
        stderr.decode("utf-8", "strict")
    except UnicodeError:
        return False
    if label == "version":
        return VERSION in text.split() and COMMIT in text.split()
    return bool(text.strip())


def artifact_bound(output):
    files, size = [], 0
    for path in output.iterdir():
        info = path.lstat()
        if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1
                or not re.fullmatch(r"[A-Za-z0-9._-]{1,128}", path.name)):
            raise ValueError("research artifact is not a flat ordinary file")
        size += info.st_size
        files.append((path, info))
        # Reserve final ZIP directory/header overhead; raw output is never uploaded.
        if size > MAX_ARTIFACT - (64 << 10) or len(files) > 64:
            raise ValueError("research artifact exceeds total byte/member bound")
    return files


def publish_artifacts(output, summary, allowed_names=None):
    """Publish one bounded ZIP, or only a bounded failure ZIP, to upload/."""
    stage = output / "upload"
    error = None
    descriptor, name = tempfile.mkstemp(prefix=".upload-", dir=output)
    temporary = Path(name)
    # Take the source list before the ZIP temporary or upload directory exists.
    os.close(descriptor)
    temporary.unlink()
    try:
        files = artifact_bound(output)
        if allowed_names is not None and any(path.name not in allowed_names for path, _ in files):
            raise ValueError("research artifact name is outside the closed workspace table")
        descriptor = os.open(temporary, os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(descriptor, "w+b") as container, zipfile.ZipFile(container, "w", compression=zipfile.ZIP_STORED) as archive:
            total = 0
            for path, recorded in files:
                source_fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
                with os.fdopen(source_fd, "rb") as source, archive.open(path.name, "w") as target:
                    if bounds.regular_identity(os.fstat(source.fileno())) != bounds.regular_identity(recorded):
                        raise ValueError("research artifact changed before upload staging")
                    size = 0
                    while True:
                        body = source.read(min(64 << 10, recorded.st_size + 1 - size))
                        if not body:
                            break
                        size += len(body)
                        total += len(body)
                        if size > recorded.st_size or total > MAX_ARTIFACT - (64 << 10):
                            raise ValueError("research artifact grew while staging")
                        target.write(body)
                    if (size != recorded.st_size
                            or bounds.regular_identity(os.fstat(source.fileno())) != bounds.regular_identity(recorded)
                            or bounds.regular_identity(path.lstat()) != bounds.regular_identity(recorded)):
                        raise ValueError("research artifact changed during upload staging")
        if temporary.stat().st_size > MAX_ARTIFACT:
            raise ValueError("staged ZIP exceeds upload bound")
    except Exception as exc:
        error = str(exc)[:512]
        temporary.unlink(missing_ok=True)
        failure = {"evidence_kind": "vscode-research-upload-refusal", "version": VERSION,
                   "source_commit": str(summary.get("source_commit", ""))[:128],
                   "collection_complete": False, "native_project_qualified": False,
                   "artifact_admission_error": error, "collection_error": str(summary.get("error", ""))[:512]}
        descriptor = os.open(temporary, os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(descriptor, "w+b") as container, zipfile.ZipFile(container, "w", compression=zipfile.ZIP_STORED) as archive:
            archive.writestr("collection-failure.json", json.dumps(failure, indent=2) + "\n")
        if temporary.stat().st_size > MAX_ARTIFACT:
            raise ValueError("bounded failure ZIP cannot fit configured upload bound")
    # upload/ did not exist during native execution and contains only this envelope.
    stage.mkdir(mode=0o700, exist_ok=False)
    os.link(temporary, stage / "research.zip", follow_symlinks=False)
    temporary.unlink()
    return error is None


def collect(root, output, summary, budget):
    if (os.getuid() == 0 or os.getgid() == 0 or os.geteuid() != os.getuid() or os.getegid() != os.getgid()
            or shutil.disk_usage(root).free < MIN_FREE):
        raise ValueError("nonroot hosted user and at least four GiB free required")
    archive = acquire_archive(root, budget)
    publisher = root / "publisher"
    publisher.mkdir()
    summary["archive"] = {"url": URL, "bytes": ARCHIVE_BYTES, "sha256": ARCHIVE_SHA256}
    summary["materialization"] = materialize_archive(archive, publisher, budget)
    before = required_files(publisher, budget)
    (publisher / PREFIX / "code").chmod(0o700)
    parent_ns = os.stat("/proc/self/ns/net").st_ino
    for label, flag in PROBES:
        probe_root = root / label
        probe_root.mkdir()
        env = probe_environment(probe_root)
        if required_files(publisher, budget) != before:
            raise ValueError("publisher identity changed before probe")
        command, argv, facts = isolated_command(root, label, env, flag, parent_ns)
        record, stdout, stderr = run_probe(command, probe_root / "cwd", {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8"},
                                           output, label, timeout=min(20, budget.check()))
        record.update({"native_argv": argv, "native_environment": env, "native_cwd": str(probe_root / "cwd")})
        record["accepted"] = validate_probe(record, stdout, stderr, label)
        if facts.is_file() and not facts.is_symlink():
            descriptor = os.open(facts, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
            with os.fdopen(descriptor, "rb") as source:
                identity = os.fstat(source.fileno())
                if not stat.S_ISREG(identity.st_mode) or identity.st_nlink != 1 or identity.st_size > MAX_METADATA:
                    raise ValueError("isolation facts are not a bounded ordinary file")
                body = bounded_read(source, MAX_METADATA)
                if bounds.regular_identity(identity) != bounds.regular_identity(os.fstat(source.fileno())):
                    raise ValueError("isolation facts changed while reading")
            facts_record = json.loads(body, object_pairs_hook=bounds.unique_object, parse_constant=bounds.invalid_constant)
            if (facts_record.get("network_namespace") == parent_ns or facts_record.get("uid") != os.getuid()
                    or facts_record.get("gid") != os.getgid() or facts_record.get("euid") != os.getuid()
                    or facts_record.get("egid") != os.getgid() or facts_record.get("groups") != []
                    or facts_record.get("interfaces") != ["lo"] or facts_record.get("capabilities_zero") is not True
                    or facts_record.get("no_new_privileges") is not True):
                raise ValueError("reported isolation facts do not match required boundary")
            record["isolation"] = facts_record
        else:
            record["accepted"] = False
        summary["probes"].append(record)
        if required_files(publisher, budget) != before:
            raise ValueError("publisher identity changed during probe")
        if not record["accepted"]:
            raise ValueError("pinned publisher probe or isolation failed")
    summary["required_files"] = before
    summary["collection_complete"] = True


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        native.require_hosted("linux-amd64")
    except RuntimeError as exc:
        print(str(exc)[:512], file=sys.stderr)
        return 2
    output = args.output.absolute()
    output.mkdir(parents=True, exist_ok=False)
    root = None
    summary = {"evidence_kind": "vscode-pinned-help-version-research", "version": VERSION,
               "publisher_commit": COMMIT, "source_commit": os.environ.get("EXPECTED_COMMIT", ""),
               "collection_complete": False, "native_project_qualified": False,
               "interpretation": "Independent publisher observation review required; no workspace contract inferred",
               "probes": [], "unobserved": ["workspace skills/MCP", "UAP installation ownership", "trust", "extension host/UI", "model/auth", "native restart"]}
    try:
        with bounds.Budget() as budget:
            root = Path(tempfile.mkdtemp(prefix="vscode-native-research-"))
            identity = root.lstat()
            collect(root, output, summary, budget)
    except Exception as exc:
        summary["collection_complete"] = False
        summary["error"] = f"{type(exc).__name__}: {str(exc)[:512]}"
    finally:
        if root is not None:
            try:
                summary["cleanup"] = cleanup(root, identity, output)
            except Exception as exc:
                summary["cleanup"] = {"complete": False, "error": str(exc)[:512]}
            if not summary["cleanup"]["complete"]:
                summary["collection_complete"] = False
    (output / "vscode-precode-research.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    uploaded = publish_artifacts(output, summary)
    return 0 if summary["collection_complete"] and uploaded else 1


if __name__ == "__main__":
    sys.exit(main())
