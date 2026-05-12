<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    variant?: 'primary' | 'ghost';
    onclick?: (e: MouseEvent) => void;
    children?: Snippet;
  }
  let { variant = 'primary', onclick, children }: Props = $props();
</script>

<button class="hud-btn hud-btn-{variant}" {onclick}>
  <div class="btn-inner">
    {@render children?.()}
  </div>
</button>

<style>
  .hud-btn {
    position: relative;
    background: transparent;
    border: none;
    padding: 0;
    cursor: pointer;
    font-family: var(--font-display);
    font-weight: 700;
    font-size: 1rem;
    letter-spacing: 0.05em;
    clip-path: var(--hud-clip-sm);
    transition: transform 0.2s var(--ease-snappy);
  }

  .hud-btn:focus-visible {
    outline: 2px solid var(--neon-cyan);
    outline-offset: 4px;
  }

  .hud-btn:hover {
    transform: scaleY(0.95) scaleX(1.02);
  }

  .hud-btn:active {
    transform: scaleY(0.9) scaleX(0.98);
  }

  .btn-inner {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--spacing-sm);
    padding: var(--spacing-sm) var(--spacing-lg);
    transition: all 0.2s var(--ease-out);
    height: 100%;
    width: 100%;
  }

  .hud-btn-primary {
    background: rgba(0, 224, 255, 0.1);
  }

  .hud-btn-primary .btn-inner {
    border: 1px solid var(--neon-cyan);
    color: var(--neon-cyan);
  }

  .hud-btn-primary:hover .btn-inner {
    background: rgba(0, 224, 255, 0.2);
    box-shadow: inset 0 0 12px rgba(0, 224, 255, 0.4);
    color: #fff;
    text-shadow: var(--glow-cyan-sm);
  }

  .hud-btn-ghost .btn-inner {
    border: 1px solid var(--ink-mute);
    color: var(--ink-secondary);
  }

  .hud-btn-ghost:hover .btn-inner {
    border-color: var(--ink-primary);
    color: var(--ink-primary);
  }

  @media (max-width: 768px) {
    .hud-btn {
      font-size: 0.8rem;
    }

    .btn-inner {
      padding: var(--spacing-xs) var(--spacing-md);
    }
  }
</style>
