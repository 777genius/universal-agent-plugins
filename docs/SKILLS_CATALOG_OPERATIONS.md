# Skills catalog operations

This is a deployment recipe, not evidence that deployment has occurred. The
service is separate from the static GitHub Pages landing site. Only
`api.universal-agent-plugins.com` routes to this service; the apex and `www`
remain on GitHub Pages. The API contract is
[skills-catalog.openapi.yaml](contracts/skills-catalog.openapi.yaml).
The optional service-only source preview has a separate
[skills-resolve.openapi.yaml](contracts/skills-resolve.openapi.yaml) contract;
it does not alter the anonymous listing DTO or landing pages.
The anonymous source-only `GET /v1/skills/document` endpoint uses the
[skills-document.openapi.yaml](contracts/skills-document.openapi.yaml) contract.
It exposes at most 64 KiB of exact `SKILL.md` UTF-8 directly from the publisher,
never install resources, without relying on the listing provider's detail API.
Git previews require the existing isolated resolver runtime, and HTTPS publishers
must have their own complete-file discovery index. Missing, nonconformant,
ambiguous or unavailable source content is reported honestly rather than guessed.
Browsers render the text with HTML disabled, image embeds disabled, only safe
absolute links, and no third-party directory backlinks.

Regenerate the landing DTOs after changing the contract:

```sh
npm exec --yes --package=openapi-typescript@7.13.0 -- openapi-typescript docs/contracts/skills-catalog.openapi.yaml -o landing/types/skillsCatalog.generated.ts
```

The generator is pinned for reproducibility and is a development tool only.
The provider adapter uses anonymous skills.sh website/CLI endpoints, which are
undocumented and may change. Its interface and cached last good results contain
that dependency; the authenticated `/api/v1` service is not used.

## Destination and owned paths

Deployment inventory supplied for this service:

- SSH identity/address: `<operator>@<host>` from the private trusted host inventory.
- Hostname: expected value from the trusted host inventory.
- Machine ID: expected value from the trusted host inventory.
- Binary: `/srv/platform/projects/universal-agent-plugins/releases/<sha>/skills-catalog-api`.
- Active release: `/srv/platform/projects/universal-agent-plugins/current` symlink.
- Unit: `/etc/systemd/system/uap-skills-catalog.service`.
- Route: `/etc/caddy/projects/skills-catalog.caddy`.
- Private persistent state: `/var/lib/uap-skills-catalog`.

Verify the configured SSH host key through the trusted host inventory, then
check the hostname and `/etc/machine-id` before remote changes. Determine the
actual architecture with `uname -m`; map `x86_64` to Go `amd64` and `aarch64` to
`arm64`. Stop on an unexpected identity or architecture. No existing UAP
service or release root is assumed. Do not replace another project's files.
Caddy already imports `/etc/caddy/projects/*.caddy`; do not rewrite its shared
configuration or open a new public port. DNS provisioning is separate: the
new API hostname must resolve to this server, including checking any existing
AAAA record. Do not alter the website's DNS records.

## Build and record the exact artifact

Build from the reviewed release commit, with the relevant checks passing on
that exact source. Use a job-owned build directory, outside the source tree.
Set `CATALOG_GOARCH` to the architecture verified above. Record the full commit,
Go version, architecture, binary SHA-256, and focused test result with the
release evidence. The hash is specific to the actual binary, not the branch
name or this template. These commands run on the build host:

```sh
set -eu
CATALOG_SHA=$(git rev-parse HEAD)
CATALOG_GOARCH=amd64
CATALOG_BUILD_DIR=$(mktemp -d)
go version
go test ./internal/skillscatalog ./internal/skillsresolve/... ./cmd/skills-catalog-api
GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH="$CATALOG_GOARCH" go build -mod=readonly -trimpath -buildvcs=false -o "$CATALOG_BUILD_DIR/skills-catalog-api" ./cmd/skills-catalog-api
shasum -a 256 "$CATALOG_BUILD_DIR/skills-catalog-api"
file "$CATALOG_BUILD_DIR/skills-catalog-api"
```

`CATALOG_GOARCH=amd64` is an example, not an assertion about the destination.
Do not label an artifact with `CATALOG_SHA` if the source has uncommitted edits.
Anonymous catalog browsing needs no Node runtime, Docker, database, tokens or
application secrets. HTTPS access to `www.skills.sh`, working DNS, and the
system CA trust store are required. Optional source resolution also needs Git,
HTTPS access to `github.com`, and its own narrow service capability. Never
expose an upstream-origin override or caller-supplied proxy URL. The root module
declares local replacements for the existing source-reader workspace modules,
so the binary above also builds reproducibly with `GOWORK=off` from this checkout.

