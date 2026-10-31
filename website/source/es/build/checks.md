---
title: "Comprobar un paquete y sus pruebas"
description: "Distingue las comprobaciones estáticas, la ejecución MCP explícita y la activación del cliente."
canonicalId: "page:build:checks"
section: "build"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Comprobar un paquete y sus pruebas

**Creación desde el código fuente:** estos comandos `agentplugins author` requieren la [compilación con creación habilitada](./#build-from-source). El instalador actual no los incluye. Ejecuta los ejemplos relativos desde el directorio padre del paquete y selecciona siempre la misma raíz exacta.

## Ejecuta el ciclo de creación sin conexión

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

La validación comprueba la configuración estándar registrada y la preparación del paquete. La inspección muestra los componentes y requisitos pendientes. Sin opciones de ejecución, `test` comprueba estáticamente configuración, higiene, Skills y MCP. Estos comandos no arrancan servidores ni ejecutan las instrucciones de un Skill.

## Interpreta los niveles de pruebas

| Nivel | Qué demuestra | Qué necesita otra comprobación |
| --- | --- | --- |
| Conformidad | Los datos estándar cumplen los esquemas y el perfil incorporados. | Un comportamiento útil en un agente concreto. |
| Seguridad del equipo | La entrada cumple comprobaciones acotadas del sistema de archivos y del equipo. | La autorización del usuario para instalar o ejecutar. |
| Preparación | Las pruebas disponibles cumplen la política comprobada. | Aprobación de publicación o aceptación por una plataforma. |
| Compatibilidad | Compatibilidad estática de los destinos y componentes seleccionados. | Detección y activación efectivas en un cliente real. |
| Herramientas | Metadatos acotados del proyecto, los archivos nativos y los ejecutables. | Resolución de dependencias y arranque del servidor. |
| Ejecución | Una prueba MCP explícita informa de las etapas observadas. | Otras herramientas, clientes, flujos OAuth o entornos no probados. |

Lee la conformidad y la preparación global por separado. Un manifiesto válido puede dejar incompletas las pruebas de componentes o del equipo. Mantén visible «no evaluado»; no lo conviertas en un resultado correcto.

## Selecciona los clientes explícitamente

```bash
agentplugins author compat ./review-helper --target codex,claude
agentplugins author inspect ./review-helper --target codex,claude
agentplugins author doctor ./review-helper
agentplugins author capabilities
```

`compat` exige identificadores de destino distintos, separados por comas, y no acepta `all`. Estas comprobaciones estáticas no requieren clientes instalados. `doctor` examina archivos nativos registrados y metadatos de ejecutables sin procesos ni red. `capabilities` informa de esquemas, metadatos de clientes y comandos implementados por este motor; no certifica un paquete ni acredita una publicación pública.

## Evalúa la ejecución MCP explícitamente {#evaluate-mcp-runtime-explicitly}

Para el paquete del [recorrido stdio](./mcp-stdio), instala sus dependencias bloqueadas reconocidas por separado y selecciona el nombre exacto del servidor de `mcp.json`:

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

El modo stdio requiere Linux con contención de procesos disponible mediante cgroup v2 delegado, bubblewrap (`bwrap`) del sistema y una comprobación correcta de sus espacios de nombres. Node debe estar en los directorios de ejecución del sistema (`/usr`, `/bin`, `/lib` o `/lib64`). Si no se puede demostrar la contención, `test` y `dev` rechazan el arranque. Este requisito del proceso stdio no se aplica al transporte Streamable HTTP.

Este comando del código fuente ejecuta el proceso MCP elegido y comprueba inicialización y detección de herramientas. Para llamar a una herramienta, indica tanto `--tool` como `--fixture`. El archivo de prueba contiene un objeto JSON con los argumentos y su ruta es relativa al paquete. Para `hello`, crea `fixtures/hello.json` con `{}`:

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools \
  --tool hello --fixture fixtures/hello.json
```

Para Streamable HTTP, autoriza explícitamente la red:

```bash
agentplugins author test ./docs-service --runtime mcp --server docs-service \
  --allow-network
```

Usa la identidad real del servidor del paquete. El plazo predeterminado es de diez segundos; `--deadline` acepta una duración positiva de hasta un minuto. Examina por separado la inicialización, las herramientas, la llamada opcional y la limpieza. Una prueba correcta aquí no es una ejecución a través de un agente instalado y no demuestra OAuth, otras herramientas ni un despliegue público.

## Itera con el ciclo de desarrollo del código fuente

```bash
agentplugins author dev ./local-tools --server local-tools
```

`dev` ejecuta ciclos MCP acotados al cambiar el paquete; deténlo con Ctrl+C. Usa `--once` para un ciclo único y cuando solicites JSON. Los servidores remotos requieren `--allow-network`; las llamadas a herramientas siguen exigiendo el par explícito de herramienta y archivo de argumentos. El comando no instala en un cliente nativo.

## Obtén un informe revisable

```bash
agentplugins author validate ./review-helper --format json
agentplugins author inspect ./review-helper --format json
```

Usa `--format human` para la salida legible y `--no-color` cuando convenga. `--include-root` revela la raíz seleccionada: evita rutas privadas innecesarias en diagnósticos compartidos. `--release-policy` añade comprobaciones acotadas de preparación para publicación; no la aprueba. Los subcomandos de Skills tienen menos opciones: consulta su propia ayuda.

## Mantén JSON de forma deliberada

El código fuente actual puede planificar una normalización determinista de un archivo estándar seleccionado y escribirla solo con `--write` explícito:

```bash
agentplugins author normalize ./review-helper --document plugin.json
agentplugins author normalize ./review-helper --document plugin.json --write
```

Selecciona `mcp.json` si ese es el documento que quieres normalizar. La importación nativa lee un archivo MCP de Claude explícito en JSON estricto; no busca perfiles. Planifica un destino absoluto nuevo e inexistente antes de aplicar:

```bash
agentplugins author import native ./claude.json --from claude \
  --output /absolute/path/to/new-plugin --name new-plugin \
  --description 'Imported Claude MCP package'
```

La importación admite únicamente entradas stdio conservadoras con `command` y, opcionalmente, `args`. Las entradas con URL HTTP, `env`, `type` u otros campos se omiten; no migra autenticación ni configuración arbitraria del cliente.

Revisa los servidores seguros u omitidos y los campos no admitidos. Añade `--write` para guardar el paquete validado en ese destino inexistente. Se omiten servidores inseguros o con credenciales: examina el resultado en vez de asumir que se importaron todos. Aquí no se expone la importación de otros clientes nativos.

## Elige el comando adecuado

`agentplugins validate ./review-helper` lee un paquete para instalarlo; `agentplugins doctor` diagnostica el estado gestionado. Son distintos de la creación desde el código fuente. Las opciones de ámbito y seguridad del instalador no se convierten en opciones de creación. `init` crea archivos reales; normalización e importación requieren `--write` para aplicar sus planes. Continúa con [la entrega del paquete](./handoff).
