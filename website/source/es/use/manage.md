---
title: "Mantener los plugins instalados"
description: "Consulta, actualiza, repara y restaura instalaciones de usuario o de proyecto."
canonicalId: "page:use:manage"
section: "use"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Mantener los plugins instalados

Usa el instalador para consultar y modificar el estado que gestiona. Al investigar un problema, conserva en el informe el origen del paquete, los clientes elegidos, el ámbito y la identidad de instalación. Estos ejemplos usan el ámbito de usuario salvo que seleccionen un proyecto explícitamente.

## Examina primero la situación

```bash
agentplugins list
agentplugins doctor
agentplugins outdated --all
```

`list` informa de las instalaciones, `doctor` diagnostica el estado gestionado y `outdated` busca posibles actualizaciones sin aplicarlas. Ninguno demuestra que un servicio remoto funcione o que un cliente haya completado la autenticación.

| Síntoma | Siguiente paso |
| --- | --- |
| Falta el paquete | Vuelve a [buscar e instalar](./install). |
| Faltan archivos gestionados o han cambiado | Revisa un plan de reparación para ese paquete y ámbito. |
| Hay una versión más reciente de un origen seguido | Revisa un plan de actualización. |
| El cliente necesita confianza, activación o inicio de sesión | Sigue las instrucciones indicadas para ese destino. |
| Falla la validación de un paquete local | Entrega al autor el informe y la [guía de comprobaciones](../build/checks). |

## API local de gestión (CLI 0.1.95) {#local-management-api}

[CLI nativo 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95) añade `agentplugins manage serve` para aplicaciones que integran el instalador y `agentplugins manage discover` para descubrir clientes y perfiles locales sin modificarlos. La detección no ejecuta clientes, consulta sus versiones, descarga orígenes ni cambia archivos.

`manage serve` es un proceso controlado por la aplicación integradora, con un contexto capturado al arrancar y una API privada autenticada en el loopback IPv4. La aplicación proporciona JSON por stdin y mantiene privados la URL inicial y la credencial bearer. Comprueba la ayuda de los comandos antes de integrar:

```bash
agentplugins version
agentplugins manage serve --help
agentplugins manage discover --help
```

