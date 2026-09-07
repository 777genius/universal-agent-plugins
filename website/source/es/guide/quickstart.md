---
title: "Inicio rápido"
description: "Instala plugins disponibles o prepara un paquete portable Agent Plugins 1.0."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

<a id="inicio-rapido"></a>

# Usar plugins / Crear plugins

Instala plugins disponibles o prepara un paquete portable Agent Plugins 1.0.

## Usar plugins {#use-plugins}

Disponible ahora: instala plugins con Universal Agent Plugins. Con Node.js 22+, ejecuta:

```bash
npx universal-agent-plugins add context7
```

Para la instalación nativa en macOS, Linux o Windows, consulta la guía. El CLI pregunta qué agentes compatibles usar; cada plugin puede necesitar su propio entorno de ejecución.

[Guía de instalación](https://github.com/777genius/universal-agent-plugins#quick-start)

Versiones verificadas el 2026-09-07: universal-agent-plugins 0.1.51 (npm), plugin-kit-ai 1.2.4 (npm/PyPI), versión estable de GitHub agentplugins-v0.1.51.

La compatibilidad depende del paquete. Validar el esquema no demuestra ejecución, OAuth ni activación. Codex no admite MCP SSE declarado; stdio y Streamable HTTP conservan el soporte de sus adaptadores actuales.

[Compatibilidad de clientes (en inglés)](https://github.com/777genius/universal-agent-plugins#supported-clients)

## Crear plugins {#build-plugins}

Crea un paquete portable Agent Plugins 1.0 basado en plugin.json, con skills/ y mcp.json opcionales. El soporte de cada cliente depende del paquete y sus componentes.

**En preparación — sin publicar**

El CLI de autoría basado en el estándar está en preparación y aún no se ha publicado. plugin-kit-ai 1.2.4, publicado en npm y PyPI, es la herramienta histórica v1, no standard-first v2. Instalar plugin-kit-ai@latest no proporciona el futuro flujo de autoría.

[Especificación Agent Plugins 1.0](https://agent-plugins.org/specification)

## Mantenimiento histórico de v1 {#historical-v1}

<a id="si-solo-lees-una-cosa"></a>
<a id="valor-predeterminado-recomendado"></a>
<a id="por-que-este-es-el-valor-predeterminado"></a>

Mantén los proyectos plugin.yaml existentes con las instrucciones v1 siguientes. Estas plantillas y salidas generadas son flujos históricos v1; no crean el nuevo flujo de autoría basado en el estándar.

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Lo que obtienes

- un repositorio de complementos desde el primer día
- archivos creados bajo `plugin/`
- generó Codex salida de tiempo de ejecución desde el mismo repositorio
- una verificación de preparación limpia a través de `validate --strict`

### Rutas Node y Python admitidas

Si su equipo ya vive en Node/TypeScript o Python, esas rutas son compatibles y visibles desde el principio:

- `codex-runtime --runtime node --typescript`
- `codex-runtime --runtime python`
- ambas son rutas de ejecución interpretadas localmente, por lo que la máquina de destino aún necesita Node.js `20+` o Python `3.10+`
- Go sigue siendo el valor predeterminado cuando deseas la historia de producción general más sólida.

### Si está comenzando intencionalmente en Node o Python

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

### Qué hacer a continuación

- edite el complemento en `plugin/`
- ejecute `plugin-kit-ai generate ./my-plugin` nuevamente después de los cambios
- ejecute `plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict` nuevamente
- solo entonces agregue otra forma de enviarlo si el producto lo necesita

### Ampliar más tarde

| Si quieres | Añade esto más tarde |
| --- | --- |
| Claude ganchos como producto real | `claude` |
| Paquete oficial Codex | `codex-package` |
| Gemini paquete de extensión | `gemini` |
| Configuración de integración de propiedad del repositorio | `opencode` o `cursor` |

Elija `claude` primero solo cuando los ganchos Claude ya sean el requisito real del producto.

### Lo que se expande más tarde

- el repositorio permanece unificado a medida que agregas más carriles
- los paquetes y las líneas de extensión provienen de la misma fuente de autor
- OpenCode y Cursor encajan cuando el repositorio debe poseer la configuración de integración
- el límite exacto de soporte permanece en los documentos de referencia, no en su primer flujo de inicio

### Después del inicio rápido

- Continúe con [Cree su primer complemento](/es/guide/first-plugin) para el tutorial histórico v1.
- Continúe con [Lo que puede construir](/es/guide/what-you-can-build) si desea el mapa completo del producto.
- Continúe con [Elija un objetivo](/es/guide/choose-a-target) cuando esté listo para hacer coincidir el repositorio con la forma en que desea enviarlo.
- Continúe con [Un proyecto, múltiples objetivos](/es/guide/one-project-multiple-targets) cuando esté listo para expandirse más allá de la primera ruta.
