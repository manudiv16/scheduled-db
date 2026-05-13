<script lang="ts">
  import { onMount } from 'svelte';
  import { fade, fly, scale } from 'svelte/transition';
  import { Server, Cpu, Webhook, BarChart3, Layers } from 'lucide-svelte';

  let containerRef: HTMLElement;
  let visible = $state(false);
  let hoveredNode = $state<string | null>(null);

  onMount(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            visible = true;
            observer.disconnect();
          }
        });
      },
      { threshold: 0.2 }
    );
    observer.observe(containerRef);
    return () => observer.disconnect();
  });

  const nodes = {
    client: { id: 'client', label: 'API Clients', detail: 'HTTP / REST', icon: Server, color: '#22d3ee' },
    leader: { id: 'leader', label: 'Node 1', role: 'LEADER', detail: 'Writes + Execution', icon: Layers, color: '#fbbf24' },
    f1: { id: 'f1', label: 'Node 2', role: 'FOLLOWER', detail: 'Log Replication', icon: Layers, color: '#3b82f6' },
    f2: { id: 'f2', label: 'Node 3', role: 'FOLLOWER', detail: 'Log Replication', icon: Layers, color: '#3b82f6' },
    worker: { id: 'worker', label: 'Worker', detail: 'Job Execution', icon: Cpu, color: '#f472b6' },
    webhook: { id: 'webhook', label: 'Webhooks', detail: 'HTTP Callbacks', icon: Webhook, color: '#34d399' },
    metrics: { id: 'metrics', label: 'Metrics', detail: 'Prometheus / OTel', icon: BarChart3, color: '#a78bfa' },
  };
</script>

