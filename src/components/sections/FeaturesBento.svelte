<script lang="ts">
  import { onMount } from 'svelte';
  import { fly } from 'svelte/transition';
  import { useInView } from '../../lib/motion';
  import NeonPanel from '../ui/NeonPanel.svelte';
  import SectionHeader from '../ui/SectionHeader.svelte';
  import TimingWheel from '../viz/TimingWheel.svelte';
  import SlotHeatmap from '../viz/SlotHeatmap.svelte';
  import { features } from '../../data/features';

  let bentoGrid: HTMLElement | null = $state(null);
  let hasRevealed = $state(false);

  function handleInView(inView: boolean) {
    if (inView && !hasRevealed && bentoGrid) {
      hasRevealed = true;
    }
  }
</script>

<section id="features" class="container features-section">
  <SectionHeader
    code="SYS-01"
    title="SYSTEM CAPABILITIES"
    subtitle="Core primitives for distributed scheduling"
  />

  <div class="bento-grid" bind:this={bentoGrid} use:useInView={handleInView}>

    <!-- CONSENSUS (Large) -->
    <div class="bento-cell consensus-cell">
      <NeonPanel tone="cyan" label="CONSENSUS & HA" code="C-100">
        <div class="cell-content">
          <div class="feature-list">
            {#each features.filter(f => f.category === 'consensus') as feature}
              <div class="feature-item">
                <h3 class="hud-text text-[var(--neon-cyan)]">{feature.title}</h3>
                <p class="text-[var(--ink-secondary)] text-sm">{feature.description}</p>
              </div>
            {/each}
          </div>
          <div class="consensus-viz">
            <svg viewBox="0 0 100 100" width="100" height="100">
              <circle cx="50" cy="50" r="40" fill="none" stroke="var(--neon-cyan)" stroke-width="1" stroke-dasharray="4 4" class="anim-rotate slow"/>
              <circle cx="50" cy="20" r="8" fill="var(--neon-cyan)" class="anim-pulse" />
              <circle cx="20" cy="70" r="6" fill="var(--neon-magenta)" />
              <circle cx="80" cy="70" r="6" fill="var(--neon-magenta)" />
              <path d="M 50 20 L 20 70 M 50 20 L 80 70 M 20 70 L 80 70" stroke="var(--border-hud)" fill="none" />
            </svg>
          </div>
        </div>
      </NeonPanel>
    </div>

    <!-- SCHEDULING (Medium/Large) -->
    <div class="bento-cell scheduling-cell">
      <NeonPanel tone="cyan" label="SCHEDULING ENGINE" code="S-200">
        <div class="cell-content">
          <div class="feature-list">
            {#each features.filter(f => f.category === 'scheduling') as feature}
              <div class="feature-item">
                <h3 class="hud-text text-[var(--neon-cyan)]">{feature.title}</h3>
                <p class="text-[var(--ink-secondary)] text-sm">{feature.description}</p>
              </div>
            {/each}
          </div>
          <div class="mini-wheel-wrapper">
             <TimingWheel size={120} opacity={0.6} />
          </div>
        </div>
      </NeonPanel>
    </div>

    <!-- STORAGE (Medium) -->
    <div class="bento-cell storage-cell">
      <NeonPanel tone="magenta" label="STORAGE TIER" code="D-300">
        <div class="cell-content">
          <div class="feature-list">
            {#each features.filter(f => f.category === 'storage') as feature}
              <div class="feature-item">
                <h3 class="hud-text text-[var(--neon-magenta)]">{feature.title}</h3>
                <p class="text-[var(--ink-secondary)] text-sm">{feature.description}</p>
              </div>
            {/each}
          </div>
          <SlotHeatmap />
        </div>
      </NeonPanel>
    </div>

    <!-- OPS (Medium) -->
    <div class="bento-cell ops-cell">
      <NeonPanel tone="phosphor" label="OPERATIONS" code="O-400">
        <div class="cell-content dense-list">
          {#each features.filter(f => f.category === 'ops') as feature}
            <div class="feature-item dense">
              <span class="hud-text text-[var(--phosphor)]">> {feature.title}</span>
              <p class="text-[var(--ink-mute)] text-xs font-mono">{feature.description}</p>
            </div>
          {/each}
        </div>
      </NeonPanel>
    </div>

  </div>
</section>

<style>
  .features-section {
    padding: var(--spacing-2xl) 0;
  }

  .bento-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    grid-auto-rows: minmax(240px, auto);
    gap: var(--spacing-lg);
  }

  .consensus-cell {
    grid-column: span 2;
  }

  .scheduling-cell {
    grid-column: span 1;
  }

  .storage-cell {
    grid-column: span 2;
  }

  .ops-cell {
    grid-column: span 1;
  }

  .cell-content {
    display: flex;
    flex-direction: column;
    height: 100%;
    position: relative;
  }

  .feature-list {
    display: flex;
    flex-direction: column;
    gap: var(--spacing-md);
    z-index: 2;
  }

  .feature-item h3 {
    margin-bottom: var(--spacing-xs);
    font-size: 1.1rem;
  }

  .feature-item p {
    font-size: 0.95rem;
    line-height: 1.4;
  }

  .dense-list {
    gap: var(--spacing-sm);
  }

  .feature-item.dense {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: var(--spacing-xs) 0;
    border-bottom: 1px dashed rgba(255,255,255,0.05);
  }

  .consensus-viz {
    position: absolute;
    bottom: -20px;
    right: -20px;
    opacity: 0.3;
    pointer-events: none;
    transform: scale(1.5);
  }

  .mini-wheel-wrapper {
    margin-top: auto;
    display: flex;
    justify-content: flex-end;
    pointer-events: none;
  }

  @media (max-width: 900px) {
    .bento-grid {
      grid-template-columns: 1fr;
      grid-auto-rows: auto;
    }
    .consensus-cell, .scheduling-cell, .storage-cell, .ops-cell {
      grid-column: span 1;
    }

    .consensus-viz {
      display: none;
    }
  }
</style>
