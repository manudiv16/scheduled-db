<script lang="ts">
  import { onMount } from 'svelte';
  import anime from 'animejs';

  let containerRef: HTMLElement;

  onMount(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            // Animate node cards
            anime({
              targets: containerRef.querySelectorAll('.arch-node'),
              opacity: [0, 1],
              translateY: [30, 0],
              scale: [0.9, 1],
              delay: anime.stagger(120, { start: 200 }),
              duration: 700,
              easing: 'easeOutCubic',
            });

            // Animate connections
            anime({
              targets: containerRef.querySelectorAll('.arch-connection'),
              opacity: [0, 1],
              scaleX: [0, 1],
              delay: anime.stagger(100, { start: 800 }),
              duration: 500,
              easing: 'easeOutCubic',
            });

            // Pulse the leader indicator
            anime({
              targets: containerRef.querySelector('.leader-pulse'),
              scale: [1, 1.6],
              opacity: [0.6, 0],
              duration: 2000,
              loop: true,
              easing: 'easeOutSine',
            });

            // Animate data flow dots
            anime({
              targets: containerRef.querySelectorAll('.flow-dot'),
              translateX: [0, 60],
              opacity: [0.8, 0],
              duration: 1500,
              loop: true,
              delay: anime.stagger(300),
              easing: 'easeInCubic',
            });

            observer.disconnect();
          }
        });
      },
      { threshold: 0.2 }
    );

    observer.observe(containerRef);
  });
</script>

<div bind:this={containerRef} class="arch-wrapper">
  <!-- Client -->
  <div class="arch-row arch-row-top">
    <div class="arch-node client-node" style="opacity: 0;">
      <div class="node-icon">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/>
          <line x1="8" y1="21" x2="16" y2="21"/>
          <line x1="12" y1="17" x2="12" y2="21"/>
        </svg>
      </div>
      <span class="node-label">API Clients</span>
      <span class="node-detail">HTTP / REST</span>
    </div>
  </div>

  <!-- Connection: Client -> Cluster -->
  <div class="arch-connection-vertical" style="opacity: 0;">
    <div class="connection-line"></div>
    <div class="flow-dot"></div>
    <div class="flow-dot"></div>
  </div>

  <!-- Cluster Box -->
  <div class="arch-cluster">
    <div class="cluster-label">Raft Cluster</div>

    <div class="arch-row arch-row-nodes">
      <!-- Leader -->
      <div class="arch-node leader-node" style="opacity: 0;">
        <div class="leader-indicator">
          <div class="leader-pulse"></div>
          <div class="leader-dot"></div>
        </div>
        <div class="node-icon leader-icon">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
          </svg>
        </div>
        <span class="node-label">Node 1</span>
        <span class="node-role leader-role">LEADER</span>
        <span class="node-detail">Writes + Execution</span>
      </div>

      <!-- Follower 1 -->
      <div class="arch-node follower-node" style="opacity: 0;">
        <div class="node-icon follower-icon">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
          </svg>
        </div>
        <span class="node-label">Node 2</span>
        <span class="node-role follower-role">FOLLOWER</span>
        <span class="node-detail">Replication</span>
      </div>

      <!-- Follower 2 -->
      <div class="arch-node follower-node" style="opacity: 0;">
        <div class="node-icon follower-icon">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
          </svg>
        </div>
        <span class="node-label">Node 3</span>
        <span class="node-role follower-role">FOLLOWER</span>
        <span class="node-detail">Replication</span>
      </div>
    </div>

    <!-- Connections between nodes -->
    <div class="arch-connections-inner">
      <div class="arch-connection horizontal-conn" style="opacity: 0;">
        <span class="conn-label">Raft Log Replication</span>
      </div>
    </div>
  </div>

  <!-- Connection: Cluster -> Worker -->
  <div class="arch-connection-vertical" style="opacity: 0;">
    <div class="connection-line"></div>
    <div class="flow-dot"></div>
  </div>

  <!-- Bottom row -->
  <div class="arch-row arch-row-bottom">
    <div class="arch-node worker-node" style="opacity: 0;">
      <div class="node-icon worker-icon">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="3"/>
          <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-2 2 2 2 0 01-2-2v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83 0 2 2 0 010-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 01-2-2 2 2 0 012-2h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 010-2.83 2 2 0 012.83 0l.06.06A1.65 1.65 0 009 4.68 1.65 1.65 0 0010 3.17V3a2 2 0 012-2 2 2 0 012 2v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 0 2 2 0 010 2.83l-.06.06A1.65 1.65 0 0019.32 9a1.65 1.65 0 001.51 1H21a2 2 0 012 2 2 2 0 01-2 2h-.09a1.65 1.65 0 00-1.51 1z"/>
        </svg>
      </div>
      <span class="node-label">Worker</span>
      <span class="node-detail">Job Execution</span>
    </div>

    <div class="arch-node webhook-node" style="opacity: 0;">
      <div class="node-icon webhook-icon">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
        </svg>
      </div>
      <span class="node-label">Webhooks</span>
      <span class="node-detail">HTTP Callbacks</span>
    </div>

    <div class="arch-node metrics-node" style="opacity: 0;">
      <div class="node-icon metrics-icon">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <line x1="18" y1="20" x2="18" y2="10"/>
          <line x1="12" y1="20" x2="12" y2="4"/>
          <line x1="6" y1="20" x2="6" y2="14"/>
        </svg>
      </div>
      <span class="node-label">Metrics</span>
      <span class="node-detail">Prometheus / OTel</span>
    </div>
  </div>
