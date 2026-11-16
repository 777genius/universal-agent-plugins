#!/usr/bin/env python3
"""Source-bound, credential-free native Pi project E2E using the real UAP facade.

Runs only on explicitly opted-in disposable GitHub-hosted runners. Pi's
--approve/--no-approve flags change trust for a single child process and never
write an approval to the user's profile. Runs without either flag leave the
decision to UAP's owned project_trust hook and Pi's default policy. All effects
stay under runner temp.
"""
import hashlib
import io
import json
import os
from pathlib import Path
import platform
import re
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import zipfile

PINS = {
    "linux-amd64": ("pi-linux-x64.tar.gz", "sha256:f2e4e4329603223d03260cdb8bd5033dbdc1f73ba58ab8939db2e56546e115d2", "pi"),
    "darwin-arm64": ("pi-darwin-arm64.tar.gz", "sha256:79d9979d76a2a5adb7c28e27e9b44629928ccd0dc9fbb6ae7217860bd5e8be5d", "pi"),
    "windows-amd64": ("pi-windows-x64.zip", "sha256:0f7256ea8bdac3af20012b94fdc1252a31cc6dc0d2f9b9eb539555d22e3e321b", "pi.exe"),
}
REPO = "earendil-works/pi"
VERSION = "v1.4.0"


def require_hosted(target):
    required = {"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted",
                "UAP_PI_NATIVE_PROJECT_E2E": "1"}
    if any(os.environ.get(k) != v for k, v in required.items()):
        raise RuntimeError("only explicitly opted-in disposable hosted runner admitted")
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine().lower())
    if platform.system().lower() + "-" + str(arch) != target:
        raise RuntimeError("selected Pi platform differs from actual runner")


def run(argv, cwd, env=None, input_bytes=None, timeout=80, max_output=1 << 20):
    result = subprocess.run(argv, cwd=cwd, env=env, input=input_bytes,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            timeout=timeout, check=False)
    if len(result.stdout) > max_output or len(result.stderr) > max_output:
        raise RuntimeError("native child output exceeded bounded limit")
    if result.returncode:
        raise RuntimeError("native child failed: " + str(Path(argv[0]).name) +
                           " exit=" + str(result.returncode) + " stderr-tail=" +
                           result.stderr[-1200:].decode("utf-8", errors="replace"))
    return result.stdout


def checked_release(root, target, acquire_env):
    archive, digest, basename = PINS[target]
    path = root / archive
    try:
        run(["gh", "release", "download", VERSION, "--repo", REPO, "--pattern",
             archive, "--dir", str(root)], root, env=acquire_env, timeout=240)
    except RuntimeError:
        # A subset of Windows hosted runners cannot reach api.github.com even
        # when github.com release assets are available. Both routes are pinned
        # to the identical signed-off upstream tag and SHA-256 bytes.
        import urllib.request
        official = f"https://github.com/{REPO}/releases/download/{VERSION}/{archive}"
        request = urllib.request.Request(official, headers={"User-Agent": "uap-pi-native-e2e"})
        count = 0
        with urllib.request.urlopen(request, timeout=45) as response, path.open("wb") as output:
            if not response.url.startswith("https://"):
                raise RuntimeError("Pi release redirected to a non-HTTPS resource")
            while True:
                chunk = response.read(1 << 20)
                if not chunk:
                    break
                count += len(chunk)
                if count > 175 << 20:
                    raise RuntimeError("Pi release archive exceeds bounded size")
                output.write(chunk)
    body = path.read_bytes()
    if len(digest) != 71 or "sha256:" + hashlib.sha256(body).hexdigest() != digest:
        raise RuntimeError("official Pi release archive SHA-256 mismatch")
    if archive.endswith(".zip"):
        with zipfile.ZipFile(io.BytesIO(body)) as item:
            members = [p for p in item.infolist() if not p.is_dir() and
                       Path(p.filename).name == basename and
                       ((p.external_attr >> 16) & 0o170000) != stat.S_IFLNK]
            if len(members) != 1:
                raise RuntimeError("Pi archive has ambiguous executable")
            binary = item.read(members[0])
    else:
        with tarfile.open(fileobj=io.BytesIO(body), mode="r:gz") as item:
            members = [p for p in item.getmembers() if p.isfile() and
                       Path(p.name).name == basename]
            if len(members) != 1:
                raise RuntimeError("Pi archive has ambiguous executable")
            stream = item.extractfile(members[0])
            if stream is None:
                raise RuntimeError("Pi executable absent")
            binary = stream.read()
    if not (1 << 20) < len(binary) < (175 << 20):
        raise RuntimeError("Pi executable size outside admitted bound")
    executable = root / basename
    executable.write_bytes(binary)
    executable.chmod(0o700)
    path.unlink()
    return executable, {"tag": VERSION, "archive": archive, "sha256": digest,
                         "binary_sha256": hashlib.sha256(binary).hexdigest()}


