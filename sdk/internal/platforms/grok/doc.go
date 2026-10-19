// Package grok houses the native Grok Build bundle codec for the scaffold-only
// plugin lane: it models the observed .grok-plugin/plugin.json manifest shape and
// the wrapped .mcp.json MCP sidecar projection. Grok exposes no first-class hook
// runtime, so this package carries manifest/shaping logic only; runtime hook
// dispatch stays unsupported until Grok ships an authorable hook surface.
package grok