## Stage and activate

Use a verified transfer into a unique operator-owned staging directory. Compare
its SHA-256 on the destination with the recorded build hash before installation.
Run the following activation steps in a root shell on the verified destination;
`CATALOG_STAGING` must contain the reviewed binary and the two templates from
`deploy/skills-catalog-api/`. Set `CATALOG_SHA` to the full reviewed commit.
Do not overwrite an existing release directory: compare and reuse an identical
artifact or stop to resolve the mismatch. Before running these commands, record
the previous `current` symlink target and save the prior versions of this
service's unit and route in the release evidence directory. Record absent files
and targets explicitly on first install. Check the staged unit's intended diff
before replacing an existing unit.

```sh
set -eu
CATALOG_ROOT=/srv/platform/projects/universal-agent-plugins
CATALOG_RELEASE="$CATALOG_ROOT/releases/$CATALOG_SHA"
test ! -e "$CATALOG_RELEASE"
install -d -o root -g root -m 0755 "$CATALOG_ROOT" "$CATALOG_ROOT/releases" "$CATALOG_RELEASE"
install -o root -g root -m 0755 "$CATALOG_STAGING/skills-catalog-api" "$CATALOG_RELEASE/skills-catalog-api"
sha256sum "$CATALOG_RELEASE/skills-catalog-api"
```

Verify that every parent of the release binary is root owned and not writable
by the service. The binary is installed before switching `current`; unit
verification follows the switch so its executable exists on first install.

Activate using a fresh temporary symlink name on the same filesystem, then
rename it over `current` atomically. This example uses a release-specific name
that must be absent before creation:

```sh
set -eu
CATALOG_NEXT="$CATALOG_ROOT/.current-$CATALOG_SHA"
test ! -e "$CATALOG_NEXT"
test ! -L "$CATALOG_NEXT"
ln -s "$CATALOG_RELEASE" "$CATALOG_NEXT"
if test -e "$CATALOG_ROOT/current" && ! test -L "$CATALOG_ROOT/current"; then
  exit 1
fi
mv -Tf "$CATALOG_NEXT" "$CATALOG_ROOT/current"
install -o root -g root -m 0644 "$CATALOG_STAGING/uap-skills-catalog.service" /etc/systemd/system/uap-skills-catalog.service
systemd-analyze verify /etc/systemd/system/uap-skills-catalog.service
systemctl daemon-reload
systemctl enable uap-skills-catalog.service
systemctl restart uap-skills-catalog.service
systemctl is-active uap-skills-catalog.service
curl --retry 5 --retry-delay 1 --retry-connrefused --fail --silent --show-error http://127.0.0.1:13120/healthz
curl --fail --silent --show-error 'http://127.0.0.1:13120/v1/skills?view=popular&page=0'
```

If unit verification fails, stop before daemon reload or restart and restore
the recorded prior unit/current target. A health response proves the process is
serving. Before routing public traffic, check that a catalog request produces valid items or a truthful structured 503;
a 503 with no last good cache is not a successful catalog qualification.
The service listens only on loopback; do not add a firewall rule for port 13120.

## Publish the dedicated route

After backend checks and DNS verification, install only this route. Validate
the complete existing Caddy configuration, including imports, before reloading:

```sh
set -eu
install -o root -g root -m 0644 "$CATALOG_STAGING/skills-catalog.caddy" /etc/caddy/projects/skills-catalog.caddy
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
systemctl reload caddy
curl --fail --silent --show-error https://api.universal-agent-plugins.com/healthz
curl --fail --silent --show-error --dump-header - -H 'Origin: https://universal-agent-plugins.com' 'https://api.universal-agent-plugins.com/v1/skills?view=popular&page=0'
```

Run the reload only if validation succeeds. On validation failure, restore this
route's prior bytes or remove this newly introduced file, then validate again;
do not reload the failed candidate. Verify the system's Caddy service reloads
`/etc/caddy/Caddyfile` rather than a different config. Public acceptance also
checks HTTPS, exact `Access-Control-Allow-Origin`, `cache.status`, `fetchedAt`,
all three views, and a two-character-or-longer search. Search omits a complete
`total` and returns a bounded result set. A disallowed Origin receives 403;
there is no credentialed CORS. Do not log full query responses or client headers
as routine operational telemetry.

Caddy's [validation command](https://caddyserver.com/docs/command-line#caddy-validate)
checks loading and provisioning, a stronger check than merely adapting the file.

## Public Skill document preview checks

