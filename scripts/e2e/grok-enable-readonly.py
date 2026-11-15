#!/usr/bin/env python3
"""Pinned, inert Grok metadata gate; no sessions, authentication or model calls."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import urllib.request

ARCHIVE = "11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc"
BINARY = "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9"
URL = "https://x.ai/cli/grok-1.0.70-linux-x86_64.gz"


def producer_context():
    """Read the delegated Python launcher, not the Go or namespaced child."""
    result = dict(qualification=False, producer="delegated Python launcher before Go helper exec; not Go/bwrap child state",
        pid=os.getpid(), uid=os.getuid(), gid=os.getgid())
    for name in ("attr/current", "uid_map", "gid_map", "status"):
        try:
            with open("/proc/self/" + name, "rb") as source:
                body = source.read(16385)
            if len(body) > 16384:
                raise ValueError("fixed proc metadata exceeds 16384 bytes")
            text = body.decode("utf-8", "backslashreplace")
            if name == "status":
                fields = ("Uid:", "Gid:", "Groups:", "CapInh:", "CapPrm:", "CapEff:", "CapBnd:", "CapAmb:", "NoNewPrivs:", "Seccomp:")
                text = "\n".join(line for line in text.splitlines() if line.startswith(fields))
            result[name] = dict(available=True, value=text)
        except (OSError, ValueError) as error:
            result[name] = dict(available=False, error=str(error)[:256])
    for name in ("user", "net", "mnt", "pid"):
        try:
            result["ns/" + name] = dict(available=True, value=os.readlink("/proc/self/ns/" + name)[:256])
        except OSError as error:
            result["ns/" + name] = dict(available=False, error=str(error)[:256])
    return result


def snapshot(root):
    result = {}
    for path in [root, *sorted(root.rglob("*"))]:
        info = path.lstat()
        if not stat.S_ISREG(info.st_mode) and not stat.S_ISDIR(info.st_mode):
            raise ValueError("inert fixture acquired an unexpected filesystem kind")
        result[str(path.relative_to(root))] = dict(mode=info.st_mode, inode=info.st_ino,
            digest=hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None)
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--test", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    try:
        (args.output / "producer-context.json").write_text(json.dumps(producer_context(), indent=2) + "\n")
    except OSError as error:
        print("Nonqualifying producer context unavailable: " + str(error)[:256])
    evidence = dict(passed=False, commands=[], model_invoked=False, authentication_used=False)
    status = 1
    try:
        with urllib.request.urlopen(URL, timeout=60) as response:
            archive = response.read((96 << 20) + 1)
        if len(archive) > 96 << 20 or hashlib.sha256(archive).hexdigest() != ARCHIVE:
            raise ValueError("official pinned Grok archive differs")
        with tempfile.TemporaryDirectory(prefix="grok-readonly-", dir=args.output) as temporary:
            root = Path(temporary).resolve()
            binary = root / "grok"
            # The measured gzip has one fixed expanded executable, not archive paths.
            import io
            with gzip.GzipFile(fileobj=io.BytesIO(archive)) as compressed:
                body = compressed.read((256 << 20) + 1)
            if len(body) > 256 << 20 or hashlib.sha256(body).hexdigest() != BINARY:
                raise ValueError("official pinned Grok executable differs")
            binary.write_bytes(body)
            binary.chmod(0o700)
            home, project = root / "home", root / "project"
            home.mkdir(mode=0o700)
            project.mkdir(mode=0o700)
            env = dict(HOME=str(home), PATH="/usr/bin:/bin", LANG="C.UTF-8", TERM="dumb",
                NO_COLOR="1", GROK_DISABLE_AUTOUPDATER="1", GIT_CONFIG_NOSYSTEM="1",
                GIT_CONFIG_GLOBAL="/dev/null", GIT_TERMINAL_PROMPT="0")

            def run(label, argv):
                result = subprocess.run(list(map(str, argv)), cwd=project, env=env,
                    capture_output=True, timeout=45, check=False)
                if len(result.stdout) + len(result.stderr) > 1 << 20:
                    raise ValueError("native fixture output exceeds its bound")
                record = dict(label=label, argv=list(map(str, argv)), exit=result.returncode,
                    stdout=result.stdout.decode("utf-8", "strict"), stderr=result.stderr.decode("utf-8", "strict"))
                evidence["commands"].append(record)
                if result.returncode:
                    raise ValueError("native gate failed: " + label)
                return result.stdout

            if run("version", [binary, "--version"]) != b"grok 1.0.70 (f50904eaee6c)\n":
                raise ValueError("native version differs")
            run("git", ["/usr/bin/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"])
            plugin = project / ".grok/plugins/uap-inert-readonly"
            (plugin / ".grok-plugin").mkdir(parents=True)
            (plugin / ".grok-plugin/plugin.json").write_text(json.dumps(dict(name="uap-inert-readonly",
                version="1.0.0", description="Inert inventory fixture without executable components")))
            config = home / ".grok/config.toml"
            config.parent.mkdir(mode=0o700, exist_ok=True)
            # The native config writer creates its permanent lock. This command
            # disables an already disabled inert entry; it never starts an MCP process.
            config.write_text('[mcp_servers.uap_inert_lock]\ncommand = "/usr/bin/false"\nenabled = false\n')
            run("native-lock", [binary, "mcp", "disable", "uap_inert_lock"])
            lock = config.parent / ".config-init.lock"
            if not lock.is_file() or lock.read_bytes():
                raise ValueError("native config lock was not established")
            config.write_text('[marketplace]\ndefault_skills_installs_purged = true\n'
                '[features]\nturn_summary = false\n')
            before = {str(path): snapshot(path) for path in (home, project)}
            original_binary = hashlib.sha256(binary.read_bytes()).hexdigest()
            output = run("readonly-production-inventory", [args.test.resolve(),
                "-test.run=^TestEnablementReadonlyNativeInventory$", "-test.count=1", "-test.v", "-test.timeout=40s",
                "-grok-readonly-home=" + str(home), "-grok-readonly-project=" + str(project),
                "-grok-readonly-native=" + str(binary), "-grok-readonly-path=" + str(plugin),
                "-grok-readonly-name=uap-inert-readonly", "-grok-readonly-version=1.0.0"])
            if b"GROK_READONLY_NATIVE_INVENTORY_PASS\n" not in output:
                raise ValueError("selected native test did not complete")
            if before != {str(path): snapshot(path) for path in (home, project)}:
                raise ValueError("readonly native inventory changed the original fixture")
            if original_binary != hashlib.sha256(binary.read_bytes()).hexdigest():
                raise ValueError("native executable changed")
            evidence.update(passed=True, fixture_unchanged=True, archive_sha256=ARCHIVE, binary_sha256=BINARY)
            status = 0
    except Exception as error:
        evidence["error"] = type(error).__name__ + ": " + str(error)
    finally:
        (args.output / "evidence.json").write_text(json.dumps(evidence, indent=2) + "\n")
        commands = []
        for command in evidence["commands"]:
            record = {key: command[key] for key in ("label", "exit")}
            for name in ("stdout", "stderr"):
                record[name + "_tail"] = command[name].encode("utf-8")[-4096:].decode("utf-8", errors="backslashreplace")
            commands.append(record)
        print(json.dumps(dict(evidence, output=str(args.output), commands=commands), ensure_ascii=True))
    return status


if __name__ == "__main__":
    raise SystemExit(main())
