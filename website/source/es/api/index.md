---
title: "API"
description: "Referencia API generada para plugin-kit-ai."
canonicalId: "page:api:index"
section: "api"
locale: "es"
generated: false
translationRequired: true
aside: false
outline: false
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

<div class="docs-hero docs-hero--compact">
  <p class="docs-kicker">REFERENCIA GENERADA</p>
  <h1>Superficies de API</h1>
  <p class="docs-lead">
    Esta sección reúne las API públicas de plugin-kit-ai: CLI, Go SDK, helpers de runtime, eventos de plataforma y capacidades.
  </p>
</div>

<div class="docs-grid">
  <a class="docs-card" href="./cli/">
    <h2>CLI</h2>
    <p>Comandos exportados desde el árbol Cobra en vivo.</p>
  </a>
  <a class="docs-card" href="./go-sdk/">
    <h2>Go SDK</h2>
    <p>Paquetes Go públicos para complementos de runtime listos para producción.</p>
  </a>
  <a class="docs-card" href="./runtime-node/">
    <h2>Runtime Node</h2>
    <p>Helpers de runtime tipados para consumidores de JS y TS.</p>
  </a>
  <a class="docs-card" href="./runtime-python/">
    <h2>Runtime Python</h2>
    <p>Solo helpers públicos de runtime para Python, no wrappers de instalación.</p>
  </a>
  <a class="docs-card" href="./platform-events/">
    <h2>Eventos de plataforma</h2>
    <p>Superficies de eventos agrupadas por plataforma de destino.</p>
  </a>
  <a class="docs-card" href="./capabilities/">
    <h2>Capacidades</h2>
    <p>Capacidades agrupadas entre plataformas y eventos.</p>
  </a>
</div>

## Abra la superficie correcta

- Abra `CLI` cuando necesite comandos, indicadores o el flujo de trabajo de creación.
- Abra `Go SDK` cuando esté creando un complemento de runtime listo para producción en Go.
- Abra `Runtime Node` o `Runtime Python` cuando necesite la API compartida de helpers para un runtime local al repositorio.
- Abra `Platform Events` cuando elija eventos de objetivos específicos.
- Abra `Capabilities` cuando desee ver qué acciones y puntos de extensión existen en todas las plataformas.

## Qué cubre esta sección API

- el árbol de comandos Cobra en vivo
- paquetes públicos Go
- ayudantes de tiempo de ejecución compartidos para Node y Python
- eventos específicos de la plataforma
- metadatos multiplataforma a nivel de capacidad

</details>
