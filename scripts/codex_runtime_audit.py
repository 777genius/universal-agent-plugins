"""Strict native Codex Skill evidence with independently checked SQLite state.

Codex 0.153.4's supported sqlite_home separates runtime databases from native
registration files. No native-profile file is excluded from the caller's raw
snapshot. SQLite accepts a changed WAL representation only when integrity,
schema and every table's contents still match the pre-install native baseline.
"""

import asyncio
import hashlib
import json
import os
from pathlib import Path
import re
from runpy import run_path
import signal
import sqlite3
import stat
import subprocess
import sys
import tempfile
import time
import tomllib

_FILES = run_path(str(Path(__file__).with_name("native_client_project_artifacts.py")))
FILE_LIMIT = 32 << 20
TOTAL_LIMIT = 64 << 20
ROW_LIMIT = 20000
TEMPORARY_GIT_HEAD = b"ref: refs/heads/main\n"
TEMPORARY_GIT_LIMIT = 64
DATABASES = frozenset(("state_5.sqlite", "logs_2.sqlite", "goals_1.sqlite",
                      "memories_1.sqlite", "queue_1.sqlite", "thread_history_1.sqlite"))


def _sqlite_config(runtime):
    # Preserve Unicode scalar values; TOML forbids unescaped DEL.
    pin = json.dumps(str(runtime), ensure_ascii=False).replace("\x7f", "\\u007f")
    return ("sqlite_home = " + pin + "\n").encode("utf-8")


def _regular_bytes(path, limit):
    before = path.lstat()
    regular, canonical = stat.S_ISREG(before.st_mode), None
    reason = ("nonregular" if not regular else "multiple-links" if before.st_nlink != 1
              else "out-of-bound" if not 0 <= before.st_size <= limit else None)
    if reason is None:
        canonical = path.resolve(strict=True) == path
        if not canonical:
            reason = "noncanonical"
    if reason is not None:
        facts = {"reason": reason, "size": before.st_size, "bound": limit,
                 "nlink": before.st_nlink, "regular": regular, "canonical": canonical}
        raise ValueError("Codex audit file is not bounded, regular and exclusively linked; "
                         + json.dumps(facts, sort_keys=True))
    descriptor = _FILES["open_member"](path)
    try:
        held = os.fstat(descriptor)
        if not _FILES["same_opened"](before, held):
            raise ValueError("Codex audit opened file identity changed")
        chunks, remaining = [], before.st_size + 1
        while remaining:
            block = os.read(descriptor, min(remaining, 65536))
            if not block:
                break
            chunks.append(block)
            remaining -= len(block)
        body = b"".join(chunks)
        if (len(body) != before.st_size
                or _FILES["identity"](held) != _FILES["identity"](os.fstat(descriptor))):
            raise ValueError("Codex audit bytes changed during the full read")
    finally:
        os.close(descriptor)
    if _FILES["identity"](before) != _FILES["identity"](path.lstat()):
        raise ValueError("Codex audit file changed after the full read")
    return body


def _real_directory(path):
    info = path.lstat()
    if (not stat.S_ISDIR(info.st_mode) or path.is_symlink()
            or getattr(info, "st_file_attributes", 0) & 0x400
            or path.resolve(strict=True) != path):
        raise ValueError("Codex audit directory is redirected")
    return info


def _bounded_member_names(path, limit):
    before = _FILES["identity"](_real_directory(path))
    names = []
    with os.scandir(path) as entries:
        for entry in entries:
            names.append(entry.name)
            if len(names) > limit:
                raise ValueError("Codex temporary Git member count exceeds its bound")
    if _FILES["identity"](_real_directory(path)) != before:
        raise ValueError("Codex temporary Git directory changed during enumeration")
    return sorted(names)


