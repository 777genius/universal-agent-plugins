---
title: "Modelo objetivo"
description: "En qué se diferencian los resultados de integración de tiempo de ejecución, paquetes, extensiones y repositorios, y cómo elegir la ruta correcta."
canonicalId: "page:concepts:target-model"
section: "concepts"
locale: "es"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Modelo objetivo

Un objetivo es el tipo de resultado que desea que produzca el repositorio.

La elección importante no es la taxonomía abstracta. La elección importante es lo que intenta enviar.

## Regla rápida

- Elija una ruta de ejecución cuando desee un complemento ejecutable.
- Elija una ruta de paquete cuando otro sistema cargue su salida empaquetada.
- Elija una ruta de extensión cuando el host espere un artefacto de extensión.
- Elija una configuración de integración de propiedad del repositorio cuando el repositorio necesite principalmente una configuración registrada para otra herramienta.

## Rutas de ejecución

Los objetivos en tiempo de ejecución producen algo ejecutable. Este es el punto de partida predeterminado para la mayoría de los equipos porque es la forma más clara de controlar el comportamiento, validar los resultados y hacer crecer el repositorio más adelante.

## Rutas de paquetes

Los destinos del paquete producen resultados empaquetados en lugar de la forma de tiempo de ejecución ejecutable principal. Utilícelos cuando el embalaje sea el verdadero requisito de entrega, no sólo una exportación adicional que pueda necesitar más adelante.

## Rutas de extensión

Los destinos de extensión se ajustan a hosts que esperan un artefacto de extensión específico o una forma de paquete instalable.

## Configuración de integración propiedad del repositorio

Algunas salidas son en su mayoría configuraciones registradas que ayudan a otra herramienta o espacio de trabajo a utilizar el complemento. Estas siguen siendo rutas compatibles útiles, pero responden a una pregunta de entrega diferente a la de un tiempo de ejecución ejecutable.

## El modelo mental seguro

Comience con el resultado que necesita primero. Si el repositorio crece más adelante, puede agregar otra salida compatible sin cambiar el hecho de que un proyecto sigue teniendo autoridad.
</details>
