---
title: "Documentación de plugin-kit-ai"
description: "Documentación pública para plugin-kit-ai."
canonicalId: "page:home"
section: "home"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">Documentación de plugin-kit-ai</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

<div class="docs-hero docs-hero--feature">
  <p class="docs-kicker">DOCUMENTACIÓN PÚBLICA</p>
  <h1>plugin-kit-ai</h1>
  <p class="docs-lead">
    Desarrolla en un solo repositorio, empieza con Go por defecto y añade más tarde packages,
    hooks de Claude, Gemini o configuración de integración gestionada por el repositorio sin dividir el proyecto.
  </p>
</div>

## Inicio predeterminado

- `Codex runtime Go` es el inicio predeterminado cuando desea el tiempo de ejecución y la historia de lanzamiento más sólidos.

## Qué saber de inmediato

- un repositorio sigue siendo la fuente de la verdad a medida que agregas más carriles
- elige el camino inicial que coincida con lo que necesitas hoy
- expandir más tarde desde el mismo repositorio cuando el producto necesite más resultados
- utilice `generate` y `validate --strict` como flujo de trabajo de preparación compartido

## Rutas Node y Python admitidas

- `codex-runtime --runtime node --typescript` es la principal ruta admitida que no es Go.
- `codex-runtime --runtime python` es la primera ruta admitida Python.
- Ambas son rutas de ejecución interpretadas localmente, por lo que la máquina de destino aún necesita Node.js `20+` o Python `3.10+`.
- Son opciones tempranas claras para los equipos que ya trabajan en esas pilas, pero no son el inicio predeterminado.

<div class="docs-grid">
  <a class="docs-card" href="./guide/quickstart">
    <h2>Inicio rápido</h2>
    <p>Utilice primero la ruta predeterminada más segura y luego amplíela solo cuando el producto necesite más resultados.</p>
  </a>
  <a class="docs-card" href="./guide/what-you-can-build">
    <h2>Ver la forma del producto</h2>
    <p>Vea cómo un repositorio crece hasta convertirse en tiempo de ejecución, paquete, extensión y configuración de integración propiedad del repositorio.</p>
  </a>
  <a class="docs-card" href="./guide/choose-a-target">
    <h2>Elija un objetivo</h2>
    <p>Haga coincidir el objetivo con la forma en que desea enviar el complemento en lugar de tratar cada salida como si fuera la misma cosa.</p>
  </a>
  <a class="docs-card" href="./reference/support-boundary">
    <h2>Verifique el contrato exacto</h2>
    <p>Utilice las páginas de referencia cuando necesite los límites de soporte precisos y los términos de compatibilidad.</p>
  </a>
</div>

## Si más adelante necesitas más

- Agregue `Claude default lane` cuando los ganchos Claude sean el requisito del producto.
- Agregue `Codex package` o `Gemini packaging` cuando el producto sea un paquete o una extensión de salida.
- Agregue `OpenCode` o `Cursor` cuando el repositorio deba poseer la configuración de integración.
- Utilice `validate --strict` como puerta de preparación antes de la transferencia o CI.

## Rutas de expansión comunes

- Comience con un repositorio de tiempo de ejecución Codex, luego agregue el paquete Codex o Gemini cuando el paquete pase a formar parte del producto.
- Comience con Claude cuando los ganchos Claude sean el producto, luego mantenga el repositorio abierto para rutas de entrega más amplias más adelante.
- Comience en Node o Python localmente y luego agregue la transferencia del paquete cuando la entrega posterior sea importante.
- Agregue OpenCode o Cursor cuando el repositorio deba administrar la configuración de integración, no solo el comportamiento ejecutable.

## Leer en este orden

<div class="docs-grid">
  <a class="docs-card" href="./guide/quickstart">
    <h2>1. Inicio rápido</h2>
    <p>Comience con una ruta recomendada antes de pensar en la expansión.</p>
  </a>
  <a class="docs-card" href="./guide/what-you-can-build">
    <h2>2. Lo que puedes construir</h2>
    <p>Vea la forma del producto en las líneas de tiempo de ejecución, paquete, extensión e integración.</p>
  </a>
  <a class="docs-card" href="./guide/choose-a-target">
    <h2>3. Elija un objetivo</h2>
    <p>Elija el destino que coincida con la forma en que realmente desea enviar el complemento.</p>
  </a>
  <a class="docs-card" href="./reference/support-boundary">
    <h2>4. Límite de soporte</h2>
    <p>Utilice el clúster de referencia cuando necesite un idioma de compatibilidad exacto y detalles de soporte.</p>
  </a>
</div>

Si es nuevo, puede detenerse después de las páginas iniciales. Todo lo demás es una referencia más profunda o detalles de implementación.

## Línea base del repositorio actual

- La línea de base pública actual en este conjunto de documentos es [`v1.1.2`](/es/releases/v1-1-2).
- Esta línea de parches restauró la compatibilidad de instalación de first-party aliases entre el layout heredado y el actual, y luego corrigió las instalaciones Gemini de varios targets desde fuentes GitHub repo-path.
- Comience allí cuando desee obtener la línea base recomendada actual.

## Qué le ayuda a hacer este sitio

- iniciar un repositorio de complementos en lugar de dividir la fuente de verdad por ecosistema
- Elija una ruta de inicio recomendada sin conocer todos los detalles del objetivo por adelantado
- expandir el mismo repositorio más adelante a más rutas de envío
- mantenga una historia de revisión y validación a medida que crece el repositorio
- encuentre el contrato exacto sólo cuando lo necesite

</details>
