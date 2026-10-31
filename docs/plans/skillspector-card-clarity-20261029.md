# SkillSpector card and detail clarity

The Directory currently shows “Static check not applicable” and installation
advice on plugins whose declared components contain no Skills. That repeats an
irrelevant check and implies a missing assessment. Keep the change within the
existing card, assessment component, feature translations and browser coverage.

1. Mount SkillSpector on plugin list cards only when the plugin declares Skills.
   Keep the existing installation and listing review behavior.
2. On plugin detail pages, explain known Skill-free packages with “No Skills to
   check” and a brief scope sentence. Do not request reports, display installation
   advice, or reuse the unsupported-source state for that case.
3. State that applicable plugin reports check Skill files, not other components.
   Present existing fully checked counts in plain language, retaining the
   distinction between known plugin inventory and selected Skills. Do not show
   an empty “0 of 0” coverage count or change assessment/scanner semantics.
4. Update the affected copy in all nine supported locales. Preserve real
   findings and loading, unavailable, pending, historical, partial, complete,
   and unsupported-source distinctions.
5. Update the obsolete MCP-only browser expectations. Cover list/detail behavior
   with no report requests, English/Russian desktop/mobile layouts and keyboard
   access. Run the existing relevant report browser suites, registry tests,
   translation/lint/type checks and production build/artifact budgets where the
   available runtime permits; report any blocked checks explicitly.

Review follow-up authorized by the owner:

- Reuse the same assessment on community detail pages so following a community
  card preserves its source-bound report and scan scope.
- Match the producer's v3 package inventory budget for `plugin_states` and
  `plugin_coverage`: accept 10,000 community packages plus Directory packages,
  up to 20,000. Add consumer boundary checks for Skill-free package states and
  one-Skill pending coverage. Keep v2, per-package and shard/byte limits intact.

Remote review, merge and deployment are coordinated separately. This change
does not alter registry data, scanner coverage, installation or report schemas.
