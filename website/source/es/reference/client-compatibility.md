---
title: "Compatibilidad de clientes UAP"
description: "La evidencia detallada de compatibilidad se mantiene en inglés."
canonicalId: "page:reference:client-compatibility"
section: "reference"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "english-fallback"
---

# Compatibilidad de clientes UAP

La evidencia detallada, las versiones probadas y sus limitaciones se mantienen en la página canónica en inglés.

[Abrir la evidencia en inglés](/en/reference/client-compatibility)

## Ámbito de proyecto en el código actual

**Solo código fuente; aún no publicado.** Las observaciones históricas de
versiones y clientes anteriores conservan su ámbito original. La instalación por
proyecto tiene un contrato propio para cada cliente: propiedad de archivos,
detección nativa y funcionamiento de MCP/modelos se verifican por separado.
ChatGPT rechaza explícitamente este ámbito; los demás destinos tienen sus
propias restricciones de componentes, preparación y cliente compatible.

La preparación de proyectos para VS Code utiliza `.github/skills` y
`.vscode/mcp.json`. La [observación fijada de VS Code 1.143.1](https://github.com/777genius/universal-agent-plugins/actions/runs/40188310034)
verificó metadatos en carpetas A/B separadas, tras un nuevo inicio de A y después
de la eliminación, sin conceder confianza al espacio de trabajo. Es evidencia
de metadatos: el adaptador informa Prepared y verifica los archivos instalados;
no demuestra inicio de MCP, llamadas a herramientas ni ejecución del modelo.

[Guía de instalación por proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Compatibilidad y límites actuales por proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
