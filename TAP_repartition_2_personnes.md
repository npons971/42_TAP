# TAP — Répartition du projet à 2

## Répartition générale

Le sujet recommande, pour un groupe de 2 :

- **Personne 1 : Serveur + client CLI**
- **Personne 2 : Client GUI + World Design**

---

# Personne 1 — Serveur + CLI

## 1. Serveur TCP

- Initialiser le projet Go
- Ouvrir un serveur TCP
- Accepter plusieurs connexions simultanées
- Gérer chaque client dans une goroutine
- Gérer les déconnexions proprement
- Maintenir la liste des joueurs connectés

## 2. Implémentation du protocole

Implémenter toutes les commandes prévues par RFC 42TAP :

- `CONNECT`
- `LOOK`
- `MOVE`
- `CHAT`
- `TAKE`
- `DROP`
- `INVENTORY`
- `TALK`
- `ATTACK`
- `STATUS`
- `QUEST`
- `QUESTS`
- `WHO`
- `GROUP`
- `QUIT`

À gérer également :

- réponses `OK`
- erreurs protocole
- événements asynchrones `EVT`
- validation des arguments
- commandes malformées

## 3. Gestion de l'état du jeu

Le serveur doit être la source de vérité.

Gérer :

- joueurs connectés
- position des joueurs
- inventaires
- HP
- états de combat
- progression des quêtes
- objets présents dans les salles
- NPC
- groupes de joueurs

Prévoir une structure centrale de type :

```go
type GameState struct {
    Players map[string]*Player
    Rooms   map[string]*Room
    Items   map[string]*Item
    NPCs    map[string]*NPC
    Quests  map[string]*Quest
}
```

## 4. Concurrence

- protéger les données partagées avec `sync.Mutex` / `sync.RWMutex`
- ou utiliser des channels lorsque pertinent
- éviter les race conditions
- supporter plusieurs joueurs agissant simultanément
- ne pas faire planter les broadcasts lorsqu'un client se déconnecte

## 5. Système d'objets

- objets uniques dans le monde
- `TAKE` retire l'objet de la salle
- `DROP` remet l'objet dans la salle
- empêcher les duplications
- recherche par ID ou nom affiché
- supporter les noms composés de plusieurs mots

## 6. Combat

Implémenter :

- joueur à 100 HP au départ
- HP variables pour les ennemis
- `ATTACK`
- dégâts
- contre-attaques
- initiative
- combat au tour par tour
- `STATUS`
- mort à 0 HP
- respawn dans une zone sûre
- éventuellement :
  - `DEFEND`
  - `FLEE`

Le fonctionnement exact du combat doit être conçu et documenté.

## 7. Quêtes

Implémenter le moteur permettant :

- de démarrer une quête
- suivre sa progression
- vérifier les objectifs
- détecter la complétion
- attribuer une récompense

## 8. Chargement du monde

- charger le fichier YAML ou JSON
- créer les structures Go correspondantes
- vérifier les références :
  - exits valides
  - NPC existants
  - objets existants
  - quêtes valides

## 9. Logging

Logger :

- connexions
- déconnexions
- IP
- timestamps
- commandes reçues
- paramètres
- réponses serveur
- erreurs
- mouvements d'objets
- combats
- interactions NPC
- progression des quêtes

Préférer un format structuré comme JSON.

Niveaux :

- `INFO`
- `WARN`
- `ERROR`

Ajouter une détection simple :

- command flooding
- connexions trop rapides

## 10. Client CLI

Le CLI doit :

- se connecter au serveur
- lire les commandes utilisateur
- envoyer les commandes au serveur
- recevoir les réponses
- recevoir les événements asynchrones
- continuer à recevoir des messages pendant que l'utilisateur écrit

Architecture possible :

```text
goroutine 1 -> lecture clavier -> serveur
goroutine 2 -> serveur -> affichage terminal
```

---

# Personne 2 — GUI + World Design

## 1. Client GUI

Choisir une vraie technologie graphique.

Exemples :

- Fyne
- GTK
- Qt
- interface web

`curses` ne compte pas comme GUI.

## 2. Connexion réseau

- connexion TCP au serveur
- envoi des commandes RFC
- réception des réponses
- réception des événements `EVT`
- gestion des déconnexions

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

## 4. Actions GUI

Prévoir des boutons ou contrôles pour :

