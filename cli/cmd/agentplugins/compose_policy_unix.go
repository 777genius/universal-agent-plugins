//go:build !windows

package main

import "runtime"

func machinePolicyPath() (string, error) {
	if runtime.GOOS == "darwin" {
		return "/Library/Application Support/AgentPlugins/policy.yaml", nil
	}
	return "/etc/agentplugins/policy.yaml", nil
}
