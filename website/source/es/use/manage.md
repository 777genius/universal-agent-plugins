---
title: "Maintain installed plugins"
description: "Traducción no disponible; consulte la página actual en inglés"
canonicalId: "page:use:manage"
section: "use"
locale: "es"
generated: false
translationRequired: true
---

# Maintain installed plugins

[Traducción no disponible; consulte la página actual en inglés](/en/use/manage). La creación de Agent Plugins está disponible mediante Agent Plugins en `universal-agent-plugins`.

[Usar plugins](/es/use/) · [Crear plugins](/es/build/)

## Mantener un proyecto en el código actual

**Solo código fuente; aún no publicado.** `agentplugins install` sin argumento
selecciona el proyecto si hay un archivo regular `uap.toml` en el directorio
actual; no busca en directorios superiores. `--scope` explícito tiene prioridad.
Un `uap.lock` existente se valida estrictamente antes de cambiar nada; solo su
ausencia permite resolver las dependencias iniciales. `--locked` exige ese
archivo. Usa `update` para cambiar las revisiones fijadas. La vista previa muestra
el ámbito y las rutas del acuerdo. Los demás comandos de mantenimiento requieren
seleccionar explícitamente el proyecto:

[Los comandos de mantenimiento están en la sección inglesa](/en/use/manage#maintain-a-project-in-the-current-source).

Guarda los archivos portables `uap.toml` y `uap.lock` en el control de versiones
para compartir dependencias y revisiones. Mantén locales los registros de
propiedad, recuperación y datos de `.uap/`; copia los datos por separado.
La eliminación conserva los datos y la caché compartida. Repair restaura los
componentes ausentes y los paquetes gestionados dañados; los archivos nativos
modificados requieren revisión. Recupera siempre el proyecto seleccionado original.

[Guía de instalación por proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Compatibilidad y límites actuales por proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
