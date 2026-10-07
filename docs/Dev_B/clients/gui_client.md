# GUI Client Specifications

> **Document Status**: `🔴 TO_FILL (Toolkit Choice Pending)`  
> **Assigned to**: Novanns (Dev B)  
> **Last Updated**: 2026-10-07

The GUI client provides an accessible, rich visual experience for the TAP adventure. It must remain interchangeable with the CLI client and strictly follow the RFC 42TAP protocol over a TCP connection.

---

## 1. GUI Toolkit Selection `🔴 TO_FILL`

The 42 subject permits any real graphical toolkit (curses is forbidden).

| Toolkit Candidate | Language | Pros | Cons / Dependencies |
|---|---|---|---|
| **Fyne** | Go | Pure Go, clean widgets, cross-platform, fast setup | Requires OpenGL/CGo drivers |
| **Wails (Webview / Svelte / React)** | Go + HTML/CSS | Modern CSS styling, easy layout and tab handling | Webview runtime required |
| **GTK+ / Qt** | Go bindings / C++ | Native OS look, comprehensive widgets | Heavy CGo bindings & library install |

> **Decision**: `🔴 TO_FILL` — Record chosen toolkit, version, and rationale for the README.

---

## 2. Component Layout & Visual Hierarchy

```text
+-------------------------------------------------------------------------------+
| Header: Server IP:Port | Player: Alice | Room Players: 2 | Server Players: 6  |
+---------------------------------------+---------------------------------------+
| [ROOM VIEW]                           | [CHARACTER & INVENTORY]               |
| Name: Village Square                  | Health: [████████░░] 80/100 HP        |
| Description: A lively cobblestone...  | Status: HORS_COMBAT                   |
|                                       | Inventory:                            |
| Available Exits:                      | - Rusty Sword        [ DROP ]         |
| [ North: Garden ] [ East: Market ]    | - Rare Herbs         [ DROP ]         |
+---------------------------------------+---------------------------------------+
| [ROOM ENTITIES]                       | [QUEST TRACKER]                       |
| Items on ground:                      | Active: The Apothecary's Remedy       |
| - Fresh Apple           [ TAKE ]      | Objective: Bring herbs to Herbalist   |
| NPCs present:                         |                                       |
| - Village Guard         [ TALK ]      |                                       |
+---------------------------------------+---------------------------------------+
| [CHAT & LOG TABS]                                                             |
| [ Tab: Global Chat ] [ Tab: Room Chat ] [ Tab: Group Chat ] [ Tab: Logs ]     |
| Text Area...                                                                  |
| [ Input Field... ]                                                 [ Send ]   |
+-------------------------------------------------------------------------------+
| [ACTION BUTTONS]                                                              |
| [LOOK] [STATUS] [QUESTS] [WHO] [GROUP] [ATTACK] [DEFEND] [FLEE] [QUIT]        |
+-------------------------------------------------------------------------------+
```

---

## 3. Mandatory GUI Features Checklist

- [ ] **Real-Time Room Updates**: Parse `EVT ROOM PRESENCE ENTER <username>`, `EVT ROOM PRESENCE LEAVE <username>`, `EVT ROOM ITEM TAKE <username> <item_id>`, and `EVT ROOM ITEM DROP <username> <item_id>`. Update presence immediately and refresh `LOOK` after item events when the item descriptor is needed. See the [shared protocol proposal](../../Commun/protocol/rfc_syntax.md).
- [ ] **Item Buttons**: Single-click `TAKE` on ground items and `DROP` on inventory items.
- [ ] **Separation of Views**: Independent tabs for Global Chat, Room Chat, Group Chat, and System/Debug Logs.
- [ ] **NPC Dialog Display**: Pop-up modal or dedicated text frame displaying the dialogue when `TALK` is triggered.
- [ ] **Live Player Counters**: Real-time counter badge for players in room and on server.
