# Milestone A public channel verification

Manually dispatch `Milestone A public channels` once the requested releases are available. This workflow installs real npm, PyPI, Homebrew and GitHub packages;
it never builds a substitute from checkout. The checkout supplies only the test.
Dispatch requires exact versions and GitHub tags for both products plus their
expected full source revision; supply the final patch pair when available. Each matrix cell retains command output,
including failure evidence. Registry requests have four attempts with bounded
backoff; individual commands and the complete job also have deadlines.

Run focused checks with:

    PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts -p test_public_channel_e2e.py

The helper creates and cleans fresh temporary homes, package caches, install
prefixes and plugin.json projects. It uses a private Homebrew checkout and prefix
instead of changing an existing installation. Nonstandard Homebrew prefixes may
require source builds; installation failure is reported, never replaced with a
local executable. Homebrew must still expose the exact requested formula version.
Only Linux amd64, macOS arm64 and Windows amd64 runners are exercised here.
Both public entrypoints are checked on npm and GitHub; PyPI and Homebrew provide
plugin-kit-ai. GitHub native payloads are checked against release checksums, then
all channels must report the pinned product version and source revision.

The acceptance journey is version/revision readback, init, validate, inspect,
compat and offline static test in a disposable plugin project. A command failure
is retained while remaining checks and the second entrypoint continue.
No public-channel success or cross-platform execution is claimed by unit tests.
Dispatch against available releases is necessary to establish executable evidence.

Historical release evidence (2026-09-13): a real Linux run verified both GitHub
release binaries for agentplugins 0.1.61 and plugin-kit-ai 2.0.1 against checksums,
versions and revision 05d1f19796b257b1f0544464145d2653f721e21a. Both entrypoints
passed init, validate, inspect and compat. PyPI 2.0.1 installed, but its launcher
requested the historical `777genius/plugin-kit-ai` repository's
`v2.0.1/checksums.txt`, which returned 404. Both exact npm version endpoints
returned 404. These remain external release blockers, not test-design blockers
or successful channel qualification. Windows, macOS and Homebrew execution
remains unverified. No publication or documentation cutover is performed here.
