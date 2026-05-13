<script lang="ts">
  import type { Snippet } from 'svelte';
  interface Props {
    title?: string;
    children?: Snippet;
  }
  let { title = 'scheduled-db@cluster ~ %', children }: Props = $props();
</script>

<div class="terminal-window">
  <div class="term-header">
    <div class="term-controls">
      <span class="term-dot close"></span>
      <span class="term-dot min"></span>
      <span class="term-dot max"></span>
    </div>
    <div class="term-title">
      <span class="hud-text text-[var(--ink-mute)]">{title}</span>
    </div>
  </div>
  <div class="term-body scanlines">
    {@render children?.()}
  </div>
</div>

<style>
  .terminal-window {
    background: var(--bg-void);
    border: 1px solid var(--ink-mute);
    border-radius: 4px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .term-header {
    background: var(--bg-panel-2);
    border-bottom: 1px solid var(--ink-mute);
    display: flex;
    align-items: center;
    padding: var(--spacing-xs) var(--spacing-md);
  }

  .term-controls {
    display: flex;
    gap: 6px;
  }

  .term-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--ink-mute);
  }

  .term-dot.close { background: #ff5f56; }
  .term-dot.min { background: #ffbd2e; }
  .term-dot.max { background: #27c93f; }

  .term-title {
    flex-grow: 1;
    text-align: center;
  }

  .term-body {
    position: relative;
    padding: var(--spacing-md);
    background: #000;
    color: var(--phosphor);
    font-family: var(--font-terminal);
    font-size: 1.1rem;
    line-height: 1.4;
    overflow-x: auto;
  }
</style>
