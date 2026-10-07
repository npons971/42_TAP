# 06 — Entités : PNJ & Objets

Ce document recense les entités non-joueurs (PNJ) et les objets physiques du monde conformément aux exigences de **RFC 42TAP** et du sujet.

---

## 1. Personnages Non-Joueurs (PNJ / NPCs)

Le sujet requiert au minimum **3 rôles distincts** de PNJ. Notre univers en propose 3 catégories clairement typées :
1. **Dialogue / Ambiance** : fournissent des indices, des salutations et de l'immersion narrative via `TALK`.
2. **Donneur de quête (Quest-giver)** : confient des missions et valident les étapes de progression.
3. **Ennemi hostile (Combat)** : peuvent être engagés au combat via `ATTACK`.

### Bestiaire & Annuaire des PNJ

| ID Technique | Nom affiché | Rôle | Salle | HP | Attaque | Dialogues (`TALK`) |
|---|---|---|---|---|---|---|
| `npc.guard` | Garde du Village | Dialogue / Lore | `loc.town_square` | 50 | 15 PV | "Restez sur vos gardes, voyageur.", "La route vers les ruines est périlleuse." |
| `npc.innkeeper` | Tavernier Jovial | Dialogue / Ambiance | `loc.tavern` | 30 | 5 PV | "Bienvenue à la taverne !", "Rien de tel qu'une bonne chope après l'aventure." |
| `npc.herbalist` | Herboriste | Donneur de quête | `loc.garden` | 20 | 0 PV | "Bonjour aventurier. Auriez-vous un instant pour m'aider ?", "Mes herbes séchées sont presque épuisées." |
| `npc.guard_captain`| Capitaine de la Garde | Donneur de quête | `loc.ruins_gate` | 70 | 20 PV | "Halte ! Seuls les braves franchissent ce seuil.", "Une prime attend quiconque vaincra le chef bandit." |
| `npc.giant_rat` | Rat Géant | Ennemi (Faible) | `loc.sewers` | 25 | 6 PV | *Couinements agressifs et dents qui claquent.* |
| `npc.bandit_leader`| Chef des Bandits | Ennemi (Boss) | `loc.ruins_den` | 80 | 22 PV | "Tu oses entrer dans mon repaire ? Tu ne repartiras pas vivant !" |

---

## 2. Système d'Objets & Inventaire

### Exigences du Sujet
- Au moins **4 objets distincts**, dont au moins **2 récupérables** dans le monde (`obtainable: true`).
- Chaque objet est une **instance unique** dans le monde :
  - `TAKE` le retire définitivement de la pièce.
  - `DROP` le replace dans la pièce où se tient le joueur.
  - Aucune duplication possible.
- Support complet des noms composés de plusieurs mots (ex: `Herbes Rares`, `Épée Rouillée`).
- Résolution indifférente par ID technique (ex: `item.rare_herbs`) ou par nom affiché sensible/insensible à la casse.

### Catalogue des Objets

| ID Technique | Nom affiché | Salle Initiale | Obtenable | Propriétés & Utilité |
|---|---|---|---|---|
| `item.ale` | Chope de Bière | `loc.tavern` | **Oui** (`true`) | Objet d'ambiance / Consommable (restaure 10 HP). |
| `item.rusty_sword`| Épée Rouillée | `loc.dark_alley` | **Oui** (`true`) | Arme : confère un bonus de $+10$ dégâts lors des attaques. |
| `item.rare_herbs` | Herbes Rares | `loc.garden` | **Oui** (`true`) | Objet de quête indispensable pour l'Herboriste (`quest.herbal_cure`). |
| `item.wooden_shield`| Bouclier Fendu | `loc.ruins_gate` | **Oui** (`true`) | Équipement défensif : renforce l'action `DEFEND`. |
| `item.apple` | Pomme Fraîche | `loc.market` | **Oui** (`true`) | Nourriture simple (restaure 5 HP). |
| `item.fountain` | Fontaine de Pierre | `loc.town_square` | **Non** (`false`) | Élément interactif du décor, impossible à ramasser. |
| `item.ancient_key`| Clé Ancienne | *Récompense de quête* | **Oui** (`true`) | Objet de progression débloquant l'accès aux salles secrètes. |

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
- `INVENTORY` : retourne la liste exhaustive des objets actuellement possédés.
