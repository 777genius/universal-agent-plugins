---
title: "Inicio rápido"
description: "Instala plugins disponibles o prepara un paquete portable Agent Plugins 1.0."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

<a id="inicio-rapido"></a>

# Usar plugins / Crear plugins

Instala plugins disponibles o prepara un paquete portable Agent Plugins 1.0.

## Usar plugins {#use-plugins}

Disponible ahora: instala plugins con Universal Agent Plugins. Con Node.js 22+, ejecuta:

```bash
npx universal-agent-plugins add context7
```

Para la instalación nativa en macOS, Linux o Windows, consulta la guía. El CLI pregunta qué agentes compatibles usar; cada plugin puede necesitar su propio entorno de ejecución.

[Guía de instalación](https://github.com/777genius/universal-agent-plugins#quick-start)

La creación de Agent Plugins está disponible en `universal-agent-plugins@0.1.65`; versión de GitHub: `agentplugins-v0.1.65`.

La compatibilidad depende del paquete. Validar el esquema no demuestra ejecución, OAuth ni activación. Codex no admite MCP SSE declarado; stdio y Streamable HTTP conservan el soporte de sus adaptadores actuales.

[Compatibilidad de clientes (en inglés)](https://github.com/777genius/universal-agent-plugins#supported-clients)

## Crear plugins {#build-plugins}

Crea un paquete portable Agent Plugins 1.0 basado en plugin.json, con skills/ y mcp.json opcionales. El soporte de cada cliente depende del paquete y sus componentes.

**La creación de Agent Plugins está disponible**

La creación estática está disponible mediante `agentplugins author` en `universal-agent-plugins@0.1.65`; versión de GitHub: `agentplugins-v0.1.65`. npm, Homebrew, archivos nativos y E2E público están verificados. Esta versión no ejecuta el código del paquete ni servidores de desarrollo, no instala dependencias, no exporta paquetes y no los publica.

[Especificación Agent Plugins 1.0](https://agent-plugins.org/specification)
