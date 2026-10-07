# 04 — Quêtes implémentées

## Progression

Les définitions de `data/world.json` sont chargées et validées avant l'écoute
TCP : donneurs, cibles, récompenses réservées, types, titre et description.
Un donneur n'a qu'une quête afin que TALK ait une interprétation unique.

QUEST <pnj> auprès du donneur commence la quête. TALK conserve aussi cette
acceptation ; si l’objectif est déjà rempli, TALK peut terminer immédiatement
la quête. QUESTS et QUESTINFO sont des lectures sans effet.
Les statuts internes exposés par QUESTINFO et QUESTS DETAILS sont :

| Statut | Signification |
|---|---|
| NOT_STARTED | Pas encore de dialogue avec le donneur |
| IN_PROGRESS | Quête acceptée, objectif non rempli |
| OBJECTIVES_MET | Objet actuellement possédé ou victoire enregistrée |
| COMPLETED | Récompense attribuée à ce pseudo |
| UNAVAILABLE | Récompense unique déjà attribuée à un autre pseudo |

QUESTS liste les quêtes acceptées/terminées par quest_id croissant, avec
status active/completed et progress 0/1 ou 1/1 pour une quête active. Une quête
acceptée dont la récompense est attribuée à un autre joueur devient unavailable.
QUESTS DETAILS liste aussi les définitions non commencées.
Le suivi est conservé par pseudo pendant la vie du serveur et survit à une
reconnexion. Il n'y a pas de stockage disque ni de mot de passe : un pseudo
n'est pas une identité sécurisée ; ce choix correspond au protocole CONNECT.

## quest.herbal_cure — The Apothecary's Remedy

1. TALK npc.herbalist dans loc.garden accepte la quête.
2. TAKE item.rare_herbs permet OBJECTIVES_MET.
3. TALK npc.herbalist consomme les herbes, attribue item.vigor_potion et soigne
   jusqu'à 100 PV. Si les herbes sont déjà possédées au premier TALK, la
   livraison réussit immédiatement.

Un DROP ou une déconnexion avant livraison fait revenir l'objectif à
IN_PROGRESS tant que l'objet n'est plus dans l'inventaire du joueur.

## quest.bandit_bounty — Bounty on the Cutthroat

1. TALK npc.guard_captain dans loc.ruins_gate accepte la quête.
2. ATTACK npc.bandit_leader dans loc.ruins_den jusqu'à sa défaite enregistre
   la victoire pour le joueur ayant porté le coup final.
3. TALK npc.guard_captain attribue item.ancient_key, une seule fois.

Une victoire obtenue avant l'acceptation reste valide. Une fuite, la défaite
du joueur ou la victoire d'un autre joueur ne constitue pas une preuve.
La preuve de victoire survit à la reconnexion du même pseudo.

## Unicité et validation atomique

Les ennemis, les herbes et les récompenses sont uniques dans ce monde partagé.
Les objets consommés et les ennemis vaincus ne réapparaissent pas avant
redémarrage. Seul le premier joueur remplissant l'objectif reçoit la récompense.
Les objets non consommés restent partageables via DROP/TAKE ; il n'existe
aucune copie personnelle de l'ennemi, de l'ingrédient ou de la récompense.

La disponibilité, la possession/victoire, la taille de la réponse et la file
de sortie sont contrôlées avant mutation. La consommation, le transfert, le
soin et l'attribution globale sont ensuite validés sous le même verrou.
Répéter TALK avec le pseudo gagnant renvoie un dialogue de quête déjà terminée ;
un autre joueur reçoit ERR 406 NO_QUEST_AVAILABLE. Déposer ou consommer une récompense
ne réinitialise pas son attribution.

La réponse TALK est du texte sur une ligne ; TALKJSON conserve npc/dialogue. Après la réponse, une livraison
diffuse ITEM DELIVER puis ITEM REWARD ; un rapport de victoire diffuse seulement
ITEM REWARD. Les changements sont journalisés ; QUESTS/QUESTINFO exposent les états
courants sans ajouter d'événement privé ambigu aux réponses.
