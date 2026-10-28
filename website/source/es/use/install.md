---
title: "Buscar e instalar un plugin"
description: "Examina el origen de un paquete, previsualiza sus cambios y completa su activación."
canonicalId: "page:use:install"
section: "use"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Buscar e instalar un plugin

Elige un paquete cuyo origen y finalidad conozcas. Estos comandos usan el instalador publicado; consulta [cómo obtener el CLI](./) si aún no tienes `agentplugins`. Revisa el plan con `--dry-run` antes de aplicar cambios.

## Descubre el paquete

```bash
agentplugins search docs
agentplugins info context7
```

Un nombre corto como `context7` procede del registro firmado Universal Agent Plugins Registry. La búsqueda y la información del paquete te ayudan a elegir; no lo ejecutan ni demuestran que su servicio esté disponible. Antes de instalar, revisa las herramientas y Skills que ofrece, los requisitos de ejecución y el acceso a cuentas que necesita.

## Previsualiza una selección explícita

```bash
agentplugins add context7 --target codex,cursor --dry-run
```

Revisa el origen, los agentes seleccionados, los destinos, los hallazgos de seguridad y los pasos de activación pendientes. Este comando muestra un plan sin aplicar la instalación. No prueba un servidor MCP.

## Instala tras revisar el plan

```bash
agentplugins add context7 --target codex,cursor
```

Para elegir de forma interactiva, omite `--target`. El CLI puede seleccionar automáticamente un único agente detectado u ofrecer un selector si hay varios. Usa destinos explícitos para repetir una selección con precisión o automatizarla.

El instalador comprueba los clientes elegidos y protege los archivos que no le pertenecen. Si una operación informa de una reversión fallida o una recuperación pendiente, conserva el informe y sigue sus instrucciones de recuperación antes de reintentar.

## Completa la activación

Sigue los pasos específicos de cada cliente: pueden quedar pendientes una recarga, una sesión nueva, la confianza en el proyecto, el inicio de sesión o la activación explícita. Los archivos preparados y el registro nativo correcto no demuestran que OAuth haya terminado ni que las llamadas a herramientas funcionen. Usa el [diagnóstico del estado gestionado](./manage) si falta la instalación o sus archivos han cambiado; comprueba la autenticación del cliente si el paquete está presente pero no disponible.

## Usa un paquete local

Selecciona el directorio con `plugin.json` en la raíz:

```bash
agentplugins validate ./my-plugin
agentplugins add ./my-plugin --target codex --dry-run
```

La validación y la planificación del instalador también aplican la política de seguridad. Pueden descargar metadatos de seguridad o una versión fijada del analizador incluso para un paquete local. `--dry-run` evita cambios gestionados de instalación, pero no promete una operación sin conexión.

Después de revisar el plan, omite `--dry-run` para instalar. Un directorio local sigue siendo una instalación local; no se convierte en una dependencia portable que otra máquina pueda descargar. Pide al autor las [pruebas que acompañan la entrega](../build/handoff).

## Selecciona un origen GitHub exacto

La sintaxis inmutable de GitHub exige el SHA completo de un commit, de 40 caracteres. Sustituye este ejemplo por un repositorio real, una revisión examinada y la ruta del paquete:

```text
agentplugins add owner/repository@0123456789abcdef0123456789abcdef01234567//path/to/plugin --target codex --dry-run
```

La forma `owner/repository@...` rechaza ramas, etiquetas y SHA abreviados. Sin ruta, el instalador selecciona un paquete válido en la raíz o el único candidato anidado válido. Si hay varios candidatos, exige una ruta explícita; si hay más de 16 paquetes posibles, la exige antes de descargarlos para evaluarlos. La reparación vuelve a usar el origen registrado. Cambiar una instalación inmutable a otro origen es una operación de selección independiente.

CLI 0.1.93 también admite Git por HTTPS directo, SSH, URL de navegación de proveedores compatibles y archivos completos HTTPS o locales. Sustituye estos ejemplos por orígenes de paquetes que hayas revisado:

