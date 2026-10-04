<script>
  import { onMount } from 'svelte';
  import PixelSprite from './PixelSprite.svelte';
  export let exits = [];
  export let kind = 'settlement';
  let terrain;
  let width = 225, height = 100;
  const trees = [[24,26],[201,26],[24,74],[201,74]];
  $: indoors = kind === 'underground' || kind === 'interior' || kind === 'ruins';
  // One scene pixel stays approximately 4 screen pixels on any window shape.
  onMount(() => {
    const resize = new ResizeObserver(([entry]) => {
      width = Math.max(1, Math.round(entry.contentRect.width / 4));
      height = Math.max(1, Math.round(entry.contentRect.height / 4));
    });
    resize.observe(terrain);
    return () => resize.disconnect();
  });
</script>

<svg class="map-terrain" bind:this={terrain} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" shape-rendering="crispEdges" aria-hidden="true">
  <defs>
    <pattern id="pixel-floor" width="16" height="16" patternUnits="userSpaceOnUse">
      <rect width="16" height="16" fill="var(--map-floor)" />
      <path d="M0 16V0H16" fill="none" stroke="var(--map-floor-detail)" stroke-width="1" />
    </pattern>
  </defs>
  <rect {width} {height} fill={indoors ? 'url(#pixel-floor)' : 'var(--map-grass)'} />
  {#each exits as exit}
    <path d={`M${Math.round(width / 2)} ${Math.round(height / 2)}H${Math.round(exit.x / 900 * width)}V${Math.round(exit.y / 400 * height)}`} fill="none" stroke="var(--map-path)" stroke-width="12" />
  {/each}
  <rect x={Math.round(width / 2) - 30} y={Math.round(height / 2) - 16} width="60" height="32" fill="var(--map-path)" />
  {#each trees as [x, y]}
    <g transform={`translate(${Math.round(x / 225 * width) - 6} ${Math.round(y / 100 * height) - 7})`}><PixelSprite name={indoors ? 'rock' : 'tree'} size={13} /></g>
  {/each}
</svg>
