<script lang="ts">
  import { useInView } from '../../lib/motion';
  import DataTrace from './DataTrace.svelte';
  import NeonPanel from '../ui/NeonPanel.svelte';
  
  let inView = $state(false);
  
  let hoverNode: any = $state(null);
  
  const nodes = [
    { id: 'leader', cx: 300, cy: 150, type: 'Leader', role: 'node-01', term: 3, commit: '1.2k/s', status: 'HEALTHY', c: 'var(--neon-cyan)' },
    { id: 'f1', cx: 100, cy: 250, type: 'Follower', role: 'node-02', term: 3, commit: '1.2k/s', status: 'SYNCED', c: 'var(--neon-magenta)' },
    { id: 'f2', cx: 500, cy: 250, type: 'Follower', role: 'node-03', term: 3, commit: '1.2k/s', status: 'SYNCED', c: 'var(--neon-magenta)' }
  ];
</script>

<div class="cluster-topology" use:useInView={(v) => inView = v}>
  <div class="svg-container">
    <svg viewBox="0 0 600 400" width="100%" height="100%">
      {#if inView}
        <!-- Traces from Leader to Followers -->
        <DataTrace pathId="trace-f1" pathData="M 300 150 Q 200 150 100 250" tone="cyan" delay={0} />
        <DataTrace pathId="trace-f1-2" pathData="M 300 150 Q 200 150 100 250" tone="cyan" delay={400} />
        
        <DataTrace pathId="trace-f2" pathData="M 300 150 Q 400 150 500 250" tone="cyan" delay={150} />
        <DataTrace pathId="trace-f2-2" pathData="M 300 150 Q 400 150 500 250" tone="cyan" delay={550} />
      {/if}

      <!-- Nodes -->
      {#each nodes as node}
        <g 
          class="node" 
          transform="translate({node.cx}, {node.cy})" 
          onmouseenter={() => hoverNode = node}
          onmouseleave={() => hoverNode = null}
          role="button"
          tabindex="0"
        >
          <circle r="30" fill="var(--bg-panel)" stroke={node.c} stroke-width="2" class={node.id === 'leader' ? 'anim-pulse' : ''} />
          <circle r="12" fill={node.c} opacity="0.2" />
          <text y="50" fill="var(--ink-primary)" font-family="var(--font-mono)" font-size="12" text-anchor="middle" class="hud-text">{node.role}</text>
        </g>
      {/each}
    </svg>
  </div>

  {#if hoverNode}
    <div class="node-tooltip">
      <NeonPanel tone={hoverNode.id === 'leader' ? 'cyan' : 'magenta'} label="NODE METRICS">
        <div class="metrics">
          <div class="metric"><span class="label">ROLE:</span><span class="val">{hoverNode.type}</span></div>
          <div class="metric"><span class="label">STATUS:</span><span class="val text-[var(--phosphor)]">{hoverNode.status}</span></div>
          <div class="metric"><span class="label">TERM:</span><span class="val">{hoverNode.term}</span></div>
          <div class="metric"><span class="label">COMMIT:</span><span class="val">{hoverNode.commit}</span></div>
        </div>
      </NeonPanel>
    </div>
  {/if}
</div>

<style>
  .cluster-topology {
    position: relative;
    width: 100%;
    aspect-ratio: 3/2;
    background: var(--bg-panel-2);
    border: 1px solid var(--border-hud);
    clip-path: var(--hud-clip);
  }

  .svg-container {
    width: 100%;
    height: 100%;
  }

  .node {
    cursor: pointer;
  }

  .node circle:first-child {
    transition: filter 0.2s;
  }

  .node:hover circle:first-child {
    filter: drop-shadow(0 0 12px var(--neon-cyan));
  }

  .node-tooltip {
    position: absolute;
    bottom: var(--spacing-lg);
    left: var(--spacing-lg);
    width: 250px;
    pointer-events: none;
  }

  .metrics {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-family: var(--font-mono);
    font-size: 0.85rem;
  }

  .metric {
    display: flex;
    justify-content: space-between;
  }

  .metric .label {
    color: var(--ink-mute);
  }
  .metric .val {
    color: var(--ink-primary);
  }
</style>
