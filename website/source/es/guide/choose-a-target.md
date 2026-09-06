---
title: "Elige un objetivo"
description: "Una guía pública práctica para elegir el destino que coincida con cómo desea enviar el complemento."
canonicalId: "page:guide:choose-a-target"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Elige un objetivo

Utilice esta página cuando ya sepa que desea `plugin-kit-ai`, pero aún necesita hacer coincidir el repositorio con la forma en que desea enviar el complemento.

Elegir un objetivo significa elegir la ruta principal que el producto necesita hoy, no bloquear el repositorio para siempre.

<MermaidDiagram
  :chart="`
flowchart TD
  Need[What does the product need right now] --> Ejecutivo{Comportamiento ejecutable}
  Necesidad --> Artefacto{Paquete o extensión}
  Necesidad --> Configuración{Integración gestionada por repositorio}
  Ejecutivo --> Codex[codex-runtime]
  Ejecutivo --> Claude[claude]
  Artefacto --> CodexPaquete[paquete-codex]
  Artefacto --> Gemini[géminis]
  Configuración --> OpenCode[código abierto]
  Configuración --> Cursor[cursor]
`"
/>

## Regla corta

- elija `codex-runtime` cuando desee la ruta de ejecución predeterminada más potente
- elija `claude` cuando los ganchos Claude sean el requisito real del producto
- elija `codex-package` cuando el producto sea un paquete oficial Codex
- elija `gemini` cuando el producto sea un paquete de extensión Gemini
- elija `opencode` o `cursor` cuando el repositorio debe poseer la configuración de integración/configuración

## Directorio de destino

| Objetivo | Elígelo cuando | Carril |
| --- | --- | --- |
| `codex-runtime` | Quiere la ruta predeterminada del complemento ejecutable | Ruta de ejecución recomendada |
| `claude` | Necesitas ganchos Claude específicamente | Ruta Claude recomendada |
| `codex-package` | Necesita Codex salida de embalaje | Ruta de paquete recomendada |
| `gemini` | Está enviando un paquete de extensión Gemini | Ruta de extensión recomendada |
| `opencode` | Quiere la configuración de integración OpenCode propiedad del repositorio | Configuración de integración de propiedad del repositorio |
| `cursor` | Quiere la configuración de integración Cursor propiedad del repositorio | Configuración de integración de propiedad del repositorio |

## Valor predeterminado seguro

Si no está seguro, comience con `codex-runtime` y la ruta predeterminada Go.

Esto le brinda el punto de partida de producción más limpio antes de elegir un camino más estrecho o más especializado.

Cuando luego pasa a `codex-package`, la ruta del paquete oficial sigue el diseño del paquete oficial `.codex-plugin/plugin.json`.

Si intencionalmente comienza con Node/TypeScript o Python compatibles, eso cambia la elección del idioma, no la necesidad de decidir cada detalle de empaquetado o integración desde el primer día.

## Qué hacer cuando necesita más de un objetivo

- elegir el camino principal que define el producto hoy
- mantener el repositorio unificado
- agregue más objetivos solo cuando aparezca un requisito real de entrega o integración

Lea [Un proyecto, múltiples objetivos](/es/guide/one-project-multiple-targets) cuando desee el modelo mental de múltiples objetivos más amplio.
</details>
