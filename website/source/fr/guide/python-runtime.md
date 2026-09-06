---
title: "Créer un plugin d'exécution Python"
description: "Un chemin simple de bout en bout pour un plugin repo-local Python."
canonicalId: "page:guide:python-runtime"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Créer un plugin d'exécution Python

Utilisez ce chemin lorsque votre équipe écrit déjà Python et que vous souhaitez que le plugin s'exécute à partir de ce dépôt.

Si vous voulez un binaire compilé et l'histoire de distribution la plus simple, choisissez plutôt Go. Python est le chemin pris en charge lorsque le dépôt lui-même reste l'endroit principal où le plugin est développé et exécuté.

## Choisissez votre chemin Python en 10 secondes

Utilisez le chemin Python par défaut lorsque vous souhaitez le premier dépôt le plus simple :

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
```

Utilisez le chemin du package partagé lorsque vous souhaitez importer `plugin_kit_ai_runtime` à partir de `requirements.txt` sur plusieurs dépôts :

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

Si vous n'êtes pas sûr, commencez par le chemin par défaut.

## Ce que ce chemin vous apporte

- un dépôt de plugin
- Python `3.10+` sur la machine qui exécute le plugin
- un `.venv` local
- un flux Python pris en charge pour `codex-runtime` ou `claude`
- une vérification principale avant validation ou transfert : `validate --strict`

## Si vous voulez seulement le chemin le plus court

Copiez ceci et accédez à la première piste verte :

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

Ne passez à `--runtime-package` qu'une fois que l'exigence de dépendance partagée est réelle.

## 1. Installez le CLI

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

## 2. Créez un projet Python

Pour le chemin normal Python-first Codex :

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
cd my-plugin
```

Si les crochets Claude sont la véritable première exigence, échafaudez plutôt Claude :

```bash
plugin-kit-ai init my-plugin --platform claude --runtime python
cd my-plugin
```

## 3. Préparer l'environnement local Python

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
```

`doctor` vous indique si le dépôt est prêt.

`bootstrap` crée `.venv` si nécessaire et installe `requirements.txt`.

## 4. Générer et valider

```bash
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

`generate` met à jour le lanceur et les fichiers de configuration générés à partir de vos fichiers sources.

Pour un premier dépôt Claude, changez la cible de validation :

```bash
plugin-kit-ai validate . --platform claude --strict
```

## 5. Ajoutez votre logique Python

L'échafaudage par défaut conserve l'assistant local dans `plugin/plugin_runtime.py`, de sorte que la première version reste autonome.

Forme typique du démarreur Codex :

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

Modifiez `plugin/main.py` pour la logique de votre plugin. Gardez stdout réservé aux réponses de l'outil et écrivez les diagnostics uniquement sur stderr.

## 6. Exécutez un test de fumée

Pour le chemin d'exécution Codex :

```bash
plugin-kit-ai test . --platform codex-runtime --event notify
```

Vous pouvez également exécuter directement le lanceur généré :

```bash
./bin/my-plugin notify '{"client":"codex-tui"}'
```

Pour Claude, le contrôle de fumée le plus simple est :

```bash
plugin-kit-ai test . --platform claude --all
```

## 7. Quand utiliser le package Python partagé

Restez sur l'assistant local par défaut lorsque vous souhaitez le premier dépôt le plus simple.

Utilisez le chemin de dépendance partagé lorsque vous souhaitez le même package d'assistance sur plusieurs dépôts :

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

Ce chemin importe [`plugin_kit_ai_runtime`](/fr/api/runtime-python/plugin-kit-ai-runtime) du package [`plugin-kit-ai-runtime`](https://github.com/777genius/plugin-kit-ai/tree/main/python/plugin-kit-ai-runtime) publié au lieu de générer `plugin/plugin_runtime.py`.

Si vous utilisez une version de développement local du CLI à partir de cette arborescence source, transmettez `--runtime-package-version` explicitement pendant `init`.
Les CLI stables publiés déduisent automatiquement la version d'assistance correspondante.

## La règle courte

- choisissez Python lorsque l'équipe est déjà en Python et que le plugin est repo-local
- choisissez Go lorsque vous souhaitez l'histoire d'emballage et de distribution la plus propre
- utiliser `doctor -> bootstrap -> generate -> validate --strict` comme flux normal Python
- passez à `--runtime-package` uniquement lorsque vous souhaitez réellement une dépendance partagée

## Prochaines étapes

- Lisez [Choisir l'environnement d'exécution](/fr/concepts/choosing-runtime) pour connaître les compromis d'exécution.
- Lisez [Choisir le modèle de livraison](/fr/guide/choose-delivery-model) pour connaître la décision entre l'assistance locale et le package partagé.
- Ouvrez [API Python Runtime](/fr/api/runtime-python/) lorsque vous avez besoin de la référence d'assistance.

</details>
