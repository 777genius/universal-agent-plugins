---
title: "SDK та інтеграції"
description: "Вибір SDK для hooks, виявлення агента, встановлення й керування."
canonicalId: "page:sdk:index"
section: "sdk"
locale: "uk"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK та інтеграції

SDK потрібні, щоб **вбудувати Universal Agent Plugins у власну програму** або обробляти нативні події агента. Звичайне встановлення дивіться в [Use plugins](../use/).

## Оберіть правильний інтерфейс

| Мета | Посібник | Інтерфейс |
| --- | --- | --- |
| Обробити подію агента | [Go Hooks](./go-hooks) | `sdk` |
| Визначити, хто запустив hook | [Host detection](./host-detection) | `sdk/hostdetect` |
| Встановлювати, оновлювати та відновлювати в Go | [Go Installer](./go-installer) | `install/integrationctl/agentplugins/installer` |
| Керувати плагінами у Node/Electron | [TypeScript Management](./typescript-management) | `universal-agent-plugins-management-sdk` |

## Версії та обмеження

- Опублікований Go Hooks модуль: `github.com/777genius/plugin-kit-ai/sdk@v1.3.0`. Стабільність визначається **окремо для кожної події**.
- `sdk/hostdetect` є в поточному вихідному коді; перевіряйте наявність у вибраному тегу модуля.
- Go Installer — окремий модуль поточного коду; фіксуйте ревізію й залежності, **не** приписуйте йому версію hooks.
- TypeScript SDK має версію джерел **0.1.1**, Node.js **22.18+**, Management API **v1**. Перед npm-інсталяцією перевіряйте доступність пакета.
- CLI **0.1.95+** підтримує `manage serve` і `manage discover`. Метод TS `checkUpdates` **відсутній у CLI 0.1.95 і старіших**.

`hostdetect.Detect` визначає хост **поточного виклику hook**; `discoverTargets` визначає **присутність нативних клієнтів і приватні профілі**; `Engine.Discover` описує лише зареєстровані адаптери. Жоден із них не здійснює встановлення автоматично. Перевірка схеми, запис файлів, активація агента та автентифікація MCP — різні етапи.

Далі: [Go Hooks](./go-hooks) · [Detection](./host-detection) · [Go Installer](./go-installer) · [TypeScript SDK](./typescript-management).
