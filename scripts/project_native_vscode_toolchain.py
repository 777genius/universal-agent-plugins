"""Prepare the pinned, sealed-input TypeScript tools for trusted VS Code research.

No editor, browser or agent is launched here. The existing strict controller
admits the returned complete runtime closure before any native observation.
"""
import hashlib
import json
import os
from pathlib import Path
import stat
import urllib.request

import project_native_research as shared
from native_client_research import bounded_read

PUBLISHER = "e646868227c3e3a9e3f24277c8c7220be42c0be7"
PACKAGES = {
    "typescript": ("5.9.3", "sha512-jl1vZzPDinLr9eUt3J/t7V6FgNEw9QjvBPdysz9KfQDD41fQrC2Y4vKQdiaUpFT4bXlb1RHhLpp8wtm6M5TgSw=="),
    "@types/node": ("22.20.1", "sha512-EANqOCF9QFyra+4pfxUcX9STKJpCLjMbObVzljIJomAWSnuSIEAvyzEU53GaajbXJEgdh0iEcPL+DGvpUd4k1Q=="),
    "undici-types": ("6.21.0", "sha512-iwDZqg0QAGrg9Rav5H4n0M64c3mkR59cJ6wQp+7C4nI0gsmExaedaYLNO44eT4AtBBwjbTiGPMlt2Md0T9H9JQ=="),
    "playwright-core": ("1.64.1", "sha512-BC4s8y6SEVA6Q1rSxZTto8hI1KUI3xXlHGZgvGqP4IqPdI/a6FX9H29IDNz7vKo9l21b6LfYosScPL6M3xxyDQ=="),
}
DECLARATIONS = {
    "vscode.d.ts": (742500, "bd520cf7bb88454e99bd15da13c6b9c6ac56fd65"),
    "vscode.proposed.chatPromptFiles.d.ts": (13619, "eb77adbd1a1084837ab2d7b8f669f8ecfc20f1b8"),
    "vscode.proposed.mcpServerDefinitions.d.ts": (3302, "81e75fe69d74e82bdfb4ca69d0e89b801e728118"),
}
INPUTS = {"extension.ts": "observer/extension.ts", "runner.ts": "observer/runner.ts",
          "driver.ts": "driver/driver.ts", "package.json": "observer/package.json",
          "driver-package.json": "driver/package.json"}
RUNTIME_FILES = ("observer/extension.js", "observer/runner.js", "driver/driver.js")


def ordinary_bytes(path, limit):
    path = Path(path)
    if not path.is_absolute() or path.resolve() != path:
        raise ValueError("toolchain input alias refused")
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or not 0 <= before.st_size <= limit:
        raise ValueError("toolchain input must be a bounded regular single-link file")
    descriptor = shared.artifacts.open_member(path)
    try:
        held = os.fstat(descriptor)
        if not shared.artifacts.same_opened(before, held):
            raise ValueError("toolchain input changed while opening")
        with os.fdopen(os.dup(descriptor), "rb") as stream:
            body = bounded_read(stream, limit)
        if shared.artifacts.identity(os.fstat(descriptor)) != shared.artifacts.identity(held):
            raise ValueError("toolchain input changed during read")
    finally:
        os.close(descriptor)
    if len(body) != before.st_size or shared.artifacts.identity(path.lstat()) != shared.artifacts.identity(before):
        raise ValueError("toolchain input path changed during read")
    return body


def write_new(path, body, executable=False):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    with path.open("xb") as output:
        output.write(body)
    path.chmod(0o700 if executable else 0o600)


def package_lock():
    dependencies = {name: version for name, (version, _) in PACKAGES.items() if name != "undici-types"}
    manifest = {"name": "uap-vscode-research-compiler", "version": "1.0.0", "private": True,
                "type": "commonjs", "devDependencies": dependencies}
    packages = {"": {"name": manifest["name"], "version": "1.0.0", "devDependencies": dependencies}}
    for name, (version, integrity) in PACKAGES.items():
        leaf = name.split("/")[-1]
        item = {"version": version, "resolved": "https://registry.npmjs.org/" + name + "/-/" + leaf + "-" + version + ".tgz",
                "integrity": integrity, "dev": True}
        if name == "@types/node":
            item["dependencies"] = {"undici-types": "~6.21.0"}
        if name == "typescript":
            item["bin"] = {"tsc": "bin/tsc", "tsserver": "bin/tsserver"}
        if name == "playwright-core":
            item["bin"] = {"playwright-core": "cli.js"}
        packages["node_modules/" + name] = item
    return manifest, {"name": manifest["name"], "version": "1.0.0", "lockfileVersion": 3,
                      "requires": True, "packages": packages}


