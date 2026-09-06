---
title: "Inicio rápido"
description: "La ruta recomendada más rápida para un proyecto plugin-kit-ai en funcionamiento."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Inicio rápido

Esta es la ruta más corta recomendada cuando desea un repositorio de complementos que luego pueda convertirse en más formas de enviar el complemento.

Comience primero con un camino sólido. Agregue paquetes, extensiones o configuraciones de integración de propiedad del repositorio más adelante, cuando el producto realmente los necesite.

## Si solo lees una cosa

Comience con la ruta predeterminada Go a menos que ya sepa que los ganchos Claude, Node/TypeScript o Python definen los requisitos del producto.

Su primera opción es el punto de partida, no el límite permanente del repositorio.

## Valor predeterminado recomendado

Si no tiene una razón de peso para elegir otro camino, comience aquí:

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

Eso le brinda la ruta predeterminada más sólida en la actualidad: un repositorio de tiempo de ejecución Codex basado en Go que sigue siendo fácil de validar, transferir y expandir más adelante.

## Por qué este es el valor predeterminado

- un repositorio desde el primer día
- el tiempo de ejecución y la historia de lanzamiento más limpios de la actualidad
- la base más sencilla para rutas posteriores de paquetes, extensiones e integración

## Lo que obtienes

- un repositorio de complementos desde el primer día
- archivos creados bajo `plugin/`
- generó Codex salida de tiempo de ejecución desde el mismo repositorio
- una verificación de preparación limpia a través de `validate --strict`

## Rutas Node y Python admitidas

Si su equipo ya vive en Node/TypeScript o Python, esas rutas son compatibles y visibles desde el principio:

- `codex-runtime --runtime node --typescript`
- `codex-runtime --runtime python`
- ambas son rutas de ejecución interpretadas localmente, por lo que la máquina de destino aún necesita Node.js `20+` o Python `3.10+`
- Go sigue siendo el valor predeterminado cuando deseas la historia de producción general más sólida.

## Si está comenzando intencionalmente en Node o Python

Utilice este flujo alternativo solo cuando la elección del idioma ya sea parte del requisito del producto:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

O comience con Python:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## Qué hacer a continuación

- edite el complemento en `plugin/`
- ejecute `plugin-kit-ai generate ./my-plugin` nuevamente después de los cambios
- ejecute `plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict` nuevamente
- solo entonces agregue otra forma de enviarlo si el producto lo necesita

## Ampliar más tarde

| Si quieres | Añade esto más tarde |
| --- | --- |
| Claude ganchos como producto real | `claude` |
| Paquete oficial Codex | `codex-package` |
| Gemini paquete de extensión | `gemini` |
| Configuración de integración de propiedad del repositorio | `opencode` o `cursor` |

Elija `claude` primero solo cuando los ganchos Claude ya sean el requisito real del producto.

## Lo que se expande más tarde

- el repositorio permanece unificado a medida que agregas más carriles
- los paquetes y las líneas de extensión provienen de la misma fuente de autor
- OpenCode y Cursor encajan cuando el repositorio debe poseer la configuración de integración
- el límite exacto de soporte permanece en los documentos de referencia, no en su primer flujo de inicio

## Después del inicio rápido

- Continúe con [Cree su primer complemento](/es/guide/first-plugin) si desea el tutorial recomendado más limitado.
- Continúe con [Lo que puede construir](/es/guide/what-you-can-build) si desea el mapa completo del producto.
- Continúe con [Elija un objetivo](/es/guide/choose-a-target) cuando esté listo para hacer coincidir el repositorio con la forma en que desea enviarlo.
- Continúe con [Un proyecto, múltiples objetivos](/es/guide/one-project-multiple-targets) cuando esté listo para expandirse más allá de la primera ruta.

</details>
