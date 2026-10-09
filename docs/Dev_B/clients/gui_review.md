# Revue et corrections du GUI — 2026-10-09

La revue porte sur le client Wails/Svelte, ses échanges TCP et les commandes
Makefile de préparation, build, lancement et test. Les corrections ci-dessous
sont implémentées.

| Problème trouvé | Correction |
|---|---|
| API Svelte 4 utilisée avec Svelte 5 : risque d'écran vide au démarrage | `mount()` dans le point d'entrée |
| Aucun lancement du GUI depuis la racine | `make gui`, `make run-gui`, `make dev-gui`, aide par défaut |
| Wails/npm dépendaient du répertoire courant et des outils globaux | Wrappers locaux, commandes frontend relatives, SDK WebKit 4.1 explicite |
| Compilateur C absent ; pkg-config du SDK inaccessible | Compilateur Zig local vérifié par SHA-256 et PATH du SDK |
| Installation GTK/WebKit dépendant de gestionnaires de paquets système | Téléchargement direct depuis les index officiels Ubuntu/Debian, contrôles SHA-256 et extraction locale |
| Cache partiel, bibliothèques multilib ou SDK d'une ancienne machine | Sélection des archives pour l'hôte amd64, validation dans un dossier temporaire puis remplacement du SDK |
| Processus Go/Vite conservés après interruption de Wails | Groupe de processus du GUI arrêté avec le serveur appartenant au lanceur |
| Script de diagnostic invoquant sudo et proposant des installations globales | Diagnostic limité aux outils du dépôt, code d'échec utile |
| Socket considérée connectée avant le greeting et l'inscription | Handshake validé et borné à cinq secondes, fermeture en cas de refus |
| Pseudo injectant une commande, validation ASCII incohérente avec le serveur | Validation Go et UI : 3–20 points de code Unicode autorisés, port/hôte validés |
| Doubles notifications de déconnexion, état conservé et callback réentrant bloquant | Propriété unique du nettoyage, génération de session, notifications hors verrou de session |
| Objet combat envoyé à ATTACK | Transmission du `target_id` |
| Salle et PNJ obsolètes après combat, fuite ou respawn | Rafraîchissement LOOK/STATUS/QUESTS depuis réponses et événements |
| GROUP ACCEPT inexistant et absence de CREATE | CREATE, INVITE, JOIN, LEAVE et résultat visible dans la fenêtre de groupe |
| Quêtes non chargées ni mises à jour ; actions de quête et de soin absentes | Chargement initial, actualisation, QUEST/USE et progression |
| Actions avec rejets Promise non gérés | Appels centralisés et erreurs visibles ; brouillon de chat conservé en cas d'échec |
| Saisie dans Logs publiée en GLOBAL | Journaux en lecture seule |
| Rafraîchissements en double et historique illimité | Rafraîchissements regroupés sur 50 ms ; limite de 500 messages par onglet |
| État du monde embarqué et ressources de template inutiles | Monde fourni par le serveur, noms appris à l'exploration, retrait du logo et de la police inutilisés |
| Interface coupée sur petites fenêtres | Mise en page adaptable, défilement et actions désactivées en combat |
| Tests utilisant le serveur utilisateur sur le port 4242 | Serveur de test isolé sur un port éphémère |

## Vérification

- `make test-gui` : tests TCP avec détecteur de concurrence, build Svelte et
  régressions des handlers du frontend.
- `make test` : tests Go de l'ensemble serveur/CLI avec détecteur de concurrence.
- `make build-gui` : compilation native Linux avec GTK3/WebKitGTK 4.1 locaux.
- `make check-gui` : diagnostic des outils locaux.
- Lancements de contrôle de `make gui` et `make dev-gui` sur ports temporaires :
  processus GUI lancé, Vite prêt en développement, arrêt des processus à la fin
  du contrôle. Le contrôle vérifie le démarrage ; il ne remplace pas un parcours
  visuel complet joué à la souris.

## Portabilité

Les archives Go/Node/Zig de ce Makefile ciblent Linux x86_64. Ubuntu/Debian
utilisent les index officiels de leur version pour récupérer localement les
headers, pkgconf et les dépendances manquantes, puis vérifient les archives par
SHA-256. Les bibliothèques déjà présentes sur l'hôte sont réutilisées ; les cibles
des liens de développement sont copiées dans le SDK. Le chemin est vérifié sur
Ubuntu 24.04. Sur Fedora/RHEL, les archives RPM doivent être préparées localement.
Une session graphique reste nécessaire pour ouvrir la fenêtre.
