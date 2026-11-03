---
title: "SDK и интеграции"
description: "Выбор Go SDK для hooks и установки, детектора агентов и TypeScript SDK управления."
canonicalId: "page:sdk:index"
section: "sdk"
locale: "ru"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK и интеграции

SDK нужны, когда вы **встраиваете Universal Agent Plugins в своё приложение** или реализуете типизированные обработчики событий агента. Для обычной установки плагинов используйте [CLI](../use/), а не runtime SDK.

## Какой SDK выбрать

| Задача | Руководство | API |
| --- | --- | --- |
| Обрабатывать нативные hooks агента | [Go Hooks SDK](./go-hooks) | `github.com/777genius/plugin-kit-ai/sdk` |
| Узнать, Claude Code или Codex запустил hook | [Определение агента](./host-detection) | `sdk/hostdetect` |
| Управлять установкой и восстановлением внутри Go-хоста | [Go Installer SDK](./go-installer) | `install/integrationctl/agentplugins/installer` |
| Управлять плагинами из Node/Electron/desktop приложения | [TypeScript Management SDK](./typescript-management) | `universal-agent-plugins-management-sdk` |

## Не путайте разные виды определения агента

`hostdetect.Detect` распознаёт **запустивший hook процесс** по его JSON. `discoverTargets` в TypeScript SDK выясняет **наличие нативных клиентов и их приватных профилей** через Go-движок. `Engine.Discover` в Go installer facade показывает возможности явно зарегистрированных адаптеров. Ни одна из этих операций не устанавливает плагин автоматически.

## Версии и доступность

- **Go Hooks SDK:** опубликованный модуль `github.com/777genius/plugin-kit-ai/sdk@v1.3.0`. У разных событий свой уровень стабильности; смотрите [политику SDK](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/STABILITY.md).
- **Host detector:** доступен в текущих исходниках `sdk/hostdetect`. Проверяйте его наличие в **конкретном теге Go-модуля** перед использованием в опубликованной интеграции.
- **Go Installer facade:** API в текущем модуле `install/integrationctl/agentplugins`; фиксируйте ревизию кода и зависимости. Не приписывайте ему версию Go Hooks SDK.
- **TypeScript Management SDK:** версия исходного пакета `0.1.1`, Node.js **22.18+**, Management API **v1**; наличие версии в npm проверяйте через `npm view`.
- **Go-движок управления:** публичный CLI **0.1.95+** поддерживает `manage serve` и `manage discover`. Но метод `checkUpdates` из TS SDK **не поддерживается CLI 0.1.95 и более ранними**.

SDK для hooks, Go установщик и TS-клиент имеют **разные контракты и независимые версии**. Проверка схемы, запись файлов, обнаружение агента, авторизация и настоящий вызов инструмента — отдельные стадии проверки.

## Что изучить дальше

[Go Hooks SDK](./go-hooks) — события, обработчики, transport и статусы стабильности. [Host detection](./host-detection) — приоритет сигналов, ошибки, неизвестный агент. [Go Installer](./go-installer) — план, подтверждение, применение, восстановление. [TypeScript Management](./typescript-management) — запуск собственного движка, обнаружение профилей, управление установленными пакетами.

[Исходники Go runtime](https://github.com/777genius/universal-agent-plugins/tree/main/sdk) · [Go installer](https://github.com/777genius/universal-agent-plugins/tree/main/install/integrationctl/agentplugins/installer) · [TypeScript SDK](https://github.com/777genius/universal-agent-plugins/tree/main/sdks/typescript)
