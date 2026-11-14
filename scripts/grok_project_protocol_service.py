"""Run either fixed Grok journey in one delegated, ordinary-user CI service.

The child evidence is private until the existing finite systemd lifecycle proves
closure. A clean negative native result stays negative and retains its original
bytes; service, identity or publication uncertainty never grants an upload.
"""
import argparse
import base64
import hashlib
import json
import math
import os
from pathlib import Path
import re
import stat
import sys
import time

import project_native_runtime as runtime
from grok_project_protocol_client import document, STATE_SCHEMA
from grok_project_protocol_enablement import ordinary_repair_state

shared = runtime.shared
LIMIT = runtime.MAX_ZIP
COMMON_FIELDS = frozenset("schema_version source_commit source_tree passed commands stages vendor_authentication_used "
    "unmanaged_project_owned_enablement_qualified installer_sha256 probe_sha256".split())
OPTIONAL_FIELDS = frozenset("acquisition all_original_events events_sha256 events_base64 error failed_command "
    "native_profile_preparation installed_config_receipts enablement_previews ordinary_repair_observation owned_enablement owned_enablement_removal repair prepared_owned_state "
    "pre_removal_data_receipts post_removal_installation project_b_unchanged protected_fixture_files_match_expected "
    "untrusted_refusal_verified removed_absent persistent_data_retained".split())
CASE_FIELDS = {
    "grok-protocol": frozenset("model_invoked grok_tools_call_qualified transport".split()),
    "grok-noauth": frozenset("github_run_id github_run_attempt github_event platform actual_model_used scripted_local_provider "
                            "grok_native_tools_call_qualified".split()),
}
CASE_OPTIONAL = {
    "grok-protocol": frozenset("native_doctor_initialize_and_list_qualified direct_installed_config_tools_call_qualified".split()),
    "grok-noauth": frozenset("project_b_negative removed_native_negative native_provider_sessions".split()),
}
STAGES = ("installed", "updated", "repaired", "plugin-cwd", "data-cwd", "restarted-data-cwd")
COMMAND_FIELDS = frozenset("probe argv cwd error exit elapsed_seconds stdout_file stderr_file stdout_bytes stderr_bytes "
    "stdout_sha256 stderr_sha256 stdout stderr stdout_base64 stderr_base64".split())


def require(condition, message):
    if not condition:
        raise ValueError(message)


def directory_identity(info):
    return info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid


def file_identity(info):
    return (*shared.artifacts.identity(info), info.st_uid, info.st_gid)


class Inputs:
    """Hold the original ancestry and bounded executable/source input leaves."""
    def __init__(self, budget):
        self.budget, self.directories, self.files = budget, {}, {}

    def close(self):
        problem = None
        for descriptor, *_ in (*self.files.values(), *self.directories.values()):
            try:
                os.close(descriptor)
            except OSError as error:
                problem = problem or error
        if problem is not None:
            raise problem

    def directory(self, path):
        require(path.is_absolute() and path.resolve() == path, "canonical input directory required")
        for parent in reversed((path, *path.parents)):
            if parent in self.directories:
                continue
            self.budget.check()
            if parent == Path("/"):
                descriptor = os.open(parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            else:
                descriptor = os.open(parent.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
                                     dir_fd=self.directories[parent.parent][0])
            info = os.fstat(descriptor)
            if not stat.S_ISDIR(info.st_mode) or info.st_uid not in (0, os.getuid()):
                os.close(descriptor)
                raise ValueError("original input ancestry required")
            self.directories[parent] = (descriptor, directory_identity(info))
        self.join_directories()
        return self.directories[path][0]

    def join_directories(self):
        for path, (descriptor, identity) in self.directories.items():
            self.budget.check()
            require(path.resolve() == path and directory_identity(path.lstat()) == identity
                    and directory_identity(os.fstat(descriptor)) == identity, "input ancestry changed")

    def pin(self, path, owners, maximum, *, executable=False, empty=False):
        require(path not in self.files and path.is_absolute() and path.resolve() == path, "distinct canonical input file required")
        parent = self.directory(path.parent)
        before = os.stat(path.name, dir_fd=parent, follow_symlinks=False)
        require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1
                and (before.st_uid, before.st_gid) in owners and not before.st_mode & 0o022
                and (0 if empty else 1) <= before.st_size <= maximum
                and (not executable or bool(before.st_mode & 0o111)), "owned bounded input required")
        descriptor = os.open(path.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent)
        identity = file_identity(before)
        if file_identity(os.fstat(descriptor)) != identity:
            os.close(descriptor)
            raise ValueError("opened input changed")
        self.files[path] = (descriptor, identity, None, maximum)
        body = self.read(path)
        digest = hashlib.sha256(body).hexdigest()
        self.files[path] = (descriptor, identity, digest, maximum)
        return digest

    def read(self, path):
        self.join_directories()
        descriptor, identity, expected, maximum = self.files[path]
        require(file_identity(path.lstat()) == identity and file_identity(os.fstat(descriptor)) == identity,
                "held input changed before read")
        os.lseek(descriptor, 0, os.SEEK_SET)
        chunks, count = [], 0
        while True:
            self.budget.check()
            part = os.read(descriptor, min(65536, maximum + 1 - count))
            if not part:
                break
            chunks.append(part)
            count += len(part)
            require(count <= maximum, "input grew")
        body = b"".join(chunks)
        require(len(body) == identity[4] and file_identity(os.fstat(descriptor)) == identity
                and file_identity(path.lstat()) == identity
                and (expected is None or hashlib.sha256(body).hexdigest() == expected), "held input changed while reading")
        self.join_directories()
        return body

    def verify(self):
        for path in self.files:
            self.read(path)


