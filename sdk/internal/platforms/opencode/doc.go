// Package opencode reserves the OpenCode target identity in the internal platform tree.
//
// OpenCode is currently scaffold/generate/import/validate only. It intentionally has
// no runtime event implementation in the SDK yet, but it still needs a distinct
// internal package so descriptor metadata does not alias existing platform internals.
package opencode
