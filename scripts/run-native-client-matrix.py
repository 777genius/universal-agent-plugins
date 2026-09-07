#!/usr/bin/env python3
"""Pinned real-client proof, exclusively on disposable GitHub-hosted machines.

Downloads do not run npm scripts. Runtime receives an explicit credential-free
allowlist and a new project/profile. No firewall isolation or real model/OAuth
proof is claimed. Native fixture evidence, not this wrapper, proves behavior.
"""
import argparse
import base64
import hashlib
import io
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request
import zipfile

PINS = {
    "darwin-arm64": {
        "rg": ("github", "BurntSushi/ripgrep", "15.2.0", "ripgrep-15.2.0-aarch64-apple-darwin.tar.gz", "sha256:3750b2e93f37e0c692657da574d7019a101c0084da05a790c83fd335bad973e4", "rg"),
        "codex": ("github", "openai/codex", "rust-v0.153.4", "codex-aarch64-apple-darwin.tar.gz", "sha256:8cf911ea676523bfb2121ec561848d2aba564890ad536db4d8a3353f2b9850b1", "codex-aarch64-apple-darwin"),
        "claude": ("npm", "@anthropic-ai/claude-code-darwin-arm64", "2.1.263", "claude-code-darwin-arm64-2.1.263.tgz", "sha512-yLv8MtgulGMGCWTwDUSmlEL0+94sxKP3vJm0SAXZX+0qVQ09PrjVSRC0ibtVeW7xymyhvP1JaUimOyp+t2wO5g==", "claude"),
        "opencode": ("npm", "opencode-darwin-arm64", "1.18.29", "opencode-darwin-arm64-1.18.29.tgz", "sha512-EU0qma5GPJXcDp5rENvedSfqJjqxatQW/Qzjs71pR2YdbrSh7YmBwtvnIb2/KIvfQr0DErX4ST14sbBQI2mQdg==", "opencode"),
        "lintai": ("github", "777genius/lintai", "v0.1.3", "lintai-v0.1.3-aarch64-apple-darwin.tar.gz", "sha256:be8b263e2323074080d928ea7c2129458299a6d03f7a9f178dfc1aa8e6bc17ff", "lintai"),
    },
    "windows-amd64": {
        "rg": ("github", "BurntSushi/ripgrep", "15.2.0", "ripgrep-15.2.0-x86_64-pc-windows-msvc.zip", "sha256:71b2fef860abe467217a538ff31de02f5258807c0129f771846f87bd029aafc5", "rg.exe"),
        "codex": ("github", "openai/codex", "rust-v0.153.4", "codex-x86_64-pc-windows-msvc.exe.zip", "sha256:c016b0e6968b78586919c720d2685a03712f6d5f11bcd9d6f92c91eb8c41ba16", "codex-x86_64-pc-windows-msvc.exe"),
        "claude": ("npm", "@anthropic-ai/claude-code-win32-x64", "2.1.263", "claude-code-win32-x64-2.1.263.tgz", "sha512-P1LnudAWj2ptyE0IZzCZKAe7nU7LxqlkcLdBfp8FsHlH5dp75Og4JJ6TClvfjanHn3shYW4CYN6oI1vSqZlkHw==", "claude.exe"),
        "opencode": ("npm", "opencode-windows-x64", "1.18.29", "opencode-windows-x64-1.18.29.tgz", "sha512-xtWiZNMiwFtBl7q9yMG+XK/BTYGfgFoHLDx4ruWVYCoxjV0NYwjJi6rB9B3xIVbTclzu81YLKNFcT3kNBEG9Yw==", "opencode.exe"),
        "lintai": ("github", "777genius/lintai", "v0.1.3", "lintai-v0.1.3-x86_64-pc-windows-msvc.zip", "sha256:2f61f6a83a160afa3feed9ea1722b82d0d938ebff865a4e20d39b5f55270c911", "lintai.exe"),
    },
}
PATTERNS = {
    "codex": r"^TestAgentpluginsCodexNativeLifecycle$",
    "claude": r"^TestAgentpluginsClaudeNative(Lifecycle|RuntimeLifecycle)$",
    "opencode": r"^TestAgentpluginsOpenCodeNative",
}
REQUIRED_TESTS = {
    "codex": {"TestAgentpluginsCodexNativeLifecycle"},
    "claude": {"TestAgentpluginsClaudeNativeLifecycle", "TestAgentpluginsClaudeNativeRuntimeLifecycle"},
    "opencode": {"TestAgentpluginsOpenCodeNativeLifecycle", "TestAgentpluginsOpenCodeNativeToolCollision"},
}