def child_environment(root, expected):
    values = {key: os.environ.get(key, "") for key in ("GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT", "GITHUB_EVENT_NAME")}
    require(re.fullmatch(r"[1-9][0-9]{0,19}", values["GITHUB_RUN_ID"])
            and re.fullmatch(r"[1-9][0-9]{0,4}", values["GITHUB_RUN_ATTEMPT"])
            and values["GITHUB_EVENT_NAME"] in ("pull_request", "push", "workflow_dispatch"), "original workflow identity required")
    return {**values, "HOME": str(root / "home"), "PATH": "/usr/bin:/bin", "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8",
            "NO_COLOR": "1", "TERM": "dumb", "TMPDIR": str(root / "tmp"), "RUNNER_TEMP": str(root / "tmp"),
            "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0",
            "PYTHONDONTWRITEBYTECODE": "1", "GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted",
            "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1", "EXPECTED_COMMIT": expected}


def source_identity(ctx, phase):
    result = {}
    for name, arguments in (("commit", ("rev-parse", "HEAD")), ("tree", ("rev-parse", "HEAD^{tree}")),
                            ("clean", ("status", "--porcelain", "--untracked-files=normal"))):
        record, stdout, stderr = ctx.command("grok-source-" + phase + "-" + name,
            ["/usr/bin/git", *arguments], runtime.SOURCE, ctx.env, 10)
        require(record["exit"] == 0 and not stderr, "source query refused")
        result[name] = stdout.decode("ascii", "strict").strip()
    require(re.fullmatch(r"[0-9a-f]{40}", result["commit"])
            and re.fullmatch(r"[0-9a-f]{40}", result["tree"]) and not result["clean"], "exact clean source required")
    return {"source_commit": result["commit"], "source_tree": result["tree"]}


def command_streams(rows, budget):
    require(type(rows) is list and len(rows) <= 128, "bounded command records required")
    seen = set()
    for row in rows:
        budget.check()
        require(type(row) is dict and set(row) == COMMAND_FIELDS and type(row["probe"]) is str
                and shared.LABEL.fullmatch(row["probe"]) and row["probe"] not in seen, "closed unique command required")
        seen.add(row["probe"])
        require(type(row["argv"]) is list and 1 <= len(row["argv"]) <= 64
                and all(type(item) is str and 0 < len(item) <= 4096 and "\x00" not in item for item in row["argv"])
                and type(row["cwd"]) is str and Path(row["cwd"]).is_absolute()
                and len(row["cwd"]) <= 4096 and type(row["exit"]) is int and -255 <= row["exit"] <= 255
                and type(row["elapsed_seconds"]) in (float, int) and math.isfinite(row["elapsed_seconds"])
                and 0 <= row["elapsed_seconds"] <= 900
                and (row["error"] is None or type(row["error"]) is str and len(row["error"]) <= 512), "command record differs")
        for stream in ("stdout", "stderr"):
            raw = base64.b64decode(row[stream + "_base64"], validate=True)
            require(len(raw) <= shared.STREAM and type(row[stream + "_bytes"]) is int
                    and len(raw) == row[stream + "_bytes"] and hashlib.sha256(raw).hexdigest() == row[stream + "_sha256"]
                    and raw.decode("utf-8", "replace") == row[stream]
                    and row[stream + "_file"] == row["probe"] + "." + stream + ".log", "original command stream differs")


