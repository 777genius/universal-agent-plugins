---
title: "Añadir más Skills"
description: "Amplía un paquete estándar existente con otro Skill inmediato."
canonicalId: "page:build:skills"
section: "build"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Añadir más Skills

**Creación desde el código fuente:** estos comandos requieren la [compilación con creación habilitada](./#build-from-source). No están incluidos en el instalador actual.

`skills init` añade un Skill a un paquete estándar existente, ya sea Skill, MCP o híbrido. Selecciona la raíz que contiene `plugin.json`, no el directorio `skills/` ni un archivo `SKILL.md` individual.

## Indica el Skill nuevo y el paquete

Supón que `./review-helper` es el paquete del [recorrido de un Skill](./skill) y que `skills/release-notes/` no existe:

```bash
agentplugins author skills init release-notes ./review-helper \
  --description 'Use when drafting release notes from reviewed changes'
```

El nombre posicional y una descripción explícita de 1 a 1024 caracteres son obligatorios. Los nombres no se normalizan. Usa un nombre portable exacto, en minúsculas y con guiones, y un destino nuevo; no esperes que se sobrescriba uno existente.

## Revisa el componente añadido

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
    release-notes/
      SKILL.md
```

Edita `skills/release-notes/SKILL.md` para explicar cuándo usarlo, su método y la salida esperada. Mantén precisos el nombre y la descripción. Por ejemplo, pídele distinguir los cambios visibles para usuarios de los de mantenimiento y citar el origen revisado de cada afirmación sobre la versión.

Los scripts, referencias y recursos pueden apoyar las instrucciones, pero su presencia no demuestra ejecución ni da permiso para ejecutarlos. `allowed-tools` no es una autorización independiente. Revisa el material de apoyo con el mismo cuidado que las instrucciones.

## Valida el paquete y los Skills inmediatos

```bash
agentplugins author skills validate ./review-helper
```

El comando informa tanto de la preparación global del paquete como de cada Skill inmediato por separado. Un error en otra parte del paquete puede afectar al resultado global aunque el Skill nuevo no tenga problemas.

Sin ruta, usa solo el directorio actual y no busca en los superiores. Las raíces explícitas aclaran scripts y diagnósticos. Ejecutar desde `skills/release-notes/` no sustituye la selección de la raíz del paquete.

## Resuelve un fallo sin sobrescribir trabajo

Si falla la creación, lee `committed` y las instrucciones del error. El Skill puede haberse guardado antes de que fallen o queden incompletas las comprobaciones del paquete. Examina los archivos y el informe antes de reintentar; un segundo `init` no sobrescribirá el destino existente.

Corrige la entrada o el problema del paquete y repite la validación. Si el nombre es incorrecto, revisa explícitamente el cambio de origen previsto en vez de añadir una opción de forzado. Este recorrido de creación no incluye la instalación, actualización o eliminación de Skills externos ni un ciclo `generate`.

Continúa con [las comprobaciones estáticas](./checks) y [la entrega](./handoff).
