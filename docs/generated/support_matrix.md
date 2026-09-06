# Generated Support Matrix

This generated table is the canonical per-event runtime reference for shipped support claims. Use it as exact contract data, not as front-door positioning. Package, extension, and repo-managed integration lanes remain summarized in SUPPORT.md and the target support matrix.

| Platform | Event | Status | Maturity | Contract Class | V1 Target | Invocation | Carrier | Transport Modes | Scaffold | Validate | Capabilities | Live Test | Summary |
|----------|-------|--------|----------|----------------|-----------|------------|---------|-----------------|----------|----------|--------------|-----------|---------|
| claude | Stop | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | stop_gate | claude_cli | Claude Stop command hook |
| claude | PreToolUse | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | tool_gate | claude_cli | Claude PreToolUse command hook |
| claude | UserPromptSubmit | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | prompt_submit_gate | claude_cli | Claude UserPromptSubmit command hook |
| claude | SessionStart | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | session_start | claude_cli | Claude SessionStart beta hook |
| claude | SessionEnd | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | session_end | claude_cli | Claude SessionEnd beta hook |
| claude | Notification | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | notify | claude_cli | Claude Notification beta hook |
| claude | PostToolUse | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | post_tool | claude_cli | Claude PostToolUse beta hook |
| claude | PostToolUseFailure | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | post_tool_failure | claude_cli | Claude PostToolUseFailure beta hook |
| claude | PermissionRequest | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | permission_request | claude_cli | Claude PermissionRequest beta hook |
| claude | SubagentStart | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | subagent_start | claude_cli | Claude SubagentStart beta hook |
| claude | SubagentStop | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | subagent_stop | claude_cli | Claude SubagentStop beta hook |
| claude | PreCompact | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | pre_compact | claude_cli | Claude PreCompact beta hook |
| claude | Setup | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | setup | claude_cli | Claude Setup beta hook |
| claude | TeammateIdle | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | teammate_idle | claude_cli | Claude TeammateIdle beta hook |
| claude | TaskCompleted | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | task_completed | claude_cli | Claude TaskCompleted beta hook |
| claude | ConfigChange | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | config_change | claude_cli | Claude ConfigChange beta hook |
| claude | WorktreeCreate | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | worktree_create | claude_cli | Claude WorktreeCreate beta hook |
| claude | WorktreeRemove | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | worktree_remove | claude_cli | Claude WorktreeRemove beta hook |
| gemini | SessionStart | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | gemini_session_start | gemini_extension | Gemini SessionStart hook |
| gemini | SessionEnd | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | gemini_session_end | gemini_extension | Gemini SessionEnd hook |
| gemini | BeforeModel | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | gemini_before_model | gemini_extension | Gemini BeforeModel hook |
| gemini | AfterModel | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | gemini_after_model | gemini_extension | Gemini AfterModel hook |
| gemini | BeforeToolSelection | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | gemini_before_tool_selection | gemini_extension | Gemini BeforeToolSelection hook |
| gemini | BeforeAgent | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | gemini_before_agent | gemini_extension | Gemini BeforeAgent hook |
| gemini | AfterAgent | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | gemini_after_agent | gemini_extension | Gemini AfterAgent hook |
| gemini | BeforeTool | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | gemini_before_tool | gemini_extension | Gemini BeforeTool hook |
| gemini | AfterTool | runtime_supported | stable | production-ready | true | argv_command_casefold | stdin_json | process | true | true | gemini_after_tool | gemini_extension | Gemini AfterTool hook |
| codex | Notify | runtime_supported | stable | production-ready | true | argv_command_casefold | argv_json | process | true | true | notify | codex_notify | Codex notify hook |
| codex | Stop | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | codex_stop | codex_notify | Codex Stop lifecycle hook |
| codex | SubagentStop | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | codex_subagent_stop | codex_notify | Codex SubagentStop lifecycle hook |
| codex | PreToolUse | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | codex_pre_tool_use | codex_notify | Codex PreToolUse hook |
| codex | PermissionRequest | runtime_supported | beta | runtime-supported but not stable | false | argv_command_casefold | stdin_json | process | true | true | codex_permission_request | codex_notify | Codex PermissionRequest hook |
