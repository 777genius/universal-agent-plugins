---
title: "Crear plugins"
description: "Prepara un Skill, un paquete MCP o un plugin híbrido y comprueba sus límites."
canonicalId: "page:build:index"
section: "build"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Crear plugins

Un paquete Agent Plugins 1.0 es un directorio con `plugin.json` en la raíz y Skills, configuración MCP o ambos. Empieza por el componente más pequeño que resuelva la tarea de tus usuarios. Un Skill aporta instrucciones; MCP aporta una conexión con herramientas.

## Disponibilidad

El instalador nativo actual, [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94), puede validar e instalar paquetes estándar. **No incluye `agentplugins author`.** Los comandos de creación de este recorrido describen **compilaciones del código fuente actual** con la creación habilitada. Las pruebas de publicación de las herramientas de creación 0.1.65 siguen siendo específicas de aquella versión y no describen el instalador actual.

Puedes escribir los archivos a mano y usar el instalador publicado, o compilar las herramientas siguientes para generar una estructura inicial, realizar comprobaciones estáticas y ejecutar explícitamente un ciclo MCP. Superar una comprobación estática no demuestra que el cliente nativo active el paquete ni que la conexión con un servicio funcione.

## Prepara un paquete pequeño a mano

Crea esta estructura:

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

Define la identidad del paquete en `plugin.json`:

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "review-helper",
  "version": "0.1.0",
  "description": "Review documentation changes for clarity and evidence"
}
```

Empieza `skills/review-docs/SKILL.md` así:

```markdown
---
name: review-docs
description: Úsalo para revisar la claridad y las pruebas de una documentación.
---

# Revisar documentación

Identifica al lector y la tarea. Comprueba las entradas y los efectos de cada
comando. Distingue el comportamiento verificado de los requisitos y las
suposiciones sin probar. Devuelve los problemas con su ubicación y una corrección.
```

Instala el gestor publicado y valida y previsualiza el paquete:

```bash
npm install -g universal-agent-plugins
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

Revisa el plan y consulta [Usar plugins](../use/install) cuando quieras aplicarlo. La [estructura del paquete](./layout) explica la responsabilidad de cada archivo.

## Prepara las herramientas de creación desde el código fuente {#build-from-source}

Esta preparación para desarrolladores compila el contrato público de creación del código fuente actual. Necesitas Git y la versión de Go declarada por el repositorio, actualmente Go 1.26.10. Anota el commit que hayas descargado cuando compartas resultados.

```bash
git clone https://github.com/777genius/universal-agent-plugins.git
cd universal-agent-plugins/cli
mkdir -p ../bin
authoring_module=$(go list -f '{{.Module.Path}}' ./cmd/agentplugins)
authoring_revision=$(git rev-parse HEAD)
go build -ldflags "-X ${authoring_module}/internal/authoring/commands.Enabled=release-cli-contract-v1 -X ${authoring_module}/internal/authoring/commands.Revision=${authoring_revision}" \
  -o ../bin/agentplugins ./cmd/agentplugins
export PATH="$(cd ../bin && pwd):$PATH"
agentplugins author --help
```

Usa otro directorio de trabajo para los paquetes de los ejemplos. Este binario es una compilación de desarrollo; su existencia no acredita una publicación pública de las herramientas de creación. Comprueba la ruta del ejecutable si el gestor sigue indicando que no conoce el comando `author`.

## Elige una estructura inicial

| Necesidad | Recorrido | Entradas de plantilla obligatorias |
| --- | --- | --- |
| Instrucciones para un agente | [Un Skill](./skill) | `--template skill` |
| Conectar un servicio existente | [MCP remoto](./mcp-remote) | `--template mcp-remote --url <endpoint>` |
| Proporcionar un proceso MCP local en Node | [MCP stdio](./mcp-stdio) | `--template mcp-stdio --runtime node` |
| Instrucciones junto con herramientas | [Plugin híbrido](./hybrid) | `--template hybrid --mcp-template <choice>` y las entradas de esa plantilla MCP |

Un paquete con un solo Skill también contiene `plugin.json` en la raíz. Instalar Skills independientes desde otro repositorio es una [operación del instalador](../use/install#install-agent-skills) distinta.

## Sigue el ciclo de creación

1. Crea un paquete en un destino que no exista y edita los archivos generados.
2. Ejecuta [validate, inspect y la prueba estática](./checks) sobre la raíz exacta.
3. Comprueba los clientes previstos con `compat` y examina las pruebas de `doctor`.
4. Para MCP, evalúa la ejecución por separado y de forma explícita con la compilación del código fuente.
5. Comparte el paquete y sus pruebas mediante la [entrega al instalador](./handoff).

El código fuente actual también ofrece `bootstrap` basado en archivos de bloqueo, `dev` continuo, `normalize` para JSON e `import native` explícito para MCP de Claude. Consulta [las comprobaciones](./checks) y [MCP stdio](./mcp-stdio). La generación de proyecciones para clientes, la exportación de paquetes y su publicación quedan fuera de estos comandos de creación. Los hooks específicos de cada cliente siguen separados de los componentes portables Skill y MCP.
