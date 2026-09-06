---
title: "Instalación"
description: "Instale plugin-kit-ai utilizando canales compatibles."
canonicalId: "page:guide:installation"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<details><summary>Material histórico de plugin-kit-ai v1 · 1.2.4; las instrucciones antiguas no son el flujo Build actual.</summary>

# Instalación

Utilice Homebrew de forma predeterminada cuando se ajuste a su entorno. El objetivo aquí es simple: instalar CLI y llegar rápidamente a su primer repositorio funcional.

## Canales admitidos

- Homebrew para la ruta CLI predeterminada más limpia.
- npm cuando su entorno ya está centrado en npm.
- PyPI/pipx cuando su entorno ya está centrado en Python.
- Script de instalación verificado como ruta alternativa.

## Comandos recomendados

### Homebrew

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

### npm

```bash
npm i -g plugin-kit-ai
plugin-kit-ai version
```

### PyPI/pipx

```bash
pipx install plugin-kit-ai
plugin-kit-ai version
```

### Guión verificado

```bash
curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh
plugin-kit-ai version
```

## ¿Cuál debería usar la mayoría de la gente?

- Utilice Homebrew si está en macOS y desea la ruta predeterminada más fluida.
- Utilice npm o pipx solo cuando ya coincida con el entorno de su equipo.
- Utilice el script verificado cuando necesite una alternativa fuera de las configuraciones del administrador de paquetes primero.

## Después de la instalación

La mayoría de las personas deberían continuar directamente con el [Inicio rápido](/es/guide/quickstart) y crear el primer repositorio en la ruta predeterminada Go.

Si eligió `pipx` porque su equipo es Python-primero y ya sabe que desea la ruta Python, continúe con [Crear un complemento de tiempo de ejecución Python](/es/guide/python-runtime).

## Ruta de instalación de CI

Para CI, prefiera la acción de configuración dedicada en lugar de enseñar a cada flujo de trabajo cómo descargar el CLI manualmente.

## Límite importante

Los paquetes npm y PyPI son canales de instalación para CLI. No son APIs de tiempo de ejecución ni SDKs.

Consulte [Referencia > Canales de instalación](/es/reference/install-channels) para conocer los límites del contrato.

</details>
