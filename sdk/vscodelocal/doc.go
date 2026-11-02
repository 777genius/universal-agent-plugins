// Package vscodelocal exposes public-beta typed observers for VS Code's Local
// harness. Stop observes an execution about to stop, not task success, idle state
// or a completed root session. SubagentStop observes a subagent about to stop.
//
// Register with App.VSCodeLocal and dispatch with App.RunContext using explicit
// Config.IO and Config.Env. Runtime identity vscode-local is separate from
// installer identity vscode and Agent Notifications identity copilot-vscode.
// These APIs do not install hooks, qualify native support or supervise processes.
package vscodelocal