The new public route is source-derived, not an arbitrary URL proxy: the query
takes only the canonical listing identity and Skill name, not a user-provided
URL or Git revision. At most four previews may be active per process (one per client),
with four global tokens initially and one token replenished every five seconds.
The dedicated Caddy route overwrites the client address header, preventing
untrusted X-Forwarded-For values from bypassing per-client limits.
Each client starts a fresh preview at most once every twenty seconds.
Successful source documents are cached for ten minutes (at most 128 entries),
and failed source coordinates cool down for thirty seconds. The hosted Git
reader continues enforcing its read-only, no-credentials runtime envelope.
HTTPS publishers use a separate public-IP-pinned TLS transport, with no
redirects or proxy inheritance, a strict declared file list and a 64-KiB
document limit. Raw text is untrusted and does not imply security or installability.

Verify the exact deployed binary and route together:

```sh
curl --fail --silent --show-error --dump-header - \
  -H 'Origin: https://universal-agent-plugins.com' \
  'https://api.universal-agent-plugins.com/v1/skills/document?id=skills-sh%3Aopen.feishu.cn%3Alark-mail&name=lark-mail'
```

The response may truthfully be a limited/failed preview when publisher content
or runtime availability changes. A 200 with `status: resolved` must contain
source-accurate Markdown; verify a known GitHub Skill and a known HTTPS
publisher, as well as invalid-origin 403, unsafe-ID 400, overload 429,
unavailable 503 and absence of all credentials/cookies in outbound reads.
No remote writes are performed by this operation.

## Persistence, limits, and normal operations

