# TAP — Aris (Dev A) : Serveur + CLI

Ce document regroupe les responsabilités et tâches attribuées à **Aris (Dev A)**.
Pour les décisions d'équipe, le protocole partagé et l'architecture globale, se référer à [TAP_taches_communes.md](../Commun/TAP_taches_communes.md).
Pour la partie de Novanns (Dev B), voir [TAP_Novanns_Dev_B.md](../Dev_B/TAP_Novanns_Dev_B.md).

---

## État de réalisation

La partie serveur et CLI est implémentée : les 15 commandes obligatoires,
combat au tour par tour avec DEFEND/FLEE, groupes/CHAT GROUP, progression des
deux quêtes, objets uniques/USE, chargement du monde, logs JSON et détection
de flood/connexions rapides. Le CLI tourne avec make run-cli et reçoit les
événements pendant la saisie. Les tests réseau, de concurrence et de terminal
sont décrits dans [aris_acceptance.md](../Commun/testing/aris_acceptance.md).

Le RFC externe fourni à la racine a été comparé au serveur et les écarts ont
été corrigés : voir [rfc_conformance.md](../Commun/protocol/rfc_conformance.md).
Les tests vérifient les trames standard, les événements et les extensions
explicites. Le backend GUI et les handlers du frontend sont testés et le
frontend compile. Une session visuelle Wails et les tests avec les clients
indépendants d'autres groupes restent des validations communes.
La liste ci-dessous conserve le périmètre de responsabilités.

## 1. Serveur TCP

- Initialiser le projet Go
- Ouvrir un serveur TCP
- Accepter plusieurs connexions simultanées
- Gérer chaque client dans une goroutine
- Gérer les déconnexions proprement
- Maintenir la liste des joueurs connectés

---

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

---

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

---

## 4. Concurrence

- protéger les données partagées avec `sync.Mutex` / `sync.RWMutex`
- ou utiliser des channels lorsque pertinent
- éviter les race conditions
- supporter plusieurs joueurs agissant simultanément
- ne pas faire planter les broadcasts lorsqu'un client se déconnecte

---

## 5. Système d'objets

- objets uniques dans le monde
- `TAKE` retire l'objet de la salle
- `DROP` remet l'objet dans la salle
- empêcher les duplications
- recherche par ID ou nom affiché
- supporter les noms composés de plusieurs mots

---

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

Le fonctionnement exact du combat doit être conçu et documenté (en concertation avec Novanns).

---

## 7. Quêtes

Implémenter le moteur permettant :

- de démarrer une quête
- suivre sa progression
- vérifier les objectifs
- détecter la complétion
- attribuer une récompense

*(Le contenu et le design narratif des quêtes sont conçus par Novanns).*

---

## 8. Chargement du monde

- charger le fichier YAML ou JSON (produit par Novanns)
- créer les structures Go correspondantes
- vérifier les références :
  - exits valides
  - NPC existants
  - objets existants
  - quêtes valides

---

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

---

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

## Répartition du README (Sections d'Aris)

- Architecture
- Protocol Implementation
- Server Logging
- Combat System
- Building and Running : serveur + CLI
- tests serveur

*(Le README doit être rédigé en anglais).*

---

## Ordre de développement recommandé (Planning Aris)

- **Phase 1 — Cadrage d'équipe** : lecture RFC, décision des structures, format JSON/YAML, conventions Git, IDs.
- **Phase 2 — Socle serveur** : serveur TCP, commandes de base (`CONNECT`, `LOOK`, `MOVE`, `CHAT`).
- **Phase 3 — Première intégration** : validation de la communication avec le GUI de Novanns (`CONNECT alice` -> `LOOK` -> `MOVE north` -> affichage).
- **Phase 4 — Fonctionnalités avancées** : objets (`TAKE`/`DROP`, inventory), NPC (`TALK`), `WHO`, `GROUP`.
- **Phase 5 — Systèmes avancés** : moteur de combat, moteur de quêtes.
- **Phase 6 — Robustesse & Sécurité** : logging, gestion des erreurs, déconnexions, flood protection, tests multijoueur.
- **Phase 7 — Finalisation** : rédaction des sections du README, lint, nettoyage du code, tests finaux, préparation à l'évaluation.

---

## Répartition approximative de la charge

| Domaine | Charge |
|---|---|
| Serveur TCP | 25 % |
| Protocole / parser | 15 % |
| Game engine | 20 % |
| CLI | 10 % |
| Combat / Quests backend | 15 % |
| Logging / tests | 15 % |