```text
agentplugins add 'https://git.example.com/team/tools.git#refs/heads/main//plugins/demo' --target codex --dry-run
agentplugins add 'ssh://git@git.example.com/team/tools.git#main//plugins/demo' --target codex --dry-run
agentplugins add 'https://downloads.example.com/plugin.tar.gz' --target codex --dry-run
```

Git usa las credenciales nativas y la confianza SSH existentes. Los archivos HTTPS no llevan credenciales y deben contener un paquete estándar completo. El bloqueo fija revisiones Git exactas o los bytes originales del archivo, además de los resúmenes del paquete. Un origen móvil puede avanzar mediante una actualización explícita; la restauración bloqueada conserva la revisión fijada. Estos orígenes de paquetes completos son distintos de la obtención de Skills independientes. Consulta [los orígenes directos de paquetes completos](https://github.com/777genius/universal-agent-plugins/blob/main/docs/DIRECT_PACKAGE_SOURCES.md) para conocer sintaxis por proveedor, selección de paquetes, sumas de comprobación y límites de extracción.

## Instala Agent Skills {#install-agent-skills}

Usa el grupo `skills` para instalar Skills independientes desde un repositorio público de GitHub, una carpeta o una URL de `SKILL.md`, o un editor HTTPS que publique un índice de Skills completo en `.well-known`:

```bash
agentplugins skills add owner/repository --ui
```

Sustituye `owner/repository` por un origen real. `--ui` abre el selector incluido para el navegador y necesita un terminal interactivo. Usa `--no-ui` para seleccionar en el terminal. Para elegir de forma explícita:

```bash
agentplugins skills add owner/repository --skill review --skill pdf \
  --target codex,claude --no-ui --yes
```

- Repite `--skill` para indicar nombres exactos. Si un nombre es ambiguo, usa `--path` con la ruta exacta de la carpeta del Skill dentro del repositorio.
- `--ref` selecciona una referencia Git exacta cuando los límites de la URL son ambiguos.
- `--dry-run` previsualiza la instalación elegida.
- El uso JSON o no interactivo requiere una selección explícita de Skills y destinos, además de `--yes`, salvo al previsualizar con `--dry-run`.
- `--ui` no se puede combinar con `--skill`, `--target` ni `--dry-run`.
- Los editores HTTPS requieren un nombre `--skill` exacto y un índice que enumere todos los archivos de ese Skill.

Los hallazgos automáticos de seguridad pueden bloquear la instalación. `--security-details` los muestra en la salida para personas; revísalos antes de decidir si procede usar `--accept-security-risk`. Los Skills instalados aparecen en `agentplugins list` y se mantienen con los comandos habituales.

## Instala para un proyecto

El ámbito de proyecto explícito está disponible desde [CLI 0.1.93](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93). Elige un directorio existente e indica el ámbito en cada comando:

```bash
agentplugins add context7 --target cursor --scope project \
  --project-root /path/to/project --dry-run
agentplugins skills add owner/repository --ui --scope project \
  --project-root /path/to/project
```

El primer comando muestra una vista previa; omite `--dry-run` para aplicarla. El segundo abre un selector con ese ámbito y puede aplicar tu selección. Sin `--project-root`, el ámbito de proyecto explícito usa el directorio desde el que ejecutas el comando. No busca en carpetas superiores. En los demás casos, el CLI publicado usa por defecto el ámbito de usuario.

El destino debe admitir los componentes de proyecto solicitados. La instalación de proyecto para ChatGPT no está admitida y se rechaza explícitamente. VS Code prepara `.github/skills` y las entradas `servers` que pertenecen a la instalación en `.vscode/mcp.json`, para una sola carpeta local. Su resultado sigue siendo Prepared: revisa la confianza del espacio de trabajo y de los servidores, y examina los ajustes del cliente. Esa observación de metadatos no demuestra el arranque de MCP, las llamadas a herramientas ni la ejecución de modelos.

Consulta la [guía de instalación de proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) y los [límites de proyecto por cliente](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md) para conocer las rutas y la activación. Continúa con [mantenimiento y restauración](./manage).
