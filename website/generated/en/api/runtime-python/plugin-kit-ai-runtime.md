---
title: "plugin_kit_ai_runtime"
description: "Generated Python runtime reference"
canonicalId: "python-runtime:plugin_kit_ai_runtime"
surface: "runtime-python"
section: "api"
locale: "en"
generated: true
editLink: false
stability: "public-stable"
maturity: "stable"
sourceRef: "python/plugin-kit-ai-runtime/src/plugin_kit_ai_runtime/__init__.py"
translationRequired: false
---
<DocMetaCard surface="runtime-python" stability="public-stable" maturity="stable" source-ref="python/plugin-kit-ai-runtime/src/plugin_kit_ai_runtime/__init__.py" source-href="https://github.com/777genius/universal-agent-plugins/blob/main/python/plugin-kit-ai-runtime/src/plugin_kit_ai_runtime/__init__.py" />

# plugin_kit_ai_runtime

Generated via pydoc-markdown.

# Table of Contents

* plugin\_kit\_ai\_runtime
  * JSONMap
  * ClaudeHandler
  * CodexHandler
  * CLAUDE\_STABLE\_HOOKS
  * CLAUDE\_EXTENDED\_HOOKS
  * allow
  * continue\_
  * ClaudeApp
    * \_\_init\_\_
    * on
    * on\_stop
    * on\_pre\_tool\_use
    * on\_user\_prompt\_submit
    * run
  * CodexApp
    * \_\_init\_\_
    * on\_notify
    * run

# plugin\_kit\_ai\_runtime

Official Python runtime helpers for plugin-kit-ai executable plugins.

#### JSONMap

JSON-shaped payload used by the Python runtime helpers.

#### ClaudeHandler

Handler signature for Claude hooks that return a JSON object or ``None``.

#### CodexHandler

Handler signature for Codex events that return an exit code or ``None``.

#### CLAUDE\_STABLE\_HOOKS

Stable Claude hook names supported by the public Python runtime lane.

#### CLAUDE\_EXTENDED\_HOOKS

Extended Claude hook names exposed by the beta Python runtime lane.

#### allow

```python
def allow() -&gt; JSONMap
```

Return the empty JSON object expected by Claude for an allow response.

#### continue\_

```python
def continue_() -&gt; int
```

Return exit code ``0`` for Codex handlers that want normal continuation.

## ClaudeApp Objects

```python
class ClaudeApp()
```

Minimal Claude hook app that dispatches supported hook names to handlers.

#### \_\_init\_\_

```python
def __init__(allowed_hooks: Iterable[str], usage: str)
```

Create a Claude runtime app.

**Arguments**:

- `allowed_hooks` - Hook names that this binary accepts on argv.
- `usage` - Usage string printed when the invocation is invalid.

#### on

```python
def on(hook_name: str) -&gt; Callable[[ClaudeHandler], ClaudeHandler]
```

Return a decorator that registers a handler for ``hook_name``.

#### on\_stop

```python
def on_stop(handler: ClaudeHandler) -&gt; ClaudeHandler
```

Register a handler for the ``Stop`` hook.

#### on\_pre\_tool\_use

```python
def on_pre_tool_use(handler: ClaudeHandler) -&gt; ClaudeHandler
```

Register a handler for the ``PreToolUse`` hook.

#### on\_user\_prompt\_submit

```python
def on_user_prompt_submit(handler: ClaudeHandler) -&gt; ClaudeHandler
```

Register a handler for the ``UserPromptSubmit`` hook.

#### run

```python
def run() -&gt; int
```

Dispatch the current process invocation and return the exit code.

## CodexApp Objects

```python
class CodexApp()
```

Minimal Codex app that dispatches the ``notify`` event to a handler.

#### \_\_init\_\_

```python
def __init__()
```

Create a Codex runtime app with no registered notify handler.

#### on\_notify

```python
def on_notify(handler: CodexHandler) -&gt; CodexHandler
```

Register a handler for the Codex ``notify`` event.

#### run

```python
def run() -&gt; int
```

Dispatch the current process invocation and return the exit code.
