---
title: "Cómo funciona plugin-kit-ai"
description: "Cómo un repositorio sigue siendo la fuente de la verdad mientras usted genera resultados, valida estrictamente y entrega un resultado limpio."
canonicalId: "page:concepts:managed-project-model"
section: "concepts"
locale: "es"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Cómo funciona plugin-kit-ai

plugin-kit-ai mantiene un repositorio como fuente de verdad para su complemento. Usted edita los archivos que posee, genera los resultados que necesita, valida el resultado estrictamente y entrega un repositorio que se mantiene predecible a lo largo del tiempo.

## La versión corta

El bucle central es simple:

```text
source -> generate -> validate --strict -> handoff
```

Ese bucle es importante porque el proyecto no es sólo una plantilla inicial. El resultado generado puede cambiar a medida que evoluciona el objetivo, mientras que la fuente creada permanece clara y fácil de mantener.

## Un repositorio como fuente de verdad

El repositorio es donde vive realmente el complemento.

- los archivos creados permanecen bajo su control
- los resultados generados se reconstruyen a partir de esa fuente
- la validación verifica el resultado que planea enviar
- la transferencia ocurre solo después de que el resultado generado esté limpio

Esto permite que un proyecto crezca cuidadosamente en lugar de distribuir la misma lógica de complemento en varios repositorios.

## Lo que realmente editas

Sigues editando la fuente del proyecto y el código del complemento que posees. No se trata el resultado generado como el lugar donde realmente vive el proyecto.

Ese límite es lo que mantiene manejables las actualizaciones, los cambios de objetivos y el trabajo de mantenimiento.

## Por qué esto es más que plantillas iniciales

Una plantilla inicial te da una forma inicial. plugin-kit-ai sigue gestionando el bucle después del primer día:

- regenera resultados específicos del objetivo desde la misma fuente
- valida lo que estás a punto de enviar
- mantiene los archivos creados y los archivos generados claramente separados
- permite que un repositorio se expanda a más resultados más adelante sin tener que reescribir todo el modelo del proyecto

## Dónde Go Siguiente

- Lea [Fuente y resultados del proyecto](/es/concepts/authoring-architecture) para conocer el límite entre creación y generación.
- Lea [Modelo de destino](/es/concepts/target-model) para conocer los diferentes tipos de salida.
- Lea [Un proyecto, múltiples objetivos](/es/guide/one-project-multiple-targets) cuando desee hacer crecer un repositorio más.
</details>