# Embedded bundle authority: openai/codex commit 3d2ee51ca2d5db578f328aa75e20aa22c0197c9a,
# tree c527c7a5f5f199231dc0d819264e9e2180eb126a, assets/samples tree
# 8048ae6d71052c4710819c2e0759edabc651bddb. Every source byte count and
# Git blob SHA-1 was verified before SHA-256; held manifest blob
# 31e8fdb02e922f658126bd99b75fc695460f6bb5, SHA-256
# 5ed68dbc719a64c7293f66589dad18734c7c189bab5343b447d86755fe36b025.
# Marker follows pinned Rust 1.95 DefaultHasher/Hash and include_dir 0.7.4.
SYSTEM_SKILL_MARKER = b"91663ef126b94ab1\n"
SYSTEM_SKILL_FILES = {
    "imagegen/LICENSE.txt": (10776, "4dd13869245e356246a5b770723247bbb80a8f07a181d1d3d873a1734297cdb9"),
    "imagegen/SKILL.md": (19201, "681ddb4ad6d06a2acc78a3535b583f8d0c1ea800ecda3d56370d3310fd2cd4ba"),
    "imagegen/agents/openai.yaml": (275, "9ca574af14580dc7a2a3dc37a1796d17f93cb8850be66501f0799ef8603e9dc0"),
    "imagegen/assets/imagegen-small.svg": (2889, "cff5f34f57ff60b3ee92eaedd17b15e96dd4b9e776df3e78936c9e00d42be294"),
    "imagegen/assets/imagegen.png": (1711, "95952f644064eb9e890f98d8db07216347186526e4c41ad66d3420629eb86e20"),
    "imagegen/references/cli.md": (9655, "ecfc2e09261a0feb3482517a5fa0ff410cb7d1958e3cbd2ac6b61586f5b81405"),
    "imagegen/references/codex-network.md": (1779, "c88298ca4481f6116a16fa7987434fc977f8b311c1bbc0c3d862ffd0c5981148"),
    "imagegen/references/image-api.md": (6072, "dc975d7af8a4888967251a0276014b4a71ea30455294944b762256373ce3e569"),
    "imagegen/references/prompting.md": (8282, "b210b051c775860267080941eba968212bf0ac7fce581d75c5dcc217d8293f8b"),
    "imagegen/references/sample-prompts.md": (17617, "70474177d151855b175c6133de2aae1d90b7f146b0dab50ec830972c47d72183"),
    "imagegen/scripts/image_gen.py": (34667, "b4345cf835e5b593b97df6bb2b70bda493b8a97eee4cabbc99b2fa57dba9b448"),
    "imagegen/scripts/remove_chroma_key.py": (13905, "a5893c4bd04b21abced33731ea89d2ac428e81d92a0368eb18ffbe4fda5253e2"),
    "openai-docs/LICENSE.txt": (10776, "4dd13869245e356246a5b770723247bbb80a8f07a181d1d3d873a1734297cdb9"),
    "openai-docs/SKILL.md": (5446, "7cb8fa1b2a0c635b5c61ffe1da7b8594a7ea0fce5b71e8d523e2025d88b2a05e"),
    "openai-docs/agents/openai.yaml": (370, "44b9efac6be1bae32d869aa2942fecbe4dcae82682ee03e4120f2f9b7d4658ec"),
    "openai-docs/assets/openai-small.svg": (1091, "45be1f0757eb18889eefb1e7db79668ef46a275dc4e0e78e8df5ebd7f6cdeadc"),
    "openai-docs/assets/openai.png": (1429, "156cc84d7332bfe95b310350bd470b690d22aa33d65340cc6c2e06022946194c"),
    "openai-docs/references/codex-self-knowledge.md": (7417, "8c8fb00e6e5cb1977924f5164684a6095427fa828bbc765225f17d9aeb79a912"),
    "openai-docs/references/latest-model.md": (2094, "f25e351e522dd6e30e82d482f31f44c992e794b11031cdcb6ac7c0e6b20c9d5d"),
    "openai-docs/references/mcp-diagnostics.md": (2318, "49bbd2f73df7bbd7f86c80425dea4da2d301c22046080399a36bfc0ca49509e9"),
    "openai-docs/references/model-migration.md": (5054, "5f20c38fbbb10319767b216d91ba74bae49c68fc1bfd6d1abd7c9b4cc9cb9ab0"),
    "openai-docs/references/model-selection.md": (1344, "ba2d164abbca30435a460a0bc3a7d82398dce2bdf092705c98ba55b3f3af38a8"),
    "openai-docs/references/official-docs.md": (3337, "7962f2dce55089b93bde4115bb89fd42f20993c1597a2b13edd4956f463875b9"),
    "openai-docs/references/prompting-guide.md": (15747, "db913884cfe0fabf29bee1a139918f56e299decfa0a14d61c48596f23f76621d"),
    "openai-docs/references/upgrade-guide.md": (1050, "ed1b75a89b8ec4d67787774ef6c4e8b98eace16c63348f42e969e6ffa67cb656"),
    "openai-docs/references/upgrading-to-gpt-5p6-sol.md": (23093, "9a918a0c8dd051d574f2fd0309201afa8a9b9c08241c11ca1fb0ac2f4724e7ac"),
    "openai-docs/scripts/fetch-codex-manual.mjs": (16085, "f53eb6d2f286e9efcc397e8bee93a938e37296c90953e4e06e94899ef1b6c363"),
    "openai-docs/scripts/resolve-latest-model-info": (1038, "7354dbb030ca0736dd633a7ca1b930cf640abd40370725dea3a458cb51d49523"),
    "openai-docs/scripts/resolve-latest-model-info.cjs": (3937, "eeb1bb486018e16b37edfc06b1a37179dbc672982d501040d4f7142f29dd2e64"),
    "plugin-creator/SKILL.md": (11467, "71b95b8219644f95d633721e7f7cd3c469edfc8fe50f8415d400dfb2d74bc7b9"),
    "plugin-creator/agents/openai.yaml": (339, "fecaf35d692bd3d33d1a065648258d12e393afa9055d78adf6e57b42f4142f6d"),
    "plugin-creator/assets/plugin-creator-small.svg": (1319, "6591bf8ea9bb9435890dbdea299e0d2bd05f3aa893a335d26e4c535e93c8e7fb"),
    "plugin-creator/assets/plugin-creator.png": (1563, "a4024b0306ddb05847e1012879d37aaf1e658205199da596f5145ed7a88d9162"),
    "plugin-creator/references/installing-and-updating.md": (6000, "91c4781d48568fcc708b45566b08fb610ad1c88672720ae512f9525a1cf9cb20"),
    "plugin-creator/references/plugin-json-spec.md": (9179, "eeb640130f69636affaa299d4170d5a7ae6a0ff978296ddf75c409ce6dd87b91"),
    "plugin-creator/scripts/create_basic_plugin.py": (11731, "272cb14e02ad7c76ac40777443c49cfecfa333ee28ab7f6214210b72c8fd02ae"),
    "plugin-creator/scripts/identifier_validation.py": (784, "a6d51ce4a9a7e8f85626ff5808a467a67574e7f8cdf1167ffb467c5f67e57223"),
    "plugin-creator/scripts/read_marketplace_name.py": (1644, "ba24e6d91eed6f778bde022a967be335c6253983b5ecd1c5e30c8483385887fd"),
    "plugin-creator/scripts/update_plugin_cachebuster.py": (3045, "6a630dfbdc2bca6952580da9c2c7ece08030926819ccd81d9deb518c24c1b0ed"),
    "plugin-creator/scripts/validate_plugin.py": (21739, "f4eeadb733b28b0c3e714de263a76d6542866a672f3e99bdffcf4dbcdf85e944"),
    "review-agent/SKILL.md": (2661, "07079efd0dc76f05fade424e5dfb048dce1de2df7626e1a4f56292a4f3f92228"),
    "review-agent/agents/openai.yaml": (252, "4d867a46d15e36ac880176484aae160f59855340c6059b2ea6ab9fbc9af084de"),
    "skill-creator/SKILL.md": (15311, "6656e54755638e8efcf275a472b9672eaa8a9a1b9e59dc210e275b03b59e1e66"),
    "skill-creator/agents/openai.yaml": (183, "d07d21b93fcf3d4dc8d9a3399c05fc226a49a333a96d3e1c68b451b8dd9eade6"),
    "skill-creator/assets/skill-creator-small.svg": (1319, "6591bf8ea9bb9435890dbdea299e0d2bd05f3aa893a335d26e4c535e93c8e7fb"),
    "skill-creator/assets/skill-creator.png": (1563, "a4024b0306ddb05847e1012879d37aaf1e658205199da596f5145ed7a88d9162"),
    "skill-creator/license.txt": (11358, "cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30"),
    "skill-creator/references/openai_yaml.md": (2356, "ffac39318e408108141d40f820968e59f70434a891694f9bf1d25be8237b150c"),
    "skill-creator/scripts/generate_openai_yaml.py": (6669, "816df12072d74faf924d802a6599a139997e1441e90bfe4869ec304f2e5e2851"),
    "skill-creator/scripts/init_skill.py": (10314, "bf0656a5f9d8d8cdecf0245eaf27e1df8e0e9b6d40ccdda814eb8e3d5f6992c8"),
    "skill-creator/scripts/quick_validate.py": (4249, "ee6dba90f44d37171c5a6edb8095979c54919ff6822c1a907afca2e78c48738c"),
    "skill-installer/LICENSE.txt": (11358, "cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30"),
    "skill-installer/SKILL.md": (3367, "d68b77e5bbb34dedab89d134da52855f140fc4b4299b80104f534e3b9e98f8ee"),
    "skill-installer/agents/openai.yaml": (221, "5ce223d8b1070b82c42298538f1b8d376f788eb9e7a42a987e8c094070d73f0e"),
    "skill-installer/assets/skill-installer-small.svg": (923, "3928703ff00dc1a681e7a22401843b7edcbd4b2051651ce4c43b75f7e140504e"),
    "skill-installer/assets/skill-installer.png": (1086, "d0a230b1a79b71b858b7c215a0fbb0768d6459c14ea4ef80c61592629bf0e605"),
    "skill-installer/scripts/github_utils.py": (659, "61c1bbe2ae217433b4b6f9f09f21aca4df52c12598068343ade719f706e4859b"),
    "skill-installer/scripts/install-skill-from-github.py": (11917, "38f311b75664bb063808f982c600271ebc4b560830f491705190f88a94e3e781"),
    "skill-installer/scripts/list-skills.py": (2948, "9d6dfb2abf3afeee7f027a89fb1626b918a973da18f4eb6c25681286301beb41"),
}
# Exact Windows rust-v0.153.4 distribution payload, verified in official asset
# 545043433: ZIP SHA-256 c016b0e6968b78586919c720d2685a03712f6d5f11bcd9d6f92c91eb8c41ba16,
# executable SHA-256 444a3f0008050605cae73cd9b7a2dcac61294062dfaab56dd20430fd6498518b.
# Every complete embedded member was matched to its pinned source path/body;
# these are exact platform bytes, never a runtime newline normalization rule.
SYSTEM_SKILL_FILES_LF = SYSTEM_SKILL_FILES
SYSTEM_SKILL_MARKER_LF = SYSTEM_SKILL_MARKER
SYSTEM_SKILL_MARKER_WINDOWS = b"9ba055b801325a\n"
SYSTEM_SKILL_FILES_WINDOWS = {
    "imagegen/LICENSE.txt": (10977, "44e03f7e263eb3ad256440c1eeab5e49168baff5a78ecef9a5447261cf3a91bc"),
    "imagegen/SKILL.md": (19516, "706d4d96e1d5c9e6023fe3ccabba1bb34b364024d344fd25b8515ec7d28fe3c4"),
    "imagegen/agents/openai.yaml": (281, "1797be4076717806df54aa9331c07ac8eebe32c69f32132d5858712b05406146"),
    "imagegen/assets/imagegen-small.svg": (2894, "087c1632a3b8a2ed4d6218952e7ba7ae2221a979f84233667ed2c587998784da"),
    "imagegen/assets/imagegen.png": (1711, "95952f644064eb9e890f98d8db07216347186526e4c41ad66d3420629eb86e20"),
    "imagegen/references/cli.md": (9897, "29dc8c1e3e5d9a20aee587fedb46a464552b6ec118d93bbd90c8d86e30502c8c"),
    "imagegen/references/codex-network.md": (1812, "cba3b4d24d30f38adf8021b6e9b49ec5261a9e45e7df31047ec451c916c99b0a"),
    "imagegen/references/image-api.md": (6162, "695909437b792aff171477dba127476a11235c220beb06533cce7c1495a042c8"),
    "imagegen/references/prompting.md": (8394, "d957a45cb24fe2cf1a85858674fc533f15a5f4621355f559a13a93168999c2dd"),
    "imagegen/references/sample-prompts.md": (18039, "76cc8a2988efb6e844a99ff9908b2b11cc07467510fc909002abcc0ee3f0701b"),
    "imagegen/scripts/image_gen.py": (35697, "3666b807de3f0a1d0a4a767b886d612c0b8f71025c500bba6693d1c6995a0873"),
    "imagegen/scripts/remove_chroma_key.py": (14354, "fa2989807052b857bed08f7fee0caa2502b54c46afc0a6f28cd706c25f636630"),
    "openai-docs/LICENSE.txt": (10977, "44e03f7e263eb3ad256440c1eeab5e49168baff5a78ecef9a5447261cf3a91bc"),
    "openai-docs/SKILL.md": (5484, "4371fcc6652d78309ccf5e7710a4fd22314ad7f1f4928a6c172ddae80c87cc87"),
    "openai-docs/agents/openai.yaml": (376, "0dd6c5ffa81a09782fd24cfa9fe1ec5c9633c90500b166ae18a462e613e876be"),
    "openai-docs/assets/openai-small.svg": (1094, "85ef0a2cde497872edec1525f1d538ccc42b33ff47acdbb88499a1631a4bdb21"),
    "openai-docs/assets/openai.png": (1429, "156cc84d7332bfe95b310350bd470b690d22aa33d65340cc6c2e06022946194c"),
    "openai-docs/references/codex-self-knowledge.md": (7488, "f945bf2a7cea6892d7a11ac38fb18b744fefd6d273654a8410c4c79e9f647fbe"),
    "openai-docs/references/latest-model.md": (2119, "8be54126ee4deec66d8257ae191ae0ecbd2c1022b2a47c6e959b11ae34aa5c97"),
    "openai-docs/references/mcp-diagnostics.md": (2345, "e9df2b2f8faa28930db4a2937cdb3665b4548ac958e80a8601a5dd0da1e6fad0"),
    "openai-docs/references/model-migration.md": (5099, "c3cc7848bf52e9ed2f03cb3a1466bd0406d7965b7782452b7d946376084fc91d"),
    "openai-docs/references/model-selection.md": (1356, "ce020100a28004500c927abcd57cff154c5c360eb72d918ef7d72492e2484cce"),
    "openai-docs/references/official-docs.md": (3362, "b2301d72b8092be5f5727d8d1cbc75740887d836f4f13b469f40c1db16b39fe3"),
    "openai-docs/references/prompting-guide.md": (16034, "31743be488f243b3fa59f8b73e4aaf6c2359e224ac642f93f9719a931acf16d5"),
    "openai-docs/references/upgrade-guide.md": (1072, "f1962dd5947041fb5344b095e639fe90c3e9ecfbb00c20d2394c912d1feec7c7"),
    "openai-docs/references/upgrading-to-gpt-5p6-sol.md": (23541, "1f77377c26c5754a0bfeaad72c9ac16ffcf25684aa51e220987a0a873aab9ca0"),
    "openai-docs/scripts/fetch-codex-manual.mjs": (16683, "1b9a36d3e59ddff3317d1faa182e5cd4967db14ff993593a5f98d5682dbffbe4"),
    "openai-docs/scripts/resolve-latest-model-info": (1077, "8f5006efa15c034ede29821b627d7b86c43f7dfaab422d335f45b9be8d04e2d5"),
    "openai-docs/scripts/resolve-latest-model-info.cjs": (4102, "91eb045abbf96ffb361e6ec2d3606540570fb32383edb22061f5549381ce0ee5"),
    "plugin-creator/SKILL.md": (11716, "9dba03c079abfbc680e8fca146069f054c1b0c5e9bcb5e5a9effcf86a95f67c6"),
    "plugin-creator/agents/openai.yaml": (345, "693e1667acc879a7b966cf073fee6cf1f268974983c00b6b779dab031d79185e"),
    "plugin-creator/assets/plugin-creator-small.svg": (1322, "edb19a0c59f9a1b5253b9af6d2f9f03cac27b2fe6a0275ebccbbdbff07891a1a"),
    "plugin-creator/assets/plugin-creator.png": (1563, "a4024b0306ddb05847e1012879d37aaf1e658205199da596f5145ed7a88d9162"),
    "plugin-creator/references/installing-and-updating.md": (6144, "476850a165cd63f8f3def5968b302c77e3cd5250af4df2dec1575a2eb4b51467"),
    "plugin-creator/references/plugin-json-spec.md": (9397, "254f18eb2cf9390c0b85f0b885db3b1facf90e060257cde70b6e55e21797382e"),
    "plugin-creator/scripts/create_basic_plugin.py": (12100, "a0dfca09c7ecc8f30012cb2111acfdf733eac7c5bd73e4edb154a45487de303f"),
    "plugin-creator/scripts/identifier_validation.py": (804, "680ce5b775ee5410ecf808fad40350dc4df2cc4a7003363f4c4c7f9d7c0be918"),
    "plugin-creator/scripts/read_marketplace_name.py": (1697, "0be0d7e327688b70300baa4fff448a8e24f4c00ce71340f9c6299d21c333cc65"),
    "plugin-creator/scripts/update_plugin_cachebuster.py": (3134, "d37acec7cfc34fb44db35bce437bf5ee8eb6da3484aece565e3a6e70e2bf121b"),
    "plugin-creator/scripts/validate_plugin.py": (22396, "1e6cb914505b458856c2cfab7d18a224731c743ef47e0c9d78afe64f35b67f7c"),
    "review-agent/SKILL.md": (2718, "bb46315f6bc4b7051c05f807d19b1689b20418ac6c42590406d805c10fa2fd1a"),
    "review-agent/agents/openai.yaml": (258, "579774dde24f21ee57d5cb63f69c2fce0da04b81538c636a0e0dd37dc836c811"),
    "skill-creator/SKILL.md": (15540, "cccd291077ec57c6f50ca6529f0f3fb93212da09473effb2fcec808e81b21288"),
    "skill-creator/agents/openai.yaml": (188, "b750627cdd588fdbb5b46e854d3384d1a504fad3b8ed73ca5d7ead3804df78b6"),
    "skill-creator/assets/skill-creator-small.svg": (1322, "edb19a0c59f9a1b5253b9af6d2f9f03cac27b2fe6a0275ebccbbdbff07891a1a"),
    "skill-creator/assets/skill-creator.png": (1563, "a4024b0306ddb05847e1012879d37aaf1e658205199da596f5145ed7a88d9162"),
    "skill-creator/license.txt": (11560, "3ddf9be5c28fe27dad143a5dc76eea25222ad1dd68934a047064e56ed2fa40c5"),
    "skill-creator/references/openai_yaml.md": (2405, "7039d465b05342cbacf861f08d20192ab1eeee112bc5d8ff0fb66234756887a9"),
    "skill-creator/scripts/generate_openai_yaml.py": (6900, "30c73f1ea7d47dbab2b231bb92c5bf9026f1ad14ba8e3b3a4a544092c4d24ebe"),
    "skill-creator/scripts/init_skill.py": (10622, "74dfa9361a93bee6932d6cbb332ed19ac26a27ba3800e69fb07ba7512c1025f9"),
    "skill-creator/scripts/quick_validate.py": (4376, "6068513d924ed3559e186dfcdead7439129828dcf402167fd925c06dffbf2806"),
    "skill-installer/LICENSE.txt": (11560, "3ddf9be5c28fe27dad143a5dc76eea25222ad1dd68934a047064e56ed2fa40c5"),
    "skill-installer/SKILL.md": (3425, "2892d8b2fb11631132118f340aed80ae17e59d5a571fc2c4f4a4a7e73f139d52"),
    "skill-installer/agents/openai.yaml": (226, "bd64b4d6893138706b3e3540f266ab809ccdcd08375652ae9f9d1e029fe50d02"),
    "skill-installer/assets/skill-installer-small.svg": (926, "099e5a4b5e5f0f918a8de0d2059f0a86211da623dbb75f86f13dc9f7c08c778d"),
    "skill-installer/assets/skill-installer.png": (1086, "d0a230b1a79b71b858b7c215a0fbb0768d6459c14ea4ef80c61592629bf0e605"),
    "skill-installer/scripts/github_utils.py": (680, "2f9702afb9f0da740181fa4a3214f663e460f80cd5cea2e7dcaab00ad269082e"),
    "skill-installer/scripts/install-skill-from-github.py": (12272, "110cc7e4e395c2c9ba070d47db7d21be5164591c35015ae364fc9c9905b08de8"),
    "skill-installer/scripts/list-skills.py": (3054, "2cd6eecbaf9be7c86e80a14d7042f95d6abb83b6baee5f17dd075fa7a71c83de"),
}
if os.name == "nt":
    SYSTEM_SKILL_FILES = SYSTEM_SKILL_FILES_WINDOWS
    SYSTEM_SKILL_MARKER = SYSTEM_SKILL_MARKER_WINDOWS

