"""One hosted Kiro 3.0.0 ACP boundary capture; never lifecycle qualification.

The immutable ZIP pin is the Linux x86_64 GNU row of the vendor's
https://prod.download.cli.kiro.dev/stable/index.json, independently read on
2026-11-21. The selected member hash is derived only after that pin verifies.
No installer, login, prompt, MCP configuration, or tool call is executed.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import stat
import time

import project_native_research as shared
from project_native_runtime import ordinary_bytes, strict_json, write_new


SOURCE = Path(__file__).resolve().parent.parent
ARCHIVE = {
    "url": "https://prod.download.cli.kiro.dev/stable/3.0.0/kirocli-x86_64-linux.zip",
    "integrity": "sha256:83dd96e00a75937958096da0436703562bc8c841bef66bde00a4caa5494d8be8",
    "expected_bytes": 137418453, "max_bytes": 137418453,
    "max_expanded_bytes": 512 << 20, "format": "zip",
    "entrypoint": "kirocli/bin/kiro-cli", "runtime": "native",
}
UNIT = re.compile(r"uap-kiro-acp-boundary-[0-9a-f]{32}\.service\Z")
SERVICE_ENV = frozenset("PATH LANG GITHUB_ACTIONS RUNNER_ENVIRONMENT AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED UAP_KIRO_BOUNDARY_CAPTURE UAP_KIRO_BOUNDARY_IDENTITY UAP_KIRO_BOUNDARY_RUNTIME".split())
QUALIFICATION = {name: False for name in shared.QUALIFICATION_CASES}
MARKER = b"UAP_KIRO_ACP_BOUNDARY="


def source_identity(ctx, label):
    values = {}
    for key, args in (("commit", ["rev-parse", "HEAD"]), ("tree", ["rev-parse", "HEAD^{tree}"]),
                      ("status", ["status", "--porcelain", "--untracked-files=normal"])):
        row, stdout, stderr = ctx.command(label + "-" + key, ["/usr/bin/git", *args], SOURCE, ctx.env, 10)
        if type(row["exit"]) is not int or row["exit"] != 0 or stderr:
            raise ValueError("source query failed")
        values[key] = stdout.decode("ascii", "strict").strip()
    if (values["status"] or values["commit"] != os.environ.get("EXPECTED_COMMIT")
            or any(not re.fullmatch(r"[0-9a-f]{40}", values[k]) for k in ("commit", "tree"))):
        raise ValueError("exact clean current source required")
    return {"commit": values["commit"], "tree": values["tree"]}


def archive_identity(receipt):
    if (receipt.get("url") != ARCHIVE["url"] or receipt.get("integrity") != ARCHIVE["integrity"]
            or receipt.get("pin_verified") is not True or type(receipt.get("bytes")) is not int
            or receipt["bytes"] != ARCHIVE["expected_bytes"]
            or receipt.get("sha256") != ARCHIVE["integrity"].removeprefix("sha256:")
            or receipt.get("installer_executed") is not False):
        raise ValueError("exact official archive pin required")
    members = receipt.get("members")
    if not isinstance(members, list) or not 1 <= len(members) <= 8192:
        raise ValueError("bounded member inventory required")
    selected = [r for r in members if r.get("name") == ARCHIVE["entrypoint"]]
    if (len(selected) != 1 or selected[0].get("kind") != "file"
            or type(selected[0].get("bytes")) is not int or not 0 < selected[0]["bytes"] <= 512 << 20
            or not re.fullmatch(r"[0-9a-f]{64}", selected[0].get("sha256", ""))
            or any(r.get("kind") not in ("file", "directory") for r in members)):
        raise ValueError("exact ordinary executable member required")
    return selected[0]


def service_argv(tests, env, root, unit, uid, gid, origin):
    root, tests = Path(root), Path(tests)
    if (not UNIT.fullmatch(unit) or type(uid) is not int or uid <= 0 or type(gid) is not int or gid <= 0
            or type(origin) is not int or origin <= 0 or set(env) != SERVICE_ENV
            or env["PATH"] != "/usr/bin:/bin" or env["LANG"] != "C.UTF-8"
            or env["GITHUB_ACTIONS"] != "true" or env["RUNNER_ENVIRONMENT"] != "github-hosted"
            or env["AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED"] != "1" or env["UAP_KIRO_BOUNDARY_CAPTURE"] != "1"
            or any(not isinstance(v, str) or len(v) > 4096 or any(c in v for c in "\x00\r\n") for v in env.values())
            or not root.is_absolute() or root.resolve() != root or root == SOURCE or SOURCE in root.parents
            or any(not re.fullmatch(r"/[A-Za-z0-9_./-]+", str(p)) for p in (root, SOURCE))
            or tests != root / "inputs/boundary.test" or tests.resolve() != tests
            or env["UAP_KIRO_BOUNDARY_IDENTITY"] != str(root / "inputs/identity.json")
            or env["UAP_KIRO_BOUNDARY_RUNTIME"] != str(root / "runtime")):
        raise ValueError("closed Kiro delegated service arguments required")
    selected = {**env, "UAP_KIRO_BOUNDARY_ORIGIN_NS": str(origin), "UAP_KIRO_BOUNDARY_UNIT": unit,
                "UAP_KIRO_BOUNDARY_UID": str(uid), "UAP_KIRO_BOUNDARY_GID": str(gid)}
    return ["/usr/bin/sudo", "-n", "/usr/bin/systemd-run", "--unit=" + unit, "--wait", "--pipe", "--collect", "--quiet",
            "--service-type=exec", "--expand-environment=no", "--uid=" + str(uid), "--gid=" + str(gid),
            "--property=Delegate=yes", "--property=NoNewPrivileges=yes", "--property=KillMode=control-group",
            "--property=RuntimeMaxSec=180s", "--property=TimeoutStopSec=10s",
            "--property=ReadOnlyPaths=" + str(root) + " " + str(SOURCE),
            "--property=ReadWritePaths=" + str(root / "runtime"), "--working-directory=" + str(root / "runtime"),
            "/usr/bin/env", "-i", *(k + "=" + selected[k] for k in sorted(selected)), str(tests),
            "-test.run=^TestAgentpluginsKiroACPBoundaryCapture$", "-test.count=1", "-test.v", "-test.timeout=150s"]


def unit_state(body, unit):
    if not UNIT.fullmatch(unit):
        raise ValueError("unregistered Kiro unit")
    value = {}
    for row in body.decode("ascii", "strict").splitlines():
        key, separator, item = row.partition("=")
        if not separator or key in value or key not in ("LoadState", "ActiveState", "ControlGroup"):
            raise ValueError("exact unit state required")
        value[key] = item
    if (set(value) != {"LoadState", "ActiveState", "ControlGroup"}
            or value["LoadState"] not in ("loaded", "not-found")
            or value["ActiveState"] not in ("active", "inactive", "failed", "activating", "deactivating")
            or value["ControlGroup"] not in ("", "/system.slice/" + unit)
            or value["LoadState"] == "not-found" and (value["ActiveState"] != "inactive" or value["ControlGroup"])):
        raise ValueError("unit state or cgroup differs")
    return value


def cgroup_empty(unit, uid):
    if not UNIT.fullmatch(unit):
        raise ValueError("unregistered Kiro cgroup")
    parent = Path("/sys/fs/cgroup/system.slice")
    if parent.resolve() != parent:
        raise ValueError("cgroup parent alias")
    parent_fd = os.open(parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        try:
            directory = os.open(unit, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent_fd)
        except FileNotFoundError:
            return True
        try:
            if os.fstat(directory).st_uid != uid:
                raise ValueError("owned delegated Kiro cgroup required")
            descriptor = os.open("cgroup.events", os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory)
            try:
                if not stat.S_ISREG(os.fstat(descriptor).st_mode):
                    raise ValueError("ordinary cgroup events required")
                body = os.read(descriptor, 4097)
            finally:
                os.close(descriptor)
        except FileNotFoundError:
            try:
                os.stat(unit, dir_fd=parent_fd, follow_symlinks=False)
            except FileNotFoundError:
                return True
            raise
        finally:
            os.close(directory)
    finally:
        os.close(parent_fd)
    if not re.fullmatch(rb"populated [01]\nfrozen [01]\n", body):
        raise ValueError("unexpected cgroup event state")
    return body.startswith(b"populated 0\n")


def service(ctx, tests, env):
    shared.native.require_hosted("linux-amd64")
    uid, gid = os.getuid(), os.getgid()
    if uid <= 0 or gid <= 0 or os.geteuid() != uid or os.getegid() != gid:
        raise ValueError("original ordinary hosted UID/GID required")
    for path in (Path("/run/systemd/system"), Path("/sys/fs/cgroup"), Path("/sys/fs/cgroup/system.slice")):
        info = path.lstat()
        if path.resolve() != path or not stat.S_ISDIR(info.st_mode) or info.st_uid != 0:
            raise ValueError("actual system manager and cgroup hierarchy required")
    system = {}
    for name in ("sudo", "systemd-run", "systemctl", "env"):
        path = Path("/usr/bin") / name
        info = path.lstat()
        body = ordinary_bytes(path, 32 << 20, owners=(0,))
        if not info.st_mode & 0o111 or info.st_mode & 0o022:
            raise ValueError("immutable root-owned system executable required")
        system[path] = (shared.artifacts.identity(info), hashlib.sha256(body).hexdigest())
    held = {p: shared.artifacts.identity(p.lstat()) for p in ctx.identities}

    def unchanged():
        for path, (identity, digest) in system.items():
            if (shared.artifacts.identity(path.lstat()) != identity
                    or hashlib.sha256(ordinary_bytes(path, 32 << 20, owners=(0,))).hexdigest() != digest):
                raise ValueError("system executable changed")
        shared.verify_files(ctx.identities, ctx.budget)
        if any(shared.artifacts.identity(p.lstat()) != identity for p, identity in held.items()):
            raise ValueError("immutable input identity changed")

    system_env = {"PATH": "/usr/bin:/bin", "LANG": "C", "SYSTEMD_PAGER": "cat", "SYSTEMD_COLORS": "0"}
    unit = "uap-kiro-acp-boundary-" + secrets.token_hex(16) + ".service"

    def query(label):
        unchanged()
        row, stdout, stderr = ctx.command(label, ["/usr/bin/systemctl", "--no-pager", "--no-ask-password", "show",
                    "--property=LoadState,ActiveState,ControlGroup", unit], ctx.root, system_env, 2)
        unchanged()
        if type(row["exit"]) is not int or row["exit"] != 0 or stderr:
            raise ValueError("owned unit query failed")
        return unit_state(stdout, unit)

    before = query("kiro-unit-before")
    if before["LoadState"] != "not-found" or not cgroup_empty(unit, uid):
        raise ValueError("fresh owned unit required")
    row, stdout, stderr = ctx.command("systemd-version", ["/usr/bin/systemd-run", "--version"], ctx.root, system_env, 5)
    if row["exit"] != 0 or stderr or not re.match(rb"systemd 255(?: \([^\r\n]*\))?\n", stdout):
        raise ValueError("reviewed systemd 255 host required")
    origin = time.monotonic_ns()
    receipt = {"unit": unit, "uid": uid, "gid": gid, "original_start_monotonic_ns": origin,
               "forced_stop": False, "forced_kill": False, "closed": False, "before": before,
               "system_executables": {str(p): digest for p, (_, digest) in system.items()}}
    ctx.summary["delegated_service"] = receipt
    argv = service_argv(tests, env, ctx.root, unit, uid, gid, origin)
    result, problem = None, None
    previous = ctx.budget
    with shared.Budget(min(previous.check(), 180)) as budget:
        ctx.budget = budget
        try:
            unchanged()
            result = ctx.command("kiro-boundary-service", argv, ctx.root, system_env,
                                 165 - (time.monotonic_ns() - origin) / 1e9)
        except Exception as error:
            problem = error
        finally:
            try:
                state = query("kiro-unit-after")
                receipt["closed"] = state["ActiveState"] in ("inactive", "failed") and cgroup_empty(unit, uid)
                receipt["after"] = state
            except Exception as error:
                problem = problem or error
            if not receipt["closed"]:
                receipt["forced_stop"] = True
                try:
                    unchanged()
                    ctx.command("kiro-unit-stop", ["/usr/bin/sudo", "-n", "/usr/bin/systemctl", "--no-ask-password",
                                "--no-block", "stop", unit], ctx.root, system_env, 1)
                    if not cgroup_empty(unit, uid):
                        receipt["forced_kill"] = True
                        ctx.command("kiro-unit-kill", ["/usr/bin/sudo", "-n", "/usr/bin/systemctl", "--no-ask-password",
                                    "kill", "--kill-whom=all", "--signal=SIGKILL", unit], ctx.root, system_env, 1)
                    state = query("kiro-unit-final")
                    receipt["closed"] = state["ActiveState"] in ("inactive", "failed") and cgroup_empty(unit, uid)
                    receipt["final"] = state
                except Exception as error:
                    problem = problem or error
            unchanged()
    ctx.budget = previous
    receipt["elapsed_ns"] = time.monotonic_ns() - origin
    if (not receipt["closed"] or receipt["forced_stop"] or receipt["forced_kill"]
            or receipt["elapsed_ns"] >= 180_000_000_000):
        raise ValueError("forced cleanup, retained descendants, or original deadline exhausted")
    if problem is not None:
        raise problem
    return result


def admit_report(stdout, identity, service_receipt):
    if (service_receipt.get("closed") is not True or service_receipt.get("forced_stop") is not False
            or service_receipt.get("forced_kill") is not False or type(service_receipt.get("elapsed_ns")) is not int
            or not 0 <= service_receipt["elapsed_ns"] < 180_000_000_000):
        raise ValueError("original deadline and unforced owned service closure required")
    lines = [line[len(MARKER):] for line in stdout.splitlines() if line.startswith(MARKER)]
    if len(lines) != 1:
        raise ValueError("single bounded helper receipt required")
    report = strict_json(lines[0])
    if (not isinstance(report, dict) or report.get("diagnostic_only") is not True
            or report.get("qualification") != QUALIFICATION
            or any(type(v) is not bool for v in report["qualification"].values())
            or report.get("identity") != identity or report.get("stderr_scope") != "unavailable-from-duplex-api"
            or report.get("stdout_scope") != "read-prefix-through-exchange-stop"
            or report.get("stdout_complete") is not False
            or report.get("delegation", {}).get("unit") != service_receipt["unit"]
            or type(report.get("delegation", {}).get("origin_ns")) is not int
            or report.get("delegation", {}).get("origin_ns") != service_receipt["original_start_monotonic_ns"]):
        raise ValueError("helper source, diagnostic or delegated identity differs")
    delegation = report["delegation"]
    if (any(type(delegation.get(k)) is not int or delegation[k] != service_receipt[k] for k in ("uid", "gid"))
            or delegation.get("no_new_privileges") is not True
            or delegation.get("membership") != "0::/system.slice/" + service_receipt["unit"] + "\n"):
        raise ValueError("actual ordinary delegated membership differs")
    for field, limit in (("stdin", 4096), ("stdout", 128 << 10)):
        body = base64.b64decode(report.get(field, ""), validate=True)
        if len(body) > limit or report.get(field + "_sha256") != hashlib.sha256(body).hexdigest():
            raise ValueError("bounded observed stream identity differs")
    if type(report.get("capture_complete")) is not bool:
        raise ValueError("exact diagnostic result required")
    if report["capture_complete"] and (report.get("duplex_capability") is not True
            or report.get("planned_shutdown_verified") is not True or "runner_error" in report or "deadline_error" in report
            or type(report.get("observed_records")) is not int or not 1 <= report["observed_records"] <= 128
            or report.get("argv") != [identity["binary"], "acp", "--agent-engine", "v3", "--auth-method", "cli"]
            or report.get("stop_reason") not in ("initialize-server-error", "initialize-server-request-unanswered",
                "initialize-result-unrecognized", "session-new-server-error", "session-new-server-request-unanswered", "session-new-result-observed")):
        raise ValueError("complete diagnostic requires exact production argv and planned shutdown")
    return report


def prepare(ctx):
    source = source_identity(ctx, "source-before")
    ctx.summary.update({"source_commit": source["commit"], "source_tree": source["tree"]})
    inputs, runtime = ctx.root / "inputs", ctx.root / "runtime"
    inputs.mkdir(mode=0o700)
    runtime.mkdir(mode=0o700)
    receipt, contents = shared.acquire(ARCHIVE, ctx.budget)
    selected = archive_identity(receipt)
    binary, identities = shared.materialize(inputs / "vendor", ARCHIVE, receipt, contents)
    ctx.identities.update(identities)
    del contents
    ctx.summary["acquisition"] = receipt
    go = Path(shutil.which("go") or "").resolve(strict=True)
    go_hash = hashlib.sha256(ordinary_bytes(go, 64 << 20, owners=(0, os.getuid()))).hexdigest()
    go_stat = shared.artifacts.identity(go.lstat())
    env = {**ctx.env, "PATH": str(go.parent) + ":/usr/bin:/bin", "GOTOOLCHAIN": "local", "GOENV": "off",
           "GOMAXPROCS": "2", "CGO_ENABLED": "0", "GOCACHE": str(ctx.root / "go-cache"),
           "GOMODCACHE": str(ctx.root / "go-modules"), "GOPROXY": "https://proxy.golang.org", "GOSUMDB": "sum.golang.org"}
    row, stdout, stderr = ctx.command("go-version", [go, "version"], SOURCE, env, 10)
    if row["exit"] != 0 or stdout != b"go version go1.26.11 linux/amd64\n" or stderr:
        raise ValueError("exact Go 1.26.11 Linux amd64 compiler required")
    tests = inputs / "boundary.test"
    row, _, _ = ctx.command("build-boundary-helper", [go, "test", "-p=1", "-c", "-trimpath", "-o", tests, "./repotests"], SOURCE, env, 240)
    if (row["exit"] != 0 or shared.artifacts.identity(go.lstat()) != go_stat
            or hashlib.sha256(ordinary_bytes(go, 64 << 20, owners=(0, os.getuid()))).hexdigest() != go_hash):
        raise ValueError("exact helper build or compiler custody failed")
    tests.chmod(0o700)
    ctx.identities[tests] = hashlib.sha256(ordinary_bytes(tests, 256 << 20)).hexdigest()
    identity = {"source": source, "archive_sha256": receipt["sha256"], "archive_bytes": receipt["bytes"],
                "archive_member": ARCHIVE["entrypoint"], "binary": str(binary), "binary_sha256": selected["sha256"],
                "helper": str(tests), "helper_sha256": ctx.identities[tests],
                "runtime_files": {str(p): digest for p, digest in identities.items()}}
    identity_path = inputs / "identity.json"
    body = json.dumps(identity, sort_keys=True).encode() + b"\n"
    if len(body) > 512 << 10:
        raise ValueError("bounded archive-derived identity required")
    write_new(identity_path, body)
    ctx.identities[identity_path] = hashlib.sha256(body).hexdigest()
    ctx.summary["compiler"] = {"version": "go1.26.11", "sha256": go_hash}
    ctx.summary["identity"] = identity
    shared.verify_files(ctx.identities, ctx.budget)
    env = {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8", "GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted",
           "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1", "UAP_KIRO_BOUNDARY_CAPTURE": "1",
           "UAP_KIRO_BOUNDARY_IDENTITY": str(identity_path), "UAP_KIRO_BOUNDARY_RUNTIME": str(runtime)}
    return tests, env, identity


def run(output):
    shared.native.require_hosted("linux-amd64")
    output = output.absolute()
    if output.resolve() != output or output == SOURCE or SOURCE in output.parents:
        raise ValueError("fresh canonical output outside source required")
    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    summary = {"client": "kiro", "version": "3.0.0", "phase": "acp-boundary", "diagnostic_only": True,
               "capture_review_required": True, "collection_complete": False, "qualification": dict(QUALIFICATION), "commands": [],
               "pin_source": {"url": "https://prod.download.cli.kiro.dev/stable/index.json", "observed_date": "2026-11-21",
                   "metadata_sha256": "763c3f5794e64a968f14f32358dedf57fb3baef0d150614eca7fad53ce3f29c6",
                   "version": "3.0.0", "target_triple": "x86_64-unknown-linux-gnu", "format": "zip"}}
    ctx = None
    try:
        with shared.Budget(900) as budget:
            ctx = shared.Context(output, summary, budget)
            tests, env, identity = prepare(ctx)
            row, stdout, _ = service(ctx, tests, env)
            report = admit_report(stdout, identity, summary["delegated_service"])
            summary["boundary"] = report
            if source_identity(ctx, "source-after") != identity["source"]:
                raise ValueError("source changed during capture")
            summary["profile_after"] = shared.legacy.inventory({"runtime": output / "runtime"}, budget)
            if type(row["exit"]) is not int or row["exit"] != 0 or report["capture_complete"] is not True:
                raise ValueError("diagnostic transport or capture incomplete")
            summary["collection_complete"] = True
    except Exception as error:
        summary["failure_kind"] = type(error).__name__
    finally:
        shared.publish(summary, ctx, output)
    print(json.dumps({k: summary[k] for k in ("diagnostic_only", "collection_complete", "qualification")}))
    return 0 if summary["collection_complete"] else 1


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    raise SystemExit(run(parser.parse_args().output))
