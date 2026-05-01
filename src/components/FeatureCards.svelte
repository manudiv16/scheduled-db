<script lang="ts">
  import { onMount } from 'svelte';
  import anime from 'animejs';

  const features = [
    {
      icon: '⚡',
      title: 'Distributed Consensus',
      description: 'Built on HashiCorp Raft for strong consistency. Automatic leader election and log replication across all nodes.',
      color: '#22d3ee',
    },
    {
      icon: '🔄',
      title: 'Two Job Types',
      description: 'Unico: one-time execution at a specific timestamp. Recurrente: recurring execution with cron expressions.',
      color: '#a78bfa',
    },
    {
      icon: '⏱',
      title: 'Time-Slotted Scheduling',
      description: 'Efficient job organization with configurable slot intervals. Optimized for high-throughput scheduling.',
      color: '#3b82f6',
    },
    {
      icon: '🛡',
      title: 'Queue Size Limits',
      description: 'Configurable memory and job count limits to prevent OOM. Smart rejection with 507 Insufficient Storage.',
      color: '#fbbf24',
    },
    {
      icon: '🔁',
      title: 'High Availability',
      description: 'Automatic failover and graceful leader resignation. Zero-downtime cluster reconfiguration.',
      color: '#34d399',
    },
    {
      icon: '🌐',
      title: 'Service Discovery',
      description: 'Multiple strategies: Kubernetes, DNS, Gossip, Static. Automatic cluster formation and node joining.',
      color: '#f472b6',
    },
  ];

  let gridRef: HTMLElement;

  onMount(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            anime({
              targets: gridRef.querySelectorAll('.feature-card'),
              opacity: [0, 1],
              translateY: [50, 0],
              scale: [0.95, 1],
              delay: anime.stagger(100),
              duration: 700,
              easing: 'easeOutCubic',
            });
            observer.disconnect();
          }
        });
      },
      { threshold: 0.15 }
    );

    observer.observe(gridRef);
  });
</script>

<div bind:this={gridRef} class="features-grid">
  {#each features as feature}
    <div class="feature-card" style="opacity: 0; --accent: {feature.color};">
      <div class="feature-icon">{feature.icon}</div>
      <h3>{feature.title}</h3>
      <p>{feature.description}</p>
      <div class="feature-glow"></div>
    </div>
  {/each}
</div>

<style>
  .features-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: 1.5rem;
  }

  .feature-card {
    position: relative;
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 12px;
    padding: 1.75rem;
    transition: all 0.3s ease;
    overflow: hidden;
  }

  .feature-card:hover {
    transform: translateY(-4px);
    border-color: var(--accent);
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.3);
  }

  .feature-card:hover .feature-glow {
    opacity: 1;
  }

  .feature-glow {
    position: absolute;
    inset: 0;
    background: radial-gradient(circle at 50% 0%, var(--accent), transparent 70%);
    opacity: 0;
    transition: opacity 0.3s;
    pointer-events: none;
    mix-blend-mode: overlay;
  }

  .feature-icon {
    font-size: 2rem;
    margin-bottom: 0.75rem;
  }

  h3 {
    font-size: 1.125rem;
    font-weight: 700;
    margin-bottom: 0.5rem;
    color: #f1f5f9;
  }

  p {
    color: #94a3b8;
    font-size: 0.9rem;
    line-height: 1.6;
  }

  @media (max-width: 768px) {
    .features-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