def admit_evidence(body, case, record, expected, env, budget):
    require(case in CASE_FIELDS and type(body) is bytes and 0 < len(body) <= LIMIT, "bounded selected evidence required")
    value = runtime.strict_json(body)
    required = COMMON_FIELDS | CASE_FIELDS[case]
    require(type(value) is dict and required <= set(value) <= required | OPTIONAL_FIELDS | CASE_OPTIONAL[case]
            and type(value["schema_version"]) is int and value["schema_version"] == 1
            and all(value.get(key) == item for key, item in expected.items()), "child evidence identity differs")
    passed = value["passed"]
    require(type(passed) is bool and type(record.get("exit")) is int and record["exit"] in (0, 1)
            and record.get("error") is None and passed is (record["exit"] == 0)
            and value["vendor_authentication_used"] is False
            and value["unmanaged_project_owned_enablement_qualified"] is passed
            and type(value["stages"]) is list and len(value["stages"]) <= 16, "native exit or qualification differs")
    require((passed and "error" not in value) or (not passed and type(value.get("error")) is str
            and 0 < len(value["error"]) <= 4096), "negative evidence must remain negative")
    require(all(type(row) is dict for row in value["stages"])
            and tuple(row.get("stage") for row in value["stages"]) == STAGES[:len(value["stages"])]
            and (not passed or len(value["stages"]) == len(STAGES)), "native stage prefix differs")
    if case == "grok-protocol":
        require(value["model_invoked"] is False and value["grok_tools_call_qualified"] is False
                and value["transport"] == "stdio", "protocol boundary differs")
        for key in CASE_OPTIONAL[case]:
            require(value.get(key, False) is passed, "protocol qualification differs")
    else:
        require(value["actual_model_used"] is False and value["scripted_local_provider"] is True
                and value["grok_native_tools_call_qualified"] is passed and value["platform"] == "Linux-x86_64"
                and value["github_run_id"] == env["GITHUB_RUN_ID"]
                and value["github_run_attempt"] == env["GITHUB_RUN_ATTEMPT"]
                and value["github_event"] == env["GITHUB_EVENT_NAME"], "local-provider boundary differs")
    command_streams(value["commands"], budget)
    if "ordinary_repair_observation" in value:
        admit_ordinary_repair(value)
    if passed:
        require(all(row["error"] is None for row in value["commands"]), "successful evidence has an incomplete capture")
    budget.check()
    return passed


def ordinary_repair_selection(value):
    before = value.get("prepared_owned_state")
    require(type(before) is dict and type(before.get("schema_version")) is int
            and before["schema_version"] == STATE_SCHEMA and type(before.get("installations")) is list
            and all(type(item) is dict for item in before["installations"]), "original prepared state required")
    rows = [row for row in value["commands"] if row["probe"] == "repair-no-enable-consent"]
    require(len(rows) == 1 and rows[0]["exit"] == 0 and rows[0]["error"] is None, "original ordinary repair command required")
    row = rows[0]
    argv, project = row["argv"], Path(row["cwd"])
    require(len(argv) == 12 and argv[1] == "repair" and str(project) == row["cwd"]
            and argv[3:] == ["--scope", "project", "--project-root", str(project), "--target", "grok", "--format", "json", "--yes"],
            "ordinary repair command selection differs")
    matches = [item for item in before["installations"] if item.get("installation_id") == argv[2]]
    require(len(matches) == 1 and type(matches[0].get("clients")) is dict and len(matches[0]["clients"]) == 1,
            "original ordinary repair installation differs")
    installation = matches[0]
    binding = next(iter(installation["clients"].values()))
    require(type(binding) is dict and binding.get("client_id") == "grok" and binding.get("scope") == "project"
            and binding.get("activation") == "prepared" and binding.get("verification") == "package_validated",
            "original ordinary repair binding differs")
    artifact = binding.get("physical_artifact_id")
    require(type(artifact) is str and artifact not in ("", ".", "..") and Path(artifact).name == artifact
            and "\\" not in artifact, "original ordinary repair artifact differs")
    active = project / ".grok/plugins" / artifact
    require(binding.get("target_locator") == str(active) and binding.get("native_profile_root") == str(project / ".grok"),
            "original ordinary repair path differs")
    return before, installation, binding, active, row


