---
title: "Crear un paquete MCP remoto"
description: "Describe un servicio MCP existente y distingue configuración, conectividad y autenticación."
canonicalId: "page:build:mcp-remote"
section: "build"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Crear un paquete MCP remoto

**Creación desde el código fuente:** estos comandos requieren la [compilación con creación habilitada](./#build-from-source). No están incluidos en el instalador actual.

Elige MCP remoto si el servidor ya existe y el paquete debe describir cómo se conecta el agente. La creación registra la URL sin contactarla. Antes de entregar el paquete, debes conocer el transporte y los requisitos de autenticación del servicio.

## Indica el endpoint explícitamente

`https://mcp.example.com/mcp` es un ejemplo de documentación. Sustitúyelo por el endpoint MCP real de tu servicio. El comando acepta la URL como configuración; no comprueba que sirva MCP ni que tu cuenta pueda usarlo.

```bash
agentplugins author init ./docs-service \
  --template mcp-remote \
  --name docs-service \
  --description 'Connect to the team documentation MCP service' \
  --url https://mcp.example.com/mcp
```

El destino debe ser inexistente y estar dentro de un directorio padre existente. La plantilla exige una URL HTTPS absoluta, o HTTP en la interfaz de bucle local. No admite credenciales, fragmentos ni marcadores URL sin resolver como entradas. No incluyas una contraseña en la URL.

## Entiende los archivos

```text
docs-service/
  plugin.json
  mcp.json
  README.md
  .gitignore
```

El manifiesto raíz define la identidad del paquete. `mcp.json` describe el servidor bajo `mcpServers`; esta plantilla usa `type: streamable-http` y la URL indicada. No genera un servidor ni añade un Skill. Usa [un paquete híbrido](./hybrid) si también necesitas instrucciones sobre cuándo llamar a sus herramientas.

Revisa el README e indica la finalidad del endpoint, los requisitos de cuenta y dónde consultar la autenticación. Mantén los secretos fuera del paquete. Una URL en un archivo conforme no demuestra que el servicio funcione.

## Comprueba la configuración y los destinos

```bash
agentplugins author validate ./docs-service
agentplugins author inspect ./docs-service --target codex,claude
agentplugins author test ./docs-service
agentplugins author compat ./docs-service --target codex,claude
agentplugins author doctor ./docs-service
```

`inspect` muestra componentes y requisitos pendientes. `compat` evalúa la compatibilidad estática de los adaptadores de los clientes elegidos. Ninguno necesita esos clientes instalados. `doctor` no contacta el endpoint. Para realizar una conexión MCP explícita desde el código fuente, consulta [las pruebas de ejecución](./checks#evaluate-mcp-runtime-explicitly); un servidor remoto requiere `--allow-network`.

## Explica el trabajo pendiente

Las pruebas estáticas deben identificar el transporte y los clientes previstos. El instalador receptor todavía evalúa su propia política y muestra instrucciones de activación. Puede ser necesario iniciar sesión, aprobar acceso o abrir una nueva sesión. La detección y las llamadas a herramientas durante la ejecución necesitan pruebas separadas.

Si falla una comprobación estática, corrige primero la configuración registrada. Si más tarde no se alcanza el endpoint, investiga el servicio o la conexión del cliente; otra prueba estática correcta no demuestra conectividad.

Continúa con [la interpretación de informes](./checks) y [la entrega al planificador](./handoff).
