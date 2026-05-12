<script lang="ts">
  interface Props {
    text: string;
    tone?: 'cyan' | 'magenta' | 'amber' | 'phosphor' | 'mute';
  }
  let { text, tone = 'cyan' }: Props = $props();

  let toneClass = $derived(
    tone === 'cyan' ? 'neon-text-cyan' :
    tone === 'magenta' ? 'neon-text-magenta' :
    tone === 'amber' ? 'text-[var(--neon-amber)]' :
    tone === 'phosphor' ? 'text-[var(--phosphor)]' :
    'text-[var(--ink-mute)]'
  );
</script>

<span class="hud-label hud-text {toneClass}">
  [ {text} ]
</span>

<style>
  .hud-label {
    display: inline-flex;
    align-items: center;
    white-space: nowrap;
  }
</style>
