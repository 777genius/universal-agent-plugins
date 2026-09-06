---
title: "Elija el modelo de entrega"
description: "Cómo elegir entre los asistentes proporcionados y el paquete de tiempo de ejecución compartido para los complementos Python y Node."
canonicalId: "page:guide:choose-delivery-model"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">Elija el modelo de entrega</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# Elija el modelo de entrega

Los complementos Python y Node tienen dos formas compatibles de enviar lógica auxiliar. Resuelven diferentes problemas prácticos.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Python or Node plugin] --> Compartido{Necesita una dependencia reutilizable en todos los repositorios}
  Compartido -->|Sí| Paquete [paquete de ejecución compartido]
  Compartido -->|No| Suave {Necesita el inicio autónomo más fluido}
  Suave -->|Sí| Vendido[ayudante vendido]
  Suave -->|No| Paquete
`"
/>

## Regla práctica rápida

Si solo desea el repositorio Python o Node que funcione más simple hoy, use primero el andamio predeterminado.

Si ya sabe que varios repositorios deben compartir una dependencia auxiliar, comience con `--runtime-package`.

## Los dos modos

- `vendored helper`: el andamio predeterminado escribe archivos auxiliares en el propio repositorio
- `shared runtime package`: `--runtime-package` importa `plugin-kit-ai-runtime` como una dependencia en lugar de escribir el ayudante en `plugin/`

## El mismo proyecto en ambos modos

Ruta de ayuda local predeterminada:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
```

Ruta del paquete compartido:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

## Elija ayudante suministrado cuando

- quieres el camino de primera ejecución más fluido
- quieres que el repositorio permanezca autónomo
- desea que la implementación auxiliar esté visible en el repositorio
- su equipo aún no está estandarizado en una versión compartida de PyPI o npm helper

Este es el valor predeterminado porque es el punto de partida más sencillo para los proyectos Python y Node.

## Elija el paquete de tiempo de ejecución compartido cuando

- desea una dependencia auxiliar reutilizable en múltiples repositorios de complementos
- prefieres actualizar el comportamiento del asistente a través de cambios normales en la versión del paquete
- Su equipo se siente cómodo fijando versiones en `requirements.txt` o `package.json`
- ya sabes que el repositorio debe seguir la ruta de dependencia compartida desde el primer día

## Lo que la gente suele decir en la práctica

- elija el asistente proporcionado cuando el objetivo principal sea "hacer que un repositorio funcione rápidamente"
- elija el paquete de tiempo de ejecución compartido cuando el objetivo principal sea "reutilizar el mismo paquete auxiliar en todos los repositorios"
- no elijas el paquete compartido sólo porque suena más a producción; no elimina el requisito de tiempo de ejecución Python o Node de la máquina de ejecución

## Lo que no cambia

- Go sigue siendo el valor predeterminado recomendado cuando desea la ruta de producción más sólida
- Python todavía requiere Python `3.10+` en la máquina de ejecución
- Node todavía requiere Node.js `20+` en la máquina de ejecución
- `validate --strict` sigue siendo la principal verificación de preparación.
- Los paquetes de instalación CLI aún no se convierten en APIs de tiempo de ejecución

## Política de equipo recomendada

- elija Go cuando desee la ruta más sólida con soporte a largo plazo
- elija ayudantes proporcionados cuando desee el inicio más fluido Python o Node
- elija el paquete de tiempo de ejecución compartido cuando ya sepa que desea una estrategia de dependencia reutilizable en todos los repositorios

Empareje esta página con [Crear un complemento de tiempo de ejecución Python](/es/guide/python-runtime), [Elegir un repositorio inicial](/es/guide/choose-a-starter), [Plantillas iniciales](/es/guide/starter-templates) y [Preparación para la producción](/es/guide/production-readiness).
</details>
