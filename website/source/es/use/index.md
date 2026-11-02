---
title: "Usar plugins"
description: "Instala paquetes para tus agentes y mantén separadas las dependencias de cada proyecto."
canonicalId: "page:use:index"
section: "use"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Usar plugins

Universal Agent Plugins instala paquetes estándar para los agentes de IA que elijas. Un paquete puede contener Skills, configuración de servidores MCP o ambos componentes. Puedes instalarlo para tu cuenta o mantener separadas las dependencias de un proyecto.

## Obtén el CLI

Con Node.js 22 o posterior, ejecuta directamente el envoltorio npm:

```bash
npx universal-agent-plugins search docs
```

Para usarlo con frecuencia, instálalo globalmente:

```bash
npm install -g universal-agent-plugins
agentplugins version
```

También puedes usar Homebrew:

```bash
brew install 777genius/agentplugins/agentplugins
```

La [guía de instalación nativa](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) incluye instrucciones para macOS, Linux y Windows. El gestor nativo no necesita Node.js, aunque cada plugin puede requerir su propio entorno de ejecución. Los ejemplos de este recorrido usan `agentplugins`; `npx universal-agent-plugins` ejecuta los mismos comandos de instalación.

## Sigue el recorrido de instalación

1. [Busca y examina un paquete](./install). Comprueba su procedencia y sus requisitos.
2. Previsualiza los agentes y el ámbito previstos con `--dry-run`.
3. Aplica la instalación revisada y sigue sus instrucciones de activación.
4. [Mantén el estado instalado](./manage): consulta, actualiza, repara o elimina paquetes.
5. Cuando lo necesites, restaura unas dependencias compartidas desde `uap.toml` y `uap.lock`.

[CLI nativo 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95) es el instalador nativo actual. Añade una API local de gestión y extensiones nativas de Pi en el ámbito de usuario. Consulta [la gestión local](./manage#local-management-api) y [la compatibilidad con Pi](../reference/client-compatibility#pi-native-extensions). Los canales de los gestores de paquetes pueden actualizarse más tarde: comprueba `agentplugins version`. El instalador nativo ofrece la versión actual verificada.

El ámbito de proyecto explícito está disponible desde 0.1.93. La mayoría de los comandos usa por defecto el ámbito de usuario. Desde 0.1.94, `agentplugins install` sin argumento de origen ni ámbito explícito selecciona el proyecto si hay un `uap.toml` regular en el directorio actual exacto; no busca en directorios superiores. Selecciona `--scope project` para las operaciones de mantenimiento de un proyecto. Los componentes admitidos y la activación dependen del cliente.

## Interpreta cada resultado

La instalación puede dejar pendientes una recarga, la confianza en el proyecto, la activación manual o un paso OAuth. El resultado Prepared indica que aún quedan pasos en el cliente. Lee el resultado de cada destino; la operación completa no demuestra por sí sola que el plugin funcione durante la ejecución. Consulta la [compatibilidad de clientes](../reference/client-compatibility) para conocer el alcance de las observaciones nativas y sus versiones.

## Si vas a crear un paquete

Empieza por [Crear plugins](../build/). Lo que se entrega al instalador es un directorio con `plugin.json` en la raíz y sus componentes. El usuario que instala el paquete no necesita ejecutar las herramientas de creación compiladas desde el código fuente.

| Comando | Función |
| --- | --- |
| `agentplugins validate ./my-plugin` | Leer y validar un paquete local para instalarlo. |
| `agentplugins doctor` | Diagnosticar el estado gestionado por el instalador. |
| `agentplugins author validate ./my-plugin` | Comprobar la conformidad y preparación del paquete en una compilación con creación habilitada. |
| `agentplugins author doctor ./my-plugin` | Examinar las pruebas del proyecto y de sus herramientas en esa compilación. |

La versión actual del instalador npm o nativo no incluye `agentplugins author`.
