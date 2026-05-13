<script lang="ts">
  import SectionHeader from '../ui/SectionHeader.svelte';
  import NeonPanel from '../ui/NeonPanel.svelte';
  import { roadmap } from '../../data/roadmap';

  let openItem: string | null = $state(null);

  function toggle(id: string) {
    openItem = openItem === id ? null : id;
  }

  function getColor(status: string) {
    if (status === 'done') return 'var(--neon-cyan)';
    if (status === 'active') return 'var(--neon-magenta)';
    return 'var(--ink-mute)';
  }

  function getTone(status: string): 'cyan' | 'magenta' | 'amber' {
    if (status === 'done') return 'cyan';
    if (status === 'active') return 'magenta';
    return 'amber';
  }

  function getLabel(status: string) {
    if (status === 'done') return 'DONE';
    if (status === 'active') return 'IN PROGRESS';
    return 'PLANNED';
  }
</script>

<section id="roadmap" class="container roadmap-section">
  <SectionHeader
    code="SYS-03"
    title="DEVELOPMENT TRACK"
    subtitle="Upcoming features and milestones"
  />

  <div class="track-container">
    <div class="pcb-trace"></div>

    <div class="stations">
      {#each roadmap as item}
        {@const color = getColor(item.status)}
        {@const isOpen = openItem === item.id}

        <div class="station {item.status}">
          <button
            class="station-trigger"
            onclick={() => toggle(item.id)}
            aria-expanded={isOpen}
          >
            <div class="station-node" style="border-color: {color}; box-shadow: {item.status === 'active' ? 'var(--glow-magenta-sm)' : 'none'};">
              {#if item.status !== 'planned'}
                <div class="station-core" style="background: {color};"></div>
              {/if}
            </div>
            <div class="station-info">
              <span class="station-title" style="color: {color};">{item.title}</span>
              <span class="station-status hud-text" style="color: {color};">{getLabel(item.status)}</span>
            </div>
          </button>

          {#if isOpen}
            <div class="station-detail">
              <NeonPanel tone={getTone(item.status)} label={item.title}>
                <ul class="feature-list">
                  {#each item.features as f}
                    <li class="text-[var(--ink-primary)] text-sm">> {f}</li>
                  {/each}
                </ul>
              </NeonPanel>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </div>
</section>

<style>
  .roadmap-section {
    padding: var(--spacing-2xl) 0;
  }

  .track-container {
    position: relative;
    padding: var(--spacing-xl) 0;
    margin: 0 auto;
    max-width: 900px;
  }

  .pcb-trace {
    position: absolute;
    top: 50%;
    left: 5%;
    right: 5%;
    height: 2px;
    background: var(--ink-mute);
    transform: translateY(-50%);
    z-index: 1;
  }

  .stations {
    display: flex;
    justify-content: space-between;
    position: relative;
    z-index: 2;
  }

  .station {
    display: flex;
    flex-direction: column;
    align-items: center;
    position: relative;
    flex: 1;
  }

  .station-trigger {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--spacing-sm);
    cursor: pointer;
    background: none;
    border: none;
    padding: var(--spacing-md);
    min-width: 44px;
    min-height: 44px;
    transition: transform 0.2s var(--ease-out);
  }

  .station-trigger:hover {
    transform: scale(1.05);
  }

  .station-trigger:focus-visible {
    outline: 2px solid var(--neon-cyan);
    outline-offset: 4px;
    border-radius: 4px;
  }

  .station-node {
    width: 24px;
    height: 24px;
    border-radius: 50%;
    border: 2px solid;
    background: var(--bg-void);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .station-core {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  .station-info {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
  }

  .station-title {
    font-family: var(--font-display);
    font-weight: 600;
    font-size: 0.85rem;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .station-status {
    font-size: 10px;
  }

  .station-detail {
    margin-top: var(--spacing-md);
    width: 220px;
    max-width: 100%;
  }

  .feature-list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--spacing-xs);
  }

  /* Responsive: vertical timeline */
  @media (max-width: 768px) {
    .track-container {
      padding: var(--spacing-md) 0;
    }

    .pcb-trace {
      top: 0;
      bottom: 0;
      left: 20px;
      right: auto;
      width: 2px;
      height: 100%;
      transform: none;
    }

    .stations {
      flex-direction: column;
      gap: var(--spacing-lg);
      align-items: flex-start;
      padding-left: 8px;
    }

    .station {
      flex-direction: row;
      align-items: flex-start;
      gap: var(--spacing-md);
      width: 100%;
    }

    .station-trigger {
      flex-direction: row;
      align-items: center;
      gap: var(--spacing-md);
    }

    .station-info {
      align-items: flex-start;
    }

    .station-detail {
      margin-top: 0;
      flex: 1;
      width: auto;
      min-width: 0;
    }
  }
</style>
