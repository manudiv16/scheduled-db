<script lang="ts">
  import type { Snippet } from 'svelte';
  
  interface Props {
    tone?: 'cyan' | 'magenta' | 'amber' | 'phosphor';
    label?: string;
    code?: string;
    children?: Snippet;
  }
  let { tone = 'cyan', label, code, children }: Props = $props();

  let borderTone = $derived(
    tone === 'cyan' ? 'border-[var(--border-hud)]' :
    tone === 'magenta' ? 'border-[rgba(255,61,240,0.22)]' :
    tone === 'amber' ? 'border-[rgba(255,182,39,0.22)]' :
    'border-[rgba(124,255,95,0.22)]'
  );

  let hoverGlowTone = $derived(
    tone === 'cyan' ? 'hover:shadow-[var(--glow-cyan-sm)]' :
    tone === 'magenta' ? 'hover:shadow-[var(--glow-magenta-sm)]' :
    tone === 'amber' ? 'hover:shadow-[var(--glow-amber-sm)]' :
    'hover:shadow-[0_0_8px_rgba(124,255,95,0.4)]'
  );
</script>

<div class="neon-panel {borderTone} {hoverGlowTone}">
  {#if label || code}
    <div class="panel-header">
      {#if code}
        <span class="hud-text text-[var(--ink-mute)]">{code} //</span>
      {/if}
      {#if label}
        <span class="hud-text" style="color: var(--neon-{tone})">{label}</span>
      {/if}
    </div>
  {/if}
  <div class="panel-content">
    {@render children?.()}
  </div>
</div>

<style>
  .neon-panel {
    position: relative;
    background: var(--bg-panel);
    border-width: 1px;
    border-style: solid;
    clip-path: var(--hud-clip);
    padding: var(--spacing-lg);
    transition: all 0.3s var(--ease-out);
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .neon-panel:hover {
    transform: translateY(-2px);
  }

  .panel-header {
    display: flex;
    gap: var(--spacing-sm);
    margin-bottom: var(--spacing-md);
    padding-bottom: var(--spacing-sm);
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  }

  .panel-content {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
  }
</style>
