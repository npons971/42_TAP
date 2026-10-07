# TAP — Novanns (Dev B) : GUI + World Design

Ce document regroupe les responsabilités et tâches attribuées à **Novanns (Dev B)**.  
Pour les décisions d'équipe, le protocole partagé et l'architecture globale, se référer à [TAP_taches_communes.md](Commun/TAP_taches_communes.md).  
Pour la partie d'Aris (Dev A), voir [TAP_Aris_Dev_A.md](../Dev_A/TAP_Aris_Dev_A.md).

---

## 1. Client GUI

Choisir une vraie technologie graphique.

Exemples :

- Fyne
- GTK
- Qt
- interface web

`curses` ne compte pas comme GUI.

---

## 2. Connexion réseau

- connexion TCP au serveur
- envoi des commandes RFC
- réception des réponses
- réception des événements `EVT`
- gestion des déconnexions

---

## 3. Interface principale

Afficher :

- nom de la salle
- description
- sorties disponibles
- joueurs présents
- objets
- NPC
- inventaire
- HP
- informations de combat
- quêtes

---

## 4. Actions GUI

Prévoir des boutons ou contrôles pour :

- `LOOK`
- `MOVE`
- `TAKE`
- `DROP`
- `TALK`
- `ATTACK`
- `STATUS`
- `QUEST`
- `QUESTS`
- `WHO`
- `GROUP`
- `QUIT`

---

## 5. Chat

Séparer visuellement :

- chat Global
- chat Room
- chat Group
- logs / événements

---

## 6. Mise à jour en temps réel

La GUI doit réagir automatiquement aux événements serveur.

Exemples :

- joueur qui entre dans une salle
- joueur qui quitte une salle
- objet pris
- objet déposé
- message de chat
- combat
- modification de HP

---

## 7. World Design

Créer le fichier YAML ou JSON contenant le monde.

Minimum obligatoire :

- **8 salles interconnectées**
- au moins **une boucle**
- au moins **une branche optionnelle**
- **3 rôles de NPC**
- **4 objets**
- au moins **2 objets récupérables**
- **2 quêtes**

---

## 8. Contenu du monde

Créer :

- noms des salles
- descriptions
- sorties
- NPC
- dialogues
- ennemis
- objets
- emplacement des objets
- emplacement des NPC
- quêtes
- récompenses
- équilibrage des ennemis

*(Le moteur de vérification des quêtes et le chargement du fichier sont assurés par Aris).*

---

## Répartition du README (Sections de Novanns)

- World Design
- GUI
- Quest System / quest design
- Building and Running : GUI
- tests GUI

*(Le README doit être rédigé en anglais).*

---

## Ordre de développement recommandé (Planning Novanns)

- **Phase 1 — Cadrage d'équipe** `🟢 VALIDÉ` : lecture RFC, décision des structures, format JSON/YAML, conventions Git, IDs.
- **Phase 2 — Prototypage** `🟢 VALIDÉ` : squelette initial du monde, maquette / prototype de la GUI Wails v2 + Svelte.
- **Phase 3 — Première intégration** `🟢 VALIDÉ` : connexion de la GUI au serveur d'Aris (`CONNECT alice` -> `LOOK` -> `MOVE north` -> mise à jour de l'affichage), harnais de test d'intégration `app_test.go`.
- **Phase 4 — Enrichissement GUI & Monde** `🟢 VALIDÉ` : déploiement du monde complet (9 salles, 2 boucles, 2 branches), affichage/interaction des objets, inventaire, PNJ, dialogues, commandes sociales (`WHO`, `GROUP`), noms lisibles des sorties.
- **Phase 5 — Intégration Combat & Quêtes** `🟢 VALIDÉ` : design narratif des quêtes dans `world.yaml`/`world.json`, widget `QUEST TRACKER` dans l'UI, barre de contrôle du combat (`ATTACK`, `DEFEND`, `FLEE`).
- **Phase 6 — Validation & Finitions UX** `🟡 EN COURS` : tests multijoueur via l'interface graphique, gestion robuste des déconnexions/reconnexions.
- **Phase 7 — Finalisation** : rédaction des sections du README, lint, nettoyage du code, tests finaux, préparation à l'évaluation.

---

## Répartition approximative de la charge

| Domaine | Charge |
|---|---|
| Réseau GUI | 15 % |
| Interface graphique | 30 % |
| World Design | 20 % |
| Intégration GUI | 15 % |
| Combat / Quests UX | 10 % |
| Tests / documentation | 10 % |
