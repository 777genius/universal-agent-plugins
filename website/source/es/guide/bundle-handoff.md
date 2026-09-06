---
title: "Transferencia de paquete"
description: "Cómo exportar, instalar, recuperar y publicar paquetes portátiles Python y Node para flujos de transferencia admitidos."
canonicalId: "page:guide:bundle-handoff"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">Transferencia de paquete</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# Transferencia de paquete

Utilice esta guía cuando un complemento Python o Node deba viajar como un artefacto portátil en lugar de como un repositorio en vivo.

Esta es una capacidad pública real, pero es intencionalmente más estrecha que la ruta principal Go.

## Qué cubre

El subconjunto de transferencia de paquete estable es para:

- paquetes `python` exportados en `codex-runtime` y `claude`
- paquetes `node` exportados en `codex-runtime` y `claude`
- instalación del paquete local
- búsqueda remota de paquetes
- GitHub Publicación del paquete de versiones

Esta es la opción adecuada cuando:

- otro equipo debería recibir un artefacto listo en lugar de tu repositorio completo
- su flujo de lanzamiento ya utiliza lanzamientos GitHub
- desea una historia de transferencia más limpia para los tiempos de ejecución Python o Node

## El flujo práctico

El lado del productor es:

```bash
plugin-kit-ai export .
plugin-kit-ai bundle publish . --platform <codex-runtime|claude> --repo <owner/repo> --tag <tag>
```

El lado del consumidor es:

```bash
plugin-kit-ai bundle install <bundle.tar.gz> --dest <path>
```

o:

```bash
plugin-kit-ai bundle fetch <owner/repo> --tag <tag> --platform <codex-runtime|claude> --runtime <python|node> --dest <path>
```

Después de la instalación o recuperación, el repositorio resultante aún necesita su arranque normal en tiempo de ejecución y comprobaciones de preparación.

## Lo que no sucede automáticamente

`bundle install` y `bundle fetch` no convierten silenciosamente el paquete en un complemento completamente validado.

Trate el paquete instalado como el inicio de la configuración posterior:

Primero, instale los requisitos previos del tiempo de ejecución
2. ejecute `plugin-kit-ai doctor .`
3. ejecute cualquier paso de arranque requerido
4. ejecute `plugin-kit-ai validate . --platform <target> --strict`

## Cuando la transferencia de paquetes es mejor que un repositorio en vivo

Elija la transferencia del paquete cuando:

- los artefactos de lanzamiento son el contrato de entrega real
- los consumidores intermedios no deben clonar el repositorio de origen
- desea una distribución de lanzamientos GitHub repetible para los carriles Python o Node

Manténgase en la ruta del repositorio en vivo cuando:

- el equipo todavía edita la fuente del proyecto directamente
- la principal necesidad es la colaboración dentro de un repositorio
- Go ya le brinda la transferencia binaria compilada limpia que necesita

## Límite importante

La transferencia de paquetes no es un “paquete universal para cada objetivo”.

Es un flujo de transferencia portátil compatible para el subconjunto exportado Python y Node en `codex-runtime` y `claude`.

No asuma que el mismo contrato se aplica a:

- Go SDK repositorios
- objetivos de configuración del espacio de trabajo como Cursor o OpenCode
- objetivos de solo embalaje como Gemini
- CLI instalar paquetes

## Orden de lectura recomendado

Empareje esta página con [Elegir modelo de entrega](/es/guide/choose-delivery-model), [Preparación para la producción](/es/guide/production-readiness) y [Límite de soporte](/es/reference/support-boundary).
</details>
