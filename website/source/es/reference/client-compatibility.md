---
title: "Compatibilidad de clientes UAP"
description: "Alcance actual y guía de las pruebas históricas por versión, plataforma y cliente."
canonicalId: "page:reference:client-compatibility"
section: "reference"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Compatibilidad de clientes UAP

## Versión actual y alcance

[CLI 0.1.93](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93), publicado el 26 de octubre de 2026, incluye la instalación de proyecto explícita y la restauración bloqueada. Las pruebas de su publicación como instalador no acreditan la ejecución de todos los clientes nativos ni el ejecutable estándar de creación. El instalador actual npm o nativo no incluye `agentplugins author`; consulta [Crear plugins](../build/) para las herramientas del código fuente.

Interpreta la compatibilidad como comprobaciones independientes:

| Comprobación | Qué significa |
| --- | --- |
| Conformidad del paquete | Sus archivos estándar cumplen el contrato del lector y del esquema. |
| Entrega de componentes | El adaptador preparó o instaló los componentes que gestiona. |
| Detección nativa | Un cliente concreto observó el componente previsto. |
| Ejecución MCP | Un proceso o conexión registrado se inicializó e hizo las llamadas probadas. |
| Autenticación y uso del modelo | Solo se demuestran si las pruebas los ejercitan explícitamente. |

La entrega de proyecto suele quedar **Prepared** mientras falten confianza, recarga, configuración explícita o una nueva sesión. ChatGPT rechaza la instalación de proyecto. VS Code prepara archivos para una sola carpeta local; observar sus metadatos no demuestra arranque MCP ni ejecución de modelos. Consulta [la compatibilidad por proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md) y [el índice de pruebas de proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/PROJECT_INSTALLATION.md).

## Cómo leer las pruebas históricas

Los registros siguientes conservan sus versiones, plataformas y límites originales. No se aplican automáticamente a 0.1.93, a un cliente posterior, a otro ámbito ni a un paquete no probado. UAP es una herramienta comunitaria independiente; estas observaciones de los mantenedores no son una certificación del proveedor.

