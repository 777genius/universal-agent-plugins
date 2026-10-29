---
title: "Entender la raíz del paquete"
description: "Mantén la identidad y los componentes portables en una raíz estándar explícita."
canonicalId: "page:build:layout"
section: "build"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Entender la raíz del paquete

**Creación desde el código fuente:** estos comandos requieren la [compilación con creación habilitada](./#build-from-source). El instalador actual no incluye `agentplugins author`.

El directorio seleccionado es la raíz del paquete. Las herramientas de creación leen su `plugin.json` y los componentes estándar. No buscan un proyecto en carpetas superiores ni recurren al manifiesto oculto de un cliente nativo.

## Estructuras mínima y combinada

Paquete con un Skill:

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

Paquete híbrido remoto:

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

Un paquete stdio incluye además el código del servidor y los manifiestos de dependencias. Son necesarios para ejecutarlo después, pero su presencia no hace que el lector de creación ejecute código.

## Asigna una responsabilidad clara a cada archivo

| Archivo o directorio | Responsabilidad |
| --- | --- |
| `plugin.json` | Identidad y metadatos estándar del paquete. |
| `mcp.json` | Configuración de servidores MCP cuando hay herramientas. |
| `skills/<name>/SKILL.md` | Identidad, descripción e instrucciones de un Skill inmediato. |
| `README.md` | Uso previsto, requisitos, validación y límites pendientes. |
| `package.json`, `package-lock.json`, `src/server.mjs` | Entradas de la estructura Node stdio para el trabajo de ejecución separado. |
| `LICENSE` | Licencia seleccionada explícitamente, si se indicó durante `init`. |

El manifiesto generado comienza en la versión `0.1.0`. Es un metadato de tu paquete nuevo, no la versión del ejecutable de creación. Distingue ambas al informar a los usuarios.

## Identidad y metadatos opcionales

Indica `--name` y `--description` explícitamente en instrucciones de `init` repetibles. El contrato público puede deducir el nombre de un destino simple y proporciona una descripción predeterminada determinista, pero rutas como `./review-helper` deben llevar un nombre explícito. Estos ejemplos indican ambos para evitar identidades accidentales.

En plantillas con un Skill, `--skill-name` usa el nombre del paquete si se omite. Elígelo explícitamente si el Skill necesita otra identidad. Los nombres de Skills nunca se normalizan silenciosamente.

`--author-name` indica un autor opcional explícito; no se deduce de Git ni del entorno. La generación de licencia admite MIT o ISC explícitas y exige `--copyright-holder` y un `--copyright-year` de cuatro dígitos. No se elige una licencia por defecto.

## Selecciona siempre la misma raíz

```bash
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper
agentplugins author test ./research-helper
```

Los tres comandos seleccionan la misma raíz. `./research-helper/skills` sería el directorio equivocado. En comandos de lectura, la ruta opcional usa solo el directorio actual, no un padre descubierto.

La conformidad estándar y la seguridad del sistema de archivos son distintas. Una identidad válida según el esquema puede no ser adecuada para una ruta del equipo, y leer el manifiesto no garantiza que estén completas las pruebas de todos los componentes. Revisa el informe completo.

Continúa con [las comprobaciones estáticas](./checks) y [la entrega](./handoff).
