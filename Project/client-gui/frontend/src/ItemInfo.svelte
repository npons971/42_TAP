<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';

  export let item;
  export let onclose = () => {};
  let closeButton;

  onMount(() => {
    const previousFocus = document.activeElement;
    closeButton?.focus();
    return () => {
      if (previousFocus?.isConnected) previousFocus.focus();
    };
  });

  // The card has one focusable control; keep keyboard focus inside the dialog.
  function handleKey(event) {
    if (event.key === 'Escape' && !event.isComposing) {
      event.preventDefault();
      onclose();
    } else if (event.key === 'Tab') {
      event.preventDefault();
      closeButton?.focus();
    }
  }
</script>

<svelte:window on:keydown={handleKey} />

<div class="modal-backdrop">
  <div class="dialogue-card item-info-card" role="dialog" aria-modal="true" aria-labelledby="item-info-title">
    <div class="dialogue-header">
      <h3 id="item-info-title">{item.name}</h3>
      <button class="btn-close" bind:this={closeButton} aria-label="Close item information" on:click={onclose}><Icon name="close" size={16}/></button>
    </div>
    {#if item.description}<p class="item-info-description">{item.description}</p>{/if}
    <dl class="item-info-stats">
      {#if item.damage_bonus > 0}<div><dt>Attack bonus</dt><dd>+{item.damage_bonus}</dd></div>{/if}
      {#if item.defense_bonus > 0}<div><dt>Defense bonus</dt><dd>+{item.defense_bonus}</dd></div>{/if}
      {#if item.heal_value > 0}<div><dt>Healing</dt><dd>Up to {item.heal_value} HP</dd></div>{/if}
    </dl>
    <p class="item-info-usage">{item.usage || 'No detailed usage information is available from this server.'}</p>
  </div>
</div>
