#!/usr/bin/env python3
"""Run the local installer resilience proof from current source.

Every black-box client HOME, installer state and package is disposable. The
runner never reads or mutates the user's real agent configuration.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import tempfile
import time


REPO = Path(__file__).resolve().parent.parent
AGENT_MODULE = REPO / "install/integrationctl/agentplugins"
INTEGRATION_MODULE = REPO / "install/integrationctl"
CLI_MODULE = REPO / "cli"
RESILIENCE_VERSION = "999.0.0-resilience"


COVERAGE = {
    "fault_injection": ["filesystem-kernels", "black-box-resilience",
                        "terminal-fault-injection"],
    "concurrent_processes": ["transaction-state-clients",
                             "black-box-resilience"],
    "corrupt_state": ["transaction-state-clients", "black-box-resilience",
                      "fuzz-state"],
    "filesystem_attacks": ["filesystem-kernels", "black-box-resilience"],
    "network_and_cache": ["network-cache-security", "npm-package-tests",
                          "npm-packed-lifecycle"],
    "tty_matrix": ["terminal-contracts", "terminal-pty",
                   "selection-matrix", "terminal-fault-injection"],
    "all_clients": ["transaction-state-clients", "plugin-matrix",
                    "black-box-resilience:fresh-multi-client-install",
                    "black-box-resilience:per-client-lifecycle"],
    "group_rollback": ["transaction-state-clients",
                       "black-box-resilience:add-remove-race"],
    "state_migrations": ["transaction-state-clients",
                         "black-box-resilience:migrate-state-v2",
                         "black-box-resilience:migrate-state-v3",
                         "historical-release-migrations:0.1.4,0.1.5,0.1.6"],
    "packed_npm": ["npm-package-tests", "npm-pack-dry-run",
                   "npm-packed-lifecycle"],
    "native_platform_runtime": [
        "terminal-ui CI: ubuntu-24.04, macos-15, windows-2025",
        "cross-build-linux-amd64", "cross-build-windows-amd64",
    ],
    "resource_errors": ["black-box-resilience", "terminal-fault-injection"],
    "ux_contracts": ["cli-ux-contracts", "terminal-contracts",
                     "terminal-pty", "selection-matrix"],
    "fuzzing": ["fuzz-state", "fuzz-manifest", "fuzz-targets",
                "fuzz-diagnostics", "fuzz-sources"],
}


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n",
                    encoding="utf-8")


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def source_fingerprint():
    paths = subprocess.check_output([
        "git", "ls-files", "-z", "--cached", "--others",
        "--exclude-standard",
    ], cwd=REPO).split(b"\0")
    value = hashlib.sha256()
    for raw in sorted(path for path in paths if path):
        path = REPO / os.fsdecode(raw)
        value.update(len(raw).to_bytes(8, "big"))
        value.update(raw)
        if path.is_symlink():
            value.update(b"L" + os.fsencode(os.readlink(path)))
        elif path.is_file():
            body = path.read_bytes()
            value.update(b"F" + len(body).to_bytes(8, "big") + body)
        else:
            value.update(b"D")
    return value.hexdigest()


class Proof:
    def __init__(self, artifacts, timeout):
        self.artifacts = artifacts
        self.timeout = timeout
        self.logs = artifacts / "logs"
        self.logs.mkdir(parents=True)
        self.phases = []
        self.source_head = subprocess.check_output(
            ["git", "rev-parse", "HEAD"], cwd=REPO, text=True).strip()
        self.source_dirty = bool(subprocess.check_output(
            ["git", "status", "--porcelain=v1", "-z"], cwd=REPO))
        self.source_fingerprint = source_fingerprint()

        temp = artifacts / "tmp"
        temp.mkdir()
        self.env = dict(os.environ)
        self.env.update({
            "GOCACHE": str(artifacts / "go-cache"),
            "NO_COLOR": "1",
            "TMPDIR": str(temp),
            "TMP": str(temp),
            "TEMP": str(temp),
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_COUNT": "1",
            "GIT_CONFIG_KEY_0": "core.hooksPath",
            "GIT_CONFIG_VALUE_0": "/dev/null",
            "PYTHONDONTWRITEBYTECODE": "1",
        })

    def save(self):
        write_json(self.artifacts / "report.json", {
            "schema_version": 1,
            "tool_version": RESILIENCE_VERSION,
            "source": str(REPO),
            "source_head": self.source_head,
            "source_dirty": self.source_dirty,
            "source_fingerprint_sha256": self.source_fingerprint,
            "platform": platform.platform(),
            "isolation": (
                "disposable HOME, client roots, installer state and package"),
            "phases": self.phases,
            "coverage": COVERAGE,
        })

    def run(self, name, argv, cwd=REPO, env=None, timeout=None):
        started = time.monotonic()
        stdout = self.logs / f"{name}.stdout"
        stderr = self.logs / f"{name}.stderr"
        row = {
            "name": name,
            "argv": [str(value) for value in argv],
            "cwd": str(cwd),
            "status": "running",
        }
        self.phases.append(row)
        self.save()
        try:
            with stdout.open("wb") as out, stderr.open("wb") as err:
                result = subprocess.run(
                    row["argv"], cwd=cwd,
                    env=dict(self.env, **(env or {})), stdout=out, stderr=err,
                    timeout=timeout or self.timeout)
            row["exit"] = result.returncode
            row["status"] = "pass" if result.returncode == 0 else "fail"
        except subprocess.TimeoutExpired:
            row.update(status="fail", error="timeout")
        except Exception as error:
            row.update(status="fail", error=repr(error))
        row["elapsed_seconds"] = round(time.monotonic() - started, 3)
        row["stdout_sha256"] = digest(stdout)
        row["stderr_sha256"] = digest(stderr)
        self.save()
        print(json.dumps({
            "phase": name,
            "status": row["status"],
            "seconds": row["elapsed_seconds"],
        }), flush=True)
        return row["status"] == "pass"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifacts", type=Path)
    parser.add_argument("--timeout", type=float, default=900)
    args = parser.parse_args()

    if args.artifacts:
        artifacts = args.artifacts.resolve()
        if artifacts.exists():
            parser.error("--artifacts must name a new directory")
        artifacts.mkdir(parents=True)
    else:
        artifacts = Path(tempfile.mkdtemp(
            prefix="agentplugins-resilience-e2e-")).resolve()

    go = shutil.which("go")
    npm = shutil.which("npm")
    node = shutil.which("node")
    if not go or not npm or not node:
        parser.error("go, npm and node must be available on PATH")

    proof = Proof(artifacts, args.timeout)
    go_test = [go, "test", "-count=1", "-timeout=10m"]
    proof.run("transaction-state-clients", go_test + [
        "./transaction", "./usecase", "./adapters/statev2",
        "./adapters/statemigration", "./adapters/nativeconfig",
        "./providers", "./clients/...",
    ], cwd=AGENT_MODULE)
    proof.run("network-cache-security", go_test + [
        "./adapters/directoryv1", "./adapters/discoveryv1",
        "./adapters/sourceacquisition", "./adapters/securityscan",
    ], cwd=AGENT_MODULE)
    proof.run("filesystem-kernels", go_test + [
        "./adapters/dirswap", "./adapters/atomicfile",
        "./adapters/pathpolicy",
    ], cwd=INTEGRATION_MODULE)
    proof.run("cli-ux-contracts", go_test + [
        "./internal/agentpluginscli", "./internal/terminalprompts",
        "./internal/promptio", "./cmd/agentplugins/...",
    ], cwd=CLI_MODULE)
    proof.run("terminal-contracts", [
        sys.executable, "-m", "unittest", "discover", "-s",
        "scripts/terminal-ui", "-p", "test_*.py",
    ])

    binary = artifacts / ("agentplugins.exe" if os.name == "nt"
                          else "agentplugins")
    built = proof.run("build-current-source", [
        go, "build", "-trimpath",
        f"-ldflags=-X=main.version={RESILIENCE_VERSION}",
        "-o", binary, "./cmd/agentplugins",
    ], cwd=CLI_MODULE)
    if built:
        proof.run("black-box-resilience", [
            sys.executable, "scripts/terminal-ui/resilience_matrix.py",
            "--binary", binary, "--artifacts",
            artifacts / "black-box", "--timeout", "20",
        ])
        if os.name == "posix":
            proof.run("historical-release-migrations", [
                sys.executable,
                "scripts/terminal-ui/historical_state_flow.py",
                "--binary", binary, "--go", go,
                "--artifacts", artifacts / "historical-release-migrations",
            ], timeout=600)

            proof.run("terminal-pty", [
                sys.executable, "scripts/terminal-ui/harness.py", "--binary",
                binary, "--artifacts", artifacts / "terminal-pty",
                "--timeout", "15",
            ])
            proof.run("plugin-matrix", [
                sys.executable, "scripts/terminal-ui/plugin_matrix.py",
                "--binary", binary, "--artifacts",
                artifacts / "plugin-matrix", "--timeout", "15",
            ])
            proof.run("selection-matrix", [
                sys.executable, "scripts/terminal-ui/selection_matrix.py",
                "--build-go", go, "--artifacts",
                artifacts / "selection-matrix", "--timeout", "15",
            ])
            proof.run("terminal-fault-injection", [
                sys.executable, "scripts/terminal-ui/qualification_faults.py",
                "--artifacts", artifacts / "terminal-faults",
                "--timeout", "15",
            ])

        proof.run("npm-packed-lifecycle", [
            sys.executable, "scripts/terminal-ui/npm_packed_flow.py",
            "--binary", binary, "--npm", npm, "--node", node,
            "--artifacts", artifacts / "npm-packed",
        ])
    for name, cwd, target in (
            ("fuzz-state", AGENT_MODULE,
             "./adapters/statev2:FuzzStateJSON"),
            ("fuzz-manifest", AGENT_MODULE,
             "./conformance:FuzzPluginManifest"),
            ("fuzz-targets", CLI_MODULE,
             "./internal/agentpluginscli:FuzzParseTargetOption"),
            ("fuzz-diagnostics", CLI_MODULE,
             "./internal/agentpluginscli:FuzzSafeDiagnosticText"),
            ("fuzz-sources", CLI_MODULE,
             "./internal/agentpluginscli:FuzzSourceClassification")):
        package, fuzz = target.split(":", 1)
        proof.run(name, [
            go, "test", package, "-run=^$", f"-fuzz=^{fuzz}$",
            "-fuzztime=5s", "-parallel=2",
        ], cwd=cwd)

    proof.run("npm-package-tests", [
        node, "--test",
        "test/bootstrap.test.js",
        "test/documentation-contract.test.js",
        "test/homebrew-formula.test.js",
        "test/notices.test.js",
        "test/npm-public-contract.test.js",
        "test/platform-proof.test.js",
    ], cwd=REPO / "npm/agentplugins")
    proof.run("npm-pack-dry-run", [
        npm, "pack", "--dry-run", "--ignore-scripts",
    ], cwd=REPO / "npm/agentplugins")

    cross = artifacts / "cross-build"
    cross.mkdir()
    for goos, goarch in (("linux", "amd64"), ("windows", "amd64")):
        suffix = ".exe" if goos == "windows" else ""
        proof.run(f"cross-build-{goos}-{goarch}", [
            go, "build", "-trimpath", "-o",
            cross / f"agentplugins-{goos}-{goarch}{suffix}",
            "./cmd/agentplugins",
        ], cwd=CLI_MODULE, env={
            "CGO_ENABLED": "0", "GOOS": goos, "GOARCH": goarch,
        })

    proof.save()
    failed = [row for row in proof.phases if row["status"] != "pass"]
    print(json.dumps({
        "result": "failure" if failed else "success",
        "artifacts": str(artifacts),
        "passed": len(proof.phases) - len(failed),
        "failed": [row["name"] for row in failed],
    }), flush=True)
    return int(bool(failed))


if __name__ == "__main__":
    raise SystemExit(main())
