---
title: "Cree un complemento de tiempo de ejecución Node/TypeScript"
description: "La ruta principal compatible que no es Go para complementos de tiempo de ejecución local."
canonicalId: "page:guide:node-typescript-runtime"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Cree un complemento de tiempo de ejecución Node/TypeScript

Esta es la ruta principal admitida que no es Go cuando su equipo quiere TypeScript pero aún necesita un complemento de tiempo de ejecución local compatible.

## Flujo recomendado

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## Qué recordar

- esta es una ruta estable de tiempo de ejecución local, no la ruta Go de dependencia de tiempo de ejecución cero
- la máquina de ejecución aún necesita Node.js `20+`
- `doctor` y `bootstrap` importan más aquí que en la ruta predeterminada Go

## Cuando esta es la elección correcta

- tu equipo ya trabaja en TypeScript
- el complemento permanece local en el repositorio por diseño
- desea la ruta principal admitida que no sea Go sin caer en una trampilla de escape beta

## Cuando Go sigue siendo mejor

Prefiere Go en su lugar cuando:

- quieres el contrato de producción más sólido
- desea que los usuarios intermedios eviten instalar Node
- desea la menor fricción de arranque en CI y en otras máquinas

Consulte [Elegir tiempo de ejecución](/es/concepts/choosing-runtime) y [Node Tiempo de ejecución API](/es/api/runtime-node/) para conocer la siguiente capa de detalles.

</details>
