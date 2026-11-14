---
title: "plugin_kit_ai_runtime"
description: "Generated Python runtime reference"
canonicalId: "python-runtime:plugin_kit_ai_runtime"
surface: "runtime-python"
section: "api"
locale: "ru"
generated: true
editLink: false
stability: "public-stable"
maturity: "stable"
sourceRef: "python/plugin-kit-ai-runtime/src/plugin_kit_ai_runtime/__init__.py"
translationRequired: false
---
<DocMetaCard surface="runtime-python" stability="public-stable" maturity="stable" source-ref="python/plugin-kit-ai-runtime/src/plugin_kit_ai_runtime/__init__.py" source-href="https://github.com/777genius/universal-agent-plugins/blob/main/python/plugin-kit-ai-runtime/src/plugin_kit_ai_runtime/__init__.py" />

# plugin_kit_ai_runtime

Сгенерировано через pydoc-markdown.

Официальные runtime-хелперы для Python-плагинов на plugin-kit-ai.

Эта страница собирает публичные типы, константы, функции и классы пакета `plugin_kit_ai_runtime`.

# Оглавление

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

Официальные runtime-хелперы для исполняемых Python-плагинов на plugin-kit-ai.

#### JSONMap

JSON-представление payload, которое используют Python runtime-хелперы.

#### ClaudeHandler

Сигнатура обработчика для Claude hooks, который возвращает JSON-объект или ``None``.

#### CodexHandler

Сигнатура обработчика для Codex events, который возвращает код выхода или ``None``.

#### CLAUDE\_STABLE\_HOOKS

Имена стабильных Claude hooks, поддерживаемых публичной Python runtime-линией.

#### CLAUDE\_EXTENDED\_HOOKS

Имена расширенных Claude hooks, доступных в beta Python runtime-линии.

#### allow

```python
def allow() -&gt; JSONMap
```

Возвращает пустой JSON-объект, который Claude ожидает для разрешающего ответа.

#### continue\_

```python
def continue_() -&gt; int
```

Возвращает код выхода ``0`` для Codex-обработчиков, которым нужно обычное продолжение.

## Объекты ClaudeApp

```python
class ClaudeApp()
```

Минимальное Claude-приложение, которое маршрутизирует поддерживаемые имена hooks к обработчикам.

#### \_\_init\_\_

```python
def __init__(allowed_hooks: Iterable[str], usage: str)
```

Создаёт Claude runtime-приложение.

**Аргументы**:

- `allowed_hooks` - Имена hooks, которые этот бинарник принимает через argv.
- `usage` - Строка помощи, которая печатается при некорректном вызове.

#### on

```python
def on(hook_name: str) -&gt; Callable[[ClaudeHandler], ClaudeHandler]
```

Возвращает декоратор, который регистрирует обработчик для ``hook_name``.

#### on\_stop

```python
def on_stop(handler: ClaudeHandler) -&gt; ClaudeHandler
```

Регистрирует обработчик для hook ``Stop``.

#### on\_pre\_tool\_use

```python
def on_pre_tool_use(handler: ClaudeHandler) -&gt; ClaudeHandler
```

Регистрирует обработчик для hook ``PreToolUse``.

#### on\_user\_prompt\_submit

```python
def on_user_prompt_submit(handler: ClaudeHandler) -&gt; ClaudeHandler
```

Регистрирует обработчик для hook ``UserPromptSubmit``.

#### run

```python
def run() -&gt; int
```

Обрабатывает текущий запуск процесса и возвращает код выхода.

## Объекты CodexApp

```python
class CodexApp()
```

Минимальное Codex-приложение, которое маршрутизирует событие ``notify`` к обработчику.

#### \_\_init\_\_

```python
def __init__()
```

Создаёт Codex runtime-приложение без зарегистрированного обработчика notify.

#### on\_notify

```python
def on_notify(handler: CodexHandler) -&gt; CodexHandler
```

Регистрирует обработчик для события Codex ``notify``.

#### run

```python
def run() -&gt; int
```

Обрабатывает текущий запуск процесса и возвращает код выхода.
