# SkillSpector: retry failed report loading without misleading scan status

## Observed defect

The report client caches transient transport failures with a 30-second cooldown, but `useSkillSpector` retries only when the repository identity changes. An already-open card therefore cannot recover on user demand. The same "Scan unavailable" text is used for a network/report-loading failure and an actual failed static scan.

## Scope

1. Keep the published-scan result model and server production untouched.
2. Distinguish **Report unavailable** (data could not be loaded) from **Scan unavailable** (a reported failed scan) in all nine locales.
3. Offer an accessible **Try loading again** action only for a recoverable transport response. Retry is explicit; do not poll or run a new security scan.
4. Reuse valid cached manifest/shard responses; invalidate only failed transient cache entries, with in-flight deduplication preserved. Do not retry invalid schemas, mismatched identity, hash or byte-count failures.
5. Preserve card disclosure, error/loading state, install actions and source-specific safety wording.

## E2E acceptance

- A temporary 503 on an actual report fetch must show "Report unavailable"; a user retry after recovery must display a valid report without navigation/reload.
- A structurally malformed report must still fail closed and offer no retry.
- Actual `status=error` reports must retain **Scan unavailable**, distinct from transport failures.
- Verify EN/RU, keyboard and mobile interactions, no duplicate concurrent retry requests, and no regression in nine locale validation.
- Run CI on the exact PR head, independently review the patch, fix findings and merge only after green checks. A live deployment check is required before calling it production accepted.

## Avoid

No new service, no persistent client settings, no timer/polling loop, no schema changes or blanket cache clear.
