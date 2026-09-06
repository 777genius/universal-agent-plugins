---
title: "Elegir el tiempo de ejecución"
description: "Cómo elegir entre Go, Python, Node y rutas de creación de shell."
canonicalId: "page:concepts:choosing-runtime"
section: "concepts"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Elegir el tiempo de ejecución

La elección del tiempo de ejecución no se trata solo de la preferencia de idioma. Cambia la forma en que se ejecuta el complemento, qué debe tener instalada la máquina de ejecución y qué tan simples serán la CI y la transferencia.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Need a runtime lane] --> Prod{Necesita el carril de ejecución más potente}
  Prod -->|Sí| Go[ir]
  Producto -->|No| Local{¿El repositorio de complementos es local por diseño?}
  Local -->|Sí| Equipo{¿Es el equipo Python primero o Node primero?}
  Equipo --> Python[python]
  Equipo --> Node[nodo o nodo --mecanografiado]
  Local -->|No| Escape{Solo necesita una trampilla de escape}
  Escape --> Cáscara[cáscara]
`"
/>

## Elija Go Cuándo

- quieres el carril de ejecución más fuerte
- quieres controladores escritos y la historia de lanzamiento más limpia
- desea la menor fricción de arranque en CI y en otras máquinas

## Elija Python o Node cuando

- el complemento es repositorio local por diseño
- tu equipo ya vive en ese tiempo de ejecución
- Aceptas ser propietario del runtime bootstrap tú mismo
- se siente cómodo con Python `3.10+` o Node.js `20+` presentes en la máquina de ejecución

## Elija Shell solo cuando

- necesitas una trampilla de escape estrecha
- acepta explícitamente una compensación experimental o avanzada

## Matriz predeterminada segura

| Situación | Elección recomendada |
| --- | --- |
| Carril de ejecución más fuerte | `go` |
| Carril de tiempo de ejecución principal no Go | `node --typescript` |
| Local Python-primer equipo | `python` |
| Trampilla de evacuación | `shell` |
</details>
