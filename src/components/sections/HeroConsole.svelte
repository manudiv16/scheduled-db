<script lang="ts">
  import { onMount } from 'svelte';
  import { bootSequence } from '../../lib/animate';
  import HudButton from '../ui/HudButton.svelte';
  import TimingWheel from '../viz/TimingWheel.svelte';

  let titleEl: HTMLElement | null = $state(null);
  let subtitleEl: HTMLElement | null = $state(null);
  let buttonsWrapper: HTMLElement | null = $state(null);
  let wheelWrapper: HTMLElement | null = $state(null);

  onMount(() => {
    let anim: any;
    if (buttonsWrapper) {
      const btnEls = Array.from(buttonsWrapper.children) as HTMLElement[];
      anim = bootSequence(titleEl, subtitleEl, btnEls, wheelWrapper);
    }
    return () => {
      if (anim) anim.pause();
    };
  });
</script>

<section class="hero-console">
  <!-- Background Wheel -->
  <div class="wheel-bg" bind:this={wheelWrapper} style="opacity: 0;">
    <TimingWheel size={800} opacity={0.35} />
  </div>

  <div class="container hero-inner">
    <!-- Status Bar HUD -->
    <div class="hero-status hud-text">
      <span class="status-item">[ CLUSTER STATUS: HEALTHY ]</span>
      <span class="status-item">[ 3 NODES ]</span>
      <span class="status-item">[ LEADER: node-01 ]</span>
      <span class="status-item text-[var(--neon-magenta)]">[ TERM 03 ]</span>
    </div>

    <!-- Main Content -->
    <div class="hero-content">
      <h1 class="hero-title neon-text-cyan" bind:this={titleEl} style="opacity: 0;">
        SCHEDULED-DB
      </h1>
      
      <p class="hero-subtitle text-[var(--ink-secondary)]" bind:this={subtitleEl} style="opacity: 0;">
        Distributed job scheduling built on 
        <span class="text-[var(--ink-primary)]">HashiCorp Raft</span> 
        and 
        <span class="text-[var(--ink-primary)]">Hierarchical Timing Wheels</span>.
      </p>

      <div class="hero-actions" bind:this={buttonsWrapper}>
        <div style="opacity: 0;"><HudButton variant="primary" onclick={() => window.location.hash = 'quickstart'}>INITIALIZE</HudButton></div>
        <div style="opacity: 0;"><HudButton variant="ghost" onclick={() => window.location.hash = 'features'}>VIEW DOCS</HudButton></div>
      </div>
    </div>
  </div>
</section>

<style>
  .hero-console {
    position: relative;
    min-height: calc(100vh - 48px);
    display: flex;
    align-items: center;
    overflow: hidden;
  }

  .wheel-bg {
    position: absolute;
    right: -200px;
    top: 50%;
    transform: translateY(-50%);
    z-index: -1;
    pointer-events: none;
    max-width: none;
  }

  .hero-inner {
    position: relative;
    width: 100%;
    z-index: 1;
    padding: var(--spacing-2xl) 0;
  }

  .hero-status {
    display: flex;
    gap: var(--spacing-lg);
    margin-bottom: var(--spacing-2xl);
    border-bottom: 1px solid var(--border-hud);
    padding-bottom: var(--spacing-sm);
  }

  .status-item {
    color: var(--ink-mute);
  }

  .hero-content {
    max-width: 800px;
  }

  .hero-title {
    font-size: 5rem;
    line-height: 1.1;
    margin-bottom: var(--spacing-lg);
    text-shadow: var(--glow-cyan-md);
  }

  .hero-subtitle {
    font-size: 1.5rem;
    line-height: 1.5;
    margin-bottom: var(--spacing-2xl);
    max-width: 600px;
  }

  .hero-actions {
    display: flex;
    gap: var(--spacing-md);
  }

  @media (max-width: 768px) {
    .hero-title {
      font-size: 3rem;
    }
    .hero-status {
      flex-direction: column;
      gap: var(--spacing-xs);
    }
    .wheel-bg {
      right: -400px;
      opacity: 0.1 !important;
    }
  }
</style>
