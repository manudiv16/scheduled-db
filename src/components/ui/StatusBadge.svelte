<script lang="ts">
  interface Props {
    variant?: 'leader' | 'follower' | 'ok' | 'warn' | 'critical' | 'info';
    pulse?: boolean;
    label: string;
  }
  let { variant = 'info', pulse = false, label }: Props = $props();

  let colorClass = $derived(
    variant === 'leader' ? 'text-[var(--neon-cyan)] border-[var(--neon-cyan)] bg-[rgba(0,224,255,0.1)]' :
    variant === 'follower' ? 'text-[var(--neon-magenta)] border-[var(--neon-magenta)] bg-[rgba(255,61,240,0.1)]' :
    variant === 'ok' ? 'text-[var(--phosphor)] border-[var(--phosphor)] bg-[rgba(124,255,95,0.1)]' :
    variant === 'warn' ? 'text-[var(--neon-amber)] border-[var(--neon-amber)] bg-[rgba(255,182,39,0.1)]' :
    variant === 'critical' ? 'text-[#ff3333] border-[#ff3333] bg-[rgba(255,51,51,0.1)]' :
    'text-[var(--ink-secondary)] border-[var(--ink-mute)] bg-[rgba(255,255,255,0.05)]'
  );

  let pulseClass = $derived(
    pulse && variant === 'leader' ? 'anim-pulse' :
    pulse && variant === 'follower' ? 'anim-pulse-magenta' :
    ''
  );
</script>

<div class="status-badge {colorClass} {pulseClass}">
  <span class="hud-text text-[10px]">{label}</span>
</div>

<style>
  .status-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 2px 6px;
    border-width: 1px;
    border-style: solid;
    clip-path: polygon(0 0, calc(100% - 4px) 0, 100% 4px, 100% 100%, 4px 100%, 0 calc(100% - 4px));
  }

  @keyframes pulse-magenta {
    0% { box-shadow: 0 0 0 0 rgba(255, 61, 240, 0.4); }
    70% { box-shadow: 0 0 0 6px rgba(255, 61, 240, 0); }
    100% { box-shadow: 0 0 0 0 rgba(255, 61, 240, 0); }
  }
  .anim-pulse-magenta {
    animation: pulse-magenta 2s infinite;
  }
</style>
