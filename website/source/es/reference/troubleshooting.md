---
title: "Solución de problemas"
description: "Pasos de recuperación rápidos para los problemas más comunes de instalación, generación, validación y arranque."
canonicalId: "page:reference:troubleshooting"
section: "reference"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">Solución de problemas</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# Solución de problemas

Utilice esta página cuando el flujo de trabajo deje de moverse. Comience primero con el control más simple.

## El CLI se instala pero no se ejecuta

Verifique que el binario esté realmente en su shell `PATH`.

Si instaló a través de npm o PyPI, asegúrese de que el paquete realmente haya descargado el binario publicado. No trate el paquete contenedor en sí como el tiempo de ejecución.

## Python o Node Los proyectos en tiempo de ejecución fallan antes de tiempo

Primero verifique el tiempo de ejecución real:

- Python los repositorios en tiempo de ejecución requieren Python `3.10+`
- Node los repositorios en tiempo de ejecución requieren Node.js `20+`

Utilice `plugin-kit-ai doctor <path>` antes de asumir que el repositorio no funciona.

Flujo de recuperación típico:

```bash
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## `validate --strict` Falla

Trate esto como una señal, no como un ruido.

Causas comunes:

- los artefactos generados están obsoletos porque se omitió `generate`
- la plataforma seleccionada no coincide con la fuente del proyecto
- la ruta del tiempo de ejecución todavía necesita arreglos de arranque o del entorno

## `generate` La salida parece diferente de lo esperado

Por lo general, eso significa que la fuente del proyecto y su modelo mental se separaron.

Vuelva a verificar el diseño estándar del paquete en lugar de editar manualmente los archivos de destino generados para forzar el resultado esperado.

## No estoy seguro de qué ruta debo utilizar

Comience con la ruta predeterminada Go si desea el contrato más sólido.

Pase a Node/TypeScript o Python solo cuando la compensación en tiempo de ejecución local sea real e intencional.

Consulte [Creación de un complemento de tiempo de ejecución Python](/es/guide/python-runtime), [Flujo de trabajo de creación](/es/reference/authoring-workflow) y [FAQ](/es/reference/faq).
</details>
