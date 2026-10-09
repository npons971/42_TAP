# 42 TAP — client graphique

Depuis la racine du dépôt :

```sh
make gui
```

Cette commande prépare les outils locaux, installe le frontend depuis son
lockfile, compile le client puis ouvre la fenêtre. Si aucun serveur TAP
n'écoute sur `127.0.0.1:4242`, elle démarre le serveur du dépôt avec
`data/world.json`. Choisissez un pseudo puis cliquez sur **CONNECT & PLAY**.
Le serveur démarré par le lanceur s'arrête à la fermeture du GUI ; un serveur
qui existait déjà continue de fonctionner. Les journaux du serveur local sont
dans `.build/gui-server.log`. Fermer le serveur remet le monde à son état initial.

| Commande | Usage |
|---|---|
| `make gui` ou `make run-gui` | Jouer en local |
| `make gui GUI_PORT=4243` | Utiliser un autre port local |
| `make gui GUI_HOST=192.168.1.10 GUI_PORT=4242 GUI_SERVER=off` | Rejoindre un serveur distant |
| `make gui GUI_SERVER=off` | Ouvrir uniquement le client |
| `make dev-gui` | Développer avec rechargement automatique |
| `make build-gui` | Compiler sans ouvrir de fenêtre |
| `make check-gui` | Diagnostiquer les dépendances locales |
| `make test-gui` | Tests TCP isolés, build frontend et régressions des handlers |

Le binaire est `Project/client-gui/build/bin/tap-gui`. Utilisez `make gui`
pour lui fournir les bibliothèques locales et la configuration du serveur.
`./wails build -tags webkit2_41` et `./wails dev -tags webkit2_41` fonctionnent
également depuis la racine ; ces commandes n'ajoutent pas de serveur TCP.

## Environnement 42

L'installation actuelle cible **Linux x86_64**. Go, Node, npm, Wails et le
compilateur C [Zig](https://ziglang.org/download/index.json) sont installés dans le dépôt par `make install`, avec des
versions fixées et des caches locaux. Aucun droit administrateur n'est requis.
Sur Ubuntu/Debian, GTK3, WebKitGTK 4.1 et pkgconf sont téléchargés depuis les
dépôts officiels, vérifiés par SHA-256 et extraits dans `.webkit-sdk`. Les archives
déjà présentes dans `install_files/webkit` sont réutilisées. Sur Fedora/RHEL,
préparez les archives RPM correspondantes dans ce dossier avant `make install`.
Les bibliothèques natives déjà présentes sur l'hôte sont réutilisées et les
cibles des liens de développement sont copiées dans le SDK.
`make fetch-webkit WEBKIT_FETCH_ARGS=--plan` affiche les archives à récupérer ;
`make fetch-webkit` les récupère sans installation système.
Les outils ne lancent aucune commande d'administration système.
Une session graphique est nécessaire pour ouvrir la fenêtre.

## Commandes de jeu

L'interface propose les déplacements, TAKE/DROP/USE, TALK/QUEST, ATTACK/DEFEND/FLEE,
les quêtes et leur progression, les groupes CREATE/INVITE/JOIN/LEAVE et les chats
GLOBAL/ROOM/GROUP. L'onglet des journaux est en lecture seule. Les commandes
indisponibles en combat sont désactivées ; le serveur vérifie les actions.
Les objets non consommables peuvent refuser USE avec une erreur visible.

Les réponses sont corrélées avec les commandes envoyées. LOOK DETAILS,
INVENTORY DETAILS et TALKJSON fournissent les descriptions depuis le serveur ;
si ces extensions sont refusées, le client utilise LOOK, INVENTORY et TALK.
Aucun catalogue du monde n'est intégré dans le frontend. Les noms connus des
salles sont mémorisés au cours de l'exploration.

La connexion valide le greeting et l'inscription avant d'activer la session,
avec un délai de cinq secondes pour le handshake. Les pseudos suivent les règles
Unicode du serveur : 3–20 lettres minuscules, chiffres ou underscores, commençant
par une lettre minuscule. Les historiques gardent les 500 derniers messages par
onglet. La déconnexion efface l'état de la partie pour la session suivante.
