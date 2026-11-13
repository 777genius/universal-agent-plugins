"""Exact task-owned Grok 1.0.70 ACP bootstrap; no runtime/trust qualification."""
import hashlib
import re
import stat

# Identical publisher-created bytes in original artifacts 14033028378/14035822654.
# These are literal admission pins, never values learned from the current run.
PINNED = {
    ".config-init.lock": (0, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", 0o644),
    ".metadata_version": (6, "f56579b5dab68a821e662fbe9c4d132a68422f1e79f6fc88cba3116017e80980", 0o644),
    "config.toml": (52, "fe7b6bc7eb3f5321649cb10fd4ce35096cee471f9c8d04fd34958e848039ff92", 0o644),
    "docs/user-guide/01-getting-started.md": (10495, "9c022943cfce0873165bd606a520e216b48a98decb1a974ca6cc6f30f0b32efd", 0o644),
    "docs/user-guide/02-authentication.md": (16236, "04630e3d30bb74c0a7b1c3df3e50fe42e4285d7aeeacc4ecd7573cfbe34852bb", 0o644),
    "docs/user-guide/03-keyboard-shortcuts.md": (37541, "47465c24978c1c20db37f9d14c1a2d40a89cb9727ff6171a826e0e0cc3d60f8d", 0o644),
    "docs/user-guide/04-slash-commands.md": (44492, "4dfa08dc2bcf8df73cdd2cf87118468495704f00631fec8a9dab7b95728bbc49", 0o644),
    "docs/user-guide/05-configuration.md": (58753, "ce49c8f08db8c05cb2bc9f040d0955573fbe6b86d5af4381ff2547197a66d433", 0o644),
    "docs/user-guide/06-theming.md": (18875, "fc914d1a0f4e9dfae661cbb47b22e91755b9a24bd3ec362d0a7b977340b6ff4b", 0o644),
    "docs/user-guide/07-mcp-servers.md": (26255, "e9aac5bbf39f16544369e95708711015025ecca14e3517c174c3a910b1404347", 0o644),
    "docs/user-guide/08-skills.md": (13886, "4afcd002489982e069e02dbb0d07fb490216035707fab208b36c434fe6852f1c", 0o644),
    "docs/user-guide/09-plugins.md": (28885, "29c1abc4d92a4450a13dfa74ceefc35409c884eb10751d90eb52aafe5780de40", 0o644),
    "docs/user-guide/10-hooks.md": (103892, "7a0138b61ced3decb5fdc99c9c82cbff694109ab77791fda5bceafddeab7ba5f", 0o644),
    "docs/user-guide/11-custom-models.md": (19362, "4a0fe03334e91c1a472cf985d490ca73810e9738a171f5e0a5bf615552a6ca9a", 0o644),
    "docs/user-guide/12-project-rules.md": (10890, "5bc157f20c2548362df49d2d704d4f4b2c568db2adc8a437680ab02064537f53", 0o644),
    "docs/user-guide/13-memory.md": (14019, "51c764bfcfef4c4f79ad6dc4a270e057a3726cae237109b950083bf7007d7831", 0o644),
    "docs/user-guide/14-headless-mode.md": (55494, "216709e31ae67ba2811194bb183827e08fa453dde8fa91826c036ffcc7365d83", 0o644),
    "docs/user-guide/15-agent-mode.md": (14507, "b4c4b840392a9b85a356b433b729ea02a89bf2742aa74894bbec56ff369c8fed", 0o644),
    "docs/user-guide/16-subagents.md": (23119, "6556ef4eab94ff5dc01343feedacf525334b23131d626b38b4cd752eeb6c8c78", 0o644),
    "docs/user-guide/17-sessions.md": (36072, "f11b570bdbf5a1b11036202af0b961c414e387f71bd3091f7c293161bc2d5589", 0o644),
    "docs/user-guide/18-sandbox.md": (27219, "82f3ca41b20a4b393cfff26a138d02464590dbff833f1817cc569520a4154181", 0o644),
    "docs/user-guide/19-plan-mode.md": (14010, "b8c557491c640891b423c497546fe46a7286819f34f27bb4448195df2fd9b61f", 0o644),
    "docs/user-guide/20-background-tasks.md": (10777, "0589ac332ab67724ffe85c9f0caf4e0e4c0b91789a4eeac19aabd561077a7a46", 0o644),
    "docs/user-guide/21-terminal-support.md": (15195, "6b6d0eefbeebe6c3edab9ffcb71405723411609d654538eb6aeb6f6e65658fbf", 0o644),
    "docs/user-guide/22-permissions-and-safety.md": (41205, "91e1beae240449f35d594b73e3e3914f516d121d20d838f42c67c768b2ebdbde", 0o644),
    "docs/user-guide/23-dashboard.md": (18968, "1463ea60102e8561d0b3a2d792c167834493a93233df7c0384855eb117929cfa", 0o644),
    "docs/user-guide/24-monitoring-usage.md": (22276, "331245078267fb8072df695815ce6fa23d184733be62b42a08fc5843f157783f", 0o644),
    "docs/user-guide/25-status-line.md": (18204, "d4d5c64ea02163171425176906f52ce36cf0154a6324608fe0179490bb4852c2", 0o644),
    "docs/user-guide/26-config-reference.md": (88675, "ebd97fec98bb00bfdfa37befddf31dd8a14e3f277db887c7b643888867865db7", 0o644),
    "docs/user-guide/27-grok-clone.md": (10752, "cb557fe0d0a82cff73daad173c113727e9ed050159e3008a13bddafd699755ce", 0o644),
    "installed-plugins/registry.lock": (0, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", 0o644),
    "managed_config.lock": (0, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", 0o644),
    "README.md": (113239, "83acda1d85f856b38acbc475d784754b109ca5eb80958e7fe6a8f935a5e1f2ff", 0o644),
}
DIRECTORIES = {"docs": 0o755, "docs/user-guide": 0o755, "logs": 0o755,
               "installed-plugins": 0o755, "sessions": 0o700}
MUTABLE = {"logs/unified.jsonl": 128 << 10, "sessions/session_search.sqlite": 256 << 10,
           "sessions/session_search.sqlite-wal": 128 << 10, "sessions/session_search.sqlite-shm": 32768}
NAMES = frozenset(PINNED) | frozenset(DIRECTORIES) | frozenset(MUTABLE) | {"agent_id"}
PREFIX = "home/.grok/"
PROFILE_ROOTS = {"user_profile": "home/.grok", "a": "a", "b": "b"}


def require(value, reason):
    if not value:
        raise ValueError(reason)


def record(value, count):
    require(type(value) in (tuple, list) and len(value) == count
            and all(type(item) is int for item in value), "bootstrap metadata schema")
    return tuple(value)


def matches_snapshot(value, meta):
    if len(meta) == 5:
        require(set(value) == {"kind", "mode", "device", "inode"} and value["kind"] == "directory",
                "bootstrap directory snapshot")
    else:
        require(set(value) == {"sha256", "mode", "bytes", "device", "inode"}
                and type(value["sha256"]) is str and re.fullmatch("[0-9a-f]{64}", value["sha256"])
                and type(value["bytes"]) is int and value["bytes"] == meta[6], "bootstrap file snapshot")
    require(all(type(value[key]) is int and value[key] == expected for key, expected in
                (("device", meta[0]), ("inode", meta[1]), ("mode", stat.S_IMODE(meta[2])))),
            "bootstrap snapshot identity")


class NativeBootstrap:
    def __init__(self):
        self.created = None
        self.uuid_hash = None

    def metadata(self, before, after):
        require(type(before) is dict and type(after) is dict and len(after) <= 4096,
                "bootstrap metadata bound")
        root = record(before["."], 5)
        for path, previous in before.items():
            require(path in after, "original member missing")
            if path == "request.json":
                old, new = record(previous, 9), record(after[path], 9)
                require(old[:6] == new[:6] and 0 <= new[6] <= 4096, "control member changed")
            else:
                require(after[path] == previous, "original member changed")
        added = set(after) - set(before)
        require(added == {PREFIX + name for name in NAMES}, "unknown or missing native bootstrap")
        for name in NAMES:
            meta = record(after[PREFIX + name], 5 if name in DIRECTORIES else 9)
            require(meta[0] == root[0], "bootstrap device differs")
            if name in DIRECTORIES:
                require(stat.S_ISDIR(meta[2]) and stat.S_IMODE(meta[2]) == DIRECTORIES[name]
                        and meta[3:] == root[3:], "bootstrap directory authority")
            else:
                mode = 0o600 if name == "agent_id" else 0o644
                require(stat.S_ISREG(meta[2]) and stat.S_IMODE(meta[2]) == mode
                        and meta[3] == 1 and meta[4:6] == root[3:], "bootstrap file authority")
                if name in PINNED:
                    require(meta[6] == PINNED[name][0], "bootstrap pinned size")
                elif name == "agent_id":
                    require(meta[6] == 36, "bootstrap UUID size")
                else:
                    require(0 <= meta[6] <= MUTABLE[name]
                            and (not name.endswith("-shm") or meta[6] == 32768), "bootstrap cache bound")
            if self.created is not None:
                original = self.created[name]
                count = 6 if name in MUTABLE else 8 if name == "agent_id" else len(meta)
                require(meta[:count] == original[:count], "bootstrap creation identity changed")
        total = 0
        for path, value in after.items():
            if any(path.startswith(prefix + "/") for prefix in PROFILE_ROOTS.values()) and len(value) == 9:
                meta = record(value, 9)
                require(0 <= meta[6] <= 1 << 20, "profile file bound")
                total += meta[6]
        require(total <= 16 << 20, "profile aggregate bound")

    def admit(self, before, after, original_profiles, current_profiles, uuid_body):
        self.metadata(before, after)
        require(type(current_profiles) is dict and set(current_profiles) == set(PROFILE_ROOTS),
                "profile snapshot sources")
        for scope in ("a", "b"):
            require(current_profiles[scope] == original_profiles[scope], "original project changed")
        original, current = original_profiles["user_profile"], current_profiles["user_profile"]
        require(set(current) == set(original) | set(NAMES)
                and all(current.get(path) == value for path, value in original.items()), "original User changed")
        for scope, prefix in PROFILE_ROOTS.items():
            for path, value in current_profiles[scope].items():
                key = prefix if path == "." else prefix + "/" + path
                require(type(value) is dict and key in after, "snapshot member missing")
                matches_snapshot(value, after[key])
        for name, (size, digest, mode) in PINNED.items():
            value = current[name]
            require((value["bytes"], value["sha256"], value["mode"]) == (size, digest, mode),
                    "bootstrap pinned content")
        require(type(uuid_body) is bytes and re.fullmatch(
            rb"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", uuid_body),
            "bootstrap UUID grammar")
        uuid_hash = hashlib.sha256(uuid_body).hexdigest()
        require(current["agent_id"]["sha256"] == uuid_hash
                and (self.uuid_hash is None or uuid_hash == self.uuid_hash), "bootstrap UUID changed")
        if self.created is None:
            self.created = {name: tuple(after[PREFIX + name]) for name in NAMES}
            self.uuid_hash = uuid_hash
        return sorted(NAMES)
