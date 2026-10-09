# Essai GUI — carte de zone

Branche : `experiment/gui-zone-map`.

La carte occupe la vue principale. Le joueur apparaît au centre d'un décor
isométrique ; les chemins correspondent exactement aux sorties reçues par LOOK.
Cliquer une destination déclenche MOVE dans la direction correspondante.
Les repères des personnages et objets ouvrent leur catégorie dans **Nearby**,
avec le nom, la description du serveur et les actions disponibles.

La connexion conserve sa présentation. Le chat reste en bas et la fiche
personnage à droite. Le jeu adopte une palette sombre, des indicateurs dorés et
une barre d'actions avec icônes, infobulles et raccourcis **1–5**.

Pour essayer depuis la racine :

```sh
git switch experiment/gui-zone-map
make gui
```

## Carte et interactions

- Carte locale construite depuis les sorties, sans catalogue du monde embarqué.
- Noms et indication de visite mémorisés pendant la connexion puis effacés.
- Carte agrandissable ; Échap la referme et Tab reste dans ses contrôles.
- Personnages, objets et voyageurs consultables dans les onglets Nearby.
- Les déplacements et actions habituelles restent bloqués pendant le combat.
- Les raccourcis n'interceptent pas la saisie de chat, les touches modifiées ou
  les interactions dans les dialogues et la carte agrandie.
- L'illustration est stylisée. Le serveur ne fournit pas de coordonnées ; la
  disposition représente la topologie locale. Les ambiances visuelles suivent
  des mots-clés dans les descriptions, avec une apparence générique en repli.

## Aperçus

Captures du frontend compilé dans WebKitGTK 4.1, avec un backend simulé basé sur
`data/world.json`. Les connexions TCP réelles sont validées séparément par les
tests du client.

- [Carte locale et actions, 1280 × 800](gui-zone-map/world.png)
- [Jardin, inventaire et quête, 1280 × 800](gui-zone-map/garden.png)
- [Petite fenêtre, 480 px](gui-zone-map/narrow.png)

## Validation

`make test-gui` couvre les tests TCP existants, la compilation frontend, les
handlers, la sélection de repères, l'isolation des raccourcis et la remise à zéro.
Les tests de carte vérifient l'orientation, les sorties verticales ou inconnues,
les destinations partagées, les salles sans sortie et les repères d'entités.
`make build-gui` produit le client desktop.

Contrôle WebKit à 1280 et 480 px : connexion, déplacement depuis la carte,
inspection des repères, agrandissement/Échap, dialogues, quêtes, prise/dépôt,
combat, chat, logs en lecture seule, groupe, déconnexion et absence de débordement
horizontal. Les ressources et icônes sont entièrement locales.
