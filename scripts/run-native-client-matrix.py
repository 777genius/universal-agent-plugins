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
import importlib.util

_DRAFT_SPEC = importlib.util.spec_from_file_location("native_client_draft", Path(__file__).with_name("native_client_draft.py"))
draft = importlib.util.module_from_spec(_DRAFT_SPEC)
_DRAFT_SPEC.loader.exec_module(draft)

PINS = {
    # Exact release archives downloaded and verified against upstream digests.
    "linux-amd64": {
        "rg": ("github", "BurntSushi/ripgrep", "15.2.0", "ripgrep-15.2.0-x86_64-unknown-linux-musl.tar.gz", "sha256:33e15bcf1624b25cdd2a55813a47a2f95dbe126268203e76aa6a585d1e7b149c", "rg"),
        "codex": ("github", "openai/codex", "rust-v0.153.4", "codex-x86_64-unknown-linux-musl.tar.gz", "sha256:f479424eca092484dc40d87ae28c44f4cc40234a60045d6131e493800d814a30", "codex-x86_64-unknown-linux-musl"),
        "claude": ("npm", "@anthropic-ai/claude-code-linux-x64", "2.1.263", "claude-code-linux-x64-2.1.263.tgz", "sha512-0IrvpLd/0FP0acQw59T4Cvx/r4nwAXKBrW0WyhIXymzYWurPCLztB+Icu9MkeewAUI+p3PTXsSfmilv/n6XlAQ==", "claude"),
        "opencode": ("npm", "opencode-linux-x64", "1.18.29", "opencode-linux-x64-1.18.29.tgz", "sha512-X8/wS/8mzL7Ko0zYYF6RzKax39KkxXDRoimhmzXuo0gPrZX4DQjqBNpPAByBwUjFapk73ZGSVsjDGvoNapBa1Q==", "opencode"),
        "lintai": ("github", "777genius/lintai", "v0.1.3", "lintai-v0.1.3-x86_64-unknown-linux-gnu.tar.gz", "sha256:2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", "lintai"),
    },
    "linux-arm64": {
        "rg": ("github", "BurntSushi/ripgrep", "15.2.0", "ripgrep-15.2.0-aarch64-unknown-linux-gnu.tar.gz", "sha256:a740b91c82eaf9914cfedd353572f2791cbe0162c84101ee0951058f4dcbc90d", "rg"),
        "codex": ("github", "openai/codex", "rust-v0.153.4", "codex-aarch64-unknown-linux-musl.tar.gz", "sha256:5cda6182bd94c3a30f2eb63a495489ebf7f691fddb14d70f48c6c1a5071b6cde", "codex-aarch64-unknown-linux-musl"),
        "claude": ("npm", "@anthropic-ai/claude-code-linux-arm64", "2.1.263", "claude-code-linux-arm64-2.1.263.tgz", "sha512-RlJtLbl8xqFMf2zUdOKD4o5FkNhcgGjlS3Un8PNfSbv1fLQg3SqBQgEJhNEtSeKlEsOs26RnCG/jVk4yah8Udw==", "claude"),
        "opencode": ("npm", "opencode-linux-arm64", "1.18.29", "opencode-linux-arm64-1.18.29.tgz", "sha512-Lr7XXik5wPJZBV5RW+aR6Uogf1n5GogpB565m+D/jiO/vRXr9LhvCpixgAr1tiwuH1ZfMsqOOlbFQz5jgH3Tqg==", "opencode"),
        "lintai": ("github", "777genius/lintai", "v0.1.3", "lintai-v0.1.3-aarch64-unknown-linux-gnu.tar.gz", "sha256:132a37610575bd251ecaf0be4c6090dad144dd1397c99aad989a3944c63c3d4a", "lintai"),
    },
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
    "opencode": {"TestAgentpluginsOpenCodeNativeLifecycle", "TestAgentpluginsOpenCodeNativeToolCollision", "TestAgentpluginsOpenCodeNativeRuntimeExtended"},
}


def require_verified_pin(pin):
    if not isinstance(pin, str) or not (
        re.fullmatch(r"sha256:[0-9a-f]{64}", pin)
        or re.fullmatch(r"sha512-[A-Za-z0-9+/]{86}==", pin)
    ):
        raise ValueError("unfilled or invalid archive pin; verify exact upstream archive before native dispatch")