def check_declaration(name, body):
    size, blob = DECLARATIONS[name]
    actual = hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()
    if len(body) != size or actual != blob:
        raise ValueError("publisher TypeScript declaration differs from pinned source")
    return {"name": name, "bytes": size, "git_blob": blob, "sha256": hashlib.sha256(body).hexdigest()}


def declarations(root, budget):
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *_args, **_kwargs):
            raise ValueError("publisher source redirect refused")

    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    receipts = []
    for name, (size, _) in DECLARATIONS.items():
        url = "https://raw.githubusercontent.com/microsoft/vscode/" + PUBLISHER + "/src/vscode-dts/" + name
        with opener.open(url, timeout=min(15, budget.check())) as response:
            body = bounded_read(shared.BudgetReader(response, budget), size)
        receipts.append(check_declaration(name, body))
        write_new(root / name, body)
    return receipts


def runtime_tree(source, destination, budget):
    """Copy the entire installed Playwright package as non-executable regular files."""
    if source.resolve() != source or not source.is_dir() or destination.exists():
        raise ValueError("ordinary fresh Playwright closure required")
    count, total = 0, 0
    for base, directories, names in os.walk(source, followlinks=False):
        budget.check()
        current = Path(base)
        for name in directories:
            path = current / name
            info = path.lstat()
            if not stat.S_ISDIR(info.st_mode) or name in (".bin", "node_modules") or len(path.relative_to(source).parts) > 32:
                raise ValueError("Playwright closure contains an alias or nested dependency")
        for name in names:
            count += 1
            if count > 4000:
                raise ValueError("Playwright closure member count exceeded")
            path = current / name
            relative = path.relative_to(source)
            if any(part in (".bin", "node_modules") for part in relative.parts):
                raise ValueError("Playwright closure contains an unreviewed dependency")
            body = ordinary_bytes(path, 64 << 20)
            total += len(body)
            if total > 128 << 20:
                raise ValueError("Playwright closure byte limit exceeded")
            write_new(destination / relative, body)
    package = json.loads(ordinary_bytes(destination / "package.json", 128 << 10))
    if package.get("name") != "playwright-core" or package.get("version") != PACKAGES["playwright-core"][0]:
        raise ValueError("Playwright runtime version differs")
    return count, total


def runtime_manifest(root, budget):
    rows, total = [], 0
    for base, directories, names in os.walk(root, followlinks=False):
        budget.check()
        for name in directories:
            if not stat.S_ISDIR((Path(base) / name).lstat().st_mode):
                raise ValueError("runtime directory alias refused")
        for name in names:
            path = Path(base) / name
            relative = path.relative_to(root).as_posix()
            allowed = relative in (*RUNTIME_FILES, "observer/package.json", "driver/package.json", "node")
            if not allowed and not relative.startswith("driver/node_modules/playwright-core/"):
                raise ValueError("unregistered compiler runtime member")
            body = ordinary_bytes(path, 256 << 20)
            total += len(body)
            if len(rows) >= 4096 or total > 384 << 20:
                raise ValueError("compiler runtime closure exceeds bound")
            rows.append({"path": relative, "bytes": len(body), "sha256": hashlib.sha256(body).hexdigest(),
                         "executable": relative == "node"})
    required = {*RUNTIME_FILES, "observer/package.json", "driver/package.json", "node", "driver/node_modules/playwright-core/package.json"}
    if not required <= {row["path"] for row in rows}:
        raise ValueError("compiler runtime closure is incomplete")
    return {"schema_version": 1, "node_version": "24.21.0", "typescript_version": "5.9.3",
            "publisher_commit": PUBLISHER, "files": sorted(rows, key=lambda item: item["path"])}


