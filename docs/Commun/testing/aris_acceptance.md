# Aris — serveur et CLI : vérifications

## Commandes reproductibles

```sh
make test
GOPATH="$PWD/.go-work" GOCACHE="$PWD/.go-cache" ./go test -race -count=30 ./...
make build-server build-cli
make lint
```

Le SDK et les caches sont locaux au dépôt. Les tests de pseudo-terminal sont
écrits en Go et exécutés sur Linux avec les autres tests CLI.

## Matrice de tests

| Domaine | Vérifications |
|---|---|
| Protocole | LF/UTF-8, ligne 4096 octets, erreurs spécifiques, retry après erreur, QUIT |
| Monde | Neuf salles parcourues, catalogues/placements, références, dialogues, données invalides |
| Objets | TAKE concurrent unique, DROP, inventaire, noms Unicode/composés, déconnexion |
| Consommables | PV bornés, overflow, pleine santé, objet consommé terminal, USE/DROP concurrent |
| Récompenses | Possession réelle, donneur local, allocation unique, reconnexion, transfert, échec atomique |
| Combat | Arme et armure, riposte, victoire sans riposte finale, STATUS réel, cible occupée, DEFEND |
| Fuite et défaite | Fuite réussie/échouée déterministe dans les tests, retour à 30 PV, libération de cible |
| Quêtes | Acceptation, possession, DROP qui invalide l'objectif, victoire réelle, rapport, statut et reconnexion |
| Groupes | IDs générés concurrents, invitations, chef sortant, membership, chat entre salles, confidentialité, suppression à vide |
| Transactions | File de sortie pleine ou réponse trop longue ne modifie pas les objets/PV/engagements |
| Monitoring | Token bucket burst/refill, fermeture TCP du client abusif seul, logs JSON/erreurs/paramètres, connexions rapides |
| CLI | Réception pendant saisie, prompt partiel, UTF-8/backspace, EOF/QUIT, annulation, trames invalides |
| Intégration | Le vrai CLI joue la prime du bandit sur le vrai serveur et reçoit la clé |
| Terminal Linux | Pseudo-terminal : saisie, déconnexion distante au clavier inactif, SIGTERM, restauration des attributs |
| Invariants | 2 000 actions d'objets mélangées par répétition, positions uniques et aucun objet consommé recréé |

## Parcours manuel complet

Terminal 1 : `make run-server`.
Terminal 2 : `make run-cli`, puis :

```text
CONNECT aris
GROUP CREATE
MOVE north
TALK npc.herbalist
QUESTS
TAKE item.rare_herbs
QUEST npc.herbalist
TALK npc.herbalist
MOVE south
MOVE west
TAKE item.rusty_sword
MOVE south
MOVE east
MOVE east
TALK npc.guard_captain
TAKE item.wooden_shield
MOVE north
ATTACK npc.bandit_leader
STATUS
ATTACK npc.bandit_leader
ATTACK npc.bandit_leader
ATTACK npc.bandit_leader
MOVE south
TALK npc.guard_captain
QUESTS
INVENTORY
USE item.vigor_potion
QUIT
```

Un troisième terminal peut rejoindre le groupe avec CONNECT bob puis GROUP
JOIN aris. CHAT GROUP fonctionne entre salles ; un non-membre ne
reçoit rien. Les autres joueurs présents dans la salle de combat voient les
events COMBAT. Le bandit et les récompenses étant uniques, relancer le serveur
pour rejouer entièrement ce scénario.

Le RFC externe fourni dans rfc.tar.gz est copié dans
[external_rfc.html](../protocol/external_rfc.html). Les tests RFC vérifient les
trames standard et les événements sans filtrage. Le backend GUI a un test TCP
(`go test -race app.go app_test.go` dans Project/client-gui avec le SDK local)
et le frontend se compile avec npm run build. Une session visuelle Wails et les
clients indépendants d'autres équipes restent des vérifications manuelles.
