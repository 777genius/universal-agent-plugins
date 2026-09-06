---
title: "v1.0.4 Go SDK"
description: "Notas de la versión del parche para la corrección de ruta del módulo Go SDK."
canonicalId: "page:releases:v1-0-4-go-sdk"
section: "releases"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">v1.0.4 Go SDK</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# v1.0.4 Go SDK

Fecha de lanzamiento: `2026-03-29`

## Por qué es importante este parche

Este parche hizo que la ruta pública del módulo Go SDK fuera veraz para el consumo normal de Go.

## ¿Qué cambió?

- la raíz del módulo Go SDK se movió de `sdk/plugin-kit-ai/` a `sdk/`
- la ruta del módulo público `github.com/777genius/plugin-kit-ai/sdk` ahora coincide con el diseño del repositorio real
- Se actualizaron los repositorios iniciales, los ejemplos y las plantillas para dejar de enseñar soluciones alternativas a los recién llegados basadas en `replace`

## Orientación práctica

- utilice `github.com/777genius/plugin-kit-ai/sdk@v1.0.4` o posterior para el consumo normal del módulo Go
- trate `v1.0.3` como conocido como incorrecto para la ruta del módulo Go SDK

## Por qué debería importarles a los usuarios

Este parche redujo la fricción para los consumidores normales de Go e hizo que la ruta recomendada SDK pareciera un módulo público normal en lugar de una solución alternativa para casos especiales.
</details>
