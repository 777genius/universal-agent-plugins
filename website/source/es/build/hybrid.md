---
title: "Crear un paquete híbrido"
description: "Combina instrucciones Skill y configuración MCP bajo una identidad de paquete."
canonicalId: "page:build:hybrid"
section: "build"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Crear un paquete híbrido

**Creación desde el código fuente:** estos comandos requieren la [compilación con creación habilitada](./#build-from-source). No están incluidos en el instalador actual.

Un paquete híbrido resulta útil cuando las instrucciones explican cómo y cuándo usar herramientas. Ambos componentes comparten un manifiesto raíz. Cada cliente puede admitirlos de forma distinta; conserva los resultados de compatibilidad de cada componente al entregar el paquete.

## Elige explícitamente la parte MCP

La opción es `--mcp-template`. Elige `mcp-remote` o `mcp-stdio` y proporciona las entradas obligatorias de esa plantilla. Un paquete híbrido no deduce el transporte a partir de la URL, los archivos locales o los entornos de ejecución instalados.

Ejemplo de híbrido remoto:

```bash
agentplugins author init ./research-helper \
  --template hybrid \
  --name research-helper \
  --description 'Guide research using the team MCP service' \
  --skill-name research-guide \
  --mcp-template mcp-remote \
  --url https://mcp.example.com/mcp
```

La URL es un ejemplo, no un servicio garantizado. Sustitúyela por el endpoint real y consulta [los requisitos de MCP remoto](./mcp-remote).

## O elige un proceso local

Para un destino diferente que tampoco exista, usa la variante stdio:

```bash
agentplugins author init ./local-research \
  --template hybrid \
  --name local-research \
  --description 'Guide research using local Node tools' \
  --skill-name research-guide \
  --mcp-template mcp-stdio \
  --runtime node
```

No añadas `--url` a esta variante. Revisa [los requisitos y límites de ejecución de stdio](./mcp-stdio).

## Relaciona las instrucciones con las herramientas

La estructura esencial del híbrido remoto es:

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

También se crean README y `.gitignore`. El híbrido stdio añade el código Node y los archivos de dependencias descritos en su recorrido. Revisa `SKILL.md` para que indique cuándo usar las herramientas, qué entradas necesitan y cómo informar de la falta de acceso. El texto de un Skill no concede autorización.

## Comprueba ambos componentes

```bash
agentplugins author skills validate ./research-helper
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper --target codex,claude
agentplugins author test ./research-helper
agentplugins author compat ./research-helper --target codex,claude
```

Para la variante local, selecciona `./local-research`. Conserva los resultados por componente: que un cliente admita el Skill no significa que admita el transporte MCP ni que active el servicio. Combinar los componentes no elimina ese límite.

Continúa con [más Skills](./skills) o [la entrega](./handoff).
