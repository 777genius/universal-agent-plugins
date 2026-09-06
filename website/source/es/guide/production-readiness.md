---
title: "Preparación para la producción"
description: "Una lista de verificación pública para decidir si un proyecto plugin-kit-ai está listo para CI, transferencia y uso compartido amplio."
canonicalId: "page:guide:production-readiness"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Preparación para la producción

Utilice esta lista de verificación antes de calificar un proyecto como listo para producción, listo para entregar o listo para mostrar ampliamente.

<MermaidDiagram
  :chart="`
flowchart LR
  path[Camino elegido con intención] --> source[Un solo repositorio fuente]
  source --> checks[Puertas de generate y validate]
  checks --> boundary[Límite de soporte confirmado]
  boundary --> handoff[Documentación y traspaso explícitos]
  handoff --> ready[Proyecto listo para producción]
`"
/>

## 1. Elija el camino correcto a propósito

- por defecto es Go cuando desea el carril de tiempo de ejecución más fuerte
- elija Node/TypeScript o Python cuando la compensación de tiempo de ejecución local no Go sea real
- elija paquetes, extensiones o líneas de integración solo cuando esos sean los resultados reales que necesita

## 2. Mantenga un repositorio honesto

- mantener la fuente del proyecto en el diseño estándar del paquete
- trate los archivos de destino generados como resultados, no como el lugar principal donde edita
- No parchee los archivos generados manualmente y espere que `generate` conserve esas ediciones.

## 3. Ejecute las puertas del contrato

Como mínimo, el repositorio debería sobrevivir limpiamente a este flujo:

```bash
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform <target> --strict
```

Para los carriles Go basados en launcher, construya primero `bin/<name>` para que la entrada del launcher exista en disco:

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

Para los carriles de tiempo de ejecución Python y Node, `doctor` y `bootstrap` son parte de la preparación.

## 4. Verifique el límite de soporte exacto

- confirmar que el carril principal y cada carril adicional dentro del alcance estén dentro del límite de apoyo público
- utilice las páginas de referencia cuando necesite términos exactos `public-stable`, `public-beta` o `public-experimental`
- verifique la matriz de soporte de objetivos generada antes de prometer compatibilidad a los usuarios intermedios

## 5. Mantenga la historia de instalación y la historia API separadas

- Los paquetes Homebrew, npm y PyPI son canales de instalación para CLI
- no son superficies APIs o SDK de tiempo de ejecución
- público API vive en la sección generada API y en los flujos de trabajo documentados

## 6. Documentar el traspaso

Un repositorio público debería dejar estas cosas obvias:

- qué carril es primario
- qué carriles adicionales son realmente compatibles
- qué tiempo de ejecución utiliza y si cambia según el objetivo
- qué conjunto de comandos es la puerta de validación canónica
- si depende de un paquete de ejecución compartido o de una ruta Go SDK

## Regla final

Si un compañero de equipo no puede clonar el repositorio, ejecutar el flujo documentado, pasar `validate --strict` y comprender el carril elegido sin conocimientos tribales, el proyecto aún no está listo para producción.

</details>