def verify_digest(body, pin):
    require_verified_pin(pin)
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
    require_verified_pin(digest)
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


def provision_release(source, directory, target, tag, commit, repository):
    """Verify public producer bytes before executing them; never rebuild installer."""
    if not re.fullmatch(r"agentplugins-v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)", tag):
        raise ValueError("an exact stable release tag is required")
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("an exact producer source commit is required")
    if repository != "777genius/universal-agent-plugins":
        raise ValueError("unsupported binary producer repository")
    metadata = json.loads(subprocess.check_output(["gh", "api", f"repos/{repository}/releases/tags/{tag}"], encoding="utf-8", errors="strict"))
    if metadata.get("draft") is not False or metadata.get("prerelease") is not False or metadata.get("tag_name") != tag:
        raise ValueError("native release proof requires an exact public stable release")
    tagged = json.loads(subprocess.check_output(["gh", "api", f"repos/{repository}/commits/{tag}"], encoding="utf-8", errors="strict"))
    if tagged.get("sha") != commit:
        raise ValueError("producer release tag differs from expected commit")
    tree = tagged["commit"]["tree"]["sha"]
    if not re.fullmatch(r"[0-9a-f]{40}", tree):
        raise ValueError("invalid producer source tree")
    assets = directory / "release-assets"
    assets.mkdir()
    subprocess.run(["gh", "release", "download", tag, "--repo", repository, "--dir", str(assets)], check=True, timeout=300)
    verified = json.loads(subprocess.check_output(["node", str(source / "npm/agentplugins/scripts/release-assets.js"), "verify", str(assets), tag, commit], encoding="utf-8", errors="strict", timeout=60))
    selected = verified["assets"][target]
    installer = assets / selected["file"]
    attestations = {}
    for name in (selected["file"], "checksums.txt", "release-manifest.json"):
        attestations[name] = json.loads(subprocess.check_output([
            "gh", "attestation", "verify", str(assets / name), "--repo", repository,
            "--signer-workflow", f"github.com/{repository}/.github/workflows/agentplugins-release.yml",
            "--source-digest", commit, "--deny-self-hosted-runners", "--format", "json"], encoding="utf-8", errors="strict", timeout=180))
    installer.chmod(0o700)
    return installer, {"acquisition": "public GitHub release download", "repository": repository,
        "tag": tag, "version": verified["version"], "commit": commit, "tree": tree,
        "file": selected["file"], "binary_sha256": selected["sha256"], "size": selected["size"],
        "manifest_sha256": verified["manifest_sha256"],
        "checksums_sha256": hashlib.sha256((assets / "checksums.txt").read_bytes()).hexdigest(),
        "attestations": attestations}


def require_hosted(target):
    if any(os.environ.get(k) != v for k, v in {"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"}.items()):
        raise RuntimeError("native proof requires explicit opt-in on a disposable GitHub-hosted runner")
    machine = {"aarch64": "arm64", "arm64": "arm64", "x86_64": "amd64", "amd64": "amd64"}.get(platform.machine().lower())
    actual = platform.system().lower() + "-" + str(machine)
    if actual != target:
        raise RuntimeError(f"target {target} does not match actual native platform {actual}")


def disposable_runtime_environment(target):
    # Recheck the actual runner before granting the runtime's Linux test opt-in.
    require_hosted(target)
    env = {"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"}
    if target in ("linux-arm64", "linux-amd64"):
        env["AGENTPLUGINS_NATIVE_DISPOSABLE_LINUX"] = "1"
    return env


