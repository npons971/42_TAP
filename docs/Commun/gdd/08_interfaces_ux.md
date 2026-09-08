# 08 — Interfaces & Expérience Utilisateur (CLI & GUI)

Le projet impose le développement de **deux clients distincts et interchangeables** pour interagir avec le serveur TCP :
1. Un client en ligne de commande (**CLI Client**)
2. Un client graphique interactif (**GUI Client**)

---

## 1. Client CLI (Interface Console)

### Principes & Ergonomie
Le client CLI propose l'expérience rétro historique des MUDs des années 90 :
- Affichage textuel direct, clair et lisible.
- Découplage strict entre la saisie clavier de l'utilisateur et l'affichage des flux entrants.
- **Réactivité en temps réel** : l'utilisateur continue de recevoir les messages des autres joueurs et les événements (`EVT`) même lorsqu'il est en train de taper une commande.

### Architecture I/O du CLI
```text
                  +---------------------------+
                  |         TERMINAL          |
                  +-------------+-------------+
                                |
             +------------------+------------------+
             |                                     |
             v                                     v
   [Goroutine 1 : Saisie]                [Goroutine 2 : Écoute]
  bufio.Reader / os.Stdin                 TCP Socket <- Serveur
             |                                     |
             v                                     v
   Envoi commande -> TCP                  Affichage fluide à l'écran
```

---

## 2. Client GUI (Interface Graphique)

### Choix Technologique
- Développé avec une boîte à outils graphique native (ex: **Fyne**, **Qt**, **GTK**) ou une interface web moderne.
- *(Rappel : `curses` est explicitement interdit comme GUI par le sujet).*

### Maquette / Wireframe de l'Interface

```text
+-----------------------------------------------------------------------------------------+
| TAP GUI — [Serveur : 127.0.0.1:4242] | Joueur : Alice | Connectés : 4 (Dans la pièce : 2)|
+-------------------------------------------------------------+---------------------------+
| [ VUE DE LA SALLE ]                                         | [ ÉTAT DU JOUEUR ]        |
| Salle : Marché Central                                      | Nom : Alice               |
| Une allée animée bordée d'étals colorés...                 | HP  : [████████░░] 80/100 |
|                                                             | État: HORS_COMBAT         |
| Sorties : [ Ouest: Place ] [ Nord: Taverne ] [ Est: Ruines ]|                           |
|-------------------------------------------------------------+---------------------------+
| [ ENTITÉS DE LA SALLE ]                                     | [ INVENTAIRE ]            |
| Objets au sol :                                             | • Épée Rouillée [DROP]    |
| • Pomme Fraîche        [ Prendre (TAKE) ]                   | • Clé Ancienne  [DROP]    |
| PNJ présents :                                              |                           |
| • Marchand Ambulant    [ Parler (TALK) ]                    |                           |
|-------------------------------------------------------------+---------------------------+
| [ COMMUNICATIONS & LOGS ]                                                               |
| [ Onglet : Global ]  [ Onglet : Salle ]  [ Onglet : Groupe ]  [ Onglet : Logs Système ] |
|-----------------------------------------------------------------------------------------|
| [14:02] <Bob> Bonjour Alice !                                                           |
| [14:03] Serveur : Bob s'est déplacé vers le nord.                                       |
| > Saisir un message...                                                        [ Envoyer ]|
+-----------------------------------------------------------------------------------------+
| [ BARRE D'ACTIONS RAPIDES ]                                                             |
| [ LOOK ] [ STATUS ] [ QUESTS ] [ WHO ] [ GROUP ] [ DEFEND ] [ FLEE ] [ QUITTER ]        |
+-----------------------------------------------------------------------------------------+
```

---

## 3. Spécifications Fonctionnelles GUI (Exigences Sujet)

1. **Affichage dynamique de la salle** :
   - Mise à jour instantanée du nom, de la description et des sorties disponibles.
   - Les sorties sont cliquables pour déclencher `MOVE <direction>`.
2. **Gestion des Objets & Boutons d'Action** :
   - Liste des objets présents dans la salle avec bouton `TAKE`.
   - Liste des objets de l'inventaire avec bouton `DROP`.
   - Mise à jour automatique de la vue de la salle dès qu'un objet est pris ou posé.
   - Support des noms complets multi-mots et des IDs techniques.
3. **Interactions PNJ & Combat** :
   - Bouton `TALK` ouvrant une boîte de dialogue ou affichant les répliques du PNJ.
   - Bouton `ATTACK` pour engager les ennemis présents dans la salle.
4. **Séparation des Vues de Chat et de Logs** :
   - Onglets ou fenêtres séparées pour `Global`, `Room`, `Group` et `Logs`.
5. **Indicateurs de Présence** :
   - Compteurs temps réel : nombre de joueurs dans la salle et nombre total sur le serveur (via `WHO` et `EVT`).
