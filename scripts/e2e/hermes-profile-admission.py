#!/usr/bin/env python3
"""Observe pinned Hermes explicit custom-runtime-root MCP; UAP remains refused."""
import hashlib
import json
import os
from pathlib import Path
import runpy
import secrets
import shutil
import stat
import subprocess
import sys
import tempfile
import time

HERE = Path(__file__).resolve().parent
BASE = runpy.run_path(str(HERE / "hermes-noauth-tool-call.py"), run_name="hermes_admission_helpers")
require, bounded, sha = (BASE[name] for name in ("require", "bounded", "sha"))
import native_client_project_artifacts as artifacts

LIMIT = 1 << 20
EVIDENCE_CLOSED = False
STABLE_FIELDS = ("st_dev", "st_ino", "st_mode", "st_nlink", "st_uid", "st_gid",
                 "st_size", "st_mtime_ns", "st_ctime_ns")


def stable_file(info):
    # Access time may change merely because this verifier read the file.
    return artifacts.identity(info) + (info.st_uid, info.st_gid)


def same_opened_capture(before, held):
    """Join Windows path/fd by stable file ID, not synthetic cross-API mode.

    A path stat and an opened Windows handle may synthesize permission/execute
    bits differently. The complete mode is still compared within each API
    domain before and after capture; only cross-domain mode equality is omitted.
    """
    if not artifacts.WINDOWS:
        return artifacts.same_opened(before, held)
    require(stat.S_ISREG(before.st_mode) and stat.S_ISREG(held.st_mode),
            "opened Windows member is not an ordinary file")
    from_path, from_handle = artifacts.windows_identity(before), artifacts.windows_identity(held)
    return from_path[:2] == from_handle[:2] and from_path[3:] == from_handle[3:]


def ordinary_bytes(path, expected=None):
    path = Path(path)
    require(path.is_absolute() and path.resolve() == path, "ordinary file alias")
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1 and 0 <= before.st_size <= LIMIT,
            "bounded ordinary single-link file required")
    require(expected is None or stable_file(before) == stable_file(expected), "file changed before open")
    # Reuse native artifact admission's Windows OPEN_REPARSE_POINT handle and
    # positive cross-API identity. Path/fd ctime domains remain separately held.
    descriptor = artifacts.open_member(path)
    try:
        held = os.fstat(descriptor)
        require(same_opened_capture(before, held)
                and (held.st_uid, held.st_gid) == (before.st_uid, before.st_gid),
                "file changed at open: " + ",".join(
                    field for field in STABLE_FIELDS
                    if not (artifacts.WINDOWS and field == "st_ctime_ns")
                    and getattr(held, field) != getattr(before, field)))
        with os.fdopen(os.dup(descriptor), "rb") as stream:
            body = stream.read(LIMIT + 1)
        require(len(body) == before.st_size <= LIMIT and stable_file(os.fstat(descriptor)) == stable_file(held)
                and stable_file(path.lstat()) == stable_file(before), "file changed during bounded capture")
        return body
    finally:
        os.close(descriptor)


def directory_identity(path):
    require(path.is_absolute() and path.resolve() == path and not path.is_symlink(), "directory alias")
    info = path.lstat()
    require(stat.S_ISDIR(info.st_mode), "ordinary directory required")
    if artifacts.WINDOWS:
        artifacts.windows_identity(info)
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)


def verify_root_identities(roots, held):
    for name, root in roots.items():
        require(directory_identity(root) == tuple(held[name]), "original custom root directory changed")