def write(root, relative, body):
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(body, encoding="utf-8")
    return path


def author_source(source, revision):
    write(source, "plugin.json", json.dumps({
        "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
        "name": "uap-pi-project-native-proof", "version": "1.0.0",
        "extensions": {"com.universal-agent-plugins.pi": {
            "version": 1, "source": "native"}}}, separators=(",", ":")))
    code = ('import { appendFileSync } from "node:fs";\n'
            'appendFileSync(process.env.UAP_PI_PROJECT_EVENT_FILE, ' +
            json.dumps(revision + "|") + ' + process.cwd() + "\\n");\n'
            'export default function piProjectProof() {}\n')
    write(source, "native/index.ts", code)


def project_operation(driver, state, user, project, source, action, evidence):
    args = [str(driver), str(state), str(user), str(project), str(source),
            action, "TEST-pi-project-native", str(evidence)]
    run(args, project, timeout=95)
    data = json.loads(evidence.read_text())
    if data.get("outcome") not in ("completed", "unchanged"):
        raise RuntimeError("project UAP facade failed operation")
    return data


def native_rpc(pi, project, user, event_file, approve, target, stage):
    """approve=None passes no trust override, so Pi asks extensions first."""
    if event_file.exists():
        event_file.unlink()
    env = {
        "PATH": os.environ.get("PATH", ""),
        "PI_CODING_AGENT_DIR": str(user),
        "PI_OFFLINE": "1",
        "UAP_PI_PROJECT_EVENT_FILE": str(event_file),
        "HOME": str(user.parent.parent),
        "USERPROFILE": str(user.parent.parent),
    }
    if target.startswith("windows"):
        for key in ("SystemRoot", "COMSPEC", "WINDIR", "PATHEXT"):
            if os.environ.get(key):
                env[key] = os.environ[key]
        env["APPDATA"] = str(user.parent.parent / "AppData/Roaming")
        env["LOCALAPPDATA"] = str(user.parent.parent / "AppData/Local")
        Path(env["APPDATA"]).mkdir(parents=True, exist_ok=True)
        Path(env["LOCALAPPDATA"]).mkdir(parents=True, exist_ok=True)
    command = [str(pi), "--mode", "rpc", "--no-session", "--offline"]
    if approve is not None:
        command.append("--approve" if approve else "--no-approve")
    payload = json.dumps({"id": stage, "type": "get_state"}).encode() + b"\n"
    output = run(command, project, env=env, input_bytes=payload, timeout=45, max_output=1 << 20)
    records = [json.loads(line) for line in output.splitlines() if line.strip()]
    matching = [row for row in records if row.get("id") == stage and
                row.get("type") == "response" and
                row.get("command") == "get_state" and row.get("success") is True]
    if len(matching) != 1:
        raise RuntimeError("native Pi RPC did not complete the bounded get_state handshake")
    lines = event_file.read_text(encoding="utf-8").splitlines() if event_file.exists() else []
    if len(lines) > 1 or any(len(s) > 1024 for s in lines):
        raise RuntimeError("Pi extension executed more than once or leaked unbounded facts")
    return lines


def assert_observation(lines, revision, project):
    if revision is None:
        if lines:
            raise RuntimeError("Pi loaded a project extension without trust or after removal")
    else:
        if len(lines) != 1 or not lines[0].startswith(revision + "|"):
            raise RuntimeError("Pi did not load exact UAP-managed project revision")
        observed = Path(lines[0].split("|", 1)[1])
        if observed.resolve() != project.resolve():
            raise RuntimeError("Pi loaded extension in a different project")


HOOK = "extensions/agentplugins-project-trust"


def trust_hook_files(user):
    return [user / HOOK / "index.ts", user / HOOK / "projects.json"]