def admit_ordinary_repair(value):
    observation = value["ordinary_repair_observation"]
    fields = {"partial", "no_consent_config_unchanged", "no_consent_exact_directory_repair", "post_repair_state_base64",
              "post_repair_state_bytes", "before_state_sha256", "after_state_sha256", "mcp_witness_execution_observed",
              "internal_acp_zero_proved_by_this_capture"}
    require(type(observation) is dict and set(observation) == fields and observation["partial"] is True
            and observation["no_consent_config_unchanged"] is True and observation["mcp_witness_execution_observed"] is False
            and observation["internal_acp_zero_proved_by_this_capture"] is False, "closed partial repair observation required")
    encoded, length = observation["post_repair_state_base64"], observation["post_repair_state_bytes"]
    require(type(encoded) is str and len(encoded) <= 4 * (((1 << 20) + 2) // 3)
            and type(length) is int and 0 < length <= 1 << 20, "partial repair original byte bound")
    raw = base64.b64decode(encoded, validate=True)
    require(len(raw) == length and base64.b64encode(raw).decode() == encoded
            and hashlib.sha256(raw).hexdigest() == observation["after_state_sha256"], "partial repair original bytes differ")
    # The original before bytes are not retained; this digest remains only a
    # closed original claim. The parsed prepared state supplies delta authority.
    require(type(observation["before_state_sha256"]) is str
            and re.fullmatch(r"[0-9a-f]{64}", observation["before_state_sha256"]), "partial repair prior digest differs")
    proof = observation["no_consent_exact_directory_repair"]
    require(type(proof) is dict and set(proof) == {"operation_id", "receipt", "updated_at", "started_at_unix_ns", "finished_at_unix_ns"},
            "closed ordinary repair proof required")
    before, installation, binding, active, row = ordinary_repair_selection(value)
    window = proof["started_at_unix_ns"], proof["finished_at_unix_ns"]
    require(all(type(item) is int for item in window) and 0 <= window[1] - window[0] <= 900_000_000_000,
            "ordinary repair original window differs")
    expected = ordinary_repair_state(before, document(raw), installation, binding, active,
                                    document(base64.b64decode(row["stdout_base64"], validate=True)), window)
    require(json.dumps(proof, sort_keys=True) == json.dumps(expected, sort_keys=True), "ordinary repair proof differs")


def revoke_output(descriptor, original, initial, payload, inputs):
    """Terminal cleanup only: never truncate a foreign path, append or parent."""
    sink = Path(os.environ["GITHUB_OUTPUT"])
    try:
        # No new timer or admission follows a deadline. These bounded metadata
        # joins permit only removal of our exact control-file tail.
        for path, (held, identity) in inputs.directories.items():
            require(path.resolve() == path and directory_identity(path.lstat()) == identity
                    and directory_identity(os.fstat(held)) == identity, "output rollback parent changed")
        info = os.fstat(descriptor)
        require(file_identity(sink.lstat()) == file_identity(info)
                and (info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_uid, info.st_gid)
                == (original[0], original[1], original[2], original[3], original[-2], original[-1])
                and original[4] <= info.st_size <= original[4] + len(payload), "workflow output rollback authority changed")
        os.lseek(descriptor, 0, os.SEEK_SET)
        body = os.read(descriptor, shared.STREAM + len(payload) + 1)
        require(body == initial + payload[:len(body) - len(initial)], "workflow output rollback bytes changed")
        os.ftruncate(descriptor, len(initial))
        return True
    except (OSError, ValueError):
        # A stale string cannot authorize upload: the workflow also requires
        # this launcher's final successful process outcome.
        return False


def workflow_output(descriptor, original, initial, inputs, passed):
    """The two literal values are provisional until the launcher exits zero."""
    require(type(passed) is bool, "literal native verdict required")
    sink = Path(os.environ["GITHUB_OUTPUT"])
    inputs.join_directories()
    require(file_identity(os.fstat(descriptor)) == original and file_identity(sink.lstat()) == original,
            "workflow output authority changed")
    payload = b"native_passed=" + (b"true" if passed else b"false") + b"\npublication=admitted\n"
    inputs.budget.check()
    try:
        require(os.write(descriptor, payload) == len(payload), "workflow output write incomplete")
        after = os.fstat(descriptor)
        require((after.st_dev, after.st_ino, after.st_mode, after.st_nlink, after.st_uid, after.st_gid)
                == (original[0], original[1], original[2], original[3], original[-2], original[-1])
                and after.st_size == len(initial) + len(payload)
                and file_identity(sink.lstat()) == file_identity(after), "workflow output authority changed after write")
        os.lseek(descriptor, 0, os.SEEK_SET)
        require(os.read(descriptor, len(initial) + len(payload) + 1) == initial + payload
                and file_identity(os.fstat(descriptor)) == file_identity(after)
                and file_identity(sink.lstat()) == file_identity(after), "workflow output bytes changed after write")
        inputs.join_directories()
        inputs.budget.check()
    except Exception:
        revoke_output(descriptor, original, initial, payload, inputs)
        raise
    return payload


def publish(output, body, receipt, inputs, output_sink):
    """Only called after positive service closure; do not reopen child paths."""
    require(receipt["delegated_service"]["closed"] is True
            and receipt["delegated_service"]["forced_stop"] is False
            and receipt["delegated_service"]["forced_kill"] is False, "service closure not proved")
    inputs.verify()
    parent = inputs.directory(output)
    require(not os.path.lexists(output / "upload"), "fresh publication directory required")
    encoded = json.dumps(receipt, sort_keys=True).encode()
    require(len(encoded) <= LIMIT, "bounded delegation receipt required")
    os.mkdir("upload", 0o700, dir_fd=parent)
    upload = inputs.directory(output / "upload")
    for name, content in (("evidence.json", body), ("delegation.json", encoded)):
        inputs.budget.check()
        inputs.join_directories()
        descriptor = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=upload)
        try:
            with os.fdopen(os.dup(descriptor), "wb") as target:
                target.write(content)
            os.fsync(descriptor)
        finally:
            os.close(descriptor)
        path = output / "upload" / name
        require(inputs.pin(path, ((os.getuid(), os.getgid()),), LIMIT) == hashlib.sha256(content).hexdigest(),
                "published bytes changed")
    inputs.verify()
    require(set(os.listdir(upload)) == {"evidence.json", "delegation.json"}, "publication members changed")
    return workflow_output(*output_sink, inputs, receipt["native_passed"])