def inventory(root, sentinels=()):
    """Capture each ordinary file once through its independently checked handle."""
    root = Path(root)
    identity = directory_identity(root)
    result, total = {}, 0
    def walk_error(error):
        raise error
    for current, dirs, names in os.walk(root, followlinks=False, onerror=walk_error):
        for path in [Path(current), *(Path(current) / name for name in dirs + names)]:
            relative = path.relative_to(root).as_posix()
            if relative in result:
                continue
            info = path.lstat()
            require(len(result) < 2048 and len(path.relative_to(root).parts) <= 24, "inventory membership bound")
            require(stat.S_ISREG(info.st_mode) or stat.S_ISDIR(info.st_mode), "inventory special member refused")
            record = {"type": "directory" if stat.S_ISDIR(info.st_mode) else "file", "mode": stat.S_IMODE(info.st_mode),
                      "device": info.st_dev, "inode": info.st_ino}
            if record["type"] == "file":
                body = ordinary_bytes(path, info)
                total += len(body)
                require(total <= 16 << 20, "inventory byte bound")
                require(not any(marker.encode() in body for marker in sentinels), "original User settings/data copied into custom root")
                record.update(bytes=len(body), sha256=hashlib.sha256(body).hexdigest())
            result[relative] = record
    require(directory_identity(root) == identity, "inventory root replaced")
    return result


def changes(before, after):
    return {name: {"before": before.get(name), "after": after.get(name)}
            for name in sorted(before.keys() | after.keys()) if before.get(name) != after.get(name)}


def protected_unchanged(before, after, paths):
    for name in paths:
        require(before.get(name) == after.get(name), "protected original User changed: " + name)


def validate_guards(value):
    require(set(value) == {"gateway_running", "default_gateway_pid", "served_profiles", "service_manager", "memory_provider"},
            "source guard fields differ")
    require(value["gateway_running"] is False and value["default_gateway_pid"] is None
            and value["served_profiles"] is None and value["service_manager"] != "s6"
            and value["memory_provider"] is None, "live shared gateway/service/memory authority refused")


def validate_roots(user_home, native_home, roots):
    user_home, native_home = Path(user_home), Path(native_home)
    for root in roots:
        root = Path(root)
        require(root.is_absolute() and root.resolve() == root and not root.is_symlink()
                and root.parent.name != "profiles" and not root.is_relative_to(native_home)
                and not native_home.is_relative_to(root) and root != user_home,
                "custom root is not independent original-User authority")
    require(len(set(map(str, roots))) == 2 and not roots[0].is_relative_to(roots[1])
            and not roots[1].is_relative_to(roots[0]), "A/B runtime roots overlap")


GUARD_CODE = r"""
import json, os
from pathlib import Path
p = Path(os.environ['HERMES_HOME'])
if any((p / n).exists() for n in ('active_profile', 'gateway.pid', 'gateway.lock', 'gateway_state.json')):
    raise RuntimeError('existing profile redirect/gateway signal')
import hermes_cli.main
from hermes_constants import get_default_hermes_root, get_hermes_home
from hermes_cli.auth import _global_auth_file_path
from hermes_cli.profiles import _check_gateway_running
from hermes_cli.gateway_multiplex_served import live_default_gateway_pid, recorded_served_profiles
from hermes_cli.service_manager import detect_service_manager
from hermes_cli.profile_memory_config import active_memory_provider
from hermes_cli.config import load_config_readonly
value = {'home': str(get_hermes_home().resolve()), 'default_root': str(get_default_hermes_root().resolve()),
         'native_root': str(get_default_hermes_root(home='').resolve()),
         'auth_fallback': str(_global_auth_file_path()) if _global_auth_file_path() is not None else None,
         'os_home': str(Path.home().resolve()), 'userprofile': os.environ['USERPROFILE'],
         'gateway_running': _check_gateway_running(p), 'default_gateway_pid': live_default_gateway_pid(),
         'served_profiles': recorded_served_profiles(), 'service_manager': detect_service_manager(),
         'memory_provider': active_memory_provider(load_config_readonly())}
print('UAP_PROFILE_RESOLUTION=' + json.dumps(value, sort_keys=True))
"""


def no_user_copies(root, sentinels):
    return inventory(root, sentinels)


def config_document_boundary(body, value, server, url, removed=False, authored_sha=None):
    """Check the exact mapping and bytes independently of the native YAML parser."""
    require(isinstance(value, dict) and set(value) <= {"mcp_servers"}, "unadmitted custom config fields")
    expected = {} if removed else {server: {"url": url}}
    require(value.get("mcp_servers", {}) == expected, "selected native MCP mapping differs")
    digest = hashlib.sha256(body).hexdigest()
    if authored_sha is not None:
        require(digest == authored_sha, "authored custom config bytes changed")
    return digest