def owned_project_trust(driver, pi, user, project, package, output, marker, target, state):
    """UAP's hook trusts an owned-only project, never foreign or revoked names."""
    def installed_artifact(evidence):
        active = Path(evidence.get("binding", {}).get("target_path") or "")
        if active.parent != project / ".pi/extensions" or not (active / "index.ts").is_file():
            raise RuntimeError("UAP public project install did not produce Pi native discovery leaf")
        return active

    active = installed_artifact(project_operation(driver, project / ".uap", user, project, package,
                                                  "install", output / "c.json"))
    if not all(path.is_file() for path in trust_hook_files(user)):
        raise RuntimeError("confirmed project install did not record UAP's Pi trust hook")
    assert_observation(native_rpc(pi, project, user, marker, None, target, "C-owned"), "one", project)
    state["stages"].append("owned_only_project_trusted_by_uap_hook")
    # An installation saved before UAP set up trust has neither hook file.
    for path in trust_hook_files(user):
        path.unlink()
    assert_observation(native_rpc(pi, project, user, marker, None, target, "C-before-grant"), None, project)
    state["stages"].append("project_untrusted_before_grant")
    project_operation(driver, project / ".uap", user, project, package, "install", output / "c-repeat.json")
    assert_observation(native_rpc(pi, project, user, marker, None, target, "C-granted"), "one", project)
    state["stages"].append("confirmed_repeat_granted_owned_project_trust")
    # Trust is bound to the recorded content, not only to the owned name.
    owned_code = active / "native/index.ts"
    original = owned_code.read_bytes()
    owned_code.write_bytes(original + b"// changed after trust was recorded\n")
    assert_observation(native_rpc(pi, project, user, marker, None, target, "C-changed"), None, project)
    owned_code.write_bytes(original)
    assert_observation(native_rpc(pi, project, user, marker, None, target, "C-restored"), "one", project)
    state["stages"].append("changed_owned_content_untrusted")
    # Decisions Pi already owns win over the hook: a saved "Do not trust" and
    # a defaultProjectTrust of "never" keep the owned project untrusted.
    for name, body, stage in (
            ("trust.json", {str(project.resolve()): False}, "C-saved-no"),
            ("settings.json", {"defaultProjectTrust": "never"}, "C-never")):
        decision = write(user, name, json.dumps(body))
        assert_observation(native_rpc(pi, project, user, marker, None, target, stage), None, project)
        decision.unlink()
    assert_observation(native_rpc(pi, project, user, marker, None, target, "C-own-again"), "one", project)
    state["stages"].append("saved_pi_decisions_override_uap_hook")
    foreign = write(project, ".pi/extensions/later-foreign.ts",
                    'import { appendFileSync } from "node:fs";\n'
                    'appendFileSync(process.env.UAP_PI_PROJECT_EVENT_FILE, "foreign|" + process.cwd() + "\\n");\n'
                    'export default function laterForeign() {}\n')
    assert_observation(native_rpc(pi, project, user, marker, None, target, "C-foreign"), None, project)
    foreign.unlink()
    state["stages"].append("later_foreign_extension_untrusted")
    author_source(package, "c-two")
    project_operation(driver, project / ".uap", user, project, package, "update", output / "c-update.json")
    assert_observation(native_rpc(pi, project, user, marker, None, target, "C-updated"), "c-two", project)
    author_source(package, "one")
    state["stages"].append("confirmed_update_rerecorded_owned_content")
    return active


def revoked_project_trust(driver, pi, user, project, package, active, output, marker, target, state):
    """Removal revokes at once, while the hook still serves other projects."""
    project_operation(driver, project / ".uap", user, project, package, "remove", output / "c-remove.json")
    if not all(path.is_file() for path in trust_hook_files(user)):
        raise RuntimeError("removing one project dropped the hook other projects use")
    # A recreated formerly owned name must not inherit revoked trust.
    write(project, ".pi/extensions/" + active.name + "/index.ts",
          'import { appendFileSync } from "node:fs";\n'
          'appendFileSync(process.env.UAP_PI_PROJECT_EVENT_FILE, "revoked|" + process.cwd() + "\\n");\n'
          'export default function revoked() {}\n')
    assert_observation(native_rpc(pi, project, user, marker, None, target, "C-revoked"), None, project)
    shutil.rmtree(active)
    state["stages"].append("removal_revoked_project_trust")


