---
title: "Elija un repositorio inicial"
description: "Una matriz práctica para elegir el iniciador oficial adecuado por objetivo, tiempo de ejecución y ruta de entrega."
canonicalId: "page:guide:choose-a-starter"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Elija un repositorio inicial

Utilice esta página cuando desee la ruta más rápida a un repositorio que luego pueda expandirse a salidas más compatibles.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Need a starter] --> Producto{La ruta principal es Codex o Claude}
  Producto --> Codex[Codex familia inicial]
  Producto --> Claude[Claude familia inicial]
  Codex --> Tiempo de ejecución{Go, Node o Python}
  Claude --> Tiempo de ejecución2{Go, Node o Python}
`"
/>

Antes de elegir, recuerda una regla importante:

- el motor de arranque te dice cómo empezar
- no es el límite final del producto
- y no impide que un repositorio admita más objetivos en el futuro

Si esa distinción aún es confusa, lea primero [Un proyecto, múltiples objetivos](/es/guide/one-project-multiple-targets).

## Elija rápido y luego expanda más tarde

- elija Go cuando desee la ruta de producción más sólida
- elija Node/TypeScript cuando desee la ruta principal admitida que no sea Go
- elija Python cuando el repositorio sea intencionalmente Python-primero y permanezca local en el repositorio
- elija los iniciadores Claude solo cuando los ganchos Claude sean el requisito real del producto

Elija el punto de partida para el primer camino correcto, no para un límite de producto permanente imaginado.

## Lo que sigue siendo cierto después de elegir

- Aún conservas un repositorio.
- Sigues manteniendo el mismo flujo de trabajo principal.
- Puede agregar objetivos admitidos más adelante a medida que el producto crezca.
- La profundidad del soporte depende del objetivo que agregues.

## Matriz inicial

| Si quieres | Mejor entrante | Por qué |
| --- | --- | --- |
| Ruta de producción más fuerte Codex | `plugin-kit-ai-starter-codex-go` | Go-primera ruta de producción con la historia de transferencia más limpia |
| Complemento repositorio local Codex en Python | `plugin-kit-ai-starter-codex-python` | Subconjunto estable Python con un diseño de repositorio en buen estado |
| Complemento repositorio local Codex en Node/TS | `plugin-kit-ai-starter-codex-node-typescript` | Ruta principal admitida que no es Go |
| Ruta de producción más fuerte Claude | `plugin-kit-ai-starter-claude-go` | Subconjunto estable Claude más la ruta de producción más limpia |
| Complemento repositorio local Claude en Python | `plugin-kit-ai-starter-claude-python` | Subconjunto de ganchos Claude estable con ayudantes Python |
| Complemento repositorio local Claude en Node/TS | `plugin-kit-ai-starter-claude-node-typescript` | Subconjunto de ganchos Claude estable para TypeScript-primeros equipos |

## Variantes de paquetes compartidos

Ignore esta sección a menos que ya sepa que su equipo quiere `plugin-kit-ai-runtime` como una dependencia reutilizable en lugar de archivos auxiliares proporcionados.

Utilice las variantes del paquete compartido cuando:

- quieres una dependencia compartida entre múltiples repositorios de complementos
- se siente cómodo fijando y actualizando el paquete de tiempo de ejecución explícitamente
- no desea que los archivos auxiliares se copien en cada repositorio

Iniciales actuales de paquetes compartidos:

- [`plugin-kit-ai-starter-codex-python-runtime-package`](https://github.com/777genius/plugin-kit-ai-starter-codex-python-runtime-package): Python Codex arrancador con `plugin-kit-ai-runtime` fijado en `requirements.txt`
- [`plugin-kit-ai-starter-claude-node-typescript-runtime-package`](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript-runtime-package): Node/TypeScript Claude arrancador con `plugin-kit-ai-runtime` fijado en `package.json`

Si elige entre el iniciador Python normal y el iniciador del paquete de tiempo de ejecución Python, lea primero [Crear un complemento de tiempo de ejecución Python](/es/guide/python-runtime) y luego [Elegir modelo de entrega](/es/guide/choose-delivery-model).

## Cuándo evitar optimizar demasiado la elección

No pierdas demasiado tiempo buscando el entrante perfecto.

Si no está seguro:

1. comience con el iniciador Go para obtener el valor predeterminado más fuerte
2. comience con el iniciador Node/TypeScript para la ruta principal admitida que no es Go
3. solo elija Python o variantes de paquete compartido cuando la compensación del equipo ya sea real

## Buena política de equipo

La elección de un titular para todo el equipo debe ser consistente el tiempo suficiente para que:

- todos reconocen el diseño del repositorio
- CI utiliza el mismo flujo de preparación
- el traspaso no depende de la explicación del mantenedor

Pero una elección inicial estable aún no impide que un repositorio agregue otros objetivos más adelante si el producto los requiere.

Empareje esta página con [Plantillas de inicio](/es/guide/starter-templates), [Elegir modelo de entrega](/es/guide/choose-delivery-model) y [Estándar de repositorio](/es/reference/repository-standard).

</details>
