# 42 TAP — client graphique

Depuis la racine du dépôt :

```sh
make gui
```

Cette commande prépare les outils locaux, installe le frontend depuis son
lockfile, compile le client puis ouvre la fenêtre. Si aucun serveur TAP
n'écoute sur `127.0.0.1:4242`, elle démarre le serveur du dépôt avec
`data/world.json`. Choisissez un pseudo puis cliquez sur **Start**.
Le serveur démarré par le lanceur s'arrête à la fermeture du GUI ; un serveur
qui existait déjà continue de fonctionner. Les journaux du serveur local sont
dans `.build/gui-server.log`. Fermer le serveur remet le monde à son état initial.

| Commande | Usage |
|---|---|
| `make gui` ou `make run-client-gui` | Jouer en local |
| `make gui GUI_PORT=4243` | Utiliser un autre port local |
| `make gui GUI_HOST=192.168.1.10 GUI_PORT=4242 GUI_SERVER=off` | Rejoindre un serveur distant |
| `make gui GUI_SERVER=off` | Ouvrir uniquement le client |
| `make dev-gui` | Développer avec rechargement automatique |
| `make build-gui` | Compiler sans ouvrir de fenêtre |
| `make check-gui` | Diagnostiquer les dépendances locales |
| `make test` | Tests serveur/CLI/GUI, build frontend et régressions des handlers |
| `make lint` | Formatage Go, analyse Go et contrôles JavaScript/Svelte |

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

Le GUI utilise huit couleurs rétro contrastées sur un fond ivoire, une police
bitmap locale et des cadres doubles. Le login est un écran titre avec le
pseudo, le serveur, le port et le bouton **Start**. Les menus sont textuels,
avec un curseur et les raccourcis indiqués à droite.
La vue principale affiche une carte locale des sorties de la salle, avec votre
position au centre. La carte adopte une vue de dessus inspirée des anciens
Pokémon : terrain uni, chemins clairs et quelques arbres. Les personnages
et objets apparaissent sous forme de petits sprites ; un point d'exclamation
signale les donneurs de quêtes. Les personnages, objets et joueurs sont présentés dans
trois colonnes simultanées sous la carte, sans onglets. Cliquez sur une
destination pour vous déplacer ou sur un repère pour sélectionner et atteindre
la fiche d'un personnage ou d'un objet. La carte peut
être agrandie et refermée avec Échap. Elle utilise les données de LOOK ; le
décor est stylisé, les positions ne sont pas des coordonnées géographiques.
Les noms des lieux visités sont mémorisés pendant la session.

Le **Menu** propose LOOK, STATUS, QUESTS, WHO et GROUP avec
infobulles. Les touches **1–5** déclenchent les mêmes actions hors
champs de saisie, dialogues et carte agrandie.

L'interface propose les déplacements, TAKE/DROP/USE, TALK/QUEST, ATTACK/DEFEND/FLEE,
les quêtes et leur progression, les groupes CREATE/INVITE/JOIN/LEAVE et les chats
GLOBAL/ROOM/GROUP. L'onglet des journaux est en lecture seule. Les commandes
indisponibles en combat sont désactivées ; le serveur vérifie les actions.
Les pseudos ciblés par INVITE/JOIN sont convertis en minuscules : `aris`,
`ARIS` et `ArIs` désignent le même joueur, dans le GUI comme dans le CLI.
Les erreurs de chat s'affichent dans le canal de la commande refusée, même si
l'utilisateur a changé d'onglet. Les erreurs de gestion des groupes s'affichent
dans GROUP ; les autres erreurs restent dans l'alerte et les journaux.
Le dialogue de gestion du groupe affiche uniquement ses propres erreurs.
Les rafraîchissements automatiques conservent l'erreur visible jusqu'à la
prochaine action de l'utilisateur.
Dans l'inventaire, **USE** est proposé uniquement pour les consommables de soin,
hors combat et lorsque des PV manquent. Les autres objets proposent **INFO** :
description, bonus d'attaque/défense et explication de leur usage. Les bonus
d'équipement s'appliquent automatiquement tant que l'objet est porté ; seul le
meilleur bonus de chaque statistique compte. INFO reste accessible en combat et
depuis les objets au sol. La fiche se ferme avec son bouton ou Échap.
Healing Salve remplace la torche sans effet et soigne de 25 PV ; la quête du
bandit récompense désormais avec Captain's Shield (+8 défense).

Le journal **Quests** affiche aussi les quêtes disponibles : titre, description,
donneur et lieu, récompense, puis les deux étapes (collecter ou vaincre, puis
livrer ou faire son rapport). Les étapes cochées suivent la possession actuelle
des herbes ou la victoire personnelle ; la livraison reste à faire tant que la
récompense n'a pas été reçue. **Next** indique l'action suivante et son lieu.
**Accept quest** et **Claim reward** apparaissent lorsque le donneur est présent,
et sont désactivés en combat. Les récompenses sont uniques dans le monde partagé ;
le journal explique quand un autre joueur a obtenu une récompense ou porte les
herbes recherchées. Les descriptions et dialogues du monde annoncent le retour
au donneur, y compris après la victoire sur le bandit.

Les chats suivent leur portée GLOBAL/ROOM/GROUP. Les présences, combats,
objets et dialogues de PNJ s'affichent dans ROOM ; les invitations, adhésions,
départs et changements de chef dans GROUP. STATS actualise le compteur du
serveur. Les réponses de données actualisent uniquement le panneau de leur
commande. Chaque réponse ou événement est journalisé une fois, y compris les
OK sans contenu ; les événements inconnus ou incomplets restent dans les journaux.

Les réponses sont corrélées avec les commandes envoyées. LOOK DETAILS,
INVENTORY DETAILS, QUESTS DETAILS et TALKJSON fournissent les descriptions depuis
le serveur ; si ces extensions sont refusées, le client utilise LOOK, INVENTORY,
QUESTS et TALK. Avec QUESTS seul, les identifiants sont convertis en noms lisibles
et le journal invite à consulter le donneur pour les instructions manquantes.
Aucun catalogue du monde n'est intégré dans le frontend. Les noms connus des
salles sont mémorisés au cours de l'exploration.

La connexion valide le greeting et l'inscription avant d'activer la session,
avec un délai de cinq secondes pour le handshake. Les pseudos suivent les règles
Unicode du serveur : 3–20 lettres minuscules, chiffres ou underscores, commençant
par une lettre minuscule. Les historiques gardent les 500 derniers messages par
onglet. La déconnexion efface l'état de la partie pour la session suivante.
