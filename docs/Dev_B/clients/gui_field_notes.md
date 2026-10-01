# Essai GUI — Field Notes

Branche : `experiment/gui-field-notes`.

Une ambiance de carnet d'exploration : papier ivoire, vert forêt, accents dorés,
titres en serif et rose des vents. L'accueil présente l'univers à gauche et la
connexion à droite. En jeu, le lieu et ses chemins occupent la vue principale ;
la fiche personnage rassemble vitalité, inventaire et quêtes. Les conversations
World, Room, Party et les journaux système gardent leurs onglets séparés.

Les composants utilisent les polices disponibles sur la machine et une
illustration SVG locale. Aucune dépendance supplémentaire ni ressource distante.
Les actions conservent les appels au backend Wails et les données du serveur.
Les panneaux défilent si leur contenu dépasse la fenêtre ; à 740 px et moins,
l'interface passe sur une colonne.

Pour essayer depuis la racine du dépôt :

```sh
git switch experiment/gui-field-notes
make gui
```

## Captures

Captures du frontend compilé dans WebKitGTK 4.1, avec réponses Wails simulées à
partir de `data/world.json`. Elles illustrent le rendu ; la validation TCP est
couverte séparément par les tests du client.

- [Accueil, 1280 × 800](gui-field-notes/arrival.png)
- [Jeu avec inventaire et quête, 1280 × 800](gui-field-notes/adventure.png)
- [Vue étroite, 480 px](gui-field-notes/narrow.png)

## Validation

- `make test-gui` : tests TCP du client, compilation frontend et régressions
  des handlers réussis.
- `make build-gui` : binaire desktop compilé et empaqueté.
- Contrôle WebKit en 1280 × 800 et 480 × 800 : connexion, validation du pseudo,
  affichage du lieu, déplacements, dialogue, quête, prise/dépôt d'objet,
  état de combat, envoi de chat, journaux en lecture seule, ouverture du groupe
  et déconnexion. Illustrations chargées hors ligne, aucun débordement horizontal.

Cet essai est destiné à valider la direction visuelle avant intégration.