def verify_digest(body, pin):
    if pin.startswith("sha256:"):
        actual = "sha256:" + hashlib.sha256(body).hexdigest()
    elif pin.startswith("sha512-"):
        actual = "sha512-" + base64.b64encode(hashlib.sha512(body).digest()).decode()
    else:
        raise ValueError("unsupported archive digest")
    if actual != pin:
        raise ValueError("archive integrity mismatch")


def extract_binary(body, archive, basename):
    # Only materialize the exact regular binary; archive paths never become paths
    # on disk, so symlinks, traversal, and arbitrary package scripts cannot run.
    if archive.endswith(".zip"):
        with zipfile.ZipFile(io.BytesIO(body)) as source:
            matches = [m for m in source.infolist() if not m.is_dir() and Path(m.filename).name == basename and ((m.external_attr >> 16) & 0o170000) != 0o120000]
            if len(matches) != 1:
                raise ValueError(f"expected one {basename} binary, found {len(matches)}")
            return source.read(matches[0])
    with tarfile.open(fileobj=io.BytesIO(body), mode="r:gz") as source:
        matches = [m for m in source.getmembers() if m.isfile() and Path(m.name).name == basename]
        if len(matches) != 1:
            raise ValueError(f"expected one {basename} binary, found {len(matches)}")
        return source.extractfile(matches[0]).read()


def provision(pin, directory, name):
    kind, repository, version, archive, digest, basename = pin
    if kind == "github":
        subprocess.run(["gh", "release", "download", version, "--repo", repository, "--pattern", archive, "--dir", str(directory)], check=True, timeout=240)
        body = (directory / archive).read_bytes()
        source = f"https://github.com/{repository}/releases/tag/{version}"
    else:
        url = f"https://registry.npmjs.org/{repository}/-/{archive}"
        with urllib.request.urlopen(url, timeout=180) as response:
            body = response.read(400 << 20)
        source = url
    verify_digest(body, digest)
    binary = extract_binary(body, archive, basename)
    path = directory / (name + (".exe" if os.name == "nt" else ""))
    path.write_bytes(binary)
    path.chmod(0o700)
    (directory / archive).unlink(missing_ok=True)
    return path, {"source": source, "version": version, "archive": archive, "archive_integrity": digest, "binary_sha256": hashlib.sha256(binary).hexdigest()}


