---
title: "Crear un paquete con un Skill"
description: "Prepara un paquete estándar con instrucciones útiles para una tarea repetible."
canonicalId: "page:build:skill"
section: "build"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Crear un paquete con un Skill

**Creación desde el código fuente:** estos comandos requieren la [compilación con creación habilitada](./#build-from-source). No están incluidos en el instalador actual.

Elige este recorrido si el agente necesita un método repetible, una lista de comprobación o instrucciones de redacción y no necesita conectarse a herramientas MCP. El paquete contiene un Skill y un manifiesto raíz. Estos pasos de creación no requieren un servidor, la instalación de un entorno de ejecución ni un perfil de cliente.

## Elige un destino explícito

Trabaja en un directorio padre desechable donde no exista `./review-helper`. Usa una identidad portable en minúsculas. La ruta indica el destino; `--name`, el nombre del paquete en su manifiesto.

```bash
agentplugins author init ./review-helper \
  --template skill \
  --name review-helper \
  --description 'Review documentation changes for clarity and evidence' \
  --skill-name review-docs
```

`init` exige un destino inexistente y no sobrescribe directorios. Si informa de un resultado guardado seguido de un fallo, examina los archivos antes de reintentar. Este recorrido no tiene una opción de sobrescritura.

## Revisa los archivos creados

```text
review-helper/
  plugin.json
  README.md
  .gitignore
  skills/
    review-docs/
      SKILL.md
```

El Skill generado es un punto de partida. Sustituye su cuerpo por un método concreto y útil, manteniendo alineados el nombre y el directorio del Skill. Por ejemplo, el cuerpo de `skills/review-docs/SKILL.md` podría ser:

```markdown
# Revisar cambios de documentación

Usa este Skill para revisar un cambio de documentación antes de entregarlo.

1. Identifica al lector y la tarea que la página le ayuda a completar.
2. Comprueba que cada comando indique sus entradas y describa sus efectos.
3. Distingue el comportamiento verificado de los requisitos y lo no probado.
4. Señala instrucciones confusas y propón una frase que las sustituya.

Devuelve una lista breve de problemas y sus ubicaciones. Si no encuentras
ninguno, indica qué páginas revisaste y qué comprobaciones quedaron fuera.
```

Conserva los metadatos iniciales generados encima del cuerpo. `name` y `description` identifican el Skill y ayudan al agente a decidir cuándo usarlo. Sus instrucciones no conceden permiso para ejecutar scripts ni acceder a recursos privados.

## Comprueba la raíz exacta del paquete

```bash
agentplugins author validate ./review-helper
agentplugins author skills validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

Estas comprobaciones son estáticas: no piden a un agente que siga las instrucciones ni puntúan su respuesta. Revisa la conformidad y la preparación por separado. Un Skill bien formado sigue necesitando revisión humana de su utilidad, precisión y alcance.

Continúa con [más Skills](./skills), [comprobaciones de destinos](./checks) o la [entrega al instalador](./handoff). Para añadir herramientas e instrucciones en un paquete nuevo, sigue el [recorrido híbrido](./hybrid).
