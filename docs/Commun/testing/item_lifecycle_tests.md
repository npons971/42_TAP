# Tests du cycle de vie des objets

Le serveur garde une instance par ID : salle, inventaire, réserve ou consommé.
Les objets consommés restent indisponibles jusqu'au redémarrage. Chaque
récompense est attribuée une fois par monde, sans copie par joueur.

## Commandes de vérification

Depuis la racine, avec le SDK local et les caches locaux :

```sh
make test-server
GOPATH="$PWD/.go-work" GOCACHE="$PWD/.go-cache" ./go test -race -count=30 ./internal/server
make build-server
GOPATH="$PWD/.go-work" GOCACHE="$PWD/.go-cache" ./go vet ./...
```

## Cas couverts

- TCP : USE avant CONNECT, arguments absents/blancs, tabulations et espaces
  périphériques, cible absente ou réservée, nom complet sans tenir compte de
  la casse, arme non consommable, objet conservé à pleine santé.
- Soin : pomme +10, bière +15, potion +100 ; plafond 100, dépassement et valeur
  maximale d'un entier sans overflow ; STATUS et INVENTORY reflètent le résultat.
- Consommation : un seul succès, disparition du sol et de l'inventaire,
  impossibilité de TAKE/DROP/USE ensuite, absence de réapparition à la déconnexion.
- Livraison TCP à deux joueurs : seul le propriétaire livre, seulement auprès
  du donneur local, herbes consommées, potion attribuée, soin à 100, OK avant
  DELIVER puis REWARD pour l'acteur et événements pour l'autre joueur.
- Anti-duplication : répétition, autre joueur, reconnexion avec le même pseudo,
  transfert de la récompense, consommation et réinitialisation au redémarrage.
- Déconnexion : la potion non consommée tombe au sol avant PRESENCE LEAVE ;
  les herbes consommées ne sont pas redéposées. Un autre joueur peut prendre
  et utiliser une récompense déposée sans réinitialiser son attribution.
- Échec atomique : récompense absente, portée, au sol ou consommée, file de
  sortie pleine, dialogue trop long ; ingrédients, PV et attribution inchangés.
- Concurrence : USE/USE, USE/DROP, double livraison et deux demandeurs de la
  même récompense ; une seule consommation ou attribution.
- Chargement : références inconnues, récompense non réservée ou partagée,
  ingrédient inadapté, donneur incorrect, type non pris en charge et soin invalide.
- Allocation interne : définition inventée/modifiée et joueur anonyme refusés.
- 2 000 actions mélangées reproductibles : TAKE, DROP, USE, TALK, déplacement,
  déconnexion et blessures simulées ; vérification de la position unique de
  chaque instance, du nombre d'objets, des PV et des invariants de livraison.
- Régression : anciennes commandes, monde complet, résolution des noms,
  erreurs de protocole et limites de ligne ; suite entière sous détecteur de courses.

Les blessures sont injectées sous verrou dans les tests ; aucune commande
publique ne modifie arbitrairement les PV. Le combat, la progression complète
de quête et la preuve d'élimination du bandit restent une étape suivante.
L'allocateur de sa récompense est testé en interne ; TALK ne délivre pas la clé.

## Essai manuel

```text
CONNECT aris
MOVE north
TAKE item.rare_herbs
TALK npc.herbalist
INVENTORY
STATUS
USE item.vigor_potion
TALK npc.herbalist
```

La potion est attribuée une fois et les PV sont à 100. USE renvoie health_full
et garde la potion. Le second TALK indique une livraison déjà terminée. Un
second joueur qui parle au même donneur reçoit reward_unavailable. Sans combat,
la branche de soin après blessure est vérifiée par les tests automatisés.
