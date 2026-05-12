<script lang="ts">
  // A simple visual for cold storage
</script>

<div class="slot-heatmap">
  {#each Array(48) as _, i}
    <div class="slot-cell" class:hot={i < 12} class:warm={i >= 12 && i < 24} class:cold={i >= 24}></div>
  {/each}
</div>

<style>
  .slot-heatmap {
    display: grid;
    grid-template-columns: repeat(12, 1fr);
    gap: 4px;
    width: 100%;
    margin-top: var(--spacing-md);
  }

  .slot-cell {
    aspect-ratio: 1;
    border-radius: 2px;
    background: var(--bg-void);
    border: 1px solid var(--border-hud);
  }

  .slot-cell.hot {
    background: rgba(0, 224, 255, 0.4);
    border-color: var(--neon-cyan);
    box-shadow: 0 0 8px rgba(0, 224, 255, 0.2);
  }

  .slot-cell.warm {
    background: rgba(0, 224, 255, 0.1);
  }

  .slot-cell.cold {
    background: var(--bg-void);
  }
</style>
