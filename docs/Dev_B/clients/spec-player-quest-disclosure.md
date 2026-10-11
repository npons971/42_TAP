---
title: 'Quests repliables dans PLAYER'
type: 'feature'
created: '2026-10-11'
status: 'done'
route: 'one-shot'
---

# Quests repliables dans PLAYER

## Intent

**Problem:** Les détails de toutes les quests encombrent le widget PLAYER.

**Approach:** Chaque quest affiche par défaut son nom et une flèche. Le contrôle natif `details`/`summary` permet de déplier ou replier indépendamment le statut, la description, les objectifs, la récompense et les actions, au clic ou au clavier. La flèche pointe vers la droite lorsque la quest est repliée, vers le bas lorsqu’elle est ouverte. Les éléments restent indexés par identifiant de quest pour préserver l’ouverture lors des mises à jour.

## Suggested Review Order

- Vérifier le regroupement des détails sous le contrôle natif de chaque quest.
  [App.svelte:726](../../../Project/client-gui/frontend/src/App.svelte#L726)

- Vérifier la flèche, le focus clavier et le retour à la ligne des titres longs.
  [style.css:177](../../../Project/client-gui/frontend/src/style.css#L177)
