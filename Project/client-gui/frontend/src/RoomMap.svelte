<script>
  import Icon from './Icon.svelte';
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
  $: appearance = getMapAppearance(room);
  function closeOnEscape(event) {
    if (!expanded) return;
    if (event.key === 'Escape') {
      expanded = false;
      mapContainer.querySelector('.map-expand')?.focus();
    }
    if (event.key === 'Tab') {
      const buttons = [...mapContainer.querySelectorAll('button:not([disabled])')];
      const first = buttons[0], last = buttons.at(-1);
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    }
  }
</script>

<svelte:window on:keydown={closeOnEscape} />

<div class="zone-map" class:map-expanded={expanded} class:map-combat={inCombat} bind:this={mapContainer} role={expanded ? 'dialog' : 'region'} aria-modal={expanded ? 'true' : undefined} aria-label="Local area map" tabindex="-1">
  <div class="map-toolbar">
    <span><Icon name="map" size={14} /> LOCAL AREA <span class="map-live-dot"></span></span>
    <button class="map-expand" on:click={() => expanded = !expanded} aria-label={expanded ? 'Close expanded map' : 'Expand map'} aria-pressed={expanded} title={expanded ? 'Close map (Escape)' : 'Expand map'}><Icon name={expanded ? 'close' : 'expand'} size={17} /></button>
  </div>
  <div class="map-scene">
    <svg class="map-terrain" viewBox="0 0 900 400" preserveAspectRatio="none" aria-hidden="true">
      <defs>
        <radialGradient id="map-ground"><stop stop-color={appearance.ground}/><stop offset="1" stop-color="#17272b"/></radialGradient>
        <pattern id="map-grid" width="60" height="30" patternUnits="userSpaceOnUse"><path d="M0 15 30 0 60 15 30 30Z" fill="none" stroke="#8eb694" stroke-opacity=".07"/></pattern>
        <pattern id="map-stone" width="36" height="18" patternUnits="userSpaceOnUse"><path d="M0 9 18 0 36 9 18 18Z" fill="#697263" stroke="#8c9179" stroke-width=".5"/><path d="M0 9v9m18-18v9" stroke="#3d4f49" stroke-width=".5"/></pattern>
        <symbol id="map-tree" viewBox="0 0 50 80"><ellipse cx="25" cy="71" rx="20" ry="7" fill="#071b1b" opacity=".45"/><path d="M23 47h5v24h-5Z" fill="#725d45"/><path d="m25 1 20 40H5Z" fill="#29473c"/><path d="m25 16 23 39H2Z" fill="#345949"/><path d="m25 30 25 34H0Z" fill="#416750"/><path d="m25 30 0 34H0Z" fill="#537759" opacity=".45"/></symbol>
        <symbol id="map-house" viewBox="0 0 70 65"><path d="m7 32 30-15 27 15v21L37 65 7 50Z" fill="#a1a086"/><path d="m37 41 27-9v21L37 65Z" fill="#657263"/><path d="m0 30 28-30 42 21-33 25Z" fill="#806958"/><path d="m37 46 33-25-6 13-27 19Z" fill="#564c44"/><path d="M17 39v11l9 4V43Z" fill="#34463f"/><path d="m45 43 9-4v8l-9 4Z" fill="#ceaf72"/><path d="M28 0 0 30l8 4L35 4Z" fill="#a18163"/></symbol>
        <symbol id="map-rock" viewBox="0 0 40 25"><path d="m1 16 10-13 18-3 10 17-16 8Z" fill="#52635b"/><path d="m11 3 18-3-6 16-22 0Z" fill="#718173"/></symbol>
        <symbol id="map-ruin" viewBox="0 0 80 60"><path d="m5 45 35-17 34 17-35 15Z" fill="#4b574d"/><path d="M12 15 24 9v37l-12 6ZM57 14l12 5v34l-12-6Z" fill="#8d9380"/><path d="M12 15 7 12l12-7 5 4Zm45-1 6-4 12 5-6 4Z" fill="#b0ae95"/><path d="M24 15q17-23 33 0v9q-17-23-33 0Z" fill="#91937e"/></symbol>
      </defs>
      <rect width="900" height="400" fill="url(#map-ground)"/>
      <rect width="900" height="400" fill="url(#map-grid)"/>
      <g fill="none" stroke="#668077" stroke-opacity=".13">
        <path d="M-40 85Q120 12 253 83T535 46T937 79M-40 101Q120 28 253 99T535 62T937 95M-40 117Q120 44 253 115T535 78T937 111"/>
        <path d="M-30 305Q140 215 320 309T637 295T930 345M-30 321Q140 231 320 325T637 311T930 361M-30 337Q140 247 320 341T637 327T930 377"/>
      </g>
      <g opacity=".85">
        {#each [[62,80,48],[98,58,56],[44,246,50],[88,279,55],[798,45,52],[839,77,58],[810,267,62],[763,304,50],[300,18,36],[567,304,44],[254,279,43],[588,25,41]] as [x,y,size]}
          <use href={appearance.kind === 'underground' || appearance.kind === 'interior' ? '#map-rock' : '#map-tree'} {x} {y} width={size} height={size * 1.6}/>
        {/each}
        <use href="#map-rock" x="245" y="170" width="37" height="24"/><use href="#map-rock" x="657" y="244" width="40" height="25"/>
      </g>
      {#each exits as exit (exit.direction)}
        <path d={`M450 200 Q${(450 + exit.x) / 2 + 18} ${(200 + exit.y) / 2} ${exit.x} ${exit.y}`} fill="none" stroke="#0b1d20" stroke-width="24"/>
        <path d={`M450 200 Q${(450 + exit.x) / 2 + 18} ${(200 + exit.y) / 2} ${exit.x} ${exit.y}`} fill="none" stroke={exit.visited ? '#879877' : '#6b7765'} stroke-width="17" stroke-linecap="round" opacity=".68"/>
        <path d={`M450 200 Q${(450 + exit.x) / 2 + 18} ${(200 + exit.y) / 2} ${exit.x} ${exit.y}`} fill="none" stroke="#ddd3a5" stroke-width="1" stroke-dasharray="3 10" opacity=".55"/>
        <path d={`m${exit.x} ${exit.y - 25} 61 29-61 30-61-30Z`} fill="#4c6653" stroke="#7c9772" stroke-opacity=".45"/>
      {/each}
      <path d="m450 119 158 81-158 81-158-81Z" fill="#112b29" transform="translate(0 8)"/>
      <path d="m450 119 158 81-158 81-158-81Z" fill={appearance.floor} stroke="#9da788" stroke-width="2"/>
      <path d="m450 119 158 81-158 81-158-81Z" fill="url(#map-stone)" opacity={appearance.kind === 'garden' ? '.18' : '.4'}/>
      <path d="m450 129 138 71-138 71-138-71Z" fill="none" stroke="#b0b398" stroke-opacity=".3"/>
      {#if appearance.kind === 'settlement' || appearance.kind === 'interior'}
        <use href="#map-house" x="390" y="86" width="67" height="64"/>
        <use href="#map-house" x="455" y="101" width="49" height="47" opacity=".8"/>
      {:else if appearance.kind === 'ruins'}
        <use href="#map-ruin" x="413" y="85" width="75" height="65"/>
      {:else if appearance.kind === 'underground'}
        <use href="#map-rock" x="395" y="122" width="48" height="27"/><use href="#map-rock" x="465" y="115" width="55" height="30"/>
      {:else}
        <use href="#map-tree" x="416" y="88" width="35" height="65"/><use href="#map-tree" x="461" y="102" width="30" height="50"/>
      {/if}
      <ellipse cx="450" cy="203" rx="26" ry="13" fill="#e5c47b" fill-opacity=".15" stroke="#e5c47b" stroke-opacity=".7"/>
      <ellipse cx="450" cy="203" rx="38" ry="19" fill="none" stroke="#e5c47b" stroke-opacity=".25" stroke-dasharray="2 5"/>
      <g transform="translate(438 169)"><ellipse cx="12" cy="35" rx="10" ry="4" fill="#132723"/><path d="m7 14-6 20h22l-6-20Z" fill="#d6ad5e"/><path d="m7 14-6 20h9V14Z" fill="#a1783f"/><path d="M8 30v8m9-8v8" stroke="#192a2a" stroke-width="4"/><circle cx="12" cy="7" r="6" fill="#dfc49b"/><path d="M6 5a6 6 0 0 1 12 0Z" fill="#304038"/></g>
    </svg>

    {#each exits as exit (exit.direction)}
      <button class="map-exit btn-exit" class:map-visited={exit.visited} style={`left:${exit.x / 9}%;top:${exit.y / 4}%`} disabled={inCombat} on:click={() => onmove(exit.direction)} title={`Move ${exit.direction}: ${exit.label}`} aria-label={`Move ${exit.direction} to ${exit.label}`}>
        <span class="map-node-icon"><Icon name={exit.visited ? 'landmark' : 'pin'} size={18}/></span>
        <span class="map-node-text"><span class="dir-tag">{exit.direction}</span><span class="dest-tag">{exit.label}</span></span>
        <Icon name="arrow" size={13}/>
      </button>
    {/each}
    <span class="map-you"><span class="map-you-dot"></span> {username || 'You'} <small>YOU ARE HERE</small></span>
    {#each markers as marker, index (`${marker.kind}:${marker.id}:${index}`)}
      <button class="map-entity" class:map-enemy={marker.role === 'enemy'} class:map-quest={marker.role === 'quest_giver'} style={`left:${marker.x / 9}%;top:${marker.y / 4}%`} title={marker.name} aria-label={`Inspect ${marker.name}`} on:click={() => { expanded = false; onselect(marker.kind, marker.id); }}><Icon name={marker.icon} size={17}/></button>
    {/each}
    <div class="map-orientation" aria-hidden="true"><span>N</span><Icon name="pin" size={15}/></div>
  </div>
  <div class="map-legend"><span><i class="legend-you"></i> Your position</span><span><i class="legend-path"></i> Available path</span><span><i class="legend-quest"></i> Character / object</span><span class="map-hint">{inCombat ? 'Finish combat to travel' : exits.length ? 'Select a destination to travel' : 'No exits from this location'}</span></div>
</div>