SYSTEM_SKILL_DIRECTORIES = frozenset(
    parent.as_posix() for name in SYSTEM_SKILL_FILES for parent in Path(name).parents
    if parent != Path("."))
if len(SYSTEM_SKILL_FILES) != 59 or len(SYSTEM_SKILL_DIRECTORIES) != 26:
    raise ValueError("Codex pinned bundled Skill manifest cardinality differs")


def system_skill_initialization(profile):
    """Verify only the complete immutable 0.153.4 embedded bundle, if present."""
    deadline = time.monotonic() + 5
    skills = profile / "skills"
    root = skills / ".system"
    _real_directory(profile)
    try:
        skills.lstat()
    except FileNotFoundError:
        if time.monotonic() > deadline:
            raise ValueError("Codex bundled Skill inspection exceeded its deadline")
        return {}, (), {}
    _real_directory(skills)
    try:
        root.lstat()
    except FileNotFoundError:
        if time.monotonic() > deadline:
            raise ValueError("Codex bundled Skill inspection exceeded its deadline")
        return {}, (), {}
    files = dict(SYSTEM_SKILL_FILES)
    directories = SYSTEM_SKILL_DIRECTORIES
    marker = SYSTEM_SKILL_MARKER
    files[".codex-system-skills.marker"] = (len(marker), hashlib.sha256(marker).hexdigest())
    expected_children = {name: set() for name in ("", *directories)}
    for name in (*directories, *files):
        parent, _, leaf = name.rpartition("/")
        expected_children[parent].add(leaf)
    paths = (skills, root, *(root / name for name in sorted(directories)))
    anchors = {path: _FILES["identity"](_real_directory(path)) for path in paths}
    identities, digests, metadata = {}, {}, {}
    total = 0
    for name, children in expected_children.items():
        if time.monotonic() > deadline:
            raise ValueError("Codex bundled Skill inspection exceeded its deadline")
        if set(_bounded_member_names(root / name, 86)) != children:
            raise ValueError("Codex bundled Skill inventory differs from the pinned source")
    for name, (size, digest) in files.items():
        if time.monotonic() > deadline or not 0 <= size <= 65536:
            raise ValueError("Codex bundled Skill file bound differs")
        path = root / name
        initial = path.lstat()
        body = _regular_bytes(path, size)
        total += len(body)
        if (len(body) != size or total > 524288
                or hashlib.sha256(body).hexdigest() != digest
                or (name == ".codex-system-skills.marker" and body != marker)):
            raise ValueError("Codex bundled Skill bytes differ from the pinned source")
        identities[path] = _FILES["identity"](initial)
        digests[path] = digest
        metadata[path] = (initial.st_mode, initial.st_dev, initial.st_ino, initial.st_mtime_ns)
    for path, identity in identities.items():
        if _FILES["identity"](path.lstat()) != identity:
            raise ValueError("Codex bundled Skill file changed during inventory")
    for path, identity in anchors.items():
        current = _real_directory(path)
        if _FILES["identity"](current) != identity:
            raise ValueError("Codex bundled Skill directory changed during inventory")
        metadata[path] = (current.st_mode, current.st_dev, current.st_ino)
    if time.monotonic() > deadline:
        raise ValueError("Codex bundled Skill inspection exceeded its deadline")
    return digests, paths, metadata


