# Curated catalog UX: practical first, full discovery preserved

Date: 2026-11-08

## Problem

- Reviewed plugins default to alphabetical order: Atlassian and five Cloudflare variants consume the first screen while general-use tools appear later.
- Many reviewed plugin descriptions lead with packaging provenance (for example, "Community package for the official...") instead of the task a visitor can accomplish.
- Skills listings mirror external skills.sh install-count order and contain no short description field; the default page can be dominated by one publisher, and popularity is not a quality or security signal.
- Installation, OAuth, activation and static-scan evidence must remain separate; neither a curated placement nor a schema pass certifies runtime safety.

## Decision and bounded implementation

1. Add a **static, reviewed-only editorial ranking** for common coding-agent workflows (GitHub, browser testing, current docs, design, issues, debugging, data). Keep a stable alphabetical tail, and add a user-controlled **Name A–Z** option. Search remains relevance-first, Discovery remains distinct, and existing filters, page size, URL state and install behavior remain intact.
2. Add a **presentation-only, accurate benefit-led summary** for the maintained reviewed products (with fallback to signed metadata for new/unknown and Discovery packages). Show the same summary on cards and product details/SEO. Do **not** mutate signed package identity, permissions, source links, author attribution, installation provenance or Directory publication artifacts.
3. Add a small **editorially selected Skills row** with exact GitHub source identity, honest brief descriptions and a link to each existing Skill detail journey; keep the unfiltered external ranking underneath. It is not a safety badge and does not imply availability of external runtimes or credentials. Avoid fabricated descriptions for other Skills because the upstream list has no description field.
4. Clearly explain that curation reflects broad utility rather than technical verification. Localize new UI copy in nine supported site languages, retaining the current nonlocalized signed descriptions as fallback.
5. Add deterministic unit checks and browser assertions for default order, alphabetical fallback, search precedence, safe source linking, filtering/URL selection, and mobile layout.

## Ranking policy

- A maintained small list of product IDs, **not** a dynamic algorithm that guesses utility from stars or install counts.
- Curate for broad usefulness, diversity of tasks, clear value proposition and low setup friction; exclude retired/temporarily unlisted products through existing visibility rules.
- Use item identity, never user-supplied titles or package URLs, as the recommendation key.
- User can switch to alphabetical sorting. Query search always uses existing relevance rules. Unlisted and unavailable entries never become installable because of sorting.
- Reevaluate the editorial shortlist during product review; all non-shortlisted items remain discoverable.

## Acceptance

- Home and directory: useful reviewed packages lead by default; Name A–Z opt-in restores previous order; search still sorts by relevance, with exact reviewed names first.
- No five Cloudflare variants among first reviewed cards by default; a Cloudflare search or filter still finds every item.
- Reviews and installation claims are unchanged.
- All first-party reviewed card blurbs lead with what the user can do; detail descriptions/SEO agree; discovered items remain source-authored.
- Skills section shows curated practical picks with exact repository identity and honest safety disclaimer; Popular, Trending and Hot views remain provider-driven and retain existing pagination, evidence filters, install paths and copy behavior.
- No new network calls for featured content; no install counts or findings invented.

## Engineering trade-off (10-point scale)

| Approach | Benefit | Complexity | Reliability | Scope/estimated LOC | Decision |
| --- | --- | --- | --- | --- | --- |
| Curated overlay + static priorities | 9/10 | 3/10 | 9/10 | 300–550 | **Choose** |
| Change signed Directory metadata/release | 5/10 | 8/10 | 5/10 | 600–1300 | Reject: publication and security risk |
| Network-ranking / machine-learning score | 5/10 | 9/10 | 4/10 | 800–1800 | Reject: opaque and difficult to audit |
| Fetch SKILL.md for every visible card | 6/10 | 7/10 | 4/10 | 450–1000 | Defer: excessive calls and unknown provenance |

## Review passes

1. **New visitor / UX:** first screen must answer "what does this do?" without hiding full discovery or making everyone install one stack.
2. **Security / provenance:** no implied endorsement, no new install authority, exact source IDs, static-scan vs runtime claims remain distinct.
3. **Architecture / maintenance:** pure data + sorting boundary, no provider API edits, no ad hoc special cases in components, deterministic tests, no feature flag.

## Out of scope

No upstream skills.sh changes, no installing/running third-party Skills, no changes to signed registry or immutable release decisions, no new third-party dependency, and no rewrite of CLI/install architecture.