def require_hosted(target):
    if any(os.environ.get(k) != v for k, v in {"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"}.items()):
        raise RuntimeError("native proof requires explicit opt-in on a disposable GitHub-hosted runner")
    machine = {"aarch64": "arm64", "arm64": "arm64", "x86_64": "amd64", "amd64": "amd64"}.get(platform.machine().lower())
    actual = platform.system().lower() + "-" + str(machine)
    if actual != target:
        raise RuntimeError(f"target {target} does not match actual native platform {actual}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--client", choices=PATTERNS, required=True)
    parser.add_argument("--target", choices=PINS, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    require_hosted(args.target)
    source = Path(__file__).resolve().parents[1]
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    scratch = Path(tempfile.mkdtemp(prefix="uap-native-hosted-", dir=os.environ["RUNNER_TEMP"])).resolve()
    identity = {"schema_version": 1, "client": args.client, "target": args.target, "started_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "isolation": "disposable GitHub-hosted machine; explicit runtime environment; fresh project and profiles", "network": "not blocked; scripted loopback model endpoints; no real-model or OAuth proof", "status": "failed", "scratch": str(scratch)}
    try:
        for key, rev in [("commit", "HEAD"), ("tree", "HEAD^{tree}")]:
            identity[key] = subprocess.check_output(["git", "rev-parse", rev], cwd=source, text=True).strip()
        expected = os.environ.get("EXPECTED_COMMIT", "")
        if not re.fullmatch(r"[0-9a-f]{40}", expected) or identity["commit"] != expected:
            raise RuntimeError("checkout differs from expected exact workflow source commit")
        if subprocess.check_output(["git", "status", "--porcelain", "--untracked-files=normal"], cwd=source):
            raise RuntimeError("native proof must build a clean exact checkout")
        binary_dir = scratch / "bin"
        binary_dir.mkdir()
        client, client_evidence = provision(PINS[args.target][args.client], binary_dir, args.client)
        scanner, scanner_evidence = provision(PINS[args.target]["lintai"], binary_dir, "lintai")
        identity["client_asset"] = client_evidence
        identity["scanner_asset"] = scanner_evidence
        _, identity["ripgrep_asset"] = provision(PINS[args.target]["rg"], binary_dir, "rg")
        suffix = ".exe" if os.name == "nt" else ""
        installer, probe, tests = [binary_dir / (p + suffix) for p in ("agentplugins", "native-probe", "repotests")]
        for command, cwd in [(["go", "build", "-trimpath", "-o", str(installer), "./cmd/agentplugins"], source / "cli/plugin-kit-ai"), (["go", "build", "-trimpath", "-o", str(probe), "./repotests/testdata/agentplugins_native_probe"], source), (["go", "test", "-c", "-o", str(tests), "./repotests"], source)]:
            subprocess.run(command, cwd=cwd, check=True, timeout=360)
        identity["build_sha256"] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in (installer, probe, tests)}
        project = scratch / "project"
        project.mkdir()
        evidence_root = scratch / "evidence"
        evidence_root.mkdir()
        home = scratch / "home"
        home.mkdir()
        env = {"HOME": str(home), "USERPROFILE": str(home), "TEMP": str(evidence_root), "TMP": str(evidence_root), "TMPDIR": str(evidence_root), "PATH": str(binary_dir), "LANG": "en_US.UTF-8", "GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1", "AGENTPLUGINS_INSTALLER_BIN": str(installer), "AGENTPLUGINS_INSTALLER_COMMIT": identity["commit"], "AGENTPLUGINS_INSTALLER_TREE": identity["tree"], "AGENTPLUGINS_NATIVE_PROBE_BIN": str(probe), "AGENTPLUGINS_LINTAI_BIN": str(scanner), "AGENTPLUGINS_LINTAI_SHA256": scanner_evidence["binary_sha256"], "AGENTPLUGINS_LINTAI_ARCHIVE_SHA256": PINS[args.target]["lintai"][4].split(":", 1)[1]}
        if os.name == "nt":
            env["SystemRoot"] = os.environ["SystemRoot"]
            env["COMSPEC"] = str(Path(env["SystemRoot"]) / "System32/cmd.exe")
            env["PATH"] += os.pathsep + str(Path(env["SystemRoot"]) / "System32")
            git = shutil.which("git")
            if not git:
                raise RuntimeError("Windows native proof requires the runner Git installation")
            if git:
                env["PATH"] += os.pathsep + str(Path(git).parent)
                env["AGENTPLUGINS_NATIVE_GIT_BIN_DIR"] = str(Path(git).parent)
                identity["git_version"] = subprocess.check_output([git, "--version"], text=True).strip()
                bash = Path(git).parent.parent / "bin/bash.exe"
                if bash.is_file():
                    env["AGENTPLUGINS_NATIVE_GIT_BASH_PATH"] = str(bash)
                    identity["git_bash_sha256"] = hashlib.sha256(bash.read_bytes()).hexdigest()
        else:
            env["PATH"] += ":/usr/bin:/bin"
        prefix = "AGENTPLUGINS_" + args.client.upper()
        env[prefix + "_NATIVE_E2E"] = "1"
        env[prefix + "_BIN"] = str(client)
        env[prefix + "_SHA256"] = client_evidence["binary_sha256"]
        env[prefix + "_VERSION"] = {"codex": "0.153.4", "claude": "2.1.263 (Claude Code)", "opencode": "1.18.29"}[args.client]
        command = [str(tests), "-test.v", "-test.timeout=15m", "-test.run=" + PATTERNS[args.client]]
        identity["test_command"] = command
        with (output / "native-tests.log").open("w", encoding="utf-8") as transcript:
            result = subprocess.run(command, cwd=project, env=env, stdout=transcript, stderr=subprocess.STDOUT, timeout=930)
        identity["exit_code"] = result.returncode
        log = (output / "native-tests.log").read_text(encoding="utf-8", errors="replace")
        print(log[-40000:])
        passed = set(re.findall(r"^--- PASS: (\w+)", log, re.M))
        skipped = re.findall(r"^\s*--- SKIP: ([^\s]+)", log, re.M)
        identity["passed_tests"] = sorted(passed)
        identity["skipped_tests"] = skipped
        if result.returncode or skipped or not REQUIRED_TESTS[args.client].issubset(passed):
            raise RuntimeError("native suite failed, skipped, or omitted a required test; inspect transcript")
        identity["status"] = "passed"
    finally:
        identity["finished_utc"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        # Keep structured evidence and transcripts, never copy client caches,
        # profile databases, source payload binaries, or ephemeral client auth.
        evidence_dir = scratch / "evidence"
        allowed = {".json", ".jsonl", ".log"}
        if evidence_dir.exists():
            for fixture in evidence_dir.iterdir():
                if not fixture.is_dir():
                    continue
                for path in fixture.iterdir():
                    if path.is_file() and path.suffix in allowed:
                        destination = output / fixture.name / path.name
                        destination.parent.mkdir(parents=True, exist_ok=True)
                        shutil.copyfile(path, destination)
        identity["artifact_sha256"] = {str(p.relative_to(output)): hashlib.sha256(p.read_bytes()).hexdigest() for p in output.rglob("*") if p.is_file()}
        (output / "runner-evidence.json").write_text(json.dumps(identity, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
