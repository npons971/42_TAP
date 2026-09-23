# 03 — Combat implémenté

## Tour et initiative

`ATTACK <npc_id ou nom complet>` engage un ennemi hostile vivant dans la salle.
Le joueur frappe d'abord, puis l'ennemi riposte s'il survit. Une commande
constitue un tour ; aucun minuteur ne frappe un joueur absent ou inactif.
Un ennemi ne peut être engagé que par un joueur à la fois ; les autres reçoivent
`target_busy`. La même cible est conservée jusqu'à victoire, défaite, fuite ou
déconnexion. Cela évite de multiples ripostes simultanées contre un PNJ unique.

## Dégâts

- Joueur : 12 PV + meilleur `damage_bonus` porté, sans cumuler les armes.
- Ennemi : `max(1, attack_power - meilleur defense_bonus porté)`.
- Dégâts appliqués limités aux PV restants ; aucune valeur ne devient négative.
- `DEFEND` : ne blesse pas l'ennemi et divise sa riposte après armure par deux,
  arrondie vers le bas. Une riposte de 1 devient donc 0.
- `FLEE [direction]` : 70 % de réussite. Sans direction, prend la première
  sortie disponible triée par nom. Une sortie invalide ne coûte aucun tour.
  La réussite libère la cible et déplace le joueur ; l'échec coûte une riposte.

Les dégâts sont déterministes pour rendre l'équilibrage lisible et les tests
reproductibles. Seule la fuite est aléatoire. Les meilleurs bonus évitent
l'empilement de plusieurs équipements et les additions sont bornées contre
les débordements d'entiers.

| Ennemi actif | PV initiaux/max | Attaque |
|---|---:|---:|
| Giant Sewer Rat | 20 | 6 |
| Bandit Leader | 70 | 20 |

## États et actions

`STATUS` renvoie `HORS_COMBAT` avec `combat:null`, ou `EN_COMBAT` avec
`target_id`, `target_name`, `target_hp`, `target_max_hp`.
MOVE, TAKE, DROP, TALK, QUEST et USE sont bloqués pendant un combat : utiliser FLEE.
LOOK, INVENTORY, STATUS, CHAT, WHO, QUESTINFO, QUESTS et GROUP restent disponibles.
DEFEND et FLEE sans engagement renvoient `not_in_combat`.

## Victoire, défaite et déconnexion

Un ennemi à zéro PV disparaît de LOOK et ne peut plus être attaqué ni interrogé.
Sa victoire est enregistrée pour le pseudo ayant porté le coup final. Ses PV,
sa mort et cette preuve de quête restent jusqu'au redémarrage du serveur.
Les ennemis sont uniques et ne réapparaissent pas automatiquement.

À zéro PV, le joueur revient immédiatement dans `world.respawn` avec **30 PV**,
quitte le combat et garde son inventaire. Une fuite ou une déconnexion libère
l'ennemi sans restaurer ses PV. À la déconnexion, les objets portés retombent
au sol comme prévu pour tous les objets uniques.

Chaque tour valide est une opération atomique sous le verrou du serveur.
La réponse OK est mise en file avant l'événement COMBAT ; si elle ne peut pas
être envoyée, les PV et les engagements restent inchangés. Un déplacement de
fuite ou de respawn diffuse ensuite PRESENCE LEAVE/ENTER aux autres joueurs.
Le tour, la victoire et la progression d'objectif sont journalisés.

## Résultat réseau

ATTACK, DEFEND et FLEE renvoient le même objet JSON :

```text
OK {"attacker_hp":80,"status":"combat","player":"alice","target":"npc.bandit_leader","damage":22,"counter_damage":20,"player_hp":80,"target_hp":48,"outcome":"ongoing","room":"loc.ruins_den"}
EVT ROOM COMBAT {"attacker_hp":80,"status":"combat","player":"alice","target":"npc.bandit_leader","damage":22,"counter_damage":20,"player_hp":80,"target_hp":48,"outcome":"ongoing","room":"loc.ruins_den"}
```

`outcome` vaut ongoing, victory, respawn, defended, fled ou flee_failed.
`player_hp` et `room` sont les valeurs après l'éventuel respawn/déplacement.
L'événement est destiné aux occupants de la salle de combat, acteur inclus.