def _temporary_git_name_diagnostic(name, phase):
    """Quote bounded listing evidence without reading the rejected child."""
    return ("Codex temporary Git directory name is not qualified; "
            f"checkpoint={ascii(phase[:32])}; basename={ascii(name[:64])}; "
            f"basename_chars={len(name)}; basename_truncated={len(name) > 64}")


def _temporary_git_state(profile, *, phase="direct-scan"):
    """Verify the pinned client's empty automatic-Git staging repositories.

    Codex 0.153.4 core-plugins/src/git_policy.rs writes this skeleton under
    CODEX_HOME/.tmp. Optional objects/refs must be empty; this structural
    check does not identify the historical writer of a matching residue.
    startup_sync.rs may also create plugins.sync.lock in this shared root.
    Retain an observed empty lock, never accept it as a repeat delta.
    """
    root = profile / ".tmp"
    try:
        root.lstat()
    except FileNotFoundError:
        return {}, {}
    _real_directory(profile)
    files, identities, witnesses = {}, {}, {}

    def directory(path):
        info = _real_directory(path)
        witnesses[path] = _FILES["identity"](info)
        identities[path] = (info.st_dev, info.st_ino, info.st_mode,
                            getattr(info, "st_file_attributes", None))

    directory(root)
    names = _bounded_member_names(root, TEMPORARY_GIT_LIMIT)
    layouts = {}
    for name in names:
        if name == "plugins.sync.lock":
            lock = root / name
            identities[lock] = _FILES["identity"](lock.lstat())
            _regular_bytes(lock, 0)
            continue
        if re.fullmatch(r"git-[A-Za-z0-9]{6}", name) is None:
            raise ValueError(_temporary_git_name_diagnostic(name, phase))
        repository = root / name
        directory(repository)
        members = _bounded_member_names(repository, 3)
        if "HEAD" not in members or not set(members) <= {"HEAD", "objects", "refs"}:
            raise ValueError("Codex temporary Git repository has unexpected members")
        layouts[repository] = members
        for child in (member for member in members if member != "HEAD"):
            path = repository / child
            directory(path)
            _bounded_member_names(path, 0)
            layouts[path] = []
        head = repository / "HEAD"
        before = _FILES["identity"](head.lstat())
        if _regular_bytes(head, len(TEMPORARY_GIT_HEAD)) != TEMPORARY_GIT_HEAD:
            raise ValueError("Codex temporary Git HEAD differs from the pinned native bytes")
        if before != _FILES["identity"](head.lstat()):
            raise ValueError("Codex temporary Git HEAD changed during inspection")
        identities[head] = before
        files[f"{profile.name}/{head.relative_to(profile)}"] = hashlib.sha256(TEMPORARY_GIT_HEAD).hexdigest()
    if _bounded_member_names(root, TEMPORARY_GIT_LIMIT) != names:
        raise ValueError("Codex temporary Git namespace changed during inspection")
    for path, members in layouts.items():
        if _bounded_member_names(path, 3) != members:
            raise ValueError("Codex temporary Git members changed during inspection")
    _confirm_temporary_git_identities(identities, witnesses)
    return files, identities


