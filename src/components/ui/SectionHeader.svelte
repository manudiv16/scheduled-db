<script lang="ts">
  interface Props {
    code: string;
    title: string;
    subtitle?: string;
  }
  let { code, title, subtitle }: Props = $props();
</script>

<div class="section-header">
  <div class="sh-top">
    <span class="hud-text text-[var(--ink-mute)]">[{code}]</span>
    <div class="sh-line"></div>
  </div>
  <h2 class="sh-title neon-text-cyan">{title}</h2>
  {#if subtitle}
    <p class="sh-subtitle text-[var(--ink-secondary)]">{subtitle}</p>
  {/if}
</div>

<style>
  .section-header {
    margin-bottom: var(--spacing-2xl);
    display: flex;
    flex-direction: column;
    gap: var(--spacing-sm);
    padding-left: var(--spacing-md);
    border-left: 2px solid var(--border-hud);
  }

  .sh-top {
    display: flex;
    align-items: center;
    gap: var(--spacing-md);
  }

  .sh-line {
    flex-grow: 1;
    height: 1px;
    background: var(--border-hud);
  }

  .sh-title {
    font-size: 2.5rem;
    margin: 0;
    line-height: 1.1;
  }

  .sh-subtitle {
    font-family: var(--font-mono);
    font-size: 0.9rem;
  }

  @media (max-width: 768px) {
    .section-header {
      padding-left: var(--spacing-sm);
    }

    .sh-title {
      font-size: 1.75rem;
    }

    .sh-subtitle {
      font-size: 0.8rem;
    }
  }
</style>
