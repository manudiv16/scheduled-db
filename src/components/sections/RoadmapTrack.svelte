<script lang="ts">
  import SectionHeader from '../ui/SectionHeader.svelte';
  import NeonPanel from '../ui/NeonPanel.svelte';
  import { roadmap } from '../../data/roadmap';
  
  let hoverItem: any = $state(null);
</script>

<section id="roadmap" class="container roadmap-section">
  <SectionHeader 
    code="SYS-03" 
    title="DEVELOPMENT TRACK" 
    subtitle="Upcoming features and milestones"
  />
  
  <div class="track-container">
    <!-- PCB Trace Line -->
    <div class="pcb-trace"></div>
    
    <div class="stations">
      {#each roadmap as item}
        {@const isActive = item.status === 'active'}
        {@const isPast = item.status === 'past'}
        {@const nodeColor = isActive ? 'var(--neon-magenta)' : isPast ? 'var(--neon-cyan)' : 'var(--ink-mute)'}
        
        <div 
          class="station {item.status}"
          onmouseenter={() => hoverItem = item}
          onmouseleave={() => hoverItem = null}
          role="button"
          tabindex="0"
        >
          <div class="station-node" style="border-color: {nodeColor}; box-shadow: {isActive ? 'var(--glow-magenta-sm)' : 'none'};">
            {#if isActive || isPast}
              <div class="station-core" style="background: {nodeColor};"></div>
            {/if}
          </div>
          <div class="station-label hud-text" style="color: {nodeColor};">
            {item.quarter}
          </div>
          
          {#if hoverItem === item}
            <div class="station-tooltip">
              <NeonPanel tone={isActive ? 'magenta' : isPast ? 'cyan' : 'amber'} label={item.title}>
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
    padding: 100px 0;
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
    gap: var(--spacing-md);
    position: relative;
    cursor: pointer;
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
    transition: transform 0.2s var(--ease-out);
  }

  .station:hover .station-node {
    transform: scale(1.2);
  }

  .station-core {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  .station-tooltip {
    position: absolute;
    top: 100%;
    margin-top: var(--spacing-lg);
    width: 240px;
    z-index: 10;
  }

  /* Offset every other tooltip up for better spacing */
  .station:nth-child(even) .station-tooltip {
    top: auto;
    bottom: 100%;
    margin-top: 0;
    margin-bottom: var(--spacing-lg);
  }

  .feature-list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--spacing-xs);
  }
  
  @media (max-width: 768px) {
    .track-container {
      padding: 20px 0;
    }
    .pcb-trace {
      top: 0;
      bottom: 0;
      left: 50%;
      right: auto;
      width: 2px;
      height: 100%;
      transform: translateX(-50%);
    }
    .stations {
      flex-direction: column;
      gap: 100px;
      align-items: center;
    }
    .station-tooltip {
      position: relative;
      top: 0;
      bottom: auto;
      left: 50%;
      transform: translateX(-50%);
      margin-top: var(--spacing-md);
    }
    .station:nth-child(even) .station-tooltip {
      bottom: auto;
      margin-bottom: 0;
      margin-top: var(--spacing-md);
    }
  }
</style>
