# TAP — Organisation à trois et tâches communes

La répartition de référence repose sur trois pôles. Chaque membre assure le développement, les tests et la documentation de son périmètre.

| Membre | Pôle | Responsabilités |
|---|---|---|
| **Aris (Dev A)** | Serveur réseau et protocole | TCP, sessions, parser RFC, réponses et événements, commandes sociales, concurrence, déconnexions, logs et détection d'abus |
| **Novanns (Dev B)** | Client GUI | Connexion TCP, réception asynchrone, vues et actions de jeu, chats, groupes, compteurs et actualisation automatique |
| **Alexis (Dev C)** | Moteur de jeu, monde et CLI | État du jeu, chargement et validation du monde, exploration, objets, PNJ, combat, quêtes, récompenses et client CLI |

Fiches détaillées : [Dev A](../Dev_A/TAP_Aris_Dev_A.md), [Dev B](../Dev_B/TAP_Novanns_Dev_B.md), [Dev C](../Dev_C/TAP_Dev_C.md).

Cette organisation définit les responsabilités pour la suite du projet. Elle ne réattribue pas les contributions passées : Aris a déjà implémenté le serveur, le moteur et le CLI ; Novanns a conçu le monde initial et développé la GUI. Dev C reprend le suivi du moteur, du monde et du CLI après une passation.

## Architecture et frontière entre Dev A et Dev C

Le serveur reste la source de vérité. Les clients affichent ses réponses et événements sans maintenir un monde indépendant.

- **Dev A** reçoit les lignes TCP, vérifie leur syntaxe RFC, gère les sessions et route les commandes. Il encode et envoie les réponses et événements vers leurs destinataires.
- **Dev C** vérifie les règles de jeu, applique les mutations de l'état et produit les résultats et événements à transmettre. Il charge et valide les données du monde.
- **Dev B** traduit les actions de l'interface en commandes et actualise les vues à partir des réponses et événements.

Dev A pilote `CONNECT`, `CHAT`, `WHO`, `GROUP` et `QUIT`. Dev C pilote la logique de `LOOK`, `MOVE`, `TAKE`, `DROP`, `INVENTORY`, `TALK`, `ATTACK`, `STATUS`, `QUEST` et `QUESTS`, ainsi que les extensions de gameplay documentées (`USE`, `DEFEND`, `FLEE`, etc.). Dev A assure la conformité des trames pour toutes les commandes.

Dev A et Dev C définissent ensemble les structures, le contrat de routage, les destinataires et l'ordre des événements, ainsi que la protection de l'état partagé. Les transferts d'objets, combats et récompenses doivent rester atomiques. Le réseau ne doit pas bloquer l'état du jeu ; une déconnexion doit nettoyer le joueur avant la diffusion du départ.

Le code actuel utilise un état partagé dans `internal/server`. Cette frontière décrit les responsabilités ; elle ne suppose pas qu'une séparation physique des modules soit déjà réalisée. Tout changement de cette architecture fait l'objet d'une revue croisée A/C.

## Décisions communes

Les trois membres lisent le RFC et définissent les conventions Git, les identifiants, les structures et le format du monde. Le contrat est consigné dans [rfc_syntax.md](protocol/rfc_syntax.md), [json_payloads.md](protocol/json_payloads.md) et le [rapport de conformité](protocol/rfc_conformance.md).

Dev C pilote la conception du monde, du combat et des quêtes avec la participation des trois membres. Dev B vérifie que les informations et actions sont accessibles dans la GUI ; Dev A vérifie leur compatibilité avec le protocole et le multijoueur.

Le monde doit proposer au moins huit salles interconnectées avec une boucle et une branche optionnelle, trois rôles de PNJ, quatre objets dont deux récupérables et deux quêtes. Le fichier actif est `data/world.json` ; `Project/data/world.yaml` conserve la proposition initiale de Novanns.

## Tests et passation

- Dev A : syntaxe RFC, erreurs, connexions, événements, commandes sociales, logs, déconnexions et clients lents.
- Dev C : validation du monde, objets uniques, combat, respawn, progression et récompenses des quêtes, CLI et réception pendant la saisie.
- Dev B : réponses et événements GUI, actions, actualisation des vues, chats, déconnexions et reconnexions.
- Ensemble : sessions CLI/GUI simultanées, concurrence sur objets et récompenses, scénarios complets, interopérabilité avec une autre équipe et préparation à l'évaluation.

La passation vers Dev C s'appuie sur le code existant, le [GDD](gdd/README.md), la [documentation CLI](../Dev_A/clients/cli_client.md) et la [matrice de vérification existante](testing/aris_acceptance.md). Chaque membre doit pouvoir expliquer l'ensemble du projet après relecture croisée.

## Répartition du README

Le README est rédigé en anglais.

| Responsable | Sections |
|---|---|
| Dev A | Architecture réseau, Protocol Implementation, Server Logging, Building and Running pour le serveur, tests réseau |
| Dev C | Architecture du moteur, Combat System, Quest System, World Design, Building and Running pour le CLI, tests gameplay et CLI |
| Dev B | GUI, Building and Running pour la GUI, tests GUI |
| Ensemble | Description, Resources et utilisation de l'IA, Group Contributions, outillage local, Testing final et relecture |

La section Group Contributions distingue les responsabilités actuelles des contributions effectivement réalisées.

## Jalons d'intégration

1. **Cadrage et passation** : contrat entre modules, RFC, structures, synchronisation et prise en main du code existant.
2. **Premier parcours commun** : deux joueurs, un CLI et une GUI, se connectent, regardent une salle, se déplacent et discutent. Dev A fournit les communications, Dev C le monde minimal et le CLI, Dev B les vues correspondantes.
3. **Objets et PNJ** : TAKE/DROP/INVENTORY/TALK, actualisation GUI et absence de duplication en multijoueur ; commandes sociales intégrées.
4. **Combat et quêtes** : règles et moteur par Dev C, trames et diffusion par Dev A, affichage et actions par Dev B.
5. **Validation finale** : robustesse, logs, déconnexions, concurrence, interopérabilité, README et soutenance.

Ces jalons servent aussi à vérifier le travail déjà implémenté ; ils ne remettent pas son état d'avancement à zéro.
