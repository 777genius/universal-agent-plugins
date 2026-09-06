---
title: "Fuente y resultados del proyecto"
description: "Cómo encajan los archivos creados, los resultados generados, la validación estricta y la transferencia en plugin-kit-ai."
canonicalId: "page:concepts:authoring-architecture"
section: "concepts"
locale: "es"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Fuente y resultados del proyecto

Esta página es más estrecha que el modelo principal del producto. Explica los límites de trabajo dentro del repositorio: lo que usted crea, lo que se genera y por qué esa división mantiene el proyecto mantenible.

## La forma del núcleo

```text
project source -> generate -> target outputs -> validate --strict -> handoff
```

La fuente se mantiene estable. Las salidas pueden cambiar según el objetivo. La validación garantiza que el resultado generado aún sea seguro para transmitir.

## Archivos creados frente a archivos generados

Los archivos creados son la parte del repositorio que se espera que mantenga directamente.

Los archivos generados son artefactos de compilación para los objetivos que eligió. Son resultados reales de la entrega, pero no son el lugar donde la verdad del proyecto debería derivar.

Esa distinción mantiene el repositorio legible y hace que la regeneración sea segura.

## Por qué es importante la división

Sin una división clara, los equipos terminan editando los resultados generados, perdiendo repetibilidad y haciendo que las actualizaciones sean más difíciles de lo necesario.

Con una división clara, puedes:

- revisar los cambios de fuente directamente
- regenerar la salida con confianza
- validar la misma forma de entrega cada vez
- agregar otra salida compatible más adelante sin reconstruir el repositorio desde cero

## Cómo se relaciona esto con el modelo más amplio

Si desea una explicación de nivel superior, comience con [Cómo funciona plugin-kit-ai](/es/concepts/managed-project-model).
</details>
