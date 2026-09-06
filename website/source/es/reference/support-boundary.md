---
title: "Límite de soporte"
description: "La respuesta práctica más corta a lo que plugin-kit-ai recomienda, apoya con cuidado y sigue siendo experimental."
canonicalId: "page:reference:support-boundary"
section: "reference"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">Límite de soporte</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# Límite de soporte

Utilice esta página cuando necesite la respuesta más breve y honesta sobre soporte.

Responde a tres preguntas del equipo:

- qué es seguro recomendar de forma predeterminada
- lo que se admite, pero debe elegirse a propósito
- lo que todavía es experimental y no debería convertirse silenciosamente en una política de equipo

## Valores predeterminados seguros

Estos son los valores predeterminados más seguros en la actualidad:

- Go es la ruta de ejecución predeterminada recomendada.
- `validate --strict` es la puerta de preparación principal para los repositorios de tiempo de ejecución locales Python y Node.
- `Codex runtime Go`, `Codex package`, `Gemini packaging`, `Gemini Go runtime` y el carril estable predeterminado Claude son los principales carriles de producción recomendados.
- `Python` y `Node` son rutas compatibles que no son Go y la opción recomendada que no es Go cuando la compensación del tiempo de ejecución interpretado local es intencional.

## Cómo se relaciona esto con el contrato formal

Los documentos públicos utilizan primero tres palabras simples:

- `Recommended` generalmente se asigna a los carriles de producción actuales más fuertes `public-stable`.
- `Advanced` significa una superficie de apoyo con un contrato más estrecho, más especializado o más cuidadoso.
- `Experimental` significa abandono de participación fuera de la expectativa de compatibilidad normal.

Cuando un equipo necesita un lenguaje de políticas exacto, los términos formales ganan: `public-stable`, `public-beta` y `public-experimental`.

## Recomendado hoy

Si necesita una respuesta práctica, comience aquí:

- Se recomienda Claude en la ruta de enlace estable predeterminada.
- Se recomienda Codex tanto para la ruta de ejecución `Notify` como para la ruta oficial `codex-package`.
- Se recomienda el empaquetado Gemini y el tiempo de ejecución promocionado Gemini Go también está listo para producción.
- OpenCode y Cursor son rutas de configuración de integración propiedad del repositorio. Son útiles, pero no son el inicio del tiempo de ejecución ejecutable predeterminado.

## Superficies avanzadas

Elija superficies avanzadas sólo cuando la compensación sea explícita y valga la pena.

Ejemplos típicos:

- OpenCode y Cursor cuando el repositorio debe poseer la configuración de integración en lugar de enviar una ruta de ejecución
- expansiones de tiempo de ejecución más limitadas o especializadas más allá de las principales rutas recomendadas
- instale contenedores cuando la verdadera preocupación sea la entrega CLI, no el tiempo de ejecución APIs o SDKs
- superficies de configuración especializadas que son útiles, pero no las primeras predeterminadas para la mayoría de los equipos

## Superficies experimentales

Trate las áreas experimentales como voluntarias y de alta rotación.

Pueden ser útiles para los primeros usuarios, pero no deberían convertirse silenciosamente en un estándar a largo plazo para el equipo.

## Regla práctica

Si elige formar parte de un equipo, estandarice el camino más estrecho cuya promesa realmente esté dispuesto a defender en CI, implementación y transferencia.
</details>
