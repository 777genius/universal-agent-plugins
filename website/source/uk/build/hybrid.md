---
title: "Створення гібридного пакета"
description: "Поєднайте один Skill і конфігурацію MCP під єдиною ідентичністю пакета."
canonicalId: "page:build:hybrid"
section: "build"
locale: "uk"
lang: "uk-UA"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Створення гібридного пакета {#build-a-hybrid-package}

**Розробка у збірці з вихідного коду:** команди на цій сторінці потребують
[збірки з увімкненими інструментами розробки](./#build-from-source).
Поточний випуск інсталятора не містить `agentplugins author`.

Гібрид корисний, коли інструкції пояснюють, як і коли використовувати інструменти.
Обидва компоненти мають один кореневий маніфест. Клієнт може підтримувати їх
по-різному, тому зберігайте результати сумісності кожного компонента під час
передачі.

## Явно оберіть частину MCP {#choose-the-mcp-side-explicitly}

Прапорець гібридного пакета — `--mcp-template`. Оберіть `mcp-remote` або
`mcp-stdio` та задайте обов'язкові параметри цього шаблону. Гібрид не визначає
транспорт із URL, локальних файлів чи встановлених середовищ виконання.

Приклад віддаленого гібрида:

```bash
agentplugins author init ./research-helper \
  --template hybrid \
  --name research-helper \
  --description 'Супровід дослідження з MCP-сервісом команди' \
  --skill-name research-guide \
  --mcp-template mcp-remote \
  --url https://mcp.example.com/mcp
```

URL — заповнювач прикладу, а не гарантовано доступний сервіс. Замініть його
реальною кінцевою точкою. Прочитайте [вимоги віддаленого MCP](./mcp-remote).

## Або оберіть локальний процес {#or-choose-a-local-process}

Для іншого, ще неіснуючого місця призначення варіант stdio має такий вигляд:

```bash
agentplugins author init ./local-research \
  --template hybrid \
  --name local-research \
  --description 'Супровід дослідження з локальними інструментами Node' \
  --skill-name research-guide \
  --mcp-template mcp-stdio \
  --runtime node
```

Не додавайте `--url` до варіанта stdio. Перегляньте
[передумови stdio та обмеження виконання](./mcp-stdio).

## Пов'яжіть інструкції з інструментами {#connect-instructions-to-tools}

Віддалений гібрид має таку основну структуру:

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

Також створюються README та `.gitignore`. Гібрид stdio додає вихідний код Node
і файли пакета з посібника stdio. Перегляньте `SKILL.md`: він має пояснювати,
коли застосовувати інструменти, які вхідні дані потрібні та як повідомляти про
відсутність доступу. Не доручайте агенту вважати текст Skill дозволом на дії.

## Перевірте обидва компоненти {#check-both-components}

```bash
agentplugins author skills validate ./research-helper
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper --target codex,claude
agentplugins author test ./research-helper
agentplugins author compat ./research-helper --target codex,claude
```

Для локального варіанта оберіть `./local-research`. Зберігайте результати для
кожного компонента: підтримка Skill клієнтом не означає автоматичної підтримки
транспорту MCP чи активації сервісу. Комбінований пакет не усуває це обмеження.

Продовжуйте з [додаткових Skills](./skills) або [передачі](./handoff).
