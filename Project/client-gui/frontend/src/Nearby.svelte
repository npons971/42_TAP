<script>
  export let room;
  export let username = '';
  export let selectedEntity = '';
  export let inCombat = false;
  export let onaction = () => {};
</script>

<div class="nearby-columns" aria-label="Nearby">
  <section class="nearby-column" aria-labelledby="nearby-characters">
    <h3 id="nearby-characters">People <span class="count">{room.npcs.length}</span></h3>
    <div class="nearby-list">
      {#each room.npcs as npc}
        <div id={`nearby-${npc.id}`} class="nearby-card" class:selected={selectedEntity === npc.id} tabindex="-1">
          <div class="entity-name" class:hostile={npc.role === 'enemy'}><strong>{npc.name}</strong></div>
          <p>{npc.description || (npc.role === 'enemy' ? 'Enemy' : npc.role === 'quest_giver' ? 'Quest giver' : 'Friendly')}</p>
          <div class="npc-buttons">
            {#if npc.role !== 'enemy'}<button class="btn-action btn-talk" disabled={inCombat} on:click={() => onaction('Talk', npc.id)}>Talk</button>{/if}
            {#if npc.role === 'quest_giver' || npc.unknownRole}<button class="btn-action btn-quest" disabled={inCombat} on:click={() => onaction('Quest', npc.id)}>Quest</button>{/if}
            {#if npc.role === 'enemy' || npc.unknownRole}<button class="btn-action btn-attack" on:click={() => onaction('Attack', npc.id)}>Attack</button>{/if}
          </div>
        </div>
      {:else}<p class="empty-list">No characters.</p>{/each}
    </div>
  </section>
  <section class="nearby-column" aria-labelledby="nearby-objects">
    <h3 id="nearby-objects">Objects <span class="count">{room.items.length}</span></h3>
    <div class="nearby-list">
      {#each room.items as item}
        <div id={`nearby-${item.id}`} class="nearby-card" class:selected={selectedEntity === item.id} tabindex="-1">
          <div class="entity-name"><strong>{item.name}</strong></div>
          {#if item.description}<p>{item.description}</p>{/if}
          {#if item.obtainable}<button class="btn-action btn-take" disabled={inCombat} on:click={() => onaction('Take', item.id)}>Take</button>{:else}<span class="scenery-tag">Scenery</span>{/if}
        </div>
      {:else}<p class="empty-list">No objects.</p>{/each}
    </div>
  </section>
  <section class="nearby-column" aria-labelledby="nearby-players">
    <h3 id="nearby-players">Players <span class="count">{room.players.length}</span></h3>
    <div class="nearby-list">
      {#each room.players as player}
        <div class="nearby-card" class:self={player === username}>
          <div class="entity-name"><strong>{player}</strong></div>
          <p>{player === username ? 'You' : 'Adventurer'}</p>
        </div>
      {:else}<p class="empty-list">No players.</p>{/each}
    </div>
  </section>
</div>
