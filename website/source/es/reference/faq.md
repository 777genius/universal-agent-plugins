---
title: "FAQ"
description: "Respuestas breves a las preguntas que los equipos hacen con más frecuencia al iniciar y escalar repositorios plugin-kit-ai."
canonicalId: "page:reference:faq"
section: "reference"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">FAQ</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# FAQ

## ¿Debería empezar con Go, Python o Node?

Comience con Go a menos que tenga una razón real para no hacerlo.

Elija Node/TypeScript como la ruta principal admitida que no es Go. Elija Python cuando el complemento permanezca local en el repositorio y su equipo ya sea Python-primero.

## ¿Cuál es la configuración Python más sencilla?

Utilice primero el andamio predeterminado Python:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

Luego edite el complemento, regenere y valide nuevamente.

Consulte [Creación de un complemento de tiempo de ejecución Python](/es/guide/python-runtime).

## ¿Cuándo debo usar `--runtime-package`?

Utilice `--runtime-package` solo cuando desee intencionalmente una dependencia auxiliar compartida en varios repositorios.

La mayoría de los equipos deberían comenzar primero con el ayudante local predeterminado.

## ¿Npm y PyPI son `plugin-kit-ai` paquetes de tiempo de ejecución APIs?

No. Instalaron el CLI. No son APIs de tiempo de ejecución ni SDKs.

## ¿Cuándo debo utilizar los comandos de paquete?

Utilice comandos de paquete cuando otra máquina necesite artefactos portátiles Python o Node para recuperarlos o instalarlos.

No confunda la entrega del paquete con la ruta de instalación principal CLI.

## ¿Puedo conservar los archivos de destino nativos como mi fuente de información?

No. El modelo previsto a largo plazo es mantener la fuente de verdad en el diseño estándar del paquete y tratar los archivos de destino como resultados generados.

## ¿Es `generate` opcional?

No, no si desea el flujo del proyecto administrado. `generate` es parte del flujo de trabajo.

## ¿Es `validate --strict` opcional?

Trátelo como la verificación de preparación principal, especialmente para los repositorios de tiempo de ejecución locales Python y Node.

## ¿Puede un repositorio poseer varios objetivos?

Sí.

La regla práctica es:

- mantener el estado de autor en un repositorio administrado
- comience con el objetivo principal que necesita hoy
- agregue más objetivos solo cuando aparezca una necesidad real de producto, entrega o integración

Consulte [Un proyecto, múltiples objetivos](/es/guide/one-project-multiple-targets) y [Modelo de destino](/es/concepts/target-model).

## ¿Son todos los objetivos igualmente estables?

No.

Diferentes caminos conllevan diferentes promesas de apoyo. Utilice [Límite de soporte](/es/reference/support-boundary) para la respuesta corta y [Soporte objetivo](/es/reference/target-support) para la matriz exacta.
</details>
