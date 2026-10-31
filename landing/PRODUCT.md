# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

People using coding agents who want to find a useful plugin or Agent Skill and install it without learning repository layouts or installer internals.

## Product Purpose

Universal Agent Plugins offers one catalog and installer journey for plugins and Skills. Success is finding an item, choosing relevant agents when installing, and running the exact command shown.

## Operating Context

The public Nuxt site is generated statically. The CLI installs public GitHub Skills through an existing scanner, explicit review and a saved, versioned environment. The Skills discovery API is a separate service on the owner's Hetzner production host. The owner supplied universal-agent-plugins.com; the proposed API subdomain is api.universal-agent-plugins.com.

## Capabilities and Constraints

- Plugins and Skills are distinct catalog modes with URL-addressable selection.
- Reuse the site's existing controls, agent selector, command presentation and visual language.
- Skills discovery initially uses skills.sh through a replaceable provider contract and last-known-good caching.
- Show installation choices after the user chooses to install an item. Do not put implementation details into the visitor flow.
- Preserve GitHub source identity; do not invent a skill folder from a registry slug. The CLI checks the selected source before installing.
- Desktop is the current priority. Replacing the React CLI picker with shared landing components is a future phase, not this catalog extension.
- Preserve plugin installation behavior, existing availability boundaries and supported-client metadata.

## Product Principles

- Let people browse before asking where to install.
- Keep equivalent controls and actions consistent across plugins and Skills.
- Preserve a useful cached catalog during upstream outages and show its age honestly.
- Use real source metadata and real installation commands; do not imply that a listing establishes compatibility or verification.