def plugin_initialization(profile):
    """Combine only pinned builtin bytes and existing bounded temporary Git proof."""
    deadline = time.monotonic() + 5
    files, directories, metadata = system_skill_initialization(profile)
    directories = list(directories)
    _, temporary = _temporary_git_state(profile, phase="plugin-prebaseline")
    for path, previous in temporary.items():
        info = path.lstat()
        if stat.S_ISDIR(info.st_mode):
            observed = (info.st_dev, info.st_ino, info.st_mode,
                        getattr(info, "st_file_attributes", None))
            if observed != previous:
                raise ValueError("Codex preinstall temporary directory identity changed")
            directories.append(path)
            metadata[path] = (info.st_mode, info.st_dev, info.st_ino)
        else:
            if _FILES["identity"](info) != previous:
                raise ValueError("Codex preinstall temporary file identity changed")
            if path.name == "HEAD":
                body = TEMPORARY_GIT_HEAD
            elif path == profile / ".tmp/plugins.sync.lock":
                body = b""
            else:
                raise ValueError("Codex preinstall temporary file differs")
            files[path] = hashlib.sha256(body).hexdigest()
            metadata[path] = (info.st_mode, info.st_dev, info.st_ino, info.st_mtime_ns)
    if time.monotonic() > deadline:
        raise ValueError("Codex plugin initialization inspection exceeded its deadline")
    return files, tuple(directories), metadata


def _confirm_temporary_git_identities(identities, witnesses):
    for path, previous in identities.items():
        if path.name in ("HEAD", "plugins.sync.lock"):
            current = _FILES["identity"](path.lstat())
        else:
            info = _real_directory(path)
            if _FILES["identity"](info) != witnesses[path]:
                raise ValueError("Codex temporary Git directory changed during inspection")
            current = (info.st_dev, info.st_ino, info.st_mode,
                       getattr(info, "st_file_attributes", None))
        if current != previous:
            raise ValueError("Codex temporary Git identity changed during inspection")


def _repeat_temporary_git_only(profile, before, after, previous):
    """Accept only newly observed empty native Git HEADs; retain raw differences."""
    old_files, old_identities = previous
    files, identities = _temporary_git_state(profile, phase="repeat-comparison")
    lock = profile / ".tmp" / "plugins.sync.lock"
    if (lock in old_identities) != (lock in identities):
        return False
    if any(identities.get(path) != value for path, value in old_identities.items()):
        return False
    if any(before.get(path) != digest or after.get(path) != digest
           for path, digest in old_files.items()):
        return False
    if any(after.get(path) != digest for path, digest in files.items()):
        return False
    changed = {path for path in before.keys() | after.keys() if before.get(path) != after.get(path)}
    return bool(changed) and all(path not in before and path not in old_files
                                and path in files and after[path] == files[path] for path in changed)


def _runtime_files(root):
    _real_directory(root)
    names = []
    with os.scandir(root) as entries:
        for entry in entries:
            names.append(entry.name)
            if len(names) > 3 * len(DATABASES):
                raise ValueError("Codex SQLite runtime member count is invalid")
    names.sort()
    if not names or len(names) > 3 * len(DATABASES):
        raise ValueError("Codex SQLite runtime member count is invalid")
    bodies, total = {}, 0
    for name in names:
        database = next((db for db in DATABASES if name in (db, db + "-wal", db + "-shm")), None)
        if database is None or database not in names:
            raise ValueError("Codex SQLite runtime contains an unexpected member")
        body = _regular_bytes(root / name, min(FILE_LIMIT, TOTAL_LIMIT - total))
        bodies[name] = body
        total += len(body)
    return bodies


def _cell(value):
    return {"bytes": value.hex()} if isinstance(value, bytes) else value


def _table_digest(connection, name):
    quoted = '"' + name.replace('"', '""') + '"'
    encoded, size = [], 0
    for row in connection.execute("SELECT * FROM " + quoted):
        value = json.dumps([_cell(cell) for cell in row], sort_keys=True, separators=(",", ":"))
        encoded.append(value)
        size += len(value)
        if len(encoded) > ROW_LIMIT or size > TOTAL_LIMIT:
            raise ValueError("Codex SQLite table exceeds the semantic audit bound")
    digest = hashlib.sha256()
    for value in sorted(encoded):
        digest.update(value.encode("utf-8") + b"\n")
    return {"rows": len(encoded), "sha256": digest.hexdigest()}


def _database_state(path, expected=None):
    started = time.monotonic()
    connection = sqlite3.connect(path.as_uri() + "?mode=ro", uri=True)
    try:
        connection.setlimit(sqlite3.SQLITE_LIMIT_LENGTH, FILE_LIMIT)
        connection.set_progress_handler(lambda: int(time.monotonic() - started > 5), 1000)
        connection.execute("PRAGMA query_only=ON")
        connection.execute("PRAGMA trusted_schema=OFF")
        schema = connection.execute(
            "SELECT type,name,tbl_name,sql FROM sqlite_schema ORDER BY type,name").fetchall()
        if not schema or expected is not None and schema != expected["schema"]:
            raise ValueError("Codex SQLite runtime schema changed")
        if connection.execute("PRAGMA integrity_check").fetchall() != [("ok",)]:
            raise ValueError("Codex SQLite runtime failed integrity_check")
        tables = {name: _table_digest(connection, name)
                  for kind, name, _, sql in schema if kind == "table" and sql is not None}
        metadata = {name: connection.execute("PRAGMA " + name).fetchone()[0]
                    for name in ("user_version", "application_id")}
        result = {"schema": schema, "tables": tables, "metadata": metadata}
        if expected is not None and result != expected:
            raise ValueError("Codex SQLite runtime table contents changed")
        return result
    finally:
        connection.close()