def profile_environment(home, target):
    """Prepare the fresh profile before any installer probe or native test."""
    env = {"HOME": str(home), "USERPROFILE": str(home)}
    if target == "windows-amd64":
        for key, directory in (("APPDATA", "Roaming"), ("LOCALAPPDATA", "Local")):
            path = home / "AppData" / directory
            path.mkdir(parents=True, exist_ok=False)
            env[key] = str(path)
    for key, directory in (("XDG_CONFIG_HOME", "config"), ("XDG_CACHE_HOME", "cache"),
                           ("XDG_STATE_HOME", "state"), ("XDG_DATA_HOME", "data"),
                           ("NPM_CONFIG_CACHE", "npm-cache")):
        path = home / directory
        path.mkdir()
        env[key] = str(path)
    for key, filename in (("NPM_CONFIG_USERCONFIG", "npm-user-config"),
                          ("NPM_CONFIG_GLOBALCONFIG", "npm-global-config"),
                          ("GIT_CONFIG_GLOBAL", "git-config")):
        path = home / filename
        path.write_text("")
        env[key] = str(path)
    env["GIT_CONFIG_NOSYSTEM"] = "1"
    env["GIT_TERMINAL_PROMPT"] = "0"
    return env


def find_git_bash(git):
    # GitHub runners may expose Git through bin, cmd, or mingw64/bin.
    # Search only the detected installation ancestors, never an ambient shell.
    path = Path(git).resolve()
    for parent in list(path.parents)[:3]:
        candidate = parent / "bin/bash.exe"
        if candidate.is_file():
            return candidate
    return None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--client", choices=PATTERNS, required=True)
    parser.add_argument("--target", choices=PINS, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--release-tag")
    parser.add_argument("--release-commit")
    parser.add_argument("--release-repo", default="777genius/universal-agent-plugins")
    parser.add_argument("--release-state", choices=("public", "draft"), default="public")
    for field in draft.FIELDS:
        parser.add_argument("--" + field.replace("_", "-"), default="")
    parser.add_argument("--producer-source", type=Path)
    args = parser.parse_args()
    draft.validate(args)
    if args.release_state == "draft" and args.producer_source is None:
        parser.error("draft mode requires --producer-source")
    if args.release_state == "public" and args.producer_source is not None:
        parser.error("public mode must not receive --producer-source")
    if bool(args.release_tag) != bool(args.release_commit):
        parser.error("--release-tag and --release-commit must be supplied together")
    require_hosted(args.target)
    source = Path(__file__).resolve().parents[1]
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    scratch = Path(tempfile.mkdtemp(prefix="uap-native-hosted-", dir=os.environ["RUNNER_TEMP"])).resolve()
    identity = {"schema_version": 1, "client": args.client, "target": args.target, "started_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "isolation": "disposable GitHub-hosted machine; explicit runtime environment; fresh project and profiles", "network": "not blocked; scripted loopback model endpoints; no real-model or OAuth proof", "status": "failed", "scratch": str(scratch)}
    try:
        for key, rev in [("commit", "HEAD"), ("tree", "HEAD^{tree}")]:
            identity[key] = subprocess.check_output(["git", "rev-parse", rev], cwd=source, encoding="utf-8", errors="strict").strip()
        expected = os.environ.get("EXPECTED_COMMIT", "")
        if not re.fullmatch(r"[0-9a-f]{40}", expected) or identity["commit"] != expected:
            raise RuntimeError("checkout differs from expected exact workflow source commit")
        if subprocess.check_output(["git", "status", "--porcelain", "--untracked-files=normal"], cwd=source):
            raise RuntimeError("native proof must build a clean exact checkout")
        for key in (args.client, "lintai", "rg"):
            require_verified_pin(PINS[args.target][key][4])
        binary_dir = scratch / "bin"
        binary_dir.mkdir()
        client, client_evidence = provision(PINS[args.target][args.client], binary_dir, args.client)
        scanner, scanner_evidence = provision(PINS[args.target]["lintai"], binary_dir, "lintai")
        identity["client_asset"] = client_evidence
        identity["scanner_asset"] = scanner_evidence
        _, identity["ripgrep_asset"] = provision(PINS[args.target]["rg"], binary_dir, "rg")
        suffix = ".exe" if os.name == "nt" else ""
        installer, probe, tests = [binary_dir / (p + suffix) for p in ("agentplugins", "native-probe", "repotests")]
        commands = [(["go", "build", "-trimpath", "-o", str(probe), "./repotests/testdata/agentplugins_native_probe"], source), (["go", "test", "-c", "-o", str(tests), "./repotests"], source)]
        if args.release_state == "draft":
            identity["helper_sha256"] = draft.helper_hashes(source)
            tarball, frozen_assets, verified, identity["installer_release"] = draft.acquire(source, binary_dir, args, output)
        elif args.release_tag:
            installer, identity["installer_release"] = provision_release(source, binary_dir, args.target, args.release_tag, args.release_commit, args.release_repo)
        else:
            commands.insert(0, (["go", "build", "-trimpath", "-o", str(installer), "./cmd/agentplugins"], source / "cli/plugin-kit-ai"))
        project = scratch / "project"
        project.mkdir()
        evidence_root = scratch / "evidence"
        evidence_root.mkdir()
        home = scratch / "home"
        home.mkdir()
        env = {**profile_environment(home, args.target), "TEMP": str(evidence_root), "TMP": str(evidence_root), "TMPDIR": str(evidence_root), "PATH": str(binary_dir), "LANG": "en_US.UTF-8", **disposable_runtime_environment(args.target), "AGENTPLUGINS_INSTALLER_BIN": str(installer), "AGENTPLUGINS_INSTALLER_COMMIT": identity["commit"], "AGENTPLUGINS_INSTALLER_TREE": identity["tree"], "AGENTPLUGINS_NATIVE_PROBE_BIN": str(probe), "AGENTPLUGINS_LINTAI_BIN": str(scanner), "AGENTPLUGINS_LINTAI_SHA256": scanner_evidence["binary_sha256"], "AGENTPLUGINS_LINTAI_ARCHIVE_SHA256": PINS[args.target]["lintai"][4].split(":", 1)[1]}
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
                identity["git_path"] = git
                identity["git_version"] = subprocess.check_output([git, "--version"], encoding="utf-8", errors="strict").strip()
                bash = find_git_bash(git)
                if args.client == "claude" and bash is None:
                    raise RuntimeError("Claude Windows proof requires Git Bash in the detected Git installation")
                if bash is not None:
                    env["AGENTPLUGINS_NATIVE_GIT_BASH_PATH"] = str(bash)
                    env["AGENTPLUGINS_NATIVE_GIT_SHELL_BIN_DIR"] = str(bash.parent)
                    env["PATH"] += os.pathsep + str(bash.parent)
                    shell = bash.parent / "sh.exe"
                    if args.client == "opencode" and not shell.is_file():
                        raise RuntimeError("OpenCode lifecycle fixture requires sh.exe from the detected Git installation")
                    if shell.is_file():
                        identity["git_sh_sha256"] = hashlib.sha256(shell.read_bytes()).hexdigest()
                    identity["git_bash_sha256"] = hashlib.sha256(bash.read_bytes()).hexdigest()
        else:
            env["PATH"] += ":/usr/bin:/bin"
        build_env = dict(env)
        go = shutil.which("go")
        if not go:
            raise RuntimeError("Go toolchain missing")
        build_env["PATH"] = str(Path(go).parent) + os.pathsep + build_env["PATH"]
        build_env["GOPATH"] = str(scratch / "go")
        build_env["GOCACHE"] = str(scratch / "go-cache")
        for command, cwd in commands:
            subprocess.run(command, cwd=cwd, env=build_env, check=True, timeout=360)
        identity["harness_build_sha256"] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in (probe, tests)}
        if args.release_state == "draft":
            installer, identity["packaged_acquisition"] = draft.bootstrap(tarball, frozen_assets, verified, args.target, scratch, env)
            env["AGENTPLUGINS_INSTALLER_BIN"] = str(installer)
        identity["installer_sha256"] = hashlib.sha256(installer.read_bytes()).hexdigest()
        if args.release_tag:
            release = identity["installer_release"]
            env["AGENTPLUGINS_INSTALLER_COMMIT"] = release["commit"]
            env["AGENTPLUGINS_INSTALLER_TREE"] = release["tree"]
            measured = subprocess.check_output([str(installer), "version"], cwd=project, env=env, encoding="utf-8", errors="strict", timeout=30).strip()
            identity["installer_version_measured"] = measured
            if measured != "agentplugins " + release["version"]:
                raise RuntimeError(f"released installer version mismatch: {measured!r}")
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
        identity["artifact_sha256"] = {p.relative_to(output).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest() for p in output.rglob("*") if p.is_file()}
        (output / "runner-evidence.json").write_text(json.dumps(identity, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
