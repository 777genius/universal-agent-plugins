---
title: "Un proyecto, múltiples objetivos"
description: "Cómo decidir cuándo un repositorio debería crecer hasta alcanzar más resultados, cuándo debería mantenerse estrecho y cuándo es el momento de dividirse."
canonicalId: "page:guide:one-project-multiple-targets"
section: "guide"
locale: "es"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Un proyecto, múltiples objetivos

Utilice esta página después del primer repositorio en funcionamiento, cuando la verdadera pregunta sea: ¿debería crecer este mismo repositorio y, de ser así, hasta qué punto?

## La regla corta

Un repositorio puede cubrir de forma segura más de una salida cuando la misma lógica del complemento, la misma intención de lanzamiento y el mismo modelo de propiedad aún se mantienen unidos.

## Cuándo debería crecer un repositorio

Haga crecer el mismo repositorio cuando:

- el comportamiento del complemento sigue siendo un producto coherente
- el nuevo resultado es otra forma de entregar el mismo complemento
- un equipo aún puede poseer limpiamente la fuente escrita
- la regeneración y la validación mantienen el repositorio fácil de revisar

## Cuándo un repositorio debe mantenerse estrecho

Mantenga el repositorio enfocado cuando la producción actual ya resuelva la necesidad real y las salidas adicionales solo agregarían gastos generales de mantenimiento.

## Cuándo dividir los repositorios

Dividir repos cuando el producto deja de ser una sola cosa en la práctica:

- diferentes equipos son dueños del trabajo
- el tiempo de liberación diverge
- El comportamiento diverge más allá de la simple adaptación al objetivo.
- sería más difícil razonar sobre el repositorio que sobre dos repositorios más pequeños

## El modelo mental seguro

Comience de manera limitada, valide una salida funcional y solo luego haga crecer el repositorio con otra salida compatible.
</details>
