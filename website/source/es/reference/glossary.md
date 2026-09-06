---
title: "Glosario"
description: "Definiciones breves de los términos públicos utilizados en los documentos plugin-kit-ai."
canonicalId: "page:reference:glossary"
section: "reference"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">Glosario</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# Glosario

Utilice esta página cuando un término de documentos le resulte lento. El objetivo no es una teoría perfecta. El objetivo es un significado compartido rápidamente.

## Estado de autor

La parte del repositorio que su equipo posee directamente. `generate` convierte esta fuente en una salida específica de destino.

## Archivos de destino generados

Archivos producidos para un objetivo específico después de su generación. Son resultados reales, pero no son la fuente de la verdad a largo plazo.

## Camino

Una forma práctica de crear y enviar el complemento. Los ejemplos incluyen la ruta de tiempo de ejecución predeterminada Go, la ruta local Node/TypeScript y la configuración de integración propiedad del repositorio.

## Objetivo

La salida a la que apunta, como `codex-runtime`, `claude`, `codex-package`, `gemini`, `opencode` o `cursor`.

## Ruta de ejecución

Una ruta donde el repositorio posee directamente el comportamiento del complemento ejecutable.

## Paquete o ruta de extensión

Una ruta centrada en producir el paquete o artefacto de extensión correcto en lugar de la forma de tiempo de ejecución ejecutable principal.

## Configuración de integración propiedad del repositorio

Una ruta donde el repositorio envía principalmente la configuración registrada para otra herramienta o espacio de trabajo.

## Instalar canal

Una forma de instalar CLI, como Homebrew, npm, PyPI o el script verificado. No es un tiempo de ejecución público API.

## Paquete de tiempo de ejecución compartido

La dependencia `plugin-kit-ai-runtime` utilizada por los flujos Python y Node aprobados en lugar de copiar archivos auxiliares en cada repositorio.

## Límite de soporte

La línea pública entre lo que el proyecto recomienda de forma predeterminada, lo que respalda con más cuidado y lo que sigue siendo experimental.

## Puerta de preparación

El cheque que debe tratar como la señal de que un repositorio está lo suficientemente sano como para transferirlo. Para la mayoría de los repositorios, esto es `validate --strict`.

## Traspaso

El punto en el que otro compañero de equipo, otra máquina u otro usuario puede usar el repositorio sin conocimientos de configuración ocultos.

Páginas relacionadas: [Modelo de destino](/es/concepts/target-model), [Límite de soporte](/es/reference/support-boundary) y [Preparación para la producción](/es/guide/production-readiness).
</details>
