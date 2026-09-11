---
title: "Integración de CI"
description: "Convierta el flujo de creación pública en una puerta de CI estable para proyectos plugin-kit-ai."
canonicalId: "page:guide:ci-integration"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">Integración de CI</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# Integración de CI

La historia de CI más segura no es complicada. Es simplemente estricto con el contrato público.

<MermaidDiagram
  :chart="`
flowchart LR
  Doctor[doctor] --> Bootstrap[arranque cuando sea necesario]
  Bootstrap --> Generar[generar]
  Generar --> Validar[validar --strict]
  Validar --> Fumar[cheques de humo o paquete]
`"
/>

## La puerta CI mínima

Para la mayoría de los proyectos escritos, esta es la línea de base:

```bash
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform <target> --strict
```

Si su carril tiene pruebas de humo estable o controles de paquetes, agréguelos después de la puerta de validación en lugar de reemplazarlos.

## Por qué funciona esto

- `doctor` detecta anticipadamente los requisitos previos de tiempo de ejecución que faltan
- `generate` demuestra que los resultados generados se pueden reproducir desde el estado de autor
- `validate --strict` demuestra que el repositorio es internamente coherente para el objetivo elegido
- para un repositorio de múltiples objetivos, la misma lógica debe aplicarse para cada objetivo en el alcance de soporte

## Notas específicas del tiempo de ejecución

### Go

Go es la ruta de CI más limpia porque la máquina de ejecución no necesita Python o Node solo para satisfacer el carril de tiempo de ejecución.

Para repositorios Go basados en launcher, construya primero el launcher verificado:

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Node/TypeScript

Agregue bootstrap explícitamente:

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Python

Utilice el mismo patrón que Node y haga explícita la versión Python en CI.

## Errores comunes de CI

- ejecutando `validate --strict` sin `generate`
- tratar los artefactos generados como archivos mantenidos manualmente
- olvidar los requisitos previos de tiempo de ejecución para los carriles Node o Python
- compatibilidad prometedora para un objetivo que está fuera del límite de soporte estable

## Regla recomendada

Si CI no puede reproducir los resultados creados y pasar `validate --strict`, el repositorio no está listo para una transferencia estable. Para un repositorio de múltiples objetivos, eso significa una ejecución verde explícita para cada objetivo dentro del alcance del soporte.

Empareje esta página con [Preparación para la producción](/es/guide/production-readiness), [Límite de soporte](/es/reference/support-boundary) y [Solución de problemas](/es/reference/troubleshooting).

</details>
