# 05 — World Design & Topologie du Monde

Le sujet impose un monde cohérent d'au moins **8 salles interconnectées**, comportant au moins **une boucle** (exploration circulaire complète) et au moins **une branche optionnelle**.

---

## 1. Topologie de la Carte (Graphe du Monde)

Le monde proposé compte **9 salles interconnectées** pour dépasser confortablement le minimum requis :
- **Une grande boucle principale (circuit)** : `Square` -> `Market` -> `Tavern` -> `Back_Alley` -> `Square`
- **Une boucle secondaire** : `Tavern` -> `Cellar` -> `Sewers` -> `Back_Alley`
- **Une branche optionnelle (impasse / donjon)** : `Market` -> `Ruins_Entrance` -> `Ruins_Den` (antre du bandit)

```mermaid
graph TD
    subgraph Boucle_Principale [Circuit Urbain Principal]
        Square["1. Place du Village (start)<br/>[Zone Sûre / Respawn]"]
        Market["2. Marché Central"]
        Tavern["3. Taverne du Dragon Assoupi"]
        Alley["4. Ruelle Sombre"]
    end

    subgraph Boucle_Souterraine [Boucle Souterraine]
        Cellar["5. Cave à Vin"]
        Sewers["6. Égouts Humides"]
    end

    subgraph Branche_Optionnelle [Branche Optionnelle des Ruines]
        RuinsGate["7. Porte des Ruines"]
        RuinsDen["8. Antre du Bandit (Boss)"]
    end

    subgraph Annexe_Herboriste [Échoppe & Jardin]
        Garden["9. Jardin Abandonné"]
    end

    %% Connexions Boucle Principale
    Square <-->|"east / west"| Market
    Market <-->|"north / south"| Tavern
    Tavern <-->|"west / east"| Alley
    Alley <-->|"south / north"| Square

    %% Connexions Boucle Souterraine
    Tavern <-->|"down / up"| Cellar
    Cellar <-->|"south / north"| Sewers
    Sewers <-->|"up / down"| Alley

    %% Connexions Branche Optionnelle
    Market <-->|"east / west"| RuinsGate
    RuinsGate <-->|"north / south"| RuinsDen

    %% Connexion Jardin
    Square <-->|"north / south"| Garden
```

---

## 2. Dictionnaire Détaillé des Salles

### Salle 1 : Place du Village (`loc.town_square`) — *Start / Safe Zone*
- **Nom affiché** : Place du Village
- **Description** : Une vaste place pavée baignée d'une lumière douce. Au centre trône une fontaine de pierre où l'eau clapote paisiblement. Des aventuriers s'y rassemblent avant de partir en expédition.
- **Sorties** : `east: loc.market`, `north: loc.garden`, `west: loc.dark_alley`
- **Objets initiaux** : *aucun*
- **PNJ présents** : Garde du Village (`npc.guard`)
- **Propriété** : Zone de respawn après la mort.

### Salle 2 : Marché Central (`loc.market`)
- **Nom affiché** : Marché Central
- **Description** : Des étals d'étoffes, d'épices et d'outils s'alignent le long de l'allée commerçante. L'odeur du pain chaud et des fruits mûrs flotte dans l'air.
- **Sorties** : `west: loc.town_square`, `north: loc.tavern`, `east: loc.ruins_gate`
- **Objets initiaux** : Pomme Fraîche (`item.apple`)
- **PNJ présents** : Marchand Ambulant (`npc.merchant`)

### Salle 3 : Taverne du Dragon Assoupi (`loc.tavern`)
- **Nom affiché** : Taverne du Dragon Assoupi
- **Description** : Une salle chaleureuse éclairée par un grand feu de cheminée. Le plancher grince sous les pas et une trappe mène au sous-sol.
- **Sorties** : `south: loc.market`, `west: loc.dark_alley`, `down: loc.cellar`
- **Objets initiaux** : Chope de Bière (`item.ale`)
- **PNJ présents** : Tavernier Jovial (`npc.innkeeper`)

### Salle 4 : Ruelle Sombre (`loc.dark_alley`)
- **Nom affiché** : Ruelle Sombre
- **Description** : Un passage étroit entre de hauts murs de brique humide. L'obscurité y est épaisse et une grille d'égout béante laisse monter une odeur fétide.
- **Sorties** : `south: loc.town_square`, `east: loc.tavern`, `down: loc.sewers`
- **Objets initiaux** : Épée Rouillée (`item.rusty_sword`)
- **PNJ présents** : Voleur à la Tire (`npc.thief`)

