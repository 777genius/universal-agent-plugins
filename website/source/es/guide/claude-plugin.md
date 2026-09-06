---
title: "Cree un complemento Claude"
description: "Una guía centrada en la ruta estable del complemento Claude en plugin-kit-ai."
canonicalId: "page:guide:claude-plugin"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">Cree un complemento Claude</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# Cree un complemento Claude

Elija esta ruta cuando esté apuntando explícitamente a ganchos Claude en lugar de la ruta de tiempo de ejecución predeterminada Codex.

## Punto de partida recomendado

```bash
plugin-kit-ai init my-claude-plugin --platform claude
cd my-claude-plugin
plugin-kit-ai generate .
plugin-kit-ai validate . --platform claude --strict
```

## Qué significa este camino

- el proyecto apunta a la ejecución del gancho Claude
- el subconjunto estable es más limitado que el conjunto completo de funciones de tiempo de ejecución Claude
- `validate --strict` sigue siendo la principal verificación de preparación.

## Utilice los ganchos extendidos con cuidado

```bash
plugin-kit-ai init my-claude-plugin --platform claude --claude-extended-hooks
```

Solo elija ganchos extendidos cuando intencionalmente desee el conjunto con soporte más amplio y acepte una estabilidad más flexible que el subconjunto estable.

## Cuándo encaja esta ruta

- un complemento que debe integrarse con los ganchos de tiempo de ejecución Claude
- equipos que quieren un repositorio y un flujo de trabajo en lugar de editar manualmente los artefactos nativos Claude
- usuarios que necesitan una estructura más sólida que los scripts locales ad-hoc

## Próximos pasos

- Lea [Modelo de destino](/es/concepts/target-model) para ver en qué se diferencia Claude de los objetivos de empaquetado o de configuración del espacio de trabajo.
- Consulte [Eventos de plataforma](/es/api/platform-events/claude) para obtener una referencia a nivel de evento.

</details>
