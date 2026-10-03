# 01 — Vision, Pitch & Piliers

## 1. Executive Summary & Pitch

- **Titre du projet** : TAP (The Answer Protocol)
- **Genre** : Rétro Text Adventure / MUD (Multi-User Dungeon) multijoueur synchrone
- **Plateformes cibles** : Linux / macOS / Windows (Console CLI + GUI native/web)
- **Technologies** : Serveur TCP en Go (ou C++/Rust/Zig), clients interchangeables respectant le protocole RFC 42TAP.

### Logline / Pitch en une phrase
> *Un MUD multijoueur où les aventuriers explorent un univers textuel partagé, coopèrent ou discutent en temps réel, accomplissent des quêtes scénarisées et affrontent des périls tactiques au tour par tour.*

---

## 2. Piliers de Game Design

Les piliers sont les principes directeurs qui guident chaque choix d'implémentation et de game design :

### Pilier 1 : Exploration Vivante & Non Linéaire
Le monde n'est pas un couloir. Il récompense l'orientation et la curiosité spatiale grâce à des boucles de déplacement naturelles et des embranchements optionnels cachant des récompenses ou des créatures uniques.

### Pilier 2 : Présence Multijoueur Synchrone
Le monde réagit en direct à la présence des autres aventuriers. Voir un joueur entrer dans la taverne, s'emparer d'une torche ou combattre un garde dans la pièce voisine crée une impression d'univers persistant et connecté.

### Pilier 3 : Tactique Textuelle & Prudence
Le combat n'est pas du matraquage de commande : chaque tour compte. Gérer ses points de vie, évaluer la puissance des ennemis, choisir quand attaquer, défendre ou fuir et préparer son inventaire font la différence entre la victoire et la mort.

---

## 3. Direction Narrative & Ambiance

*Note : La thématique précise (ex: Médiéval-Fantastique sombre, Cyberpunk rétro, Donjon classique) est à affiner par l'équipe, sous le pilotage de Dev C.*

- **Ambiance sonore et visuelle projetée** : descriptions textuelles courtes mais percutantes, vocabulaire évocateur mettant en valeur les détails de chaque lieu.
- **Ton du jeu** : teinté de mystère et d'aventure rétro, avec des PNJ aux personnalités marquées (le garde méfiant, le marchand opportuniste, le monstre menaçant).

---

## 4. Conditions de Victoire & Défaite

### Conditions de Victoire / Progression
- Complétion des quêtes principales scénarisées.
- Découverte de toutes les salles secrètes ou obtention des artefacts majeurs du monde.
- Élimination des ennemis majeurs.

### Conditions de Défaite
- Réduction des HP du joueur à 0 lors d'un combat.
- Conséquences : mort du joueur, notification broadcastée aux joueurs de la salle, réapparition immédiate dans la zone sûre (ex: Place du Village) avec des points de vie réduits (ex: 20 HP ou 50%).
