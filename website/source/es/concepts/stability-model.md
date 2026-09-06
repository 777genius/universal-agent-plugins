---
title: "Modelo de estabilidad"
description: "Cómo plugin-kit-ai clasifica las áreas pública estable, pública beta y pública experimental."
canonicalId: "page:concepts:stability-model"
section: "concepts"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Modelo de estabilidad

`plugin-kit-ai` utiliza términos contractuales formales para que los equipos puedan decidir exactamente qué quieren estandarizar.

<MermaidDiagram
  :chart="`
flowchart TD
  Stable[public stable] --> Beta[beta pública]
  Beta --> Experimental[experimental público]
  StableNote[Expectativas de producción normales] -.-> Estable
  BetaNote[Soportado pero no congelado] -.-> Beta
  Nota experimental[Optar por abandonar] -.-> Experimental
`"
/>

## Lenguaje público versus lenguaje formal

Los documentos públicos utilizan un vocabulario de primer paso más simple:

- `Recommended` generalmente apunta a las rutas de corriente más fuertes `public-stable`
- `Advanced` puntos en superficies de soporte que son más estrechas o más especializadas
- `Experimental` se asigna a `public-experimental`

Cuando se establece una política de compatibilidad, los términos formales ganan.

## Cómo leer recomendado

`Recommended` es el lenguaje del producto, no un reemplazo del contrato formal.

- normalmente significa una ruta de producción promocionada `public-stable`
- no significa paridad en todos los objetivos
- no mejora las superficies `public-beta` o `public-experimental` solo con palabras

## Público-Estable

Trate `public-stable` como el nivel que puede desarrollar con expectativas de producción normales.

Este es el nivel que la mayoría de los equipos deberían preferir para los estándares predeterminados y la implementación a largo plazo.

## Beta pública

Trate `public-beta` como compatible, pero no congelado.

Utilice la versión beta sólo cuando la compensación sea explícita y valga la pena para el producto.

## Público-Experimental

Trate `public-experimental` como abandono de participación fuera de la expectativa de compatibilidad normal.

Puede ser útil para el aprendizaje o la adopción temprana, pero no debería convertirse silenciosamente en el valor predeterminado del equipo.

## Regla práctica

1. Prefiera la ruta recomendada para el producto que está creando.
2. Utilice los términos formales exactos sólo cuando necesite precisión en la política o la compatibilidad.
3. Utilice `validate --strict` como puerta de preparación para el repositorio que planea enviar.
</details>
