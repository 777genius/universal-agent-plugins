---
title: "Ejemplos y recetas"
description: "Un mapa guiado de los repositorios de ejemplo públicos, los repositorios de inicio, las referencias de tiempo de ejecución local y los ejemplos de habilidades en plugin-kit-ai."
canonicalId: "page:guide:examples-and-recipes"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Ejemplos y recetas

Utilice esta página cuando desee ver cómo se ve `plugin-kit-ai` en repositorios reales en lugar de leer solo una guía abstracta.

## 1. Ejemplos de complementos de producción

Estos son los ejemplos más claros de formas públicas terminadas:

- [`codex-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/codex-basic-prod): repositorio de producción Go más `codex-runtime`
- [`claude-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/claude-basic-prod): repositorio de producción Go más `claude`
- [`codex-package-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/codex-package-prod): destino `codex-package`
- [`gemini-extension-package`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/gemini-extension-package): destino de empaquetado `gemini`
- [`cursor-basic`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/cursor-basic): destino `cursor` de configuración de espacio de trabajo
- [`opencode-basic`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/opencode-basic): destino `opencode` de configuración de espacio de trabajo

Lee estos cuando quieras:

- un diseño de repositorio concreto
- resultados reales generados
- un ejemplo público veraz de lo que es "saludable"

Importante: estos ejemplos muestran distintas formas de productos públicos. No implican que un sistema real deba dividirse en un repositorio separado para cada objetivo.

## 2. Repositorios iniciales

Utilice repositorios de inicio cuando desee comenzar desde una línea de base en buen estado en lugar de desde un directorio vacío.

Son mejores para:

- configuración por primera vez
- incorporación del equipo
- elegir entre los puntos de partida Go, Python, Node, Claude y Codex

Los enlaces code-first más directos son:

- [`plugin-kit-ai-starter-codex-go`](https://github.com/777genius/plugin-kit-ai-starter-codex-go)
- [`plugin-kit-ai-starter-codex-python`](https://github.com/777genius/plugin-kit-ai-starter-codex-python)
- [`plugin-kit-ai-starter-codex-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-codex-node-typescript)
- [`plugin-kit-ai-starter-claude-go`](https://github.com/777genius/plugin-kit-ai-starter-claude-go)
- [`plugin-kit-ai-starter-claude-python`](https://github.com/777genius/plugin-kit-ai-starter-claude-python)
- [`plugin-kit-ai-starter-claude-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript)

Si aún elige, vincúlelo con [Elija un repositorio inicial](/es/guide/choose-a-starter).

## 3. Referencias de tiempo de ejecución local

El área `examples/local` muestra referencias de tiempo de ejecución Python y Node para repositorios que permanecen localmente primero.

Estos son útiles cuando:

- desea comprender más profundamente la historia interpretada en tiempo de ejecución
- desea comparar configuraciones de tiempo de ejecución local de JavaScript, TypeScript y Python
- necesitas una referencia concreta más allá de los repositorios iniciales

Comience con:

- [`codex-node-typescript-local`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/local/codex-node-typescript-local)
- [`codex-python-local`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/local/codex-python-local)

## 4. Ejemplos de habilidades

El área `examples/skills` muestra ejemplos de habilidades de apoyo e integraciones de ayuda.

Estos no son el punto de entrada principal para la mayoría de los autores de complementos, pero son valiosos cuando:

- desea conectar documentos, revisar o formatear ayudas en el flujo de trabajo más amplio
- desea comprender cómo las habilidades adyacentes pueden encajar en los repositorios de complementos

## Lectura sugerida según el objetivo

- Si desea el ejemplo de runtime más sólido: comience con el ejemplo de producción de Codex o Claude y luego lea [Crear un complemento listo para el equipo](/es/guide/team-ready-plugin).
- Si desea un ejemplo code-first por lenguaje y destino: comience con el repositorio de inicio Go, Python o Node enlazado arriba y luego lea [Build Custom Plugin Logic](/en/guide/build-custom-plugin-logic).
- Si desea ejemplos de empaquetado o configuración del espacio de trabajo: comience con los ejemplos del paquete Codex, Gemini, Cursor o OpenCode y luego lea [Destinos del paquete y del espacio de trabajo](/es/guide/package-and-workspace-targets).
- Si desea un punto de partida claro, no un ejemplo terminado: vaya a [Plantillas de inicio](/es/guide/starter-templates).
- Quiere elegir el objetivo antes de mirar los repositorios: lea [Elija un objetivo](/es/guide/choose-a-target).
- Si primero quiere entender cómo puede expandirse un repositorio: lea [Lo que puedes construir](/es/guide/what-you-can-build).

## Regla final

Los ejemplos deberían aclarar el contrato público, no reemplazarlo.

Utilice repositorios de ejemplo para ver la forma y los resultados saludables. Para el modelo mental de un repositorio y múltiples objetivos, lea [Un proyecto, múltiples objetivos](/es/guide/one-project-multiple-targets).

</details>
