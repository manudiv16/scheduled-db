<!-- RoadmapSection.svelte — Muestra las próximas iteraciones del proyecto -->
<!-- Tres áreas principales: DX/Deploy, Clustering y K8s Operator -->
<script lang="ts">
  import { onMount } from 'svelte';
  import anime from 'animejs';

  // Cada card del roadmap con su estado, items y color
  const roadmapItems = [
    {
      title: 'Developer Experience & Deployment',
      icon: 'tools',
      status: 'in-progress',
      color: '#22d3ee',
      items: [
        'Hot-reload y ciclo de desarrollo local mejorado',
        'Skaffold para workflow unificado de desarrollo',
        'Entorno de desarrollo con un solo comando',
        'Mejora de scripts de setup y bootstrap',
      ],
    },
    {
      title: 'Clustering Mejorado',
      icon: 'cluster',
      status: 'planned',
      color: '#a78bfa',
      items: [
        'Cluster sharding para escalado horizontal',
        'Topología flexible (multi-region, edge nodes)',
        'Auto-scaling basado en carga de cola',
        'Mejora de service discovery y gossip protocol',
      ],
    },
    {
      title: 'Kubernetes Operator',
      icon: 'k8s',
      status: 'planned',
      color: '#f472b6',
      items: [
        'CRD ScheduledDBCluster para gestión declarativa',
        'Auto-join de nodos al escalar el StatefulSet',
        'Backup y restore automático de estado Raft',
        'Scaling automático basado en métricas Prometheus',
      ],
    },
  ];

  let containerRef: HTMLElement;

  onMount(() => {
    // Animación de entrada al hacer scroll visible
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            anime({
              targets: containerRef.querySelectorAll('.roadmap-card'),
              opacity: [0, 1],
              translateY: [40, 0],
              delay: anime.stagger(120),
              duration: 600,
              easing: 'easeOutCubic',
            });
            observer.disconnect();
          }
        });
      },
      { threshold: 0.15 }
    );

    observer.observe(containerRef);
  });
</script>

<div bind:this={containerRef} class="roadmap-container">
  {#each roadmapItems as item}
    <div class="roadmap-card" style="opacity: 0; --accent: {item.color};">
      <!-- Cabecera de la card con icono, título y badge de estado -->
      <div class="roadmap-header">
        <div class="roadmap-icon" style="background: {item.color}15; color: {item.color};">
          {#if item.icon === 'tools'}
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M14.7 6.3a1 1 0 000 1.4l1.6 1.6a1 1 0 001.4 0l3.77-3.77a6 6 0 01-7.94 7.94l-6.91 6.91a2.12 2.12 0 01-3-3l6.91-6.91a6 6 0 017.94-7.94l-3.76 3.76z"/>
            </svg>
          {:else if item.icon === 'cluster'}
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="5" r="3"/><circle cx="5" cy="19" r="3"/><circle cx="19" cy="19" r="3"/>
              <line x1="12" y1="8" x2="5" y2="16"/><line x1="12" y1="8" x2="19" y2="16"/>
            </svg>
          {:else}
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/>
            </svg>
          {/if}
        </div>
        <div class="roadmap-title-wrap">
          <h3>{item.title}</h3>
          <span class="status-badge" class:status-in-progress={item.status === 'in-progress'} class:status-planned={item.status === 'planned'}>
            {item.status === 'in-progress' ? 'En progreso' : 'Planeado'}
          </span>
        </div>
      </div>

      <!-- Lista de items del roadmap -->
      <ul class="roadmap-list">
        {#each item.items as listItem, i}
          <li style="opacity: 0;">
            <span class="list-bullet" style="background: {item.color};"></span>
            <span>{listItem}</span>
          </li>
        {/each}
      </ul>
    </div>
  {/each}
</div>

<style>
  .roadmap-container {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 1.5rem;
  }

  .roadmap-card {
    position: relative;
    background: rgba(30, 41, 59, 0.5);
    backdrop-filter: blur(10px);
    border: 1px solid #334155;
    border-radius: 16px;
    padding: 1.75rem;
    transition: all 0.3s ease;
    overflow: hidden;
  }

  .roadmap-card:hover {
    transform: translateY(-4px);
    border-color: var(--accent);
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.3);
  }

  .roadmap-header {
    display: flex;
    align-items: flex-start;
    gap: 1rem;
    margin-bottom: 1.25rem;
  }

  .roadmap-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    border-radius: 12px;
    flex-shrink: 0;
  }

  .roadmap-title-wrap {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    flex: 1;
  }

  .roadmap-title-wrap h3 {
    font-size: 1.05rem;
    font-weight: 700;
    color: #f1f5f9;
    margin: 0;
    line-height: 1.3;
  }

  .status-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.7rem;
    font-weight: 600;
    letter-spacing: 0.03em;
    padding: 0.2rem 0.6rem;
    border-radius: 999px;
    width: fit-content;
    text-transform: uppercase;
  }

  .status-in-progress {
    background: rgba(34, 211, 238, 0.12);
    color: #22d3ee;
    border: 1px solid rgba(34, 211, 238, 0.25);
  }

  .status-in-progress::before {
    content: '';
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #22d3ee;
    animation: pulse-dot 2s ease-in-out infinite;
  }

  .status-planned {
    background: rgba(148, 163, 184, 0.1);
    color: #94a3b8;
    border: 1px solid rgba(148, 163, 184, 0.2);
  }

  .status-planned::before {
    content: '';
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #94a3b8;
  }

  @keyframes pulse-dot {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.4; transform: scale(0.7); }
  }

  .roadmap-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
  }

  .roadmap-list li {
    display: flex;
    align-items: flex-start;
    gap: 0.65rem;
    font-size: 0.875rem;
    color: #94a3b8;
    line-height: 1.5;
  }

  .list-bullet {
    width: 6px;
    height: 6px;
    border-radius: 2px;
    flex-shrink: 0;
    margin-top: 0.4rem;
  }

  @media (max-width: 768px) {
    .roadmap-container {
      grid-template-columns: 1fr;
    }
  }
</style>
