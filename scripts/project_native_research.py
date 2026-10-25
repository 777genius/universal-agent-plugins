"""Bounded current-source hosted research for explicitly registered project clients.

This is acquisition/observation plumbing, not a second installer or a runtime
qualification gate. Native lifecycle and MCP cases remain separate required work.
"""
import argparse
import gzip
import hashlib
import importlib.util
import io
import json
import lzma
import os
from pathlib import Path, PurePosixPath
import re
import stat
import sys
import tarfile
import tempfile
import urllib.parse
import urllib.request
import zipfile

from native_client_research import bounded_read, run_probe
import native_client_project_artifacts as artifacts


def adjacent(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


legacy = adjacent("project_native_research_budget", "research-grok-native.py")
native = legacy.native
Budget, BudgetReader = legacy.Budget, legacy.BudgetReader
SOURCE = Path(__file__).resolve().parent.parent
CLIENTS = ("gemini", "copilot", "kiro", "cursor", "cline", "qoder", "windsurf", "kimi", "grok", "vscode")
ORIGINS = {"github.com", "downloads.cursor.com", "registry.npmjs.org", "prod.download.cli.kiro.dev",
           "qoder-ide.oss-accelerate.aliyuncs.com", "x.ai", "downloads.devin.ai", "update.code.visualstudio.com"}
PHASE_CASES = {"acquire": ("acquisition",), "help": ("acquisition", "help"),
               "observe": ("acquisition", "help", "scope-observation")}
QUALIFICATION_CASES = ("project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list", "mcp-tools-call")
NODE = {"url": "https://nodejs.org/dist/v24.21.0/node-v24.21.0-linux-x64.tar.xz",
        "integrity": "sha256:fd8e59d5a511510f6a298afb548f18c7d2b1be404d8b4a27d94fbe49f56cb2d6",
        "max_bytes": 64 << 20, "max_expanded_bytes": 256 << 20, "format": "tar.xz",
        "entrypoint": "node-v24.21.0-linux-x64/bin/node", "runtime": "native"}
LABEL = re.compile(r"[a-z][a-z0-9-]{0,90}\Z")
STREAM = 1 << 20


def member_name(name):
    """Normalize only an archive's harmless leading './', never traversal."""
    while name.startswith("./"):
        name = name[2:]
    name = name.rstrip("/")
    if (not name or len(name.encode()) > 1024 or "\\" in name or "\x00" in name
            or name.startswith("/") or any(part in ("", ".", "..") for part in name.split("/"))
            or len(PurePosixPath(name).parts) > 32 or ":" in name):
        raise ValueError("unsafe archive member name")
    return name


def inventory_archive(body, spec, budget):
    """Read bounded regular members; never invoke an archive extractor or script.

Links/special entries may be described during acquisition, but cannot be
materialized into an executable distribution. Node selects only its exact binary.
"""
    entries, contents, total = [], {}, 0
    maximum = spec.get("max_expanded_bytes", 512 << 20)
    if type(maximum) is not int or not 1 <= maximum <= 768 << 20:
        raise ValueError("expanded archive cap refused")
    seen = set()

    def add(name, kind, size=0, stream=None, mode=0):
        nonlocal total
        budget.check()
        name = member_name(name)
        if name in seen or len(seen) >= 8192:
            raise ValueError("duplicate or excessive archive members")
        seen.add(name)
        if type(size) is not int or not 0 <= size <= maximum or total + size > maximum:
            raise ValueError("expanded archive byte limit exceeded")
        row = {"name": name, "kind": kind, "bytes": size, "executable": bool(mode & 0o111)}
        if kind == "file":
            data = bounded_read(BudgetReader(stream, budget), min(size + 1, maximum))
            if len(data) != size:
                raise ValueError("archive member size differs")
            total += len(data)
            row["sha256"] = hashlib.sha256(data).hexdigest()
            contents[name] = data
        entries.append(row)

    form = spec["format"]
    if form == "gzip":
        with gzip.GzipFile(fileobj=io.BytesIO(body)) as stream:
            data = bounded_read(BudgetReader(stream, budget), maximum)
        add(spec.get("entrypoint") or "payload", "file", len(data), io.BytesIO(data), 0o700)
    elif form in ("tar.gz", "tar.xz"):
        # Bound decompression before tarfile reads extension/PAX headers: entry
        # size checks alone cannot bound a malicious metadata header or padding.
        decoder = gzip.GzipFile(fileobj=io.BytesIO(body)) if form == "tar.gz" else lzma.LZMAFile(io.BytesIO(body))
        with decoder as stream:
            expanded = bounded_read(BudgetReader(stream, budget), maximum + (8 << 20))
        with tarfile.open(fileobj=io.BytesIO(expanded), mode="r:") as archive:
            for member in archive:
                if member.name.rstrip("/") in (".", "") and member.isdir():
                    continue
                if member.issparse():
                    raise ValueError("sparse archive member refused")
                kind = "file" if member.isfile() else "directory" if member.isdir() else "link" if member.issym() or member.islnk() else "special"
                stream = archive.extractfile(member) if kind == "file" else None
                try:
                    add(member.name, kind, member.size if kind == "file" else 0, stream, member.mode)
                finally:
                    if stream is not None:
                        stream.close()
    elif form == "zip":
        with zipfile.ZipFile(io.BytesIO(body)) as archive:
            if len(archive.infolist()) > 8192:
                raise ValueError("excessive archive members")
            for member in archive.infolist():
                mode = member.external_attr >> 16
                kind = "directory" if member.is_dir() else "link" if stat.S_ISLNK(mode) else "special" if stat.S_IFMT(mode) not in (0, stat.S_IFREG) else "file"
                if member.flag_bits & 1:
                    raise ValueError("encrypted archive member refused")
                with archive.open(member) as stream:
                    add(member.filename, kind, member.file_size if kind == "file" else 0, stream, mode)
    else:
        raise ValueError("unregistered archive format")
    if not contents:
        raise ValueError("archive has no regular payload")
    return entries, contents


def acquire(spec, budget, *, node=False):
    parsed = urllib.parse.urlsplit(spec["url"])
    if (parsed.scheme != "https" or parsed.hostname not in (ORIGINS | ({"nodejs.org"} if node else set()))
            or parsed.username or parsed.password or parsed.query or parsed.fragment or parsed.port):
        raise ValueError("unregistered immutable acquisition origin")
    maximum = spec["max_bytes"]
    if type(maximum) is not int or not 1 <= maximum <= 512 << 20:
        raise ValueError("compressed archive cap refused")
    # No ambient proxy or credential handlers; HTTPS redirects are checked before
    # reading and TLS verification remains the Python default.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(spec["url"], timeout=min(90, budget.check())) as response:
        if urllib.parse.urlsplit(response.url).scheme != "https":
            raise ValueError("non-HTTPS acquisition redirect refused")
        body = bounded_read(BudgetReader(response, budget), maximum)
    if spec.get("expected_bytes") is not None and len(body) != spec["expected_bytes"]:
        raise ValueError("archive size differs from reviewed pin")
    pin = spec.get("integrity")
    if pin is not None:
        native.verify_digest(body, pin)
    entries, contents = inventory_archive(body, spec, budget)
    resolved = urllib.parse.urlsplit(response.url)
    return {"url": spec["url"], "resolved_origin": resolved.scheme + "://" + resolved.netloc, "bytes": len(body),
            "sha256": hashlib.sha256(body).hexdigest(), "integrity": pin,
            "pin_verified": pin is not None, "members": entries, "installer_executed": False}, contents


def materialize(directory, spec, receipt, contents, *, selected_only=False):
    if not receipt["pin_verified"] or not spec.get("entrypoint"):
        raise ValueError("reviewed archive integrity and entrypoint required before execution")
    entrypoint = member_name(spec["entrypoint"])
    if entrypoint not in contents or not contents[entrypoint]:
        raise ValueError("exact nonempty entrypoint absent")
    if not selected_only and any(row["kind"] not in ("file", "directory") for row in receipt["members"]):
        raise ValueError("executable distribution contains unreviewed links/special members")
    selected = {entrypoint: contents[entrypoint]} if selected_only else contents
    rows = {row["name"]: row for row in receipt["members"]}
    identities = {}
    for name, body in selected.items():
        path = directory / name
        path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        with path.open("xb") as output:
            output.write(body)
        path.chmod(0o700 if name == entrypoint or rows[name]["executable"] else 0o600)
        identities[path] = rows[name]["sha256"]
    binary = directory / entrypoint
    if spec.get("binary_sha256") and identities[binary] != spec["binary_sha256"]:
        raise ValueError("executable differs from reviewed binary pin")
    return binary, identities


def verify_files(identities, budget):
    for path, expected in identities.items():
        budget.check()
        before = path.lstat()
        if (path.resolve() != path or not stat.S_ISREG(before.st_mode) or before.st_nlink != 1
                or before.st_uid != os.getuid() or before.st_size > 512 << 20):
            raise ValueError("runtime member identity/type refused")
        fd = artifacts.open_member(path)
        try:
            held = os.fstat(fd)
            if not artifacts.same_opened(before, held):
                raise ValueError("runtime member changed while opening")
            digest = hashlib.sha256()
            size = 0
            while True:
                budget.check()
                chunk = os.read(fd, 64 << 10)
                if not chunk:
                    break
                size += len(chunk)
                if size > before.st_size:
                    raise ValueError("runtime member grew")
                digest.update(chunk)
            if artifacts.identity(os.fstat(fd)) != artifacts.identity(held):
                raise ValueError("runtime member changed while reading")
        finally:
            os.close(fd)
        if (size != before.st_size or digest.hexdigest() != expected
                or artifacts.identity(path.lstat()) != artifacts.identity(before)):
            raise ValueError("runtime bytes changed from pinned acquisition")


class Context:
    def __init__(self, root, summary, budget):
        self.root, self.summary, self.budget = root, summary, budget
        self.home, self.private, self.output = root / "home", root / "private", root / "evidence"
        self.projects = {"a": root / "project-a", "b": root / "project-b", "user": root / "outside"}
        for path in (self.home, self.private, self.output, root / "tmp", *self.projects.values()):
            path.mkdir(mode=0o700)
        self.env = legacy.runtime_environment(self.home, root / "tmp")
        self.env.update({"NO_COLOR": "1", "TERM": "dumb"})
        self.argv, self.identities, self.captures, self.secrets = [], {}, {}, []

    def register_secret(self, value):
        value = value.encode() if isinstance(value, str) else value
        if not isinstance(value, bytes) or not 8 <= len(value) <= 4096:
            raise ValueError("invalid private secret registration")
        self.secrets.append(value)

    def command(self, label, argv, cwd, env, timeout=30):
        native.require_hosted("linux-amd64")
        if not LABEL.fullmatch(label) or any(row["probe"] == label for row in self.summary["commands"]):
            raise ValueError("invalid or duplicate probe label")
        if len(self.summary["commands"]) >= 60:
            raise ValueError("command count limit exceeded")
        record, stdout, stderr = run_probe([str(part) for part in argv], cwd, env, self.private, label,
                                          timeout=min(timeout, self.budget.check()), limit=STREAM)
        self.summary["commands"].append(record)
        self.captures[record["stdout_file"]] = stdout
        self.captures[record["stderr_file"]] = stderr
        if record["error"] or record["exit"] is None:
            raise ValueError(label + " failed its bounded capture")
        return record, stdout, stderr

    def probe(self, label, arguments, project="a", extra_env=None, timeout=30):
        if project not in self.projects or not 0 < timeout <= 60 or not self.argv:
            raise ValueError("unregistered native project/timeout/argv")
        env = dict(self.env)
        # Concrete leaves may add documented client controls, but may not replace
        # profile authority, runtime lookup or inject process-loader controls.
        for key, value in (extra_env or {}).items():
            if key in env and env[key] == value:
                continue
            if key in env or not re.fullmatch(r"(?:CLINE|GEMINI|COPILOT|KIRO|CURSOR|KIMI|GROK|QODER|DEVIN)_[A-Z0-9_]+", key) or not isinstance(value, str):
                raise ValueError("native environment override refused")
            env[key] = value
        verify_files(self.identities, self.budget)
        try:
            return self.command(label, [*self.argv, *arguments], self.projects[project], env, timeout)
        finally:
            verify_files(self.identities, self.budget)

    def source_and_projects(self):
        values = {}
        for label, arguments, key in (("source-commit", ["rev-parse", "HEAD"], "source_commit"),
                                      ("source-tree", ["rev-parse", "HEAD^{tree}"], "source_tree"),
                                      ("source-clean", ["status", "--porcelain", "--untracked-files=normal"], "dirty")):
            record, stdout, stderr = self.command(label, ["/usr/bin/git", *arguments], SOURCE, self.env, 10)
            if record["exit"] != 0 or stderr:
                raise ValueError("source identity query failed")
            values[key] = stdout.decode("ascii", errors="strict").strip()
        expected = os.environ.get("EXPECTED_COMMIT", "")
        if (not re.fullmatch(r"[0-9a-f]{40}", expected) or values["source_commit"] != expected
                or not re.fullmatch(r"[0-9a-f]{40}", values["source_tree"]) or values["dirty"]):
            raise ValueError("exact clean current-source checkout required")
        self.summary.update({key: value for key, value in values.items() if key != "dirty"})
        for key in ("a", "b"):
            record, _, _ = self.command("git-init-" + key, ["/usr/bin/git", "init", "--quiet", "--initial-branch=main"], self.projects[key], self.env, 10)
            if record["exit"] != 0:
                raise ValueError("owned real project creation failed")


def validate_cases(client, phase, cases):
    required = {client + "." + name for name in PHASE_CASES[phase]}
    if not isinstance(cases, list) or len(cases) != len(required):
        raise ValueError("missing or extra selected research cases")
    seen = set()
    for row in cases:
        if (not isinstance(row, dict) or row.get("id") not in required or row["id"] in seen
                or row.get("status") != "passed" or not isinstance(row.get("reason"), str) or not row["reason"]):
            raise ValueError("selected research case did not pass explicitly")
        seen.add(row["id"])
    if seen != required:
        raise ValueError("selected research case was not observed")


def evidence(summary, captures, secrets):
    """Admit only recorded streams and JSON; never native files or directories."""
    summary = json.loads(json.dumps(summary))
    bodies, expected = {}, set()
    for record in summary["commands"]:
        for stream in ("stdout", "stderr"):
            name = record[stream + "_file"]
            if name != record["probe"] + "." + stream + ".log" or name in expected or not artifacts.NAME.fullmatch(name):
                raise ValueError("unrecorded or duplicate capture refused")
            expected.add(name)
            body = captures[name]
            if len(body) > STREAM or hashlib.sha256(body).hexdigest() != record[stream + "_sha256"] or len(body) != record[stream + "_bytes"]:
                raise ValueError("capture receipt differs")
            for secret in secrets:
                body = body.replace(secret, b"[redacted]")
            record[stream + "_bytes"] = len(body)
            record[stream + "_sha256"] = hashlib.sha256(body).hexdigest()
            bodies[name] = body
    if set(captures) != expected:
        raise ValueError("extra private capture refused")
    body = json.dumps(summary, sort_keys=True, indent=2).encode() + b"\n"
    for secret in secrets:
        body = body.replace(secret, b"[redacted]")
    bodies["research.json"] = body
    admit_bodies(bodies, secrets)
    return bodies


def admit_bodies(bodies, secrets):
    if len(bodies) > artifacts.FILES or "research.json" not in bodies or sum(map(len, bodies.values())) > artifacts.LIMIT - 65536:
        raise ValueError("evidence aggregate refused")
    for name, body in bodies.items():
        if not artifacts.NAME.fullmatch(name) or len(body) > STREAM or any(secret in name.encode() or secret in body for secret in secrets):
            raise ValueError("private token or unsafe evidence member refused")
    json.loads(bodies["research.json"])


def run(client, phase, output):
    native.require_hosted("linux-amd64")
    if output.resolve() != output or output.exists() or SOURCE == output or SOURCE in output.parents:
        raise ValueError("fresh ordinary output outside source required")
    output.mkdir(mode=0o700, parents=True)
    summary = {"client": client, "phase": phase, "collection_complete": False,
               "qualification": {name: False for name in QUALIFICATION_CASES}, "commands": [], "cases": [],
               "host": {key: os.environ.get(key) for key in ("RUNNER_OS", "RUNNER_ARCH", "ImageOS", "ImageVersion")}}
    ctx = None
    try:
        with Budget(360) as budget:
            ctx = Context(output, summary, budget)
            ctx.source_and_projects()
            leaf = adjacent("project_native_leaf_" + client, "project_native_collectors/" + client + ".py")
            phases = getattr(leaf, "AVAILABLE_PHASES", ("acquire",) if getattr(leaf, "ACQUISITION_ONLY", False) else ())
            if leaf.CLIENT != client or phase not in phases:
                raise ValueError("client phase has no reviewed collector")
            summary["version"] = leaf.VERSION
            receipt, contents = acquire(leaf.ACQUISITION, budget)
            summary["acquisition"] = receipt
            summary["cases"].append({"id": client + ".acquisition", "status": "passed", "reason": "Bounded official immutable archive inventoried; pin status is explicit"})
            if phase != "acquire":
                if getattr(leaf, "ACQUISITION_ONLY", False):
                    raise ValueError("acquisition-only client cannot execute")
                binary, identities = materialize(output / "client", leaf.ACQUISITION, receipt, contents)
                ctx.identities.update(identities)
                ctx.argv = [str(binary)]
                if leaf.ACQUISITION["runtime"] == "node":
                    node_receipt, node_contents = acquire(NODE, budget, node=True)
                    node, node_ids = materialize(output / "node", NODE, node_receipt, node_contents, selected_only=True)
                    ctx.identities.update(node_ids)
                    ctx.argv.insert(0, str(node))
                    summary["node"] = node_receipt
                    ctx.env["PATH"] = str(node.parent) + ":/usr/bin:/bin"
                    verify_files(ctx.identities, budget)
                    record, stdout, stderr = ctx.command("node-version", [node, "--version"], ctx.projects["a"], ctx.env, 10)
                    verify_files(ctx.identities, budget)
                    if record["exit"] != 0 or stdout != b"v24.21.0\n" or stderr:
                        raise ValueError("exact pinned Node runtime version required")
                elif leaf.ACQUISITION["runtime"] != "native":
                    raise ValueError("unregistered native runtime")
                summary["runtime_files"] = [{"path": str(path), "sha256": digest} for path, digest in ctx.identities.items()]
                result = leaf.collect(ctx)
                if not isinstance(result, dict) or set(result) - {"observations", "cases", "unobserved"}:
                    raise ValueError("invalid collector result")
                summary["observations"] = result.get("observations", {})
                summary["cases"].extend(result["cases"])
                summary["unobserved"] = result.get("unobserved", [])
            validate_cases(client, phase, summary["cases"])
            summary["collection_complete"] = True
    except Exception as error:
        summary["error"] = type(error).__name__ + ": " + str(error)[:512]
    finally:
        bodies = evidence(summary, ctx.captures if ctx else {}, ctx.secrets if ctx else [])
        destination = output / "upload"
        destination.mkdir(mode=0o700)
        artifacts.write_zip(destination / "research.zip", bodies)
        # A short CI log contains no native transcript or private profile.
        print(json.dumps({key: summary.get(key) for key in ("client", "phase", "collection_complete", "source_commit", "source_tree")}))
    return 0 if summary["collection_complete"] else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--matrix", action="store_true", help="List explicitly declared research phases; never run a client")
    parser.add_argument("--client", choices=CLIENTS)
    parser.add_argument("--phase", choices=PHASE_CASES)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if args.matrix:
        entries = []
        for client in CLIENTS:
            path = SOURCE / "scripts" / "project_native_collectors" / (client + ".py")
            if not path.exists():
                continue  # Initial slices arrive independently; no qualification is implied.
            leaf = adjacent("project_native_matrix_" + client, "project_native_collectors/" + client + ".py")
            phases = getattr(leaf, "AVAILABLE_PHASES", ("acquire",) if getattr(leaf, "ACQUISITION_ONLY", False) else ())
            if leaf.CLIENT != client or not phases or any(phase not in PHASE_CASES for phase in phases):
                raise ValueError("invalid explicit client phase registration")
            phase = next(phase for phase in ("observe", "help", "acquire") if phase in phases)
            entries.append({"client": client, "phase": phase, "host": "ubuntu-22.04" if client == "vscode" else "ubuntu-24.04"})
        print(json.dumps({"include": entries}, separators=(",", ":")))
        return 0
    if args.client is None or args.phase is None or args.output is None:
        parser.error("--client, --phase and --output are required outside --matrix")
    return run(args.client, args.phase, args.output)


if __name__ == "__main__":
    sys.exit(main())
