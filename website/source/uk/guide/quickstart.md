---
title: "Швидкий старт"
description: "Установлюйте доступні плагіни або підготуйте переносний пакет Agent Plugins 1.0."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "uk"
lang: "uk-UA"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

<a id="quickstart"></a>

# Використання плагінів / Створення плагінів {#use-plugins-build-plugins}

Установіть пакет для своїх агентів або створіть переносний пакет Agent Plugins
1.0 з інструкціями та конфігурацією інструментів.

## Використання плагінів {#use-plugins}

<a id="optional-first-proof"></a>

З Node.js 22 або новішою версією:

```bash
npx universal-agent-plugins add context7
```

CLI запитає, які сумісні агенти використовувати, і повідомить подальші кроки
активації. Щоб спершу отримати відомості та переглянути план:

```bash
npx universal-agent-plugins info context7
npx universal-agent-plugins add context7 --target codex,cursor --dry-run
```

[Посібник із нативного встановлення](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)
також надає виконувані файли для macOS, Linux і Windows. Нативному менеджеру
Node.js не потрібен, хоча окремі плагіни можуть потребувати власних середовищ
виконання. Під час використання npm перевіряйте `npx universal-agent-plugins version`:
канал менеджера пакетів може ще надавати старішу CLI. Автоматичний вибір нижче
потребує 0.1.94; перевірений випуск можна отримати через нативний інсталятор.
Явні команди для проєкту в цьому посібнику також працюють із 0.1.93.

[CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95)
зберігає явну область проєкту та автоматичний вибір для `install`, запроваджений
у 0.1.94. Також доступні [локальний API керування](../use/manage#local-management-api)
та [нативні розширення Pi](../reference/client-compatibility#pi-native-extensions).
Щоб переглянути план додавання пакета до проєкту:

Для проєкту скопіюйте з перевіреної картки каталогу точне Git-джерело з повним SHA коміту та сумісний агент. Псевдоніми Directory v1 (наприклад, `context7`) діють лише в області користувача. Замініть `OWNER/REPOSITORY`, `FULL_COMMIT_SHA` і `PACKAGE_PATH`; для пакета в корені репозиторію приберіть `//PACKAGE_PATH`.

```bash
npx universal-agent-plugins add 'github:OWNER/REPOSITORY@FULL_COMMIT_SHA//PACKAGE_PATH' --target codex --scope project \
  --project-root /path/to/project --dry-run
```

Оберіть наявний каталог проєкту. Команди додавання й обслуговування за
замовчуванням працюють в області користувача, тому явно обирайте область
проєкту. У 0.1.94 `install` без аргументу джерела обирає проєкт лише якщо саме
поточна папка містить звичайний файл `uap.toml`; явна область має пріоритет,
батьківські папки не перевіряються. Перегляньте план, перш ніж прибрати
`--dry-run`. Нативне виявлення, довіра, автентифікація та успішні виклики MCP
залишаються окремими від запису файлів пакета.

[Повний шлях встановлення](../use/install) ·
[Сумісність клієнтів і межі тестування](../reference/client-compatibility)

### Відновлення збереженого набору залежностей {#restore-a-saved-dependency-set}

Перевірте, що записано в обраному середовищі користувача:

```bash
npx universal-agent-plugins deps status
```

Для збереженої пари `uap.toml`/`uap.lock` виконайте експорт із папки обміну:

```bash
npx universal-agent-plugins deps export
```

Скопіюйте папку на інший комп'ютер, установіть цільові клієнти та виконайте там:

```bash
npx universal-agent-plugins deps import
npx universal-agent-plugins install --locked --scope user
```

Переносяться лише перевірений опис залежностей і файл фіксації. Профілі,
налаштування, облікові дані, квитанції та дані плагінів залишаються окремо.
Для встановлення й далі потрібні точні джерела або перевірені кешовані пакети.
Для проєкту додайте пару файлів до контролю версій і відновлюйте через явний
`install --locked --scope project`. Подробиці відновлення після помилок і
перенесення див. в [обслуговуванні та відновленні](../use/manage).

## Створення плагінів {#build-plugins}

<a id="recommended-default"></a>
<a id="if-you-only-read-one-thing"></a>

Стандартний пакет починається з кореневого `plugin.json` і може містити
`skills/<name>/SKILL.md`, `mcp.json` або обидва. Можна написати ці файли вручну
та перевірити пакет випущеним інсталятором:

```bash
npx universal-agent-plugins validate ./my-plugin
npx universal-agent-plugins add ./my-plugin --target codex --dry-run
```

Ці приклади передбачають, що `./my-plugin` уже містить повний пакет.
[Посібник зі створення](../build/) пояснює невеликий пакет Skill, написаний
вручну, та інструменти заготовок зі збірки з вихідного коду.

**Доступність інструментів розробки:** поточний випуск npm і нативного
інсталятора не містить `agentplugins author`. Поточний вихідний код реалізує
створення заготовок, статичні перевірки, явні тести виконання MCP, цикли
розробки, початкове встановлення залежностей і обслуговування JSON.
Для цих команд скористайтеся [налаштуванням збірки з вихідного коду](../build/#build-from-source).
Докази старіших випусків інструментів розробки прив'язані до власних версій;
вони не є обіцянкою щодо поточного завантаження інсталятора.

Статична перевірка не доводить виконання, OAuth або активацію. Підтримка
транспортів MCP також залежить від клієнта; наприклад, Codex не підтримує
оголошений MCP SSE. Чітко зазначайте вимоги пакета та невиконані перевірки в README.

[Специфікація Agent Plugins 1.0](https://agent-plugins.org/specification)
