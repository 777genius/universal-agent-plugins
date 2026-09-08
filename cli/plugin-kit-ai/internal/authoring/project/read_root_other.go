//go:build !windows

package project

func readRoot(name string) (string, error) { return name, nil }
