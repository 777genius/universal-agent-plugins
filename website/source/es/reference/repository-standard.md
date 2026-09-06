---
title: "Estándar de repositorio"
description: "Cómo debería verse un repositorio plugin-kit-ai saludable y cómo separar el origen del proyecto de los resultados generados."
canonicalId: "page:reference:repository-standard"
section: "reference"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Estándar de repositorio

Esta página define la forma pública de un repositorio `plugin-kit-ai` saludable.

## La regla principal

El repositorio debe hacer obvia su configuración prevista y sus resultados generados reproducibles.

En la práctica, eso significa:

- la fuente del proyecto es fácil de localizar
- Los archivos de destino generados son claramente resultados.
- el objetivo principal u objetivos dentro del alcance son visibles
- la elección del tiempo de ejecución o la política de tiempo de ejecución es visible
- el comando de validación está documentado

## ¿Qué debería ser fácil de encontrar?

Un repositorio saludable debería hacer que estas cosas sean detectables sin necesidad de excavar:

- el objetivo principal u objetivos dentro del alcance
- el tiempo de ejecución elegido o la política de tiempo de ejecución por objetivo
- el comando canónico `validate --strict`, o los comandos de validación si hay varios objetivos
- requisitos previos de tiempo de ejecución como Go, Python o Node
- si el repositorio utiliza una ruta Go SDK o un paquete de tiempo de ejecución compartido

## Lo que no debería ser la fuente de la verdad

Estos no deberían actuar como la principal fuente de verdad:

- archivos de destino generados editados a mano
- paquetes de instalación de contenedor tratados como tiempo de ejecución APIs
- conocimiento tribal sobre "el comando que realmente necesitas ejecutar"

## Señales de repositorio saludable

- `generate` puede reproducir las salidas de destino
- `validate --strict` pasa limpiamente para el objetivo previsto, o para cada objetivo que el repositorio afirma públicamente respaldar
- el repositorio explica la ruta elegida en documentos públicos o material README
- CI utiliza el mismo flujo de preparación pública que el desarrollo local.

## Señales de repositorio débiles

- los archivos de destino se parchean manualmente después de la generación
- la elección del tiempo de ejecución o del objetivo es implícita o inconsistente en todas las máquinas
- Los usuarios intermedios necesitan la orientación del mantenedor para reproducir el flujo básico.
- el repositorio promete soporte para áreas fuera del límite de soporte declarado

## Relación con este sitio de documentos

Este sitio de documentos públicos trata el estándar de repositorio como el lugar donde:

- la guía de creación se vuelve operativa
- los límites de apoyo se vuelven ejecutables
- el traspaso se vuelve creíble

Empareje esta página con [Flujo de trabajo de creación](/es/reference/authoring-workflow), [Preparación para la producción](/es/guide/production-readiness) y [Glosario](/es/reference/glossary).
</details>
