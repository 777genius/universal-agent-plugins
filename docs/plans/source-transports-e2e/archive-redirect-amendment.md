# Archive downloads: transient signed HTTPS redirects

Owner authorization and both independent plan reviews completed on 2026-10-23.
This narrowly amends the downloader redirect policy in `sources-plan.md`.
The initial source grammar and persistent source identity remain unchanged.

## Concrete interoperability evidence

The parent worker followed the official public asset
<https://github.com/golangci/golangci-lint/releases/download/v2.13.2/golangci-lint-2.13.2-linux-amd64.tar.gz>
with an HTTPS HEAD request. The original URL had no query. Redirects reached
`release-assets.githubusercontent.com` over HTTPS with a query and final HTTP
200. Query values were neither printed nor saved. Rejecting every redirect
query therefore rejects an ordinary public release download despite a stable,
credential-free initial source.

## Approved policy

The initial and persisted archive URL must be credential-free HTTPS without a
query or fragment. The downloader may follow at most five transient redirects.
Each target must use verified HTTPS, have a nonempty host, have no userinfo,
fragment, backslash or control characters, and have a complete URL of at most
16 KiB. Signed path/query encoding is preserved without decoding, rewriting or
canonicalizing the signature input.

Every redirect clears all request headers, including Referer, Authorization,
Cookie and custom headers. The downloader rebuilds only its nonsecret
Accept-Encoding header. It uses no cookie jar, Git helper, netrc, browser or
session credentials, and does not inherit custom redirect callbacks. Host CA
transport configuration does not authorize disabling TLS verification.

The final target, Location header, response body and raw HTTP errors are never
included in user-visible errors or persistent metadata. Request failures report
only a sanitized phase/status. SourceURL, RequestedSource, the canonical source
key and lock intent keep the original stable endpoint and selected logical
package path. Original archive-byte SHA256 and tree/manifest verification still
apply. A redirected or repacked byte change does not evade a locked byte pin.

## Independent reviews and verification

R1 by `project_core` required a separate transient validator, bounded verified
HTTPS, no credential/header propagation, no secret persistence or diagnostics,
and original source/checksum preservation. R2 by `sources_lead` independently
returned GO under those conditions; its full parent-workspace ledger is
`research/sources-redirect-review-r2.md`. The parent recorded both GO decisions
before implementation.

`archive_compat_test.go` exercises real cross-origin TLS redirects to an opaque
signed query. It checks exact query encoding, no credential/Referer forwarding,
original-only serialized provenance, sanitized HTTP failure, excessive target
length, downgrade/userinfo/fragment refusal and strict initial query rejection.
Other archive tests retain byte-pin mismatch, hop-limit and cleanup coverage.
These are native HTTPS/archive transport fixtures, separate from native client
activation proof.
