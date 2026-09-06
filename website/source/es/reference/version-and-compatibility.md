---
title: "Política de versión y compatibilidad"
description: "Cómo pensar en lanzamientos, promesas de compatibilidad, contenedores, SDK y vocabulario de soporte en plugin-kit-ai."
canonicalId: "page:reference:version-and-compatibility"
section: "reference"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Política de versión y compatibilidad

Esta página es para una decisión práctica del equipo: ¿qué estamos estandarizando y qué tan sólida es esa promesa?

## Elige en 60 segundos

- lea esta página cuando su equipo necesite una política compacta para lanzamientos, contenedores, SDKs, tiempos de ejecución y promesas de compatibilidad.
- lea [Límite de soporte](/es/reference/support-boundary) cuando desee la respuesta de soporte práctica más corta
- lea [Lanzamientos](/es/releases/) cuando desee la historia de un lanzamiento específico

## La línea de base pública

Piense en la estandarización en tres capas:

- la línea de lanzamiento que elijas en los repositorios
- el nivel de soporte de la ruta que elijas dentro de esa línea de lanzamiento
- el mecanismo de instalación o entrega alrededor de esa ruta

Estas capas están relacionadas, pero no son intercambiables.

## Carriles recomendados y niveles formales

Utilice una traducción simple entre documentos y políticas:

- `Recommended` generalmente significa una ruta de producción promocionada `public-stable`
- `Advanced` significa una superficie de soporte con un contrato más estrecho o más especializado
- `Experimental` significa abandono de participación fuera de la expectativa de compatibilidad normal.

Los principales caminos recomendados hoy en día son:

- `Codex runtime Go`
- `Codex package`
- `Gemini packaging`
- `Gemini Go runtime`
- `Claude default stable lane`
- Rutas de ejecución locales `Python` y `Node` como opción de creación compatible y recomendada no Go en destinos compatibles

## Qué cubre realmente la compatibilidad aquí

La promesa pública más fuerte gira en torno a:

- el contrato público declarado CLI
- la ruta recomendada Go SDK y las rutas de producción recomendadas enumeradas anteriormente
- las rutas de tiempo de ejecución locales recomendadas Python y Node en objetivos compatibles
- el comportamiento documentado de las salidas generadas `public-stable`

La compatibilidad no significa que todos los envoltorios, rutas de conveniencia o superficies especializadas se muevan con la misma promesa.

## Lenguaje público versus términos formales

Utilice esta traducción cuando hable con un equipo:

- `Recommended` generalmente significa que la ruta está dentro del contrato `public-stable` actual más fuerte
- `Advanced` significa que la superficie es compatible, pero más especializada o más estrecha que la primera opción predeterminada
- `Experimental` significa abandono voluntario sin expectativa de compatibilidad normal

Cuando el equipo necesite una política exacta, utilice los términos formales `public-stable`, `public-beta` y `public-experimental`.

## Envoltorios, SDKs y tiempo de ejecución APIs

No los estandarice como si fueran lo mismo.

- Homebrew, npm, PyPI y el script verificado son canales de instalación para CLI
- la Go SDK es una superficie pública SDK
- Los API de tiempo de ejecución están vinculados a sus rutas de tiempo de ejecución declaradas.

Si trata los contenedores de instalación como si tuvieran la misma promesa que un SDK o una ruta de tiempo de ejecución, estandarizará la capa incorrecta.

## Qué deberían estandarizar los equipos

Los equipos sanos suelen estandarizar:

- una línea base de lanzamiento declarada
- un camino principal con una historia de apoyo clara
- una puerta de validación antes de la transferencia y el lanzamiento
- una interpretación compartida de los términos formales de compatibilidad

## Regla final

Estandarice solo la línea de lanzamiento y la ruta cuya promesa pública su equipo esté realmente dispuesto a defender en CI, transferencia e implementación.
</details>
