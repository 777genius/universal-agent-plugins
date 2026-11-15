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


def prepare_writer_lock(config):
    """Prepare the permanent UAP writer lock before freezing fixture originals."""
    parent = os.open(config.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        original = os.fstat(parent)
        name = config.name + ".agentplugins.lock"
        lock = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=parent)
        try:
            held = os.fstat(lock)
            named = os.stat(name, dir_fd=parent, follow_symlinks=False)
            facts = lambda value: (value.st_dev, value.st_ino, value.st_mode, value.st_uid,
                                   value.st_gid, value.st_nlink, value.st_size)
            if not stat.S_ISREG(held.st_mode) or stat.S_IMODE(held.st_mode) != 0o600 or \
                    (held.st_uid, held.st_gid, held.st_nlink, held.st_size) != (os.getuid(), os.getgid(), 1, 0) or facts(held) != facts(named):
                raise ValueError("prepared UAP writer lock has unsafe metadata or identity")
            current = config.parent.lstat()
            if (original.st_dev, original.st_ino, original.st_mode, original.st_uid, original.st_gid) != \
                    (current.st_dev, current.st_ino, current.st_mode, current.st_uid, current.st_gid):
                raise ValueError("prepared UAP writer lock parent changed")
        finally:
            os.close(lock)
    finally:
        os.close(parent)


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


def fixture_delta(before, after, home, project):
    # The fixture is disposable, but never expose native-provided filenames or
    # content in failure artifacts. Stable relative-name hashes aid diagnosis.
    changes = []
    total = 0
    for role, root in (("home", home), ("project", project)):
        previous, current = before[str(root)], after[str(root)]
        for relative in sorted(previous.keys() | current.keys()):
            old, new = previous.get(relative), current.get(relative)
            if old == new:
                continue
            total += 1
            if len(changes) >= 16:
                continue
            changes.append(dict(role=role,
                relative_sha256=hashlib.sha256(relative.encode("utf-8")).hexdigest(),
                known_uap_config_lock=(role == "home" and relative == ".grok/config.toml.agentplugins.lock"),
                effect="added" if old is None else "removed" if new is None else "changed",
                inode_changed=(old is not None and new is not None and old["inode"] != new["inode"]),
                mode_changed=(old is not None and new is not None and old["mode"] != new["mode"]),
                content_changed=(old is not None and new is not None and old["digest"] != new["digest"])))
    return dict(count=total, changes=changes, truncated=total > len(changes))


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
            prepare_writer_lock(config)
            before = {str(path): snapshot(path) for path in (home, project)}
            original_binary = hashlib.sha256(binary.read_bytes()).hexdigest()
            output = run("readonly-production-inventory", [args.test.resolve(),
                "-test.run=^TestEnablementReadonlyNativeInventory$", "-test.count=1", "-test.v", "-test.timeout=40s",
                "-grok-readonly-home=" + str(home), "-grok-readonly-project=" + str(project),
                "-grok-readonly-native=" + str(binary), "-grok-readonly-path=" + str(plugin),
                "-grok-readonly-name=uap-inert-readonly", "-grok-readonly-version=1.0.0"])
            if b"GROK_READONLY_NATIVE_INVENTORY_PASS\n" not in output:
                raise ValueError("selected native test did not complete")
            after = {str(path): snapshot(path) for path in (home, project)}
            if before != after:
                evidence["fixture_delta"] = fixture_delta(before, after, home, project)
                raise ValueError("readonly native inventory changed the original fixture")
            if original_binary != hashlib.sha256(binary.read_bytes()).hexdigest():
                raise ValueError("native executable changed")
        # Qualification is committed only after the disposable fixture is cleaned up.
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
