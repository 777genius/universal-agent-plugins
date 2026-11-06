# BUG-050 — VS Code full-native matrix blocked by mutable, unverified editor executable

## Original evidence and independent provenance

The historical 30×30 audit and present-source workflow label both VS Code source lanes `client-unavailable` because the vendor bootstrap downloads a mutable `latest` Linux `.deb` without an independently approved hash. This is the correct fail-closed result, but it attempts **zero** of 60 native source lifecycles and cannot satisfy actual VS Code E2E.

The official [Visual Studio Code 1.144 release notes](https://code.visualstudio.com/updates/v1_144) identify stable **1.144.0**, released October 28, 2026. [Microsoft's official download SHA-256 table](https://code.visualstudio.com/download) published the exact **Linux .deb (x64)** digest:
`b8fbff92d8d95908990eed0acf004c980f29003725469e794ddbd017ea511a20`.
The [independent AUR 1.144.0 → 1.144.1 update diff](https://aur-review.com/versions/3027) independently identifies that same digest as the **1.144.0 x86_64 .deb**, and shows the different `1.144.1` digest. Do **not** confuse these versions or copy a hash from the CLI or tar.gz artifact.

The official version-bound distribution endpoint is `https://update.code.visualstudio.com/1.144.0/linux-deb-x64/stable`, consistent with Microsoft's [documented release URL scheme](https://code.visualstudio.com/docs/supporting/faq). The workflow must check the exact digest before extraction, then observe the **real** editor executable and its version on a disposable profile. A download mismatch or missing executable is an explicit non-PASS; these pins do not grant user login or native plugin activation.

## Scoped fix

1. Replace only the VS Code `latest` URL with the documented `1.144.0` endpoint and independently published digest. Keep the existing fail-closed SHA-256 gate **before** `dpkg-deb --extract` and execution; no `sudo`, `curl | sh` or package maintainer-script execution.
2. Strengthen the common vendor `--version` subprocess with `env -i` and an exact minimal allowlist (`PATH`, disposable `HOME`/`USERPROFILE`/XDG roots, disposable `TMPDIR`, `LANG`, `CI`, `DO_NOT_TRACK`). Never inherit GitHub Actions tokens, proxy credentials, arbitrary client profile overrides, Git injection knobs or SSH agents. This applies to already-digest-pinned Kiro and Cursor Agent, too.
3. Preserve every source, signed-release pin, repeat add/list, native-file attribution, removal, and foreign-neighbor oracle. A successful `code --version` alone does **not** make a single source case PASS. Incomplete activation remains non-PASS.
4. Inspect original VS Code **Plugin/Skill** ZIPs and all final-head required CI after the new commit. Handle genuine OS dependency or GUI constraints as explicit native gaps, never as fabricated success.

## Two review perspectives

**Safety:** Microsoft download digest plus independent version evidence prevent executing a mutable unverified `.deb`. Removing inherited process env from native version probes narrows attacker access to runner credentials and out-of-scope profiles. Abort before extraction or execution on any mismatch.

**Correctness:** Pinning an authentic GUI editor can still fail to provide headless native discovery, project Skills, or a logged-in model. The classifier must distinguish real lifecycle PASS, prepared/manual, unsupported and client unavailable. Do not modify production adapter behavior or security containment to make the matrix green.

## Status

Source and digest research completed; exact final-HEAD real GitHub-runner results and independent external security review remain necessary. No PASS or merge claimed by this document.
