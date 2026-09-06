---
title: "Configuración de paquetes y integración"
description: "Cuando el empaquetado o la configuración de integración registrada es la respuesta correcta en lugar de un complemento de tiempo de ejecución ejecutable."
canonicalId: "page:guide:package-and-workspace-targets"
section: "guide"
locale: "es"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Configuración de paquetes y integración

No todos los proyectos deberían enviarse como un complemento de tiempo de ejecución ejecutable.

A veces, el requisito real es un paquete que cargará otro sistema, un artefacto de extensión o una configuración de integración registrada que se encuentra en el repositorio.

## La regla corta

Elija paquetes o configuración de integración cuando la forma de entrega sea más importante que ejecutar el complemento directamente.

## Elija esta página cuando

Este es el camino correcto cuando:

- el embalaje es el verdadero requisito de entrega
- el anfitrión espera una extensión o un artefacto empaquetado
- el repositorio necesita principalmente una configuración de integración registrada para otra herramienta
- un tiempo de ejecución ejecutable agregaría trabajo operativo innecesario

## ¿Qué lo diferencia de una ruta de ejecución?

Una ruta de tiempo de ejecución suele ser la opción predeterminada más clara cuando desea un complemento ejecutable.

Los paquetes y la configuración de integración responden a una pregunta diferente: ¿cómo se debe entregar o conectar este complemento a otro sistema?

## El modelo mental seguro

Elija el tiempo de ejecución cuando desee ejecutar el complemento directamente. Elija paquetes o configuración de integración cuando la forma de entrega sea el requisito principal.

## Codex Límite del paquete

Para la ruta oficial del paquete Codex, mantenga el diseño del paquete explícito y limitado:

- `.codex-plugin/` contiene solo `plugin.json`
- `.app.json` y `.mcp.json` opcionales permanecen en la raíz del complemento

Esta ruta del paquete es para la superficie oficial del paquete de complementos Codex, no para mezclar el cableado del tiempo de ejecución local del repositorio en el diseño del paquete.

</details>