### Salle 5 : Cave à Vin (`loc.cellar`)
- **Nom affiché** : Cave de la Taverne
- **Description** : De gros tonneaux de chêne s'empilent dans l'ombre. De la poussière recouvre les bouteilles rangées sur les étagères. Un passage dérobé semble s'ouvrir vers le sud.
- **Sorties** : `up: loc.tavern`, `south: loc.sewers`
- **Objets initiaux** : Torche Éteinte (`item.torch`)
- **PNJ présents** : *aucun*

### Salle 6 : Égouts Humides (`loc.sewers`)
- **Nom affiché** : Égouts de la Ville
- **Description** : Un canal d'eaux usées bordé par un rebord glissant. Des clapotis résonnent dans le lointain et des yeux luisants vous observent depuis les recoins.
- **Sorties** : `north: loc.cellar`, `up: loc.dark_alley`
- **Objets initiaux** : *aucun*
- **PNJ présents** : Rat Géant (`npc.giant_rat`)

### Salle 7 : Porte des Ruines (`loc.ruins_gate`)
- **Nom affiché** : Porte des Ruines Extérieures
- **Description** : Une arche de pierre effondrée marquant l'entrée d'anciennes fortifications oubliées. Les ronces ont envahi les pavés fêlés.
- **Sorties** : `west: loc.market`, `north: loc.ruins_den`
- **Objets initiaux** : Bouclier Fendu (`item.wooden_shield`)
- **PNJ présents** : Capitaine de la Garde (`npc.guard_captain`)

### Salle 8 : Antre du Bandit (`loc.ruins_den`) — *Branche optionnelle & Boss*
- **Nom affiché** : Antre du Coupe-Gorge
- **Description** : Un campement de fortune aménagé sous une voûte de pierre brisée. Un feu de camp crépite au milieu de caisses pillées. L'atmosphère est lourde et hostile.
- **Sorties** : `south: loc.ruins_gate`
- **Objets initiaux** : Coffre Mystérieux (`item.mystery_chest`)
- **PNJ présents** : Chef des Bandits (`npc.bandit_leader`)

### Salle 9 : Jardin Abandonné (`loc.garden`)
- **Nom affiché** : Jardin Abandonné
- **Description** : Une enclave de verdure sauvage où la nature a repris ses droits. De vieux parterres de fleurs côtoient des buissons épineux et des herbes aromatiques.
- **Sorties** : `south: loc.town_square`
- **Objets initiaux** : Herbes Rares (`item.rare_herbs`)
- **PNJ présents** : Herboriste (`npc.herbalist`)

---

## 3. Schéma de Données (JSON)

Le serveur charge `data/world.json`. Ce fichier contient le catalogue des objets uniques dans `world.items` et leur placement initial dans `world.locations.<id>.items`. Chaque objet doit être déclaré une seule fois et placé dans exactement une salle. Le chargement refuse les IDs inconnus, les doublons et les objets sans salle initiale.

Les identifiants suivent la [convention du protocole](../protocol/rfc_syntax.md) : `loc.*` pour les salles, `item.*` pour chaque instance physique unique, `npc.*` pour les PNJ et `quest.*` pour les quêtes. Ils sont stables, uniques, en ASCII minuscule avec `_` entre les mots ; les noms affichés restent du texte UTF-8. Deux exemplaires d'un objet reçoivent deux IDs différents, par exemple `item.apple_1` et `item.apple_2`.

Exemple minimal du format chargé :

```json
{
  "world": {
    "start": "loc.town_square",
    "items": [
      {"id": "item.apple", "name": "Pomme fraîche", "obtainable": true}
    ],
    "locations": {
      "loc.town_square": {
        "name": "Place du Village",
        "description": "Une vaste place pavée.",
        "exits": {"north": "loc.garden"},
        "items": ["item.apple"]
      },
      "loc.garden": {
        "name": "Jardin",
        "description": "Un jardin paisible.",
        "exits": {"south": "loc.town_square"},
        "items": []
      }
    }
  }
}
```

`obtainable: false` permet d'afficher un objet fixe dans `LOOK` tout en refusant `TAKE`. Les noms sont du texte UTF-8 sur une seule ligne, sans tabulation ni espaces en début ou fin.

Le fichier actuel est un monde de test de deux salles ; ses placements servent aux essais du serveur. Le monde complet décrit dans les sections 1 et 2 sera fourni par le Dev B. Le schéma des PNJ sera ajouté avec leur implémentation.

Pendant l'exécution, le serveur conserve une position unique par objet : une salle ou l'inventaire d'un joueur. `TAKE` et `DROP` modifient cette position ; `LOOK` et `INVENTORY` lisent l'état courant. À la déconnexion, les objets portés sont déposés dans la salle actuelle du joueur. Un redémarrage recharge les placements initiaux du fichier.