</div>

<style>
  .arch-wrapper {
    max-width: 750px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0;
  }

  .arch-row {
    display: flex;
    gap: 1.25rem;
    justify-content: center;
    flex-wrap: wrap;
  }

  .arch-node {
    position: relative;
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 12px;
    padding: 1.25rem 1.5rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.4rem;
    min-width: 130px;
    transition: all 0.3s ease;
  }

  .arch-node:hover {
    border-color: #475569;
    transform: translateY(-2px);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3);
  }

  .node-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 42px;
    height: 42px;
    border-radius: 10px;
    margin-bottom: 0.25rem;
  }

  .client-node { border-color: #334155; }
  .client-node .node-icon { background: rgba(34, 211, 238, 0.1); color: #22d3ee; }

  .leader-node { border-color: rgba(251, 191, 36, 0.3); }
  .leader-node .node-icon { background: rgba(251, 191, 36, 0.1); color: #fbbf24; }

  .follower-node { border-color: rgba(59, 130, 246, 0.3); }
  .follower-node .node-icon { background: rgba(59, 130, 246, 0.1); color: #3b82f6; }

  .worker-node .node-icon { background: rgba(244, 114, 182, 0.1); color: #f472b6; }
  .webhook-node .node-icon { background: rgba(52, 211, 153, 0.1); color: #34d399; }
  .metrics-node .node-icon { background: rgba(167, 139, 250, 0.1); color: #a78bfa; }

  .node-label {
    font-size: 0.85rem;
    font-weight: 600;
    color: #f1f5f9;
  }

  .node-role {
    font-size: 0.65rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
  }

  .leader-role {
    background: rgba(251, 191, 36, 0.15);
    color: #fbbf24;
  }

  .follower-role {
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
  }

  .node-detail {
    font-size: 0.7rem;
    color: #64748b;
  }

  .leader-indicator {
    position: absolute;
    top: -6px;
    right: -6px;
  }

  .leader-dot {
    width: 12px;
    height: 12px;
    background: #fbbf24;
    border-radius: 50%;
    position: relative;
    z-index: 1;
  }

  .leader-pulse {
    position: absolute;
    inset: -4px;
    border-radius: 50%;
    background: #fbbf24;
    z-index: 0;
  }

  .arch-cluster {
    position: relative;
    border: 1px dashed rgba(34, 211, 238, 0.3);
    border-radius: 16px;
    padding: 2.5rem 2rem 1.5rem;
    margin: 0.5rem 0;
    background: rgba(34, 211, 238, 0.02);
  }

  .cluster-label {
    position: absolute;
    top: -10px;
    left: 50%;
    transform: translateX(-50%);
    background: #0a0e1a;
    padding: 0 0.75rem;
    font-size: 0.75rem;
    font-weight: 600;
    color: #22d3ee;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .arch-connections-inner {
    margin-top: 1rem;
    display: flex;
    justify-content: center;
  }

  .arch-connection {
    display: flex;
    align-items: center;
    justify-content: center;
    transform-origin: center;
  }

  .horizontal-conn {
    width: 80%;
    height: 1px;
    background: linear-gradient(90deg, transparent, rgba(34, 211, 238, 0.4), transparent);
    position: relative;
  }

  .conn-label {
    position: absolute;
    top: 8px;
    left: 50%;
    transform: translateX(-50%);
    font-size: 0.65rem;
    color: #64748b;
    white-space: nowrap;
  }

  .arch-connection-vertical {
    width: 1px;
    height: 32px;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .connection-line {
    position: absolute;
    inset: 0;
    width: 1px;
    margin: 0 auto;
    background: linear-gradient(180deg, rgba(34, 211, 238, 0.4), rgba(167, 139, 250, 0.4));
  }

  .flow-dot {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: #22d3ee;
    position: absolute;
    top: 0;
  }

  .arch-row-nodes {
    gap: 1.5rem;
  }

  @media (max-width: 768px) {
    .arch-row-nodes {
      flex-direction: column;
      align-items: center;
    }

    .arch-cluster {
      padding: 2.5rem 1rem 1.5rem;
    }

    .arch-node {
      min-width: 110px;
    }
  }
</style>