def config_boundary(root, server, url, removed=False, authored_sha=None):
    import hermes_yaml as yaml
    body = ordinary_bytes(root / "config.yaml")
    return config_document_boundary(body, yaml.safe_load(body), server, url, removed, authored_sha)


def protocol_join(before, after, selected, expected):
    delta = {}
    for name in before:
        require(after[name][:len(before[name])] == before[name], "MCP method ledger rewritten")
        delta[name] = after[name][len(before[name]):]
    require(all(not methods for name, methods in delta.items() if name != selected), "sibling/default probe contacted")
    methods = delta.get(selected, [])
    require(methods.count("initialize") == expected and methods.count("tools/list") == expected
            and all(method in {"initialize", "notifications/initialized", "tools/list"} for method in methods)
            and (not expected or methods.index("initialize") < methods.index("tools/list")),
            "selected native initialize/list differs")
    return delta


def write_json(path, value):
    body = json.dumps(value, indent=2).encode() + b"\n"
    require(len(body) <= LIMIT and not path.exists(), "fresh bounded evidence required")
    path.write_bytes(body)


def observe_profiles(probes):
    evidence = Path(os.environ["HERMES_NATIVE_EVIDENCE"])
    user = Path(os.environ["HOME"])
    outer, nonce = user.parent, os.environ["HERMES_CALL_NONCE"]
    native = json.loads(ordinary_bytes(Path(os.environ["HERMES_NATIVE_RUNTIME_COMMAND_FILE"])).decode("utf-8-sig"))
    from hermes_constants import get_default_hermes_root
    import hermes_yaml as yaml
    original = get_default_hermes_root(home="").resolve()
    require(original.is_relative_to(user) and original != user and os.environ["USERPROFILE"] == str(user), "actual native User root differs")
    original.mkdir(parents=True, exist_ok=True)
    sentinels = ["USER_" + kind + "_" + nonce for kind in ("MODEL", "PROVIDER", "AUTH", "HISTORY", "FOREIGN")]
    authored = {"config.yaml": "model: " + sentinels[0] + "\nproviders:\n  custom:\n    base_url: http://127.0.0.1:9/" + sentinels[1] + "\nmcp_servers:\n  foreign:\n    command: echo\n    args: [retained]\n",
                "auth.json": json.dumps({"version": 1, "providers": {}, "admission_marker": sentinels[2]}),
                ".env": "# inert admission; no credentials\n",
                "sessions/retained-history.txt": sentinels[3],
                "skills/foreign/SKILL.md": "---\nname: foreign\ndescription: Retained foreign User Skill.\n---\n" + sentinels[4]}
    for name, body in authored.items():
        path = original / name
        require(not path.exists(), "original User fixture collision")
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(body, encoding="utf-8")
        path.chmod(0o600)
    spec, records, shared = {}, [], []
    for scope in ("a", "b"):
        project = outer / ("project-" + scope)
        project.mkdir()
        (project / "foreign.txt").write_text(nonce + scope)
        bounded(["git", "init", "-q", str(project)], outer, dict(os.environ), evidence / ("git-" + scope), timeout=30)
        root = project / ".uap" / "hermes-runtime"
        root.mkdir(parents=True)
        entry = {"url": probes[scope].url}
        config = root / "config.yaml"
        config.write_text(yaml.safe_dump({"mcp_servers": {"uap-root-" + scope: entry}}), encoding="utf-8")
        config.chmod(0o600)
        spec[scope] = {"root": str(root), "project": str(project), "server": "uap-root-" + scope,
                       "url": probes[scope].url, "config_sha256": sha(config), "entry": entry}
    roots = [Path(spec[scope]["root"]) for scope in ("a", "b")]
    validate_roots(user, original, roots)
    root_paths = {scope: Path(spec[scope]["root"]) for scope in ("a", "b")}
    held_roots = {scope: directory_identity(root) for scope, root in root_paths.items()}
    removed = set()
    for scope, root in root_paths.items():
        spec[scope]["directory_identity"] = held_roots[scope]
        config_boundary(root, spec[scope]["server"], spec[scope]["url"], authored_sha=spec[scope]["config_sha256"])
    spec["user"] = {"root": str(original), "project": str(user)}
    baseline = inventory(outer)
    protected = [str((original / name).relative_to(outer)).replace('\\', '/') for name in authored]
    controls = [str((outer / ("project-" + scope) / "foreign.txt").relative_to(outer)).replace('\\', '/') for scope in ("a", "b")]
    protected += controls
    write_json(evidence / "original-user-baseline.json", baseline)
    write_json(evidence / "profile-endpoints.json", {"selected": spec, "original_native_root": str(original),
               "os_home": str(user), "sentinel_sha256": [hashlib.sha256(x.encode()).hexdigest() for x in sentinels]})
    current = baseline

    def command(label, scope, args=None, guard=False, expected_failure=False):
        nonlocal current
        verify_root_identities(root_paths, held_roots)
        for name, root in root_paths.items():
            config_boundary(root, spec[name]["server"], spec[name]["url"], name in removed,
                            None if name in removed else spec[name]["config_sha256"])
        before = inventory(outer)
        require(before == current, "unmeasured inter-command mutation")
        selected = spec[scope]
        env = {**os.environ, "HERMES_HOME": selected["root"]}
        require(env["HOME"] == str(user) and env["USERPROFILE"] == str(user), "OS User home changed")
        if guard:
            from hermes_cli._launchers import runtime_command
            argv = runtime_command(Path(os.environ["RUNNER_TEMP"]) / "hermes-agent", python=native[0], code=GUARD_CODE)
        else:
            argv = [*native, *args]
        methods_before = {name: list(probe.methods()) for name, probe in probes.items()}
        failure, output = None, b""
        try:
            output = bounded(argv, Path(selected["project"]), env, evidence / label, timeout=60, expected_failure=expected_failure)
        except BaseException as error:
            failure = error
        verify_root_identities(root_paths, held_roots)
        after = inventory(outer)
        delta = changes(before, after)
        records.append({"label": label, "scope": scope, "runtime_root": selected["root"],
                        "delta": delta, "error": type(failure).__name__ if failure else None})
        current = after
        if label == "remove-mcp-" + scope and failure is None:
            removed.add(scope)
        for name, root in root_paths.items():
            config_boundary(root, spec[name]["server"], spec[name]["url"], name in removed,
                            None if name in removed else spec[name]["config_sha256"])
        shared.extend({"label": label, "path": path, **change} for path, change in delta.items()
                      if not any(path == str(root.relative_to(outer)).replace('\\', '/')
                                 or path.startswith(str(root.relative_to(outer)).replace('\\', '/') + "/") for root in roots))
        protected_unchanged(baseline, after, protected)
        for other in (name for name in ("a", "b") if name != scope):
            prefix = str(Path(spec[other]["root"]).relative_to(outer)).replace('\\', '/')
            require({key: value for key, value in before.items() if key == prefix or key.startswith(prefix + "/")}
                    == {key: value for key, value in after.items() if key == prefix or key.startswith(prefix + "/")},
                    "sibling custom root changed")
        for root in roots:
            no_user_copies(root, sentinels)
        if failure:
            raise failure
        methods_after = {name: list(probe.methods()) for name, probe in probes.items()}
        tested = label in ("test-a", "test-b", "test-a-restart")
        records[-1]["native_methods"] = protocol_join(methods_before, methods_after, scope if tested else None, 1 if tested else 0)
        if guard:
            prefix = b"UAP_PROFILE_RESOLUTION="
            rows = [line[len(prefix):] for line in output.splitlines() if line.startswith(prefix)]
            require(len(rows) == 1, "native resolution receipt missing/ambiguous")
            receipt = json.loads(rows[0])
            validate_guards({key: receipt[key] for key in ("gateway_running", "default_gateway_pid", "served_profiles", "service_manager", "memory_provider")})
            require(receipt["home"] == selected["root"] == receipt["default_root"]
                    and receipt["native_root"] == str(original) and receipt["auth_fallback"] is None
                    and receipt["os_home"] == str(user) and receipt["userprofile"] == str(user), "native custom/User root authority differs")
            records[-1]["resolution"] = receipt
        return output

    try:
        for scope in ("a", "b"):
            command("guard-" + scope, scope, guard=True)
            output = command("list-" + scope, scope, ["mcp", "list"])
            require(spec[scope]["server"].encode() in output, "selected native MCP name absent")
            command("test-" + scope, scope, ["mcp", "test", spec[scope]["server"]])
        command("test-a-restart", "a", ["mcp", "test", spec["a"]["server"]])
        output = command("list-user", "user", ["mcp", "list"])
        require(b"foreign" in output and all(spec[x]["server"].encode() not in output for x in ("a", "b")), "original User MCP selection changed")
        for scope in ("a", "b"):
            command("remove-mcp-" + scope, scope, ["mcp", "remove", spec[scope]["server"]])
            output = command("removed-list-" + scope, scope, ["mcp", "list"])
            require(spec[scope]["server"].encode() not in output, "removed MCP still listed")
            command("removed-test-" + scope, scope, ["mcp", "test", spec[scope]["server"]], expected_failure=True)
        write_json(evidence / "original-user-final.json", inventory(outer))
    finally:
        write_json(evidence / "profile-command-deltas.json", {"commands": records, "unadmitted_shared_effects": shared,
                   "production_project_mcp": False, "admission_qualified": False,
                   "methods": {name: list(probe.methods()) for name, probe in probes.items()}})


