# TAP — Alexis (Dev C) : moteur de jeu, monde et client CLI

**Responsable : Alexis (Dev C).** Répartition de référence : [organisation à trois](../Commun/TAP_taches_communes.md). Autres pôles : [réseau et protocole — Aris](../Dev_A/TAP_Aris_Dev_A.md), [GUI — Novanns](../Dev_B/TAP_Novanns_Dev_B.md).

## Responsabilités

- État du jeu : positions, inventaires, objets, PNJ, HP, engagements et progression des quêtes.
- Chargement de `data/world.json` et validation des sorties, références et placements uniques.
- Exploration : logique de LOOK et MOVE.
- Objets : TAKE/DROP/INVENTORY, instances uniques, noms composés, recherche par ID ou nom et absence de duplication.
- PNJ : TALK, dialogues et interactions.
- Combat : ATTACK/STATUS, 100 HP initiaux, dégâts, initiative, contre-attaques, DEFEND/FLEE et respawn sûr avec santé réduite.
- Quêtes : QUEST/QUESTS, acceptation, objectifs, progression, validation, récompenses et absence de double attribution.
- Extensions de gameplay existantes, notamment USE et les vues détaillées.
- Conception et équilibrage du monde avec l'équipe : au moins huit salles, boucle et branche optionnelle, trois rôles de PNJ, quatre objets dont deux récupérables et deux quêtes.
- Client CLI : connexion, saisie, envoi de commandes, réception des événements pendant la saisie, affichage, fermeture et restauration du terminal.
- Tests du monde, du moteur, de la concurrence métier et du CLI.
- Documentation : architecture du moteur, Combat System, Quest System, World Design, lancement et tests du CLI.

## Interface avec les autres pôles

Dev A gère le transport, le parser RFC, les sessions et la diffusion. Dev C vérifie les règles métier, applique les mutations et produit les résultats et événements. Dev A et Dev C définissent ensemble la protection de l'état partagé, les destinataires, l'ordre des événements et le nettoyage des joueurs déconnectés.

Dev B affiche les résultats et propose les actions graphiques. Les choix de combat, quêtes et contenu sont discutés à trois ; leur implémentation métier et le fichier du monde sont pilotés par Dev C.

## Prise en main et contributions passées

Le moteur et le CLI existent déjà et ont été implémentés par Aris. Le monde initial et la conception narrative ont été réalisés par Novanns. Dev C reprend leur suivi après passation ; aucune contribution passée ne lui est attribuée par ce changement d'organisation.

Le code actuel conserve l'état dans `internal/server`, le CLI dans `cmd/client-cli` et `internal/cli`. Cette répartition ne déplace pas le code.

- [Documentation CLI](../Dev_A/clients/cli_client.md) : emplacement historique conservé, responsabilité Dev C.
- [GDD partagé](../Commun/gdd/README.md)
- [Architecture serveur](../Dev_A/architecture/server_architecture.md) et [concurrence](../Dev_A/architecture/concurrency_model.md)
- [Matrice de vérification existante](../Commun/testing/aris_acceptance.md)
- [Protocole](../Commun/protocol/rfc_syntax.md)

Les tests et jalons communs sont définis dans [l'organisation d'équipe](../Commun/TAP_taches_communes.md). Utiliser les outils locaux et les cibles du Makefile pour construire et vérifier les composants.
