---
title: "Lo que puedes construir"
description: "Utilice esta página como mapa del producto: qué salidas existen, cómo se ve el inicio predeterminado y cómo se puede expandir un repositorio más adelante."
canonicalId: "page:guide:what-you-can-build"
section: "guide"
locale: "es"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Lo que puedes construir

Utilice esta página como mapa del producto. Muestra qué tipos de resultados existen, no cuándo un repositorio debería crecer o dividirse más adelante.

plugin-kit-ai puede comenzar con un complemento ejecutable y expandirse a salidas adicionales compatibles con el tiempo.

## Forma inicial recomendada

Comience con una ruta de ejecución, generalmente Codex tiempo de ejecución con Go. Esto mantiene el primer repositorio simple y le brinda el ciclo de validación y envío más claro.

Si su equipo ya trabaja en Node/TypeScript o Python, esas rutas de inicio también son compatibles.

## Un repositorio, muchas salidas compatibles

A partir de un mismo proyecto, puedes crecer hacia:

- salidas en tiempo de ejecución para hosts compatibles
- salidas empaquetadas cuando el embalaje es el requisito real de entrega
- salidas de extensión para hosts que esperan un artefacto de extensión
- configuración de integración de propiedad del repositorio cuando el repositorio necesita principalmente una configuración registrada para otra herramienta

## Para qué no es esta página

Elegir Node o Python no lo obliga a decidir cada detalle de empaquetado o integración desde el primer día.

Esta página es la descripción general. Si su pregunta es si un repositorio debería seguir creciendo, lea [Un proyecto, múltiples objetivos](/es/guide/one-project-multiple-targets).
</details>