| Registro | Alcance y resultado observado | Fuente verificable |
| --- | --- | --- |
| Borrador histórico 0.1.55, septiembre de 2026 | Dos ámbitos independientes: 12 combinaciones de cliente y plataforma, 24 pruebas principales obligatorias, sin omisiones. En aquel punto era un borrador sin publicar. | [Metadatos exactos](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-draft-0.1.55.json), ejecuciones [34309083566](https://github.com/777genius/universal-agent-plugins/actions/runs/34309083566) y [34307075390](https://github.com/777genius/universal-agent-plugins/actions/runs/34307075390). |
| Instalador publicado 0.1.53 | Binarios públicos: nueve combinaciones en Linux arm64, macOS arm64 y Windows amd64; 18 pruebas obligatorias sin omisiones. Se verificaron manifiesto y atestaciones antes de ejecutar. | [Publicación](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.53), [ejecución 34166817934](https://github.com/777genius/universal-agent-plugins/actions/runs/34166817934), [identidades y hashes](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-released-0.1.53.json). |
| Suplemento Linux amd64 de 0.1.53 | Mismos binarios publicados, Ubuntu 24.04; tres combinaciones y seis pruebas obligatorias sin omisiones. Complementa los nueve trabajos anteriores sin reemplazarlos. | [Ejecución 34195481283](https://github.com/777genius/universal-agent-plugins/actions/runs/34195481283), [identidades y hashes](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-linux-amd64-0.1.53.json). |
| Código fuente del 7 de septiembre de 2026 | Linux arm64 tiene identidad propia. macOS arm64 y Windows amd64 usaron `ef20b93`; seis combinaciones sin omisiones, 348 hashes verificados. No son binarios publicados ni una repetición del commit de fusión. | [Identidades históricas](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-2026-09-07.json), [ejecución 34158664586](https://github.com/777genius/universal-agent-plugins/actions/runs/34158664586). |

Los clientes fijados en esos registros son Codex 0.153.4, Claude Code 2.1.263 y OpenCode 1.18.29. Las pruebas de ciclo de vida cubren instalación, actualización de versión, refresco de la misma versión, reparación y eliminación. Las pruebas MCP stdio/HTTP y de Skills tienen un alcance propio; no amplíes una celda correcta a otro transporte o configuración.

El borrador 0.1.55 usó el tarball npm original, instalación npm sin conexión con scripts desactivados, arranque en frío con el binario congelado y arranque posterior sin el origen de las pruebas. Los dos ámbitos se vinculan a [un mismo productor](https://github.com/777genius/universal-agent-plugins/actions/runs/34305637370). Revalidar hashes sin conexión no es una ejecución nativa nueva ni una comprobación actual del estado de aquel borrador. Sus registros y archivos completos siguen siendo privados, pendientes de revisión para divulgación; solo los metadatos son públicos.

Las pruebas 0.1.53 se publican en [un archivo independiente](https://github.com/777genius/universal-agent-plugins/releases/tag/native-evidence-0.1.53-34166817934), con [ZIP](https://github.com/777genius/universal-agent-plugins/releases/download/native-evidence-0.1.53-34166817934/uap-0.1.53-native-34166817934.zip) y [suma de comprobación](https://github.com/777genius/universal-agent-plugins/releases/download/native-evidence-0.1.53-34166817934/uap-0.1.53-native-34166817934.zip.sha256). El [archivo Linux amd64](https://github.com/777genius/universal-agent-plugins/releases/tag/native-evidence-0.1.53-linux-amd64-34195481283) también es independiente. La [guía de reproducción](https://github.com/777genius/universal-agent-plugins/blob/main/docs/released-native-client-proof.md) explica la verificación. Una prueba del lanzador en seis plataformas no amplía por sí sola las pruebas nativas a otras arquitecturas.

## Límites que siguen siendo explícitos

- **Claude en Windows:** funcionan las etapas observadas HTTP y Skills instalados. Stdio gestionado informa `managed_stdio_platform_unsupported`; no se demuestran su ejecución ni cwd, argv, entorno o continuidad de datos.
- **OpenCode:** la colisión de nombres de herramientas sigue sin resolverse y produce `opencode_tool_namespace_not_evaluated`. Conservar claves de configuración distintas no garantiza nombres invocables distintos. Las pruebas JSON/JSONC de ciclo de vida no demuestran ejecución stdio ni cuerpo de Skills en JSONC; la prueba extendida corresponde a JSON global.
- **Codex:** excluye MCP SSE declarado. Sus pruebas de fallos, cabeceras y redirecciones tienen su propio alcance; no se atribuyen automáticamente a Claude u OpenCode. La adquisición Git observada no convierte las pruebas en una demostración de ejecución MCP pública por Git.
- **Origen y publicación:** el Linux histórico usó un instalador compilado desde código fuente. La publicación 0.1.51 verificada entonces no incluía PR #184. No traslades sus resultados candidatos a ese binario ni a otro SHA.
- **Entorno:** se usaron clientes reales con proveedores de modelo programados en bucle local y acceso a red. Perfiles desechables y entornos de procesos explícitos no equivalen a aislamiento del sistema operativo. No se demuestra razonamiento de modelos reales, OAuth, inicio de sesión, aplicaciones de escritorio, servicios públicos, adopción independiente ni certificación oficial. La protección observada de archivos ajenos no garantiza seguridad frente a escritores arbitrarios concurrentes.

El [registro macOS anterior](https://github.com/777genius/universal-agent-plugins/blob/main/docs/AGENTPLUGINS_CLIENT_E2E.md) de UAP 0.1.22 solo cubre detección y ciclo de vida con sus propios clientes y resúmenes. No acredita ejecución actual. La [referencia histórica detallada en inglés](/en/reference/client-compatibility) conserva todos los hashes, identidades de los arneses, tablas y distinciones de pruebas; esta página ofrece su guía de lectura en español.

## Pruebas del ámbito de proyecto

La entrega de proyecto explícita se publicó en CLI 0.1.93. La propiedad de archivos, la detección nativa y el funcionamiento MCP o del modelo se informan por separado, según cada cliente. Los resultados de ámbito de usuario no acreditan el de proyecto.

La [observación fijada de VS Code 1.143.1](https://github.com/777genius/universal-agent-plugins/actions/runs/40188310034) verificó metadatos en carpetas A/B separadas, un nuevo arranque de A y la eliminación, sin otorgar confianza al espacio de trabajo. El adaptador prepara `.github/skills` y `.vscode/mcp.json`, mantiene el resultado Prepared y verifica los archivos instalados. No afirma arranque MCP, llamadas a herramientas ni ejecución de modelos.

[Guía de instalación de proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Compatibilidad y límites de proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
