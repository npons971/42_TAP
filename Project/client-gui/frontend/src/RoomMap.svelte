<script>
  import { tick } from 'svelte';
  import Icon from './Icon.svelte';
  import PixelScene from './PixelScene.svelte';
  import PixelSprite from './PixelSprite.svelte';
  import { buildLocalMap, buildLocalMarkers, getMapAppearance } from './local-map.js';
  export let room;
  export let names = {};
  export let username = '';
  export let inCombat = false;
  export let onmove = () => {};
  export let onselect = () => {};
  export let expanded = false;
  let mapContainer;
  $: exits = buildLocalMap(room, names);
  $: markers = buildLocalMarkers(room);
  $: kind = getMapAppearance(room);
  $: if (expanded) tick().then(() => {
    if (expanded) mapContainer?.querySelector('.map-expand')?.focus();
  });

  function handleMapKey(event) {
    if (!expanded) return;
    if (event.key === 'Escape') {
      expanded = false;
      mapContainer.querySelector('.map-expand')?.focus();
    }
    if (event.key === 'Tab') {
      const buttons = [...mapContainer.querySelectorAll('button:not([disabled])')];
      const first = buttons[0], last = buttons.at(-1);
      if (!buttons.includes(document.activeElement) || (!event.shiftKey && document.activeElement === last)) {
        event.preventDefault(); first?.focus();
      } else if (event.shiftKey && document.activeElement === first) {
        event.preventDefault(); last?.focus();
      }
    }
  }
</script>

<svelte:window on:keydown={handleMapKey} />

<div class="zone-map" class:map-expanded={expanded} class:map-combat={inCombat} bind:this={mapContainer} role={expanded ? 'dialog' : 'region'} aria-modal={expanded ? 'true' : undefined} aria-label="Local area map" tabindex="-1">
  <div class="map-toolbar">
    <button class="map-expand" on:click={() => expanded = !expanded} aria-label={expanded ? 'Close expanded map' : 'Expand map'} aria-pressed={expanded} title={expanded ? 'Close map (Escape)' : 'Expand map'}><Icon name={expanded ? 'close' : 'expand'} size={16} /></button>
  </div>
  <div class="map-scene">
    <PixelScene {exits} {kind} />
    {#each exits as exit (exit.direction)}
      <button class="map-exit btn-exit" class:map-visited={exit.visited} style={`left:${exit.x / 9}%;top:${exit.y / 4}%`} disabled={inCombat} on:click={() => onmove(exit.direction)} title={`Move ${exit.direction}: ${exit.label}`} aria-label={`Move ${exit.direction} to ${exit.label}`}>
        <span class="dir-tag">{exit.direction}</span><span class="dest-tag">{exit.label}</span>
      </button>
    {/each}
    <span class="map-player"><PixelSprite name="hero" size={32} /></span>
    <span class="map-you">{username || 'You'}</span>
    {#each markers as marker, index (`${marker.kind}:${marker.id}:${index}`)}
      <button class="map-entity" class:map-enemy={marker.role === 'enemy'} class:map-quest={marker.role === 'quest_giver'} style={`left:${marker.x / 9}%;top:${marker.y / 4}%`} title={marker.name} aria-label={`Inspect ${marker.name}`} on:click={() => { expanded = false; onselect(marker.kind, marker.id); }}><PixelSprite name={marker.kind === 'npc' ? marker.role === 'enemy' ? 'enemy' : 'villager' : marker.obtainable ? 'chest' : 'rock'} size={28}/>{#if marker.role === 'quest_giver'}<span class="map-quest-hint">!</span>{/if}</button>
    {/each}
  </div>
  <div class="map-legend">{inCombat ? 'In combat. Movement blocked.' : exits.length ? 'Choose a direction.' : 'No exits.'}</div>
</div>
