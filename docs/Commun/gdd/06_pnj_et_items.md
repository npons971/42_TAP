# 06 — Entités : PNJ & Objets

Ce document recense les entités non-joueurs (PNJ) et les objets physiques du monde conformément aux exigences de **RFC 42TAP** et du sujet.

---

## 1. Personnages Non-Joueurs (PNJ / NPCs)

Le sujet requiert au minimum **3 rôles distincts** de PNJ. Notre univers en propose 3 catégories clairement typées :
1. **Dialogue / Ambiance** : fournissent des indices, des salutations et de l'immersion narrative via `TALK`.
2. **Donneur de quête (Quest-giver)** : confient des missions et valident les étapes de progression.
3. **Ennemi hostile (Combat)** : peuvent être engagés au combat via `ATTACK`.

**État de l'implémentation :** le serveur charge le monde complet du Dev B depuis `data/world.json`. `LOOK` affiche les PNJ de la salle et `TALK` renvoie toutes leurs répliques dans l'ordre, jointes par un saut de ligne échappé dans la réponse JSON. Les rôles sont `dialogue`, `quest_giver` et `enemy`. Les livraisons et récompenses uniques sont actives ; la progression complète et le combat seront implémentés ensuite ; les statistiques ci-dessous sont les valeurs initiales chargées, sans effet de combat actuellement.

`TALK npc.guard` et `TALK Village Guard` sont équivalents. Les noms complets sont acceptés sans guillemets et sans tenir compte de la casse. Seuls les PNJ de la salle actuelle sont accessibles : une cible inconnue ou ailleurs produit `ERR target_not_found`, un nom ambigu produit `ERR invalid_arguments` avec une invitation à utiliser l'ID. La réponse conserve le format `OK {"npc":"npc.guard","dialogue":"..."}` et est envoyée uniquement au joueur qui parle. Les noms affichés ci-dessous sont ceux du monde actif, en anglais.

### Bestiaire & Annuaire des PNJ

| ID Technique | Nom affiché | Rôle | Salle | HP | Attaque | Répliques |
|---|---|---|---|---|---|---|
| `npc.guard` | Village Guard | `dialogue` | `loc.town_square` | 50 | 12 | 3 |
| `npc.innkeeper` | Jovial Innkeeper | `dialogue` | `loc.tavern` | 30 | 5 | 3 |
| `npc.merchant` | Traveling Merchant | `dialogue` | `loc.market` | 35 | 6 | 3 |
| `npc.thief` | Shady Cutpurse | `dialogue` | `loc.dark_alley` | 30 | 8 | 2 |
| `npc.herbalist` | Village Herbalist | `quest_giver` | `loc.garden` | 25 | 2 | 2 |
| `npc.guard_captain` | Guard Captain | `quest_giver` | `loc.ruins_gate` | 75 | 18 | 2 |
| `npc.giant_rat` | Giant Sewer Rat | `enemy` | `loc.sewers` | 20 | 6 | 1 |
| `npc.bandit_leader` | Bandit Leader | `enemy` | `loc.ruins_den` | 70 | 20 | 1 |

---

## 2. Système d'Objets & Inventaire

**Décision du Dev A : objets uniques.** Chaque entrée du catalogue représente une instance physique avec son propre ID. Le serveur conserve une seule position par ID : une salle, un joueur, la réserve des récompenses ou un état consommé. Il n'y a pas de quantité ni de pile dans l'état du jeu. `TAKE`, `DROP` et `INVENTORY` sont implémentés ; `LOOK` reflète les objets au sol et les changements produisent les événements `EVT ROOM ITEM TAKE/DROP`. À la déconnexion, l'inventaire est déposé dans la salle courante avant l'événement de départ.

### Exigences du Sujet
- Au moins **4 objets distincts**, dont au moins **2 récupérables** dans le monde (`obtainable: true`).
- Chaque objet est une **instance unique** dans le monde :
  - `TAKE` le retire de la pièce et le place dans l'inventaire du joueur.
  - `DROP` le replace dans la pièce où se tient le joueur.
  - Aucune duplication possible.
- Support complet des noms composés de plusieurs mots (ex: `Rare Herbs`, `Rusty Sword`).
- Résolution par ID technique (ex: `item.rare_herbs`) ou par nom affiché complet sans tenir compte de la casse. Si plusieurs objets du même contexte ont le même nom affiché, le serveur répond par exemple `ERR invalid_arguments TAKE name matches multiple items; use an item ID` ; les clients envoient de préférence l'ID unique.

### Catalogue des Objets

| ID Technique | Nom affiché | Placement initial | Obtenable | Propriétés prévues |
|---|---|---|---|---|
| `item.apple` | Fresh Apple | `loc.market` | Oui | consumable ; Soin : 10 |
| `item.ale` | Frothy Ale | `loc.tavern` | Oui | consumable ; Soin : 15 |
| `item.rusty_sword` | Rusty Sword | `loc.dark_alley` | Oui | weapon ; Bonus attaque : 10 |
| `item.torch` | Wooden Torch | `loc.cellar` | Oui | tool |
| `item.wooden_shield` | Battered Shield | `loc.ruins_gate` | Oui | armor ; Bonus défense : 5 |
| `item.rare_herbs` | Rare Herbs | `loc.garden` | Oui | quest |
| `item.mystery_chest` | Heavy Iron Chest | `loc.ruins_den` | Non | scenery |
| `item.fountain` | Stone Fountain | `loc.town_square` | Non | scenery |
| `item.ancient_key` | Ancient Key | Réserve de quête | Oui | quest_reward |
| `item.vigor_potion` | Vigor Potion | Réserve de quête | Oui | consumable ; Soin : 100 |

Les soins des consommables sont actifs via `USE`. Les herbes sont consommées par la livraison via `TALK`. Les bonus de combat attendent le moteur de combat. Les objets réservés ne sont pas affichés au sol et ne peuvent pas être pris tant qu’ils ne sont pas attribués. Un objet doit avoir un placement unique : une salle ou `initial_location: {"kind":"reserve"}`, jamais les deux.

---

## 3. Matrice d'Interaction

- `LOOK` : liste les objets présents au sol dans la pièce ainsi que les PNJ.
- `TAKE <objet>` :
  - Si l'objet est `obtainable: true` : retiré du sol, placé dans l'inventaire -> `OK taken=item.<id>`
  - Si l'objet est `obtainable: false` : refusé -> `ERR item_not_obtainable`
  - Si l'objet n'existe pas : `ERR item_not_found`
- `DROP <objet>` :
  - Si présent dans l'inventaire : retiré de l'inventaire, placé au sol -> `OK dropped=item.<id>`
  - Si absent de l'inventaire : `ERR not_in_inventory`
- `INVENTORY` : retourne la liste exhaustive des objets actuellement possédés, sous la forme `OK [{"id":"item.apple","name":"Fresh Apple"}]` ([schéma JSON partagé](../protocol/json_payloads.md)).

### USE et consommation

USE accepte un ID exact ou un nom complet sans tenir compte de la casse,
et agit uniquement sur l'inventaire. Il restaure heal_value PV sans dépasser
100, puis retire l'instance définitivement jusqu'au redémarrage. À pleine
santé, health_full conserve l'objet. Une arme, un outil ou un ingrédient de
quête reçoit item_not_usable ; un objet absent reçoit not_in_inventory.
Une livraison à l'herboriste consomme les herbes, attribue la potion de réserve
et soigne à 100, une seule fois par monde. Les objets non consommés sont
partageables ; aucun objet unique n'est recréé à la déconnexion.
