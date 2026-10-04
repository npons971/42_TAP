# GUI rétro — palette de huit couleurs

L'interface utilise huit couleurs contrastées, une police bitmap et des cadres
doubles, de la connexion au chat et aux dialogues. Le fond général et les
panneaux sont ivoire, avec des textes noirs et des titres ivoire sur bleu.
Les textes secondaires, les champs et les messages utilisent également le
noir. Les couleurs restent présentes dans les sprites, la barre de HP,
les cadres de combat et les repères de quête.
Aucun dégradé, police distante ou paquet graphique supplémentaire n'est utilisé.

L'écran titre propose le pseudo, le serveur, le port et **Start**.
L'exploration présente le lieu, une carte de dessus et la description reçue du
serveur. Le terrain est simple : un fond uni, quatre arbres et des
chemins clairs correspondant aux sorties de LOOK. Les personnages et objets
sont des sprites cliquables ; les donneurs de quêtes portent un **!**.

Sous la carte, **People / Objects / Players** restent visibles dans trois
colonnes. Les fiches contiennent les actions habituelles et la sélection d'un
sprite amène le focus à la bonne fiche.

La fiche joueur présente le pseudo, les HP, l'inventaire, les quêtes et un menu
textuel **Look / Status / Quests / Players / Party**. Les raccourcis **1–5**
restent disponibles hors des champs et dialogues. Les canaux du chat sont
**Global / Room / Party / Logs** ; les journaux sont en lecture seule.

## Police locale

**TAP Pixel** est une police originale sur une grille de 5 × 7 pixels,
embarquée dans `frontend/public/fonts/tap-pixel.ttf`. Elle utilise les formes
capitales pour les deux casses ; les pseudos et commandes ne sont pas modifiés.
Les caractères Unicode non couverts utilisent le repli monospace du système.

La reconstruire depuis la racine :

```sh
./node scripts/build-pixel-font.mjs
```

Le générateur utilise uniquement Node local, sans dépendance supplémentaire.
Les huit couleurs de toute l'interface sont définies dans `style.css`.

## Aperçus

- [Exploration, 1280 px](gui-retro/world-1280.png)
- [Exploration, 480 px](gui-retro/world-480.png)
- [Jardin et inventaire](gui-retro/garden-1280.png)
- [Dialogue](gui-retro/dialogue-1280.png)

Captures du frontend compilé dans WebKitGTK 4.1 avec réponses Wails simulées.
La carte représente la topologie locale ; le serveur ne fournit pas de
coordonnées géographiques.

## Vérification

- `make test-gui` : tests TCP du client, compilation Svelte, handlers et carte.
- `make build-gui` : compilation desktop avec les outils locaux.
- Parcours WebKit à 1280 et 480 px : chargement de la police, connexion et pseudo
  invalide, trois colonnes sans débordement, sélection d'objet, agrandissement
  et Échap, dialogue, déplacement, quête, prise/dépôt, blocage en combat,
  chat, logs, groupe et déconnexion.