The unit uses `DynamicUser` and `StateDirectory=uap-skills-catalog` with mode 0700. Systemd preserves the state directory across stops and restarts and manages
its ownership. With `DynamicUser`, the host may expose it through a symlink into
`/var/lib/private`; do not manually chown, move, replace, or clear it. See the
[systemd directory contract](https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#StateDirectory=).

After one successful browse, record its `fetchedAt`, restart only this service,
and request the same page within its 30-minute freshness window. The timestamp
should remain unchanged and the page should still be fresh. This verifies reuse
of persisted cache without blocking the real upstream or changing shared network
rules. Upstream outage behavior is covered by the focused local tests; do not
fault-inject on the production host. Search freshness lasts ten minutes. Valid
stale data remains usable up to seven days from its last successful fetch;
malformed, timed-out, rate-limited, or failed refreshes do not overwrite it. With
no usable cache, the API returns structured 503 and a retry delay.

The cache holds at most 128 records with at most 2 MiB per response. Fetches have
bounded concurrency and rate, and upstream 429 retry delays apply. No cron or
refresh daemon is required. The unit's Go memory target is 192 MiB, with a
512 MiB cgroup hard limit, and task/file-descriptor limits. The memory target is
soft; the [cgroup limit](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html#MemoryMax=)
can terminate the service if exceeded. Check normal browse/search traffic and
journal evidence before changing these limits.

```sh
set -eu
systemctl status --no-pager uap-skills-catalog.service
journalctl -u uap-skills-catalog.service -n 50 --no-pager
systemctl show uap-skills-catalog.service -p MemoryCurrent -p MemoryPeak -p NRestarts -p ActiveState
```

After the bounded restart budget is exhausted, diagnose the failure before using
`systemctl reset-failed uap-skills-catalog.service` and restarting. Keep previous
release binaries and deployment evidence for rollback. Do not run
`systemctl clean` or remove the state directory during a normal update.

## Optional immutable source preview

This feature is disabled until the operator supplies a dedicated capability in
the root-owned mode-0600 `/etc/uap-skills-catalog/resolver.env` file:
`SKILLS_RESOLVER_SERVICE_TOKEN` must contain 32–512 non-control bytes. Do not reuse
a GitHub token, an end-user session, or a database credential. Provision the same
capability only to the authorized backend consumer. Token provisioning and
production activation are separate operator actions, not performed by this
source change. Missing/invalid configuration leaves source preview unavailable
while existing anonymous search and health continue to work.

Deploy the maintained Caddy route together with this capability. Only the exact
`/v1/skills/resolve` path forwards its service `Authorization` header; cookies are
still removed. Its upstream response timeout is 40 seconds, allowing the bounded
resolver to finish within its 35-second server write deadline. All anonymous
catalog routes continue to strip both Authorization and Cookie and keep the
12-second response timeout. A local resolver success followed by public 401 can
mean that an older proxy route is still stripping the service capability.

Use the reviewed unit on a Linux cgroup-v2 host, with Git and CA certificates
installed. It mounts a **private 256 MiB tmpfs** at
`/run/uap-skills-resolver`, selected by `SKILLS_RESOLVER_TEMP_DIR`; it does not
mount or clean the host's temporary directory. Keep this dedicated mount separate
from `/tmp`: [systemd 255 implies `PrivateTmp` for `DynamicUser`](https://www.freedesktop.org/software/systemd/man/255/systemd.exec.html#DynamicUser=),
which can shadow a `TemporaryFileSystem=/tmp` mount with host-backed private
storage. The resolver correctly refuses that unbounded filesystem. The service is unprivileged and has a
512 MiB cgroup memory limit, 64-task limit and one-CPU quota. `NewHosted` verifies
the temporary filesystem type/size and the actual current cgroup memory/PID
limits before enabling the endpoint. Unsupported hosts fail unavailable. The
unit's Go heap target remains 192 MiB because tmpfs and Git also count toward the
memory ceiling. Keep **one service replica**: resolver admission is one active
request per process, burst two with one token added every five seconds. Do not
scale this recipe without revisiting the aggregate capacity limit.

The source-resolution CI exercises both the Docker envelope and a disposable
systemd service derived from this unit. The systemd probe uses the unit's scratch
environment and mount settings, checks actual cgroup admission, fills the tmpfs
until `ENOSPC`, then confirms scratch is writable after cleanup. It uses no
production token, cache or listener.

`POST /v1/skills/resolve` accepts no browser Origin, query string, arbitrary URL,
credentials, CLI arguments or local path. It requires its service bearer token
and a strict JSON body of at most 8 KiB. Unknown/duplicate fields and null values
are errors. Initial discovery resolves the public GitHub default branch once;
explicit selection requires the returned full commit and actual skill directory
path. The empty path means repository root. This handler never resolves a moving
branch again during explicit selection. Missing/ambiguous/partial discovery is
reported truthfully; the consumer must not guess a directory from a listing slug.

Runtime bounds are 20,000 tree entries, depth 32, 128 candidates, 4 MiB metadata,
64 KiB per document, 2 MiB aggregate document reads and 256 KiB encoded response.
The overall HTTP budget is 30 seconds; Git work stops early enough to retain five
seconds for process-tree termination, inside the 35-second server write timeout.
Git uses filtered immutable trees without checkout, HTTPS only, no redirects,
and isolated configuration/credentials. Temporary files are removed on success
and failure. Filtering plus time/disk limits is not a total network-byte counter.
If process supervision cannot confirm Git cleanup, the hosted resolver refuses
all further Git launches until the service restarts; it does not release physical
capacity merely because a request returned an error. Diagnose the failure before
restarting. Anonymous catalog browsing remains available.

Only the original UTF-8 `SKILL.md` is returned, together with its SHA-256 and
immutable source identity. CRLF is preserved and invalid UTF-8 is rejected.
Symlinks, gitlinks and unsupported selected-tree kinds cannot produce a resolved
preview. Ordinary companion files are reported as present but their bytes are
not requested; this is advisory presence information, not a full bundle manifest.
Consumers must apply their existing text-dependency rules, disclose omissions,
and scan exactly the text they will import. Preview itself makes no safety claim.

Before activation, qualify `Skills source resolution` on the exact reviewed
commit. Its real Git tests exercise discovery, source movement, selected unsafe
entries, size limits and cleanup. Its second step runs a static Go probe in a
network-disabled, read-only `FROM scratch` container as UID 65532, under the actual
memory/PID/tmpfs limits, and fills that private filesystem until `ENOSPC`. It then
verifies cleanup restores space. This is isolated CI validation, not a production
fault-injection recipe. Record the run/commit and binary digest with the existing
release evidence. Do not count the ordinary test lane's skipped kernel probe as
passing; the dedicated container execution is required.

After operator activation, use the authorized backend consumer to verify a
known public immutable fixture and rehash the returned UTF-8 text. Also verify
unauthorized requests fail, browser Origin is rejected, and anonymous browsing
remains available. A health response alone does not qualify source resolution.
Never log the service token, raw source responses or Git diagnostics as routine
telemetry. HTTP errors are sanitized; 503 responses include a retry delay.

## Rollback this service only

For an update rollback, verify the prior release's recorded SHA-256, restore
this service's previous unit if it changed, atomically repoint `current` to the
recorded prior release, run `systemctl daemon-reload`, and restart only
`uap-skills-catalog.service`. Check loopback health and catalog behavior. Leave
the persistent cache intact. Restore this service's prior route if changed,
validate the full Caddy configuration, and reload Caddy only on success.

For first-install removal, remove only
`/etc/caddy/projects/skills-catalog.caddy`, validate the full Caddy configuration,
and reload Caddy only if it succeeds. Then disable and stop only
`uap-skills-catalog.service`. Preserve its binary, unit, state, and evidence until
the rollback is confirmed. Do not remove the release root, cache, shared Caddy
files, other services, website DNS, or GitHub Pages configuration. Removing the
API DNS record is a separate explicit action, not part of this template.
