// Package grok reserves the Grok Build target identity in the internal platform tree.
//
// Grok is currently scaffold/generate/import/validate only. It intentionally has
// no runtime event implementation in the SDK yet, but it still needs a distinct
// internal package so descriptor metadata does not alias existing platform internals.
package grok
