# Milestone A public channel verification

Manually dispatch `Milestone A public channels` after publishing the pinned
releases. This workflow installs real npm, PyPI, Homebrew and GitHub packages;
it never builds a substitute from checkout. The checkout supplies only the test.
It expects universal-agent-plugins 0.1.61, plugin-kit-ai 2.0.1, and source revision
05d1f19796b257b1f0544464145d2653f721e21a. Each matrix cell retains command output,
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

Known qualification gaps: the pinned source command list has no `pack` command.
This requested check deliberately fails rather than silently substituting `test`
or introducing deferred export/bundle functionality. Remaining commands and the
second entrypoint still run after a command failure. On 2026-09-13, a direct GET
of https://registry.npmjs.org/universal-agent-plugins/0.1.61 returned HTTP 404.
No public-channel success or cross-platform execution is claimed by static tests.
Dispatch after publication is necessary to establish executable evidence.
