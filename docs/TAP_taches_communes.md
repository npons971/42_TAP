# TAP — Tâches Communes et Décisions d'Équipe

Ce document regroupe les tâches partagées, l'architecture globale, les décisions techniques communes ainsi que l'ordre de développement recommandé pour le binôme.

- **Aris (Dev A)** : Serveur + client CLI — voir [TAP_Aris_Dev_A.md](file:///home/almarc/42/42_TAP/TAP_Aris_Dev_A.md)
- **Novanns (Dev B)** : Client GUI + World Design — voir [TAP_Novanns_Dev_B.md](file:///home/almarc/42/42_TAP/TAP_Novanns_Dev_B.md)

---

## Architecture cible

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

---

## Décisions à prendre ensemble

### 1. Format du protocole

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

### 2. Structures communes

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

### 3. Combat

Décider ensemble :

- formule de dégâts
- initiative
- HP des ennemis
- fonctionnement de `DEFEND`
- fonctionnement de `FLEE`
- respawn
- récompenses

> **Répartition du rôle :**  
> - **Aris (Dev A)** implémente le moteur.  
> - **Novanns (Dev B)** participe au game design et à l'affichage GUI.

### 4. Quêtes

Novanns (Dev B) peut concevoir les quêtes.

Exemple :

```text
Quest 1
1. Trouver Ancient Key
2. Parler au Guard
3. Rendre la clé
4. Recevoir une récompense
```

> **Répartition du rôle :**  
> - **Novanns (Dev B)** conçoit l'histoire et les objectifs des quêtes.  
> - **Aris (Dev A)** implémente le moteur permettant de vérifier ces étapes.

---

## Répartition du README

Le README doit être rédigé en anglais.

### Aris (Dev A)
- Architecture
- Protocol Implementation
- Server Logging
- Combat System
- Building and Running : serveur + CLI
- Tests serveur

### Novanns (Dev B)
- World Design
- GUI
- Quest System / quest design
- Building and Running : GUI
- Tests GUI

### Ensemble (Tâches communes)
- Description
- Resources
- Utilisation de l'IA
- Group Contributions
- Testing final
- Relecture générale

---

## Ordre de développement recommandé

### Phase 1 — Ensemble
- lire le RFC
- décider des structures
- décider du format JSON/YAML
- définir les conventions Git
- définir les IDs

### Phase 2 — Développement initial
- **Aris (Dev A)** : Serveur TCP, `CONNECT`, `LOOK`, `MOVE`, `CHAT`
- **Novanns (Dev B)** : Squelette du monde, maquette GUI

### Phase 3 — Première intégration
Objectif minimal commun :
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

### Phase 4 — Objets & Entités
Ajouter :
- objets
- `TAKE` / `DROP`
- inventory
- NPC
- `TALK`
- `WHO`
- `GROUP`

### Phase 5 — Gameplay avancé
Ajouter :
- combat
- quêtes

### Phase 6 — Robustesse & Multijoueur
Finaliser :
- logging
- gestion des erreurs
- déconnexions
- flood protection
- tests multijoueur

### Phase 7 — Finalisation du projet
- README complet
- lint
- nettoyage du code
- tests finaux
- préparation à l'évaluation

---

## Répartition approximative de la charge

| Aris (Dev A) — Serveur + CLI | Novanns (Dev B) — GUI + Monde |
|---|---|
| Serveur TCP — 25 % | Réseau GUI — 15 % |
| Protocole / parser — 15 % | Interface graphique — 30 % |
| Game engine — 20 % | World Design — 20 % |
| CLI — 10 % | Intégration GUI — 15 % |
| Combat / Quests backend — 15 % | Combat / Quests UX — 10 % |
| Logging / tests — 15 % | Tests / documentation — 10 % |