def prepare(root, sources, budget, capture):
    """capture(label, argv, cwd, env, seconds) returns bounded stdout/stderr bytes."""
    root, sources = Path(root), Path(sources)
    if root.exists() or root.resolve() != root or sources.resolve() != sources:
        raise ValueError("fresh canonical compiler root and source directory required")
    root.mkdir(mode=0o700)
    receipt, contents = shared.acquire(shared.NODE, budget, node=True)
    prefix = "node-v24.21.0-linux-x64/"
    node_root = root / "node-dist"
    identities = {}
    inventory = {row["name"]: row for row in receipt["members"]}
    selected = {name: body for name, body in contents.items()
                if name == shared.NODE["entrypoint"] or name.startswith(prefix + "lib/node_modules/npm/")}
    if (any(row["kind"] not in ("file", "directory") and row["name"].startswith(prefix + "lib/node_modules/npm/") for row in receipt["members"])
            or shared.NODE["entrypoint"] not in selected or prefix + "lib/node_modules/npm/bin/npm-cli.js" not in selected):
        raise ValueError("exact Node/npm regular runtime closure absent")
    for name, body in selected.items():
        path = node_root / name.removeprefix(prefix)
        write_new(path, body, executable=name == shared.NODE["entrypoint"])
        identities[path] = inventory[name]["sha256"]
    contents.clear()
    node, npm = node_root / "bin/node", node_root / "lib/node_modules/npm/bin/npm-cli.js"
    home, temp = root / "home", root / "tmp"
    home.mkdir(mode=0o700)
    temp.mkdir(mode=0o700)
    env = {"HOME": str(home), "TMPDIR": str(temp), "PATH": str(node.parent) + ":/usr/bin:/bin", "LANG": "C.UTF-8",
           "NPM_CONFIG_USERCONFIG": "/dev/null", "NPM_CONFIG_GLOBALCONFIG": "/dev/null",
           "NPM_CONFIG_CACHE": str(root / "npm-cache"), "NPM_CONFIG_UPDATE_NOTIFIER": "false",
           "PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD": "1"}
    shared.verify_files(identities, budget)
    stdout, stderr = capture("trusted-node-version", [node, "--version"], root, env, 10)
    if stdout != b"v24.21.0\n" or stderr:
        raise ValueError("pinned compiler Node version differs")
    manifest, lock = package_lock()
    for name, value in (("package.json", manifest), ("package-lock.json", lock)):
        write_new(root / name, json.dumps(value, sort_keys=True, indent=2).encode() + b"\n")
    capture("trusted-npm-ci", [node, npm, "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--registry=https://registry.npmjs.org"], root, env, 120)
    shared.verify_files(identities, budget)
    for name, (version, _) in PACKAGES.items():
        package = json.loads(ordinary_bytes(root / "node_modules" / name / "package.json", 128 << 10))
        if package.get("name") != name or package.get("version") != version:
            raise ValueError("compiler package differs from the exact lock")
    source_hashes = {}
    for name, target in INPUTS.items():
        body = ordinary_bytes(sources / name, 256 << 10)
        write_new(root / "build" / target, body)
        source_hashes[name] = hashlib.sha256(body).hexdigest()
    declaration_receipts = declarations(root / "declarations", budget)
    config = {"compilerOptions": {"target": "ES2023", "module": "NodeNext", "moduleResolution": "NodeNext",
                                   "lib": ["ES2023"], "strict": True, "skipLibCheck": True, "esModuleInterop": True,
                                   "noEmitOnError": True, "types": ["node"], "sourceMap": False, "declaration": False,
                                   "rootDir": "build", "outDir": "emitted"},
              "include": ["build/**/*.ts", "declarations/*.d.ts"]}
    write_new(root / "tsconfig.json", json.dumps(config, sort_keys=True).encode() + b"\n")
    capture("trusted-tsc", [node, root / "node_modules/typescript/bin/tsc", "--project", root / "tsconfig.json"], root, env, 60)
    shared.verify_files(identities, budget)
    for name, expected in source_hashes.items():
        if hashlib.sha256(ordinary_bytes(sources / name, 256 << 10)).hexdigest() != expected:
            raise ValueError("observer source changed during compilation")
    tools = root / "tools"
    tools.mkdir(mode=0o700)
    write_new(tools / "node", ordinary_bytes(node, 256 << 20), executable=True)
    for name in RUNTIME_FILES:
        write_new(tools / name, ordinary_bytes(root / "emitted" / name, 1 << 20))
    for source, target in (("package.json", "observer/package.json"), ("driver-package.json", "driver/package.json")):
        body = ordinary_bytes(root / "build" / target, 16 << 10)
        if hashlib.sha256(body).hexdigest() != source_hashes[source]:
            raise ValueError("staged runtime package manifest changed")
        write_new(tools / target, body)
    count, size = runtime_tree(root / "node_modules/playwright-core", tools / "driver/node_modules/playwright-core", budget)
    runtime = runtime_manifest(tools, budget)
    body = json.dumps(runtime, sort_keys=True, indent=2).encode() + b"\n"
    if len(body) > 1 << 20:
        raise ValueError("trusted runtime manifest exceeds bound")
    write_new(tools / "manifest.json", body)
    return tools, {"node": shared.selected_node_receipt(receipt), "packages": {name: {"version": version, "integrity": integrity} for name, (version, integrity) in PACKAGES.items()},
                   "declarations": declaration_receipts, "source_sha256": source_hashes,
                   "playwright_runtime_files": count, "playwright_runtime_bytes": size,
                   "tools_manifest_sha256": hashlib.sha256(body).hexdigest(), "runtime_file_count": len(runtime["files"])}