COMMANDS = ({prefix + x for prefix in ("git-", "guard-", "list-", "test-", "remove-mcp-", "removed-list-", "removed-test-") for x in ("a", "b")}
            | {"test-a-restart", "list-user"})
MEMBERS = ({name + suffix for name in COMMANDS for suffix in (".stdout", ".stderr", ".status.json")}
           | {"original-user-baseline.json", "original-user-final.json", "source-guards.json", "profile-endpoints.json",
              "profile-command-deltas.json", "profile-admission.json", "lifecycle.stdout", "lifecycle.stderr", "owned-start-gate"})


def admit_evidence(directory):
    total = 0
    for path in directory.iterdir():
        info = path.lstat()
        require(path.name in MEMBERS and stat.S_ISREG(info.st_mode) and info.st_nlink == 1
                and info.st_size <= LIMIT, "closed admission evidence member refused")
        total += info.st_size
        require(total <= 16 << 20, "admission evidence aggregate bound")


def stage_evidence(directory):
    global EVIDENCE_CLOSED
    identity = directory_identity(directory)
    admit_evidence(directory)
    sources = {path.name: path.lstat() for path in directory.iterdir()}
    upload = directory / "upload"
    require(not upload.exists() and not upload.is_symlink(), "upload destination already exists")
    pending = directory / (".upload-pending-" + secrets.token_hex(16))
    pending.mkdir(mode=0o700)
    pending_identity = directory_identity(pending)
    try:
        for name, expected in sorted(sources.items()):
            body = ordinary_bytes(directory / name, expected)
            with (pending / name).open("xb") as stream:
                stream.write(body)
        require(directory_identity(directory) == identity
                and {path.name for path in directory.iterdir()} == set(sources) | {pending.name},
                "evidence directory/membership changed before publication")
        for name, expected in sources.items():
            require(stable_file((directory / name).lstat()) == stable_file(expected), "evidence changed before publication")
        pending.rename(upload)
        EVIDENCE_CLOSED = True
    except BaseException:
        # Only this fresh private staging directory is ours; no visible partial upload.
        require(directory_identity(pending) == pending_identity, "private evidence staging directory changed")
        shutil.rmtree(pending)
        raise