def main():
    if len(sys.argv) != 3 or sys.argv[1] not in PINS:
        raise SystemExit("usage: pi_project_native_e2e.py TARGET OUTPUT")
    target, output = sys.argv[1], Path(sys.argv[2]).resolve()
    require_hosted(target)
    source = Path(__file__).resolve().parents[1]
    commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=source, text=True).strip()
    if commit != os.environ.get("EXPECTED_COMMIT") or not re.fullmatch("[0-9a-f]{40}", commit):
        raise RuntimeError("exact UAP source SHA required")
    if subprocess.check_output(["git", "status", "--porcelain"], cwd=source):
        raise RuntimeError("UAP source checkout not clean")
    output.mkdir(parents=True, exist_ok=False)
    state = {"status": "failed", "source_commit": commit, "target": target, "pi": {},
             "stages": [], "vendor_auth_qualified": False, "real_model_qualified": False}
    try:
        with tempfile.TemporaryDirectory(prefix="uap-pi-project-hosted-", dir=os.environ["RUNNER_TEMP"]) as t:
            root = Path(t).resolve()
            acquire_env = {key: value for key, value in os.environ.items() if key in
                           ("PATH", "GH_TOKEN", "GITHUB_TOKEN", "SystemRoot", "COMSPEC")}
            pi, pin = checked_release(root, target, acquire_env)
            state["pi"] = pin
            driver = root / ("uap-pi-project-driver.exe" if target.startswith("windows") else "uap-pi-project-driver")
            run(["go", "build", "-trimpath", "-o", str(driver), "./clients/pi/testdata/project-driver"],
                source / "install/integrationctl/agentplugins", timeout=240)
            user = root / "home/.pi/agent"
            a, b = root / "project-A", root / "project-B"
            package = root / "source"
            for path in (user, a, b, package):
                path.mkdir(parents=True)
            for project in (a, b):
                write(project, ".pi/extensions/foreign.ts", "export default function foreign() {}\n")
            write(user, "extensions/foreign.ts", "export default function foreign() {}\n")
            author_source(package, "one")
            marker = root / "pi-load-events.log"
            c = root / "project-C"
            c.mkdir()
            c_active = owned_project_trust(driver, pi, user, c, package, output, marker, target, state)
            file_a, file_b = output / "a.json", output / "b.json"
            installed_a = project_operation(driver, a / ".uap", user, a, package, "install", file_a)
            installed_b = project_operation(driver, b / ".uap", user, b, package, "install", file_b)
            for project, row in ((a, installed_a), (b, installed_b)):
                binding = row.get("binding", {})
                if binding.get("scope") != "project":
                    raise RuntimeError("UAP Pi native install receipt lost selected project scope")
                active = Path(binding.get("TargetPath") or binding.get("target_path") or "")
                if active.parent != project / ".pi/extensions" or not (active / "index.ts").is_file():
                    raise RuntimeError("UAP public project install did not produce Pi native discovery leaf")
            assert_observation(native_rpc(pi, a, user, marker, None, target, "A-foreign"), None, a)
            state["stages"].append("foreign_project_extension_untrusted")
            assert_observation(native_rpc(pi, a, user, marker, False, target, "untrusted"), None, a)
            state["stages"].append("untrusted_project_suppressed")
            assert_observation(native_rpc(pi, a, user, marker, True, target, "A-1"), "one", a)
            assert_observation(native_rpc(pi, b, user, marker, True, target, "B-1"), "one", b)
            state["stages"].append("native_A_B_extension_discovery")
            author_source(package, "two")
            project_operation(driver, a / ".uap", user, a, package, "update", output / "a-update.json")
            assert_observation(native_rpc(pi, a, user, marker, True, target, "A-2"), "two", a)
            assert_observation(native_rpc(pi, b, user, marker, True, target, "B-stable"), "one", b)
            state["stages"].append("native_A_update_B_preserved")
            project_operation(driver, a / ".uap", user, a, package, "remove", output / "a-remove.json")
            assert_observation(native_rpc(pi, a, user, marker, True, target, "A-removed"), None, a)
            assert_observation(native_rpc(pi, b, user, marker, True, target, "B-retained"), "one", b)
            for path in (a / ".pi/extensions/foreign.ts", b / ".pi/extensions/foreign.ts",
                         user / "extensions/foreign.ts"):
                if path.read_text() != "export default function foreign() {}\n":
                    raise RuntimeError("Pi project native cleanup changed caller-owned extension")
            state["stages"].append("native_A_remove_B_user_foreign_preserved")
            revoked_project_trust(driver, pi, user, c, package, c_active, output, marker, target, state)
            project_operation(driver, b / ".uap", user, b, package, "remove", output / "b-remove.json")
            if any(path.exists() for path in trust_hook_files(user)):
                raise RuntimeError("removing the last owned project left UAP's Pi trust hook")
            if (b / ".pi/extensions/foreign.ts").read_text() != "export default function foreign() {}\n":
                raise RuntimeError("Pi project native cleanup changed caller-owned extension")
            state["stages"].append("last_project_removal_removed_trust_hook")
            if (user / "trust.json").exists():
                raise RuntimeError("Pi persisted a trust decision during the E2E")
            state["status"] = "passed"
    finally:
        (output / "native-pi-project-evidence.json").write_text(
            json.dumps(state, sort_keys=True, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
