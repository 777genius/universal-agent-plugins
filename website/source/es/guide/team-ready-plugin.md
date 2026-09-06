---
title: "Cree un complemento listo para el equipo"
description: "Un tutorial público emblemático para llevar un complemento desde el andamio hasta una forma lista para CI, lista para transferencia y legible por el equipo."
canonicalId: "page:guide:team-ready-plugin"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">Cree un complemento listo para el equipo</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# Cree un complemento listo para el equipo

Este tutorial continúa donde termina el primer complemento exitoso. El objetivo no es simplemente "funciona en mi máquina", sino un repositorio que otro compañero de equipo pueda clonar, validar y enviar sin conocimientos ocultos.

<MermaidDiagram
  :chart="`
flowchart LR
  scaffold[Repositorio base] --> explicit[Camino y alcance explícitos]
  explicit --> honest[Archivos generados mantenidos honestos]
  honest --> ci[Puerta de CI repetible]
  ci --> handoff[Traspaso visible para el equipo]
  handoff --> ready[Repositorio listo para el equipo]
`"
/>

## Resultado

Al final, deberías tener:

- un repositorio creado según el estándar del paquete
- archivos generados reproducidos desde la fuente del proyecto
- un estricto control de validación que pasa limpiamente
- un objetivo principal claro u objetivos dentro del alcance documentados para los compañeros de equipo
- una elección clara de tiempo de ejecución o política de tiempo de ejecución por objetivo
- una ruta compatible con CI que se puede repetir en otra máquina

## 1. Comience desde el camino estable más estrecho

Comience con el camino más estrecho que realmente coincida con el trabajo:

```bash
plugin-kit-ai init my-plugin --template custom-logic
cd my-plugin
plugin-kit-ai inspect . --authoring
go mod tidy
plugin-kit-ai validate . --platform codex-runtime --strict
```

Esto le brinda la base más limpia para una transferencia posterior.

## 2. Haga la elección explícita

Un repositorio listo para el equipo debería decir, como mínimo:

- qué objetivo es principal y qué objetivos adicionales están realmente respaldados
- qué tiempo de ejecución utiliza y si cambia según el objetivo
- cuál es el comando de validación principal o qué comandos de validación se requieren para un repositorio de múltiples objetivos
- si depende de una ruta Go SDK o de un paquete de tiempo de ejecución compartido

Si esa información solo está en la cabeza de un mantenedor, el repositorio no está listo.

## 3. Mantenga el repositorio honesto

Antes de expandir el proyecto, aplique tres reglas:

- la fuente del proyecto se encuentra en el diseño estándar del paquete
- los archivos de destino generados son salidas
- `generate` y `validate --strict` siguen siendo parte del flujo de trabajo normal

No parchee los archivos generados manualmente y luego espere que el equipo nunca vuelva a ejecutar la generación.

## 4. Agregue una puerta CI repetible

Para la ruta Go launcher predeterminada, la puerta mínima debería verse así:

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

Si la ruta elegida es Node o Python, incluya `bootstrap` y fije la versión del tiempo de ejecución en CI:

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

Si el repositorio admite múltiples objetivos, la puerta de CI debe verificar explícitamente cada objetivo admitido en lugar de asumir una cobertura indirecta.

## 5. Compruebe si realmente necesita un camino diferente

Sólo aléjese de la ruta predeterminada cuando la compensación sea real:

- use `claude` cuando los ganchos Claude sean el requisito del producto
- use `node --typescript` cuando el equipo sea TypeScript-primero y la compensación del tiempo de ejecución local sea aceptable
- use `python` cuando el proyecto sea intencionalmente local para el repositorio y Python-first

Cambiar de carril debería resolver un problema de producto o equipo, no solo reflejar la preferencia de idioma. Si el producto es realmente para múltiples objetivos, dígalo directamente: el repositorio tiene una ruta principal y objetivos adicionales dentro del alcance admitido.

## 6. Hacer visible la transferencia

Un nuevo compañero de equipo debería poder responder estas preguntas del repositorio y los documentos:

- ¿Cómo instalo los requisitos previos?
- ¿Qué comando prueba que el repositorio está en buen estado?
- ¿Para qué objetivo estoy validando?
- ¿Qué archivos están en estado de creación y cuáles se generan?

Si la respuesta a cualquiera de ellas es "pregúntele al autor original", el repositorio aún no está listo.

## 7. Vincular el repositorio al contrato público

Un repositorio de complementos listo para el equipo debería indicar a las personas:

- [Preparación para la producción](/es/guide/production-readiness)
- [Integración de CI](/es/guide/ci-integration)
- [Estándar de repositorio](/es/reference/repository-standard)
- la nota de publicación pública actual, ahora [v1.1.2](/es/releases/v1-1-2)

## Regla final

El repositorio está listo cuando otro compañero de equipo puede clonarlo, comprender la ruta y el alcance objetivo, reproducir los resultados generados y pasar la estricta puerta de validación sin improvisación.

Combine este tutorial con [Cree su primer complemento](/es/guide/first-plugin), [Arquitectura de creación](/es/concepts/authoring-architecture) y [Límite de soporte](/es/reference/support-boundary).

</details>