def safe_failure_evidence(runner, failure_kind="unclosed-native-admission"):
    directory = Path(tempfile.mkdtemp(prefix="hermes-profile-admission-evidence-", dir=runner))
    write_json(directory / "profile-admission.json", {"status": "failed", "error": "native admission aborted before evidence closure", "original_failure_kind": failure_kind,
               "admission_qualified": False, "production_project_mcp": False, "vendor_authentication": False,
               "real_model_inference": False, "tools_call": False})
    stage_evidence(directory)
    return directory


def child_case():
    from contextlib import ExitStack
    from pi_project_components_native_e2e import PiHTTPMCPProbe
    with ExitStack() as stack:
        probes = {name: stack.enter_context(PiHTTPMCPProbe(nonce=os.environ["HERMES_CALL_NONCE"]))
                  for name in ("a", "b")}
        observe_profiles(probes)


def run():
    require(os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted",
            "genuine GitHub-hosted admission required")
    runner, checkout = Path(os.environ["RUNNER_TEMP"]).resolve(), HERE.parents[1]
    before, official = BASE["source_identity"](checkout), BASE["source_identity"](runner / "hermes-agent")
    require(before["commit"] == os.environ["HERMES_NATIVE_UAP_HEAD"] and not before["dirty"], "exact clean UAP source required")
    require(official["commit"] == BASE["PIN"] and not official["dirty"], "unmodified pinned Hermes required")
    evidence = Path(tempfile.mkdtemp(prefix="hermes-profile-admission-evidence-", dir=runner))
    home = Path(tempfile.mkdtemp(prefix="hermes-profile-admission-home-", dir=runner))
    identity = (home.stat().st_dev, home.stat().st_ino, home.stat().st_mode)
    evidence_identity = (evidence.stat().st_dev, evidence.stat().st_ino, evidence.stat().st_mode)
    user = home / "user"
    user.mkdir(mode=0o700)
    nonce, child, job, failure = secrets.token_hex(32), None, None, None
    proof = {"source": before, "hermes_source": official, "nonce": nonce, "capture_complete": False,
             "admission_qualified": False, "production_project_mcp": False,
             "vendor_authentication": False, "real_model_inference": False, "tools_call": False}
    runtime_file = Path(os.environ["HERMES_NATIVE_RUNTIME_COMMAND_FILE"])
    native = json.loads(ordinary_bytes(runtime_file).decode("utf-8-sig"))
    held = {"runtime": sha(Path(native[0])), "runtime_command": sha(runtime_file), "launcher": sha(Path(os.environ["HERMES_BIN"]))}
    helpers = runpy.run_path(str(HERE / "hermes-native.py"), run_name="hermes_profile_process_helpers")
    try:
        env = BASE["closed_environment"](user, evidence, nonce, "http://127.0.0.1:9/unused")
        env.pop("HERMES_NATIVE_MCP_URL", None)  # This case has no User/legacy-call endpoint.
        gate = evidence / "owned-start-gate"
        env["HERMES_CALL_START_GATE"] = str(gate)
        sys.path.insert(0, str(runner / "hermes-agent"))
        from hermes_cli._launchers import runtime_command
        argv = runtime_command(runner / "hermes-agent", python=native[0], args=["--child"],
                               code=f"runpy.run_path({str(Path(__file__).resolve())!r}, run_name='__main__')")
        flags = {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP} if os.name == "nt" else {"start_new_session": True}
        with (evidence / "lifecycle.stdout").open("wb") as out, (evidence / "lifecycle.stderr").open("wb") as err:
            child = subprocess.Popen(argv, env=env, stdin=subprocess.DEVNULL, stdout=out, stderr=err, **flags)
            if os.name == "nt":
                job = BASE["WindowsJob"](child)
            gate.write_text("owned\n")
            deadline = time.monotonic() + 720
            while child.poll() is None:
                require(time.monotonic() < deadline, "admission deadline")
                require(all((evidence / name).stat().st_size <= LIMIT for name in ("lifecycle.stdout", "lifecycle.stderr")), "admission output bound")
                time.sleep(.1)
        require(child.returncode == 0, "native profile observation failed")
        deltas = json.loads(ordinary_bytes(evidence / "profile-command-deltas.json"))
        methods = deltas["methods"]
        require(methods["a"].count("initialize") == 2 and methods["a"].count("tools/list") == 2
                and methods["b"].count("initialize") == 1 and methods["b"].count("tools/list") == 1
                and all("tools/call" not in value for value in methods.values()), "native profile MCP ledger differs")
        endpoints = json.loads(ordinary_bytes(evidence / "profile-endpoints.json"))
        proof.update(capture_complete=True, methods=methods, runtime_roots=endpoints, deltas=deltas)
        require(not deltas["unadmitted_shared_effects"], "shared metadata effects require independent admission")
    except BaseException as error:
        failure = error
        proof.update(status="failed", error=type(error).__name__ + ": " + str(error)[:300])
    finally:
        process_settled = child is None
        try:
            if child is not None:
                if job is not None:
                    job.close()
                    child.wait(timeout=5)
                else:
                    require(os.name != "nt", "Windows child lacks owned job accounting")
                    helpers["stop_lifecycle"](child)
                    deadline = time.monotonic() + 5
                    while True:
                        try:
                            os.killpg(child.pid, 0)
                        except ProcessLookupError:
                            break
                        require(time.monotonic() < deadline, "owned process group still present")
                        time.sleep(.05)
                process_settled = True
            endpoint_file = evidence / "profile-endpoints.json"
            root_paths = {name: home / ("project-" + name) / ".uap" / "hermes-runtime" for name in ("a", "b")}
            if endpoint_file.exists():
                selected = json.loads(ordinary_bytes(endpoint_file))["selected"]
                require(all(selected[name]["root"] == str(root) for name, root in root_paths.items()), "cleanup root receipt differs")
                verify_root_identities(root_paths, {name: selected[name]["directory_identity"] for name in root_paths})
            else:
                require(all(not root.exists() and not root.is_symlink() for root in root_paths.values()), "cleanup lacks original custom-root receipt")
            require(home.resolve() == home and not home.is_symlink()
                    and (home.stat().st_dev, home.stat().st_ino, home.stat().st_mode) == identity, "owned outer root changed")
            shutil.rmtree(home)
            proof["cleanup"] = "completed"
        except BaseException as error:
            failure = failure or error
            proof.update(status="failed", cleanup="failed", cleanup_error=type(error).__name__)
        if not process_settled:
            safe_failure_evidence(runner, type(failure).__name__ if failure else "process-cleanup-unproved")
            raise RuntimeError("native process cleanup unproved; raw evidence withheld") from failure
        require(evidence.resolve() == evidence and not evidence.is_symlink()
                and (evidence.stat().st_dev, evidence.stat().st_ino, evidence.stat().st_mode) == evidence_identity,
                "evidence directory identity changed")
        admit_evidence(evidence)
        proof.update(source_after=BASE["source_identity"](checkout), hermes_source_after=BASE["source_identity"](runner / "hermes-agent"),
                     streams={p.name: {"bytes": p.stat().st_size, "sha256": hashlib.sha256(ordinary_bytes(p)).hexdigest()} for p in evidence.iterdir() if p.is_file()})
        final = {"runtime": sha(Path(native[0])), "runtime_command": sha(runtime_file), "launcher": sha(Path(os.environ["HERMES_BIN"]))}
        if proof["source_after"] != before or proof["hermes_source_after"] != official or final != held:
            failure = failure or RuntimeError("source/runtime identity changed")
            proof.update(status="failed", source_error="source/runtime identity changed")
        proof.update(runtime_before=held, runtime_after=final, lifecycle_exit=child.returncode if child else None,
                     run_id=os.environ["GITHUB_RUN_ID"], run_attempt=os.environ["GITHUB_RUN_ATTEMPT"], runner_os=os.environ["RUNNER_OS"])
        write_json(evidence / "profile-admission.json", proof)
        stage_evidence(evidence)
    if failure:
        raise RuntimeError("Hermes profile admission failed; inspect " + str(evidence)) from failure
    print("Captured unqualified explicit Hermes profile metadata/MCP evidence: " + str(evidence))


if __name__ == "__main__":
    if sys.argv[1:] == ["--child"]:
        gate, deadline = Path(os.environ["HERMES_CALL_START_GATE"]), time.monotonic() + 10
        while not gate.is_file():
            require(time.monotonic() < deadline, "owned process gate missing")
            time.sleep(.02)
        child_case()
    elif not sys.argv[1:]:
        try:
            run()
        except BaseException:
            # Any early failure before run() closes its envelope gets only an
            # inert public report; never upload a partially admitted directory.
            if not EVIDENCE_CLOSED:
                safe_failure_evidence(Path(os.environ["RUNNER_TEMP"]).resolve())
            raise
    else:
        raise SystemExit("no arguments, or private --child")
