---
title: "SDK e integraciones"
description: "Guía para elegir SDK de hooks, detección, instalación o gestión."
canonicalId: "page:sdk:index"
section: "sdk"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK e integraciones

Utiliza estos SDK para **integrar Universal Agent Plugins en otra aplicación** o atender eventos nativos de un agente. Para una instalación normal, consulta [Usar plugins](../use/).

## Elige un SDK

| Objetivo | Guía | API |
| --- | --- | --- |
| Procesar hooks nativos | [Go Hooks](./go-hooks) | `sdk` |
| Identificar el host de un hook | [Detección](./host-detection) | `sdk/hostdetect` |
| Integrar instalación y recuperación en Go | [Go Installer](./go-installer) | `install/integrationctl/agentplugins/installer` |
| Gestionar plugins desde Node/Electron | [TypeScript Management](./typescript-management) | `universal-agent-plugins-management-sdk` |

## Versiones y límites

- Go Hooks: módulo publicado `github.com/777genius/plugin-kit-ai/sdk@v1.3.0`, con estabilidad **por evento**.
- El detector `sdk/hostdetect` existe en el código fuente actual; verifica que el tag exacto del módulo lo incluya.
- El instalador Go es otro módulo; fija una revisión comprobada y sus dependencias. No reutilices la versión de hooks.
- Management SDK de TypeScript: versión de fuente **0.1.1**, Node **22.18+**, API **v1**; comprueba que esté publicado en npm.
- El CLI **0.1.95+** ofrece `manage serve` y `manage discover`. `checkUpdates` del SDK **no está disponible en el CLI 0.1.95 ni anteriores**.

`hostdetect.Detect` reconoce el **host que invoca un hook**; `discoverTargets` detecta **clientes y perfiles privados**; `Engine.Discover` presenta adaptadores Go registrados. Ninguno instala automáticamente. Validación, escritura de archivos, activación nativa y autenticación del servicio son pruebas diferentes.

Continúa con [Go Hooks](./go-hooks), [detección](./host-detection), [instalador](./go-installer) o [gestión TypeScript](./typescript-management).
