---
title: "Створення плагінів"
description: "Створюйте пакети Skill, MCP або гібридні пакети за допомогою інструментів Agent Plugins."
canonicalId: "page:build:index"
section: "build"
locale: "uk"
lang: "uk-UA"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Створення плагінів {#build-plugins}

Пакет Agent Plugins 1.0 — це каталог із кореневим `plugin.json` та Skills,
конфігурацією MCP або обома компонентами. Почніть із найменшого компонента, що
розв'язує задачу користувачів. Skill надає інструкції; MCP — з'єднання з інструментами.

## Доступність {#availability}

Поточний випуск нативного інсталятора,
[CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94),
може перевіряти й установлювати стандартні пакети. Канали менеджерів пакетів
можуть відставати; перевірте встановлену версію через `agentplugins version`.
Він **не містить** `agentplugins author`. Команди розробки в цьому посібнику
описують **поточні збірки з вихідного коду** з увімкненими інструментами розробки.
Історичні докази випуску інструментів розробки 0.1.65 стосуються саме тієї версії
та не описують сьогоднішній інсталятор.

Ви можете написати файли вручну та скористатися випущеним інсталятором або
зібрати наведені нижче інструменти для створення заготовок, статичних перевірок
і явного циклу виконання MCP. Успішна статична перевірка не доводить активацію
в нативному клієнті чи робоче з'єднання із сервісом.

## Створіть невеликий пакет вручну {#make-a-small-package-by-hand}

Створіть таку структуру:

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

Укажіть ідентифікаційні дані пакета в `plugin.json`:

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "review-helper",
  "version": "0.1.0",
  "description": "Перевірка змін документації на зрозумілість і підтвердженість"
}
```

Почніть `skills/review-docs/SKILL.md` таким вмістом:

```markdown
---
name: review-docs
description: Застосовуйте під час перевірки зрозумілості та підтвердженості документації.
---

# Перевірка документації

Визначте цільового читача й задачу. Перевірте вхідні дані та дію кожної команди.
Відокремте підтверджену поведінку від передумов і неперевірених тверджень.
Поверніть зауваження з розташуванням у файлах та запропонованими виправленнями.
```

Установіть випущений менеджер, потім перевірте пакет і перегляньте план:

```bash
npm install -g universal-agent-plugins
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

Перегляньте план і перейдіть до [використання плагінів](../use/install), коли
будете готові його застосувати. Відповідальність кожного файлу пояснено у
[структурі пакета](./layout).

## Зберіть із вихідного коду {#build-from-source}

Наведені нижче команди збирають поточний CLI з вихідного коду з увімкненими
інструментами розробки пакетів. Потрібні Git та інструментарій Go, зазначений
у репозиторії (зараз Go 1.26.10). Поширюючи результати, записуйте вибраний коміт.

```bash
git clone https://github.com/777genius/universal-agent-plugins.git
cd universal-agent-plugins/cli
mkdir -p ../bin
authoring_module=$(go list -f '{{.Module.Path}}' ./cmd/agentplugins)
authoring_revision=$(git rev-parse HEAD)
go build -ldflags "-X ${authoring_module}/internal/authoring/commands.Enabled=release-cli-contract-v1 -X ${authoring_module}/internal/authoring/commands.Revision=${authoring_revision}" \
  -o ../bin/agentplugins ./cmd/agentplugins
export PATH="$(cd ../bin && pwd):$PATH"
agentplugins author --help
```

Для навчальних пакетів використовуйте окремий робочий каталог. Цей виконуваний
файл є збіркою для розробки. Його наявність не означає кваліфікований публічний
випуск інструментів розробки. Якщо встановлений менеджер усе ще не розпізнає
`author`, перевірте шлях до виконуваного файлу.

## Оберіть заготовку пакета {#choose-a-package-to-scaffold}

| Потреба | Посібник | Обов'язкові параметри шаблону |
| --- | --- | --- |
| Інструкції, яких може дотримуватися агент | [Окремий Skill](./skill) | `--template skill` |
| З'єднання з наявним сервісом | [Віддалений MCP](./mcp-remote) | `--template mcp-remote --url <endpoint>` |
| Локальний MCP-процес Node | [Stdio MCP](./mcp-stdio) | `--template mcp-stdio --runtime node` |
| Інструкції разом з інструментами | [Гібридний пакет](./hybrid) | `--template hybrid --mcp-template <choice>` та параметри обраного шаблону MCP |

Пакет з окремим Skill також має кореневий `plugin.json`. Установлення окремих
Skills з іншого репозиторію — це окрема [операція інсталятора](../use/install#install-agent-skills).

## Пройдіть цикл розробки {#follow-the-authoring-loop}

1. Створіть пакет у ще неіснуючому місці призначення та відредагуйте згенеровані файли.
2. Виконайте [validate, inspect і статичний test](./checks) для точного кореня пакета.
3. Перевірте потрібні клієнти через `compat` та дослідіть докази `doctor`.
4. Для MCP окремо перевірте виконання явним запуском зі збірки з вихідного коду.
5. Передайте пакет і докази через [передачу інсталятору](./handoff).

Поточний вихідний код також надає `bootstrap` на основі файлу фіксації,
безперервний `dev`, JSON `normalize` та явний Claude MCP `import native`.
Див. [перевірки](./checks) та [stdio MCP](./mcp-stdio). Створення проєкцій для
клієнтів, експорт наборів і публікація пакетів не входять до поточного набору
команд розробки. Специфічні для клієнта хуки залишаються окремими від переносних
компонентів Skill і MCP.
