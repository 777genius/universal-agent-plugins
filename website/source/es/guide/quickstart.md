---
title: "Inicio rápido"
description: "Instala paquetes publicados o prepara tu propio paquete portable Agent Plugins 1.0."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

<a id="inicio-rapido"></a>

# Usar plugins / Crear plugins

Instala un paquete para tus agentes o crea un paquete portable Agent Plugins 1.0 con instrucciones y configuración de herramientas.

## Usar plugins {#use-plugins}

Con Node.js 22 o posterior:

```bash
npx universal-agent-plugins add context7
```

El CLI pregunta qué agentes compatibles quieres usar e indica los pasos de activación pendientes. Para examinar y previsualizar primero:

```bash
npx universal-agent-plugins info context7
npx universal-agent-plugins add context7 --target codex,cursor --dry-run
```

La [guía de instalación nativa](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) ofrece ejecutables para macOS, Linux y Windows. El gestor nativo no necesita Node.js, aunque cada plugin puede requerir su propio entorno de ejecución.

[CLI nativo 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95) conserva el ámbito de proyecto explícito y la selección automática de `install` introducida en 0.1.94. También añade una [API local de gestión](../use/manage#local-management-api) y [extensiones nativas de Pi](../reference/client-compatibility#pi-native-extensions). Para previsualizar una instalación de proyecto:

Para una entrada revisada y compatible del catálogo, copia su origen Git fijado a un SHA completo y el cliente adecuado. Los alias de Directory v1, como `context7`, solo sirven en el ámbito de usuario. Sustituye `OWNER/REPOSITORY`, `FULL_COMMIT_SHA` y `PACKAGE_PATH`; si el paquete está en la raíz del repositorio, omite `//PACKAGE_PATH`.

```bash
npx universal-agent-plugins add 'github:OWNER/REPOSITORY@FULL_COMMIT_SHA//PACKAGE_PATH' --target codex --scope project \
  --project-root /path/to/project --dry-run
```

Elige un directorio de proyecto existente. `add` usa por defecto el ámbito de usuario: selecciona proyecto explícitamente para esta operación. Desde [CLI nativo 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94), `install` sin argumento de origen detecta el proyecto solo si existe un `uap.toml` regular en el directorio actual exacto; no busca en los superiores. Un ámbito explícito tiene prioridad. Esa selección automática exige 0.1.94 o posterior: comprueba `agentplugins version` y usa el instalador nativo para obtener la versión verificada. Los canales de npm y Homebrew pueden actualizarse más tarde; los ejemplos `npx` con ámbito explícito de esta página siguen siendo compatibles con 0.1.93. Revisa el plan antes de omitir `--dry-run`. La detección nativa, la confianza, la autenticación y las llamadas MCP correctas siguen siendo independientes de escribir los archivos del paquete.

[Recorrido completo de instalación](../use/install) · [Compatibilidad de clientes y límites de las pruebas](../reference/client-compatibility)

### Restaura unas dependencias guardadas

Consulta lo registrado en el entorno de usuario seleccionado:

```bash
npx universal-agent-plugins deps status
```

Para un par `uap.toml`/`uap.lock` guardado, exporta desde una carpeta de intercambio:

```bash
npx universal-agent-plugins deps export
```

Copia la carpeta a otra máquina, instala los clientes de destino y ejecuta allí:

```bash
npx universal-agent-plugins deps import
npx universal-agent-plugins install --locked --scope user
```

Solo se transfieren el acuerdo y el bloqueo validados. Perfiles, ajustes, credenciales, recibos y datos del plugin se mantienen aparte. La instalación sigue necesitando los orígenes exactos o paquetes verificados en caché. Para un proyecto, guarda ambos archivos en control de versiones y restaura con `install --locked --scope project` explícito. Consulta [mantenimiento y restauración](../use/manage) para conocer recuperación y transferencia.

## Crear plugins {#build-plugins}

Un paquete estándar comienza con `plugin.json` en la raíz y puede incluir `skills/<name>/SKILL.md`, `mcp.json` o ambos. Puedes escribir esos archivos a mano y validar el paquete con el instalador publicado:

```bash
npx universal-agent-plugins validate ./my-plugin
npx universal-agent-plugins add ./my-plugin --target codex --dry-run
```

Estos ejemplos suponen que `./my-plugin` ya contiene un paquete completo. La [guía de creación](../build/) explica un paquete Skill manual y las herramientas que generan estructuras iniciales desde el código fuente.

**Disponibilidad de creación:** el instalador actual npm o nativo no incluye `agentplugins author`. El código fuente actual implementa generación inicial, comprobaciones estáticas, pruebas MCP explícitas, ciclos de desarrollo, instalación de dependencias y mantenimiento JSON. Usa [la preparación desde el código fuente](../build/#build-from-source) para esos comandos. Las pruebas de publicaciones anteriores corresponden a sus propias versiones; no prometen lo mismo para el instalador actual.

La validación estática no demuestra ejecución, OAuth ni activación. Los transportes MCP admitidos también dependen del cliente: por ejemplo, Codex no admite MCP SSE declarado. Explica los requisitos del paquete y las comprobaciones pendientes en el README.

[Especificación Agent Plugins 1.0](https://agent-plugins.org/specification)


<!-- locale-historical-source:start
locale-historical-frontmatter:start
---
title: "Inicio rápido"
description: "La ruta recomendada más rápida para un proyecto plugin-kit-ai en funcionamiento."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---
locale-historical-frontmatter:end
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
locale-historical-source:end -->
