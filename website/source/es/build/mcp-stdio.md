---
title: "Crear un paquete MCP stdio"
description: "Prepara un servidor MCP local en Node y evalúa dependencias y ejecución por separado."
canonicalId: "page:build:mcp-stdio"
section: "build"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Crear un paquete MCP stdio

**Creación desde el código fuente:** estos comandos requieren la [compilación con creación habilitada](./#build-from-source). No están incluidos en el instalador actual.

Elige stdio si el paquete proporciona un proceso local que se comunica por entrada y salida estándar. La estructura inicial admite Node.js 22 o posterior y usa el SDK oficial de MCP. Crearla no instala dependencias ni arranca un servidor.

## Crea la estructura inicial

Elige un destino inexistente dentro de un directorio padre existente:

```bash
agentplugins author init ./local-tools \
  --template mcp-stdio \
  --name local-tools \
  --description 'Provide local tools through a Node MCP server' \
  --runtime node
```

`node` es el entorno de ejecución admitido por esta plantilla portable. Las opciones de Python o Go no forman parte del contrato actual de este comando.

## Revisa los archivos

```text
local-tools/
  plugin.json
  mcp.json
  package.json
  package-lock.json
  src/
    server.mjs
  README.md
  .gitignore
```

`mcp.json` declara el servidor `local-tools`, con el comando `node` y el argumento `${PLUGIN_ROOT}/src/server.mjs`. El marcador del paquete se resuelve dentro del paquete seleccionado; no significa el directorio actual del terminal. El código comienza con una herramienta pequeña, `hello`. Adáptala a tu tarea y envía los registros de la aplicación a stderr para reservar stdout al protocolo MCP.

Revisa el manifiesto de dependencias y el archivo de bloqueo. `init` los escribe, pero no ejecuta npm ni demuestra que el equipo pueda importar el SDK.

## Obtén pruebas estáticas

```bash
agentplugins author validate ./local-tools
agentplugins author inspect ./local-tools
agentplugins author test ./local-tools
agentplugins author compat ./local-tools --target codex,claude
agentplugins author doctor ./local-tools
```

Son comprobaciones estáticas. `doctor` examina metadatos acotados del proyecto y los ejecutables; no demuestra que las dependencias estén instaladas, que la negociación MCP funcione ni que una herramienta se ejecute correctamente. Interpreta por separado [cada nivel de pruebas](./checks).

## Instala las dependencias de la plantilla reconocida

Para la estructura Node generada con su archivo de bloqueo coincidente, el código fuente actual permite previsualizar e instalar las dependencias:

```bash
agentplugins author bootstrap ./local-tools --dry-run
agentplugins author bootstrap ./local-tools
```

Revisa la vista previa antes de aplicar. `bootstrap` usa una instalación npm bloqueada y desactiva los scripts de ciclo de vida. Solo acepta la estructura generada reconocida, archivos de paquete coincidentes y la ausencia de `node_modules`; no sustituye a un gestor de paquetes general. Puede rechazar archivos de dependencias modificados, bloqueos en conflicto o salidas existentes. Resuelve el requisito indicado en vez de buscar una opción de sobrescritura. La obtención de dependencias puede usar la red.

La aplicación de `bootstrap` requiere Linux y acceso válido a los directorios mediante descriptores de `/proc`; si el equipo no lo admite, devuelve `bootstrap_platform_unsupported`. En macOS y Windows puedes obtener el plan con `--dry-run`, pero este comando no aplica la instalación de dependencias.

## Ejecuta una comprobación MCP explícita

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

La ejecución stdio requiere Linux, una delegación cgroup v2 que supere la comprobación de contención de procesos y bubblewrap (`bwrap`) del sistema con espacios de nombres utilizables. Node debe resolverse dentro de los directorios del sistema `/usr`, `/bin`, `/lib` o `/lib64`. Si no se cumplen estos requisitos, la prueba y `dev` rechazan el arranque; no ejecutan el servidor sin contención.

Este comando ejecuta el proceso local seleccionado y comprueba la inicialización MCP y la detección de herramientas. Para llamar a `hello`, hace falta un archivo JSON de argumentos explícito; consulta [el ejemplo de ejecución](./checks#evaluate-mcp-runtime-explicitly). Para iterar continuamente:

```bash
agentplugins author dev ./local-tools --server local-tools
```

Conserva la identidad exacta del motor, el paquete y la plataforma junto al resultado. Una prueba directa de creación no demuestra que el servidor funcione a través de todos los agentes nativos ni que el cliente haya otorgado confianza o completado la autenticación.

## Prepara la entrega

Explica las herramientas, el requisito de Node.js, los archivos de dependencias y las pruebas de ejecución en el README. Indica el servidor elegido y las herramientas o entornos no probados. Comparte el paquete mediante [la entrega al instalador](./handoff). Añade [más Skills](./skills) si los usuarios necesitan instrucciones junto a las herramientas.
