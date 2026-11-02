---
title: "Entregar un paquete al planificador de instalación"
description: "Entrega el origen y sus pruebas manteniendo una decisión de instalación explícita."
canonicalId: "page:build:handoff"
section: "build"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Entregar un paquete al planificador de instalación

Una entrega útil reúne el directorio de un paquete estándar y una explicación clara de lo comprobado. El instalador receptor crea su propio plan para los agentes y el ámbito elegidos. Las herramientas de creación compiladas desde el código fuente pueden aportar pruebas, pero no son necesarias en la máquina receptora.

## Prepara el paquete y sus pruebas

Selecciona el directorio con `plugin.json` en la raíz, en lugar de un repositorio padre ambiguo. Incluye:

- Identidad y versión del paquete, y revisión exacta o copia revisada del origen.
- Raíz relativa del paquete y finalidad de sus Skills y servidores MCP.
- Resultados de validación e inspección, con la versión o revisión de la herramienta.
- Identificadores de clientes previstos y compatibilidad por componente.
- Requisitos de ejecución, autenticación y trabajo todavía sin evaluar.
- Cualquier prueba de ejecución explícita, con servidor, herramienta y plataforma.

La versión del paquete y la del CLI describen cosas distintas. Mantén esa distinción. Evita credenciales y rutas absolutas innecesarias en diagnósticos compartidos.

## Completa las comprobaciones de creación disponibles

Estos comandos opcionales requieren la [compilación con creación habilitada](./#build-from-source), distinta del instalador publicado actual:

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
agentplugins author compat ./review-helper --target codex
agentplugins author doctor ./review-helper
```

La secuencia es estática: no ejecuta código del paquete ni examina el perfil de un usuario real. Registra por separado [las pruebas de ejecución MCP](./checks) que hayas hecho. Una prueba directa correcta tampoco demuestra detección por el cliente nativo ni OAuth.

## Pide un plan al instalador publicado

```bash
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

El instalador puede descargar metadatos de seguridad o su analizador fijado incluso para un origen local. Su validación y su vista previa son distintas de las comprobaciones de creación sin conexión.

El primer comando comprueba la lectura del paquete por el instalador. El segundo selecciona Codex y previsualiza la instalación sin aplicar cambios gestionados. Para otro cliente, elige un destino admitido según los resultados de compatibilidad. Para planificar un proyecto, añade `--scope project --project-root /path/to/project` explícitamente.

Comprueba identidad, origen, destinos, hallazgos de seguridad y activación pendiente. Resuelve las diferencias respecto a lo previsto por el autor antes de aplicar. Un plan no es un recibo de instalación realizada.

## Comparte un origen reproducible

Para distribuir a distancia, usa un origen admitido por el CLI receptor: un paquete del directorio firmado, un origen GitHub exacto o un origen Git o archivo comprimido directo compatible. Consulta [los orígenes de instalación](../use/install) para conocer sintaxis y disponibilidad. Usa una revisión inmutable o un archivo versionado duradero si necesitas restauración exacta. Un directorio local exige una transferencia deliberada separada.

El CLI de creación desde el código fuente no expone comandos de publicación de paquetes, exportación de bundles ni envío al directorio. Alojar el origen es distinto de validarlo. Las declaraciones de `uap.toml` pertenecen al contrato del gestor UAP; `plugin.json` raíz sigue definiendo la identidad del paquete portable.

## Explica lo pendiente

| Prueba entregada | Trabajo pendiente |
| --- | --- |
| Conformidad estándar | Instrucciones útiles y herramientas correctas. |
| Comprobaciones estáticas de creación | Aprobación de publicación y verificación de canales públicos. |
| Compatibilidad estática del cliente | Detección y activación nativas efectivas. |
| Prueba MCP directa, si se hizo | Otras herramientas, clientes, autenticación y entornos no probados. |
| Vista previa del instalador | Aplicar los cambios revisados y completar la configuración del cliente. |

El receptor puede seguir [Usar plugins](../use/install) cuando quiera instalar. Conserva junto al paquete las instrucciones de confianza y activación de cada cliente.
