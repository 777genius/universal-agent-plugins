---
title: "Cree su primer complemento"
description: "Un tutorial mínimo de principio a fin desde el inicio hasta la validación estricta."
canonicalId: "page:guide:first-plugin"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Cree su primer complemento

Este tutorial le brinda el primer repositorio de trabajo más simple en la ruta predeterminada más sólida.

Mantiene el alcance intencionalmente limitado:

- primer objetivo: `codex-runtime`
- primer idioma: `go`
- primera puerta de preparación: `validate --strict`

Esa forma estrecha es sólo para la primera ejecución. Si lo que más le interesa es la historia más amplia de un repositorio y muchos resultados, lea [Un proyecto, múltiples objetivos](/es/guide/one-project-multiple-targets) justo después de este tutorial.

## 1. Instale el CLI

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

## 2. Estructurar un proyecto

```bash
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
```

La ruta predeterminada `init` ya es el punto de inicio de producción recomendado.

## 3. Generar los archivos de destino

```bash
plugin-kit-ai generate .
```

Trate los archivos de destino generados como salidas. Siga editando el repositorio a través de `plugin-kit-ai` en lugar de mantener manualmente los archivos generados.

## 4. Ejecute la puerta de preparación

```bash
plugin-kit-ai validate . --platform codex-runtime --strict
```

Utilice esto como puerta principal de grado CI para un proyecto de complemento local.

## Lo que tienes ahora

- un repositorio de complementos
- archivos creados bajo `plugin/` para repos nuevos
- generó Codex salida de tiempo de ejecución
- una puerta de preparación clara a través de `validate --strict`

## 5. Cuándo cambiar de ruta

Cambie a otra ruta sólo cuando realmente la necesite:

- elija `claude` para los complementos Claude
- elija `--runtime node --typescript` para la ruta principal admitida que no sea Go
- elija `--runtime python` cuando el proyecto permanezca local en el repositorio y su equipo sea Python-primero
- elija `codex-package`, `gemini`, `opencode` o `cursor` solo cuando realmente necesite una forma diferente de enviar el complemento

Eso no significa que el repositorio deba permanecer con un único objetivo para siempre: comience con el objetivo más importante hoy y agregue los demás sólo cuando el producto se expanda genuinamente.

## Próximos pasos

- Lea [Elegir tiempo de ejecución](/es/concepts/choosing-runtime) antes de abandonar la ruta predeterminada.
- Lea [Un proyecto, múltiples objetivos](/es/guide/one-project-multiple-targets) si la idea de un repositorio y muchos resultados es la razón principal por la que le interesa el producto.
- Utilice [Plantillas de inicio](/es/guide/starter-templates) cuando desee un repositorio de ejemplo en buen estado.
- Busque [CLI Referencia](/es/api/cli/) cuando necesite un comportamiento de comando exacto.

</details>