def runtime_state(root, temporary, baseline=None):
    """Fully read live bytes; interpret WAL only in a private, disposable copy."""
    original_root = _real_directory(root)
    bodies = _runtime_files(root)
    databases = {name for name in bodies if name in DATABASES}
    if baseline is not None and databases != set(baseline):
        raise ValueError("Codex SQLite database inventory changed")
    with tempfile.TemporaryDirectory(prefix="codex-sqlite-proof-", dir=temporary) as scratch:
        copy = Path(scratch)
        for name, body in bodies.items():
            with (copy / name).open("xb") as stream:
                stream.write(body)
        result = {name: _database_state(copy / name, baseline[name] if baseline else None)
                  for name in sorted(databases)}
    if (not os.path.samestat(original_root, _real_directory(root))
            or _runtime_files(root) != bodies):
        raise ValueError("Codex SQLite runtime changed while its copy was inspected")
    files = {name: {"bytes": len(body), "sha256": hashlib.sha256(body).hexdigest()}
             for name, body in bodies.items()}
    return result, files


async def _drain(reader, limit):
    parts, size = [], 0
    while block := await reader.read(65536):
        size += len(block)
        if size > limit:
            raise ValueError("Codex native discovery output exceeds audit bound")
        parts.append(block)
    return b"".join(parts)


def _windows_active(job):
    counts = job.Accounting()
    job.check(job.api.QueryInformationJobObject(
        job.handle, 1, job.C.byref(counts), job.C.sizeof(counts), None))
    return counts.active


def _posix_group_active(pid):
    try:
        os.killpg(pid, 0)
        return True
    except ProcessLookupError:
        return False


def _posix_group_diagnostic(pid, *, proc_root=Path("/proc")):
    """Bounded, non-atomic observations only; never an alternative closure gate."""
    result = {"pgid": pid, "group_exists": None, "members": [],
              "scan_complete": False, "unreadable": 0, "diagnostic_only": True}
    try:
        result["group_exists"] = _posix_group_active(pid)
        if not sys.platform.startswith("linux"):
            result["unavailable"] = "Linux proc stat is unavailable on this platform"
            return result
        deadline = time.monotonic() + .25
        with os.scandir(proc_root) as entries:
            for scanned, entry in enumerate(entries, 1):
                if scanned > 4096 or time.monotonic() >= deadline:
                    result["truncated"] = True
                    break
                if not entry.name.isdecimal():
                    continue
                try:
                    if not entry.is_dir(follow_symlinks=False):
                        continue
                    descriptor = os.open(Path(entry.path) / "stat",
                                         os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK)
                    try:
                        if not stat.S_ISREG(os.fstat(descriptor).st_mode):
                            raise ValueError("nonregular proc stat")
                        raw = os.read(descriptor, 4097)
                    finally:
                        os.close(descriptor)
                    if len(raw) > 4096 or not raw.endswith(b"\n"):
                        raise ValueError("unbounded or incomplete proc stat")
                    observed, _ = raw.split(b" (", 1)
                    fields = raw.rsplit(b") ", 1)[1].split()
                    if observed != entry.name.encode("ascii") or len(fields) < 20:
                        raise ValueError("inexact proc identity")
                    numbers = fields[1:4] + fields[19:20]
                    if (len(fields[0]) != 1 or not fields[0].isalpha()
                            or any(not value.isdigit() or len(value) > 20 for value in numbers)):
                        raise ValueError("inexact proc fields")
                    parent, group, session, started = map(int, numbers)
                    if group != pid:
                        continue
                    if len(result["members"]) == 8:
                        result["truncated"] = True
                        break
                    result["members"].append({"pid": int(entry.name), "ppid": parent,
                                              "pgid": group, "sid": session,
                                              "state": fields[0].decode("ascii"), "start_ticks": started})
                except (OSError, ValueError, IndexError, UnicodeError):
                    result["unreadable"] += 1
            else:
                result["scan_complete"] = True
    except Exception as error:
        result["observation_error"] = type(error).__name__
    result["members"].sort(key=lambda member: member["pid"])
    return result


def _note_group_drain_failure(error, process, when, transcript=None):
    # Exception notes survive the existing bounded harness traceback report.
    # Retain no argv, environment, comm or unrelated-process metadata.
    try:
        detail = {"phase": "group-drain", "when": when, "pid": process.pid,
                  "exit_code": process.returncode, "group": _posix_group_diagnostic(process.pid)}
        if transcript is not None:
            received = b"".join(transcript)
            detail["rpc_stdout"] = {
                "bytes": len(received), "messages": len(transcript),
                "sha256": hashlib.sha256(received).hexdigest()}
        body = json.dumps(detail, sort_keys=True, separators=(",", ":"))
        if len(body) > 3072:
            body = json.dumps({"phase": "group-drain", "when": when, "pid": process.pid,
                               "diagnostic_truncated": True, "bytes": len(body),
                               "sha256": hashlib.sha256(body.encode()).hexdigest(),
                               "prefix": body[:1024]}, separators=(",", ":"))
        error.add_note("Codex native group diagnostic: " + body)
    except Exception as diagnostic:
        error.add_note("Codex native group diagnostic unavailable: " + type(diagnostic).__name__)


async def _start_native(binary, project, environment):
    windows = os.name == "nt"
    argv = ([sys.executable, str(Path(__file__).resolve()), "--windows-native-start", str(binary)]
            if windows else [str(binary), "app-server"])
    options = {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP} if windows else {"start_new_session": True}
    process = await asyncio.create_subprocess_exec(
        *argv, cwd=project, env=environment,
        stdin=asyncio.subprocess.PIPE, stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE, limit=2 << 20, **options)
    job = None
    try:
        if windows:
            # Reuse the tested job/accounting owner. The bootstrap cannot start
            # npm/native descendants before this assignment and one-byte gate.
            helpers = run_path(str(Path(__file__).parent / "e2e/hermes-noauth-tool-call.py"))
            owner = process._transport.get_extra_info("subprocess")
            job = helpers["WindowsJob"](owner)
            process.stdin.write(b"!")
            await process.stdin.drain()
        return process, job
    except BaseException:
        process.stdin.close()
        try:
            if job is not None:
                await asyncio.to_thread(job.close)
            elif process.returncode is None:
                process.kill()  # The unreleased bootstrap has no native descendants.
        finally:
            try:
                await asyncio.wait_for(process.wait(), 5)
            finally:
                await _close_native_transport(process)
        raise


async def _close_native_transport(process):
    # CPython Process has no public close API. Its owned subprocess transport
    # closes all three pipe transports; one loop turn runs their close callbacks.
    # Use this private adapter only after the bounded process cleanup/reap.
    process._transport.close()
    await asyncio.sleep(0)


async def _finish_native(process, job, completed, readers):
    try:
        try:
            process.stdin.close()
        finally:
            if job is not None:
                await asyncio.to_thread(job.close)
            elif not completed:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
            await asyncio.wait_for(process.wait(), 5)
    finally:
        try:
            for reader in readers:
                if not reader.done():
                    reader.cancel()
            await asyncio.gather(*readers, return_exceptions=True)
        finally:
            await _close_native_transport(process)