Consulta el [contrato del protocolo de gestión](https://github.com/777genius/universal-agent-plugins/blob/main/contracts/management/README.md) para conocer las reglas de arranque y ciclo de vida. El [TypeScript Management SDK](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md) sigue siendo un paquete de desarrollo independiente que todavía no se ha publicado.

## Actualiza de forma deliberada

```bash
agentplugins update context7 --target codex,cursor --dry-run
```

Revisa el origen, la versión y los destinos elegidos. Omite `--dry-run` para aplicar:

```bash
agentplugins update context7 --target codex,cursor
```

`update --all` abarca todos los candidatos de actualización gestionados. Úsalo cuando quieras abarcar ese conjunto completo. Un origen fijado a un commit exacto sigue siendo inmutable: actualizar no lo convierte en un seguimiento de rama. Los rangos de dependencias guardados y las referencias móviles avanzan mediante la operación explícita de actualización que corresponda.

## Repara el estado registrado

```bash
agentplugins repair context7 --target codex,cursor --dry-run
agentplugins repair context7 --target codex,cursor
```

El primer comando previsualiza y el segundo aplica. La reparación vuelve a usar el origen registrado para restaurar el contenido gestionado. No selecciona una revisión remota nueva ni sobrescribe silenciosamente entradas nativas modificadas externamente. Revisa los conflictos y las instrucciones de recuperación pendientes antes de continuar.

## Elimina la instalación seleccionada

```bash
agentplugins remove context7 --target codex,cursor --dry-run
agentplugins remove context7 --target codex,cursor
```

Revisa el primer plan antes de ejecutar el segundo comando. La eliminación usa la propiedad registrada por el CLI. No revoca cuentas de servicios ni elimina entornos de ejecución ajenos ni borra el código fuente del autor. Por defecto, eliminar una instalación de proyecto conserva los datos del plugin y el almacén compartido de contenido verificado.

## Usa un entorno de dependencias guardado

`uap.toml` registra los paquetes y destinos deseados; `uap.lock`, sus orígenes resueltos exactos y sus resúmenes criptográficos. Examina el entorno seleccionado:

```bash
agentplugins deps status
agentplugins deps tree
```

Para un entorno nuevo, ejecuta `agentplugins deps init`, edita el manifiesto deliberadamente, resuélvelo con `agentplugins deps lock` y aplica el bloqueo con `agentplugins install --locked --scope user`. Revisa las confirmaciones y usa `--dry-run` para previsualizar. La [guía de dependencias](https://github.com/777genius/universal-agent-plugins/blob/main/docs/UAP_DEPENDENCIES.md) explica la sintaxis del manifiesto, las dependencias obligatorias, los rangos de versiones y las actualizaciones.

Desde CLI 0.1.93, las instalaciones remotas elegibles guardan las dependencias por defecto. `--no-save`, los orígenes locales, los selectores literales `discovery:` y la preparación de ChatGPT quedan fuera de ese guardado portable. Si una selección combina ChatGPT con otros destinos, sepárala o usa `--no-save`. No des por hecho que una instalación imperativa anterior forme parte del acuerdo: consulta `deps status` y los archivos reales antes de transferirlos.

## Restaura en otra máquina

Transfiere un entorno de usuario guardado mediante una carpeta de intercambio. Desde esa carpeta en la máquina de origen:

```bash
agentplugins deps export
```

Copia la carpeta al destino, instala allí los clientes y ejecuta:

```bash
agentplugins deps import
agentplugins install --locked --scope user
```

También puedes combinar importación y restauración:

```bash
agentplugins deps import --install
```

El argumento de directorio opcional selecciona otra carpeta de intercambio. La exportación lee el entorno de usuario y la importación lo escribe; `AGENTPLUGINS_HOME` permite seleccionar uno concreto. La carpeta transferida no sustituye el perfil del cliente. `--scope user` explícito mantiene la restauración en el entorno de usuario.

Solo se transfieren los bytes originales de `uap.toml` y `uap.lock`, validados estrictamente. Cada copia efectiva muestra una vista previa y exige confirmación o `--yes`; `--dry-run` previsualiza. La validación de importación funciona sin conexión, pero la instalación necesita los orígenes registrados o paquetes verificados en caché. Una restauración sin caché descarga las revisiones exactas y comprueba sus resúmenes; no elige versiones nuevas.

No se transfieren perfiles, ajustes, credenciales, recibos ni `PLUGIN_DATA`. Configura la autenticación en la máquina nueva y respalda los datos por separado. Si falla la instalación después de guardar los dos archivos, corrige la causa y repite `install --locked --scope user`. Ejecuta antes `deps recover --scope user` si se informa de recuperación pendiente. No recrees las revisiones transferidas con `deps lock`: un selector móvil podría resolverse de otra forma.

<a id="restore-project-dependencies"></a>

## Restaura y mantén un proyecto

La instalación automática de proyecto está disponible desde [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94). Guarda `uap.toml` y `uap.lock` del proyecto en control de versiones. En otra copia existente del repositorio, instala los clientes necesarios y ejecuta desde el directorio exacto del proyecto:

```bash
agentplugins install
```

Un bloqueo existente se valida estrictamente. `--locked` exige además que exista, lo que resulta útil en CI; sin bloqueo, la primera instalación puede crearlo. Si ejecutas desde otro directorio, selecciona explícitamente el proyecto existente:

```bash
agentplugins install --locked --scope project --project-root /path/to/project
```

Selecciona el mismo ámbito y raíz al mantenerlo:

```bash
agentplugins list --scope project --project-root /path/to/project
agentplugins deps status --scope project --project-root /path/to/project
agentplugins repair review --target cursor --scope project --project-root /path/to/project
agentplugins deps recover --scope project --project-root /path/to/project
```

Si un nombre es ambiguo, usa la identidad de instalación que devuelve `list`. `add` y los comandos de mantenimiento usan por defecto el ámbito de usuario; selecciona el proyecto explícitamente. Desde 0.1.94, `install` sin argumento de origen puede seleccionarlo automáticamente como se explica a continuación. `--scope project` explícito, sin `--project-root`, usa solo el directorio de ejecución.

Conserva localmente los recibos, diarios de recuperación y datos de `.uap/`. Restaura los componentes nativos desde el acuerdo portable y sigue las instrucciones del cliente sobre confianza o activación. Los orígenes y el acceso a los privados deben seguir disponibles. Consulta la [instalación de proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) y los [límites por cliente](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md).

## Instalación automática de proyecto en CLI 0.1.94

**Disponible desde CLI 0.1.94:** comprueba primero la versión del ejecutable:

```bash
agentplugins version
```

Necesitas 0.1.94 o posterior para este comportamiento. Los canales pueden actualizarse en momentos distintos; si el tuyo aún ofrece una versión anterior, usa [el instalador nativo](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) o Homebrew. Los ejemplos anteriores con ámbito explícito siguen funcionando con 0.1.93.

`agentplugins install` sin argumento de origen selecciona el ámbito de proyecto si existe un `uap.toml` regular en el directorio actual exacto. No busca en directorios superiores. Durante la selección automática se rechaza un directorio o enlace simbólico llamado `uap.toml`; indica el ámbito explícitamente para resolver la ambigüedad. Un ámbito explícito tiene prioridad; sin ese manifiesto local, se selecciona el entorno de usuario.

Un `uap.lock` existente se valida estrictamente antes de cambiar nada, incluso sin `--locked`: no se vuelven a resolver sus revisiones fijadas. Solo la ausencia del bloqueo permite resolver las dependencias iniciales y crear uno. `--locked` exige un bloqueo válido existente. Usa `update` para avanzar las revisiones fijadas; no uses una instalación ordinaria para cambiarlas. Los ejemplos anteriores con ámbito explícito y `--locked` siguen siendo una forma reproducible de restaurar el proyecto.

## Informa de un problema con claridad

Anota la versión del CLI, operación, ámbito, origen del paquete, clientes y activación pendiente. Indica si el fallo surgió al obtener el origen, planificar, aplicar archivos, recuperar, detectar en el cliente, autenticar o ejecutar. Elimina las credenciales del diagnóstico que compartas. `author doctor ./my-plugin` examina un paquete fuente; `agentplugins doctor` diagnostica el estado gestionado por el instalador.
