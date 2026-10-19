// Package kimi houses the native Kimi Code bundle codec for the scaffold-only
// plugin lane: it models the observed .kimi-plugin/plugin.json manifest shape,
// with MCP servers inlined directly into the manifest because Kimi does not
// resolve external MCP file refs. Kimi exposes no first-class hook runtime, so
// this package carries manifest/shaping logic only; runtime hook dispatch stays
// unsupported until Kimi ships an authorable hook surface.
package kimi