- LOOK
- MOVE
- TAKE
- DROP
- TALK
- ATTACK
- STATUS
- QUEST
- QUESTS
- WHO
- GROUP
- QUIT

## 5. Chat

Séparer visuellement :

- chat Global
- chat Room
- chat Group
- logs / événements

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

---

# Décisions à prendre ensemble

## 1. Format du protocole

Définir précisément :

- format `OK`
- format `ERR`
- format `EVT`
- format des réponses JSON
- noms des identifiants
- conventions pour rooms/items/NPC/quests

Exemple :

```text
C: MOVE north
S: OK room=loc.tavern
S: EVT ROOM PRESENCE ENTER alice
```

## 2. Structures communes

Définir ensemble les principales structures :

- `Player`
- `Room`
- `Item`
- `NPC`
- `Quest`

Exemple :

```go
type Room struct {
    ID          string
    Name        string
    Description string
    Exits       map[string]string
    Items       []string
    NPCs        []string
}
```

## 3. Combat

Décider ensemble :

- formule de dégâts
- initiative
- HP des ennemis
- fonctionnement de DEFEND
- fonctionnement de FLEE
- respawn
- récompenses

Personne 1 implémente le moteur.

Personne 2 participe au game design et à l'affichage GUI.

## 4. Quêtes

Personne 2 peut concevoir les quêtes.

Exemple :

```text
Quest 1
1. Trouver Ancient Key
2. Parler au Guard
3. Rendre la clé
4. Recevoir une récompense
```

Personne 1 implémente le moteur permettant de vérifier ces étapes.

---

# Répartition du README

## Personne 1

- Architecture
- Protocol Implementation
- Server Logging
- Combat System
- Building and Running : serveur + CLI
- tests serveur

## Personne 2

- World Design
- GUI
- Quest System / quest design
- Building and Running : GUI
- tests GUI

## Ensemble

- Description
- Resources
- utilisation de l'IA
- Group Contributions
- Testing final
- relecture générale

Le README doit être rédigé en anglais.

---

# Ordre de développement recommandé

## Phase 1 — Ensemble

- lire le RFC
- décider des structures
- décider du format JSON/YAML
- définir les conventions Git
- définir les IDs

## Phase 2

### Personne 1

Implémenter :

- serveur TCP
- CONNECT
- LOOK
- MOVE
- CHAT

### Personne 2

Créer :

- squelette du monde
- maquette GUI

## Phase 3 — Première intégration

Objectif minimal :

```text
GUI
  ↓
CONNECT alice
  ↓
LOOK
  ↓
MOVE north
  ↓
affichage de la nouvelle salle
```

## Phase 4

Ajouter :

- objets
- TAKE / DROP
- inventory
- NPC
- TALK
- WHO
- GROUP

## Phase 5

Ajouter :

- combat
- quêtes

## Phase 6

Finaliser :

- logging
- gestion des erreurs
- déconnexions
- flood protection
- tests multijoueur

## Phase 7

- README
- lint
- nettoyage du code
- tests finaux
- préparation à l'évaluation

---

# Répartition approximative de la charge

| Personne 1 — Serveur + CLI | Personne 2 — GUI + Monde |
|---|---|
| Serveur TCP — 25 % | Réseau GUI — 15 % |
| Protocole / parser — 15 % | Interface graphique — 30 % |
| Game engine — 20 % | World Design — 20 % |
| CLI — 10 % | Intégration GUI — 15 % |
| Combat / Quests backend — 15 % | Combat / Quests UX — 10 % |
| Logging / tests — 15 % | Tests / documentation — 10 % |

---

# Architecture cible

```text
                         WORLD
                      YAML / JSON
                          |
                          v
                 +------------------+
                 |      SERVER      |
                 |                  |
                 |    GameState     |
                 |                  |
                 | - Players        |
                 | - Rooms          |
                 | - Items          |
                 | - NPCs           |
                 | - Quests         |
                 | - Combat         |
                 +--------+---------+
                          |
               TCP        |        TCP
                          |
          +---------------+---------------+
          |                               |
          v                               v
     +---------+                     +---------+
     |   CLI   |                     |   GUI   |
     +---------+                     +---------+
```

Le **serveur reste la source de vérité**.  
Le CLI et la GUI ne doivent pas maintenir leur propre version indépendante du monde.
