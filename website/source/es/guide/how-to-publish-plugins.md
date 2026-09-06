---
title: "Cómo publicar complementos"
description: "Una guía práctica para publicar proyectos plugin-kit-ai en Codex, Claude y Gemini sin confundir la aplicación local con la planificación de publicaciones."
canonicalId: "page:guide:how-to-publish-plugins"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Cómo publicar complementos

Utilice esta guía cuando su repositorio ya esté creado en `plugin-kit-ai` y desee el siguiente paso más claro para la publicación Codex, Claude o Gemini.

Empiece con un repositorio en el que `plugin-kit-ai generate .` y `plugin-kit-ai validate . --strict` ya pasen, para que los comandos de publicación lean artefactos gestionados actuales y no archivos desactualizados.

## Qué cubre esta guía

- qué plataformas admiten aplicaciones locales reales hoy
- qué plataforma utiliza plan-and-readiness en su lugar
- qué comando ejecutar primero
- qué resultado esperar después de que finalice el comando

## Comparación rápida

| Plataforma | Modelo de publicación | Real aplica en `plugin-kit-ai` | Comando principal | Lo que obtienes |
|---|---|---:|---|---|
| Codex | raíz del mercado local | sí | `publish --channel codex-marketplace` | `.agents/plugins/marketplace.json` más `plugins/<name>/...` |
| Claude | raíz del mercado local | sí | `publish --channel claude-marketplace` | `.claude-plugin/marketplace.json` más `plugins/<name>/...` |
| Gemini | preparación del repositorio/lanzamiento | no | `publish --channel gemini-gallery --dry-run` | un plan de publicación acotado y diagnóstico de preparación |

## La regla corta

- utilice `publish` cuando desee un flujo de trabajo de publicación
- use `publication` cuando quiera primero una vista de inspección o de diagnóstico
- Codex y Claude admiten solicitudes locales reales hoy
- Gemini utiliza la publicación de planificación y preparación en v1, no la aplicación local

La forma del repositorio sigue siendo la misma:

- `plugin.yaml` es el manifiesto del complemento principal
- `targets/...` contiene entradas escritas específicas del objetivo
- `publish/...` tiene intención de publicación
- `publication` es la superficie de inspección y tratamiento
- `publish` es la superficie del flujo de trabajo de publicación.

## Publicar en Codex

Para Codex, la publicación significa materializar una raíz en el mercado local.

Ejecute esto primero:

```bash
plugin-kit-ai publish ./my-plugin --channel codex-marketplace --dest ./local-codex-marketplace --dry-run
```

Aplíquelo cuando el plan parezca correcto:

```bash
plugin-kit-ai publish ./my-plugin --channel codex-marketplace --dest ./local-codex-marketplace
```

Resultado esperado:

- `.agents/plugins/marketplace.json`
- `plugins/<name>/...`

Una raíz local como esa ya puede actuar como fuente de complemento Codex.

## Publicar en Claude

Para Claude, la publicación también significa materializar una raíz en el mercado local.

Ejecute esto primero:

```bash
plugin-kit-ai publish ./my-plugin --channel claude-marketplace --dest ./local-claude-marketplace --dry-run
```

Aplíquelo cuando el plan parezca correcto:

```bash
plugin-kit-ai publish ./my-plugin --channel claude-marketplace --dest ./local-claude-marketplace
```

Resultado esperado:

- `.claude-plugin/marketplace.json`
- `plugins/<name>/...`

## Publicar en Gemini

Para Gemini, la publicación **no** significa crear una raíz de mercado local.

En v1, `plugin-kit-ai` hace tres cosas limitadas:

- valida la intención de publicación
- comprueba la preparación del repositorio
- construye un plan de publicación

Comience con la preparación:

```bash
plugin-kit-ai publication doctor ./my-plugin --target gemini
```

Luego inspeccione el plan de publicación:

```bash
plugin-kit-ai publish ./my-plugin --channel gemini-gallery --dry-run
```

Requisitos previos esperados:

- un repositorio público GitHub
- un control remoto `origin` válido que apunte a GitHub
- el tema GitHub `gemini-cli-extension`
- `gemini-extension.json` en la raíz correcta

Gemini utiliza la publicación de planificación y preparación en v1, no la aplicación local.

## Planifique en todos los canales creados

Utilice esto cuando un repositorio cree más de un canal de publicación:

```bash
plugin-kit-ai publish ./my-plugin --all --dry-run --dest ./local-marketplaces --format json
```

Reglas importantes:

- utiliza únicamente canales creados por `publish/...`
- no infiere canales de `targets`
- es solo de planificación en v1
- `--dest` se requiere solo cuando los canales creados incluyen Codex o Claude flujos de mercado local.
- La orquestación exclusiva Gemini no requiere `--dest`

Si los autores del repositorio solo son `gemini-gallery`, esto también funciona:

```bash
plugin-kit-ai publish ./my-plugin --all --dry-run --format json
```

## ¿Qué comando debo ejecutar?

- Quiero una raíz de mercado local Codex: `plugin-kit-ai publish --channel codex-marketplace --dest <marketplace-root>`
- Quiero una raíz de mercado local Claude: `plugin-kit-ai publish --channel claude-marketplace --dest <marketplace-root>`
- Quiero que Gemini esté listo para la publicación: `plugin-kit-ai publication doctor --target gemini`
- Quiero un plan de publicación Gemini: `plugin-kit-ai publish --channel gemini-gallery --dry-run`
- Quiero un plan de publicación combinado: `plugin-kit-ai publish --all --dry-run` y agregar `--dest <marketplace-root>` cuando se incluyan canales de autor Codex o Claude

## Lectura adicional

- [CLI sección de publicación README](https://github.com/777genius/plugin-kit-ai/tree/main/cli/plugin-kit-ai)
- [`plugin-kit-ai publish`](/es/api/cli/plugin-kit-ai-publish)
- [`plugin-kit-ai publication`](/es/api/cli/plugin-kit-ai-publication)
- [`plugin-kit-ai publication doctor`](/es/api/cli/plugin-kit-ai-publication-doctor)

</details>
