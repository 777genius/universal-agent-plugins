# BUG-054 — live Devin project Skills use incompatible vendor version

## Reproduced on PR HEAD `64d437c8deb0f17e5f5767c829c1a36e12cdd771`

The original [Windsurf/Devin Skills E2E artifact #13399170688](https://github.com/777genius/universal-agent-plugins/actions/runs/41992237917/artifacts/13399170688), produced by source SHA `64d437c8`, contains 30 distinct project Skill cases with nonzero `PLAN_REJECTED`, zero project/native registrations and preserved foreign files. Verified vendor binary `devin --version` returned **3000.11.234**. In production `install/integrationctl/agentplugins/clients/windsurf/project_validation.go`, `requireCurrentDevin` explicitly permits **exactly 3000.11.205** for the project native configuration lane. This establishes a test fixture version mismatch; it does not establish a product adapter bug or a successful install.

Independent immutable official-release proof: [Homebrew Cask commit `b0b0899868cd5816cfc754d2461a50920af612e0`](https://github.com/Homebrew/homebrew-cask/blob/b0b0899868cd5816cfc754d2461a50920af612e0/Casks/d/devin-cli.rb) lists Linux x86_64 archive `https://static.devin.ai/cli/3000.11.205/devin-3000.11.205-x86_64-unknown-linux.tar.gz`, SHA-256 `2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4`. The already verified 3000.11.234 user-plugin release remains pinned separately.

## Minimal scoped plan and two critical passes

**Correctness critic:** Change only the **Windsurf project Skill** (`KIND=skill`) URL and SHA in the existing GitHub workflow, leaving `KIND=plugin` on pinned 3000.11.234. Production exact-version admission, adapters, clients, fixtures and source identities remain untouched. Add a no-network regression asserting two distinct branches and source digests. A successful vendor version probe is **not** an install PASS; inspect 30 new original native JSON results before changing outcomes.

**Security critic:** Keep independent immutable Homebrew provenance, pre-extraction SHA verification, `tarfile.extractall(filter="data")`, exact regular executable, low-priority symlink into disposable HOME, trusted toolchain PATH and `env -i` profile/secret isolation. Never run an unpinned release, vendor shell installer, paid models or unknown user client profile. No LintAI/cgroup/symlink/identity relaxation or new feature flags. A wrong digest, auth requirement or unsupported native behavior remains a real non-PASS.

## Acceptance

- Required, Core fast/focused Python regression and all 30 original ZIP/900 outcomes on the **new exact HEAD**. Verify native version, install/list/lock/repeat/remove and foreign sentinels with 30 distinct Skill sources.
- Record any new refusal or native-unqualified outcome without upgrading it to PASS; investigate only independently reproduced defects. Resolve all independent CodeRabbit/Strix security review gates before merge.
- Existing 64d437c8 900-case counts (549 PASS, 351 non-PASS) are **historical** after this workflow change.