async def _exchange(binary, project, profile, environment):
    process, job = await _start_native(binary, project, environment)
    errors = asyncio.create_task(_drain(process.stderr, 1 << 20))
    readers, completed = [errors], False
    transcript, total, group_drain = [], 0, False

    async def send(message):
        process.stdin.write((json.dumps(message) + "\n").encode())
        await process.stdin.drain()

    async def response(identity):
        nonlocal total
        for _ in range(64):
            line = await process.stdout.readline()
            total += len(line)
            if not line or total > 4 << 20:
                raise ValueError("Codex native discovery ended or exceeded output bound")
            message = json.loads(line)
            transcript.append(line)
            if message.get("id") == identity:
                if "error" in message or not isinstance(message.get("result"), dict):
                    raise ValueError("Codex native discovery returned an error")
                return message["result"]
            if "id" in message or not isinstance(message.get("method"), str):
                raise ValueError("Codex native discovery returned an unexpected response")
        raise ValueError("Codex native discovery notification bound exceeded")

    try:
        async with asyncio.timeout(15):
            await send({"id": 1, "method": "initialize", "params": {
                "clientInfo": {"name": "agentplugins", "version": "1"},
                "capabilities": {"experimentalApi": True}}})
            initialized = await response(1)
            if not _path_equal(initialized.get("codexHome"), profile):
                raise ValueError("Codex native discovery initialized another profile")
            if not str(initialized.get("userAgent", "")).startswith("agentplugins/0.153.4 "):
                raise ValueError("Codex runtime audit requires the qualified native 0.153.4")
            await send({"method": "initialized"})
            await send({"id": 2, "method": "skills/list", "params": {
                "cwds": [str(project)], "forceReload": True}})
            listing = await response(2)
            process.stdin.close()
            await process.stdin.wait_closed()
            tail = asyncio.create_task(_drain(process.stdout, (4 << 20) - total))
            readers.append(tail)
            await process.wait()
            await tail
            stderr = await errors
            if process.returncode != 0 or stderr:
                raise ValueError(
                    "Codex native discovery did not exit cleanly: "
                    f"exit_code={process.returncode}, stderr_bytes={len(stderr)}, "
                    f"stderr_prefix={stderr[:2048]!r}")
            group_drain = True
            while _windows_active(job) if job is not None else _posix_group_active(process.pid):
                await asyncio.sleep(.02)  # Still inside the same finite 15-second budget.
            completed = True
            return listing
    finally:
        failure = sys.exception()
        diagnose = failure is not None and group_drain and job is None
        try:
            if diagnose:
                _note_group_drain_failure(failure, process, "before-owned-cleanup", transcript)
        finally:
            try:
                await _finish_native(process, job, completed, readers)
            except Exception as cleanup:
                if failure is not None:
                    raise RuntimeError(f"Codex discovery failed ({failure}); owned cleanup failed ({cleanup})") from failure
                raise
            finally:
                if diagnose:
                    _note_group_drain_failure(failure, process, "after-owned-cleanup")


def _path_equal(value, expected):
    if not isinstance(value, str) or not Path(value).is_absolute():
        return False
    path = Path(value)
    try:
        # Real Windows paths may have extended-length spelling. Directory/file
        # identity admits that form without accepting redirected aliases.
        return (not any(member.is_symlink() or member.is_junction()
                        for member in (path, *path.parents))
                and os.path.normcase(path.name) == os.path.normcase(expected.name)
                and path.parent.samefile(expected.parent)
                and path.samefile(expected))
    except (OSError, ValueError):
        return False


def _references_skill_path(value, expected):
    """Compare owned identity even when its final path components are absent."""
    def anchor(value):
        if not isinstance(value, str) or not Path(value).is_absolute():
            raise ValueError("Codex native Skill path is ambiguous")
        path, missing = Path(value), []
        if ".." in path.parts:
            raise ValueError("Codex native Skill path is redirected")
        while True:
            try:
                info = path.lstat()
                break
            except FileNotFoundError:
                missing.append(os.path.normcase(path.name))
                parent = path.parent
                if parent == path:
                    raise ValueError("Codex native Skill path has no existing anchor")
                path = parent
        for member in (path, *path.parents):
            observed = member.lstat()
            if member == path and not os.path.samestat(info, observed):
                raise ValueError("Codex native Skill anchor identity changed")
            if (stat.S_ISLNK(observed.st_mode)
                    or getattr(observed, "st_file_attributes", 0) & 0x400):
                raise ValueError("Codex native Skill path is redirected")
            if member != path and not stat.S_ISDIR(observed.st_mode):
                raise ValueError("Codex native Skill ancestor is not a directory")
        if not (stat.S_ISREG(info.st_mode) or stat.S_ISDIR(info.st_mode)):
            raise ValueError("Codex native Skill anchor is not regular")
        return info, tuple(reversed(missing))

    observed, tail = anchor(value)
    owned, owned_tail = anchor(str(expected))
    return tail == owned_tail and os.path.samestat(observed, owned)


def verify_skill_inventory(listing, project, identity, selected, foreign, present):
    rows = listing.get("data")
    if not isinstance(rows, list) or len(rows) != 1 or not isinstance(rows[0], dict):
        raise ValueError("Codex native discovery returned ambiguous projects")
    row = rows[0]
    if not _path_equal(row.get("cwd"), project) or row.get("errors") not in (None, []):
        raise ValueError("Codex native discovery returned another project or errors")
    skills = row.get("skills")
    if not isinstance(skills, list) or any(not isinstance(skill, dict) for skill in skills):
        raise ValueError("Codex native Skill inventory is malformed")
    for name, expected, required in (("foreign-neighbor", foreign, True), (identity, selected, present)):
        matched = [skill for skill in skills
                   if _references_skill_path(skill.get("path"), expected)]
        if not required:
            if matched:
                raise ValueError("removed Codex Skill is still natively discoverable")
            continue
        if (len(matched) != 1 or matched[0].get("name") != name
                or not _path_equal(matched[0].get("path"), expected)
                or matched[0].get("enabled") is not True or matched[0].get("scope") != "repo"
                or matched[0].get("pluginId") is not None):
            raise ValueError("Codex native Skill identity, path or enablement is unqualified")


def public_skill_ready(added, matches, identity):
    if not isinstance(added, dict) or not isinstance(matches, list) or len(matches) != 1:
        return False
    if (added.get("schema_version") != 1 or added.get("command") != "skills add"
            or added.get("result") != "success" or added.get("status") != "succeeded"
            or added.get("scope") != "project" or added.get("readiness") != "ready"
            or added.get("next_actions") not in (None, [])):
        return False
    items = added.get("items")
    if (not isinstance(items, list) or len(items) != 1 or not isinstance(items[0], dict)
            or items[0].get("alias") != identity or items[0].get("targetId") != "codex"
            or items[0].get("status") not in ("succeeded", "unchanged")):
        return False
    clients = matches[0].get("clients") if isinstance(matches[0], dict) else None
    if not isinstance(clients, list) or any(not isinstance(client, dict) for client in clients):
        return False
    selected = [client for client in clients if client.get("client_id") == "codex"]
    return len(selected) == 1 and all(selected[0].get(key) == value for key, value in {
        "scope": "project", "materialization": "materialized", "activation": "active",
        "verification": "installation_verified", "authentication": "not_required", "policy": "allowed",
    }.items())


