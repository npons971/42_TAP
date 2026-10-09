# TAP — Novanns (Dev B) : client GUI

Répartition de référence : [organisation à trois](../Commun/TAP_taches_communes.md). Autres pôles : [réseau et protocole — Dev A](../Dev_A/TAP_Aris_Dev_A.md), [moteur, monde et CLI — Dev C](../Dev_C/TAP_Dev_C.md).

## Responsabilités

- Client graphique Wails v2 + Svelte et connexion TCP au serveur.
- Envoi des commandes RFC, réception asynchrone des réponses et événements, déconnexions et reconnexions.
- Affichage des salles, descriptions, sorties, objets, PNJ, joueurs présents et compteurs de joueurs.
- Inventaire et actions TAKE/DROP par ID ou nom, avec actualisation automatique.
- Dialogues TALK, HP, combat, respawn, suivi des quêtes et récompenses.
- Contrôles LOOK, MOVE, TAKE, DROP, TALK, ATTACK, STATUS, QUEST, QUESTS, WHO, GROUP et QUIT ; extensions documentées du jeu.
- Chats GLOBAL, ROOM et GROUP séparés du journal d'événements.
- Tests du backend TCP GUI, handlers frontend, parcours visuels et réactivité pendant les événements.
- Documentation GUI, lancement, dépendances locales et tests associés.

## Coordination

Dev A fournit le contrat réseau et les événements. Dev C fournit la logique de jeu et pilote les données du monde. Novanns participe aux choix de gameplay et de contenu avec l'équipe et vérifie leur présentation dans l'interface. Le serveur reste la source de vérité.

## Travail existant et passation

Novanns a conçu le monde initial et les objectifs narratifs des quêtes, développé la GUI, ses vues, ses actions et son intégration TCP. Le monde comporte neuf salles, deux boucles et deux branches. La conception passée reste attribuée à Novanns ; le suivi du monde et du moteur revient désormais à Dev C.

Le prototype, les interactions d'objets et PNJ, les commandes sociales, le suivi des quêtes et les contrôles ATTACK/DEFEND/FLEE sont implémentés. Les tests multijoueur visuels et la validation des déconnexions/reconnexions restent à finaliser, puis la documentation et la préparation à l'évaluation.

## Références

- [Client GUI](clients/gui_client.md)
- [Revue GUI](clients/gui_review.md)
- [GDD partagé, piloté par Dev C](../Commun/gdd/README.md)
- [Jalons et tests communs](../Commun/TAP_taches_communes.md)