<div bind:this={containerRef} class="relative mx-auto w-full max-w-3xl select-none">
  {#if visible}
    <!-- Raft Cluster Label -->
    <div class="absolute left-1/2 top-[88px] z-10 -translate-x-1/2 md:top-[108px]">
      <span class="whitespace-nowrap rounded-full border border-brand-cyan/20 bg-brand-bg/80 px-3 py-1 text-[10px] font-bold uppercase tracking-widest text-brand-cyan backdrop-blur-sm md:text-xs">
        Raft Cluster
      </span>
    </div>

    <svg viewBox="0 0 640 520" class="h-auto w-full" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <!-- Arrow marker -->
        <marker id="arrow-cyan" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
          <path d="M0,0 L8,4 L0,8 L2,4 Z" fill="#22d3ee" opacity="0.8" />
        </marker>
        <marker id="arrow-amber" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
          <path d="M0,0 L8,4 L0,8 L2,4 Z" fill="#fbbf24" opacity="0.8" />
        </marker>
        <marker id="arrow-blue" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
          <path d="M0,0 L8,4 L0,8 L2,4 Z" fill="#3b82f6" opacity="0.8" />
        </marker>

        <!-- Glow filter -->
        <filter id="glow" x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur stdDeviation="3" result="coloredBlur" />
          <feMerge>
            <feMergeNode in="coloredBlur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      <!-- CONNECTIONS (rendered first so they're behind nodes) -->

      <!-- Client -> Leader -->
      <line x1="320" y1="50" x2="320" y2="115" stroke="#22d3ee" stroke-width="1.5" opacity="0.3" marker-end="url(#arrow-cyan)" />
      <!-- Animated request dot -->
      <circle r="3" fill="#22d3ee">
        <animate attributeName="cy" values="50;115;50" dur="2.5s" repeatCount="indefinite" />
        <animate attributeName="opacity" values="1;0.3;1" dur="2.5s" repeatCount="indefinite" />
      </circle>

      <!-- Leader -> F1 -->
      <path d="M 320 175 L 320 200 L 180 200 L 180 225" fill="none" stroke="#fbbf24" stroke-width="1.5" opacity="0.25" marker-end="url(#arrow-amber)" />
      <!-- Leader -> F2 -->
      <path d="M 320 175 L 320 200 L 460 200 L 460 225" fill="none" stroke="#fbbf24" stroke-width="1.5" opacity="0.25" marker-end="url(#arrow-amber)" />
      <!-- Log replication dots -->
      <circle r="3" fill="#fbbf24">
        <animate attributeName="opacity" values="0;1;0" dur="1.8s" repeatCount="indefinite" />
        <animate attributeName="cx" values="320;320;180;180" dur="1.8s" repeatCount="indefinite" />
        <animate attributeName="cy" values="175;200;200;225" dur="1.8s" repeatCount="indefinite" />
      </circle>
      <circle r="3" fill="#fbbf24">
        <animate attributeName="opacity" values="0;1;0" dur="1.8s" begin="0.6s" repeatCount="indefinite" />
        <animate attributeName="cx" values="320;320;460;460" dur="1.8s" begin="0.6s" repeatCount="indefinite" />
        <animate attributeName="cy" values="175;200;200;225" dur="1.8s" begin="0.6s" repeatCount="indefinite" />
      </circle>

      <!-- F1 -> Leader ACK -->
      <path d="M 180 285 L 180 310 L 300 310 L 300 330" fill="none" stroke="#3b82f6" stroke-width="1" opacity="0.15" stroke-dasharray="4,4" marker-end="url(#arrow-blue)" />
      <!-- F2 -> Leader ACK -->
      <path d="M 460 285 L 460 310 L 340 310 L 340 330" fill="none" stroke="#3b82f6" stroke-width="1" opacity="0.15" stroke-dasharray="4,4" marker-end="url(#arrow-blue)" />

      <!-- Heartbeat rings (invisible hit area for visual effect) -->
      <circle cx="320" cy="150" r="8" fill="none" stroke="#fbbf24" stroke-width="1" opacity="0">
        <animate attributeName="r" values="8;60" dur="2s" repeatCount="indefinite" />
        <animate attributeName="opacity" values="0.6;0" dur="2s" repeatCount="indefinite" />
      </circle>
      <circle cx="320" cy="150" r="8" fill="none" stroke="#fbbf24" stroke-width="1" opacity="0">
        <animate attributeName="r" values="8;60" dur="2s" begin="1s" repeatCount="indefinite" />
        <animate attributeName="opacity" values="0.6;0" dur="2s" begin="1s" repeatCount="indefinite" />
      </circle>

      <!-- Cluster -> Bottom row -->
      <line x1="180" y1="285" x2="120" y2="370" stroke="#f472b6" stroke-width="1.5" opacity="0.2" marker-end="url(#arrow-cyan)" />
      <line x1="320" y1="285" x2="320" y2="370" stroke="#34d399" stroke-width="1.5" opacity="0.2" marker-end="url(#arrow-cyan)" />
      <line x1="460" y1="285" x2="520" y2="370" stroke="#a78bfa" stroke-width="1.5" opacity="0.2" marker-end="url(#arrow-cyan)" />

      <!-- Execution flow dots -->
      <circle r="3" fill="#f472b6" opacity="0.8">
        <animate attributeName="cy" values="285;370" dur="2s" repeatCount="indefinite" />
        <animate attributeName="cx" values="180;120" dur="2s" repeatCount="indefinite" />
      </circle>
      <circle r="3" fill="#34d399" opacity="0.8">
        <animate attributeName="cy" values="285;370" dur="2s" begin="0.7s" repeatCount="indefinite" />
      </circle>
      <circle r="3" fill="#a78bfa" opacity="0.8">
        <animate attributeName="cy" values="285;370" dur="2s" begin="1.4s" repeatCount="indefinite" />
        <animate attributeName="cx" values="460;520" dur="2s" begin="1.4s" repeatCount="indefinite" />
      </circle>

      <!-- NODES (rendered on top of connections) -->

      <!-- Client Node -->
      <g transform="translate(320, 25)" class="cursor-pointer" onmouseenter={() => hoveredNode = 'client'} onmouseleave={() => hoveredNode = null}>
        {#if visible}
          <g in:scale={{ duration: 500, delay: 100, start: 0.8 }}>
            <rect x="-60" y="-25" width="120" height="50" rx="10" fill="#0f172a" stroke={hoveredNode === 'client' ? '#22d3ee' : '#334155'} stroke-width={hoveredNode === 'client' ? 2 : 1} class="transition-all duration-300" />
            <text x="0" y="-5" text-anchor="middle" fill="#f1f5f9" font-size="12" font-weight="600">API Clients</text>
            <text x="0" y="10" text-anchor="middle" fill="#64748b" font-size="9">HTTP / REST</text>
          </g>
        {/if}
      </g>

      <!-- Leader Node -->
      <g transform="translate(320, 150)" class="cursor-pointer" onmouseenter={() => hoveredNode = 'leader'} onmouseleave={() => hoveredNode = null}>
        {#if visible}
          <g in:scale={{ duration: 500, delay: 300, start: 0.8 }}>
            <!-- Pulse ring -->
            <circle r="42" fill="none" stroke="#fbbf24" stroke-width="1" opacity="0.3">
              <animate attributeName="r" values="38;48" dur="1.5s" repeatCount="indefinite" />
              <animate attributeName="opacity" values="0.4;0" dur="1.5s" repeatCount="indefinite" />
            </circle>
            <rect x="-55" y="-30" width="110" height="60" rx="10" fill="#0f172a" stroke={hoveredNode === 'leader' ? '#fbbf24' : 'rgba(251,191,36,0.3)'} stroke-width={hoveredNode === 'leader' ? 2 : 1} class="transition-all duration-300" />
            <circle cx="35" cy="-20" r="5" fill="#fbbf24" />
            <text x="0" y="-5" text-anchor="middle" fill="#f1f5f9" font-size="12" font-weight="600">Node 1</text>
            <text x="0" y="8" text-anchor="middle" fill="#fbbf24" font-size="8" font-weight="700" letter-spacing="1">LEADER</text>
            <text x="0" y="20" text-anchor="middle" fill="#64748b" font-size="9">Writes + Execution</text>
          </g>
        {/if}
      </g>

      <!-- Follower 1 -->
      <g transform="translate(180, 255)" class="cursor-pointer" onmouseenter={() => hoveredNode = 'f1'} onmouseleave={() => hoveredNode = null}>
        {#if visible}
          <g in:scale={{ duration: 500, delay: 500, start: 0.8 }}>
            <rect x="-55" y="-30" width="110" height="60" rx="10" fill="#0f172a" stroke={hoveredNode === 'f1' ? '#3b82f6' : 'rgba(59,130,246,0.3)'} stroke-width={hoveredNode === 'f1' ? 2 : 1} class="transition-all duration-300" />
            <text x="0" y="-5" text-anchor="middle" fill="#f1f5f9" font-size="12" font-weight="600">Node 2</text>
            <text x="0" y="8" text-anchor="middle" fill="#60a5fa" font-size="8" font-weight="700" letter-spacing="1">FOLLOWER</text>
            <text x="0" y="20" text-anchor="middle" fill="#64748b" font-size="9">Log Replication</text>
          </g>
        {/if}
      </g>

      <!-- Follower 2 -->
      <g transform="translate(460, 255)" class="cursor-pointer" onmouseenter={() => hoveredNode = 'f2'} onmouseleave={() => hoveredNode = null}>
        {#if visible}
          <g in:scale={{ duration: 500, delay: 600, start: 0.8 }}>
            <rect x="-55" y="-30" width="110" height="60" rx="10" fill="#0f172a" stroke={hoveredNode === 'f2' ? '#3b82f6' : 'rgba(59,130,246,0.3)'} stroke-width={hoveredNode === 'f2' ? 2 : 1} class="transition-all duration-300" />
            <text x="0" y="-5" text-anchor="middle" fill="#f1f5f9" font-size="12" font-weight="600">Node 3</text>
            <text x="0" y="8" text-anchor="middle" fill="#60a5fa" font-size="8" font-weight="700" letter-spacing="1">FOLLOWER</text>
            <text x="0" y="20" text-anchor="middle" fill="#64748b" font-size="9">Log Replication</text>
          </g>
        {/if}
      </g>

      <!-- Cluster bracket lines (visual group indicator) -->
      {#if visible}
        <g in:fade={{ duration: 800, delay: 400 }}>
          <path d="M 110 195 L 110 310 Q 110 330 130 330 L 510 330 Q 530 330 530 310 L 530 195" fill="none" stroke="#22d3ee" stroke-width="1" stroke-dasharray="6,4" opacity="0.15" />
        </g>
      {/if}

      <!-- Worker Node -->
      <g transform="translate(120, 395)" class="cursor-pointer" onmouseenter={() => hoveredNode = 'worker'} onmouseleave={() => hoveredNode = null}>
        {#if visible}
          <g in:scale={{ duration: 500, delay: 700, start: 0.8 }}>
            <rect x="-50" y="-25" width="100" height="50" rx="10" fill="#0f172a" stroke={hoveredNode === 'worker' ? '#f472b6' : '#334155'} stroke-width={hoveredNode === 'worker' ? 2 : 1} class="transition-all duration-300" />
            <text x="0" y="-5" text-anchor="middle" fill="#f1f5f9" font-size="12" font-weight="600">Worker</text>
            <text x="0" y="10" text-anchor="middle" fill="#64748b" font-size="9">Job Execution</text>
          </g>
        {/if}
      </g>

      <!-- Webhooks Node -->
      <g transform="translate(320, 395)" class="cursor-pointer" onmouseenter={() => hoveredNode = 'webhook'} onmouseleave={() => hoveredNode = null}>
        {#if visible}
          <g in:scale={{ duration: 500, delay: 800, start: 0.8 }}>
            <rect x="-50" y="-25" width="100" height="50" rx="10" fill="#0f172a" stroke={hoveredNode === 'webhook' ? '#34d399' : '#334155'} stroke-width={hoveredNode === 'webhook' ? 2 : 1} class="transition-all duration-300" />
            <text x="0" y="-5" text-anchor="middle" fill="#f1f5f9" font-size="12" font-weight="600">Webhooks</text>
            <text x="0" y="10" text-anchor="middle" fill="#64748b" font-size="9">HTTP Callbacks</text>
          </g>
        {/if}
      </g>

      <!-- Metrics Node -->
      <g transform="translate(520, 395)" class="cursor-pointer" onmouseenter={() => hoveredNode = 'metrics'} onmouseleave={() => hoveredNode = null}>
        {#if visible}
          <g in:scale={{ duration: 500, delay: 900, start: 0.8 }}>
            <rect x="-50" y="-25" width="100" height="50" rx="10" fill="#0f172a" stroke={hoveredNode === 'metrics' ? '#a78bfa' : '#334155'} stroke-width={hoveredNode === 'metrics' ? 2 : 1} class="transition-all duration-300" />
            <text x="0" y="-5" text-anchor="middle" fill="#f1f5f9" font-size="12" font-weight="600">Metrics</text>
            <text x="0" y="10" text-anchor="middle" fill="#64748b" font-size="9">Prometheus / OTel</text>
          </g>
        {/if}
      </g>
    </svg>

    <!-- Legend / Info panel -->
    {#if hoveredNode}
      <div class="absolute bottom-2 left-1/2 z-20 -translate-x-1/2 rounded-lg border border-brand-border bg-brand-card/90 px-4 py-2 backdrop-blur-sm" transition:fade={{ duration: 200 }}>
        <div class="flex items-center gap-2 text-xs">
          <span class="h-2 w-2 rounded-full" style="background: {nodes[hoveredNode as keyof typeof nodes].color}"></span>
          <span class="font-semibold text-brand-text">{nodes[hoveredNode as keyof typeof nodes].label}</span>
          {#if 'role' in nodes[hoveredNode as keyof typeof nodes]}
            <span class="rounded px-1.5 py-0.5 text-[10px] font-bold uppercase" style="background: {nodes[hoveredNode as keyof typeof nodes].color}20; color: {nodes[hoveredNode as keyof typeof nodes].color}">
              {nodes[hoveredNode as keyof typeof nodes].role}
            </span>
          {/if}
          <span class="text-brand-text-muted">{nodes[hoveredNode as keyof typeof nodes].detail}</span>
        </div>
      </div>
    {/if}
  {/if}
</div>