class CodexPluginSQLiteAudit:
    """Forward plugin measurement: owned SQLite root, unchanged semantic state.

    Plugin registration and removal remain independently checked by the caller.
    Native-profile paths are never excluded from its generic scanner.
    """
    def __init__(self, binary, case, home, project, environment):
        self.case, self.project = case, project
        self.profile, self.temporary = home / ".codex", case / "tmp"
        self.runtime = home / ".local/state/codex-sqlite"
        self.config = self.profile / "config.toml"
        self.pin = str(self.runtime)
        for directory in (home / ".local", home / ".local/state", self.runtime):
            _real_directory(directory.parent)
            if directory == self.runtime or not directory.exists():
                directory.mkdir(mode=0o700, exist_ok=False)
            _real_directory(directory)
        paths = {case, home, project, self.profile, self.temporary, self.runtime}
        paths.update(self.runtime.parents[:3])
        self.anchors = {path: _real_directory(path) for path in paths}
        body = _sqlite_config(self.pin)
        with self.config.open("xb") as stream:
            stream.write(body)
        initial_config = _FILES["identity"](self.config.lstat())
        self.guard()
        native_env = {key: value for key, value in environment.items()
                      if key.upper() in {"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR",
                                         "TEMP", "TMP", "TMPDIR"}}
        native_env["CODEX_HOME"] = str(self.profile)
        listing = asyncio.run(_exchange(
            Path(binary), project, self.profile, native_env))
        self.native_version = "0.153.4"
        self.guard()
        rows = listing.get("data")
        if (not isinstance(rows, list) or len(rows) != 1 or not isinstance(rows[0], dict)
                or not _path_equal(rows[0].get("cwd"), project)
                or rows[0].get("errors") not in (None, [])
                or not isinstance(rows[0].get("skills"), list)):
            raise ValueError("Codex preinstall inventory is not the selected context")
        # The preinstall profile must still contain only the authored pin.
        if (_regular_bytes(self.config, 65536) != body
                or _FILES["identity"](self.config.lstat()) != initial_config):
            raise ValueError("Codex preinstall changed the authored config")
        self.baseline, files = runtime_state(self.runtime, self.temporary)
        self.primary_anchors = {name: (self.runtime / name).lstat() for name in self.baseline}
        self.guard()
        self.observations = [{"phase": "before-install", "runtime_files_fully_read": files}]

    def guard(self):
        for path, original in self.anchors.items():
            if not os.path.samestat(original, _real_directory(path)):
                raise ValueError("Codex plugin SQLite authority root changed")
        for name, original in getattr(self, "primary_anchors", {}).items():
            if not os.path.samestat(original, (self.runtime / name).lstat()):
                raise ValueError("Codex plugin SQLite primary database identity changed")
        config = tomllib.loads(_regular_bytes(self.config, 65536).decode("utf-8"))
        if config.get("sqlite_home") != self.pin:
            raise ValueError("Codex plugin SQLite config authority changed")

    def checkpoint(self, phase):
        self.guard()
        _, files = runtime_state(self.runtime, self.temporary, self.baseline)
        self.guard()
        self.observations.append({"phase": phase, "runtime_files_fully_read": files})

    def evidence(self):
        return {"native_version": self.native_version,
                "runtime_root": str(self.runtime.relative_to(self.case)),
                "runtime_semantics_unchanged": True, "runtime_baseline": self.baseline,
                "observations": self.observations}


class CodexRuntimeAudit:
    def __init__(self, binary, case, home, project, environment, identity, foreign):
        self.binary, self.case, self.project = Path(binary), case, project
        self.profile, self.temporary = home / ".codex", case / "tmp"
        self.runtime = home / ".local/state/codex-sqlite"
        self.config = self.profile / "config.toml"
        self.identity, self.foreign = identity, foreign
        self.selected = project / ".agents/skills" / identity / "SKILL.md"
        self.environment = {key: value for key, value in environment.items()
                            if key.upper() in {"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR",
                                               "TEMP", "TMP", "TMPDIR"}}
        self.environment["CODEX_HOME"] = str(self.profile)
        for directory in (home / ".local", home / ".local/state", self.runtime):
            _real_directory(directory.parent)
            if directory == self.runtime or not directory.exists():
                directory.mkdir(exist_ok=False)
            _real_directory(directory)
        paths = {case, home, self.profile, project, self.temporary, self.runtime}
        paths.update(self.runtime.parents[:3])
        self.anchors = {path: _real_directory(path) for path in paths}
        self.config_bytes = _sqlite_config(self.runtime)
        with self.config.open("xb") as stream:
            stream.write(self.config_bytes)
        self.foreign_bytes = _regular_bytes(foreign, FILE_LIMIT)
        self.baseline, self.observations = None, []
        self.checkpoint("before-install", False)

    def guard(self):
        for path, previous in self.anchors.items():
            if not os.path.samestat(previous, _real_directory(path)):
                raise ValueError("Codex runtime audit root identity changed")
        if _regular_bytes(self.config, 65536) != self.config_bytes:
            raise ValueError("Codex native runtime configuration changed")
        if _regular_bytes(self.foreign, FILE_LIMIT) != self.foreign_bytes:
            raise ValueError("Codex foreign Skill bytes changed")

    def checkpoint(self, phase, present):
        self.guard()
        selected_bytes = _regular_bytes(self.selected, FILE_LIMIT) if present else None
        try:
            listing = asyncio.run(_exchange(self.binary, self.project, self.profile, self.environment))
        except Exception as error:
            error.add_note(f"Codex native skill inventory phase={phase!r}; "
                           f"command={[str(self.binary), 'app-server']!r}")
            raise
        self.guard()
        if present and _regular_bytes(self.selected, FILE_LIMIT) != selected_bytes:
            raise ValueError("Codex selected Skill bytes changed during native discovery")
        verify_skill_inventory(listing, self.project, self.identity, self.selected, self.foreign, present)
        state, files = runtime_state(self.runtime, self.temporary, self.baseline)
        git_files, _ = _temporary_git_state(self.profile, phase=phase)
        self.guard()
        if self.baseline is None:
            self.baseline = state
        self.observations.append({"phase": phase, "selected_skill_present": present,
                                  "foreign_skill_present": True, "native_exit_code": 0,
                                  "runtime_integrity_verified": True, "runtime_semantics_unchanged": True,
                                  "runtime_files_fully_read": files,
                                  "temporary_git_files_fully_read": git_files})
        return True

    def temporary_git_state(self):
        self.guard()
        state = _temporary_git_state(self.profile, phase="before-repeat")
        self.guard()
        return state

    def repeat_runtime_only(self, before, after, previous):
        self.guard()
        qualified = _repeat_temporary_git_only(self.profile, before, after, previous)
        self.guard()
        return qualified

    def evidence(self):
        return {"native_version": "0.153.4", "runtime_root": str(self.runtime.relative_to(self.case)),
                "runtime_baseline": self.baseline, "observations": self.observations}


if __name__ == "__main__":
    if os.name != "nt" or len(sys.argv) != 3 or sys.argv[1] != "--windows-native-start":
        raise SystemExit("private Windows native observer bootstrap required")
    # Exactly one unbuffered byte: no JSON protocol input can be consumed here.
    if os.read(0, 1) != b"!":
        raise SystemExit("native observer process ownership was not established")
    raise SystemExit(subprocess.call([sys.argv[2], "app-server"], stdin=sys.stdin,
                                    stdout=sys.stdout, stderr=sys.stderr))
