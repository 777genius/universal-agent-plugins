---
title: "Flujo de trabajo de creación"
description: "El flujo de trabajo principal desde init para generar, validar, probar y transferir."
canonicalId: "page:reference:authoring-workflow"
section: "reference"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">Flujo de trabajo de creación</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# Flujo de trabajo de creación

El flujo de trabajo recomendado es intencionalmente simple:

```text
init -> generate -> validate --strict -> test -> handoff
```

<MermaidDiagram
  :chart="`
flowchart LR
  Init[init] --> Generar[generar]
  Generar --> Validar[validar --strict]
  Validar --> Prueba[prueba o controles de humo]
  Prueba --> Traspaso[traspaso]
  Bootstrap[doctor o bootstrap cuando sea necesario] -. soporta .-> Generar
  Arranque -. soporta .-> Validar
`"
/>

## Qué significa cada paso

| Paso | Propósito |
| --- | --- |
| `init` | Crear un diseño de proyecto estándar de paquete |
| `generate` | Generar artefactos de destino desde el origen del proyecto |
| `validate --strict` | Ejecute la verificación de preparación principal |
| `test` | Realice pruebas de humo estable cuando corresponda |
| `export` / flujo de empaquetado | Producir artefactos de entrega para casos compatibles con Python y Node |

## Reglas que mantienen saludable el repositorio

- la fuente del proyecto vive en el diseño estándar del proyecto de paquete
- Los archivos de destino generados son resultados, no la fuente de verdad a largo plazo.
- la validación estricta es una comprobación obligatoria, no un extra opcional

Este flujo de trabajo es igualmente importante para repositorios de un solo objetivo y de múltiples objetivos.

La única diferencia es que en un proyecto de múltiples objetivos, el bucle `generate` y `validate` se repite para cada objetivo que el repositorio realmente promete admitir.

## Cuando cambia el flujo de trabajo

El flujo de trabajo puede ampliarse para casos especiales:

- `doctor` y `bootstrap` son importantes para las rutas de ejecución Python y Node
- `import` y `normalize` son importantes al consolidar archivos de destino administrados manualmente en el modelo de proyecto administrado
- Los comandos del paquete son importantes para los flujos de transferencia portátiles Python y Node

Comience con [Inicio rápido](/es/guide/quickstart) cuando necesite la ruta más corta.
</details>
