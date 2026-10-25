---
title: "Find and install a plugin"
description: "Traducción no disponible; consulte la página actual en inglés"
canonicalId: "page:use:install"
section: "use"
locale: "es"
generated: false
translationRequired: true
---

# Find and install a plugin

[Traducción no disponible; consulte la página actual en inglés](/en/use/install). La creación de Agent Plugins está disponible mediante Agent Plugins en `universal-agent-plugins`.

[Usar plugins](/es/use/) · [Crear plugins](/es/build/)

## Instalación por proyecto en el código actual

**Solo código fuente; aún no publicado.** Una compilación compatible permite
seleccionar explícitamente un proyecto para paquetes y Agent Skills:

```sh
agentplugins add owner/repo --alias review --target cursor \
  --scope project --project-root /path/to/project
agentplugins skills add owner/repository --ui \
  --scope project --project-root /path/to/project
```

El directorio debe existir. El ámbito predeterminado sigue siendo `user`;
`project` sin `--project-root` selecciona el directorio de ejecución sin buscar
en sus padres. El navegador y la terminal usan el mismo ámbito y muestran solo
los destinos admitidos. Tras preparar los archivos, el cliente puede requerir
reinicio, confianza del espacio de trabajo o una configuración explícita de sesión.

[Guía de instalación por proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Compatibilidad y límites actuales por proyecto](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
