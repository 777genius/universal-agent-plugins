# Skills catalog operations

This is a deployment recipe, not evidence that deployment has occurred. The
service is separate from the static GitHub Pages landing site. Only
`api.universal-agent-plugins.com` routes to this service; the apex and `www`
remain on GitHub Pages. The API contract is
[skills-catalog.openapi.yaml](contracts/skills-catalog.openapi.yaml).

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

- SSH identity/address: `iliya@37.27.125.57`.
- Hostname: `prod-hel1-01`.
- Machine ID: `b28fc7b17042414386eb9b114046e50c`.
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
go test ./internal/skillscatalog ./cmd/skills-catalog-api
CGO_ENABLED=0 GOOS=linux GOARCH="$CATALOG_GOARCH" go build -trimpath -buildvcs=false -o "$CATALOG_BUILD_DIR/skills-catalog-api" ./cmd/skills-catalog-api
shasum -a 256 "$CATALOG_BUILD_DIR/skills-catalog-api"
file "$CATALOG_BUILD_DIR/skills-catalog-api"
```

`CATALOG_GOARCH=amd64` is an example, not an assertion about the destination.
Do not label an artifact with `CATALOG_SHA` if the source has uncommitted edits.
No Node runtime, Docker, database, tokens, or application secrets are needed
on the destination. HTTPS access to `www.skills.sh`, working DNS, and the
system CA trust store are required. Never expose an upstream-origin override
or caller-supplied proxy URL.

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

## Persistence, limits, and normal operations

The unit uses `DynamicUser` and `StateDirectory=uap-skills-catalog` with mode
0700. Systemd preserves the state directory across stops and restarts and manages
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
256 MiB cgroup hard limit, and task/file-descriptor limits. The memory target is
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
