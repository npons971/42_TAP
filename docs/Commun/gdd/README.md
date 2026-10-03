# TAP — Game Design Document (GDD)

Le GDD est piloté par Alexis (Dev C), avec la participation
d’Aris et Novanns. Le monde initial reste une contribution de Novanns.
Voir [l’organisation à trois](../TAP_taches_communes.md).

Ce dossier constitue le **Game Design Document (GDD)** de référence pour le projet **TAP (The Answer Protocol)**, un jeu d'aventure textuelle multijoueur rétro (MUD) client-serveur TCP.

Le GDD est la source de vérité pour le game design, le contenu narratif, l'agencement du monde, l'équilibrage et l'expérience utilisateur. Il sert de spécification fonctionnelle pour le serveur, le client CLI et le client GUI.

---

## Sommaire de la documentation

1. [01. Vision & Piliers](01_vision_piliers.md) : Concept, ambiance, intentions et piliers de game design.
2. [02. Boucle de Gameplay](02_boucle_gameplay.md) : Core loop, règles générales du monde et cycle d'une session.
3. [03. Système de Combat](03_systeme_combat.md) : Combat tour par tour, calcul des dégâts, initiative, défenses, fuite et mort/respawn.
4. [04. Système de Quêtes](04_systeme_quetes.md) : Moteur de quêtes, machine à états, validation d'objectifs et fiches des quêtes.
5. [05. World Design & Salles](05_world_design.md) : Topologie de la carte (8+ salles, boucles, branches) et fiches descriptives des lieux.
6. [06. Entités (PNJ & Objets)](06_pnj_et_items.md) : Rôles des PNJ (dialogue, quête, ennemi) et catalogue des objets uniques.
7. [07. Multijoueur & Social](07_multijoueur_social.md) : Canaux de discussion (Global, Room, Group), présence et groupes.
8. [08. Interfaces & Expérience Joueur (CLI / GUI)](08_interfaces_ux.md) : Spécifications ergonomiques, maquettes et réactivité en temps réel.

---

## Contraintes impératives du Sujet (Checklist de conformité)

Pour valider le projet, le design du jeu consigné dans ce GDD doit respecter strictement les minima suivants :

- [ ] **Topologie du monde** : au moins **8 salles interconnectées** formant au moins **une boucle complète** (exploration circulaire, pas de carte en ligne droite) et au moins **une branche optionnelle**.
- [ ] **Rôles de PNJ** : au moins **3 rôles distincts** (par exemple : dialogue/lore, donneur de quête, ennemi hostile).
- [ ] **Objets uniques** : au moins **4 objets distincts**, dont au moins **2 obtenables en jeu** (`obtainable: true`).
- [ ] **Gestion des objets** : instances uniques dans le monde (aucun doublon lors du `TAKE`/`DROP`), support des noms composés de plusieurs mots.
- [ ] **Quêtes** : au moins **2 quêtes distinctes** avec progression, validation d'objectifs et récompenses.
- [ ] **Combat au tour par tour** : joueur démarrant à 100 HP, ennemis avec HP variables, gestion des dégâts/ripostes, commande `STATUS`, respawn en zone sûre avec HP réduits à 0 HP.
- [ ] **Multijoueur synchrone** : gestion des événements `EVT` asynchrones (déplacements, prises d'objets, combats, chat).
