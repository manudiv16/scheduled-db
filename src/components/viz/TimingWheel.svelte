<script lang="ts">
  import { useInView } from '../../lib/motion';
  import { respectsReducedMotion } from '../../lib/animate';

  interface Props {
    size?: number;
    opacity?: number;
  }
  let { size = 600, opacity = 1 }: Props = $props();

  let inView = $state(false);
  let reducedMotion = $state(false);
  
  $effect(() => {
    reducedMotion = respectsReducedMotion();
  });

  let rotateClass = $derived(!reducedMotion && inView ? 'anim-rotate' : '');
</script>

<div 
  class="timing-wheel-wrapper" 
  style="width: {size}px; height: {size}px; opacity: {opacity};"
  use:useInView={(v) => inView = v}
>
  <svg viewBox="0 0 400 400" width="100%" height="100%" class="wheel {rotateClass} slow">
    <!-- Outer Ring (Hours) -->
    <circle cx="200" cy="200" r="180" fill="none" stroke="var(--border-hud)" stroke-width="1" stroke-dasharray="4 8" />
    {#each Array(60) as _, i}
      <line x1="200" y1="20" x2="200" y2="30" stroke={i % 5 === 0 ? 'var(--neon-cyan)' : 'var(--ink-mute)'} stroke-width={i % 5 === 0 ? 2 : 1} transform="rotate({i * 6} 200 200)" />
    {/each}
  </svg>
  
  <svg viewBox="0 0 400 400" width="100%" height="100%" class="wheel {rotateClass} medium" style="animation-direction: reverse;">
    <!-- Middle Ring (Minutes) -->
    <circle cx="200" cy="200" r="130" fill="none" stroke="var(--border-hud)" stroke-width="1" />
    <circle cx="200" cy="200" r="140" fill="none" stroke="var(--neon-cyan-soft)" stroke-width="8" stroke-dasharray="2 12" />
    {#each Array(12) as _, i}
      <line x1="200" y1="70" x2="200" y2="80" stroke="var(--neon-magenta)" stroke-width="1.5" transform="rotate({i * 30} 200 200)" />
    {/each}
  </svg>
  
  <svg viewBox="0 0 400 400" width="100%" height="100%" class="wheel {rotateClass} fast">
    <!-- Inner Ring (Seconds) -->
    <circle cx="200" cy="200" r="80" fill="none" stroke="var(--border-hud)" stroke-width="1" stroke-dasharray="1 4" />
    {#each Array(8) as _, i}
      <circle cx="200" cy="120" r="3" fill="var(--phosphor)" transform="rotate({i * 45} 200 200)" opacity={i === 0 ? 1 : 0.3} />
    {/each}
    <!-- Center Core -->
    <circle cx="200" cy="200" r="30" fill="var(--bg-panel-2)" stroke="var(--neon-cyan)" stroke-width="2" />
    <circle cx="200" cy="200" r="10" fill="var(--neon-cyan)" />
  </svg>
</div>

<style>
  .timing-wheel-wrapper {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .wheel {
    position: absolute;
    top: 0;
    left: 0;
    transform-origin: center;
  }

  @keyframes rotate-wheel {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .anim-rotate {
    animation: rotate-wheel linear infinite;
  }
  
  .anim-rotate.slow { animation-duration: 60s; }
  .anim-rotate.medium { animation-duration: 30s; }
  .anim-rotate.fast { animation-duration: 10s; }
</style>
