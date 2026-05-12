<script lang="ts">
  interface Props {
    hotPercent?: number;
  }
  let { hotPercent = 25 }: Props = $props();
</script>

<div class="hotcold-bar">
  <div class="hot-zone" style="width: {hotPercent}%;">
    <span class="zone-label hud-text">HOT / MEMORY</span>
  </div>
  <div class="cold-zone">
    <span class="zone-label hud-text">COLD / DISK</span>
  </div>
  <div class="spill-marker" style="left: {hotPercent}%;">
    <div class="marker-line"></div>
    <span class="marker-label hud-text">SPILL</span>
  </div>
</div>

<style>
  .hotcold-bar {
    position: relative;
    display: flex;
    width: 100%;
    height: 48px;
    border: 1px solid var(--border-hud);
    clip-path: var(--hud-clip-sm);
    overflow: hidden;
    margin-top: var(--spacing-md);
  }

  .hot-zone {
    background: rgba(0, 224, 255, 0.15);
    border-right: 1px solid var(--neon-cyan);
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    z-index: 2;
  }

  .cold-zone {
    flex: 1;
    background: var(--bg-void);
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    z-index: 2;
  }

  .zone-label {
    font-size: 10px;
    color: var(--ink-mute);
    letter-spacing: 0.1em;
  }

  .hot-zone .zone-label {
    color: var(--neon-cyan);
    text-shadow: var(--glow-cyan-sm);
  }

  .spill-marker {
    position: absolute;
    top: 0;
    bottom: 0;
    z-index: 3;
    display: flex;
    flex-direction: column;
    align-items: center;
    transform: translateX(-50%);
  }

  .marker-line {
    width: 2px;
    flex: 1;
    background: var(--neon-amber);
    box-shadow: 0 0 8px rgba(255, 182, 39, 0.6);
  }

  .marker-label {
    font-size: 9px;
    color: var(--neon-amber);
    background: var(--bg-void);
    padding: 0 4px;
    white-space: nowrap;
    text-shadow: var(--glow-amber-sm);
  }
</style>
