#!/usr/bin/env python3
"""Build a Plain-only rollback rehearsal in a disposable exact-base archive.

This is evidence tooling, never production fallback. Source and the index stay
unchanged. Removed UI files exist only in the disposable proof tree.
"""
import argparse
import difflib
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile

BASE = "75851d78e4d54d586b1697efd5a0ab30f1f5b6a8"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", default="/tmp/uap-go-toolchain/go/bin/go")
    parser.add_argument("--artifacts", required=True)
    args = parser.parse_args()
    source = Path(__file__).resolve().parents[2]
    artifacts = Path(args.artifacts).resolve()
    artifacts.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="plain-rollback-", dir=artifacts))
    env = dict(os.environ, GOWORK="off", GOTOOLCHAIN="local", GOPROXY="off",
               GOMODCACHE="/tmp/uap-go-modcache", GOCACHE="/tmp/uap-go-buildcache",
               GOFLAGS="-buildvcs=false",
               HOME=str(work / "home"), XDG_CONFIG_HOME=str(work / "config"))
    Path(env["HOME"]).mkdir()
    archive = subprocess.check_output(["git", "archive", BASE, "cli", "sdk", "install", ".github", "scripts"], cwd=source)
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        tar.extractall(work, filter="data")
    module = work / "cli/plugin-kit-ai"
    adapters = module / "internal/terminalprompts"
    manifest = {"base": BASE, "tree": str(work), "archive_sha256": hashlib.sha256(archive).hexdigest(),
                "removed_fixture_files": [], "commands": [], "status": "running"}
    original = {p: p.read_text() for p in (adapters / "mode.go", module / "go.mod", module / "go.sum")}

    def run(argv, log):
        with (artifacts / log).open("w") as output:
            result = subprocess.run(argv, cwd=module, env=env, stdout=output, stderr=subprocess.STDOUT)
        manifest["commands"].append({"argv": argv, "log": log, "exit": result.returncode})
        if result.returncode:
            raise RuntimeError(f"command failed: see {artifacts / log}")

    try:
        # Exclude rich source and its tests only in the archived rehearsal.
        for path in sorted(adapters.glob("*.go")):
            if path.name not in ("plain.go", "mode.go"):
                manifest["removed_fixture_files"].append(str(path.relative_to(work)))
                path.unlink()
        mode = adapters / "mode.go"
        branch = '\tif mode == Rich {\n\t\treturn HuhPrompter{Input: input, Output: visible, NoColor: noColor || os.Getenv("NO_COLOR") != ""}, visible, nil\n\t}\n'
        text = mode.read_text()
        if text.count(branch) != 1 or text.count('\treturn Rich\n') != 1:
            raise RuntimeError("base wiring changed")
        mode.write_text(text.replace(branch, "").replace('\treturn Rich\n', '\treturn Plain\n'))
        contract = (source / "cli/plugin-kit-ai/internal/terminalprompts/adapter_success_contract_test.go").read_text()
        manifest["shared_contract_sha256"] = hashlib.sha256(contract.encode()).hexdigest()
        rich = '\t\t\t\t\tif adapter == "huh" {\n\t\t\t\t\t\tp = HuhPrompter{Input: input, Output: &output, NoColor: true}\n\t\t\t\t\t}\n'
        if contract.count(rich) != 1:
            raise RuntimeError("shared contract construction changed")
        (adapters / "adapter_success_contract_test.go").write_text(
            contract.replace('[]string{"plain", "huh"}', '[]string{"plain"}').replace(rich, ""))
        run([args.go, "mod", "tidy"], "rollback-tidy.log")
        run([args.go, "list", "-deps", "./cmd/agentplugins"], "rollback-deps.log")
        deps = (artifacts / "rollback-deps.log").read_text().splitlines()
        forbidden = [d for d in deps if d.startswith(("charm.land/", "github.com/charmbracelet/"))]
        if forbidden:
            raise RuntimeError(f"rich dependencies remain: {forbidden}")
        run([args.go, "test", "./internal/agentpluginscli/prompt", "./internal/terminalprompts",
             "./internal/agentpluginscli", "./cmd/agentplugins/..."], "rollback-tests.log")
        binary = artifacts / "agentplugins-plain"
        run([args.go, "build", "-o", str(binary), "./cmd/agentplugins"], "rollback-build.log")
        run([str(binary), "--help"], "rollback-help.log")
        manifest.update(status="passed", binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest(),
                        rich_dependency_count=len(forbidden))
    except Exception as error:
        manifest.update(status="failed", error=str(error))
        raise
    finally:
        diff = "".join("".join(difflib.unified_diff(before.splitlines(True), path.read_text().splitlines(True),
                       fromfile=str(path.relative_to(work)), tofile=str(path.relative_to(work))))
                       for path, before in original.items())
        (artifacts / "rollback-wiring-modules.diff").write_text(diff)
        (artifacts / "rollback.json").write_text(json.dumps(manifest, indent=2) + "\n")
    print(artifacts / "rollback.json")


if __name__ == "__main__":
    main()