def run(case, installer, probe, output):
    origin = time.monotonic_ns()
    budget = shared.Budget(900)
    budget.deadline = origin / 1e9 + 900
    inputs, sink, payload, status = Inputs(budget), None, None, 1
    stage = "preflight"
    try:
        with budget:
            runtime.cline_host()
            require(case in runtime.GROK_SCRIPTS, "fixed Grok case required")
            expected = os.environ.get("EXPECTED_COMMIT", "")
            require(re.fullmatch(r"[0-9a-f]{40}", expected), "exact source commit required")
            require(output.is_absolute() and output.resolve() == output and not os.path.lexists(output)
                    and runtime.SOURCE != output and runtime.SOURCE not in output.parents, "fresh output outside source required")
            parent = inputs.directory(output.parent)
            os.mkdir(output.name, 0o700, dir_fd=parent)
            inputs.directory(output)
            summary = {"commands": []}
            ctx = shared.Context(output, summary, budget)
            ctx.env = child_environment(output, expected)
            inputs.directory(runtime.SOURCE)
            inputs.pin(runtime.SOURCE / "scripts" / runtime.GROK_SCRIPTS[case], ((os.getuid(), os.getgid()),), 2 << 20)
            python = Path("/usr/bin/python3").resolve(strict=True)
            python_hash = inputs.pin(python, ((0, 0),), 64 << 20, executable=True)
            require(Path(sys.executable).resolve(strict=True) == python, "selected parent interpreter differs")
            require(installer != probe and installer.name == "agentplugins" and probe.name == "probe"
                    and installer.parent == probe.parent, "exact compiled role pair required")
            expected_files = {"installer_sha256": inputs.pin(installer, ((os.getuid(), os.getgid()),), 512 << 20, executable=True),
                              "probe_sha256": inputs.pin(probe, ((os.getuid(), os.getgid()),), 32 << 20, executable=True)}
            ctx.identities.update({installer: expected_files["installer_sha256"], probe: expected_files["probe_sha256"]})
            source = source_identity(ctx, "before")
            require(source["source_commit"] == expected, "unexpected source commit")
            sink_path = Path(os.environ["GITHUB_OUTPUT"])
            parent = inputs.directory(sink_path.parent)
            before = sink_path.lstat()
            require(sink_path.resolve() == sink_path and stat.S_ISREG(before.st_mode) and before.st_nlink == 1
                    and before.st_uid == os.getuid() and before.st_gid == os.getgid()
                    and not before.st_mode & 0o022 and before.st_size == 0, "fresh owned workflow output required")
            sink = os.open(sink_path.name, os.O_RDWR | os.O_APPEND | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent)
            sink_identity = file_identity(before)
            require(file_identity(os.fstat(sink)) == sink_identity, "workflow output changed while opening")
            sink_initial = os.read(sink, shared.STREAM + 1)
            require(len(sink_initial) == before.st_size and file_identity(os.fstat(sink)) == sink_identity,
                    "workflow output changed while reading")
            inputs.verify()
            stage = "service"
            record, _, _ = runtime.grok_service(ctx, case, (python, installer, probe), ctx.env, output, origin)
            closure = summary.get("delegated_service", {})
            require(closure.get("closed") is True and closure.get("forced_stop") is False
                    and closure.get("forced_kill") is False and record.get("error") is None
                    and type(record.get("exit")) is int and record["exit"] in (0, 1), "service completion not proved")
            stage = "admission"
            inputs.verify()
            require(source_identity(ctx, "after") == source, "source changed")
            child = output / "child"
            child_fd = inputs.directory(child)
            child_info = os.fstat(child_fd)
            require((child_info.st_uid, child_info.st_gid, stat.S_IMODE(child_info.st_mode))
                    == (os.getuid(), os.getgid(), 0o700)
                    and set(os.listdir(child_fd)) == {"evidence.json"}, "closed owned child evidence directory required")
            evidence_path = child / "evidence.json"
            inputs.pin(evidence_path, ((os.getuid(), os.getgid()),), LIMIT)
            body = inputs.read(evidence_path)
            passed = admit_evidence(body, case, record, {**source, **expected_files}, ctx.env, budget)
            receipt = {"schema_version": 1, "kind": "grok-delegated-service", "case": case, **source, **expected_files,
                "python_sha256": python_hash, "original_start_monotonic_ns": origin, "native_passed": passed,
                "evidence_bytes": len(body), "evidence_sha256": hashlib.sha256(body).hexdigest(),
                "delegated_service": summary["delegated_service"], "commands": summary["commands"],
                "streams_base64": {key: base64.b64encode(value).decode() for key, value in ctx.captures.items()}}
            stage = "publication"
            payload = publish(output, body, receipt, inputs, (sink, sink_identity, sink_initial))
        # This is publication success only. The mandatory workflow verdict
        # retains a clean native exit1 as a failed native job.
        status = 0
    except Exception:
        if payload is not None:
            revoke_output(sink, sink_identity, sink_initial, payload, inputs)
    finally:
        try:
            inputs.close()
        except Exception:
            status = 1
            if payload is not None:
                revoke_output(sink, sink_identity, sink_initial, payload, inputs)
            stage = "finalization"
        finally:
            if sink is not None:
                try:
                    os.close(sink)
                except OSError:
                    status = 1
                    stage = "finalization"
    try:
        budget.check()
    except Exception:
        status = 1
        stage = "finalization"
    if status:
        print(json.dumps({"publication": "withheld", "native_passed": False, "stage": stage}, sort_keys=True))
    return status


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--case", choices=runtime.GROK_SCRIPTS, required=True)
    for name in ("installer", "probe", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    args = parser.parse_args()
    return run(args.case, args.installer, args.probe, args.output)


if __name__ == "__main__":
    sys.exit(main())
