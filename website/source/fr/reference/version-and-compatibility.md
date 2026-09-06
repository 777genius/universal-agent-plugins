---
title: "Politique de version et de compatibilité"
description: "Comment penser aux versions, aux promesses de compatibilité, aux wrappers, aux SDK et au vocabulaire de support dans plugin-kit-ai."
canonicalId: "page:reference:version-and-compatibility"
section: "reference"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Politique de version et de compatibilité

Cette page est destinée à une décision pratique d'équipe : que normalisons-nous et quelle est la force de cette promesse ?

## Choisissez en 60 secondes

- lisez cette page lorsque votre équipe a besoin d'une politique compacte pour les versions, les wrappers, les SDK, les environnements d'exécution et les promesses de compatibilité
- lisez [Limite de support](/fr/reference/support-boundary) lorsque vous souhaitez la réponse d'assistance pratique la plus courte
- lisez [Versions](/fr/releases/) lorsque vous voulez l'histoire d'une version spécifique

## La référence publique

Pensez à la normalisation en trois niveaux :

- la ligne de version que vous choisissez parmi les dépôts
- le niveau de support du chemin que vous choisissez à l'intérieur de cette ligne de version
- le mécanisme d'installation ou de livraison autour de ce chemin

Ces couches sont liées, mais elles ne sont pas interchangeables.

## Voies recommandées et niveaux formels

Utilisez une traduction simple dans les documents et les règles :

- `Recommended` signifie généralement un chemin de production promu `public-stable`
- `Advanced` signifie une surface supportée avec un contrat plus étroit ou plus spécialisé
- `Experimental` signifie une désabonnement opt-in en dehors des attentes normales de compatibilité

Les principaux chemins recommandés aujourd’hui sont :

- `Codex runtime Go`
- `Codex package`
- `Gemini packaging`
- `Gemini Go runtime`
- `Claude default stable lane`
- Chemins d'exécution locaux `Python` et `Node` comme choix de création non-Go pris en charge et recommandé sur les cibles prises en charge

## Ce que couvre réellement la compatibilité ici

La promesse publique la plus forte concerne :

- le contrat public CLI déclaré
- le chemin recommandé Go SDK et les chemins de production recommandés listés ci-dessus
- les chemins d'exécution locaux recommandés Python et Node sur les cibles prises en charge
- le comportement documenté des sorties générées par `public-stable`

La compatibilité ne signifie pas que chaque emballage, chemin de commodité ou surface spécialisée se déplace avec la même promesse.

## Langage public et termes formels

Utilisez cette traduction lorsque vous parlez à une équipe :

- `Recommended` signifie généralement que le chemin se trouve à l'intérieur du contrat `public-stable` actuel le plus fort.
- `Advanced` signifie que la surface est prise en charge, mais plus spécialisée ou plus étroite que la première valeur par défaut
- `Experimental` signifie une désabonnement opt-in sans attente normale de compatibilité

Lorsque l'équipe a besoin d'une politique précise, utilisez les termes formels `public-stable`, `public-beta` et `public-experimental`.

## Wrappers, SDKs et runtime APIs

Ne les standardisez pas comme s’il s’agissait de la même chose.

- Homebrew, npm, PyPI et le script vérifié sont des canaux d'installation pour CLI
- la Go SDK est une surface publique SDK
- les API d'exécution sont liés à leurs chemins d'exécution déclarés

Si vous traitez les wrappers d'installation comme s'ils portaient la même promesse qu'un SDK ou un chemin d'exécution, vous standardiserez la mauvaise couche.

## Ce que les équipes devraient normaliser

Les équipes saines standardisent généralement :

- une référence de version déclarée
- un chemin principal avec une histoire de support claire
- une porte de validation avant le transfert et le déploiement
- une interprétation partagée des termes formels de compatibilité

## Règle finale

Standardisez uniquement la ligne de publication et le chemin dont votre équipe est réellement prête à défendre la promesse publique lors de l'IC, du transfert et du déploiement.

</details>
