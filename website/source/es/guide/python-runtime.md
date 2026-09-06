---
title: "Cree un complemento de tiempo de ejecución Python"
description: "Una ruta simple de un extremo a otro para un complemento de repositorio local Python."
canonicalId: "page:guide:python-runtime"
section: "guide"
locale: "es"
generated: false
translationRequired: true
---

[Usar plugins](/es/use/) · [Crear plugins](/es/build/) — Preparación, no es un lanzamiento. La migración de proyectos a v2 aún no está disponible.
<p class="locale-historical-identity">Cree un complemento de tiempo de ejecución Python</p>

[Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.](/en/legacy/v1/)

<details><summary>Instantánea histórica de la rama plugin-kit-ai v1. 1.2.4 es la referencia de comandos, no la versión exacta de esta instantánea.</summary>

# Cree un complemento de tiempo de ejecución Python

Utilice esta ruta cuando su equipo ya escriba Python y desee que el complemento se ejecute desde este repositorio.

Si desea un binario compilado y la historia de distribución más sencilla, elija Go en su lugar. Python es la ruta admitida cuando el repositorio en sí sigue siendo el lugar principal donde se desarrolla y ejecuta el complemento.

## Elige tu ruta Python en 10 segundos

Utilice la ruta predeterminada Python cuando desee el primer repositorio más simple:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
```

Utilice la ruta del paquete compartido cuando desee importar `plugin_kit_ai_runtime` desde `requirements.txt` en varios repositorios:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

Si no está seguro, comience primero con la ruta predeterminada.

## Lo que te brinda este camino

- un repositorio de complementos
- Python `3.10+` en la máquina que ejecuta el complemento
- un local `.venv`
- un flujo Python admitido para `codex-runtime` o `claude`
- una verificación principal antes de confirmar o traspasar: `validate --strict`

## Si solo quieres el camino más corto

Copie esto y llegue a la primera pista verde:

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
cd my-plugin
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
plugin-kit-ai test . --platform codex-runtime --event notify
```

Solo cambie a `--runtime-package` después de que el requisito de dependencia compartida sea real.

## 1. Instale el CLI

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

## 2. Proyecto Andamio A Python

Para la ruta normal Python-primera Codex:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
cd my-plugin
```

Si los ganchos de Claude son el requisito principal real, utilice el andamio de Claude en su lugar:

```bash
plugin-kit-ai init my-plugin --platform claude --runtime python
cd my-plugin
```

## 3. Prepare el entorno local Python

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
```

`doctor` le indica si el repositorio está listo.

`bootstrap` crea `.venv` cuando es necesario e instala `requirements.txt`.

## 4. Generar y validar

```bash
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

`generate` actualiza el iniciador generado y los archivos de configuración a partir de sus archivos fuente.

Para un primer repositorio Claude, cambie el destino de validación:

```bash
plugin-kit-ai validate . --platform claude --strict
```

## 5. Agregue su lógica Python

El andamio predeterminado mantiene el asistente local en `plugin/plugin_runtime.py`, por lo que la primera versión permanece autónoma.

Forma típica de arranque Codex:

```python
from plugin_runtime import CodexApp, continue_

app = CodexApp()


@app.on_notify
def on_notify(event):
    _ = event
    return continue_()


if __name__ == "__main__":
    raise SystemExit(app.run())
```

Edite `plugin/main.py` para la lógica de su complemento. Mantenga stdout reservado para respuestas de herramientas y escriba diagnósticos solo en stderr.

## 6. Realice una prueba de humo

Para la ruta de tiempo de ejecución Codex:

```bash
plugin-kit-ai test . --platform codex-runtime --event notify
```

También puedes ejecutar el lanzador generado directamente:

```bash
./bin/my-plugin notify '{"client":"codex-tui"}'
```

Para Claude, la comprobación de humo más sencilla es:

```bash
plugin-kit-ai test . --platform claude --all
```

## 7. Cuándo utilizar el paquete Python compartido

Manténgase en el asistente local predeterminado cuando desee el primer repositorio más simple.

Utilice la ruta de dependencia compartida cuando desee el mismo paquete auxiliar en varios repositorios:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

Esa ruta importa [`plugin_kit_ai_runtime`](/es/api/runtime-python/plugin-kit-ai-runtime) del paquete publicado [`plugin-kit-ai-runtime`](https://github.com/777genius/plugin-kit-ai/tree/main/python/plugin-kit-ai-runtime) en lugar de generar `plugin/plugin_runtime.py`.

Si está utilizando una compilación de desarrollo local de CLI de este árbol de fuentes, pase `--runtime-package-version` explícitamente durante `init`.
Los CLIs estables publicados infieren automáticamente la versión auxiliar correspondiente.

## La regla corta

- elija Python cuando el equipo ya esté en Python y el complemento sea local de repositorio
- elija Go cuando desee la historia de embalaje y distribución más limpia
- use `doctor -> bootstrap -> generate -> validate --strict` como flujo normal Python
- cambie a `--runtime-package` solo cuando realmente desee una dependencia compartida

## Próximos pasos

- Lea [Elección del tiempo de ejecución](/es/concepts/choosing-runtime) para conocer las compensaciones del tiempo de ejecución.
- Lea [Elegir modelo de entrega](/es/guide/choose-delivery-model) para conocer la decisión entre ayuda local y paquete compartido.
- Abra [Python Runtime API](/es/api/runtime-python/) cuando necesite la referencia de ayuda.

</details>
